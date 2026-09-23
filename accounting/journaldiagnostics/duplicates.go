package journaldiagnostics

import (
	"sort"
	"strconv"
	"strings"
)

// normalizedLineSignature returns one deterministic signature string for a
// single normalized (account ID, debit, credit) triple — amounts formatted
// to a fixed 2-decimal precision so ordinary floating-point noise does not
// defeat matching.
func normalizedLineSignature(accountID string, debit, credit float64) string {
	return accountID + "|" + strconv.FormatFloat(debit, 'f', 2, 64) + "|" + strconv.FormatFloat(credit, 'f', 2, 64)
}

// entrySignature builds the deterministic economic-content signature for an
// entry per the package doc's "Exact duplicate entries" section (23):
// effective date + normalized, sorted lines (account ID, debit, credit).
// EntryID is never included — two entries with different IDs but identical
// economic content produce the same signature by design.
func entrySignature(a analyzedEntry) string {
	lineSigs := make([]string, 0, len(a.entry.Lines))
	for _, l := range a.entry.Lines {
		lineSigs = append(lineSigs, normalizedLineSignature(l.AccountID, l.Debit, l.Credit))
	}
	sort.Strings(lineSigs)
	return a.date + "||" + strings.Join(lineSigs, ";")
}

// contentSignature is entrySignature without the date component — the key
// possible-duplicate (near-duplicate, within a date window) matching uses,
// since exact-duplicate already requires date equality via entrySignature
// itself.
func contentSignature(a analyzedEntry) string {
	lineSigs := make([]string, 0, len(a.entry.Lines))
	for _, l := range a.entry.Lines {
		lineSigs = append(lineSigs, normalizedLineSignature(l.AccountID, l.Debit, l.Credit))
	}
	sort.Strings(lineSigs)
	return strings.Join(lineSigs, ";")
}

// findDuplicateFindings implements sections 23-24: groups analyzed entries
// by normalized signature (via a map, not pairwise comparison — see the
// package doc's benchmark note on avoiding O(N^2) duplicate detection).
// Exact duplicates share full entrySignature (date + content). Possible
// duplicates share only contentSignature and fall within
// policy.DuplicateWindowDays of each other; only entries not already
// claimed by an exact-duplicate group are considered for the possible-
// duplicate pass, so the same pair is never reported under both codes.
func findDuplicateFindings(all []analyzedEntry, policy Policy) (findings []Finding, groups []DuplicateGroup, exactState, possibleState RuleState) {
	if len(all) == 0 {
		return nil, nil, RuleAvailable, RuleUnavailable
	}

	exactGroups := make(map[string][]analyzedEntry)
	for _, a := range all {
		sig := entrySignature(a)
		exactGroups[sig] = append(exactGroups[sig], a)
	}

	claimed := make(map[string]bool) // entry ID -> already in an exact-duplicate group
	var exactSigs []string
	for sig, members := range exactGroups {
		if len(members) < 2 {
			continue
		}
		exactSigs = append(exactSigs, sig)
		for _, m := range members {
			claimed[m.entry.ID] = true
		}
	}
	sort.Strings(exactSigs)
	for _, sig := range exactSigs {
		members := exactGroups[sig]
		findings = append(findings, buildDuplicateFinding(members, FindingExactDuplicateEntry, true, msgExactDuplicateEntry))
		groups = append(groups, buildDuplicateGroup(members, true, sig))
	}

	if policy.DuplicateWindowDays <= 0 {
		return findings, groups, RuleAvailable, RuleDisabled
	}

	byContent := make(map[string][]analyzedEntry)
	for _, a := range all {
		if claimed[a.entry.ID] {
			continue
		}
		sig := contentSignature(a)
		byContent[sig] = append(byContent[sig], a)
	}
	var contentSigs []string
	for sig := range byContent {
		contentSigs = append(contentSigs, sig)
	}
	sort.Strings(contentSigs)

	for _, sig := range contentSigs {
		members := byContent[sig]
		if len(members) < 2 {
			continue
		}
		clusters := clusterByDateWindow(members, policy.DuplicateWindowDays)
		for _, cluster := range clusters {
			if len(cluster) < 2 {
				continue
			}
			findings = append(findings, buildDuplicateFinding(cluster, FindingPossibleDuplicateEntry, false, msgPossibleDuplicateEntry))
			groups = append(groups, buildDuplicateGroup(cluster, false, sig))
		}
	}

	return findings, groups, RuleAvailable, RuleAvailable
}

// clusterByDateWindow groups members (already sharing one content
// signature, sorted by date via the caller's upstream buildPopulation sort)
// into runs where each consecutive pair falls within windowDays — a simple
// deterministic chaining rule (not full pairwise clustering): entry N joins
// the current cluster if it is within windowDays of entry N-1, so a chain
// of near-duplicates spanning more than windowDays end-to-end can still
// form one cluster, consistent with "near-duplicate" being a conservative,
// explainable rule rather than a fuzzy one.
func clusterByDateWindow(members []analyzedEntry, windowDays int) [][]analyzedEntry {
	sorted := make([]analyzedEntry, len(members))
	copy(sorted, members)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].date != sorted[j].date {
			return sorted[i].date < sorted[j].date
		}
		return sorted[i].entry.ID < sorted[j].entry.ID
	})

	var clusters [][]analyzedEntry
	var current []analyzedEntry
	for i, m := range sorted {
		if i == 0 {
			current = []analyzedEntry{m}
			continue
		}
		days, ok := daysBetweenDates(sorted[i-1].date, m.date)
		if ok && days >= 0 && days <= windowDays {
			current = append(current, m)
			continue
		}
		clusters = append(clusters, current)
		current = []analyzedEntry{m}
	}
	if len(current) > 0 {
		clusters = append(clusters, current)
	}
	return clusters
}

func buildDuplicateFinding(members []analyzedEntry, code FindingCode, exact bool, msg string) Finding {
	ids := make([]string, len(members))
	var accountSet []string
	seenAcct := make(map[string]bool)
	for i, m := range members {
		ids[i] = m.entry.ID
		for _, acct := range entryAccountIDs(m.entry) {
			if !seenAcct[acct] {
				seenAcct[acct] = true
				accountSet = append(accountSet, acct)
			}
		}
	}
	ids = sortStrings(ids)
	severity := SeverityInfo
	if exact {
		severity = SeverityWarning
	}
	return Finding{
		Code: code, Severity: severity,
		Period: members[0].period, EntryIDs: ids, AccountIDs: sortStrings(accountSet),
		Amount: AvailableValue(members[0].magnitude),
		Evidence: Evidence{
			NormalizedSignature: entrySignature(members[0]),
			EntryIDs:            ids,
			EntryAmount:         AvailableValue(members[0].magnitude),
		},
		Message: msg,
	}
}

func buildDuplicateGroup(members []analyzedEntry, exact bool, signature string) DuplicateGroup {
	ids := make([]string, len(members))
	dates := make([]string, len(members))
	var amount float64
	for i, m := range members {
		ids[i] = m.entry.ID
		dates[i] = m.date
		amount += m.magnitude
	}
	sort.Strings(dates)
	return DuplicateGroup{
		Exact: exact, NormalizedSignature: signature, EntryIDs: sortStrings(ids),
		EarliestDate: dates[0], LatestDate: dates[len(dates)-1], Amount: amount,
	}
}

// findRepeatedIdenticalAmountFindings implements section 25: entries
// sharing the same EntryMagnitude (rounded to cents), with magnitude >=
// policy.RepeatedAmountMinAmount, at least policy.MinRepeatedAmountCount of
// them. An entry whose EntryMetadata.Source == SourceRecurring is excluded
// from this rule — see the package doc's "do not conflate legitimate
// recurring journals with suspicious duplication" instruction; a caller
// wanting a full recurring-entry inventory instead uses SourceSummary.
func findRepeatedIdenticalAmountFindings(all []analyzedEntry, policy Policy) (findings []Finding, state RuleState) {
	if policy.RepeatedAmountMinAmount <= 0 || policy.MinRepeatedAmountCount <= 1 {
		return nil, RuleDisabled
	}

	byAmount := make(map[string][]analyzedEntry)
	for _, a := range all {
		if a.magnitude < policy.RepeatedAmountMinAmount {
			continue
		}
		if a.hasMeta && a.meta.Source == SourceRecurring {
			continue
		}
		key := strconv.FormatFloat(a.magnitude, 'f', 2, 64)
		byAmount[key] = append(byAmount[key], a)
	}

	var keys []string
	for k := range byAmount {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		members := byAmount[key]
		if len(members) < policy.MinRepeatedAmountCount {
			continue
		}
		ids := make([]string, len(members))
		for i, m := range members {
			ids[i] = m.entry.ID
		}
		ids = sortStrings(ids)
		findings = append(findings, Finding{
			Code: FindingRepeatedIdenticalAmount, Severity: SeverityInfo,
			Period: members[0].period, EntryIDs: ids,
			Amount:   AvailableValue(members[0].magnitude),
			Evidence: Evidence{EntryAmount: AvailableValue(members[0].magnitude), EntryIDs: ids},
			Message:  msgRepeatedIdenticalAmount,
		})
	}
	return findings, RuleAvailable
}
