// Scanned/image-only PDF fixture builders for the OCR ingestion corpus
// (ingestion/pdf's OCR integration + ingestion/pdf/pdfimage tests). Every
// fixture embeds a synthetic, redistributable "scanned page" raster image
// (see scan_image.go) as a JPEG XObject covering the full page — never any
// proprietary/customer document, matching the existing CSV/XLSX/text-PDF
// corpus's identical constraint.
package main

import "path/filepath"

// generateOCRPDFFixtures writes every scanned-PDF fixture the OCR
// ingestion tests depend on. Called from main() alongside the text-PDF
// fixtures.
func generateOCRPDFFixtures(dir string) error {
	writers := []struct {
		name string
		fn   func(path string) error
	}{
		{"scanned_pl.pdf", writeScannedPL},
		{"scanned_balance_sheet.pdf", writeScannedBalanceSheet},
		{"scanned_low_resolution.pdf", writeScannedLowResolution},
		{"scanned_skewed.pdf", writeScannedSkewed},
		{"scanned_negative_parentheses.pdf", writeScannedNegativeParentheses},
		{"scanned_multi_year.pdf", writeScannedMultiYear},
		{"scanned_multi_page.pdf", writeScannedMultiPage},
		{"mixed_text_and_scanned.pdf", writeMixedTextAndScanned},
		{"scanned_with_logo.pdf", writeScannedWithLogo},
		{"scanned_ambiguous_numeric.pdf", writeScannedAmbiguousNumeric},
	}
	for _, w := range writers {
		if err := w.fn(filepath.Join(dir, w.name)); err != nil {
			return err
		}
	}
	return nil
}

// fullPageImage renders lines to a scan image at opts and returns a
// pdfPageImage placed to cover the entire 612x792 US-Letter page (with a
// small margin), the shape ingestion/pdf/pdfimage's dominant-image
// selection expects for a genuine full-page scan.
func fullPageImage(lines []scanLine, opts scanImageOptions) pdfPageImage {
	data, w, h := renderScanImage(lines, opts)
	return pdfPageImage{
		jpegData:    data,
		pixelWidth:  w,
		pixelHeight: h,
		x:           18, y: 18, w: 576, h: 756,
	}
}

// writeScannedPL builds a one-page scanned income statement: a synthetic
// full-page JPEG raster (no embedded text layer at all — see
// ingestion/pdf's OCR-required detection, which this fixture must trigger
// before OCR fallback engages) containing a title, a period header, and
// revenue/COGS/gross-profit/opex/net-income lines.
func writeScannedPL(path string) error {
	doc := &pdfDoc{}
	lines := []scanLine{
		{label: "Riverside Consulting LLC"},
		{label: "Income Statement"},
		{label: "Account", value: "FY2025"},
		{label: "Revenue", value: "850,000"},
		{label: "Cost of Goods Sold", value: "320,000"},
		{label: "Gross Profit", value: "530,000"},
		{label: "Operating Expenses", value: "210,000"},
		{label: "Net Income", value: "320,000"},
	}
	var page pdfPage
	page.images = append(page.images, fullPageImage(lines, scanImageOptions{}))
	doc.addPage(page)
	return savePDF(path, doc)
}

// writeScannedBalanceSheet builds a one-page scanned balance sheet.
func writeScannedBalanceSheet(path string) error {
	doc := &pdfDoc{}
	lines := []scanLine{
		{label: "Cascade Manufacturing Inc."},
		{label: "Balance Sheet"},
		{label: "Account", value: "2025"},
		{label: "Cash", value: "410,000"},
		{label: "Accounts Receivable", value: "325,000"},
		{label: "Total Current Assets", value: "735,000"},
		{label: "Total Assets", value: "735,000"},
		{label: "Accounts Payable", value: "180,000"},
		{label: "Total Liabilities", value: "180,000"},
		{label: "Retained Earnings", value: "555,000"},
		{label: "Total Equity", value: "555,000"},
	}
	var page pdfPage
	page.images = append(page.images, fullPageImage(lines, scanImageOptions{}))
	doc.addPage(page)
	return savePDF(path, doc)
}

// writeScannedLowResolution builds a scanned P&L rendered at a much lower
// pixel resolution (roughly 75 DPI equivalent) than the other fixtures,
// exercising LOW_OCR_RESOLUTION detection.
func writeScannedLowResolution(path string) error {
	doc := &pdfDoc{}
	lines := []scanLine{
		{label: "Bramblewood Supply Co."},
		{label: "Income Statement"},
		{label: "Account", value: "FY2025"},
		{label: "Revenue", value: "410,000"},
		{label: "Net Income", value: "88,000"},
	}
	var page pdfPage
	page.images = append(page.images, fullPageImage(lines, scanImageOptions{
		widthPx: defaultScanWidthPx / 2, heightPx: defaultScanHeightPx / 2,
	}))
	doc.addPage(page)
	return savePDF(path, doc)
}

// writeScannedSkewed builds a scanned P&L where each successive line is
// shifted slightly further right, simulating a mildly skewed scan.
func writeScannedSkewed(path string) error {
	doc := &pdfDoc{}
	lines := []scanLine{
		{label: "Harmon Freight Systems"},
		{label: "Income Statement"},
		{label: "Account", value: "FY2025"},
		{label: "Revenue", value: "1,240,000"},
		{label: "Cost of Goods Sold", value: "610,000"},
		{label: "Net Income", value: "290,000"},
	}
	var page pdfPage
	page.images = append(page.images, fullPageImage(lines, scanImageOptions{skewPixelsPerLine: 3}))
	doc.addPage(page)
	return savePDF(path, doc)
}

// writeScannedNegativeParentheses builds a scanned P&L with a
// parenthetical-negative value, exercising OCR numeric-safety handling for
// negative amounts.
func writeScannedNegativeParentheses(path string) error {
	doc := &pdfDoc{}
	lines := []scanLine{
		{label: "Bright Harbor Studio"},
		{label: "Income Statement"},
		{label: "Account", value: "FY2025"},
		{label: "Revenue", value: "500,000"},
		{label: "Interest Expense", value: "(12,400)"},
		{label: "Net Income", value: "487,600"},
	}
	var page pdfPage
	page.images = append(page.images, fullPageImage(lines, scanImageOptions{}))
	doc.addPage(page)
	return savePDF(path, doc)
}

// writeScannedMultiYear builds a scanned statement with three period
// columns.
func writeScannedMultiYear(path string) error {
	doc := &pdfDoc{}
	lines := []scanLine{
		{label: "Northgate Retail Group"},
		{label: "Statement of Operations"},
		{label: "Account (2023/2024/2025 combined below)"},
		{label: "Revenue 2023", value: "1,200,000"},
		{label: "Revenue 2024", value: "1,450,000"},
		{label: "Revenue 2025", value: "1,780,000"},
		{label: "Net Income 2025", value: "375,000"},
	}
	var page pdfPage
	page.images = append(page.images, fullPageImage(lines, scanImageOptions{}))
	doc.addPage(page)
	return savePDF(path, doc)
}

// writeScannedMultiPage builds a two-page scanned income statement, one
// full-page scan image per page, exercising multi-page OCR aggregation.
func writeScannedMultiPage(path string) error {
	doc := &pdfDoc{}

	page1Lines := []scanLine{
		{label: "Cascade Manufacturing Inc."},
		{label: "Income Statement"},
		{label: "Account", value: "FY2025"},
		{label: "Product Sales", value: "2,400,000"},
		{label: "Service Revenue", value: "600,000"},
		{label: "Total Revenue", value: "3,000,000"},
	}
	var page1 pdfPage
	page1.images = append(page1.images, fullPageImage(page1Lines, scanImageOptions{}))
	doc.addPage(page1)

	page2Lines := []scanLine{
		{label: "Total Operating Expenses", value: "1,305,000"},
		{label: "Net Income", value: "1,695,000"},
	}
	var page2 pdfPage
	page2.images = append(page2.images, fullPageImage(page2Lines, scanImageOptions{}))
	doc.addPage(page2)

	return savePDF(path, doc)
}

// writeMixedTextAndScanned builds a two-page PDF where page 1 has a real
// embedded text layer (an ordinary text-based income statement, exactly
// like simple_pl.pdf) and page 2 is a scanned/image-only balance sheet —
// exercising OCR_AUTO mode's per-page fallback: page 1 must be read via
// the existing embedded-text path, page 2 via OCR, combined into one
// deterministic pipeline (MIXED_TEXT_AND_OCR_PAGES).
func writeMixedTextAndScanned(path string) error {
	doc := &pdfDoc{}

	var page1 pdfPage
	page1.newTitle(72, 730, "Fairview Health Partners")
	page1.newTitle(72, 712, "Income Statement")
	page1.newLine(72, 680, "Account")
	page1.newLine(300, 680, "FY2025")
	page1.newLine(72, 655, "Revenue")
	page1.newLine(300, 655, "4,200,000")
	page1.newLine(72, 630, "Net Income")
	page1.newLine(300, 630, "1,100,000")
	doc.addPage(page1)

	page2Lines := []scanLine{
		{label: "Fairview Health Partners"},
		{label: "Balance Sheet"},
		{label: "Account", value: "2025"},
		{label: "Cash", value: "890,000"},
		{label: "Total Assets", value: "890,000"},
	}
	var page2 pdfPage
	page2.images = append(page2.images, fullPageImage(page2Lines, scanImageOptions{}))
	doc.addPage(page2)

	return savePDF(path, doc)
}

// writeScannedWithLogo builds a scanned P&L page containing TWO embedded
// images: a small "logo" raster in the corner plus the dominant full-page
// scan — exercising dominant-page-image selection (the small logo must be
// ignored via minPageCoverageRatio, never mistaken for the page).
func writeScannedWithLogo(path string) error {
	doc := &pdfDoc{}
	lines := []scanLine{
		{label: "Golden Valley Farms Co-op"},
		{label: "Income Statement"},
		{label: "Account", value: "FY2025"},
		{label: "Revenue", value: "920,000"},
		{label: "Net Income", value: "565,000"},
	}
	logoData, logoW, logoH := renderScanImage([]scanLine{{label: "LOGO"}}, scanImageOptions{widthPx: 120, heightPx: 60})

	var page pdfPage
	page.images = append(page.images, fullPageImage(lines, scanImageOptions{}))
	page.images = append(page.images, pdfPageImage{
		jpegData: logoData, pixelWidth: logoW, pixelHeight: logoH,
		x: 480, y: 750, w: 100, h: 30,
	})
	doc.addPage(page)
	return savePDF(path, doc)
}

// writeScannedAmbiguousNumeric builds a scanned P&L containing a
// deliberately ambiguous-looking numeric value (rendered small/adjacent to
// other glyphs in a way real OCR engines commonly misread, e.g. "1O,000"
// using a letter O in place of a zero) alongside otherwise-clean lines —
// exercising OCR_NUMERIC_AMBIGUOUS handling. This fixture is only
// meaningful for real-Tesseract integration testing (the deterministic
// fake-engine tests instead construct ambiguous ocr.Word values directly
// in Go — see ingestion/pdf's ocr_test.go), but is still included in the
// corpus per the task contract's fixture list.
func writeScannedAmbiguousNumeric(path string) error {
	doc := &pdfDoc{}
	lines := []scanLine{
		{label: "Ironwood Logistics"},
		{label: "Income Statement"},
		{label: "Account", value: "FY2025"},
		{label: "Revenue", value: "1O,OOO"}, // deliberate letter-O-for-zero ambiguity
		{label: "Net Income", value: "2,500"},
	}
	var page pdfPage
	page.images = append(page.images, fullPageImage(lines, scanImageOptions{}))
	doc.addPage(page)
	return savePDF(path, doc)
}
