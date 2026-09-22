package pdf_test

import (
	"image"
	"image/color"
	"testing"

	ipdf "github.com/themurtez/go-valuate/ingestion/pdf"
)

func makeGrayGradient(w, h int, lo, hi uint8) *image.Gray {
	img := image.NewGray(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		v := lo + uint8(int(hi-lo)*x/(w-1))
		for y := 0; y < h; y++ {
			img.SetGray(x, y, color.Gray{Y: v})
		}
	}
	return img
}

func TestPreprocess_ZeroOptionsReturnsUnmodifiedImage(t *testing.T) {
	src := makeGrayGradient(10, 10, 50, 200)
	out, warnings := ipdf.Preprocess(src, ipdf.PreprocessOptions{})
	if len(warnings) != 0 {
		t.Errorf("warnings = %v, want none", warnings)
	}
	if out != image.Image(src) {
		t.Error("expected the exact same image value back for zero-value options")
	}
}

func TestPreprocess_Grayscale(t *testing.T) {
	rgba := image.NewRGBA(image.Rect(0, 0, 5, 5))
	for y := 0; y < 5; y++ {
		for x := 0; x < 5; x++ {
			rgba.Set(x, y, color.RGBA{R: 100, G: 150, B: 200, A: 255})
		}
	}
	out, _ := ipdf.Preprocess(rgba, ipdf.PreprocessOptions{Grayscale: true})
	gray, ok := out.(*image.Gray)
	if !ok {
		t.Fatalf("out = %T, want *image.Gray", out)
	}
	if gray.Bounds() != rgba.Bounds() {
		t.Errorf("bounds = %v, want %v", gray.Bounds(), rgba.Bounds())
	}
}

func TestPreprocess_ContrastStretch_ExpandsNarrowRange(t *testing.T) {
	src := makeGrayGradient(100, 10, 100, 150) // narrow range
	out, _ := ipdf.Preprocess(src, ipdf.PreprocessOptions{Grayscale: true, ContrastStretch: true})
	gray := out.(*image.Gray)

	minV, maxV := uint8(255), uint8(0)
	b := gray.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			v := gray.GrayAt(x, y).Y
			if v < minV {
				minV = v
			}
			if v > maxV {
				maxV = v
			}
		}
	}
	if minV > 5 {
		t.Errorf("stretched min = %d, want near 0", minV)
	}
	if maxV < 250 {
		t.Errorf("stretched max = %d, want near 255", maxV)
	}
}

func TestPreprocess_ContrastStretch_FlatImageUnchanged(t *testing.T) {
	flat := image.NewGray(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			flat.SetGray(x, y, color.Gray{Y: 128})
		}
	}
	out, _ := ipdf.Preprocess(flat, ipdf.PreprocessOptions{Grayscale: true, ContrastStretch: true})
	gray := out.(*image.Gray)
	if gray.GrayAt(5, 5).Y != 128 {
		t.Errorf("flat image was altered: got %d, want 128", gray.GrayAt(5, 5).Y)
	}
}

func TestPreprocess_Threshold_Binarizes(t *testing.T) {
	src := makeGrayGradient(256, 4, 0, 255)
	out, _ := ipdf.Preprocess(src, ipdf.PreprocessOptions{Grayscale: true, Threshold: 128})
	gray := out.(*image.Gray)

	b := gray.Bounds()
	for x := b.Min.X; x < b.Max.X; x++ {
		v := gray.GrayAt(x, 0).Y
		if v != 0 && v != 255 {
			t.Fatalf("pixel at x=%d has value %d, want 0 or 255 (binarized)", x, v)
		}
	}
}

func TestPreprocess_ThresholdThenGrayscaleOrderIsDeterministic(t *testing.T) {
	src := makeGrayGradient(50, 5, 0, 255)
	out1, _ := ipdf.Preprocess(src, ipdf.PreprocessOptions{Grayscale: true, ContrastStretch: true, Threshold: 128})
	out2, _ := ipdf.Preprocess(src, ipdf.PreprocessOptions{Grayscale: true, ContrastStretch: true, Threshold: 128})

	g1, g2 := out1.(*image.Gray), out2.(*image.Gray)
	for i := range g1.Pix {
		if g1.Pix[i] != g2.Pix[i] {
			t.Fatalf("preprocessing is not deterministic: pixel %d differs (%d vs %d)", i, g1.Pix[i], g2.Pix[i])
		}
	}
}
