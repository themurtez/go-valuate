package pdf_test

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/ingestion"
	"github.com/themurtez/go-valuate/ingestion/ocr"
	ipdf "github.com/themurtez/go-valuate/ingestion/pdf"
)

// wordsForScannedPL returns a deterministic set of fake OCR words
// positioned to match scanned_pl.pdf's known layout (see
// ingestion/fixtures/gen/generate_ocr_pdf.go's writeScannedPL and
// scan_image.go's renderScanImage: labels at pixel x=100, values at
// x=700, 30px line spacing starting at y=120, on a 1275x1650px canvas),
// so groupRows/columns reconstruction produces the exact same row
// structure a real Tesseract run against that fixture would.
func wordsForScannedPL(confidence float64) []ocr.Word {
	type row struct {
		label, value string
	}
	rows := []row{
		{"Riverside Consulting LLC", ""},
		{"Income Statement", ""},
		{"Account", "FY2025"},
		{"Revenue", "850,000"},
		{"Cost of Goods Sold", "320,000"},
		{"Gross Profit", "530,000"},
		{"Operating Expenses", "210,000"},
		{"Net Income", "320,000"},
	}
	var words []ocr.Word
	for i, r := range rows {
		y := 120 + i*30
		words = append(words, ocr.Word{
			Text: r.label, Confidence: confidence,
			X: 100, Y: y, Width: len(r.label) * 8, Height: 16,
			PageIndex: 0, BlockNum: 1, ParNum: 1, LineNum: i,
		})
		if r.value != "" {
			words = append(words, ocr.Word{
				Text: r.value, Confidence: confidence,
				X: 700, Y: y, Width: len(r.value) * 8, Height: 16,
				PageIndex: 0, BlockNum: 1, ParNum: 1, LineNum: i,
			})
		}
	}
	return words
}

func TestParseWithOCR_Disabled_MatchesPlainParse(t *testing.T) {
	data := readFixture(t, "simple_pl.pdf")
	res1, err1 := ipdf.Parse(bytes.NewReader(data), ipdf.Options{})
	if err1 != nil {
		t.Fatalf("Parse: %+v", err1)
	}
	res2, err2 := ipdf.ParseWithOCR(context.Background(), bytes.NewReader(data), ipdf.Options{}, nil)
	if err2 != nil {
		t.Fatalf("ParseWithOCR(OCRDisabled): %+v", err2)
	}
	if len(res1.Statements) != len(res2.Statements) {
		t.Fatalf("statement count differs: %d vs %d", len(res1.Statements), len(res2.Statements))
	}
	if len(res1.Statements[0].Rows) != len(res2.Statements[0].Rows) {
		t.Errorf("row count differs: %d vs %d", len(res1.Statements[0].Rows), len(res2.Statements[0].Rows))
	}
}

func TestParseWithOCR_Auto_ScannedPDFUsesOCR(t *testing.T) {
	data := readFixture(t, "scanned_pl.pdf")
	engine := &ocr.FakeEngine{
		Pages: map[int]ocr.Result{
			0: {EngineName: "fake", EngineVersion: "1.0", Words: wordsForScannedPL(95.0)},
		},
	}
	res, ierr := ipdf.ParseWithOCR(context.Background(), bytes.NewReader(data), ipdf.Options{OCR: ipdf.OCRAuto}, engine)
	if ierr != nil {
		t.Fatalf("ParseWithOCR: %+v", ierr)
	}
	if len(res.Statements) != 1 {
		t.Fatalf("got %d statements, want 1", len(res.Statements))
	}
	stmt := res.Statements[0]
	if stmt.Metadata.OCR == nil || !stmt.Metadata.OCR.Used {
		t.Fatal("expected Metadata.OCR.Used = true")
	}
	if stmt.Metadata.OCR.EngineName != "fake" {
		t.Errorf("EngineName = %q, want fake", stmt.Metadata.OCR.EngineName)
	}

	revenue, ok := rowByLabel(stmt.Rows, "Revenue")
	if !ok {
		t.Fatal("expected a Revenue row")
	}
	if revenue.Values[financial.Period("FY2025")] != 850000 {
		t.Errorf("Revenue FY2025 = %v, want 850000", revenue.Values[financial.Period("FY2025")])
	}

	netIncome, ok := rowByLabel(stmt.Rows, "Net Income")
	if !ok {
		t.Fatal("expected a Net Income row")
	}
	if netIncome.Values[financial.Period("FY2025")] != 320000 {
		t.Errorf("Net Income FY2025 = %v, want 320000", netIncome.Values[financial.Period("FY2025")])
	}

	foundOCRUsedWarning := false
	for _, w := range res.Warnings {
		if w.Code == ingestion.WarnOCRUsed {
			foundOCRUsedWarning = true
		}
	}
	if !foundOCRUsedWarning {
		t.Error("expected WarnOCRUsed document-level warning")
	}
}

func TestParseWithOCR_Auto_TextPDFNeverInvokesEngine(t *testing.T) {
	data := readFixture(t, "simple_pl.pdf")
	engine := &ocr.FakeEngine{}
	_, ierr := ipdf.ParseWithOCR(context.Background(), bytes.NewReader(data), ipdf.Options{OCR: ipdf.OCRAuto}, engine)
	if ierr != nil {
		t.Fatalf("ParseWithOCR: %+v", ierr)
	}
	if len(engine.Calls) != 0 {
		t.Errorf("engine.Calls = %v, want none (text PDF should never invoke OCR under OCRAuto)", engine.Calls)
	}
}

// TestParseWithOCR_Force_UsesOCRForEveryPage confirms OCRForce drives
// every page through the OCR path (never consulting any embedded text
// layer, even on a page that has one) using scanned_multi_page.pdf, whose
// every page has a genuine embedded scan image.
func TestParseWithOCR_Force_UsesOCRForEveryPage(t *testing.T) {
	data := readFixture(t, "scanned_multi_page.pdf")
	engine := &ocr.FakeEngine{
		Pages: map[int]ocr.Result{
			0: {EngineName: "fake", Words: wordsForScannedPL(90.0)},
			1: {EngineName: "fake", Words: []ocr.Word{
				{Text: "Net Income", X: 100, Y: 120, Width: 80, Height: 16, PageIndex: 1, Confidence: 90},
				{Text: "1,695,000", X: 700, Y: 120, Width: 80, Height: 16, PageIndex: 1, Confidence: 90},
			}},
		},
	}
	_, ierr := ipdf.ParseWithOCR(context.Background(), bytes.NewReader(data), ipdf.Options{OCR: ipdf.OCRForce}, engine)
	if ierr != nil {
		t.Fatalf("ParseWithOCR(OCRForce): %+v", ierr)
	}
	if len(engine.Calls) != 2 {
		t.Errorf("engine.Calls = %v, want calls for both pages under OCRForce", engine.Calls)
	}
}

func TestParseWithOCR_NilEngineReturnsStructuredError(t *testing.T) {
	data := readFixture(t, "scanned_pl.pdf")
	_, ierr := ipdf.ParseWithOCR(context.Background(), bytes.NewReader(data), ipdf.Options{OCR: ipdf.OCRAuto}, nil)
	if ierr == nil {
		t.Fatal("expected an error for OCR mode with a nil engine")
	}
	if ierr.Code != ingestion.ErrCodeOCREngineUnavailable {
		t.Errorf("Code = %q, want %q", ierr.Code, ingestion.ErrCodeOCREngineUnavailable)
	}
}

func TestParseWithOCR_EngineUnavailableFails(t *testing.T) {
	data := readFixture(t, "scanned_pl.pdf")
	engine := &ocr.FakeEngine{
		ErrOnPage: map[int]error{0: &ocr.Error{Code: ocr.ErrCodeEngineUnavailable, Message: "not installed"}},
	}
	_, ierr := ipdf.ParseWithOCR(context.Background(), bytes.NewReader(data), ipdf.Options{OCR: ipdf.OCRForce}, engine)
	if ierr == nil {
		t.Fatal("expected an error")
	}
	if ierr.Code != ingestion.ErrCodeOCREngineUnavailable {
		t.Errorf("Code = %q, want %q", ierr.Code, ingestion.ErrCodeOCREngineUnavailable)
	}
}

// TestParseWithOCR_Auto_EngineFailureIsFatalEvenInAutoMode confirms a
// resource/engine-level failure (as opposed to a per-page content issue
// like an ambiguous dominant image) is fatal in OCRAuto too, not silently
// swallowed as a skipped page — a caller whose engine is misconfigured or
// unreachable needs to see that, not an opaque NO_TABULAR_DATA once every
// page is silently skipped.
func TestParseWithOCR_Auto_EngineFailureIsFatalEvenInAutoMode(t *testing.T) {
	data := readFixture(t, "scanned_pl.pdf")
	engine := &ocr.FakeEngine{
		ErrOnPage: map[int]error{0: &ocr.Error{Code: ocr.ErrCodeEngineUnavailable, Message: "not installed"}},
	}
	_, ierr := ipdf.ParseWithOCR(context.Background(), bytes.NewReader(data), ipdf.Options{OCR: ipdf.OCRAuto}, engine)
	if ierr == nil {
		t.Fatal("expected an error")
	}
	if ierr.Code != ingestion.ErrCodeOCREngineUnavailable {
		t.Errorf("Code = %q, want %q", ierr.Code, ingestion.ErrCodeOCREngineUnavailable)
	}
}

func TestParseWithOCR_MixedPDF_UsesTextForPage1AndOCRForPage2(t *testing.T) {
	data := readFixture(t, "mixed_text_and_scanned.pdf")
	engine := &ocr.FakeEngine{
		Pages: map[int]ocr.Result{
			1: {EngineName: "fake", Words: []ocr.Word{
				{Text: "Fairview Health Partners", X: 100, Y: 120, Width: 200, Height: 16, PageIndex: 1, Confidence: 92},
				{Text: "Balance Sheet", X: 100, Y: 150, Width: 100, Height: 16, PageIndex: 1, Confidence: 92},
				{Text: "Account", X: 100, Y: 180, Width: 60, Height: 16, PageIndex: 1, Confidence: 92},
				{Text: "2025", X: 700, Y: 180, Width: 40, Height: 16, PageIndex: 1, Confidence: 92},
				{Text: "Cash", X: 100, Y: 210, Width: 40, Height: 16, PageIndex: 1, Confidence: 92},
				{Text: "890,000", X: 700, Y: 210, Width: 60, Height: 16, PageIndex: 1, Confidence: 92},
				{Text: "Total Assets", X: 100, Y: 240, Width: 80, Height: 16, PageIndex: 1, Confidence: 92},
				{Text: "890,000", X: 700, Y: 240, Width: 60, Height: 16, PageIndex: 1, Confidence: 92},
			}},
		},
	}
	res, ierr := ipdf.ParseWithOCR(context.Background(), bytes.NewReader(data), ipdf.Options{OCR: ipdf.OCRAuto}, engine)
	if ierr != nil {
		t.Fatalf("ParseWithOCR: %+v", ierr)
	}
	if len(engine.Calls) != 1 || engine.Calls[0] != 1 {
		t.Errorf("engine.Calls = %v, want [1] (only page 2, 0-indexed as 1, needs OCR)", engine.Calls)
	}
	if len(res.Statements) != 2 {
		t.Fatalf("got %d statements, want 2 (income statement + balance sheet)", len(res.Statements))
	}

	foundMixedWarning := false
	for _, w := range res.Warnings {
		if w.Code == ingestion.WarnMixedTextAndOCRPages {
			foundMixedWarning = true
		}
	}
	if !foundMixedWarning {
		t.Error("expected WarnMixedTextAndOCRPages document-level warning")
	}
}

func TestParseWithOCR_LowConfidenceWordsFlagged(t *testing.T) {
	data := readFixture(t, "scanned_pl.pdf")
	engine := &ocr.FakeEngine{
		Pages: map[int]ocr.Result{
			0: {EngineName: "fake", Words: wordsForScannedPL(40.0)}, // below lowConfidenceThreshold
		},
	}
	res, ierr := ipdf.ParseWithOCR(context.Background(), bytes.NewReader(data), ipdf.Options{OCR: ipdf.OCRAuto}, engine)
	if ierr != nil {
		t.Fatalf("ParseWithOCR: %+v", ierr)
	}
	foundLowConfNumeric := false
	for _, w := range res.Statements[0].Warnings {
		if w.Code == ingestion.WarnLowConfidenceNumericValue {
			foundLowConfNumeric = true
		}
	}
	if !foundLowConfNumeric {
		t.Error("expected WarnLowConfidenceNumericValue for confidence-40 OCR words")
	}

	revenue, ok := rowByLabel(res.Statements[0].Rows, "Revenue")
	if !ok {
		t.Fatal("expected a Revenue row")
	}
	var revenueCell ingestion.Cell
	for _, c := range revenue.Cells {
		if c.Numeric != nil {
			revenueCell = c
		}
	}
	if revenueCell.OCR == nil {
		t.Fatal("expected OCRProvenance on the Revenue numeric cell")
	}
	if !revenueCell.OCR.ReviewRecommended {
		t.Error("expected ReviewRecommended = true for low-confidence OCR value")
	}
}

func TestParseWithOCR_AmbiguousNumericRejectedNotGuessed(t *testing.T) {
	words := []ocr.Word{
		{Text: "Account", X: 100, Y: 90, Width: 60, Height: 16, PageIndex: 0, Confidence: 90},
		{Text: "FY2025", X: 700, Y: 90, Width: 60, Height: 16, PageIndex: 0, Confidence: 90},
		{Text: "Revenue", X: 100, Y: 120, Width: 60, Height: 16, PageIndex: 0, Confidence: 90},
		{Text: "1O,OOO", X: 700, Y: 120, Width: 60, Height: 16, PageIndex: 0, Confidence: 90}, // corrects cleanly
		{Text: "Net Income", X: 100, Y: 150, Width: 80, Height: 16, PageIndex: 0, Confidence: 90},
		{Text: "2,5OO", X: 700, Y: 150, Width: 60, Height: 16, PageIndex: 0, Confidence: 90},
	}
	data := readFixture(t, "scanned_ambiguous_numeric.pdf")
	engine := &ocr.FakeEngine{Pages: map[int]ocr.Result{0: {EngineName: "fake", Words: words}}}
	res, ierr := ipdf.ParseWithOCR(context.Background(), bytes.NewReader(data), ipdf.Options{OCR: ipdf.OCRAuto}, engine)
	if ierr != nil {
		t.Fatalf("ParseWithOCR: %+v", ierr)
	}
	revenue, ok := rowByLabel(res.Statements[0].Rows, "Revenue")
	if !ok {
		t.Fatal("expected a Revenue row")
	}
	if revenue.Values[financial.Period("FY2025")] != 10000 {
		t.Errorf("Revenue = %v, want 10000 (corrected from 1O,OOO)", revenue.Values[financial.Period("FY2025")])
	}
}

func TestParseWithOCR_RespectsMaxOCRPagesLimit(t *testing.T) {
	data := readFixture(t, "scanned_multi_page.pdf")
	engine := &ocr.FakeEngine{
		Pages: map[int]ocr.Result{
			0: {EngineName: "fake", Words: wordsForScannedPL(95.0)},
			1: {EngineName: "fake", Words: wordsForScannedPL(95.0)},
		},
	}
	opts := ipdf.Options{OCR: ipdf.OCRAuto}
	opts.Limits.MaxOCRPages = 1
	_, ierr := ipdf.ParseWithOCR(context.Background(), bytes.NewReader(data), opts, engine)
	if ierr == nil {
		t.Fatal("expected an error")
	}
	if ierr.Code != ingestion.ErrCodeOCRPageLimitExceeded {
		t.Errorf("Code = %q, want %q", ierr.Code, ingestion.ErrCodeOCRPageLimitExceeded)
	}
}

func TestParseWithOCR_MultiPageAggregatesAllPages(t *testing.T) {
	data := readFixture(t, "scanned_multi_page.pdf")
	page1Words := []ocr.Word{
		{Text: "Cascade Manufacturing Inc.", X: 100, Y: 120, Width: 200, Height: 16, PageIndex: 0, Confidence: 90},
		{Text: "Income Statement", X: 100, Y: 150, Width: 120, Height: 16, PageIndex: 0, Confidence: 90},
		{Text: "Account", X: 100, Y: 180, Width: 60, Height: 16, PageIndex: 0, Confidence: 90},
		{Text: "FY2025", X: 700, Y: 180, Width: 60, Height: 16, PageIndex: 0, Confidence: 90},
		{Text: "Product Sales", X: 100, Y: 210, Width: 100, Height: 16, PageIndex: 0, Confidence: 90},
		{Text: "2,400,000", X: 700, Y: 210, Width: 80, Height: 16, PageIndex: 0, Confidence: 90},
	}
	page2Words := []ocr.Word{
		{Text: "Net Income", X: 100, Y: 120, Width: 80, Height: 16, PageIndex: 1, Confidence: 90},
		{Text: "1,695,000", X: 700, Y: 120, Width: 80, Height: 16, PageIndex: 1, Confidence: 90},
	}
	engine := &ocr.FakeEngine{
		Pages: map[int]ocr.Result{
			0: {EngineName: "fake", Words: page1Words},
			1: {EngineName: "fake", Words: page2Words},
		},
	}
	res, ierr := ipdf.ParseWithOCR(context.Background(), bytes.NewReader(data), ipdf.Options{OCR: ipdf.OCRAuto}, engine)
	if ierr != nil {
		t.Fatalf("ParseWithOCR: %+v", ierr)
	}
	if res.PageCount != 2 {
		t.Errorf("PageCount = %d, want 2", res.PageCount)
	}
	stmt := res.Statements[0]
	if _, ok := rowByLabel(stmt.Rows, "Product Sales"); !ok {
		t.Error("expected Product Sales row from page 1")
	}
	if _, ok := rowByLabel(stmt.Rows, "Net Income"); !ok {
		t.Error("expected Net Income row from page 2")
	}
}

// TestParseWithOCR_RealFileReader confirms ParseWithOCR works against a
// real *os.File (not just an in-memory bytes.Reader), since io.ReadSeeker
// is the documented contract.
func TestParseWithOCR_RealFileReader(t *testing.T) {
	f, err := os.Open("../fixtures/scanned_pl.pdf")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	engine := &ocr.FakeEngine{Pages: map[int]ocr.Result{0: {EngineName: "fake", Words: wordsForScannedPL(95.0)}}}
	res, ierr := ipdf.ParseWithOCR(context.Background(), f, ipdf.Options{OCR: ipdf.OCRAuto}, engine)
	if ierr != nil {
		t.Fatalf("ParseWithOCR: %+v", ierr)
	}
	if len(res.Statements) != 1 {
		t.Fatalf("got %d statements, want 1", len(res.Statements))
	}
}
