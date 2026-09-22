package reconciliation

import (
	"testing"

	"github.com/themurtez/go-valuate/financial"
)

func balancedBalanceSheetDataset() financial.FinancialDataset {
	return financial.FinancialDataset{
		Currency: "USD",
		Items: []financial.NormalizedItem{
			item(financial.CodeBsCash, "2025", 100000),
			item(financial.CodeBsAccountsReceivable, "2025", 80000),
			item(financial.CodeBsInventory, "2025", 60000),
			item(financial.CodeBsFixedAssets, "2025", 100000),
			item(financial.CodeBsAccumDepreciation, "2025", 20000),
			// total assets = 100000+80000+60000+100000-20000 = 320000

			item(financial.CodeBsAccountsPayable, "2025", 50000),
			item(financial.CodeBsShortTermDebt, "2025", 30000),
			item(financial.CodeBsLongTermDebt, "2025", 100000),
			item(financial.CodeBsOwnerEquity, "2025", 140000),
			// total liab+equity = 50000+30000+100000+140000 = 320000
		},
	}
}

func TestBalanceSheetBalances_ExactPass(t *testing.T) {
	ds := balancedBalanceSheetDataset()
	result := Run(ds, Options{})
	check := mustFindCheck(t, result, CheckBalanceSheetBalances, "2025")
	if check.Status != StatusPass {
		t.Errorf("status = %v, want PASS: %s", check.Status, check.Explanation)
	}
	if check.Actual == nil || *check.Actual != 320000 {
		t.Errorf("actual (assets) = %v, want 320000", check.Actual)
	}
	if check.Expected == nil || *check.Expected != 320000 {
		t.Errorf("expected (liab+equity) = %v, want 320000", check.Expected)
	}
}

func TestBalanceSheetBalances_PassWithinTolerance(t *testing.T) {
	ds := balancedBalanceSheetDataset()
	// Nudge owner equity down by 1 to create a $1 rounding difference.
	for i, it := range ds.Items {
		if it.Code == financial.CodeBsOwnerEquity {
			ds.Items[i].Amount -= 1
		}
	}
	result := Run(ds, Options{Tolerance: Tolerance{Absolute: 2.0}})
	check := mustFindCheck(t, result, CheckBalanceSheetBalances, "2025")
	if check.Status != StatusPass {
		t.Errorf("status = %v, want PASS (within tolerance): %s", check.Status, check.Explanation)
	}
}

func TestBalanceSheetBalances_FailOutsideTolerance(t *testing.T) {
	ds := balancedBalanceSheetDataset()
	for i, it := range ds.Items {
		if it.Code == financial.CodeBsOwnerEquity {
			ds.Items[i].Amount -= 50000
		}
	}
	result := Run(ds, Options{Tolerance: Tolerance{Absolute: 1.0}})
	check := mustFindCheck(t, result, CheckBalanceSheetBalances, "2025")
	if check.Status != StatusFail {
		t.Errorf("status = %v, want FAIL: %s", check.Status, check.Explanation)
	}
	if check.Difference == nil || *check.Difference != 50000 {
		t.Errorf("difference = %v, want 50000", check.Difference)
	}
}

func TestBalanceSheetBalances_NotApplicableWhenNoBalanceSheetData(t *testing.T) {
	ds := financial.FinancialDataset{
		Currency: "USD",
		Items:    []financial.NormalizedItem{item(financial.CodeRevProduct, "2025", 1000)},
	}
	result := Run(ds, Options{})
	check := mustFindCheck(t, result, CheckBalanceSheetBalances, "2025")
	if check.Status != StatusNotApplicable {
		t.Errorf("status = %v, want NOT_APPLICABLE for an income-statement-only dataset", check.Status)
	}
}

func TestBalanceSheetBalances_AccumulatedDepreciationTreatedAsContraAsset(t *testing.T) {
	// Fixed assets 100000, accum dep 100000 -> net fixed assets 0. If this
	// were NOT treated as a contra-asset, assets would be double what
	// liab+equity report, and the check would incorrectly FAIL.
	ds := financial.FinancialDataset{
		Currency: "USD",
		Items: []financial.NormalizedItem{
			item(financial.CodeBsCash, "2025", 50000),
			item(financial.CodeBsFixedAssets, "2025", 100000),
			item(financial.CodeBsAccumDepreciation, "2025", 100000),
			item(financial.CodeBsOwnerEquity, "2025", 50000),
		},
	}
	result := Run(ds, Options{})
	check := mustFindCheck(t, result, CheckBalanceSheetBalances, "2025")
	if check.Status != StatusPass {
		t.Errorf("status = %v, want PASS (accumulated depreciation must reduce assets): %s", check.Status, check.Explanation)
	}
}

func TestCurrentAssetsSubtotal_PassWhenCalculable(t *testing.T) {
	ds := balancedBalanceSheetDataset()
	result := Run(ds, Options{})
	check := mustFindCheck(t, result, CheckCurrentAssetsSubtotal, "2025")
	if check.Status != StatusPass {
		t.Errorf("status = %v, want PASS: %s", check.Status, check.Explanation)
	}
	want := 100000.0 + 80000.0 + 60000.0
	if check.Actual == nil || *check.Actual != want {
		t.Errorf("actual = %v, want %v", check.Actual, want)
	}
}

func TestCurrentAssetsSubtotal_WarningWhenUnavailable(t *testing.T) {
	ds := financial.FinancialDataset{
		Currency: "USD",
		Items:    []financial.NormalizedItem{item(financial.CodeRevProduct, "2025", 1000)},
	}
	result := Run(ds, Options{})
	check := mustFindCheck(t, result, CheckCurrentAssetsSubtotal, "2025")
	if check.Status != StatusWarning {
		t.Errorf("status = %v, want WARNING when no current asset codes are present", check.Status)
	}
}

func TestWorkingCapitalCalculated_Pass(t *testing.T) {
	ds := balancedBalanceSheetDataset()
	result := Run(ds, Options{})
	check := mustFindCheck(t, result, CheckWorkingCapitalCalculated, "2025")
	if check.Status != StatusPass {
		t.Errorf("status = %v, want PASS: %s", check.Status, check.Explanation)
	}
	// current assets 240000 - current liabilities (50000 AP + 30000 STD) = 160000
	want := 160000.0
	if check.Actual == nil || *check.Actual != want {
		t.Errorf("actual = %v, want %v", check.Actual, want)
	}
}

func TestDebtTotalsCalculated_Pass(t *testing.T) {
	ds := balancedBalanceSheetDataset()
	result := Run(ds, Options{})
	check := mustFindCheck(t, result, CheckDebtTotalsCalculated, "2025")
	if check.Status != StatusPass {
		t.Errorf("status = %v, want PASS: %s", check.Status, check.Explanation)
	}
	want := 130000.0 // 30000 short-term + 100000 long-term
	if check.Actual == nil || *check.Actual != want {
		t.Errorf("actual = %v, want %v", check.Actual, want)
	}
}

func TestBalanceSheetHasLiabilitiesOrEquity_WarningWhenMissing(t *testing.T) {
	ds := financial.FinancialDataset{
		Currency: "USD",
		Items:    []financial.NormalizedItem{item(financial.CodeBsCash, "2025", 5000)},
	}
	result := Run(ds, Options{})
	check := mustFindCheck(t, result, CheckBalanceSheetHasLiabilitiesOrEquity, "2025")
	if check.Status != StatusWarning {
		t.Errorf("status = %v, want WARNING: %s", check.Status, check.Explanation)
	}
}

func TestBalanceSheetHasLiabilitiesOrEquity_NotApplicableWhenNoAssets(t *testing.T) {
	ds := financial.FinancialDataset{
		Currency: "USD",
		Items:    []financial.NormalizedItem{item(financial.CodeRevProduct, "2025", 1000)},
	}
	result := Run(ds, Options{})
	check := mustFindCheck(t, result, CheckBalanceSheetHasLiabilitiesOrEquity, "2025")
	if check.Status != StatusNotApplicable {
		t.Errorf("status = %v, want NOT_APPLICABLE", check.Status)
	}
}

func TestBalanceSheetHasLiabilitiesOrEquity_PassWhenBothPresent(t *testing.T) {
	ds := balancedBalanceSheetDataset()
	result := Run(ds, Options{})
	check := mustFindCheck(t, result, CheckBalanceSheetHasLiabilitiesOrEquity, "2025")
	if check.Status != StatusPass {
		t.Errorf("status = %v, want PASS: %s", check.Status, check.Explanation)
	}
}
