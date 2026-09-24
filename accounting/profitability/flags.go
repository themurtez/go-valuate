package profitability

// FlagSeverity mirrors labor.FlagSeverity/cashforecast.FlagSeverity's
// role: a structured, matchable urgency signal, never inferred from
// Message text.
type FlagSeverity string

const (
	FlagSeverityInfo     FlagSeverity = "info"
	FlagSeverityWarning  FlagSeverity = "warning"
	FlagSeverityCritical FlagSeverity = "critical"
)

// FlagCode is a stable identifier for one kind of deterministic
// profitability-review signal — as opposed to an Issue, which is an
// input/validation problem (see issues.go). This package never derives a
// hidden composite/completeness score; every flag is a simple, documented
// threshold comparison the caller can fully see and override via Policy.
// Every generated Flag.Message uses neutral, factual, non-prescriptive
// language — see safety_test.go and task sections 63-64.
type FlagCode string

const (
	FlagNegativeGrossProfit     FlagCode = "NEGATIVE_GROSS_PROFIT"
	FlagNegativeContribution    FlagCode = "NEGATIVE_CONTRIBUTION"
	FlagNegativeAllocatedProfit FlagCode = "NEGATIVE_ALLOCATED_PROFIT"

	FlagMarginCompression                    FlagCode = "MARGIN_COMPRESSION"
	FlagRevenueGrowthWithContributionDecline FlagCode = "REVENUE_GROWTH_WITH_CONTRIBUTION_DECLINE"
	FlagRevenueGrowthWithMarginCompression   FlagCode = "REVENUE_GROWTH_WITH_MARGIN_COMPRESSION"

	FlagHighReturnRate   FlagCode = "HIGH_RETURN_RATE"
	FlagHighDiscountRate FlagCode = "HIGH_DISCOUNT_RATE"

	FlagDirectLaborShareIncreasing    FlagCode = "DIRECT_LABOR_SHARE_INCREASING"
	FlagDirectMaterialShareIncreasing FlagCode = "DIRECT_MATERIAL_SHARE_INCREASING"
	FlagFulfillmentShareIncreasing    FlagCode = "FULFILLMENT_SHARE_INCREASING"
	FlagVariableCostShareIncreasing   FlagCode = "VARIABLE_COST_SHARE_INCREASING"

	FlagLowRevenueAttributionCoverage FlagCode = "LOW_REVENUE_ATTRIBUTION_COVERAGE"
	FlagLowCostAttributionCoverage    FlagCode = "LOW_COST_ATTRIBUTION_COVERAGE"
	FlagMaterialUnattributedRevenue   FlagCode = "MATERIAL_UNATTRIBUTED_REVENUE"
	FlagMaterialUnattributedCost      FlagCode = "MATERIAL_UNATTRIBUTED_COST"

	FlagSharedCostPoolUnallocated    FlagCode = "SHARED_COST_POOL_UNALLOCATED"
	FlagPartiallyAllocatedSharedCost FlagCode = "PARTIALLY_ALLOCATED_SHARED_COST"

	FlagControlTotalMismatch FlagCode = "CONTROL_TOTAL_MISMATCH"
)

// flagCodeOrder fixes FlagCode declaration order for deterministic Flags
// sorting — task section 71 "flags severity/code/dimension/entity."
var flagCodeOrder = []FlagCode{
	FlagNegativeGrossProfit,
	FlagNegativeContribution,
	FlagNegativeAllocatedProfit,
	FlagMarginCompression,
	FlagRevenueGrowthWithContributionDecline,
	FlagRevenueGrowthWithMarginCompression,
	FlagHighReturnRate,
	FlagHighDiscountRate,
	FlagDirectLaborShareIncreasing,
	FlagDirectMaterialShareIncreasing,
	FlagFulfillmentShareIncreasing,
	FlagVariableCostShareIncreasing,
	FlagLowRevenueAttributionCoverage,
	FlagLowCostAttributionCoverage,
	FlagMaterialUnattributedRevenue,
	FlagMaterialUnattributedCost,
	FlagSharedCostPoolUnallocated,
	FlagPartiallyAllocatedSharedCost,
	FlagControlTotalMismatch,
}

func flagRank(c FlagCode) int {
	for i, fc := range flagCodeOrder {
		if fc == c {
			return i
		}
	}
	return len(flagCodeOrder)
}

func severityRank(s FlagSeverity) int {
	switch s {
	case FlagSeverityCritical:
		return 0
	case FlagSeverityWarning:
		return 1
	case FlagSeverityInfo:
		return 2
	default:
		return 3
	}
}

// Flag is a single structured, deterministic review signal.
type Flag struct {
	Code     FlagCode     `json:"code"`
	Severity FlagSeverity `json:"severity"`
	Message  string       `json:"message"`

	Dimension Dimension `json:"dimension,omitempty"`
	EntityID  string    `json:"entity_id,omitempty"`
	Period    string    `json:"period,omitempty"`
	PoolID    string    `json:"pool_id,omitempty"`
}
