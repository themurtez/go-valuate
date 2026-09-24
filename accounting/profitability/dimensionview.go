package profitability

import "sort"

// buildDimensionView computes the full DimensionView for one Dimension:
// attribution, entity-period results (from both Facts and summary rows,
// converging per task section 17), all-period totals, group/category
// summaries, coverage, allocation, rankings, trend, and flags.
func buildDimensionView(dim Dimension, periods []PeriodInfo, periodLabels []string, facts []Fact, summaries map[summaryKey]EntityPeriodSummaryInput,
	entitiesByKey map[entityKey]Entity, drivers []DriverObservation, driverIndex map[driverObsKey]float64,
	pools map[string]SharedCostPool, rules map[ruleKey]AllocationRule, policy Policy) (DimensionView, []Flag, []Issue, []AttributionCoverage, []DriverCoverage, []AllocationCoverage, []DimensionReconciliation) {

	attr := attributeFactsForDimension(facts, dim, policy.AttributionTolerance)

	// Merge in summary-input rows for this dimension: each summary row
	// contributes as if it were a single already-attributed "fact" for
	// its own entity/period — including into the SAME coverage/business-
	// total accumulators attributeFactsForDimension populates from Facts,
	// since a summary row is (by construction) 100% attributed to its one
	// EntityID. Without this, AttributionCoverage/DimensionReconciliation
	// would report zero economic activity for a summary-only dimension/
	// period even though EntityPeriods/AllPeriod show the real amounts —
	// a real gap caught during development.
	for k, s := range summaries {
		if k.Dimension != dim {
			continue
		}
		if attr.byEntityPeriod[k.EntityID] == nil {
			attr.byEntityPeriod[k.EntityID] = map[string]*entityAmounts{}
		}
		if attr.byEntityPeriod[k.EntityID][k.Period] == nil {
			attr.byEntityPeriod[k.EntityID][k.Period] = newEntityAmounts()
		}
		ea := attr.byEntityPeriod[k.EntityID][k.Period]
		for c, amt := range componentBridgeFromSummary(s) {
			ea.amounts[c] += amt
			if amt == 0 {
				continue
			}
			if attr.totalFactIDsByPeriod[k.Period] == nil {
				attr.totalFactIDsByPeriod[k.Period] = map[string]bool{}
			}
			if attr.attributedFactIDsByPeriod[k.Period] == nil {
				attr.attributedFactIDsByPeriod[k.Period] = map[string]bool{}
			}
			syntheticFactID := "SUMMARY-" + string(k.Dimension) + "-" + k.EntityID + "-" + k.Period + "-" + string(c)
			attr.totalFactIDsByPeriod[k.Period][syntheticFactID] = true
			attr.attributedFactIDsByPeriod[k.Period][syntheticFactID] = true
			attr.totalAmountByPeriod[k.Period] += amt
			attr.attributedTotalByPeriod[k.Period] += amt
			switch {
			case isRevenueComponent(c):
				attr.totalRevenueByPeriod[k.Period] += amt
				attr.attributedRevenueByPeriod[k.Period] += amt
			case isDirectCostComponent(c):
				attr.totalDirectCostByPeriod[k.Period] += amt
				attr.attributedDirectCostByPeriod[k.Period] += amt
			case isVariableCostComponent(c):
				attr.totalVariableCostByPeriod[k.Period] += amt
				attr.attributedVariableCostByPeriod[k.Period] += amt
			}
		}
	}

	hasAnyReference := len(attr.byEntityPeriod) > 0
	if !hasAnyReference {
		return DimensionView{Dimension: dim, Status: ViewStatusUnavailable}, nil, nil, nil, nil, nil, nil
	}

	entityIDs := make([]string, 0, len(attr.byEntityPeriod))
	for id := range attr.byEntityPeriod {
		entityIDs = append(entityIDs, id)
	}
	sort.Strings(entityIDs)

	// hasAnyRuleForDim gates the (potentially large) allocation-basis
	// precompute below: with zero AllocationRules for this dimension,
	// every pool is reported PoolStatusNotAllocated without ever needing
	// a single entity's NetRevenue/DirectCost/DirectLabor figure.
	hasAnyRuleForDim := false
	for k := range rules {
		if k.Dimension == dim {
			hasAnyRuleForDim = true
			break
		}
	}

	// entityPeriodBasis holds only the 3 float64 figures an allocation
	// basis can need (NetRevenue/DirectCost/DirectLabor) per entity/
	// period — never a full EntityPeriodResult, which carries several
	// nested structs/slices (RevenueBridge/DirectCostBridge/
	// VariableCostBridge/Margins/PerUnitMetrics) that allocation itself
	// never reads. Building the full struct across every entity x every
	// period was a real, measurable memory/GC cost at scale (a
	// benchmark_test.go scaling run showed allocation growing linearly
	// but GC-dominated wall-clock time growing faster than fact count),
	// caught by profiling this package's own large-population benchmark.
	type entityPeriodBasis struct {
		netRevenue, directCost, directLabor float64
	}
	var basisByEntityPeriod map[string]map[string]entityPeriodBasis
	if hasAnyRuleForDim {
		basisByEntityPeriod = make(map[string]map[string]entityPeriodBasis, len(entityIDs))
		for _, id := range entityIDs {
			basisByEntityPeriod[id] = make(map[string]entityPeriodBasis, len(periodLabels))
			for _, period := range periodLabels {
				ea := attr.byEntityPeriod[id][period]
				if ea == nil {
					continue
				}
				revenue := buildRevenueBridge(ea.amounts)
				directCost := buildDirectCostBridge(ea.amounts)
				basisByEntityPeriod[id][period] = entityPeriodBasis{
					netRevenue:  revenue.NetRevenue,
					directCost:  directCost.Total,
					directLabor: directCost.DirectLabor,
				}
			}
		}
	}

	// Allocation: for each pool with a rule for this dimension, allocate
	// per period.
	var allocationResults []PoolAllocationResult
	var allocIssues []Issue
	allocatedByEntityPeriod := map[string]map[string]float64{}
	traceByEntityPeriod := map[string]map[string][]AllocationTraceEntry{}
	poolIDs := sortedPoolIDs(pools)
	for _, poolID := range poolIDs {
		pool := pools[poolID]
		rule, ok := rules[ruleKey{PoolID: poolID, Dimension: dim}]
		if !ok {
			allocationResults = append(allocationResults, PoolAllocationResult{
				PoolID: poolID, Dimension: dim, Period: pool.Period, Status: PoolStatusNotAllocated,
				PoolAmount: pool.Amount, UnallocatedAmount: pool.Amount,
			})
			continue
		}
		entityNetRevenue := map[string]float64{}
		entityDirectCost := map[string]float64{}
		entityDirectLaborCost := map[string]float64{}
		entityDriverTotals := map[string]float64{}
		for _, id := range entityIDs {
			b := basisByEntityPeriod[id][pool.Period]
			entityNetRevenue[id] = b.netRevenue
			entityDirectCost[id] = b.directCost
			entityDirectLaborCost[id] = b.directLabor
			entityDriverTotals[id] = driverIndex[driverObsKey{Dimension: dim, EntityID: id, Period: pool.Period, DriverKey: rule.DriverKey}]
		}
		res, iss := allocatePool(pool, dim, rule, entityIDs, entityNetRevenue, entityDirectCost, entityDirectLaborCost, entityDriverTotals, policy.AllocationTolerance)
		allocationResults = append(allocationResults, res)
		allocIssues = append(allocIssues, iss...)
		for _, t := range res.Trace {
			if allocatedByEntityPeriod[t.EntityID] == nil {
				allocatedByEntityPeriod[t.EntityID] = map[string]float64{}
				traceByEntityPeriod[t.EntityID] = map[string][]AllocationTraceEntry{}
			}
			allocatedByEntityPeriod[t.EntityID][pool.Period] += t.AllocatedAmount
			traceByEntityPeriod[t.EntityID][pool.Period] = append(traceByEntityPeriod[t.EntityID][pool.Period], t)
		}
	}

	// Rebuild entity-period results including allocation.
	driverKeysIndex := driverKeysByEntityPeriod(dim, drivers)
	entityPeriodsByEntity := map[string][]EntityPeriodResult{}
	var allEntityPeriods []EntityPeriodResult
	for _, id := range entityIDs {
		var chron []EntityPeriodResult
		for _, period := range periodLabels {
			ea := attr.byEntityPeriod[id][period]
			amounts := map[Component]float64{}
			var factIDs []string
			if ea != nil {
				amounts = ea.amounts
				factIDs = sortedFactIDs(ea.factIDs)
			}
			allocated := allocatedByEntityPeriod[id][period]
			trace := traceByEntityPeriod[id][period]
			r := buildEntityPeriodResult(dim, id, period, amounts, allocated, trace, factIDs)

			driverKey := policy.PrimaryDriverKey[dim]
			if driverKey != "" {
				if v, ok := driverIndex[driverObsKey{Dimension: dim, EntityID: id, Period: period, DriverKey: driverKey}]; ok {
					r.PerUnitMetrics = buildPerUnitMetrics(driverKey, AvailableValue(v), r.RevenueBridge.NetRevenue, r.DirectCostBridge.Total, r.GrossProfit, r.ContributionProfit, true, true, true)
				}
			}
			r.ActivityDrivers = entityActivityDrivers(dim, id, period, driverIndex, driverKeysIndex)

			chron = append(chron, r)
		}
		entityPeriodsByEntity[id] = chron
		allEntityPeriods = append(allEntityPeriods, chron...)
	}

	// Only keep entity-periods where the entity had SOME activity that
	// period (non-zero amounts or allocation) to avoid emitting an
	// all-zero row for every entity x every period cross product.
	var filteredEntityPeriods []EntityPeriodResult
	filteredByEntity := map[string][]EntityPeriodResult{}
	for _, id := range entityIDs {
		for _, r := range entityPeriodsByEntity[id] {
			if !entityPeriodHasActivity(r) {
				continue
			}
			filteredEntityPeriods = append(filteredEntityPeriods, r)
			filteredByEntity[id] = append(filteredByEntity[id], r)
		}
	}

	var allPeriodResults []AllPeriodResult
	var trends []EntityTrend
	for _, id := range entityIDs {
		eps := filteredByEntity[id]
		if len(eps) == 0 {
			continue
		}
		allPeriodResults = append(allPeriodResults, buildAllPeriodResult(dim, id, eps))
		trends = append(trends, buildEntityTrend(dim, id, eps, policy.MinimumTrendPeriods))
	}

	sortEntityPeriodResultsForDisplay(filteredEntityPeriods)
	sortAllPeriodResultsForDisplay(allPeriodResults)

	groupOf := map[string]string{}
	categoryOf := map[string]string{}
	for k, e := range entitiesByKey {
		if k.Dimension != dim {
			continue
		}
		groupOf[k.EntityID] = e.Group
		categoryOf[k.EntityID] = e.Category
	}

	view := DimensionView{
		Dimension:         dim,
		Status:            ViewStatusAvailable,
		Periods:           periods,
		EntityPeriods:     filteredEntityPeriods,
		AllPeriod:         allPeriodResults,
		GroupSummaries:    buildGroupSummaries(dim, allPeriodResults, groupOf),
		CategorySummaries: buildGroupSummaries(dim, allPeriodResults, categoryOf),
		AllocationResults: allocationResults,
		Rankings:          buildRankings(allPeriodResults, policy.TopN, policy.NegativeContributionMateriality),
		MarginRanking:     buildMarginRanking(allPeriodResults, policy.MinimumRevenueForMarginRanking),
		Trends:            trends,
	}

	var coverage []AttributionCoverage
	unattributedByPeriod := map[string]UnattributedAmounts{}
	view.Unattributed = map[string]UnattributedAmounts{}
	for _, period := range periodLabels {
		c := buildAttributionCoverage(dim, period, attr)
		coverage = append(coverage, c)
		u := buildUnattributedAmounts(attr.unattributedByPeriod[period])
		unattributedByPeriod[period] = u
		view.Unattributed[period] = u
	}
	view.AttributionCoverage = coverage

	if policy.StrictAttributionCoverage {
		for _, c := range coverage {
			shortfall := (policy.MinRevenueAttributionCoverage > 0 && c.RevenueCoveragePercent.Available && c.RevenueCoveragePercent.Amount < policy.MinRevenueAttributionCoverage) ||
				(policy.MinCostAttributionCoverage > 0 && c.CostCoveragePercent.Available && c.CostCoveragePercent.Amount < policy.MinCostAttributionCoverage)
			if shortfall {
				view.Status = ViewStatusAvailableWithGaps
			}
		}
	}

	var driverCoverage []DriverCoverage
	for _, period := range periodLabels {
		entitiesWithDrivers := 0
		for _, id := range entityIDs {
			if len(driverKeysIndex[entityPeriodKey{EntityID: id, Period: period}]) > 0 {
				entitiesWithDrivers++
			}
		}
		driverCoverage = append(driverCoverage, DriverCoverage{Dimension: dim, Period: period, EntitiesWithDrivers: entitiesWithDrivers, TotalEntities: len(entityIDs)})
	}

	var allocCoverage []AllocationCoverage
	for _, period := range periodLabels {
		total, allocated, unallocated := 0, 0, 0
		for _, r := range allocationResults {
			if r.Period != period {
				continue
			}
			total++
			if r.Status == PoolStatusAllocated {
				allocated++
			} else {
				unallocated++
			}
		}
		if total > 0 {
			allocCoverage = append(allocCoverage, AllocationCoverage{Dimension: dim, Period: period, TotalPools: total, AllocatedPools: allocated, UnallocatedPools: unallocated})
		}
	}

	// allPeriodByEntity/trendByEntity give O(1) lookup by EntityID for the
	// flags loop below — allPeriodResults was just reordered by
	// sortAllPeriodResultsForDisplay, so it (and trends, built in the same
	// original entityIDs order) can no longer be indexed positionally by
	// entityIDs. A linear indexOfAllPeriod scan per entity here was a
	// real O(entities^2) bug caught by benchmark_test.go's scaling
	// benchmark (100,000 facts took >13x the 10,000-fact runtime instead
	// of the expected ~10x).
	allPeriodByEntity := make(map[string]AllPeriodResult, len(allPeriodResults))
	for _, a := range allPeriodResults {
		allPeriodByEntity[a.EntityID] = a
	}
	trendByEntity := make(map[string]EntityTrend, len(trends))
	for _, tr := range trends {
		trendByEntity[tr.EntityID] = tr
	}

	var flags []Flag
	flags = append(flags, computeCoverageFlags(dim, coverage, unattributedByPeriod, policy)...)
	flags = append(flags, computeAllocationFlags(dim, allocationResults, policy.AllocationTolerance)...)
	for _, id := range entityIDs {
		eps := filteredByEntity[id]
		if len(eps) == 0 {
			continue
		}
		latest := eps[len(eps)-1]
		flags = append(flags, computeEntityFlags(dim, allPeriodByEntity[id], latest, trendByEntity[id], policy)...)
	}
	view.Flags = flags

	var dimRecon []DimensionReconciliation
	for _, period := range periodLabels {
		businessAmount := attr.totalAmountByPeriod[period]
		attributed := attr.attributedTotalByPeriod[period]
		u := unattributedByPeriod[period]
		unattributedTotal := u.UnattributedRevenue + u.UnattributedDirectCost + u.UnattributedVariableCost
		dimRecon = append(dimRecon, DimensionReconciliation{
			Dimension: dim, Period: period, Attributed: attributed, Unattributed: unattributedTotal,
			BusinessAmount: businessAmount, Reconciled: absFloat((attributed+unattributedTotal)-businessAmount) <= policy.AllocationTolerance,
		})
	}

	return view, flags, allocIssues, coverage, driverCoverage, allocCoverage, dimRecon
}

func entityPeriodHasActivity(r EntityPeriodResult) bool {
	return r.RevenueBridge.GrossRevenue != 0 || r.RevenueBridge.OtherRevenue != 0 ||
		r.RevenueBridge.Returns != 0 || r.RevenueBridge.Discounts != 0 || r.RevenueBridge.OtherRevenueReduction != 0 ||
		r.DirectCostBridge.Total != 0 || r.VariableCostBridge.Total != 0 || r.AllocatedSharedCosts != 0
}

func sortedFactIDs(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for id := range m {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func sortedPoolIDs(pools map[string]SharedCostPool) []string {
	out := make([]string, 0, len(pools))
	for id := range pools {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// entityPeriodKey identifies one (EntityID, Period) slot — used by
// driverKeysByEntityPeriod below to pre-group DriverObservations once
// (O(drivers)) rather than rescanning the full drivers slice once per
// (entity, period) pair (which would be O(entities * periods * drivers))
// — the latter was a real quadratic-in-drivers risk caught during
// development, invisible to benchmark_test.go's own fixtures (which
// supply zero DriverObservations) but real for any caller who populates
// one driver observation per entity per period.
type entityPeriodKey struct{ EntityID, Period string }

// driverKeysByEntityPeriod groups drivers by (EntityID, Period) for one
// Dimension, returning each slot's distinct DriverKeys in deterministic
// ascending order.
func driverKeysByEntityPeriod(dim Dimension, drivers []DriverObservation) map[entityPeriodKey][]string {
	seen := map[entityPeriodKey]map[string]bool{}
	for _, d := range drivers {
		if d.Dimension != dim {
			continue
		}
		k := entityPeriodKey{EntityID: d.EntityID, Period: d.Period}
		if seen[k] == nil {
			seen[k] = map[string]bool{}
		}
		seen[k][d.DriverKey] = true
	}
	out := make(map[entityPeriodKey][]string, len(seen))
	for k, keySet := range seen {
		keys := make([]string, 0, len(keySet))
		for dk := range keySet {
			keys = append(keys, dk)
		}
		sort.Strings(keys)
		out[k] = keys
	}
	return out
}

func entityActivityDrivers(dim Dimension, entityID, period string, index map[driverObsKey]float64, driverKeys map[entityPeriodKey][]string) map[string]float64 {
	keys := driverKeys[entityPeriodKey{EntityID: entityID, Period: period}]
	if len(keys) == 0 {
		return nil
	}
	out := map[string]float64{}
	for _, k := range keys {
		out[k] = index[driverObsKey{Dimension: dim, EntityID: entityID, Period: period, DriverKey: k}]
	}
	return out
}
