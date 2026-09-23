package ledger

import (
	"strconv"
	"testing"
)

// benchAccounts builds n accounts: a flat pool of leaf accounts under 10
// top-level parents (for the hierarchy-rollup benchmark to have something
// to roll up), alternating ASSET/EXPENSE/REVENUE/LIABILITY/EQUITY types.
func benchAccounts(n int) []Account {
	types := []AccountType{AccountAsset, AccountExpense, AccountRevenue, AccountLiability, AccountEquity}
	const numParents = 10
	accounts := make([]Account, 0, n+numParents)
	for p := 0; p < numParents; p++ {
		accounts = append(accounts, Account{
			ID: "P" + strconv.Itoa(p), Name: "Parent " + strconv.Itoa(p),
			Type: types[p%len(types)], Active: true,
		})
	}
	for i := 0; i < n; i++ {
		accounts = append(accounts, Account{
			ID:       "A" + strconv.Itoa(i),
			Name:     "Account " + strconv.Itoa(i),
			Type:     types[i%len(types)],
			ParentID: "P" + strconv.Itoa(i%numParents),
			Active:   true,
		})
	}
	return accounts
}

// benchEntries builds n balanced two-line journal entries (2n lines total)
// posting between account 0 and account 1 of accounts, alternating date.
func benchEntries(n int, accounts []Account) []JournalEntry {
	entries := make([]JournalEntry, 0, n)
	a, b := accounts[len(accounts)-2].ID, accounts[len(accounts)-1].ID
	for i := 0; i < n; i++ {
		day := 1 + i%28
		month := 1 + (i/28)%12
		date := "2025-" + pad2(month) + "-" + pad2(day)
		entries = append(entries, JournalEntry{
			ID: "JE-" + strconv.Itoa(i), Date: date, Period: "2025", Status: StatusPosted,
			Lines: []JournalLine{
				{AccountID: a, Debit: 10.5},
				{AccountID: b, Credit: 10.5},
			},
		})
	}
	return entries
}

func pad2(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

func BenchmarkValidateAccounts_1000(b *testing.B) {
	accounts := benchAccounts(1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidateAccounts(accounts)
	}
}

func BenchmarkCalculateBalances_100kLines(b *testing.B) {
	accounts := benchAccounts(1000)
	chart := BuildChartOfAccounts(accounts)
	entries := benchEntries(50_000, accounts) // 50,000 entries x 2 lines = 100,000 lines
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CalculateBalances(chart, entries, BalanceOptions{})
	}
}

func BenchmarkBuildTrialBalance_100kLines(b *testing.B) {
	accounts := benchAccounts(1000)
	chart := BuildChartOfAccounts(accounts)
	entries := benchEntries(50_000, accounts)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = BuildTrialBalance(chart, entries, BalanceOptions{}, ModeEndingBalances, 0.01)
	}
}

func BenchmarkBuildRollups_1000Accounts(b *testing.B) {
	accounts := benchAccounts(1000)
	chart := BuildChartOfAccounts(accounts)
	entries := benchEntries(5000, accounts)
	balances := CalculateBalances(chart, entries, BalanceOptions{})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = BuildRollups(chart, balances)
	}
}
