package ar_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/ar"
)

func TestDSO_ValidSimpleDSO(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	receivables := []ar.Receivable{
		recv("R-1", "C1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), 20000, 20000, ar.StatusOpen),
	}
	sales := []ar.SalesPeriod{
		{Period: "2025-Q2", SalesAmount: 200000, Days: 90, Basis: ar.SalesBasisCreditSales},
	}
	result := ar.Calculate(ar.Input{Receivables: receivables, SalesHistory: sales}, ar.Options{AsOfDate: asOf})

	if !result.DSO.Available {
		t.Fatalf("expected DSO.Available=true")
	}
	// DSO = (20000/200000)*90 = 9.
	if result.DSO.Value != 9 {
		t.Errorf("DSO = %v, want 9", result.DSO.Value)
	}
	if result.DSO.Basis != ar.SalesBasisCreditSales {
		t.Errorf("DSO.Basis = %v, want CREDIT_SALES", result.DSO.Basis)
	}
}

func TestDSO_MissingSalesUnavailable(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	result := ar.Calculate(ar.Input{Receivables: nil}, ar.Options{AsOfDate: asOf})
	if result.DSO.Available {
		t.Errorf("expected DSO.Available=false with no sales history")
	}
	if !hasIssueCode(result.Issues, ar.IssueMissingSalesForDSO) {
		t.Errorf("expected IssueMissingSalesForDSO, got %+v", result.Issues)
	}
}

func TestDSO_ZeroSalesUnavailable(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	sales := []ar.SalesPeriod{{Period: "2025-Q2", SalesAmount: 0, Days: 90}}
	result := ar.Calculate(ar.Input{Receivables: nil, SalesHistory: sales}, ar.Options{AsOfDate: asOf})
	if result.DSO.Available {
		t.Errorf("expected DSO.Available=false with zero sales")
	}
}

func TestDSO_HistoryTrend(t *testing.T) {
	sales := []ar.SalesPeriod{
		{Period: "2025-Q1", SalesAmount: 100000, Days: 90, EndingAR: ptrF(15000)}, // DSO=13.5
		{Period: "2025-Q2", SalesAmount: 100000, Days: 90, EndingAR: ptrF(30000)}, // DSO=27
	}
	asOf := mustDate(t, "2025-06-30")
	result := ar.Calculate(ar.Input{Receivables: nil, SalesHistory: sales}, ar.Options{AsOfDate: asOf})

	if !result.DSOHistory.Available {
		t.Fatalf("expected DSOHistory.Available=true")
	}
	if len(result.DSOHistory.Points) != 2 {
		t.Fatalf("expected 2 history points, got %d", len(result.DSOHistory.Points))
	}
	if result.DSOHistory.Trend != "deteriorating" {
		t.Errorf("Trend = %v, want deteriorating", result.DSOHistory.Trend)
	}
	if !result.DSOHistory.FirstVsLastChange.Available || result.DSOHistory.FirstVsLastChange.Value <= 0 {
		t.Errorf("FirstVsLastChange = %+v, want positive", result.DSOHistory.FirstVsLastChange)
	}
}

func TestDSO_HistoryRequiresEndingAR(t *testing.T) {
	sales := []ar.SalesPeriod{
		{Period: "2025-Q1", SalesAmount: 100000, Days: 90}, // no EndingAR
	}
	asOf := mustDate(t, "2025-06-30")
	result := ar.Calculate(ar.Input{Receivables: nil, SalesHistory: sales}, ar.Options{AsOfDate: asOf})
	if result.DSOHistory.Available {
		t.Errorf("expected DSOHistory.Available=false without EndingAR")
	}
}

func ptrF(v float64) *float64 { return &v }
