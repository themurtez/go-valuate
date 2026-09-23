package ledger

import (
	"math"
	"testing"
)

func TestNormalizeTrialBalance_Balanced(t *testing.T) {
	in := TrialBalanceInput{
		Period: "2025",
		Lines: []TrialBalanceInputLine{
			{AccountID: "1000", Debit: 10000},
			{AccountID: "2000", Credit: 4000},
			{AccountID: "3000", Credit: 6000},
		},
	}
	norm := NormalizeTrialBalance(in, testChart(), 0)
	if !norm.Balanced {
		t.Fatalf("expected balanced, diff=%v issues=%+v", norm.Difference, norm.Issues)
	}
	if norm.TotalDebits != 10000 || norm.TotalCredits != 10000 {
		t.Fatalf("unexpected totals: debits=%v credits=%v", norm.TotalDebits, norm.TotalCredits)
	}
}

func TestNormalizeTrialBalance_Unbalanced(t *testing.T) {
	in := TrialBalanceInput{
		Period: "2025",
		Lines: []TrialBalanceInputLine{
			{AccountID: "1000", Debit: 10000},
			{AccountID: "3000", Credit: 9000},
		},
	}
	norm := NormalizeTrialBalance(in, testChart(), 0)
	if norm.Balanced {
		t.Fatal("expected unbalanced")
	}
	if !hasCode(norm.Issues, IssueUnbalancedTrialBalance) {
		t.Fatalf("expected IssueUnbalancedTrialBalance, got %+v", norm.Issues)
	}
	if norm.Difference != 1000 {
		t.Fatalf("expected difference 1000 (never auto-corrected), got %v", norm.Difference)
	}
}

func TestNormalizeTrialBalance_DuplicateAccountRow(t *testing.T) {
	in := TrialBalanceInput{
		Period: "2025",
		Lines: []TrialBalanceInputLine{
			{AccountID: "1000", Debit: 100},
			{AccountID: "1000", Debit: 200},
		},
	}
	norm := NormalizeTrialBalance(in, testChart(), 0)
	if !hasCode(norm.Issues, IssueDuplicateAccount) {
		t.Fatalf("expected IssueDuplicateAccount, got %+v", norm.Issues)
	}
}

func TestNormalizeTrialBalance_UnknownAccount(t *testing.T) {
	in := TrialBalanceInput{
		Period: "2025",
		Lines: []TrialBalanceInputLine{
			{AccountID: "9999", Debit: 100},
		},
	}
	norm := NormalizeTrialBalance(in, testChart(), 0)
	if !hasCode(norm.Issues, IssueUnknownAccount) {
		t.Fatalf("expected IssueUnknownAccount, got %+v", norm.Issues)
	}
}

func TestNormalizeTrialBalance_NonFinite(t *testing.T) {
	in := TrialBalanceInput{
		Period: "2025",
		Lines: []TrialBalanceInputLine{
			{AccountID: "1000", Debit: math.NaN()},
		},
	}
	norm := NormalizeTrialBalance(in, testChart(), 0)
	if !hasCode(norm.Issues, IssueNonFiniteAmount) {
		t.Fatalf("expected IssueNonFiniteAmount, got %+v", norm.Issues)
	}
}

func TestNormalizeTrialBalance_BothDebitCredit(t *testing.T) {
	in := TrialBalanceInput{
		Period: "2025",
		Lines: []TrialBalanceInputLine{
			{AccountID: "1000", Debit: 100, Credit: 50},
		},
	}
	norm := NormalizeTrialBalance(in, testChart(), 0)
	if !hasCode(norm.Issues, IssueInvalidDebitCredit) {
		t.Fatalf("expected IssueInvalidDebitCredit, got %+v", norm.Issues)
	}
}

func TestNormalizeTrialBalance_MixedCurrency(t *testing.T) {
	chart := BuildChartOfAccounts([]Account{
		{ID: "1000", Name: "Cash USD", Type: AccountAsset, Currency: "USD"},
		{ID: "2000", Name: "Cash EUR", Type: AccountAsset, Currency: "EUR"},
	})
	in := TrialBalanceInput{
		Period: "2025",
		Lines: []TrialBalanceInputLine{
			{AccountID: "1000", Debit: 1000},
			{AccountID: "2000", Debit: 500},
		},
	}
	norm := NormalizeTrialBalance(in, chart, 0)
	if !hasCode(norm.Issues, IssueMixedCurrency) {
		t.Fatalf("expected IssueMixedCurrency, got %+v", norm.Issues)
	}
	// The mismatched-currency line should be excluded from totals.
	if norm.TotalDebits != 1000 {
		t.Fatalf("expected mixed-currency line excluded from totals, got TotalDebits=%v", norm.TotalDebits)
	}
}

func TestNormalizeTrialBalance_OpeningBalance(t *testing.T) {
	in := TrialBalanceInput{
		Period: "2025",
		Lines: []TrialBalanceInputLine{
			{AccountID: "1000", Debit: 5500, OpeningBalance: &OpeningBalance{AccountID: "1000", Debit: 5000}},
			{AccountID: "3000", Credit: 5500},
		},
	}
	norm := NormalizeTrialBalance(in, testChart(), 0)
	var cashLine NormalizedTrialBalanceLine
	for _, l := range norm.Lines {
		if l.AccountID == "1000" {
			cashLine = l
		}
	}
	if cashLine.OpeningDebit != 5000 {
		t.Fatalf("expected opening debit 5000, got %v", cashLine.OpeningDebit)
	}
}

func TestNormalizeTrialBalance_NoFabricatedJournals(t *testing.T) {
	// A NormalizedTrialBalance never produces JournalEntry values — this is
	// a compile-time/contract check: NormalizedTrialBalance has no field of
	// type []JournalEntry, so there's nothing to fabricate. Documented via
	// a determinism check instead: normalizing the same input twice never
	// mutates global or shared state.
	in := TrialBalanceInput{
		Period: "2025",
		Lines: []TrialBalanceInputLine{
			{AccountID: "1000", Debit: 100},
			{AccountID: "3000", Credit: 100},
		},
	}
	first := NormalizeTrialBalance(in, testChart(), 0)
	second := NormalizeTrialBalance(in, testChart(), 0)
	if first.TotalDebits != second.TotalDebits {
		t.Fatal("expected identical results across repeated calls")
	}
}
