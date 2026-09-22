// Package ingestion converts tabular financial statement exports (CSV,
// XLSX) into financial.RawLineItem values, deterministically.
//
// This package sits ahead of financial/classification in the pipeline:
//
//	bytes / io.Reader
//	        -> ingestion (this package + ingestion/csv, ingestion/xlsx)
//	        -> []financial.RawLineItem
//	        -> financial/classification.ClassifyBatch
//	        -> []financial.MappedLineItem
//	        -> financial.Normalize
//	        -> financial.FinancialDataset
//
// It performs only structural interpretation of a tabular document: which
// row is a header, which columns are reporting periods, which rows are
// blank/heading/subtotal/total, what a cell's numeric value is, and (best
// effort) which section a row belongs to. It never decides which canonical
// financial.Code a row's label maps to — that is exactly the job
// financial/classification already does, and duplicating any part of it
// here would create two places that could disagree about the same
// question. See Result and Row below for the exact boundary.
//
// Every function in this package is a pure, deterministic function of its
// input bytes/reader and Options: the same input always produces the same
// Result. Nothing here performs AI/LLM inference, network I/O, database
// access, or file-system access beyond reading the caller-supplied
// io.Reader. See the repository README's "What this project intentionally
// does not contain" section — this package does not add any of those
// either.
package ingestion

import "github.com/themurtez/go-valuate/financial"

// Format identifies which tabular parser produced a Result.
type Format string

const (
	FormatCSV  Format = "csv"
	FormatXLSX Format = "xlsx"
	FormatPDF  Format = "pdf"
)

// DashTreatment controls how a bare dash/em-dash cell ("-", "—", "--") is
// interpreted during numeric parsing. Financial exports use a dash
// inconsistently — sometimes meaning exactly zero, sometimes meaning "no
// data reported" — so this is caller-configurable rather than a fixed
// assumption. See Options.DashTreatment and the numeric parsing rules in
// the tabular package.
type DashTreatment string

const (
	// DashAsZero treats a bare dash cell as the value 0.
	DashAsZero DashTreatment = "zero"
	// DashAsBlank treats a bare dash cell as no value (the cell is omitted
	// from the row's Values map entirely, same as an empty cell). This is
	// the default when DashTreatment is unset.
	DashAsBlank DashTreatment = "blank"
)

// StatementTypeOverride lets a caller force the detected statement type for
// an entire parse, bypassing heuristic detection in section 8 of the
// ingestion contract. Distinct from financial.StatementType so an
// explicit "force UNKNOWN" (StatementOverrideUnknown) can be represented
// separately from "no override supplied" (the zero value).
type StatementTypeOverride string

const (
	// StatementOverrideNone means no override was supplied; the parser
	// detects the statement type itself. This is the zero value.
	StatementOverrideNone            StatementTypeOverride = ""
	StatementOverrideIncomeStatement StatementTypeOverride = "income_statement"
	StatementOverrideBalanceSheet    StatementTypeOverride = "balance_sheet"
	StatementOverrideCashFlow        StatementTypeOverride = "cash_flow"
	// StatementOverrideUnknown forces the result to be treated as
	// undetected, skipping heuristic detection entirely.
	StatementOverrideUnknown StatementTypeOverride = "unknown"
)

// PeriodColumnOverride lets a caller pin a specific column to a specific
// canonical period, bypassing header/period detection for that column.
// ColumnIndex is 0-based within the sheet/row as read (matching Cell's
// ColumnIndex).
type PeriodColumnOverride struct {
	ColumnIndex int              `json:"column_index"`
	Period      financial.Period `json:"period"`
	PeriodType  PeriodType       `json:"period_type,omitempty"`
	Label       string           `json:"label,omitempty"`
}

// Limits bounds resource consumption while parsing untrusted input. Every
// field has a safe, conservative default (see DefaultLimits) applied by
// each parser when the caller leaves a field at its zero value.
type Limits struct {
	// MaxFileSizeBytes caps the size of the input bytes/stream. 0 means
	// DefaultLimits.MaxFileSizeBytes.
	MaxFileSizeBytes int64
	// MaxSheets caps the number of worksheets an XLSX parser will
	// enumerate. Ignored by the CSV parser. 0 means DefaultLimits.MaxSheets.
	MaxSheets int
	// MaxRows caps the number of rows read from a single sheet/document. 0
	// means DefaultLimits.MaxRows.
	MaxRows int
	// MaxColumns caps the number of columns read from a single row. 0 means
	// DefaultLimits.MaxColumns.
	MaxColumns int
	// MaxCellTextLength caps the number of runes retained from a single
	// cell's text. Longer values are truncated and a warning is emitted
	// (see WarnCellTextTruncated). 0 means DefaultLimits.MaxCellTextLength.
	MaxCellTextLength int
	// MaxPages caps the number of pages ingestion/pdf will read from a PDF
	// document. Ignored by CSV/XLSX. 0 means DefaultLimits.MaxPages. A
	// document with more pages than this is a fatal ErrCodeLimitExceeded
	// (PDF_PAGE_LIMIT_EXCEEDED), not silently truncated, since a partial
	// read of a multi-page statement could silently drop financial rows.
	MaxPages int
	// MaxTextFragments caps the number of positioned-text fragments (see
	// ingestion/pdf's extraction doc comment — roughly one per glyph/run,
	// well below one per word) ingestion/pdf will extract across the whole
	// document before giving up, bounding parser work against a
	// pathologically dense or maliciously crafted PDF. Ignored by
	// CSV/XLSX. 0 means DefaultLimits.MaxTextFragments. Exceeding this is a
	// fatal ErrCodeLimitExceeded (PDF_TEXT_LIMIT_EXCEEDED).
	MaxTextFragments int
	// MaxTextLengthPerPage caps the number of runes ingestion/pdf will
	// retain from a single page's reconstructed text before giving up on
	// that page. Ignored by CSV/XLSX. 0 means
	// DefaultLimits.MaxTextLengthPerPage. Exceeding this is a fatal
	// ErrCodeLimitExceeded (PDF_TEXT_LIMIT_EXCEEDED), distinct from
	// MaxCellTextLength (which truncates a single reconstructed cell with a
	// warning, not a fatal error — a whole page containing far more text
	// than any real financial statement page plausibly has is instead
	// treated as a resource-exhaustion signal worth refusing outright).
	MaxTextLengthPerPage int

	// MaxOCRPages caps the number of pages ingestion/pdf will send to OCR
	// across a single document (OCRAuto/OCRForce modes only — see
	// pdf.Options.OCR). Distinct from MaxPages (which bounds the whole
	// document's page count regardless of OCR): a caller may want a much
	// lower OCR-specific ceiling, since OCR is far more expensive per page
	// than embedded-text extraction. 0 means DefaultLimits.MaxOCRPages.
	// Exceeding this is a fatal ErrCodeOCRPageLimitExceeded. PDF only.
	MaxOCRPages int
	// MaxImagePixels caps a single scanned page's dominant image's total
	// pixel count (width * height) before OCR is attempted on it — a
	// defensive bound against a decompression-bomb-style oversized raster
	// embedded in an untrusted PDF. 0 means DefaultLimits.MaxImagePixels.
	// Exceeding this is a fatal ErrCodeOCRImageLimitExceeded. PDF only.
	MaxImagePixels int64
	// MaxImageDimension caps a single scanned page's dominant image's
	// width or height individually (in pixels), independent of
	// MaxImagePixels (which bounds total area — a pathologically
	// wide-and-thin image could pass a pixel-count check while still being
	// unreasonable to process). 0 means DefaultLimits.MaxImageDimension.
	// Exceeding this is a fatal ErrCodeOCRImageLimitExceeded. PDF only.
	MaxImageDimension int
	// MaxOCRWords caps the number of OCR-recognized words ingestion/pdf
	// will retain across a single document before giving up, bounding
	// downstream row/column reconstruction work against a pathological
	// recognition result. 0 means DefaultLimits.MaxOCRWords. Exceeding
	// this is a fatal ErrCodeLimitExceeded. PDF only.
	MaxOCRWords int
	// MaxOCRTextBytes caps the total number of bytes of OCR-recognized
	// text ingestion/pdf will retain across a single document. 0 means
	// DefaultLimits.MaxOCRTextBytes. Exceeding this is a fatal
	// ErrCodeLimitExceeded. PDF only.
	MaxOCRTextBytes int
	// OCRPageTimeoutSeconds bounds how long OCR recognition may run for a
	// SINGLE page, in whole seconds. 0 means
	// DefaultLimits.OCRPageTimeoutSeconds. Exceeding this is a fatal
	// ErrCodeOCRTimeout for that page (which fails the whole parse in
	// OCRForce mode, or is recorded as a page-level failure in OCRAuto
	// mode for that page only — see the ingestion/pdf package doc
	// comment's OCR modes section). PDF only.
	OCRPageTimeoutSeconds int
	// OCRTotalTimeoutSeconds bounds how long OCR recognition may run
	// ACROSS THE WHOLE DOCUMENT (all pages combined), in whole seconds. 0
	// means DefaultLimits.OCRTotalTimeoutSeconds (no additional
	// whole-document bound beyond the sum of per-page timeouts — still
	// subject to any deadline already present on the caller's own
	// context.Context). PDF only.
	OCRTotalTimeoutSeconds int
}

// DefaultLimits returns the conservative default Limits applied whenever a
// caller-supplied Limits field is left at its zero value.
func DefaultLimits() Limits {
	return Limits{
		MaxFileSizeBytes:     50 * 1024 * 1024, // 50 MiB
		MaxSheets:            100,
		MaxRows:              100_000,
		MaxColumns:           500,
		MaxCellTextLength:    4096,
		MaxPages:             500,
		MaxTextFragments:     2_000_000,
		MaxTextLengthPerPage: 200_000,

		MaxOCRPages:            50,
		MaxImagePixels:         50_000_000, // e.g. ~7071x7071px; well above any realistic single-page scan
		MaxImageDimension:      10_000,
		MaxOCRWords:            200_000,
		MaxOCRTextBytes:        10 * 1024 * 1024, // 10 MiB
		OCRPageTimeoutSeconds:  60,
		OCRTotalTimeoutSeconds: 600,
	}
}

// withDefaults returns a copy of l with every zero-value field replaced by
// DefaultLimits's corresponding value.
func (l Limits) withDefaults() Limits {
	d := DefaultLimits()
	if l.MaxFileSizeBytes <= 0 {
		l.MaxFileSizeBytes = d.MaxFileSizeBytes
	}
	if l.MaxSheets <= 0 {
		l.MaxSheets = d.MaxSheets
	}
	if l.MaxRows <= 0 {
		l.MaxRows = d.MaxRows
	}
	if l.MaxColumns <= 0 {
		l.MaxColumns = d.MaxColumns
	}
	if l.MaxCellTextLength <= 0 {
		l.MaxCellTextLength = d.MaxCellTextLength
	}
	if l.MaxPages <= 0 {
		l.MaxPages = d.MaxPages
	}
	if l.MaxTextFragments <= 0 {
		l.MaxTextFragments = d.MaxTextFragments
	}
	if l.MaxTextLengthPerPage <= 0 {
		l.MaxTextLengthPerPage = d.MaxTextLengthPerPage
	}
	if l.MaxOCRPages <= 0 {
		l.MaxOCRPages = d.MaxOCRPages
	}
	if l.MaxImagePixels <= 0 {
		l.MaxImagePixels = d.MaxImagePixels
	}
	if l.MaxImageDimension <= 0 {
		l.MaxImageDimension = d.MaxImageDimension
	}
	if l.MaxOCRWords <= 0 {
		l.MaxOCRWords = d.MaxOCRWords
	}
	if l.MaxOCRTextBytes <= 0 {
		l.MaxOCRTextBytes = d.MaxOCRTextBytes
	}
	if l.OCRPageTimeoutSeconds <= 0 {
		l.OCRPageTimeoutSeconds = d.OCRPageTimeoutSeconds
	}
	if l.OCRTotalTimeoutSeconds <= 0 {
		l.OCRTotalTimeoutSeconds = d.OCRTotalTimeoutSeconds
	}
	return l
}

// Options controls parsing behavior for both the csv and xlsx parsers.
// Auto-detection is the default for every field left at its zero value —
// see each field's doc comment for its specific fallback.
type Options struct {
	// SheetName selects a specific worksheet by name. XLSX only, ignored by
	// CSV. Empty means auto-select (see the xlsx package's sheet-selection
	// rules).
	SheetName string
	// Delimiter overrides CSV field delimiter auto-detection. CSV only,
	// ignored by XLSX. Zero rune means auto-detect among comma, semicolon,
	// and tab.
	Delimiter rune
	// StatementTypeOverride forces the detected statement type. See
	// StatementTypeOverride.
	StatementTypeOverride StatementTypeOverride
	// LabelColumnOverride pins the 0-based column index containing line
	// item labels, bypassing auto-detection. nil (the zero value) means
	// auto-detect; a non-nil pointer (including one pointing at 0) is an
	// explicit override.
	LabelColumnOverride *int
	// HeaderRowOverride pins the 0-based row index containing period
	// headers, bypassing auto-detection. nil (the zero value) means
	// auto-detect; a non-nil pointer (including one pointing at 0) is an
	// explicit override.
	HeaderRowOverride *int
	// PeriodColumnOverrides pins specific columns to specific canonical
	// periods, bypassing header/period detection for just those columns.
	PeriodColumnOverrides []PeriodColumnOverride
	// Locale selects the numeric/period formatting convention. Only
	// LocaleEnUS is implemented; see Locale's doc comment.
	Locale Locale
	// DashTreatment controls how bare-dash cells are interpreted. Zero
	// value means DashAsBlank.
	DashTreatment DashTreatment
	// Limits bounds resource consumption. Zero-value fields fall back to
	// DefaultLimits.
	Limits Limits
}

// Locale selects the numeric and period formatting convention a parser
// applies. Only LocaleEnUS (comma thousands separator, period decimal
// point, month names in English) is implemented in this version; the field
// exists so a future locale can be added without changing Options's shape.
type Locale string

const (
	// LocaleEnUS is the default and only currently supported locale:
	// comma (",") thousands separators, period (".") decimal points,
	// "$" currency prefix, English month names.
	LocaleEnUS Locale = "en-US"
)

// defaultOptions returns Options with every field normalized to its
// documented auto-detect default, applied by each parser before use.
func defaultOptions(o Options) Options {
	if o.Locale == "" {
		o.Locale = LocaleEnUS
	}
	if o.DashTreatment == "" {
		o.DashTreatment = DashAsBlank
	}
	o.Limits = o.Limits.withDefaults()
	return o
}

// PeriodType classifies the granularity/kind of a detected reporting
// period, independent of financial.Period's plain-string representation.
type PeriodType string

const (
	PeriodTypeFiscalYear   PeriodType = "fiscal_year"
	PeriodTypeCalendarYear PeriodType = "calendar_year"
	PeriodTypeQuarter      PeriodType = "quarter"
	PeriodTypeMonth        PeriodType = "month"
	PeriodTypeYTD          PeriodType = "ytd"
	// PeriodTypeUnknown means a period column was detected but its label
	// could not be parsed into a specific granularity; the original label
	// is preserved verbatim (see DetectedPeriod.OriginalLabel) rather than
	// fabricating a date.
	PeriodTypeUnknown PeriodType = "unknown"
)

// DetectedPeriod describes one reporting-period column found in a tabular
// document, before it is used as a financial.Period key on any Row.
type DetectedPeriod struct {
	// ColumnIndex is the 0-based column this period was detected in.
	ColumnIndex int `json:"column_index"`
	// Period is the canonical period identifier assigned to this column,
	// suitable for use as a financial.Period. Always non-empty: when the
	// original label could not be parsed confidently, Period falls back to
	// a normalized form of OriginalLabel and PeriodType is
	// PeriodTypeUnknown (see WarnPeriodLabelAmbiguous).
	Period financial.Period `json:"period"`
	// PeriodType classifies what kind of period this is, or
	// PeriodTypeUnknown if it could not be determined confidently.
	PeriodType PeriodType `json:"period_type"`
	// OriginalLabel is the header text exactly as it appeared in the
	// source, always preserved regardless of whether parsing succeeded.
	OriginalLabel string `json:"original_label"`
	// StartDate is the period's start date in YYYY-MM-DD form, when it
	// could be determined. Empty otherwise.
	StartDate string `json:"start_date,omitempty"`
	// EndDate is the period's end date in YYYY-MM-DD form, when it could
	// be determined. Empty otherwise.
	EndDate string `json:"end_date,omitempty"`
	// Confidence is a deterministic heuristic strength in [0, 1] for this
	// period detection, using the same non-statistical convention as
	// classification.Confidence. 1.0 for an explicit caller override or an
	// unambiguous exact-format match (e.g. "FY2025"); lower for weaker
	// signals.
	Confidence float64 `json:"confidence"`
	// Evidence is a short human-readable explanation of how this period
	// was detected (e.g. "matched FY YYYY pattern" or "caller override").
	Evidence string `json:"evidence,omitempty"`
}

// StructuralKind classifies the structural role of a single row, as
// distinct from financial.RowStatus: StructuralKind is the ingestion
// layer's own best-effort read of row shape (including heading rows and
// blank rows, neither of which financial.RowStatus represents on its own),
// while Row.Status carries the financial.RowStatus value
// classification/normalization actually consume. See ToRawLineItems and
// structuralKindToRowKind, which translates StructuralKind into the
// analogous financial.RowKind carried on RawLineItem.Kind (everything
// except StructuralBlank, which never reaches RawLineItem at all).
type StructuralKind string

const (
	StructuralNormal   StructuralKind = "normal"
	StructuralHeading  StructuralKind = "heading"
	StructuralSubtotal StructuralKind = "subtotal"
	StructuralTotal    StructuralKind = "total"
	StructuralBlank    StructuralKind = "blank"
)

// Cell is a single source cell's value, preserved for traceability
// alongside its parsed interpretation.
type Cell struct {
	// ColumnIndex is the 0-based column this cell was read from.
	ColumnIndex int `json:"column_index"`
	// Raw is the cell's original text exactly as read from the source
	// (post-decoding, pre-numeric-parsing), truncated to
	// Limits.MaxCellTextLength if necessary. For PDF, this is the
	// reconstructed word/cell text after row/column grouping (see
	// ingestion/pdf), not the raw per-glyph extraction fragments.
	Raw string `json:"raw"`
	// Numeric is the parsed numeric value, when Raw was successfully
	// interpreted as a monetary amount. Nil when the cell was blank, was
	// not numeric (e.g. a label or heading cell), or failed to parse (see
	// Parsed).
	Numeric *float64 `json:"numeric,omitempty"`
	// Parsed is true when Numeric parsing was attempted and succeeded.
	// false with a non-empty Raw and no Numeric means parsing was
	// attempted and failed (see WarnUnparseableNumericCell) or the cell
	// was intentionally not treated as numeric (e.g. a label column).
	Parsed bool `json:"parsed"`
	// Formula is the cell's formula text, XLSX only, when the cell
	// contains a formula. Empty otherwise. See FORMULA_WITHOUT_CACHED_VALUE.
	Formula string `json:"formula,omitempty"`
	// Bounds is the cell's position on its source page, PDF only. Nil for
	// CSV/XLSX (which have no coordinate concept) and nil for any PDF cell
	// this package could not confidently attribute a single bounding box to
	// (e.g. one reconstructed by merging fragments spanning a wide X
	// range) — see ingestion/pdf's layout doc comment for when this is and
	// isn't populated.
	Bounds *CellBounds `json:"bounds,omitempty"`
	// OCR carries OCR-specific provenance/confidence for this cell, when
	// it was reconstructed from OCR-recognized words rather than an
	// embedded PDF text layer. Nil for CSV/XLSX and for any PDF cell read
	// via the embedded-text path (OCRDisabled, or a page that had a usable
	// text layer under OCRAuto). See OCRProvenance.
	OCR *OCRProvenance `json:"ocr,omitempty"`
}

// OCRProvenance traces one cell's value back to the OCR recognition that
// produced it, so a future review UI can show a user exactly what the OCR
// engine reported (Cell.Raw already carries the FINAL text used for
// parsing, which may differ from OriginalText — see OCRNumericCorrected)
// without needing database IDs or anything beyond what this package
// already computes in-memory.
type OCRProvenance struct {
	// OriginalText is the OCR engine's own recognized text for this cell,
	// EXACTLY as reported, before any numeric-safety normalization (see
	// ingestion/pdf's ocr_numeric.go) — always preserved even when
	// Cell.Raw ends up different (OCRNumericCorrected) or the cell could
	// not be parsed at all (OCRNumericAmbiguous).
	OriginalText string `json:"original_text"`
	// Confidence is the engine-reported confidence for this cell's
	// dominant/lowest-confidence constituent word, on whatever scale the
	// engine uses (see ingestion/ocr's package doc comment: this is NEVER
	// a calibrated probability). -1 means the engine did not report a
	// confidence.
	Confidence float64 `json:"confidence"`
	// NumericCorrected is true when this cell's numeric value required a
	// deterministic heuristic correction before it would parse (see
	// WarnOCRNumericCorrected) — a signal the future review UI can use to
	// flag this specific value for a closer look, distinct from a
	// low-confidence value that parsed cleanly on the first attempt.
	NumericCorrected bool `json:"numeric_corrected,omitempty"`
	// ReviewRecommended is true when this cell's OCR provenance meets one
	// or more of this package's own review-worthiness signals (low
	// confidence, a numeric correction was applied, or the value could
	// not be parsed at all) — a single boolean summary so a consuming
	// application does not need to reimplement this package's own
	// confidence-threshold logic just to decide whether to highlight a
	// row for human review.
	ReviewRecommended bool `json:"review_recommended,omitempty"`
	// PixelBounds is the cell's approximate bounding box on the SOURCE
	// SCANNED IMAGE (not the PDF page — see CellBounds for the
	// PDF-points equivalent, which is still populated alongside this for
	// an OCR cell), in source-image pixels, top-left origin. Nil if not
	// available.
	PixelBounds *PixelBounds `json:"pixel_bounds,omitempty"`
}

// PixelBounds is a bounding box in source-image pixel coordinates
// (top-left origin, Y increasing downward — the raster-image convention,
// distinct from CellBounds's PDF bottom-up points convention).
type PixelBounds struct {
	X, Y, Width, Height int
}

// CellBounds is a cell's approximate bounding box on its source PDF page,
// in PDF user-space points (1/72 inch), with Y increasing upward from the
// page's bottom edge, matching the PDF coordinate convention ledongthuc/pdf
// itself exposes (see ingestion/pdf's extraction code) — this package
// performs no coordinate-system translation, so a consumer overlaying these
// coordinates on a rendered page image must account for that convention
// itself (most page-rendering libraries use top-down Y).
type CellBounds struct {
	// X0 is the left edge.
	X0 float64 `json:"x0"`
	// X1 is the right edge.
	X1 float64 `json:"x1"`
	// Y0 is the bottom edge.
	Y0 float64 `json:"y0"`
	// Y1 is the top edge.
	Y1 float64 `json:"y1"`
}

// Row is one row of a tabular document as structurally interpreted by this
// package, before classification. It carries everything section 3 of the
// ingestion contract requires: raw cells, the detected label, detected
// period values, structural role, and section/parent context.
type Row struct {
	// ID is a deterministic identifier unique within a single Result:
	// "sheet-<index>-row-<n>" (0-based sheet index, 0-based row index). See
	// ToRawLineItems, which copies this into financial.RawLineItem.ID.
	ID string `json:"id"`
	// SheetIndex is the 0-based index of the source worksheet/tab. Always 0
	// for CSV. For PDF, always 0 as well — a PDF has no worksheet concept;
	// see PageIndex for PDF's analogous provenance unit.
	SheetIndex int `json:"sheet_index"`
	// SheetName is the source worksheet/tab name, when applicable. Empty
	// for CSV and PDF.
	SheetName string `json:"sheet_name,omitempty"`
	// RowIndex is the 0-based row index within the sheet as read from the
	// source (not reindexed after skipping blank rows), so it always
	// matches the original document for traceability. For PDF, this is the
	// row's 0-based index within its detected statement section (which may
	// span multiple pages — see PageIndex for the specific source page).
	RowIndex int `json:"row_index"`
	// PageIndex is the 0-based source page this row was extracted from.
	// Populated only by ingestion/pdf; always 0 for CSV/XLSX (which have no
	// page concept), so this field is harmless additive metadata for those
	// formats — never omitted from JSON (unlike most PDF-only fields on
	// this struct) specifically so a zero PageIndex is indistinguishable
	// from "not a PDF row" only by context, matching SheetIndex's identical
	// always-present convention.
	PageIndex int `json:"page_index"`
	// Label is the detected line-item label for this row (the text of the
	// label column). Empty for a genuinely blank row.
	Label string `json:"label"`
	// ParentLabel is the best-effort detected enclosing section/heading
	// label, if any. See section 13 of the ingestion contract.
	ParentLabel string `json:"parent_label,omitempty"`
	// Kind classifies the row's structural role.
	Kind StructuralKind `json:"kind"`
	// Status is the financial.RowStatus this row maps to for downstream
	// normalization, derived deterministically from Kind (see
	// ToRawLineItems). Blank rows are dropped before reaching RawLineItem
	// entirely; heading rows are NOT dropped (see Result.Rows vs
	// ToRawLineItems, and financial.RowKind).
	Status financial.RowStatus `json:"status"`
	// IndentLevel is a best-effort structural indentation depth (0 = no
	// indentation detected), used as one signal for Kind/ParentLabel
	// detection. Not a guarantee of accounting hierarchy depth. For PDF,
	// this is derived from X-offset rather than leading whitespace — see
	// ingestion/pdf's layout doc comment.
	IndentLevel int `json:"indent_level,omitempty"`
	// Cells holds every cell in the row, including the label cell and any
	// cell not recognized as a detected period column, in original column
	// order.
	Cells []Cell `json:"cells"`
	// Values maps each detected period to the numeric amount found in that
	// period's column for this row. Only populated for periods that parsed
	// successfully; see Cells for the full raw record including failures.
	Values map[financial.Period]float64 `json:"values,omitempty"`
}

// WarningCode is a stable identifier for a non-fatal parsing issue. See the
// Warn* constants.
type WarningCode string

const (
	WarnPeriodLabelAmbiguous              WarningCode = "PERIOD_LABEL_AMBIGUOUS"
	WarnStatementTypeUnknown              WarningCode = "STATEMENT_TYPE_UNKNOWN"
	WarnMultiplePossibleHeaderRows        WarningCode = "MULTIPLE_POSSIBLE_HEADER_ROWS"
	WarnUnparseableNumericCell            WarningCode = "UNPARSEABLE_NUMERIC_CELL"
	WarnFormulaWithoutCachedValue         WarningCode = "FORMULA_WITHOUT_CACHED_VALUE"
	WarnMultiplePlausibleSheets           WarningCode = "MULTIPLE_PLAUSIBLE_SHEETS"
	WarnMalformedRowSkipped               WarningCode = "MALFORMED_ROW_SKIPPED"
	WarnEmptySheetSkipped                 WarningCode = "EMPTY_SHEET_SKIPPED"
	WarnCellTextTruncated                 WarningCode = "CELL_TEXT_TRUNCATED"
	WarnAmbiguousLabelColumn              WarningCode = "AMBIGUOUS_LABEL_COLUMN"
	WarnStructuralInterpretationUncertain WarningCode = "STRUCTURAL_INTERPRETATION_UNCERTAIN"

	// WarnPDFTextLayerMissing means a PDF page (or the whole document) has
	// little or no extractable embedded text, short of the
	// ErrCodeOCRRequired threshold (see that error code's doc comment for
	// the distinction) but still worth flagging — e.g. one image-heavy page
	// among otherwise-normal text pages. PDF only.
	WarnPDFTextLayerMissing WarningCode = "PDF_TEXT_LAYER_MISSING"
	// WarnPDFLayoutAmbiguous means row/column reconstruction from
	// positioned text could not confidently determine structure for part
	// of a page (e.g. text fragments whose Y-coordinates cluster into no
	// clean row grouping at the configured tolerance). PDF only.
	WarnPDFLayoutAmbiguous WarningCode = "PDF_LAYOUT_AMBIGUOUS"
	// WarnMultipleStatementsDetected means ingestion/pdf's boundary
	// detection found more than one statement section in a single PDF
	// document (see the ingestion/pdf package doc comment's Option A
	// multi-statement behavior) — informational, not a problem: every
	// detected statement is still returned, one Result per statement. PDF
	// only.
	WarnMultipleStatementsDetected WarningCode = "MULTIPLE_STATEMENTS_DETECTED"
	// WarnStatementBoundaryAmbiguous means ingestion/pdf found conflicting
	// or insufficient deterministic signals (page titles, statement-type
	// keywords, large vertical gaps, header changes) to confidently place
	// a statement section boundary, and declined to guess — see the
	// ingestion/pdf package doc comment's boundary-detection section. PDF
	// only.
	WarnStatementBoundaryAmbiguous WarningCode = "STATEMENT_BOUNDARY_AMBIGUOUS"
	// WarnColumnAlignmentAmbiguous means column reconstruction found more
	// than one equally plausible way to group positioned text into
	// columns for part of a page (e.g. no recurring numeric-column X
	// position was confidently identifiable). PDF only.
	WarnColumnAlignmentAmbiguous WarningCode = "COLUMN_ALIGNMENT_AMBIGUOUS"
	// WarnRepeatedHeaderRemoved means ingestion/pdf identified and
	// suppressed a repeated page header/column-heading row on a page after
	// the first it appeared on (see the ingestion/pdf package doc
	// comment's multi-page behavior), so it does not become a duplicate
	// row. PDF only.
	WarnRepeatedHeaderRemoved WarningCode = "REPEATED_HEADER_REMOVED"
	// WarnUnparseablePDFValue mirrors WarnUnparseableNumericCell but is
	// emitted specifically for a PDF-extraction-quirk value ingestion/pdf's
	// numeric handling recognized as monetary-shaped but could not safely
	// resolve deterministically (see the ingestion/pdf package doc
	// comment's numeric-parsing section) — kept as its own code, rather
	// than reusing WarnUnparseableNumericCell, so a caller can distinguish
	// "malformed in the source" from "an extraction-layout ambiguity
	// specific to PDF text reconstruction." PDF only.
	WarnUnparseablePDFValue WarningCode = "UNPARSEABLE_PDF_VALUE"

	// WarnOCRUsed means at least one page of this result was read via OCR
	// rather than an embedded text layer (see ingestion/pdf's OCR_AUTO/
	// OCR_FORCE modes). Purely informational — every OCR-derived row is
	// still returned normally; this is the signal a caller uses to decide
	// whether extra review is warranted for this document. PDF only.
	WarnOCRUsed WarningCode = "OCR_USED"
	// WarnLowOCRResolution means a scanned page's estimated effective
	// resolution (derived from its dominant image's pixel dimensions
	// against the PDF page's own known point size) is below the minimum
	// this package considers reliable for OCR (see ingestion/pdf's
	// minPlausibleDPI) — OCR is still attempted and its output still
	// returned, but with lower expected accuracy. PDF only.
	WarnLowOCRResolution WarningCode = "LOW_OCR_RESOLUTION"
	// WarnLowConfidenceLabel means a row's label text was reconstructed
	// substantially or entirely from OCR words below a confidence
	// threshold this package considers reliable (see ingestion/pdf's
	// numeric/label-confidence handling) — the label is still used
	// as-is (never silently dropped), but a caller should treat it as a
	// candidate for human review rather than fully trusted text. PDF
	// only.
	WarnLowConfidenceLabel WarningCode = "LOW_CONFIDENCE_LABEL"
	// WarnLowConfidenceNumericValue means a cell's numeric value was
	// parsed from OCR text below the confidence threshold — the value is
	// still parsed and returned when it matches a defensible numeric
	// pattern (see WarnOCRNumericAmbiguous for when it does NOT), but
	// should be treated as a review candidate. PDF only.
	WarnLowConfidenceNumericValue WarningCode = "LOW_CONFIDENCE_NUMERIC_VALUE"
	// WarnOCRNumericCorrected means a numeric cell's OCR text was
	// rewritten by a narrow, deterministic, context-sensitive correction
	// (e.g. a digit-shaped confusion within an otherwise unambiguous
	// numeric run — see ingestion/pdf's ocr_numeric.go) before parsing.
	// The ORIGINAL OCR text is always preserved in Cell.Raw/
	// OCRProvenance.OriginalText regardless of this warning, so a review
	// UI can always show what the engine actually reported. PDF only.
	WarnOCRNumericCorrected WarningCode = "OCR_NUMERIC_CORRECTED"
	// WarnOCRNumericAmbiguous means a numeric cell's OCR text contained a
	// pattern this package considers too ambiguous to safely parse or
	// correct (e.g. a mix of digits and letters with no single
	// defensible reading) — the cell is left unparsed (Cell.Parsed ==
	// false, Cell.Numeric == nil) rather than guessing, exactly like
	// WarnUnparseableNumericCell, but flagged with this more specific
	// code so a caller can distinguish "OCR read something numeric-
	// shaped but ambiguous" from an ordinary malformed value. PDF only.
	WarnOCRNumericAmbiguous WarningCode = "OCR_NUMERIC_AMBIGUOUS"
	// WarnMultiplePageImages means a scanned page contained two or more
	// embedded raster images that each independently looked page-shaped
	// (see ingestion/pdf/pdfimage's dominant-image selection) — this
	// package declined to guess which one is the real page scan; the
	// page could not be OCR'd. PDF only.
	WarnMultiplePageImages WarningCode = "MULTIPLE_PAGE_IMAGES"
	// WarnNoDominantPageImage means a scanned page had no embedded raster
	// image that plausibly represented the full page (e.g. only a small
	// logo/icon was found, or no image at all) — the page could not be
	// OCR'd. PDF only.
	WarnNoDominantPageImage WarningCode = "NO_DOMINANT_PAGE_IMAGE"
	// WarnMixedTextAndOCRPages means a single PDF document contained both
	// pages with a usable embedded text layer and pages requiring OCR
	// (OCR_AUTO mode) — informational: both kinds of pages were
	// successfully combined into one result via the same layout/
	// statement-interpretation pipeline. PDF only.
	WarnMixedTextAndOCRPages WarningCode = "MIXED_TEXT_AND_OCR_PAGES"
	// WarnUnsupportedEmbeddedImage means one or more embedded images on a
	// page used an encoding this package's PDF image-extraction adapter
	// could not decode (e.g. JBIG2/JPEG2000 — see
	// ingestion/pdf/pdfimage's doc comment) and were skipped; OCR
	// proceeded using whatever other image(s) remained. PDF only.
	WarnUnsupportedEmbeddedImage WarningCode = "UNSUPPORTED_EMBEDDED_IMAGE"
)

// Warning is a non-fatal parsing issue: the parser produced a result, but
// something about it should be reviewed. Distinct from Error (this
// package's fatal-error type), mirroring the reconciliation package's
// Check/Status split between "this ran and found something worth
// attention" versus "this could not run at all."
type Warning struct {
	// Code is the stable warning identifier. See the Warn* constants.
	Code WarningCode `json:"code"`
	// Message is a short human-readable explanation.
	Message string `json:"message"`
	// SheetIndex identifies the affected worksheet, when applicable.
	SheetIndex int `json:"sheet_index,omitempty"`
	// SheetName identifies the affected worksheet by name, when
	// applicable.
	SheetName string `json:"sheet_name,omitempty"`
	// RowIndex identifies the affected row, when applicable. -1 (omitted
	// from JSON via the pointer-free zero check callers should use
	// RowIndex >= 0 for) means not row-scoped.
	RowIndex int `json:"row_index,omitempty"`
	// ColumnIndex identifies the affected column, when applicable.
	ColumnIndex int `json:"column_index,omitempty"`
	// RowID identifies the affected Row.ID, when applicable.
	RowID string `json:"row_id,omitempty"`
	// PageIndex identifies the affected PDF page, when applicable. Always
	// omitted (zero value) for CSV/XLSX warnings. PDF only.
	PageIndex int `json:"page_index,omitempty"`
}

// ErrorCode is a stable identifier for a fatal parsing error. See the Err*
// constants.
type ErrorCode string

const (
	ErrCodeInvalidFile       ErrorCode = "INVALID_FILE"
	ErrCodeNoTabularData     ErrorCode = "NO_TABULAR_DATA"
	ErrCodeLimitExceeded     ErrorCode = "LIMIT_EXCEEDED"
	ErrCodeUnsupportedFormat ErrorCode = "UNSUPPORTED_FORMAT"

	// ErrCodeOCRRequired means a PDF was successfully opened as a valid PDF
	// document, but contains little or no extractable embedded text (e.g.
	// an image-only/scanned statement) — see ingestion/pdf's OCR-required
	// detection for the exact trigger thresholds. This is deliberately a
	// DIFFERENT code from ErrCodeNoTabularData: ErrCodeNoTabularData means
	// "there is no usable data here at all" (e.g. an empty CSV), while
	// ErrCodeOCRRequired means "there IS a real document, it simply has no
	// text layer the DEFAULT, non-OCR parse path can read." With
	// pdf.Options.OCR left at its default (OCRDisabled), this is still the
	// terminal outcome for such a document, exactly as before. A caller
	// that supplies pdf.Options.OCR = OCRAuto or OCRForce (along with an
	// ingestion/ocr.Engine) instead triggers OCR fallback for exactly the
	// pages that would otherwise produce this error — see the
	// ingestion/pdf package doc comment's OCR modes section.
	ErrCodeOCRRequired ErrorCode = "OCR_REQUIRED"
	// ErrCodePDFPageLimitExceeded means a PDF document has more pages than
	// Limits.MaxPages. A more specific ErrorCode than the generic
	// ErrCodeLimitExceeded so a caller can special-case "this document is
	// just too long" (e.g. offer to retry with a page range via
	// pdf.Options.PageStart/PageEnd) without string-matching Error.Detail.
	// PDF only.
	ErrCodePDFPageLimitExceeded ErrorCode = "PDF_PAGE_LIMIT_EXCEEDED"
	// ErrCodePDFTextLimitExceeded means a PDF document exceeded
	// Limits.MaxTextFragments (across the whole document) or
	// Limits.MaxTextLengthPerPage (on any single page) during extraction.
	// A more specific ErrorCode than the generic ErrCodeLimitExceeded,
	// analogous to ErrCodePDFPageLimitExceeded. PDF only.
	ErrCodePDFTextLimitExceeded ErrorCode = "PDF_TEXT_LIMIT_EXCEEDED"

	// ErrCodeOCREngineUnavailable means OCR was requested (OCRAuto or
	// OCRForce) but the supplied ingestion/ocr.Engine could not run (e.g.
	// ingestion/ocr/tesseract's configured executable is not installed) —
	// propagated from the engine's own ocr.ErrCodeEngineUnavailable. PDF
	// only.
	ErrCodeOCREngineUnavailable ErrorCode = "OCR_ENGINE_UNAVAILABLE"
	// ErrCodeOCREngineFailed means the OCR engine ran but reported a
	// failure recognizing a page (a non-zero exit code, malformed output,
	// etc.) — propagated from ocr.ErrCodeEngineFailed, or raised directly
	// by this package for an OCR-pipeline failure with no more specific
	// code (e.g. a decode failure of the engine's own preprocessed input
	// image). PDF only.
	ErrCodeOCREngineFailed ErrorCode = "OCR_ENGINE_FAILED"
	// ErrCodeOCRTimeout means OCR recognition did not complete within the
	// configured per-page or total OCR timeout (see Limits.OCRPageTimeout/
	// Limits.OCRTotalTimeout). PDF only.
	ErrCodeOCRTimeout ErrorCode = "OCR_TIMEOUT"
	// ErrCodeOCRPageLimitExceeded means a document requires OCR on more
	// pages than Limits.MaxOCRPages allows. PDF only.
	ErrCodeOCRPageLimitExceeded ErrorCode = "OCR_PAGE_LIMIT_EXCEEDED"
	// ErrCodeOCRImageLimitExceeded means a scanned page's dominant image
	// exceeds Limits.MaxImagePixels (or Limits.MaxImageDimension), a
	// defensive bound against a decompression-bomb-style oversized raster
	// embedded in an untrusted PDF. PDF only.
	ErrCodeOCRImageLimitExceeded ErrorCode = "OCR_IMAGE_LIMIT_EXCEEDED"
	// ErrCodeScannedPageImageUnavailable means a page requiring OCR has no
	// single extractable raster image this package can act on — either no
	// embedded image at all, or two or more images that each
	// independently looked page-shaped (see
	// ingestion/pdf/pdfimage.SelectDominantImage's ambiguity handling; the
	// specific reason is in Error.Detail). PDF only.
	ErrCodeScannedPageImageUnavailable ErrorCode = "SCANNED_PAGE_IMAGE_UNAVAILABLE"
	// ErrCodePDFPageRenderRequired means a scanned page's real content is
	// not representable as a single extractable embedded raster image at
	// all (e.g. genuine vector-drawn page content) — true PDF page
	// rendering, which this package does not implement (see the
	// ingestion/pdf/pdfimage package doc comment's scope note), would be
	// required to read this page. PDF only.
	ErrCodePDFPageRenderRequired ErrorCode = "PDF_PAGE_RENDER_REQUIRED"
)

// Error is a fatal parsing error: no usable Result could be produced. It
// implements the standard error interface. Distinct from Warning, which
// never prevents a Result from being returned.
type Error struct {
	// Code is the stable error identifier. See the ErrCode* constants.
	Code ErrorCode `json:"code"`
	// Message is a human-readable explanation.
	Message string `json:"message"`
	// Detail carries additional context, when useful (e.g. the limit that
	// was exceeded).
	Detail string `json:"detail,omitempty"`
}

func (e *Error) Error() string {
	if e.Detail != "" {
		return "ingestion: " + string(e.Code) + ": " + e.Message + " (" + e.Detail + ")"
	}
	return "ingestion: " + string(e.Code) + ": " + e.Message
}

// SheetInfo describes one worksheet/tab considered during parsing, XLSX
// only. CSV results carry a single synthetic SheetInfo with Index 0.
type SheetInfo struct {
	Index    int    `json:"index"`
	Name     string `json:"name,omitempty"`
	RowCount int    `json:"row_count"`
	Empty    bool   `json:"empty,omitempty"`
	Selected bool   `json:"selected"`
}

// Metadata carries parser provenance and detection summary information
// about a single parse, separate from the row data itself.
type Metadata struct {
	// Format identifies which parser produced this Result.
	Format Format `json:"format"`
	// Sheets lists every worksheet/tab considered. Always length 1 for
	// CSV.
	Sheets []SheetInfo `json:"sheets"`
	// SelectedSheetIndex is the 0-based index of the sheet Rows was built
	// from. Always 0 for CSV.
	SelectedSheetIndex int `json:"selected_sheet_index"`
	// StatementType is the deterministically detected statement type, or
	// financial.StatementType("") combined with StatementTypeUnknown==true
	// if detection could not identify one confidently, or the caller
	// forced it via Options.StatementTypeOverride.
	StatementType financial.StatementType `json:"statement_type,omitempty"`
	// StatementTypeUnknown is true when no statement type could be
	// deterministically identified (or Options.StatementTypeOverride was
	// StatementOverrideUnknown).
	StatementTypeUnknown bool `json:"statement_type_unknown,omitempty"`
	// StatementTypeEvidence is a short human-readable explanation of how
	// StatementType was determined (e.g. "sheet name contains 'balance
	// sheet'" or "caller override").
	StatementTypeEvidence string `json:"statement_type_evidence,omitempty"`
	// HeaderRowIndex is the 0-based row index the parser used as the
	// period-header row, when one was identified.
	HeaderRowIndex int `json:"header_row_index"`
	// LabelColumnIndex is the 0-based column index the parser used as the
	// line-item label column.
	LabelColumnIndex int `json:"label_column_index"`
	// Periods lists every detected reporting-period column, in column
	// order.
	Periods []DetectedPeriod `json:"periods"`
	// RowsRead is the total number of source rows read from the selected
	// sheet, including blank/heading rows.
	RowsRead int `json:"rows_read"`
	// Delimiter is the CSV delimiter used, CSV only. Empty for XLSX.
	Delimiter string `json:"delimiter,omitempty"`
	// Dependency documents the external library used to parse this file,
	// XLSX only (empty for CSV, which uses only the Go standard library).
	// See the ingestion/xlsx package doc comment and the repository
	// README.
	Dependency string `json:"dependency,omitempty"`
	// OCR carries OCR-usage summary metadata for this result, when OCR was
	// requested (pdf.Options.OCR != OCRDisabled) and at least one page was
	// evaluated for OCR. Nil when OCR was never requested/used. PDF only.
	// See OCRMetadata.
	OCR *OCRMetadata `json:"ocr,omitempty"`
}

// OCRMetadata summarizes OCR usage across a single parsed statement
// section, for caller-side reporting/review-UI decisions — never used as a
// guarantee of accounting correctness (see AverageConfidence's doc
// comment).
type OCRMetadata struct {
	// Used is true if any row in this result came from an OCR-read page.
	Used bool `json:"used"`
	// Pages lists the 0-based page indices that were read via OCR.
	Pages []int `json:"pages,omitempty"`
	// EmbeddedTextPages lists the 0-based page indices that were read via
	// the existing embedded-text-layer path (populated only when this
	// result mixes both — see WarnMixedTextAndOCRPages).
	EmbeddedTextPages []int `json:"embedded_text_pages,omitempty"`
	// EngineName identifies the OCR engine used (e.g. "tesseract"), copied
	// from ocr.Result.EngineName.
	EngineName string `json:"engine_name,omitempty"`
	// EngineVersion is the engine's own reported version string, when
	// obtainable.
	EngineVersion string `json:"engine_version,omitempty"`
	// AverageConfidence is the mean engine-reported confidence across
	// every OCR word contributing to this result, on whatever scale the
	// engine uses. INFORMATIONAL ONLY — never a guarantee of accounting
	// correctness or a calibrated probability (see the ingestion/ocr
	// package doc comment); a high average can still coexist with one
	// badly misread critical number, which is exactly why per-cell
	// OCRProvenance and the LowConfidence*/OCRNumericAmbiguous warnings
	// exist rather than relying on this single aggregate figure. -1 means
	// no confidence data was available.
	AverageConfidence float64 `json:"average_confidence"`
	// LowConfidenceNumericCount is the number of numeric cells in this
	// result whose OCRProvenance.ReviewRecommended is true due to low
	// confidence or a numeric correction.
	LowConfidenceNumericCount int `json:"low_confidence_numeric_count,omitempty"`
	// UnsupportedScanPageCount is the number of pages in the source
	// document that required OCR but could not be processed (ambiguous or
	// missing dominant page image, unsupported layout) — these pages
	// contributed no rows to this result. See ingestion/pdf's per-page
	// error handling in OCRAuto mode.
	UnsupportedScanPageCount int `json:"unsupported_scan_page_count,omitempty"`
}

// SchemaVersion identifies this package's fixed Result shape: the exact set
// and meaning of fields across Row, Cell, Metadata, DetectedPeriod, Warning,
// and OCR-specific metadata (OCRProvenance, OCRMetadata) that together make
// up a parsed ingestion contract, produced by every format (csv.Parse,
// xlsx.Parse, ingestion/pdf) via the shared BuildResult entry point. Bump
// this whenever a field is added, removed, or changes meaning in a way that
// could make a persisted historical Result not reproduce identically under
// the new code — see the repository README's versioning-strategy section
// and financial/metrics.FormulaVersion for the same convention applied
// elsewhere. Echoed on every Result so a persisted historical parse remains
// self-describing about exactly which ingestion contract produced it.
const SchemaVersion = "1.0.0"

// Result is the complete output of parsing a single tabular financial
// document: every structurally interpreted row, detection metadata, and
// non-fatal warnings. A Result is always returned together with a nil
// error on success; Error is returned alone (with a zero-value Result) on
// fatal failure — see csv.Parse and xlsx.Parse.
type Result struct {
	// SchemaVersion identifies which version of this package's fixed Result
	// shape produced this value — see the SchemaVersion constant's doc
	// comment. Always populated by BuildResult; a zero value means this
	// Result was constructed directly rather than via a parser.
	SchemaVersion string `json:"schema_version"`
	// Rows is every non-blank row read from the selected sheet, in
	// original source order (stable — see the package README's
	// determinism guarantees). Blank rows are omitted from Rows but do
	// still advance RowIndex numbering and are counted in
	// Metadata.RowsRead.
	Rows []Row `json:"rows"`
	// Metadata carries detection/provenance information about this parse.
	Metadata Metadata `json:"metadata"`
	// Warnings is every non-fatal issue found during parsing, in the order
	// encountered.
	Warnings []Warning `json:"warnings,omitempty"`
}

// ToRawLineItems converts every non-blank Row in the result into a
// financial.RawLineItem, ready to pass to
// financial/classification.ClassifyBatch. Only genuinely blank rows (Kind
// == StructuralBlank) are dropped — they never reach Result.Rows in the
// first place (see the Result.Rows doc comment), so this is a no-op filter
// in practice today, kept for defense in depth.
//
// Heading rows ARE included (as of the financial.RowKind field being
// added to RawLineItem): each Row's Kind is translated to the
// corresponding financial.RowKind via structuralKindToRowKind and carried
// on RawLineItem.Kind, so a heading row survives all the way through
// classification (which maps RowKindHeading to RowStatusIgnored — see
// financial/classification.Classify) into financial.MappedLineItem,
// letting a caller display section headings without separately keeping
// Result.Rows around and re-correlating by RowID. Subtotal/total rows are
// likewise included, with both Status (financial.RowStatusSubtotal/
// RowStatusTotal, unchanged from before) and the new Kind populated.
//
// RawLineItem.Kind is this package's best-effort structural hint, not a
// binding decision: financial/classification.Classify reads it before
// falling back to its own label-based heuristic (see that package's
// structural-detection stage), so the two packages' structural reads are
// now unified rather than independently re-derived — closing the
// known gap documented in the README's "Known deterministic ingestion
// gaps" section prior to this field's introduction.
func (r Result) ToRawLineItems() []financial.RawLineItem {
	items := make([]financial.RawLineItem, 0, len(r.Rows))
	for _, row := range r.Rows {
		if row.Kind == StructuralBlank {
			continue
		}
		values := make(map[financial.Period]float64, len(row.Values))
		for period, amount := range row.Values {
			values[period] = amount
		}
		items = append(items, financial.RawLineItem{
			ID:            row.ID,
			StatementType: r.Metadata.StatementType,
			Label:         row.Label,
			ParentLabel:   row.ParentLabel,
			Kind:          structuralKindToRowKind(row.Kind),
			Values:        values,
		})
	}
	return items
}
