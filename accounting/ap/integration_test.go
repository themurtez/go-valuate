package ap_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/ap"
	"github.com/themurtez/go-valuate/accounting/ledger"
	"github.com/themurtez/go-valuate/accounting/statements"
	"github.com/themurtez/go-valuate/financial"
)

// TestIntegration_StatementBuilderAPBalanceMatchesAgingTotal proves the
// invariant task section 18 requires: when both accounting/statements and
// accounting/ap are built from the same consistent synthetic data, the
// statement builder's CodeBsAccountsPayable balance for a period equals
// this package's TotalOpenPayables for a matching open-item list. This is
// an invariant test, not hard coupling — accounting/ap never imports
// accounting/statements or accounting/ledger; it only reconciles against
// statements' own output here, in the test. Mirrors
// accounting/ar/integration_test.go's identical pattern.
func TestIntegration_StatementBuilderAPBalanceMatchesAgingTotal(t *testing.T) {
	chart := []ledger.Account{
		{ID: "1000", Name: "Cash", Type: ledger.AccountAsset, Currency: "USD", Active: true},
		{ID: "2000", Name: "Accounts Payable", Type: ledger.AccountLiability, Currency: "USD", Active: true},
		{ID: "3000", Name: "Owner's Equity", Type: ledger.AccountEquity, Currency: "USD", Active: true},
		{ID: "6000", Name: "Professional Fees Expense", Type: ledger.AccountExpense, Currency: "USD", Active: true},
	}
	entries := []ledger.JournalEntry{
		{
			ID: "JE-1", Date: "2025-01-05", Period: "2025-01", Status: ledger.StatusPosted,
			Description: "Owner capital contribution",
			Lines: []ledger.JournalLine{
				{ID: "L1", AccountID: "1000", Debit: 50000},
				{ID: "L2", AccountID: "3000", Credit: 50000},
			},
		},
		{
			ID: "JE-2", Date: "2025-01-10", Period: "2025-01", Status: ledger.StatusPosted,
			Description: "Professional fees bill #501", Reference: "BILL-501",
			Lines: []ledger.JournalLine{
				{ID: "L1", AccountID: "6000", Debit: 9000},
				{ID: "L2", AccountID: "2000", Credit: 9000},
			},
		},
	}
	mappings := []statements.AccountMapping{
		{AccountID: "1000", FinancialCode: financial.CodeBsCash, StatementType: financial.StatementBalanceSheet, Source: statements.MappingSourceExplicit, Confirmed: true},
		{AccountID: "2000", FinancialCode: financial.CodeBsAccountsPayable, StatementType: financial.StatementBalanceSheet, Source: statements.MappingSourceExplicit, Confirmed: true},
		{AccountID: "3000", FinancialCode: financial.CodeBsOwnerEquity, StatementType: financial.StatementBalanceSheet, Source: statements.MappingSourceExplicit, Confirmed: true},
		{AccountID: "6000", FinancialCode: financial.CodeOpexProfessionalFees, StatementType: financial.StatementIncomeStatement, Source: statements.MappingSourceExplicit, Confirmed: true},
	}

	stmtInput := statements.Input{
		Source:    statements.SourceLedger,
		Chart:     chart,
		Entries:   entries,
		Periods:   []financial.Period{"2025-01"},
		Mappings:  mappings,
		Selection: statements.SelectionBalanceOnly,
	}
	stmtResult := statements.Build(stmtInput, statements.Options{})

	apItem, ok := stmtResult.Dataset.ByCodeAndPeriod(financial.CodeBsAccountsPayable, "2025-01")
	if !ok {
		t.Fatalf("expected CodeBsAccountsPayable to be present in the built dataset, issues: %+v", stmtResult.Issues)
	}
	if apItem.Amount != 9000 {
		t.Fatalf("sanity check failed: statement builder AP balance = %v, want 9000", apItem.Amount)
	}

	// The same underlying fact (one $9,000 professional fees bill, posted
	// 2025-01-10, still fully open at period end) expressed as an
	// accounting/ap Payable.
	payables := []ap.Payable{
		bill("BILL-501", "SUP-OFFICE", mustDate(t, "2025-01-10"), mustDate(t, "2025-02-09"), 9000, 9000, ap.StatusOpen),
	}
	asOf := mustDate(t, "2025-01-31")
	apResult := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf})

	if !apResult.Available {
		t.Fatalf("expected ap.Result.Available=true, issues: %+v", apResult.Issues)
	}
	if apResult.PortfolioSummary.TotalOpenPayables != apItem.Amount {
		t.Errorf("AP aging total (%v) does not match statement builder AP balance (%v)",
			apResult.PortfolioSummary.TotalOpenPayables, apItem.Amount)
	}
}
