package classification

import "github.com/themurtez/go-valuate/financial"

// structuralTotalTokens are whole-word tokens whose presence in a normalized
// label indicates the row is a calculated total/subtotal rather than an
// ordinary account (e.g. "Total Operating Expenses", "Subtotal COGS", "Net
// Income"). Matching is whole-word only (see containsToken) so this never
// fires on an ordinary account whose name happens to contain one of these
// letter sequences as a substring.
var structuralTotalTokens = []string{"total", "subtotal", "net"}

// detectStructuralStatus determines whether a row is structural (heading,
// subtotal, or total) rather than an ordinary account line, and if so what
// financial.RowStatus it should carry.
//
// It checks raw.Kind FIRST: when the row's originating adapter (ingestion,
// or any other caller) already supplied a non-zero RowKind, that upstream
// read wins outright and this function never falls back to its own label
// heuristic for that row. This closes a known gap where ingestion's
// broader label-shape detection (ingestion/internal/tabular.ClassifyRowKind,
// which recognizes e.g. "Gross Profit" as a subtotal) disagreed with this
// package's narrower total/subtotal/net token check — see the README's
// "Known deterministic ingestion gaps" section for the case history.
//
// Only when raw.Kind is the zero value (RowKindNormal, meaning "no upstream
// structural signal supplied") does this function fall back to its own
// label heuristic, exactly as it always has — this is what preserves
// existing behavior for hand-built RawLineItem values (including most
// existing tests) and for any row an adapter itself judged RowKindNormal.
//
// The label heuristic: "Total X" style labels that summarize a group (e.g.
// "Total Operating Expenses", "Total COGS") are treated as subtotals; a
// label that is exactly a bare total of the whole statement (e.g. "Total
// Expenses", "Total Revenue", "Net Income") is treated as a total. The
// distinction mirrors financial.RowStatusSubtotal vs financial.RowStatusTotal,
// both of which financial.Normalize excludes from aggregation identically,
// so the two are useful primarily for downstream display rather than
// aggregation correctness.
func detectStructuralStatus(label NormalizedLabel, kind financial.RowKind) (financial.RowStatus, bool) {
	switch kind {
	case financial.RowKindHeading:
		return financial.RowStatusIgnored, true
	case financial.RowKindSubtotal:
		return financial.RowStatusSubtotal, true
	case financial.RowKindTotal:
		return financial.RowStatusTotal, true
	}

	token, ok := containsAnyToken(label.Comparable, structuralTotalTokens...)
	if !ok {
		return "", false
	}

	switch token {
	case "total", "subtotal":
		if containsToken(label.Comparable, "total") && !isBareStatementTotal(label.Comparable) {
			return financial.RowStatusSubtotal, true
		}
		return financial.RowStatusTotal, true
	case "net":
		return financial.RowStatusTotal, true
	}
	return "", false
}

// bareStatementTotalPhrases are normalized labels that summarize an entire
// statement (as opposed to summarizing one section/group within it) and
// should therefore be classified as a grand total rather than a subtotal.
var bareStatementTotalPhrases = []string{
	"total revenue",
	"total expenses",
	"total income",
}

func isBareStatementTotal(comparable string) bool {
	for _, phrase := range bareStatementTotalPhrases {
		if comparable == phrase {
			return true
		}
	}
	return false
}
