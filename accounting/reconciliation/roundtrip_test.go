package reconciliation

import (
	"encoding/json"
	"testing"
)

// TestRoundtrip_ResultSurvivesJSON confirms Result round-trips through
// JSON without losing information relevant to equality (spot-checked
// field by field rather than full deep-equal, since a few fields
// legitimately zero-value/omitempty round-trip to Go zero values that
// are semantically identical either way).
func TestRoundtrip_ResultSurvivesJSON(t *testing.T) {
	in := buildDeterminismInput()
	r := Calculate(in)

	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var round Result
	if err := json.Unmarshal(b, &round); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if round.AccountID != r.AccountID {
		t.Errorf("AccountID mismatch: %q vs %q", round.AccountID, r.AccountID)
	}
	if round.Status != r.Status {
		t.Errorf("Status mismatch: %q vs %q", round.Status, r.Status)
	}
	if len(round.MatchedGroups) != len(r.MatchedGroups) {
		t.Errorf("MatchedGroups count mismatch: %d vs %d", len(round.MatchedGroups), len(r.MatchedGroups))
	}
	if round.Versions != r.Versions {
		t.Errorf("Versions mismatch: %+v vs %+v", round.Versions, r.Versions)
	}

	// Re-marshal the round-tripped value and confirm it matches the
	// original byte-for-byte (the strongest single check).
	b2, err := json.Marshal(round)
	if err != nil {
		t.Fatalf("re-marshal error: %v", err)
	}
	if string(b) != string(b2) {
		t.Fatalf("round-trip did not reproduce identical JSON:\noriginal=%s\nround-tripped=%s", b, b2)
	}
}

// TestRoundtrip_InputSurvivesJSON confirms Input itself round-trips,
// since callers persist/replay Input as often as Result.
func TestRoundtrip_InputSurvivesJSON(t *testing.T) {
	in := buildDeterminismInput()
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var round Input
	if err := json.Unmarshal(b, &round); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	b2, err := json.Marshal(round)
	if err != nil {
		t.Fatalf("re-marshal error: %v", err)
	}
	if string(b) != string(b2) {
		t.Fatalf("Input round-trip did not reproduce identical JSON:\noriginal=%s\nround-tripped=%s", b, b2)
	}
}

// TestRoundtrip_SnakeCaseTags spot-checks that every top-level JSON key
// in a Result is snake_case, never camelCase/PascalCase leaking through
// an untagged field (task section 67).
func TestRoundtrip_SnakeCaseTags(t *testing.T) {
	in := buildDeterminismInput()
	r := Calculate(in)
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	for key := range raw {
		for _, c := range key {
			if c >= 'A' && c <= 'Z' {
				t.Errorf("top-level key %q is not snake_case", key)
			}
		}
	}
}
