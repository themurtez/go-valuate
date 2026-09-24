package profitability_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/profitability"
	"github.com/themurtez/go-valuate/accounting/profitability/fixtures"
)

// TestAllocation_NotAllocatedByDefault locks task section 21: a pool with
// no AllocationRule is reported unallocated with no error/warning
// (expected default behavior).
func TestAllocation_NotAllocatedByDefault(t *testing.T) {
	entities := []profitability.Entity{
		{Dimension: profitability.DimensionCustomer, EntityID: "C1", Active: true},
		{Dimension: profitability.DimensionJob, EntityID: "J1", Active: true},
		{Dimension: profitability.DimensionProduct, EntityID: "PR1", Active: true},
	}
	facts := []profitability.Fact{
		{FactID: "F1", Period: "2025-01", Component: profitability.ComponentGrossRevenue, Amount: 100, Currency: "USD",
			Attributions: []profitability.Attribution{
				{Dimension: profitability.DimensionCustomer, EntityID: "C1", Share: 1},
				{Dimension: profitability.DimensionJob, EntityID: "J1", Share: 1},
				{Dimension: profitability.DimensionProduct, EntityID: "PR1", Share: 1},
			}},
	}
	pools := []profitability.SharedCostPool{{PoolID: "P1", Period: "2025-01", Amount: 500}}
	r := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), Entities: entities, Facts: facts, SharedCostPools: pools}, profitability.DefaultPolicy())

	for _, view := range []profitability.DimensionView{r.CustomerView, r.JobView, r.ProductView} {
		found := false
		for _, res := range view.AllocationResults {
			if res.PoolID == "P1" {
				found = true
				if res.Status != profitability.PoolStatusNotAllocated {
					t.Errorf("dimension %s: expected PoolStatusNotAllocated, got %s", view.Dimension, res.Status)
				}
				if res.UnallocatedAmount != 500 {
					t.Errorf("dimension %s: expected full 500 unallocated, got %v", view.Dimension, res.UnallocatedAmount)
				}
			}
		}
		if !found {
			t.Errorf("dimension %s: expected a PoolAllocationResult for P1", view.Dimension)
		}
	}
	for _, iss := range r.Issues {
		if iss.Code == profitability.IssueAllocationDenominatorUnavailable {
			t.Error("no allocation rule was supplied; must not raise IssueAllocationDenominatorUnavailable")
		}
	}
}

// TestAllocation_ZeroDenominatorLeavesUnallocated locks task section 25:
// a zero/unavailable basis denominator leaves the pool unallocated with a
// structured Issue, never falling back to equal allocation.
func TestAllocation_ZeroDenominatorLeavesUnallocated(t *testing.T) {
	entities := []profitability.Entity{{Dimension: profitability.DimensionCustomer, EntityID: "C1", Active: true}}
	// C1 has zero net revenue (only a cost fact), so NET_REVENUE basis
	// denominator is zero.
	facts := []profitability.Fact{
		{FactID: "F1", Period: "2025-01", Component: profitability.ComponentDirectOther, Amount: 50, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"CUSTOMER", "C1"})},
	}
	pools := []profitability.SharedCostPool{{PoolID: "P1", Period: "2025-01", Amount: 200}}
	rules := []profitability.AllocationRule{{PoolID: "P1", Dimension: profitability.DimensionCustomer, Basis: profitability.AllocationBasisNetRevenue}}
	r := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), Entities: entities, Facts: facts, SharedCostPools: pools, AllocationRules: rules}, profitability.DefaultPolicy())

	if !hasIssue(r.Issues, profitability.IssueAllocationDenominatorUnavailable) {
		t.Error("expected IssueAllocationDenominatorUnavailable")
	}
	for _, res := range r.CustomerView.AllocationResults {
		if res.PoolID != "P1" {
			continue
		}
		if res.Status != profitability.PoolStatusDenominatorUnavailable {
			t.Errorf("expected PoolStatusDenominatorUnavailable, got %s", res.Status)
		}
		if res.UnallocatedAmount != 200 {
			t.Errorf("expected full pool amount (200) unallocated, got %v", res.UnallocatedAmount)
		}
		if res.AllocatedAmount != 0 {
			t.Error("must never fall back to equal allocation when the basis denominator is unavailable")
		}
	}
}

// TestAllocation_DifferentBasisPerDimension locks task section 23: the
// same pool can carry independent allocation rules/results per
// dimension.
func TestAllocation_DifferentBasisPerDimension(t *testing.T) {
	entities, facts, pools, rules := fixtures.DifferentBasisPerDimensionScenario()
	r := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), Entities: entities, Facts: facts, SharedCostPools: pools, AllocationRules: rules}, profitability.DefaultPolicy())

	// CUSTOMER: EQUAL basis across CUST-X/CUST-Y => 2000/2000.
	custAlloc := map[string]float64{}
	for _, e := range r.CustomerView.AllPeriod {
		custAlloc[e.EntityID] = e.AllocatedSharedCosts
	}
	if !closeEnough(custAlloc["CUST-X"], 2000) || !closeEnough(custAlloc["CUST-Y"], 2000) {
		t.Errorf("expected equal 2000/2000 customer allocation, got %v", custAlloc)
	}

	// JOB: FIXED_WEIGHT 3:1 across JOB-X/JOB-Y => 3000/1000.
	jobAlloc := map[string]float64{}
	for _, e := range r.JobView.AllPeriod {
		jobAlloc[e.EntityID] = e.AllocatedSharedCosts
	}
	if !closeEnough(jobAlloc["JOB-X"], 3000) || !closeEnough(jobAlloc["JOB-Y"], 1000) {
		t.Errorf("expected fixed-weight 3000/1000 job allocation, got %v", jobAlloc)
	}
}

// TestAllocation_TraceReconciles verifies AllocationTraceEntry rows sum
// to the AllocatedAmount reported for each pool.
func TestAllocation_TraceReconciles(t *testing.T) {
	entities, facts, pools, rules := fixtures.SharedCostAllocationScenario()
	r := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), Entities: entities, Facts: facts, SharedCostPools: pools, AllocationRules: rules}, profitability.DefaultPolicy())

	for _, res := range r.CustomerView.AllocationResults {
		var traceSum float64
		for _, tr := range res.Trace {
			traceSum += tr.AllocatedAmount
			if tr.PoolID != res.PoolID || tr.Dimension != res.Dimension || tr.Period != res.Period {
				t.Errorf("trace entry %+v does not match its owning result %+v", tr, res)
			}
		}
		if !closeEnough(traceSum, res.AllocatedAmount) {
			t.Errorf("trace entries sum to %v, want AllocatedAmount %v", traceSum, res.AllocatedAmount)
		}
	}
}

// TestAllocation_DirectFactsNeverAllocated locks task section 28: a
// directly-attributed direct-cost fact flows only through
// DirectCostBridge/GrossProfit, never through AllocatedSharedCosts — an
// entity with only direct costs and no shared-cost pool referencing it
// must show AllocatedSharedCosts == 0 while its direct costs still
// reduce ContributionProfit/AllocatedProfit normally.
func TestAllocation_DirectFactsNeverAllocated(t *testing.T) {
	entities := []profitability.Entity{{Dimension: profitability.DimensionCustomer, EntityID: "C1", Active: true}}
	facts := []profitability.Fact{
		{FactID: "F-REV", Period: "2025-01", Component: profitability.ComponentGrossRevenue, Amount: 1000, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"CUSTOMER", "C1"})},
		{FactID: "F-MAT", Period: "2025-01", Component: profitability.ComponentDirectMaterial, Amount: 400, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"CUSTOMER", "C1"})},
	}
	r := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), Entities: entities, Facts: facts}, profitability.DefaultPolicy())

	if len(r.CustomerView.AllPeriod) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(r.CustomerView.AllPeriod))
	}
	e := r.CustomerView.AllPeriod[0]
	if e.AllocatedSharedCosts != 0 {
		t.Errorf("AllocatedSharedCosts = %v, want 0 (no shared-cost pool was ever supplied)", e.AllocatedSharedCosts)
	}
	if !closeEnough(e.DirectCostBridge.DirectMaterial, 400) {
		t.Errorf("DirectMaterial = %v, want 400", e.DirectCostBridge.DirectMaterial)
	}
	if !closeEnough(e.GrossProfit, 600) {
		t.Errorf("GrossProfit = %v, want 600", e.GrossProfit)
	}
	if !closeEnough(e.AllocatedProfit, e.ContributionProfit) {
		t.Errorf("AllocatedProfit (%v) should equal ContributionProfit (%v) when AllocatedSharedCosts is 0", e.AllocatedProfit, e.ContributionProfit)
	}
}
