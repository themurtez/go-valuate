// OCR-to-layout reconstruction: converting ingestion/ocr.Result (positioned
// OCR words, in source-image pixel coordinates, top-left origin, Y
// increasing downward) into this package's own word/line types (layout.go
// — the SAME types embedded-PDF-text extraction already produces, in PDF
// user-space points, bottom-up Y), so every downstream stage — groupRows,
// detectColumnBoundaries/buildSectionGrid (columns.go/build.go),
// splitSections (detect.go), and ultimately ingestion.BuildResult itself —
// runs completely unmodified for OCR-derived text exactly as it already
// does for embedded PDF text. This file is the ENTIRE OCR-specific
// reconstruction surface for row/column/statement interpretation: nothing
// downstream of ocrWordsToWords needs to know its words came from OCR
// rather than a PDF's own text layer.
package pdf

// ocrWordsToWords converts every recognized ocr.Word on one page into this
// package's word type, mapping pixel coordinates to PDF points via the
// page's own known point dimensions (pageWidthPts/pageHeightPts — the
// page's MediaBox size, from pdfimage.PageDims) and the source image's
// pixel dimensions, and flipping Y from top-down (image convention) to
// bottom-up (PDF convention) so the result is coordinate-system-identical
// to a word produced from embedded PDF text fragments.
//
// fontSize is approximated from each word's pixel Height, converted to
// points — close enough for this package's own purposes (wordGap/
// defaultColumnGapFactor scaling, dominantFontSize-based heuristics),
// since OCR provides no font-size metadata of its own the way embedded PDF
// text fragments do.
func ocrWordsToWords(words []recognizedWord, pageWidthPts, pageHeightPts float64, imgWidthPx, imgHeightPx int) []word {
	if imgWidthPx <= 0 || imgHeightPx <= 0 {
		return nil
	}
	scaleX := pageWidthPts / float64(imgWidthPx)
	scaleY := pageHeightPts / float64(imgHeightPx)

	out := make([]word, 0, len(words))
	for _, w := range words {
		x0 := float64(w.X) * scaleX
		x1 := float64(w.X+w.Width) * scaleX
		// Y flip: pixel Y is measured down from the image's top edge;
		// PDF points are measured up from the page's bottom edge. A
		// word's PIXEL top edge (w.Y) maps to its PDF-points TOP edge,
		// which in bottom-up coordinates is (pageHeightPts - topInPoints).
		// layout.go's word.y is used as a baseline/top reference
		// consistently (groupRows clusters by y, sorted descending), so
		// using the converted top edge here (not the vertical center)
		// keeps this consistent with how a PDF text fragment's own y
		// (its Td-positioned baseline, effectively near the glyph top for
		// this package's tolerance-based grouping) is already used.
		topPts := float64(w.Y) * scaleY
		y := pageHeightPts - topPts
		fontSizePts := float64(w.Height) * scaleY

		out = append(out, word{
			pageIndex: w.PageIndex,
			x0:        x0,
			x1:        x1,
			y:         y,
			fontSize:  fontSizePts,
			text:      w.Text,
		})
	}
	return out
}

// recognizedWord is this package's own copy of the fields it needs from
// ocr.Word, decoupled from that package's type so ocr_layout.go's
// conversion logic has a stable, minimal, purely-positional input shape
// independent of ocr.Word's own evolution (mirroring extract.go's
// identical fragment/upstream.Text decoupling for embedded PDF text). See
// ocr_engine.go for where an ocr.Word is converted into a recognizedWord
// (carrying confidence/grouping IDs onward for the numeric-safety and
// provenance layers, which read recognizedWord directly rather than
// re-deriving anything from the plain word/line types this file emits).
type recognizedWord struct {
	Text                      string
	Confidence                float64
	X, Y, Width, Height       int
	PageIndex                 int
	BlockNum, ParNum, LineNum int
}
