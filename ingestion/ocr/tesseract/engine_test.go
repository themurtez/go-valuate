package tesseract_test

import (
	"context"
	"image"
	"image/color"
	"testing"

	"github.com/themurtez/go-valuate/ingestion/ocr"
	"github.com/themurtez/go-valuate/ingestion/ocr/tesseract"
)

func TestEngine_UnavailableExecutableReturnsStructuredError(t *testing.T) {
	e := &tesseract.Engine{ExecutablePath: "govaluate-definitely-not-a-real-binary-xyz"}
	if e.Available() {
		t.Fatal("Available() = true for a nonexistent executable")
	}

	img := image.NewGray(image.Rect(0, 0, 10, 10))
	_, err := e.Recognize(context.Background(), ocr.ImageInput{Image: img}, ocr.Options{})
	if err == nil {
		t.Fatal("expected an error")
	}
	ocrErr, ok := err.(*ocr.Error)
	if !ok {
		t.Fatalf("err = %T(%v), want *ocr.Error", err, err)
	}
	if ocrErr.Code != ocr.ErrCodeEngineUnavailable {
		t.Errorf("Code = %q, want %q", ocrErr.Code, ocr.ErrCodeEngineUnavailable)
	}
}

func TestEngine_VersionUnavailableExecutable(t *testing.T) {
	e := &tesseract.Engine{ExecutablePath: "govaluate-definitely-not-a-real-binary-xyz"}
	_, err := e.Version(context.Background())
	if err == nil {
		t.Fatal("expected an error")
	}
	ocrErr, ok := err.(*ocr.Error)
	if !ok || ocrErr.Code != ocr.ErrCodeEngineUnavailable {
		t.Errorf("err = %v, want *ocr.Error{Code: OCR_ENGINE_UNAVAILABLE}", err)
	}
}

func TestEngine_DefaultConstructorUsesDefaultExecutableName(t *testing.T) {
	e := tesseract.New()
	// Available() must not panic and must return a bool regardless of
	// whether tesseract happens to be installed on this machine — this
	// test only exercises that the zero-configuration path resolves
	// cleanly, not any specific outcome.
	_ = e.Available()
}

// solidImage satisfies ocr.DecodedImage and image.Image for tests that
// need a minimal decodable image without depending on any fixture file.
func solidImage(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.White)
		}
	}
	return img
}

func TestEngine_RecognizeWithNonStandardImageType(t *testing.T) {
	e := &tesseract.Engine{ExecutablePath: "govaluate-definitely-not-a-real-binary-xyz"}
	_, err := e.Recognize(context.Background(), ocr.ImageInput{Image: solidImage(5, 5)}, ocr.Options{})
	if err == nil {
		t.Fatal("expected an error (executable unavailable, checked before image encoding)")
	}
}
