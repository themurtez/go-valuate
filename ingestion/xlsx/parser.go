// Package xlsx parses XLSX financial statement workbooks into
// ingestion.Result.
//
// It uses github.com/xuri/excelize/v2 (BSD-3-Clause license; see the
// repository README's XLSX dependency section for the version pinned and
// the reasoning for choosing it over a hand-rolled Office Open XML
// reader). excelize's own types (*excelize.File, etc.) never appear in
// this package's exported API — every exported function here takes and
// returns only io.Reader/[]byte and ingestion package types, so a future
// caller can depend on ingestion/xlsx without ever importing excelize
// directly, and the dependency could be swapped later without an API
// break.
//
// Statement-type/period/structural interpretation is delegated entirely
// to the internal tabular package, shared with ingestion/csv — see that
// package's doc comment. This package's own job is narrow: enumerate
// sheets, select one, read its cells (preferring cached/calculated values
// over re-evaluating formulas — see the package doc comment on formula
// handling below), and hand a tabular.Grid to ingestion.BuildResult.
//
// Security. XLSX files are treated as untrusted input throughout:
//   - macros are never executed (excelize does not implement a VBA/macro
//     runtime at all);
//   - formulas are never evaluated — this package reads only the cached
//     value excelize surfaces from the workbook's last save (via
//     File.GetRows/File.GetCellValue), and emits
//     ingestion.WarnFormulaWithoutCachedValue when a formula cell has no
//     usable cached result, rather than computing one;
//   - external workbook links are never followed or refreshed (this
//     package never calls File.UpdateLinkedValue);
//   - ingestion.Limits bounds file size (checked before excelize ever
//     opens the archive), sheet count, row count, column count, and cell
//     text length, and excelize's own UnzipSizeLimit/UnzipXMLSizeLimit are
//     set from the same MaxFileSizeBytes limit to bound decompressed
//     memory use against a zip-bomb-style pathological input.
package xlsx

import (
	"bytes"
	"fmt"
	"io"

	"github.com/xuri/excelize/v2"

	"github.com/themurtez/go-valuate/ingestion"
	"github.com/themurtez/go-valuate/ingestion/internal/tabular"
)

// Dependency documents the external library this package wraps, for
// ingestion.Metadata.Dependency and the README.
const Dependency = "github.com/xuri/excelize/v2 (BSD-3-Clause)"

// Parse reads r as an XLSX workbook and produces an ingestion.Result. On
// fatal failure it returns a nil *ingestion.Result and a non-nil
// *ingestion.Error; on success it returns a populated Result and a nil
// error, with any non-fatal issues recorded in Result.Warnings.
//
// Sheet selection follows ingestion.Options.SheetName if set (fatal
// ErrCodeInvalidFile if no sheet with that exact name exists); otherwise
// the most plausible non-empty sheet is auto-selected, or, if more than
// one sheet is equally plausible, Parse returns a Result with zero Rows
// and a WarnMultiplePlausibleSheets warning describing every candidate —
// see the package-level security/determinism discussion for why this
// package never silently guesses.
func Parse(r io.Reader, opts ingestion.Options) (*ingestion.Result, *ingestion.Error) {
	opts = ingestion.ResolveOptions(opts)
	limits := opts.Limits

	limited := io.LimitReader(r, limits.MaxFileSizeBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, &ingestion.Error{
			Code:    ingestion.ErrCodeInvalidFile,
			Message: "failed to read input",
			Detail:  err.Error(),
		}
	}
	if int64(len(data)) > limits.MaxFileSizeBytes {
		return nil, &ingestion.Error{
			Code:    ingestion.ErrCodeLimitExceeded,
			Message: "input exceeds maximum file size",
			Detail:  fmt.Sprintf("limit is %d bytes", limits.MaxFileSizeBytes),
		}
	}
	if len(data) == 0 {
		return nil, &ingestion.Error{
			Code:    ingestion.ErrCodeNoTabularData,
			Message: "input is empty",
		}
	}

	f, err := excelize.OpenReader(bytes.NewReader(data), excelize.Options{
		UnzipSizeLimit:    limits.MaxFileSizeBytes * 20,
		UnzipXMLSizeLimit: limits.MaxFileSizeBytes * 20,
	})
	if err != nil {
		return nil, &ingestion.Error{
			Code:    ingestion.ErrCodeInvalidFile,
			Message: "failed to open XLSX workbook",
			Detail:  err.Error(),
		}
	}
	defer f.Close()

	names := f.GetSheetList()
	if len(names) == 0 {
		return nil, &ingestion.Error{
			Code:    ingestion.ErrCodeNoTabularData,
			Message: "workbook contains no worksheets",
		}
	}
	if len(names) > limits.MaxSheets {
		return nil, &ingestion.Error{
			Code:    ingestion.ErrCodeLimitExceeded,
			Message: "workbook exceeds maximum sheet count",
			Detail:  fmt.Sprintf("limit is %d sheets, workbook has %d", limits.MaxSheets, len(names)),
		}
	}

	sheetInfos := make([]ingestion.SheetInfo, len(names))
	candidates := make([]tabular.SheetCandidate, len(names))
	rowsBySheet := make([][][]string, len(names))

	for i, name := range names {
		rows, rerr := f.GetRows(name)
		if rerr != nil {
			return nil, &ingestion.Error{
				Code:    ingestion.ErrCodeInvalidFile,
				Message: fmt.Sprintf("failed to read sheet %q", name),
				Detail:  rerr.Error(),
			}
		}
		if len(rows) > limits.MaxRows {
			return nil, &ingestion.Error{
				Code:    ingestion.ErrCodeLimitExceeded,
				Message: fmt.Sprintf("sheet %q exceeds maximum row count", name),
				Detail:  fmt.Sprintf("limit is %d rows", limits.MaxRows),
			}
		}
		rows = enforceCellLimits(rows, limits)
		rowsBySheet[i] = rows

		empty := isSheetEmpty(rows)
		sheetInfos[i] = ingestion.SheetInfo{
			Index:    i,
			Name:     name,
			RowCount: len(rows),
			Empty:    empty,
		}
		candidates[i] = tabular.SheetCandidate{
			Index:             i,
			Name:              name,
			Empty:             empty,
			PlausibilityScore: plausibilityScore(name, rows),
		}
	}

	selection := tabular.SelectSheet(candidates, opts.SheetName)
	if selection.SelectedIndex < 0 {
		if opts.SheetName != "" {
			return nil, &ingestion.Error{
				Code:    ingestion.ErrCodeInvalidFile,
				Message: fmt.Sprintf("no sheet named %q in workbook", opts.SheetName),
			}
		}
		if selection.Ambiguous {
			var names []string
			for _, c := range candidates {
				if !c.Empty && c.PlausibilityScore == bestScore(candidates) {
					names = append(names, c.Name)
				}
			}
			return &ingestion.Result{
				SchemaVersion: ingestion.SchemaVersion,
				Metadata: ingestion.Metadata{
					Format:     ingestion.FormatXLSX,
					Sheets:     sheetInfos,
					Dependency: Dependency,
				},
				Warnings: []ingestion.Warning{{
					Code:    ingestion.WarnMultiplePlausibleSheets,
					Message: fmt.Sprintf("multiple sheets are equally plausible candidates for the primary financial statement: %v; specify Options.SheetName to select one", names),
				}},
			}, nil
		}
		return nil, &ingestion.Error{
			Code:    ingestion.ErrCodeNoTabularData,
			Message: "workbook has no non-empty worksheets",
		}
	}

	for i := range sheetInfos {
		sheetInfos[i].Selected = i == selection.SelectedIndex
	}

	selectedName := names[selection.SelectedIndex]
	rows := rowsBySheet[selection.SelectedIndex]
	if len(rows) == 0 {
		return nil, &ingestion.Error{
			Code:    ingestion.ErrCodeNoTabularData,
			Message: fmt.Sprintf("selected sheet %q has no data", selectedName),
		}
	}

	grid := make(tabular.Grid, len(rows))
	for i, r := range rows {
		grid[i] = r
	}

	formulaFn := func(row, col int) (string, bool) {
		cellName, cerr := excelize.CoordinatesToCellName(col+1, row+1)
		if cerr != nil {
			return "", false
		}
		formula, ferr := f.GetCellFormula(selectedName, cellName)
		if ferr != nil || formula == "" {
			return "", false
		}
		value, verr := f.GetCellValue(selectedName, cellName)
		hasCached := verr == nil && value != ""
		return formula, hasCached
	}

	result, _ := ingestion.BuildResult(ingestion.BuildInput{
		Format:     ingestion.FormatXLSX,
		SheetIndex: selection.SelectedIndex,
		SheetName:  selectedName,
		Grid:       grid,
		Opts:       opts,
		Formula:    formulaFn,
	})
	result.Metadata.Sheets = sheetInfos
	result.Metadata.Dependency = Dependency

	for _, info := range sheetInfos {
		if info.Empty {
			result.Warnings = append(result.Warnings, ingestion.Warning{
				Code:       ingestion.WarnEmptySheetSkipped,
				Message:    fmt.Sprintf("sheet %q is empty and was skipped from sheet selection", info.Name),
				SheetIndex: info.Index,
				SheetName:  info.Name,
			})
		}
	}

	return &result, nil
}

func enforceCellLimits(rows [][]string, limits ingestion.Limits) [][]string {
	out := make([][]string, len(rows))
	for i, row := range rows {
		r := row
		if len(r) > limits.MaxColumns {
			r = r[:limits.MaxColumns]
		}
		trimmed := make([]string, len(r))
		for j, cell := range r {
			if len(cell) > limits.MaxCellTextLength {
				cell = truncateRunes(cell, limits.MaxCellTextLength)
			}
			trimmed[j] = cell
		}
		out[i] = trimmed
	}
	return out
}

func truncateRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}

func isSheetEmpty(rows [][]string) bool {
	for _, row := range rows {
		for _, cell := range row {
			if cell != "" {
				return false
			}
		}
	}
	return true
}

func plausibilityScore(name string, rows [][]string) int {
	score := tabular.ScoreSheetName(name)
	score += len(rows)
	return score
}

func bestScore(candidates []tabular.SheetCandidate) int {
	best := 0
	first := true
	for _, c := range candidates {
		if c.Empty {
			continue
		}
		if first || c.PlausibilityScore > best {
			best = c.PlausibilityScore
			first = false
		}
	}
	return best
}
