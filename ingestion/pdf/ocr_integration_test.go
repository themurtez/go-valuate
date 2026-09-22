package pdf_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/classification"
	"github.com/themurtez/go-valuate/financial/reconciliation"
	"github.com/themurtez/go-valuate/ingestion/ocr"
	ipdf "github.com/themurtez/go-valuate/ingestion/pdf"
)

// TestIntegration_ScannedPDFIncomeStatementToNormalizedDataset exercises
// the full advertised OCR pipeline: scanned PDF bytes -> image extraction
// (ingestion/pdf/pdfimage) -> fake OCR engine -> positioned text
// (ocr_layout.go) -> PDF layout reconstruction (the SAME layout.go/
// columns.go/build.go this package already uses for embedded text) ->
// ingestion.Result -> ToRawLineItems() -> classification.ClassifyBatch ->
// financial.Normalize -> FinancialDataset, mirroring
// integration_test.go's embedded-text-PDF equivalent exactly — proving
// the OCR path converges on the identical downstream pipeline, with zero
// OCR-specific classification/normalization logic anywhere.
func TestIntegration_ScannedPDFIncomeStatementToNormalizedDataset(t *testing.T) {
	data := readFixture(t, "scanned_pl.pdf")
	engine := &ocr.FakeEngine{
		Pages: map[int]ocr.Result{
			0: {EngineName: "fake", Words: wordsForScannedPL(96.0)},
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

	rawItems := stmt.ToRawLineItems()
	if len(rawItems) == 0 {
		t.Fatal("expected at least one raw line item")
	}

	cfg := classification.Config{Rules: classification.DefaultRules()}
	results := classification.ClassifyBatch(rawItems, cfg)

	fallback := map[string]financial.Code{
		"Revenue":            financial.CodeRevProduct,
		"Cost of Goods Sold": financial.CodeCogsOther,
		"Operating Expenses": financial.CodeOpexOther,
	}

	mapped := make([]financial.MappedLineItem, 0, len(rawItems))
	for i, raw := range rawItems {
		result := results[i]
		if result.IsUnknown() {
			code, ok := fallback[raw.Label]
			if !ok {
				t.Fatalf("row %q classified UNKNOWN with no test fallback mapping", raw.Label)
			}
			result.Code = code
			result.Status = financial.RowStatusNormal
		}
		mapped = append(mapped, result.ToMappedLineItem(raw))
	}

	dataset, nerr := financial.Normalize(mapped, financial.NormalizeOptions{Currency: "USD"})
	if nerr != nil {
		t.Fatalf("Normalize: %v", nerr)
	}

	revenue, ok := dataset.ByCodeAndPeriod(financial.CodeRevProduct, "FY2025")
	if !ok {
		t.Fatal("expected REV_PRODUCT for FY2025 in normalized dataset")
	}
	if revenue.Amount != 850000 {
		t.Errorf("REV_PRODUCT FY2025 = %v, want 850000", revenue.Amount)
	}
}

// TestIntegration_ScannedPDFBalanceSheetReconciliation mirrors
// integration_test.go's TestIntegration_PDFBalanceSheetReconciliation for
// the OCR path: scanned PDF -> OCR -> RawLineItem[] ->
// classification.ClassifyBatch -> financial.Normalize ->
// financial/reconciliation, confirming the OCR path converges on the
// exact same reconciliation logic CSV/XLSX/text-PDF already do.
func TestIntegration_ScannedPDFBalanceSheetReconciliation(t *testing.T) {
	data := readFixture(t, "scanned_balance_sheet.pdf")
	words := []ocr.Word{
		{Text: "Cascade Manufacturing Inc.", X: 100, Y: 120, Width: 220, Height: 16, PageIndex: 0, Confidence: 93},
		{Text: "Balance Sheet", X: 100, Y: 150, Width: 100, Height: 16, PageIndex: 0, Confidence: 93},
		{Text: "Account", X: 100, Y: 180, Width: 60, Height: 16, PageIndex: 0, Confidence: 93},
		{Text: "2025", X: 700, Y: 180, Width: 40, Height: 16, PageIndex: 0, Confidence: 93},
		{Text: "Cash", X: 100, Y: 210, Width: 40, Height: 16, PageIndex: 0, Confidence: 93},
		{Text: "410,000", X: 700, Y: 210, Width: 60, Height: 16, PageIndex: 0, Confidence: 93},
		{Text: "Accounts Receivable", X: 100, Y: 240, Width: 150, Height: 16, PageIndex: 0, Confidence: 93},
		{Text: "325,000", X: 700, Y: 240, Width: 60, Height: 16, PageIndex: 0, Confidence: 93},
		{Text: "Total Current Assets", X: 100, Y: 270, Width: 150, Height: 16, PageIndex: 0, Confidence: 93},
		{Text: "735,000", X: 700, Y: 270, Width: 60, Height: 16, PageIndex: 0, Confidence: 93},
		{Text: "Total Assets", X: 100, Y: 300, Width: 100, Height: 16, PageIndex: 0, Confidence: 93},
		{Text: "735,000", X: 700, Y: 300, Width: 60, Height: 16, PageIndex: 0, Confidence: 93},
		{Text: "Accounts Payable", X: 100, Y: 330, Width: 120, Height: 16, PageIndex: 0, Confidence: 93},
		{Text: "180,000", X: 700, Y: 330, Width: 60, Height: 16, PageIndex: 0, Confidence: 93},
		{Text: "Total Liabilities", X: 100, Y: 360, Width: 120, Height: 16, PageIndex: 0, Confidence: 93},
		{Text: "180,000", X: 700, Y: 360, Width: 60, Height: 16, PageIndex: 0, Confidence: 93},
		{Text: "Retained Earnings", X: 100, Y: 390, Width: 130, Height: 16, PageIndex: 0, Confidence: 93},
		{Text: "555,000", X: 700, Y: 390, Width: 60, Height: 16, PageIndex: 0, Confidence: 93},
		{Text: "Total Equity", X: 100, Y: 420, Width: 100, Height: 16, PageIndex: 0, Confidence: 93},
		{Text: "555,000", X: 700, Y: 420, Width: 60, Height: 16, PageIndex: 0, Confidence: 93},
	}
	engine := &ocr.FakeEngine{Pages: map[int]ocr.Result{0: {EngineName: "fake", Words: words}}}

	res, ierr := ipdf.ParseWithOCR(context.Background(), bytes.NewReader(data), ipdf.Options{OCR: ipdf.OCRAuto}, engine)
	if ierr != nil {
		t.Fatalf("ParseWithOCR: %+v", ierr)
	}
	if len(res.Statements) != 1 {
		t.Fatalf("got %d statements, want 1", len(res.Statements))
	}
	stmt := res.Statements[0]

	rawItems := stmt.ToRawLineItems()
	cfg := classification.Config{Rules: classification.DefaultRules()}
	results := classification.ClassifyBatch(rawItems, cfg)

	fallback := map[string]financial.Code{
		"Cash":                financial.CodeBsCash,
		"Accounts Receivable": financial.CodeBsAccountsReceivable,
		"Accounts Payable":    financial.CodeBsAccountsPayable,
		"Retained Earnings":   financial.CodeBsRetainedEarnings,
	}

	mapped := make([]financial.MappedLineItem, 0, len(rawItems))
	for i, raw := range rawItems {
		result := results[i]
		if result.IsUnknown() {
			code, ok := fallback[raw.Label]
			if !ok {
				t.Fatalf("row %q classified UNKNOWN with no test fallback mapping", raw.Label)
			}
			result.Code = code
			result.Status = financial.RowStatusNormal
		}
		mapped = append(mapped, result.ToMappedLineItem(raw))
	}

	dataset, nerr := financial.Normalize(mapped, financial.NormalizeOptions{Currency: "USD"})
	if nerr != nil {
		t.Fatalf("Normalize: %v", nerr)
	}

	cash, ok := dataset.ByCodeAndPeriod(financial.CodeBsCash, "2025")
	if !ok {
		t.Fatal("expected BS_CASH for 2025")
	}
	if cash.Amount != 410000 {
		t.Errorf("BS_CASH 2025 = %v, want 410000", cash.Amount)
	}

	result := reconciliation.Run(dataset, reconciliation.Options{})
	if result.HasFailures() {
		t.Errorf("balance sheet reconciliation reported failures: %+v", result.Checks)
	}
}
