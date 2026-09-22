package pdfimage_test

import (
	"os"
	"testing"

	"github.com/themurtez/go-valuate/ingestion/pdf/pdfimage"
)

func openFixture(t *testing.T, name string) *os.File {
	t.Helper()
	f, err := os.Open("../../fixtures/" + name)
	if err != nil {
		t.Fatalf("open fixture %q: %v", name, err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

func TestPageCount_ScannedPL(t *testing.T) {
	f := openFixture(t, "scanned_pl.pdf")
	n, err := pdfimage.PageCount(f)
	if err != nil {
		t.Fatalf("PageCount: %v", err)
	}
	if n != 1 {
		t.Errorf("PageCount = %d, want 1", n)
	}
}

func TestPageDims_ScannedPL(t *testing.T) {
	f := openFixture(t, "scanned_pl.pdf")
	dims, err := pdfimage.PageDims(f)
	if err != nil {
		t.Fatalf("PageDims: %v", err)
	}
	if len(dims) != 1 {
		t.Fatalf("got %d page dims, want 1", len(dims))
	}
	if dims[0].WidthPoints != 612 || dims[0].HeightPoints != 792 {
		t.Errorf("dims = %+v, want 612x792 (US Letter)", dims[0])
	}
}

func TestExtractPageImages_ScannedPL_FindsOneFullPageImage(t *testing.T) {
	f := openFixture(t, "scanned_pl.pdf")
	images, skipped, err := pdfimage.ExtractPageImages(f, 1)
	if err != nil {
		t.Fatalf("ExtractPageImages: %v", err)
	}
	if skipped != 0 {
		t.Errorf("skipped = %d, want 0", skipped)
	}
	if len(images) != 1 {
		t.Fatalf("got %d images, want 1", len(images))
	}
	img := images[0]
	if img.Format != pdfimage.ImageFormatJPEG {
		t.Errorf("Format = %q, want jpeg", img.Format)
	}
	if img.PixelWidth <= 0 || img.PixelHeight <= 0 {
		t.Errorf("pixel dims = %dx%d, want positive", img.PixelWidth, img.PixelHeight)
	}
	if img.Image == nil {
		t.Error("Image is nil, want a decoded raster")
	}
}

func TestExtractPageImages_TextOnlyPDFFindsNoImages(t *testing.T) {
	f := openFixture(t, "simple_pl.pdf")
	images, skipped, err := pdfimage.ExtractPageImages(f, 1)
	if err != nil {
		t.Fatalf("ExtractPageImages: %v", err)
	}
	if len(images) != 0 || skipped != 0 {
		t.Errorf("images = %d, skipped = %d, want 0/0 for a text-only PDF page", len(images), skipped)
	}
}

func TestExtractPageImages_ImageOnlyRectFixtureFindsNoRasterImages(t *testing.T) {
	// image_only.pdf (the existing OCR_REQUIRED-detection fixture) contains
	// only a drawn rectangle primitive, never a raster image XObject — so
	// pdfimage must find zero embedded images, distinct from the
	// scanned-page JPEG fixtures.
	f := openFixture(t, "image_only.pdf")
	images, _, err := pdfimage.ExtractPageImages(f, 1)
	if err != nil {
		t.Fatalf("ExtractPageImages: %v", err)
	}
	if len(images) != 0 {
		t.Errorf("got %d images, want 0 (a drawn rectangle is not a raster image)", len(images))
	}
}

func TestSelectDominantImage_ScannedWithLogo_IgnoresLogo(t *testing.T) {
	f := openFixture(t, "scanned_with_logo.pdf")
	dims, err := pdfimage.PageDims(f)
	if err != nil {
		t.Fatalf("PageDims: %v", err)
	}
	f2 := openFixture(t, "scanned_with_logo.pdf")
	images, _, err := pdfimage.ExtractPageImages(f2, 1)
	if err != nil {
		t.Fatalf("ExtractPageImages: %v", err)
	}
	if len(images) != 2 {
		t.Fatalf("got %d images, want 2 (logo + full page)", len(images))
	}
	dominant, ok, reason := pdfimage.SelectDominantImage(images, dims[0])
	if !ok {
		t.Fatalf("ok = false, reason = %q, want a dominant image found despite the logo", reason)
	}
	// The dominant image must be the larger, page-proportioned one, not
	// the small logo.
	if dominant.PixelWidth < 1000 {
		t.Errorf("dominant.PixelWidth = %d, want the full-page image (>=1000px), not the logo", dominant.PixelWidth)
	}
}

func TestSelectDominantImage_MixedPDF_TextPageHasNoImages(t *testing.T) {
	f := openFixture(t, "mixed_text_and_scanned.pdf")
	dims, err := pdfimage.PageDims(f)
	if err != nil {
		t.Fatalf("PageDims: %v", err)
	}
	f2 := openFixture(t, "mixed_text_and_scanned.pdf")
	page1Images, _, err := pdfimage.ExtractPageImages(f2, 1)
	if err != nil {
		t.Fatalf("ExtractPageImages(page 1): %v", err)
	}
	_, ok, reason := pdfimage.SelectDominantImage(page1Images, dims[0])
	if ok {
		t.Error("page 1 (text page) unexpectedly found a dominant image")
	}
	if reason != pdfimage.ReasonNoImages {
		t.Errorf("reason = %q, want %q", reason, pdfimage.ReasonNoImages)
	}

	f3 := openFixture(t, "mixed_text_and_scanned.pdf")
	page2Images, _, err := pdfimage.ExtractPageImages(f3, 2)
	if err != nil {
		t.Fatalf("ExtractPageImages(page 2): %v", err)
	}
	dominant, ok, _ := pdfimage.SelectDominantImage(page2Images, dims[1])
	if !ok {
		t.Fatal("page 2 (scanned page) expected a dominant image")
	}
	if dominant.PixelWidth <= 0 {
		t.Error("expected positive pixel dimensions for page 2's dominant image")
	}
}
