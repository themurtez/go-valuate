package ledger

// PeriodRange selects entries by Period and/or Date, for balance/trial-
// balance calculation. All set fields are ANDed together. A caller wanting
// "everything up to and including a date" leaves StartDate empty and sets
// only EndDate, and so on. This package never guesses fiscal-period
// boundaries — see the package doc comment's period-handling section; a
// caller wanting fiscal-year-aware filtering supplies Periods explicitly
// (e.g. resolved via financial/metrics.PeriodInfo upstream) rather than
// this package inferring one from Date strings.
type PeriodRange struct {
	// Periods, if non-empty, restricts to entries whose Period is in this
	// set. Order does not matter for matching.
	Periods []string `json:"periods,omitempty"`
	// StartDate, if non-empty, restricts to entries whose Date is
	// lexically >= StartDate (works correctly for "YYYY-MM-DD" dates, per
	// JournalEntry.Date's doc comment).
	StartDate string `json:"start_date,omitempty"`
	// EndDate, if non-empty, restricts to entries whose Date is lexically
	// <= EndDate.
	EndDate string `json:"end_date,omitempty"`
}

// Valid reports whether r's StartDate/EndDate are consistent (StartDate <=
// EndDate when both are set). Callers should check this before filtering
// and surface IssueInvalidPeriod if false, rather than silently returning
// an empty result — see ValidateRange.
func (r PeriodRange) Valid() bool {
	if r.StartDate != "" && r.EndDate != "" && r.StartDate > r.EndDate {
		return false
	}
	return true
}

// ValidateRange returns an IssueInvalidPeriod Issue if r is structurally
// invalid (see PeriodRange.Valid), or nil otherwise.
func ValidateRange(r PeriodRange) []Issue {
	if r.Valid() {
		return nil
	}
	return []Issue{{
		Code:     IssueInvalidPeriod,
		Severity: SeverityError,
		Message:  "period range has StartDate (" + r.StartDate + ") after EndDate (" + r.EndDate + ")",
	}}
}

// Matches reports whether entry e falls within r. An empty PeriodRange
// (zero value) matches every entry. When r.Periods is non-empty, e.Period
// must be a member. When r.StartDate/EndDate are set, e.Date must be
// non-empty and within range — an entry with no Date never matches a
// date-bounded range, even if r.Periods is also empty (a caller filtering
// by date range is asking a date-shaped question; an entry with no date
// cannot answer it).
func (r PeriodRange) Matches(e JournalEntry) bool {
	if len(r.Periods) > 0 {
		found := false
		for _, p := range r.Periods {
			if e.Period == p {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if r.StartDate != "" || r.EndDate != "" {
		if e.Date == "" {
			return false
		}
		if r.StartDate != "" && e.Date < r.StartDate {
			return false
		}
		if r.EndDate != "" && e.Date > r.EndDate {
			return false
		}
	}
	return true
}

// before reports whether entry e's Date/Period falls strictly before
// asOf's boundary — used to split entries into "opening" (before the
// range) vs. "in range" for balance calculation. An entry is "before" the
// range only when it has a Date and that Date is < the range's StartDate;
// an entry with no Date, or when the range has no StartDate, is never
// treated as an opening-period entry by date. This mirrors Matches' rule
// that a dateless entry cannot answer a date-shaped question.
func (r PeriodRange) before(e JournalEntry) bool {
	if r.StartDate == "" || e.Date == "" {
		return false
	}
	return e.Date < r.StartDate
}
