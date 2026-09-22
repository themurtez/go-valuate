package tabular

import "strings"

// StatementType mirrors financial.StatementType's three string values plus
// an explicit "unknown", without importing the financial package (kept
// dependency-free/internal so this package can be reused identically by
// csv and xlsx without a cycle risk).
type StatementType string

const (
	StatementIncomeStatement StatementType = "income_statement"
	StatementBalanceSheet    StatementType = "balance_sheet"
	StatementCashFlow        StatementType = "cash_flow"
	StatementUnknown         StatementType = ""
)

// StatementDetection is the outcome of attempting to determine which
// financial statement a grid represents.
type StatementDetection struct {
	Type     StatementType
	Evidence string
}

// incomeStatementSignals are phrases that strongly indicate an income
// statement / profit & loss, checked against sheet names, title rows, and
// distinctive structural line-item labels (e.g. "Gross Profit" appears in
// essentially no other statement type).
var incomeStatementSignals = []string{
	"income statement", "profit and loss", "profit & loss", "p&l", "p & l",
	"statement of operations", "revenue and expense",
}

var balanceSheetSignals = []string{
	"balance sheet", "statement of financial position",
}

var cashFlowSignals = []string{
	"cash flow statement", "statement of cash flows", "cash flow",
}

// incomeStatementLineItemSignals are structural line-item labels distinctive
// enough to an income statement that seeing them among a grid's row labels
// is itself good evidence, even absent a title row naming the statement.
var incomeStatementLineItemSignals = []string{
	"gross profit", "cost of goods sold", "cost of sales", "operating income", "net income", "ebitda", "total revenue", "total expenses",
}

var balanceSheetLineItemSignals = []string{
	"total assets", "total liabilities", "accounts payable", "accounts receivable", "retained earnings", "stockholders equity", "shareholders equity", "current assets", "current liabilities",
}

var cashFlowLineItemSignals = []string{
	"cash flow from operations", "cash flows from operating", "cash flows from investing", "cash flows from financing", "net change in cash", "net increase in cash",
}

// DetectStatementType applies deterministic signal matching, in descending
// priority: sheetName, then title/header rows (the first few non-blank
// rows of the grid, before the detected header row), then row labels
// throughout the grid. The first tier that produces an unambiguous match
// wins; if signals from more than one statement type are found at the same
// tier, or no tier matches, the result is StatementUnknown — detection
// never forces a guess under weak/conflicting evidence (see the ingestion
// contract's statement-type-detection section).
func DetectStatementType(sheetName string, titleRows []string, rowLabels []string) StatementDetection {
	if t, ev, ok := matchSignals(strings.ToLower(sheetName)); ok {
		return StatementDetection{Type: t, Evidence: "sheet name: " + ev}
	}

	for _, row := range titleRows {
		if t, ev, ok := matchSignals(strings.ToLower(row)); ok {
			return StatementDetection{Type: t, Evidence: "title row: " + ev}
		}
	}

	joined := strings.ToLower(strings.Join(rowLabels, " | "))
	counts := map[StatementType]int{}
	var evidence map[StatementType]string = map[StatementType]string{}
	for _, sig := range incomeStatementLineItemSignals {
		if strings.Contains(joined, sig) {
			counts[StatementIncomeStatement]++
			if evidence[StatementIncomeStatement] == "" {
				evidence[StatementIncomeStatement] = sig
			}
		}
	}
	for _, sig := range balanceSheetLineItemSignals {
		if strings.Contains(joined, sig) {
			counts[StatementBalanceSheet]++
			if evidence[StatementBalanceSheet] == "" {
				evidence[StatementBalanceSheet] = sig
			}
		}
	}
	for _, sig := range cashFlowLineItemSignals {
		if strings.Contains(joined, sig) {
			counts[StatementCashFlow]++
			if evidence[StatementCashFlow] == "" {
				evidence[StatementCashFlow] = sig
			}
		}
	}

	best := StatementUnknown
	bestCount := 0
	tie := false
	for t, c := range counts {
		if c > bestCount {
			best = t
			bestCount = c
			tie = false
		} else if c == bestCount && c > 0 {
			tie = true
		}
	}
	if bestCount == 0 || tie {
		return StatementDetection{Type: StatementUnknown, Evidence: "no unambiguous statement-type signal found"}
	}
	return StatementDetection{Type: best, Evidence: "line-item labels: found \"" + evidence[best] + "\""}
}

func matchSignals(lower string) (StatementType, string, bool) {
	for _, sig := range incomeStatementSignals {
		if strings.Contains(lower, sig) {
			return StatementIncomeStatement, sig, true
		}
	}
	for _, sig := range balanceSheetSignals {
		if strings.Contains(lower, sig) {
			return StatementBalanceSheet, sig, true
		}
	}
	for _, sig := range cashFlowSignals {
		if strings.Contains(lower, sig) {
			return StatementCashFlow, sig, true
		}
	}
	return StatementUnknown, "", false
}
