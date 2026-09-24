package profitability

// UnattributedAmounts reports the portion of revenue/direct cost/variable
// cost for one dimension/period that no Attribution claimed — task
// section 13. Never dropped; always surfaced with component detail.
type UnattributedAmounts struct {
	UnattributedRevenue      float64               `json:"unattributed_revenue"`
	UnattributedDirectCost   float64               `json:"unattributed_direct_cost"`
	UnattributedVariableCost float64               `json:"unattributed_variable_cost"`
	ByComponent              map[Component]float64 `json:"by_component,omitempty"`
}

func buildUnattributedAmounts(byComponent map[Component]float64) UnattributedAmounts {
	u := UnattributedAmounts{ByComponent: byComponent}
	for c, amt := range byComponent {
		switch {
		case isRevenueComponent(c):
			// Revenue-reducing components (RETURN/DISCOUNT/OTHER_REVENUE_REDUCTION)
			// still represent an unattributed revenue-side fact; their
			// unattributed amount nets into UnattributedRevenue exactly as
			// the fact's own sign convention (all Fact.Amount are
			// non-negative magnitudes, so this sums magnitudes of
			// reductions together with additions, mirroring how NetRevenue
			// itself combines them in buildRevenueBridge).
			if c == ComponentReturn || c == ComponentDiscount || c == ComponentOtherRevenueReduction {
				u.UnattributedRevenue -= amt
			} else {
				u.UnattributedRevenue += amt
			}
		case isDirectCostComponent(c):
			u.UnattributedDirectCost += amt
		case isVariableCostComponent(c):
			u.UnattributedVariableCost += amt
		}
	}
	return u
}

// RevenueQualityMargins bundles the three profitability-bridge margins —
// task section 9.
type Margins struct {
	GrossMargin        Value `json:"gross_margin"`
	ContributionMargin Value `json:"contribution_margin"`
	AllocatedMargin    Value `json:"allocated_margin"`
}

// EntityPeriodResult is one entity's full profitability computation for
// one period — task section 29.
type EntityPeriodResult struct {
	Dimension Dimension `json:"dimension"`
	EntityID  string    `json:"entity_id"`
	Period    string    `json:"period"`

	RevenueBridge      RevenueBridge      `json:"revenue_bridge"`
	DirectCostBridge   DirectCostBridge   `json:"direct_cost_bridge"`
	VariableCostBridge VariableCostBridge `json:"variable_cost_bridge"`

	GrossProfit        float64 `json:"gross_profit"`
	ContributionProfit float64 `json:"contribution_profit"`

	AllocatedSharedCosts float64 `json:"allocated_shared_costs"`
	AllocatedProfit      float64 `json:"allocated_profit"`

	Margins Margins `json:"margins"`

	ActivityDrivers map[string]float64 `json:"activity_drivers,omitempty"`
	PerUnitMetrics  PerUnitMetrics     `json:"per_unit_metrics"`

	AllocationTrace []AllocationTraceEntry `json:"allocation_trace,omitempty"`

	FactIDs []string `json:"fact_ids,omitempty"`
}

// buildEntityPeriodResult computes one entity/period's full bridge from
// its already-resolved component amounts — shared by both the detailed
// Fact path and the summary-input path (task section 17).
func buildEntityPeriodResult(dim Dimension, entityID, period string, amounts map[Component]float64, allocatedSharedCosts float64, allocationTrace []AllocationTraceEntry, factIDs []string) EntityPeriodResult {
	r := EntityPeriodResult{Dimension: dim, EntityID: entityID, Period: period}
	r.RevenueBridge = buildRevenueBridge(amounts)
	r.DirectCostBridge = buildDirectCostBridge(amounts)
	r.VariableCostBridge = buildVariableCostBridge(amounts)

	r.GrossProfit = r.RevenueBridge.NetRevenue - r.DirectCostBridge.Total
	r.ContributionProfit = r.GrossProfit - r.VariableCostBridge.Total
	r.AllocatedSharedCosts = allocatedSharedCosts
	r.AllocatedProfit = r.ContributionProfit - allocatedSharedCosts

	r.Margins = Margins{
		GrossMargin:        marginValue(r.GrossProfit, r.RevenueBridge.NetRevenue),
		ContributionMargin: marginValue(r.ContributionProfit, r.RevenueBridge.NetRevenue),
		AllocatedMargin:    marginValue(r.AllocatedProfit, r.RevenueBridge.NetRevenue),
	}

	r.AllocationTrace = allocationTrace
	r.FactIDs = factIDs
	return r
}

// AllPeriodResult is one entity's all-period totals — task section 30.
// Margins here are computed as total profit / total revenue, NEVER as an
// arithmetic average of period margins — see
// TestInvariant_AllPeriodMarginIsNotAverage.
type AllPeriodResult struct {
	Dimension Dimension `json:"dimension"`
	EntityID  string    `json:"entity_id"`

	RevenueBridge      RevenueBridge      `json:"revenue_bridge"`
	DirectCostBridge   DirectCostBridge   `json:"direct_cost_bridge"`
	VariableCostBridge VariableCostBridge `json:"variable_cost_bridge"`

	GrossProfit          float64 `json:"gross_profit"`
	ContributionProfit   float64 `json:"contribution_profit"`
	AllocatedSharedCosts float64 `json:"allocated_shared_costs"`
	AllocatedProfit      float64 `json:"allocated_profit"`

	Margins Margins `json:"margins"`

	PeriodCount int `json:"period_count"`
}

// buildAllPeriodResult sums an entity's per-period EntityPeriodResults
// into one all-period total, with margins computed from the summed
// totals (never averaged) — task section 30.
func buildAllPeriodResult(dim Dimension, entityID string, periods []EntityPeriodResult) AllPeriodResult {
	a := AllPeriodResult{Dimension: dim, EntityID: entityID, PeriodCount: len(periods)}
	for _, p := range periods {
		a.RevenueBridge.GrossRevenue += p.RevenueBridge.GrossRevenue
		a.RevenueBridge.Returns += p.RevenueBridge.Returns
		a.RevenueBridge.Discounts += p.RevenueBridge.Discounts
		a.RevenueBridge.OtherRevenueReduction += p.RevenueBridge.OtherRevenueReduction
		a.RevenueBridge.OtherRevenue += p.RevenueBridge.OtherRevenue
		a.RevenueBridge.NetRevenue += p.RevenueBridge.NetRevenue

		a.DirectCostBridge.DirectMaterial += p.DirectCostBridge.DirectMaterial
		a.DirectCostBridge.DirectLabor += p.DirectCostBridge.DirectLabor
		a.DirectCostBridge.DirectSubcontractor += p.DirectCostBridge.DirectSubcontractor
		a.DirectCostBridge.DirectFulfillment += p.DirectCostBridge.DirectFulfillment
		a.DirectCostBridge.DirectOther += p.DirectCostBridge.DirectOther
		a.DirectCostBridge.Total += p.DirectCostBridge.Total

		a.VariableCostBridge.VariableCommission += p.VariableCostBridge.VariableCommission
		a.VariableCostBridge.VariablePaymentFees += p.VariableCostBridge.VariablePaymentFees
		a.VariableCostBridge.VariableOther += p.VariableCostBridge.VariableOther
		a.VariableCostBridge.Total += p.VariableCostBridge.Total

		a.GrossProfit += p.GrossProfit
		a.ContributionProfit += p.ContributionProfit
		a.AllocatedSharedCosts += p.AllocatedSharedCosts
		a.AllocatedProfit += p.AllocatedProfit
	}
	a.Margins = Margins{
		GrossMargin:        marginValue(a.GrossProfit, a.RevenueBridge.NetRevenue),
		ContributionMargin: marginValue(a.ContributionProfit, a.RevenueBridge.NetRevenue),
		AllocatedMargin:    marginValue(a.AllocatedProfit, a.RevenueBridge.NetRevenue),
	}
	return a
}
