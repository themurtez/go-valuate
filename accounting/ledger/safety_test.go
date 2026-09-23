package ledger

import (
	"math"
	"testing"
)

func TestSafety_NaNRejectedInEntry(t *testing.T) {
	e := balancedEntry()
	e.Lines[0].Debit = math.NaN()
	issues := ValidateEntries([]JournalEntry{e}, testChart(), ValidateOptions{})
	if !hasCode(issues, IssueNonFiniteAmount) {
		t.Fatalf("expected IssueNonFiniteAmount for NaN, got %+v", issues)
	}
}

func TestSafety_InfRejectedInEntry(t *testing.T) {
	e := balancedEntry()
	e.Lines[0].Debit = math.Inf(1)
	issues := ValidateEntries([]JournalEntry{e}, testChart(), ValidateOptions{})
	if !hasCode(issues, IssueNonFiniteAmount) {
		t.Fatalf("expected IssueNonFiniteAmount for +Inf, got %+v", issues)
	}

	e2 := balancedEntry()
	e2.Lines[1].Credit = math.Inf(-1)
	issues2 := ValidateEntries([]JournalEntry{e2}, testChart(), ValidateOptions{})
	if !hasCode(issues2, IssueNonFiniteAmount) {
		t.Fatalf("expected IssueNonFiniteAmount for -Inf, got %+v", issues2)
	}
}

func TestSafety_NaNNeverPropagatesIntoBalances(t *testing.T) {
	e := balancedEntry()
	e.Lines[0].Debit = math.NaN()
	balances := CalculateBalances(testChart(), []JournalEntry{e}, BalanceOptions{})
	for _, b := range balances {
		if math.IsNaN(b.RawBalance) || math.IsInf(b.RawBalance, 0) {
			t.Fatalf("NaN/Inf leaked into Balance.RawBalance for account %s: %v", b.AccountID, b.RawBalance)
		}
		if math.IsNaN(b.DisplayBalance) || math.IsInf(b.DisplayBalance, 0) {
			t.Fatalf("NaN/Inf leaked into Balance.DisplayBalance for account %s: %v", b.AccountID, b.DisplayBalance)
		}
	}
}

func TestSafety_NegativeDebitCreditRejected(t *testing.T) {
	e := balancedEntry()
	e.Lines[0].Debit = -1
	issues := ValidateEntries([]JournalEntry{e}, testChart(), ValidateOptions{})
	if !hasCode(issues, IssueNegativeAmount) {
		t.Fatalf("expected IssueNegativeAmount, got %+v", issues)
	}
}

func TestSafety_OpeningBalanceNonFiniteRejected(t *testing.T) {
	opts := BalanceOptions{Openings: []OpeningBalance{{AccountID: "1000", Debit: math.Inf(1)}}}
	issues := validateOpeningBalances(opts.Openings, testChart())
	if !hasCode(issues, IssueNonFiniteAmount) {
		t.Fatalf("expected IssueNonFiniteAmount for infinite opening balance, got %+v", issues)
	}
	// And confirm it doesn't corrupt CalculateBalances either.
	balances := CalculateBalances(testChart(), nil, opts)
	cash := findBalance(t, balances, "1000")
	if cash.OpeningDebit != 0 {
		t.Fatalf("expected invalid opening balance to be excluded (0), got %v", cash.OpeningDebit)
	}
}

func TestSafety_MixedCurrencyNeverFetchesFX(t *testing.T) {
	// This package has no HTTP client, no FX-rate field anywhere in its
	// types, and no function signature that could accept one — mixed
	// currency is always either excluded or flagged, never silently
	// converted. This test locks the flagging behavior specifically.
	chart := BuildChartOfAccounts([]Account{
		{ID: "1000", Name: "Cash USD", Type: AccountAsset, Currency: "USD"},
		{ID: "2000", Name: "Cash EUR", Type: AccountAsset, Currency: "EUR"},
		{ID: "3000", Name: "Equity", Type: AccountEquity},
	})
	in := TrialBalanceInput{
		Period: "2025",
		Lines: []TrialBalanceInputLine{
			{AccountID: "1000", Debit: 1000},
			{AccountID: "2000", Debit: 500},
			{AccountID: "3000", Credit: 1000},
		},
	}
	norm := NormalizeTrialBalance(in, chart, 0)
	if !hasCode(norm.Issues, IssueMixedCurrency) {
		t.Fatalf("expected IssueMixedCurrency, got %+v", norm.Issues)
	}
}

func TestSafety_ConcurrentCalculateBalances(t *testing.T) {
	entries := balancedLedgerEntries()
	chart := testChart()
	done := make(chan bool, 20)
	for i := 0; i < 20; i++ {
		go func() {
			_ = CalculateBalances(chart, entries, BalanceOptions{})
			_ = BuildTrialBalance(chart, entries, BalanceOptions{}, ModeEndingBalances, 0.01)
			done <- true
		}()
	}
	for i := 0; i < 20; i++ {
		<-done
	}
}
