// Package ratios computes a deterministic financial-ratio suite — profitability,
// liquidity, leverage, efficiency, and growth ratios — from a normalized
// financial.FinancialDataset, plus period-over-period trend/comparison output
// and caller-configurable, rule-based health signals.
//
// This package does not classify raw rows, does not decide adjustments, and
// does not pick a valuation method. It reads a financial.FinancialDataset
// directly (recomputing whatever financial/metrics.Snapshot figures it needs
// itself, the same way analytics/qoe and analytics/workingcapital each
// recompute rather than requiring a caller to separately run
// financial/metrics first) and turns that into one explainable Result.
//
// Every ratio is an ending-balance calculation for its period — this package
// never averages a balance-sheet figure across two periods (e.g. "average
// receivables"), consistent with every other formula in this repository
// (financial/metrics.debtMetrics, workingCapital, and
// analytics/workingcapital.PeriodNWC are all ending-balance-only). A caller
// wanting an average-balance variant of a turnover ratio can compute it from
// two adjacent PeriodRatios.Components entries.
//
// Missing data is never treated as zero. Every computed figure is a
// metrics.MetricValue (reused directly from financial/metrics — this
// package's own Value type would be identical in shape, and every ratio
// here is downstream of exactly the kind of "calculated vs. cannot be
// calculated" distinction that type exists for), which distinguishes
// "calculated to be exactly 0" from "cannot be calculated because a
// required input is absent or a denominator is zero/unavailable." Callers
// must check MetricValue.Available before trusting MetricValue.Value — see
// each ratio function's doc comment in profitability.go/liquidity.go/
// leverage.go/efficiency.go/growth.go for its exact availability rule.
//
// Every function here is pure: no I/O, no mutation of caller-owned input, no
// package-global mutable state. Calculate can be called concurrently and
// repeatedly against identical input and always returns byte-for-byte
// identical JSON — see determinism_test.go.
//
// Health signals (see Signal) are entirely rule-based against
// caller-configurable Thresholds, mirroring analytics/qoe.Flag exactly —
// this package contains no AI/LLM, no opaque scoring, and no composite
// "health score" (per this package's task-level requirement: "No opaque
// composite score unless formula is explicit" — this package defines none).
package ratios

import (
	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/metrics"
)

// FormulaVersion identifies this package's fixed ratio formula set (every
// formula in profitability.go/liquidity.go/leverage.go/efficiency.go/
// growth.go, the Total Assets/Total Equity sum-of-codes definitions in
// components.go, and the Trend/Comparison methodology in trend.go). Bump
// this whenever any of that changes in a way that could make a historical
// Result not reproduce identically under new code — see the repository
// README's versioning-strategy section, which this constant follows exactly
// (metrics.FormulaVersion, qoe.FormulaVersion, workingcapital.FormulaVersion,
// etc.). Echoed on every Result.
const FormulaVersion = "1.0.0"

// SignalRulesVersion identifies this package's fixed signal-trigger rule set
// (every rule in signals.go), versioned separately from FormulaVersion since
// a caller may change how ratios are computed independently of which
// deterministic health signals are derived from them — mirroring
// qoe.ScoreVersion's identical rationale for being versioned apart from
// qoe.FormulaVersion. Echoed on every Result.
const SignalRulesVersion = "1.0.0"

// PeriodType mirrors metrics.PeriodType (fiscal year, YTD, quarter, month),
// duplicated here rather than imported so a caller can use this package
// without also importing financial/metrics solely for these values —
// mirroring analytics/workingcapital.PeriodType's identical duplication and
// its doc comment's rationale.
type PeriodType string

const (
	PeriodTypeFiscalYear PeriodType = "fiscal_year"
	PeriodTypeYTD        PeriodType = "ytd"
	PeriodTypeQuarter    PeriodType = "quarter"
	PeriodTypeMonth      PeriodType = "month"
)

// PeriodInfo is caller-supplied metadata about a single financial.Period,
// used to determine chronological order for Trend/Comparison/growth-signal
// output. financial.Period is intentionally just a string with no
// guaranteed sort order (see financial.FinancialDataset.Periods' doc
// comment); this package requires PeriodInfo rather than inferring order or
// granularity from the string, mirroring financial/metrics',
// financial/earnings', and analytics/workingcapital's identical
// no-guessing rule.
type PeriodInfo struct {
	// Type is this period's granularity.
	Type PeriodType `json:"type"`
	// FiscalYear is the fiscal year this period falls within. Required for
	// chronological ordering.
	FiscalYear int `json:"fiscal_year"`
	// SequenceInYear orders periods that share the same FiscalYear and Type
	// (e.g. Q1=1..Q4=4, or month 1-12). Ignored for PeriodTypeFiscalYear and
	// PeriodTypeYTD.
	SequenceInYear int `json:"sequence_in_year,omitempty"`
}

// Input bundles everything Calculate needs.
type Input struct {
	// Dataset is the normalized financial history to analyze. Required;
	// Calculate returns a Result with Available == false and an Errors
	// entry if Dataset has no periods.
	Dataset financial.FinancialDataset
	// PeriodMeta supplies chronological ordering for Dataset's periods.
	// Required for Trend, every period-over-period Comparison, and every
	// Growth ratio/signal this package produces; if nil, or a period
	// present in Dataset has no entry, those specific outputs are left
	// unavailable/empty rather than the whole Result failing — mirroring
	// analytics/workingcapital.Input.PeriodMeta's identical convention.
	// Per-period ratio calculation itself does not require PeriodMeta.
	PeriodMeta map[financial.Period]PeriodInfo
}

// Options controls Calculate's optional behavior. The zero Options is
// valid: it uses DefaultThresholds and computes no signals beyond what
// Thresholds' defaults would trigger against the computed ratios (signals
// are always evaluated — see Options.Thresholds; there is no opt-out
// analogous to qoe.Options.ComputeScore, since unlike qoe.Score, a Signal is
// not a composite/opaque figure but simply a restatement of "this specific
// already-computed ratio crossed this specific already-computed threshold,"
// which the "Signals" section of this package's task explicitly calls for
// as always-on, caller-configurable output).
type Options struct {
	// Thresholds configures every deterministic signal trigger point (see
	// Thresholds). If the zero value, DefaultThresholds() is used —
	// mirroring qoe.Options.Thresholds/review.Policy's identical
	// zero-value-means-defaults convention.
	Thresholds Thresholds
}

// Component is one named input that contributed to a computed ratio or
// synthetic subtotal (e.g. Total Assets, Total Equity), mirroring
// metrics.Component/workingcapital.Component's identical role: carried so
// callers can explain how a figure was produced without this package
// implementing a generic expression engine.
type Component struct {
	// Code identifies the canonical financial.Code this component came
	// from, a metrics.Metric* name for a metrics-derived component, or a
	// synthetic identifier (e.g. "TOTAL_ASSETS", "TOTAL_EQUITY") for a
	// component that is itself a computed subtotal.
	Code string `json:"code"`
	// Label is a short human-readable description of the component.
	Label string `json:"label,omitempty"`
	// Amount is the value contributed by this component, in the dataset's
	// native sign convention (assets/expenses positive, matching
	// financial/metrics' documented sign convention — see
	// metrics/income_statement.go's package-level comment).
	Amount float64 `json:"amount"`
}

// Ratio is a single named ratio/margin/turnover/day-count figure for one
// period, with its computed value, the components it was built from, and a
// fixed formula description — mirroring metrics.MetricResult's shape
// exactly (Metric/Period/Value/Formula/Components), reused as a distinct
// type in this package rather than imported because a Ratio's Metric names
// (see the RatioXxx constants) are this package's own durable API surface,
// independent of financial/metrics.MetricResult's.
type Ratio struct {
	// Metric is the stable name of this ratio (e.g. "GROSS_MARGIN"). See the
	// Ratio* constants.
	Metric string `json:"metric"`
	// Period is the reporting period this result applies to.
	Period financial.Period `json:"period"`
	// Value is the computed figure and its availability.
	Value metrics.MetricValue `json:"value"`
	// Formula is a short, fixed, human-readable description of exactly how
	// Value was computed, independent of which Components happened to be
	// present. Always populated, even when Value is unavailable.
	Formula string `json:"formula"`
	// Components lists the named inputs that were combined to produce
	// Value, when Value.Available is true. Omitted for simple lookups or
	// when the ratio is unavailable.
	Components []Component `json:"components,omitempty"`
}

// Stable ratio name constants, used as Ratio.Metric and as map keys in
// PeriodRatios lookups. Names are part of this package's durable API
// surface: once published, a name's string value should not change.
const (
	// Profitability.
	RatioGrossMargin     = "GROSS_MARGIN"
	RatioOperatingMargin = "OPERATING_MARGIN"
	RatioEBITDAMargin    = "EBITDA_MARGIN"
	RatioNetMargin       = "NET_MARGIN"
	RatioReturnOnAssets  = "RETURN_ON_ASSETS"
	RatioReturnOnEquity  = "RETURN_ON_EQUITY"

	// Liquidity.
	RatioCurrentRatio = "CURRENT_RATIO"
	RatioQuickRatio   = "QUICK_RATIO"
	RatioCashRatio    = "CASH_RATIO"

	// Leverage.
	RatioDebtToEquity     = "DEBT_TO_EQUITY"
	RatioDebtToAssets     = "DEBT_TO_ASSETS"
	RatioDebtToEBITDA     = "DEBT_TO_EBITDA"
	RatioNetDebtToEBITDA  = "NET_DEBT_TO_EBITDA"
	RatioInterestCoverage = "INTEREST_COVERAGE"

	// Efficiency.
	RatioAssetTurnover            = "ASSET_TURNOVER"
	RatioReceivablesTurnover      = "RECEIVABLES_TURNOVER"
	RatioInventoryTurnover        = "INVENTORY_TURNOVER"
	RatioDaysSalesOutstanding     = "DAYS_SALES_OUTSTANDING"
	RatioDaysInventoryOutstanding = "DAYS_INVENTORY_OUTSTANDING"
	RatioDaysPayableOutstanding   = "DAYS_PAYABLE_OUTSTANDING"
	RatioCashConversionCycle      = "CASH_CONVERSION_CYCLE"
)

// PeriodRatios holds every ratio computed for one financial.Period. Each
// field is independently a Ratio with its own MetricValue.Available; a
// business missing a balance sheet (income-statement-only data) simply has
// Available == false on every balance-sheet-derived field rather than
// failing the whole calculation, exactly mirroring metrics.Snapshot's
// per-field independence.
type PeriodRatios struct {
	Period financial.Period `json:"period"`

	// Profitability.
	GrossMargin     Ratio `json:"gross_margin"`
	OperatingMargin Ratio `json:"operating_margin"`
	EBITDAMargin    Ratio `json:"ebitda_margin"`
	NetMargin       Ratio `json:"net_margin"`
	ReturnOnAssets  Ratio `json:"return_on_assets"`
	ReturnOnEquity  Ratio `json:"return_on_equity"`

	// Liquidity.
	CurrentRatio Ratio `json:"current_ratio"`
	QuickRatio   Ratio `json:"quick_ratio"`
	CashRatio    Ratio `json:"cash_ratio"`

	// Leverage.
	DebtToEquity     Ratio `json:"debt_to_equity"`
	DebtToAssets     Ratio `json:"debt_to_assets"`
	DebtToEBITDA     Ratio `json:"debt_to_ebitda"`
	NetDebtToEBITDA  Ratio `json:"net_debt_to_ebitda"`
	InterestCoverage Ratio `json:"interest_coverage"`

	// Efficiency.
	AssetTurnover            Ratio `json:"asset_turnover"`
	ReceivablesTurnover      Ratio `json:"receivables_turnover"`
	InventoryTurnover        Ratio `json:"inventory_turnover"`
	DaysSalesOutstanding     Ratio `json:"days_sales_outstanding"`
	DaysInventoryOutstanding Ratio `json:"days_inventory_outstanding"`
	DaysPayableOutstanding   Ratio `json:"days_payable_outstanding"`
	CashConversionCycle      Ratio `json:"cash_conversion_cycle"`

	// Snapshot is the full financial/metrics.Snapshot this period's ratios
	// were derived from, carried for callers that want a raw underlying
	// figure (revenue, EBITDA, etc.) this package does not itself surface
	// by name — mirroring qoe.PeriodFigures.Snapshot's identical role.
	Snapshot metrics.Snapshot `json:"snapshot"`
	// TotalAssets/TotalEquity are this package's own sum-of-codes
	// components (see components.go), carried alongside Snapshot since
	// financial/metrics does not itself compute either figure.
	TotalAssets metrics.MetricValue `json:"total_assets"`
	TotalEquity metrics.MetricValue `json:"total_equity"`

	// Results carries every computed Ratio for this period (same values as
	// the typed fields above, plus Formula/Components), keyed by ratio
	// name, for callers that want explainability without knowing each field
	// name up front — mirroring metrics.Snapshot.Results' identical role.
	Results map[string]Ratio `json:"results"`
}

// RatioAt returns the Ratio for a given metric name from a PeriodRatios'
// Results map, and whether it was found. Convenience lookup mirroring
// metrics.Result.SnapshotFor's style.
func (p PeriodRatios) RatioAt(metric string) (Ratio, bool) {
	r, ok := p.Results[metric]
	return r, ok
}

// GrowthPoint is a single period-over-period growth calculation between two
// chronologically adjacent periods (not restricted to fiscal-year-to-
// fiscal-year, unlike financial/metrics.Trend's GrowthPoint — this package
// computes growth across whatever chronological sequence Input.PeriodMeta
// establishes, since analytics/ratios has no equivalent MVP restriction to
// carry forward). Mirrors metrics.GrowthPoint's shape.
type GrowthPoint struct {
	// FromPeriod and ToPeriod are the periods compared.
	FromPeriod financial.Period `json:"from_period"`
	ToPeriod   financial.Period `json:"to_period"`
	// Growth is (to - from) / |from|, as a decimal (0.10 = 10% growth).
	// Available is false if either value is Unavailable, or if from is
	// exactly 0 (growth from a zero base is undefined as a percentage; see
	// GrowthFromZeroBase for the explicit signal instead).
	Growth metrics.MetricValue `json:"growth"`
	// GrowthFromZeroBase is true when FromPeriod's value was exactly 0,
	// explaining why Growth is Unavailable in that specific case.
	GrowthFromZeroBase bool `json:"growth_from_zero_base,omitempty"`
}

// Growth holds every period-over-period GrowthPoint series this package
// computes, ordered chronologically (matching Result.ChronologicalPeriods)
// when Input.PeriodMeta covers every period in Dataset; empty otherwise.
type Growth struct {
	// RevenueGrowth is period-over-period total revenue growth.
	RevenueGrowth []GrowthPoint `json:"revenue_growth,omitempty"`
	// GrossProfitGrowth is period-over-period gross profit growth.
	GrossProfitGrowth []GrowthPoint `json:"gross_profit_growth,omitempty"`
	// EBITDAGrowth is period-over-period EBITDA growth.
	EBITDAGrowth []GrowthPoint `json:"ebitda_growth,omitempty"`
	// NetIncomeGrowth is period-over-period net income growth.
	NetIncomeGrowth []GrowthPoint `json:"net_income_growth,omitempty"`
}

// RatioTrendDirection is a coarse, deterministic characterization of one
// ratio's overall direction across History, computed from its first vs.
// last available observation — never inferred from a Message string.
// Mirrors workingcapital.TrendDirection's identical four-way model.
type RatioTrendDirection string

const (
	TrendIncreasing  RatioTrendDirection = "increasing"
	TrendDeclining   RatioTrendDirection = "declining"
	TrendStable      RatioTrendDirection = "stable"
	TrendUnavailable RatioTrendDirection = "unavailable"
)

// TrendFlatBandPercent is the |change| / |first value| band, as a decimal,
// within which a ratio series is characterized TrendStable rather than
// TrendIncreasing/TrendDeclining. Fixed (not caller-configurable) because it
// is part of this package's versioned FormulaVersion — mirroring
// workingcapital.TrendFlatBandPercent's identical rationale and value.
const TrendFlatBandPercent = 0.05

// RatioTrend is a first-vs-last-observation direction characterization of
// one ratio's series across History (see RatioTrendDirection), plus the
// underlying comparison values for display. Mirrors workingcapital.Trend's
// shape.
type RatioTrend struct {
	Metric    string              `json:"metric"`
	Direction RatioTrendDirection `json:"direction"`
	// FirstPeriod/LastPeriod are the chronologically first/last periods
	// with an available observation, when Direction is not
	// TrendUnavailable.
	FirstPeriod financial.Period `json:"first_period,omitempty"`
	LastPeriod  financial.Period `json:"last_period,omitempty"`
	// FirstValue/LastValue are the corresponding observation values.
	FirstValue metrics.MetricValue `json:"first_value"`
	LastValue  metrics.MetricValue `json:"last_value"`
	// PercentChange is (LastValue - FirstValue) / |FirstValue|. Available
	// only if both values are available and FirstValue.Value != 0.
	PercentChange metrics.MetricValue `json:"percent_change"`
}

// Comparison is one ratio's period-over-period change between two
// chronologically adjacent periods — the finer-grained counterpart to
// RatioTrend's first-vs-last view, giving a caller every adjacent-period
// delta rather than only the endpoints.
type Comparison struct {
	Metric     string              `json:"metric"`
	FromPeriod financial.Period    `json:"from_period"`
	ToPeriod   financial.Period    `json:"to_period"`
	FromValue  metrics.MetricValue `json:"from_value"`
	ToValue    metrics.MetricValue `json:"to_value"`
	// Change is ToValue - FromValue (an absolute difference, not a percent
	// — a ratio is often already a percent/decimal itself, e.g. a margin,
	// so a further "percent change of a percent" would be confusing;
	// display code wanting a relative change can divide Change by
	// |FromValue| itself). Available only if both values are available.
	Change metrics.MetricValue `json:"change"`
}

// SignalCode is a stable identifier for one kind of deterministic,
// caller-configurable health signal, mirroring qoe.FlagCode's identical
// role and rationale (this package defines its own separate taxonomy
// rather than reusing qoe.FlagCode/workingcapital.IssueCode — a ratio
// health signal is a distinct problem domain).
type SignalCode string

const (
	// SignalWeakeningLiquidity means CurrentRatio's most recent
	// period-over-period Comparison declined by at least
	// Thresholds.LiquidityDeclineThreshold.
	SignalWeakeningLiquidity SignalCode = "WEAKENING_LIQUIDITY"
	// SignalRisingLeverage means DebtToEBITDA's most recent
	// period-over-period Comparison increased by at least
	// Thresholds.LeverageIncreaseThreshold.
	SignalRisingLeverage SignalCode = "RISING_LEVERAGE"
	// SignalMarginCompression means EBITDAMargin's most recent
	// period-over-period Comparison declined by at least
	// Thresholds.MarginCompressionThreshold (absolute percentage points).
	SignalMarginCompression SignalCode = "MARGIN_COMPRESSION"
	// SignalSlowingCollections means DaysSalesOutstanding's most recent
	// period-over-period Comparison increased by at least
	// Thresholds.CollectionsSlowdownDays.
	SignalSlowingCollections SignalCode = "SLOWING_COLLECTIONS"
	// SignalInventoryBuildup means DaysInventoryOutstanding's most recent
	// period-over-period Comparison increased by at least
	// Thresholds.InventoryBuildupDays.
	SignalInventoryBuildup SignalCode = "INVENTORY_BUILDUP"
	// SignalWeakInterestCoverage means the most recent period's
	// InterestCoverage is available and at or below
	// Thresholds.WeakInterestCoverageRatio.
	SignalWeakInterestCoverage SignalCode = "WEAK_INTEREST_COVERAGE"
	// SignalImprovingProfitability means EBITDAMargin's most recent
	// period-over-period Comparison improved by at least
	// Thresholds.ProfitabilityChangeThreshold.
	SignalImprovingProfitability SignalCode = "IMPROVING_PROFITABILITY"
	// SignalDeterioratingProfitability means EBITDAMargin's most recent
	// period-over-period Comparison worsened by at least
	// Thresholds.ProfitabilityChangeThreshold. Distinct from
	// SignalMarginCompression: compression is EBITDAMargin-specific by
	// name in the task's own signal list, deteriorating/improving
	// profitability is the same underlying measurement re-triggered under
	// its own named signal per the task's explicit separate bullet — see
	// signals.go for why both fire together off one comparison rather than
	// this package inventing a second, different profitability measure to
	// tell them apart.
	SignalDeterioratingProfitability SignalCode = "DETERIORATING_PROFITABILITY"
)

// SignalSeverity mirrors qoe.FlagSeverity's role: a structured, matchable
// urgency signal, never inferred from Message text.
type SignalSeverity string

const (
	SignalSeverityInfo     SignalSeverity = "info"
	SignalSeverityWarning  SignalSeverity = "warning"
	SignalSeverityCritical SignalSeverity = "critical"
)

// Signal is a single deterministic, explainable health signal. See the
// package doc comment: every Signal here is rule-based against Thresholds,
// never AI-scored, and never rolled into an opaque composite score.
type Signal struct {
	// Code identifies which health signal this is. See the SignalCode
	// constants.
	Code SignalCode `json:"code"`
	// Severity is this signal's structured urgency — see SignalSeverity.
	Severity SignalSeverity `json:"severity"`
	// Period is the period this signal most directly concerns (the ToPeriod
	// of the Comparison/most recent period it was evaluated against).
	Period financial.Period `json:"period,omitempty"`
	// Message is a short, human-readable explanation, already filled in
	// with the specific figures that triggered this signal. Display only —
	// this package's own logic never parses Message; see Code for the
	// stable, matchable signal.
	Message string `json:"message"`
	// Value is the specific computed figure that triggered this signal (a
	// ratio level, or a period-over-period change), meaningful only in
	// combination with Code.
	Value float64 `json:"value"`
	// Threshold is the configured Thresholds value Value was compared
	// against, echoed here so a caller can display "X vs. a Y threshold"
	// without separately re-reading Options.Thresholds.
	Threshold float64 `json:"threshold"`
}

// IssueSeverity distinguishes an input problem Calculate could not proceed
// past for some portion of the analysis (SeverityError) from one that is
// advisory only (SeverityWarning) — the same two-severity model
// qoe.IssueSeverity/workingcapital.IssueSeverity use.
type IssueSeverity string

const (
	SeverityError   IssueSeverity = "error"
	SeverityWarning IssueSeverity = "warning"
)

// IssueCode is a stable identifier for one kind of Calculate-time input
// problem. This package defines its own separate taxonomy, consistent with
// every other package in this repository, rather than reusing
// qoe.IssueCode/workingcapital.IssueCode.
type IssueCode string

const (
	// IssueNoPeriods means Input.Dataset has no periods at all; Calculate
	// returns Available == false.
	IssueNoPeriods IssueCode = "NO_PERIODS"
	// IssueNoPeriodMeta means Input.PeriodMeta was nil or empty, so Trend,
	// Comparisons, and Growth could not be computed. Advisory only:
	// per-period History ratios are still fully computed in
	// Dataset.Periods() lexical order.
	IssueNoPeriodMeta IssueCode = "NO_PERIOD_META"
	// IssuePeriodMissingFromMeta means at least one period present in
	// Dataset has no PeriodMeta entry, so History falls back to lexical
	// order and Trend/Comparisons/Growth are unavailable — mirroring
	// workingcapital.IssuePeriodMissingFromMeta's identical all-or-nothing
	// ordering rule.
	IssuePeriodMissingFromMeta IssueCode = "PERIOD_MISSING_FROM_META"
)

// Issue is a single Calculate-time input finding, mirroring
// qoe.Issue/workingcapital.Issue's shape.
type Issue struct {
	Code     IssueCode     `json:"code"`
	Severity IssueSeverity `json:"severity"`
	Message  string        `json:"message"`
}

// HasErrors reports whether any Issue in issues has SeverityError.
//
// Intentionally duplicated from qoe.HasErrors/workingcapital.HasErrors
// rather than shared — see adjustments.HasErrors's doc comment (referenced
// by both of those) for the full rationale: each package's Issue is a
// distinct Go type with no common interface worth introducing for one
// boolean function.
func HasErrors(issues []Issue) bool {
	for _, i := range issues {
		if i.Severity == SeverityError {
			return true
		}
	}
	return false
}

// Result is the output of Calculate: the full per-period ratio history,
// trend/comparison/growth output, deterministic health signals, and
// versioning.
type Result struct {
	// FormulaVersion identifies which version of this package's fixed ratio
	// formula set produced this Result.
	FormulaVersion string `json:"formula_version"`
	// SignalRulesVersion identifies which version of this package's fixed
	// signal-trigger rule set produced Result.Signals.
	SignalRulesVersion string `json:"signal_rules_version"`
	// Available is false only if Calculate could not proceed at all (see
	// IssueNoPeriods) — every other field is then zero-value.
	Available bool `json:"available"`

	// History is one PeriodRatios per period in Input.Dataset, ordered
	// chronologically when Input.PeriodMeta was supplied and covers every
	// period; otherwise in Dataset.Periods()'s lexical order.
	History []PeriodRatios `json:"history,omitempty"`

	// Trends holds one RatioTrend per ratio metric that had at least one
	// available observation in History, in RatioXxx declaration order.
	// Empty if Input.PeriodMeta did not cover every period (see
	// IssuePeriodMissingFromMeta) — a Trend's first-vs-last comparison
	// requires true chronological order.
	Trends []RatioTrend `json:"trends,omitempty"`
	// Comparisons holds every adjacent-period Comparison for every ratio
	// metric that had at least one available observation in History,
	// ordered by metric (RatioXxx declaration order), then chronologically
	// within a metric. Empty under the same condition as Trends.
	Comparisons []Comparison `json:"comparisons,omitempty"`
	// Growth holds every period-over-period growth series (revenue, gross
	// profit, EBITDA, net income). Empty under the same condition as
	// Trends.
	Growth Growth `json:"growth"`

	// Signals is every deterministic health signal Calculate triggered,
	// ordered by SignalCode declaration order, then by Period — never Go
	// map order, mirroring qoe.Result.Flags' identical ordering rule.
	Signals []Signal `json:"signals,omitempty"`

	// Thresholds echoes the resolved Options.Thresholds (after
	// DefaultThresholds substitution) this Result was computed under, so a
	// persisted Result remains self-describing about exactly which trigger
	// points produced its Signals.
	Thresholds Thresholds `json:"thresholds"`

	// Warnings carries every Issue with SeverityWarning.
	Warnings []Issue `json:"warnings,omitempty"`
	// Errors carries every Issue with SeverityError.
	Errors []Issue `json:"errors,omitempty"`
}
