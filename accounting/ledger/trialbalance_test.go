package ledger

import "testing"

func balancedLedgerEntries() []JournalEntry {
	return []JournalEntry{
		{ID: "JE-1", Date: "2025-01-01", Period: "2025", Status: StatusPosted, Lines: []JournalLine{
			{AccountID: "1000", Debit: 10000},
			{AccountID: "3000", Credit: 10000},
		}},
		{ID: "JE-2", Date: "2025-02-01", Period: "2025", Status: StatusPosted, Lines: []JournalLine{
			{AccountID: "1100", Debit: 2000},
			{AccountID: "4000", Credit: 2000},
		}},
		{ID: "JE-3", Date: "2025-03-01", Period: "2025", Status: StatusPosted, Lines: []JournalLine{
			{AccountID: "5000", Debit: 500},
			{AccountID: "1000", Credit: 500},
		}},
	}
}

func TestBuildTrialBalance_Balanced(t *testing.T) {
	tb := BuildTrialBalance(testChart(), balancedLedgerEntries(), BalanceOptions{}, ModeEndingBalances, 0)
	if !tb.Balanced {
		t.Fatalf("expected balanced trial balance, diff=%v issues=%+v", tb.Difference, tb.Issues)
	}
	if tb.TotalDebits != tb.TotalCredits {
		t.Fatalf("total debits (%v) should equal total credits (%v)", tb.TotalDebits, tb.TotalCredits)
	}
	if hasCode(tb.Issues, IssueUnbalancedTrialBalance) {
		t.Fatalf("balanced TB should not carry IssueUnbalancedTrialBalance: %+v", tb.Issues)
	}
}

func TestBuildTrialBalance_Unbalanced(t *testing.T) {
	entries := balancedLedgerEntries()
	entries[0].Lines[1].Credit = 9000 // break balance deliberately (entry itself becomes unbalanced too)
	tb := BuildTrialBalance(testChart(), entries, BalanceOptions{}, ModeEndingBalances, 0)
	if tb.Balanced {
		t.Fatal("expected unbalanced trial balance")
	}
	if !hasCode(tb.Issues, IssueUnbalancedTrialBalance) {
		t.Fatalf("expected IssueUnbalancedTrialBalance, got %+v", tb.Issues)
	}
	// Never forced to true.
	if tb.Difference == 0 {
		t.Fatal("expected a nonzero difference to be reported, not silently corrected")
	}
}

func TestBuildTrialBalance_ActivityVsEnding(t *testing.T) {
	entries := balancedLedgerEntries()
	opts := BalanceOptions{Range: PeriodRange{StartDate: "2025-01-01", EndDate: "2025-12-31"}}

	activity := BuildTrialBalance(testChart(), entries, opts, ModePeriodActivity, 0)
	ending := BuildTrialBalance(testChart(), entries, opts, ModeEndingBalances, 0)

	var activityCashLine, endingCashLine TrialBalanceLine
	for _, l := range activity.Lines {
		if l.AccountID == "1000" {
			activityCashLine = l
		}
	}
	for _, l := range ending.Lines {
		if l.AccountID == "1000" {
			endingCashLine = l
		}
	}
	if activityCashLine.PeriodDebit == 0 {
		t.Fatal("expected ModePeriodActivity to populate PeriodDebit")
	}
	if endingCashLine.PeriodDebit != 0 || endingCashLine.PeriodCredit != 0 {
		t.Fatalf("expected ModeEndingBalances to zero PeriodDebit/PeriodCredit, got %+v", endingCashLine)
	}
	// Closing figures should match between modes regardless.
	if activityCashLine.RawBalance != endingCashLine.RawBalance {
		t.Fatalf("closing balance should be identical between modes: %v vs %v", activityCashLine.RawBalance, endingCashLine.RawBalance)
	}
}

func TestBuildTrialBalance_NeverForcesBalanced(t *testing.T) {
	// Directly construct an obviously-unbalanced ledger via entries with
	// mismatched (but individually valid-looking) postings across
	// different accounts, to prove BuildTrialBalance reports reality
	// rather than adjusting anything.
	entries := []JournalEntry{
		{ID: "JE-1", Date: "2025-01-01", Status: StatusPosted, Lines: []JournalLine{
			{AccountID: "1000", Debit: 100},
			{AccountID: "4000", Credit: 50}, // deliberately unbalanced entry
		}},
	}
	// This entry itself would fail ValidateEntries (IssueUnbalancedEntry),
	// but BuildTrialBalance doesn't refuse to build a report over
	// already-invalid data — it reports what's there.
	tb := BuildTrialBalance(testChart(), entries, BalanceOptions{}, ModeEndingBalances, 0)
	if tb.Balanced {
		t.Fatal("expected Balanced=false; must never be forced true")
	}
	if tb.TotalDebits == tb.TotalCredits {
		t.Fatal("test setup error: expected totals to actually differ")
	}
}

func TestBuildTrialBalance_Deterministic(t *testing.T) {
	entries := balancedLedgerEntries()
	first := BuildTrialBalance(testChart(), entries, BalanceOptions{}, ModeEndingBalances, 0.01)
	for i := 0; i < 10; i++ {
		got := BuildTrialBalance(testChart(), entries, BalanceOptions{}, ModeEndingBalances, 0.01)
		if got.TotalDebits != first.TotalDebits || got.TotalCredits != first.TotalCredits {
			t.Fatalf("run %d: totals differ", i)
		}
	}
}
