package inventory_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/inventory"
	"github.com/themurtez/go-valuate/accounting/ledger"
	"github.com/themurtez/go-valuate/accounting/statements"
	"github.com/themurtez/go-valuate/financial"
)

// TestAdapter_StatementBuilderInventoryBalanceMatchesSubledgerTotal proves
// the invariant task section 39 requires: when both accounting/statements
// and accounting/inventory are built from the same consistent synthetic
// facts, the statement builder's CodeBsInventory balance for a period
// equals this package's PortfolioSummary.TotalInventoryValue for a
// matching as-of snapshot. This is an invariant test, not hard coupling —
// accounting/inventory never imports accounting/statements or
// accounting/ledger; it only reconciles against statements' own output
// here, in the test. Mirrors accounting/ap's/ar's identical
// statement-builder integration-test pattern.
func TestAdapter_StatementBuilderInventoryBalanceMatchesSubledgerTotal(t *testing.T) {
	chart := []ledger.Account{
		{ID: "1000", Name: "Cash", Type: ledger.AccountAsset, Currency: "USD", Active: true},
		{ID: "1200", Name: "Inventory", Type: ledger.AccountAsset, Currency: "USD", Active: true},
		{ID: "3000", Name: "Owner's Equity", Type: ledger.AccountEquity, Currency: "USD", Active: true},
	}
	entries := []ledger.JournalEntry{
		{
			ID: "JE-1", Date: "2025-01-02", Period: "2025-01", Status: ledger.StatusPosted,
			Description: "Owner capital contribution",
			Lines: []ledger.JournalLine{
				{ID: "L1", AccountID: "1000", Debit: 50000},
				{ID: "L2", AccountID: "3000", Credit: 50000},
			},
		},
		{
			ID: "JE-2", Date: "2025-01-10", Period: "2025-01", Status: ledger.StatusPosted,
			Description: "Inventory purchase, paid in cash", Reference: "PO-100",
			Lines: []ledger.JournalLine{
				{ID: "L1", AccountID: "1200", Debit: 12000},
				{ID: "L2", AccountID: "1000", Credit: 12000},
			},
		},
	}
	mappings := []statements.AccountMapping{
		{AccountID: "1000", FinancialCode: financial.CodeBsCash, StatementType: financial.StatementBalanceSheet, Source: statements.MappingSourceExplicit, Confirmed: true},
		{AccountID: "1200", FinancialCode: financial.CodeBsInventory, StatementType: financial.StatementBalanceSheet, Source: statements.MappingSourceExplicit, Confirmed: true},
		{AccountID: "3000", FinancialCode: financial.CodeBsOwnerEquity, StatementType: financial.StatementBalanceSheet, Source: statements.MappingSourceExplicit, Confirmed: true},
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

	invItem, ok := stmtResult.Dataset.ByCodeAndPeriod(financial.CodeBsInventory, "2025-01")
	if !ok {
		t.Fatalf("expected CodeBsInventory to be present in the built dataset, issues: %+v", stmtResult.Issues)
	}
	if invItem.Amount != 12000 {
		t.Fatalf("sanity check failed: statement builder inventory balance = %v, want 12000", invItem.Amount)
	}

	// The same underlying fact ($12,000 of inventory purchased 2025-01-10,
	// still fully on hand at period end) expressed as an
	// accounting/inventory snapshot.
	in := inventory.Input{
		AsOfDate: "2025-01-31",
		Items:    []inventory.Item{item("ITEM-1", "Merchandise")},
		Snapshots: []inventory.InventorySnapshot{
			{ID: "SNAP-1", ItemID: "ITEM-1", AsOfDate: mustDate(t, "2025-01-31"),
				InventoryValue: inventory.AvailableValue(12000), Currency: "USD"},
		},
	}
	invResult := inventory.Calculate(in, inventory.Policy{})
	if !invResult.Available {
		t.Fatalf("expected inventory.Result.Available=true, issues: %+v", invResult.Issues)
	}
	if invResult.Portfolio.TotalInventoryValue != invItem.Amount {
		t.Errorf("inventory subledger total (%v) does not match statement builder inventory balance (%v)",
			invResult.Portfolio.TotalInventoryValue, invItem.Amount)
	}
}
