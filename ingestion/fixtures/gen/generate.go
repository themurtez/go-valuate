// Command generate builds the binary XLSX and PDF fixtures under
// ingestion/fixtures from code, since both are binary formats unsuitable
// for hand authoring or diffing as text. Run with:
//
//	go run ./ingestion/fixtures/gen
//
// Regenerate whenever a fixture's shape needs to change; the fixtures
// themselves are checked in so tests don't depend on this generator at
// test time. XLSX fixtures are built with github.com/xuri/excelize/v2
// (already a project dependency via ingestion/xlsx); PDF fixtures are
// built with a small hand-rolled writer (pdf_writer.go) rather than a
// second PDF-writing dependency — see that file's doc comment.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/xuri/excelize/v2"
)

func main() {
	dir := "ingestion/fixtures"
	if _, err := os.Stat(dir); err != nil {
		dir = "."
	}

	must(writeMultiSheetWorkbook(filepath.Join(dir, "multi_sheet_workbook.xlsx")))
	must(writeAmbiguousWorkbook(filepath.Join(dir, "ambiguous_workbook.xlsx")))
	must(writeTotalsSubtotalsWorkbook(filepath.Join(dir, "totals_subtotals.xlsx")))
	must(generatePDFFixtures(dir))
	must(generateOCRPDFFixtures(dir))

	fmt.Println("fixtures generated")
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// writeMultiSheetWorkbook builds a workbook with three tabs: a "Notes"
// sheet (should be scored low / skipped), an "Income Statement" sheet, and
// a "Balance Sheet" sheet — exercising deterministic sheet enumeration and
// auto-selection of the most plausible financial-statement sheet.
func writeMultiSheetWorkbook(path string) error {
	f := excelize.NewFile()
	defer f.Close()

	f.SetSheetName("Sheet1", "Notes")
	rows(f, "Notes", [][]interface{}{
		{"This workbook contains illustrative fixture data for testing."},
		{"See the Income Statement and Balance Sheet tabs."},
	})

	if _, err := f.NewSheet("Income Statement"); err != nil {
		return err
	}
	rows(f, "Income Statement", [][]interface{}{
		{"Account", "2024", "2025"},
		{"Revenue", 1000000.0, 1150000.0},
		{"Cost of Goods Sold", 400000.0, 445000.0},
		{"Gross Profit", 600000.0, 705000.0},
		{"Operating Expenses", 350000.0, 390000.0},
		{"Net Income", 250000.0, 315000.0},
	})

	if _, err := f.NewSheet("Balance Sheet"); err != nil {
		return err
	}
	rows(f, "Balance Sheet", [][]interface{}{
		{"Account", "2024", "2025"},
		{"Cash", 180000.0, 225000.0},
		{"Accounts Receivable", 210000.0, 240000.0},
		{"Total Assets", 390000.0, 465000.0},
	})

	idx, _ := f.GetSheetIndex("Income Statement")
	f.SetActiveSheet(idx)

	return f.SaveAs(path)
}

// writeAmbiguousWorkbook builds a workbook with two sheets of near-equal
// size and neutral names, so no signal deterministically identifies which
// one holds the primary financial statement — exercising
// WarnMultiplePlausibleSheets.
func writeAmbiguousWorkbook(path string) error {
	f := excelize.NewFile()
	defer f.Close()

	f.SetSheetName("Sheet1", "Q1 Data")
	rows(f, "Q1 Data", [][]interface{}{
		{"Account", "2025"},
		{"Revenue", 500000.0},
		{"Expenses", 350000.0},
	})

	if _, err := f.NewSheet("Q2 Data"); err != nil {
		return err
	}
	rows(f, "Q2 Data", [][]interface{}{
		{"Account", "2025"},
		{"Revenue", 540000.0},
		{"Expenses", 365000.0},
	})

	return f.SaveAs(path)
}

// writeTotalsSubtotalsWorkbook builds a single-sheet workbook exercising
// nested sections, subtotals, and a grand total, with indentation on
// child rows via leading spaces (excelize's SetCellValue preserves
// leading whitespace in string cells, unlike numeric formatting).
func writeTotalsSubtotalsWorkbook(path string) error {
	f := excelize.NewFile()
	defer f.Close()

	rows(f, "Sheet1", [][]interface{}{
		{"Account", "2025"},
		{"Operating Expenses", nil},
		{"  Advertising", 18000.0},
		{"  Payroll", 210000.0},
		{"  Rent", 48000.0},
		{"Total Operating Expenses", 276000.0},
		{"Other Expenses", nil},
		{"  Interest Expense", 9800.0},
		{"Total Other Expenses", 9800.0},
		{"Net Income", 150000.0},
	})

	return f.SaveAs(path)
}

func rows(f *excelize.File, sheet string, data [][]interface{}) {
	for r, row := range data {
		cell, _ := excelize.CoordinatesToCellName(1, r+1)
		rowCopy := row
		_ = f.SetSheetRow(sheet, cell, &rowCopy)
	}
}
