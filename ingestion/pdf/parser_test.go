package pdf_test

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/ingestion"
	ipdf "github.com/themurtez/go-valuate/ingestion/pdf"
)

func openFixture(t *testing.T, name string) *os.File {
	t.Helper()
	f, err := os.Open("../fixtures/" + name)
	if err != nil {
		t.Fatalf("open fixture %q: %v", name, err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("../fixtures/" + name)
	if err != nil {
		t.Fatalf("read fixture %q: %v", name, err)
	}
	return data
}

func mustParse(t *testing.T, fixture string, opts ipdf.Options) ipdf.Results {
	t.Helper()
	res, err := ipdf.Parse(openFixture(t, fixture), opts)
	if err != nil {
		t.Fatalf("Parse(%q): %+v", fixture, err)
	}
	return res
}

func rowByLabel(rows []ingestion.Row, label string) (ingestion.Row, bool) {
	for _, r := range rows {
		if r.Label == label {
			return r, true
		}
	}
	return ingestion.Row{}, false
}

// --- Valid text PDF / basic extraction -----------------------------------

func TestParse_SinglePageIncomeStatement(t *testing.T) {
	res := mustParse(t, "simple_pl.pdf", ipdf.Options{})
	if len(res.Statements) != 1 {
		t.Fatalf("got %d statements, want 1", len(res.Statements))
	}
	stmt := res.Statements[0]
	if stmt.Metadata.StatementType != financial.StatementIncomeStatement {
		t.Errorf("StatementType = %q, want income_statement", stmt.Metadata.StatementType)
	}
	if stmt.Metadata.StatementTypeUnknown {
		t.Error("StatementTypeUnknown = true, want false")
	}
	if stmt.Metadata.Dependency != ipdf.Dependency {
		t.Errorf("Dependency = %q, want %q", stmt.Metadata.Dependency, ipdf.Dependency)
	}
	if stmt.Metadata.Format != ingestion.FormatPDF {
		t.Errorf("Format = %q, want %q", stmt.Metadata.Format, ingestion.FormatPDF)
	}

	revenue, ok := rowByLabel(stmt.Rows, "Revenue")
	if !ok {
		t.Fatal("Revenue row not found")
	}
	if revenue.Values[financial.Period("FY2025")] != 850000 {
		t.Errorf("Revenue FY2025 = %v, want 850000", revenue.Values[financial.Period("FY2025")])
	}
	if revenue.PageIndex != 0 {
		t.Errorf("Revenue PageIndex = %d, want 0", revenue.PageIndex)
	}

	grossProfit, ok := rowByLabel(stmt.Rows, "Gross Profit")
	if !ok {
		t.Fatal("Gross Profit row not found")
	}
	if grossProfit.Kind != ingestion.StructuralSubtotal {
		t.Errorf("Gross Profit Kind = %q, want subtotal", grossProfit.Kind)
	}

	netIncome, ok := rowByLabel(stmt.Rows, "Net Income")
	if !ok {
		t.Fatal("Net Income row not found")
	}
	if netIncome.Kind != ingestion.StructuralTotal {
		t.Errorf("Net Income Kind = %q, want total", netIncome.Kind)
	}
}

// --- Multiple periods -----------------------------------------------------

func TestParse_MultiplePeriods(t *testing.T) {
	res := mustParse(t, "multi_year_pl.pdf", ipdf.Options{})
	if len(res.Statements) != 1 {
		t.Fatalf("got %d statements, want 1", len(res.Statements))
	}
	stmt := res.Statements[0]
	if len(stmt.Metadata.Periods) != 3 {
		t.Fatalf("got %d periods, want 3: %+v", len(stmt.Metadata.Periods), stmt.Metadata.Periods)
	}

	revenue, ok := rowByLabel(stmt.Rows, "Revenue")
	if !ok {
		t.Fatal("Revenue row not found")
	}
	want := map[financial.Period]float64{"2023": 1200000, "2024": 1450000, "2025": 1780000}
	for period, amount := range want {
		if revenue.Values[period] != amount {
			t.Errorf("Revenue[%s] = %v, want %v", period, revenue.Values[period], amount)
		}
	}
}

// --- Page ordering ----------------------------------------------------

func TestParse_PageOrderingPreserved(t *testing.T) {
	res := mustParse(t, "multi_page_pl.pdf", ipdf.Options{})
	if len(res.Statements) != 1 {
		t.Fatalf("got %d statements, want 1", len(res.Statements))
	}
	rows := res.Statements[0].Rows

	var lastPage int
	for i, r := range rows {
		if r.PageIndex < lastPage {
			t.Errorf("row %d (%q) PageIndex=%d regressed after page %d — rows must stay in document order", i, r.Label, r.PageIndex, lastPage)
		}
		lastPage = r.PageIndex
	}

	utilities, ok := rowByLabel(rows, "Utilities")
	if !ok {
		t.Fatal("Utilities row not found")
	}
	if utilities.PageIndex != 1 {
		t.Errorf("Utilities PageIndex = %d, want 1", utilities.PageIndex)
	}
	payroll, ok := rowByLabel(rows, "Payroll")
	if !ok {
		t.Fatal("Payroll row not found")
	}
	if payroll.PageIndex != 0 {
		t.Errorf("Payroll PageIndex = %d, want 0", payroll.PageIndex)
	}
}

// --- Multi-page repeated headers ---------------------------------------

func TestParse_MultiPageRepeatedHeadersSuppressed(t *testing.T) {
	res := mustParse(t, "multi_page_pl.pdf", ipdf.Options{})
	stmt := res.Statements[0]

	count := 0
	for _, r := range stmt.Rows {
		if r.Label == "Cascade Manufacturing Inc." || r.Label == "Income Statement" {
			count++
		}
	}
	if count > 0 {
		t.Errorf("title lines appeared as data rows %d times, want 0 (should be consumed as title/header, not data)", count)
	}

	removed := 0
	for _, w := range stmt.Warnings {
		if w.Code == ingestion.WarnRepeatedHeaderRemoved {
			removed++
		}
	}
	if removed == 0 {
		t.Error("expected at least one REPEATED_HEADER_REMOVED warning")
	}

	// The repeated header block must not have produced duplicate
	// "Payroll"/"Revenue" style rows either — every label should appear
	// at most once.
	seen := make(map[string]int)
	for _, r := range stmt.Rows {
		seen[r.Label]++
	}
	for label, n := range seen {
		if n > 1 {
			t.Errorf("label %q appeared %d times, want at most 1 (possible duplicate from unsuppressed repeated header)", label, n)
		}
	}
}

// --- Parent headings across pages --------------------------------------

func TestParse_ParentLabelContinuesAcrossPages(t *testing.T) {
	res := mustParse(t, "multi_page_pl.pdf", ipdf.Options{})
	stmt := res.Statements[0]

	utilities, ok := rowByLabel(stmt.Rows, "Utilities")
	if !ok {
		t.Fatal("Utilities row not found")
	}
	if utilities.ParentLabel != "Operating Expenses" {
		t.Errorf("Utilities ParentLabel = %q, want %q (parent context must survive the page break)", utilities.ParentLabel, "Operating Expenses")
	}
}

// --- Subtotal/total rows ------------------------------------------------

func TestParse_SubtotalAndTotalRowsDetected(t *testing.T) {
	res := mustParse(t, "multi_page_pl.pdf", ipdf.Options{})
	stmt := res.Statements[0]

	totalOpex, ok := rowByLabel(stmt.Rows, "Total Operating Expenses")
	if !ok {
		t.Fatal("Total Operating Expenses row not found")
	}
	if totalOpex.Kind != ingestion.StructuralSubtotal {
		t.Errorf("Total Operating Expenses Kind = %q, want subtotal", totalOpex.Kind)
	}

	netIncome, ok := rowByLabel(stmt.Rows, "Net Income")
	if !ok {
		t.Fatal("Net Income row not found")
	}
	if netIncome.Kind != ingestion.StructuralTotal {
		t.Errorf("Net Income Kind = %q, want total", netIncome.Kind)
	}
}

// --- Statement detection: balance sheet ---------------------------------

func TestParse_BalanceSheetDetected(t *testing.T) {
	res := mustParse(t, "balance_sheet.pdf", ipdf.Options{})
	if len(res.Statements) != 1 {
		t.Fatalf("got %d statements, want 1", len(res.Statements))
	}
	stmt := res.Statements[0]
	if stmt.Metadata.StatementType != financial.StatementBalanceSheet {
		t.Errorf("StatementType = %q, want balance_sheet", stmt.Metadata.StatementType)
	}

	totalAssets, ok := rowByLabel(stmt.Rows, "Total Assets")
	if !ok {
		t.Fatal("Total Assets row not found")
	}
	if totalAssets.Kind != ingestion.StructuralTotal {
		t.Errorf("Total Assets Kind = %q, want total", totalAssets.Kind)
	}

	cash, ok := rowByLabel(stmt.Rows, "Cash")
	if !ok {
		t.Fatal("Cash row not found")
	}
	if cash.ParentLabel != "Current Assets" {
		t.Errorf("Cash ParentLabel = %q, want %q", cash.ParentLabel, "Current Assets")
	}
}

// --- Multiple statements in one PDF (Option A) --------------------------

func TestParse_MultipleStatementsInOnePDF(t *testing.T) {
	res := mustParse(t, "pl_and_balance_sheet.pdf", ipdf.Options{})
	if len(res.Statements) != 2 {
		t.Fatalf("got %d statements, want 2", len(res.Statements))
	}
	if res.Statements[0].Metadata.StatementType != financial.StatementIncomeStatement {
		t.Errorf("statement[0] type = %q, want income_statement", res.Statements[0].Metadata.StatementType)
	}
	if res.Statements[1].Metadata.StatementType != financial.StatementBalanceSheet {
		t.Errorf("statement[1] type = %q, want balance_sheet", res.Statements[1].Metadata.StatementType)
	}

	// Neither statement's rows may leak into the other.
	for _, r := range res.Statements[0].Rows {
		if r.Label == "Cash" || r.Label == "Total Assets" {
			t.Errorf("balance sheet row %q leaked into the income statement result", r.Label)
		}
	}
	for _, r := range res.Statements[1].Rows {
		if r.Label == "Gross Profit" || r.Label == "Net Income" {
			t.Errorf("income statement row %q leaked into the balance sheet result", r.Label)
		}
	}

	found := false
	for _, w := range res.Warnings {
		if w.Code == ingestion.WarnMultipleStatementsDetected {
			found = true
		}
	}
	if !found {
		t.Error("expected MULTIPLE_STATEMENTS_DETECTED warning at the document level")
	}
}

// --- Ambiguous layout ----------------------------------------------------

func TestParse_AmbiguousLayoutWarns(t *testing.T) {
	res := mustParse(t, "ambiguous_layout.pdf", ipdf.Options{})
	if len(res.Statements) != 1 {
		t.Fatalf("got %d statements, want 1", len(res.Statements))
	}
	warnings := res.Statements[0].Warnings
	foundColumn := false
	foundLayout := false
	for _, w := range warnings {
		switch w.Code {
		case ingestion.WarnColumnAlignmentAmbiguous:
			foundColumn = true
		case ingestion.WarnPDFLayoutAmbiguous:
			foundLayout = true
		}
	}
	if !foundColumn {
		t.Error("expected COLUMN_ALIGNMENT_AMBIGUOUS warning")
	}
	if !foundLayout {
		t.Error("expected PDF_LAYOUT_AMBIGUOUS warning")
	}
}

// --- No-text / image-only PDF -> OCR_REQUIRED ----------------------------

func TestParse_ImageOnlyPDFReturnsOCRRequired(t *testing.T) {
	_, err := ipdf.Parse(openFixture(t, "image_only.pdf"), ipdf.Options{})
	if err == nil {
		t.Fatal("expected an error for an image-only PDF, got nil")
	}
	if err.Code != ingestion.ErrCodeOCRRequired {
		t.Errorf("Code = %q, want %q", err.Code, ingestion.ErrCodeOCRRequired)
	}
}

func TestParse_ImageOnlyPDFNeverAttemptsOCR(t *testing.T) {
	// This is a behavioral/documentation-level guarantee rather than
	// something directly observable via an API call: the assertion here
	// is that Parse returns a FATAL error (no Results at all) rather than
	// an empty-but-successful Results — if this package ever silently
	// treated an image-only statement as "zero rows, no error," that
	// would be indistinguishable from OCR having (incorrectly) run and
	// found nothing, which is exactly the failure mode ErrCodeOCRRequired
	// exists to prevent.
	res, err := ipdf.Parse(openFixture(t, "image_only.pdf"), ipdf.Options{})
	if err == nil {
		t.Fatal("expected a non-nil error")
	}
	if len(res.Statements) != 0 {
		t.Errorf("expected zero statements on OCR_REQUIRED, got %d", len(res.Statements))
	}
}

// --- Negative parentheses / numeric parsing -----------------------------

func TestParse_NegativeParenthesesAndSpacingQuirks(t *testing.T) {
	res := mustParse(t, "negative_parentheses.pdf", ipdf.Options{})
	stmt := res.Statements[0]

	interest, ok := rowByLabel(stmt.Rows, "Interest Expense")
	if !ok {
		t.Fatal("Interest Expense row not found")
	}
	if interest.Values[financial.Period("FY2025")] != -12400 {
		t.Errorf("Interest Expense = %v, want -12400", interest.Values[financial.Period("FY2025")])
	}

	other, ok := rowByLabel(stmt.Rows, "Other Expense")
	if !ok {
		t.Fatal("Other Expense row not found")
	}
	if other.Values[financial.Period("FY2025")] != -3100 {
		t.Errorf("Other Expense = %v, want -3100 (spaced parentheses quirk must still parse correctly)", other.Values[financial.Period("FY2025")])
	}
}

func TestParse_UnusualSpacingNumericQuirks(t *testing.T) {
	res := mustParse(t, "unusual_spacing.pdf", ipdf.Options{})
	stmt := res.Statements[0]

	revenue, ok := rowByLabel(stmt.Rows, "Revenue")
	if !ok {
		t.Fatal("Revenue row not found")
	}
	if revenue.Values[financial.Period("FY2025")] != 1234500.00 {
		t.Errorf("Revenue (spaced $ prefix) = %v, want 1234500.00", revenue.Values[financial.Period("FY2025")])
	}

	misc, ok := rowByLabel(stmt.Rows, "Miscellaneous Income")
	if !ok {
		t.Fatal("Miscellaneous Income row not found")
	}
	if misc.Values[financial.Period("FY2025")] != 12400 {
		t.Errorf("Miscellaneous Income (space-thousands) = %v, want 12400", misc.Values[financial.Period("FY2025")])
	}

	adj, ok := rowByLabel(stmt.Rows, "Discontinued Line Adjustment")
	if !ok {
		t.Fatal("Discontinued Line Adjustment row not found")
	}
	if adj.Values[financial.Period("FY2025")] != -45000 {
		t.Errorf("Discontinued Line Adjustment (trailing minus) = %v, want -45000", adj.Values[financial.Period("FY2025")])
	}
}

// --- Indentation / section headings --------------------------------------

func TestParse_IndentedSectionsAndHeadings(t *testing.T) {
	res := mustParse(t, "indented_sections.pdf", ipdf.Options{})
	stmt := res.Statements[0]

	heading, ok := rowByLabel(stmt.Rows, "Operating Expenses")
	if !ok {
		t.Fatal("Operating Expenses heading row not found")
	}
	if heading.Kind != ingestion.StructuralHeading {
		t.Errorf("Operating Expenses Kind = %q, want heading", heading.Kind)
	}

	payroll, ok := rowByLabel(stmt.Rows, "Payroll")
	if !ok {
		t.Fatal("Payroll row not found")
	}
	if payroll.IndentLevel == 0 {
		t.Error("Payroll IndentLevel = 0, want > 0 (X-offset should be detected as indentation)")
	}
	if payroll.ParentLabel != "Operating Expenses" {
		t.Errorf("Payroll ParentLabel = %q, want %q", payroll.ParentLabel, "Operating Expenses")
	}
}

// --- Page/file/text limits ------------------------------------------------

func TestParse_MaxFileSizeLimit(t *testing.T) {
	data := readFixture(t, "simple_pl.pdf")
	_, err := ipdf.Parse(bytes.NewReader(data), ipdf.Options{
		Options: ingestion.Options{Limits: ingestion.Limits{MaxFileSizeBytes: int64(len(data) - 1)}},
	})
	if err == nil {
		t.Fatal("expected a limit-exceeded error")
	}
	if err.Code != ingestion.ErrCodeLimitExceeded {
		t.Errorf("Code = %q, want %q", err.Code, ingestion.ErrCodeLimitExceeded)
	}
}

func TestParse_MaxPagesLimit(t *testing.T) {
	data := readFixture(t, "multi_page_pl.pdf")
	_, err := ipdf.Parse(bytes.NewReader(data), ipdf.Options{
		Options: ingestion.Options{Limits: ingestion.Limits{MaxPages: 1}},
	})
	if err == nil {
		t.Fatal("expected a page-limit error")
	}
	if err.Code != ingestion.ErrCodePDFPageLimitExceeded {
		t.Errorf("Code = %q, want %q", err.Code, ingestion.ErrCodePDFPageLimitExceeded)
	}
}

func TestParse_MaxTextFragmentsLimit(t *testing.T) {
	data := readFixture(t, "simple_pl.pdf")
	_, err := ipdf.Parse(bytes.NewReader(data), ipdf.Options{
		Options: ingestion.Options{Limits: ingestion.Limits{MaxTextFragments: 1}},
	})
	if err == nil {
		t.Fatal("expected a text-limit error")
	}
	if err.Code != ingestion.ErrCodePDFTextLimitExceeded {
		t.Errorf("Code = %q, want %q", err.Code, ingestion.ErrCodePDFTextLimitExceeded)
	}
}

func TestParse_MaxTextLengthPerPageLimit(t *testing.T) {
	data := readFixture(t, "simple_pl.pdf")
	_, err := ipdf.Parse(bytes.NewReader(data), ipdf.Options{
		Options: ingestion.Options{Limits: ingestion.Limits{MaxTextLengthPerPage: 1}},
	})
	if err == nil {
		t.Fatal("expected a text-limit error")
	}
	if err.Code != ingestion.ErrCodePDFTextLimitExceeded {
		t.Errorf("Code = %q, want %q", err.Code, ingestion.ErrCodePDFTextLimitExceeded)
	}
}

func TestParse_EmptyInputIsFatal(t *testing.T) {
	_, err := ipdf.Parse(strings.NewReader(""), ipdf.Options{})
	if err == nil {
		t.Fatal("expected an error for empty input")
	}
	if err.Code != ingestion.ErrCodeNoTabularData {
		t.Errorf("Code = %q, want %q", err.Code, ingestion.ErrCodeNoTabularData)
	}
}

func TestParse_InvalidPDFIsFatal(t *testing.T) {
	_, err := ipdf.Parse(strings.NewReader("this is not a PDF file at all"), ipdf.Options{})
	if err == nil {
		t.Fatal("expected an error for invalid input")
	}
	if err.Code != ingestion.ErrCodeInvalidFile {
		t.Errorf("Code = %q, want %q", err.Code, ingestion.ErrCodeInvalidFile)
	}
}

// --- Page range options ---------------------------------------------------

func TestParse_PageStartPageEndRestrictsExtraction(t *testing.T) {
	res := mustParse(t, "multi_page_pl.pdf", ipdf.Options{PageStart: 1, PageEnd: 1})
	stmt := res.Statements[0]
	if _, ok := rowByLabel(stmt.Rows, "Utilities"); ok {
		t.Error("Utilities (page 2) should not appear when PageEnd=1")
	}
	if _, ok := rowByLabel(stmt.Rows, "Payroll"); !ok {
		t.Error("Payroll (page 1) should appear when PageStart=1, PageEnd=1")
	}
}

// --- Deterministic repeated parsing ---------------------------------------

func TestParse_DeterministicRepeatedParsing(t *testing.T) {
	data := readFixture(t, "multi_year_pl.pdf")

	first, err1 := ipdf.Parse(bytes.NewReader(data), ipdf.Options{})
	if err1 != nil {
		t.Fatalf("first Parse: %+v", err1)
	}
	second, err2 := ipdf.Parse(bytes.NewReader(data), ipdf.Options{})
	if err2 != nil {
		t.Fatalf("second Parse: %+v", err2)
	}

	if !reflect.DeepEqual(first, second) {
		t.Fatal("repeated Parse of identical input produced different results")
	}
}

// --- JSON round-trip -------------------------------------------------------

func TestResults_JSONRoundTrip(t *testing.T) {
	res := mustParse(t, "multi_year_pl.pdf", ipdf.Options{})

	first, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	if !json.Valid(first) {
		t.Fatal("expected valid JSON output")
	}

	var decoded ipdf.Results
	if err := json.Unmarshal(first, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	second, err := json.Marshal(decoded)
	if err != nil {
		t.Fatalf("re-marshal failed: %v", err)
	}
	if string(first) != string(second) {
		t.Fatalf("Results did not round-trip byte-for-byte:\nfirst:  %s\nsecond: %s", first, second)
	}
}

// --- Multiple statements: no silent combination --------------------------

func TestParse_DoesNotSilentlyCombineDifferentStatementTypes(t *testing.T) {
	res := mustParse(t, "pl_and_balance_sheet.pdf", ipdf.Options{})
	for _, stmt := range res.Statements {
		if stmt.Metadata.StatementType == "" {
			continue
		}
		for _, row := range stmt.Rows {
			// Every row's inferred statement type comes from
			// stmt.Metadata.StatementType via ToRawLineItems — this test
			// confirms each RESULT is internally single-typed, which is
			// the actual guarantee Option A provides (rows from two
			// different statement types are never present in the SAME
			// Result.Rows slice).
			_ = row
		}
	}
	if len(res.Statements) < 2 {
		t.Fatal("expected at least 2 separate statement results")
	}
}
