package tabular

import (
	"regexp"
	"strings"
)

// RowKind mirrors ingestion.StructuralKind (see the DashTreatment doc
// comment on this package for the import-cycle rationale).
type RowKind string

const (
	RowNormal   RowKind = "normal"
	RowHeading  RowKind = "heading"
	RowSubtotal RowKind = "subtotal"
	RowTotal    RowKind = "total"
	RowBlank    RowKind = "blank"
)

var (
	reTotalPrefix    = regexp.MustCompile(`(?i)^total\b`)
	reSubtotalPrefix = regexp.MustCompile(`(?i)^sub[\s-]?total\b`)
	reNetPrefix      = regexp.MustCompile(`(?i)^net\s`)
	reGrossProfit    = regexp.MustCompile(`(?i)^gross\s+profit\b`)
)

// bareStatementTotalPhrases are labels that summarize the entire
// statement (as opposed to summarizing one section/group within it) and
// are therefore classified as a grand total (RowTotal) rather than a
// subtotal. "Gross Profit" is deliberately excluded here: it summarizes a
// subset of the statement (revenue less COGS) the same way "Total
// Operating Expenses" summarizes the opex section, so it is handled by
// reGrossProfit as RowSubtotal instead — see ClassifyRowKind.
var bareStatementTotalPhrases = map[string]bool{
	"total revenue":              true,
	"total revenues":             true,
	"total expenses":             true,
	"total expense":              true,
	"total income":               true,
	"total assets":               true,
	"total liabilities":          true,
	"total equity":               true,
	"net income":                 true,
	"net loss":                   true,
	"net profit":                 true,
	"total revenue and expenses": true,
}

// ClassifyRowKind determines a row's structural role from its label text,
// whether it has any numeric values, and whether it's blank. hasValues
// should be true if at least one non-label cell parsed as numeric or was a
// dash; hasAnyText should be true if any cell (label or otherwise) has
// non-empty text.
//
// Rules, in priority order:
//  1. no text anywhere -> RowBlank
//  2. label text present but no numeric/dash values anywhere in the row ->
//     RowHeading (a section header like "Operating Expenses" that
//     introduces a group of following rows, or a title row)
//  3. label starts with "total"/"subtotal" or is a well-known
//     whole-statement total phrase -> RowSubtotal or RowTotal
//  4. otherwise -> RowNormal
func ClassifyRowKind(label string, hasValues, hasAnyText bool) (kind RowKind, uncertain bool) {
	trimmedLabel := strings.TrimSpace(label)
	comparable := strings.ToLower(trimmedLabel)

	if !hasAnyText {
		return RowBlank, false
	}

	if trimmedLabel == "" {
		// A row with numeric values but no label is unusual and
		// structurally ambiguous (could be a continuation row, or a
		// mis-detected label column) — flag it rather than silently
		// treating it as normal.
		return RowNormal, true
	}

	if !hasValues {
		return RowHeading, false
	}

	if bareStatementTotalPhrases[comparable] {
		return RowTotal, false
	}
	if reGrossProfit.MatchString(trimmedLabel) {
		return RowSubtotal, false
	}
	if reSubtotalPrefix.MatchString(trimmedLabel) {
		return RowSubtotal, false
	}
	if reTotalPrefix.MatchString(trimmedLabel) {
		// "Total X" where X is a section name (e.g. "Total Operating
		// Expenses") summarizes a group -> subtotal. A handful of
		// well-known bare "Total X" phrases that summarize the WHOLE
		// statement are already handled above via bareStatementTotalPhrases.
		return RowSubtotal, false
	}
	if reNetPrefix.MatchString(trimmedLabel) {
		return RowTotal, false
	}

	return RowNormal, false
}

// DetectIndentLevel returns a best-effort indentation depth for a raw label
// cell's text, counting leading whitespace: every 2 leading spaces (or one
// leading tab) counts as one indent level. This is intentionally coarse —
// it is one signal among several for parent/section detection, not a
// guarantee of true accounting hierarchy depth (see the ingestion
// contract's parent/section-context section, which explicitly says not to
// over-engineer this).
func DetectIndentLevel(rawLabel string) int {
	units := 0
	for _, r := range rawLabel {
		switch r {
		case '\t':
			units += 2
		case ' ':
			units++
		default:
			return units / 2
		}
	}
	return units / 2
}

// ParentTracker assigns a ParentLabel to each normal/subtotal/total row
// based on the most recent heading row seen at a shallower (or equal? no —
// strictly shallower) indent level, or the most recent heading row at all
// if indentation is not informative (e.g. CSV exports commonly have no
// leading whitespace preserved). It is used by scanning rows in order and
// calling Observe for every row in sequence.
type ParentTracker struct {
	// stack holds (indentLevel, label) pairs for currently-open sections,
	// outermost first.
	stack []parentFrame
}

type parentFrame struct {
	indent int
	label  string
}

// Observe processes one row in document order. For a heading row, it opens
// a new section context. For any other row, it returns the label of the
// innermost currently-open section (or "" if none).
func (t *ParentTracker) Observe(kind RowKind, indent int, label string) string {
	if kind == RowBlank {
		return t.current()
	}

	if kind == RowHeading {
		for len(t.stack) > 0 && t.stack[len(t.stack)-1].indent >= indent {
			t.stack = t.stack[:len(t.stack)-1]
		}
		t.stack = append(t.stack, parentFrame{indent: indent, label: strings.TrimSpace(label)})
		return t.current()
	}

	// A subtotal/total row at or below the current section's indent level
	// closes that section (its "Total X" line marks the end of the group),
	// but the ParentLabel it reports should still be the section it
	// belongs to.
	parent := t.current()
	for len(t.stack) > 0 && indent < t.stack[len(t.stack)-1].indent {
		t.stack = t.stack[:len(t.stack)-1]
	}
	if kind == RowSubtotal || kind == RowTotal {
		if len(t.stack) > 0 && t.stack[len(t.stack)-1].indent >= indent {
			t.stack = t.stack[:len(t.stack)-1]
		}
	}
	return parent
}

func (t *ParentTracker) current() string {
	if len(t.stack) == 0 {
		return ""
	}
	return t.stack[len(t.stack)-1].label
}
