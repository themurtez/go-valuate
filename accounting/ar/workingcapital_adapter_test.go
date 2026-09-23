package ar_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/ar"
	"github.com/themurtez/go-valuate/analytics/workingcapital"
	"github.com/themurtez/go-valuate/financial"
)

// TestAdapter_ARTotalReconcilesIntoWorkingCapitalComponent shows AR ->
// working-capital component reconciliation (section 28): this package's
// own PortfolioSummary.TotalOpenReceivables, when fed into a
// financial.FinancialDataset as CodeBsAccountsReceivable, contributes
// exactly that amount to analytics/workingcapital's
// OperatingCurrentAssets under DefaultInclusionPolicy (which includes AR
// by default). A test adapter, not a hard package dependency —
// accounting/ar never imports analytics/workingcapital in its own source.
func TestAdapter_ARTotalReconcilesIntoWorkingCapitalComponent(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	receivables := []ar.Receivable{
		recv("INV-1", "C1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), 7000, 7000, ar.StatusOpen),
		recv("INV-2", "C2", mustDate(t, "2025-04-01"), mustDate(t, "2025-04-30"), 3000, 3000, ar.StatusOpen),
	}
	arResult := ar.Calculate(ar.Input{Receivables: receivables}, ar.Options{AsOfDate: asOf})
	if !arResult.Available {
		t.Fatalf("expected ar.Result.Available=true, issues: %+v", arResult.Issues)
	}

	const period financial.Period = "2025"
	ds := financial.FinancialDataset{
		Currency: "USD",
		Items: []financial.NormalizedItem{
			{Code: financial.CodeBsAccountsReceivable, Period: period, Amount: arResult.PortfolioSummary.TotalOpenReceivables},
		},
	}
	wcResult := workingcapital.Calculate(workingcapital.Input{Dataset: ds}, workingcapital.Options{})
	if !wcResult.Available || len(wcResult.History) != 1 {
		t.Fatalf("expected workingcapital.Result.Available with 1 period, got %+v", wcResult)
	}

	assets := wcResult.History[0].OperatingCurrentAssets
	if !assets.Available {
		t.Fatalf("expected OperatingCurrentAssets.Available=true")
	}
	if assets.Value != arResult.PortfolioSummary.TotalOpenReceivables {
		t.Errorf("OperatingCurrentAssets (%v) does not match AR aging total (%v)", assets.Value, arResult.PortfolioSummary.TotalOpenReceivables)
	}
}
