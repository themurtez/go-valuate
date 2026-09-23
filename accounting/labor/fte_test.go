package labor_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/labor"
	"github.com/themurtez/go-valuate/accounting/labor/fixtures"
)

func TestFTE_HoursBased(t *testing.T) {
	records := fixtures.FTECalculationCase()
	in := labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: records}
	policy := labor.DefaultPolicy()
	policy.StandardFullTimeHoursPerPeriod = 173.33

	r := labor.Calculate(in, policy)
	for _, p := range r.Periods {
		if p.Period.Period != "2025-06" {
			continue
		}
		if !p.FTE.Available {
			t.Fatal("expected FTE available with hours and standard hours supplied")
		}
		if p.FTE.Method != labor.FTEMethodHoursBased {
			t.Errorf("Method = %v, want HOURS_BASED", p.FTE.Method)
		}
		want := 1.5 // (173.33+86.67)/173.33
		if diff := p.FTE.FTE - want; diff > 0.01 || diff < -0.01 {
			t.Errorf("FTE = %v, want ~%v", p.FTE.FTE, want)
		}
	}
}

func TestFTE_CallerSuppliedTakesPrecedence(t *testing.T) {
	records := fixtures.FTECalculationCase()
	in := labor.Input{
		Periods:        fixtures.TwoMonthPeriods(),
		PayrollRecords: records,
		WorkerPeriodFTEs: []labor.WorkerPeriodFTE{
			{WorkerID: "FCW1", Period: "2025-06", FTE: 1.0},
			{WorkerID: "FCW2", Period: "2025-06", FTE: 1.0},
		},
	}
	policy := labor.DefaultPolicy()
	policy.StandardFullTimeHoursPerPeriod = 173.33

	r := labor.Calculate(in, policy)
	for _, p := range r.Periods {
		if p.Period.Period != "2025-06" {
			continue
		}
		if p.FTE.Method != labor.FTEMethodCallerSupplied {
			t.Errorf("Method = %v, want CALLER_SUPPLIED", p.FTE.Method)
		}
		if p.FTE.FTE != 2.0 {
			t.Errorf("FTE = %v, want 2.0", p.FTE.FTE)
		}
	}
}

func TestFTE_UnavailableWithoutStandardHours(t *testing.T) {
	records := fixtures.FTECalculationCase()
	in := labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: records}
	r := labor.Calculate(in, labor.DefaultPolicy()) // no StandardFullTimeHoursPerPeriod set
	for _, p := range r.Periods {
		if p.FTE.Available {
			t.Error("expected FTE unavailable without StandardFullTimeHoursPerPeriod or caller-supplied FTE")
		}
	}
}

func TestFTE_UnavailableWithoutHours(t *testing.T) {
	records := fixtures.MissingHours()
	in := labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: records}
	policy := labor.DefaultPolicy()
	policy.StandardFullTimeHoursPerPeriod = 173.33
	r := labor.Calculate(in, policy)
	for _, p := range r.Periods {
		if p.Period.Period == "2025-06" && p.FTE.Available {
			t.Error("expected FTE unavailable when no payroll record in the period supplies hours")
		}
	}
}
