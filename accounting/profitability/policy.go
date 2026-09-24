package profitability

// Policy is every caller-configurable threshold this package uses,
// following labor/closequality/journaldiagnostics's single-monolithic-
// struct convention. Nothing that materially changes a Flag or a
// reconciliation/coverage determination is a hidden code constant. This
// package never invents target margins — task section 59.
type Policy struct {
	// AttributionTolerance is how far a Fact's per-dimension attribution
	// share sum may exceed 1 before IssueAttributionExceeds100Percent is
	// raised, and how close to 0/1 a share must be to still be treated as
	// exactly 0/1 for coverage purposes. Default shareTolerance
	// (0.0001).
	AttributionTolerance float64 `json:"attribution_tolerance,omitempty"`
	// AllocationTolerance is the tolerance for "Allocated + Unallocated
	// == Pool total" reconciliation. Default amountTolerance (0.005).
	AllocationTolerance float64 `json:"allocation_tolerance,omitempty"`
	// ControlTolerance is the tolerance for business-vs-control-total
	// component reconciliation. Default amountTolerance (0.005).
	ControlTolerance float64 `json:"control_tolerance,omitempty"`

	// MinRevenueAttributionCoverage/MinCostAttributionCoverage are the
	// fraction (0-1) of revenue/cost that must be attributed (by amount)
	// for a dimension/period before FlagLowRevenueAttributionCoverage/
	// FlagLowCostAttributionCoverage trigger. Default 0 (never flagged)
	// unless the caller sets a threshold; StrictAttributionCoverage
	// controls whether a gap here also blocks computation. Default 0.
	MinRevenueAttributionCoverage float64 `json:"min_revenue_attribution_coverage,omitempty"`
	MinCostAttributionCoverage    float64 `json:"min_cost_attribution_coverage,omitempty"`
	// StrictAttributionCoverage, when true, causes a dimension/period
	// falling short of MinRevenueAttributionCoverage/
	// MinCostAttributionCoverage to report ViewStatusAvailableWithGaps
	// instead of ViewStatusAvailable — task section 15 "calculate despite
	// gaps unless caller enables strict mode." Default false: gaps never
	// block calculation, only inform Coverage/Flags.
	StrictAttributionCoverage bool `json:"strict_attribution_coverage,omitempty"`

	// MinimumRevenueForMarginRanking excludes an entity from
	// margin-based rankings (but not amount-based rankings) when its
	// NetRevenue is below this absolute amount — task section 45. Default
	// 0 (no exclusion).
	MinimumRevenueForMarginRanking float64 `json:"minimum_revenue_for_margin_ranking,omitempty"`

	// NegativeContributionMateriality is the materiality policy applied
	// to negative-contribution/negative-allocated-profit magnitude before
	// FlagNegativeContribution/FlagNegativeAllocatedProfit trigger — task
	// sections 46, 58.
	NegativeContributionMateriality MaterialityPolicy `json:"negative_contribution_materiality"`
	// UnattributedMateriality is the materiality policy applied to
	// unattributed revenue/cost amounts before
	// FlagMaterialUnattributedRevenue/FlagMaterialUnattributedCost
	// trigger — task section 58.
	UnattributedMateriality MaterialityPolicy `json:"unattributed_materiality"`
	// ControlMateriality is the materiality policy applied to a control-
	// total difference before FlagControlTotalMismatch triggers.
	ControlMateriality MaterialityPolicy `json:"control_materiality"`

	// GrossMarginCompressionPoints/ContributionMarginCompressionPoints
	// trigger FlagMarginCompression when adjacent-period GrossMargin/
	// ContributionMargin falls by more than this many percentage points
	// (expressed as a fraction, e.g. 0.05 == 5 points). Default 0.05
	// each.
	GrossMarginCompressionPoints        float64 `json:"gross_margin_compression_points,omitempty"`
	ContributionMarginCompressionPoints float64 `json:"contribution_margin_compression_points,omitempty"`

	// HighReturnRate/HighDiscountRate trigger FlagHighReturnRate/
	// FlagHighDiscountRate when ReturnRate/DiscountRate exceeds this
	// fraction. Default 0.10 (10%) each.
	HighReturnRate   float64 `json:"high_return_rate,omitempty"`
	HighDiscountRate float64 `json:"high_discount_rate,omitempty"`

	// CostShareIncreasePoints triggers
	// FlagDirectLaborShareIncreasing/FlagDirectMaterialShareIncreasing/
	// FlagFulfillmentShareIncreasing/FlagVariableCostShareIncreasing when
	// the respective percent-of-revenue share increases by more than this
	// many percentage points period-over-period. Default 0.03 (3
	// points).
	CostShareIncreasePoints float64 `json:"cost_share_increase_points,omitempty"`

	// TopN is how many entities RankingList's Top rankings include.
	// Default 10.
	TopN int `json:"top_n,omitempty"`

	// MinimumTrendPeriods is the minimum number of chronological periods
	// required before trend/flag calculations requiring history are
	// computed. Default 2.
	MinimumTrendPeriods int `json:"minimum_trend_periods,omitempty"`

	// DimensionApplicability optionally marks a Dimension as
	// ApplicabilityNotApplicable — task section 56. A Dimension absent
	// from this map defaults to ApplicabilityEnabled.
	DimensionApplicability map[Dimension]Applicability `json:"dimension_applicability,omitempty"`

	// PrimaryDriverKey optionally selects, per Dimension, which
	// DriverObservation.DriverKey is used for PerUnitMetrics — task
	// section 19. A Dimension absent from this map has no per-unit
	// metrics computed.
	PrimaryDriverKey map[Dimension]string `json:"primary_driver_key,omitempty"`

	// Currency is the single reporting currency for this Calculate call —
	// task section 60. Required for any Fact/summary row with a
	// non-matching, non-empty Currency to be excluded
	// (IssueMixedCurrency).
	Currency string `json:"currency,omitempty"`
}

// MaterialityPolicy pairs an absolute-amount and percent-of-revenue
// threshold — task section 58. A value is material if it meets or
// exceeds AbsoluteAmount OR (PercentOfRevenue > 0 AND it meets or
// exceeds PercentOfRevenue * reference revenue). If BOTH thresholds are
// left at their zero value (the caller configured no materiality
// threshold at all for this policy field), every nonzero amount is
// treated as material — mirroring review.IsMaterial's identical
// "unconfigured means never silently suppressed" convention, so a Flag
// gated on materiality is never dead by default the way an all-zero
// MaterialityPolicy would otherwise make it. Deliberately not called
// "audit materiality" anywhere in this package.
type MaterialityPolicy struct {
	AbsoluteAmount   float64 `json:"absolute_amount,omitempty"`
	PercentOfRevenue float64 `json:"percent_of_revenue,omitempty"`
}

func (m MaterialityPolicy) isMaterial(amount, referenceRevenue float64) bool {
	mag := absFloat(amount)
	if m.AbsoluteAmount > 0 && mag >= m.AbsoluteAmount {
		return true
	}
	if m.PercentOfRevenue > 0 && referenceRevenue != 0 && mag >= m.PercentOfRevenue*absFloat(referenceRevenue) {
		return true
	}
	if m.AbsoluteAmount <= 0 && m.PercentOfRevenue <= 0 {
		return mag != 0
	}
	return false
}

const (
	defaultGrossMarginCompressionPoints        = 0.05
	defaultContributionMarginCompressionPoints = 0.05
	defaultHighReturnRate                      = 0.10
	defaultHighDiscountRate                    = 0.10
	defaultCostShareIncreasePoints             = 0.03
	defaultTopN                                = 10
	defaultMinimumTrendPeriods                 = 2
)

// DefaultPolicy returns this package's baseline, broadly-safe Policy.
// Every threshold here is a review-attention trigger point, not a
// pricing/margin target — task section 59 "do not invent target
// margins."
func DefaultPolicy() Policy {
	return Policy{
		AttributionTolerance:                shareTolerance,
		AllocationTolerance:                 amountTolerance,
		ControlTolerance:                    amountTolerance,
		GrossMarginCompressionPoints:        defaultGrossMarginCompressionPoints,
		ContributionMarginCompressionPoints: defaultContributionMarginCompressionPoints,
		HighReturnRate:                      defaultHighReturnRate,
		HighDiscountRate:                    defaultHighDiscountRate,
		CostShareIncreasePoints:             defaultCostShareIncreasePoints,
		TopN:                                defaultTopN,
		MinimumTrendPeriods:                 defaultMinimumTrendPeriods,
	}
}

// resolvePolicy merges p over DefaultPolicy field by field: a zero-valued
// field takes the default. Fields with a legitimate zero default
// (MinRevenueAttributionCoverage, MinCostAttributionCoverage,
// MinimumRevenueForMarginRanking, the materiality policies, the
// Strict*/Disable* bools, DimensionApplicability, PrimaryDriverKey,
// Currency) are never defaulted away from zero.
func resolvePolicy(p Policy) Policy {
	d := DefaultPolicy()
	if p.AttributionTolerance == 0 {
		p.AttributionTolerance = d.AttributionTolerance
	}
	if p.AllocationTolerance == 0 {
		p.AllocationTolerance = d.AllocationTolerance
	}
	if p.ControlTolerance == 0 {
		p.ControlTolerance = d.ControlTolerance
	}
	if p.GrossMarginCompressionPoints == 0 {
		p.GrossMarginCompressionPoints = d.GrossMarginCompressionPoints
	}
	if p.ContributionMarginCompressionPoints == 0 {
		p.ContributionMarginCompressionPoints = d.ContributionMarginCompressionPoints
	}
	if p.HighReturnRate == 0 {
		p.HighReturnRate = d.HighReturnRate
	}
	if p.HighDiscountRate == 0 {
		p.HighDiscountRate = d.HighDiscountRate
	}
	if p.CostShareIncreasePoints == 0 {
		p.CostShareIncreasePoints = d.CostShareIncreasePoints
	}
	if p.TopN == 0 {
		p.TopN = d.TopN
	}
	if p.MinimumTrendPeriods == 0 {
		p.MinimumTrendPeriods = d.MinimumTrendPeriods
	}
	return p
}

// validatePolicy checks Policy for structurally invalid configuration:
// non-finite thresholds and a negative TopN/MinimumTrendPeriods.
func validatePolicy(p Policy) []Issue {
	var issues []Issue
	checkFinite := func(v float64, label string) {
		if isNonFinite(v) {
			issues = append(issues, Issue{Code: IssueInvalidPolicy, Severity: SeverityError,
				Message: "policy threshold " + label + " is non-finite"})
		}
	}
	checkFinite(p.AttributionTolerance, "attribution_tolerance")
	checkFinite(p.AllocationTolerance, "allocation_tolerance")
	checkFinite(p.ControlTolerance, "control_tolerance")
	checkFinite(p.MinRevenueAttributionCoverage, "min_revenue_attribution_coverage")
	checkFinite(p.MinCostAttributionCoverage, "min_cost_attribution_coverage")
	checkFinite(p.MinimumRevenueForMarginRanking, "minimum_revenue_for_margin_ranking")
	checkFinite(p.NegativeContributionMateriality.AbsoluteAmount, "negative_contribution_materiality.absolute_amount")
	checkFinite(p.NegativeContributionMateriality.PercentOfRevenue, "negative_contribution_materiality.percent_of_revenue")
	checkFinite(p.UnattributedMateriality.AbsoluteAmount, "unattributed_materiality.absolute_amount")
	checkFinite(p.UnattributedMateriality.PercentOfRevenue, "unattributed_materiality.percent_of_revenue")
	checkFinite(p.ControlMateriality.AbsoluteAmount, "control_materiality.absolute_amount")
	checkFinite(p.ControlMateriality.PercentOfRevenue, "control_materiality.percent_of_revenue")
	checkFinite(p.GrossMarginCompressionPoints, "gross_margin_compression_points")
	checkFinite(p.ContributionMarginCompressionPoints, "contribution_margin_compression_points")
	checkFinite(p.HighReturnRate, "high_return_rate")
	checkFinite(p.HighDiscountRate, "high_discount_rate")
	checkFinite(p.CostShareIncreasePoints, "cost_share_increase_points")
	if p.TopN < 0 {
		issues = append(issues, Issue{Code: IssueInvalidPolicy, Severity: SeverityError,
			Message: "policy top_n must not be negative"})
	}
	if p.MinimumTrendPeriods < 0 {
		issues = append(issues, Issue{Code: IssueInvalidPolicy, Severity: SeverityError,
			Message: "policy minimum_trend_periods must not be negative"})
	}
	for dim, a := range p.DimensionApplicability {
		if !isRecognizedDimension(dim) || !isRecognizedApplicability(a) {
			issues = append(issues, Issue{Code: IssueInvalidPolicy, Severity: SeverityError,
				Message: "policy dimension_applicability has an unrecognized dimension or applicability value"})
		}
	}
	return issues
}

// applicabilityFor returns the resolved Applicability for dim, defaulting
// to ApplicabilityEnabled when unset.
func applicabilityFor(p Policy, dim Dimension) Applicability {
	if a, ok := p.DimensionApplicability[dim]; ok && isRecognizedApplicability(a) {
		return a
	}
	return ApplicabilityEnabled
}
