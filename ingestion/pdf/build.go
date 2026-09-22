// Assembling reconstructed sections (see detect.go) into ingestion.Result
// values, reusing ingestion.BuildResult — the exact same statement-
// interpretation entry point ingestion/csv and ingestion/xlsx already
// drive — for every bit of header/period/statement-type/structural-row
// detection, rather than reimplementing any of it (per the ingestion/pdf
// task contract's core reuse requirement). This file's own job is narrow:
// turn a section's lines into a tabular.Grid (columns.go), track which
// grid cells needed PDF-specific numeric normalization (numeric.go) so
// their eventual parse failures can be reported as UNPARSEABLE_PDF_VALUE
// rather than the generic UNPARSEABLE_NUMERIC_CELL, and patch PDF-only
// provenance (PageIndex, Cell.Bounds) onto BuildResult's output afterward,
// since BuildResult itself has no PDF-specific knowledge at all.
package pdf

import (
	"fmt"
	"strings"

	"github.com/themurtez/go-valuate/ingestion"
	"github.com/themurtez/go-valuate/ingestion/internal/tabular"
)

// pdfQuirkCell marks one grid cell whose raw text was changed by
// normalizePDFNumericText, so a resulting parse failure there (if any) can
// be attributed to a PDF extraction quirk rather than a generically
// malformed source value.
type pdfQuirkCell struct {
	row, col int
}

// sectionGrid is a section's data reshaped into exactly what
// ingestion.BuildResult needs, plus the bookkeeping this package layers on
// top: per-row PageIndex (for provenance) and per-cell bounds/quirk
// tracking.
type sectionGrid struct {
	grid       tabular.Grid
	rowPages   []int // rowPages[i] = PageIndex of grid row i
	cellBounds [][]*ingestion.CellBounds
	quirkCells map[pdfQuirkCell]bool
}

// buildSectionGrid converts sec.lines into a sectionGrid: one grid row per
// line, one grid column per word-column detected by buildGrid (this
// function re-derives columns itself rather than calling buildGrid
// directly, since it also needs per-cell bounds and quirk tracking that
// buildGrid's plain [][]string return can't carry).
func buildSectionGrid(sec section) sectionGrid {
	lines := sec.lines
	out := sectionGrid{
		rowPages:   make([]int, len(lines)),
		cellBounds: make([][]*ingestion.CellBounds, len(lines)),
		quirkCells: make(map[pdfQuirkCell]bool),
	}

	boundaries := detectColumnBoundaries(lines)
	baseLabelX := minLabelX(lines, boundaries)
	indentStep := indentStepPoints(lines)

	out.grid = make(tabular.Grid, len(lines))
	for i, ln := range lines {
		out.rowPages[i] = ln.pageIndex

		numCols := len(boundaries)
		row := make([]string, numCols)
		bounds := make([]*ingestion.CellBounds, numCols)
		cellWords := make([][]word, numCols)

		for _, w := range ln.words {
			col := columnFor(w.x0, boundaries)
			cellWords[col] = append(cellWords[col], w)
		}
		for col, words := range cellWords {
			texts := make([]string, len(words))
			for k, w := range words {
				texts[k] = w.text
			}
			original := joinWords(texts)
			normalized := normalizePDFNumericText(original)
			if col == 0 {
				// Prepend synthetic leading spaces proportional to this
				// label's X-offset from the section's baseline label X, so
				// ingestion/internal/tabular.DetectIndentLevel (which reads
				// LEADING WHITESPACE — the same signal XLSX cell text
				// naturally carries) can be reused completely unmodified
				// for PDF's X-offset-based indentation too, rather than
				// this package needing its own parallel indent algorithm.
				// DetectIndentLevel treats every 2 leading spaces as one
				// indent level, so indentStep (one visual indent step, in
				// points) is scaled accordingly.
				if len(words) > 0 && indentStep > 0 {
					offset := words[0].x0 - baseLabelX
					if offset > 0 {
						levels := int(offset/indentStep + 0.5)
						normalized = strings.Repeat("  ", levels) + normalized
					}
				}
			}
			row[col] = normalized
			if normalized != original {
				out.quirkCells[pdfQuirkCell{row: i, col: col}] = true
			}
			if b, ok := boundsForWords(words); ok {
				bounds[col] = b
			}
		}

		out.grid[i] = row
		out.cellBounds[i] = bounds
	}

	return out
}

// minLabelX returns the smallest label-column (column 0) word start-X
// across every line, used as the "no indentation" baseline X that every
// other label's offset is measured from.
func minLabelX(lines []line, boundaries []columnBoundary) float64 {
	min := 0.0
	first := true
	for _, ln := range lines {
		for _, w := range ln.words {
			if columnFor(w.x0, boundaries) != 0 {
				continue
			}
			if first || w.x0 < min {
				min = w.x0
				first = false
			}
		}
	}
	return min
}

// indentStepPointsFactor, multiplied by the section's dominant font size,
// approximates one visual indentation step — the X distance a real
// statement typically indents a child row under its parent heading by.
// 1.5 (150% of font size) approximates roughly 2-3 average character
// widths, a common visual indent step in real financial statements (and
// in this package's own generated fixtures — see
// ingestion/fixtures/gen/generate_pdf.go's indented_sections.pdf).
const indentStepPointsFactor = 1.5

func indentStepPoints(lines []line) float64 {
	return dominantFontSize(lines) * indentStepPointsFactor
}

// boundsForWords returns the tight bounding box covering every word in
// words (a grid cell's constituent words), or ok=false if words is empty
// (an empty cell has no bounds).
func boundsForWords(words []word) (*ingestion.CellBounds, bool) {
	if len(words) == 0 {
		return nil, false
	}
	b := ingestion.CellBounds{X0: words[0].x0, X1: words[0].x1, Y0: words[0].y, Y1: words[0].y}
	for _, w := range words[1:] {
		if w.x0 < b.X0 {
			b.X0 = w.x0
		}
		if w.x1 > b.X1 {
			b.X1 = w.x1
		}
		if w.y < b.Y0 {
			b.Y0 = w.y
		}
		if w.y > b.Y1 {
			b.Y1 = w.y
		}
	}
	return &b, true
}

// buildSectionResult runs sec through ingestion.BuildResult (reusing every
// bit of header/period/statement-type/structural detection unchanged —
// see the file doc comment) and patches on PDF-specific provenance and
// warning reclassification.
func buildSectionResult(sectionIndex int, sec section, sg sectionGrid, opts ingestion.Options) (ingestion.Result, []ingestion.Warning) {
	result, warnings := ingestion.BuildResult(ingestion.BuildInput{
		Format:     ingestion.FormatPDF,
		SheetIndex: sectionIndex,
		SheetName:  "",
		Grid:       sg.grid,
		Opts:       opts,
	})

	rowsByGridIndex := make(map[int]int, len(result.Rows))
	for i, row := range result.Rows {
		rowsByGridIndex[row.RowIndex] = i
	}

	for gridRow, resultRowIdx := range rowsByGridIndex {
		if gridRow < 0 || gridRow >= len(sg.rowPages) {
			continue
		}
		result.Rows[resultRowIdx].PageIndex = sg.rowPages[gridRow]
		result.Rows[resultRowIdx].ID = fmt.Sprintf("pdf-section-%d-row-%d", sectionIndex, gridRow)

		if gridRow < len(sg.cellBounds) {
			for c := range result.Rows[resultRowIdx].Cells {
				colIdx := result.Rows[resultRowIdx].Cells[c].ColumnIndex
				if colIdx >= 0 && colIdx < len(sg.cellBounds[gridRow]) {
					result.Rows[resultRowIdx].Cells[c].Bounds = sg.cellBounds[gridRow][colIdx]
				}
			}
		}
	}

	warnings = reclassifyPDFNumericWarnings(warnings, result, sg)

	for i := range warnings {
		if warnings[i].RowIndex >= 0 && warnings[i].RowIndex < len(sg.rowPages) {
			warnings[i].PageIndex = sg.rowPages[warnings[i].RowIndex]
		}
	}

	return result, warnings
}

// reclassifyPDFNumericWarnings changes any WarnUnparseableNumericCell
// warning whose (RowIndex, ColumnIndex) was tracked as a PDF numeric-quirk
// cell (see buildSectionGrid's quirkCells) into WarnUnparseablePDFValue —
// the cell's raw text needed PDF-specific spacing normalization
// (numeric.go) and STILL failed to parse afterward, so the failure is
// attributable to PDF extraction reconstruction rather than a plain
// malformed source value.
func reclassifyPDFNumericWarnings(warnings []ingestion.Warning, result ingestion.Result, sg sectionGrid) []ingestion.Warning {
	if len(sg.quirkCells) == 0 {
		return warnings
	}
	out := make([]ingestion.Warning, len(warnings))
	copy(out, warnings)
	for i, w := range out {
		if w.Code != ingestion.WarnUnparseableNumericCell {
			continue
		}
		if sg.quirkCells[pdfQuirkCell{row: w.RowIndex, col: w.ColumnIndex}] {
			out[i].Code = ingestion.WarnUnparseablePDFValue
		}
	}
	return out
}
