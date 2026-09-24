package profitability_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/profitability"
	"github.com/themurtez/go-valuate/accounting/profitability/fixtures"
)

const tol = 0.01

func closeEnough(a, b float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= tol
}

// TestInvariant_BusinessComponentTotalsMatchSourceFacts locks task
// section 67 "Business total: business component totals == source facts
// by component."
func TestInvariant_BusinessComponentTotalsMatchSourceFacts(t *testing.T) {
	entities, facts := fixtures.HighRevenueLowMarginCustomer()
	in := profitability.Input{Periods: fixtures.TwoMonthPeriods(), Entities: entities, Facts: facts}
	r := profitability.Calculate(in, profitability.DefaultPolicy())

	var wantRevenue, wantMaterial, wantLabor float64
	for _, f := range facts {
		switch f.Component {
		case profitability.ComponentGrossRevenue:
			wantRevenue += f.Amount
		case profitability.ComponentDirectMaterial:
			wantMaterial += f.Amount
		case profitability.ComponentDirectLabor:
			wantLabor += f.Amount
		}
	}
	bt := r.BusinessTotals.AllPeriod
	if !closeEnough(bt.RevenueBridge.GrossRevenue, wantRevenue) {
		t.Errorf("business gross revenue = %v, want %v", bt.RevenueBridge.GrossRevenue, wantRevenue)
	}
	if !closeEnough(bt.DirectCostBridge.DirectMaterial, wantMaterial) {
		t.Errorf("business direct material = %v, want %v", bt.DirectCostBridge.DirectMaterial, wantMaterial)
	}
	if !closeEnough(bt.DirectCostBridge.DirectLabor, wantLabor) {
		t.Errorf("business direct labor = %v, want %v", bt.DirectCostBridge.DirectLabor, wantLabor)
	}
}

// TestInvariant_DimensionReconciliation locks task section 67
// "Dimension reconciliation: Attributed + Unattributed == Business
// total."
func TestInvariant_DimensionReconciliation(t *testing.T) {
	entities, facts := fixtures.PartialAttributionAcrossTwoProducts()
	facts = append(facts, fixtures.UnattributedFacts()...)
	in := profitability.Input{Periods: fixtures.TwoMonthPeriods(), Entities: entities, Facts: facts}
	r := profitability.Calculate(in, profitability.DefaultPolicy())

	if len(r.DimensionReconciliation) == 0 {
		t.Fatal("expected DimensionReconciliation entries")
	}
	for _, dr := range r.DimensionReconciliation {
		if !dr.Reconciled {
			t.Errorf("dimension %s period %s: attributed(%v) + unattributed(%v) != business(%v)",
				dr.Dimension, dr.Period, dr.Attributed, dr.Unattributed, dr.BusinessAmount)
		}
	}
}

// TestInvariant_AllocationReconciliation locks task section 67
// "Allocation: Allocated + Unallocated == Pool total."
func TestInvariant_AllocationReconciliation(t *testing.T) {
	entities, facts, pools, rules := fixtures.SharedCostAllocationScenario()
	in := profitability.Input{Periods: fixtures.TwoMonthPeriods(), Entities: entities, Facts: facts, SharedCostPools: pools, AllocationRules: rules}
	r := profitability.Calculate(in, profitability.DefaultPolicy())

	found := false
	for _, res := range r.CustomerView.AllocationResults {
		found = true
		sum := res.AllocatedAmount + res.UnallocatedAmount
		if !closeEnough(sum, res.PoolAmount) {
			t.Errorf("pool %s: allocated(%v) + unallocated(%v) = %v, want %v", res.PoolID, res.AllocatedAmount, res.UnallocatedAmount, sum, res.PoolAmount)
		}
	}
	if !found {
		t.Fatal("expected at least one allocation result")
	}
}

// TestInvariant_NoDoubleCountAcrossDimensions locks task section 67 "A
// single $1,000 fact attributed 100% to one customer, one job, and one
// product must yield $1,000 in each view, never $3,000 business
// revenue."
func TestInvariant_NoDoubleCountAcrossDimensions(t *testing.T) {
	entities := []profitability.Entity{
		{Dimension: profitability.DimensionCustomer, EntityID: "C1", Active: true},
		{Dimension: profitability.DimensionJob, EntityID: "J1", Active: true},
		{Dimension: profitability.DimensionProduct, EntityID: "PR1", Active: true},
	}
	facts := []profitability.Fact{
		{FactID: "TRIPLE-1", Period: "2025-01", Component: profitability.ComponentGrossRevenue, Amount: 1000, Currency: "USD",
			Attributions: []profitability.Attribution{
				{Dimension: profitability.DimensionCustomer, EntityID: "C1", Share: 1},
				{Dimension: profitability.DimensionJob, EntityID: "J1", Share: 1},
				{Dimension: profitability.DimensionProduct, EntityID: "PR1", Share: 1},
			}},
	}
	in := profitability.Input{Periods: fixtures.TwoMonthPeriods(), Entities: entities, Facts: facts}
	r := profitability.Calculate(in, profitability.DefaultPolicy())

	if got := r.BusinessTotals.AllPeriod.RevenueBridge.GrossRevenue; !closeEnough(got, 1000) {
		t.Fatalf("business gross revenue = %v, want 1000 (never 3000)", got)
	}

	for _, view := range []profitability.DimensionView{r.CustomerView, r.JobView, r.ProductView} {
		if len(view.AllPeriod) != 1 {
			t.Fatalf("dimension %s: expected exactly 1 entity, got %d", view.Dimension, len(view.AllPeriod))
		}
		if got := view.AllPeriod[0].RevenueBridge.GrossRevenue; !closeEnough(got, 1000) {
			t.Errorf("dimension %s entity revenue = %v, want 1000", view.Dimension, got)
		}
	}
}

// TestInvariant_PartialAttribution locks task section 67 "Partial
// attribution: $1,000 revenue with 60% P1 + 20% P2 => 600 / 200 / 200
// unattributed."
func TestInvariant_PartialAttribution(t *testing.T) {
	entities, facts := fixtures.PartialAttributionAcrossTwoProducts()
	in := profitability.Input{Periods: fixtures.TwoMonthPeriods(), Entities: entities, Facts: facts}
	r := profitability.Calculate(in, profitability.DefaultPolicy())

	var p1, p2 float64
	for _, e := range r.ProductView.AllPeriod {
		switch e.EntityID {
		case "P1":
			p1 = e.RevenueBridge.GrossRevenue
		case "P2":
			p2 = e.RevenueBridge.GrossRevenue
		}
	}
	if !closeEnough(p1, 600) {
		t.Errorf("P1 revenue = %v, want 600", p1)
	}
	if !closeEnough(p2, 200) {
		t.Errorf("P2 revenue = %v, want 200", p2)
	}
	u := r.ProductView.Unattributed["2025-01"]
	if !closeEnough(u.UnattributedRevenue, 200) {
		t.Errorf("unattributed revenue = %v, want 200", u.UnattributedRevenue)
	}
}

// TestInvariant_AllPeriodMarginIsNotAverage locks task section 67/30
// "All-period margin = total profit / total revenue, not average of
// period margins."
func TestInvariant_AllPeriodMarginIsNotAverage(t *testing.T) {
	entities := []profitability.Entity{
		{Dimension: profitability.DimensionCustomer, EntityID: "C1", Active: true},
	}
	// Period 1: revenue 100, profit 90 (margin 0.9). Period 2: revenue
	// 900, profit 90 (margin 0.1). Average of margins = 0.5. True
	// combined margin = 180/1000 = 0.18.
	facts := []profitability.Fact{
		{FactID: "M1-REV", Period: "2025-01", Component: profitability.ComponentGrossRevenue, Amount: 100, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"CUSTOMER", "C1"})},
		{FactID: "M1-COST", Period: "2025-01", Component: profitability.ComponentDirectOther, Amount: 10, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"CUSTOMER", "C1"})},
		{FactID: "M2-REV", Period: "2025-02", Component: profitability.ComponentGrossRevenue, Amount: 900, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"CUSTOMER", "C1"})},
		{FactID: "M2-COST", Period: "2025-02", Component: profitability.ComponentDirectOther, Amount: 810, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"CUSTOMER", "C1"})},
	}
	in := profitability.Input{Periods: fixtures.TwoMonthPeriods(), Entities: entities, Facts: facts}
	r := profitability.Calculate(in, profitability.DefaultPolicy())

	if len(r.CustomerView.AllPeriod) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(r.CustomerView.AllPeriod))
	}
	all := r.CustomerView.AllPeriod[0]
	wantMargin := 180.0 / 1000.0
	if !all.Margins.GrossMargin.Available || !closeEnough(all.Margins.GrossMargin.Amount, wantMargin) {
		t.Errorf("all-period gross margin = %+v, want %v (total profit / total revenue, not averaged)", all.Margins.GrossMargin, wantMargin)
	}
	avgMargin := 0.5
	if closeEnough(all.Margins.GrossMargin.Amount, avgMargin) {
		t.Error("all-period margin equals the naive average of period margins; it must be total profit / total revenue instead")
	}
}
