// Package salereadiness assesses how prepared a business is for a sale
// process, from whichever of its already-computed financial/analytics
// results a caller has on hand plus a caller-supplied business profile and
// data-quality read. It answers "what would a buyer's diligence likely
// flag, and what is still missing to know," never "will this business
// sell" or "what is it worth" — see Calculate's doc comment and the
// explicit prohibition against a guaranteed-sale claim below.
//
// This package computes no figure of its own from a
// financial.FinancialDataset, an adjustment set, or a customer ledger. It
// is a pure aggregation layer over results other packages already produced
// deterministically: analytics/qoe (earnings quality/normalization),
// analytics/workingcapital (working-capital stability), analytics/
// concentration (customer/counterparty concentration), analytics/
// revenuequality (recurring-revenue composition), valuation/consensus
// (method agreement), financial/metrics (margin trend, leverage), and
// valuation/profile (owner dependence, qualitative reads). Every one of
// those inputs is optional — see Input's doc comment — and an absent input
// never contributes a negative signal; it only narrows Result.Coverage and
// is listed in Result.MissingInformation.
//
// Every dimension in Dimensions is a deterministic, rule-based
// classification (a Status, not a machine-scored probability) driven by
// caller-supplied Policy thresholds and the fixed rules documented on each
// dimension's DimensionCode constant. Result.OverallScore is a single
// explicit, fixed formula over the dimension Statuses actually assessed
// (see score.go) — never an opaque or learned weighting, and never
// computed when Coverage.AssessedDimensions is zero.
//
// This package makes no representation, express or implied, that a
// business found "ready" will sell, will sell at any particular price, or
// will sell within any particular timeframe. Every Blocker, Risk,
// Strength, and Opportunity is a factual observation about the supplied
// data, not a guarantee about a future transaction outcome.
//
// Every function here is pure: no I/O, no mutation of caller-owned input,
// no package-global mutable state. Calculate can be called concurrently and
// repeatedly against identical input and always returns byte-for-byte
// identical JSON — see determinism_test.go.
package salereadiness

import (
	"github.com/themurtez/go-valuate/analytics/concentration"
	"github.com/themurtez/go-valuate/analytics/qoe"
	"github.com/themurtez/go-valuate/analytics/revenuequality"
	"github.com/themurtez/go-valuate/analytics/workingcapital"
	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/metrics"
	"github.com/themurtez/go-valuate/valuation/consensus"
	"github.com/themurtez/go-valuate/valuation/profile"
)

// FormulaVersion identifies this package's fixed rule set: every
// DimensionCode's classification thresholds and precedence-of-inputs rule
// (dimensions.go), the Blocker/Risk/Strength/MissingInformation/
// Opportunity derivation rules (findings.go), and DefaultPolicy. Bump
// whenever any of that changes in a way that could make a historical
// Result not reproduce identically under new code — see the repository
// README's versioning-strategy section, which this constant follows
// exactly (qoe.FormulaVersion, concentration.FormulaVersion,
// acquisition.FormulaVersion, etc.).
const FormulaVersion = "1.0.0"

// ScoreVersion identifies the exact formula computeOverallScore implements
// (score.go). Distinct from FormulaVersion for the same reason
// qoe.ScoreVersion is distinct from qoe.FormulaVersion: a caller may
// reasonably want to change how dimensions are classified independently of
// how already-classified dimensions are weighted into one composite
// number.
const ScoreVersion = "1.0.0"

// Value represents a single figure that may or may not be available,
// distinguishing "computed/reported to be exactly 0" from "unknown because
// a required input was absent" — this package's own copy of the
// convention every sibling package duplicates locally (see
// acquisition.Value's doc comment for the full rationale) rather than
// importing another package's Value.
type Value struct {
	Available bool    `json:"available"`
	Amount    float64 `json:"amount"`
}

// Unavailable is the canonical zero-information Value.
func Unavailable() Value { return Value{} }

// AvailableValue reports a Value for a known figure (which may legitimately
// be zero or negative).
func AvailableValue(v float64) Value { return Value{Available: true, Amount: v} }

// DataQuality captures caller-supplied indicators of the underlying
// financial records' and supporting documentation's quality — signals this
// package cannot derive from any of its other optional inputs, since none
// of them inspect source-document completeness or bookkeeping practice
// directly. Every field is a plain bool/pointer; the zero value of the
// struct (every bool false, every pointer nil) is a legitimate, maximally
// conservative "nothing confirmed" state, mirroring
// valuation/profile.DataAvailability's convention.
type DataQuality struct {
	// HasReviewedOrAuditedFinancials indicates whether at least one period
	// of financials has been reviewed or audited by an independent
	// accountant, as opposed to internally prepared only.
	HasReviewedOrAuditedFinancials bool `json:"has_reviewed_or_audited_financials,omitempty"`
	// HasMultiYearFinancials indicates whether more than one period of
	// financial history is available — mirrors
	// profile.DataAvailability.HasMultiYearFinancials, duplicated here
	// since a caller may supply DataQuality without a full Profile.
	HasMultiYearFinancials bool `json:"has_multi_year_financials,omitempty"`
	// HasMonthlyDetail indicates whether monthly (not just annual/
	// quarterly) financial detail is available, which affects how
	// thoroughly a buyer can verify seasonality and timing.
	HasMonthlyDetail bool `json:"has_monthly_detail,omitempty"`
	// HasTaxReturnReconciliation indicates whether the caller has confirmed
	// the financial statements reconcile to filed tax returns.
	HasTaxReturnReconciliation bool `json:"has_tax_return_reconciliation,omitempty"`
	// HasFormalAdjustmentDocumentation indicates whether every proposed
	// add-back/adjustment (see QoE.Adjustments) has supporting
	// documentation on file, as opposed to being asserted without evidence.
	HasFormalAdjustmentDocumentation bool `json:"has_formal_adjustment_documentation,omitempty"`
	// OpenAccountingIssueCount is the number of known open bookkeeping/
	// accounting issues (e.g. unreconciled accounts, restatements in
	// progress, pending disputes with a former accountant). nil means
	// unknown; 0 means confirmed none.
	OpenAccountingIssueCount *int `json:"open_accounting_issue_count,omitempty"`
}

// Policy is the caller-supplied threshold/weighting configuration driving
// every dimension classification in Dimensions. Every threshold's zero
// value means "not specified" (that dimension's check falls back to
// Status Unassessed rather than silently applying an invented default) —
// the same fully caller-driven threshold model analytics/debt.LenderPolicy
// and transactions/acquisition.RedFlagThresholds already established.
type Policy struct {
	// MinYearsHistoryForStrong is the minimum number of financial-history
	// years (financial.FinancialDataset.Periods(), or
	// DataQuality.HasMultiYearFinancials as a coarser fallback) required
	// for FinancialRecordQuality to reach StatusStrong. Zero means not
	// specified (the years-of-history check is skipped; the dimension is
	// still assessed from DataQuality's other fields when available).
	MinYearsHistoryForStrong int `json:"min_years_history_for_strong,omitempty"`
	// MaxAcceptableEarningsVolatility is the highest
	// qoe.Result.EBITDAVolatility (or, absent QoE,
	// metrics.Trend.EBITDAVolatility) decimal value still classified
	// StatusAcceptable for EarningsStability. Zero means not specified.
	MaxAcceptableEarningsVolatility float64 `json:"max_acceptable_earnings_volatility,omitempty"`
	// MaxAcceptableAdjustmentToEBITDARatio is the highest
	// qoe.Result.Ratios.AdjustmentToEBITDA decimal value still classified
	// StatusAcceptable for NormalizationBurden. Zero means not specified.
	MaxAcceptableAdjustmentToEBITDARatio float64 `json:"max_acceptable_adjustment_to_ebitda_ratio,omitempty"`
	// MaxAcceptableLargestCustomerShare is the highest
	// concentration.PeriodConcentration.LargestEntityShare decimal value
	// still classified StatusAcceptable for CustomerConcentration. Zero
	// means not specified.
	MaxAcceptableLargestCustomerShare float64 `json:"max_acceptable_largest_customer_share,omitempty"`
	// MinAcceptableRecurringRevenuePercent is the lowest recurring-revenue
	// decimal share (from revenuequality.PeriodRevenue.RecurringPercent, or
	// profile.Profile.RecurringRevenuePercent as a fallback) still
	// classified StatusAcceptable for RecurringRevenue. Zero means not
	// specified.
	MinAcceptableRecurringRevenuePercent float64 `json:"min_acceptable_recurring_revenue_percent,omitempty"`
	// MaxAcceptableNWCVolatility is the highest
	// workingcapital.Statistics.Volatility decimal value still classified
	// StatusAcceptable for WorkingCapitalStability. Zero means not
	// specified.
	MaxAcceptableNWCVolatility float64 `json:"max_acceptable_nwc_volatility,omitempty"`
	// MaxAcceptableNetDebtToEBITDA is the highest NetDebt/EBITDA decimal
	// multiple (from the most recent metrics.Snapshot) still classified
	// StatusAcceptable for DebtLeverage. Zero means not specified.
	MaxAcceptableNetDebtToEBITDA float64 `json:"max_acceptable_net_debt_to_ebitda,omitempty"`
	// MaxAcceptableValuationDispersion is the highest
	// consensus.Statistics.CoefficientOfVariation decimal value still
	// classified StatusAcceptable for ValuationMethodConsensus. Zero means
	// not specified.
	MaxAcceptableValuationDispersion float64 `json:"max_acceptable_valuation_dispersion,omitempty"`
}

// Input bundles everything Calculate needs. Every field beyond Policy is
// optional; Calculate degrades gracefully when a field is left at its zero
// value (Dataset with no Items, a Result with Available == false, a nil
// pointer) — see the package doc comment and each dimension's own doc
// comment in dimensions.go for exactly which fields become
// StatusUnassessed when an input is missing.
type Input struct {
	// Dataset is the underlying normalized financial data, used only for
	// FinancialRecordQuality's period-count check and as a last-resort
	// fallback when neither Metrics nor QoE is supplied. Optional.
	Dataset financial.FinancialDataset `json:"dataset"`
	// PeriodMeta provides chronological ordering for Dataset's periods,
	// mirroring every analytics sibling package's identical field. Optional
	// — absent PeriodMeta only affects which periods MarginTrend treats as
	// "most recent" when Metrics is supplied without its own Trend.
	PeriodMeta map[financial.Period]metrics.PeriodInfo `json:"period_meta,omitempty"`
	// Metrics is the already-computed metrics.Result for Dataset, used for
	// MarginTrend (via Metrics.Trend) and DebtLeverage (via the most recent
	// Snapshot's NetDebt/EBITDA). Optional; a nil/zero Result degrades both
	// dimensions to StatusUnassessed unless QoE separately supplies a
	// margin trend.
	Metrics metrics.Result `json:"metrics"`
	// QoE is the already-computed analytics/qoe.Result, the primary source
	// for EarningsStability, NormalizationBurden, and a contributing source
	// for OwnerDependence and MarginTrend. Optional; a Result with
	// Available == false degrades every dimension that depends on it to
	// StatusUnassessed.
	QoE qoe.Result `json:"qoe"`
	// WorkingCapital is the already-computed analytics/workingcapital.Result,
	// the sole source for WorkingCapitalStability. Optional.
	WorkingCapital workingcapital.Result `json:"working_capital"`
	// Concentration is the already-computed analytics/concentration.Result,
	// the sole source for CustomerConcentration. Optional.
	Concentration concentration.Result `json:"concentration"`
	// RevenueQuality is the already-computed analytics/revenuequality.Result,
	// the primary source for RecurringRevenue. Optional.
	RevenueQuality revenuequality.Result `json:"revenue_quality"`
	// Consensus is the already-computed valuation/consensus.Result, the
	// sole source for ValuationMethodConsensus. Optional.
	Consensus consensus.Result `json:"consensus"`
	// Profile is the caller-supplied business profile, a contributing
	// source for OwnerDependence, RecurringRevenue (as a fallback), and
	// FinancialRecordQuality (via Profile.DataAvailability). Optional;
	// every Profile field is independently optional — see profile.Profile's
	// own doc comment.
	Profile profile.Profile `json:"profile"`
	// DataQuality is the caller-supplied document/record-quality read, the
	// primary source for FinancialRecordQuality and a contributing source
	// for DataCompleteness. Optional.
	DataQuality DataQuality `json:"data_quality"`
	// Policy is the caller-supplied threshold configuration. The zero value
	// means every threshold-driven check is skipped (see Policy's doc
	// comment); dimensions still classify StatusUnassessed vs. assessed
	// purely on input availability regardless of Policy.
	Policy Policy `json:"policy"`
}

// DimensionCode is a stable identifier for one sale-readiness dimension.
// Declaration order below is Result.Dimensions' fixed output order — see
// the repository README's deterministic-ordering-guarantees section, which
// this package's own new row follows exactly.
type DimensionCode string

const (
	// DimensionFinancialRecordQuality reflects how complete, reviewed, and
	// well-documented the underlying financial records are — see
	// classifyFinancialRecordQuality.
	DimensionFinancialRecordQuality DimensionCode = "FINANCIAL_RECORD_QUALITY"
	// DimensionEarningsStability reflects how volatile historical
	// EBITDA/SDE has been — see classifyEarningsStability.
	DimensionEarningsStability DimensionCode = "EARNINGS_STABILITY"
	// DimensionNormalizationBurden reflects how large the add-back/
	// adjustment burden is relative to reported EBITDA — a heavier burden
	// means a buyer's diligence will scrutinize more of the earnings
	// narrative — see classifyNormalizationBurden.
	DimensionNormalizationBurden DimensionCode = "NORMALIZATION_BURDEN"
	// DimensionCustomerConcentration reflects how dependent revenue is on
	// its largest customer(s) — see classifyCustomerConcentration.
	DimensionCustomerConcentration DimensionCode = "CUSTOMER_CONCENTRATION"
	// DimensionRecurringRevenue reflects what share of revenue is
	// contractually recurring — see classifyRecurringRevenue.
	DimensionRecurringRevenue DimensionCode = "RECURRING_REVENUE"
	// DimensionOwnerDependence reflects how much of operations and
	// earnings depend on the current owner's direct involvement — see
	// classifyOwnerDependence.
	DimensionOwnerDependence DimensionCode = "OWNER_DEPENDENCE"
	// DimensionMarginTrend reflects whether gross/EBITDA margins have been
	// improving, stable, or eroding — see classifyMarginTrend.
	DimensionMarginTrend DimensionCode = "MARGIN_TREND"
	// DimensionWorkingCapitalStability reflects how volatile net working
	// capital has been relative to revenue — see
	// classifyWorkingCapitalStability.
	DimensionWorkingCapitalStability DimensionCode = "WORKING_CAPITAL_STABILITY"
	// DimensionDebtLeverage reflects the business's net-debt-to-EBITDA
	// multiple — see classifyDebtLeverage.
	DimensionDebtLeverage DimensionCode = "DEBT_LEVERAGE"
	// DimensionDataCompleteness reflects how many of this package's
	// optional inputs were actually supplied and available — distinct from
	// FinancialRecordQuality, which reflects the quality of the records
	// themselves rather than how many analytical modules ran over them —
	// see classifyDataCompleteness.
	DimensionDataCompleteness DimensionCode = "DATA_COMPLETENESS"
	// DimensionValuationMethodConsensus reflects how tightly the valuation
	// methods included in Input.Consensus agree — see
	// classifyValuationMethodConsensus.
	DimensionValuationMethodConsensus DimensionCode = "VALUATION_METHOD_CONSENSUS"
)

// dimensionOrder is DimensionCode's fixed declaration/output order,
// duplicated here as a slice so buildDimensions and any future ordering
// check iterate it once rather than re-listing the ten codes by hand.
var dimensionOrder = []DimensionCode{
	DimensionFinancialRecordQuality,
	DimensionEarningsStability,
	DimensionNormalizationBurden,
	DimensionCustomerConcentration,
	DimensionRecurringRevenue,
	DimensionOwnerDependence,
	DimensionMarginTrend,
	DimensionWorkingCapitalStability,
	DimensionDebtLeverage,
	DimensionDataCompleteness,
	DimensionValuationMethodConsensus,
}

// Status is a coarse, deterministic classification of one Dimension,
// computed from a fixed rule (documented on the corresponding classify*
// function) and, where Policy supplies a threshold, that threshold —
// mirroring workingcapital.TrendDirection/review.ReadinessState's identical
// "structured classification, never inferred from free text" convention.
// This package deliberately does not assign a per-dimension numeric score:
// Prompt 31 calls for "dimension scores or statuses," and a Status is the
// honest representation here since no single per-dimension formula would
// be more meaningful than the classification itself (see score.go for the
// one place this package does compute an explicit numeric formula, the
// overall Score).
type Status string

const (
	// StatusStrong means the dimension was assessed and found to pose no
	// meaningful concern under the applicable Policy threshold (or fixed
	// rule, where no threshold applies).
	StatusStrong Status = "STRONG"
	// StatusAcceptable means the dimension was assessed and found within
	// normal/tolerable range.
	StatusAcceptable Status = "ACCEPTABLE"
	// StatusWeak means the dimension was assessed and found below normal
	// range — a likely diligence question, not yet a blocker.
	StatusWeak Status = "WEAK"
	// StatusConcerning means the dimension was assessed and found to pose a
	// material concern a buyer's diligence would very likely flag.
	StatusConcerning Status = "CONCERNING"
	// StatusUnassessed means the input(s) this dimension depends on were
	// not supplied or were unavailable — the dimension contributes nothing
	// to Result.OverallScore and is listed in Result.MissingInformation
	// instead of being scored as though it were absent-and-therefore-bad.
	// This is the mechanism implementing Prompt 31's "missing modules
	// reduce coverage, not silently score zero" requirement at the
	// dimension level.
	StatusUnassessed Status = "UNASSESSED"
)

// Dimension is one sale-readiness dimension's classification, the concrete
// figure(s) it was classified from, and a human-readable explanation.
type Dimension struct {
	// Code identifies which dimension this is.
	Code DimensionCode `json:"code"`
	// Status is the deterministic classification.
	Status Status `json:"status"`
	// Value is the underlying figure the classification was computed from
	// (e.g. EBITDA volatility as a decimal, largest-customer share as a
	// decimal), when Status is not StatusUnassessed and a single figure
	// applies. Unavailable when Status is StatusUnassessed or no single
	// figure drives the classification (e.g. FinancialRecordQuality, which
	// combines several DataQuality booleans).
	Value Value `json:"value"`
	// Threshold is the Policy threshold Value was compared against, when
	// one was supplied and used. Zero when no threshold applied (either
	// Policy left it unset, or the dimension does not use a Policy
	// threshold at all).
	Threshold float64 `json:"threshold,omitempty"`
	// Explanation is a short, fixed-form human-readable statement of why
	// Status was reached (e.g. "EBITDA volatility of 34% exceeds the 25%
	// threshold"), built entirely from already-computed fields — never
	// free text a caller must parse for meaning distinct from Status
	// itself.
	Explanation string `json:"explanation"`
	// Source names which Input field(s) this classification was derived
	// from (e.g. "qoe", "metrics", "profile"), for traceability when
	// multiple optional inputs could each independently drive a dimension
	// (e.g. RecurringRevenue from RevenueQuality vs. Profile).
	Source string `json:"source,omitempty"`
}

// Severity distinguishes how urgently a Blocker or Risk should be
// addressed — this package's own three-level scale, kept separate from
// Status (an assessment of a dimension) since a single dimension's
// StatusConcerning classification can surface zero, one, or several
// individual Blockers/Risks with different severities (e.g. two separate
// concentration observations at different customer thresholds).
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// Blocker is a deterministic finding severe enough that it would very
// likely stop or significantly delay a sale process until addressed (e.g.
// negative maintainable earnings, no financial history at all). Every
// Blocker traces back to a specific Dimension's StatusConcerning
// classification or a fixed structural rule (see findings.go) — never a
// subjective judgment call.
type Blocker struct {
	Dimension DimensionCode `json:"dimension"`
	Severity  Severity      `json:"severity"`
	Message   string        `json:"message"`
}

// Risk is a deterministic finding worth a seller's attention before a sale
// process begins, but not severe enough alone to block one — typically
// traces back to a StatusWeak or StatusConcerning dimension that did not
// meet Blocker's stricter bar.
type Risk struct {
	Dimension DimensionCode `json:"dimension"`
	Severity  Severity      `json:"severity"`
	Message   string        `json:"message"`
}

// Strength is a deterministic positive finding — a StatusStrong dimension
// classification, surfaced as its own list so a caller building a
// seller-facing summary is not left inferring strengths from the absence
// of a Blocker/Risk.
type Strength struct {
	Dimension DimensionCode `json:"dimension"`
	Message   string        `json:"message"`
}

// MissingInformation is one StatusUnassessed dimension and which Input
// field(s) supplying it would enable an assessment — the mechanism
// implementing Prompt 31's requirement to report missing modules as
// reduced coverage, listed explicitly rather than left for a caller to
// infer from Result.Dimensions alone.
type MissingInformation struct {
	Dimension DimensionCode `json:"dimension"`
	// NeededInput names the Input field(s) (e.g. "QoE", "Concentration")
	// that would let this dimension be assessed.
	NeededInput string `json:"needed_input"`
	Message     string `json:"message"`
}

// OpportunityCode is a stable identifier for one kind of factual
// improvement action — see Opportunity's doc comment on why every entry
// here is framed as an action, never an evaluative judgment.
type OpportunityCode string

const (
	OpportunityCommissionReviewedFinancials  OpportunityCode = "COMMISSION_REVIEWED_FINANCIALS"
	OpportunityDocumentAdjustments           OpportunityCode = "DOCUMENT_ADJUSTMENTS"
	OpportunityDiversifyCustomerBase         OpportunityCode = "DIVERSIFY_CUSTOMER_BASE"
	OpportunityIncreaseRecurringRevenueShare OpportunityCode = "INCREASE_RECURRING_REVENUE_SHARE"
	OpportunityReduceOwnerDependence         OpportunityCode = "REDUCE_OWNER_DEPENDENCE"
	OpportunityStabilizeMargins              OpportunityCode = "STABILIZE_MARGINS"
	OpportunityStabilizeWorkingCapital       OpportunityCode = "STABILIZE_WORKING_CAPITAL"
	OpportunityReduceLeverage                OpportunityCode = "REDUCE_LEVERAGE"
	OpportunitySupplyMissingModuleInput      OpportunityCode = "SUPPLY_MISSING_MODULE_INPUT"
)

// Opportunity is one factual, actionable step that could improve a
// dimension's classification — always phrased as "do X," derived
// mechanically from a StatusWeak/StatusConcerning/StatusUnassessed
// dimension, never as a subjective recommendation like "this business
// would be more attractive if..." — see the package doc comment's
// guaranteed-sale disclaimer, which this framing is designed to stay
// consistent with (a factual action, not a promise about outcome).
type Opportunity struct {
	Code      OpportunityCode `json:"code"`
	Dimension DimensionCode   `json:"dimension"`
	Message   string          `json:"message"`
}

// Coverage summarizes how much of this package's total possible
// assessment surface was actually reached given the supplied Input — the
// structured counterpart to MissingInformation, letting a caller see at a
// glance (e.g. "7 of 11 dimensions assessed") without counting
// Result.Dimensions/MissingInformation itself.
type Coverage struct {
	// TotalDimensions is len(dimensionOrder) — always 11, fixed by this
	// package's version, echoed here so a caller need not import/count
	// DimensionCode constants to compute a percentage.
	TotalDimensions int `json:"total_dimensions"`
	// AssessedDimensions is the number of Result.Dimensions entries with
	// Status != StatusUnassessed.
	AssessedDimensions int `json:"assessed_dimensions"`
	// CoveragePercent is AssessedDimensions / TotalDimensions, as a
	// decimal.
	CoveragePercent float64 `json:"coverage_percent"`
}

// IssueSeverity distinguishes an input problem Calculate could not proceed
// past for some portion of the analysis (SeverityError) from one that is
// advisory only (SeverityWarning) — this package's own two-severity model,
// matching every sibling package's identical convention.
type IssueSeverity string

const (
	IssueSeverityError   IssueSeverity = "error"
	IssueSeverityWarning IssueSeverity = "warning"
)

// IssueCode is a stable identifier for one kind of Calculate-time input
// problem. This package defines its own separate taxonomy, consistent with
// every other package in this repository — see the repository README's
// error-taxonomy section.
type IssueCode string

const (
	// IssueNoInputSupplied means every optional Input field was left at its
	// zero value / unavailable, so no dimension could be assessed at all.
	IssueNoInputSupplied IssueCode = "NO_INPUT_SUPPLIED"
	// IssueQoEUnavailable means Input.QoE.Available was false, so
	// EarningsStability and NormalizationBurden (and MarginTrend's
	// QoE-sourced fallback) could not be assessed from it. Advisory only.
	IssueQoEUnavailable IssueCode = "QOE_UNAVAILABLE"
	// IssueWorkingCapitalUnavailable means Input.WorkingCapital.Available
	// was false. Advisory only.
	IssueWorkingCapitalUnavailable IssueCode = "WORKING_CAPITAL_UNAVAILABLE"
	// IssueConcentrationUnavailable means Input.Concentration.Available was
	// false. Advisory only.
	IssueConcentrationUnavailable IssueCode = "CONCENTRATION_UNAVAILABLE"
	// IssueRevenueQualityUnavailable means Input.RevenueQuality.Available
	// was false. Advisory only.
	IssueRevenueQualityUnavailable IssueCode = "REVENUE_QUALITY_UNAVAILABLE"
	// IssueConsensusUnavailable means Input.Consensus.Available was false.
	// Advisory only.
	IssueConsensusUnavailable IssueCode = "CONSENSUS_UNAVAILABLE"
	// IssueMetricsUnavailable means Input.Metrics.Snapshots was empty, so
	// MarginTrend and DebtLeverage could not be assessed from it (either
	// may still be assessed from QoE where noted). Advisory only.
	IssueMetricsUnavailable IssueCode = "METRICS_UNAVAILABLE"
	// IssueNoPolicyThresholds means Input.Policy was the zero value, so
	// every Policy-threshold-driven classification fell back to
	// StatusUnassessed for the threshold-comparison portion of its rule
	// (a dimension may still classify from a fixed, non-Policy rule — see
	// each classify* function). Advisory only.
	IssueNoPolicyThresholds IssueCode = "NO_POLICY_THRESHOLDS"
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

// Score is the optional deterministic 0-100 heuristic overall
// sale-readiness score — see score.go's computeOverallScore for the exact,
// fixed formula. Present only when Coverage.AssessedDimensions > 0 (see
// Result.OverallScore); a Result with zero assessed dimensions has nothing
// for a formula to weight.
type Score struct {
	// Version echoes ScoreVersion.
	Version string `json:"version"`
	// Value is the final clamped [0, 100] score.
	Value float64 `json:"value"`
	// Components lists every contribution to Value, in Result.Dimensions'
	// order (never Go map order), so a caller can reconstruct exactly how
	// Value was reached.
	Components []ScoreComponent `json:"components"`
	// Label is a short, fixed, human-readable characterization of Value's
	// range — purely descriptive; a caller should branch on Value's
	// numeric range directly if it needs to make a decision, never parse
	// Label.
	Label string `json:"label"`
	// Heuristic is always true — see the package doc comment's
	// guaranteed-sale disclaimer, which this explicit label reinforces at
	// the score level: Value is a heuristic composite, not a calibrated
	// probability of sale.
	Heuristic bool `json:"heuristic"`
}

// ScoreComponent is one assessed dimension's contribution to Score.Value.
type ScoreComponent struct {
	Dimension DimensionCode `json:"dimension"`
	Status    Status        `json:"status"`
	Points    float64       `json:"points"`
}

// Result is the output of Calculate: every dimension's classification,
// blockers, risks, strengths, missing information, improvement
// opportunities, coverage summary, optional overall score, and calculation
// trace/version.
type Result struct {
	// FormulaVersion identifies which version of this package's fixed rule
	// set produced this Result.
	FormulaVersion string `json:"formula_version"`
	// Available is false only if Calculate could not proceed at all — every
	// optional Input field was left at its zero value / unavailable, so no
	// dimension could be assessed. Every other field is then zero-value
	// (including Dimensions, which is empty rather than 11 StatusUnassessed
	// entries in this one degenerate case) except FormulaVersion and
	// Errors — mirroring acquisition.Result.Available/debt.Result.
	// Available's identical "every other field is zero-value" convention.
	// In every other case Calculate assesses whatever subset of Dimensions
	// the supplied Input supports and reports gaps via
	// Warnings/MissingInformation/Coverage.
	Available bool `json:"available"`

	// Dimensions is every one of the 11 fixed dimensions' classification,
	// in dimensionOrder (see DimensionCode) — always all 11 entries, one
	// per DimensionCode, regardless of how many were actually assessed,
	// whenever Available is true (an unassessed dimension still appears
	// with Status == StatusUnassessed, never omitted, so a caller can
	// always range over a complete, fixed-shape list). Empty when
	// Available is false — see Available's doc comment.
	Dimensions []Dimension `json:"dimensions"`

	// OverallScore is the optional composite score — nil when
	// Coverage.AssessedDimensions == 0 (Prompt 31: "overall readiness score
	// only if formula is explicit"; the explicit formula in score.go still
	// requires at least one assessed dimension to weight).
	OverallScore *Score `json:"overall_score,omitempty"`

	// Blockers is every deterministic finding severe enough to likely stop
	// or significantly delay a sale process, ordered by dimensionOrder
	// then by declaration order within a dimension.
	Blockers []Blocker `json:"blockers,omitempty"`
	// Risks is every deterministic finding worth attention but not severe
	// enough to be a Blocker, same ordering as Blockers.
	Risks []Risk `json:"risks,omitempty"`
	// Strengths is every StatusStrong dimension's positive finding, in
	// dimensionOrder.
	Strengths []Strength `json:"strengths,omitempty"`
	// MissingInformation is every StatusUnassessed dimension and what
	// Input field would enable it, in dimensionOrder.
	MissingInformation []MissingInformation `json:"missing_information,omitempty"`
	// Opportunities is every factual, actionable improvement step derived
	// from a StatusWeak/StatusConcerning/StatusUnassessed dimension, in
	// dimensionOrder.
	Opportunities []Opportunity `json:"opportunities,omitempty"`

	// Coverage summarizes how many of the 11 dimensions were actually
	// assessed.
	Coverage Coverage `json:"coverage"`

	// Policy echoes Input.Policy this Result was assessed against.
	Policy Policy `json:"policy"`

	// Warnings carries every Issue with IssueSeverityWarning.
	Warnings []Issue `json:"warnings,omitempty"`
	// Errors carries every Issue with IssueSeverityError.
	Errors []Issue `json:"errors,omitempty"`
}
