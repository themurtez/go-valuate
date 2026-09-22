// Package variance compares actual financial results against a
// caller-supplied baseline — a budget, a forecast, a prior-period actual, or
// any other reference series — line by line, by category, and in aggregate.
//
// This package is deliberately independent of financial.FinancialDataset.
// Unlike analytics/qoe/workingcapital/cashflow/revenuequality/anomalies
// (which read a normalized dataset directly), a budget or forecast is
// frequently produced and stored entirely outside any financial dataset
// this repository would ever see (a spreadsheet budget, an FP&A tool's
// forecast export, a board-approved plan). So this package defines its own
// minimal, fully portable LineObservation{AccountCode, Period, Actual,
// Baseline, BaselineType} tuple — mirroring analytics/concentration's
// identical "define a portable tuple rather than force a dataset shape"
// choice — and never requires a financial.FinancialDataset as input.
// AccountCode is still typed as financial.Code (not a bare string) because
// this package's favorable/unfavorable classification is genuinely
// taxonomy-dependent (see Policy and directionIndex.resolve): a revenue
// account increasing is favorable, an expense account increasing is
// unfavorable,
// and financial.Code plus financial.CodesByCategory is this repository's
// one canonical source for which accounts are which. A caller whose
// account is not part of the canonical taxonomy (e.g. a budget line with no
// natural financial.Code mapping) can still supply LineObservation.Category
// directly, or override direction per-code via Policy.DirectionOverrides.
//
// Every function here is pure: no I/O, no mutation of caller-owned input, no
// package-global mutable state. Calculate can be called concurrently and
// repeatedly against identical input and always returns byte-for-byte
// identical JSON — see determinism_test.go.
package variance

import "github.com/themurtez/go-valuate/financial"

// FormulaVersion identifies this package's fixed formula set: the
// absolute/percentage variance formulas, the favorable/unfavorable
// direction rules (including category defaults and override precedence),
// the materiality test, the contribution-to-total-variance formula, the
// category rollup, and the period-trend formula. Bump this whenever any of
// that changes in a way that could make a historical Result not reproduce
// identically under new code — see the repository README's
// versioning-strategy section, which this constant follows exactly
// (financial.TaxonomyVersion, metrics.FormulaVersion,
// workingcapital.FormulaVersion, cashflow.FormulaVersion,
// revenuequality.FormulaVersion, concentration.FormulaVersion,
// anomalies.FormulaVersion, etc.).
const FormulaVersion = "1.0.0"

// PeriodType mirrors metrics.PeriodType/workingcapital.PeriodType/
// revenuequality.PeriodType/concentration.PeriodType/anomalies.PeriodType
// (fiscal year, YTD, quarter, month), duplicated here rather than aliased
// so this package's own doc comments apply directly at the call site — the
// same choice every analytics sibling package already made for its own
// PeriodType.
type PeriodType string

const (
	PeriodTypeFiscalYear PeriodType = "fiscal_year"
	PeriodTypeYTD        PeriodType = "ytd"
	PeriodTypeQuarter    PeriodType = "quarter"
	PeriodTypeMonth      PeriodType = "month"
)

// PeriodInfo is caller-supplied metadata about a single financial.Period,
// used to determine chronological order for PeriodTrends. financial.Period
// is intentionally just a string with no guaranteed sort order; this
// package requires PeriodInfo rather than inferring order or granularity
// from the string, mirroring every analytics sibling package's identical
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

// VarianceValue represents a single variance figure that may or may not be
// calculable, mirroring metrics.MetricValue/revenuequality.RevenueValue/
// concentration.ConcentrationValue's identical availability convention:
// Available distinguishes "computed to be exactly $0/0%" from "cannot be
// computed because a required input is absent."
type VarianceValue struct {
	Available bool    `json:"available"`
	Value     float64 `json:"value"`
}

// Unavailable is the canonical zero-information VarianceValue.
func Unavailable() VarianceValue { return VarianceValue{} }

// AvailableValue reports a VarianceValue for a successfully computed
// figure.
func AvailableValue(v float64) VarianceValue { return VarianceValue{Available: true, Value: v} }

// BaselineType labels what a LineObservation.Baseline was compared against,
// for display/audit purposes only. This package computes variance
// identically regardless of BaselineType (the arithmetic of "actual minus
// baseline" does not change based on what the baseline represents) —
// mirroring concentration.Basis's identical "label only, same formula"
// role.
type BaselineType string

const (
	BaselineTypeBudget      BaselineType = "budget"
	BaselineTypeForecast    BaselineType = "forecast"
	BaselineTypePriorPeriod BaselineType = "prior_period"
	BaselineTypeCustom      BaselineType = "custom"
)

// LineObservation is one account's actual-vs-baseline comparison for one
// period — the portable input tuple this package requires for every
// computation (see the package doc comment). A caller assembles a slice of
// these from whatever source it has (a budget spreadsheet import, an FP&A
// forecast export, a prior-year actuals pull); this package has no opinion
// on where either figure came from.
type LineObservation struct {
	// AccountCode is the canonical financial.Code this line belongs to.
	// Required for taxonomy-driven favorable/unfavorable classification
	// (see Policy) and for CategorySummary rollups. A caller with an
	// account outside the canonical taxonomy still supplies its best-fit
	// financial.Code (or financial.CodeOpexOther/financial.CodeRevOther as
	// a fallback) and may override this line's direction individually via
	// Policy.DirectionOverrides.
	AccountCode financial.Code `json:"account_code"`
	// Label is a short human-readable name for this line, for display only
	// (e.g. a specific GL account name finer than AccountCode's taxonomy
	// bucket). Optional; falls back to AccountCode's financial.CodeMeta.Label
	// when empty.
	Label string `json:"label,omitempty"`
	// Period is the reporting period this comparison applies to. Must be
	// present in Input.PeriodMeta for this line to be included in
	// PeriodTrends; a line for a period absent from PeriodMeta is still
	// included in every other output (LineVariances, CategorySummaries,
	// TopFavorable/TopUnfavorable, MaterialExceptions, Bridge).
	Period financial.Period `json:"period"`
	// Actual is this account's actual amount for Period, in the caller's
	// native currency. This package's favorable/unfavorable direction rule
	// (see directionIndex.resolve and computeLineVariance) is a plain sign
	// comparison on Actual - Baseline; it assumes revenue/expense amounts
	// follow this
	// repository's usual non-negative sign convention (see
	// financial/metrics' package doc comment and
	// anomalies.RuleUnexpectedNegativeAmount). A caller whose Actual or
	// Baseline is negative for an otherwise-conventional revenue/expense
	// code still gets a correct arithmetic AbsoluteVariance, but
	// Favorability is not adjusted for the sign break — see
	// TestCalculate_NegativeValues for the exact resulting classification.
	Actual float64 `json:"actual"`
	// BaselineAvailable distinguishes "no baseline exists for this line"
	// (BaselineAvailable == false; e.g. a new account with no budget line
	// ever set for it) from "baseline is exactly $0" (BaselineAvailable ==
	// true, Baseline == 0; e.g. a budget that planned zero spend on this
	// account). A missing baseline makes every variance figure for this
	// line Unavailable rather than silently treating it as a $0 baseline —
	// see IssueMissingBaseline.
	BaselineAvailable bool `json:"baseline_available"`
	// Baseline is this account's budget/forecast/prior-period/custom amount
	// for Period. Meaningful only when BaselineAvailable is true.
	Baseline float64 `json:"baseline"`
	// BaselineType labels what Baseline represents (see BaselineType).
	// Required whenever BaselineAvailable is true; a true BaselineAvailable
	// with an empty BaselineType is recorded as IssueMissingBaselineType
	// but does not exclude the line from computation.
	BaselineType BaselineType `json:"baseline_type,omitempty"`
	// Category, when non-empty, is a caller-assigned grouping label (e.g. a
	// department, cost center, or product line) used only for
	// CategorySummary's optional by-category breakdown in addition to the
	// taxonomy-derived rollup. Plain string (mirroring
	// financial.Code/concentration.Observation.Category) so this package
	// never hard-codes a category taxonomy beyond financial.CodeCategory
	// itself.
	Category string `json:"category,omitempty"`
}

// DirectionOverride pins a specific financial.Code's favorable/unfavorable
// direction regardless of its taxonomy category — see Policy.
type DirectionOverride struct {
	AccountCode financial.Code `json:"account_code"`
	// IncreaseIsFavorable, when true, means an actual above baseline is
	// favorable for this code (like revenue); when false, an actual above
	// baseline is unfavorable (like an expense).
	IncreaseIsFavorable bool `json:"increase_is_favorable"`
}

// Input bundles everything Calculate needs.
type Input struct {
	// Lines is the full set of account/period/actual/baseline rows to
	// analyze. Required; Calculate returns a Result with Available == false
	// and an Errors entry if empty.
	Lines []LineObservation `json:"lines"`
	// PeriodMeta supplies chronological ordering for Lines' periods.
	// Required for PeriodTrends; if nil, or a period present in Lines has
	// no entry, PeriodTrends is left empty rather than the whole Result
	// failing — mirroring analytics/concentration.Input.PeriodMeta's
	// identical convention.
	PeriodMeta map[financial.Period]PeriodInfo

	// Policy configures materiality thresholds, top-N cutoffs, and
	// direction overrides. If the zero value, DefaultPolicy() is used.
	Policy Policy
}

// Policy configures Calculate's optional, caller-adjustable behavior.
// Unlike some analytics siblings' Policy/Thresholds split, this package
// keeps materiality and direction rules in one Policy: both change what a
// LineVariance's classification fields (Materiality, Favorability) report,
// not merely a downstream flag — there is no separate always-computed
// figure that materiality gates the way, say,
// concentration.Thresholds.HighHHI gates a Flag against an
// already-available HHI value.
type Policy struct {
	// MaterialAmountThreshold is the absolute-dollar floor at or above which
	// a line's variance is material regardless of
	// MaterialPercentOfBaseline. 0 means "not applied by amount alone" —
	// mirrors review.Policy.MaterialAmountThreshold/
	// review.IsMaterial's identical OR-of-two-legs, off-by-default design.
	MaterialAmountThreshold float64 `json:"material_amount_threshold"`
	// MaterialPercentOfBaseline is the fraction of |Baseline| (e.g. 0.1 =
	// 10%) at or above which a line's variance is material, when Baseline
	// is available and nonzero. 0 means "not applied by percent alone."
	// mirroring review.Policy.MaterialPercentOfRevenue's identical role.
	MaterialPercentOfBaseline float64 `json:"material_percent_of_baseline"`
	// TopN is the number of entries LineVariance.TopFavorable and
	// TopUnfavorable each report. If 0, DefaultPolicy's 5 is used.
	TopN int `json:"top_n"`
	// DirectionOverrides pins specific financial.Code values' favorable/
	// unfavorable direction, taking precedence over the taxonomy-category
	// default (see classifyDirection). A code listed more than once uses
	// its first entry — deterministic, caller-controlled precedence, never
	// Go map order.
	DirectionOverrides []DirectionOverride `json:"direction_overrides,omitempty"`
}

// DefaultPolicy returns this package's baseline materiality and reporting
// settings: materiality gating off (a caller must opt in, mirroring
// review.DefaultPolicy's identical off-by-default rationale — see
// review.IsMaterial), and a top-5 cutoff for TopFavorable/TopUnfavorable.
func DefaultPolicy() Policy {
	return Policy{
		MaterialAmountThreshold:   0,
		MaterialPercentOfBaseline: 0,
		TopN:                      5,
	}
}

// resolvePolicy returns p with TopN defaulted to DefaultPolicy's value when
// zero — the same zero-value-means-defaults rule every analytics sibling
// package's resolvePolicy/resolveThresholds uses. MaterialAmountThreshold/
// MaterialPercentOfBaseline are never defaulted away from their
// caller-supplied zero, mirroring review.IsMaterial's explicit
// off-by-default behavior for those two fields.
func resolvePolicy(p Policy) Policy {
	if p.TopN == 0 {
		p.TopN = DefaultPolicy().TopN
	}
	return p
}

// IssueSeverity distinguishes an input problem Calculate could not proceed
// past for some portion of the analysis (SeverityError) from one that is
// advisory only (SeverityWarning) — the same two-severity model every
// analytics sibling package uses.
type IssueSeverity string

const (
	SeverityError   IssueSeverity = "error"
	SeverityWarning IssueSeverity = "warning"
)

// IssueCode is a stable identifier for one kind of Calculate-time input
// problem. This package defines its own separate taxonomy, consistent with
// every other package in this repository, rather than reusing one of
// theirs — a variance-analysis input problem is a distinct problem domain.
type IssueCode string

const (
	// IssueNoLines means Input.Lines was empty; Calculate returns
	// Available == false.
	IssueNoLines IssueCode = "NO_LINES"
	// IssueMissingBaseline means at least one LineObservation had
	// BaselineAvailable == false; that line's variance figures are left
	// Unavailable but the line still appears in LineVariances with its
	// Actual figure intact. Advisory only.
	IssueMissingBaseline IssueCode = "MISSING_BASELINE"
	// IssueMissingBaselineType means at least one LineObservation had
	// BaselineAvailable == true but an empty BaselineType. Advisory only;
	// the line's variance is still computed.
	IssueMissingBaselineType IssueCode = "MISSING_BASELINE_TYPE"
	// IssueUnknownAccountCode means at least one LineObservation's
	// AccountCode was not a recognized financial.Code (per
	// financial.IsValidCode) and had no matching Policy.DirectionOverrides
	// entry, so that line's Favorability is left FavorabilityUnknown.
	// Advisory only; every other figure for the line is still computed.
	IssueUnknownAccountCode IssueCode = "UNKNOWN_ACCOUNT_CODE"
	// IssueNoPeriodMeta means Input.PeriodMeta was nil or empty, so
	// PeriodTrends could not be computed. Advisory only.
	IssueNoPeriodMeta IssueCode = "NO_PERIOD_META"
	// IssuePeriodMissingFromMeta means at least one period present in
	// Input.Lines has no PeriodMeta entry, so PeriodTrends is unavailable —
	// mirroring workingcapital.IssuePeriodMissingFromMeta's identical rule.
	IssuePeriodMissingFromMeta IssueCode = "PERIOD_MISSING_FROM_META"
)

// Issue is a single Calculate-time input finding, mirroring every analytics
// sibling package's identical Issue shape.
type Issue struct {
	Code     IssueCode     `json:"code"`
	Severity IssueSeverity `json:"severity"`
	Message  string        `json:"message"`
}

// HasErrors reports whether any Issue in issues has SeverityError.
//
// Intentionally duplicated from adjustments.HasErrors/qoe.HasErrors/
// workingcapital.HasErrors/cashflow.HasErrors/revenuequality.HasErrors/
// concentration.HasErrors/anomalies.HasErrors rather than shared — see
// adjustments.HasErrors's doc comment for the full rationale (each
// package's Issue is a distinct Go type with no common interface worth
// introducing for one boolean function).
func HasErrors(issues []Issue) bool {
	for _, i := range issues {
		if i.Severity == SeverityError {
			return true
		}
	}
	return false
}

// Favorability classifies whether a line's variance direction is good or
// bad news for the business, given its taxonomy category (or an explicit
// Policy.DirectionOverrides entry) — never inferred from the sign of
// Variance alone, since a negative variance on a revenue account
// (unfavorable) and a negative variance on an expense account (favorable,
// i.e. spending less than planned) mean opposite things despite having the
// same sign.
type Favorability string

const (
	// FavorabilityFavorable means this line's actual-vs-baseline movement
	// is good news (revenue/other-income above baseline, or an
	// expense/COGS below baseline).
	FavorabilityFavorable Favorability = "favorable"
	// FavorabilityUnfavorable means this line's actual-vs-baseline movement
	// is bad news (revenue/other-income below baseline, or an
	// expense/COGS above baseline).
	FavorabilityUnfavorable Favorability = "unfavorable"
	// FavorabilityNeutral means Variance is exactly $0 — neither favorable
	// nor unfavorable.
	FavorabilityNeutral Favorability = "neutral"
	// FavorabilityUnknown means this line's AccountCode has no recognized
	// taxonomy category and no matching Policy.DirectionOverrides entry, so
	// no favorable/unfavorable direction could be determined (see
	// IssueUnknownAccountCode). Variance itself is still computed and
	// reported.
	FavorabilityUnknown Favorability = "unknown"
)

// Materiality classifies whether a line's variance crosses Policy's
// materiality thresholds — a structured field, never left for a caller to
// re-derive from Variance and a threshold it would have to know to look up.
type Materiality string

const (
	// MaterialityMaterial means the variance met Policy's
	// MaterialAmountThreshold or MaterialPercentOfBaseline test (see
	// isMaterial).
	MaterialityMaterial Materiality = "material"
	// MaterialityImmaterial means the variance did not meet either test.
	MaterialityImmaterial Materiality = "immaterial"
	// MaterialityUnknown means Variance was Unavailable (see
	// IssueMissingBaseline), so materiality could not be evaluated.
	MaterialityUnknown Materiality = "unknown"
)

// LineVariance is one LineObservation's full variance computation.
type LineVariance struct {
	// AccountCode/Label/Period/Category echo the source LineObservation.
	AccountCode financial.Code   `json:"account_code"`
	Label       string           `json:"label,omitempty"`
	Period      financial.Period `json:"period"`
	Category    string           `json:"category,omitempty"`
	// TaxonomyCategory is AccountCode's financial.CodeCategory, when
	// AccountCode is recognized — the taxonomy-derived grouping
	// CategorySummaries rolls up by, distinct from the caller's own
	// optional Category label.
	TaxonomyCategory financial.CodeCategory `json:"taxonomy_category,omitempty"`

	// Actual echoes the source LineObservation.Actual.
	Actual float64 `json:"actual"`
	// BaselineAvailable/Baseline/BaselineType echo the source
	// LineObservation. BaselineAvailable == false means every field below
	// is Unavailable/zero-value.
	BaselineAvailable bool         `json:"baseline_available"`
	Baseline          float64      `json:"baseline"`
	BaselineType      BaselineType `json:"baseline_type,omitempty"`

	// AbsoluteVariance is Actual - Baseline. Available only if
	// BaselineAvailable.
	AbsoluteVariance VarianceValue `json:"absolute_variance"`
	// PercentVariance is (Actual - Baseline) / |Baseline|. Available only
	// if AbsoluteVariance is available and Baseline != 0 — a zero baseline
	// makes a percentage mathematically undefined ("infinite" growth from
	// nothing), so this package reports AbsoluteVariance alone rather than
	// an infinite or fabricated percentage, mirroring
	// metrics.GrowthPoint.GrowthFromZeroBase's identical zero-base
	// convention.
	PercentVariance VarianceValue `json:"percent_variance"`
	// VarianceFromZeroBase is true when BaselineAvailable and Baseline == 0
	// (PercentVariance is deliberately left Unavailable in that case).
	VarianceFromZeroBase bool `json:"variance_from_zero_base,omitempty"`

	// Favorability classifies AbsoluteVariance's direction given
	// AccountCode's taxonomy category or an explicit override (see
	// Favorability). FavorabilityUnknown when AccountCode's direction
	// cannot be determined or AbsoluteVariance is Unavailable;
	// FavorabilityNeutral when AbsoluteVariance is exactly 0; otherwise
	// FavorabilityFavorable/FavorabilityUnfavorable per the account's
	// direction.
	Favorability Favorability `json:"favorability"`
	// Materiality classifies AbsoluteVariance/PercentVariance against
	// Policy's thresholds (see Materiality, isMaterial).
	Materiality Materiality `json:"materiality"`
}

// CategorySummary aggregates every LineVariance sharing a single grouping
// key (either a financial.CodeCategory or a caller-supplied Category
// string — see Result.CategorySummaries/Result.CustomCategorySummaries).
type CategorySummary struct {
	// Category is the grouping key: a financial.CodeCategory value (as a
	// plain string) for Result.CategorySummaries, or the caller-supplied
	// LineObservation.Category for Result.CustomCategorySummaries.
	Category string `json:"category"`
	// LineCount is the number of LineVariance entries in this group.
	LineCount int `json:"line_count"`
	// TotalActual is the sum of every line's Actual in this group.
	TotalActual float64 `json:"total_actual"`
	// TotalBaseline is the sum of Baseline across lines in this group with
	// BaselineAvailable == true. Available only if at least one line in
	// the group has BaselineAvailable == true.
	TotalBaseline VarianceValue `json:"total_baseline"`
	// TotalVariance is TotalActual (restricted to lines with an available
	// baseline, so it stays consistent with TotalBaseline) minus
	// TotalBaseline. Available under the same rule as TotalBaseline.
	TotalVariance VarianceValue `json:"total_variance"`
	// TotalPercentVariance is TotalVariance.Value / |TotalBaseline.Value|.
	// Available only if TotalVariance is available and TotalBaseline.Value
	// != 0.
	TotalPercentVariance VarianceValue `json:"total_percent_variance"`
	// FavorableCount/UnfavorableCount/MaterialCount tally this group's
	// LineVariance.Favorability/Materiality classifications, so a caller
	// doesn't have to re-scan LineVariances itself.
	FavorableCount   int `json:"favorable_count"`
	UnfavorableCount int `json:"unfavorable_count"`
	MaterialCount    int `json:"material_count"`
}

// TrendDirection is a coarse, deterministic characterization of a series'
// overall direction, computed from its first vs. last available
// observation — never inferred from a Message string. Mirrors every
// analytics sibling package's identical TrendDirection.
type TrendDirection string

const (
	TrendIncreasing  TrendDirection = "increasing"
	TrendDeclining   TrendDirection = "declining"
	TrendStable      TrendDirection = "stable"
	TrendUnavailable TrendDirection = "unavailable"
)

// TrendFlatBandPercent is the |change| / |first value| band, as a decimal,
// within which a series is characterized TrendStable rather than
// TrendIncreasing/TrendDeclining. Fixed (not caller-configurable) because it
// is part of this package's versioned FormulaVersion — the same fixed ±5%
// band every analytics sibling package's TrendFlatBandPercent uses.
const TrendFlatBandPercent = 0.05

// PeriodTrend is one period's aggregate variance figures across every
// LineVariance for that period, plus a first-vs-last direction
// characterization of the resulting series across all periods present in
// Result.PeriodTrends (see TrendDirection).
type PeriodTrend struct {
	// Period is the period this aggregate applies to.
	Period financial.Period `json:"period"`
	// TotalActual/TotalBaseline/TotalVariance mirror
	// CategorySummary.TotalActual/TotalBaseline/TotalVariance, aggregated
	// across every LineVariance in Period instead of a category.
	TotalActual   float64       `json:"total_actual"`
	TotalBaseline VarianceValue `json:"total_baseline"`
	TotalVariance VarianceValue `json:"total_variance"`
	// TotalPercentVariance mirrors CategorySummary.TotalPercentVariance.
	TotalPercentVariance VarianceValue `json:"total_percent_variance"`
}

// TrendSummary characterizes TotalVariance's overall first-vs-last
// direction across Result.PeriodTrends, in chronological order.
type TrendSummary struct {
	Direction TrendDirection `json:"direction"`
	// FirstPeriod/LastPeriod are the chronologically first/last periods
	// with an available TotalVariance, when Direction is not
	// TrendUnavailable.
	FirstPeriod financial.Period `json:"first_period,omitempty"`
	LastPeriod  financial.Period `json:"last_period,omitempty"`
	// FirstValue/LastValue are the corresponding TotalVariance values.
	FirstValue VarianceValue `json:"first_value"`
	LastValue  VarianceValue `json:"last_value"`
}

// MaterialException is a LineVariance flagged MaterialityMaterial, echoed
// here (rather than requiring a caller to filter LineVariances itself) for
// direct display in an exceptions report.
type MaterialException struct {
	LineVariance
	// ContributionToTotalVariance is this line's AbsoluteVariance.Value as
	// a fraction of Bridge.TotalVariance.Value (unsigned lines summed in
	// the denominator's absolute terms — see Bridge.TotalAbsoluteVariance).
	// Available only if AbsoluteVariance and Bridge.TotalAbsoluteVariance
	// are both available and Bridge.TotalAbsoluteVariance.Value != 0.
	ContributionToTotalVariance VarianceValue `json:"contribution_to_total_variance"`
}

// Bridge is the total actual-vs-baseline reconciliation across every
// LineVariance with an available baseline — the single aggregate figure a
// "budget vs. actual" report leads with.
type Bridge struct {
	// LineCount is the number of LineVariance entries with
	// BaselineAvailable == true that contributed to this Bridge.
	LineCount int `json:"line_count"`
	// TotalActual is the sum of Actual across every contributing line.
	TotalActual float64 `json:"total_actual"`
	// TotalBaseline is the sum of Baseline across every contributing line.
	TotalBaseline float64 `json:"total_baseline"`
	// TotalVariance is TotalActual - TotalBaseline.
	TotalVariance float64 `json:"total_variance"`
	// TotalPercentVariance is TotalVariance / |TotalBaseline|. Available
	// only if TotalBaseline != 0.
	TotalPercentVariance VarianceValue `json:"total_percent_variance"`
	// TotalAbsoluteVariance is the sum of |AbsoluteVariance.Value| across
	// every contributing line — the denominator MaterialException.
	// ContributionToTotalVariance divides into, so a handful of offsetting
	// large favorable/unfavorable lines don't net down to a misleadingly
	// small contribution base.
	TotalAbsoluteVariance float64 `json:"total_absolute_variance"`
	// FavorableVariance/UnfavorableVariance are the summed
	// AbsoluteVariance.Value across every contributing line classified
	// FavorabilityFavorable/FavorabilityUnfavorable respectively (each a
	// signed contribution to TotalVariance, so
	// FavorableVariance + UnfavorableVariance == TotalVariance whenever
	// every line has a known Favorability).
	FavorableVariance   float64 `json:"favorable_variance"`
	UnfavorableVariance float64 `json:"unfavorable_variance"`
}

// Result is the output of Calculate: every line's variance, category and
// custom-category rollups, top favorable/unfavorable lines, material
// exceptions, period trends, the total bridge, and formula version.
type Result struct {
	// FormulaVersion identifies which version of this package's fixed
	// formula set produced this Result.
	FormulaVersion string `json:"formula_version"`
	// Available is false only if Calculate could not proceed at all (see
	// IssueNoLines) — every other field is then zero-value.
	Available bool `json:"available"`

	// LineVariances is one LineVariance per Input.Lines entry, ordered by
	// Period ascending (lexically, unless Input.PeriodMeta establishes
	// chronological order — see orderedPeriods), then AccountCode
	// ascending, then by original input order for ties (e.g. duplicate
	// AccountCode/Period pairs).
	LineVariances []LineVariance `json:"line_variances,omitempty"`

	// CategorySummaries is one CategorySummary per distinct
	// financial.CodeCategory present among LineVariances (via a
	// recognized AccountCode), sorted by Category ascending.
	CategorySummaries []CategorySummary `json:"category_summaries,omitempty"`
	// CustomCategorySummaries is one CategorySummary per distinct non-empty
	// LineObservation.Category value present in Input.Lines, sorted by
	// Category ascending. Empty if no line supplied a Category.
	CustomCategorySummaries []CategorySummary `json:"custom_category_summaries,omitempty"`

	// TopFavorable is the Policy.TopN LineVariances with
	// Favorability == FavorabilityFavorable, sorted by AbsoluteVariance.Value
	// descending (largest favorable dollar impact first), ties broken by
	// AccountCode then Period ascending.
	TopFavorable []LineVariance `json:"top_favorable,omitempty"`
	// TopUnfavorable is the Policy.TopN LineVariances with
	// Favorability == FavorabilityUnfavorable, sorted by
	// AbsoluteVariance.Value ascending (most negative dollar impact first —
	// i.e. by magnitude of the unfavorable variance), ties broken by
	// AccountCode then Period ascending.
	TopUnfavorable []LineVariance `json:"top_unfavorable,omitempty"`

	// MaterialExceptions is every LineVariance classified
	// MaterialityMaterial, in the same order as LineVariances.
	MaterialExceptions []MaterialException `json:"material_exceptions,omitempty"`

	// PeriodTrends is one PeriodTrend per period present in Input.Lines,
	// ordered chronologically when Input.PeriodMeta was supplied and
	// covers every period; otherwise in lexical Period order.
	PeriodTrends []PeriodTrend `json:"period_trends,omitempty"`
	// TrendSummary characterizes TotalVariance's overall first-vs-last
	// direction across PeriodTrends. TrendUnavailable if chronological
	// order was unavailable (see IssueNoPeriodMeta/
	// IssuePeriodMissingFromMeta).
	TrendSummary TrendSummary `json:"trend_summary"`

	// Bridge is the total actual-vs-baseline reconciliation across every
	// line with an available baseline.
	Bridge Bridge `json:"bridge"`

	// Policy echoes the resolved Input.Policy (after DefaultPolicy
	// substitution) this Result was computed under.
	Policy Policy `json:"policy"`

	// Warnings carries every Issue with SeverityWarning.
	Warnings []Issue `json:"warnings,omitempty"`
	// Errors carries every Issue with SeverityError.
	Errors []Issue `json:"errors,omitempty"`
}
