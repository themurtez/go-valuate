package reconciliation

// Calculate runs one deterministic reconciliation pass over in and
// returns a Result. It does not mutate in or any slice/map it contains
// — see immutability_test.go. Calling Calculate repeatedly with
// identical in always returns byte-for-byte identical JSON — see
// determinism_test.go — and concurrent calls with distinct in values are
// safe — see concurrency_test.go.
//
// Overview of the pipeline (see docs/ACCOUNT_RECONCILIATION.md for the
// full walkthrough):
//
//  1. Validate the reconciliation key/AsOfDate/Policy.
//  2. Deduplicate and validate BookItems/ExternalItems/ReconcilingItems.
//  3. Resolve Balance for each side (supplied and/or derived).
//  4. Apply ConfirmedMatches (highest precedence — never displaced).
//  5. Run automatic matching by precedence (reference+amount+date, then
//     amount+date, then amount+window).
//  6. Run bounded composite matching, if enabled.
//  7. Compute the reconciliation equation from resolved balances and
//     ReconcilingItems.
//  8. Compute coverage, match-quality, aging/staleness, and Findings.
//  9. Derive Status.
func Calculate(in Input) Result {
	var issues []Issue

	issues = append(issues, validatePolicy(in.Policy)...)

	keyValid := validReconciliationKey(in.AccountID)
	if !keyValid {
		issues = append(issues, Issue{Code: IssueInvalidReconciliationKey, Severity: IssueSeverityError, Message: "account_id is required"})
	}
	_, asOfValid := parseDate(in.AsOfDate)
	if !asOfValid {
		issues = append(issues, Issue{Code: IssueInvalidAsOfDate, Severity: IssueSeverityError, Message: "as_of_date is required and must be YYYY-MM-DD"})
	}

	reportingCurrency := in.Policy.ReportingCurrency
	book, bookIssues := dedupBookItems(in.BookItems, reportingCurrency)
	issues = append(issues, bookIssues...)
	external, externalIssues := dedupExternalItems(in.ExternalItems, reportingCurrency)
	issues = append(issues, externalIssues...)
	reconcilingItems, reconcilingIssues := dedupReconcilingItems(in.ReconcilingItems)
	issues = append(issues, reconcilingIssues...)

	bookByID := make(map[string]BookItem, len(book))
	for _, b := range book {
		bookByID[b.ItemID] = b
	}
	externalByID := make(map[string]ExternalItem, len(external))
	for _, e := range external {
		externalByID[e.ItemID] = e
	}

	// Balances.
	tolerance := in.Policy.balanceTolerance()
	bookOriented := make([]float64, len(book))
	for i, b := range book {
		bookOriented[i] = b.SignedAmount()
	}
	externalOriented := make([]float64, len(external))
	for i, e := range external {
		externalOriented[i] = e.OrientedSignedAmount(in.Policy.orientation())
	}
	bookBalance, bookBalIssues := resolveBalance(in.BookBalance, bookOriented, in.Policy.DeriveBalances, tolerance, 1)
	issues = append(issues, bookBalIssues...)
	externalBalance, extBalIssues := resolveBalance(in.ExternalBalance, externalOriented, in.Policy.DeriveBalances, tolerance, orientationMultiplier(in.Policy.orientation()))
	issues = append(issues, extBalIssues...)

	// Matching.
	state := newMatchState(len(book), len(external))
	var groups []MatchGroup
	var ambiguousCandidates []Candidate

	confirmedAccepted, confirmedIssues := validateConfirmedMatches(in.ConfirmedMatches, bookByID, externalByID, in.Policy)
	issues = append(issues, confirmedIssues...)
	for i, m := range confirmedAccepted {
		var bookSum, extSum float64
		for _, id := range m.BookItemIDs {
			bookSum += bookByID[id].SignedAmount()
			state.consumeBook(id)
		}
		for _, id := range m.ExternalItemIDs {
			extSum += externalByID[id].OrientedSignedAmount(in.Policy.orientation())
			state.consumeExternal(id)
		}
		matchID := m.MatchID
		if matchID == "" {
			matchID = confirmedMatchFallbackRef(i)
		}
		groups = append(groups, MatchGroup{
			MatchID:         matchID,
			BookItemIDs:     append([]string(nil), m.BookItemIDs...),
			ExternalItemIDs: append([]string(nil), m.ExternalItemIDs...),
			BookAmount:      bookSum,
			ExternalAmount:  extSum,
			Difference:      bookSum - extSum,
			MatchType:       confirmedMatchType(m),
			MatchReason:     ReasonConfirmed,
			Confidence:      confirmedConfidence(m),
		})
	}

	// Automatic matching over whatever confirmed matches did not consume.
	remainingBook := remainingBookItems(book, state)
	remainingExternal := remainingExternalItems(external, state)
	autoGroups, autoAmbiguous := runAutoMatching(remainingBook, remainingExternal, in.Policy, state)
	groups = append(groups, autoGroups...)
	ambiguousCandidates = append(ambiguousCandidates, autoAmbiguous...)

	// Composite matching over whatever auto matching did not consume.
	remainingBook = remainingBookItems(book, state)
	remainingExternal = remainingExternalItems(external, state)
	compositeGroups, compositeAmbiguous, compositeIssues := runCompositeMatching(remainingBook, remainingExternal, in.Policy, state)
	groups = append(groups, compositeGroups...)
	ambiguousCandidates = append(ambiguousCandidates, compositeAmbiguous...)
	issues = append(issues, compositeIssues...)

	sortMatchGroups(groups)
	assignMatchIDs(groups)

	// Unmatched items = book/external items never consumed by any group.
	matchedBookIDs := make(map[string]bool)
	matchedExternalIDs := make(map[string]bool)
	for _, g := range groups {
		for _, id := range g.BookItemIDs {
			matchedBookIDs[id] = true
		}
		for _, id := range g.ExternalItemIDs {
			matchedExternalIDs[id] = true
		}
	}
	var unmatchedBookIDs, unmatchedExternalIDs []string
	for _, b := range book {
		if !matchedBookIDs[b.ItemID] {
			unmatchedBookIDs = append(unmatchedBookIDs, b.ItemID)
		}
	}
	for _, e := range external {
		if !matchedExternalIDs[e.ItemID] {
			unmatchedExternalIDs = append(unmatchedExternalIDs, e.ItemID)
		}
	}
	sortByID(unmatchedBookIDs)
	sortByID(unmatchedExternalIDs)

	// Items that appear as an ambiguous anchor or candidate counterpart
	// are excluded from UnmatchedBookItems/UnmatchedExternalItems'
	// ordinary "unmatched" classification for match-quality purposes
	// (they get their own "ambiguous" bucket — see
	// buildMatchQualitySummary) but remain listed in
	// Result.UnmatchedBookItems/UnmatchedExternalItems, since an
	// ambiguous item is still, factually, unmatched — task section 9's
	// "never drop unmatched rows" and section 22's ambiguity handling are
	// not in tension: ambiguous items are a labeled SUBSET of unmatched,
	// not a separately-hidden population.
	ambiguousItemIDs := make(map[string]bool)
	for _, c := range ambiguousCandidates {
		if c.BookItemID != "" {
			ambiguousItemIDs[c.BookItemID] = true
		}
		for _, id := range c.BookItemIDs {
			ambiguousItemIDs[id] = true
		}
		if c.ExternalItemID != "" {
			ambiguousItemIDs[c.ExternalItemID] = true
		}
		for _, id := range c.ExternalItemIDs {
			ambiguousItemIDs[id] = true
		}
	}

	// Equation.
	equation := applyReconciliationEquation(bookBalance, externalBalance, reconcilingItems, tolerance)

	// Coverage / match quality / summaries.
	coverage := buildCoverage(book, external, matchedBookIDs, matchedExternalIDs, bookBalance.EndingAvailable, externalBalance.EndingAvailable)
	matchQuality := buildMatchQualitySummary(book, external, groups, unmatchedBookIDs, unmatchedExternalIDs, bookByID, externalByID, ambiguousItemIDs, in.Policy)
	matchSummaryBook := buildMatchSummaryBook(book, matchedBookIDs)
	matchSummaryExternal := buildMatchSummaryExternal(external, matchedExternalIDs, in.Policy.orientation())

	// Aging.
	var agedBook, agedExternal, agedReconciling []AgedItem
	for _, id := range unmatchedBookIDs {
		if d, ok := daysOutstanding(bookByID[id].Date, in.AsOfDate); ok {
			agedBook = append(agedBook, AgedItem{ItemID: id, DaysOutstanding: d, Bucket: bucketFor(d, in.AgingBuckets)})
		}
	}
	for _, id := range unmatchedExternalIDs {
		if d, ok := daysOutstanding(externalByID[id].Date, in.AsOfDate); ok {
			agedExternal = append(agedExternal, AgedItem{ItemID: id, DaysOutstanding: d, Bucket: bucketFor(d, in.AgingBuckets)})
		}
	}
	for _, ri := range reconcilingItems {
		if d, ok := daysOutstanding(ri.Date, in.AsOfDate); ok {
			agedReconciling = append(agedReconciling, AgedItem{ItemID: ri.ItemID, DaysOutstanding: d, Bucket: bucketFor(d, in.AgingBuckets)})
		}
	}
	sortAgedItems(agedBook)
	sortAgedItems(agedExternal)
	sortAgedItems(agedReconciling)

	// Items already explained by an explicit ReconcilingItem are excluded
	// from the material-unmatched Finding — see
	// buildMaterialUnmatchedFindings's doc comment.
	explainedItemIDs := make(map[string]bool, len(reconcilingItems))
	for _, ri := range reconcilingItems {
		if ri.RelatedItemID != "" {
			explainedItemIDs[ri.RelatedItemID] = true
		}
	}

	// Findings.
	var findings []Finding
	findings = append(findings, buildBalanceFindings(equation, tolerance)...)
	findings = append(findings, buildRollforwardFindings(bookBalance, externalBalance)...)
	materialUnmatched := buildMaterialUnmatchedFindings(book, external, unmatchedBookIDs, unmatchedExternalIDs, bookByID, externalByID, explainedItemIDs, in.Policy)
	findings = append(findings, materialUnmatched...)
	ambiguousFindings := buildAmbiguousFindings(ambiguousCandidates)
	findings = append(findings, ambiguousFindings...)
	findings = append(findings, staleUnmatchedFindings(agedBook, agedExternal, in.Policy.StaleDaysThreshold)...)
	findings = append(findings, staleReconcilingFindings(agedReconciling, in.Policy.StaleDaysThreshold)...)
	findings = append(findings, buildMaterialReconcilingItemsFinding(equation, in.Policy)...)
	sortFindings(findings)

	status := deriveStatus(statusInputs{
		reconciliationKeyValid:         keyValid,
		asOfDateValid:                  asOfValid,
		balanceDataSufficient:          equation.Available || len(book) > 0 || len(external) > 0,
		equationAvailable:              equation.Available,
		withinTolerance:                equation.WithinTolerance,
		hasUnresolvedMaterialUnmatched: len(materialUnmatched) > 0,
		hasUnresolvedReconcilingItems:  len(reconcilingItems) > 0,
		hasAmbiguousMatches:            len(ambiguousFindings) > 0,
	})

	sortIssues(issues)

	return Result{
		AccountID:                  in.AccountID,
		ExternalAccountID:          in.ExternalAccountID,
		Period:                     in.Period,
		AsOfDate:                   in.AsOfDate,
		Type:                       in.effectiveType(),
		Status:                     status,
		Equation:                   equation,
		BookBalance:                bookBalance,
		ExternalBalance:            externalBalance,
		TransactionModeAvailable:   len(book) > 0 || len(external) > 0,
		MatchedGroups:              groups,
		UnmatchedBookItems:         unmatchedBookIDs,
		UnmatchedExternalItems:     unmatchedExternalIDs,
		AmbiguousCandidates:        ambiguousCandidates,
		ReconcilingItems:           reconcilingItems,
		AgedUnmatchedBookItems:     agedBook,
		AgedUnmatchedExternalItems: agedExternal,
		AgedReconcilingItems:       agedReconciling,
		MatchSummaryBook:           matchSummaryBook,
		MatchSummaryExternal:       matchSummaryExternal,
		MatchQuality:               matchQuality,
		Coverage:                   coverage,
		Findings:                   findings,
		Issues:                     issues,
		Versions:                   currentVersions(),
	}
}

func remainingBookItems(book []BookItem, state *matchState) []BookItem {
	out := make([]BookItem, 0, len(book))
	for _, b := range book {
		if state.bookAvailable(b.ItemID) {
			out = append(out, b)
		}
	}
	return out
}

func remainingExternalItems(external []ExternalItem, state *matchState) []ExternalItem {
	out := make([]ExternalItem, 0, len(external))
	for _, e := range external {
		if state.externalAvailable(e.ItemID) {
			out = append(out, e)
		}
	}
	return out
}

func confirmedMatchType(m ConfirmedMatch) MatchType {
	if len(m.BookItemIDs) <= 1 && len(m.ExternalItemIDs) <= 1 {
		return MatchOneToOne
	}
	return MatchManualGroup
}

func confirmedConfidence(m ConfirmedMatch) MatchConfidence {
	if m.ExplainedDifference != nil {
		return ConfidenceManual
	}
	return ConfidenceExact
}
