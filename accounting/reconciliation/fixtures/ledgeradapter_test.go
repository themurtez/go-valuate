package fixtures

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/ledger"
	"github.com/themurtez/go-valuate/accounting/reconciliation"
)

// TestLedgerAdapter_ConvertsCashAccountActivity confirms
// BookItemsFromLedger correctly extracts ServiceBusiness's real Cash
// account (ID "1000") postings, preserving EntryID-derived provenance
// and excluding draft/voided entries by default.
func TestLedgerAdapter_ConvertsCashAccountActivity(t *testing.T) {
	items := BankBookItems()
	if len(items) != 4 {
		t.Fatalf("expected 4 Cash-account lines (JE-1, JE-3, JE-4, JE-5), got %d: %+v", len(items), items)
	}
	var total float64
	for _, it := range items {
		total += it.SignedAmount()
	}
	// +50000 (JE-1) -8000 (JE-3) -3000 (JE-4) +12000 (JE-5) = 51000
	if total != 51000 {
		t.Fatalf("expected net signed total 51000 matching ServiceBusiness's Cash ending balance, got %v", total)
	}
	for _, it := range items {
		if it.SourceType != "ledger" {
			t.Errorf("expected source_type 'ledger', got %q", it.SourceType)
		}
		if it.SourceRef.System != "ledger" {
			t.Errorf("expected source_ref.system 'ledger', got %q", it.SourceRef.System)
		}
	}
}

// TestLedgerAdapter_LiabilityOrientation confirms
// ledgerLineDirectionAndAmount correctly maps a CREDIT-normal account
// (LIABILITY) so a credit line (increasing what is owed) becomes
// DirectionInflow, not DirectionOutflow — task section 33's "do not
// assume cash-asset semantics for liabilities."
func TestLedgerAdapter_LiabilityOrientation(t *testing.T) {
	chart := ledger.BuildChartOfAccounts([]ledger.Account{
		{ID: "2000", Type: ledger.AccountLiability, Currency: "USD", Active: true},
		{ID: "1000", Type: ledger.AccountAsset, Currency: "USD", Active: true},
	})
	entries := []ledger.JournalEntry{
		{
			ID: "JE-1", Date: "2025-01-05", Status: ledger.StatusPosted,
			Lines: []ledger.JournalLine{
				{ID: "L1", AccountID: "1000", Debit: 1000},
				{ID: "L2", AccountID: "2000", Credit: 1000}, // increases the liability
			},
		},
	}
	items := reconciliation.BookItemsFromLedger(entries, chart, reconciliation.LedgerBookItemsOptions{
		AccountID: "2000",
		StartDate: "2025-01-01",
		EndDate:   "2025-01-31",
	})
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Direction != reconciliation.DirectionInflow {
		t.Fatalf("expected DirectionInflow for a credit line increasing a liability balance, got %s", items[0].Direction)
	}
}

// TestLedgerAdapter_ExcludesDraftAndVoidedByDefault confirms the
// exclusion default from task section 32.
func TestLedgerAdapter_ExcludesDraftAndVoidedByDefault(t *testing.T) {
	chart := ledger.BuildChartOfAccounts([]ledger.Account{
		{ID: "1000", Type: ledger.AccountAsset, Currency: "USD", Active: true},
		{ID: "3000", Type: ledger.AccountEquity, Currency: "USD", Active: true},
	})
	entries := []ledger.JournalEntry{
		{
			ID: "JE-DRAFT", Date: "2025-01-05", Status: ledger.StatusDraft,
			Lines: []ledger.JournalLine{
				{AccountID: "1000", Debit: 500},
				{AccountID: "3000", Credit: 500},
			},
		},
		{
			ID: "JE-VOID", Date: "2025-01-06", Status: ledger.StatusVoided,
			Lines: []ledger.JournalLine{
				{AccountID: "1000", Debit: 700},
				{AccountID: "3000", Credit: 700},
			},
		},
	}
	items := reconciliation.BookItemsFromLedger(entries, chart, reconciliation.LedgerBookItemsOptions{
		AccountID: "1000",
		StartDate: "2025-01-01",
		EndDate:   "2025-01-31",
	})
	if len(items) != 0 {
		t.Fatalf("expected draft/voided entries excluded by default, got %+v", items)
	}
}
