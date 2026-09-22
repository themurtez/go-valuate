package ingestion_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/ingestion"
	icsv "github.com/themurtez/go-valuate/ingestion/csv"
	ixlsx "github.com/themurtez/go-valuate/ingestion/xlsx"
)

func TestResultJSONRoundTrip(t *testing.T) {
	res, err := icsv.Parse(strings.NewReader("Account,2023,2024\nRevenue,1000,1200\nTotal Revenue,1000,1200\n"), ingestion.Options{})
	if err != nil {
		t.Fatalf("Parse: %+v", err)
	}

	first, jerr := json.Marshal(res)
	if jerr != nil {
		t.Fatalf("Marshal: %v", jerr)
	}
	var decoded ingestion.Result
	if jerr := json.Unmarshal(first, &decoded); jerr != nil {
		t.Fatalf("Unmarshal: %v", jerr)
	}
	second, jerr := json.Marshal(decoded)
	if jerr != nil {
		t.Fatalf("re-Marshal: %v", jerr)
	}
	if string(first) != string(second) {
		t.Errorf("round-trip mismatch:\nfirst:  %s\nsecond: %s", first, second)
	}
}

func TestResultSchemaVersionPopulatedByCSV(t *testing.T) {
	res, err := icsv.Parse(strings.NewReader("Account,2023,2024\nRevenue,1000,1200\n"), ingestion.Options{})
	if err != nil {
		t.Fatalf("Parse: %+v", err)
	}
	if res.SchemaVersion != ingestion.SchemaVersion {
		t.Errorf("SchemaVersion = %q, want %q", res.SchemaVersion, ingestion.SchemaVersion)
	}
	if res.SchemaVersion == "" {
		t.Error("SchemaVersion must not be empty")
	}
}

func TestResultSchemaVersionPopulatedByXLSX(t *testing.T) {
	res, err := ixlsx.Parse(openFixture(t, "multi_sheet_workbook.xlsx"), ingestion.Options{})
	if err != nil {
		t.Fatalf("Parse: %+v", err)
	}
	if res.SchemaVersion != ingestion.SchemaVersion {
		t.Errorf("SchemaVersion = %q, want %q", res.SchemaVersion, ingestion.SchemaVersion)
	}
}

// TestResultSchemaVersionPopulatedOnAmbiguousSheetSelection covers the one
// Result construction path that does not go through the shared
// ingestion.BuildResult entry point (xlsx.Parse's early return when sheet
// selection is ambiguous — see ingestion/xlsx/parser.go) to guard against
// SchemaVersion silently regressing to empty on that path specifically.
func TestResultSchemaVersionPopulatedOnAmbiguousSheetSelection(t *testing.T) {
	res, err := ixlsx.Parse(openFixture(t, "ambiguous_workbook.xlsx"), ingestion.Options{})
	if err != nil {
		t.Fatalf("Parse: %+v", err)
	}
	if res.SchemaVersion != ingestion.SchemaVersion {
		t.Errorf("SchemaVersion = %q, want %q", res.SchemaVersion, ingestion.SchemaVersion)
	}
}

func TestToRawLineItemsFieldMapping(t *testing.T) {
	res, err := icsv.Parse(strings.NewReader("Account,2023,2024\nRevenue,1000,1200\n"), ingestion.Options{})
	if err != nil {
		t.Fatalf("Parse: %+v", err)
	}
	items := res.ToRawLineItems()
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	item := items[0]
	if item.ID != "sheet-0-row-1" {
		t.Errorf("ID = %q, want sheet-0-row-1", item.ID)
	}
	if item.StatementType != res.Metadata.StatementType {
		t.Errorf("StatementType = %q, want %q", item.StatementType, res.Metadata.StatementType)
	}
	if item.Label != "Revenue" {
		t.Errorf("Label = %q, want Revenue", item.Label)
	}
	if item.Values[financial.Period("2023")] != 1000 {
		t.Errorf("Values[2023] = %v, want 1000", item.Values[financial.Period("2023")])
	}
	if item.Kind != financial.RowKindNormal {
		t.Errorf("Kind = %q, want %q (zero value) for an ordinary row", item.Kind, financial.RowKindNormal)
	}
}

// TestToRawLineItemsKindMapping confirms every ingestion.StructuralKind
// value a Row can carry (other than StructuralBlank, which never reaches
// Result.Rows — see that field's doc comment) translates to the correct
// financial.RowKind on the resulting RawLineItem, and that heading rows —
// previously dropped by ToRawLineItems() entirely — now survive.
func TestToRawLineItemsKindMapping(t *testing.T) {
	input := "Account,2024\n" +
		"Operating Expenses,\n" + // heading: label, no numeric value
		"  Advertising,5000\n" + // normal
		"Total Operating Expenses,5000\n" + // subtotal
		"Net Income,5000\n" // total (bareStatementTotalPhrases)

	res, err := icsv.Parse(strings.NewReader(input), ingestion.Options{})
	if err != nil {
		t.Fatalf("Parse: %+v", err)
	}
	items := res.ToRawLineItems()

	want := map[string]financial.RowKind{
		"Operating Expenses":       financial.RowKindHeading,
		"Advertising":              financial.RowKindNormal,
		"Total Operating Expenses": financial.RowKindSubtotal,
		"Net Income":               financial.RowKindTotal,
	}
	got := make(map[string]financial.RowKind, len(items))
	for _, item := range items {
		got[item.Label] = item.Kind
	}
	for label, wantKind := range want {
		gotKind, ok := got[label]
		if !ok {
			t.Errorf("row %q missing from ToRawLineItems() output", label)
			continue
		}
		if gotKind != wantKind {
			t.Errorf("row %q Kind = %q, want %q", label, gotKind, wantKind)
		}
	}
	if len(got) != len(want) {
		t.Errorf("got %d rows, want %d: %v", len(got), len(want), got)
	}
}

func TestDefaultLimits(t *testing.T) {
	d := ingestion.DefaultLimits()
	if d.MaxFileSizeBytes <= 0 {
		t.Error("MaxFileSizeBytes should be positive")
	}
	if d.MaxRows <= 0 {
		t.Error("MaxRows should be positive")
	}
	if d.MaxColumns <= 0 {
		t.Error("MaxColumns should be positive")
	}
	if d.MaxSheets <= 0 {
		t.Error("MaxSheets should be positive")
	}
	if d.MaxCellTextLength <= 0 {
		t.Error("MaxCellTextLength should be positive")
	}
}

func TestErrorImplementsError(t *testing.T) {
	var err error = &ingestion.Error{Code: ingestion.ErrCodeInvalidFile, Message: "boom"}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("Error() = %q, want it to contain %q", err.Error(), "boom")
	}
}

func TestErrorWithDetail(t *testing.T) {
	err := &ingestion.Error{Code: ingestion.ErrCodeLimitExceeded, Message: "too big", Detail: "limit is 10 bytes"}
	got := err.Error()
	if !strings.Contains(got, "too big") || !strings.Contains(got, "limit is 10 bytes") {
		t.Errorf("Error() = %q, want it to contain both message and detail", got)
	}
}
