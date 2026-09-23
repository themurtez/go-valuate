package labor_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/labor"
	"github.com/themurtez/go-valuate/accounting/labor/fixtures"
)

func TestProductivity_RevenuePerEmployeeAndFTE(t *testing.T) {
	workers, records := fixtures.StableServiceBusiness()
	in := labor.Input{
		Periods: fixtures.TwoMonthPeriods(), Workers: workers, PayrollRecords: records,
		BusinessMetrics: []labor.BusinessMetrics{
			{Period: "2025-06", Revenue: labor.AvailableValue(300000), GrossProfit: labor.AvailableValue(150000), EBITDA: labor.AvailableValue(60000)},
		},
	}
	r := labor.Calculate(in, labor.DefaultPolicy())
	for _, p := range r.Periods {
		if p.Period.Period != "2025-06" {
			continue
		}
		if !p.Productivity.RevenuePerEmployee.Available {
			t.Fatal("expected RevenuePerEmployee available")
		}
		want := 300000.0 / 3
		if p.Productivity.RevenuePerEmployee.Amount != want {
			t.Errorf("RevenuePerEmployee = %v, want %v", p.Productivity.RevenuePerEmployee.Amount, want)
		}
		if !p.Productivity.GrossProfitPerEmployee.Available {
			t.Error("expected GrossProfitPerEmployee available")
		}
		if !p.Productivity.EBITDAPerEmployee.Available {
			t.Error("expected EBITDAPerEmployee available")
		}
	}
}

func TestProductivity_LaborCostPercentRevenue(t *testing.T) {
	_, records := fixtures.StableServiceBusiness()
	in := labor.Input{
		Periods: fixtures.TwoMonthPeriods(), PayrollRecords: records,
		BusinessMetrics: []labor.BusinessMetrics{{Period: "2025-06", Revenue: labor.AvailableValue(100000)}},
	}
	r := labor.Calculate(in, labor.DefaultPolicy())
	for _, p := range r.Periods {
		if p.Period.Period != "2025-06" {
			continue
		}
		if !p.Productivity.LaborCostPercentRevenue.Available {
			t.Fatal("expected LaborCostPercentRevenue available")
		}
		want := p.LaborCostBridge.TotalLaborCost / 100000
		if p.Productivity.LaborCostPercentRevenue.Amount != want {
			t.Errorf("LaborCostPercentRevenue = %v, want %v", p.Productivity.LaborCostPercentRevenue.Amount, want)
		}
	}
}

func TestProductivity_ZeroRevenueUnavailableNotInf(t *testing.T) {
	records, metrics := fixtures.ZeroRevenueBusiness()
	in := labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: records, BusinessMetrics: metrics}
	r := labor.Calculate(in, labor.DefaultPolicy())
	for _, p := range r.Periods {
		if p.Period.Period != "2025-06" {
			continue
		}
		if p.Productivity.RevenuePerEmployee.Available {
			t.Error("expected RevenuePerEmployee unavailable, not computed against zero revenue")
		}
		if p.Productivity.LaborCostPercentRevenue.Available {
			t.Error("expected LaborCostPercentRevenue unavailable when revenue is zero")
		}
	}
}

func TestProductivity_UnavailableWithoutMetrics(t *testing.T) {
	_, records := fixtures.StableServiceBusiness()
	in := labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: records}
	r := labor.Calculate(in, labor.DefaultPolicy())
	for _, p := range r.Periods {
		if p.Productivity.RevenuePerEmployee.Available || p.Productivity.LaborCostPercentRevenue.Available {
			t.Error("expected productivity ratios unavailable without BusinessMetrics")
		}
	}
	if r.Coverage.FinancialMetricsAvailable {
		t.Error("expected FinancialMetricsAvailable false without BusinessMetrics")
	}
}

func TestProductivity_PerFTERequiresFTE(t *testing.T) {
	_, records := fixtures.StableServiceBusiness()
	in := labor.Input{
		Periods: fixtures.TwoMonthPeriods(), PayrollRecords: records,
		BusinessMetrics: []labor.BusinessMetrics{{Period: "2025-06", Revenue: labor.AvailableValue(100000)}},
	}
	r := labor.Calculate(in, labor.DefaultPolicy()) // no StandardFullTimeHoursPerPeriod, no hours -> FTE unavailable
	for _, p := range r.Periods {
		if p.Period.Period != "2025-06" {
			continue
		}
		if p.Productivity.RevenuePerFTE.Available {
			t.Error("expected RevenuePerFTE unavailable without FTE")
		}
	}
}
