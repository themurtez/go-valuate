// Real-Tesseract integration tests, gated on the executable actually being
// installed and resolvable — see the task contract's "Tesseract
// integration tests" requirement: run against a real binary when present,
// skip with a clear reason when not, and every OTHER test in this package
// (engine_test.go, tsv_test.go) must still pass regardless, since they
// never depend on a real Tesseract install.
package tesseract_test

import (
	"context"
	"image"
	"image/color"
	"image/draw"
	"testing"
	"time"

	"github.com/themurtez/go-valuate/ingestion/ocr"
	"github.com/themurtez/go-valuate/ingestion/ocr/tesseract"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// requireTesseract skips the calling test with a clear reason if no
// tesseract executable is resolvable on this machine, matching the task
// contract's "SKIPPED — tesseract executable not installed" reporting
// requirement.
func requireTesseract(t *testing.T) *tesseract.Engine {
	t.Helper()
	e := tesseract.New()
	if !e.Available() {
		t.Skip("SKIPPED — tesseract executable not installed")
	}
	return e
}

// renderTextImage draws text onto a white raster image using only the Go
// standard library's basicfont (via golang.org/x/image/font, already an
// indirect dependency of this module via excelize — see go.mod), producing
// a real, if simple, OCR-able synthetic image without any external asset
// file.
func renderTextImage(text string) image.Image {
	width, height := 400, 60
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)

	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(color.Black),
		Face: basicfont.Face7x13,
		Dot:  fixed.Point26_6{X: fixed.I(10), Y: fixed.I(30)},
	}
	d.DrawString(text)
	return img
}

func TestTesseractIntegration_RecognizesSimpleText(t *testing.T) {
	e := requireTesseract(t)

	img := renderTextImage("REVENUE")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	res, err := e.Recognize(ctx, ocr.ImageInput{Image: img, PageIndex: 0}, ocr.Options{})
	if err != nil {
		t.Fatalf("Recognize: %v", err)
	}
	if len(res.Words) == 0 {
		t.Fatal("expected at least one recognized word")
	}
	found := false
	for _, w := range res.Words {
		if w.Text != "" {
			found = true
		}
		if w.Confidence < 0 {
			t.Errorf("word %q has no confidence reported", w.Text)
		}
	}
	if !found {
		t.Error("no non-empty word text recognized")
	}
}

func TestTesseractIntegration_Version(t *testing.T) {
	e := requireTesseract(t)
	v, err := e.Version(context.Background())
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if v == "" {
		t.Error("expected a non-empty version string")
	}
}

func TestTesseractIntegration_TimeoutIsRespected(t *testing.T) {
	e := requireTesseract(t)
	img := renderTextImage("TIMEOUT TEST")

	// An already-cancelled context must fail fast rather than hang or
	// silently ignore cancellation.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := e.Recognize(ctx, ocr.ImageInput{Image: img}, ocr.Options{})
	if err == nil {
		t.Fatal("expected an error from an already-cancelled context")
	}
}
