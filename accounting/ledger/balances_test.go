package ledger

import "testing"

func TestCalculateBalances_Activity(t *testing.T) {
	entries := []JournalEntry{
		{ID: "JE-1", Date: "2025-01-15", Period: "2025", Status: StatusPosted, Lines: []JournalLine{
			{AccountID: "1000", Debit: 1000},
			{AccountID: "4000", Credit: 1000},
		}},
		{ID: "JE-2", Date: "2025-02-01", Period: "2025", Status: StatusPosted, Lines: []JournalLine{
			{AccountID: "5000", Debit: 200},
			{AccountID: "1000", Credit: 200},
		}},
	}
	balances := CalculateBalances(testChart(), entries, BalanceOptions{})

	cash := findBalance(t, balances, "1000")
	if cash.PeriodDebits != 1000 || cash.PeriodCredits != 200 {
		t.Fatalf("unexpected cash activity: debits=%v credits=%v", cash.PeriodDebits, cash.PeriodCredits)
	}
	if cash.RawBalance != 800 {
		t.Fatalf("expected cash raw balance 800, got %v", cash.RawBalance)
	}
	if cash.DisplayBalance != 800 {
		t.Fatalf("ASSET display balance should equal raw balance, got %v", cash.DisplayBalance)
	}

	revenue := findBalance(t, balances, "4000")
	if revenue.RawBalance != -1000 {
		t.Fatalf("expected revenue raw balance -1000 (credit-positive activity, debit-positive convention), got %v", revenue.RawBalance)
	}
	if revenue.DisplayBalance != 1000 {
		t.Fatalf("REVENUE display balance should flip sign to positive, got %v", revenue.DisplayBalance)
	}
}

func TestCalculateBalances_OpeningExplicit(t *testing.T) {
	entries := []JournalEntry{
		{ID: "JE-1", Date: "2025-06-01", Period: "2025", Status: StatusPosted, Lines: []JournalLine{
			{AccountID: "1000", Debit: 500},
			{AccountID: "4000", Credit: 500},
		}},
	}
	opts := BalanceOptions{
		Range: PeriodRange{StartDate: "2025-01-01", EndDate: "2025-12-31"},
		Openings: []OpeningBalance{
			{AccountID: "1000", Debit: 10000},
		},
	}
	balances := CalculateBalances(testChart(), entries, opts)
	cash := findBalance(t, balances, "1000")
	if cash.OpeningDebit != 10000 {
		t.Fatalf("expected explicit opening debit 10000, got %v", cash.OpeningDebit)
	}
	if cash.RawBalance != 10500 {
		t.Fatalf("expected closing 10500 (10000 opening + 500 activity), got %v", cash.RawBalance)
	}
}

func TestCalculateBalances_OpeningDerivedFromPriorEntries(t *testing.T) {
	entries := []JournalEntry{
		{ID: "JE-PRIOR", Date: "2024-12-15", Period: "2024", Status: StatusPosted, Lines: []JournalLine{
			{AccountID: "1000", Debit: 5000},
			{AccountID: "3000", Credit: 5000},
		}},
		{ID: "JE-CURRENT", Date: "2025-01-15", Period: "2025", Status: StatusPosted, Lines: []JournalLine{
			{AccountID: "1000", Debit: 500},
			{AccountID: "4000", Credit: 500},
		}},
	}
	opts := BalanceOptions{Range: PeriodRange{StartDate: "2025-01-01", EndDate: "2025-12-31"}}
	balances := CalculateBalances(testChart(), entries, opts)
	cash := findBalance(t, balances, "1000")
	if cash.OpeningDebit != 5000 {
		t.Fatalf("expected opening derived from the pre-range entry (5000), got %v", cash.OpeningDebit)
	}
	if cash.PeriodDebits != 500 {
		t.Fatalf("expected only the in-range entry counted as period activity, got %v", cash.PeriodDebits)
	}
	if cash.RawBalance != 5500 {
		t.Fatalf("expected closing 5500, got %v", cash.RawBalance)
	}
}

func TestCalculateBalances_RevenueExpenseSigns(t *testing.T) {
	entries := []JournalEntry{
		{ID: "JE-1", Date: "2025-01-01", Status: StatusPosted, Lines: []JournalLine{
			{AccountID: "1000", Debit: 1000},
			{AccountID: "4000", Credit: 1000},
		}},
		{ID: "JE-2", Date: "2025-01-02", Status: StatusPosted, Lines: []JournalLine{
			{AccountID: "5000", Debit: 300},
			{AccountID: "1000", Credit: 300},
		}},
	}
	balances := CalculateBalances(testChart(), entries, BalanceOptions{})

	expense := findBalance(t, balances, "5000")
	if expense.RawBalance != 300 || expense.DisplayBalance != 300 {
		t.Fatalf("EXPENSE is natural-debit: raw and display should both be 300, got raw=%v display=%v", expense.RawBalance, expense.DisplayBalance)
	}

	liability := findBalance(t, balances, "2000")
	if liability.RawBalance != 0 || liability.DisplayBalance != 0 {
		t.Fatalf("untouched liability account should be zero, got raw=%v display=%v", liability.RawBalance, liability.DisplayBalance)
	}
}

func TestCalculateBalances_NormalBalanceDisplay(t *testing.T) {
	tests := []struct {
		accountType AccountType
		want        DebitCredit
	}{
		{AccountAsset, Debit},
		{AccountExpense, Debit},
		{AccountLiability, Credit},
		{AccountEquity, Credit},
		{AccountRevenue, Credit},
	}
	for _, tc := range tests {
		got, ok := NormalBalance(tc.accountType)
		if !ok || got != tc.want {
			t.Errorf("NormalBalance(%s) = %s, %v; want %s, true", tc.accountType, got, ok, tc.want)
		}
	}
	if _, ok := NormalBalance(AccountType("BOGUS")); ok {
		t.Error("expected unrecognized AccountType to return ok=false")
	}
}

func TestCalculateBalances_ProvenanceCounts(t *testing.T) {
	entries := []JournalEntry{
		{ID: "JE-1", Date: "2025-01-01", Status: StatusPosted, Lines: []JournalLine{
			{ID: "L1", AccountID: "1000", Debit: 100},
			{ID: "L2", AccountID: "4000", Credit: 100},
		}},
		{ID: "JE-2", Date: "2025-01-02", Status: StatusPosted, Lines: []JournalLine{
			{ID: "L3", AccountID: "1000", Debit: 50},
			{ID: "L4", AccountID: "4000", Credit: 50},
		}},
	}
	balances := CalculateBalances(testChart(), entries, BalanceOptions{})
	cash := findBalance(t, balances, "1000")
	if cash.SourceEntryCount != 2 || cash.SourceLineCount != 2 {
		t.Fatalf("expected 2 entries/2 lines, got entries=%d lines=%d", cash.SourceEntryCount, cash.SourceLineCount)
	}
}

func TestCalculateBalances_ExcludesDraftAndVoided(t *testing.T) {
	entries := []JournalEntry{
		{ID: "JE-DRAFT", Date: "2025-01-01", Status: StatusDraft, Lines: []JournalLine{
			{AccountID: "1000", Debit: 999999},
			{AccountID: "4000", Credit: 999999},
		}},
		{ID: "JE-VOIDED", Date: "2025-01-02", Status: StatusVoided, Lines: []JournalLine{
			{AccountID: "1000", Debit: 888888},
			{AccountID: "4000", Credit: 888888},
		}},
		{ID: "JE-POSTED", Date: "2025-01-03", Status: StatusPosted, Lines: []JournalLine{
			{AccountID: "1000", Debit: 100},
			{AccountID: "4000", Credit: 100},
		}},
	}
	balances := CalculateBalances(testChart(), entries, BalanceOptions{})
	cash := findBalance(t, balances, "1000")
	if cash.RawBalance != 100 {
		t.Fatalf("expected only the posted entry to count, got %v", cash.RawBalance)
	}
}

func TestCalculateBalances_Deterministic(t *testing.T) {
	entries := []JournalEntry{
		{ID: "JE-1", Date: "2025-01-01", Status: StatusPosted, Lines: []JournalLine{
			{AccountID: "1000", Debit: 100.1},
			{AccountID: "4000", Credit: 100.1},
		}},
		{ID: "JE-2", Date: "2025-01-02", Status: StatusPosted, Lines: []JournalLine{
			{AccountID: "1000", Debit: 200.2},
			{AccountID: "4000", Credit: 200.2},
		}},
		{ID: "JE-3", Date: "2025-01-03", Status: StatusPosted, Lines: []JournalLine{
			{AccountID: "1000", Debit: 300.3},
			{AccountID: "4000", Credit: 300.3},
		}},
	}
	first := CalculateBalances(testChart(), entries, BalanceOptions{})
	for i := 0; i < 20; i++ {
		got := CalculateBalances(testChart(), entries, BalanceOptions{})
		for j := range got {
			if got[j] != first[j] {
				t.Fatalf("run %d: balance for %s differs: %+v vs %+v", i, got[j].AccountID, got[j], first[j])
			}
		}
	}
}
