// OCR numeric safety: narrow, deterministic, context-sensitive correction
// of common OCR digit-shaped character confusions ahead of
// ingestion/internal/tabular's existing, already-correct numeric parser
// (tabular.ParseNumeric) — never a parallel reimplementation of it, and
// never an aggressive global character substitution (per the task
// contract's explicit warning: financial-number OCR errors are high risk,
// and "aggressive character substitution globally" is exactly what this
// file must NOT do).
//
// # The correction rule
//
// A candidate numeric cell's OCR text is corrected ONLY when:
//  1. tabular.ParseNumeric already FAILS to parse the raw text as-is
//     (a cell that already parses cleanly is never touched, even if it
//     happens to contain a letter elsewhere in the row — this file never
//     "improves" an already-valid parse); AND
//  2. after substituting known digit-confusable letters (O/o -> 0, I/l/i
//     -> 1, S/s -> 5, B -> 8 — see letterDigitConfusions) ONLY in
//     positions where a digit is grammatically required by the
//     surrounding numeric-cell shape (never touching a currency symbol,
//     parenthesis, comma, decimal point, or minus sign, and never
//     substituting inside what is otherwise valid running text), the
//     corrected text parses successfully via the SAME unmodified
//     tabular.ParseNumeric.
//
// If both conditions hold, the corrected value is used and
// WarnOCRNumericCorrected is emitted; Cell.Raw/OCRProvenance.OriginalText
// always preserve the UNCORRECTED text regardless, so a future review UI
// can show exactly what OCR reported. If condition 1 fails (already
// parses) nothing happens. If the raw text fails AND the correction
// attempt ALSO fails (e.g. the confusion is genuinely ambiguous, or
// correcting still doesn't yield a valid numeric shape), the cell is left
// unparsed and WarnOCRNumericAmbiguous is emitted — this file never
// guesses a value it cannot deterministically justify.
package pdf

import (
	"regexp"
	"strings"

	"github.com/themurtez/go-valuate/ingestion"
	"github.com/themurtez/go-valuate/ingestion/internal/tabular"
)

// letterDigitConfusions maps each commonly OCR-confused letter to the
// single digit it is corrected to, applied only within a substring that
// otherwise already looks like the digit-run of a numeric cell (see
// looksLikeDigitRun) — never applied to arbitrary text.
var letterDigitConfusions = map[rune]rune{
	'O': '0', 'o': '0',
	'I': '1', 'l': '1', 'i': '1',
	'S': '5', 's': '5',
	'B': '8',
}

// reNumericShapeWithLetters matches a whole-cell candidate for letter
// -confusion correction: optional leading "$"/"(", then a run of
// digits/confusable-letters/commas/periods, optional trailing ")"/"%"/
// trailing minus, and nothing else — deliberately narrow so this file
// never attempts correction on a cell that also contains genuine running
// text (a label accidentally routed through numeric parsing is left
// completely alone, exactly as tabular.ParseNumeric already leaves it
// alone).
var reNumericShapeWithLetters = regexp.MustCompile(`^[$(]?[0-9OoIlisSB,.]+[)%]?[-–—]?$`)

// ocrNumericResult is the outcome of attempting to parse one OCR-derived
// numeric cell, carrying enough detail for both Cell.Numeric/Parsed and
// the Warn*/OCRProvenance annotations built on top of it.
type ocrNumericResult struct {
	tabular.NumericResult
	// CorrectedText is the text actually parsed, when it differs from the
	// original (empty if no correction was needed/attempted).
	CorrectedText string
	// Ambiguous is true when raw failed to parse AND no safe correction
	// was found — WarnOCRNumericAmbiguous territory. Mutually exclusive
	// with NumericResult.Parsed.
	Ambiguous bool
}

// parseOCRNumeric is the single call site every OCR-derived numeric cell
// goes through, mirroring numeric.go's parsePDFNumeric for the embedded-
// text path exactly: try the unmodified parser first, attempt one narrow
// documented correction only on failure, and never invent a value outside
// those two paths.
func parseOCRNumeric(raw string, dash tabular.DashTreatment) ocrNumericResult {
	direct := tabular.ParseNumeric(raw, dash)
	if direct.Parsed || direct.IsBlank || direct.IsDash || direct.LooksLikePercentage {
		// Already handled correctly by the unmodified parser — including
		// the "recognized as blank/dash/percentage" cases, which are not
		// failures at all and must never be routed through correction.
		return ocrNumericResult{NumericResult: direct}
	}

	trimmed := strings.TrimSpace(raw)
	if !reNumericShapeWithLetters.MatchString(trimmed) || !containsConfusableLetter(trimmed) {
		// Doesn't even look like a numeric cell with letter confusions, or
		// direct.Failed is for some other reason entirely (e.g. genuinely
		// non-numeric text) — leave it exactly as tabular.ParseNumeric
		// already reported, no ambiguity signal specific to OCR.
		return ocrNumericResult{NumericResult: direct}
	}

	corrected := correctConfusableLetters(trimmed)
	retry := tabular.ParseNumeric(corrected, dash)
	if retry.Parsed {
		return ocrNumericResult{NumericResult: retry, CorrectedText: corrected}
	}

	// The text looked numeric-shaped-with-letters, but even after the
	// narrow, documented correction it still doesn't parse — genuinely
	// ambiguous OCR output; reject rather than invent a value.
	return ocrNumericResult{NumericResult: direct, Ambiguous: true}
}

func containsConfusableLetter(s string) bool {
	for _, r := range s {
		if _, ok := letterDigitConfusions[r]; ok {
			return true
		}
	}
	return false
}

func correctConfusableLetters(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if d, ok := letterDigitConfusions[r]; ok {
			b.WriteRune(d)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// ocrGridCellKey identifies one grid cell for the correction/ambiguity
// tracking below, mirroring build.go's pdfQuirkCell.
type ocrGridCellKey struct{ row, col int }

// applyOCRNumericCorrections rewrites sg.grid IN PLACE for every cell on an
// OCR page (per ocrPages) that parseOCRNumeric can safely correct,
// BEFORE sg.grid is handed to ingestion.BuildResult — this is the single
// integration point between ocr_numeric.go's correction logic and the
// UNMODIFIED tabular.ParseNumeric call BuildResult itself makes: by the
// time BuildResult sees the grid, a correctable cell's text has already
// been rewritten to the form tabular.ParseNumeric parses on its own first
// attempt (exactly like numeric.go's normalizePDFNumericText already does
// for the embedded-text PDF-quirk cases — this is the identical pattern,
// applied to OCR letter-confusion cases instead). Returns the set of
// corrected cells (for WarnOCRNumericCorrected) and ambiguous cells (for
// WarnOCRNumericAmbiguous, reclassified from a resulting
// WarnUnparseableNumericCell exactly as reclassifyPDFNumericWarnings
// already does for PDF quirk cells).
func applyOCRNumericCorrections(sg *sectionGrid, dash tabular.DashTreatment, isOCRPage map[int]bool) (corrected, ambiguous map[ocrGridCellKey]string) {
	corrected = make(map[ocrGridCellKey]string)
	ambiguous = make(map[ocrGridCellKey]string)

	for r, row := range sg.grid {
		if r >= len(sg.rowPages) || !isOCRPage[sg.rowPages[r]] {
			continue // only OCR-derived rows are eligible for this correction pass
		}
		for c, cellText := range row {
			if c == 0 {
				continue // label column: never routed through numeric correction
			}
			res := parseOCRNumeric(cellText, dash)
			key := ocrGridCellKey{row: r, col: c}
			if res.CorrectedText != "" {
				sg.grid[r][c] = res.CorrectedText
				corrected[key] = cellText // original text, for provenance/warning detail
			} else if res.Ambiguous {
				ambiguous[key] = cellText
			}
		}
	}
	return corrected, ambiguous
}

// ocrNumericWarnings converts applyOCRNumericCorrections's corrected cell
// set into WarnOCRNumericCorrected ingestion.Warning values, resolving
// each grid (row, col) back to the ingestion.Result row it ended up as
// (via Row.RowIndex, which always equals the grid row index it came from
// — see build.go's buildSectionResult, which never renumbers RowIndex).
// Ambiguous cells are NOT given a new warning here — BuildResult already
// emits WarnUnparseableNumericCell for them (ParseNumeric failed on the
// UNCORRECTED text, which is what the grid still holds for an ambiguous
// cell — see applyOCRNumericCorrections, which only rewrites sg.grid for
// cells it successfully corrected), so reclassifyOCRAmbiguousWarnings
// (called separately by the caller) converts that existing warning into
// WarnOCRNumericAmbiguous instead, mirroring build.go's
// reclassifyPDFNumericWarnings pattern exactly rather than emitting a
// second, duplicate warning for the same cell.
func ocrNumericWarnings(result ingestion.Result, corrected map[ocrGridCellKey]string) []ingestion.Warning {
	if len(corrected) == 0 {
		return nil
	}
	rowByGridIndex := make(map[int]ingestion.Row, len(result.Rows))
	for _, row := range result.Rows {
		rowByGridIndex[row.RowIndex] = row
	}

	var warnings []ingestion.Warning
	for key := range corrected {
		row, ok := rowByGridIndex[key.row]
		if !ok {
			continue
		}
		warnings = append(warnings, ingestion.Warning{
			Code: ingestion.WarnOCRNumericCorrected, RowIndex: row.RowIndex, ColumnIndex: key.col,
			RowID: row.ID, PageIndex: row.PageIndex,
			Message: "this numeric value was corrected from a common OCR digit-shaped character confusion",
		})
	}
	return warnings
}

// reclassifyOCRAmbiguousWarnings changes any WarnUnparseableNumericCell
// warning whose (RowIndex, ColumnIndex) was tracked as an OCR-ambiguous
// cell (see applyOCRNumericCorrections's ambiguous return) into
// WarnOCRNumericAmbiguous — mirroring build.go's
// reclassifyPDFNumericWarnings exactly, one level up (OCR letter-confusion
// ambiguity vs. PDF-extraction-spacing ambiguity).
func reclassifyOCRAmbiguousWarnings(warnings []ingestion.Warning, ambiguous map[ocrGridCellKey]string) []ingestion.Warning {
	if len(ambiguous) == 0 {
		return warnings
	}
	out := make([]ingestion.Warning, len(warnings))
	copy(out, warnings)
	for i, w := range out {
		if w.Code != ingestion.WarnUnparseableNumericCell {
			continue
		}
		if _, isAmbiguous := ambiguous[ocrGridCellKey{row: w.RowIndex, col: w.ColumnIndex}]; isAmbiguous {
			out[i].Code = ingestion.WarnOCRNumericAmbiguous
			out[i].Message = "this cell's OCR text looks numeric-shaped but is too ambiguous to parse safely"
		}
	}
	return out
}
