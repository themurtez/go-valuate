package ingestion

import (
	"fmt"
	"strings"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/ingestion/internal/tabular"
)

// ResolveOptions returns o with every zero-value field normalized to its
// documented default. Exported so ingestion/csv and ingestion/xlsx can
// apply the same defaulting rules as this package without duplicating the
// logic (they cannot call the unexported defaultOptions directly, and
// re-implementing it in each package would risk the two formats drifting
// out of sync).
func ResolveOptions(o Options) Options {
	return defaultOptions(o)
}

// BuildInput is the format-agnostic input to BuildResult: a single sheet's
// grid, already read into memory by the csv or xlsx parser, plus resolved
// Options.
type BuildInput struct {
	Format     Format
	SheetIndex int
	SheetName  string
	Grid       tabular.Grid
	Opts       Options
	// Formula, when non-nil, supplies formula text/cached-value presence
	// for a given cell. XLSX only.
	Formula tabular.CellFormula
}

// BuildResult runs the full statement-interpretation pipeline (label
// column detection, header/period detection, statement-type detection,
// row assembly with structural/parent detection) over a single grid and
// produces the Rows/Metadata portion of a Result. The caller (csv.Parse or
// xlsx.Parse) fills in Metadata.Sheets/Dependency/Delimiter and merges any
// warnings it collected itself (e.g. CSV malformed-row warnings) with the
// ones returned here.
//
// This is exported (rather than kept internal) specifically so
// ingestion/csv and ingestion/xlsx — separate packages, per the ingestion
// contract's requirement that XLSX-specific types never leak into a
// shared public API — can both drive the identical interpretation logic
// without duplicating it.
func BuildResult(in BuildInput) (Result, []Warning) {
	opts := in.Opts
	grid := in.Grid
	var warnings []Warning

	labelCol := 0
	if opts.LabelColumnOverride != nil {
		labelCol = *opts.LabelColumnOverride
	} else {
		labelCol = tabular.DetectLabelColumn(grid)
	}

	headerDetection := tabular.HeaderDetection{RowIndex: -1}
	switch {
	case opts.HeaderRowOverride != nil:
		headerDetection = detectColumnsForRow(grid, *opts.HeaderRowOverride, labelCol)
	default:
		headerDetection = tabular.DetectHeaderRow(grid, labelCol)
		if headerDetection.RowIndex < 0 && len(opts.PeriodColumnOverrides) > 0 {
			// Auto-detection found no row with a confidently parseable
			// period (e.g. "Current"/"Prior" column headers, which carry
			// no absolute date). The caller has told us exactly which
			// columns are periods via PeriodColumnOverrides, which only
			// makes sense if row 0 is the header row — a document without
			// ANY header row would have nothing for those column indices
			// to label. Falling back to row 0 here avoids the header
			// labels themselves leaking through as a bogus data row.
			headerDetection = detectColumnsForRow(grid, 0, labelCol)
		}
	}

	if len(headerDetection.CandidateRows) > 1 {
		warnings = append(warnings, Warning{
			Code:    WarnMultiplePossibleHeaderRows,
			Message: fmt.Sprintf("multiple rows look like plausible period headers: %v; selected row %d", headerDetection.CandidateRows, headerDetection.RowIndex),
		})
	}

	columns := headerDetection.Columns
	columns = applyPeriodColumnOverrides(columns, opts.PeriodColumnOverrides)

	dash := tabular.DashAsBlank
	if opts.DashTreatment == DashAsZero {
		dash = tabular.DashAsZero
	}

	assembled, assembleWarnings := tabular.Assemble(grid, tabular.AssembleOptions{
		LabelColumn:   labelCol,
		HeaderRow:     headerDetection.RowIndex,
		Columns:       columns,
		DashTreatment: dash,
		Formula:       in.Formula,
	})

	for _, w := range assembleWarnings {
		warnings = append(warnings, Warning{
			Code:        WarningCode(w.Code),
			Message:     w.Message,
			SheetIndex:  in.SheetIndex,
			SheetName:   in.SheetName,
			RowIndex:    w.RowIndex,
			ColumnIndex: w.ColumnIndex,
		})
	}

	// Statement type detection.
	statementType, statementUnknown, evidence := detectStatement(in, grid, headerDetection.RowIndex)

	// Assemble title rows (rows strictly before the header) for evidence
	// only — already consumed inside detectStatement; nothing further
	// needed here.

	rows := make([]Row, 0, len(assembled))
	for _, ar := range assembled {
		if ar.Kind == tabular.RowBlank {
			// Blank rows carry no data and are omitted from Result.Rows
			// (see the ingestion.Result.Rows doc comment); RowIndex on
			// surrounding rows is never renumbered, so their position in
			// the source document remains traceable without them.
			continue
		}
		rowID := fmt.Sprintf("sheet-%d-row-%d", in.SheetIndex, ar.RowIndex)

		cells := make([]Cell, 0, len(ar.Cells))
		for _, ac := range ar.Cells {
			cells = append(cells, Cell{
				ColumnIndex: ac.ColumnIndex,
				Raw:         ac.Raw,
				Numeric:     ac.Numeric,
				Parsed:      ac.Parsed,
				Formula:     ac.Formula,
			})
		}

		values := make(map[financial.Period]float64, len(ar.Values))
		for periodID, amount := range ar.Values {
			values[financial.Period(periodID)] = amount
		}

		if ar.Uncertain {
			warnings = append(warnings, Warning{
				Code:       WarnStructuralInterpretationUncertain,
				Message:    fmt.Sprintf("row %d: structural interpretation uncertain", ar.RowIndex),
				SheetIndex: in.SheetIndex,
				SheetName:  in.SheetName,
				RowIndex:   ar.RowIndex,
				RowID:      rowID,
			})
		}

		rows = append(rows, Row{
			ID:          rowID,
			SheetIndex:  in.SheetIndex,
			SheetName:   in.SheetName,
			RowIndex:    ar.RowIndex,
			Label:       ar.Label,
			ParentLabel: ar.ParentLabel,
			Kind:        StructuralKind(ar.Kind),
			Status:      structuralKindToStatus(StructuralKind(ar.Kind)),
			IndentLevel: ar.IndentLevel,
			Cells:       cells,
			Values:      values,
		})
	}

	periods := make([]DetectedPeriod, 0, len(columns))
	for _, c := range columns {
		periods = append(periods, DetectedPeriod{
			ColumnIndex:   c.ColumnIndex,
			Period:        financial.Period(c.Period.CanonicalID),
			PeriodType:    PeriodType(c.Period.Type),
			OriginalLabel: grid.TrimmedCell(headerDetection.RowIndex, c.ColumnIndex),
			StartDate:     c.Period.StartDate,
			EndDate:       c.Period.EndDate,
			Confidence:    c.Period.Confidence,
			Evidence:      c.Period.Evidence,
		})
		if c.Period.Confidence == 0 || c.Period.Type == tabular.PeriodTypeUnknown {
			warnings = append(warnings, Warning{
				Code:        WarnPeriodLabelAmbiguous,
				Message:     fmt.Sprintf("column %d header %q could not be parsed into a canonical period with confidence", c.ColumnIndex, grid.TrimmedCell(headerDetection.RowIndex, c.ColumnIndex)),
				SheetIndex:  in.SheetIndex,
				SheetName:   in.SheetName,
				ColumnIndex: c.ColumnIndex,
			})
		}
	}

	if statementUnknown {
		warnings = append(warnings, Warning{
			Code:       WarnStatementTypeUnknown,
			Message:    "could not deterministically identify the statement type",
			SheetIndex: in.SheetIndex,
			SheetName:  in.SheetName,
		})
	}

	result := Result{
		SchemaVersion: SchemaVersion,
		Rows:          rows,
		Metadata: Metadata{
			Format:                in.Format,
			SelectedSheetIndex:    in.SheetIndex,
			StatementType:         statementType,
			StatementTypeUnknown:  statementUnknown,
			StatementTypeEvidence: evidence,
			HeaderRowIndex:        headerDetection.RowIndex,
			LabelColumnIndex:      labelCol,
			Periods:               periods,
			RowsRead:              grid.RowCount(),
		},
		Warnings: warnings,
	}
	return result, nil
}

func detectColumnsForRow(grid tabular.Grid, row, labelCol int) tabular.HeaderDetection {
	var cols []tabular.DetectedColumn
	for c := 0; c < grid.ColCount(row); c++ {
		if c == labelCol {
			continue
		}
		text := grid.TrimmedCell(row, c)
		if text == "" {
			continue
		}
		p := tabular.ParsePeriodLabel(text)
		cols = append(cols, tabular.DetectedColumn{ColumnIndex: c, Period: p})
	}
	return tabular.HeaderDetection{RowIndex: row, Columns: cols, CandidateRows: []int{row}}
}

func applyPeriodColumnOverrides(cols []tabular.DetectedColumn, overrides []PeriodColumnOverride) []tabular.DetectedColumn {
	if len(overrides) == 0 {
		return cols
	}
	byIndex := make(map[int]tabular.DetectedColumn, len(cols))
	for _, c := range cols {
		byIndex[c.ColumnIndex] = c
	}
	for _, o := range overrides {
		byIndex[o.ColumnIndex] = tabular.DetectedColumn{
			ColumnIndex: o.ColumnIndex,
			Period: tabular.ParsedPeriod{
				CanonicalID: string(o.Period),
				Type:        tabular.PeriodType(o.PeriodType),
				Confidence:  1.0,
				Evidence:    "caller override",
			},
		}
	}
	result := make([]tabular.DetectedColumn, 0, len(byIndex))
	for _, c := range byIndex {
		result = append(result, c)
	}
	sortDetectedColumns(result)
	return result
}

func sortDetectedColumns(cols []tabular.DetectedColumn) {
	for i := 1; i < len(cols); i++ {
		j := i
		for j > 0 && cols[j].ColumnIndex < cols[j-1].ColumnIndex {
			cols[j], cols[j-1] = cols[j-1], cols[j]
			j--
		}
	}
}

func structuralKindToStatus(k StructuralKind) financial.RowStatus {
	switch k {
	case StructuralSubtotal:
		return financial.RowStatusSubtotal
	case StructuralTotal:
		return financial.RowStatusTotal
	case StructuralHeading, StructuralBlank:
		return financial.RowStatusIgnored
	default:
		return financial.RowStatusNormal
	}
}

// structuralKindToRowKind maps a Row's StructuralKind to the
// financial.RowKind carried on RawLineItem.Kind by ToRawLineItems. Unlike
// structuralKindToStatus, StructuralBlank has no meaningful mapping here —
// blank rows never reach ToRawLineItems at all (see that method), so
// StructuralBlank falls into the same default case as StructuralNormal,
// which is never actually exercised for a blank row in practice.
func structuralKindToRowKind(k StructuralKind) financial.RowKind {
	switch k {
	case StructuralHeading:
		return financial.RowKindHeading
	case StructuralSubtotal:
		return financial.RowKindSubtotal
	case StructuralTotal:
		return financial.RowKindTotal
	default:
		return financial.RowKindNormal
	}
}

func detectStatement(in BuildInput, grid tabular.Grid, headerRow int) (financial.StatementType, bool, string) {
	if in.Opts.StatementTypeOverride != StatementOverrideNone {
		switch in.Opts.StatementTypeOverride {
		case StatementOverrideUnknown:
			return "", true, "caller override: forced unknown"
		case StatementOverrideIncomeStatement:
			return financial.StatementIncomeStatement, false, "caller override"
		case StatementOverrideBalanceSheet:
			return financial.StatementBalanceSheet, false, "caller override"
		case StatementOverrideCashFlow:
			return financial.StatementCashFlow, false, "caller override"
		}
	}

	var titleRows []string
	limit := headerRow
	if limit < 0 || limit > grid.RowCount() {
		limit = grid.RowCount()
	}
	for r := 0; r < limit; r++ {
		if grid.IsRowBlank(r) {
			continue
		}
		titleRows = append(titleRows, joinRow(grid, r))
	}

	// Statement-type line-item signals (e.g. "gross profit", "total
	// assets") are matched against each row's full joined text rather than
	// just the label column, since this helper runs before label-column
	// detection is threaded through and joined text is robust to the
	// label appearing in any column.
	var rowLabels []string
	for r := 0; r < grid.RowCount(); r++ {
		if grid.IsRowBlank(r) {
			continue
		}
		rowLabels = append(rowLabels, joinRow(grid, r))
	}

	det := tabular.DetectStatementType(in.SheetName, titleRows, rowLabels)
	if det.Type == tabular.StatementUnknown {
		return "", true, det.Evidence
	}
	return financial.StatementType(det.Type), false, det.Evidence
}

func joinRow(grid tabular.Grid, row int) string {
	n := grid.ColCount(row)
	parts := make([]string, 0, n)
	for c := 0; c < n; c++ {
		if t := grid.TrimmedCell(row, c); t != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, " ")
}
