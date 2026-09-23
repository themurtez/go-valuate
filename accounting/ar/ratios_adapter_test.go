package ar_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/ar"
	"github.com/themurtez/go-valuate/analytics/ratios"
	"github.com/themurtez/go-valuate/financial"
)

// TestAdapter_DSOMatchesRatiosPackage_WhenDefinitionsAlign compares DSO
// from accounting/ar (this package's own AR-aging-based DSO) against DSO
// from analytics/ratios (financial.FinancialDataset-based: (AR/Revenue)*365
// — see analytics/ratios/efficiency.go), ONLY when both are fed equivalent
// inputs and equivalent definitions: this package's Ending AR and Sales
// values equal to the exact AR/Revenue figures in the FinancialDataset, and
// both computed over the same 365-day period. This is a test adapter, not
// a hard package dependency — accounting/ar never imports analytics/ratios
// in its own source, per the task's "do not force package dependency if a
// test adapter is cleaner" instruction.
func TestAdapter_DSOMatchesRatiosPackage_WhenDefinitionsAlign(t *testing.T) {
	const period financial.Period = "2025"
	const endingAR = 20000.0
	const revenue = 200000.0

	ds := financial.FinancialDataset{
		Currency: "USD",
		Items: []financial.NormalizedItem{
			{Code: financial.CodeBsAccountsReceivable, Period: period, Amount: endingAR},
			{Code: financial.CodeRevService, Period: period, Amount: revenue},
		},
	}
	ratiosResult := ratios.Calculate(ratios.Input{Dataset: ds}, ratios.Options{})
	if !ratiosResult.Available || len(ratiosResult.History) != 1 {
		t.Fatalf("expected ratios.Result.Available with 1 period, got %+v", ratiosResult)
	}
	ratiosDSO := ratiosResult.History[0].DaysSalesOutstanding
	if !ratiosDSO.Value.Available {
		t.Fatalf("expected analytics/ratios DSO to be available")
	}

	// accounting/ar's own simple DSO, fed the exact same AR/revenue figures
	// and the same 365-day period length analytics/ratios uses for an
	// annual period.
	asOf := mustDate(t, "2025-12-31")
	arResult := ar.Calculate(ar.Input{
		Receivables: []ar.Receivable{
			recv("INV-1", "C1", mustDate(t, "2025-01-01"), mustDate(t, "2025-01-31"), endingAR, endingAR, ar.StatusOpen),
		},
		SalesHistory: []ar.SalesPeriod{
			{Period: period, SalesAmount: revenue, Days: 365, Basis: ar.SalesBasisTotalSales},
		},
	}, ar.Options{AsOfDate: asOf})

	if !arResult.DSO.Available {
		t.Fatalf("expected accounting/ar DSO to be available")
	}

	const tolerance = 0.01
	diff := arResult.DSO.Value - ratiosDSO.Value.Value
	if diff < -tolerance || diff > tolerance {
		t.Errorf("DSO mismatch: accounting/ar=%v, analytics/ratios=%v", arResult.DSO.Value, ratiosDSO.Value.Value)
	}
}
