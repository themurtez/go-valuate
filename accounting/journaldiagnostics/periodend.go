package journaldiagnostics

import "time"

// daysBetweenDates returns the whole-day difference between two
// "YYYY-MM-DD" date strings (b - a, positive when b is after a), or
// (0, false) if either fails to parse.
func daysBetweenDates(a, b string) (int, bool) {
	ta, err := time.Parse("2006-01-02", a)
	if err != nil {
		return 0, false
	}
	tb, err := time.Parse("2006-01-02", b)
	if err != nil {
		return 0, false
	}
	return int(tb.Sub(ta).Hours() / 24), true
}

// isPeriodEnd reports whether date falls within policy.PeriodEndDays
// (inclusive) before or on window.EndDate, and not after it — an entry
// dated after the window's own EndDate is out of this period-end window's
// concern (it belongs to whatever later period it falls in).
func isPeriodEnd(date string, window PeriodWindow, policy Policy) (isEnd bool, daysFromEnd int, ok bool) {
	if date == "" || window.EndDate == "" {
		return false, 0, false
	}
	d, ok2 := daysBetweenDates(date, window.EndDate)
	if !ok2 {
		return false, 0, false
	}
	if d < 0 {
		return false, 0, true // after period end
	}
	return d <= policy.PeriodEndDays, d, true
}

// computePeriodEndSummary and findPeriodEndFindings together implement the
// package doc's "Period-end activity" section (10): count, amount, percent
// of total, and material/unusual findings. This package never claims
// period-end entries are improper — see msgMaterialPeriodEndEntry's neutral
// wording.
func computePeriodEndSummary(all []analyzedEntry, window PeriodWindow, policy Policy, totalAmount float64) (PeriodEndSummary, RuleState) {
	if !window.Valid() {
		return PeriodEndSummary{}, RuleUnavailable
	}
	sum := PeriodEndSummary{Available: true}
	for _, a := range all {
		isEnd, _, ok := isPeriodEnd(a.date, window, policy)
		if !ok || !isEnd {
			continue
		}
		sum.PeriodEndEntryCount++
		sum.PeriodEndAmount += a.magnitude
	}
	if totalAmount != 0 {
		sum.PercentOfTotal = AvailableValue(sum.PeriodEndAmount / totalAmount)
	}
	return sum, RuleAvailable
}

// findPeriodEndFindings emits FindingMaterialPeriodEndEntry for each
// period-end entry meeting Policy.MaterialAmount.
func findPeriodEndFindings(all []analyzedEntry, window PeriodWindow, policy Policy) (findings []Finding, state RuleState) {
	if !window.Valid() {
		return nil, RuleUnavailable
	}
	if policy.MaterialAmount <= 0 {
		return nil, RuleDisabled
	}
	for _, a := range all {
		isEnd, days, ok := isPeriodEnd(a.date, window, policy)
		if !ok || !isEnd || a.magnitude < policy.MaterialAmount {
			continue
		}
		findings = append(findings, Finding{
			Code: FindingMaterialPeriodEndEntry, Severity: SeverityWarning,
			Period: a.period, EntryIDs: []string{a.entry.ID}, AccountIDs: entryAccountIDs(a.entry),
			Amount: AvailableValue(a.magnitude),
			Evidence: Evidence{
				EntryAmount: AvailableValue(a.magnitude), EntryDate: a.date,
				PeriodEnd: window.EndDate, DaysFromPeriodEnd: AvailableValue(float64(days)),
			},
			Message: msgMaterialPeriodEndEntry,
		})
	}
	return findings, RuleAvailable
}

// findPostCloseFindings implements section 11: an entry posted (per
// EntryMetadata.PostedAt, preferred, or CreatedAt) after window.CloseDate
// but whose own effective Date falls within the closed window (<=
// window.EndDate) is a post-close entry. Unavailable when CloseDate is not
// supplied (this package never guesses one) or no entry has a usable
// posted timestamp.
func findPostCloseFindings(all []analyzedEntry, window PeriodWindow) (findings []Finding, state RuleState) {
	if window.CloseDate == nil || *window.CloseDate == "" {
		return nil, RuleUnavailable
	}
	closeDate := *window.CloseDate

	anyTimestamp := false
	for _, a := range all {
		if a.hasMeta && (a.meta.PostedAt != nil || a.meta.CreatedAt != nil) {
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
		ts := a.meta.PostedAt
		if ts == nil {
			ts = a.meta.CreatedAt
		}
		if ts == nil {
			continue
		}
		postedDate := ts.Format("2006-01-02")
		if postedDate <= closeDate {
			continue // posted at or before close — not post-close
		}
		if a.date == "" || a.date > window.EndDate {
			continue // entry's own effective date is not within the closed period
		}
		findings = append(findings, Finding{
			Code: FindingPostCloseEntry, Severity: SeverityHigh,
			Period: a.period, EntryIDs: []string{a.entry.ID}, AccountIDs: entryAccountIDs(a.entry),
			Amount: AvailableValue(a.magnitude),
			Evidence: Evidence{
				EntryAmount: AvailableValue(a.magnitude), EntryDate: a.date,
				CloseDate: closeDate, PeriodEnd: window.EndDate,
			},
			Message: msgPostCloseEntry,
		})
	}
	return findings, RuleAvailable
}
