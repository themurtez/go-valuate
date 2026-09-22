// Column reconstruction: turning a page/section's lines (see layout.go)
// into a tabular.Grid — the same [][]string shape ingestion.BuildResult
// already consumes for CSV/XLSX — by detecting recurring word X-positions
// as column boundaries.
//
// This package does NOT assume fixed X coordinates across documents (per
// the ingestion/pdf task contract): column boundaries are derived per
// document/section from where words actually cluster on the page, using
// header-row alignment as the strongest signal when a header row is
// identifiable and recurring numeric-column X-positions across the body
// otherwise (see detectColumns). A hardcoded X threshold would silently
// misalign the moment a statement uses different margins/font sizes.
package pdf

import (
	"sort"

	"github.com/themurtez/go-valuate/ingestion/internal/tabular"
)

// columnBoundary is one detected column's left edge (X), in document/
// section order (left to right). The label column is always boundary[0];
// every other boundary is a candidate value/period column.
type columnBoundary struct {
	x float64
}

// defaultColumnGapFactor, multiplied by the section's dominant font size,
// is the minimum horizontal gap between two word start-X positions for
// them to be treated as different columns rather than the same column
// (e.g. two numbers in the same column across different rows rarely land
// on the exact same X due to differing digit counts/right-alignment, but
// cluster closely; two genuinely different columns are separated by a
// full column's worth of whitespace, always much larger). 3.0 (300% of
// font size) is deliberately much larger than defaultWordGapFactor's 0.30
// (which separates WORDS within a line): a gap this wide is column-scale,
// not word-scale.
const defaultColumnGapFactor = 3.0

// buildGrid converts lines (already page/section-scoped and in row order —
// see selectSection in build.go) into a tabular.Grid, detecting column
// boundaries from the lines themselves. Each returned grid row corresponds
// 1:1 to one input line, in the same order (row index i in the grid
// matches lines[i]), which callers rely on for provenance mapping (e.g.
// PageIndex).
//
// Column 0 is always the label column (leftmost detected boundary).
// Detection strategy, applied in order:
//  1. if a header-like line exists (see findHeaderLine — a line whose
//     non-leftmost words look period-shaped per
//     ingestion/internal/tabular.ParsePeriodLabel), its words' X positions
//     anchor the column boundaries directly, since a real period header
//     is the strongest possible column-alignment signal a statement can
//     offer.
//  2. otherwise, column boundaries are inferred purely from where word
//     start-X positions cluster across every line (see clusterColumnXs).
func buildGrid(lines []line) [][]string {
	if len(lines) == 0 {
		return nil
	}
	boundaries := detectColumnBoundaries(lines)
	grid := make([][]string, len(lines))
	for i, ln := range lines {
		grid[i] = assignLineToColumns(ln, boundaries)
	}
	return grid
}

// detectColumnBoundaries is the shared boundary-detection strategy buildGrid
// and buildSectionGrid (build.go — which additionally needs per-cell
// bounds/quirk tracking buildGrid's plain [][]string return can't carry)
// both drive, so the two never disagree about where a document's columns
// are.
func detectColumnBoundaries(lines []line) []columnBoundary {
	fontSize := dominantFontSize(lines)
	var boundaries []columnBoundary
	if header, ok := findHeaderLine(lines); ok {
		boundaries = boundariesFromLine(header)
	} else {
		boundaries = clusterColumnXs(lines, fontSize)
	}
	if len(boundaries) == 0 {
		boundaries = []columnBoundary{{x: 0}}
	}
	return boundaries
}

// findHeaderLine returns the first line (scanning from the top of the
// section, mirroring ingestion/internal/tabular's own headerScanLimit-style
// "periods appear early" assumption) with at least one word, beyond the
// first, that parses as a period via tabular.ParsePeriodLabel with
// Confidence > 0. This mirrors ingestion.BuildResult's own header-row
// detection closely on purpose (see the package doc comment on reuse), but
// operates on words-with-X rather than a Grid row, since it runs BEFORE a
// Grid exists — the two are not literally the same function because their
// inputs differ in shape, but the underlying period-recognition call
// (tabular.ParsePeriodLabel) is the identical, un-reimplemented one.
func findHeaderLine(lines []line) (line, bool) {
	limit := len(lines)
	if limit > headerScanWindow {
		limit = headerScanWindow
	}
	for i := 0; i < limit; i++ {
		ln := lines[i]
		if len(ln.words) < 2 {
			continue
		}
		periodLike := 0
		for _, w := range ln.words[1:] {
			if tabular.ParsePeriodLabel(w.text).Confidence > 0 {
				periodLike++
			}
		}
		if periodLike >= 1 {
			return ln, true
		}
	}
	return line{}, false
}

// headerScanWindow bounds how many leading lines findHeaderLine considers,
// mirroring ingestion/internal/tabular's headerScanLimit rationale (a real
// statement's period header appears within the first handful of lines of
// its section; scanning further risks matching an ordinary data row whose
// value happens to look like a period).
const headerScanWindow = 15

// boundariesFromLine anchors one column boundary at each word's start-X in
// ln, in X order — used when ln is a recognized header line, since its
// word positions ARE the column positions by definition.
func boundariesFromLine(ln line) []columnBoundary {
	bounds := make([]columnBoundary, 0, len(ln.words))
	for _, w := range ln.words {
		bounds = append(bounds, columnBoundary{x: w.x0})
	}
	return bounds
}

// clusterColumnXs greedily clusters every word's start-X across all lines
// into column groups: sorted ascending, a new cluster starts whenever the
// gap from the previous X exceeds the column-scale threshold (see
// defaultColumnGapFactor). Each cluster's boundary is its minimum X (the
// leftmost a word in that column ever starts, which is the natural anchor
// for both left-aligned labels and right-aligned numbers — a
// right-aligned number's start-X varies with its own digit count, but
// never starts LEFT of the column's true boundary).
func clusterColumnXs(lines []line, fontSize float64) []columnBoundary {
	var xs []float64
	for _, ln := range lines {
		for _, w := range ln.words {
			xs = append(xs, w.x0)
		}
	}
	if len(xs) == 0 {
		return nil
	}
	sort.Float64s(xs)

	gap := fontSize * defaultColumnGapFactor
	var bounds []columnBoundary
	clusterMin := xs[0]
	prev := xs[0]
	for _, x := range xs[1:] {
		if x-prev > gap {
			bounds = append(bounds, columnBoundary{x: clusterMin})
			clusterMin = x
		}
		prev = x
	}
	bounds = append(bounds, columnBoundary{x: clusterMin})
	return bounds
}

// assignLineToColumns builds one grid row from ln given boundaries: every
// word is assigned to the LAST boundary at or before its start-X (i.e. the
// column region it falls within), and words assigned to the same column
// are joined with a single space in X order — this is what lets a
// multi-word label ("Total Operating Expenses") reconstruct as one cell
// even though word-grouping (layout.go) already split it into three
// separate words, since all three fall within the label column's X region
// (up to the next column's boundary).
func assignLineToColumns(ln line, boundaries []columnBoundary) []string {
	row := make([]string, len(boundaries))
	cellWords := make([][]string, len(boundaries))

	for _, w := range ln.words {
		col := columnFor(w.x0, boundaries)
		cellWords[col] = append(cellWords[col], w.text)
	}
	for i, words := range cellWords {
		row[i] = joinWords(words)
	}
	return row
}

func columnFor(x float64, boundaries []columnBoundary) int {
	col := 0
	for i, b := range boundaries {
		if x+0.01 >= b.x {
			col = i
		} else {
			break
		}
	}
	return col
}

func joinWords(words []string) string {
	if len(words) == 0 {
		return ""
	}
	out := words[0]
	for _, w := range words[1:] {
		out += " " + w
	}
	return out
}

// dominantFontSize returns the most common font size across every word in
// lines (ties broken by smallest size, for determinism), used to scale
// column-clustering tolerance to the document's actual text size rather
// than a fixed point value — a statement set in 6pt footnotes and one set
// in 14pt headers need proportionally different column gaps.
func dominantFontSize(lines []line) float64 {
	counts := make(map[float64]int)
	for _, ln := range lines {
		for _, w := range ln.words {
			if w.fontSize > 0 {
				counts[w.fontSize]++
			}
		}
	}
	if len(counts) == 0 {
		return 10
	}
	var sizes []float64
	for s := range counts {
		sizes = append(sizes, s)
	}
	sort.Float64s(sizes)
	best := sizes[0]
	bestCount := counts[sizes[0]]
	for _, s := range sizes[1:] {
		if counts[s] > bestCount {
			best = s
			bestCount = counts[s]
		}
	}
	return best
}
