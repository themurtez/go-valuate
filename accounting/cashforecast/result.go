package cashforecast

// CategoryAmount is one category's aggregated amount within a week,
// paired with the event count and provenance that produced it — see the
// task's section 37 "every forecast amount must trace to source events"
// requirement.
type CategoryAmount struct {
	Category CashCategory `json:"category"`
	Amount   float64      `json:"amount"`
	Count    int          `json:"count"`
	// EventIDs is every CashFlowEvent.ID contributing to Amount, sorted
	// ascending.
	EventIDs []string `json:"event_ids,omitempty"`
}

// BasisAmount is one CashBasis's aggregated amount within a week/
// direction, for the "by source/basis" breakdown the task's section 10
// requires alongside the category breakdown.
type BasisAmount struct {
	Basis  CashBasis `json:"basis"`
	Amount float64   `json:"amount"`
	Count  int       `json:"count"`
}

// FlowTotal bundles one direction's total, category breakdown, and basis
// breakdown for one week.
type FlowTotal struct {
	Total      float64          `json:"total"`
	ByCategory []CategoryAmount `json:"by_category,omitempty"`
	ByBasis    []BasisAmount    `json:"by_basis,omitempty"`
}

// ThresholdPosition is one week's minimum-cash comparison, unavailable in
// every field except Available when no minimum-cash threshold is
// resolved.
type ThresholdPosition struct {
	Available bool `json:"available"`
	// MinimumCash is the resolved threshold this week was compared
	// against.
	MinimumCash float64 `json:"minimum_cash,omitempty"`
	// Headroom is EndingCash - MinimumCash when positive or zero.
	Headroom float64 `json:"headroom,omitempty"`
	// FundingGap is MinimumCash - EndingCash when positive (i.e.
	// EndingCash is below MinimumCash); zero otherwise.
	FundingGap float64 `json:"funding_gap,omitempty"`
	// BelowMinimum is true when EndingCash < MinimumCash.
	BelowMinimum bool `json:"below_minimum"`
}

// WeeklyForecast is one week's full cash position.
type WeeklyForecast struct {
	WeekNumber int    `json:"week_number"`
	StartDate  string `json:"start_date"`
	EndDate    string `json:"end_date"`

	OpeningCash float64 `json:"opening_cash"`

	Inflows  FlowTotal `json:"inflows"`
	Outflows FlowTotal `json:"outflows"`

	NetCashFlow float64 `json:"net_cash_flow"`
	EndingCash  float64 `json:"ending_cash"`

	Threshold ThresholdPosition `json:"threshold"`

	// OperatingInflows/OperatingOutflows/InvestingInflows/
	// InvestingOutflows/FinancingInflows/FinancingOutflows are the
	// operating/investing/financing cash-flow-statement-style split of
	// Inflows.Total/Outflows.Total, derived from each CashCategory's fixed
	// CashFlowClass — see the task's section 38.
	OperatingInflows  float64 `json:"operating_inflows"`
	OperatingOutflows float64 `json:"operating_outflows"`
	InvestingInflows  float64 `json:"investing_inflows"`
	InvestingOutflows float64 `json:"investing_outflows"`
	FinancingInflows  float64 `json:"financing_inflows"`
	FinancingOutflows float64 `json:"financing_outflows"`
}

// ScheduleEntry is one presentation-neutral row in a DetailedSchedule —
// see the task's section 39.
type ScheduleEntry struct {
	Date           string        `json:"date"`
	Amount         float64       `json:"amount"`
	Direction      CashDirection `json:"direction"`
	Category       CashCategory  `json:"category"`
	Description    string        `json:"description,omitempty"`
	SourceType     SourceType    `json:"source_type,omitempty"`
	SourceID       string        `json:"source_id,omitempty"`
	Basis          CashBasis     `json:"basis"`
	Certainty      Certainty     `json:"certainty,omitempty"`
	CounterpartyID string        `json:"counterparty_id,omitempty"`
	WeekNumber     int           `json:"week_number"`
}

// LiquiditySummary is one scenario's top-level headline figures — see the
// task's section 30.
type LiquiditySummary struct {
	StartingCash  float64 `json:"starting_cash"`
	TotalInflows  float64 `json:"total_inflows"`
	TotalOutflows float64 `json:"total_outflows"`
	NetChange     float64 `json:"net_change"`
	EndingCash    float64 `json:"ending_cash"`

	LowestCashBalance float64 `json:"lowest_cash_balance"`
	LowestCashWeek    int     `json:"lowest_cash_week"`

	// MinimumCashThreshold, FirstWeekBelowMinimum, WeeksBelowMinimum,
	// MaximumFundingGap, and EndingFundingGap are all zero-value/
	// unavailable together via ThresholdAvailable when no minimum-cash
	// threshold was resolved — see the task's "if no minimum threshold
	// absent, threshold fields are unavailable" instruction.
	ThresholdAvailable    bool    `json:"threshold_available"`
	MinimumCashThreshold  float64 `json:"minimum_cash_threshold,omitempty"`
	FirstWeekBelowMinimum int     `json:"first_week_below_minimum,omitempty"`
	WeeksBelowMinimum     int     `json:"weeks_below_minimum,omitempty"`
	MaximumFundingGap     float64 `json:"maximum_funding_gap,omitempty"`
	EndingFundingGap      float64 `json:"ending_funding_gap,omitempty"`

	// FirstNegativeCashWeek is the first week number whose EndingCash is
	// negative, 0 if the forecast never goes negative (see
	// NegativeCashReached).
	FirstNegativeCashWeek int  `json:"first_negative_cash_week,omitempty"`
	NegativeCashReached   bool `json:"negative_cash_reached"`

	// RequiredFunding is the funding-requirement analysis — see the task's
	// section 32.
	RequiredFunding RequiredFunding `json:"required_funding"`

	// FacilityCapacity is the sum of every valid CreditFacility's
	// AvailableToDraw, echoed here for convenience alongside
	// GapAfterFacility.
	FacilityCapacity float64 `json:"facility_capacity,omitempty"`
	// GapAfterFacility is MaximumFundingGap - FacilityCapacity when
	// positive (the gap remaining even after available facility capacity),
	// zero otherwise. Available only when both MaximumFundingGap and a
	// facility are present — this never asserts a draw occurred, only
	// reports the arithmetic remainder.
	GapAfterFacility AmountValue `json:"gap_after_facility"`
}

// RequiredFunding is the task's section 32 deterministic single-upfront-
// injection funding requirement.
type RequiredFunding struct {
	Available bool `json:"available"`
	// RequiredAtStart is the minimum single upfront cash injection at Week
	// 1's start that keeps every week's EndingCash at or above
	// MinimumCashThreshold across the entire horizon — exactly
	// MaximumFundingGap when MaximumFundingGap is positive, 0 otherwise.
	// See docs/CASH_FORECAST_13_WEEK.md for the exact formula and why an
	// upfront injection generalizes to the whole horizon without
	// week-by-week re-solving.
	RequiredAtStart float64 `json:"required_at_start"`
	// PeakCumulativeGap is the largest single-week FundingGap observed
	// (equal to RequiredAtStart by construction, included for clarity
	// since it is the more intuitive label for the same figure).
	PeakCumulativeGap float64 `json:"peak_cumulative_gap"`
}

// DeltaVsBase is one scenario's week-by-week and summary variance against
// the base scenario — see the task's section 29. Zero-value/unavailable
// for the base scenario itself.
type DeltaVsBase struct {
	Available        bool          `json:"available"`
	Weekly           []WeeklyDelta `json:"weekly,omitempty"`
	NetCashFlowDelta float64       `json:"net_cash_flow_delta,omitempty"`
	EndingCashDelta  float64       `json:"ending_cash_delta,omitempty"`
	FundingGapDelta  float64       `json:"funding_gap_delta,omitempty"`
}

// WeeklyDelta is one week's scenario-vs-base variance.
type WeeklyDelta struct {
	WeekNumber       int     `json:"week_number"`
	NetCashFlowDelta float64 `json:"net_cash_flow_delta"`
	EndingCashDelta  float64 `json:"ending_cash_delta"`
	FundingGapDelta  float64 `json:"funding_gap_delta"`
}

// ScenarioResult is one Scenario's complete forecast output.
type ScenarioResult struct {
	Label            string           `json:"label"`
	Weekly           []WeeklyForecast `json:"weekly"`
	Summary          LiquiditySummary `json:"summary"`
	DetailedSchedule []ScheduleEntry  `json:"detailed_schedule,omitempty"`
	DeltaVsBase      DeltaVsBase      `json:"delta_vs_base"`
}

// UnscheduledSummary is the task's section 40 "known receivables/payables
// omitted from scheduling" report.
type UnscheduledSummary struct {
	Available bool     `json:"available"`
	Count     int      `json:"count"`
	Amount    float64  `json:"amount"`
	IDs       []string `json:"ids,omitempty"`
}

// BeyondHorizonSummary summarizes CashFlowEvents dated after the last
// forecast week, excluded from weekly totals — see the task's section 11.
type BeyondHorizonSummary struct {
	Count  int     `json:"count"`
	Amount float64 `json:"amount"`
}

// BeforeStartSummary summarizes CashFlowEvents dated before
// Input.ForecastStartDate, excluded from weekly totals — see the task's
// section 11.
type BeforeStartSummary struct {
	Count  int     `json:"count"`
	Amount float64 `json:"amount"`
}

// Result is the output of Calculate: the base scenario, every named
// scenario, coverage/completeness, unscheduled AR/AP, and every deterministic
// issue/flag.
type Result struct {
	SchemaVersion  string `json:"schema_version"`
	FormulaVersion string `json:"formula_version"`
	// Available is false only if Calculate could not proceed at all (see
	// IssueInvalidForecastStart/IssueInvalidHorizon) — every other field is
	// then zero-value.
	Available bool `json:"available"`

	ForecastStartDate string        `json:"forecast_start_date"`
	ForecastEndDate   string        `json:"forecast_end_date"`
	HorizonWeeks      int           `json:"horizon_weeks"`
	WeekAlignment     WeekAlignment `json:"week_alignment"`
	ReportingCurrency string        `json:"reporting_currency"`

	OpeningPosition OpeningPosition `json:"opening_position"`

	// BaseScenario is the forecast under Input's events with no scenario
	// transformation applied — always present.
	BaseScenario ScenarioResult `json:"base_scenario"`
	// Scenarios is one ScenarioResult per Input.Scenarios entry, in the
	// order supplied. Does not include BaseScenario again.
	Scenarios []ScenarioResult `json:"scenarios,omitempty"`

	Coverage Coverage `json:"coverage"`

	UnscheduledAR UnscheduledSummary `json:"unscheduled_ar"`
	UnscheduledAP UnscheduledSummary `json:"unscheduled_ap"`

	BeforeStart   BeforeStartSummary   `json:"before_start"`
	BeyondHorizon BeyondHorizonSummary `json:"beyond_horizon"`

	Issues []Issue `json:"issues,omitempty"`
	Flags  []Flag  `json:"flags,omitempty"`
}
