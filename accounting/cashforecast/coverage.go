package cashforecast

import "time"

// SourceCoverage reports whether one upstream category (AR/AP/Payroll/
// DebtService/Tax/RecurringOpex/Capex) was supplied at all, and — for
// AR/AP specifically — what percentage of the known open amount was
// actually scheduled into a cash event. This package never invents a
// universal completeness score; every figure here is a factual count or
// percentage — see the task's section 41.
type SourceCoverage struct {
	// Supplied is true if the caller provided any data at all for this
	// category (a source Result, a direct schedule/plan, or at least one
	// directly-supplied CashFlowEvent of the matching category).
	Supplied bool `json:"supplied"`
	// ScheduledPercent is, for AR/AP only, the fraction (0-1) of total
	// known open amount that was scheduled into a cash event. Unavailable
	// for categories with no "open amount" concept (Payroll/DebtService/
	// Tax/RecurringOpex/Capex) — those only ever report Supplied.
	ScheduledPercent AmountValue `json:"scheduled_percent"`
}

// StalenessInfo reports one source's as-of-date age, when both the as-of
// date and a threshold are available. This package never infers a
// staleness threshold — see StalenessPolicy's doc comment.
type StalenessInfo struct {
	Available bool   `json:"available"`
	AsOfDate  string `json:"as_of_date,omitempty"`
	AgeDays   int    `json:"age_days,omitempty"`
	// ThresholdDays is the caller-supplied Max*AgeDays this was compared
	// against, 0 if no threshold was supplied (in which case Stale is
	// always false — age is reported, but "stale" requires an explicit
	// policy).
	ThresholdDays int  `json:"threshold_days,omitempty"`
	Stale         bool `json:"stale"`
}

func buildStaleness(asOf time.Time, forecastStart time.Time, thresholdDays int) StalenessInfo {
	if asOf.IsZero() {
		return StalenessInfo{}
	}
	age := int(truncateToDate(forecastStart).Sub(truncateToDate(asOf)).Hours() / 24)
	if age < 0 {
		age = 0
	}
	info := StalenessInfo{Available: true, AsOfDate: asOf.Format("2006-01-02"), AgeDays: age}
	if thresholdDays > 0 {
		info.ThresholdDays = thresholdDays
		info.Stale = age > thresholdDays
	}
	return info
}

// Coverage is the task's section 41 completeness report plus section 43
// staleness metadata, gathered in one place.
type Coverage struct {
	OpeningCashSupplied bool `json:"opening_cash_supplied"`

	AR            SourceCoverage `json:"ar"`
	AP            SourceCoverage `json:"ap"`
	Payroll       SourceCoverage `json:"payroll"`
	DebtService   SourceCoverage `json:"debt_service"`
	Tax           SourceCoverage `json:"tax"`
	RecurringOpex SourceCoverage `json:"recurring_opex"`
	Capex         SourceCoverage `json:"capex"`

	ARStaleness   StalenessInfo `json:"ar_staleness"`
	APStaleness   StalenessInfo `json:"ap_staleness"`
	CashStaleness StalenessInfo `json:"cash_staleness"`

	// RequiredInputsMissing lists every RequiredInputs category the caller
	// marked required but which Supplied == false for, in a fixed order
	// (AR, AP, Payroll, DebtService, Tax).
	RequiredInputsMissing []string `json:"required_inputs_missing,omitempty"`
}
