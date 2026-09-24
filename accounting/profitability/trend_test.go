package profitability_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/profitability"
	"github.com/themurtez/go-valuate/accounting/profitability/fixtures"
)

func findTrend(trends []profitability.EntityTrend, entityID string) (profitability.EntityTrend, bool) {
	for _, tr := range trends {
		if tr.EntityID == entityID {
			return tr, true
		}
	}
	return profitability.EntityTrend{}, false
}

func TestTrend_MarginCompressionFlag(t *testing.T) {
	entities := []profitability.Entity{{Dimension: profitability.DimensionCustomer, EntityID: "C1", Active: true}}
	facts := []profitability.Fact{
		// Period 1: 1000 revenue, 100 cost -> gross margin 0.90.
		{FactID: "P1-REV", Period: "2025-01", Component: profitability.ComponentGrossRevenue, Amount: 1000, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"CUSTOMER", "C1"})},
		{FactID: "P1-COST", Period: "2025-01", Component: profitability.ComponentDirectOther, Amount: 100, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"CUSTOMER", "C1"})},
		// Period 2: 1000 revenue, 500 cost -> gross margin 0.50 (40-point drop).
		{FactID: "P2-REV", Period: "2025-02", Component: profitability.ComponentGrossRevenue, Amount: 1000, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"CUSTOMER", "C1"})},
		{FactID: "P2-COST", Period: "2025-02", Component: profitability.ComponentDirectOther, Amount: 500, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"CUSTOMER", "C1"})},
	}
	r := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), Entities: entities, Facts: facts}, profitability.DefaultPolicy())

	tr, ok := findTrend(r.CustomerView.Trends, "C1")
	if !ok {
		t.Fatal("expected a trend for C1")
	}
	if !tr.GrossMarginChange.Available {
		t.Fatal("expected GrossMarginChange available")
	}
	if tr.GrossMarginChange.AbsoluteChange >= 0 {
		t.Errorf("expected a margin decline, got %v", tr.GrossMarginChange.AbsoluteChange)
	}
	if !hasFlag(r.CustomerView.Flags, profitability.FlagMarginCompression) {
		t.Error("expected FlagMarginCompression")
	}
}

func TestTrend_RevenueGrowthWithContributionDecline(t *testing.T) {
	entities := []profitability.Entity{{Dimension: profitability.DimensionCustomer, EntityID: "C1", Active: true}}
	facts := []profitability.Fact{
		{FactID: "P1-REV", Period: "2025-01", Component: profitability.ComponentGrossRevenue, Amount: 1000, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"CUSTOMER", "C1"})},
		{FactID: "P1-COST", Period: "2025-01", Component: profitability.ComponentDirectOther, Amount: 100, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"CUSTOMER", "C1"})},
		// Revenue grows to 2000 but cost grows to 1950 -> contribution falls from 900 to 50.
		{FactID: "P2-REV", Period: "2025-02", Component: profitability.ComponentGrossRevenue, Amount: 2000, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"CUSTOMER", "C1"})},
		{FactID: "P2-COST", Period: "2025-02", Component: profitability.ComponentDirectOther, Amount: 1950, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"CUSTOMER", "C1"})},
	}
	r := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), Entities: entities, Facts: facts}, profitability.DefaultPolicy())

	if !hasFlag(r.CustomerView.Flags, profitability.FlagRevenueGrowthWithContributionDecline) {
		t.Error("expected FlagRevenueGrowthWithContributionDecline")
	}
}

func TestTrend_CostShareIncreasing(t *testing.T) {
	entities := []profitability.Entity{{Dimension: profitability.DimensionJob, EntityID: "J1", Active: true}}
	facts := []profitability.Fact{
		{FactID: "P1-REV", Period: "2025-01", Component: profitability.ComponentGrossRevenue, Amount: 1000, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"JOB", "J1"})},
		{FactID: "P1-LAB", Period: "2025-01", Component: profitability.ComponentDirectLabor, Amount: 100, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"JOB", "J1"})},
		{FactID: "P2-REV", Period: "2025-02", Component: profitability.ComponentGrossRevenue, Amount: 1000, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"JOB", "J1"})},
		{FactID: "P2-LAB", Period: "2025-02", Component: profitability.ComponentDirectLabor, Amount: 400, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"JOB", "J1"})},
	}
	r := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), Entities: entities, Facts: facts}, profitability.DefaultPolicy())

	if !hasFlag(r.JobView.Flags, profitability.FlagDirectLaborShareIncreasing) {
		t.Error("expected FlagDirectLaborShareIncreasing")
	}
}

func TestMarginLeakage_HighReturnAndDiscountFlags(t *testing.T) {
	entities, facts := fixtures.HighReturnsDiscountsCustomer()
	r := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), Entities: entities, Facts: facts}, profitability.DefaultPolicy())

	if !hasFlag(r.CustomerView.Flags, profitability.FlagHighReturnRate) {
		t.Error("expected FlagHighReturnRate")
	}
	if !hasFlag(r.CustomerView.Flags, profitability.FlagHighDiscountRate) {
		t.Error("expected FlagHighDiscountRate")
	}
}
