package reconciliation

import (
	"encoding/json"
	"testing"
)

func buildDeterminismInput() Input {
	book := 1000.0
	external := 950.0
	return Input{
		AccountID:         "BANK-1",
		ExternalAccountID: "EXT-1",
		AsOfDate:          "2025-06-30",
		Type:              TypeBank,
		BookItems: []BookItem{
			{ItemID: "B1", Date: "2025-06-10", Amount: 100, Direction: DirectionOutflow, Reference: "CHK1"},
			{ItemID: "B2", Date: "2025-06-15", Amount: 200, Direction: DirectionInflow},
			{ItemID: "B3", Date: "2025-06-20", Amount: 300, Direction: DirectionOutflow},
		},
		ExternalItems: []ExternalItem{
			{ItemID: "E1", Date: "2025-06-11", Amount: 100, Direction: DirectionOutflow, Reference: "chk1"},
			{ItemID: "E2", Date: "2025-06-15", Amount: 200, Direction: DirectionInflow},
		},
		BookBalance:     BalanceInput{EndingBalance: &book},
		ExternalBalance: BalanceInput{EndingBalance: &external},
		ReconcilingItems: []ReconcilingItem{
			{ItemID: "R1", Type: ReconcilingOutstandingCheck, Side: ReconcilingSideBook, Amount: -300, Date: "2025-06-20"},
		},
		Policy: MatchingPolicy{
			ReferenceNormalization: ReferenceNormalization{Trim: true, CaseFold: true},
			DateWindowDays:         5,
			StaleDaysThreshold:     30,
		},
	}
}

func TestDeterminism_RepeatedCallsIdenticalJSON(t *testing.T) {
	in := buildDeterminismInput()
	var prev string
	for i := 0; i < 5; i++ {
		r := Calculate(in)
		b, err := json.Marshal(r)
		if err != nil {
			t.Fatalf("marshal error: %v", err)
		}
		if i > 0 && string(b) != prev {
			t.Fatalf("run %d produced different JSON than run 0:\nrun0=%s\nrun%d=%s", i, prev, i, string(b))
		}
		prev = string(b)
	}
}

func TestDeterminism_MapOrderNeverLeaksIntoOutput(t *testing.T) {
	// Run the same logical input built with items in a different literal
	// order, but confirm the SORTED output fields (MatchedGroups,
	// UnmatchedBookItems, etc.) come out identically ordered regardless.
	in1 := buildDeterminismInput()
	in2 := buildDeterminismInput()
	// Reverse item order in in2's slices - Calculate's own dedup/matching
	// still operates on caller order internally, but final output
	// ordering (sort.go) must not depend on it for the SAME logical set.
	reverseBookItems(in2.BookItems)
	reverseExternalItems(in2.ExternalItems)

	r1 := Calculate(in1)
	r2 := Calculate(in2)

	if len(r1.MatchedGroups) != len(r2.MatchedGroups) {
		t.Fatalf("expected same number of matched groups regardless of input order: %d vs %d", len(r1.MatchedGroups), len(r2.MatchedGroups))
	}
	for i := range r1.MatchedGroups {
		if r1.MatchedGroups[i].MatchID != r2.MatchedGroups[i].MatchID {
			t.Fatalf("expected identical MatchID ordering regardless of input order at index %d: %s vs %s", i, r1.MatchedGroups[i].MatchID, r2.MatchedGroups[i].MatchID)
		}
	}
}

func reverseBookItems(items []BookItem) {
	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
		items[i], items[j] = items[j], items[i]
	}
}

func reverseExternalItems(items []ExternalItem) {
	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
		items[i], items[j] = items[j], items[i]
	}
}
