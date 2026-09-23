package ap_test

import (
	"math"
	"testing"

	"github.com/themurtez/go-valuate/accounting/ap"
)

func TestValidate_PartialPayment(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	payables := []ap.Payable{bill("B-1", "S1", mustDate(t, "2025-05-01"), mustDate(t, "2025-05-31"), 7500, 2500, ap.StatusPartiallyPaid)}
	result := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf})
	if !result.Available {
		t.Fatalf("expected Available=true, issues: %+v", result.Issues)
	}
	if result.PortfolioSummary.TotalOpenPayables != 2500 {
		t.Errorf("TotalOpenPayables = %v, want 2500 (open, not original)", result.PortfolioSummary.TotalOpenPayables)
	}
	if result.PortfolioSummary.TotalGrossPayables != 7500 {
		t.Errorf("TotalGrossPayables = %v, want 7500", result.PortfolioSummary.TotalGrossPayables)
	}
}

func TestValidate_PaidExcludedByDefault(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	payables := []ap.Payable{
		bill("B-1", "S1", mustDate(t, "2025-05-01"), mustDate(t, "2025-05-31"), 1000, 1000, ap.StatusOpen),
		bill("B-2", "S1", mustDate(t, "2025-04-01"), mustDate(t, "2025-05-01"), 500, 0, ap.StatusPaid),
	}
	result := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf})
	if result.PortfolioSummary.TotalOpenPayables != 1000 {
		t.Errorf("TotalOpenPayables = %v, want 1000 (PAID excluded)", result.PortfolioSummary.TotalOpenPayables)
	}
}

func TestValidate_VoidedExcludedByDefault(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	payables := []ap.Payable{
		bill("B-1", "S1", mustDate(t, "2025-05-01"), mustDate(t, "2025-05-31"), 1000, 1000, ap.StatusOpen),
		bill("B-2", "S1", mustDate(t, "2025-04-01"), mustDate(t, "2025-05-01"), 500, 500, ap.StatusVoided),
	}
	result := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf})
	if result.PortfolioSummary.TotalOpenPayables != 1000 {
		t.Errorf("TotalOpenPayables = %v, want 1000 (VOIDED excluded)", result.PortfolioSummary.TotalOpenPayables)
	}
}

func TestValidate_DisputedIncludedByDefault(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	payables := []ap.Payable{bill("B-1", "S1", mustDate(t, "2025-03-01"), mustDate(t, "2025-03-31"), 20000, 20000, ap.StatusDisputed)}
	result := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf})
	if !result.Available {
		t.Fatalf("expected Available=true, issues: %+v", result.Issues)
	}
	if result.PortfolioSummary.TotalOpenPayables != 20000 {
		t.Errorf("TotalOpenPayables = %v, want 20000 (disputed still ages by default)", result.PortfolioSummary.TotalOpenPayables)
	}
	if result.PortfolioSummary.DisputedAmount != 20000 {
		t.Errorf("DisputedAmount = %v, want 20000", result.PortfolioSummary.DisputedAmount)
	}
}

func TestValidate_StatusNeverInferredFromOpenAmount(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	// Status PAID but nonzero OpenAmount: should be flagged, NOT silently
	// treated as open, and NOT silently corrected to zero.
	payables := []ap.Payable{bill("B-1", "S1", mustDate(t, "2025-05-01"), mustDate(t, "2025-05-31"), 1000, 400, ap.StatusPaid)}
	result := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf})
	if !hasIssueCode(result.Issues, ap.IssuePaidWithOpenBalance) {
		t.Errorf("expected IssuePaidWithOpenBalance, got %+v", result.Issues)
	}
	// PAID is excluded from aging by default regardless of OpenAmount.
	if result.PortfolioSummary.TotalOpenPayables != 0 {
		t.Errorf("TotalOpenPayables = %v, want 0 (PAID status excluded regardless of OpenAmount)", result.PortfolioSummary.TotalOpenPayables)
	}
}

func TestValidate_VendorCredit(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	payables := []ap.Payable{
		bill("B-1", "S1", mustDate(t, "2025-06-01"), mustDate(t, "2025-07-01"), 8000, 8000, ap.StatusOpen),
		{ID: "VC-1", SupplierID: "S1", DocumentType: ap.DocumentTypeVendorCredit,
			BillDate: mustDate(t, "2025-06-10"), DueDate: mustDate(t, "2025-06-10"),
			OriginalAmount: -1500, OpenAmount: -1500, Currency: "USD", Status: ap.StatusOpen},
	}
	result := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf})
	if !result.Available {
		t.Fatalf("expected Available=true, issues: %+v", result.Issues)
	}
	// Default: no netting. Bill total unaffected by credit.
	if result.PortfolioSummary.TotalOpenPayables != 8000 {
		t.Errorf("TotalOpenPayables = %v, want 8000 (no netting by default)", result.PortfolioSummary.TotalOpenPayables)
	}
	if result.PortfolioSummary.VendorCredits.TotalCreditBalance != -1500 {
		t.Errorf("VendorCredits.TotalCreditBalance = %v, want -1500", result.PortfolioSummary.VendorCredits.TotalCreditBalance)
	}
	if result.PortfolioSummary.VendorCredits.VendorCreditCount != 1 {
		t.Errorf("VendorCreditCount = %v, want 1", result.PortfolioSummary.VendorCredits.VendorCreditCount)
	}

	// Net-by-supplier policy.
	netted := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf, CreditNetting: ap.CreditNetBySupplier})
	if netted.PortfolioSummary.TotalOpenPayables != 6500 {
		t.Errorf("netted TotalOpenPayables = %v, want 6500", netted.PortfolioSummary.TotalOpenPayables)
	}
	// Gross credit balance is still exposed even under netting.
	if netted.PortfolioSummary.VendorCredits.TotalCreditBalance != -1500 {
		t.Errorf("netted VendorCredits.TotalCreditBalance = %v, want -1500 (gross always exposed)", netted.PortfolioSummary.VendorCredits.TotalCreditBalance)
	}
}

func TestValidate_ZeroAP(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	result := ap.Calculate(ap.Input{Payables: nil}, ap.Options{AsOfDate: asOf})
	if !result.Available {
		t.Fatalf("expected Available=true for empty payables, issues: %+v", result.Issues)
	}
	if result.PortfolioSummary.TotalOpenPayables != 0 {
		t.Errorf("TotalOpenPayables = %v, want 0", result.PortfolioSummary.TotalOpenPayables)
	}
	if len(result.SupplierSummaries) != 0 {
		t.Errorf("expected no SupplierSummaries, got %+v", result.SupplierSummaries)
	}
	if !result.AgingReconciliation.Balanced {
		t.Errorf("expected AgingReconciliation.Balanced=true for empty portfolio")
	}
}

func TestValidate_InvalidAmount_SignMismatch(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	// Ordinary bill with negative OriginalAmount, no vendor-credit type.
	payables := []ap.Payable{bill("B-1", "S1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), -500, -500, ap.StatusOpen)}
	result := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf})
	if !hasIssueCode(result.Issues, ap.IssueInvalidAmount) {
		t.Errorf("expected IssueInvalidAmount, got %+v", result.Issues)
	}
}

func TestValidate_OpenExceedsOriginal(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	payables := []ap.Payable{bill("B-1", "S1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), 500, 900, ap.StatusOpen)}
	result := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf})
	if !hasIssueCode(result.Issues, ap.IssueOpenExceedsOriginal) {
		t.Errorf("expected IssueOpenExceedsOriginal, got %+v", result.Issues)
	}
	// Not silently repaired: OpenAmount used as-is.
	if result.PortfolioSummary.TotalOpenPayables != 900 {
		t.Errorf("TotalOpenPayables = %v, want 900 (not silently corrected)", result.PortfolioSummary.TotalOpenPayables)
	}
}

func TestValidate_NonFiniteAmount(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	payables := []ap.Payable{
		bill("B-NaN", "S1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), math.NaN(), math.NaN(), ap.StatusOpen),
		bill("B-Inf", "S1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), math.Inf(1), math.Inf(1), ap.StatusOpen),
		bill("B-OK", "S1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), 100, 100, ap.StatusOpen),
	}
	result := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf})
	if !hasIssueCode(result.Issues, ap.IssueNonFiniteAmount) {
		t.Errorf("expected IssueNonFiniteAmount, got %+v", result.Issues)
	}
	if result.PortfolioSummary.TotalOpenPayables != 100 {
		t.Errorf("TotalOpenPayables = %v, want 100 (non-finite rows excluded)", result.PortfolioSummary.TotalOpenPayables)
	}
}

func TestValidate_DuplicatePayableID(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	payables := []ap.Payable{
		bill("B-1", "S1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), 100, 100, ap.StatusOpen),
		bill("B-1", "S1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), 999, 999, ap.StatusOpen),
	}
	result := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf})
	if !hasIssueCode(result.Issues, ap.IssueDuplicatePayable) {
		t.Errorf("expected IssueDuplicatePayable, got %+v", result.Issues)
	}
	if result.PortfolioSummary.TotalOpenPayables != 100 {
		t.Errorf("TotalOpenPayables = %v, want 100 (only first occurrence used)", result.PortfolioSummary.TotalOpenPayables)
	}
}

func TestValidate_MissingSupplierID(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	payables := []ap.Payable{{ID: "B-1", BillDate: mustDate(t, "2025-06-01"), DueDate: mustDate(t, "2025-06-15"), OriginalAmount: 100, OpenAmount: 100, Currency: "USD", Status: ap.StatusOpen}}
	result := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf})
	if !hasIssueCode(result.Issues, ap.IssueMissingSupplier) {
		t.Errorf("expected IssueMissingSupplier, got %+v", result.Issues)
	}
	if result.PortfolioSummary.TotalOpenPayables != 0 {
		t.Errorf("TotalOpenPayables = %v, want 0 (excluded)", result.PortfolioSummary.TotalOpenPayables)
	}
}

func TestValidate_FutureBillDate(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	payables := []ap.Payable{bill("B-1", "S1", mustDate(t, "2025-08-01"), mustDate(t, "2025-08-31"), 100, 100, ap.StatusOpen)}
	result := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf})
	if !hasIssueCode(result.Issues, ap.IssueFutureBill) {
		t.Errorf("expected IssueFutureBill, got %+v", result.Issues)
	}
}

func TestValidate_InvalidStatus(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	payables := []ap.Payable{{ID: "B-1", SupplierID: "S1", BillDate: mustDate(t, "2025-06-01"), DueDate: mustDate(t, "2025-06-15"), OriginalAmount: 100, OpenAmount: 100, Currency: "USD", Status: "BOGUS"}}
	result := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf})
	if !hasIssueCode(result.Issues, ap.IssueInvalidStatus) {
		t.Errorf("expected IssueInvalidStatus, got %+v", result.Issues)
	}
}
