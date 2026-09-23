package ledger

import (
	"encoding/json"
	"testing"
)

func roundTripJSON[T any](t *testing.T, label string, v T) {
	t.Helper()
	first, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("%s: json.Marshal failed: %v", label, err)
	}
	if !json.Valid(first) {
		t.Fatalf("%s: expected valid JSON output", label)
	}
	var roundTripped T
	if err := json.Unmarshal(first, &roundTripped); err != nil {
		t.Fatalf("%s: json.Unmarshal failed: %v", label, err)
	}
	second, err := json.Marshal(roundTripped)
	if err != nil {
		t.Fatalf("%s: re-marshal failed: %v", label, err)
	}
	if string(first) != string(second) {
		t.Fatalf("%s: round-trip mismatch:\nfirst:  %s\nsecond: %s", label, first, second)
	}
}

func TestJSONRoundTrip_Account(t *testing.T) {
	roundTripJSON(t, "Account", Account{
		ID: "1000", Number: "1000", Name: "Cash", Type: AccountAsset,
		Subtype: "current_asset", ParentID: "1000-PARENT", Currency: "USD", Active: true,
	})
}

func TestJSONRoundTrip_JournalEntry(t *testing.T) {
	roundTripJSON(t, "JournalEntry", JournalEntry{
		ID: "JE-1", Date: "2025-01-01", Period: "2025", Description: "Test entry",
		Source: "manual", Reference: "REF-1", ExternalReference: "EXT-1", Status: StatusPosted,
		Reversal: Reversal{ReversedByEntryID: "JE-2"},
		Lines: []JournalLine{
			{ID: "L1", AccountID: "1000", Debit: 1000, Memo: "note",
				Dimensions: []Dimension{{Key: DimensionDepartment, Value: "Sales"}}, SourceRef: "src-1"},
			{ID: "L2", AccountID: "4000", Credit: 1000},
		},
	})
}

func TestJSONRoundTrip_Ledger(t *testing.T) {
	roundTripJSON(t, "Ledger", Ledger{
		Accounts: validChart(),
		Entries:  balancedLedgerEntries(),
	})
}

func TestJSONRoundTrip_TrialBalanceInput(t *testing.T) {
	roundTripJSON(t, "TrialBalanceInput", TrialBalanceInput{
		Period: "2025",
		Lines: []TrialBalanceInputLine{
			{AccountID: "1000", Debit: 1000, OpeningBalance: &OpeningBalance{AccountID: "1000", Debit: 500}, Currency: "USD"},
			{AccountID: "3000", Credit: 1000},
		},
	})
}

func TestJSONRoundTrip_NormalizedTrialBalance(t *testing.T) {
	in := TrialBalanceInput{
		Period: "2025",
		Lines: []TrialBalanceInputLine{
			{AccountID: "1000", Debit: 1000},
			{AccountID: "3000", Credit: 1000},
		},
	}
	roundTripJSON(t, "NormalizedTrialBalance", NormalizeTrialBalance(in, testChart(), 0))
}

func TestJSONRoundTrip_Balance(t *testing.T) {
	balances := CalculateBalances(testChart(), balancedLedgerEntries(), BalanceOptions{})
	roundTripJSON(t, "[]Balance", balances)
}

func TestJSONRoundTrip_TrialBalance(t *testing.T) {
	tb := BuildTrialBalance(testChart(), balancedLedgerEntries(), BalanceOptions{}, ModeEndingBalances, 0.01)
	roundTripJSON(t, "TrialBalance", tb)
}

func TestJSONRoundTrip_RollupBalance(t *testing.T) {
	chart := BuildChartOfAccounts(hierarchyChart())
	balances := CalculateBalances(chart, balancedLedgerEntries(), BalanceOptions{})
	roundTripJSON(t, "[]RollupBalance", BuildRollups(chart, balances))
}

func TestJSONRoundTrip_Issues(t *testing.T) {
	issues := ValidateAccounts([]Account{{ID: "1000", Type: AccountType("BOGUS")}})
	roundTripJSON(t, "[]Issue", issues)
}

func TestJSONRoundTrip_IntegrityReport(t *testing.T) {
	l := Ledger{Accounts: validChart(), Entries: balancedLedgerEntries()}
	roundTripJSON(t, "IntegrityReport", CheckLedgerIntegrity(l, ValidateOptions{}))
}

func TestJSONRoundTrip_NoNaNInfInValidOutput(t *testing.T) {
	tb := BuildTrialBalance(testChart(), balancedLedgerEntries(), BalanceOptions{}, ModeEndingBalances, 0.01)
	b, err := json.Marshal(tb)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	s := string(b)
	for _, bad := range []string{"NaN", "Infinity", "-Infinity"} {
		if contains(s, bad) {
			t.Fatalf("valid TrialBalance JSON output contains %q: %s", bad, s)
		}
	}
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
