// Package tabular implements the statement-interpretation logic shared by
// ingestion/csv and ingestion/xlsx: given a plain 2-D grid of cell text (one
// sheet's worth), it detects the header/period row, the label column,
// statement type, per-row structural kind, and parent/section context, and
// parses numeric cell values. Neither format-specific parser (encoding/csv,
// excelize) is imported here — this package operates purely on []Grid, so
// the exact same interpretation rules apply to both formats and can never
// drift apart between them.
//
// This package is internal: its types are assembly-line intermediates
// consumed by ingestion/csv and ingestion/xlsx, which translate them into
// the public ingestion.Row/Result shapes. It performs no classification —
// see the ingestion package doc comment for that boundary.
package tabular

import "strings"

// Grid is one worksheet/document's cell text, read into memory as a plain
// 2-D slice: Grid[row][col]. A cell with no data is the empty string. Every
// row in Grid has already been trimmed to at most Options.MaxColumns
// columns by the caller (csv/xlsx parser) before being handed to this
// package, and Grid itself has already been trimmed to at most
// Options.MaxRows rows.
//
// Cell text is stored with trailing whitespace and line endings removed,
// but LEADING whitespace is deliberately preserved (where the source
// format has any — XLSX cell text commonly does, plain CSV usually
// doesn't), since it is the only signal DetectIndentLevel has for
// section/parent detection. Every comparison in this package (numeric
// parsing, label matching, period parsing) trims its own input, so
// preserved leading whitespace never affects anything except indentation
// detection.
type Grid [][]string

// RowCount returns the number of rows in the grid.
func (g Grid) RowCount() int { return len(g) }

// Cell returns the cell text at (row, col) with leading whitespace
// preserved (see the Grid doc comment), or "" if out of bounds.
func (g Grid) Cell(row, col int) string {
	if row < 0 || row >= len(g) {
		return ""
	}
	if col < 0 || col >= len(g[row]) {
		return ""
	}
	return g[row][col]
}

// TrimmedCell returns Cell(row, col) with leading and trailing whitespace
// removed, for callers that only care about comparable content.
func (g Grid) TrimmedCell(row, col int) string {
	return strings.TrimSpace(g.Cell(row, col))
}

// ColCount returns the number of columns in a specific row, or 0 if the row
// is out of bounds.
func (g Grid) ColCount(row int) int {
	if row < 0 || row >= len(g) {
		return 0
	}
	return len(g[row])
}

// MaxColCount returns the widest row's column count across the whole grid.
func (g Grid) MaxColCount() int {
	max := 0
	for _, row := range g {
		if len(row) > max {
			max = len(row)
		}
	}
	return max
}

// IsRowBlank reports whether every cell in the row is empty after
// trimming.
func (g Grid) IsRowBlank(row int) bool {
	for _, cell := range g.rowOrEmpty(row) {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}

func (g Grid) rowOrEmpty(row int) []string {
	if row < 0 || row >= len(g) {
		return nil
	}
	return g[row]
}
