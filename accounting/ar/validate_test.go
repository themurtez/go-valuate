package ar_test

import (
	"math"
	"testing"

	"github.com/themurtez/go-valuate/accounting/ar"
)

func hasIssueCode(issues []ar.Issue, code ar.IssueCode) bool {
	for _, i := range issues {
		if i.Code == code {
			return true
		}
	}
	return false
}

func TestValidate_PartiallyPaidIncluded(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	receivables := []ar.Receivable{
		recv("R-1", "C1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), 1000, 400, ar.StatusPartiallyPaid),
	}
	result := ar.Calculate(ar.Input{Receivables: receivables}, ar.Options{AsOfDate: asOf})
	if result.PortfolioSummary.TotalOpenReceivables != 400 {
		t.Errorf("TotalOpenReceivables = %v, want 400", result.PortfolioSummary.TotalOpenReceivables)
	}
	if result.PortfolioSummary.TotalGrossReceivables != 1000 {
		t.Errorf("TotalGrossReceivables = %v, want 1000", result.PortfolioSummary.TotalGrossReceivables)
	}
}

func TestValidate_PaidExcludedByDefault(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	receivables := []ar.Receivable{
		recv("R-1", "C1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), 1000, 0, ar.StatusPaid),
	}
	result := ar.Calculate(ar.Input{Receivables: receivables}, ar.Options{AsOfDate: asOf})
	if result.PortfolioSummary.TotalOpenReceivables != 0 {
		t.Errorf("TotalOpenReceivables = %v, want 0 (paid excluded)", result.PortfolioSummary.TotalOpenReceivables)
	}
	if result.PortfolioSummary.OpenInvoiceCount != 0 {
		t.Errorf("OpenInvoiceCount = %v, want 0", result.PortfolioSummary.OpenInvoiceCount)
	}
}

func TestValidate_VoidedExcluded(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	receivables := []ar.Receivable{
		recv("R-1", "C1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), 1000, 1000, ar.StatusVoided),
	}
	result := ar.Calculate(ar.Input{Receivables: receivables}, ar.Options{AsOfDate: asOf})
	if result.PortfolioSummary.TotalOpenReceivables != 0 {
		t.Errorf("TotalOpenReceivables = %v, want 0 (voided excluded)", result.PortfolioSummary.TotalOpenReceivables)
	}
}

func TestValidate_DisputedIncludedAndTracked(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	receivables := []ar.Receivable{
		recv("R-1", "C1", mustDate(t, "2025-05-01"), mustDate(t, "2025-05-31"), 2000, 2000, ar.StatusDisputed),
	}
	result := ar.Calculate(ar.Input{Receivables: receivables}, ar.Options{AsOfDate: asOf})
	if result.PortfolioSummary.TotalOpenReceivables != 2000 {
		t.Errorf("disputed should still age: TotalOpenReceivables = %v, want 2000", result.PortfolioSummary.TotalOpenReceivables)
	}
	if result.PortfolioSummary.DisputedAmount != 2000 {
		t.Errorf("DisputedAmount = %v, want 2000", result.PortfolioSummary.DisputedAmount)
	}
}

func TestValidate_CreditMemoNotInvalid(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	receivables := []ar.Receivable{
		{
			ID: "CM-1", CustomerID: "C1", DocumentType: ar.DocumentTypeCreditMemo,
			InvoiceDate: mustDate(t, "2025-06-01"), DueDate: mustDate(t, "2025-06-01"),
			OriginalAmount: -500, OpenAmount: -500, Currency: "USD", Status: ar.StatusOpen,
		},
	}
	result := ar.Calculate(ar.Input{Receivables: receivables}, ar.Options{AsOfDate: asOf})
	if hasIssueCode(result.Issues, ar.IssueInvalidAmount) {
		t.Errorf("credit memo with negative amounts should not be flagged invalid: %+v", result.Issues)
	}
	if result.PortfolioSummary.Credits.TotalCreditBalance != -500 {
		t.Errorf("Credits.TotalCreditBalance = %v, want -500", result.PortfolioSummary.Credits.TotalCreditBalance)
	}
	if result.PortfolioSummary.Credits.CreditMemoCount != 1 {
		t.Errorf("Credits.CreditMemoCount = %v, want 1", result.PortfolioSummary.Credits.CreditMemoCount)
	}
	// Default policy: no netting against invoice totals.
	if result.PortfolioSummary.TotalOpenReceivables != 0 {
		t.Errorf("TotalOpenReceivables = %v, want 0 (no invoices, credit reported separately)", result.PortfolioSummary.TotalOpenReceivables)
	}
}

func TestValidate_CreditNettingByCustomer(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	receivables := []ar.Receivable{
		recv("INV-1", "C1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), 1000, 1000, ar.StatusOpen),
		{
			ID: "CM-1", CustomerID: "C1", DocumentType: ar.DocumentTypeCreditMemo,
			InvoiceDate: mustDate(t, "2025-06-01"), DueDate: mustDate(t, "2025-06-01"),
			OriginalAmount: -300, OpenAmount: -300, Currency: "USD", Status: ar.StatusOpen,
		},
	}
	noNetting := ar.Calculate(ar.Input{Receivables: receivables}, ar.Options{AsOfDate: asOf})
	if noNetting.PortfolioSummary.TotalOpenReceivables != 1000 {
		t.Errorf("no netting: TotalOpenReceivables = %v, want 1000", noNetting.PortfolioSummary.TotalOpenReceivables)
	}

	netted := ar.Calculate(ar.Input{Receivables: receivables}, ar.Options{AsOfDate: asOf, CreditNetting: ar.CreditNetByCustomer})
	if netted.PortfolioSummary.TotalOpenReceivables != 700 {
		t.Errorf("netted: TotalOpenReceivables = %v, want 700", netted.PortfolioSummary.TotalOpenReceivables)
	}
	for _, cs := range netted.CustomerSummaries {
		if cs.CustomerID == "C1" && cs.CreditAmount != -300 {
			t.Errorf("customer credit amount = %v, want -300", cs.CreditAmount)
		}
	}
}

func TestValidate_ZeroBalanceReceivable(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	receivables := []ar.Receivable{
		recv("R-1", "C1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), 1000, 0, ar.StatusOpen),
	}
	result := ar.Calculate(ar.Input{Receivables: receivables}, ar.Options{AsOfDate: asOf})
	if result.PortfolioSummary.OpenInvoiceCount != 0 {
		t.Errorf("zero-balance receivable should not count as an open invoice: got %v", result.PortfolioSummary.OpenInvoiceCount)
	}
}

func TestValidate_InvalidNegativeOpenExceedsOriginal(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	receivables := []ar.Receivable{
		recv("R-1", "C1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), 100, 500, ar.StatusOpen),
	}
	result := ar.Calculate(ar.Input{Receivables: receivables}, ar.Options{AsOfDate: asOf})
	if !hasIssueCode(result.Issues, ar.IssueOpenExceedsOriginal) {
		t.Errorf("expected IssueOpenExceedsOriginal, got %+v", result.Issues)
	}
}

func TestValidate_NonFiniteAmountExcluded(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	receivables := []ar.Receivable{
		recv("R-1", "C1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), math.NaN(), 100, ar.StatusOpen),
		recv("R-2", "C1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), 100, math.Inf(1), ar.StatusOpen),
		recv("R-3", "C1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), 500, 500, ar.StatusOpen),
	}
	result := ar.Calculate(ar.Input{Receivables: receivables}, ar.Options{AsOfDate: asOf})
	if !hasIssueCode(result.Issues, ar.IssueNonFiniteAmount) {
		t.Errorf("expected IssueNonFiniteAmount, got %+v", result.Issues)
	}
	if result.PortfolioSummary.TotalOpenReceivables != 500 {
		t.Errorf("TotalOpenReceivables = %v, want 500 (only valid row counted)", result.PortfolioSummary.TotalOpenReceivables)
	}
}

func TestValidate_PaidWithOpenBalanceFlagged(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	receivables := []ar.Receivable{
		recv("R-1", "C1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), 1000, 200, ar.StatusPaid),
	}
	result := ar.Calculate(ar.Input{Receivables: receivables}, ar.Options{AsOfDate: asOf})
	if !hasIssueCode(result.Issues, ar.IssuePaidWithOpenBalance) {
		t.Errorf("expected IssuePaidWithOpenBalance, got %+v", result.Issues)
	}
}

func TestValidate_DuplicateReceivableID(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	receivables := []ar.Receivable{
		recv("R-1", "C1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), 500, 500, ar.StatusOpen),
		recv("R-1", "C1", mustDate(t, "2025-06-02"), mustDate(t, "2025-06-16"), 700, 700, ar.StatusOpen),
	}
	result := ar.Calculate(ar.Input{Receivables: receivables}, ar.Options{AsOfDate: asOf})
	if !hasIssueCode(result.Issues, ar.IssueDuplicateReceivable) {
		t.Errorf("expected IssueDuplicateReceivable, got %+v", result.Issues)
	}
	if result.PortfolioSummary.TotalOpenReceivables != 500 {
		t.Errorf("only first occurrence should be used: TotalOpenReceivables = %v, want 500", result.PortfolioSummary.TotalOpenReceivables)
	}
}

func TestValidate_InvalidStatus(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	receivables := []ar.Receivable{
		recv("R-1", "C1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), 500, 500, ar.ReceivableStatus("BOGUS")),
	}
	result := ar.Calculate(ar.Input{Receivables: receivables}, ar.Options{AsOfDate: asOf})
	if !hasIssueCode(result.Issues, ar.IssueInvalidStatus) {
		t.Errorf("expected IssueInvalidStatus, got %+v", result.Issues)
	}
}

func TestValidate_MissingCustomerID(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	receivables := []ar.Receivable{
		recv("R-1", "", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), 500, 500, ar.StatusOpen),
	}
	result := ar.Calculate(ar.Input{Receivables: receivables}, ar.Options{AsOfDate: asOf})
	if !hasIssueCode(result.Issues, ar.IssueMissingCustomer) {
		t.Errorf("expected IssueMissingCustomer, got %+v", result.Issues)
	}
}

func TestValidate_MissingCurrency(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	receivables := []ar.Receivable{
		{ID: "R-1", CustomerID: "C1", InvoiceDate: mustDate(t, "2025-06-01"), DueDate: mustDate(t, "2025-06-15"),
			OriginalAmount: 500, OpenAmount: 500, Status: ar.StatusOpen},
	}
	result := ar.Calculate(ar.Input{Receivables: receivables}, ar.Options{AsOfDate: asOf})
	if !hasIssueCode(result.Issues, ar.IssueMixedCurrency) {
		t.Errorf("expected IssueMixedCurrency for missing currency, got %+v", result.Issues)
	}
}

func TestValidate_InvalidDateOrder(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	receivables := []ar.Receivable{
		recv("R-1", "C1", mustDate(t, "2025-06-15"), mustDate(t, "2025-06-01"), 500, 500, ar.StatusOpen),
	}
	result := ar.Calculate(ar.Input{Receivables: receivables}, ar.Options{AsOfDate: asOf})
	if !hasIssueCode(result.Issues, ar.IssueInvalidDate) {
		t.Errorf("expected IssueInvalidDate, got %+v", result.Issues)
	}
}
