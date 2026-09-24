package reconciliation

import (
	"encoding/json"
	"testing"
)

// TestImmutability_CalculateNeverMutatesInput builds an Input, snapshots
// its JSON, calls Calculate, and confirms the Input's JSON is byte-for-
// byte identical afterward — locking the "no mutation of caller-owned
// input" guarantee for every slice/map Input contains.
func TestImmutability_CalculateNeverMutatesInput(t *testing.T) {
	in := buildDeterminismInput()
	before, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	_ = Calculate(in)

	after, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	if string(before) != string(after) {
		t.Fatalf("Calculate mutated its Input:\nbefore=%s\nafter=%s", before, after)
	}
}

func TestImmutability_ConfirmedMatchesNotMutated(t *testing.T) {
	explained := 5.0
	confirmed := []ConfirmedMatch{
		{MatchID: "M1", BookItemIDs: []string{"B1"}, ExternalItemIDs: []string{"E1"}, ExplainedDifference: &explained},
	}
	before, _ := json.Marshal(confirmed)

	book := testBookByID(BookItem{ItemID: "B1", Amount: 100, Direction: DirectionOutflow})
	external := testExternalByID(ExternalItem{ItemID: "E1", Amount: 105, Direction: DirectionOutflow})
	_, _ = validateConfirmedMatches(confirmed, book, external, MatchingPolicy{})

	after, _ := json.Marshal(confirmed)
	if string(before) != string(after) {
		t.Fatalf("validateConfirmedMatches mutated confirmed matches:\nbefore=%s\nafter=%s", before, after)
	}
}

func TestImmutability_PolicyNotMutated(t *testing.T) {
	policy := DefaultMatchingPolicy()
	before, _ := json.Marshal(policy)
	_ = validatePolicy(policy)
	after, _ := json.Marshal(policy)
	if string(before) != string(after) {
		t.Fatalf("validatePolicy mutated policy:\nbefore=%s\nafter=%s", before, after)
	}
}
