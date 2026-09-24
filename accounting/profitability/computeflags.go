package profitability

// computeEntityFlags derives every entity-level Flag for one dimension
// from its all-period totals, its most recent period, and its trend —
// task sections 46-53. Every message uses neutral, factual language — see
// safety_test.go.
func computeEntityFlags(dim Dimension, allPeriod AllPeriodResult, latest EntityPeriodResult, trend EntityTrend, policy Policy) []Flag {
	var flags []Flag

	if allPeriod.GrossProfit < 0 {
		flags = append(flags, Flag{Code: FlagNegativeGrossProfit, Severity: FlagSeverityWarning, Dimension: dim, EntityID: allPeriod.EntityID,
			Message: "entity " + allPeriod.EntityID + " has negative gross profit across the analyzed periods"})
	}
	if allPeriod.ContributionProfit < 0 && policy.NegativeContributionMateriality.isMaterial(allPeriod.ContributionProfit, allPeriod.RevenueBridge.NetRevenue) {
		flags = append(flags, Flag{Code: FlagNegativeContribution, Severity: FlagSeverityWarning, Dimension: dim, EntityID: allPeriod.EntityID,
			Message: "entity " + allPeriod.EntityID + " has negative contribution profit across the analyzed periods"})
	}
	if allPeriod.AllocatedProfit < 0 && policy.NegativeContributionMateriality.isMaterial(allPeriod.AllocatedProfit, allPeriod.RevenueBridge.NetRevenue) {
		flags = append(flags, Flag{Code: FlagNegativeAllocatedProfit, Severity: FlagSeverityWarning, Dimension: dim, EntityID: allPeriod.EntityID,
			Message: "entity " + allPeriod.EntityID + " has negative allocated profit across the analyzed periods"})
	}

	leak := buildMarginLeakage(latest)
	if leak.ReturnRate.Available && leak.ReturnRate.Amount >= policy.HighReturnRate {
		flags = append(flags, Flag{Code: FlagHighReturnRate, Severity: FlagSeverityInfo, Dimension: dim, EntityID: allPeriod.EntityID, Period: latest.Period,
			Message: "entity " + allPeriod.EntityID + " return rate in period " + latest.Period + " is at or above the configured threshold"})
	}
	if leak.DiscountRate.Available && leak.DiscountRate.Amount >= policy.HighDiscountRate {
		flags = append(flags, Flag{Code: FlagHighDiscountRate, Severity: FlagSeverityInfo, Dimension: dim, EntityID: allPeriod.EntityID, Period: latest.Period,
			Message: "entity " + allPeriod.EntityID + " discount rate in period " + latest.Period + " is at or above the configured threshold"})
	}

	// Margin compression: gross/contribution margin adjacent-period
	// decline exceeding the configured percentage-point threshold — task
	// section 49.
	if compressed(trend.GrossMarginChange, policy.GrossMarginCompressionPoints) || compressed(trend.ContributionMarginChange, policy.ContributionMarginCompressionPoints) {
		flags = append(flags, Flag{Code: FlagMarginCompression, Severity: FlagSeverityWarning, Dimension: dim, EntityID: allPeriod.EntityID, Period: latest.Period,
			Message: "entity " + allPeriod.EntityID + " gross or contribution margin declined by more than the configured percentage-point threshold between " + trend.GrossMarginChange.FromPeriod + " and " + trend.GrossMarginChange.ToPeriod})
	}

	// Revenue growth with contribution/margin deterioration — task
	// section 50. No causal conclusion; purely a factual co-occurrence.
	revenueGrowing := trend.NetRevenueChange.Available && trend.NetRevenueChange.AbsoluteChange > 0
	if revenueGrowing && trend.ContributionProfitChange.Available && trend.ContributionProfitChange.AbsoluteChange < 0 {
		flags = append(flags, Flag{Code: FlagRevenueGrowthWithContributionDecline, Severity: FlagSeverityWarning, Dimension: dim, EntityID: allPeriod.EntityID, Period: latest.Period,
			Message: "entity " + allPeriod.EntityID + " net revenue increased while contribution profit decreased between " + trend.NetRevenueChange.FromPeriod + " and " + trend.NetRevenueChange.ToPeriod})
	}
	if revenueGrowing && compressed(trend.ContributionMarginChange, policy.ContributionMarginCompressionPoints) {
		flags = append(flags, Flag{Code: FlagRevenueGrowthWithMarginCompression, Severity: FlagSeverityWarning, Dimension: dim, EntityID: allPeriod.EntityID, Period: latest.Period,
			Message: "entity " + allPeriod.EntityID + " net revenue increased while contribution margin compressed between " + trend.ContributionMarginChange.FromPeriod + " and " + trend.ContributionMarginChange.ToPeriod})
	}

	// Cost-share increase flags — task section 52.
	if increasing(trend.DirectLaborPercentChange, policy.CostShareIncreasePoints) {
		flags = append(flags, Flag{Code: FlagDirectLaborShareIncreasing, Severity: FlagSeverityInfo, Dimension: dim, EntityID: allPeriod.EntityID, Period: latest.Period,
			Message: "entity " + allPeriod.EntityID + " direct labor share of revenue increased by more than the configured threshold between " + trend.DirectLaborPercentChange.FromPeriod + " and " + trend.DirectLaborPercentChange.ToPeriod})
	}
	if increasing(trend.DirectMaterialPercentChange, policy.CostShareIncreasePoints) {
		flags = append(flags, Flag{Code: FlagDirectMaterialShareIncreasing, Severity: FlagSeverityInfo, Dimension: dim, EntityID: allPeriod.EntityID, Period: latest.Period,
			Message: "entity " + allPeriod.EntityID + " direct material share of revenue increased by more than the configured threshold between " + trend.DirectMaterialPercentChange.FromPeriod + " and " + trend.DirectMaterialPercentChange.ToPeriod})
	}

	return flags
}

// compressed reports whether change represents a margin decline
// exceeding thresholdPoints (expressed as a fraction).
func compressed(change AdjacentChange, thresholdPoints float64) bool {
	return change.Available && -change.AbsoluteChange >= thresholdPoints
}

// increasing reports whether change represents an increase exceeding
// thresholdPoints (expressed as a fraction).
func increasing(change AdjacentChange, thresholdPoints float64) bool {
	return change.Available && change.AbsoluteChange >= thresholdPoints
}

// computeCoverageFlags derives LOW_*_ATTRIBUTION_COVERAGE and
// MATERIAL_UNATTRIBUTED_* flags from AttributionCoverage/Unattributed —
// task section 53.
func computeCoverageFlags(dim Dimension, coverage []AttributionCoverage, unattributed map[string]UnattributedAmounts, policy Policy) []Flag {
	var flags []Flag
	for _, c := range coverage {
		if policy.MinRevenueAttributionCoverage > 0 && c.RevenueCoveragePercent.Available && c.RevenueCoveragePercent.Amount < policy.MinRevenueAttributionCoverage {
			flags = append(flags, Flag{Code: FlagLowRevenueAttributionCoverage, Severity: FlagSeverityWarning, Dimension: dim, Period: c.Period,
				Message: "revenue attribution coverage for dimension " + string(dim) + " in period " + c.Period + " is below the configured minimum"})
		}
		if policy.MinCostAttributionCoverage > 0 && c.CostCoveragePercent.Available && c.CostCoveragePercent.Amount < policy.MinCostAttributionCoverage {
			flags = append(flags, Flag{Code: FlagLowCostAttributionCoverage, Severity: FlagSeverityWarning, Dimension: dim, Period: c.Period,
				Message: "cost attribution coverage for dimension " + string(dim) + " in period " + c.Period + " is below the configured minimum"})
		}
		u := unattributed[c.Period]
		if policy.UnattributedMateriality.isMaterial(u.UnattributedRevenue, c.TotalRevenueAmount) {
			flags = append(flags, Flag{Code: FlagMaterialUnattributedRevenue, Severity: FlagSeverityWarning, Dimension: dim, Period: c.Period,
				Message: "unattributed revenue for dimension " + string(dim) + " in period " + c.Period + " meets the configured materiality threshold"})
		}
		totalCost := u.UnattributedDirectCost + u.UnattributedVariableCost
		if policy.UnattributedMateriality.isMaterial(totalCost, c.TotalRevenueAmount) {
			flags = append(flags, Flag{Code: FlagMaterialUnattributedCost, Severity: FlagSeverityWarning, Dimension: dim, Period: c.Period,
				Message: "unattributed cost for dimension " + string(dim) + " in period " + c.Period + " meets the configured materiality threshold"})
		}
	}
	return flags
}

// computeAllocationFlags derives SHARED_COST_POOL_UNALLOCATED and
// PARTIALLY_ALLOCATED_SHARED_COST flags from a dimension's
// PoolAllocationResults — task section 54. Distinguishes "not allocated
// by choice" (PoolStatusNotAllocated, no flag — this is the expected
// default) from "allocation requested but incomplete"
// (PoolStatusDenominatorUnavailable, or Allocated partially covering the
// pool).
func computeAllocationFlags(dim Dimension, results []PoolAllocationResult, tolerance float64) []Flag {
	var flags []Flag
	for _, r := range results {
		switch r.Status {
		case PoolStatusDenominatorUnavailable:
			flags = append(flags, Flag{Code: FlagSharedCostPoolUnallocated, Severity: FlagSeverityInfo, Dimension: dim, Period: r.Period, PoolID: r.PoolID,
				Message: "shared cost pool " + r.PoolID + " for dimension " + string(dim) + " in period " + r.Period + " has no available allocation basis and remains unallocated"})
		case PoolStatusAllocated:
			if r.UnallocatedAmount > tolerance {
				flags = append(flags, Flag{Code: FlagPartiallyAllocatedSharedCost, Severity: FlagSeverityInfo, Dimension: dim, Period: r.Period, PoolID: r.PoolID,
					Message: "shared cost pool " + r.PoolID + " for dimension " + string(dim) + " in period " + r.Period + " was only partially allocated"})
			}
		}
	}
	return flags
}

// computeControlFlags derives CONTROL_TOTAL_MISMATCH flags from
// ControlReconciliation.
func computeControlFlags(recs []ControlReconciliation, policy Policy) []Flag {
	var flags []Flag
	for _, rec := range recs {
		if !rec.Available {
			continue
		}
		var netRevenue float64
		for _, c := range rec.Components {
			if c.Label == "net_revenue" && c.Computed.Available {
				netRevenue = c.Computed.Amount
			}
		}
		for _, c := range rec.Components {
			if !c.Difference.Available || c.Reconciled {
				continue
			}
			if policy.ControlMateriality.isMaterial(c.Difference.Amount, netRevenue) {
				flags = append(flags, Flag{Code: FlagControlTotalMismatch, Severity: FlagSeverityWarning, Period: rec.Period,
					Message: "control total for " + c.Label + " in period " + rec.Period + " differs from the computed business total by a material amount"})
			}
		}
	}
	return flags
}
