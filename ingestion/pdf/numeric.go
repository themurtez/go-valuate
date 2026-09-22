// PDF-specific numeric extraction quirks: a thin normalization pass ahead
// of ingestion/internal/tabular's existing, already-correct numeric
// parser (tabular.ParseNumeric), never a parallel reimplementation of it
// (per the ingestion/pdf task contract's explicit "reuse, do not
// reimplement" requirement).
//
// PDF text reconstruction (see layout.go's word-grouping) can produce a
// numeric cell with internal spaces that a CSV/XLSX cell never would,
// because a PDF "cell" is reconstructed from multiple independently
// positioned glyph runs rather than read as one already-whole string:
//   - "$ 1,234.00"   — a currency symbol drawn as a separate glyph run
//     with its own small gap before the digits
//   - "( 1,234 )"    — parenthesis glyphs likewise drawn separately, with
//     a gap wide enough to have been treated as a word boundary
//   - "1,234 -"       — a TRAILING minus sign (the accounting/European
//     convention where the sign follows the digits, e.g. "1.234,00-"),
//     drawn with a gap between the number and the dash glyph. This is
//     distinct from a BARE dash cell ("-" alone, meaning zero/no-value
//     per Options.DashTreatment): tabular.ParseNumeric already handles a
//     bare dash correctly on its own without any help from this package,
//     since a bare-dash cell was never split into two words in the first
//     place (nothing precedes it to create a word-boundary gap). This
//     rule only fires when a dash follows an actual number.
//
// normalizePDFNumericText rewrites the trailing-minus case into the
// leading-minus form tabular.ParseNumeric already understands, and
// collapses the other two spacing-only cases, before handing the result
// to tabular.ParseNumeric unchanged (never a bare "1 234" mid-number
// space — see below).
package pdf

import (
	"regexp"
	"strings"

	"github.com/themurtez/go-valuate/ingestion/internal/tabular"
)

var (
	// reSpacedCurrencyPrefix matches a currency symbol separated from the
	// following digits by whitespace ("$ 1,234.00", "$  1,234").
	reSpacedCurrencyPrefix = regexp.MustCompile(`^([$])\s+`)
	// reSpacedParens matches parentheses with internal whitespace before
	// the enclosed value and/or after it ("( 1,234 )", "(1,234 )").
	reSpacedOpenParen  = regexp.MustCompile(`^\(\s+`)
	reSpacedCloseParen = regexp.MustCompile(`\s+\)$`)
	// reTrailingMinusSpace matches one or more DIGITS followed by
	// whitespace then a bare dash/em-dash at the end of the cell
	// ("1,234 -", "1,234.00  —"), i.e. a trailing minus sign separated
	// from the number by a word-boundary-sized gap. Requiring a digit
	// immediately before the whitespace (rather than matching any
	// preceding text) ensures this never fires on an unrelated trailing
	// dash after non-numeric text.
	reTrailingMinusSpace = regexp.MustCompile(`(\d)\s+(-|–|—)$`)
	// reMidNumberSpaceThousands matches a European-style space thousands
	// separator strictly BETWEEN two runs of 1-3 and 3 digits respectively
	// ("1 234", "12 345 678"), anchored so it never matches ordinary
	// running text that merely contains a number followed by unrelated
	// words (e.g. "1 234 Main Street" is NOT touched, because the anchors
	// require the WHOLE cell to be nothing but digit groups separated by
	// single spaces, optionally with a decimal tail).
	reMidNumberSpaceThousands = regexp.MustCompile(`^\d{1,3}( \d{3})+(\.\d+)?$`)
)

// normalizePDFNumericText rewrites raw into the tighter form
// tabular.ParseNumeric already understands, handling exactly the
// deterministic, unambiguous PDF-extraction spacing quirks documented on
// this file — anything else is left completely untouched and falls
// through to tabular.ParseNumeric's own (correct, unmodified) handling, so
// this function can never mask a genuinely malformed value: it only ever
// removes whitespace matched by one of the explicit patterns above, never
// alters digits/sign/decimal content.
func normalizePDFNumericText(raw string) string {
	s := raw

	if reMidNumberSpaceThousands.MatchString(strings.TrimSpace(s)) {
		s = strings.ReplaceAll(strings.TrimSpace(s), " ", "")
	}

	s = reSpacedCurrencyPrefix.ReplaceAllString(s, "$")
	s = reSpacedOpenParen.ReplaceAllString(s, "(")
	s = reSpacedCloseParen.ReplaceAllString(s, ")")

	if reTrailingMinusSpace.MatchString(s) {
		s = reTrailingMinusSpace.ReplaceAllString(s, "$1")
		s = "-" + strings.TrimSpace(s)
	}

	return s
}

// parsePDFNumeric normalizes raw for known PDF extraction quirks and
// parses it via tabular.ParseNumeric — the single call site every PDF
// numeric cell goes through, so ingestion/pdf's own numeric behavior can
// never drift from CSV/XLSX's for any input the normalization pass leaves
// unchanged (the overwhelming majority of cells: normalizePDFNumericText
// is a no-op on any already-well-formed value).
func parsePDFNumeric(raw string, dash tabular.DashTreatment) tabular.NumericResult {
	return tabular.ParseNumeric(normalizePDFNumericText(raw), dash)
}
