package tabular

import (
	"fmt"
	"strings"
)

// CellFormula, when non-nil, supplies formula text for a given (row, col)
// cell — used only by the xlsx parser (CSV never has formulas). nil means
// "no formula lookup available."
type CellFormula func(row, col int) (formula string, hasCachedValue bool)

// AssembledCell mirrors ingestion.Cell without importing the parent
// package.
type AssembledCell struct {
	ColumnIndex int
	Raw         string
	Numeric     *float64
	Parsed      bool
	Formula     string
}

// AssembledRow mirrors ingestion.Row's structural fields, produced purely
// from a Grid plus detection results, without any format-specific
// knowledge.
type AssembledRow struct {
	RowIndex    int
	Label       string
	ParentLabel string
	Kind        RowKind
	IndentLevel int
	Uncertain   bool
	Cells       []AssembledCell
	Values      map[string]float64 // canonical period ID -> amount
}

// AssembleWarning is a warning produced during row assembly, using string
// codes matching ingestion.WarningCode's values (kept as plain strings here
// to avoid an import cycle; csv/xlsx translate back to the typed constant).
type AssembleWarning struct {
	Code        string
	Message     string
	RowIndex    int
	ColumnIndex int
	HasRow      bool
	HasColumn   bool
}

// AssembleOptions configures Assemble. All fields are required to be
// resolved by the caller (csv/xlsx) before calling — this package applies
// no further defaulting.
type AssembleOptions struct {
	LabelColumn   int
	HeaderRow     int
	Columns       []DetectedColumn
	DashTreatment DashTreatment
	Formula       CellFormula // may be nil
}

// Assemble walks every data row of g (excluding HeaderRow and any row
// before it — title/header rows are not emitted as data rows) and produces
// one AssembledRow per non-header row, classifying structure and numeric
// values, and tracking parent/section context across the whole grid.
//
// Rows strictly before HeaderRow are skipped entirely (treated as title
// rows), matching how financial statement exports conventionally place a
// company/statement title above the real period header.
func Assemble(g Grid, opts AssembleOptions) ([]AssembledRow, []AssembleWarning) {
	var warnings []AssembleWarning
	var rows []AssembledRow
	tracker := &ParentTracker{}

	start := 0
	if opts.HeaderRow >= 0 {
		start = opts.HeaderRow + 1
	}

	for r := start; r < g.RowCount(); r++ {
		rawLabel := g.Cell(r, opts.LabelColumn)
		label := strings.TrimSpace(rawLabel)
		indent := DetectIndentLevel(rawLabel)

		var cells []AssembledCell
		values := make(map[string]float64)
		hasAnyText := false
		hasValues := false

		colCount := g.ColCount(r)
		if colCount == 0 {
			colCount = g.MaxColCount()
		}

		for c := 0; c < colCount; c++ {
			text := g.TrimmedCell(r, c)
			cell := AssembledCell{ColumnIndex: c, Raw: text}

			if text != "" {
				hasAnyText = true
			}

			if c == opts.LabelColumn {
				cells = append(cells, cell)
				continue
			}

			var formulaText string
			var hasCached bool
			if opts.Formula != nil {
				formulaText, hasCached = opts.Formula(r, c)
			}
			if formulaText != "" {
				cell.Formula = formulaText
				if !hasCached {
					warnings = append(warnings, AssembleWarning{
						Code: "FORMULA_WITHOUT_CACHED_VALUE",
						Message: fmt.Sprintf("cell at row %d, column %d contains formula %q with no usable cached value",
							r, c, formulaText),
						RowIndex: r, HasRow: true, ColumnIndex: c, HasColumn: true,
					})
				}
			}

			periodCol, isPeriodCol := columnFor(opts.Columns, c)
			num := ParseNumeric(text, opts.DashTreatment)
			switch {
			case num.Parsed:
				v := num.Value
				cell.Numeric = &v
				cell.Parsed = true
				hasValues = true
				if isPeriodCol {
					values[periodCol.Period.CanonicalID] += v
				}
			case num.IsDash:
				hasValues = true
			case num.Failed:
				warnings = append(warnings, AssembleWarning{
					Code:        "UNPARSEABLE_NUMERIC_CELL",
					Message:     fmt.Sprintf("cell at row %d, column %d has text %q that could not be parsed as a numeric value", r, c, text),
					RowIndex:    r,
					HasRow:      true,
					ColumnIndex: c,
					HasColumn:   true,
				})
			}
			cells = append(cells, cell)
		}

		kind, uncertain := ClassifyRowKind(label, hasValues, hasAnyText)
		if uncertain {
			warnings = append(warnings, AssembleWarning{
				Code:     "STRUCTURAL_INTERPRETATION_UNCERTAIN",
				Message:  fmt.Sprintf("row %d has numeric values but no discernible label", r),
				RowIndex: r,
				HasRow:   true,
			})
		}

		parent := tracker.Observe(kind, indent, label)

		rows = append(rows, AssembledRow{
			RowIndex:    r,
			Label:       label,
			ParentLabel: parent,
			Kind:        kind,
			IndentLevel: indent,
			Uncertain:   uncertain,
			Cells:       cells,
			Values:      values,
		})
	}

	return rows, warnings
}

func columnFor(cols []DetectedColumn, idx int) (DetectedColumn, bool) {
	for _, c := range cols {
		if c.ColumnIndex == idx {
			return c, true
		}
	}
	return DetectedColumn{}, false
}
