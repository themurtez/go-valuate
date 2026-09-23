package journaldiagnostics

import "sort"

// findThresholdClusterFindings implements section 28: entries whose
// magnitude falls within [LowerPercent, 1.0) * ApprovalThreshold — a band
// just under the threshold, never at or above it (an entry meeting or
// exceeding the threshold is not "clustering below" it) — grouped by date,
// since a cluster of near-threshold entries on unrelated dates is a weaker
// signal than several on the same day. Requires at least
// ThresholdClusterMinCount entries in the band, on the same date, to fire.
// Unavailable when ApprovalThreshold is not supplied — this package never
// invents a business-specific approval limit.
func findThresholdClusterFindings(all []analyzedEntry, policy Policy) (findings []Finding, state RuleState) {
	if policy.ApprovalThreshold <= 0 {
		return nil, RuleUnavailable
	}
	lower := policy.ApprovalThreshold * policy.ThresholdClusterLowerPercent
	upper := policy.ApprovalThreshold

	byDate := make(map[string][]analyzedEntry)
	for _, a := range all {
		if a.magnitude >= lower && a.magnitude < upper {
			byDate[a.date] = append(byDate[a.date], a)
		}
	}
	var dates []string
	for d := range byDate {
		dates = append(dates, d)
	}
	sort.Strings(dates)

	for _, d := range dates {
		members := byDate[d]
		if len(members) < policy.ThresholdClusterMinCount {
			continue
		}
		ids := make([]string, len(members))
		amounts := make([]float64, len(members))
		var combined float64
		for i, m := range members {
			ids[i] = m.entry.ID
			amounts[i] = m.magnitude
			combined += m.magnitude
		}
		ids = sortStrings(ids)
		sort.Float64s(amounts)
		findings = append(findings, Finding{
			Code: FindingThresholdCluster, Severity: SeverityWarning,
			Period: members[0].period, EntryIDs: ids,
			Amount: AvailableValue(combined),
			Evidence: Evidence{
				Threshold: AvailableValue(policy.ApprovalThreshold), IndividualAmounts: amounts,
				CombinedAmount: AvailableValue(combined), EntryIDs: ids, EntryDate: d,
			},
			Message: msgThresholdCluster,
		})
	}
	return findings, RuleAvailable
}

// accountPatternKey builds a deterministic key for an entry's account
// combination — the same sorted-debit/sorted-credit convention
// accountCombinationKey (accounts summaries) uses, reused here as the
// "same/similar account pattern" grouping key for split-entry clustering.
func accountPatternKey(a analyzedEntry) string {
	return accountCombinationKey(a.entry)
}

// findSplitEntryClusterFindings implements section 29: multiple entries on
// the same date, with the same account pattern, each individually below
// ApprovalThreshold, whose combined magnitude meets or exceeds it. When
// EntryMetadata.PreparerID is available for every candidate member, the
// group is further restricted to a single preparer (per the task's "same
// preparer if available" instruction); when preparer data is absent for
// any member, the date+account-pattern grouping alone is used — this is
// intentionally conservative (fewer, more explainable clusters) rather
// than fuzzy-matching accounts or descriptions. Unavailable when
// ApprovalThreshold is not supplied.
func findSplitEntryClusterFindings(all []analyzedEntry, policy Policy) (findings []Finding, state RuleState) {
	if policy.ApprovalThreshold <= 0 {
		return nil, RuleUnavailable
	}

	type groupKey struct{ date, pattern, preparer string }
	groups := make(map[groupKey][]analyzedEntry)
	for _, a := range all {
		if a.magnitude >= policy.ApprovalThreshold {
			continue // only entries individually below the threshold participate
		}
		preparer := ""
		if a.hasMeta {
			preparer = a.meta.PreparerID
		}
		key := groupKey{date: a.date, pattern: accountPatternKey(a), preparer: preparer}
		groups[key] = append(groups[key], a)
	}

	var keys []groupKey
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].date != keys[j].date {
			return keys[i].date < keys[j].date
		}
		if keys[i].pattern != keys[j].pattern {
			return keys[i].pattern < keys[j].pattern
		}
		return keys[i].preparer < keys[j].preparer
	})

	for _, k := range keys {
		members := groups[k]
		if len(members) < policy.ThresholdClusterMinCount {
			continue
		}
		var combined float64
		for _, m := range members {
			combined += m.magnitude
		}
		if combined < policy.ApprovalThreshold {
			continue
		}
		ids := make([]string, len(members))
		amounts := make([]float64, len(members))
		for i, m := range members {
			ids[i] = m.entry.ID
			amounts[i] = m.magnitude
		}
		ids = sortStrings(ids)
		sort.Float64s(amounts)
		findings = append(findings, Finding{
			Code: FindingSplitEntryCluster, Severity: SeverityHigh,
			Period: members[0].period, EntryIDs: ids,
			Amount: AvailableValue(combined),
			Evidence: Evidence{
				Threshold: AvailableValue(policy.ApprovalThreshold), IndividualAmounts: amounts,
				CombinedAmount: AvailableValue(combined), EntryIDs: ids, EntryDate: k.date,
				PreparerID: k.preparer,
			},
			Message: msgSplitEntryCluster,
		})
	}
	return findings, RuleAvailable
}
