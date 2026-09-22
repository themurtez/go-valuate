package tabular

import "testing"

func TestDetectStatementTypeBySheetName(t *testing.T) {
	got := DetectStatementType("Balance Sheet", nil, nil)
	if got.Type != StatementBalanceSheet {
		t.Errorf("Type = %q, want %q", got.Type, StatementBalanceSheet)
	}
}

func TestDetectStatementTypeByTitleRow(t *testing.T) {
	got := DetectStatementType("Sheet1", []string{"Acme Corp", "Income Statement", "For the Year Ended 12/31/2024"}, nil)
	if got.Type != StatementIncomeStatement {
		t.Errorf("Type = %q, want %q", got.Type, StatementIncomeStatement)
	}
}

func TestDetectStatementTypeByLineItems(t *testing.T) {
	got := DetectStatementType("Sheet1", nil, []string{"Revenue", "COGS", "Gross Profit", "Operating Expenses", "Net Income"})
	if got.Type != StatementIncomeStatement {
		t.Errorf("Type = %q, want %q", got.Type, StatementIncomeStatement)
	}
}

func TestDetectStatementTypeBalanceSheetLineItems(t *testing.T) {
	got := DetectStatementType("Sheet1", nil, []string{"Cash", "Accounts Receivable", "Total Assets", "Accounts Payable", "Total Liabilities", "Retained Earnings"})
	if got.Type != StatementBalanceSheet {
		t.Errorf("Type = %q, want %q", got.Type, StatementBalanceSheet)
	}
}

func TestDetectStatementTypeCashFlow(t *testing.T) {
	got := DetectStatementType("Cash Flow Statement", nil, nil)
	if got.Type != StatementCashFlow {
		t.Errorf("Type = %q, want %q", got.Type, StatementCashFlow)
	}
}

func TestDetectStatementTypeUnknownOnWeakSignal(t *testing.T) {
	got := DetectStatementType("Sheet1", nil, []string{"Miscellaneous", "Notes", "Other Stuff"})
	if got.Type != StatementUnknown {
		t.Errorf("Type = %q, want StatementUnknown for weak/no signal", got.Type)
	}
}

func TestDetectStatementTypeSheetNameTakesPriorityOverLineItems(t *testing.T) {
	// Sheet name says balance sheet; line items are ambiguous/mixed. Sheet
	// name should win since it's checked first.
	got := DetectStatementType("Balance Sheet", nil, []string{"Revenue", "Total Assets"})
	if got.Type != StatementBalanceSheet {
		t.Errorf("Type = %q, want %q (sheet name should take priority)", got.Type, StatementBalanceSheet)
	}
}

func TestDetectStatementTypeNeverForcesGuessOnTie(t *testing.T) {
	// Equal signal strength for two statement types via line items alone
	// (no sheet name, no title) should resolve to Unknown rather than
	// picking one arbitrarily.
	got := DetectStatementType("", nil, []string{"Gross Profit", "Total Assets"})
	if got.Type != StatementUnknown {
		t.Errorf("Type = %q, want StatementUnknown on a tie", got.Type)
	}
}
