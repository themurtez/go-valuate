package closequality_test

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/themurtez/go-valuate/accounting/closequality"
	"github.com/themurtez/go-valuate/accounting/closequality/fixtures"
)

// TestRoundTrip_JSON proves Result survives a JSON marshal/unmarshal
// round trip with no field loss for a Result carrying a comparison.
func TestRoundTrip_JSON(t *testing.T) {
	in := fixtures.CleanCloseInput()
	policy := fixtures.CleanPolicy()
	prior := closequality.Calculate(in, policy)

	mutated := fixtures.CleanCloseInput()
	mutated.AR = fixtures.ARResultControlMismatch()
	result := closequality.CalculateWithPrior(mutated, policy, prior)

	b, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var roundTripped closequality.Result
	if err := json.Unmarshal(b, &roundTripped); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	b2, err := json.Marshal(roundTripped)
	if err != nil {
		t.Fatalf("re-marshal: %v", err)
	}
	if string(b) != string(b2) {
		t.Errorf("round trip changed JSON:\nbefore: %s\nafter:  %s", b, b2)
	}
}

// TestRoundTrip_SnakeCaseTags spot-checks that JSON keys are snake_case,
// matching repo convention.
func TestRoundTrip_SnakeCaseTags(t *testing.T) {
	result := closequality.Calculate(fixtures.CleanCloseInput(), fixtures.CleanPolicy())
	b, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, key := range []string{`"status"`, `"coverage"`, `"dimension_coverage"`, `"close_task_summary"`, `"schema_version"`, `"formula_version"`} {
		if !strings.Contains(string(b), key) {
			t.Errorf("expected JSON to contain key %s", key)
		}
	}
	if strings.Contains(string(b), `"Status"`) {
		t.Error("found PascalCase key in JSON output")
	}
}

// TestNoNaNOrInf proves Calculate never produces a Result containing a
// NaN or Inf float, across every Value/Evidence numeric field, by
// round-tripping through JSON (encoding/json itself errors on NaN/Inf,
// so a successful Marshal already proves this — this test documents the
// invariant explicitly and also directly fuzzes a non-finite policy
// input to prove it is rejected via Issues rather than propagated).
func TestNoNaNOrInf(t *testing.T) {
	in := fixtures.CleanCloseInput()
	policy := fixtures.CleanPolicy()
	policy.Materiality.AbsoluteAmount = math.NaN()

	result := closequality.Calculate(in, policy)

	if _, err := json.Marshal(result); err != nil {
		t.Fatalf("Result with non-finite policy input produced non-JSON-safe output: %v", err)
	}
	found := false
	for _, iss := range result.Issues {
		if iss.Code == closequality.IssueNonFiniteValue {
			found = true
		}
	}
	if !found {
		t.Errorf("expected IssueNonFiniteValue for NaN materiality threshold, got issues=%+v", result.Issues)
	}
}
