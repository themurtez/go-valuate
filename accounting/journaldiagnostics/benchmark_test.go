package journaldiagnostics_test

import (
	"fmt"
	"testing"

	"github.com/themurtez/go-valuate/accounting/journaldiagnostics"
	"github.com/themurtez/go-valuate/accounting/ledger"
)

// generateLedger builds a synthetic ledger of n balanced 2-line entries
// across accountCount expense/cash pairs, spread over a year, for
// benchmarking. Every 500th entry repeats a prior amount exactly (to give
// duplicate/repeated-amount detection something realistic to do without
// making every entry a duplicate of every other).
func generateLedger(n, accountCount int) ledger.Ledger {
	accounts := make([]ledger.Account, 0, accountCount+1)
	accounts = append(accounts, ledger.Account{ID: "CASH", Name: "Cash", Type: ledger.AccountAsset, Currency: "USD", Active: true})
	for i := 0; i < accountCount; i++ {
		accounts = append(accounts, ledger.Account{
			ID: fmt.Sprintf("EXP-%d", i), Name: fmt.Sprintf("Expense %d", i),
			Type: ledger.AccountExpense, Currency: "USD", Active: true,
		})
	}

	entries := make([]ledger.JournalEntry, n)
	for i := 0; i < n; i++ {
		day := i % 365
		date := fmt.Sprintf("2025-%02d-%02d", (day/28)+1, (day%28)+1)
		amount := float64(100 + (i*37)%9000)
		if i%500 == 0 && i > 0 {
			amount = 1234.56 // deliberate repeated amount
		}
		acct := fmt.Sprintf("EXP-%d", i%accountCount)
		entries[i] = ledger.JournalEntry{
			ID: fmt.Sprintf("JE-%d", i), Date: date, Period: fmt.Sprintf("2025-%02d", (day/28)+1),
			Status: ledger.StatusPosted, Description: "Benchmark entry",
			Lines: []ledger.JournalLine{
				{ID: "L1", AccountID: acct, Debit: amount},
				{ID: "L2", AccountID: "CASH", Credit: amount},
			},
		}
	}
	return ledger.Ledger{Accounts: accounts, Entries: entries}
}

func benchmarkPolicy() journaldiagnostics.Policy {
	p := journaldiagnostics.DefaultPolicy()
	p.MaterialAmount = 5000
	p.RoundDollarMinAmount = 1000
	p.LargeEntryAbsoluteThreshold = 8000
	p.RepeatedAmountMinAmount = 1000
	p.ApprovalThreshold = 9000
	return p
}

func benchmarkWindow() journaldiagnostics.PeriodWindow {
	closeDate := "2025-12-05"
	return journaldiagnostics.PeriodWindow{Period: "2025-12", StartDate: "2025-01-01", EndDate: "2025-12-31", CloseDate: &closeDate}
}

func BenchmarkCalculate_100kEntries_500Accounts(b *testing.B) {
	l := generateLedger(100000, 500)
	policy := benchmarkPolicy()
	window := benchmarkWindow()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = journaldiagnostics.Calculate(l, nil, window, policy)
	}
}

func BenchmarkCalculate_10kEntries_100Accounts(b *testing.B) {
	l := generateLedger(10000, 100)
	policy := benchmarkPolicy()
	window := benchmarkWindow()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = journaldiagnostics.Calculate(l, nil, window, policy)
	}
}

// BenchmarkCalculate_ScalingCheck runs Calculate at 1x, 2x, and 4x the same
// entry count and reports ns/op for each — used manually (via `go test
// -bench=ScalingCheck -run='^$'`) to eyeball whether runtime grows roughly
// linearly (expected) or quadratically (a regression) with entry count,
// since duplicate detection is the one rule family with real O(N^2)
// potential if implemented with pairwise comparison instead of normalized-
// signature maps (see duplicates.go's doc comment).
func BenchmarkCalculate_ScalingCheck(b *testing.B) {
	sizes := []int{5000, 10000, 20000}
	for _, n := range sizes {
		l := generateLedger(n, n/50+1)
		policy := benchmarkPolicy()
		window := benchmarkWindow()
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = journaldiagnostics.Calculate(l, nil, window, policy)
			}
		})
	}
}

func BenchmarkDuplicateDetection_100kEntries(b *testing.B) {
	// A ledger where every entry shares one of only 50 distinct
	// (account, amount) signatures, so duplicate/possible-duplicate
	// grouping has real work to do across a large population, rather than
	// each entry being trivially distinct.
	accounts := []ledger.Account{
		{ID: "CASH", Name: "Cash", Type: ledger.AccountAsset, Currency: "USD", Active: true},
		{ID: "EXP", Name: "Expense", Type: ledger.AccountExpense, Currency: "USD", Active: true},
	}
	entries := make([]ledger.JournalEntry, 100000)
	for i := range entries {
		amount := float64(100 * (1 + i%50))
		day := i % 28
		entries[i] = ledger.JournalEntry{
			ID: fmt.Sprintf("JE-%d", i), Date: fmt.Sprintf("2025-06-%02d", day+1), Period: "2025-06",
			Status: ledger.StatusPosted,
			Lines: []ledger.JournalLine{
				{ID: "L1", AccountID: "EXP", Debit: amount},
				{ID: "L2", AccountID: "CASH", Credit: amount},
			},
		}
	}
	l := ledger.Ledger{Accounts: accounts, Entries: entries}
	policy := benchmarkPolicy()
	window := journaldiagnostics.PeriodWindow{Period: "2025-06", StartDate: "2025-06-01", EndDate: "2025-06-30"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = journaldiagnostics.Calculate(l, nil, window, policy)
	}
}

// BenchmarkAccountHistoryBaseline_100kEntries_1kAccounts stresses
// buildAccountBaselines' median/MAD calculation specifically by using many
// distinct accounts (1000) against a large entry count, so each account's
// baseline slice is still substantial. Calculate has no exported entry
// point for running a single rule family in isolation, so — like every
// benchmark in this file — this measures the whole Calculate call; the
// account/entry-count shape is what isolates this scenario's cost profile
// from the others (BenchmarkDuplicateDetection_100kEntries uses few
// distinct signatures instead, BenchmarkCalculate_100kEntries_500Accounts
// is the general-shape baseline).
func BenchmarkAccountHistoryBaseline_100kEntries_1kAccounts(b *testing.B) {
	l := generateLedger(100000, 1000)
	policy := benchmarkPolicy()
	window := benchmarkWindow()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = journaldiagnostics.Calculate(l, nil, window, policy)
	}
}

// BenchmarkPeriodEndScan_100kEntries spreads 100k entries across a full
// year so only a small fraction fall within Policy.PeriodEndDays of
// PeriodWindow.EndDate, stressing the period-end scan's per-entry date
// comparison over the full population rather than a population
// concentrated near period end. Measures the whole Calculate call — see
// BenchmarkAccountHistoryBaseline_100kEntries_1kAccounts's doc comment for
// why.
func BenchmarkPeriodEndScan_100kEntries(b *testing.B) {
	l := generateLedger(100000, 200)
	policy := benchmarkPolicy()
	window := benchmarkWindow()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = journaldiagnostics.Calculate(l, nil, window, policy)
	}
}
