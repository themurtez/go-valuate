package pdf_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/themurtez/go-valuate/ingestion"
	"github.com/themurtez/go-valuate/ingestion/ocr"
	ipdf "github.com/themurtez/go-valuate/ingestion/pdf"
)

func TestParseWithOCR_RespectsMaxImagePixelsLimit(t *testing.T) {
	data := readFixture(t, "scanned_pl.pdf")
	engine := &ocr.FakeEngine{Pages: map[int]ocr.Result{0: {EngineName: "fake", Words: wordsForScannedPL(95.0)}}}
	opts := ipdf.Options{OCR: ipdf.OCRAuto}
	opts.Limits.MaxImagePixels = 1000 // far below scanned_pl.pdf's real 1275x1650 image
	_, ierr := ipdf.ParseWithOCR(context.Background(), bytes.NewReader(data), opts, engine)
	if ierr == nil {
		t.Fatal("expected an error")
	}
	if ierr.Code != ingestion.ErrCodeOCRImageLimitExceeded {
		t.Errorf("Code = %q, want %q", ierr.Code, ingestion.ErrCodeOCRImageLimitExceeded)
	}
}

func TestParseWithOCR_RespectsMaxImageDimensionLimit(t *testing.T) {
	data := readFixture(t, "scanned_pl.pdf")
	engine := &ocr.FakeEngine{Pages: map[int]ocr.Result{0: {EngineName: "fake", Words: wordsForScannedPL(95.0)}}}
	opts := ipdf.Options{OCR: ipdf.OCRAuto}
	opts.Limits.MaxImageDimension = 500 // scanned_pl.pdf's image is 1275x1650, exceeding this on both axes
	_, ierr := ipdf.ParseWithOCR(context.Background(), bytes.NewReader(data), opts, engine)
	if ierr == nil {
		t.Fatal("expected an error")
	}
	if ierr.Code != ingestion.ErrCodeOCRImageLimitExceeded {
		t.Errorf("Code = %q, want %q", ierr.Code, ingestion.ErrCodeOCRImageLimitExceeded)
	}
}

func TestParseWithOCR_RespectsMaxOCRWordsLimit(t *testing.T) {
	data := readFixture(t, "scanned_pl.pdf")
	// wordsForScannedPL(...) produces 14 words total; set the limit below that.
	engine := &ocr.FakeEngine{Pages: map[int]ocr.Result{0: {EngineName: "fake", Words: wordsForScannedPL(95.0)}}}
	opts := ipdf.Options{OCR: ipdf.OCRAuto}
	opts.Limits.MaxOCRWords = 3
	_, ierr := ipdf.ParseWithOCR(context.Background(), bytes.NewReader(data), opts, engine)
	if ierr == nil {
		t.Fatal("expected an error")
	}
	if ierr.Code != ingestion.ErrCodeLimitExceeded {
		t.Errorf("Code = %q, want %q", ierr.Code, ingestion.ErrCodeLimitExceeded)
	}
}

func TestParseWithOCR_RespectsMaxOCRTextBytesLimit(t *testing.T) {
	data := readFixture(t, "scanned_pl.pdf")
	engine := &ocr.FakeEngine{Pages: map[int]ocr.Result{0: {EngineName: "fake", Words: wordsForScannedPL(95.0)}}}
	opts := ipdf.Options{OCR: ipdf.OCRAuto}
	opts.Limits.MaxOCRTextBytes = 5 // far below the total recognized text size
	_, ierr := ipdf.ParseWithOCR(context.Background(), bytes.NewReader(data), opts, engine)
	if ierr == nil {
		t.Fatal("expected an error")
	}
	if ierr.Code != ingestion.ErrCodeLimitExceeded {
		t.Errorf("Code = %q, want %q", ierr.Code, ingestion.ErrCodeLimitExceeded)
	}
}

func TestParseWithOCR_DefaultLimitsDoNotRejectNormalFixtures(t *testing.T) {
	data := readFixture(t, "scanned_pl.pdf")
	engine := &ocr.FakeEngine{Pages: map[int]ocr.Result{0: {EngineName: "fake", Words: wordsForScannedPL(95.0)}}}
	_, ierr := ipdf.ParseWithOCR(context.Background(), bytes.NewReader(data), ipdf.Options{OCR: ipdf.OCRAuto}, engine)
	if ierr != nil {
		t.Fatalf("default limits unexpectedly rejected a normal fixture: %+v", ierr)
	}
}
