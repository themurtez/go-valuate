package journaldiagnostics_test

import (
	"strconv"
	"testing"

	"github.com/themurtez/go-valuate/accounting/journaldiagnostics"
	"github.com/themurtez/go-valuate/accounting/journaldiagnostics/fixtures"
)

func groupContainingAll(groups []journaldiagnostics.DuplicateGroup, ids ...string) *journaldiagnostics.DuplicateGroup {
	want := make(map[string]bool, len(ids))
	for _, id := range ids {
		want[id] = true
	}
	for i := range groups {
		g := groups[i]
		if len(g.EntryIDs) != len(want) {
			continue
		}
		match := true
		for _, id := range g.EntryIDs {
			if !want[id] {
				match = false
				break
			}
		}
		if match {
			return &g
		}
	}
	return nil
}

func TestExactDuplicate_DetectsIdenticalEntries(t *testing.T) {
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	found := findingsWithCode(r.Findings, journaldiagnostics.FindingExactDuplicateEntry)
	matched := false
	for _, f := range found {
		if containsEntryID(f, "JE-DUP-01") && containsEntryID(f, "JE-DUP-02") {
			matched = true
		}
	}
	if !matched {
		t.Error("expected JE-DUP-01/JE-DUP-02 (identical date+lines) to be flagged as EXACT_DUPLICATE_ENTRY")
	}

	g := groupContainingAll(r.DuplicateGroups, "JE-DUP-01", "JE-DUP-02")
	if g == nil {
		t.Fatal("expected a DuplicateGroup for JE-DUP-01/JE-DUP-02")
	}
	if !g.Exact {
		t.Error("expected Exact=true for identical-date, identical-content duplicates")
	}
}

func TestExactDuplicate_SignatureExcludesEntryID(t *testing.T) {
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	g := groupContainingAll(r.DuplicateGroups, "JE-DUP-01", "JE-DUP-02")
	if g == nil {
		t.Fatal("expected a DuplicateGroup for JE-DUP-01/JE-DUP-02")
	}
	if len(g.NormalizedSignature) == 0 {
		t.Fatal("expected a non-empty NormalizedSignature")
	}
	// The signature must not literally contain either entry's own ID.
	for _, id := range []string{"JE-DUP-01", "JE-DUP-02"} {
		if containsSubstring(g.NormalizedSignature, id) {
			t.Errorf("duplicate signature must not include EntryID, got signature %q containing %q", g.NormalizedSignature, id)
		}
	}
}

func containsSubstring(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestPossibleDuplicate_DetectsNearDuplicateWithinWindow(t *testing.T) {
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	found := findingsWithCode(r.Findings, journaldiagnostics.FindingPossibleDuplicateEntry)
	matched := false
	for _, f := range found {
		if containsEntryID(f, "JE-NEARDUP-01") && containsEntryID(f, "JE-NEARDUP-02") {
			matched = true
		}
	}
	if !matched {
		t.Error("expected JE-NEARDUP-01/02 (same content, 2 days apart, within DuplicateWindowDays=3) to be flagged as POSSIBLE_DUPLICATE_ENTRY")
	}
}

func TestPossibleDuplicate_OutsideWindowNotFlagged(t *testing.T) {
	policy := fixtures.Policy()
	policy.DuplicateWindowDays = 1 // JE-NEARDUP-01/02 are 2 days apart
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), policy)
	for _, f := range findingsWithCode(r.Findings, journaldiagnostics.FindingPossibleDuplicateEntry) {
		if containsEntryID(f, "JE-NEARDUP-01") && containsEntryID(f, "JE-NEARDUP-02") {
			t.Error("JE-NEARDUP-01/02 are 2 days apart and must not match a 1-day window")
		}
	}
}

func TestPossibleDuplicate_NeverDoubleCountedWithExactDuplicate(t *testing.T) {
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	exactIDs := make(map[string]bool)
	for _, f := range findingsWithCode(r.Findings, journaldiagnostics.FindingExactDuplicateEntry) {
		for _, id := range f.EntryIDs {
			exactIDs[id] = true
		}
	}
	for _, f := range findingsWithCode(r.Findings, journaldiagnostics.FindingPossibleDuplicateEntry) {
		for _, id := range f.EntryIDs {
			if exactIDs[id] {
				t.Errorf("entry %s appears in both EXACT_DUPLICATE_ENTRY and POSSIBLE_DUPLICATE_ENTRY findings; the same pair must not match both", id)
			}
		}
	}
}

func TestRepeatedIdenticalAmount_DetectsRecurringAmountAcrossEntries(t *testing.T) {
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	found := findingsWithCode(r.Findings, journaldiagnostics.FindingRepeatedIdenticalAmount)
	matched := false
	for _, f := range found {
		if containsEntryID(f, "JE-REPEAT-01") && containsEntryID(f, "JE-REPEAT-02") && containsEntryID(f, "JE-REPEAT-03") {
			matched = true
		}
	}
	if !matched {
		t.Error("expected JE-REPEAT-01/02/03 ($1,234 each, 3 occurrences) to be flagged as REPEATED_IDENTICAL_AMOUNT")
	}
}

func TestRepeatedIdenticalAmount_RecurringSourceSuppressed(t *testing.T) {
	// JE-RECUR-01/02/03 share the same $500 amount 3 times (matching
	// MinRepeatedAmountCount), but their metadata declares
	// Source=RECURRING, so they must be suppressed from this rule — the
	// task's explicit "do not conflate legitimate recurring journals with
	// suspicious duplication" instruction.
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	for _, f := range findingsWithCode(r.Findings, journaldiagnostics.FindingRepeatedIdenticalAmount) {
		if containsEntryID(f, "JE-RECUR-01") {
			t.Error("recurring-sourced entries must be suppressed from REPEATED_IDENTICAL_AMOUNT")
		}
	}
}

func TestRepeatedIdenticalAmount_BelowMinCountNotFlagged(t *testing.T) {
	policy := fixtures.Policy()
	policy.MinRepeatedAmountCount = 10 // higher than any fixture group
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), policy)
	if len(findingsWithCode(r.Findings, journaldiagnostics.FindingRepeatedIdenticalAmount)) != 0 {
		t.Error("expected zero REPEATED_IDENTICAL_AMOUNT findings when MinRepeatedAmountCount exceeds every group size")
	}
}

func TestDuplicateDetection_ManyDistinctEntriesProduceNoFalseDuplicates(t *testing.T) {
	// A correctness proxy for the "no O(N^2) pairwise comparison" design
	// requirement: with a large synthetic set of entries that are all
	// distinct (each a different amount), duplicate detection must complete
	// quickly and report zero groups. Actual performance scaling is covered
	// separately by benchmark_test.go.
	l := fixtures.Ledger()
	for i := 0; i < 500; i++ {
		l.Entries = append(l.Entries, journalEntryHelper(uniqueID(i), "2025-03-19", float64(1000+i)))
	}
	r := journaldiagnostics.Calculate(l, fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	for _, g := range r.DuplicateGroups {
		for _, id := range g.EntryIDs {
			if len(id) >= 7 && id[:7] == "JE-BULK" {
				t.Errorf("bulk-generated entries are all distinct amounts and must never form a duplicate group, got group %+v", g)
			}
		}
	}
}

func uniqueID(i int) string {
	return "JE-BULK-" + strconv.Itoa(i)
}
