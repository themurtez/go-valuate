package ap_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/ap"
	apfixtures "github.com/themurtez/go-valuate/accounting/ap/fixtures"
)

func TestPayments_EarlyOnTimeLateBehavior(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	payables := []ap.Payable{
		bill("B-1", "S1", mustDate(t, "2025-05-01"), mustDate(t, "2025-05-31"), 1000, 0, ap.StatusPaid), // paid early
		bill("B-2", "S1", mustDate(t, "2025-05-01"), mustDate(t, "2025-05-31"), 1000, 0, ap.StatusPaid), // paid exactly on due date
		bill("B-3", "S1", mustDate(t, "2025-05-01"), mustDate(t, "2025-05-31"), 1000, 0, ap.StatusPaid), // paid late
	}
	payments := []ap.SupplierPayment{
		{ID: "P-1", PayableID: "B-1", SupplierID: "S1", Date: mustDate(t, "2025-05-25"), Amount: 1000}, // 6 days early
		{ID: "P-2", PayableID: "B-2", SupplierID: "S1", Date: mustDate(t, "2025-05-31"), Amount: 1000}, // exactly due
		{ID: "P-3", PayableID: "B-3", SupplierID: "S1", Date: mustDate(t, "2025-06-10"), Amount: 1000}, // 10 days late
	}
	result := ap.Calculate(ap.Input{Payables: payables, Payments: payments}, ap.Options{AsOfDate: asOf})
	timing := result.TermsAnalysis.PaymentTiming
	if !timing.Available {
		t.Fatalf("expected PaymentTiming.Available=true, issues: %+v", result.Issues)
	}
	if timing.MatchedPaymentCount != 3 {
		t.Fatalf("expected 3 matched payments, got %d", timing.MatchedPaymentCount)
	}
	// 2 of 3 payments were on-or-before due date.
	if !timing.PercentPaidOnOrBeforeDue.Available {
		t.Fatalf("expected PercentPaidOnOrBeforeDue.Available=true")
	}
	wantOnTime := 2.0 / 3.0
	if diff := timing.PercentPaidOnOrBeforeDue.Value - wantOnTime; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("PercentPaidOnOrBeforeDue = %v, want %v", timing.PercentPaidOnOrBeforeDue.Value, wantOnTime)
	}
	wantLate := 1.0 / 3.0
	if diff := timing.PercentPaidLate.Value - wantLate; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("PercentPaidLate = %v, want %v", timing.PercentPaidLate.Value, wantLate)
	}
}

// TestPayments_LongTermsNotLate proves long contractual terms (net 60)
// are distinguishable from chronic lateness: this supplier pays before
// the due date every time, despite the long terms.
func TestPayments_LongTermsNotLate(t *testing.T) {
	asOf := mustDate(t, apfixtures.AsOfDate)
	payables, payments := apfixtures.LongTermsNotLate()
	result := ap.Calculate(ap.Input{Payables: payables, Payments: payments}, ap.Options{AsOfDate: asOf})
	if !result.TermsAnalysis.Available {
		t.Fatalf("expected TermsAnalysis.Available=true, issues: %+v", result.Issues)
	}
	if result.TermsAnalysis.AverageTermsDays.Value != 60 {
		t.Errorf("AverageTermsDays = %v, want 60 (long terms)", result.TermsAnalysis.AverageTermsDays.Value)
	}
	timing := result.TermsAnalysis.PaymentTiming
	if !timing.PercentPaidLate.Available || timing.PercentPaidLate.Value != 0 {
		t.Errorf("PercentPaidLate = %+v, want 0 (paid before due date despite long terms)", timing.PercentPaidLate)
	}
	for _, f := range result.Flags {
		if f.Code == ap.FlagRepeatedLatePayment {
			t.Errorf("did not expect FlagRepeatedLatePayment for a supplier with long terms but on-time payment")
		}
	}
}

// TestPayments_ChronicallyLatePayerTriggersFlag proves the opposite case:
// short terms (net 15) with genuinely late payment behavior.
func TestPayments_ChronicallyLatePayerTriggersFlag(t *testing.T) {
	asOf := mustDate(t, apfixtures.AsOfDate)
	payables, payments := apfixtures.ChronicallyLatePayer()
	result := ap.Calculate(ap.Input{Payables: payables, Payments: payments}, ap.Options{AsOfDate: asOf})
	timing := result.TermsAnalysis.PaymentTiming
	if !timing.Available {
		t.Fatalf("expected PaymentTiming.Available=true, issues: %+v", result.Issues)
	}
	if !timing.PercentPaidLate.Available || timing.PercentPaidLate.Value != 1.0 {
		t.Errorf("PercentPaidLate = %+v, want 1.0 (all 3 payments late)", timing.PercentPaidLate)
	}
	foundFlag := false
	for _, f := range result.Flags {
		if f.Code == ap.FlagRepeatedLatePayment && f.SupplierID == "SUP-LATE" {
			foundFlag = true
		}
	}
	if !foundFlag {
		t.Errorf("expected FlagRepeatedLatePayment for SUP-LATE (3 late payments >= default threshold of 3), flags: %+v", result.Flags)
	}
}

func TestPayments_UnknownPayablePayment(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	payables := []ap.Payable{bill("B-1", "S1", mustDate(t, "2025-05-01"), mustDate(t, "2025-05-31"), 1000, 0, ap.StatusPaid)}
	payments := []ap.SupplierPayment{{ID: "P-1", PayableID: "UNKNOWN", SupplierID: "S1", Date: mustDate(t, "2025-05-25"), Amount: 1000}}
	result := ap.Calculate(ap.Input{Payables: payables, Payments: payments}, ap.Options{AsOfDate: asOf})
	if !hasIssueCode(result.Issues, ap.IssueUnknownPayablePayment) {
		t.Errorf("expected IssueUnknownPayablePayment, got %+v", result.Issues)
	}
}

func TestPayments_InvalidPayment(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	payables := []ap.Payable{bill("B-1", "S1", mustDate(t, "2025-05-01"), mustDate(t, "2025-05-31"), 1000, 0, ap.StatusPaid)}
	payments := []ap.SupplierPayment{
		{ID: "P-1", PayableID: "B-1", SupplierID: "S1", Date: mustDate(t, "2025-05-25"), Amount: -50}, // negative amount
		{ID: "", PayableID: "B-1", SupplierID: "S1", Date: mustDate(t, "2025-05-25"), Amount: 100},    // missing ID
	}
	result := ap.Calculate(ap.Input{Payables: payables, Payments: payments}, ap.Options{AsOfDate: asOf})
	if !hasIssueCode(result.Issues, ap.IssueInvalidPayment) {
		t.Errorf("expected IssueInvalidPayment, got %+v", result.Issues)
	}
}

func TestPayments_WeightedAverageTermsDays(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	payables := []ap.Payable{
		bill("B-1", "S1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), 9000, 9000, ap.StatusOpen), // TermsDays not set here; set below.
	}
	payables[0].TermsDays = 60
	payables = append(payables, ap.Payable{
		ID: "B-2", SupplierID: "S1", BillDate: mustDate(t, "2025-06-01"), DueDate: mustDate(t, "2025-06-15"),
		OriginalAmount: 1000, OpenAmount: 1000, Currency: "USD", Status: ap.StatusOpen, TermsDays: 10,
	})
	result := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf})
	if !result.TermsAnalysis.Available {
		t.Fatalf("expected TermsAnalysis.Available=true")
	}
	// Simple average: (60+10)/2 = 35.
	if result.TermsAnalysis.AverageTermsDays.Value != 35 {
		t.Errorf("AverageTermsDays = %v, want 35", result.TermsAnalysis.AverageTermsDays.Value)
	}
	// Weighted by OriginalAmount: (60*9000 + 10*1000) / 10000 = 55.
	want := (60.0*9000 + 10.0*1000) / 10000.0
	if result.TermsAnalysis.WeightedAverageTermsDays.Value != want {
		t.Errorf("WeightedAverageTermsDays = %v, want %v", result.TermsAnalysis.WeightedAverageTermsDays.Value, want)
	}
}
