// Dominant page-image selection: given every embedded raster image found
// on a page, decide whether exactly one of them plausibly represents the
// whole scanned page, as opposed to a logo/icon/decorative element. Per
// the task contract, this NEVER simply picks the largest byte stream — see
// looksPageShaped and SelectDominantImage.
package pdfimage

import "math"

// aspectRatioTolerance is the maximum relative difference allowed between
// an embedded image's own pixel aspect ratio (width/height) and the PDF
// page's own aspect ratio (MediaBox width/height) for that image to be
// considered "page-shaped." Aspect ratio is used as the PRIMARY signal —
// deliberately NOT an assumed-DPI area projection, and never raw
// byte-stream/pixel-count size — specifically because it is
// resolution-independent: a genuine full-page scan has the same aspect
// ratio as its page regardless of whether it was scanned at 75 DPI or 600
// DPI, so this correctly distinguishes "a low-resolution full-page scan"
// (same shape, fewer pixels — still a dominant-image candidate) from "a
// small logo/icon" (almost always a very different aspect ratio than a
// full page — e.g. a wide-and-short letterhead banner, or a roughly-square
// stamp). See minAbsolutePixelFraction below for the secondary signal that
// still rules out a small image that HAPPENS to share the page's aspect
// ratio, e.g. a small centered thumbnail.
//
// 0.15 (15% relative difference) comfortably tolerates ordinary scan
// margin/cropping variation (a real scan is rarely pixel-perfectly
// proportional to its page's MediaBox) while still rejecting a landscape
// logo against a portrait page or similar.
const aspectRatioTolerance = 0.15

// minAbsolutePixelFraction is the minimum fraction of
// minPlausibleScanLongEdgePixels an image's own longer pixel dimension
// must reach to be considered a plausible full-page scan candidate, even
// if its aspect ratio matches the page — this rules out a tiny thumbnail
// that happens to share the page's proportions (e.g. a 40x52px preview
// icon) without depending on any assumed absolute DPI for the PRIMARY
// aspect-ratio signal above.
const minAbsolutePixelFraction = 0.5

// minPlausibleScanLongEdgePixels is a floor for a real scanned page's
// longer pixel dimension: even a deliberately low-resolution scan (see
// ingestion/pdf's LOW_OCR_RESOLUTION warning, which fires separately once
// a dominant image IS selected) still typically has at least a few
// hundred pixels along its long edge — this is only a coarse floor to
// reject genuinely icon-sized images, not a resolution-quality judgment
// (that judgment belongs to ingestion/pdf's own DPI-estimation logic,
// downstream of selection).
const minPlausibleScanLongEdgePixels = 300

// pageAspectRatio returns page's width/height ratio, or 0 if page has no
// usable dimensions.
func pageAspectRatio(page PageDimensions) float64 {
	if page.WidthPoints <= 0 || page.HeightPoints <= 0 {
		return 0
	}
	return page.WidthPoints / page.HeightPoints
}

// imageAspectRatio returns img's pixel width/height ratio, or 0 if img has
// no usable dimensions.
func imageAspectRatio(img PageImage) float64 {
	if img.PixelWidth <= 0 || img.PixelHeight <= 0 {
		return 0
	}
	return float64(img.PixelWidth) / float64(img.PixelHeight)
}

// looksPageShaped reports whether img's aspect ratio is within
// aspectRatioTolerance of page's aspect ratio AND img's longer pixel edge
// meets the absolute-size floor — see the constants above for why both
// checks are needed together.
func looksPageShaped(img PageImage, page PageDimensions) bool {
	pageRatio := pageAspectRatio(page)
	imgRatio := imageAspectRatio(img)
	if pageRatio == 0 || imgRatio == 0 {
		return false
	}
	relDiff := math.Abs(imgRatio-pageRatio) / pageRatio
	if relDiff > aspectRatioTolerance {
		return false
	}

	longEdge := img.PixelWidth
	if img.PixelHeight > longEdge {
		longEdge = img.PixelHeight
	}
	return float64(longEdge) >= minPlausibleScanLongEdgePixels*minAbsolutePixelFraction
}

// SelectDominantImage picks the single image among images that plausibly
// represents page's entire scanned content, using aspect-ratio shape
// matching against the page's own MediaBox proportions (never raw
// byte-stream size, and never an assumed absolute DPI — see
// looksPageShaped) as the deciding signal:
//
//   - zero images -> ok=false, ReasonNoImages
//   - exactly one image judged page-shaped -> that image, ok=true
//   - two or more images independently judged page-shaped -> ok=false,
//     ReasonMultiplePlausible (ambiguous; this package refuses to guess
//     which one is the real page scan rather than arbitrarily picking the
//     largest)
//   - one or more images present but NONE are page-shaped -> ok=false,
//     ReasonUnsupportedLayout (every candidate looks like a logo/icon/
//     partial image, not a full-page scan)
func SelectDominantImage(images []PageImage, page PageDimensions) (dominant PageImage, ok bool, reason Reason) {
	if len(images) == 0 {
		return PageImage{}, false, ReasonNoImages
	}

	var candidates []PageImage
	for _, img := range images {
		if looksPageShaped(img, page) {
			candidates = append(candidates, img)
		}
	}

	switch len(candidates) {
	case 0:
		return PageImage{}, false, ReasonUnsupportedLayout
	case 1:
		return candidates[0], true, ReasonNone
	default:
		return PageImage{}, false, ReasonMultiplePlausible
	}
}
