// Multi-page repeated-content suppression: repeated page headers/column
// headings must not become duplicate financial rows, and page
// footers/page numbers should be dropped where deterministically
// identifiable — both per the ingestion/pdf task contract's multi-page
// behavior requirements.
package pdf

import (
	"regexp"
	"strings"
)

// suppressRepeatedContent removes lines from a single section's lines that
// are near-exact repeats of an earlier line in the SAME section (a
// multi-page statement's title/column-header block reprinted at the top of
// every page is the common case) or that look like a page
// footer/page-number line, returning the filtered lines plus one warning
// per suppressed repeated-header occurrence (WarnRepeatedHeaderRemoved).
// Page footers are dropped silently (no warning) since they were never
// financial content in the first place — unlike a repeated header, which
// WAS legitimate content on its first occurrence.
//
// A line only counts as "repeated" if it matches an EARLIER line's
// reconstructed text exactly (after whitespace normalization) — this is
// deliberately conservative: an ordinary data row is never suppressed
// merely for looking similar to another row, only for being byte-for-byte
// identical to one already seen once in this section (a real statement
// line, unlike a header, essentially never repeats verbatim across pages).
func suppressRepeatedContent(lines []line, pageHeights map[int]pageExtent) ([]line, []repeatedHeaderHit) {
	seen := make(map[string]bool)
	var out []line
	var hits []repeatedHeaderHit

	for _, ln := range lines {
		text := normalizeForRepeatCompare(lineText(ln))
		if text == "" {
			continue
		}
		if looksLikePageFooter(ln, pageHeights) {
			continue
		}
		if seen[text] {
			hits = append(hits, repeatedHeaderHit{pageIndex: ln.pageIndex, text: text})
			continue
		}
		seen[text] = true
		out = append(out, ln)
	}
	return out, hits
}

// repeatedHeaderHit records one suppressed repeated-header line, for
// WarnRepeatedHeaderRemoved warning generation.
type repeatedHeaderHit struct {
	pageIndex int
	text      string
}

var repeatWhitespace = regexp.MustCompile(`\s+`)

func normalizeForRepeatCompare(s string) string {
	return strings.ToLower(repeatWhitespace.ReplaceAllString(strings.TrimSpace(s), " "))
}

// pageExtent is a page's observed Y range across every line extracted from
// it, used only to judge whether a candidate line sits in the bottom
// margin (a footer position) of its page.
type pageExtent struct {
	minY, maxY float64
}

// computePageExtents derives each page's observed Y range from lines,
// before any suppression — used by looksLikePageFooter to judge relative
// vertical position (a page's true printable bottom margin is generally
// unknown without parsing /MediaBox, but the lowest Y any real content
// reaches on that page is a reasonable, fully-deterministic proxy: a line
// noticeably below all other content on its page, that ALSO looks
// footer-shaped by text pattern, is treated as a footer).
func computePageExtents(lines []line) map[int]pageExtent {
	extents := make(map[int]pageExtent)
	for _, ln := range lines {
		e, ok := extents[ln.pageIndex]
		if !ok {
			e = pageExtent{minY: ln.y, maxY: ln.y}
		}
		if ln.y < e.minY {
			e.minY = ln.y
		}
		if ln.y > e.maxY {
			e.maxY = ln.y
		}
		extents[ln.pageIndex] = e
	}
	return extents
}

// footerMarginFactor: a line within this fraction of its page's own
// observed Y range (from computePageExtents) of the page's minimum Y is a
// candidate footer position. 0.05 (bottom 5% of the page's observed
// content range) is deliberately tight, since it is only a SECONDARY
// signal — looksLikePageFooter also requires the line's TEXT to match a
// footer-shaped pattern (see reFooterPattern); position alone is never
// sufficient (a statement's last real line, e.g. "Net Income", legitimately
// sits at the bottom of its page too).
const footerMarginFactor = 0.05

// reFooterPattern matches deterministically footer-shaped text: a bare
// page number ("3", "Page 3", "Page 3 of 12", "- 3 -") or nothing else —
// never a pattern broad enough to match a real financial line, since a
// dollar amount or an account label never matches any of these forms.
var reFooterPattern = regexp.MustCompile(`(?i)^(page\s+\d+(\s+of\s+\d+)?|-?\s*\d{1,4}\s*-?)$`)

// looksLikePageFooter reports whether ln is both text-shaped like a page
// footer/page-number AND positioned in the bottom margin of its page,
// relative to that page's own observed content range.
func looksLikePageFooter(ln line, extents map[int]pageExtent) bool {
	text := strings.TrimSpace(lineText(ln))
	if !reFooterPattern.MatchString(text) {
		return false
	}
	e, ok := extents[ln.pageIndex]
	if !ok {
		return false
	}
	span := e.maxY - e.minY
	if span <= 0 {
		return true // a single-line page where that line is footer-shaped
	}
	return ln.y-e.minY <= span*footerMarginFactor
}
