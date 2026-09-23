package ar_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/ar"
	"github.com/themurtez/go-valuate/accounting/ledger"
	"github.com/themurtez/go-valuate/accounting/statements"
	"github.com/themurtez/go-valuate/financial"
)

// TestIntegration_StatementBuilderARBalanceMatchesAgingTotal proves the
// invariant section 29 requires: when both accounting/statements and
// accounting/ar are built from the same consistent synthetic data, the
// statement builder's CodeBsAccountsReceivable balance for a period equals
// this package's TotalOpenReceivables for a matching open-item list. This
// is an invariant test, not hard coupling — accounting/ar never imports
// accounting/statements or accounting/ledger; it only reconciles against
// statements' own output here, in the test.
//
// A minimal, self-contained ledger is used (rather than
// accounting/ledger/fixtures.ServiceBusinessChart/Entries) because that
// fixture's own invoice is fully collected within the same period (see its
// "SVC-JE-5" cash-collection entry), leaving a zero period-end AR balance
// — the wrong shape for an aging-total invariant, which needs a receivable
// still open at AsOfDate.
func TestIntegration_StatementBuilderARBalanceMatchesAgingTotal(t *testing.T) {
	chart := []ledger.Account{
		{ID: "1000", Name: "Cash", Type: ledger.AccountAsset, Currency: "USD", Active: true},
		{ID: "1100", Name: "Accounts Receivable", Type: ledger.AccountAsset, Currency: "USD", Active: true},
		{ID: "3000", Name: "Owner's Equity", Type: ledger.AccountEquity, Currency: "USD", Active: true},
		{ID: "4000", Name: "Consulting Revenue", Type: ledger.AccountRevenue, Currency: "USD", Active: true},
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
			Description: "Consulting invoice #101", Reference: "INV-101",
			Lines: []ledger.JournalLine{
				{ID: "L1", AccountID: "1100", Debit: 12000},
				{ID: "L2", AccountID: "4000", Credit: 12000},
			},
		},
	}
	mappings := []statements.AccountMapping{
		{AccountID: "1000", FinancialCode: financial.CodeBsCash, StatementType: financial.StatementBalanceSheet, Source: statements.MappingSourceExplicit, Confirmed: true},
		{AccountID: "1100", FinancialCode: financial.CodeBsAccountsReceivable, StatementType: financial.StatementBalanceSheet, Source: statements.MappingSourceExplicit, Confirmed: true},
		{AccountID: "3000", FinancialCode: financial.CodeBsOwnerEquity, StatementType: financial.StatementBalanceSheet, Source: statements.MappingSourceExplicit, Confirmed: true},
		{AccountID: "4000", FinancialCode: financial.CodeRevService, StatementType: financial.StatementIncomeStatement, Source: statements.MappingSourceExplicit, Confirmed: true},
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

	arItem, ok := stmtResult.Dataset.ByCodeAndPeriod(financial.CodeBsAccountsReceivable, "2025-01")
	if !ok {
		t.Fatalf("expected CodeBsAccountsReceivable to be present in the built dataset, issues: %+v", stmtResult.Issues)
	}
	if arItem.Amount != 12000 {
		t.Fatalf("sanity check failed: statement builder AR balance = %v, want 12000", arItem.Amount)
	}

	// The same underlying fact (one $12,000 consulting invoice, posted
	// 2025-01-10, still fully open at period end) expressed as an
	// accounting/ar Receivable.
	receivables := []ar.Receivable{
		recv("INV-101", "CUST-SVC", mustDate(t, "2025-01-10"), mustDate(t, "2025-02-09"), 12000, 12000, ar.StatusOpen),
	}
	asOf := mustDate(t, "2025-01-31")
	arResult := ar.Calculate(ar.Input{Receivables: receivables}, ar.Options{AsOfDate: asOf})

	if !arResult.Available {
		t.Fatalf("expected ar.Result.Available=true, issues: %+v", arResult.Issues)
	}
	if arResult.PortfolioSummary.TotalOpenReceivables != arItem.Amount {
		t.Errorf("AR aging total (%v) does not match statement builder AR balance (%v)",
			arResult.PortfolioSummary.TotalOpenReceivables, arItem.Amount)
	}
}
