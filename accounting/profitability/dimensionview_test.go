package profitability_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/profitability"
	"github.com/themurtez/go-valuate/accounting/profitability/fixtures"
)

func TestDimensionView_NotApplicable(t *testing.T) {
	entities, facts := fixtures.LaborHeavyJob()
	policy := fixtures.DimensionNotApplicablePolicy()
	r := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), Entities: entities, Facts: facts}, policy)

	if r.JobView.Status != profitability.ViewStatusNotApplicable {
		t.Errorf("JobView.Status = %s, want NOT_APPLICABLE", r.JobView.Status)
	}
	if len(r.JobView.AllPeriod) != 0 {
		t.Error("a NOT_APPLICABLE view must not compute entity results")
	}
}

func TestDimensionView_UnavailableWhenNoReferences(t *testing.T) {
	r := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods()}, profitability.DefaultPolicy())
	for _, view := range []profitability.DimensionView{r.CustomerView, r.JobView, r.ProductView} {
		if view.Status != profitability.ViewStatusUnavailable {
			t.Errorf("dimension %s status = %s, want UNAVAILABLE when nothing references it", view.Dimension, view.Status)
		}
	}
}

func TestDimensionView_StrictCoverageReportsGaps(t *testing.T) {
	entities, facts := fixtures.PartialAttributionAcrossTwoProducts()
	policy := profitability.DefaultPolicy()
	policy.StrictAttributionCoverage = true
	policy.MinRevenueAttributionCoverage = 0.99
	r := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), Entities: entities, Facts: facts}, policy)

	if r.ProductView.Status != profitability.ViewStatusAvailableWithGaps {
		t.Errorf("ProductView.Status = %s, want AVAILABLE_WITH_GAPS under strict coverage with an 80%% actual coverage", r.ProductView.Status)
	}
}

func TestDimensionView_NonStrictCoverageStillComputes(t *testing.T) {
	entities, facts := fixtures.PartialAttributionAcrossTwoProducts()
	policy := profitability.DefaultPolicy()
	policy.MinRevenueAttributionCoverage = 0.99
	// StrictAttributionCoverage left false (default): gaps never block
	// computation, task section 15.
	r := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), Entities: entities, Facts: facts}, policy)

	if r.ProductView.Status != profitability.ViewStatusAvailable {
		t.Errorf("ProductView.Status = %s, want AVAILABLE (non-strict mode never downgrades status)", r.ProductView.Status)
	}
	if len(r.ProductView.AllPeriod) == 0 {
		t.Error("expected computation to proceed despite the coverage gap")
	}
}

func TestRankings_TopByRevenueAndNegativeContribution(t *testing.T) {
	entities, facts := fixtures.NegativeContributionCustomer()
	e2, f2 := fixtures.HighRevenueLowMarginCustomer()
	entities = append(entities, e2...)
	facts = append(facts, f2...)

	policy := profitability.DefaultPolicy()
	policy.NegativeContributionMateriality = profitability.MaterialityPolicy{AbsoluteAmount: 1}
	r := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), Entities: entities, Facts: facts}, policy)

	if len(r.CustomerView.Rankings.TopByNetRevenue) == 0 {
		t.Fatal("expected TopByNetRevenue entries")
	}
	if r.CustomerView.Rankings.TopByNetRevenue[0].EntityID != "CUST-HRLM" {
		t.Errorf("top revenue entity = %s, want CUST-HRLM", r.CustomerView.Rankings.TopByNetRevenue[0].EntityID)
	}

	found := false
	for _, n := range r.CustomerView.Rankings.NegativeContribution {
		if n.EntityID == "CUST-NEG" {
			found = true
		}
	}
	if !found {
		t.Error("expected CUST-NEG in NegativeContribution ranking")
	}
}

func TestRankings_MinimumRevenueForMarginRankingExcludesTinyEntity(t *testing.T) {
	entities := []profitability.Entity{
		{Dimension: profitability.DimensionCustomer, EntityID: "TINY", Active: true},
		{Dimension: profitability.DimensionCustomer, EntityID: "BIG", Active: true},
	}
	facts := []profitability.Fact{
		// TINY: $1 revenue, $0 cost => 100% margin (would dominate a naive ranking).
		{FactID: "T-REV", Period: "2025-01", Component: profitability.ComponentGrossRevenue, Amount: 1, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"CUSTOMER", "TINY"})},
		// BIG: $100,000 revenue, 50% margin.
		{FactID: "B-REV", Period: "2025-01", Component: profitability.ComponentGrossRevenue, Amount: 100000, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"CUSTOMER", "BIG"})},
		{FactID: "B-COST", Period: "2025-01", Component: profitability.ComponentDirectOther, Amount: 50000, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"CUSTOMER", "BIG"})},
	}
	policy := profitability.DefaultPolicy()
	policy.MinimumRevenueForMarginRanking = 1000
	r := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), Entities: entities, Facts: facts}, policy)

	for _, m := range r.CustomerView.MarginRanking {
		if m.EntityID == "TINY" {
			t.Error("TINY should be excluded from margin ranking below MinimumRevenueForMarginRanking")
		}
	}
	if len(r.CustomerView.MarginRanking) != 1 || r.CustomerView.MarginRanking[0].EntityID != "BIG" {
		t.Errorf("expected only BIG in margin ranking, got %+v", r.CustomerView.MarginRanking)
	}
}

func TestGroupSummaries_RollUpByGroupAndCategory(t *testing.T) {
	entities, facts := fixtures.GroupCategorySummaryScenario()
	r := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), Entities: entities, Facts: facts}, profitability.DefaultPolicy())

	var enterprise *profitability.GroupSummary
	for i := range r.CustomerView.GroupSummaries {
		if r.CustomerView.GroupSummaries[i].GroupKey == "Enterprise" {
			enterprise = &r.CustomerView.GroupSummaries[i]
		}
	}
	if enterprise == nil {
		t.Fatal("expected an Enterprise GroupSummary")
	}
	if enterprise.EntityCount != 2 {
		t.Errorf("Enterprise EntityCount = %d, want 2", enterprise.EntityCount)
	}
	if !closeEnough(enterprise.NetRevenue, 30000) {
		t.Errorf("Enterprise NetRevenue = %v, want 30000", enterprise.NetRevenue)
	}

	var retail *profitability.GroupSummary
	for i := range r.CustomerView.CategorySummaries {
		if r.CustomerView.CategorySummaries[i].GroupKey == "Retail" {
			retail = &r.CustomerView.CategorySummaries[i]
		}
	}
	if retail == nil {
		t.Fatal("expected a Retail CategorySummary")
	}
	if retail.EntityCount != 2 {
		t.Errorf("Retail EntityCount = %d, want 2", retail.EntityCount)
	}
}

func TestControlReconciliation_Match(t *testing.T) {
	facts, controls := fixtures.ControlReconciliationMatch()
	r := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), Facts: facts, Controls: controls}, profitability.DefaultPolicy())

	found := false
	for _, rec := range r.ControlReconciliation {
		if rec.Period != "2025-01" {
			continue
		}
		found = true
		for _, c := range rec.Components {
			if c.Label == "net_revenue" && !c.Reconciled {
				t.Errorf("net_revenue should reconcile: %+v", c)
			}
		}
	}
	if !found {
		t.Fatal("expected a 2025-01 ControlReconciliation entry")
	}
}

func TestControlReconciliation_Mismatch(t *testing.T) {
	facts, controls := fixtures.ControlReconciliationMismatch()
	policy := profitability.DefaultPolicy()
	policy.ControlMateriality = profitability.MaterialityPolicy{AbsoluteAmount: 1}
	r := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), Facts: facts, Controls: controls}, policy)

	mismatchFound := false
	for _, rec := range r.ControlReconciliation {
		for _, c := range rec.Components {
			if c.Label == "net_revenue" && c.Difference.Available && !c.Reconciled {
				mismatchFound = true
			}
		}
	}
	if !mismatchFound {
		t.Fatal("expected a net_revenue reconciliation mismatch")
	}
	if !hasFlag(r.Flags, profitability.FlagControlTotalMismatch) {
		t.Error("expected FlagControlTotalMismatch")
	}
}

func hasFlag(flags []profitability.Flag, code profitability.FlagCode) bool {
	for _, f := range flags {
		if f.Code == code {
			return true
		}
	}
	return false
}
