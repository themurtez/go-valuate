package profitability

// AttributionCoverage reports factual attribution coverage for one
// dimension/period — task sections 14-15. No opaque score: every field
// is a directly meaningful count or amount.
type AttributionCoverage struct {
	Dimension Dimension `json:"dimension"`
	Period    string    `json:"period"`

	TotalFactCount      int `json:"total_fact_count"`
	AttributedFactCount int `json:"attributed_fact_count"`

	TotalRevenueAmount      float64 `json:"total_revenue_amount"`
	AttributedRevenueAmount float64 `json:"attributed_revenue_amount"`

	TotalDirectCostAmount      float64 `json:"total_direct_cost_amount"`
	AttributedDirectCostAmount float64 `json:"attributed_direct_cost_amount"`

	TotalVariableCostAmount      float64 `json:"total_variable_cost_amount"`
	AttributedVariableCostAmount float64 `json:"attributed_variable_cost_amount"`

	TotalEconomicAmount      float64 `json:"total_economic_amount"`
	AttributedEconomicAmount float64 `json:"attributed_economic_amount"`

	// RevenueCoveragePercent/CostCoveragePercent are the attributed
	// fraction (0-1) of revenue/(direct+variable cost) amount,
	// Unavailable when the respective total is zero.
	RevenueCoveragePercent Value `json:"revenue_coverage_percent"`
	CostCoveragePercent    Value `json:"cost_coverage_percent"`
}

// buildAttributionCoverage derives AttributionCoverage from one
// dimension's attribution result for one period.
func buildAttributionCoverage(dim Dimension, period string, r *dimensionAttributionResult) AttributionCoverage {
	c := AttributionCoverage{
		Dimension:                    dim,
		Period:                       period,
		TotalFactCount:               len(r.totalFactIDsByPeriod[period]),
		AttributedFactCount:          len(r.attributedFactIDsByPeriod[period]),
		TotalRevenueAmount:           r.totalRevenueByPeriod[period],
		AttributedRevenueAmount:      r.attributedRevenueByPeriod[period],
		TotalDirectCostAmount:        r.totalDirectCostByPeriod[period],
		AttributedDirectCostAmount:   r.attributedDirectCostByPeriod[period],
		TotalVariableCostAmount:      r.totalVariableCostByPeriod[period],
		AttributedVariableCostAmount: r.attributedVariableCostByPeriod[period],
		TotalEconomicAmount:          r.totalAmountByPeriod[period],
		AttributedEconomicAmount:     r.attributedTotalByPeriod[period],
	}
	c.RevenueCoveragePercent = marginValue(c.AttributedRevenueAmount, c.TotalRevenueAmount)
	totalCost := c.TotalDirectCostAmount + c.TotalVariableCostAmount
	attributedCost := c.AttributedDirectCostAmount + c.AttributedVariableCostAmount
	c.CostCoveragePercent = marginValue(attributedCost, totalCost)
	return c
}

// DriverCoverage reports how many entities in a dimension/period have at
// least one DriverObservation — task section 55.
type DriverCoverage struct {
	Dimension           Dimension `json:"dimension"`
	Period              string    `json:"period"`
	EntitiesWithDrivers int       `json:"entities_with_drivers"`
	TotalEntities       int       `json:"total_entities"`
}

// AllocationCoverage reports how many pool/dimension/period slots were
// allocated vs left unallocated — task section 55.
type AllocationCoverage struct {
	Dimension        Dimension `json:"dimension"`
	Period           string    `json:"period"`
	TotalPools       int       `json:"total_pools"`
	AllocatedPools   int       `json:"allocated_pools"`
	UnallocatedPools int       `json:"unallocated_pools"`
}

// Coverage is this package's top-level factual coverage summary — task
// section 55. No opaque score.
type Coverage struct {
	PeriodsSupplied         bool `json:"periods_supplied"`
	EntitiesSupplied        bool `json:"entities_supplied"`
	FactsSupplied           bool `json:"facts_supplied"`
	SummariesSupplied       bool `json:"summaries_supplied"`
	DriversSupplied         bool `json:"drivers_supplied"`
	SharedCostPoolsSupplied bool `json:"shared_cost_pools_supplied"`
	AllocationRulesSupplied bool `json:"allocation_rules_supplied"`
	ControlsSupplied        bool `json:"controls_supplied"`

	AttributionByDimension []AttributionCoverage `json:"attribution_by_dimension,omitempty"`
	DriverCoverage         []DriverCoverage      `json:"driver_coverage,omitempty"`
	AllocationCoverage     []AllocationCoverage  `json:"allocation_coverage,omitempty"`

	ControlTotalCoveragePeriods int `json:"control_total_coverage_periods"`
}
