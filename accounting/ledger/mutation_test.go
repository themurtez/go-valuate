package ledger

import (
	"encoding/json"
	"testing"
)

// TestNoMutation_Accounts proves ValidateAccounts/BuildChartOfAccounts
// never mutate the caller-owned accounts slice, mirroring
// transactions/dealstructure's TestBuild_NoMutationOfInput pattern.
func TestNoMutation_Accounts(t *testing.T) {
	accounts := hierarchyChart()
	before, err := json.Marshal(accounts)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	_ = ValidateAccounts(accounts)
	chart := BuildChartOfAccounts(accounts)
	_ = chart.Descendants("1000")
	_ = chart.Children("1000")

	after, err := json.Marshal(accounts)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if string(before) != string(after) {
		t.Fatalf("accounts mutated:\nbefore: %s\nafter:  %s", before, after)
	}
}

// TestNoMutation_EntriesAndLines proves ValidateEntries/CalculateBalances/
// BuildTrialBalance never mutate caller-owned entries, lines, or
// dimensions.
func TestNoMutation_EntriesAndLines(t *testing.T) {
	entries := []JournalEntry{
		{
			ID: "JE-1", Date: "2025-01-01", Period: "2025", Status: StatusPosted,
			Lines: []JournalLine{
				{ID: "L1", AccountID: "1000", Debit: 1000, Dimensions: []Dimension{{Key: DimensionDepartment, Value: "Sales"}}},
				{ID: "L2", AccountID: "4000", Credit: 1000},
			},
		},
	}
	before, err := json.Marshal(entries)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	chart := testChart()
	_ = ValidateEntries(entries, chart, ValidateOptions{})
	_ = CalculateBalances(chart, entries, BalanceOptions{})
	_ = BuildTrialBalance(chart, entries, BalanceOptions{}, ModeEndingBalances, 0)

	after, err := json.Marshal(entries)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if string(before) != string(after) {
		t.Fatalf("entries mutated:\nbefore: %s\nafter:  %s", before, after)
	}
}

// TestNoMutation_OpeningBalances proves CalculateBalances/BuildTrialBalance
// never mutate the caller-owned Openings slice.
func TestNoMutation_OpeningBalances(t *testing.T) {
	openings := []OpeningBalance{{AccountID: "1000", Debit: 5000}}
	before, err := json.Marshal(openings)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	opts := BalanceOptions{Openings: openings}
	_ = CalculateBalances(testChart(), nil, opts)

	after, err := json.Marshal(openings)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if string(before) != string(after) {
		t.Fatalf("openings mutated:\nbefore: %s\nafter:  %s", before, after)
	}
}

// TestNoMutation_TrialBalanceInput proves NormalizeTrialBalance never
// mutates its input.
func TestNoMutation_TrialBalanceInput(t *testing.T) {
	in := TrialBalanceInput{
		Period: "2025",
		Lines: []TrialBalanceInputLine{
			{AccountID: "1000", Debit: 100, Dimensions: []Dimension{{Key: DimensionClass, Value: "Retail"}}},
			{AccountID: "3000", Credit: 100},
		},
	}
	before, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	_ = NormalizeTrialBalance(in, testChart(), 0)

	after, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if string(before) != string(after) {
		t.Fatalf("TrialBalanceInput mutated:\nbefore: %s\nafter:  %s", before, after)
	}
}
