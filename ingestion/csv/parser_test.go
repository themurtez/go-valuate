package csv_test

import (
	"strings"
	"testing"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/ingestion"
	icsv "github.com/themurtez/go-valuate/ingestion/csv"
)

func mustParse(t *testing.T, input string, opts ingestion.Options) *ingestion.Result {
	t.Helper()
	res, err := icsv.Parse(strings.NewReader(input), opts)
	if err != nil {
		t.Fatalf("Parse failed: %+v", err)
	}
	return res
}

func TestBasicPL(t *testing.T) {
	input := "Account,2023,2024\nRevenue,1000,1200\nCOGS,400,450\n"
	res := mustParse(t, input, ingestion.Options{})
	if len(res.Rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(res.Rows))
	}
	if res.Rows[0].Label != "Revenue" {
		t.Errorf("Label = %q, want Revenue", res.Rows[0].Label)
	}
	if res.Rows[0].Values["2023"] != 1000 {
		t.Errorf("Values[2023] = %v, want 1000", res.Rows[0].Values["2023"])
	}
}

func TestQuotedLabels(t *testing.T) {
	input := "Account,2024\n\"Advertising, Promotion\",5000\n"
	res := mustParse(t, input, ingestion.Options{})
	if len(res.Rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(res.Rows))
	}
	if res.Rows[0].Label != "Advertising, Promotion" {
		t.Errorf("Label = %q, want %q", res.Rows[0].Label, "Advertising, Promotion")
	}
}

func TestEmbeddedCommasInQuotedValue(t *testing.T) {
	input := "Account,2024\nRevenue,\"1,234,567.89\"\n"
	res := mustParse(t, input, ingestion.Options{})
	if got := res.Rows[0].Values["2024"]; got != 1234567.89 {
		t.Errorf("Values[2024] = %v, want 1234567.89", got)
	}
}

func TestMultiplePeriods(t *testing.T) {
	input := "Account,2021,2022,2023,2024\nRevenue,900,950,1000,1100\n"
	res := mustParse(t, input, ingestion.Options{})
	if len(res.Metadata.Periods) != 4 {
		t.Fatalf("got %d periods, want 4", len(res.Metadata.Periods))
	}
}

func TestAlternateDelimiterSemicolon(t *testing.T) {
	input := "Account;2023;2024\nRevenue;1000;1200\n"
	res := mustParse(t, input, ingestion.Options{})
	if res.Metadata.Delimiter != ";" {
		t.Errorf("Delimiter = %q, want ;", res.Metadata.Delimiter)
	}
	if res.Rows[0].Values["2023"] != 1000 {
		t.Errorf("Values[2023] = %v, want 1000", res.Rows[0].Values["2023"])
	}
}

func TestAlternateDelimiterTab(t *testing.T) {
	input := "Account\t2023\t2024\nRevenue\t1000\t1200\n"
	res := mustParse(t, input, ingestion.Options{})
	if res.Metadata.Delimiter != "\t" {
		t.Errorf("Delimiter = %q, want tab", res.Metadata.Delimiter)
	}
}

func TestForcedDelimiter(t *testing.T) {
	input := "Account|2023|2024\nRevenue|1000|1200\n"
	res := mustParse(t, input, ingestion.Options{Delimiter: '|'})
	if res.Rows[0].Values["2023"] != 1000 {
		t.Errorf("Values[2023] = %v, want 1000", res.Rows[0].Values["2023"])
	}
}

func TestBOMStripped(t *testing.T) {
	input := string([]byte{0xEF, 0xBB, 0xBF}) + "Account,2024\nRevenue,1000\n"
	res := mustParse(t, input, ingestion.Options{})
	if res.Rows[0].Label != "Revenue" {
		t.Errorf("Label = %q, want Revenue (BOM should not corrupt first cell)", res.Rows[0].Label)
	}
}

func TestCRLFLineEndings(t *testing.T) {
	input := "Account,2024\r\nRevenue,1000\r\nCOGS,500\r\n"
	res := mustParse(t, input, ingestion.Options{})
	if len(res.Rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(res.Rows))
	}
}

func TestNegativeParentheses(t *testing.T) {
	input := "Account,2024\nAdvertising,(5000)\n"
	res := mustParse(t, input, ingestion.Options{})
	if got := res.Rows[0].Values["2024"]; got != -5000 {
		t.Errorf("Values[2024] = %v, want -5000", got)
	}
}

func TestBlankValues(t *testing.T) {
	input := "Account,2023,2024\nRevenue,1000,\n"
	res := mustParse(t, input, ingestion.Options{})
	if _, ok := res.Rows[0].Values["2024"]; ok {
		t.Error("expected no value for blank cell")
	}
	if res.Rows[0].Values["2023"] != 1000 {
		t.Errorf("Values[2023] = %v, want 1000", res.Rows[0].Values["2023"])
	}
}

func TestMalformedValueWarns(t *testing.T) {
	input := "Account,2024\nRevenue,N/A\n"
	res := mustParse(t, input, ingestion.Options{})
	found := false
	for _, w := range res.Warnings {
		if w.Code == ingestion.WarnUnparseableNumericCell {
			found = true
		}
	}
	if !found {
		t.Errorf("expected WarnUnparseableNumericCell, got warnings: %+v", res.Warnings)
	}
	if _, ok := res.Rows[0].Values["2024"]; ok {
		t.Error("malformed value must not silently become a value")
	}
}

func TestHeaderDetection(t *testing.T) {
	input := "Acme Corp\nIncome Statement\nAccount,2023,2024\nRevenue,1000,1200\n"
	res := mustParse(t, input, ingestion.Options{})
	if res.Metadata.HeaderRowIndex != 2 {
		t.Errorf("HeaderRowIndex = %d, want 2", res.Metadata.HeaderRowIndex)
	}
	if res.Metadata.StatementType != financial.StatementIncomeStatement {
		t.Errorf("StatementType = %q, want income_statement", res.Metadata.StatementType)
	}
}

func TestLabelColumnOverride(t *testing.T) {
	input := "Code,Account,2024\n4000,Revenue,1000\n"
	col := 1
	res := mustParse(t, input, ingestion.Options{LabelColumnOverride: &col})
	if res.Metadata.LabelColumnIndex != 1 {
		t.Fatalf("LabelColumnIndex = %d, want 1", res.Metadata.LabelColumnIndex)
	}
	if res.Rows[0].Label != "Revenue" {
		t.Errorf("Label = %q, want Revenue", res.Rows[0].Label)
	}
}

func TestHeaderRowOverride(t *testing.T) {
	input := "Some junk,,\nAccount,2023,2024\nRevenue,1000,1200\n"
	row := 1
	res := mustParse(t, input, ingestion.Options{HeaderRowOverride: &row})
	if res.Metadata.HeaderRowIndex != 1 {
		t.Fatalf("HeaderRowIndex = %d, want 1", res.Metadata.HeaderRowIndex)
	}
}

func TestStatementTypeOverride(t *testing.T) {
	input := "Account,2024\nCash,1000\n"
	res := mustParse(t, input, ingestion.Options{StatementTypeOverride: ingestion.StatementOverrideBalanceSheet})
	if res.Metadata.StatementType != financial.StatementBalanceSheet {
		t.Errorf("StatementType = %q, want balance_sheet", res.Metadata.StatementType)
	}
	if res.Metadata.StatementTypeUnknown {
		t.Error("StatementTypeUnknown should be false when overridden")
	}
}

func TestPeriodColumnOverride(t *testing.T) {
	input := "Account,Current,Prior\nRevenue,1000,900\n"
	res := mustParse(t, input, ingestion.Options{
		PeriodColumnOverrides: []ingestion.PeriodColumnOverride{
			{ColumnIndex: 1, Period: "2024"},
			{ColumnIndex: 2, Period: "2023"},
		},
	})
	if res.Rows[0].Values["2024"] != 1000 {
		t.Errorf("Values[2024] = %v, want 1000", res.Rows[0].Values["2024"])
	}
	if res.Rows[0].Values["2023"] != 900 {
		t.Errorf("Values[2023] = %v, want 900", res.Rows[0].Values["2023"])
	}
}

func TestDashTreatmentZero(t *testing.T) {
	input := "Account,2024\nRevenue,-\n"
	res := mustParse(t, input, ingestion.Options{DashTreatment: ingestion.DashAsZero})
	if got, ok := res.Rows[0].Values["2024"]; !ok || got != 0 {
		t.Errorf("Values[2024] = %v, ok=%v; want 0, true", got, ok)
	}
}

func TestDashTreatmentBlankDefault(t *testing.T) {
	input := "Account,2024\nRevenue,-\n"
	res := mustParse(t, input, ingestion.Options{})
	if _, ok := res.Rows[0].Values["2024"]; ok {
		t.Error("expected no value for dash cell under default blank treatment")
	}
}

func TestBlankRowsSkippedFromRows(t *testing.T) {
	input := "Account,2024\nRevenue,1000\n,,\nCOGS,500\n"
	res := mustParse(t, input, ingestion.Options{})
	for _, r := range res.Rows {
		if r.Kind == ingestion.StructuralBlank {
			t.Error("blank rows should not appear in Result.Rows")
		}
	}
	if len(res.Rows) != 2 {
		t.Errorf("got %d rows, want 2 (blank row excluded)", len(res.Rows))
	}
}

func TestStableRowOrdering(t *testing.T) {
	input := "Account,2024\nZebra,1\nApple,2\nMango,3\n"
	res := mustParse(t, input, ingestion.Options{})
	want := []string{"Zebra", "Apple", "Mango"}
	for i, w := range want {
		if res.Rows[i].Label != w {
			t.Errorf("Rows[%d].Label = %q, want %q (source order must be preserved)", i, res.Rows[i].Label, w)
		}
	}
}

func TestRepeatedDeterministicParsing(t *testing.T) {
	input := "Account,2023,2024\nRevenue,1000,1200\nCOGS,(400),(450)\nTotal Expenses,400,450\n"
	first := mustParse(t, input, ingestion.Options{})
	for i := 0; i < 5; i++ {
		got := mustParse(t, input, ingestion.Options{})
		if len(got.Rows) != len(first.Rows) {
			t.Fatalf("iteration %d: row count differs", i)
		}
		for j := range first.Rows {
			if got.Rows[j].ID != first.Rows[j].ID || got.Rows[j].Label != first.Rows[j].Label || got.Rows[j].Kind != first.Rows[j].Kind {
				t.Fatalf("iteration %d: row %d differs: %+v vs %+v", i, j, got.Rows[j], first.Rows[j])
			}
		}
	}
}

func TestStableRowIDs(t *testing.T) {
	input := "Account,2024\nRevenue,1000\n"
	res := mustParse(t, input, ingestion.Options{})
	if res.Rows[0].ID != "sheet-0-row-1" {
		t.Errorf("ID = %q, want sheet-0-row-1", res.Rows[0].ID)
	}
}

func TestEmptyInputIsFatal(t *testing.T) {
	_, err := icsv.Parse(strings.NewReader(""), ingestion.Options{})
	if err == nil {
		t.Fatal("expected fatal error for empty input")
	}
	if err.Code != ingestion.ErrCodeNoTabularData {
		t.Errorf("Code = %q, want %q", err.Code, ingestion.ErrCodeNoTabularData)
	}
}

func TestMaxFileSizeLimit(t *testing.T) {
	big := strings.Repeat("a", 100)
	input := "Account,2024\n" + big + ",1000\n"
	_, err := icsv.Parse(strings.NewReader(input), ingestion.Options{
		Limits: ingestion.Limits{MaxFileSizeBytes: 10},
	})
	if err == nil {
		t.Fatal("expected fatal error for oversized input")
	}
	if err.Code != ingestion.ErrCodeLimitExceeded {
		t.Errorf("Code = %q, want %q", err.Code, ingestion.ErrCodeLimitExceeded)
	}
}

func TestMaxRowsLimit(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("Account,2024\n")
	for i := 0; i < 20; i++ {
		sb.WriteString("Row,1\n")
	}
	_, err := icsv.Parse(strings.NewReader(sb.String()), ingestion.Options{
		Limits: ingestion.Limits{MaxRows: 5},
	})
	if err == nil {
		t.Fatal("expected fatal error for exceeding max rows")
	}
	if err.Code != ingestion.ErrCodeLimitExceeded {
		t.Errorf("Code = %q, want %q", err.Code, ingestion.ErrCodeLimitExceeded)
	}
}

func TestMaxCellTextLengthTruncatesWithWarning(t *testing.T) {
	long := strings.Repeat("x", 50)
	input := "Account,2024\n" + long + ",1000\n"
	res := mustParse(t, input, ingestion.Options{
		Limits: ingestion.Limits{MaxCellTextLength: 10},
	})
	if len(res.Rows[0].Label) > 10 {
		t.Errorf("Label length = %d, want <= 10", len(res.Rows[0].Label))
	}
	found := false
	for _, w := range res.Warnings {
		if w.Code == ingestion.WarnCellTextTruncated {
			found = true
		}
	}
	if !found {
		t.Error("expected WarnCellTextTruncated warning")
	}
}

func TestToRawLineItemsExcludesHeadingsAndBlanks(t *testing.T) {
	input := "Account,2024\nOperating Expenses,\n  Advertising,5000\n,,\nTotal Operating Expenses,5000\n"
	res := mustParse(t, input, ingestion.Options{})
	items := res.ToRawLineItems()
	for _, item := range items {
		if item.Label == "Operating Expenses" {
			t.Error("heading row should be excluded from RawLineItems")
		}
	}
	// Subtotal row should still be present, with Status set.
	foundSubtotal := false
	for _, item := range items {
		if item.Label == "Total Operating Expenses" {
			foundSubtotal = true
		}
	}
	if !foundSubtotal {
		t.Error("subtotal row should be present in RawLineItems")
	}
}
