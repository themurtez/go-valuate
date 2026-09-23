package statements_test

import (
	"fmt"
	"testing"

	"github.com/themurtez/go-valuate/accounting/ledger"
	"github.com/themurtez/go-valuate/accounting/statements"
	"github.com/themurtez/go-valuate/financial"
)

// largeChart builds n accounts: roughly 1/4 revenue, 1/4 expense, 1/4
// asset, 1/4 liability/equity, plus one "balancer" EQUITY account every
// synthetic entry offsets against so every entry stays balanced.
func largeChart(n int) []ledger.Account {
	accounts := make([]ledger.Account, 0, n+1)
	accounts = append(accounts, ledger.Account{ID: "balancer", Number: "balancer", Name: "Balancer", Type: ledger.AccountEquity, Currency: "USD", Active: true})
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("acct-%d", i)
		var t ledger.AccountType
		switch i % 4 {
		case 0:
			t = ledger.AccountRevenue
		case 1:
			t = ledger.AccountExpense
		case 2:
			t = ledger.AccountAsset
		default:
			t = ledger.AccountLiability
		}
		accounts = append(accounts, ledger.Account{
			ID: id, Number: id, Name: fmt.Sprintf("Account %d", i), Type: t, Currency: "USD", Active: true,
		})
	}
	return accounts
}

// largeEntries builds one small balanced JournalEntry per period touching
// every account exactly once (paired with a synthetic cash/equity
// account to keep every entry balanced).
func largeEntries(accounts []ledger.Account, periods []financial.Period) []ledger.JournalEntry {
	var entries []ledger.JournalEntry
	entryID := 0
	for _, p := range periods {
		for i, acct := range accounts {
			entryID++
			var debitID, creditID string
			if acct.Type == ledger.AccountAsset || acct.Type == ledger.AccountExpense {
				debitID, creditID = acct.ID, "balancer"
			} else {
				debitID, creditID = "balancer", acct.ID
			}
			entries = append(entries, ledger.JournalEntry{
				ID: fmt.Sprintf("bench-je-%d", entryID), Date: string(p) + "-01", Period: string(p), Status: ledger.StatusPosted,
				Lines: []ledger.JournalLine{
					{AccountID: debitID, Debit: float64(100 + i)},
					{AccountID: creditID, Credit: float64(100 + i)},
				},
			})
		}
	}
	return entries
}

// manyToOneMappings maps every account to just 4 canonical codes (one per
// AccountType bucket) — the many-to-one stress case task section 46 asks
// for specifically.
func manyToOneMappings(accounts []ledger.Account) []statements.AccountMapping {
	mappings := make([]statements.AccountMapping, 0, len(accounts))
	for _, acct := range accounts {
		var code financial.Code
		var st financial.StatementType
		switch acct.Type {
		case ledger.AccountRevenue:
			code, st = financial.CodeRevOther, financial.StatementIncomeStatement
		case ledger.AccountExpense:
			code, st = financial.CodeOpexOther, financial.StatementIncomeStatement
		case ledger.AccountAsset:
			code, st = financial.CodeBsCurrentAssetOther, financial.StatementBalanceSheet
		default:
			code, st = financial.CodeBsCurrentLiabilityOther, financial.StatementBalanceSheet
		}
		mappings = append(mappings, statements.AccountMapping{
			AccountID: acct.ID, FinancialCode: code, StatementType: st, Source: statements.MappingSourceExplicit, Confirmed: true,
		})
	}
	return mappings
}

func tenPeriods() []financial.Period {
	periods := make([]financial.Period, 10)
	for i := range periods {
		periods[i] = financial.Period(fmt.Sprintf("2025-%02d", i+1))
	}
	return periods
}

// BenchmarkBuild_1000Accounts_10Periods_ManyToOne is task section 46's
// primary benchmark: 1,000 accounts, 10 periods, many-to-one mapping,
// full statement construction, FinancialDataset conversion, all in one
// Build call.
func BenchmarkBuild_1000Accounts_10Periods_ManyToOne(b *testing.B) {
	accounts := largeChart(1000)
	periods := tenPeriods()
	entries := largeEntries(accounts, periods)
	mappings := manyToOneMappings(accounts)

	input := statements.Input{
		Source:    statements.SourceLedger,
		Chart:     accounts,
		Entries:   entries,
		Periods:   periods,
		Mappings:  mappings,
		Selection: statements.SelectionBoth,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = statements.Build(input, statements.Options{})
	}
}

// BenchmarkBuild_ScalingByAccountCount runs the same shape at several
// account counts so a superlinear (O(N^2) or worse) path shows up as
// non-proportional scaling between sub-benchmarks, rather than needing a
// separate profiling pass to notice.
func BenchmarkBuild_ScalingByAccountCount(b *testing.B) {
	for _, n := range []int{100, 500, 1000, 2000} {
		n := n
		b.Run(fmt.Sprintf("accounts=%d", n), func(b *testing.B) {
			accounts := largeChart(n)
			periods := tenPeriods()
			entries := largeEntries(accounts, periods)
			mappings := manyToOneMappings(accounts)

			input := statements.Input{
				Source:    statements.SourceLedger,
				Chart:     accounts,
				Entries:   entries,
				Periods:   periods,
				Mappings:  mappings,
				Selection: statements.SelectionBoth,
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = statements.Build(input, statements.Options{})
			}
		})
	}
}

// BenchmarkResolveTemplate_1000Accounts benchmarks mapping-template
// resolution in isolation at scale.
func BenchmarkResolveTemplate_1000Accounts(b *testing.B) {
	accounts := largeChart(1000)
	chart := ledger.BuildChartOfAccounts(accounts)

	rules := make([]statements.AccountMappingRule, 0, len(accounts))
	for _, acct := range accounts {
		rules = append(rules, statements.AccountMappingRule{
			AccountNumber: acct.Number,
			Mapping:       statements.AccountMapping{FinancialCode: financial.CodeOpexOther, StatementType: financial.StatementIncomeStatement},
		})
	}
	template := statements.MappingTemplate{Version: "bench-v1", Mappings: rules}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = statements.ResolveTemplate(template, chart)
	}
}
