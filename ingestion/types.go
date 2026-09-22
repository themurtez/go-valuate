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
}

// DefaultLimits returns the conservative default Limits applied whenever a
// caller-supplied Limits field is left at its zero value.
func DefaultLimits() Limits {
	return Limits{
		MaxFileSizeBytes:  50 * 1024 * 1024, // 50 MiB
		MaxSheets:         100,
		MaxRows:           100_000,
		MaxColumns:        500,
		MaxCellTextLength: 4096,
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
// blank rows, neither of which financial.RowStatus represents), while
// Row.Status carries the financial.RowStatus value classification/
// normalization actually consume. See ToRawLineItem.
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
	// Limits.MaxCellTextLength if necessary.
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
}

// Row is one row of a tabular document as structurally interpreted by this
// package, before classification. It carries everything section 3 of the
// ingestion contract requires: raw cells, the detected label, detected
// period values, structural role, and section/parent context.
type Row struct {
	// ID is a deterministic identifier unique within a single Result:
	// "sheet-<index>-row-<n>" (0-based sheet index, 0-based row index). See
	// ToRawLineItem, which copies this into financial.RawLineItem.ID.
	ID string `json:"id"`
	// SheetIndex is the 0-based index of the source worksheet/tab. Always 0
	// for CSV.
	SheetIndex int `json:"sheet_index"`
	// SheetName is the source worksheet/tab name, when applicable. Empty
	// for CSV.
	SheetName string `json:"sheet_name,omitempty"`
	// RowIndex is the 0-based row index within the sheet as read from the
	// source (not reindexed after skipping blank rows), so it always
	// matches the original document for traceability.
	RowIndex int `json:"row_index"`
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
	// ToRawLineItem). Blank/heading rows are dropped before reaching
	// RawLineItem entirely — see Result.Rows vs ToRawLineItems.
	Status financial.RowStatus `json:"status"`
	// IndentLevel is a best-effort structural indentation depth (0 = no
	// indentation detected), used as one signal for Kind/ParentLabel
	// detection. Not a guarantee of accounting hierarchy depth.
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
}

// ErrorCode is a stable identifier for a fatal parsing error. See the Err*
// constants.
type ErrorCode string

const (
	ErrCodeInvalidFile       ErrorCode = "INVALID_FILE"
	ErrCodeNoTabularData     ErrorCode = "NO_TABULAR_DATA"
	ErrCodeLimitExceeded     ErrorCode = "LIMIT_EXCEEDED"
	ErrCodeUnsupportedFormat ErrorCode = "UNSUPPORTED_FORMAT"
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
}

// Result is the complete output of parsing a single tabular financial
// document: every structurally interpreted row, detection metadata, and
// non-fatal warnings. A Result is always returned together with a nil
// error on success; Error is returned alone (with a zero-value Result) on
// fatal failure — see csv.Parse and xlsx.Parse.
type Result struct {
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

// ToRawLineItems converts every non-structural Row in the result into a
// financial.RawLineItem, ready to pass to
// financial/classification.ClassifyBatch. Heading and blank rows (Kind ==
// StructuralHeading or StructuralBlank) are dropped: they carry no
// classifiable label/values and financial.RawLineItem has no field to
// represent "this row is a section heading". Subtotal/total rows ARE
// included (with Status set to financial.RowStatusSubtotal/
// RowStatusTotal), matching the way financial.RawLineItem carries
// structural rows through to classification (see
// financial/classification's structural detection, which this package's
// own detection deliberately mirrors but does not replace — a caller may
// still see classification independently reclassify a row's Status; this
// package's Kind/Status are a best-effort hint, not a binding decision).
func (r Result) ToRawLineItems() []financial.RawLineItem {
	items := make([]financial.RawLineItem, 0, len(r.Rows))
	for _, row := range r.Rows {
		if row.Kind == StructuralHeading || row.Kind == StructuralBlank {
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
			Values:        values,
		})
	}
	return items
}
