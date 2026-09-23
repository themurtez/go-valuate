package journaldiagnostics

import "time"

// findWeekendFindings implements section 12: an entry whose effective
// timestamp (PostedAt preferred, else CreatedAt — see
// EntryMetadata.effectiveTimestamp) falls on a day not in
// policy.WorkingDays is a weekend/non-working-day entry. A date-only
// JournalEntry.Date is not treated as a midnight posting for this
// purpose — weekend detection reads EntryMetadata's actual timestamp, not
// JournalEntry.Date, since Date alone carries no time-of-day and this
// package never assumes midnight for it (matching the package doc's
// "do not treat date-only journal entries as midnight postings" rule,
// applied here to the weekend check too since inferring a weekday from a
// bare calendar date without knowing whether the caller's Date is even in
// the same convention as a timestamp risks exactly that same mistake for
// timezone-adjacent dates). A weekend entry alone is INFO severity — see
// the package doc's "use combinations for higher review priority" guidance
// (a caller layering Policy.MaterialAmount is expected via a future
// combination rule; V1 reports the base signal only).
func findWeekendFindings(all []analyzedEntry, policy Policy) (findings []Finding, state RuleState) {
	anyTimestamp := false
	for _, a := range all {
		if a.hasMeta && a.meta.effectiveTimestamp() != nil {
			anyTimestamp = true
			break
		}
	}
	if !anyTimestamp {
		return nil, RuleUnavailable
	}

	for _, a := range all {
		if !a.hasMeta {
			continue
		}
		ts := a.meta.effectiveTimestamp()
		if ts == nil {
			continue
		}
		wd := Weekday(ts.Weekday())
		if isWorkingDay(wd, policy.WorkingDays) {
			continue
		}
		findings = append(findings, Finding{
			Code: FindingWeekendEntry, Severity: SeverityInfo,
			Period: a.period, EntryIDs: []string{a.entry.ID}, AccountIDs: entryAccountIDs(a.entry),
			Amount:   AvailableValue(a.magnitude),
			Evidence: Evidence{EntryAmount: AvailableValue(a.magnitude), EntryDate: ts.Format("2006-01-02")},
			Message:  msgWeekendEntry,
		})
	}
	return findings, RuleAvailable
}

// findOutsideBusinessHoursFindings implements section 13: only runs when
// policy.BusinessHours is fully configured (StartHour/EndHour/TimeZone —
// see BusinessHours.configured). A missing/invalid TimeZone makes this
// diagnostic unavailable, never assumed as UTC or host-local. Only entries
// with a precise EntryMetadata timestamp are evaluated — a date-only
// JournalEntry.Date is never treated as a midnight posting (see the
// package doc's explicit instruction on this point).
func findOutsideBusinessHoursFindings(all []analyzedEntry, policy Policy) (findings []Finding, state RuleState) {
	if !policy.BusinessHours.configured() {
		return nil, RuleUnavailable
	}
	loc, err := time.LoadLocation(policy.BusinessHours.TimeZone)
	if err != nil {
		return nil, RuleUnavailable
	}

	anyTimestamp := false
	for _, a := range all {
		if a.hasMeta && a.meta.effectiveTimestamp() != nil {
			anyTimestamp = true
			break
		}
	}
	if !anyTimestamp {
		return nil, RuleUnavailable
	}

	for _, a := range all {
		if !a.hasMeta {
			continue
		}
		ts := a.meta.effectiveTimestamp()
		if ts == nil {
			continue
		}
		local := ts.In(loc)
		hour := local.Hour()
		if hour >= policy.BusinessHours.StartHour && hour < policy.BusinessHours.EndHour {
			continue
		}
		findings = append(findings, Finding{
			Code: FindingOutsideBusinessHours, Severity: SeverityInfo,
			Period: a.period, EntryIDs: []string{a.entry.ID}, AccountIDs: entryAccountIDs(a.entry),
			Amount:   AvailableValue(a.magnitude),
			Evidence: Evidence{EntryAmount: AvailableValue(a.magnitude), EntryDate: local.Format("2006-01-02")},
			Message:  msgOutsideBusinessHours,
		})
	}
	return findings, RuleAvailable
}
