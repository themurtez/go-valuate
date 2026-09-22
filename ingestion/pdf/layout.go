// Row and word reconstruction from positioned text.
//
// ledongthuc/pdf's Page.Content() returns one Text primitive per glyph/short
// run (see extract.go's fragment type and the spike this package's design
// was validated against), each carrying its own X/Y in PDF user-space
// points, NOT one primitive per word or per line — PDF content streams have
// no inherent concept of "word" or "row" at all, they are simply positioned
// drawing operations, and viewers/extractors reconstruct reading order
// themselves. This file does exactly that reconstruction, in two stages:
//
//  1. word grouping (groupWords): merge adjacent same-line fragments
//     whose horizontal gap is small relative to the text's own glyph
//     spacing into single logical words/cells, splitting on any larger
//     gap (a real inter-word space, or a jump to a different column).
//  2. row grouping (groupRows): cluster words into logical rows using
//     Y-proximity, tolerant of small sub-pixel Y differences between
//     glyphs nominally "on the same line" (common with kerning/rounding
//     in real PDF producers) via a configurable tolerance
//     (pdf.Options.RowYTolerance) rather than an exact Y match, which
//     would systematically under-group real single lines.
//
// Extraction order is explicitly NOT assumed to equal visual reading
// order (per the ingestion/pdf task contract): fragments are sorted by
// (Y descending, X ascending) before any grouping, so a content stream
// that draws e.g. the value column before the label column for a given
// row (some PDF producers do exactly this) still reconstructs correctly.
package pdf

import "sort"

// word is a reconstructed run of same-line, closely-spaced fragments: this
// package's unit of "one label or one number as printed," before column
// assignment groups words across a row into cells.
type word struct {
	pageIndex int
	// x0/x1 are the word's left/right edges; y is its baseline (the Y every
	// fragment in the word shared, within tolerance — see groupRows).
	x0, x1, y float64
	fontSize  float64
	text      string
}

// defaultRowYTolerance is used when Options.RowYTolerance is left at its
// zero value. 2.0 points (1/36 inch) comfortably absorbs the sub-point Y
// jitter real PDF producers introduce between glyphs nominally on the same
// baseline (observed directly in this package's own generated fixtures —
// see ingestion/fixtures/gen), while remaining well under a realistic
// minimum line-to-line spacing (even a dense 8pt-leading statement has
// >=8pt between baselines), so two genuinely distinct rows are never
// merged.
const defaultRowYTolerance = 2.0

// defaultWordGapFactor, multiplied by a fragment's FontSize, gives the
// minimum horizontal gap (in points) between two consecutive same-line
// fragments that is treated as a genuine inter-word space rather than
// ordinary intra-word glyph spacing. 0.30 (30% of font size) is a
// conservative middle ground for common proportional fonts at typical
// financial-statement sizes (8-12pt): comfortably wider than the gap
// between adjacent letters within a word (which W already accounts for via
// each fragment's own advance), comfortably narrower than a real space
// character's width in most fonts (~0.25-0.35em, plus the visual gap a
// statement's own column layout adds on top). This is a heuristic, not a
// guarantee — see the package doc comment's known-limitations discussion
// for layouts it doesn't handle well.
const defaultWordGapFactor = 0.30

// groupWords sorts fragments into reading order and merges adjacent
// same-line fragments with a small horizontal gap into words. Fragments
// with empty text (Td-only position markers some producers emit) are
// dropped before grouping.
func groupWords(fragments []fragment, rowYTolerance float64) []word {
	frags := make([]fragment, 0, len(fragments))
	for _, f := range fragments {
		if f.s == "" {
			continue
		}
		frags = append(frags, f)
	}

	sort.SliceStable(frags, func(i, j int) bool {
		if frags[i].pageIndex != frags[j].pageIndex {
			return frags[i].pageIndex < frags[j].pageIndex
		}
		if yDiffer(frags[i].y, frags[j].y, rowYTolerance) {
			return frags[i].y > frags[j].y
		}
		return frags[i].x < frags[j].x
	})

	var words []word
	var cur *word
	for _, f := range frags {
		w := fragmentWidth(f)
		if cur != nil &&
			cur.pageIndex == f.pageIndex &&
			!yDiffer(cur.y, f.y, rowYTolerance) &&
			f.x-cur.x1 < wordGap(f.fontSize) {
			cur.x1 = f.x + w
			cur.text += f.s
			continue
		}
		if cur != nil {
			words = append(words, *cur)
		}
		cur = &word{pageIndex: f.pageIndex, x0: f.x, x1: f.x + w, y: f.y, fontSize: f.fontSize, text: f.s}
	}
	if cur != nil {
		words = append(words, *cur)
	}
	return words
}

// yDiffer reports whether a and b are far enough apart (beyond tolerance)
// to be considered different baselines/rows.
func yDiffer(a, b, tolerance float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d > tolerance
}

func wordGap(fontSize float64) float64 {
	if fontSize <= 0 {
		fontSize = 10 // conservative fallback for a malformed/missing font size
	}
	return fontSize * defaultWordGapFactor
}

// fragmentWidth returns a fragment's usable width: its own W when
// positive, or a small positive fallback derived from FontSize when W is
// zero/unavailable (some PDF producers/fonts yield a zero advance for a
// fragment despite genuinely occupying space — see the package's font/
// Widths-array discussion in the README), so consecutive fragments are
// never spuriously merged/split purely because W was missing.
func fragmentWidth(f fragment) float64 {
	if f.w > 0 {
		return f.w
	}
	if f.fontSize > 0 {
		return f.fontSize * 0.5
	}
	return 1
}

// line is every word sharing one reconstructed row on one page, in
// left-to-right order.
type line struct {
	pageIndex int
	y         float64
	words     []word
}

// groupRows clusters words into lines by Y-proximity (within tolerance),
// preserving left-to-right order within each line via a final X sort. A
// word is compared against the RUNNING line reference Y (the first word's
// Y in that line), not the previous word's Y, so small consistent drift
// across a wide line (e.g. superscript/subscript glyphs) cannot chain
// multiple within-tolerance steps into merging two genuinely different
// rows.
func groupRows(words []word, rowYTolerance float64) []line {
	sorted := make([]word, len(words))
	copy(sorted, words)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].pageIndex != sorted[j].pageIndex {
			return sorted[i].pageIndex < sorted[j].pageIndex
		}
		return sorted[i].y > sorted[j].y
	})

	var lines []line
	var cur *line
	for _, w := range sorted {
		if cur == nil || cur.pageIndex != w.pageIndex || yDiffer(cur.y, w.y, rowYTolerance) {
			if cur != nil {
				sortLineWords(cur)
				lines = append(lines, *cur)
			}
			cur = &line{pageIndex: w.pageIndex, y: w.y}
		}
		cur.words = append(cur.words, w)
	}
	if cur != nil {
		sortLineWords(cur)
		lines = append(lines, *cur)
	}
	return lines
}

func sortLineWords(l *line) {
	sort.SliceStable(l.words, func(i, j int) bool {
		return l.words[i].x0 < l.words[j].x0
	})
}
