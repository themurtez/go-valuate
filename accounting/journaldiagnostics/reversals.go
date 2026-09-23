package journaldiagnostics

import "sort"

// explicitReversalPairs finds every explicit original/reversal relationship
// within all, using only ledger.Reversal (never equal-and-opposite amounts
// — see the package doc's "Reversals" section). Both entries must be in
// the analyzed population (an entry excluded by validation or out of
// posted-status scope cannot participate). Returns pairs sorted by
// (original date, original EntryID) for determinism.
func explicitReversalPairs(all []analyzedEntry, policy Policy) []ReversalPair {
	byID := make(map[string]analyzedEntry, len(all))
	for _, a := range all {
		byID[a.entry.ID] = a
	}

	var pairs []ReversalPair
	for _, a := range all {
		revID := a.entry.Reversal.ReversedByEntryID
		if revID == "" {
			continue
		}
		rev, ok := byID[revID]
		if !ok {
			continue // reversing entry not in analyzed population (excluded/out of scope)
		}
		if rev.entry.Reversal.ReversalOfEntryID != a.entry.ID {
			continue // ledger validation already flags this inconsistency separately
		}
		days, hasDays := daysBetweenDates(a.date, rev.date)
		pair := ReversalPair{
			OriginalEntryID: a.entry.ID, ReversingEntryID: rev.entry.ID,
			OriginalDate: a.date, ReversingDate: rev.date, Amount: a.magnitude,
		}
		if hasDays {
			pair.DaysBetween = AvailableValue(float64(days))
			pair.Rapid = days >= 0 && days <= policy.RapidReversalDays
		}
		if a.period != "" && rev.period != "" && a.period != rev.period {
			pair.CrossPeriod = true
		}
		pairs = append(pairs, pair)
	}

	sort.SliceStable(pairs, func(i, j int) bool {
		if pairs[i].OriginalDate != pairs[j].OriginalDate {
			return pairs[i].OriginalDate < pairs[j].OriginalDate
		}
		return pairs[i].OriginalEntryID < pairs[j].OriginalEntryID
	})
	return pairs
}

// computeReversalSummary and findReversalFindings implement section 26.
// Available whenever any analyzed entry declares a Reversal relationship —
// an empty Ledger.Entries reversal set is a legitimate "no reversals this
// period" result, distinguished from "reversal analysis could not run" by
// always returning Available: true here (reversal detection needs no
// optional metadata, only ledger.Reversal, which is always structurally
// present on JournalEntry).
func computeReversalSummary(all []analyzedEntry, policy Policy) ReversalSummary {
	pairs := explicitReversalPairs(all, policy)
	return ReversalSummary{Available: true, ReversalCount: len(pairs), Pairs: pairs}
}

func findReversalFindings(all []analyzedEntry, policy Policy) (findings []Finding, state RuleState) {
	pairs := explicitReversalPairs(all, policy)
	for _, p := range pairs {
		if p.Rapid {
			findings = append(findings, Finding{
				Code: FindingRapidReversal, Severity: SeverityWarning,
				Period: periodFor(all, p.OriginalEntryID), EntryIDs: sortStrings([]string{p.OriginalEntryID, p.ReversingEntryID}),
				Amount: AvailableValue(p.Amount),
				Evidence: Evidence{
					OriginalEntryID: p.OriginalEntryID, ReversingEntryID: p.ReversingEntryID,
					DaysBetween: p.DaysBetween, EntryAmount: AvailableValue(p.Amount), EntryDate: p.OriginalDate,
				},
				Message: msgRapidReversal,
			})
		}
		if p.CrossPeriod {
			findings = append(findings, Finding{
				Code: FindingCrossPeriodReversal, Severity: SeverityInfo,
				Period: periodFor(all, p.OriginalEntryID), EntryIDs: sortStrings([]string{p.OriginalEntryID, p.ReversingEntryID}),
				Amount: AvailableValue(p.Amount),
				Evidence: Evidence{
					OriginalEntryID: p.OriginalEntryID, ReversingEntryID: p.ReversingEntryID,
					OriginalPeriod: periodFor(all, p.OriginalEntryID), ReversingPeriod: periodFor(all, p.ReversingEntryID),
					EntryAmount: AvailableValue(p.Amount), EntryDate: p.OriginalDate,
				},
				Message: msgCrossPeriodReversal,
			})
		}
	}
	return findings, RuleAvailable
}

func periodFor(all []analyzedEntry, entryID string) string {
	for _, a := range all {
		if a.entry.ID == entryID {
			return a.period
		}
	}
	return ""
}

// findPeriodEndEarlyReversalFindings implements section 27: a material
// entry posted near period end (per isPeriodEnd) that was explicitly
// reversed within policy.RapidReversalDays of period start (the analysis
// window's own start — PeriodWindow.StartDate — of the *next* period is
// not knowable from one PeriodWindow, so this rule instead measures the
// reversal's proximity to the original's own period-end date, which is the
// deterministic signal actually available: "posted near period end and
// reversed shortly after"). No motive is inferred — see
// msgPeriodEndEntryWithEarlyReversal's neutral wording.
func findPeriodEndEarlyReversalFindings(all []analyzedEntry, window PeriodWindow, policy Policy) (findings []Finding, state RuleState) {
	if !window.Valid() || policy.MaterialAmount <= 0 {
		return nil, RuleUnavailable
	}
	pairs := explicitReversalPairs(all, policy)
	byOriginal := make(map[string]analyzedEntry, len(all))
	for _, a := range all {
		byOriginal[a.entry.ID] = a
	}

	for _, p := range pairs {
		orig, ok := byOriginal[p.OriginalEntryID]
		if !ok || orig.magnitude < policy.MaterialAmount {
			continue
		}
		isEnd, days, ok := isPeriodEnd(orig.date, window, policy)
		if !ok || !isEnd {
			continue
		}
		if !p.DaysBetween.Available || p.DaysBetween.Amount > float64(policy.RapidReversalDays) {
			continue
		}
		findings = append(findings, Finding{
			Code: FindingPeriodEndEntryWithEarlyReversal, Severity: SeverityHigh,
			Period: orig.period, EntryIDs: sortStrings([]string{p.OriginalEntryID, p.ReversingEntryID}),
			AccountIDs: entryAccountIDs(orig.entry),
			Amount:     AvailableValue(orig.magnitude),
			Evidence: Evidence{
				OriginalEntryID: p.OriginalEntryID, ReversingEntryID: p.ReversingEntryID,
				EntryAmount: AvailableValue(orig.magnitude), EntryDate: orig.date,
				PeriodEnd: window.EndDate, DaysFromPeriodEnd: AvailableValue(float64(days)),
				DaysBetween: p.DaysBetween,
			},
			Message: msgPeriodEndEntryWithEarlyReversal,
		})
	}
	return findings, RuleAvailable
}
