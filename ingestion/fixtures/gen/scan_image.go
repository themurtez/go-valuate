// Synthetic "scanned page" raster image generation, for the OCR fixture
// corpus (generate_ocr_pdf.go). Renders plain financial-statement-shaped
// text onto a white raster canvas using only the Go standard library plus
// golang.org/x/image/font's basicfont (already a project dependency — see
// ingestion/ocr/tesseract's real-Tesseract integration test, which uses
// the same approach), then JPEG-encodes it, producing a deterministic,
// fully redistributable synthetic "scan" with no proprietary/customer
// content and no external asset file.
package main

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// scanLine is one line of text to render at a given row on a synthetic
// scanned page image, in a simple two-column layout (label left-aligned at
// a fixed X, value right-aligned within a fixed-width value column) that
// approximates how a real scanned financial statement lays out.
type scanLine struct {
	label string
	value string // empty means label-only (e.g. a title or section heading)
}

// scanImageOptions controls synthetic scan rendering, letting fixtures
// exercise deliberate quality variations (low resolution, skew) that a
// real scanned document can exhibit.
type scanImageOptions struct {
	// widthPx, heightPx are the canvas size in pixels. Zero means the
	// package default (a realistic ~150 DPI US-Letter page).
	widthPx, heightPx int
	// skewPixelsPerLine, when non-zero, shifts each successive line
	// slightly further right, simulating a mild page skew from an
	// imperfectly-aligned scanner feed.
	skewPixelsPerLine int
	// jpegQuality controls JPEG compression quality (1-100); 0 means a
	// high-quality default (90).
	jpegQuality int
}

// defaultScanWidthPx, defaultScanHeightPx approximate a US-Letter page
// (8.5x11in) scanned at 150 DPI: 1275x1650 pixels — comfortably above
// Tesseract's recommended minimum (~150 DPI) for reliable recognition, and
// matching pdfimage's own assumedScanDPI baseline so these fixtures land
// solidly above minPageCoverageRatio during dominant-image selection.
const (
	defaultScanWidthPx  = 1275
	defaultScanHeightPx = 1650
)

// renderScanImage draws lines onto a white canvas starting near the top,
// left-aligned labels at x=100px with right-aligned-ish values at
// x=700px, and returns the encoded JPEG bytes plus the canvas's pixel
// dimensions.
func renderScanImage(lines []scanLine, opts scanImageOptions) (jpegData []byte, widthPx, heightPx int) {
	w := opts.widthPx
	if w == 0 {
		w = defaultScanWidthPx
	}
	h := opts.heightPx
	if h == 0 {
		h = defaultScanHeightPx
	}
	quality := opts.jpegQuality
	if quality == 0 {
		quality = 90
	}

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)

	face := basicfont.Face7x13
	lineHeight := 30 // pixels between lines, generously spaced for legibility at this resolution
	startY := 120
	labelX := 100
	valueX := 700

	for i, ln := range lines {
		y := startY + i*lineHeight
		skew := opts.skewPixelsPerLine * i

		d := &font.Drawer{
			Dst:  img,
			Src:  image.NewUniform(color.Black),
			Face: face,
			Dot:  fixed.P(labelX+skew, y),
		}
		d.DrawString(ln.label)

		if ln.value != "" {
			d2 := &font.Drawer{
				Dst:  img,
				Src:  image.NewUniform(color.Black),
				Face: face,
				Dot:  fixed.P(valueX+skew, y),
			}
			d2.DrawString(ln.value)
		}
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		panic(err) // fixture generation is a dev-time tool; a codec failure here is a programming error
	}
	return buf.Bytes(), w, h
}
