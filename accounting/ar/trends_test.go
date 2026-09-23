package ar_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/ar"
)

func TestMigration_Deterioration(t *testing.T) {
	// Snapshot: receivable was CURRENT (0 days) at 2025-05-31.
	// Current: same receivable is now 61 days past due (31_60 -> 61_90).
	snapshots := []ar.Snapshot{
		{
			AsOfDate: "2025-05-31",
			Receivables: []ar.Receivable{
				recv("R-1", "C1", mustDate(t, "2025-05-01"), mustDate(t, "2025-05-31"), 5000, 5000, ar.StatusOpen),
			},
		},
	}
	current := []ar.Receivable{
		recv("R-1", "C1", mustDate(t, "2025-05-01"), mustDate(t, "2025-05-31"), 5000, 5000, ar.StatusOpen),
	}
	asOf := mustDate(t, "2025-07-31") // 61 days past due.

	result := ar.Calculate(ar.Input{Receivables: current, Snapshots: snapshots}, ar.Options{AsOfDate: asOf})
	if !result.Migration.Available {
		t.Fatalf("expected Migration.Available=true")
	}
	if result.Migration.DeteriorationAmount != 5000 {
		t.Errorf("DeteriorationAmount = %v, want 5000", result.Migration.DeteriorationAmount)
	}
	if result.Migration.CureAmount != 0 {
		t.Errorf("CureAmount = %v, want 0", result.Migration.CureAmount)
	}
	if result.Migration.CustomersDeteriorating != 1 {
		t.Errorf("CustomersDeteriorating = %v, want 1", result.Migration.CustomersDeteriorating)
	}
}

func TestMigration_Improvement_MovesToPaid(t *testing.T) {
	snapshots := []ar.Snapshot{
		{
			AsOfDate: "2025-04-30",
			Receivables: []ar.Receivable{
				recv("R-1", "C1", mustDate(t, "2025-02-01"), mustDate(t, "2025-03-03"), 6000, 6000, ar.StatusOpen),
			},
		},
	}
	asOf := mustDate(t, "2025-06-30")
	// Receivable no longer present/open in current data -> paid/cleared.
	result := ar.Calculate(ar.Input{Receivables: nil, Snapshots: snapshots}, ar.Options{AsOfDate: asOf})

	if !result.Migration.Available {
		t.Fatalf("expected Migration.Available=true")
	}
	if result.Migration.CureAmount != 6000 {
		t.Errorf("CureAmount = %v, want 6000", result.Migration.CureAmount)
	}
	found := false
	for _, e := range result.Migration.Entries {
		if e.ToPaid && e.Amount == 6000 {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a ToPaid migration entry, got %+v", result.Migration.Entries)
	}
}

func TestMigration_UnavailableWithoutSnapshots(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	result := ar.Calculate(ar.Input{Receivables: nil}, ar.Options{AsOfDate: asOf})
	if result.Migration.Available {
		t.Errorf("expected Migration.Available=false with no snapshots")
	}
}

func TestPaymentTiming_AverageDaysToPay(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	receivables := []ar.Receivable{
		recv("R-1", "C1", mustDate(t, "2025-05-01"), mustDate(t, "2025-05-31"), 1000, 0, ar.StatusPaid),
	}
	payments := []ar.Payment{
		{ID: "P-1", ReceivableID: "R-1", CustomerID: "C1", Date: mustDate(t, "2025-05-21"), Amount: 1000}, // 20 days after invoice
	}
	result := ar.Calculate(ar.Input{Receivables: receivables, Payments: payments}, ar.Options{AsOfDate: asOf})

	if !result.TermsAnalysis.PaymentTiming.Available {
		t.Fatalf("expected PaymentTiming.Available=true")
	}
	if result.TermsAnalysis.PaymentTiming.AverageDaysToPay.Value != 20 {
		t.Errorf("AverageDaysToPay = %v, want 20", result.TermsAnalysis.PaymentTiming.AverageDaysToPay.Value)
	}
}

func TestPayment_UnknownReceivable(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	payments := []ar.Payment{
		{ID: "P-1", ReceivableID: "DOES-NOT-EXIST", CustomerID: "C1", Date: mustDate(t, "2025-05-21"), Amount: 500},
	}
	result := ar.Calculate(ar.Input{Receivables: nil, Payments: payments}, ar.Options{AsOfDate: asOf})
	if !hasIssueCode(result.Issues, ar.IssueUnknownReceivablePayment) {
		t.Errorf("expected IssueUnknownReceivablePayment, got %+v", result.Issues)
	}
}

func TestPayment_InvalidPayment(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	payments := []ar.Payment{
		{ID: "P-1", ReceivableID: "R-1", CustomerID: "C1", Date: mustDate(t, "2025-05-21"), Amount: -50},
	}
	result := ar.Calculate(ar.Input{Receivables: nil, Payments: payments}, ar.Options{AsOfDate: asOf})
	if !hasIssueCode(result.Issues, ar.IssueInvalidPayment) {
		t.Errorf("expected IssueInvalidPayment, got %+v", result.Issues)
	}
}
