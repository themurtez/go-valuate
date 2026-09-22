package ingestion_test

import (
	"os"
	"testing"

	"github.com/themurtez/go-valuate/ingestion"
	icsv "github.com/themurtez/go-valuate/ingestion/csv"
	ixlsx "github.com/themurtez/go-valuate/ingestion/xlsx"
)

func openFixture(t *testing.T, name string) *os.File {
	t.Helper()
	f, err := os.Open("fixtures/" + name)
	if err != nil {
		t.Fatalf("open fixture %q: %v", name, err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

func TestFixtureQuickBooksPL(t *testing.T) {
	res, err := icsv.Parse(openFixture(t, "quickbooks_pl.csv"), ingestion.Options{})
	if err != nil {
		t.Fatalf("Parse: %+v", err)
	}
	if len(res.Rows) == 0 {
		t.Fatal("expected rows")
	}
	if res.Metadata.StatementType != "income_statement" {
		t.Errorf("StatementType = %q, want income_statement", res.Metadata.StatementType)
	}
}

func TestFixtureAccountantCustomPL(t *testing.T) {
	res, err := icsv.Parse(openFixture(t, "accountant_custom_pl.csv"), ingestion.Options{})
	if err != nil {
		t.Fatalf("Parse: %+v", err)
	}
	if len(res.Metadata.Periods) != 2 {
		t.Fatalf("got %d periods, want 2", len(res.Metadata.Periods))
	}
}

func TestFixtureBalanceSheet(t *testing.T) {
	res, err := icsv.Parse(openFixture(t, "balance_sheet.csv"), ingestion.Options{})
	if err != nil {
		t.Fatalf("Parse: %+v", err)
	}
	var totalAssetsFound bool
	for _, r := range res.Rows {
		if r.Label == "Total Assets" {
			totalAssetsFound = true
			if r.Kind != ingestion.StructuralTotal {
				t.Errorf("Total Assets Kind = %q, want total", r.Kind)
			}
		}
	}
	if !totalAssetsFound {
		t.Error("Total Assets row not found")
	}
}

func TestFixtureMultiYearSaaSPL(t *testing.T) {
	res, err := icsv.Parse(openFixture(t, "multi_year_saas_pl.csv"), ingestion.Options{})
	if err != nil {
		t.Fatalf("Parse: %+v", err)
	}
	if len(res.Metadata.Periods) != 4 {
		t.Fatalf("got %d periods, want 4", len(res.Metadata.Periods))
	}
}

func TestFixtureMalformedNumericValues(t *testing.T) {
	res, err := icsv.Parse(openFixture(t, "malformed_numeric_values.csv"), ingestion.Options{})
	if err != nil {
		t.Fatalf("Parse: %+v", err)
	}
	count := 0
	for _, w := range res.Warnings {
		if w.Code == ingestion.WarnUnparseableNumericCell {
			count++
		}
	}
	// N/A, TBD, "36,000.00 USD", 12.5% (rejected as percentage), and
	// "--pending--" are all malformed/ambiguous in this fixture.
	if count < 4 {
		t.Errorf("got %d UNPARSEABLE_NUMERIC_CELL warnings, want at least 4", count)
	}
}

func TestFixtureSparseBlankRows(t *testing.T) {
	res, err := icsv.Parse(openFixture(t, "sparse_blank_rows.csv"), ingestion.Options{})
	if err != nil {
		t.Fatalf("Parse: %+v", err)
	}
	for _, r := range res.Rows {
		if r.Kind == ingestion.StructuralBlank {
			t.Error("blank rows must not appear in Result.Rows")
		}
	}
	var netIncomeFound bool
	for _, r := range res.Rows {
		if r.Label == "Net Income" {
			netIncomeFound = true
		}
	}
	if !netIncomeFound {
		t.Error("Net Income row not found despite surrounding blank rows")
	}
}

func TestFixtureMultiSheetWorkbook(t *testing.T) {
	res, err := ixlsx.Parse(openFixture(t, "multi_sheet_workbook.xlsx"), ingestion.Options{})
	if err != nil {
		t.Fatalf("Parse: %+v", err)
	}
	if len(res.Metadata.Sheets) != 3 {
		t.Fatalf("got %d sheets, want 3", len(res.Metadata.Sheets))
	}
	selected := res.Metadata.Sheets[res.Metadata.SelectedSheetIndex]
	if selected.Name != "Income Statement" {
		t.Errorf("selected sheet = %q, want Income Statement", selected.Name)
	}
}

func TestFixtureAmbiguousWorkbook(t *testing.T) {
	res, err := ixlsx.Parse(openFixture(t, "ambiguous_workbook.xlsx"), ingestion.Options{})
	if err != nil {
		t.Fatalf("Parse: %+v", err)
	}
	found := false
	for _, w := range res.Warnings {
		if w.Code == ingestion.WarnMultiplePlausibleSheets {
			found = true
		}
	}
	if !found {
		t.Error("expected WarnMultiplePlausibleSheets")
	}
}

func TestFixtureAmbiguousWorkbookResolvedByExplicitSheet(t *testing.T) {
	res, err := ixlsx.Parse(openFixture(t, "ambiguous_workbook.xlsx"), ingestion.Options{SheetName: "Q1 Data"})
	if err != nil {
		t.Fatalf("Parse: %+v", err)
	}
	if len(res.Rows) == 0 {
		t.Error("expected rows once sheet is explicitly selected")
	}
}

func TestFixtureTotalsSubtotalsWorkbook(t *testing.T) {
	res, err := ixlsx.Parse(openFixture(t, "totals_subtotals.xlsx"), ingestion.Options{})
	if err != nil {
		t.Fatalf("Parse: %+v", err)
	}
	kinds := map[string]ingestion.StructuralKind{}
	for _, r := range res.Rows {
		kinds[r.Label] = r.Kind
	}
	if kinds["Total Operating Expenses"] != ingestion.StructuralSubtotal {
		t.Errorf("Total Operating Expenses Kind = %q, want subtotal", kinds["Total Operating Expenses"])
	}
	if kinds["Net Income"] != ingestion.StructuralTotal {
		t.Errorf("Net Income Kind = %q, want total", kinds["Net Income"])
	}
}
