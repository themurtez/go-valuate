package ar_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/ar"
)

func TestReconcile_AgingBucketsBalance(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	receivables := []ar.Receivable{
		recv("R-1", "C1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), 1000, 1000, ar.StatusOpen),
		recv("R-2", "C2", mustDate(t, "2025-04-01"), mustDate(t, "2025-04-30"), 2000, 2000, ar.StatusOpen),
	}
	result := ar.Calculate(ar.Input{Receivables: receivables}, ar.Options{AsOfDate: asOf})

	if !result.AgingReconciliation.Balanced {
		t.Errorf("expected Balanced=true, got %+v", result.AgingReconciliation)
	}
	if result.AgingReconciliation.Difference != 0 {
		t.Errorf("Difference = %v, want 0", result.AgingReconciliation.Difference)
	}
}

func TestReconcile_ControlAccountMatches(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	receivables := []ar.Receivable{
		recv("R-1", "C1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), 5000, 5000, ar.StatusOpen),
	}
	control := 5000.0
	result := ar.Calculate(ar.Input{Receivables: receivables}, ar.Options{AsOfDate: asOf, ControlAccountBalance: &control})

	if !result.ControlAccountReconciliation.Available {
		t.Fatalf("expected ControlAccountReconciliation.Available=true")
	}
	if !result.ControlAccountReconciliation.Reconciled {
		t.Errorf("expected Reconciled=true, got %+v", result.ControlAccountReconciliation)
	}
}

func TestReconcile_ControlAccountMismatch(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	receivables := []ar.Receivable{
		recv("R-1", "C1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), 5000, 5000, ar.StatusOpen),
	}
	control := 9999.0
	result := ar.Calculate(ar.Input{Receivables: receivables}, ar.Options{AsOfDate: asOf, ControlAccountBalance: &control})

	if result.ControlAccountReconciliation.Reconciled {
		t.Errorf("expected Reconciled=false for mismatched control balance")
	}
	if !hasIssueCode(result.Issues, ar.IssueControlAccountMismatch) {
		t.Errorf("expected IssueControlAccountMismatch, got %+v", result.Issues)
	}
}

func TestReconcile_ControlAccountUnavailableWhenNotSupplied(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	result := ar.Calculate(ar.Input{Receivables: nil}, ar.Options{AsOfDate: asOf})
	if result.ControlAccountReconciliation.Available {
		t.Errorf("expected ControlAccountReconciliation.Available=false when not supplied")
	}
}
