// OCR orchestration: extracting a scanned page's dominant raster image
// (ingestion/pdf/pdfimage), running it through an ingestion/ocr.Engine,
// and converting the result into this package's own word/line types
// (ocr_layout.go). This file contains every bit of OCR-specific
// resource-limit/timeout/error handling; it has zero statement-
// interpretation logic of its own (see this package's doc comment on
// reuse).
package pdf

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/themurtez/go-valuate/ingestion"
	"github.com/themurtez/go-valuate/ingestion/ocr"
	"github.com/themurtez/go-valuate/ingestion/pdf/pdfimage"
)

// ocrPageResult is everything ocrPage produces for one page: reconstructed
// words (ready for groupRows exactly like embedded-text fragments),
// per-word confidence/provenance (ocrWordMeta, keyed identically to the
// returned words slice by index), and non-fatal warnings specific to this
// page's OCR pass.
type ocrPageResult struct {
	words         []word
	wordMeta      []ocrWordMeta
	warnings      []ingestion.Warning
	engineName    string
	engineVersion string
}

// ocrWordMeta carries the OCR-specific provenance/confidence data
// ocr_layout.go's plain word type has no room for, indexed identically to
// ocrPageResult.words (wordMeta[i] describes words[i]) so later stages
// (numeric-safety annotation, provenance attachment) can look up a given
// reconstructed word's OCR origin without threading a parallel structure
// through groupRows/groupWords, which know nothing about OCR.
type ocrWordMeta struct {
	confidence float64
	pageIndex  int
	// pixelBounds is the word's ORIGINAL bounding box in source-image
	// pixel coordinates (distinct from the word's converted PDF-points
	// x0/x1/y — see ocr_layout.go), preserved for OCRProvenance.
	pixelX, pixelY, pixelWidth, pixelHeight int
}

// minPlausibleDPI is the resolution, in pixels per inch, below which a
// scanned page's effective resolution is flagged WarnLowOCRResolution:
// Tesseract's own documentation recommends at least 300 DPI for best
// results and degrades noticeably below ~150 DPI for typical document
// text sizes; 150 is used here as the warning threshold (not a hard
// minimum — OCR is still attempted) since real-world scans below this are
// common enough that refusing them outright would be overly strict, but
// the resulting text/numeric confidence is genuinely less trustworthy and
// callers should know that.
const minPlausibleDPI = 150.0

// estimateDPI computes a page's effective scan resolution from its known
// point dimensions (page.WidthPoints/HeightPoints, from pdfimage.PageDims)
// and its dominant image's actual pixel dimensions — a direct, exact
// computation (unlike pdfimage's own resolution-independent
// aspect-ratio-based SELECTION heuristic, which deliberately avoids
// assuming any DPI baseline — see that package's dominant.go doc comment);
// here, DPI is exactly what it should be once a specific image has already
// been chosen, since page point-size and image pixel-size are both known
// precisely at this stage.
func estimateDPI(pixelWidth, pixelHeight int, pageWidthPts, pageHeightPts float64) float64 {
	if pageWidthPts <= 0 || pageHeightPts <= 0 {
		return 0
	}
	dpiX := float64(pixelWidth) / (pageWidthPts / 72.0)
	dpiY := float64(pixelHeight) / (pageHeightPts / 72.0)
	// The smaller of the two axes is used (a page skewed/cropped
	// asymmetrically in one dimension is only as good as its worse axis
	// for OCR legibility), rounding down rather than averaging toward an
	// overly optimistic figure.
	if dpiX < dpiY {
		return dpiX
	}
	return dpiY
}

// ocrPageBudget bounds a single page's OCR work, derived from
// ingestion.Limits — see limits.go for the additive OCR-only Limits
// fields this package defines.
type ocrPageBudget struct {
	maxImagePixels    int64
	maxImageDimension int
	perPageTimeout    time.Duration
}

// ocrPage extracts pageNumber's (1-based) dominant page image from r,
// preprocesses it (preprocess.go), runs it through engine, and converts
// the result into this package's own word/line-ready words. A page with
// no usable dominant image (SCANNED_PAGE_IMAGE_UNAVAILABLE /
// PDF_PAGE_RENDER_REQUIRED / UNSUPPORTED_SCANNED_PDF_LAYOUT) returns a
// zero ocrPageResult and a non-nil *ingestion.Error — the caller
// (ocr_parse.go) decides whether that fails the whole parse or is
// recorded as a per-page warning, depending on OCR mode.
func ocrPage(
	ctx context.Context,
	r io.ReadSeeker,
	pageNumber int,
	pageDims pdfimage.PageDimensions,
	engine ocr.Engine,
	engineOpts ocr.Options,
	budget ocrPageBudget,
	preprocessOpts PreprocessOptions,
) (ocrPageResult, *ingestion.Error) {
	// pdfcpu's api functions read from r's CURRENT position and do not
	// rewind on their own — see this package's OCR orchestration doc
	// comment (ocr_parse.go) for why every pdfcpu-backed call in this
	// package's OCR path seeks to the start first, rather than requiring
	// every caller to remember to do so themselves.
	if _, seekErr := r.Seek(0, io.SeekStart); seekErr != nil {
		return ocrPageResult{}, &ingestion.Error{
			Code:    ingestion.ErrCodeInvalidFile,
			Message: "failed to seek PDF input for image extraction",
			Detail:  seekErr.Error(),
		}
	}
	images, skipped, extractErr := pdfimage.ExtractPageImages(r, pageNumber)
	if extractErr != nil {
		return ocrPageResult{}, &ingestion.Error{
			Code:    ingestion.ErrCodeInvalidFile,
			Message: fmt.Sprintf("failed to extract images from page %d", pageNumber),
			Detail:  extractErr.Error(),
		}
	}

	var pageWarnings []ingestion.Warning
	if skipped > 0 {
		pageWarnings = append(pageWarnings, ingestion.Warning{
			Code:      ingestion.WarnUnsupportedEmbeddedImage,
			Message:   fmt.Sprintf("%d embedded image(s) on this page used an unsupported encoding and were skipped", skipped),
			RowIndex:  -1,
			PageIndex: pageNumber - 1,
		})
	}

	dominant, ok, reason := pdfimage.SelectDominantImage(images, pageDims)
	if !ok {
		// ReasonUnsupportedLayout means the page's real content is not
		// representable as a single extractable raster image at all (true
		// vector/rendered content this package cannot read without a full
		// PDF page renderer) — a different, more specific condition than
		// "no/ambiguous raster image present" (ReasonNoImages/
		// ReasonMultiplePlausible), so it gets its own, more precise error
		// code (see the ingestion/pdf/pdfimage package doc comment's scope
		// note).
		code := ingestion.ErrCodeScannedPageImageUnavailable
		if reason == pdfimage.ReasonUnsupportedLayout {
			code = ingestion.ErrCodePDFPageRenderRequired
		}
		return ocrPageResult{}, &ingestion.Error{
			Code:    code,
			Message: fmt.Sprintf("page %d has no usable single scanned-page image", pageNumber),
			Detail:  string(reason),
		}
	}

	pixelCount := int64(dominant.PixelWidth) * int64(dominant.PixelHeight)
	if budget.maxImagePixels > 0 && pixelCount > budget.maxImagePixels {
		return ocrPageResult{}, &ingestion.Error{
			Code:    ingestion.ErrCodeOCRImageLimitExceeded,
			Message: fmt.Sprintf("page %d image exceeds maximum pixel count", pageNumber),
			Detail:  fmt.Sprintf("limit is %d pixels, image has %d", budget.maxImagePixels, pixelCount),
		}
	}
	if budget.maxImageDimension > 0 && (dominant.PixelWidth > budget.maxImageDimension || dominant.PixelHeight > budget.maxImageDimension) {
		return ocrPageResult{}, &ingestion.Error{
			Code:    ingestion.ErrCodeOCRImageLimitExceeded,
			Message: fmt.Sprintf("page %d image exceeds maximum single-dimension size", pageNumber),
			Detail:  fmt.Sprintf("limit is %d px per axis, image is %dx%d", budget.maxImageDimension, dominant.PixelWidth, dominant.PixelHeight),
		}
	}

	dpi := estimateDPI(dominant.PixelWidth, dominant.PixelHeight, pageDims.WidthPoints, pageDims.HeightPoints)
	if dpi > 0 && dpi < minPlausibleDPI {
		pageWarnings = append(pageWarnings, ingestion.Warning{
			Code:      ingestion.WarnLowOCRResolution,
			Message:   fmt.Sprintf("page %d's scanned image resolution (~%.0f DPI) is below the recommended minimum for reliable OCR", pageNumber, dpi),
			RowIndex:  -1,
			PageIndex: pageNumber - 1,
		})
	}

	preprocessed, preWarnings := Preprocess(dominant.Image, preprocessOpts)
	pageWarnings = append(pageWarnings, taggedWarnings(preWarnings, pageNumber-1)...)

	runCtx := ctx
	var cancel context.CancelFunc
	if budget.perPageTimeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, budget.perPageTimeout)
		defer cancel()
	}

	recognized, recErr := engine.Recognize(runCtx, ocr.ImageInput{
		Image:     preprocessed,
		PageIndex: pageNumber - 1,
		DPI:       dpi,
	}, engineOpts)
	if recErr != nil {
		if ocrErr, ok := recErr.(*ocr.Error); ok {
			return ocrPageResult{}, &ingestion.Error{
				Code:    ocrErrorCodeToIngestionCode(ocrErr.Code),
				Message: fmt.Sprintf("OCR failed on page %d: %s", pageNumber, ocrErr.Message),
				Detail:  ocrErr.Detail,
			}
		}
		return ocrPageResult{}, &ingestion.Error{
			Code:    ingestion.ErrCodeOCREngineFailed,
			Message: fmt.Sprintf("OCR failed on page %d", pageNumber),
			Detail:  recErr.Error(),
		}
	}

	bounds := preprocessed.Bounds()
	imgW, imgH := bounds.Dx(), bounds.Dy()

	converted := make([]recognizedWord, len(recognized.Words))
	for i, w := range recognized.Words {
		converted[i] = recognizedWord{
			Text: w.Text, Confidence: w.Confidence,
			X: w.X, Y: w.Y, Width: w.Width, Height: w.Height,
			PageIndex: pageNumber - 1,
			BlockNum:  w.BlockNum, ParNum: w.ParNum, LineNum: w.LineNum,
		}
	}

	words := ocrWordsToWords(converted, pageDims.WidthPoints, pageDims.HeightPoints, imgW, imgH)
	meta := make([]ocrWordMeta, len(converted))
	for i, w := range converted {
		meta[i] = ocrWordMeta{
			confidence: w.Confidence, pageIndex: w.PageIndex,
			pixelX: w.X, pixelY: w.Y, pixelWidth: w.Width, pixelHeight: w.Height,
		}
	}

	pageWarnings = append(pageWarnings, ingestion.Warning{
		Code:      ingestion.WarnOCRUsed,
		Message:   fmt.Sprintf("page %d was read via OCR (no usable embedded text layer)", pageNumber),
		RowIndex:  -1,
		PageIndex: pageNumber - 1,
	})

	return ocrPageResult{
		words: words, wordMeta: meta, warnings: pageWarnings,
		engineName: recognized.EngineName, engineVersion: recognized.EngineVersion,
	}, nil
}

func taggedWarnings(in []ingestion.Warning, pageIndex int) []ingestion.Warning {
	out := make([]ingestion.Warning, len(in))
	for i, w := range in {
		w.PageIndex = pageIndex
		out[i] = w
	}
	return out
}

func ocrErrorCodeToIngestionCode(code ocr.ErrorCode) ingestion.ErrorCode {
	switch code {
	case ocr.ErrCodeEngineUnavailable:
		return ingestion.ErrCodeOCREngineUnavailable
	case ocr.ErrCodeTimeout:
		return ingestion.ErrCodeOCRTimeout
	default:
		return ingestion.ErrCodeOCREngineFailed
	}
}
