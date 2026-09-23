package fixtures

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/ledger"
)

func TestServiceBusiness_Balances(t *testing.T) {
	chart := ledger.BuildChartOfAccounts(ServiceBusinessChart())
	entries := ServiceBusinessEntries()

	if issues := ledger.ValidateAccounts(ServiceBusinessChart()); ledger.HasErrors(issues) {
		t.Fatalf("expected valid chart, got %+v", issues)
	}
	if issues := ledger.ValidateEntries(entries, chart, ledger.ValidateOptions{}); ledger.HasErrors(issues) {
		t.Fatalf("expected balanced entries, got %+v", issues)
	}

	tb := ledger.BuildTrialBalance(chart, entries, ledger.BalanceOptions{}, ledger.ModeEndingBalances, 0.005)
	if !tb.Balanced {
		t.Fatalf("expected balanced trial balance, diff=%v", tb.Difference)
	}
}

func TestRetailer_Balances(t *testing.T) {
	chart := ledger.BuildChartOfAccounts(RetailerChart())
	entries := RetailerEntries()

	if issues := ledger.ValidateEntries(entries, chart, ledger.ValidateOptions{}); ledger.HasErrors(issues) {
		t.Fatalf("expected balanced entries, got %+v", issues)
	}
	tb := ledger.BuildTrialBalance(chart, entries, ledger.BalanceOptions{}, ledger.ModeEndingBalances, 0.005)
	if !tb.Balanced {
		t.Fatalf("expected balanced trial balance, diff=%v", tb.Difference)
	}
}

func TestOwnerOperated_Balances(t *testing.T) {
	chart := ledger.BuildChartOfAccounts(OwnerOperatedChart())
	entries := OwnerOperatedEntries()

	if issues := ledger.ValidateEntries(entries, chart, ledger.ValidateOptions{}); ledger.HasErrors(issues) {
		t.Fatalf("expected balanced entries, got %+v", issues)
	}
	tb := ledger.BuildTrialBalance(chart, entries, ledger.BalanceOptions{}, ledger.ModeEndingBalances, 0.005)
	if !tb.Balanced {
		t.Fatalf("expected balanced trial balance, diff=%v", tb.Difference)
	}
}

func TestMultiDepartment_RollupsNoDoubleCounting(t *testing.T) {
	chart := ledger.BuildChartOfAccounts(MultiDepartmentChart())
	entries := MultiDepartmentEntries()

	if issues := ledger.ValidateAccounts(MultiDepartmentChart()); ledger.HasErrors(issues) {
		t.Fatalf("expected valid hierarchy, got %+v", issues)
	}
	if issues := ledger.ValidateEntries(entries, chart, ledger.ValidateOptions{}); ledger.HasErrors(issues) {
		t.Fatalf("expected balanced entries, got %+v", issues)
	}

	balances := ledger.CalculateBalances(chart, entries, ledger.BalanceOptions{})
	rollups := ledger.BuildRollups(chart, balances)

	var totalExpenses ledger.RollupBalance
	for _, r := range rollups {
		if r.AccountID == "6000" {
			totalExpenses = r
		}
	}
	// Direct (1000, posted straight to the parent) + Child (4000 product +
	// 2500 services) = 7500.
	if totalExpenses.DirectRawBalance != 1000 {
		t.Fatalf("expected direct=1000, got %v", totalExpenses.DirectRawBalance)
	}
	if totalExpenses.ChildRawBalance != 6500 {
		t.Fatalf("expected child=6500 (4000+2500), got %v", totalExpenses.ChildRawBalance)
	}
	if totalExpenses.TotalRawBalance != 7500 {
		t.Fatalf("expected total=7500, got %v", totalExpenses.TotalRawBalance)
	}
}

func TestUnbalancedJournal_FlagsIssue(t *testing.T) {
	chart := ledger.BuildChartOfAccounts(ServiceBusinessChart())
	issues := ledger.ValidateEntries(UnbalancedJournalEntries(), chart, ledger.ValidateOptions{})
	found := false
	for _, i := range issues {
		if i.Code == ledger.IssueUnbalancedEntry {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected IssueUnbalancedEntry, got %+v", issues)
	}
}

func TestUnbalancedTrialBalance_FlagsIssue(t *testing.T) {
	chart := ledger.BuildChartOfAccounts(ServiceBusinessChart())
	norm := ledger.NormalizeTrialBalance(UnbalancedTrialBalance(), chart, 0)
	if norm.Balanced {
		t.Fatal("expected unbalanced")
	}
	if norm.Difference != 1000 {
		t.Fatalf("expected difference 1000, got %v", norm.Difference)
	}
}

func TestOpeningBalances_AppliedCorrectly(t *testing.T) {
	chart := ledger.BuildChartOfAccounts(ServiceBusinessChart())
	opts := ledger.BalanceOptions{Openings: OpeningBalancesFixture()}
	balances := ledger.CalculateBalances(chart, ServiceBusinessEntries(), opts)

	var cash ledger.Balance
	for _, b := range balances {
		if b.AccountID == "1000" {
			cash = b
		}
	}
	if cash.OpeningDebit != 25000 {
		t.Fatalf("expected opening debit 25000, got %v", cash.OpeningDebit)
	}
}

func TestReversalPair_ValidatesConsistently(t *testing.T) {
	chart := ledger.BuildChartOfAccounts(ServiceBusinessChart())
	entries := ReversalPairEntries()
	issues := ledger.ValidateEntries(entries, chart, ledger.ValidateOptions{})
	for _, i := range issues {
		if i.Code == ledger.IssueInvalidReversal {
			t.Fatalf("expected no IssueInvalidReversal, got %+v", issues)
		}
	}

	balances := ledger.CalculateBalances(chart, entries, ledger.BalanceOptions{})
	var rent ledger.Balance
	for _, b := range balances {
		if b.AccountID == "6100" {
			rent = b
		}
	}
	// The original (REVERSED) entry posts +3000 debit; the reversal
	// (POSTED) posts -3000 (credits 3000) — both affect balances, net zero.
	if rent.RawBalance != 0 {
		t.Fatalf("expected net-zero rent after reversal, got %v", rent.RawBalance)
	}
}

func TestMixedCurrencyTrialBalance_FlagsIssue(t *testing.T) {
	chart := ledger.BuildChartOfAccounts(MixedCurrencyChart())
	norm := ledger.NormalizeTrialBalance(MixedCurrencyTrialBalance(), chart, 0)
	found := false
	for _, i := range norm.Issues {
		if i.Code == ledger.IssueMixedCurrency {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected IssueMixedCurrency, got %+v", norm.Issues)
	}
}
