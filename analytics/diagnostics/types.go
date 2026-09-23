// Package diagnostics combines the already-computed output of many
// deterministic financial-analytics packages into one structured business
// diagnostic — a single-business, single-period-focused "what does the
// data say about this business, across every dimension we have a module
// for" read, distinct from [portfolio/diagnostics] (which scans many
// businesses from condensed summaries and ranks change-based findings
// across a whole book) and from [transactions/salereadiness] (which
// classifies eleven fixed sale-readiness dimensions specifically). This
// package instead mines whichever of up to fifteen optional sibling
// [Result] types a caller has on hand for their own already-computed
// Flags, Signals, Anomalies, and Status classifications, and republishes
// them as [Finding]s grouped into fixed [Category] buckets.
//
// This is not an AI narrative layer. Every Finding traces back to a
// specific field on a specific sibling Result — a Flag/Signal/Anomaly
// Code, a Status classification, a breach, an unfavorable benchmark
// comparison — never a generated summary, and never a new calculation:
// this package computes no financial figure of its own. See
// [SourceModule]/[Finding.SourceCode] on why every Finding is fully
// traceable to its origin.
//
// # No legal, tax, or investment conclusions
//
// Every Finding states a fact about already-computed data plus a
// neutrally phrased [Finding.RecommendedAction] naming what to look into
// next (e.g. "review the drivers of margin compression"), never a legal,
// tax, or investment conclusion, and never a promise about a future
// transaction or valuation outcome — mirroring
// [transactions/salereadiness]'s identical no-promise discipline.
//
// # Optional inputs, explicit coverage
//
// Every field of [Input] beyond Dataset/PeriodMeta is optional. An absent
// or unavailable sibling Result narrows which mining functions can find
// anything for that module (see [Coverage] and [Result.MissingDataAreas])
// rather than failing the whole diagnostic or being silently treated as
// "no findings because nothing is wrong."
//
// # Determinism
//
// Every function here is pure: no I/O, no mutation of caller-owned input,
// no package-global mutable state, no wall-clock/randomness. Calculate can
// be called concurrently and repeatedly against identical input and always
// returns byte-for-byte identical JSON — see determinism_test.go.
package diagnostics

import (
	"github.com/themurtez/go-valuate/analytics/anomalies"
	"github.com/themurtez/go-valuate/analytics/benchmarks"
	"github.com/themurtez/go-valuate/analytics/cashflow"
	"github.com/themurtez/go-valuate/analytics/concentration"
	"github.com/themurtez/go-valuate/analytics/covenants"
	"github.com/themurtez/go-valuate/analytics/debt"
	"github.com/themurtez/go-valuate/analytics/forecast"
	"github.com/themurtez/go-valuate/analytics/qoe"
	"github.com/themurtez/go-valuate/analytics/ratios"
	"github.com/themurtez/go-valuate/analytics/revenuequality"
	"github.com/themurtez/go-valuate/analytics/valuedrivers"
	"github.com/themurtez/go-valuate/analytics/variance"
	"github.com/themurtez/go-valuate/analytics/workingcapital"
	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/metrics"
	"github.com/themurtez/go-valuate/transactions/salereadiness"
)

// FormulaVersion identifies this package's fixed mining/classification
// rules: every mine* function's field-to-Finding mapping (findings_*.go),
// Category assignment, and DefaultPolicy. Bump whenever any of that
// changes in a way that could make a historical Result not reproduce
// identically under new code — see the repository README's
// versioning-strategy section, which this constant follows exactly
// (salereadiness.FormulaVersion, management.FormulaVersion, etc.).
const FormulaVersion = "1.0.0"

// ScoreVersion identifies the exact formula computeHealthScore implements
// (score.go), versioned separately from FormulaVersion for the same reason
// salereadiness.ScoreVersion is distinct from salereadiness.FormulaVersion:
// a caller may reasonably want to change which findings are mined
// independently of how already-mined findings are weighted into one
// composite health score.
const ScoreVersion = "1.0.0"

// Value represents a single figure that may or may not be available,
// distinguishing "computed/reported to be exactly 0" from "unknown because
// a required input was absent" — this package's own copy of the
// convention every sibling package duplicates locally (see
// transactions/salereadiness.Value's doc comment for the full rationale)
// rather than importing another package's Value. This package has no
// financial.FinancialDataset dependency of its own (it only reads already
// -computed sibling Results), so it follows the debt.Value/
// salereadiness.Value naming (Amount, not Value.Value) rather than
// metrics.MetricValue's.
type Value struct {
	Available bool    `json:"available"`
	Amount    float64 `json:"amount"`
}

// Unavailable is the canonical zero-information Value.
func Unavailable() Value { return Value{} }

// AvailableValue reports a Value for a known figure (which may legitimately
// be zero or negative).
func AvailableValue(v float64) Value { return Value{Available: true, Amount: v} }

// Severity distinguishes how urgently a Finding should be addressed — this
// package's own three-level scale, matching every sibling analytics
// package's identical Flag/Signal severity convention (qoe.FlagSeverity,
// ratios.SignalSeverity, cashflow.FlagSeverity, etc.).
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// Category is a stable identifier for one of the fixed diagnostic
// categories every Finding is grouped into — the taxonomy Prompt 35 names
// (profitability, growth, liquidity, leverage, cash conversion, working
// capital, revenue quality, concentration, earnings quality, operational
// cost control, valuation, transaction readiness). A Finding's Category is
// about the business dimension it concerns, independent of which sibling
// module happened to detect it (e.g. a margin-related signal from
// analytics/ratios and a margin-related flag from analytics/qoe both file
// under CategoryProfitability).
type Category string

const (
	CategoryProfitability          Category = "PROFITABILITY"
	CategoryGrowth                 Category = "GROWTH"
	CategoryLiquidity              Category = "LIQUIDITY"
	CategoryLeverage               Category = "LEVERAGE"
	CategoryCashConversion         Category = "CASH_CONVERSION"
	CategoryWorkingCapital         Category = "WORKING_CAPITAL"
	CategoryRevenueQuality         Category = "REVENUE_QUALITY"
	CategoryConcentration          Category = "CONCENTRATION"
	CategoryEarningsQuality        Category = "EARNINGS_QUALITY"
	CategoryOperationalCostControl Category = "OPERATIONAL_COST_CONTROL"
	CategoryValuation              Category = "VALUATION"
	CategoryTransactionReadiness   Category = "TRANSACTION_READINESS"
)

// categoryOrder is Category's fixed declaration/output order, used
// wherever this package groups or counts by Category (Coverage, Summary)
// so iteration never depends on Go map order.
var categoryOrder = []Category{
	CategoryProfitability,
	CategoryGrowth,
	CategoryLiquidity,
	CategoryLeverage,
	CategoryCashConversion,
	CategoryWorkingCapital,
	CategoryRevenueQuality,
	CategoryConcentration,
	CategoryEarningsQuality,
	CategoryOperationalCostControl,
	CategoryValuation,
	CategoryTransactionReadiness,
}

// SourceModule names which optional Input field a Finding was mined from —
// a fixed, stable identifier distinct from the Go field name so a
// persisted Finding remains meaningful even if this package's Input field
// names ever changed internally. Matches the lowercase module names
// reporting/management.Coverage's WithX naming already established.
type SourceModule string

const (
	SourceMetrics        SourceModule = "metrics"
	SourceRatios         SourceModule = "ratios"
	SourceQoE            SourceModule = "qoe"
	SourceWorkingCapital SourceModule = "working_capital"
	SourceCashFlow       SourceModule = "cash_flow"
	SourceRevenueQuality SourceModule = "revenue_quality"
	SourceConcentration  SourceModule = "concentration"
	SourceAnomalies      SourceModule = "anomalies"
	SourceVariance       SourceModule = "variance"
	SourceForecast       SourceModule = "forecast"
	SourceDebt           SourceModule = "debt"
	SourceCovenants      SourceModule = "covenants"
	SourceBenchmarks     SourceModule = "benchmarks"
	SourceValueDrivers   SourceModule = "value_drivers"
	SourceSaleReadiness  SourceModule = "sale_readiness"
)

// FindingCode is a stable identifier for one kind of diagnostic finding —
// this package's own normalized taxonomy, distinct from (but traceable to,
// via Finding.SourceCode) each sibling module's own Flag/Signal/RuleCode.
// Several sibling codes from different modules can map to the same
// FindingCode when they describe the same business-level observation from
// different angles (e.g. ratios.SignalMarginCompression and
// qoe.FlagInconsistentMargins both map to FindingMarginPressure) — a
// caller filtering by FindingCode sees one coherent signal regardless of
// which module(s) corroborate it.
type FindingCode string

const (
	// Profitability
	FindingMarginPressure           FindingCode = "MARGIN_PRESSURE"
	FindingImprovingProfitability   FindingCode = "IMPROVING_PROFITABILITY"
	FindingEarningsVolatility       FindingCode = "EARNINGS_VOLATILITY"
	FindingRevenueOutpacedByExpense FindingCode = "REVENUE_OUTPACED_BY_EXPENSE"

	// Growth
	FindingRevenueVolatility     FindingCode = "REVENUE_VOLATILITY"
	FindingGrowthDependency      FindingCode = "GROWTH_DEPENDENCY"
	FindingOnePeriodRevenueSpike FindingCode = "ONE_PERIOD_REVENUE_SPIKE"

	// Liquidity
	FindingWeakeningLiquidity FindingCode = "WEAKENING_LIQUIDITY"
	FindingSlowingCollections FindingCode = "SLOWING_COLLECTIONS"
	FindingInventoryBuildup   FindingCode = "INVENTORY_BUILDUP"

	// Leverage
	FindingRisingLeverage       FindingCode = "RISING_LEVERAGE"
	FindingWeakInterestCoverage FindingCode = "WEAK_INTEREST_COVERAGE"
	FindingBelowMinimumDSCR     FindingCode = "BELOW_MINIMUM_DSCR"
	FindingAboveLeverageCap     FindingCode = "ABOVE_LEVERAGE_CAP"
	FindingNegativeDebtHeadroom FindingCode = "NEGATIVE_DEBT_HEADROOM"
	FindingCovenantBreach       FindingCode = "COVENANT_BREACH"
	FindingCovenantNearBreach   FindingCode = "COVENANT_NEAR_BREACH"
	FindingNoDebtService        FindingCode = "NO_DEBT_SERVICE"

	// Cash conversion
	FindingWeakCashConversion      FindingCode = "WEAK_CASH_CONVERSION"
	FindingDecliningCashConversion FindingCode = "DECLINING_CASH_CONVERSION"
	FindingLowCashRunway           FindingCode = "LOW_CASH_RUNWAY"
	FindingDistributionsExceedFCF  FindingCode = "DISTRIBUTIONS_EXCEED_FCF"
	FindingHighCapexBurden         FindingCode = "HIGH_CAPEX_BURDEN"

	// Working capital
	FindingWorkingCapitalVolatility FindingCode = "WORKING_CAPITAL_VOLATILITY"
	FindingHighWorkingCapitalBurden FindingCode = "HIGH_WORKING_CAPITAL_BURDEN"

	// Revenue quality
	FindingDecliningRecurringMix      FindingCode = "DECLINING_RECURRING_MIX"
	FindingShrinkingExistingCustomers FindingCode = "SHRINKING_EXISTING_CUSTOMERS"
	FindingHighLostCustomerRevenue    FindingCode = "HIGH_LOST_CUSTOMER_REVENUE"

	// Concentration
	FindingHighCustomerConcentration       FindingCode = "HIGH_CUSTOMER_CONCENTRATION"
	FindingHighTop5Concentration           FindingCode = "HIGH_TOP_5_CONCENTRATION"
	FindingIncreasingConcentration         FindingCode = "INCREASING_CONCENTRATION"
	FindingHighConcentrationScenarioImpact FindingCode = "HIGH_CONCENTRATION_SCENARIO_IMPACT"

	// Earnings quality
	FindingLargeNormalizationBurden         FindingCode = "LARGE_NORMALIZATION_BURDEN"
	FindingLargeOwnerDiscretionaryComponent FindingCode = "LARGE_OWNER_DISCRETIONARY_COMPONENT"
	FindingRepeatedOneTimeAdjustments       FindingCode = "REPEATED_ONE_TIME_ADJUSTMENTS"
	FindingNonOperatingIncomeReliance       FindingCode = "NON_OPERATING_INCOME_RELIANCE"
	FindingNegativeMaintainableEarnings     FindingCode = "NEGATIVE_MAINTAINABLE_EARNINGS"
	FindingHighOwnerDiscretionaryShare      FindingCode = "HIGH_OWNER_DISCRETIONARY_SHARE"

	// Operational cost control
	FindingExpenseSpike                FindingCode = "EXPENSE_SPIKE"
	FindingNewMaterialExpense          FindingCode = "NEW_MATERIAL_EXPENSE"
	FindingUnusualAccountActivity      FindingCode = "UNUSUAL_ACCOUNT_ACTIVITY"
	FindingRepeatedUnusualValue        FindingCode = "REPEATED_UNUSUAL_VALUE"
	FindingUnfavorableCostBenchmark    FindingCode = "UNFAVORABLE_COST_BENCHMARK"
	FindingMaterialUnfavorableVariance FindingCode = "MATERIAL_UNFAVORABLE_VARIANCE"

	// Valuation
	FindingHighDownsideValueSensitivity FindingCode = "HIGH_DOWNSIDE_VALUE_SENSITIVITY"

	// Transaction readiness
	FindingSaleReadinessBlocker     FindingCode = "SALE_READINESS_BLOCKER"
	FindingSaleReadinessRisk        FindingCode = "SALE_READINESS_RISK"
	FindingSaleReadinessStrength    FindingCode = "SALE_READINESS_STRENGTH"
	FindingSaleReadinessOpportunity FindingCode = "SALE_READINESS_OPPORTUNITY_AREA"

	// Cross-cutting missing-data finding, filed under whichever Category
	// the absent module would have contributed to most — see
	// buildMissingDataAreas.
	FindingModuleUnavailable FindingCode = "MODULE_UNAVAILABLE"
)

// findingSortOrder is FindingCode's fixed declaration order, duplicated
// here as a slice so sortFindings' tie-break has a stable rank to consult
// without reflection — see sortFindings in diagnostics.go.
var findingSortOrder = []FindingCode{
	FindingMarginPressure,
	FindingImprovingProfitability,
	FindingEarningsVolatility,
	FindingRevenueOutpacedByExpense,
	FindingRevenueVolatility,
	FindingGrowthDependency,
	FindingOnePeriodRevenueSpike,
	FindingWeakeningLiquidity,
	FindingSlowingCollections,
	FindingInventoryBuildup,
	FindingRisingLeverage,
	FindingWeakInterestCoverage,
	FindingBelowMinimumDSCR,
	FindingAboveLeverageCap,
	FindingNegativeDebtHeadroom,
	FindingCovenantBreach,
	FindingCovenantNearBreach,
	FindingNoDebtService,
	FindingWeakCashConversion,
	FindingDecliningCashConversion,
	FindingLowCashRunway,
	FindingDistributionsExceedFCF,
	FindingHighCapexBurden,
	FindingWorkingCapitalVolatility,
	FindingHighWorkingCapitalBurden,
	FindingDecliningRecurringMix,
	FindingShrinkingExistingCustomers,
	FindingHighLostCustomerRevenue,
	FindingHighCustomerConcentration,
	FindingHighTop5Concentration,
	FindingIncreasingConcentration,
	FindingHighConcentrationScenarioImpact,
	FindingLargeNormalizationBurden,
	FindingLargeOwnerDiscretionaryComponent,
	FindingRepeatedOneTimeAdjustments,
	FindingNonOperatingIncomeReliance,
	FindingNegativeMaintainableEarnings,
	FindingHighOwnerDiscretionaryShare,
	FindingExpenseSpike,
	FindingNewMaterialExpense,
	FindingUnusualAccountActivity,
	FindingRepeatedUnusualValue,
	FindingUnfavorableCostBenchmark,
	FindingMaterialUnfavorableVariance,
	FindingHighDownsideValueSensitivity,
	FindingSaleReadinessBlocker,
	FindingSaleReadinessRisk,
	FindingSaleReadinessStrength,
	FindingSaleReadinessOpportunity,
	FindingModuleUnavailable,
}

// Finding is one diagnostic observation: a stable code, category,
// severity, human-readable title/evidence, its exact source module and
// origin code, the metric value(s) involved, an explanation, and a
// neutrally phrased next step — see Result's doc comment on the
// strengths/concerns/opportunities split this feeds.
type Finding struct {
	// Code is this package's own stable, normalized identifier — see
	// FindingCode.
	Code FindingCode `json:"code"`
	// Category is which of the fixed diagnostic categories this Finding
	// concerns.
	Category Category `json:"category"`
	// Severity is this Finding's structured urgency.
	Severity Severity `json:"severity"`
	// Title is a short, fixed-form human-readable label (e.g. "EBITDA
	// margin compression"), built entirely from already-computed fields —
	// never free text a caller must parse for meaning distinct from Code
	// itself.
	Title string `json:"title"`
	// Evidence is one or more short, fixed-form statements supporting this
	// Finding (e.g. "EBITDA margin fell from 18.2% to 14.1%", "ratio-health
	// signal MARGIN_COMPRESSION is active for FY2025"). Always at least one
	// entry.
	Evidence []string `json:"evidence"`
	// SourceModule names which optional Input field this Finding was mined
	// from.
	SourceModule SourceModule `json:"source_module"`
	// SourceCode is the exact origin Flag/Signal/RuleCode/Status string
	// from SourceModule's own Result (e.g. "MARGIN_COMPRESSION",
	// "WEAKENING_LIQUIDITY", "fail") — full traceability back to the
	// specific field that produced this Finding, independent of Code's own
	// normalized taxonomy.
	SourceCode string `json:"source_code,omitempty"`
	// MetricLabel names the figure Metric holds (e.g. "EBITDA_MARGIN",
	// "DSCR", "LARGEST_ENTITY_SHARE"). Display/matching only.
	MetricLabel string `json:"metric_label,omitempty"`
	// Metric is the underlying figure this Finding was computed from, when
	// a single figure applies. Unavailable when no single figure drives
	// the Finding (e.g. a structural classification with no numeric
	// value).
	Metric Value `json:"metric"`
	// ComparisonLabel names what Comparison represents (e.g. "PRIOR_PERIOD",
	// "THRESHOLD", "BENCHMARK_MEDIAN"). Empty when Comparison is
	// unavailable.
	ComparisonLabel string `json:"comparison_label,omitempty"`
	// Comparison is the prior/threshold/benchmark figure Metric is being
	// measured against, when SourceModule's own Result supplied one (a
	// Signal's Threshold, a covenant's Threshold, a benchmark's median, a
	// Flag's Threshold). Unavailable when no comparison figure applies —
	// this package never fabricates a prior-period comparison SourceModule
	// did not itself already compute (see the package doc comment: this
	// package mines, it does not calculate).
	Comparison Value `json:"comparison"`
	// Period is the reporting period this Finding most directly concerns,
	// when SourceModule's own field carried one. Empty for a Finding
	// computed across a whole history or with no period concept (e.g. a
	// single point-in-time debt/covenant/benchmark comparison).
	Period financial.Period `json:"period,omitempty"`
	// Explanation is a short, fixed-form human-readable statement of why
	// this Finding fired, echoing SourceModule's own Message where one
	// exists.
	Explanation string `json:"explanation"`
	// RecommendedAction is a neutrally phrased next step naming what to
	// investigate (e.g. "review the drivers of margin compression versus
	// the prior period"), never a legal, tax, or investment conclusion and
	// never a promise about outcome — see the package doc comment.
	RecommendedAction string `json:"recommended_action"`
}

// Policy is caller-supplied configuration for the few genuinely
// configurable thresholds this package applies on top of its sibling
// modules' own already-classified output — deliberately small, since this
// package mines pre-classified signals rather than re-deriving thresholds
// its sibling packages already own (e.g. ratios.Signal's own threshold is
// never re-graded here).
type Policy struct {
	// MinCoveragePercentForScore is the lowest Coverage.CoveragePercent at
	// which computeHealthScore still produces a Score — see Score's doc
	// comment. Zero (the default) means no minimum is enforced beyond
	// requiring at least one available module.
	MinCoveragePercentForScore float64 `json:"min_coverage_percent_for_score,omitempty"`
}

// DefaultPolicy is a reasonable, documented starting Policy. Calculate
// never applies this implicitly — a caller who leaves Input.Policy at its
// zero value gets the zero-value Policy's behavior (see each field's doc
// comment), not DefaultPolicy; a caller must pass DefaultPolicy explicitly
// to opt in, mirroring salereadiness.Policy's identical
// no-invented-default discipline.
var DefaultPolicy = Policy{
	MinCoveragePercentForScore: 0.3,
}

// Input bundles everything Calculate needs. Every field beyond
// Dataset/PeriodMeta is optional; Calculate degrades gracefully when a
// field is left at its zero value (a Result with Available == false, an
// empty Snapshots) — see the package doc comment and Coverage.
type Input struct {
	// Dataset is the underlying normalized financial data. Optional; this
	// package draws no finding directly from it (every finding comes from
	// an already-computed sibling Result), but PeriodMeta is only
	// meaningful alongside it for display purposes on Metrics-derived
	// findings.
	Dataset financial.FinancialDataset `json:"dataset"`
	// PeriodMeta provides chronological ordering for Dataset's periods,
	// mirroring every analytics sibling package's identical field.
	// Optional.
	PeriodMeta map[financial.Period]metrics.PeriodInfo `json:"period_meta,omitempty"`

	// Metrics is the already-computed financial/metrics.Result, a
	// contributing source for FindingEarningsVolatility and
	// FindingImprovingProfitability via Metrics.Trend. Optional.
	Metrics metrics.Result `json:"metrics"`
	// Ratios is the already-computed analytics/ratios.Result, the primary
	// source for profitability/liquidity/leverage Findings via
	// Ratios.Signals. Optional.
	Ratios ratios.Result `json:"ratios"`
	// QoE is the already-computed analytics/qoe.Result, the primary source
	// for earnings-quality Findings via QoE.Flags. Optional.
	QoE qoe.Result `json:"qoe"`
	// WorkingCapital is the already-computed
	// analytics/workingcapital.Result, the sole source for working-capital
	// Findings via WorkingCapital.Trend/NWCPercentOfRevenueStatistics.
	// Optional.
	WorkingCapital workingcapital.Result `json:"working_capital"`
	// CashFlow is the already-computed analytics/cashflow.Result, the
	// primary source for cash-conversion Findings via CashFlow.Flags.
	// Optional.
	CashFlow cashflow.Result `json:"cash_flow"`
	// RevenueQuality is the already-computed
	// analytics/revenuequality.Result, the primary source for
	// revenue-quality/growth Findings via RevenueQuality.Flags. Optional.
	RevenueQuality revenuequality.Result `json:"revenue_quality"`
	// Concentration is the already-computed analytics/concentration.Result,
	// the sole source for concentration Findings via Concentration.Flags.
	// Optional.
	Concentration concentration.Result `json:"concentration"`
	// Anomalies is the already-computed analytics/anomalies.Result, the
	// primary source for operational-cost-control Findings via
	// Anomalies.Anomalies. Optional.
	Anomalies anomalies.Result `json:"anomalies"`
	// Variance is the already-computed analytics/variance.Result, a
	// contributing source for operational-cost-control Findings via
	// Variance.MaterialExceptions. Optional.
	Variance variance.Result `json:"variance"`
	// Forecast is the already-computed analytics/forecast.Result. Optional;
	// currently contributes to Coverage only — see the package doc comment
	// on this package mining classified signals, and forecast.Result
	// carrying no Flag/Signal/Status of its own to mine (a caller wanting
	// a forecast-derived finding computes one from ScenarioResult figures
	// upstream and, if desired, feeds it in via a future module).
	Forecast forecast.Result `json:"forecast"`
	// Debt is the already-computed analytics/debt.Result, the primary
	// source for leverage Findings via Debt.Flags. Optional.
	Debt debt.Result `json:"debt"`
	// Covenants is the already-computed analytics/covenants.Result, a
	// contributing source for leverage Findings via Covenants.Tests'
	// Status/WarningBufferStatus. Optional.
	Covenants covenants.Result `json:"covenants"`
	// Benchmarks is the already-computed analytics/benchmarks.Result, the
	// sole source for FindingUnfavorableCostBenchmark via
	// Benchmarks.Comparisons' Favorable classification. Optional.
	Benchmarks benchmarks.Result `json:"benchmarks"`
	// ValueDrivers is the already-computed analytics/valuedrivers.Result, a
	// contributing source for FindingHighDownsideValueSensitivity via
	// ValueDrivers.Scenarios' ConsensusPercentDelta. Optional.
	ValueDrivers valuedrivers.Result `json:"value_drivers"`
	// SaleReadiness is the already-computed
	// transactions/salereadiness.Result, the sole source for
	// transaction-readiness Findings via SaleReadiness.Blockers/Risks/
	// Opportunities. Optional.
	SaleReadiness salereadiness.Result `json:"sale_readiness"`

	// Policy is the caller-supplied threshold configuration for this
	// package's own small set of configurable behaviors. The zero value is
	// itself a legitimate, conservative Policy — see Policy's doc comment.
	Policy Policy `json:"policy"`
}

// IssueSeverity distinguishes an input problem Calculate could not proceed
// past for some portion of the analysis (SeverityError) from one that is
// advisory only (SeverityWarning) — the same two-severity model every
// sibling package uses.
type IssueSeverity string

const (
	IssueSeverityError   IssueSeverity = "error"
	IssueSeverityWarning IssueSeverity = "warning"
)

// IssueCode is a stable identifier for one kind of Calculate-time input
// problem. This package defines its own separate taxonomy, consistent with
// every other package in this repository.
type IssueCode string

const (
	// IssueNoInputSupplied means every optional Input field was left at
	// its zero value / unavailable, so no finding could be mined at all.
	IssueNoInputSupplied IssueCode = "NO_INPUT_SUPPLIED"
	// IssueModuleUnavailable means one specific optional Input module's
	// Result.Available was false (or, for Metrics, Snapshots was empty),
	// narrowing which Findings could be mined. One such Issue is recorded
	// per unavailable module that was at least referenced in Input (i.e.
	// the caller does not receive fifteen boilerplate Issues when they
	// only ever intended to supply one or two modules) — see
	// buildCoverage/buildIssues for the exact "was this field the zero
	// value entirely, vs. non-zero but Available == false" distinction.
	IssueModuleUnavailable IssueCode = "MODULE_UNAVAILABLE"
)

// Issue is a single Calculate-time input finding.
type Issue struct {
	Code     IssueCode     `json:"code"`
	Severity IssueSeverity `json:"severity"`
	Message  string        `json:"message"`
	// Module names which SourceModule this Issue relates to. Empty when
	// the Issue is not module-specific.
	Module SourceModule `json:"module,omitempty"`
}

// HasErrors reports whether any Issue in issues has IssueSeverityError.
//
// Intentionally duplicated from every sibling package's identical
// HasErrors rather than shared — see adjustments.HasErrors's doc comment
// for the full rationale (each package's Issue is a distinct Go type with
// no common interface worth introducing for one boolean function).
func HasErrors(issues []Issue) bool {
	for _, i := range issues {
		if i.Severity == IssueSeverityError {
			return true
		}
	}
	return false
}

// Coverage summarizes how many of this package's fifteen optional Input
// modules were actually available, mirroring
// reporting/management.Coverage's identical one-bool-per-sibling-module
// shape (a closer fit than transactions/salereadiness.Coverage's
// abstract-dimension-count shape, since this package's Findings map to
// named sibling modules rather than to abstract classified dimensions).
type Coverage struct {
	// TotalModules is the fixed number of optional sibling modules this
	// package can draw from — always 15.
	TotalModules int `json:"total_modules"`
	// AvailableModules is how many of the WithX fields below are true.
	AvailableModules int `json:"available_modules"`
	// CoveragePercent is AvailableModules / TotalModules, as a decimal.
	CoveragePercent float64 `json:"coverage_percent"`

	WithMetrics        bool `json:"with_metrics"`
	WithRatios         bool `json:"with_ratios"`
	WithQoE            bool `json:"with_qoe"`
	WithWorkingCapital bool `json:"with_working_capital"`
	WithCashFlow       bool `json:"with_cash_flow"`
	WithRevenueQuality bool `json:"with_revenue_quality"`
	WithConcentration  bool `json:"with_concentration"`
	WithAnomalies      bool `json:"with_anomalies"`
	WithVariance       bool `json:"with_variance"`
	WithForecast       bool `json:"with_forecast"`
	WithDebt           bool `json:"with_debt"`
	WithCovenants      bool `json:"with_covenants"`
	WithBenchmarks     bool `json:"with_benchmarks"`
	WithValueDrivers   bool `json:"with_value_drivers"`
	WithSaleReadiness  bool `json:"with_sale_readiness"`

	// MissingModules lists the fixed name of every module (in Input's own
	// field declaration order) with WithX == false, so a caller need not
	// inspect every WithX field individually to know what to supply next.
	MissingModules []string `json:"missing_modules,omitempty"`
}

// MissingDataArea is one unavailable module and which diagnostic
// categories it would have contributed Findings to — the structured
// counterpart to Coverage.MissingModules, framed around what a reader
// cannot currently learn rather than only what was not supplied.
type MissingDataArea struct {
	Module     SourceModule `json:"module"`
	Categories []Category   `json:"categories"`
	Message    string       `json:"message"`
}

// Score is the optional deterministic 0-100 heuristic overall business
// health score — see score.go's computeHealthScore for the exact, fixed
// formula. Present only when Coverage.AvailableModules > 0 and (when
// Policy.MinCoveragePercentForScore is set) Coverage.CoveragePercent meets
// it — a Result with too little coverage has nothing meaningful for a
// formula to weight.
type Score struct {
	// Version echoes ScoreVersion.
	Version string `json:"version"`
	// Value is the final clamped [0, 100] score.
	Value float64 `json:"value"`
	// Components lists every contribution to Value, in categoryOrder, so a
	// caller can reconstruct exactly how Value was reached.
	Components []ScoreComponent `json:"components"`
	// Label is a short, fixed, human-readable characterization of Value's
	// range — purely descriptive; a caller should branch on Value's
	// numeric range directly if it needs to make a decision, never parse
	// Label.
	Label string `json:"label"`
	// Heuristic is always true — this is a heuristic composite over
	// already-classified severities, never a calibrated probability of
	// anything.
	Heuristic bool `json:"heuristic"`
}

// ScoreComponent is one category's contribution to Score.Value.
type ScoreComponent struct {
	Category     Category `json:"category"`
	FindingCount int      `json:"finding_count"`
	Points       float64  `json:"points"`
}

// Result is the output of Calculate: every mined diagnostic Finding,
// grouped into Strengths/Concerns/Opportunities, missing-data areas,
// module coverage, an optional overall health score, and formula version.
type Result struct {
	// FormulaVersion identifies which version of this package's fixed
	// mining/classification rule set produced this Result.
	FormulaVersion string `json:"formula_version"`
	// Available is false only if Calculate could not proceed at all —
	// every optional Input field was left at its zero value / unavailable,
	// so nothing could be mined. Every other field is then zero-value
	// except FormulaVersion and Errors — mirroring
	// salereadiness.Result.Available's identical "every other field is
	// zero-value" convention.
	Available bool `json:"available"`

	// Findings is every mined diagnostic observation, across every
	// Category, in a fixed deterministic order (categoryOrder, then
	// Severity descending, then FindingCode declaration order, then
	// Period ascending) — see sortFindings.
	Findings []Finding `json:"findings,omitempty"`

	// Strengths is every Finding with SeverityInfo whose SourceModule
	// classification was explicitly positive (e.g. ratios'
	// IMPROVING_PROFITABILITY signal, a salereadiness Strength) — the
	// structured counterpart to Concerns, since a caller should not have
	// to infer "this is fine" from the absence of a Concern.
	Strengths []Finding `json:"strengths,omitempty"`
	// Concerns is every Finding with SeverityWarning or SeverityCritical —
	// the negative-signal findings a reviewer would act on first.
	Concerns []Finding `json:"concerns,omitempty"`
	// Opportunities is every Finding this package frames as an improvement
	// area rather than a strength or an acute concern — currently sourced
	// from SaleReadiness.Opportunities (see mineSaleReadiness) and any
	// Finding explicitly marked as an opportunity by its mining function.
	Opportunities []Finding `json:"opportunities,omitempty"`

	// MissingDataAreas is every unavailable module and what categories it
	// would have contributed to, in Coverage.MissingModules' order.
	MissingDataAreas []MissingDataArea `json:"missing_data_areas,omitempty"`

	// Coverage summarizes how many of the 15 optional modules were
	// actually available.
	Coverage Coverage `json:"coverage"`

	// OverallHealthScore is the optional composite score — nil when
	// Coverage.AvailableModules == 0, or when Policy.MinCoveragePercentForScore
	// is set and not met (Prompt 35: "overall health score only with
	// explicit formula"; see score.go).
	OverallHealthScore *Score `json:"overall_health_score,omitempty"`

	// Policy echoes Input.Policy this Result was assessed against.
	Policy Policy `json:"policy"`

	// Warnings carries every Issue with IssueSeverityWarning.
	Warnings []Issue `json:"warnings,omitempty"`
	// Errors carries every Issue with IssueSeverityError.
	Errors []Issue `json:"errors,omitempty"`
}
