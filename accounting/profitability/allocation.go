package profitability

import "sort"

// allocationBasisAmounts computes each candidate entity's raw basis
// amount for one pool/dimension/period allocation, plus the ordered list
// of candidate EntityIDs (deterministic ascending order) — task sections
// 22-24.
//
// candidateEntities is every EntityID known for dim (from Entities, or
// from any entity referenced by a Fact/summary for dim) — the universe an
// EQUAL/DRIVER/basis-total allocation draws from. entityNetRevenue/
// entityDirectCost/entityDirectLaborCost are that period's already-
// computed per-entity bridge figures; driverTotals is that period's
// per-entity DriverObservation totals for rule.DriverKey.
//
// For AllocationBasisFixedWeight/AllocationBasisEqual, an EntityID named
// in rule.FixedWeights that is NOT in candidateEntities (no fact/summary
// activity that period for this dimension) is excluded here rather than
// allocated to: the caller-supplied membership list can reference an
// entity that turns out inactive for this specific period, and silently
// including it would compute a nonzero AllocatedAmount that the
// entity-period rebuild step can never attach to any real
// EntityPeriodResult — an allocated-dollars-vanish bug caught during
// development. excludedIDs reports every such skipped EntityID so the
// caller gets a structured signal rather than a quietly short pool.
func allocationBasisAmounts(rule AllocationRule, candidateEntities []string,
	entityNetRevenue, entityDirectCost, entityDirectLaborCost, driverTotals map[string]float64) (amounts map[string]float64, ids []string, excludedIDs []string) {

	amounts = map[string]float64{}
	candidateSet := make(map[string]bool, len(candidateEntities))
	for _, id := range candidateEntities {
		candidateSet[id] = true
	}

	switch rule.Basis {
	case AllocationBasisNetRevenue:
		for _, id := range candidateEntities {
			amounts[id] = entityNetRevenue[id]
		}
		ids = append(ids, candidateEntities...)
	case AllocationBasisDirectCost:
		for _, id := range candidateEntities {
			amounts[id] = entityDirectCost[id]
		}
		ids = append(ids, candidateEntities...)
	case AllocationBasisDirectLaborCost:
		for _, id := range candidateEntities {
			amounts[id] = entityDirectLaborCost[id]
		}
		ids = append(ids, candidateEntities...)
	case AllocationBasisDriver:
		for _, id := range candidateEntities {
			amounts[id] = driverTotals[id]
		}
		ids = append(ids, candidateEntities...)
	case AllocationBasisFixedWeight:
		for _, w := range rule.FixedWeights {
			if !candidateSet[w.EntityID] {
				excludedIDs = append(excludedIDs, w.EntityID)
				continue
			}
			amounts[w.EntityID] = w.Weight
			ids = append(ids, w.EntityID)
		}
	case AllocationBasisEqual:
		members := candidateEntities
		if len(rule.FixedWeights) > 0 {
			members = nil
			for _, w := range rule.FixedWeights {
				if !candidateSet[w.EntityID] {
					excludedIDs = append(excludedIDs, w.EntityID)
					continue
				}
				members = append(members, w.EntityID)
			}
		}
		for _, id := range members {
			amounts[id] = 1
			ids = append(ids, id)
		}
	}

	sort.Strings(ids)
	sort.Strings(excludedIDs)
	return amounts, ids, excludedIDs
}

// allocatePool computes one pool's allocation for one Dimension/Period
// under rule, given the candidate basis amounts. Returns
// PoolAllocationResult plus any Issues — IssueAllocationEntityExcluded
// for a FixedWeights-named entity absent from candidateEntities, and/or
// IssueAllocationDenominatorUnavailable if the resulting basis
// denominator is zero/unavailable; every other rule problem was already
// caught by validateAllocationRules.
func allocatePool(pool SharedCostPool, dim Dimension, rule AllocationRule, candidateEntities []string,
	entityNetRevenue, entityDirectCost, entityDirectLaborCost, driverTotals map[string]float64, tolerance float64) (PoolAllocationResult, []Issue) {

	result := PoolAllocationResult{PoolID: pool.PoolID, Dimension: dim, Period: pool.Period, PoolAmount: pool.Amount}

	amounts, ids, excludedIDs := allocationBasisAmounts(rule, candidateEntities, entityNetRevenue, entityDirectCost, entityDirectLaborCost, driverTotals)

	var issues []Issue
	for _, excludedID := range excludedIDs {
		issues = append(issues, Issue{Code: IssueAllocationEntityExcluded, Severity: SeverityWarning,
			Message:  "allocation rule for pool " + pool.PoolID + " dimension " + string(dim) + " names entity " + excludedID + " which has no activity in period " + pool.Period + "; excluded from this allocation",
			SourceID: pool.PoolID, Dimension: dim, Period: pool.Period})
	}

	var denominator float64
	for _, id := range ids {
		denominator += amounts[id]
	}

	if denominator <= 0 || isNonFinite(denominator) {
		result.Status = PoolStatusDenominatorUnavailable
		result.UnallocatedAmount = pool.Amount
		issues = append(issues, Issue{Code: IssueAllocationDenominatorUnavailable, Severity: SeverityWarning,
			Message:  "shared cost pool " + pool.PoolID + " allocation basis denominator is zero or unavailable for dimension " + string(dim) + "; left unallocated",
			SourceID: pool.PoolID, Dimension: dim, Period: pool.Period})
		return result, issues
	}

	result.Status = PoolStatusAllocated
	var allocatedSum float64
	trace := make([]AllocationTraceEntry, 0, len(ids))
	for _, id := range ids {
		share := amounts[id] / denominator
		allocated := pool.Amount * share
		allocatedSum += allocated
		trace = append(trace, AllocationTraceEntry{
			PoolID: pool.PoolID, Dimension: dim, Basis: rule.Basis, DriverKey: rule.DriverKey,
			EntityID: id, Period: pool.Period, DriverAmount: amounts[id], AllocationShare: share, AllocatedAmount: allocated,
		})
	}
	result.Trace = trace
	result.AllocatedAmount = allocatedSum
	result.UnallocatedAmount = pool.Amount - allocatedSum
	if absFloat(result.UnallocatedAmount) < tolerance {
		result.UnallocatedAmount = 0
	}
	return result, issues
}
