package xlsx_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/ingestion"
	ixlsx "github.com/themurtez/go-valuate/ingestion/xlsx"
)

// newWorkbook builds an in-memory XLSX file from a sheetName -> rows map,
// where each row is a slice of cell values (string or float64), and
// returns its bytes. sheetOrder controls the order sheets are added (and
// therefore which becomes the active/default sheet).
func newWorkbook(t *testing.T, sheetOrder []string, sheets map[string][][]interface{}) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()

	created := map[string]bool{}
	for _, name := range sheetOrder {
		if name == "Sheet1" {
			created[name] = true
			continue
		}
		if _, err := f.NewSheet(name); err != nil {
			t.Fatalf("NewSheet(%q): %v", name, err)
		}
		created[name] = true
	}
	if !created["Sheet1"] {
		f.DeleteSheet("Sheet1")
	}

	for _, name := range sheetOrder {
		rows := sheets[name]
		for r, row := range rows {
			cell, _ := excelize.CoordinatesToCellName(1, r+1)
			rowCopy := row
			if err := f.SetSheetRow(name, cell, &rowCopy); err != nil {
				t.Fatalf("SetSheetRow(%q, row %d): %v", name, r, err)
			}
		}
	}
	if len(sheetOrder) > 0 {
		idx, _ := f.GetSheetIndex(sheetOrder[0])
		f.SetActiveSheet(idx)
	}

	var buf bytes.Buffer
	if _, err := f.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	return buf.Bytes()
}

func mustParseBytes(t *testing.T, data []byte, opts ingestion.Options) *ingestion.Result {
	t.Helper()
	res, err := ixlsx.Parse(bytes.NewReader(data), opts)
	if err != nil {
		t.Fatalf("Parse failed: %+v", err)
	}
	return res
}

func TestXLSXBasicPL(t *testing.T) {
	data := newWorkbook(t, []string{"Sheet1"}, map[string][][]interface{}{
		"Sheet1": {
			{"Account", "2023", "2024"},
			{"Revenue", 1000.0, 1200.0},
			{"COGS", 400.0, 450.0},
		},
	})
	res := mustParseBytes(t, data, ingestion.Options{})
	if len(res.Rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(res.Rows))
	}
	if res.Rows[0].Values["2023"] != 1000 {
		t.Errorf("Values[2023] = %v, want 1000", res.Rows[0].Values["2023"])
	}
}

func TestXLSXSheetAutoSelection(t *testing.T) {
	data := newWorkbook(t, []string{"Notes", "P&L"}, map[string][][]interface{}{
		"Notes": {
			{"Some notes about this workbook"},
		},
		"P&L": {
			{"Account", "2024"},
			{"Revenue", 1000.0},
		},
	})
	res := mustParseBytes(t, data, ingestion.Options{})
	if res.Metadata.Sheets[res.Metadata.SelectedSheetIndex].Name != "P&L" {
		t.Errorf("selected sheet = %q, want P&L", res.Metadata.Sheets[res.Metadata.SelectedSheetIndex].Name)
	}
}

func TestXLSXExplicitSheetSelection(t *testing.T) {
	data := newWorkbook(t, []string{"Sheet1", "Sheet2"}, map[string][][]interface{}{
		"Sheet1": {{"Account", "2024"}, {"Revenue", 1000.0}},
		"Sheet2": {{"Account", "2024"}, {"COGS", 500.0}},
	})
	res := mustParseBytes(t, data, ingestion.Options{SheetName: "Sheet2"})
	if res.Rows[0].Label != "COGS" {
		t.Errorf("Label = %q, want COGS", res.Rows[0].Label)
	}
}

func TestXLSXExplicitSheetSelectionNotFound(t *testing.T) {
	data := newWorkbook(t, []string{"Sheet1"}, map[string][][]interface{}{
		"Sheet1": {{"Account", "2024"}, {"Revenue", 1000.0}},
	})
	_, err := ixlsx.Parse(bytes.NewReader(data), ingestion.Options{SheetName: "DoesNotExist"})
	if err == nil {
		t.Fatal("expected fatal error for nonexistent sheet name")
	}
	if err.Code != ingestion.ErrCodeInvalidFile {
		t.Errorf("Code = %q, want %q", err.Code, ingestion.ErrCodeInvalidFile)
	}
}

func TestXLSXAmbiguousSheets(t *testing.T) {
	// Two equally plausible, equally sized, neutrally-named sheets: sheet
	// selection must not silently guess.
	data := newWorkbook(t, []string{"Sheet1", "Sheet2"}, map[string][][]interface{}{
		"Sheet1": {{"Account", "2024"}, {"Revenue", 1000.0}},
		"Sheet2": {{"Account", "2024"}, {"Expense", 500.0}},
	})
	res := mustParseBytes(t, data, ingestion.Options{})
	if len(res.Rows) != 0 {
		t.Errorf("expected no rows when sheet selection is ambiguous, got %d", len(res.Rows))
	}
	found := false
	for _, w := range res.Warnings {
		if w.Code == ingestion.WarnMultiplePlausibleSheets {
			found = true
		}
	}
	if !found {
		t.Errorf("expected WarnMultiplePlausibleSheets, got %+v", res.Warnings)
	}
}

func TestXLSXEmptySheetSkipped(t *testing.T) {
	data := newWorkbook(t, []string{"Empty", "P&L"}, map[string][][]interface{}{
		"Empty": {},
		"P&L":   {{"Account", "2024"}, {"Revenue", 1000.0}},
	})
	res := mustParseBytes(t, data, ingestion.Options{})
	if res.Metadata.Sheets[res.Metadata.SelectedSheetIndex].Name != "P&L" {
		t.Errorf("selected = %q, want P&L", res.Metadata.Sheets[res.Metadata.SelectedSheetIndex].Name)
	}
	found := false
	for _, w := range res.Warnings {
		if w.Code == ingestion.WarnEmptySheetSkipped {
			found = true
		}
	}
	if !found {
		t.Error("expected WarnEmptySheetSkipped warning")
	}
}

func TestXLSXTotalsAndSubtotals(t *testing.T) {
	data := newWorkbook(t, []string{"Sheet1"}, map[string][][]interface{}{
		"Sheet1": {
			{"Account", "2024"},
			{"Operating Expenses"},
			{"Advertising", 5000.0},
			{"Payroll", 20000.0},
			{"Total Operating Expenses", 25000.0},
		},
	})
	res := mustParseBytes(t, data, ingestion.Options{})
	var subtotalRow *ingestion.Row
	for i := range res.Rows {
		if res.Rows[i].Label == "Total Operating Expenses" {
			subtotalRow = &res.Rows[i]
		}
	}
	if subtotalRow == nil {
		t.Fatal("subtotal row not found")
	}
	if subtotalRow.Kind != ingestion.StructuralSubtotal {
		t.Errorf("Kind = %q, want subtotal", subtotalRow.Kind)
	}
	if subtotalRow.Status != financial.RowStatusSubtotal {
		t.Errorf("Status = %q, want subtotal", subtotalRow.Status)
	}
}

func TestXLSXParentDetection(t *testing.T) {
	data := newWorkbook(t, []string{"Sheet1"}, map[string][][]interface{}{
		"Sheet1": {
			{"Account", "2024"},
			{"Operating Expenses"},
			{"  Advertising", 5000.0},
		},
	})
	res := mustParseBytes(t, data, ingestion.Options{})
	var adRow *ingestion.Row
	for i := range res.Rows {
		if strings.TrimSpace(res.Rows[i].Label) == "Advertising" {
			adRow = &res.Rows[i]
		}
	}
	if adRow == nil {
		t.Fatal("Advertising row not found")
	}
	if adRow.ParentLabel != "Operating Expenses" {
		t.Errorf("ParentLabel = %q, want %q", adRow.ParentLabel, "Operating Expenses")
	}
}

func TestXLSXFormulaWithoutCachedValue(t *testing.T) {
	f := excelize.NewFile()
	f.SetCellValue("Sheet1", "A1", "Account")
	f.SetCellValue("Sheet1", "B1", "2024")
	f.SetCellValue("Sheet1", "A2", "Revenue")
	f.SetCellFormula("Sheet1", "B2", "=1+1")
	var buf bytes.Buffer
	f.WriteTo(&buf)
	f.Close()

	res := mustParseBytes(t, buf.Bytes(), ingestion.Options{})
	found := false
	for _, w := range res.Warnings {
		if w.Code == ingestion.WarnFormulaWithoutCachedValue {
			found = true
		}
	}
	if !found {
		t.Errorf("expected WarnFormulaWithoutCachedValue, got %+v", res.Warnings)
	}
}

func TestXLSXFormulaWithCachedValue(t *testing.T) {
	// excelize's SetCellValue/SetCellFormula don't support writing both a
	// formula and a cached <v> on the same cell through the public API (a
	// later SetCellValue call replaces the formula rather than adding a
	// cached result alongside it) — see buildRawFormulaCellXLSX, which
	// patches the sheet XML directly the way a real authoring application
	// (Excel, Google Sheets, QuickBooks) would save it, to test this path
	// faithfully.
	data := buildRawFormulaCellXLSX(t)
	res := mustParseBytes(t, data, ingestion.Options{})
	for _, w := range res.Warnings {
		if w.Code == ingestion.WarnFormulaWithoutCachedValue {
			t.Errorf("did not expect WarnFormulaWithoutCachedValue when a cached value is present, got %+v", res.Warnings)
		}
	}
	if res.Rows[0].Values["2024"] != 300 {
		t.Errorf("Values[2024] = %v, want 300 (cached formula result)", res.Rows[0].Values["2024"])
	}
}

func TestXLSXMaxSheetsLimit(t *testing.T) {
	sheets := map[string][][]interface{}{}
	order := []string{"Sheet1"}
	sheets["Sheet1"] = [][]interface{}{{"a"}}
	for i := 0; i < 5; i++ {
		name := "Extra" + string(rune('A'+i))
		order = append(order, name)
		sheets[name] = [][]interface{}{{"a"}}
	}
	data := newWorkbook(t, order, sheets)

	_, err := ixlsx.Parse(bytes.NewReader(data), ingestion.Options{
		Limits: ingestion.Limits{MaxSheets: 3},
	})
	if err == nil {
		t.Fatal("expected fatal error for exceeding max sheets")
	}
	if err.Code != ingestion.ErrCodeLimitExceeded {
		t.Errorf("Code = %q, want %q", err.Code, ingestion.ErrCodeLimitExceeded)
	}
}

func TestXLSXMaxFileSizeLimit(t *testing.T) {
	data := newWorkbook(t, []string{"Sheet1"}, map[string][][]interface{}{
		"Sheet1": {{"Account", "2024"}, {"Revenue", 1000.0}},
	})
	_, err := ixlsx.Parse(bytes.NewReader(data), ingestion.Options{
		Limits: ingestion.Limits{MaxFileSizeBytes: 10},
	})
	if err == nil {
		t.Fatal("expected fatal error for oversized input")
	}
	if err.Code != ingestion.ErrCodeLimitExceeded {
		t.Errorf("Code = %q, want %q", err.Code, ingestion.ErrCodeLimitExceeded)
	}
}

func TestXLSXMalformedWorkbook(t *testing.T) {
	_, err := ixlsx.Parse(strings.NewReader("this is not a valid xlsx file"), ingestion.Options{})
	if err == nil {
		t.Fatal("expected fatal error for malformed workbook")
	}
	if err.Code != ingestion.ErrCodeInvalidFile {
		t.Errorf("Code = %q, want %q", err.Code, ingestion.ErrCodeInvalidFile)
	}
}

func TestXLSXEmptyInput(t *testing.T) {
	_, err := ixlsx.Parse(bytes.NewReader(nil), ingestion.Options{})
	if err == nil {
		t.Fatal("expected fatal error for empty input")
	}
	if err.Code != ingestion.ErrCodeNoTabularData {
		t.Errorf("Code = %q, want %q", err.Code, ingestion.ErrCodeNoTabularData)
	}
}

func TestXLSXStatementDetectionBySheetName(t *testing.T) {
	data := newWorkbook(t, []string{"Balance Sheet"}, map[string][][]interface{}{
		"Balance Sheet": {
			{"Account", "2024"},
			{"Cash", 1000.0},
		},
	})
	res := mustParseBytes(t, data, ingestion.Options{})
	if res.Metadata.StatementType != financial.StatementBalanceSheet {
		t.Errorf("StatementType = %q, want balance_sheet", res.Metadata.StatementType)
	}
}

func TestXLSXStableRowOrderingAndRepeatedParsing(t *testing.T) {
	data := newWorkbook(t, []string{"Sheet1"}, map[string][][]interface{}{
		"Sheet1": {
			{"Account", "2024"},
			{"Zebra", 1.0},
			{"Apple", 2.0},
			{"Mango", 3.0},
		},
	})
	first := mustParseBytes(t, data, ingestion.Options{})
	want := []string{"Zebra", "Apple", "Mango"}
	for i, w := range want {
		if first.Rows[i].Label != w {
			t.Fatalf("Rows[%d].Label = %q, want %q", i, first.Rows[i].Label, w)
		}
	}
	for i := 0; i < 3; i++ {
		got := mustParseBytes(t, data, ingestion.Options{})
		for j := range first.Rows {
			if got.Rows[j].ID != first.Rows[j].ID || got.Rows[j].Label != first.Rows[j].Label {
				t.Fatalf("iteration %d: row %d differs", i, j)
			}
		}
	}
}

func TestXLSXJSONRoundTrip(t *testing.T) {
	data := newWorkbook(t, []string{"Sheet1"}, map[string][][]interface{}{
		"Sheet1": {
			{"Account", "2024"},
			{"Revenue", 1000.0},
		},
	})
	res := mustParseBytes(t, data, ingestion.Options{})
	roundTripJSON(t, res)
}
