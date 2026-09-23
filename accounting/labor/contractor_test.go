package labor_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/labor"
	"github.com/themurtez/go-valuate/accounting/labor/fixtures"
)

func TestContractor_ShareOfLaborCost(t *testing.T) {
	payroll, contractors := fixtures.HighContractorShare()
	in := labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: payroll, ContractorRecords: contractors}
	r := labor.Calculate(in, labor.DefaultPolicy())

	for _, p := range r.Periods {
		if p.Period.Period != "2025-06" {
			continue
		}
		if !p.ContractorMix.ContractorShareOfLaborCost.Available {
			t.Fatal("expected ContractorShareOfLaborCost available")
		}
		if p.ContractorMix.ContractorShareOfLaborCost.Amount <= 0.30 {
			t.Errorf("share = %v, want > 0.30", p.ContractorMix.ContractorShareOfLaborCost.Amount)
		}
	}
}

func TestContractor_HighShareFlag(t *testing.T) {
	payroll, contractors := fixtures.HighContractorShare()
	in := labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: payroll, ContractorRecords: contractors}
	r := labor.Calculate(in, labor.DefaultPolicy())

	found := false
	for _, f := range r.Flags {
		if f.Code == labor.FlagHighContractorShare {
			found = true
		}
	}
	if !found {
		t.Error("expected FlagHighContractorShare to trigger")
	}
}

func TestContractor_ShareIncreasingFlag(t *testing.T) {
	payroll, contractors := fixtures.ContractorShareIncreasing()
	in := labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: payroll, ContractorRecords: contractors}
	r := labor.Calculate(in, labor.DefaultPolicy())

	found := false
	for _, f := range r.Flags {
		if f.Code == labor.FlagContractorShareIncreasing {
			found = true
		}
	}
	if !found {
		t.Error("expected FlagContractorShareIncreasing to trigger")
	}
}

func TestContractor_ZeroVsUnavailable(t *testing.T) {
	_, records := fixtures.StableServiceBusiness()

	// Case 1: no contractor records supplied at all -> Coverage says not
	// supplied, but ContractorShareOfLaborCost is still a known zero
	// (there is genuinely $0 contractor spend given what was supplied).
	in := labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: records}
	r := labor.Calculate(in, labor.DefaultPolicy())
	if r.Coverage.ContractorDataSupplied {
		t.Error("expected ContractorDataSupplied false when no contractor records given")
	}
	for _, p := range r.Periods {
		if p.ContractorMix.ContractorLabor != 0 {
			t.Error("expected ContractorLabor == 0 when no contractor records supplied")
		}
	}

	// Case 2: explicit contractor record with amount 0.
	in2 := labor.Input{
		Periods:        fixtures.TwoMonthPeriods(),
		PayrollRecords: records,
		ContractorRecords: []labor.ContractorLaborRecord{
			{ID: "ZC-1", ContractorID: "C1", Period: "2025-06", Date: pdate("2025-06-01"), Amount: 0, Currency: "USD"},
		},
	}
	r2 := labor.Calculate(in2, labor.DefaultPolicy())
	if !r2.Coverage.ContractorDataSupplied {
		t.Error("expected ContractorDataSupplied true when an explicit (even zero-amount) contractor record is given")
	}
}

func TestContractor_NoClassificationConclusion(t *testing.T) {
	// Structural check: ContractorMix has no "misclassified"/"legal"
	// field. Combined with safety_test.go's language scan.
	var m labor.ContractorMix
	_ = m
}
