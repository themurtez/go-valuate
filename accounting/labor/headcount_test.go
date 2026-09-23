package labor_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/labor"
	"github.com/themurtez/go-valuate/accounting/labor/fixtures"
)

func TestHeadcount_BeginningEndingAverage(t *testing.T) {
	workers, records := fixtures.StableServiceBusiness()
	in := labor.Input{Periods: fixtures.TwoMonthPeriods(), Workers: workers, PayrollRecords: records}
	r := labor.Calculate(in, labor.DefaultPolicy())

	for _, p := range r.Periods {
		if !p.Headcount.Available {
			t.Fatalf("period %s: expected Headcount available with a worker roster supplied", p.Period.Period)
		}
		if p.Headcount.BeginningHeadcount != 3 || p.Headcount.EndingHeadcount != 3 {
			t.Errorf("period %s: beginning=%d ending=%d, want 3/3", p.Period.Period, p.Headcount.BeginningHeadcount, p.Headcount.EndingHeadcount)
		}
		if p.Headcount.AverageHeadcount != 3 {
			t.Errorf("AverageHeadcount = %v, want 3", p.Headcount.AverageHeadcount)
		}
	}
}

func TestHeadcount_HiresAndDepartures(t *testing.T) {
	workers, metrics := fixtures.RevenueDeclineHeadcountGrowth()
	periods := []labor.PeriodInfo{
		{Period: "2025-05", StartDate: pdate("2025-05-01"), EndDate: pdate("2025-05-31")},
		{Period: "2025-06", StartDate: pdate("2025-06-01"), EndDate: pdate("2025-06-30")},
	}
	in := labor.Input{Periods: periods, Workers: workers, BusinessMetrics: metrics}
	r := labor.Calculate(in, labor.DefaultPolicy())

	var junePeriod *labor.PeriodSummary
	for i := range r.Periods {
		if r.Periods[i].Period.Period == "2025-06" {
			junePeriod = &r.Periods[i]
		}
	}
	if junePeriod == nil {
		t.Fatal("expected June period")
	}
	if junePeriod.WorkforceMovement.Hires != 1 {
		t.Errorf("Hires = %d, want 1 (RD-W3 hired 2025-06-05)", junePeriod.WorkforceMovement.Hires)
	}
}

func TestHeadcount_UnavailableWithoutRosterOrApproximation(t *testing.T) {
	_, records := fixtures.StableServiceBusiness()
	in := labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: records}
	r := labor.Calculate(in, labor.DefaultPolicy())
	for _, p := range r.Periods {
		if p.Headcount.Available {
			t.Error("expected Headcount unavailable when no roster is supplied and approximation is not opted into")
		}
	}
}

func TestHeadcount_PayrollActiveWorkerApproximation(t *testing.T) {
	_, records := fixtures.StableServiceBusiness()
	in := labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: records}
	policy := labor.DefaultPolicy()
	policy.PayrollActiveWorkerApproximation = true
	r := labor.Calculate(in, policy)
	for _, p := range r.Periods {
		if !p.Headcount.Available || !p.Headcount.Approximated {
			t.Errorf("period %s: expected approximated Headcount available", p.Period.Period)
		}
		if p.Headcount.EndingHeadcount != 3 {
			t.Errorf("period %s: approximated EndingHeadcount = %d, want 3", p.Period.Period, p.Headcount.EndingHeadcount)
		}
	}
}

func TestHeadcount_ZeroWorkforce(t *testing.T) {
	workers, records := fixtures.ZeroWorkforce()
	in := labor.Input{Periods: fixtures.TwoMonthPeriods(), Workers: workers, PayrollRecords: records}
	r := labor.Calculate(in, labor.DefaultPolicy())
	for _, p := range r.Periods {
		if p.Headcount.Available {
			t.Error("expected Headcount unavailable for zero workforce")
		}
		if p.LaborCostBridge.TotalLaborCost != 0 {
			t.Error("expected zero TotalLaborCost for zero workforce")
		}
	}
}

func TestHeadcount_TurnoverUnavailableWithoutDates(t *testing.T) {
	workers := []labor.Worker{
		{WorkerID: "NT1", WorkerType: labor.WorkerTypeEmployee, Active: true},
		{WorkerID: "NT2", WorkerType: labor.WorkerTypeEmployee, Active: true},
	}
	in := labor.Input{Periods: fixtures.TwoMonthPeriods(), Workers: workers}
	r := labor.Calculate(in, labor.DefaultPolicy())
	for _, p := range r.Periods {
		if p.WorkforceMovement.TurnoverRate.Available {
			t.Error("expected TurnoverRate unavailable when no hire/termination dates are supplied")
		}
	}
}
