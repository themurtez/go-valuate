package kpi

// Period is one caller-supplied explicit period — task section 4. This
// package never infers a fiscal calendar and never calls time.Now(); every
// computation operates only on Periods the caller explicitly lists in
// Input.Periods, each evaluated independently. There is no implicit
// forward-fill or backfill across periods a caller did not supply.
//
// StartDate/EndDate/Days are plain strings/ints rather than time.Time, by
// design: this package performs no date arithmetic of its own (a caller
// wanting duration-sensitive KPIs, e.g. an annualized rate, passes that
// already-resolved figure as a MetricValue or a Days-driven aggregation
// input) — Days exists only so downstream analytics that DO need a day
// count (e.g. a caller-authored WEIGHTED_AVERAGE weight) has one without
// this package parsing dates itself.
type Period struct {
	// Code is this period's stable identity (e.g. "2025-06", "2025-Q2",
	// "FY2025") — the string every MetricValue.Period and every Expression
	// time-series reference (CURRENT, PRIOR_PERIOD, ...) matches against.
	// Required, non-empty, unique within Input.Periods.
	Code string `json:"code"`
	// Sequence orders Periods chronologically for PRIOR_PERIOD/
	// PRIOR_YEAR_SAME_PERIOD/TRAILING_N/trend evaluation — task section
	// 4's "caller supplies ordered periods" instruction. This package
	// never infers chronological order from Code's string form (e.g. it
	// never assumes "2025-10" < "2025-9" sorts correctly as text).
	// Required to be strictly increasing across distinct Periods in one
	// Input.Periods slice (ties are an IssueInvalidPeriod).
	Sequence int `json:"sequence"`
	// StartDate/EndDate are caller-supplied ISO-8601 date strings
	// (YYYY-MM-DD), optional and carried through for display/provenance
	// only — this package performs no date parsing or arithmetic on them.
	StartDate string `json:"start_date,omitempty"`
	EndDate   string `json:"end_date,omitempty"`
	// Days is the caller-supplied day count for this period, used only as
	// an optional weight source for WEIGHTED_AVERAGE-style aggregation a
	// Definition explicitly configures — never inferred from
	// StartDate/EndDate.
	Days int `json:"days,omitempty"`
	// FiscalYear is a caller-supplied opaque label used only to resolve
	// PRIOR_YEAR_SAME_PERIOD references (task section 20: "prior-year
	// requires caller period metadata sufficient to identify comparable
	// periods... do not parse semantics from arbitrary period strings").
	// Two Periods are "the same position in different fiscal years" only
	// when SamePositionKey (see below) matches; FiscalYear is one of the
	// two inputs to that key.
	FiscalYear string `json:"fiscal_year,omitempty"`
	// PositionInYear is a caller-supplied opaque label identifying this
	// Period's position within its fiscal year (e.g. "Q2", "06") — the
	// second input to SamePositionKey. Left empty, PRIOR_YEAR_SAME_PERIOD
	// is never resolvable for this Period (explicit unavailability, not a
	// guess).
	PositionInYear string `json:"position_in_year,omitempty"`
}

// samePositionKey returns the (FiscalYear, PositionInYear) pair
// PRIOR_YEAR_SAME_PERIOD matching uses, or ok=false if either half is
// empty.
func (p Period) samePositionKey() (key [2]string, ok bool) {
	if p.FiscalYear == "" || p.PositionInYear == "" {
		return key, false
	}
	return [2]string{p.FiscalYear, p.PositionInYear}, true
}
