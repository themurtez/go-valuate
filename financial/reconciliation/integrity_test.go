package reconciliation

import (
	"math"
	"testing"

	"github.com/themurtez/go-valuate/financial"
)

func TestDatasetNotEmpty_WarningOnEmptyDataset(t *testing.T) {
	result := Run(financial.FinancialDataset{Currency: "USD"}, Options{})
	check := mustFindDatasetCheck(t, result, CheckDatasetNotEmpty)
	if check.Status != StatusWarning {
		t.Errorf("status = %v, want WARNING", check.Status)
	}
}

func TestDatasetNotEmpty_PassWithItems(t *testing.T) {
	ds := financial.FinancialDataset{Currency: "USD", Items: []financial.NormalizedItem{item(financial.CodeRevProduct, "2025", 1000)}}
	result := Run(ds, Options{})
	check := mustFindDatasetCheck(t, result, CheckDatasetNotEmpty)
	if check.Status != StatusPass {
		t.Errorf("status = %v, want PASS", check.Status)
	}
}

func TestEmptyDataset_OnlyProducesDatasetLevelChecks(t *testing.T) {
	result := Run(financial.FinancialDataset{}, Options{})
	for _, c := range result.Checks {
		if c.Period != "" {
			t.Errorf("expected no period-scoped checks for an empty dataset, found %s for period %s", c.Code, c.Period)
		}
	}
}

func TestValidCurrency_FailWhenMissing(t *testing.T) {
	ds := financial.FinancialDataset{Items: []financial.NormalizedItem{item(financial.CodeRevProduct, "2025", 1000)}}
	result := Run(ds, Options{})
	check := mustFindDatasetCheck(t, result, CheckValidCurrency)
	if check.Status != StatusFail {
		t.Errorf("status = %v, want FAIL", check.Status)
	}
}

func TestValidCurrency_PassWhenSet(t *testing.T) {
	ds := financial.FinancialDataset{Currency: "USD", Items: []financial.NormalizedItem{item(financial.CodeRevProduct, "2025", 1000)}}
	result := Run(ds, Options{})
	check := mustFindDatasetCheck(t, result, CheckValidCurrency)
	if check.Status != StatusPass {
		t.Errorf("status = %v, want PASS", check.Status)
	}
}

func TestFiniteValues_FailOnNaN(t *testing.T) {
	ds := financial.FinancialDataset{
		Currency: "USD",
		Items:    []financial.NormalizedItem{item(financial.CodeRevProduct, "2025", math.NaN())},
	}
	result := Run(ds, Options{})
	check := mustFindDatasetCheck(t, result, CheckFiniteValues)
	if check.Status != StatusFail {
		t.Errorf("status = %v, want FAIL", check.Status)
	}
}

func TestFiniteValues_FailOnInfinity(t *testing.T) {
	ds := financial.FinancialDataset{
		Currency: "USD",
		Items:    []financial.NormalizedItem{item(financial.CodeRevProduct, "2025", math.Inf(1))},
	}
	result := Run(ds, Options{})
	check := mustFindDatasetCheck(t, result, CheckFiniteValues)
	if check.Status != StatusFail {
		t.Errorf("status = %v, want FAIL", check.Status)
	}
}

func TestFiniteValues_PassOnOrdinaryValues(t *testing.T) {
	ds := financial.FinancialDataset{
		Currency: "USD",
		Items:    []financial.NormalizedItem{item(financial.CodeRevProduct, "2025", -500.25)},
	}
	result := Run(ds, Options{})
	check := mustFindDatasetCheck(t, result, CheckFiniteValues)
	if check.Status != StatusPass {
		t.Errorf("status = %v, want PASS (negative finite values are valid, e.g. a loss)", check.Status)
	}
}

func TestNoDuplicateEntries_FailOnDuplicateCodePeriod(t *testing.T) {
	// A hand-built dataset bypassing Normalize's own dedup-by-summation can
	// still carry a duplicate (code, period) pair.
	ds := financial.FinancialDataset{
		Currency: "USD",
		Items: []financial.NormalizedItem{
			item(financial.CodeRevProduct, "2025", 1000),
			item(financial.CodeRevProduct, "2025", 2000),
		},
	}
	result := Run(ds, Options{})
	check := mustFindDatasetCheck(t, result, CheckNoDuplicateEntries)
	if check.Status != StatusFail {
		t.Errorf("status = %v, want FAIL", check.Status)
	}
}

func TestNoDuplicateEntries_PassWithoutDuplicates(t *testing.T) {
	ds := financial.FinancialDataset{
		Currency: "USD",
		Items: []financial.NormalizedItem{
			item(financial.CodeRevProduct, "2024", 1000),
			item(financial.CodeRevProduct, "2025", 2000),
		},
	}
	result := Run(ds, Options{})
	check := mustFindDatasetCheck(t, result, CheckNoDuplicateEntries)
	if check.Status != StatusPass {
		t.Errorf("status = %v, want PASS", check.Status)
	}
}

func TestNoUnknownPeriodReferences_FailOnEmptyPeriod(t *testing.T) {
	ds := financial.FinancialDataset{
		Currency: "USD",
		Items:    []financial.NormalizedItem{item(financial.CodeRevProduct, "", 1000)},
	}
	result := Run(ds, Options{})
	check := mustFindDatasetCheck(t, result, CheckNoUnknownPeriods)
	if check.Status != StatusFail {
		t.Errorf("status = %v, want FAIL", check.Status)
	}
}

func TestNoUnknownPeriodReferences_FailOnUnrecognizedCode(t *testing.T) {
	ds := financial.FinancialDataset{
		Currency: "USD",
		Items:    []financial.NormalizedItem{item(financial.Code("NOT_A_REAL_CODE"), "2025", 1000)},
	}
	result := Run(ds, Options{})
	check := mustFindDatasetCheck(t, result, CheckNoUnknownPeriods)
	if check.Status != StatusFail {
		t.Errorf("status = %v, want FAIL", check.Status)
	}
}

func TestNoUnknownPeriodReferences_PassOnWellFormedData(t *testing.T) {
	ds := financial.FinancialDataset{
		Currency: "USD",
		Items:    []financial.NormalizedItem{item(financial.CodeRevProduct, "2025", 1000)},
	}
	result := Run(ds, Options{})
	check := mustFindDatasetCheck(t, result, CheckNoUnknownPeriods)
	if check.Status != StatusPass {
		t.Errorf("status = %v, want PASS", check.Status)
	}
}

func TestNoSuspiciousDuplicateSources_WarningOnRepeatedRowInSources(t *testing.T) {
	ds := financial.FinancialDataset{
		Currency: "USD",
		Items: []financial.NormalizedItem{
			{
				Code: financial.CodeRevProduct, Period: "2025", Amount: 2000,
				Sources: []financial.SourceRef{
					{RowID: "row-1", Period: "2025", Amount: 1000},
					{RowID: "row-1", Period: "2025", Amount: 1000},
				},
			},
		},
	}
	result := Run(ds, Options{})
	check := mustFindDatasetCheck(t, result, CheckNoSuspiciousDuplicateSources)
	if check.Status != StatusWarning {
		t.Errorf("status = %v, want WARNING", check.Status)
	}
}

func TestNoSuspiciousDuplicateSources_PassWithDistinctRows(t *testing.T) {
	ds := financial.FinancialDataset{
		Currency: "USD",
		Items: []financial.NormalizedItem{
			{
				Code: financial.CodeRevProduct, Period: "2025", Amount: 2000,
				Sources: []financial.SourceRef{
					{RowID: "row-1", Period: "2025", Amount: 1000},
					{RowID: "row-2", Period: "2025", Amount: 1000},
				},
			},
		},
	}
	result := Run(ds, Options{})
	check := mustFindDatasetCheck(t, result, CheckNoSuspiciousDuplicateSources)
	if check.Status != StatusPass {
		t.Errorf("status = %v, want PASS", check.Status)
	}
}

func TestNoSuspiciousDuplicateSources_PassWithNoProvenance(t *testing.T) {
	ds := financial.FinancialDataset{
		Currency: "USD",
		Items:    []financial.NormalizedItem{item(financial.CodeRevProduct, "2025", 2000)},
	}
	result := Run(ds, Options{})
	check := mustFindDatasetCheck(t, result, CheckNoSuspiciousDuplicateSources)
	if check.Status != StatusPass {
		t.Errorf("status = %v, want PASS (nothing to inspect without provenance)", check.Status)
	}
}

func TestIncomeStatementHasRevenue_WarningWhenActivityButNoRevenue(t *testing.T) {
	ds := financial.FinancialDataset{
		Currency: "USD",
		Items:    []financial.NormalizedItem{item(financial.CodeOpexPayroll, "2025", 50000)},
	}
	result := Run(ds, Options{})
	check := mustFindCheck(t, result, CheckIncomeStatementHasRevenue, "2025")
	if check.Status != StatusWarning {
		t.Errorf("status = %v, want WARNING", check.Status)
	}
}

func TestIncomeStatementHasRevenue_NotApplicableWhenNoIncomeStatementDataAtAll(t *testing.T) {
	ds := financial.FinancialDataset{
		Currency: "USD",
		Items:    []financial.NormalizedItem{item(financial.CodeBsCash, "2025", 50000)},
	}
	result := Run(ds, Options{})
	check := mustFindCheck(t, result, CheckIncomeStatementHasRevenue, "2025")
	if check.Status != StatusNotApplicable {
		t.Errorf("status = %v, want NOT_APPLICABLE for a balance-sheet-only period", check.Status)
	}
}

func TestIncomeStatementHasRevenue_PassWhenRevenuePresent(t *testing.T) {
	ds := financial.FinancialDataset{
		Currency: "USD",
		Items:    []financial.NormalizedItem{item(financial.CodeRevProduct, "2025", 50000)},
	}
	result := Run(ds, Options{})
	check := mustFindCheck(t, result, CheckIncomeStatementHasRevenue, "2025")
	if check.Status != StatusPass {
		t.Errorf("status = %v, want PASS", check.Status)
	}
}

func TestMultiplePeriods_IntegrityChecksRunPerPeriod(t *testing.T) {
	ds := financial.FinancialDataset{
		Currency: "USD",
		Items: []financial.NormalizedItem{
			item(financial.CodeRevProduct, "2024", 1000),
			item(financial.CodeRevProduct, "2025", 2000),
			item(financial.CodeOpexPayroll, "2025", 500), // no revenue counterpart complaint since 2025 has revenue
		},
	}
	result := Run(ds, Options{})
	c2024 := mustFindCheck(t, result, CheckIncomeStatementHasRevenue, "2024")
	c2025 := mustFindCheck(t, result, CheckIncomeStatementHasRevenue, "2025")
	if c2024.Status != StatusPass {
		t.Errorf("2024 status = %v, want PASS", c2024.Status)
	}
	if c2025.Status != StatusPass {
		t.Errorf("2025 status = %v, want PASS", c2025.Status)
	}
}

// mustFindDatasetCheck returns the single dataset-wide (period-less) Check
// with the given code.
func mustFindDatasetCheck(t *testing.T, result Result, code CheckCode) Check {
	t.Helper()
	var found *Check
	for i := range result.Checks {
		c := result.Checks[i]
		if c.Code == code && c.Period == "" {
			if found != nil {
				t.Fatalf("found more than one dataset-level Check with code %s", code)
			}
			found = &c
		}
	}
	if found == nil {
		t.Fatalf("no dataset-level Check found with code %s", code)
	}
	return *found
}
