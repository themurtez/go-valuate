// OCR provenance attachment: annotating ingestion.Row/Cell values produced
// by buildSectionResult (build.go, via the unmodified
// ingestion.BuildResult) with OCR-specific metadata (confidence, original
// text, review recommendations) — see ingestion.OCRProvenance. This file
// runs strictly AFTER statement interpretation, exactly like build.go's
// own PDF-provenance patching (PageIndex, Cell.Bounds) for the
// embedded-text path — it adds information, never changes which
// row/cell/value BuildResult decided on.
package pdf

import (
	"strings"

	"github.com/themurtez/go-valuate/ingestion"
)

// lowConfidenceThreshold is the engine-reported confidence (on Tesseract's
// 0-100 scale — see the ingestion/ocr package doc comment's "confidence is
// not a probability" section) below which a word is considered
// review-worthy. 70 is a conservative middle ground: Tesseract's own
// documentation treats confidence below roughly 60-70 as "low" for
// general text; this package uses the more conservative (higher) end of
// that range specifically because financial NUMBERS carry outsized risk
// from a misread character (per the task contract's numeric-safety
// section) — a threshold a caller finds too strict/loose for their own
// documents is not currently configurable, matching this package's
// existing "conservative fixed threshold, not a tunable knob" pattern for
// analogous heuristics (e.g. defaultWordGapFactor).
const lowConfidenceThreshold = 70.0

// annotateOCRCells walks result's rows/cells and, for any row whose
// PageIndex was OCR'd (per ocrMeta's tracked pages), attaches
// ingestion.OCRProvenance to each cell by matching the cell's Bounds
// (already populated by buildSectionResult from the SAME word data — see
// build.go's boundsForWords) against ocrMeta's per-word confidence index,
// returning any WarnLowConfidenceLabel/WarnLowConfidenceNumericValue
// warnings generated along the way. ocrCorrected identifies cells
// applyOCRNumericCorrections already rewrote (ocr_numeric.go), so their
// provenance can record NumericCorrected=true. The caller (ocr_parse.go)
// merges the returned warnings into its own local warnings slice rather
// than relying on result.Warnings (which is populated separately and
// overwritten afterward — see ocr_parse.go's per-section loop).
func annotateOCRCells(result *ingestion.Result, ocrMeta ocrAggregateMeta, ocrCorrected map[ocrGridCellKey]string) []ingestion.Warning {
	if !ocrMeta.hasAny() {
		return nil
	}

	const labelColumnIndex = 0 // this package's grid always places the label in column 0 — see build.go's buildSectionGrid.

	var extraWarnings []ingestion.Warning
	for ri := range result.Rows {
		row := &result.Rows[ri]
		if !ocrMeta.pageWasOCR[row.PageIndex] {
			continue
		}
		for ci := range row.Cells {
			cell := &row.Cells[ci]
			if cell.Bounds == nil {
				continue
			}
			prov, ok := ocrMeta.provenanceForBounds(row.PageIndex, *cell.Bounds)
			if !ok {
				continue
			}
			if _, wasCorrected := ocrCorrected[ocrGridCellKey{row: row.RowIndex, col: cell.ColumnIndex}]; wasCorrected {
				prov.NumericCorrected = true
				prov.ReviewRecommended = true
			}
			cell.OCR = &prov

			lowConf := prov.Confidence >= 0 && prov.Confidence < lowConfidenceThreshold
			if cell.ColumnIndex == labelColumnIndex && lowConf {
				extraWarnings = append(extraWarnings, ingestion.Warning{
					Code: ingestion.WarnLowConfidenceLabel, RowIndex: row.RowIndex, ColumnIndex: cell.ColumnIndex,
					RowID: row.ID, PageIndex: row.PageIndex,
					Message: "this row's label was reconstructed from low-confidence OCR text",
				})
			}
			if cell.Numeric != nil && lowConf {
				extraWarnings = append(extraWarnings, ingestion.Warning{
					Code: ingestion.WarnLowConfidenceNumericValue, RowIndex: row.RowIndex, ColumnIndex: cell.ColumnIndex,
					RowID: row.ID, PageIndex: row.PageIndex,
					Message: "this numeric value was parsed from low-confidence OCR text",
				})
			}
		}
	}
	return extraWarnings
}

// ocrAggregateMeta accumulates OCR usage/confidence data across every page
// processed during ParseWithOCR, both for Metadata.OCR (document-level
// summary) and per-cell provenance lookup (annotateOCRCells).
type ocrAggregateMeta struct {
	pageWasOCR      map[int]bool
	engineName      string
	engineVersion   string
	confidenceSum   float64
	confidenceCount int
	lowConfCount    int
	// wordsByPage indexes every OCR word's converted (PDF-points) position
	// and original pixel bounds/confidence, per page, for provenance
	// lookup by cell bounds.
	wordsByPage map[int][]ocrProvenanceWord
}

type ocrProvenanceWord struct {
	x0, y                                   float64
	text                                    string
	confidence                              float64
	pixelX, pixelY, pixelWidth, pixelHeight int
}

// accumulate folds one page's OCR result into m, initializing internal maps
// on first use.
func (m *ocrAggregateMeta) accumulate(res ocrPageResult) {
	if m.pageWasOCR == nil {
		m.pageWasOCR = make(map[int]bool)
		m.wordsByPage = make(map[int][]ocrProvenanceWord)
	}
	if m.engineName == "" {
		m.engineName = res.engineName
	}
	if m.engineVersion == "" {
		m.engineVersion = res.engineVersion
	}
	for i, w := range res.words {
		meta := ocrWordMeta{confidence: -1}
		if i < len(res.wordMeta) {
			meta = res.wordMeta[i]
		}
		m.pageWasOCR[w.pageIndex] = true
		m.wordsByPage[w.pageIndex] = append(m.wordsByPage[w.pageIndex], ocrProvenanceWord{
			x0: w.x0, y: w.y, text: w.text, confidence: meta.confidence,
			pixelX: meta.pixelX, pixelY: meta.pixelY, pixelWidth: meta.pixelWidth, pixelHeight: meta.pixelHeight,
		})
		if meta.confidence >= 0 {
			m.confidenceSum += meta.confidence
			m.confidenceCount++
			if meta.confidence < lowConfidenceThreshold {
				m.lowConfCount++
			}
		}
	}
}

func (m *ocrAggregateMeta) hasAny() bool {
	return len(m.pageWasOCR) > 0
}

// provenanceForBounds finds the OCR word(s) on pageIndex whose converted
// (PDF-points) position falls within bounds, returning an
// ingestion.OCRProvenance summarizing them (confidence = the MINIMUM
// across contributing words — a cell is only as trustworthy as its
// least-confident constituent word — and original text = every
// contributing word's text joined in X order).
func (m ocrAggregateMeta) provenanceForBounds(pageIndex int, bounds ingestion.CellBounds) (ingestion.OCRProvenance, bool) {
	words, ok := m.wordsByPage[pageIndex]
	if !ok {
		return ingestion.OCRProvenance{}, false
	}

	const tolerance = 0.5 // PDF points; absorbs float rounding from the pixel->points conversion
	var matched []ocrProvenanceWord
	for _, w := range words {
		if w.x0 >= bounds.X0-tolerance && w.x0 <= bounds.X1+tolerance &&
			w.y >= bounds.Y0-tolerance && w.y <= bounds.Y1+tolerance {
			matched = append(matched, w)
		}
	}
	if len(matched) == 0 {
		return ingestion.OCRProvenance{}, false
	}

	minConf := matched[0].confidence
	var texts []string
	minX, minY := matched[0].pixelX, matched[0].pixelY
	maxX, maxY := matched[0].pixelX+matched[0].pixelWidth, matched[0].pixelY+matched[0].pixelHeight
	for _, w := range matched {
		if w.confidence >= 0 && (minConf < 0 || w.confidence < minConf) {
			minConf = w.confidence
		}
		texts = append(texts, w.text)
		if w.pixelX < minX {
			minX = w.pixelX
		}
		if w.pixelY < minY {
			minY = w.pixelY
		}
		if w.pixelX+w.pixelWidth > maxX {
			maxX = w.pixelX + w.pixelWidth
		}
		if w.pixelY+w.pixelHeight > maxY {
			maxY = w.pixelY + w.pixelHeight
		}
	}

	prov := ingestion.OCRProvenance{
		OriginalText: strings.Join(texts, " "),
		Confidence:   minConf,
		PixelBounds:  &ingestion.PixelBounds{X: minX, Y: minY, Width: maxX - minX, Height: maxY - minY},
	}
	prov.ReviewRecommended = minConf >= 0 && minConf < lowConfidenceThreshold
	return prov, true
}

func (m ocrAggregateMeta) toMetadata(ocrPages, embeddedTextPages []int, unsupportedScanPages int) *ingestion.OCRMetadata {
	avg := -1.0
	if m.confidenceCount > 0 {
		avg = m.confidenceSum / float64(m.confidenceCount)
	}
	return &ingestion.OCRMetadata{
		Used:                      true,
		Pages:                     ocrPages,
		EmbeddedTextPages:         embeddedTextPages,
		EngineName:                m.engineName,
		EngineVersion:             m.engineVersion,
		AverageConfidence:         avg,
		LowConfidenceNumericCount: m.lowConfCount,
		UnsupportedScanPageCount:  unsupportedScanPages,
	}
}
