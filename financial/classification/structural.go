package classification

import "github.com/themurtez/go-valuate/financial"

// structuralTotalTokens are whole-word tokens whose presence in a normalized
// label indicates the row is a calculated total/subtotal rather than an
// ordinary account (e.g. "Total Operating Expenses", "Subtotal COGS", "Net
// Income"). Matching is whole-word only (see containsToken) so this never
// fires on an ordinary account whose name happens to contain one of these
// letter sequences as a substring.
var structuralTotalTokens = []string{"total", "subtotal", "net"}

// detectStructuralStatus inspects a normalized label and reports whether the
// row looks like a subtotal or grand-total row rather than an ordinary
// account line. "Total X" style labels that summarize a group (e.g. "Total
// Operating Expenses", "Total COGS") are treated as subtotals; a label that
// is exactly a bare total of the whole statement (e.g. "Total Expenses",
// "Total Revenue", "Net Income") is treated as a total. The distinction
// mirrors financial.RowStatusSubtotal vs financial.RowStatusTotal, both of
// which financial.Normalize excludes from aggregation identically, so the
// two are useful primarily for downstream display rather than aggregation
// correctness.
//
// This is intentionally a coarse heuristic: it only recognizes label text,
// since financial.RawLineItem carries no explicit status field. A caller
// with more reliable source metadata (e.g. an indentation level or a flag
// from the original document) can bypass this by overriding the resulting
// Result.Status directly.
func detectStructuralStatus(label NormalizedLabel) (financial.RowStatus, bool) {
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
