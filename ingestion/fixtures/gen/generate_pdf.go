// PDF fixture builders for ingestion/pdf's test corpus. Uses only the
// hand-rolled writer in pdf_writer.go (see that file's doc comment for why
// no PDF-writing dependency was added). Every fixture here is synthetic
// and redistributable — no proprietary/customer statement data, matching
// the existing CSV/XLSX fixture corpus's identical constraint.
package main

import (
	"os"
	"path/filepath"
)

// generatePDFFixtures writes every PDF fixture ingestion/pdf's tests
// depend on. Called from main() in generate.go alongside the XLSX
// fixtures.
func generatePDFFixtures(dir string) error {
	writers := []struct {
		name string
		fn   func(path string) error
	}{
		{"simple_pl.pdf", writeSimplePL},
		{"multi_year_pl.pdf", writeMultiYearPL},
		{"multi_page_pl.pdf", writeMultiPagePL},
		{"balance_sheet.pdf", writeBalanceSheetPDF},
		{"pl_and_balance_sheet.pdf", writePLAndBalanceSheet},
		{"negative_parentheses.pdf", writeNegativeParenthesesPDF},
		{"indented_sections.pdf", writeIndentedSectionsPDF},
		{"unusual_spacing.pdf", writeUnusualSpacingPDF},
		{"image_only.pdf", writeImageOnlyPDF},
		{"ambiguous_layout.pdf", writeAmbiguousLayoutPDF},
	}
	for _, w := range writers {
		if err := w.fn(filepath.Join(dir, w.name)); err != nil {
			return err
		}
	}
	return nil
}

func savePDF(path string, doc *pdfDoc) error {
	return os.WriteFile(path, writePDF(doc), 0644)
}

// writeSimplePL builds a one-page income statement: title, a period
// header, revenue/COGS/gross-profit/opex/net-income rows — the baseline
// happy-path PDF fixture.
func writeSimplePL(path string) error {
	doc := &pdfDoc{}
	var page pdfPage
	page.newTitle(72, 730, "Riverside Consulting LLC")
	page.newTitle(72, 712, "Income Statement")
	page.newLine(72, 680, "Account")
	page.newLine(300, 680, "FY2025")
	page.newLine(72, 655, "Revenue")
	page.newLine(300, 655, "850,000")
	page.newLine(72, 635, "Cost of Goods Sold")
	page.newLine(300, 635, "320,000")
	page.newLine(72, 615, "Gross Profit")
	page.newLine(300, 615, "530,000")
	page.newLine(72, 590, "Operating Expenses")
	page.newLine(300, 590, "210,000")
	page.newLine(72, 565, "Net Income")
	page.newLine(300, 565, "320,000")
	doc.addPage(page)
	return savePDF(path, doc)
}

// writeMultiYearPL builds a one-page income statement with three period
// columns, exercising multi-period column reconstruction.
func writeMultiYearPL(path string) error {
	doc := &pdfDoc{}
	var page pdfPage
	page.newTitle(72, 730, "Northgate Retail Group")
	page.newTitle(72, 712, "Statement of Operations")
	page.newTitle(72, 696, "For the Years Ended December 31, 2023, 2024 and 2025")
	page.newLine(72, 665, "Account")
	page.newLine(260, 665, "2023")
	page.newLine(340, 665, "2024")
	page.newLine(420, 665, "2025")
	page.newLine(72, 640, "Revenue")
	page.newLine(260, 640, "1,200,000")
	page.newLine(340, 640, "1,450,000")
	page.newLine(420, 640, "1,780,000")
	page.newLine(72, 620, "Cost of Goods Sold")
	page.newLine(260, 620, "480,000")
	page.newLine(340, 620, "560,000")
	page.newLine(420, 620, "690,000")
	page.newLine(72, 600, "Total Operating Expenses")
	page.newLine(260, 600, "540,000")
	page.newLine(340, 600, "610,000")
	page.newLine(420, 600, "715,000")
	page.newLine(72, 575, "Net Income")
	page.newLine(260, 575, "180,000")
	page.newLine(340, 575, "280,000")
	page.newLine(420, 575, "375,000")
	doc.addPage(page)
	return savePDF(path, doc)
}

// writeMultiPagePL builds a two-page income statement where the title and
// column header block are reprinted at the top of page 2 — exercising
// repeated-header suppression (WarnRepeatedHeaderRemoved) and multi-page
// row/parent continuity.
func writeMultiPagePL(path string) error {
	doc := &pdfDoc{}

	header := func(p *pdfPage) {
		p.newTitle(72, 730, "Cascade Manufacturing Inc.")
		p.newTitle(72, 712, "Income Statement")
		p.newLine(72, 680, "Account")
		p.newLine(300, 680, "FY2025")
	}

	var page1 pdfPage
	header(&page1)
	page1.newLine(72, 655, "Revenue")
	page1.newLine(300, 655, "Operating Section")
	page1.newLine(72, 630, "Product Sales")
	page1.newLine(300, 630, "2,400,000")
	page1.newLine(72, 610, "Service Revenue")
	page1.newLine(300, 610, "600,000")
	page1.newLine(72, 585, "Total Revenue")
	page1.newLine(300, 585, "3,000,000")
	page1.newLine(72, 560, "Operating Expenses")
	page1.newLine(300, 560, "")
	page1.newLine(72, 535, "  Payroll")
	page1.newLine(300, 535, "980,000")
	page1.newLine(72, 515, "  Rent")
	page1.newLine(300, 515, "240,000")
	doc.addPage(page1)

	var page2 pdfPage
	header(&page2) // repeated title/header block, must be suppressed
	page2.newLine(72, 655, "  Utilities")
	page2.newLine(300, 655, "85,000")
	page2.newLine(72, 630, "Total Operating Expenses")
	page2.newLine(300, 630, "1,305,000")
	page2.newLine(72, 605, "Net Income")
	page2.newLine(300, 605, "1,695,000")
	doc.addPage(page2)

	return savePDF(path, doc)
}

// writeBalanceSheetPDF builds a one-page balance sheet with nested
// asset/liability/equity sections, subtotals, and a grand total.
func writeBalanceSheetPDF(path string) error {
	doc := &pdfDoc{}
	var page pdfPage
	page.newTitle(72, 730, "Cascade Manufacturing Inc.")
	page.newTitle(72, 712, "Balance Sheet")
	page.newTitle(72, 696, "As of December 31, 2025")
	page.newLine(72, 665, "Account")
	page.newLine(300, 665, "2025")
	page.newLine(72, 640, "Current Assets")
	page.newLine(72, 620, "  Cash")
	page.newLine(300, 620, "410,000")
	page.newLine(72, 600, "  Accounts Receivable")
	page.newLine(300, 600, "325,000")
	page.newLine(72, 580, "Total Current Assets")
	page.newLine(300, 580, "735,000")
	page.newLine(72, 555, "Total Assets")
	page.newLine(300, 555, "735,000")
	page.newLine(72, 530, "Current Liabilities")
	page.newLine(72, 510, "  Accounts Payable")
	page.newLine(300, 510, "180,000")
	page.newLine(72, 490, "Total Liabilities")
	page.newLine(300, 490, "180,000")
	page.newLine(72, 465, "Equity")
	page.newLine(72, 445, "  Retained Earnings")
	page.newLine(300, 445, "555,000")
	page.newLine(72, 425, "Total Equity")
	page.newLine(300, 425, "555,000")
	doc.addPage(page)
	return savePDF(path, doc)
}

// writePLAndBalanceSheet builds a two-page PDF containing an income
// statement on page 1 and a balance sheet on page 2 — the multiple-
// statements-in-one-document fixture (Option A: both are returned as
// separate ingestion.Result values from a single pdf.Parse call).
func writePLAndBalanceSheet(path string) error {
	doc := &pdfDoc{}

	var page1 pdfPage
	page1.newTitle(72, 730, "Fairview Health Partners")
	page1.newTitle(72, 712, "Income Statement")
	page1.newLine(72, 680, "Account")
	page1.newLine(300, 680, "FY2025")
	page1.newLine(72, 655, "Revenue")
	page1.newLine(300, 655, "4,200,000")
	page1.newLine(72, 630, "Cost of Goods Sold")
	page1.newLine(300, 630, "1,650,000")
	page1.newLine(72, 605, "Gross Profit")
	page1.newLine(300, 605, "2,550,000")
	page1.newLine(72, 580, "Net Income")
	page1.newLine(300, 580, "1,100,000")
	doc.addPage(page1)

	var page2 pdfPage
	page2.newTitle(72, 730, "Fairview Health Partners")
	page2.newTitle(72, 712, "Balance Sheet")
	page2.newLine(72, 680, "Account")
	page2.newLine(300, 680, "2025")
	page2.newLine(72, 655, "Cash")
	page2.newLine(300, 655, "890,000")
	page2.newLine(72, 630, "Total Assets")
	page2.newLine(300, 630, "890,000")
	doc.addPage(page2)

	return savePDF(path, doc)
}

// writeNegativeParenthesesPDF builds a one-page P&L exercising
// parenthetical-negative values, including the PDF-specific "( 1,234 )"
// internal-whitespace quirk (see ingestion/pdf's numeric.go).
func writeNegativeParenthesesPDF(path string) error {
	doc := &pdfDoc{}
	var page pdfPage
	page.newTitle(72, 730, "Bright Harbor Studio")
	page.newTitle(72, 712, "Income Statement")
	page.newLine(72, 680, "Account")
	page.newLine(300, 680, "FY2025")
	page.newLine(72, 655, "Revenue")
	page.newLine(300, 655, "500,000")
	page.newLine(72, 630, "Interest Expense")
	page.newLine(300, 630, "(12,400)")
	page.newLine(72, 605, "Other Expense")
	// Deliberately drawn with a gap wide enough that word-grouping treats
	// the opening/closing parens as separate glyph runs from the digits —
	// this is the "( 1,234 )" spacing quirk numeric.go's
	// reSpacedOpenParen/reSpacedCloseParen specifically normalize.
	page.newLine(300, 605, "(")
	page.newLine(310, 605, "3,100")
	page.newLine(340, 605, ")")
	page.newLine(72, 580, "Net Income")
	page.newLine(300, 580, "484,500")
	doc.addPage(page)
	return savePDF(path, doc)
}

// writeIndentedSectionsPDF builds a one-page P&L with X-offset indentation
// on child rows under section headings, exercising ingestion/pdf's
// X-offset-derived IndentLevel/ParentLabel detection.
func writeIndentedSectionsPDF(path string) error {
	doc := &pdfDoc{}
	var page pdfPage
	page.newTitle(72, 730, "Golden Valley Farms Co-op")
	page.newTitle(72, 712, "Income Statement")
	page.newLine(72, 680, "Account")
	page.newLine(300, 680, "FY2025")
	page.newLine(72, 655, "Revenue")
	page.newLine(300, 655, "920,000")
	page.newLine(72, 630, "Operating Expenses")
	page.newLine(90, 610, "Payroll")
	page.newLine(300, 610, "310,000")
	page.newLine(90, 590, "Advertising")
	page.newLine(300, 590, "45,000")
	page.newLine(72, 565, "Total Operating Expenses")
	page.newLine(300, 565, "355,000")
	page.newLine(72, 540, "Net Income")
	page.newLine(300, 540, "565,000")
	doc.addPage(page)
	return savePDF(path, doc)
}

// writeUnusualSpacingPDF builds a one-page P&L exercising every deterministic
// PDF numeric-extraction quirk numeric.go handles: a spaced currency
// prefix ("$ 1,234.00"), a trailing dash with a gap ("1,234 -"), and a
// space-separated thousands value ("1 234 500").
func writeUnusualSpacingPDF(path string) error {
	doc := &pdfDoc{}
	var page pdfPage
	page.newTitle(72, 730, "Ironwood Logistics")
	page.newTitle(72, 712, "Income Statement")
	page.newLine(72, 680, "Account")
	page.newLine(300, 680, "FY2025")
	page.newLine(72, 655, "Revenue")
	// "$ 1,234,500.00" split across two glyph runs with a gap after $.
	page.newLine(300, 655, "$")
	page.newLine(312, 655, "1,234,500.00")
	page.newLine(72, 630, "Miscellaneous Income")
	// space-separated thousands, European-extraction-style: "12 400"
	page.newLine(300, 630, "12 400")
	page.newLine(72, 605, "Discontinued Line Adjustment")
	// trailing dash meaning "no value" with a gap before it.
	page.newLine(300, 605, "45,000")
	page.newLine(340, 605, "-")
	doc.addPage(page)
	return savePDF(path, doc)
}

// writeImageOnlyPDF builds a page with zero text objects — only a drawn
// rectangle, simulating an image-only/scanned statement with no
// extractable text layer at all, for OCR-required-detection testing. A
// "real" scanned-looking raster image is unnecessary (per the task
// brief): the OCR-required trigger is "no usable extractable text," which
// a text-free content stream demonstrates just as validly and far more
// deterministically/reproducibly than embedding a JPEG.
func writeImageOnlyPDF(path string) error {
	doc := &pdfDoc{}
	var page pdfPage
	page.rects = append(page.rects, [4]float64{72, 72, 468, 648})
	doc.addPage(page)
	return savePDF(path, doc)
}

// writeAmbiguousLayoutPDF builds a page where text is scattered with
// highly irregular X/Y spacing and no discernible column alignment or
// period header — exercising WarnPDFLayoutAmbiguous / the general
// "declined to guess" behavior rather than a confident-but-wrong
// reconstruction.
func writeAmbiguousLayoutPDF(path string) error {
	doc := &pdfDoc{}
	var page pdfPage
	page.newTitle(72, 730, "Scattered Notes Page")
	page.newLine(75, 700, "alpha")
	page.newLine(210, 695, "beta")
	page.newLine(140, 688, "gamma")
	page.newLine(400, 680, "delta")
	page.newLine(95, 670, "epsilon")
	page.newLine(320, 660, "zeta")
	doc.addPage(page)
	return savePDF(path, doc)
}
