package profitability_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/profitability"
	"github.com/themurtez/go-valuate/accounting/profitability/fixtures"
)

func TestDrivers_PerUnitMetrics(t *testing.T) {
	entities := []profitability.Entity{{Dimension: profitability.DimensionProduct, EntityID: "P1", Active: true}}
	facts := []profitability.Fact{
		{FactID: "F-REV", Period: "2025-01", Component: profitability.ComponentGrossRevenue, Amount: 1000, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"PRODUCT", "P1"})},
		{FactID: "F-MAT", Period: "2025-01", Component: profitability.ComponentDirectMaterial, Amount: 400, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"PRODUCT", "P1"})},
	}
	drivers := []profitability.DriverObservation{
		{ObservationID: "D1", Dimension: profitability.DimensionProduct, EntityID: "P1", Period: "2025-01", DriverKey: "units_sold", Value: 100, Unit: "units"},
	}
	policy := profitability.DefaultPolicy()
	policy.PrimaryDriverKey = map[profitability.Dimension]string{profitability.DimensionProduct: "units_sold"}

	r := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), Entities: entities, Facts: facts, Drivers: drivers}, policy)

	if len(r.ProductView.EntityPeriods) == 0 {
		t.Fatal("expected entity periods")
	}
	pu := r.ProductView.EntityPeriods[0].PerUnitMetrics
	if !pu.NetRevenuePerUnit.Available || !closeEnough(pu.NetRevenuePerUnit.Amount, 10) {
		t.Errorf("NetRevenuePerUnit = %+v, want 10", pu.NetRevenuePerUnit)
	}
	if !pu.GrossProfitPerUnit.Available || !closeEnough(pu.GrossProfitPerUnit.Amount, 6) {
		t.Errorf("GrossProfitPerUnit = %+v, want 6", pu.GrossProfitPerUnit)
	}
}

func TestDrivers_NoDenominatorUnavailable(t *testing.T) {
	entities := []profitability.Entity{{Dimension: profitability.DimensionProduct, EntityID: "P1", Active: true}}
	facts := []profitability.Fact{
		{FactID: "F-REV", Period: "2025-01", Component: profitability.ComponentGrossRevenue, Amount: 1000, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"PRODUCT", "P1"})},
	}
	policy := profitability.DefaultPolicy()
	policy.PrimaryDriverKey = map[profitability.Dimension]string{profitability.DimensionProduct: "units_sold"}
	r := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), Entities: entities, Facts: facts}, policy)

	pu := r.ProductView.EntityPeriods[0].PerUnitMetrics
	if pu.NetRevenuePerUnit.Available {
		t.Error("expected NetRevenuePerUnit unavailable with no driver observation supplied")
	}
}

func TestDrivers_DuplicateObservation(t *testing.T) {
	drivers := []profitability.DriverObservation{
		{ObservationID: "D1", Dimension: profitability.DimensionProduct, EntityID: "P1", Period: "2025-01", DriverKey: "units_sold", Value: 100},
		{ObservationID: "D1", Dimension: profitability.DimensionProduct, EntityID: "P1", Period: "2025-01", DriverKey: "units_sold", Value: 200},
	}
	r := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), Drivers: drivers}, profitability.DefaultPolicy())
	if !hasIssue(r.Issues, profitability.IssueDuplicateDriver) {
		t.Error("expected IssueDuplicateDriver")
	}
}

func TestDrivers_NegativeValueRejected(t *testing.T) {
	drivers := []profitability.DriverObservation{
		{ObservationID: "D1", Dimension: profitability.DimensionProduct, EntityID: "P1", Period: "2025-01", DriverKey: "units_sold", Value: -5},
	}
	r := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), Drivers: drivers}, profitability.DefaultPolicy())
	if !hasIssue(r.Issues, profitability.IssueInvalidDriver) {
		t.Error("expected IssueInvalidDriver for a negative value")
	}
}

func TestDirectAttribution_HelperShapesShare100Percent(t *testing.T) {
	a := profitability.DirectAttribution(profitability.DimensionCustomer, "C1")
	if a.Share != 1 || a.Dimension != profitability.DimensionCustomer || a.EntityID != "C1" {
		t.Errorf("unexpected DirectAttribution: %+v", a)
	}
}

func TestDirectAttributions_HelperBuildsMultipleDimensions(t *testing.T) {
	attrs := profitability.DirectAttributions([2]string{"CUSTOMER", "C1"}, [2]string{"JOB", "J1"})
	if len(attrs) != 2 {
		t.Fatalf("expected 2 attributions, got %d", len(attrs))
	}
	if attrs[0].Dimension != profitability.DimensionCustomer || attrs[1].Dimension != profitability.DimensionJob {
		t.Errorf("unexpected dimensions: %+v", attrs)
	}
}
