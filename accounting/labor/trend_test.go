package labor_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/labor"
	"github.com/themurtez/go-valuate/accounting/labor/fixtures"
)

func TestTrend_LaborCostGrowthOutpacesRevenue(t *testing.T) {
	records, metrics := fixtures.LaborGrowthOutpacingRevenue()
	in := labor.Input{Periods: fixtures.ThreeMonthPeriods(), PayrollRecords: records, BusinessMetrics: metrics}
	r := labor.Calculate(in, labor.DefaultPolicy())

	if !r.Trend.LaborCostGrowth.Available || !r.Trend.RevenueGrowth.Available {
		t.Fatal("expected both growth series available with 3 periods")
	}
	if r.Trend.LaborCostGrowth.Amount <= r.Trend.RevenueGrowth.Amount {
		t.Error("expected labor cost growth to exceed revenue growth in this fixture")
	}

	found := false
	for _, f := range r.Flags {
		if f.Code == labor.FlagLaborCostGrowthOutpacesRevenue {
			found = true
		}
	}
	if !found {
		t.Error("expected FlagLaborCostGrowthOutpacesRevenue to trigger")
	}
}

func TestTrend_ProportionalGrowthNoFlag(t *testing.T) {
	records, metrics := fixtures.GrowingRevenueProportionalLabor()
	in := labor.Input{Periods: fixtures.ThreeMonthPeriods(), PayrollRecords: records, BusinessMetrics: metrics}
	r := labor.Calculate(in, labor.DefaultPolicy())

	for _, f := range r.Flags {
		if f.Code == labor.FlagLaborCostGrowthOutpacesRevenue {
			t.Error("did not expect FlagLaborCostGrowthOutpacesRevenue for proportional growth")
		}
	}
}

func TestTrend_HeadcountGrowthWithRevenueDecline(t *testing.T) {
	workers, metrics := fixtures.RevenueDeclineHeadcountGrowth()
	periods := []labor.PeriodInfo{
		{Period: "2025-05", StartDate: pdate("2025-05-01"), EndDate: pdate("2025-05-31")},
		{Period: "2025-06", StartDate: pdate("2025-06-01"), EndDate: pdate("2025-06-30")},
	}
	in := labor.Input{Periods: periods, Workers: workers, BusinessMetrics: metrics}
	r := labor.Calculate(in, labor.DefaultPolicy())

	found := false
	for _, f := range r.Flags {
		if f.Code == labor.FlagHeadcountGrowthWithRevenueDecline {
			found = true
		}
	}
	if !found {
		t.Error("expected FlagHeadcountGrowthWithRevenueDecline to trigger")
	}
}

func TestTrend_HeadcountVsRevenueSignalDisableable(t *testing.T) {
	workers, metrics := fixtures.RevenueDeclineHeadcountGrowth()
	periods := []labor.PeriodInfo{
		{Period: "2025-05", StartDate: pdate("2025-05-01"), EndDate: pdate("2025-05-31")},
		{Period: "2025-06", StartDate: pdate("2025-06-01"), EndDate: pdate("2025-06-30")},
	}
	in := labor.Input{Periods: periods, Workers: workers, BusinessMetrics: metrics}
	policy := labor.DefaultPolicy()
	policy.DisableHeadcountVsRevenueSignal = true
	r := labor.Calculate(in, policy)

	for _, f := range r.Flags {
		if f.Code == labor.FlagHeadcountGrowthWithRevenueDecline {
			t.Error("did not expect FlagHeadcountGrowthWithRevenueDecline when disabled via policy")
		}
	}
}

func TestTrend_UnavailableWithFewerThanMinimumPeriods(t *testing.T) {
	_, records := fixtures.StableServiceBusiness()
	singlePeriod := []labor.PeriodInfo{{Period: "2025-06", StartDate: pdate("2025-06-01"), EndDate: pdate("2025-06-30")}}
	in := labor.Input{Periods: singlePeriod, PayrollRecords: records}
	r := labor.Calculate(in, labor.DefaultPolicy())

	if r.Trend.TotalLaborCost.Available {
		t.Error("expected TotalLaborCost trend unavailable with only 1 period (minimum is 2)")
	}
}
