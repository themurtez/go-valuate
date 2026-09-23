package labor_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/labor"
	"github.com/themurtez/go-valuate/accounting/labor/fixtures"
)

func TestDuplicates_PossibleDuplicateDetected(t *testing.T) {
	records := fixtures.DuplicatePayrollRecords()
	in := labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: records}
	r := labor.Calculate(in, labor.DefaultPolicy())

	if len(r.DuplicateGroups) != 1 {
		t.Fatalf("expected 1 duplicate group, got %d", len(r.DuplicateGroups))
	}
	g := r.DuplicateGroups[0]
	if g.RecordIDA != "DUP-1" || g.RecordIDB != "DUP-2" {
		t.Errorf("unexpected duplicate pair: %+v", g)
	}

	found := false
	for _, f := range r.Flags {
		if f.Code == labor.FlagPossibleDuplicatePayrollRecord {
			found = true
		}
	}
	if !found {
		t.Error("expected FlagPossibleDuplicatePayrollRecord to trigger")
	}
}

func TestDuplicates_DifferentAmountsNotFlagged(t *testing.T) {
	records := []labor.PayrollRecord{
		{ID: "ND-1", WorkerID: "NDW1", Period: "2025-06", PayDate: pdate("2025-06-30"), RegularPay: 3000, Currency: "USD"},
		{ID: "ND-2", WorkerID: "NDW1", Period: "2025-06", PayDate: pdate("2025-06-30"), RegularPay: 3500, Currency: "USD"},
	}
	in := labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: records}
	r := labor.Calculate(in, labor.DefaultPolicy())
	if len(r.DuplicateGroups) != 0 {
		t.Errorf("expected no duplicate groups for differing amounts, got %d", len(r.DuplicateGroups))
	}
}

func TestDuplicates_ExactIDDuplicateIsIssueNotDuplicateGroup(t *testing.T) {
	records := []labor.PayrollRecord{
		{ID: "SAME-ID", WorkerID: "W1", Period: "2025-06", PayDate: pdate("2025-06-30"), RegularPay: 1000, Currency: "USD"},
		{ID: "SAME-ID", WorkerID: "W1", Period: "2025-06", PayDate: pdate("2025-06-30"), RegularPay: 1000, Currency: "USD"},
	}
	in := labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: records}
	r := labor.Calculate(in, labor.DefaultPolicy())

	found := false
	for _, iss := range r.Issues {
		if iss.Code == labor.IssueDuplicatePayrollRecord {
			found = true
		}
	}
	if !found {
		t.Error("expected IssueDuplicatePayrollRecord for exact ID duplicate")
	}
	// Only the first occurrence is used downstream, so there is only one
	// record in the economic-signature scan — no DuplicateGroup possible.
	if len(r.DuplicateGroups) != 0 {
		t.Error("expected no DuplicateGroups since the second row with the same ID was excluded entirely")
	}
}
