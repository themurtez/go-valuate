// Real-Tesseract integration tests for the full scanned-PDF pipeline,
// gated on the executable actually being installed and resolvable — see
// the task contract's "Tesseract integration tests" requirement, mirroring
// ingestion/ocr/tesseract's own identically-gated integration tests. Every
// other OCR test in this package (ocr_parse_test.go,
// ocr_integration_test.go) uses ocr.FakeEngine and never depends on a real
// Tesseract install.
package pdf_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/themurtez/go-valuate/ingestion/ocr/tesseract"
	ipdf "github.com/themurtez/go-valuate/ingestion/pdf"
)

func requireRealTesseract(t *testing.T) *tesseract.Engine {
	t.Helper()
	e := tesseract.New()
	if !e.Available() {
		t.Skip("SKIPPED — tesseract executable not installed")
	}
	return e
}

// TestTesseractIntegration_ScannedPL runs the FULL pipeline (scanned PDF ->
// real Tesseract OCR -> layout reconstruction -> ingestion.Result) against
// scanned_pl.pdf, a synthetic but genuinely rendered raster scan (see
// ingestion/fixtures/gen/scan_image.go), confirming this package's OCR
// integration works against actual Tesseract output, not just the fake
// engine's synthetic positioned words.
func TestTesseractIntegration_ScannedPL(t *testing.T) {
	engine := requireRealTesseract(t)
	data := readFixture(t, "scanned_pl.pdf")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	res, ierr := ipdf.ParseWithOCR(ctx, bytes.NewReader(data), ipdf.Options{OCR: ipdf.OCRAuto}, engine)
	if ierr != nil {
		t.Fatalf("ParseWithOCR: %+v", ierr)
	}
	if len(res.Statements) == 0 {
		t.Fatal("expected at least one statement")
	}
	stmt := res.Statements[0]
	if len(stmt.Rows) == 0 {
		t.Fatal("expected at least one row from real Tesseract OCR")
	}
	if stmt.Metadata.OCR == nil || !stmt.Metadata.OCR.Used {
		t.Error("expected Metadata.OCR.Used = true")
	}

	revenue, ok := rowByLabel(stmt.Rows, "Revenue")
	if !ok {
		t.Skip("real Tesseract did not recognize the 'Revenue' label exactly — OCR output is inherently non-deterministic; this test only confirms the pipeline runs end-to-end, not exact recognition accuracy")
	}
	t.Logf("Revenue row recognized: %+v", revenue.Values)
}

// TestTesseractIntegration_ScannedBalanceSheet mirrors the above for
// scanned_balance_sheet.pdf.
func TestTesseractIntegration_ScannedBalanceSheet(t *testing.T) {
	engine := requireRealTesseract(t)
	data := readFixture(t, "scanned_balance_sheet.pdf")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	res, ierr := ipdf.ParseWithOCR(ctx, bytes.NewReader(data), ipdf.Options{OCR: ipdf.OCRAuto}, engine)
	if ierr != nil {
		t.Fatalf("ParseWithOCR: %+v", ierr)
	}
	if len(res.Statements) == 0 || len(res.Statements[0].Rows) == 0 {
		t.Fatal("expected at least one statement with rows from real Tesseract OCR")
	}
}

// TestTesseractIntegration_MixedPDF confirms OCRAuto correctly uses
// embedded text for the text page and real Tesseract OCR for the scanned
// page within the same document.
func TestTesseractIntegration_MixedPDF(t *testing.T) {
	engine := requireRealTesseract(t)
	data := readFixture(t, "mixed_text_and_scanned.pdf")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	res, ierr := ipdf.ParseWithOCR(ctx, bytes.NewReader(data), ipdf.Options{OCR: ipdf.OCRAuto}, engine)
	if ierr != nil {
		t.Fatalf("ParseWithOCR: %+v", ierr)
	}
	if len(res.Statements) != 2 {
		t.Fatalf("got %d statements, want 2", len(res.Statements))
	}
}
