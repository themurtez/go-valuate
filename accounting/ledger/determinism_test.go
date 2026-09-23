package ledger

import (
	"encoding/json"
	"testing"
)

// TestDeterministic_FullPipeline proves repeated execution of the whole
// chain (validate -> balances -> trial balance -> rollups) against
// identical input returns byte-for-byte identical JSON, mirroring
// analytics/debt/determinism_test.go's TestCalculate_Deterministic.
func TestDeterministic_FullPipeline(t *testing.T) {
	chart := BuildChartOfAccounts(hierarchyChart())
	entries := []JournalEntry{
		{ID: "JE-1", Date: "2025-01-01", Period: "2025", Status: StatusPosted, Lines: []JournalLine{
			{AccountID: "1110", Debit: 1234.56},
			{AccountID: "4000", Credit: 1234.56},
		}},
		{ID: "JE-2", Date: "2025-02-01", Period: "2025", Status: StatusPosted, Lines: []JournalLine{
			{AccountID: "1120", Debit: 789.01},
			{AccountID: "4000", Credit: 789.01},
		}},
	}
	opts := BalanceOptions{Range: PeriodRange{StartDate: "2025-01-01", EndDate: "2025-12-31"}}

	run := func() []byte {
		balances := CalculateBalances(chart, entries, opts)
		tb := BuildTrialBalance(chart, entries, opts, ModeEndingBalances, 0.01)
		rollups := BuildRollups(chart, balances)
		out := struct {
			Balances []Balance
			TB       TrialBalance
			Rollups  []RollupBalance
		}{balances, tb, rollups}
		b, err := json.Marshal(out)
		if err != nil {
			t.Fatalf("marshal failed: %v", err)
		}
		return b
	}

	first := run()
	for i := 0; i < 15; i++ {
		got := run()
		if string(got) != string(first) {
			t.Fatalf("run %d: output differs from first run", i)
		}
	}
}

// TestDeterministic_NormalizeTrialBalance mirrors the same guarantee for
// the TB-import path.
func TestDeterministic_NormalizeTrialBalance(t *testing.T) {
	in := TrialBalanceInput{
		Period: "2025",
		Lines: []TrialBalanceInputLine{
			{AccountID: "1000", Debit: 111.11},
			{AccountID: "1100", Debit: 222.22},
			{AccountID: "2000", Credit: 100},
			{AccountID: "3000", Credit: 233.33},
		},
	}
	chart := testChart()

	first, err := json.Marshal(NormalizeTrialBalance(in, chart, 0.01))
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	for i := 0; i < 15; i++ {
		got, err := json.Marshal(NormalizeTrialBalance(in, chart, 0.01))
		if err != nil {
			t.Fatalf("marshal failed: %v", err)
		}
		if string(got) != string(first) {
			t.Fatalf("run %d: output differs from first run", i)
		}
	}
}
