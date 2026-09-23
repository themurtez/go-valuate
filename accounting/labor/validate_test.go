package labor_test

import (
	"math"
	"testing"

	"github.com/themurtez/go-valuate/accounting/labor"
	"github.com/themurtez/go-valuate/accounting/labor/fixtures"
)

func TestValidate_InvalidPeriod(t *testing.T) {
	in := labor.Input{Periods: []labor.PeriodInfo{{Period: "bad", StartDate: pdate("2025-06-30"), EndDate: pdate("2025-06-01")}}}
	r := labor.Calculate(in, labor.DefaultPolicy())
	found := false
	for _, iss := range r.Issues {
		if iss.Code == labor.IssueInvalidPeriod {
			found = true
		}
	}
	if !found {
		t.Error("expected IssueInvalidPeriod for end date before start date")
	}
	if len(r.Periods) != 0 {
		t.Error("expected the invalid period excluded from Periods")
	}
}

func TestValidate_DuplicatePeriod(t *testing.T) {
	dup := []labor.PeriodInfo{
		{Period: "2025-06", StartDate: pdate("2025-06-01"), EndDate: pdate("2025-06-30")},
		{Period: "2025-06", StartDate: pdate("2025-06-01"), EndDate: pdate("2025-06-30")},
	}
	r := labor.Calculate(labor.Input{Periods: dup}, labor.DefaultPolicy())
	found := false
	for _, iss := range r.Issues {
		if iss.Code == labor.IssueDuplicatePeriod {
			found = true
		}
	}
	if !found {
		t.Error("expected IssueDuplicatePeriod")
	}
	if len(r.Periods) != 1 {
		t.Errorf("expected only first occurrence kept, got %d periods", len(r.Periods))
	}
}

func TestValidate_DuplicateWorker(t *testing.T) {
	workers := []labor.Worker{
		{WorkerID: "DW", WorkerType: labor.WorkerTypeEmployee, Department: "A"},
		{WorkerID: "DW", WorkerType: labor.WorkerTypeEmployee, Department: "B"},
	}
	r := labor.Calculate(labor.Input{Periods: fixtures.TwoMonthPeriods(), Workers: workers}, labor.DefaultPolicy())
	found := false
	for _, iss := range r.Issues {
		if iss.Code == labor.IssueDuplicateWorker {
			found = true
		}
	}
	if !found {
		t.Error("expected IssueDuplicateWorker")
	}
}

func TestValidate_DuplicatePayrollRecord(t *testing.T) {
	records := []labor.PayrollRecord{
		{ID: "DUP", WorkerID: "W1", Period: "2025-06", PayDate: pdate("2025-06-30"), RegularPay: 1000, Currency: "USD"},
		{ID: "DUP", WorkerID: "W1", Period: "2025-06", PayDate: pdate("2025-06-30"), RegularPay: 2000, Currency: "USD"},
	}
	r := labor.Calculate(labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: records}, labor.DefaultPolicy())
	found := false
	for _, iss := range r.Issues {
		if iss.Code == labor.IssueDuplicatePayrollRecord {
			found = true
		}
	}
	if !found {
		t.Error("expected IssueDuplicatePayrollRecord")
	}
	for _, p := range r.Periods {
		if p.Period.Period == "2025-06" && p.LaborCostBridge.RegularPay != 1000 {
			t.Errorf("expected first occurrence (1000) kept, got %v", p.LaborCostBridge.RegularPay)
		}
	}
}

func TestValidate_UnknownWorkerOnlyWhenRosterSupplied(t *testing.T) {
	records := []labor.PayrollRecord{
		{ID: "UW-1", WorkerID: "GHOST", Period: "2025-06", PayDate: pdate("2025-06-30"), RegularPay: 1000, Currency: "USD"},
	}

	// No roster supplied at all -> no IssueUnknownWorker.
	r1 := labor.Calculate(labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: records}, labor.DefaultPolicy())
	for _, iss := range r1.Issues {
		if iss.Code == labor.IssueUnknownWorker {
			t.Error("did not expect IssueUnknownWorker when no roster was supplied at all")
		}
	}

	// Roster supplied but missing this worker -> IssueUnknownWorker.
	workers := []labor.Worker{{WorkerID: "SOMEONE_ELSE"}}
	r2 := labor.Calculate(labor.Input{Periods: fixtures.TwoMonthPeriods(), Workers: workers, PayrollRecords: records}, labor.DefaultPolicy())
	found := false
	for _, iss := range r2.Issues {
		if iss.Code == labor.IssueUnknownWorker {
			found = true
		}
	}
	if !found {
		t.Error("expected IssueUnknownWorker when a roster was supplied but does not include this worker")
	}
}

func TestValidate_NegativeAmountRejectedUnlessAdjustment(t *testing.T) {
	normal := []labor.PayrollRecord{
		{ID: "NEG-1", WorkerID: "W1", Period: "2025-06", PayDate: pdate("2025-06-30"), RegularPay: -500, Currency: "USD"},
	}
	r1 := labor.Calculate(labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: normal}, labor.DefaultPolicy())
	found := false
	for _, iss := range r1.Issues {
		if iss.Code == labor.IssueNegativeAmount {
			found = true
		}
	}
	if !found {
		t.Error("expected IssueNegativeAmount for a negative amount on a NORMAL record")
	}

	adjustment := []labor.PayrollRecord{
		{ID: "ADJ-1", WorkerID: "W1", Period: "2025-06", PayDate: pdate("2025-06-30"), RegularPay: -500, Currency: "USD", RecordType: labor.RecordTypeAdjustment},
	}
	r2 := labor.Calculate(labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: adjustment}, labor.DefaultPolicy())
	for _, iss := range r2.Issues {
		if iss.Code == labor.IssueNegativeAmount {
			t.Error("did not expect IssueNegativeAmount for an explicit ADJUSTMENT record type")
		}
	}
	var junePay float64
	var foundJune bool
	for _, p := range r2.Periods {
		if p.Period.Period == "2025-06" {
			junePay = p.LaborCostBridge.RegularPay
			foundJune = true
		}
	}
	if !foundJune || junePay != -500 {
		t.Errorf("expected the adjustment's negative amount (-500) included in the June bridge, got found=%v amount=%v", foundJune, junePay)
	}
}

func TestValidate_NonFiniteExcluded(t *testing.T) {
	records := []labor.PayrollRecord{
		{ID: "NF-1", WorkerID: "W1", Period: "2025-06", PayDate: pdate("2025-06-30"), RegularPay: math.NaN(), Currency: "USD"},
	}
	r := labor.Calculate(labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: records}, labor.DefaultPolicy())
	found := false
	for _, iss := range r.Issues {
		if iss.Code == labor.IssueNonFiniteAmount {
			found = true
		}
	}
	if !found {
		t.Error("expected IssueNonFiniteAmount for a NaN RegularPay")
	}
	for _, p := range r.Periods {
		for _, id := range p.Provenance.PayrollRecordIDs {
			if id == "NF-1" {
				t.Error("expected the non-finite record excluded from provenance")
			}
		}
	}
}

func TestValidate_MixedCurrencyIssue(t *testing.T) {
	records := fixtures.MixedCurrencyInvalid()
	r := labor.Calculate(labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: records}, labor.DefaultPolicy())
	found := false
	for _, iss := range r.Issues {
		if iss.Code == labor.IssueMixedCurrency {
			found = true
		}
	}
	if !found {
		t.Error("expected IssueMixedCurrency")
	}
}

func TestValidate_InvalidPolicyThreshold(t *testing.T) {
	policy := labor.DefaultPolicy()
	policy.PostTerminationPayGraceDays = -5
	r := labor.Calculate(labor.Input{Periods: fixtures.TwoMonthPeriods()}, policy)
	found := false
	for _, iss := range r.Issues {
		if iss.Code == labor.IssueInvalidPolicy {
			found = true
		}
	}
	if !found {
		t.Error("expected IssueInvalidPolicy for a negative grace-days value")
	}
}

func TestValidate_InvalidWorkerDates(t *testing.T) {
	workers := []labor.Worker{
		{WorkerID: "BAD-DATES", HireDate: pptr("2025-06-15"), TerminationDate: pptr("2025-01-01")},
	}
	r := labor.Calculate(labor.Input{Periods: fixtures.TwoMonthPeriods(), Workers: workers}, labor.DefaultPolicy())
	found := false
	for _, iss := range r.Issues {
		if iss.Code == labor.IssueInvalidWorkerDates {
			found = true
		}
	}
	if !found {
		t.Error("expected IssueInvalidWorkerDates for termination before hire")
	}
}
