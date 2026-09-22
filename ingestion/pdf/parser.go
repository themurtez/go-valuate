// Package pdf parses text-based (born-digital) PDF financial statement
// exports into one or more ingestion.Result values, sitting in the exact
// same position in the pipeline as ingestion/csv and ingestion/xlsx:
//
//	PDF bytes/io.Reader
//	        -> pdf.Parse (this package)
//	        -> []ingestion.Result (one per detected statement — see below)
//	        -> Result.ToRawLineItems() -> []financial.RawLineItem
//	        -> financial/classification.ClassifyBatch
//	        -> []financial.MappedLineItem
//	        -> financial.Normalize
//	        -> financial.FinancialDataset
//
// Parse itself is NOT OCR. A PDF with no usable embedded text layer (an
// image-only/scanned statement) returns an explicit ErrCodeOCRRequired
// error rather than silently producing an empty or fabricated result —
// see hasUsableTextLayer in detect.go for the exact trigger thresholds.
// Parse never attempts OCR, never shells out to an external command-line
// utility, and never executes anything embedded in the PDF (JavaScript,
// launch actions, embedded files, forms) — see the Dependency constant in
// extract.go and the repository README's PDF section for what
// github.com/ledongthuc/pdf does and does not implement.
//
// A separate, entirely opt-in entry point, ParseWithOCR (see
// ocr_parse.go), extends this same pipeline with optional OCR fallback
// for scanned pages, via a caller-supplied ingestion/ocr.Engine (e.g.
// ingestion/ocr/tesseract, a local Tesseract adapter). Parse's own
// behavior — including ErrCodeOCRRequired — is completely unchanged by
// this: ParseWithOCR called with its default Options.OCR (OCRDisabled)
// is defined to behave identically to Parse. See the repository README's
// "Scanned/image PDF support (OCR)" section for the full design.
//
// # Multiple statements per PDF
//
// A single PDF document commonly contains more than one financial
// statement (e.g. an income statement followed by a balance sheet in one
// filing). Parse never silently combines them into one result: it detects
// statement section boundaries (see detect.go's splitSections — page
// titles, statement-type keywords, and large vertical gaps, all
// deterministic signals, never a guess) and returns one ingestion.Result
// PER detected section, each independently statement-typed, so a caller
// never gets income-statement rows and balance-sheet rows merged into one
// FinancialDataset by accident. A single-statement PDF is simply the same
// return shape with length 1 — a caller never needs a second call or an
// option to get every statement out.
//
// # Positioned-text extraction and layout reconstruction
//
// PDF content streams have no inherent row/column/table structure — see
// layout.go and columns.go's doc comments for the full row/word/column
// reconstruction algorithm this package builds from raw positioned text.
// Once a section's lines are reshaped into a tabular.Grid (the identical
// [][]string shape ingestion/csv and ingestion/xlsx already produce), this
// package hands off to the shared ingestion.BuildResult entry point for
// EVERY bit of header/period/statement-type/structural-row detection —
// see build.go. Nothing in this package reimplements any part of that
// logic; the only genuinely PDF-specific work is getting from positioned
// glyphs to a Grid in the first place, and a small set of documented PDF
// extraction-quirk accommodations (numeric.go, repeated.go).
package pdf

import (
	"fmt"
	"io"
	"sort"

	"github.com/themurtez/go-valuate/ingestion"
)

// Options controls PDF parsing behavior. It embeds ingestion.Options for
// every field shared with the csv/xlsx adapters (Locale, DashTreatment,
// Limits, PeriodColumnOverrides, LabelColumnOverride, HeaderRowOverride,
// StatementTypeOverride, etc.) rather than duplicating them, matching how
// csv.Parse/xlsx.Parse already share ingestion.Options directly — the only
// difference here is Options additionally needs PDF-only layout/page
// fields no tabular format has a use for.
//
// SheetName and Delimiter (ingestion.Options fields with no PDF meaning —
// a PDF has neither worksheets nor a field delimiter) are simply ignored
// by this package if set, exactly as XLSX-only/CSV-only fields are already
// ignored by the other format's parser.
type Options struct {
	ingestion.Options

	// PageStart, when non-zero, restricts extraction to pages at or after
	// this 1-based page number (inclusive). 0 (the zero value) means "from
	// the first page." Combined with PageEnd, lets a caller narrow a very
	// large PDF to a known relevant range without raising Limits.MaxPages
	// for the whole document.
	PageStart int
	// PageEnd, when non-zero, restricts extraction to pages at or before
	// this 1-based page number (inclusive). 0 (the zero value) means
	// "through the last page."
	PageEnd int
	// RowYTolerance is the maximum Y-coordinate difference (in PDF
	// user-space points) between two text fragments/words for them to be
	// considered part of the same visual row. 0 (the zero value) means
	// defaultRowYTolerance (2.0 points) — see layout.go's doc comment for
	// why that default was chosen. Increase this for a statement with
	// unusually loose baseline alignment; decrease it if two genuinely
	// distinct tightly-leaded rows are incorrectly merging.
	RowYTolerance float64

	// OCR selects OCR fallback behavior for a scanned/image-only PDF or
	// individual scanned pages within an otherwise text-based PDF. The
	// zero value (OCRDisabled) means no OCR is ever attempted — IDENTICAL
	// behavior to before this field existed: Parse never reads this
	// field at all (it always behaves as OCRDisabled); only
	// ParseWithOCR (ocr_parse.go) honors OCRAuto/OCRForce, and only when
	// called with a non-nil ingestion/ocr.Engine. See OCRMode and the
	// package doc comment's OCR modes section.
	OCR OCRMode
	// Preprocess controls deterministic image preprocessing applied to a
	// scanned page's image before OCR (see preprocess.go). Ignored when
	// OCR is OCRDisabled. Zero value means no preprocessing (OCR runs
	// against the extracted page image unmodified).
	Preprocess PreprocessOptions
}

// resolveRowYTolerance returns o.RowYTolerance, or defaultRowYTolerance if
// it is at its zero value.
func (o Options) resolveRowYTolerance() float64 {
	if o.RowYTolerance <= 0 {
		return defaultRowYTolerance
	}
	return o.RowYTolerance
}

// Results is the multi-statement output of Parse: one ingestion.Result per
// detected statement section (see the package doc comment's "Multiple
// statements per PDF" section), plus any document-level warnings that
// don't belong to a single section (e.g. a page that produced no text at
// all — see hasUsableTextLayer/WarnPDFTextLayerMissing).
type Results struct {
	// Statements is every detected statement section's result, in document
	// order (by first page/line encountered). Always length >= 1 on
	// success — see Parse's doc comment for when Parse instead returns a
	// nil Results and a non-nil *ingestion.Error.
	Statements []ingestion.Result `json:"statements"`
	// Warnings carries document-level warnings not scoped to a single
	// statement section (e.g. WarnPDFTextLayerMissing for an image-only
	// page found alongside otherwise-usable text). Per-section warnings
	// live on each Statements[i].Warnings, exactly like csv/xlsx.
	Warnings []ingestion.Warning `json:"warnings,omitempty"`
	// PageCount is the total number of pages in the source PDF document
	// (before any PageStart/PageEnd restriction), for caller-side
	// reporting/UI.
	PageCount int `json:"page_count"`
}

// Parse reads r as a PDF document and produces Results. On fatal failure
// (including ErrCodeOCRRequired — see the package doc comment) it returns
// a zero Results and a non-nil *ingestion.Error; on success it returns a
// populated Results (always at least one Statements entry) and a nil
// error, with any non-fatal issues recorded in Results.Warnings and/or
// each statement's own Result.Warnings.
//
// Unlike csv.Parse/xlsx.Parse, which accept only an io.Reader, PDF is not
// a streaming format — every general-purpose Go PDF library, including
// github.com/ledongthuc/pdf, requires random access (io.ReaderAt) to parse
// the cross-reference table and object graph. Parse still accepts a plain
// io.Reader as its public interface (never requiring a filesystem path) by
// buffering the input into memory itself, up to Options.Limits.MaxFileSizeBytes,
// exactly mirroring how csv.Parse/xlsx.Parse already read their entire
// input into memory before parsing.
func Parse(r io.Reader, opts Options) (Results, *ingestion.Error) {
	resolved := ingestion.ResolveOptions(opts.Options)
	opts.Options = resolved
	limits := resolved.Limits

	data, err := readAllLimited(r, limits.MaxFileSizeBytes)
	if err != nil {
		return Results{}, &ingestion.Error{
			Code:    ingestion.ErrCodeInvalidFile,
			Message: "failed to read input",
			Detail:  err.Error(),
		}
	}
	if int64(len(data)) > limits.MaxFileSizeBytes {
		return Results{}, &ingestion.Error{
			Code:    ingestion.ErrCodeLimitExceeded,
			Message: "input exceeds maximum file size",
			Detail:  fmt.Sprintf("limit is %d bytes", limits.MaxFileSizeBytes),
		}
	}
	if len(data) == 0 {
		return Results{}, &ingestion.Error{
			Code:    ingestion.ErrCodeNoTabularData,
			Message: "input is empty",
		}
	}

	extracted, ierr := extractDocument(data, limits)
	if ierr != nil {
		return Results{}, ierr
	}

	if ok, reason := hasUsableTextLayer(extracted); !ok {
		return Results{}, &ingestion.Error{
			Code:    ingestion.ErrCodeOCRRequired,
			Message: "PDF has no usable embedded text layer; OCR would be required to extract this document's content, which this package does not perform",
			Detail:  reason,
		}
	}

	fragments := filterPageRange(extracted.fragments, opts.PageStart, opts.PageEnd)
	if len(fragments) == 0 {
		return Results{}, &ingestion.Error{
			Code:    ingestion.ErrCodeNoTabularData,
			Message: "no extractable text found in the requested page range",
		}
	}

	rowYTolerance := opts.resolveRowYTolerance()
	words := groupWords(fragments, rowYTolerance)
	allLines := groupRows(words, rowYTolerance)

	sections, boundaryWarnings := splitSections(allLines, rowYTolerance)

	var docWarnings []ingestion.Warning
	docWarnings = append(docWarnings, boundaryWarnings...)
	for _, pageIdx := range pagesWithNoText(extracted) {
		docWarnings = append(docWarnings, ingestion.Warning{
			Code:      ingestion.WarnPDFTextLayerMissing,
			Message:   "this page produced no extractable text (possibly an image/exhibit page within an otherwise text-based document)",
			RowIndex:  -1,
			PageIndex: pageIdx,
		})
	}

	results := make([]ingestion.Result, 0, len(sections))
	for i, sec := range sections {
		pageExtents := computePageExtents(sec.lines)
		filteredLines, repeatedHits := suppressRepeatedContent(sec.lines, pageExtents)
		if len(filteredLines) == 0 {
			continue
		}
		sec.lines = filteredLines

		sg := buildSectionGrid(sec)
		result, warnings := buildSectionResult(i, sec, sg, opts.Options)
		result.Metadata.Dependency = Dependency

		for _, hit := range repeatedHits {
			warnings = append(warnings, ingestion.Warning{
				Code:      ingestion.WarnRepeatedHeaderRemoved,
				Message:   "repeated header/column-heading line removed to avoid a duplicate row",
				RowIndex:  -1,
				PageIndex: hit.pageIndex,
			})
		}
		if ambiguous, ambigWarning := checkColumnAlignmentAmbiguity(sg); ambiguous {
			warnings = append(warnings, ambigWarning)
		}
		if ambiguous, ambigWarning := checkRowLayoutAmbiguity(sec); ambiguous {
			warnings = append(warnings, ambigWarning)
		}

		sortWarningsDeterministically(warnings)
		result.Warnings = warnings
		results = append(results, result)
	}

	if len(results) == 0 {
		return Results{}, &ingestion.Error{
			Code:    ingestion.ErrCodeNoTabularData,
			Message: "no statement sections with data rows were found",
		}
	}

	return Results{
		Statements: results,
		Warnings:   docWarnings,
		PageCount:  extracted.pageCount,
	}, nil
}

// filterPageRange returns only fragments on pages within [start, end]
// (1-based, inclusive; 0 means unbounded on that side), converting to the
// 0-based pageIndex fragments actually carry.
func filterPageRange(fragments []fragment, start, end int) []fragment {
	if start <= 0 && end <= 0 {
		return fragments
	}
	out := make([]fragment, 0, len(fragments))
	for _, f := range fragments {
		page1based := f.pageIndex + 1
		if start > 0 && page1based < start {
			continue
		}
		if end > 0 && page1based > end {
			continue
		}
		out = append(out, f)
	}
	return out
}

// checkColumnAlignmentAmbiguity reports whether sec's reconstructed grid
// looks structurally uncertain enough to warrant
// WarnColumnAlignmentAmbiguous: more than half its non-label cells ended up
// empty despite the section having multiple detected columns, which
// usually means column-boundary detection (columns.go) could not
// confidently align this section's rows into consistent columns.
func checkColumnAlignmentAmbiguity(sg sectionGrid) (bool, ingestion.Warning) {
	if len(sg.grid) == 0 || len(sg.grid[0]) < 2 {
		return false, ingestion.Warning{}
	}
	total := 0
	empty := 0
	for _, row := range sg.grid {
		for c := 1; c < len(row); c++ {
			total++
			if row[c] == "" {
				empty++
			}
		}
	}
	if total == 0 {
		return false, ingestion.Warning{}
	}
	if float64(empty)/float64(total) > 0.6 {
		return true, ingestion.Warning{
			Code:     ingestion.WarnColumnAlignmentAmbiguous,
			Message:  "column reconstruction produced an unusually sparse grid for this section; columns may not have been detected confidently",
			RowIndex: -1,
		}
	}
	return false, ingestion.Warning{}
}

// checkRowLayoutAmbiguity reports whether sec's lines look like they came
// from a page with no real row structure at all, warranting
// WarnPDFLayoutAmbiguous: most lines in the section contain only a single
// word, which is what happens when text is scattered across a page with
// no consistent baseline alignment (row grouping — layout.go — ends up
// creating nearly one "row" per word, since nothing shares a close-enough
// Y to merge). A genuine financial statement line, even a short one,
// almost always has at least a label plus one value on the same row; a
// section dominated by one-word lines is a strong signal this package
// could not reconstruct real rows from this page's positioning.
func checkRowLayoutAmbiguity(sec section) (bool, ingestion.Warning) {
	if len(sec.lines) < 3 {
		return false, ingestion.Warning{}
	}
	singleWord := 0
	for _, ln := range sec.lines {
		if len(ln.words) <= 1 {
			singleWord++
		}
	}
	if float64(singleWord)/float64(len(sec.lines)) > 0.7 {
		return true, ingestion.Warning{
			Code:     ingestion.WarnPDFLayoutAmbiguous,
			Message:  "most lines in this section reconstructed as a single word, suggesting row grouping could not confidently determine this page's structure",
			RowIndex: -1,
		}
	}
	return false, ingestion.Warning{}
}

// sortWarningsDeterministically sorts warnings by (Code, RowIndex,
// ColumnIndex, PageIndex) so Parse's output ordering never depends on map
// iteration order (reclassifyPDFNumericWarnings and the docWarnings/
// repeatedHits append sequences both build up warnings via loops over maps
// or independent passes) — matching this repository's general
// deterministic-ordering discipline.
func sortWarningsDeterministically(warnings []ingestion.Warning) {
	sort.SliceStable(warnings, func(i, j int) bool {
		a, b := warnings[i], warnings[j]
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		if a.RowIndex != b.RowIndex {
			return a.RowIndex < b.RowIndex
		}
		if a.ColumnIndex != b.ColumnIndex {
			return a.ColumnIndex < b.ColumnIndex
		}
		return a.PageIndex < b.PageIndex
	})
}
