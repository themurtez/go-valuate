package ledger

import "testing"

func TestValidateReversals_Consistent(t *testing.T) {
	orig := balancedEntry()
	orig.ID = "JE-ORIG"
	orig.Reversal.ReversedByEntryID = "JE-REV"

	rev := balancedEntry()
	rev.ID = "JE-REV"
	rev.Lines[0].AccountID, rev.Lines[1].AccountID = rev.Lines[1].AccountID, rev.Lines[0].AccountID // opposite postings
	rev.Reversal.ReversalOfEntryID = "JE-ORIG"

	issues := ValidateEntries([]JournalEntry{orig, rev}, testChart(), ValidateOptions{})
	if hasCode(issues, IssueInvalidReversal) {
		t.Fatalf("expected consistent reversal pair to have no IssueInvalidReversal, got %+v", issues)
	}
}

func TestValidateReversals_UnknownTarget(t *testing.T) {
	rev := balancedEntry()
	rev.ID = "JE-REV"
	rev.Reversal.ReversalOfEntryID = "JE-DOES-NOT-EXIST"

	issues := ValidateEntries([]JournalEntry{rev}, testChart(), ValidateOptions{})
	if !hasCode(issues, IssueInvalidReversal) {
		t.Fatalf("expected IssueInvalidReversal for unknown reversal target, got %+v", issues)
	}
}

func TestValidateReversals_OneSided(t *testing.T) {
	orig := balancedEntry()
	orig.ID = "JE-ORIG"
	// orig does NOT declare ReversedByEntryID

	rev := balancedEntry()
	rev.ID = "JE-REV"
	rev.Reversal.ReversalOfEntryID = "JE-ORIG"

	issues := ValidateEntries([]JournalEntry{orig, rev}, testChart(), ValidateOptions{})
	if !hasCode(issues, IssueInvalidReversal) {
		t.Fatalf("expected IssueInvalidReversal for a one-sided reversal claim, got %+v", issues)
	}
}

func TestReversal_NeverInferredFromEqualAndOpposite(t *testing.T) {
	// Two entries that are equal-and-opposite in amount but declare no
	// Reversal relationship at all must not be treated as a reversal pair
	// by anything in this package — there is no function that would even
	// attempt to infer one, but this test documents/locks that contract:
	// both entries post normally and independently affect balances.
	a := balancedEntry()
	a.ID = "JE-A"
	b := balancedEntry()
	b.ID = "JE-B"
	b.Lines[0].AccountID, b.Lines[1].AccountID = b.Lines[1].AccountID, b.Lines[0].AccountID

	balances := CalculateBalances(testChart(), []JournalEntry{a, b}, BalanceOptions{})
	cash := findBalance(t, balances, "1000")
	// a debits 1000 cash; b credits 1000 cash (opposite). Net should be 0,
	// from two independent postings, not from special reversal handling.
	if cash.RawBalance != 0 {
		t.Fatalf("expected net-zero cash balance from two independent opposite postings, got %v", cash.RawBalance)
	}
	if cash.SourceEntryCount != 2 {
		t.Fatalf("expected both entries counted as independent postings, got SourceEntryCount=%d", cash.SourceEntryCount)
	}
}
