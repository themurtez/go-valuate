// Automatic OCR fallback: the high-level entry point that decides, PER
// PAGE, whether to use this package's existing embedded-text extraction
// (extract.go/layout.go) or OCR (ocr_engine.go/ocr_layout.go), then feeds
// EITHER source's reconstructed lines into the exact same
// splitSections/buildSectionGrid/buildSectionResult pipeline Parse already
// uses (parser.go) — see this package's doc comment's core reuse
// requirement. There is exactly one statement-interpretation pipeline in
// this package; this file only decides which extraction path feeds it,
// per page.
package pdf

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/themurtez/go-valuate/ingestion"
	"github.com/themurtez/go-valuate/ingestion/internal/tabular"
	"github.com/themurtez/go-valuate/ingestion/ocr"
	"github.com/themurtez/go-valuate/ingestion/pdf/pdfimage"
)

// OCRMode selects how ParseWithOCR decides between embedded-text
// extraction and OCR. See OCRDisabled/OCRAuto/OCRForce.
type OCRMode string

const (
	// OCRDisabled means never attempt OCR — identical behavior to Parse:
	// a page/document with no usable embedded text layer produces
	// ErrCodeOCRRequired. This is the default (zero value) so existing
	// callers of Parse (which internally uses OCRDisabled) see no
	// behavior change whatsoever.
	OCRDisabled OCRMode = ""
	// OCRAuto tries embedded-text extraction first, for the WHOLE
	// document; only if the document's text layer is missing/unusable
	// (the same hasUsableTextLayer check Parse already uses) does it fall
	// back to OCR — and even then, only for the SPECIFIC PAGES that need
	// it (see ocrPagesNeeded), never OCR-ing a page whose own embedded
	// text was already usable. This is the mode that supports "mixed"
	// PDFs (some pages text, some pages scanned) — see the package doc
	// comment's mixed-PDF section.
	OCRAuto OCRMode = "auto"
	// OCRForce ignores any embedded text layer entirely and OCRs every
	// page's dominant image, regardless of whether usable embedded text
	// exists. Useful when a caller knows a document's embedded text layer
	// is unreliable (e.g. a bad OCR pass baked in by a previous tool) and
	// wants this package's own OCR path used uniformly instead.
	OCRForce OCRMode = "force"
)

// ParseWithOCR extends Parse (parser.go) with optional OCR fallback for
// scanned/image-only pages — see OCRMode. With opts.OCR left at
// OCRDisabled (the zero value), ParseWithOCR simply delegates to the
// unmodified Parse and returns its result directly — Parse's own
// behavior is completely unaffected by this function's existence, and a
// caller who never sets opts.OCR sees no difference from calling Parse
// itself. r must support io.Seek (matching pdfimage's own requirement —
// see that package's doc comment) in addition to the plain io.Reader
// Parse itself accepts, since OCR mode needs random access for BOTH
// github.com/ledongthuc/pdf's embedded-text extraction and
// github.com/pdfcpu/pdfcpu's image extraction, potentially interleaved
// across pages.
//
// ctx bounds the OCR portion of parsing (embedded-text extraction has no
// meaningful cancellation point of its own, matching Parse's existing
// synchronous behavior) — see Limits.OCRPageTimeoutSeconds/
// OCRTotalTimeoutSeconds for how ctx combines with this package's own
// timeout bounds.
func ParseWithOCR(ctx context.Context, r io.ReadSeeker, opts Options, engine ocr.Engine) (Results, *ingestion.Error) {
	if opts.OCR == OCRDisabled {
		return Parse(r, opts)
	}
	if engine == nil {
		return Results{}, &ingestion.Error{
			Code:    ingestion.ErrCodeOCREngineUnavailable,
			Message: "OCR mode requested but no ingestion/ocr.Engine was supplied",
		}
	}

	resolved := ingestion.ResolveOptions(opts.Options)
	opts.Options = resolved
	limits := resolved.Limits

	data, err := readAllLimited(r, limits.MaxFileSizeBytes)
	if err != nil {
		return Results{}, &ingestion.Error{Code: ingestion.ErrCodeInvalidFile, Message: "failed to read input", Detail: err.Error()}
	}
	if int64(len(data)) > limits.MaxFileSizeBytes {
		return Results{}, &ingestion.Error{Code: ingestion.ErrCodeLimitExceeded, Message: "input exceeds maximum file size", Detail: fmt.Sprintf("limit is %d bytes", limits.MaxFileSizeBytes)}
	}
	if len(data) == 0 {
		return Results{}, &ingestion.Error{Code: ingestion.ErrCodeNoTabularData, Message: "input is empty"}
	}

	pdfReader := bytes.NewReader(data)

	pageCount, pcErr := pdfimage.PageCount(pdfReader)
	if pcErr != nil {
		return Results{}, &ingestion.Error{Code: ingestion.ErrCodeInvalidFile, Message: "failed to read PDF page count", Detail: pcErr.Error()}
	}
	if pageCount > limits.MaxPages {
		return Results{}, &ingestion.Error{
			Code:    ingestion.ErrCodePDFPageLimitExceeded,
			Message: "PDF document exceeds maximum page count",
			Detail:  fmt.Sprintf("limit is %d pages, document has %d", limits.MaxPages, pageCount),
		}
	}

	pdfReader.Seek(0, io.SeekStart)
	pageDims, pdErr := pdfimage.PageDims(pdfReader)
	if pdErr != nil {
		return Results{}, &ingestion.Error{Code: ingestion.ErrCodeInvalidFile, Message: "failed to read PDF page dimensions", Detail: pdErr.Error()}
	}

	pdfReader.Seek(0, io.SeekStart)
	extracted, extractErr := extractDocument(data, limits)
	if extractErr != nil {
		return Results{}, extractErr
	}

	textUsable, _ := hasUsableTextLayer(extracted)

	ocrPageSet := decideOCRPages(opts.OCR, textUsable, extracted, pageCount)
	if len(ocrPageSet) > limits.MaxOCRPages {
		return Results{}, &ingestion.Error{
			Code:    ingestion.ErrCodeOCRPageLimitExceeded,
			Message: "document requires OCR on more pages than the configured limit allows",
			Detail:  fmt.Sprintf("limit is %d pages, %d pages require OCR", limits.MaxOCRPages, len(ocrPageSet)),
		}
	}

	rowYTolerance := opts.resolveRowYTolerance()

	var allLines []line
	var docWarnings []ingestion.Warning
	var ocrPagesUsed, embeddedTextPagesUsed []int
	var ocrMeta ocrAggregateMeta
	var unsupportedScanPages int
	var totalOCRWords, totalOCRTextBytes int

	totalDeadline := time.Time{}
	if limits.OCRTotalTimeoutSeconds > 0 {
		totalDeadline = time.Now().Add(time.Duration(limits.OCRTotalTimeoutSeconds) * time.Second)
	}

	engineOpts := ocr.Options{}
	if limits.OCRPageTimeoutSeconds > 0 {
		engineOpts.Timeout = ocr.Timeout(limits.OCRPageTimeoutSeconds)
	}

	for pageNum := 1; pageNum <= pageCount; pageNum++ {
		pageIdx := pageNum - 1
		if !ocrPageSet[pageIdx] {
			pageFragments := filterPageRange(extracted.fragments, pageNum, pageNum)
			words := groupWords(pageFragments, rowYTolerance)
			allLines = append(allLines, groupRows(words, rowYTolerance)...)
			embeddedTextPagesUsed = append(embeddedTextPagesUsed, pageIdx)
			continue
		}

		if !totalDeadline.IsZero() && time.Now().After(totalDeadline) {
			return Results{}, &ingestion.Error{
				Code:    ingestion.ErrCodeOCRTimeout,
				Message: "OCR did not complete within the total document timeout",
				Detail:  fmt.Sprintf("%d of %d OCR pages remaining", len(ocrPageSet)-len(ocrPagesUsed)-unsupportedScanPages, len(ocrPageSet)),
			}
		}

		var dims pdfimage.PageDimensions
		if pageIdx < len(pageDims) {
			dims = pageDims[pageIdx]
		}

		pageResult, pageErr := ocrPage(ctx, pdfReader, pageNum, dims, engine, engineOpts, ocrPageBudget{
			maxImagePixels:    limits.MaxImagePixels,
			maxImageDimension: limits.MaxImageDimension,
			perPageTimeout:    time.Duration(limits.OCRPageTimeoutSeconds) * time.Second,
		}, opts.Preprocess)
		if pageErr != nil {
			if opts.OCR == OCRForce || isFatalOCRPageError(pageErr.Code) {
				// OCRForce: any page failure fails the whole parse (the
				// caller explicitly asked for every page to be OCR'd).
				// OCRAuto: a CONTENT-shaped per-page issue (ambiguous/
				// missing dominant image — see isFatalOCRPageError) is
				// non-fatal and skips just that page, but a RESOURCE-LIMIT
				// violation (image too large, OCR timeout, engine
				// unavailable) is always fatal regardless of mode — a
				// caller who configured a limit needs to know it was hit,
				// not have the violation silently swallowed as if the page
				// were merely unreadable content.
				return Results{}, pageErr
			}
			// OCRAuto, content-shaped issue: a single unreadable scanned
			// page does not fail the whole document — it contributes no
			// rows, and the document-level warning/metadata records that
			// it was skipped (see Metadata.OCR.UnsupportedScanPageCount).
			unsupportedScanPages++
			code := ingestion.WarnNoDominantPageImage
			if pageErr.Code == ingestion.ErrCodeScannedPageImageUnavailable && pageErr.Detail == string(pdfimage.ReasonMultiplePlausible) {
				code = ingestion.WarnMultiplePageImages
			}
			docWarnings = append(docWarnings, ingestion.Warning{
				Code: code, Message: pageErr.Message, RowIndex: -1, PageIndex: pageIdx,
			})
			continue
		}

		totalOCRWords += len(pageResult.words)
		if limits.MaxOCRWords > 0 && totalOCRWords > limits.MaxOCRWords {
			return Results{}, &ingestion.Error{
				Code:    ingestion.ErrCodeLimitExceeded,
				Message: "OCR-recognized word count exceeds the configured maximum",
				Detail:  fmt.Sprintf("limit is %d words", limits.MaxOCRWords),
			}
		}
		for _, w := range pageResult.words {
			totalOCRTextBytes += len(w.text)
		}
		if limits.MaxOCRTextBytes > 0 && totalOCRTextBytes > limits.MaxOCRTextBytes {
			return Results{}, &ingestion.Error{
				Code:    ingestion.ErrCodeLimitExceeded,
				Message: "OCR-recognized text size exceeds the configured maximum",
				Detail:  fmt.Sprintf("limit is %d bytes", limits.MaxOCRTextBytes),
			}
		}

		allLines = append(allLines, groupRows(pageResult.words, rowYTolerance)...)
		docWarnings = append(docWarnings, pageResult.warnings...)
		ocrPagesUsed = append(ocrPagesUsed, pageIdx)
		ocrMeta.accumulate(pageResult)
	}

	if len(ocrPagesUsed) > 0 && len(embeddedTextPagesUsed) > 0 {
		docWarnings = append(docWarnings, ingestion.Warning{
			Code: ingestion.WarnMixedTextAndOCRPages, RowIndex: -1,
			Message: "this document mixes pages read via embedded text and pages read via OCR",
		})
	}

	if len(allLines) == 0 {
		return Results{}, &ingestion.Error{
			Code:    ingestion.ErrCodeNoTabularData,
			Message: "no extractable content found (via embedded text or OCR) in this document",
		}
	}

	sortLinesDocumentOrder(allLines)

	sections, boundaryWarnings := splitSections(allLines, rowYTolerance)
	docWarnings = append(docWarnings, boundaryWarnings...)

	results := make([]ingestion.Result, 0, len(sections))
	for i, sec := range sections {
		pageExtents := computePageExtents(sec.lines)
		filteredLines, repeatedHits := suppressRepeatedContent(sec.lines, pageExtents)
		if len(filteredLines) == 0 {
			continue
		}
		sec.lines = filteredLines

		sg := buildSectionGrid(sec)

		dash := tabular.DashAsBlank
		if opts.Options.DashTreatment == ingestion.DashAsZero {
			dash = tabular.DashAsZero
		}
		ocrCorrected, ocrAmbiguous := applyOCRNumericCorrections(&sg, dash, ocrMeta.pageWasOCR)

		result, warnings := buildSectionResult(i, sec, sg, opts.Options)
		result.Metadata.Dependency = Dependency

		for _, hit := range repeatedHits {
			warnings = append(warnings, ingestion.Warning{
				Code: ingestion.WarnRepeatedHeaderRemoved, RowIndex: -1, PageIndex: hit.pageIndex,
				Message: "repeated header/column-heading line removed to avoid a duplicate row",
			})
		}
		if ambiguous, ambigWarning := checkColumnAlignmentAmbiguity(sg); ambiguous {
			warnings = append(warnings, ambigWarning)
		}
		if ambiguous, ambigWarning := checkRowLayoutAmbiguity(sec); ambiguous {
			warnings = append(warnings, ambigWarning)
		}

		warnings = reclassifyOCRAmbiguousWarnings(warnings, ocrAmbiguous)
		warnings = append(warnings, ocrNumericWarnings(result, ocrCorrected)...)
		warnings = append(warnings, annotateOCRCells(&result, ocrMeta, ocrCorrected)...)

		if ocrMeta.hasAny() {
			result.Metadata.OCR = ocrMeta.toMetadata(ocrPagesUsed, embeddedTextPagesUsed, unsupportedScanPages)
		}

		sortWarningsDeterministically(warnings)
		result.Warnings = warnings
		results = append(results, result)
	}

	if len(results) == 0 {
		return Results{}, &ingestion.Error{Code: ingestion.ErrCodeNoTabularData, Message: "no statement sections with data rows were found"}
	}

	return Results{Statements: results, Warnings: docWarnings, PageCount: pageCount}, nil
}

// isFatalOCRPageError reports whether a per-page OCR error is a RESOURCE
// or ENGINE failure (always fatal, in every OCR mode) rather than a
// CONTENT-shaped issue specific to that one page's scanned image (only
// fatal under OCRForce — see the call site in the page loop above).
// SCANNED_PAGE_IMAGE_UNAVAILABLE and PDF_PAGE_RENDER_REQUIRED are the only
// two codes ocrPage returns that describe "this particular page's content
// could not be resolved to a usable image," which OCRAuto tolerates as a
// per-page skip; every other code ocrPage can return (image too large,
// too many pixels, engine unavailable/failed, timeout) describes a
// resource/engine condition that applies regardless of which specific
// page triggered it, so silently continuing past it would hide a
// configuration or environment problem the caller needs to know about.
func isFatalOCRPageError(code ingestion.ErrorCode) bool {
	switch code {
	case ingestion.ErrCodeScannedPageImageUnavailable, ingestion.ErrCodePDFPageRenderRequired:
		return false
	default:
		return true
	}
}

// decideOCRPages returns the set of 0-based page indices that need OCR,
// per opts.OCR's semantics:
//   - OCRForce: every page.
//   - OCRAuto: no pages if the WHOLE document already has a usable text
//     layer (textUsable, from the exact same hasUsableTextLayer check
//     Parse uses); otherwise, exactly the pages that individually
//     produced zero extractable text fragments (pagesWithNoText) — the
//     same per-page signal Parse already computes for
//     WarnPDFTextLayerMissing, reused here to decide OCR eligibility
//     rather than just warning about it.
func decideOCRPages(mode OCRMode, textUsable bool, extracted extractResult, pageCount int) map[int]bool {
	set := make(map[int]bool)
	if mode == OCRForce {
		for i := 0; i < pageCount; i++ {
			set[i] = true
		}
		return set
	}
	// OCRAuto.
	if !textUsable {
		// The whole document looks image-only/scanned (same threshold
		// Parse uses for ErrCodeOCRRequired) — every page needs OCR.
		for i := 0; i < pageCount; i++ {
			set[i] = true
		}
		return set
	}
	// Document as a whole is usable, but individual pages may still have
	// zero extractable text (e.g. a scanned exhibit page mixed into an
	// otherwise text-based statement) — those specific pages need OCR.
	for _, p := range pagesWithNoText(extracted) {
		set[p] = true
	}
	return set
}

// sortLinesDocumentOrder sorts a combined (embedded-text + OCR) lines
// slice into (page ascending, Y descending) order — the same ordering
// groupRows already produces within a single page's own fragments/words,
// generalized here across a mix of pages that were populated by two
// different extraction paths in the page loop above (which does not
// itself guarantee page-then-Y ordering when pages are interleaved between
// the two branches, even though PAGE NUMBER order is already respected by
// the for-loop itself — this sort makes the Y-within-page ordering
// guarantee explicit and independent of that loop's structure).
func sortLinesDocumentOrder(lines []line) {
	sort.SliceStable(lines, func(i, j int) bool {
		if lines[i].pageIndex != lines[j].pageIndex {
			return lines[i].pageIndex < lines[j].pageIndex
		}
		return lines[i].y > lines[j].y
	})
}
