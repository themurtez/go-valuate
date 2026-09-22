package reconciliation

import (
	"testing"

	"github.com/themurtez/go-valuate/financial"
)

func item(code financial.Code, period string, amount float64) financial.NormalizedItem {
	return financial.NormalizedItem{Code: code, Period: financial.Period(period), Amount: amount}
}

func ptr(v float64) *float64 { return &v }

// baseIncomeStatementDataset returns a minimal, internally-consistent
// income statement for one period ("2025"): revenue 620000, COGS 200000
// (gross profit 420000), opex 150000 (EBIT 270000), D&A 30000+10000
// (EBITDA 310000), no below-the-line items (net income == EBIT == 270000).
func baseIncomeStatementDataset() financial.FinancialDataset {
	return financial.FinancialDataset{
		Currency: "USD",
		Items: []financial.NormalizedItem{
			item(financial.CodeRevProduct, "2025", 620000),
			item(financial.CodeCogsMaterial, "2025", 200000),
			item(financial.CodeOpexPayroll, "2025", 150000),
			item(financial.CodeDepreciation, "2025", 30000),
			item(financial.CodeAmortization, "2025", 10000),
		},
	}
}

func TestGrossProfit_ExactPass(t *testing.T) {
	ds := baseIncomeStatementDataset()
	result := Run(ds, Options{
		Reported: ReportedTotals{"2025": {GrossProfit: ptr(420000)}},
	})
	check := mustFindCheck(t, result, CheckGrossProfit, "2025")
	if check.Status != StatusPass {
		t.Errorf("status = %v, want PASS: %s", check.Status, check.Explanation)
	}
	if check.Difference == nil || *check.Difference != 0 {
		t.Errorf("difference = %v, want 0", check.Difference)
	}
}

func TestGrossProfit_PassWithinTolerance(t *testing.T) {
	ds := baseIncomeStatementDataset()
	// reported 420001 vs reconstructed 420000 -> difference 1, default
	// tolerance absolute 1.0 -> PASS.
	result := Run(ds, Options{
		Reported: ReportedTotals{"2025": {GrossProfit: ptr(420001)}},
	})
	check := mustFindCheck(t, result, CheckGrossProfit, "2025")
	if check.Status != StatusPass {
		t.Errorf("status = %v, want PASS (within default tolerance): %s", check.Status, check.Explanation)
	}
}

func TestGrossProfit_FailOutsideTolerance(t *testing.T) {
	ds := baseIncomeStatementDataset()
	result := Run(ds, Options{
		Reported:  ReportedTotals{"2025": {GrossProfit: ptr(425000)}},
		Tolerance: Tolerance{Absolute: 1.0},
	})
	check := mustFindCheck(t, result, CheckGrossProfit, "2025")
	if check.Status != StatusFail {
		t.Errorf("status = %v, want FAIL: %s", check.Status, check.Explanation)
	}
	if check.Difference == nil || *check.Difference != -5000 {
		t.Errorf("difference = %v, want -5000", check.Difference)
	}
}

func TestGrossProfit_NotApplicableWhenNoReportedValueSupplied(t *testing.T) {
	ds := baseIncomeStatementDataset()
	result := Run(ds, Options{})
	check := mustFindCheck(t, result, CheckGrossProfit, "2025")
	if check.Status != StatusNotApplicable {
		t.Errorf("status = %v, want NOT_APPLICABLE when no reported gross profit is supplied", check.Status)
	}
	if check.Expected != nil || check.Actual != nil {
		t.Error("expected NOT_APPLICABLE check to have no Expected/Actual values")
	}
}

func TestGrossProfit_NotApplicableWhenReconstructionUnavailable(t *testing.T) {
	// No revenue or COGS codes at all -> gross profit cannot be
	// reconstructed, even though a reported value was supplied.
	ds := financial.FinancialDataset{
		Currency: "USD",
		Items: []financial.NormalizedItem{
			item(financial.CodeOpexPayroll, "2025", 50000),
		},
	}
	result := Run(ds, Options{
		Reported: ReportedTotals{"2025": {GrossProfit: ptr(100000)}},
	})
	check := mustFindCheck(t, result, CheckGrossProfit, "2025")
	if check.Status != StatusNotApplicable {
		t.Errorf("status = %v, want NOT_APPLICABLE when gross profit cannot be reconstructed", check.Status)
	}
}

func TestOperatingIncome_ExactPass(t *testing.T) {
	ds := baseIncomeStatementDataset()
	result := Run(ds, Options{
		Reported: ReportedTotals{"2025": {OperatingIncome: ptr(270000)}},
	})
	check := mustFindCheck(t, result, CheckOperatingIncome, "2025")
	if check.Status != StatusPass {
		t.Errorf("status = %v, want PASS: %s", check.Status, check.Explanation)
	}
}

func TestOperatingIncome_NotApplicableWithoutReportedValue(t *testing.T) {
	ds := baseIncomeStatementDataset()
	result := Run(ds, Options{})
	check := mustFindCheck(t, result, CheckOperatingIncome, "2025")
	if check.Status != StatusNotApplicable {
		t.Errorf("status = %v, want NOT_APPLICABLE", check.Status)
	}
}

func TestEBITDABridge_ExactPass(t *testing.T) {
	ds := baseIncomeStatementDataset()
	result := Run(ds, Options{
		Reported: ReportedTotals{"2025": {EBITDA: ptr(310000)}},
	})
	check := mustFindCheck(t, result, CheckEBITDABridge, "2025")
	if check.Status != StatusPass {
		t.Errorf("status = %v, want PASS: %s", check.Status, check.Explanation)
	}
	if check.Actual == nil || *check.Actual != 310000 {
		t.Errorf("actual = %v, want 310000", check.Actual)
	}
}

func TestEBITDABridge_NotApplicableWhenNoReportedEBITDA(t *testing.T) {
	// Most statements never report EBITDA at all; this must not be
	// mistaken for a failure.
	ds := baseIncomeStatementDataset()
	result := Run(ds, Options{})
	check := mustFindCheck(t, result, CheckEBITDABridge, "2025")
	if check.Status != StatusNotApplicable {
		t.Errorf("status = %v, want NOT_APPLICABLE", check.Status)
	}
}

func TestEBITDABridge_FailOutsideTolerance(t *testing.T) {
	ds := baseIncomeStatementDataset()
	result := Run(ds, Options{
		Reported:  ReportedTotals{"2025": {EBITDA: ptr(350000)}},
		Tolerance: Tolerance{Absolute: 1.0},
	})
	check := mustFindCheck(t, result, CheckEBITDABridge, "2025")
	if check.Status != StatusFail {
		t.Errorf("status = %v, want FAIL: %s", check.Status, check.Explanation)
	}
}

func TestNetIncome_ExactPassWithNoBelowTheLineItems(t *testing.T) {
	ds := baseIncomeStatementDataset()
	result := Run(ds, Options{
		Reported: ReportedTotals{"2025": {NetIncome: ptr(270000)}},
	})
	check := mustFindCheck(t, result, CheckNetIncome, "2025")
	if check.Status != StatusPass {
		t.Errorf("status = %v, want PASS: %s", check.Status, check.Explanation)
	}
}

func TestNetIncome_ExactPassWithFullBelowTheLineBridge(t *testing.T) {
	ds := baseIncomeStatementDataset()
	ds.Items = append(ds.Items,
		item(financial.CodeOtherIncome, "2025", 5000),
		item(financial.CodeInterestIncome, "2025", 2000),
		item(financial.CodeInterestExpense, "2025", 8000),
		item(financial.CodeOtherExpense, "2025", 1000),
		item(financial.CodeIncomeTax, "2025", 60000),
	)
	// Net income = 270000 + 5000 + 2000 - 8000 - 1000 - 60000 = 208000
	result := Run(ds, Options{
		Reported: ReportedTotals{"2025": {NetIncome: ptr(208000)}},
	})
	check := mustFindCheck(t, result, CheckNetIncome, "2025")
	if check.Status != StatusPass {
		t.Errorf("status = %v, want PASS: %s", check.Status, check.Explanation)
	}
	if check.Actual == nil || *check.Actual != 208000 {
		t.Errorf("actual = %v, want 208000", check.Actual)
	}
}

func TestNetIncome_NotApplicableWhenEBITUnavailable(t *testing.T) {
	ds := financial.FinancialDataset{Currency: "USD"}
	result := Run(ds, Options{
		Reported: ReportedTotals{"2025": {NetIncome: ptr(1000)}},
	})
	// No period "2025" exists in an empty dataset, so no such check runs at
	// all; verify instead with a dataset that has the period but no
	// income statement codes.
	if len(result.Checks) == 0 {
		t.Fatal("expected at least the dataset-level integrity checks to run")
	}

	dsWithPeriod := financial.FinancialDataset{
		Currency: "USD",
		Items:    []financial.NormalizedItem{item(financial.CodeBsCash, "2025", 500)},
	}
	result2 := Run(dsWithPeriod, Options{
		Reported: ReportedTotals{"2025": {NetIncome: ptr(1000)}},
	})
	check := mustFindCheck(t, result2, CheckNetIncome, "2025")
	if check.Status != StatusNotApplicable {
		t.Errorf("status = %v, want NOT_APPLICABLE when EBIT cannot be reconstructed", check.Status)
	}
}

// mustFindCheck returns the single Check matching code and period, failing
// the test if it's absent or duplicated.
func mustFindCheck(t *testing.T, result Result, code CheckCode, period string) Check {
	t.Helper()
	var found *Check
	for i := range result.Checks {
		c := result.Checks[i]
		if c.Code == code && c.Period == financial.Period(period) {
			if found != nil {
				t.Fatalf("found more than one Check with code %s period %s", code, period)
			}
			found = &c
		}
	}
	if found == nil {
		t.Fatalf("no Check found with code %s period %s", code, period)
	}
	return *found
}
