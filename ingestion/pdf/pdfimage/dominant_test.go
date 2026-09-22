package pdfimage

import "testing"

// a letter-sized page in PDF points (612x792 = US Letter).
var letterPage = PageDimensions{WidthPoints: 612, HeightPoints: 792}

func fullPageImageAt150DPI() PageImage {
	// 612/72*150 x 792/72*150 = 1275 x 1650 pixels: a full-page scan at a
	// common scanning resolution, sharing the page's exact aspect ratio.
	return PageImage{PixelWidth: 1275, PixelHeight: 1650}
}

func lowResFullPageImage() PageImage {
	// Half the resolution of fullPageImageAt150DPI but the SAME aspect
	// ratio — must still be recognized as page-shaped (this is the case
	// the earlier assumed-DPI area-projection design got wrong).
	return PageImage{PixelWidth: 637, PixelHeight: 825}
}

func smallLogoImage() PageImage {
	// A wide-and-short letterhead-logo shape, clearly not page-proportioned.
	return PageImage{PixelWidth: 400, PixelHeight: 100}
}

func TestLooksPageShaped_FullPageScanAtBaselineDPI(t *testing.T) {
	if !looksPageShaped(fullPageImageAt150DPI(), letterPage) {
		t.Error("expected a full-resolution page-proportioned scan to look page-shaped")
	}
}

func TestLooksPageShaped_LowResolutionFullPageScanStillRecognized(t *testing.T) {
	if !looksPageShaped(lowResFullPageImage(), letterPage) {
		t.Error("expected a lower-resolution BUT page-proportioned scan to still look page-shaped (resolution-independent)")
	}
}

func TestLooksPageShaped_LogoShapeRejected(t *testing.T) {
	if looksPageShaped(smallLogoImage(), letterPage) {
		t.Error("expected a wide-and-short logo shape to NOT look page-shaped")
	}
}

func TestLooksPageShaped_TinyThumbnailWithPageAspectRejected(t *testing.T) {
	// Same aspect ratio as the page, but far too few pixels to be a real
	// scan — the absolute-size floor must still reject it.
	tiny := PageImage{PixelWidth: 61, PixelHeight: 79}
	if looksPageShaped(tiny, letterPage) {
		t.Error("expected a tiny page-proportioned thumbnail to be rejected on absolute size")
	}
}

func TestLooksPageShaped_ZeroPageDimensionsReturnsFalse(t *testing.T) {
	if looksPageShaped(fullPageImageAt150DPI(), PageDimensions{}) {
		t.Error("expected false for a zero-sized page")
	}
}

func TestLooksPageShaped_ZeroImageDimensionsReturnsFalse(t *testing.T) {
	if looksPageShaped(PageImage{}, letterPage) {
		t.Error("expected false for a zero-sized image")
	}
}

func TestSelectDominantImage_NoImages(t *testing.T) {
	_, ok, reason := SelectDominantImage(nil, letterPage)
	if ok {
		t.Fatal("ok = true, want false")
	}
	if reason != ReasonNoImages {
		t.Errorf("reason = %q, want %q", reason, ReasonNoImages)
	}
}

func TestSelectDominantImage_SingleDominantImage(t *testing.T) {
	full := fullPageImageAt150DPI()
	logo := smallLogoImage()
	dominant, ok, reason := SelectDominantImage([]PageImage{logo, full}, letterPage)
	if !ok {
		t.Fatalf("ok = false, reason = %q, want true", reason)
	}
	if reason != ReasonNone {
		t.Errorf("reason = %q, want empty", reason)
	}
	if dominant.PixelWidth != full.PixelWidth || dominant.PixelHeight != full.PixelHeight {
		t.Errorf("dominant = %+v, want the full-page image", dominant)
	}
}

func TestSelectDominantImage_LowResolutionScanStillSelected(t *testing.T) {
	lowRes := lowResFullPageImage()
	logo := smallLogoImage()
	dominant, ok, reason := SelectDominantImage([]PageImage{logo, lowRes}, letterPage)
	if !ok {
		t.Fatalf("ok = false, reason = %q, want true", reason)
	}
	if dominant.PixelWidth != lowRes.PixelWidth {
		t.Errorf("dominant = %+v, want the low-res full-page image", dominant)
	}
}

func TestSelectDominantImage_MultiplePlausibleImagesIsAmbiguous(t *testing.T) {
	full1 := fullPageImageAt150DPI()
	full2 := PageImage{PixelWidth: 1200, PixelHeight: 1600} // also clearly page-shaped
	_, ok, reason := SelectDominantImage([]PageImage{full1, full2}, letterPage)
	if ok {
		t.Fatal("ok = true, want false (ambiguous)")
	}
	if reason != ReasonMultiplePlausible {
		t.Errorf("reason = %q, want %q", reason, ReasonMultiplePlausible)
	}
}

func TestSelectDominantImage_OnlyLogosNoQualifyingImage(t *testing.T) {
	_, ok, reason := SelectDominantImage([]PageImage{smallLogoImage(), smallLogoImage()}, letterPage)
	if ok {
		t.Fatal("ok = true, want false")
	}
	if reason != ReasonUnsupportedLayout {
		t.Errorf("reason = %q, want %q", reason, ReasonUnsupportedLayout)
	}
}

// TestSelectDominantImage_NeverPicksByByteSizeAlone confirms the selection
// signal is page-aspect-ratio shape, not raw pixel/byte volume: a very
// large but clearly non-page-shaped image (e.g. a huge decorative banner
// far wider than the page but very short) must not be picked over a
// genuine page-shaped candidate just because it has more total pixels.
func TestSelectDominantImage_NeverPicksByByteSizeAlone(t *testing.T) {
	full := fullPageImageAt150DPI() // 1275 x 1650 = 2,103,750 px
	wideBanner := PageImage{PixelWidth: 4000, PixelHeight: 300}
	// wideBanner has more total pixels (1,200,000) than would matter under
	// a byte/pixel-volume heuristic in some scenarios, but its aspect
	// ratio (13.3:1) is nowhere near the page's (~0.77:1), so it must be
	// rejected regardless of pixel count.
	dominant, ok, reason := SelectDominantImage([]PageImage{wideBanner, full}, letterPage)
	if !ok {
		t.Fatalf("ok = false, reason = %q", reason)
	}
	if dominant.PixelWidth != full.PixelWidth {
		t.Errorf("dominant = %+v, want the page-shaped image, not the banner", dominant)
	}
}
