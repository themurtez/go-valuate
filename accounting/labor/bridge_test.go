package labor_test

import (
	"testing"
	"time"

	"github.com/themurtez/go-valuate/accounting/labor"
	"github.com/themurtez/go-valuate/accounting/labor/fixtures"
)

func TestBridge_Identity(t *testing.T) {
	_, records := fixtures.StableServiceBusiness()
	in := labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: records}
	r := labor.Calculate(in, labor.DefaultPolicy())

	for _, p := range r.Periods {
		b := p.LaborCostBridge
		wantGross := b.RegularPay + b.OvertimePay + b.BonusPay + b.CommissionPay + b.OtherPay
		if b.GrossEmployeePay != wantGross {
			t.Errorf("period %s: GrossEmployeePay = %v, want %v", p.Period.Period, b.GrossEmployeePay, wantGross)
		}
		wantBurden := b.EmployerTaxes + b.Benefits + b.OtherEmployerCosts
		if b.EmployerBurden != wantBurden {
			t.Errorf("period %s: EmployerBurden = %v, want %v", p.Period.Period, b.EmployerBurden, wantBurden)
		}
		wantEmployeeCost := b.GrossEmployeePay + b.EmployerBurden
		if b.TotalEmployeeCost != wantEmployeeCost {
			t.Errorf("period %s: TotalEmployeeCost = %v, want %v", p.Period.Period, b.TotalEmployeeCost, wantEmployeeCost)
		}
		wantTotal := b.TotalEmployeeCost + b.ContractorLabor
		if b.TotalLaborCost != wantTotal {
			t.Errorf("period %s: TotalLaborCost = %v, want %v", p.Period.Period, b.TotalLaborCost, wantTotal)
		}
	}
}

func TestBridge_WithContractors(t *testing.T) {
	payroll, contractors := fixtures.HighContractorShare()
	in := labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: payroll, ContractorRecords: contractors}
	r := labor.Calculate(in, labor.DefaultPolicy())

	var found bool
	for _, p := range r.Periods {
		if p.Period.Period != "2025-06" {
			continue
		}
		found = true
		if p.LaborCostBridge.ContractorLabor != 4000 {
			t.Errorf("ContractorLabor = %v, want 4000", p.LaborCostBridge.ContractorLabor)
		}
		if p.LaborCostBridge.TotalLaborCost != p.LaborCostBridge.TotalEmployeeCost+4000 {
			t.Error("TotalLaborCost must include contractor labor")
		}
	}
	if !found {
		t.Fatal("expected 2025-06 period in result")
	}
}

func TestBridge_BurdenRateUnavailableWhenGrossZero(t *testing.T) {
	in := labor.Input{
		Periods: fixtures.TwoMonthPeriods(),
		PayrollRecords: []labor.PayrollRecord{
			{ID: "Z1", WorkerID: "W1", Period: "2025-06", PayDate: pdate("2025-06-30"), Currency: "USD"},
		},
	}
	r := labor.Calculate(in, labor.DefaultPolicy())
	for _, p := range r.Periods {
		if p.Period.Period == "2025-06" && p.LaborCostBridge.BurdenRate.Available {
			t.Error("expected BurdenRate unavailable when GrossEmployeePay is zero")
		}
	}
}

func TestBridge_EmployeeDeductionsNeverAutoincluded(t *testing.T) {
	// PayrollRecord has no "employee deduction" field at all — this test
	// documents that TotalEmployeeCost is exactly Gross + employer burden,
	// with no hidden deduction subtracted or added.
	_, records := fixtures.StableServiceBusiness()
	in := labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: records}
	r := labor.Calculate(in, labor.DefaultPolicy())
	for _, p := range r.Periods {
		b := p.LaborCostBridge
		if b.TotalEmployeeCost != b.GrossEmployeePay+b.EmployerTaxes+b.Benefits+b.OtherEmployerCosts {
			t.Error("TotalEmployeeCost must equal gross pay plus only the three employer-cost fields")
		}
	}
}

func pdate(s string) time.Time {
	tm, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return tm
}
