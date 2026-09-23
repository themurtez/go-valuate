package ap_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/ap"
	"github.com/themurtez/go-valuate/analytics/workingcapital"
	"github.com/themurtez/go-valuate/financial"
)

// TestAdapter_APTotalReconcilesIntoWorkingCapitalComponent shows AP ->
// working-capital component reconciliation (task section 18): this
// package's own PortfolioSummary.TotalOpenPayables, when fed into a
// financial.FinancialDataset as CodeBsAccountsPayable, contributes exactly
// that amount to analytics/workingcapital's OperatingCurrentLiabilities
// under DefaultInclusionPolicy (which includes AP by default). A test
// adapter, not a hard package dependency — accounting/ap never imports
// analytics/workingcapital in its own source. Mirrors
// accounting/ar/workingcapital_adapter_test.go's identical pattern (AR ->
// OperatingCurrentAssets there; AP -> OperatingCurrentLiabilities here).
func TestAdapter_APTotalReconcilesIntoWorkingCapitalComponent(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	payables := []ap.Payable{
		bill("BILL-1", "S1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), 7000, 7000, ap.StatusOpen),
		bill("BILL-2", "S2", mustDate(t, "2025-04-01"), mustDate(t, "2025-04-30"), 3000, 3000, ap.StatusOpen),
	}
	apResult := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf})
	if !apResult.Available {
		t.Fatalf("expected ap.Result.Available=true, issues: %+v", apResult.Issues)
	}

	const period financial.Period = "2025"
	ds := financial.FinancialDataset{
		Currency: "USD",
		Items: []financial.NormalizedItem{
			{Code: financial.CodeBsAccountsPayable, Period: period, Amount: apResult.PortfolioSummary.TotalOpenPayables},
		},
	}
	wcResult := workingcapital.Calculate(workingcapital.Input{Dataset: ds}, workingcapital.Options{})
	if !wcResult.Available || len(wcResult.History) != 1 {
		t.Fatalf("expected workingcapital.Result.Available with 1 period, got %+v", wcResult)
	}

	liabilities := wcResult.History[0].OperatingCurrentLiabilities
	if !liabilities.Available {
		t.Fatalf("expected OperatingCurrentLiabilities.Available=true")
	}
	if liabilities.Value != apResult.PortfolioSummary.TotalOpenPayables {
		t.Errorf("OperatingCurrentLiabilities (%v) does not match AP aging total (%v)", liabilities.Value, apResult.PortfolioSummary.TotalOpenPayables)
	}
}
