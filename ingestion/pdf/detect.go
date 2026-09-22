// Statement boundary detection (splitting one PDF document into one or
// more statement sections — see the package doc comment's Option A
// discussion) and OCR-required / text-layer detection.
package pdf

import (
	"strings"

	"github.com/themurtez/go-valuate/ingestion"
	"github.com/themurtez/go-valuate/ingestion/internal/tabular"
)

// minTextFragmentsForUsableDocument is the safe minimum total extractable
// text fragment count (see extract.go's fragment — roughly one per
// glyph/run) below which a document is treated as having no usable text
// layer at all, regardless of page count: a real single-page financial
// statement, even a very short one, has at minimum a title and a handful
// of line items, comfortably producing hundreds of fragments. A handful of
// stray fragments (e.g. a page number, a filename in a footer) is not
// enough to be a usable statement, but is also not zero — this threshold
// distinguishes "genuinely no text layer" (ErrCodeOCRRequired) from "a
// real but very sparse document" (allowed through, possibly with
// WarnPDFTextLayerMissing on individual pages — see hasUsableTextLayer).
const minTextFragmentsForUsableDocument = 20

// minFragmentsPerPageRatio is the minimum average fragment count per page
// below which the document is treated as image-only/scanned, scaled by
// page count so a genuinely long image-only document (many blank-of-text
// pages) is caught even though its raw total might exceed
// minTextFragmentsForUsableDocument.
const minFragmentsPerPageRatio = 3.0

// hasUsableTextLayer reports whether extracted has enough text to
// deterministically interpret as a financial statement, and the reason if
// not — this is the OCR-required trigger (see decision-record for the
// exact thresholds this package uses; both are conservative on purpose,
// since a false "OCR required" on a real text PDF is a much worse failure
// mode for this package than occasionally not catching an extremely
// sparse image-only page).
func hasUsableTextLayer(extracted extractResult) (ok bool, reason string) {
	total := len(extracted.fragments)
	if total == 0 {
		return false, "no extractable text found in any page"
	}
	if total < minTextFragmentsForUsableDocument {
		return false, "extractable text is far below the minimum expected for a real financial statement"
	}
	avgPerPage := float64(total) / float64(extracted.pageCount)
	if avgPerPage < minFragmentsPerPageRatio {
		return false, "average extractable text per page is far below the minimum expected for a real financial statement"
	}
	return true, ""
}

// pagesWithNoText returns the 0-based indices of every page that produced
// zero text fragments despite the document as a whole passing
// hasUsableTextLayer — e.g. one scanned exhibit page inserted into an
// otherwise-normal text statement. Each such page gets a
// WarnPDFTextLayerMissing warning rather than failing the whole parse.
func pagesWithNoText(extracted extractResult) []int {
	seen := make(map[int]bool, extracted.pageCount)
	for _, f := range extracted.fragments {
		seen[f.pageIndex] = true
	}
	var pages []int
	for i := 0; i < extracted.pageCount; i++ {
		if !seen[i] {
			pages = append(pages, i)
		}
	}
	return pages
}

// titleSignal is one detected statement-type title/keyword occurrence
// within the document, anchored to the line it was found on.
type titleSignal struct {
	lineIndex int // index into the document-order concatenated lines slice
	statement tabular.StatementType
	evidence  string
}

// findTitleSignals scans lines (already in full-document reading order —
// concatenated across every page in page order, each page's own lines in
// Y-then-X order per groupRows) for statement-type title signals, reusing
// tabular.DetectStatementType's own signal-matching per candidate line
// (via matchTitleLine) rather than a separate keyword list — see the
// package doc comment on reuse. Only lines that look like a title (short,
// and matching a known statement-type phrase) are considered; this is
// deliberately narrower than tabular.DetectStatementType's own third tier
// (line-item label scanning across the WHOLE grid), since that tier is
// still applied per-section afterward via ingestion.BuildResult itself —
// this function's job is only to find SECTION BOUNDARIES, not to make the
// final statement-type call for each resulting section (BuildResult does
// that, exactly as it already does for CSV/XLSX).
func findTitleSignals(lines []line) []titleSignal {
	var signals []titleSignal
	for i, ln := range lines {
		text := lineText(ln)
		if text == "" || len(text) > 80 {
			// A real title row is short; a long line of running text at
			// this stage is virtually always an ordinary data/label row,
			// not a title, even if it happens to contain a matching
			// phrase substring — skip it rather than risk a false
			// boundary split mid-statement.
			continue
		}
		det := tabular.DetectStatementType("", []string{text}, nil)
		if det.Type == tabular.StatementUnknown {
			continue
		}
		signals = append(signals, titleSignal{lineIndex: i, statement: det.Type, evidence: det.Evidence})
	}
	return signals
}

func lineText(ln line) string {
	var b strings.Builder
	for i, w := range ln.words {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(w.text)
	}
	return strings.TrimSpace(b.String())
}

// section is one detected statement section: a contiguous run of lines
// (spanning one or more pages) believed to belong to a single financial
// statement.
type section struct {
	lines []line
	// titleEvidence is set when a title signal anchored this section's
	// start, for Metadata.StatementTypeEvidence provenance; empty when the
	// section is the implicit whole-document fallback (see splitSections).
	titleEvidence string
}

// splitSections partitions lines into one or more sections using
// findTitleSignals as the deterministic boundary signal (item 16 of the
// ingestion/pdf task contract: page titles / statement-type keywords).
// Consecutive signals for the SAME statement type are treated as a
// repeated header (e.g. the title reprinted at the top of every page of a
// multi-page income statement), not a new section boundary — only a
// signal for a DIFFERENT statement type than the currently-open section
// starts a new one. A large vertical Y-gap between consecutive lines on
// the same page is also a boundary signal (a big blank gap conventionally
// separates one statement from the next when two are laid out on the same
// physical page), independent of any title match, via
// largeVerticalGapBoundaries.
//
// If no title signals are found anywhere, the whole document is returned
// as a single section (matching CSV/XLSX's existing behavior when no
// statement-type title row exists at all — detection falls through to
// BuildResult's own line-item-label tier).
func splitSections(lines []line, rowYTolerance float64) ([]section, []ingestion.Warning) {
	signals := findTitleSignals(lines)
	gapBoundaries := largeVerticalGapBoundaries(lines, rowYTolerance)

	var boundaries []sectionBoundary
	var lastType tabular.StatementType
	for i, sig := range signals {
		if i == 0 || sig.statement != lastType {
			boundaries = append(boundaries, sectionBoundary{lineIndex: sig.lineIndex, evidence: sig.evidence})
		}
		lastType = sig.statement
	}

	var ambiguousGapCount int
	for _, gb := range gapBoundaries {
		// A gap boundary only matters if it doesn't already coincide with
		// (or immediately follow) a title-signal boundary — otherwise it
		// would create a spurious empty/duplicate section right next to a
		// real one.
		tooClose := false
		for _, b := range boundaries {
			if abs(gb-b.lineIndex) <= 1 {
				tooClose = true
				break
			}
		}
		if tooClose {
			continue
		}
		// A gap unconfirmed by any title/keyword signal is split on
		// anyway (a large vertical gap is itself one of this package's
		// deterministic boundary signals, per the ingestion/pdf task
		// contract), but is genuinely less certain than a title-confirmed
		// boundary — no keyword evidence exists to say the content on
		// either side is a DIFFERENT statement type versus merely a
		// visually separated section of the same one — so it is flagged
		// via WarnStatementBoundaryAmbiguous rather than silently treated
		// as equally confident.
		boundaries = append(boundaries, sectionBoundary{lineIndex: gb, evidence: "large vertical gap"})
		ambiguousGapCount++
	}

	if len(boundaries) == 0 {
		return []section{{lines: lines}}, nil
	}

	sortBoundaries(boundaries)

	var warnings []ingestion.Warning
	if len(boundaries) > 1 {
		warnings = append(warnings, ingestion.Warning{
			Code:     ingestion.WarnMultipleStatementsDetected,
			Message:  "multiple statement sections detected in this PDF document; each is returned as a separate result",
			RowIndex: -1,
		})
	}
	if ambiguousGapCount > 0 {
		warnings = append(warnings, ingestion.Warning{
			Code:     ingestion.WarnStatementBoundaryAmbiguous,
			Message:  "a large vertical gap was treated as a statement section boundary with no confirming title/keyword signal on either side; the split may not reflect a genuinely different statement",
			RowIndex: -1,
		})
	}

	// Lines before the first detected title signal (e.g. a company-name
	// line preceding the actual "Income Statement" title line, which is
	// itself the thing findTitleSignals matched) are folded into the FIRST
	// section rather than becoming their own section: on every fixture and
	// realistic statement layout, this leading text is preamble belonging
	// to the section that follows it, not an independent statement — only
	// a signal for a genuinely DIFFERENT statement type (handled by the
	// loop below) or a large vertical gap ever starts a new section.
	var sections []section
	for i, b := range boundaries {
		start := b.lineIndex
		if i == 0 {
			start = 0
		}
		end := len(lines)
		if i+1 < len(boundaries) {
			end = boundaries[i+1].lineIndex
		}
		if start >= end {
			continue
		}
		sections = append(sections, section{lines: lines[start:end], titleEvidence: b.evidence})
	}

	return sections, warnings
}

// largeVerticalGapFactor, multiplied by the document's dominant font size,
// is the minimum Y gap between two consecutive lines ON THE SAME PAGE that
// is treated as a candidate section boundary. 8.0 (800% of font size) is
// intentionally large — well beyond ordinary paragraph/section spacing
// within one statement (typically 1.5-3x line height), so this only fires
// on a genuinely large deliberate separation, matching the task contract's
// "large vertical gaps" boundary signal.
const largeVerticalGapFactor = 8.0

// largeVerticalGapBoundaries returns the line indices where a candidate
// section boundary exists due to unusually large vertical spacing from the
// previous line on the same page. A page break itself is NOT treated as a
// gap boundary (a single statement routinely spans multiple pages — see
// the package doc comment's multi-page behavior), only an unusually large
// gap WITHIN a page's own flow.
func largeVerticalGapBoundaries(lines []line, rowYTolerance float64) []int {
	if len(lines) < 2 {
		return nil
	}
	fontSize := dominantFontSizeFromLines(lines)
	threshold := fontSize * largeVerticalGapFactor

	var boundaries []int
	for i := 1; i < len(lines); i++ {
		if lines[i].pageIndex != lines[i-1].pageIndex {
			continue
		}
		gap := lines[i-1].y - lines[i].y
		if gap > threshold+rowYTolerance {
			boundaries = append(boundaries, i)
		}
	}
	return boundaries
}

func dominantFontSizeFromLines(lines []line) float64 {
	return dominantFontSize(lines)
}

// sectionBoundary is one candidate statement-section start, at lineIndex
// (into the document-order lines slice splitSections was called with),
// with a human-readable evidence string for provenance.
type sectionBoundary struct {
	lineIndex int
	evidence  string
}

func sortBoundaries(b []sectionBoundary) {
	for i := 1; i < len(b); i++ {
		j := i
		for j > 0 && b[j].lineIndex < b[j-1].lineIndex {
			b[j], b[j-1] = b[j-1], b[j]
			j--
		}
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
