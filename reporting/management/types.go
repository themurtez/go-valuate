// Package management assembles presentation-neutral management-reporting
// data from whichever of this repository's already-computed financial and
// analytics results a caller has on hand: financial/metrics,
// analytics/ratios, analytics/cashflow, analytics/workingcapital,
// analytics/qoe, analytics/variance, analytics/forecast,
// analytics/anomalies, analytics/concentration, analytics/revenuequality,
// analytics/debt, analytics/covenants, and valuation/consensus.
//
// This package computes no figure of its own from a
// financial.FinancialDataset, an adjustment set, or a customer ledger — see
// [Calculate]'s doc comment. It is a pure aggregation/reshaping layer over
// results other packages already produced deterministically, exactly like
// transactions/salereadiness and portfolio/diagnostics. Every input is
// optional; an absent input never fabricates a figure, it only narrows
// [Report.Coverage] and is listed in [Coverage.MissingModules].
//
// # No presentation, no narrative
//
// [Calculate] produces structured data only: numbers, labels, and
// deterministic classifications. It never renders PDF, HTML, or charts, and
// it never generates narrative text via AI/LLM — every string field here is
// either a caller-supplied label passed through unchanged or a short,
// fixed-form template built entirely from already-computed fields (the same
// discipline every sibling package in this repository already follows for
// its own Explanation/Reason/Message fields). A caller wanting a rendered
// report or a written narrative takes this package's [Report] as its data
// source and builds that presentation layer itself.
//
// # Sections, not dimensions or findings
//
// Unlike transactions/salereadiness (which classifies fixed dimensions) or
// portfolio/diagnostics (which ranks findings across a portfolio), this
// package's output is organized into fixed report sections identified by
// [SectionCode] — an executive KPI summary, historical/profitability/
// liquidity-leverage/cash-flow/working-capital series, variance and
// forecast tables, a top-issues rollup, and chart-ready series — always
// populated in sectionOrder, one typed field of content per section on
// [Report], each independently reporting its own availability. This
// mirrors valuation/orchestrator.Run.Methods' "fixed order regardless of
// availability" rule, the closest existing precedent for this pattern.
//
// # Determinism
//
// Every function here is pure: no I/O, no mutation of caller-owned input,
// no package-global mutable state, no wall-clock/randomness. Calculate can
// be called concurrently and repeatedly against identical input and always
// returns byte-for-byte identical JSON — see determinism_test.go.
package management

import (
	"github.com/themurtez/go-valuate/analytics/anomalies"
	"github.com/themurtez/go-valuate/analytics/cashflow"
	"github.com/themurtez/go-valuate/analytics/concentration"
	"github.com/themurtez/go-valuate/analytics/covenants"
	"github.com/themurtez/go-valuate/analytics/debt"
	"github.com/themurtez/go-valuate/analytics/forecast"
	"github.com/themurtez/go-valuate/analytics/qoe"
	"github.com/themurtez/go-valuate/analytics/ratios"
	"github.com/themurtez/go-valuate/analytics/revenuequality"
	"github.com/themurtez/go-valuate/analytics/variance"
	"github.com/themurtez/go-valuate/analytics/workingcapital"
	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/metrics"
	"github.com/themurtez/go-valuate/valuation/consensus"
)

// FormulaVersion identifies this package's fixed rule set: which sibling
// fields populate each [Section] (series.go, tables.go, issues.go,
// chart.go), the KPI-selection rule (kpi.go), and the
// coverage/version-echo computation (coverage.go). Bump whenever any of
// that changes in a way that could make a historical [Result] not
// reproduce identically under new code — see the repository README's
// versioning-strategy section, which this constant follows exactly
// (qoe.FormulaVersion, salereadiness.FormulaVersion, diagnostics.FormulaVersion,
// etc.).
const FormulaVersion = "1.0.0"

// Value represents a single figure that may or may not be available,
// distinguishing "computed/reported to be exactly 0" from "unknown because
// a required input was absent" — this package's own copy of the convention
// every sibling package duplicates locally (see
// transactions/salereadiness.Value's doc comment for the full rationale)
// rather than importing another package's Value.
type Value struct {
	Available bool    `json:"available"`
	Amount    float64 `json:"amount"`
}

// Unavailable is the canonical zero-information Value.
func Unavailable() Value { return Value{} }

// AvailableValue reports a Value for a known figure (which may legitimately
// be zero or negative).
func AvailableValue(v float64) Value { return Value{Available: true, Amount: v} }

// fromMetricValue converts a financial/metrics.MetricValue into this
// package's own Value, the standard bridge every section builder uses when
// reading a sibling package's own Available/Value-shaped figure.
func fromMetricValue(v metrics.MetricValue) Value {
	return Value{Available: v.Available, Amount: v.Value}
}

// Input bundles every optional analytics result this package can draw a
// report from. Every field beyond Dataset/PeriodMeta is independently
// optional; Calculate degrades gracefully when a field is left at its zero
// value (a Result with Available == false, an empty slice) — see each
// section type's own doc comment (series_types.go, table_types.go,
// issue_types.go) for exactly which Input field(s) populate it and what
// happens when they are absent.
//
// Every embedded Result is a value type populated by the caller's own
// already-computed call to that package's Calculate (or, for Consensus,
// consensus.Calculate — see Consensus's doc comment for its distinct
// signature). This package never calls any sibling package's Calculate
// itself and never mutates any field of Input.
type Input struct {
	// Dataset is the underlying normalized financial data, used only as a
	// last-resort source of Periods() for HistoricalSeries when Metrics is
	// not supplied. Optional.
	Dataset financial.FinancialDataset `json:"dataset"`
	// PeriodMeta provides chronological ordering for Dataset/Metrics
	// periods, mirroring every analytics sibling package's identical field.
	// Optional — absent PeriodMeta only affects which period
	// ExecutiveSummary treats as "most recent" when more than one candidate
	// exists.
	PeriodMeta map[financial.Period]metrics.PeriodInfo `json:"period_meta,omitempty"`

	// Metrics is the already-computed financial/metrics.Result (from
	// metrics.Calculate), the primary source for HistoricalSeries,
	// ProfitabilitySeries, and ExecutiveSummary. Optional.
	Metrics metrics.Result `json:"metrics"`
	// Ratios is the already-computed analytics/ratios.Result (from
	// ratios.Calculate), the primary source for LiquidityLeverageSeries and
	// a contributing source for ProfitabilitySeries. Optional.
	Ratios ratios.Result `json:"ratios"`
	// CashFlow is the already-computed analytics/cashflow.Result (from
	// cashflow.Calculate), the sole source for CashFlowSeries. Optional.
	CashFlow cashflow.Result `json:"cash_flow"`
	// WorkingCapital is the already-computed analytics/workingcapital.Result
	// (from workingcapital.Calculate), the sole source for
	// WorkingCapitalSeries. Optional.
	WorkingCapital workingcapital.Result `json:"working_capital"`
	// QoE is the already-computed analytics/qoe.Result (from qoe.Calculate),
	// a contributing source for ExecutiveSummary and TopIssues. Optional.
	QoE qoe.Result `json:"qoe"`
	// Variance is the already-computed analytics/variance.Result (from
	// variance.Calculate), the sole source for VarianceTables. Optional.
	Variance variance.Result `json:"variance"`
	// Forecast is the already-computed analytics/forecast.Result (from
	// forecast.Calculate), the sole source for ForecastTables. Optional.
	Forecast forecast.Result `json:"forecast"`
	// Anomalies is the already-computed analytics/anomalies.Result (from
	// anomalies.Calculate), a contributing source for TopIssues. Optional.
	Anomalies anomalies.Result `json:"anomalies"`
	// Concentration is the already-computed analytics/concentration.Result
	// (from concentration.Calculate), a contributing source for
	// ExecutiveSummary and TopIssues. Optional.
	Concentration concentration.Result `json:"concentration"`
	// RevenueQuality is the already-computed analytics/revenuequality.Result
	// (from revenuequality.Calculate), a contributing source for
	// ExecutiveSummary. Optional.
	RevenueQuality revenuequality.Result `json:"revenue_quality"`
	// Debt is the already-computed analytics/debt.Result (from
	// debt.Calculate), a contributing source for TopIssues (via
	// Debt.Flags). Optional. Debt does not contribute to
	// LiquidityLeverageSeries: analytics/debt.Result's coverage figures are
	// single point-in-time results with no Period of their own, so they
	// have no per-period row to join into that series — see
	// LiquidityLeveragePeriod's doc comment.
	Debt debt.Result `json:"debt"`
	// Covenants is the already-computed analytics/covenants.Result (from
	// covenants.Calculate), a contributing source for TopIssues. Optional.
	Covenants covenants.Result `json:"covenants"`
	// Consensus is the already-computed valuation/consensus.Result (from
	// consensus.Calculate(inputs []consensus.Input, opts) — note this one
	// sibling entrypoint takes a slice, unlike every other Input field's
	// single-value producer), a contributing source for ExecutiveSummary.
	// Optional.
	Consensus consensus.Result `json:"consensus"`
}

// IssueSeverity distinguishes an input problem Calculate could not proceed
// past for some portion of the analysis (IssueSeverityError) from one that
// is advisory only (IssueSeverityWarning) — this package's own two-severity
// model, matching every sibling package's identical convention.
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
	// IssueNoInputSupplied means every optional Input field was left at its
	// zero value / unavailable, so no section could be populated at all.
	IssueNoInputSupplied IssueCode = "NO_INPUT_SUPPLIED"
	// IssueMetricsUnavailable means Input.Metrics.Snapshots was empty, so
	// HistoricalSeries and ExecutiveSummary's metrics-sourced figures could
	// not be populated. Advisory only.
	IssueMetricsUnavailable IssueCode = "METRICS_UNAVAILABLE"
	// IssueRatiosUnavailable means Input.Ratios.Available was false, so
	// LiquidityLeverageSeries could not be populated. Advisory only.
	IssueRatiosUnavailable IssueCode = "RATIOS_UNAVAILABLE"
	// IssueCashFlowUnavailable means Input.CashFlow.Available was false, so
	// CashFlowSeries could not be populated. Advisory only.
	IssueCashFlowUnavailable IssueCode = "CASH_FLOW_UNAVAILABLE"
	// IssueWorkingCapitalUnavailable means Input.WorkingCapital.Available
	// was false, so WorkingCapitalSeries could not be populated. Advisory
	// only.
	IssueWorkingCapitalUnavailable IssueCode = "WORKING_CAPITAL_UNAVAILABLE"
	// IssueQoEUnavailable means Input.QoE.Available was false. Advisory
	// only.
	IssueQoEUnavailable IssueCode = "QOE_UNAVAILABLE"
	// IssueVarianceUnavailable means Input.Variance.Available was false, so
	// VarianceTables could not be populated. Advisory only.
	IssueVarianceUnavailable IssueCode = "VARIANCE_UNAVAILABLE"
	// IssueForecastUnavailable means Input.Forecast.Available was false, so
	// ForecastTables could not be populated. Advisory only.
	IssueForecastUnavailable IssueCode = "FORECAST_UNAVAILABLE"
	// IssueAnomaliesUnavailable means Input.Anomalies.Available was false.
	// Advisory only.
	IssueAnomaliesUnavailable IssueCode = "ANOMALIES_UNAVAILABLE"
	// IssueConcentrationUnavailable means Input.Concentration.Available was
	// false. Advisory only.
	IssueConcentrationUnavailable IssueCode = "CONCENTRATION_UNAVAILABLE"
	// IssueRevenueQualityUnavailable means Input.RevenueQuality.Available
	// was false. Advisory only.
	IssueRevenueQualityUnavailable IssueCode = "REVENUE_QUALITY_UNAVAILABLE"
	// IssueDebtUnavailable means Input.Debt.Available was false. Advisory
	// only.
	IssueDebtUnavailable IssueCode = "DEBT_UNAVAILABLE"
	// IssueCovenantsUnavailable means Input.Covenants.Available was false.
	// Advisory only.
	IssueCovenantsUnavailable IssueCode = "COVENANTS_UNAVAILABLE"
	// IssueConsensusUnavailable means Input.Consensus.Available was false.
	// Advisory only.
	IssueConsensusUnavailable IssueCode = "CONSENSUS_UNAVAILABLE"
	// IssueNoPeriodMetaForHistoricalOrder means Input.PeriodMeta was empty
	// or did not cover every period in the snapshots HistoricalSeries was
	// built from, so HistoricalSeries.Periods (and every section/KPI
	// derived from it) falls back to those snapshots' own original order,
	// which is not guaranteed chronological — see orderedSnapshots.
	// Advisory only.
	IssueNoPeriodMetaForHistoricalOrder IssueCode = "NO_PERIOD_META_FOR_HISTORICAL_ORDER"
)

// Issue is a single Calculate-time input finding.
type Issue struct {
	Code     IssueCode     `json:"code"`
	Severity IssueSeverity `json:"severity"`
	Message  string        `json:"message"`
}

// HasErrors reports whether any Issue in issues has IssueSeverityError.
//
// Intentionally duplicated from every sibling package's identical
// HasErrors rather than shared — see
// transactions/salereadiness.HasErrors's doc comment for the full
// rationale (each package's Issue is a distinct Go type with no common
// interface worth introducing for one boolean function).
func HasErrors(issues []Issue) bool {
	for _, i := range issues {
		if i.Severity == IssueSeverityError {
			return true
		}
	}
	return false
}

// SectionCode is a stable identifier for one fixed report section.
// Declaration order below is Result's field order and every
// section-ordered list's fixed order — see sectionOrder.
type SectionCode string

const (
	SectionExecutiveSummary        SectionCode = "EXECUTIVE_SUMMARY"
	SectionHistoricalSeries        SectionCode = "HISTORICAL_SERIES"
	SectionProfitabilitySeries     SectionCode = "PROFITABILITY_SERIES"
	SectionLiquidityLeverageSeries SectionCode = "LIQUIDITY_LEVERAGE_SERIES"
	SectionCashFlowSeries          SectionCode = "CASH_FLOW_SERIES"
	SectionWorkingCapitalSeries    SectionCode = "WORKING_CAPITAL_SERIES"
	SectionVarianceTables          SectionCode = "VARIANCE_TABLES"
	SectionForecastTables          SectionCode = "FORECAST_TABLES"
	SectionTopIssues               SectionCode = "TOP_ISSUES"
	SectionChartSeries             SectionCode = "CHART_SERIES"
)

// sectionOrder is SectionCode's fixed declaration/output order, mirrored by
// Result's field order and by ModuleVersions/Coverage's per-section
// ordering.
var sectionOrder = []SectionCode{
	SectionExecutiveSummary,
	SectionHistoricalSeries,
	SectionProfitabilitySeries,
	SectionLiquidityLeverageSeries,
	SectionCashFlowSeries,
	SectionWorkingCapitalSeries,
	SectionVarianceTables,
	SectionForecastTables,
	SectionTopIssues,
	SectionChartSeries,
}

// KPI is one named headline figure in ExecutiveSummary, e.g. "Total
// Revenue" or "EBITDA Margin" for the most recent available period.
type KPI struct {
	// Label is a short, fixed, human-readable name for this figure (e.g.
	// "Total Revenue", "EBITDA Margin", "Net Debt / EBITDA").
	Label string `json:"label"`
	// Value is this KPI's figure for Period.
	Value Value `json:"value"`
	// Unit describes how Value should be displayed — see UnitCurrency etc.
	Unit Unit `json:"unit"`
	// Period is the period Value applies to.
	Period financial.Period `json:"period,omitempty"`
	// PriorValue is the same KPI for the immediately preceding period in
	// the same series, when one exists, for a caller wanting a
	// period-over-period delta without recomputing it from a series.
	// Unavailable when no prior period exists or PriorValue itself could
	// not be computed.
	PriorValue Value `json:"prior_value"`
	// Change is (Value - PriorValue) / |PriorValue|, as a decimal.
	// Available only when both Value and PriorValue are Available and
	// PriorValue.Amount is nonzero.
	Change Value `json:"change"`
	// Source names which Input field this KPI was derived from (e.g.
	// "metrics", "ratios", "consensus"), for traceability.
	Source string `json:"source,omitempty"`
}

// Unit is a stable identifier for how a Value should be displayed —
// purely descriptive; this package performs no unit conversion or
// formatting itself.
type Unit string

const (
	UnitCurrency Unit = "currency"
	UnitPercent  Unit = "percent"
	UnitMultiple Unit = "multiple"
	UnitDays     Unit = "days"
	UnitCount    Unit = "count"
	UnitMonths   Unit = "months"
)

// ExecutiveSummary is the top-level headline-figures section: a small,
// fixed set of KPIs for the most recent available period, each
// independently sourced from whichever Input field supplies it.
type ExecutiveSummary struct {
	// Available is true when at least one KPI could be populated.
	Available bool `json:"available"`
	// Period is the most recent period across every KPI actually included
	// in KPIs, per periodIsAfter's best-effort chronological comparison
	// (exact when Input.PeriodMeta covers every candidate period, a plain
	// string comparison otherwise). Empty when KPIs is empty. Individual
	// KPI entries may reference an earlier period than Period when their
	// own source series ends earlier — see KPI.Period.
	Period financial.Period `json:"period,omitempty"`
	// KPIs is every headline figure this package could populate, in a
	// fixed declaration order — see buildExecutiveSummary. Never includes
	// an entry for a KPI whose underlying figure was entirely unavailable;
	// such a KPI is omitted from the slice rather than included with
	// Value.Available == false, so callers see only what could actually be
	// reported (an unavailable KPI's absence is instead reflected in
	// Coverage).
	KPIs []KPI `json:"kpis,omitempty"`
}
