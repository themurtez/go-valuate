package ledger

import "testing"

func hierarchyChart() []Account {
	return []Account{
		{ID: "1000", Name: "Assets", Type: AccountAsset, Active: true},
		{ID: "1100", Name: "Current Assets", Type: AccountAsset, ParentID: "1000", Active: true},
		{ID: "1110", Name: "Cash", Type: AccountAsset, ParentID: "1100", Active: true},
		{ID: "1120", Name: "AR", Type: AccountAsset, ParentID: "1100", Active: true},
		{ID: "4000", Name: "Revenue", Type: AccountRevenue, Active: true},
	}
}

func TestBuildRollups_NoDoubleCounting(t *testing.T) {
	chart := BuildChartOfAccounts(hierarchyChart())
	entries := []JournalEntry{
		{ID: "JE-1", Date: "2025-01-01", Status: StatusPosted, Lines: []JournalLine{
			{AccountID: "1110", Debit: 1000},
			{AccountID: "4000", Credit: 1000},
		}},
		{ID: "JE-2", Date: "2025-01-02", Status: StatusPosted, Lines: []JournalLine{
			{AccountID: "1120", Debit: 500},
			{AccountID: "4000", Credit: 500},
		}},
	}
	balances := CalculateBalances(chart, entries, BalanceOptions{})
	rollups := BuildRollups(chart, balances)

	assets := findRollup(t, rollups, "1000")
	if assets.DirectRawBalance != 0 {
		t.Fatalf("Assets has no direct postings, expected DirectRawBalance=0, got %v", assets.DirectRawBalance)
	}
	if assets.ChildRawBalance != 1500 {
		t.Fatalf("expected ChildRawBalance=1500 (1000+500 rolled up through Current Assets), got %v", assets.ChildRawBalance)
	}
	if assets.TotalRawBalance != 1500 {
		t.Fatalf("expected TotalRawBalance=1500 without double counting, got %v", assets.TotalRawBalance)
	}

	currentAssets := findRollup(t, rollups, "1100")
	if currentAssets.TotalRawBalance != 1500 {
		t.Fatalf("expected Current Assets total=1500, got %v", currentAssets.TotalRawBalance)
	}
}

func TestBuildRollups_DirectParentPosting(t *testing.T) {
	chart := BuildChartOfAccounts(hierarchyChart())
	entries := []JournalEntry{
		// Post directly to the parent "Current Assets" (1100) AND to its
		// child "Cash" (1110) — parent must show both Direct and Child
		// distinctly, and Total must be their sum, not a double count.
		{ID: "JE-1", Date: "2025-01-01", Status: StatusPosted, Lines: []JournalLine{
			{AccountID: "1100", Debit: 300},
			{AccountID: "4000", Credit: 300},
		}},
		{ID: "JE-2", Date: "2025-01-02", Status: StatusPosted, Lines: []JournalLine{
			{AccountID: "1110", Debit: 700},
			{AccountID: "4000", Credit: 700},
		}},
	}
	balances := CalculateBalances(chart, entries, BalanceOptions{})
	rollups := BuildRollups(chart, balances)

	currentAssets := findRollup(t, rollups, "1100")
	if currentAssets.DirectRawBalance != 300 {
		t.Fatalf("expected direct=300, got %v", currentAssets.DirectRawBalance)
	}
	if currentAssets.ChildRawBalance != 700 {
		t.Fatalf("expected child=700, got %v", currentAssets.ChildRawBalance)
	}
	if currentAssets.TotalRawBalance != 1000 {
		t.Fatalf("expected total=1000 (300 direct + 700 child), got %v", currentAssets.TotalRawBalance)
	}
}

func TestBuildRollups_Deterministic(t *testing.T) {
	chart := BuildChartOfAccounts(hierarchyChart())
	entries := []JournalEntry{
		{ID: "JE-1", Date: "2025-01-01", Status: StatusPosted, Lines: []JournalLine{
			{AccountID: "1110", Debit: 111.11},
			{AccountID: "4000", Credit: 111.11},
		}},
		{ID: "JE-2", Date: "2025-01-02", Status: StatusPosted, Lines: []JournalLine{
			{AccountID: "1120", Debit: 222.22},
			{AccountID: "4000", Credit: 222.22},
		}},
	}
	balances := CalculateBalances(chart, entries, BalanceOptions{})
	first := BuildRollups(chart, balances)
	for i := 0; i < 20; i++ {
		got := BuildRollups(chart, balances)
		for j := range got {
			if got[j] != first[j] {
				t.Fatalf("run %d: rollup for %s differs", i, got[j].AccountID)
			}
		}
	}
}

func findRollup(t *testing.T, rollups []RollupBalance, accountID string) RollupBalance {
	t.Helper()
	for _, r := range rollups {
		if r.AccountID == accountID {
			return r
		}
	}
	t.Fatalf("no rollup found for account %s", accountID)
	return RollupBalance{}
}
