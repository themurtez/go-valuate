package pdf_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/themurtez/go-valuate/ingestion"
	"github.com/themurtez/go-valuate/ingestion/ocr"
	ipdf "github.com/themurtez/go-valuate/ingestion/pdf"
)

// TestFixture_ScannedWithLogo_DominantImageSelectedNotLogo exercises
// scanned_with_logo.pdf end-to-end through ParseWithOCR: the page
// contains two embedded images (a small logo plus the dominant full-page
// scan), and OCR must run against the dominant scan image, never the
// logo, without any MULTIPLE_PAGE_IMAGES/NO_DOMINANT_PAGE_IMAGE warning
// (since exactly one image is unambiguously page-shaped).
func TestFixture_ScannedWithLogo_DominantImageSelectedNotLogo(t *testing.T) {
	data := readFixture(t, "scanned_with_logo.pdf")
	words := []ocr.Word{
		{Text: "Golden Valley Farms Co-op", X: 100, Y: 120, Width: 220, Height: 16, PageIndex: 0, Confidence: 91},
		{Text: "Income Statement", X: 100, Y: 150, Width: 120, Height: 16, PageIndex: 0, Confidence: 91},
		{Text: "Account", X: 100, Y: 180, Width: 60, Height: 16, PageIndex: 0, Confidence: 91},
		{Text: "FY2025", X: 700, Y: 180, Width: 60, Height: 16, PageIndex: 0, Confidence: 91},
		{Text: "Revenue", X: 100, Y: 210, Width: 60, Height: 16, PageIndex: 0, Confidence: 91},
		{Text: "920,000", X: 700, Y: 210, Width: 70, Height: 16, PageIndex: 0, Confidence: 91},
		{Text: "Net Income", X: 100, Y: 240, Width: 80, Height: 16, PageIndex: 0, Confidence: 91},
		{Text: "565,000", X: 700, Y: 240, Width: 70, Height: 16, PageIndex: 0, Confidence: 91},
	}
	engine := &ocr.FakeEngine{Pages: map[int]ocr.Result{0: {EngineName: "fake", Words: words}}}

	res, ierr := ipdf.ParseWithOCR(context.Background(), bytes.NewReader(data), ipdf.Options{OCR: ipdf.OCRAuto}, engine)
	if ierr != nil {
		t.Fatalf("ParseWithOCR: %+v", ierr)
	}
	if len(engine.Calls) != 1 {
		t.Fatalf("engine.Calls = %v, want exactly 1 (OCR should run once against the dominant image)", engine.Calls)
	}
	if len(res.Statements) != 1 {
		t.Fatalf("got %d statements, want 1", len(res.Statements))
	}

	revenue, ok := rowByLabel(res.Statements[0].Rows, "Revenue")
	if !ok || revenue.Values["FY2025"] != 920000 {
		t.Errorf("Revenue = %+v, want FY2025=920000", revenue)
	}

	for _, w := range res.Warnings {
		if w.Code == ingestion.WarnMultiplePageImages || w.Code == ingestion.WarnNoDominantPageImage {
			t.Errorf("unexpected warning %s: the logo must not cause dominant-image ambiguity", w.Code)
		}
	}
}

// TestFixture_ScannedLowResolution_StillOCRsWithWarning exercises
// scanned_low_resolution.pdf: a genuinely lower-pixel-resolution but
// still page-proportioned scan must still be selected as the dominant
// image (see ingestion/pdf/pdfimage's aspect-ratio-based selection) and
// still successfully OCR'd, while emitting LOW_OCR_RESOLUTION.
func TestFixture_ScannedLowResolution_StillOCRsWithWarning(t *testing.T) {
	data := readFixture(t, "scanned_low_resolution.pdf")
	words := []ocr.Word{
		{Text: "Bramblewood Supply Co.", X: 50, Y: 60, Width: 110, Height: 8, PageIndex: 0, Confidence: 80},
		{Text: "Income Statement", X: 50, Y: 75, Width: 60, Height: 8, PageIndex: 0, Confidence: 80},
		{Text: "Account", X: 50, Y: 90, Width: 30, Height: 8, PageIndex: 0, Confidence: 80},
		{Text: "FY2025", X: 350, Y: 90, Width: 30, Height: 8, PageIndex: 0, Confidence: 80},
		{Text: "Revenue", X: 50, Y: 105, Width: 30, Height: 8, PageIndex: 0, Confidence: 80},
		{Text: "410,000", X: 350, Y: 105, Width: 35, Height: 8, PageIndex: 0, Confidence: 80},
	}
	engine := &ocr.FakeEngine{Pages: map[int]ocr.Result{0: {EngineName: "fake", Words: words}}}

	res, ierr := ipdf.ParseWithOCR(context.Background(), bytes.NewReader(data), ipdf.Options{OCR: ipdf.OCRAuto}, engine)
	if ierr != nil {
		t.Fatalf("ParseWithOCR: %+v", ierr)
	}
	if len(engine.Calls) != 1 {
		t.Fatalf("engine.Calls = %v, want 1", engine.Calls)
	}

	foundLowRes := false
	for _, w := range res.Warnings {
		if w.Code == ingestion.WarnLowOCRResolution {
			foundLowRes = true
		}
	}
	if !foundLowRes {
		t.Error("expected WarnLowOCRResolution for the deliberately low-resolution scan")
	}
}

// TestFixture_ScannedSkewed_StillReconstructsRows confirms a mildly
// skewed scan (each line shifted slightly right — see scan_image.go's
// skewPixelsPerLine) still reconstructs into distinct rows via the shared
// row/column reconstruction logic, without requiring any deskew step
// (this package's row grouping tolerance already absorbs small skew via
// the same Y-tolerance mechanism used for embedded PDF text).
func TestFixture_ScannedSkewed_StillReconstructsRows(t *testing.T) {
	data := readFixture(t, "scanned_skewed.pdf")
	words := []ocr.Word{
		{Text: "Harmon Freight Systems", X: 100, Y: 120, Width: 200, Height: 16, PageIndex: 0, Confidence: 88},
		{Text: "Income Statement", X: 103, Y: 150, Width: 120, Height: 16, PageIndex: 0, Confidence: 88},
		{Text: "Account", X: 106, Y: 180, Width: 60, Height: 16, PageIndex: 0, Confidence: 88},
		{Text: "FY2025", X: 706, Y: 180, Width: 60, Height: 16, PageIndex: 0, Confidence: 88},
		{Text: "Revenue", X: 109, Y: 210, Width: 60, Height: 16, PageIndex: 0, Confidence: 88},
		{Text: "1,240,000", X: 709, Y: 210, Width: 80, Height: 16, PageIndex: 0, Confidence: 88},
		{Text: "Net Income", X: 112, Y: 240, Width: 80, Height: 16, PageIndex: 0, Confidence: 88},
		{Text: "290,000", X: 712, Y: 240, Width: 70, Height: 16, PageIndex: 0, Confidence: 88},
	}
	engine := &ocr.FakeEngine{Pages: map[int]ocr.Result{0: {EngineName: "fake", Words: words}}}

	res, ierr := ipdf.ParseWithOCR(context.Background(), bytes.NewReader(data), ipdf.Options{OCR: ipdf.OCRAuto}, engine)
	if ierr != nil {
		t.Fatalf("ParseWithOCR: %+v", ierr)
	}
	revenue, ok := rowByLabel(res.Statements[0].Rows, "Revenue")
	if !ok || revenue.Values["FY2025"] != 1240000 {
		t.Errorf("Revenue = %+v, want FY2025=1240000", revenue)
	}
}
