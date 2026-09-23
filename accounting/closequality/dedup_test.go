package closequality_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/closequality"
	"github.com/themurtez/go-valuate/accounting/closequality/fixtures"
)

// TestDedup_ARControlMismatch_SingleFinding proves the AR control
// mismatch condition, which this package could in principle attribute
// through more than one code path, appears exactly once in the combined
// Blockers+Warnings+Information list.
func TestDedup_ARControlMismatch_SingleFinding(t *testing.T) {
	in := fixtures.CleanCloseInput()
	in.AR = fixtures.ARResultControlMismatch()

	result := closequality.Calculate(in, fixtures.CleanPolicy())

	count := 0
	for _, f := range allFindings(result) {
		if f.Code == closequality.FindingARControlMismatch {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly one AR_CONTROL_MISMATCH finding after dedup, got %d", count)
	}
}

// TestDedup_DistinctAccounts_NotMerged proves two ClearingAccountNotCleared
// findings for two *different* accounts are never merged into one — dedup
// keys on AccountID and must not conflate distinct conditions that merely
// share a Code and Dimension.
func TestDedup_DistinctAccounts_NotMerged(t *testing.T) {
	materiality := 1.0
	policy := closequality.DefaultPolicy()
	policy.AccountExpectations = []closequality.AccountExpectation{
		{AccountID: "1000", ShouldClear: true, Materiality: &materiality, Label: "cash treated as clearing for this test"},
		{AccountID: "3900", ShouldClear: true, Materiality: &materiality, Label: "retained earnings treated as clearing for this test"},
	}

	in := fixtures.CleanCloseInput()
	in.CloseTasks = nil
	result := closequality.Calculate(in, policy)

	accounts := map[string]int{}
	for _, f := range allFindings(result) {
		if f.Code == closequality.FindingClearingAccountNotCleared {
			if len(f.AccountIDs) == 1 {
				accounts[f.AccountIDs[0]]++
			}
		}
	}
	if len(accounts) < 2 {
		t.Fatalf("expected clearing findings for at least 2 distinct accounts (fixture has nonzero balances in both 1000 and 3900), got %v", accounts)
	}
	for acct, n := range accounts {
		if n > 1 {
			t.Errorf("account %s has %d CLEARING_ACCOUNT_NOT_CLEARED findings, expected exactly 1", acct, n)
		}
	}
}

func allFindings(r closequality.Result) []closequality.Finding {
	out := append([]closequality.Finding{}, r.Blockers...)
	out = append(out, r.Warnings...)
	out = append(out, r.Information...)
	return out
}
