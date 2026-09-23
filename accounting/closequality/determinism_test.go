package closequality_test

import (
	"encoding/json"
	"testing"

	"github.com/themurtez/go-valuate/accounting/closequality"
	"github.com/themurtez/go-valuate/accounting/closequality/fixtures"
)

// TestDeterminism_CleanClose proves repeated Calculate calls against the
// same input produce byte-for-byte identical JSON.
func TestDeterminism_CleanClose(t *testing.T) {
	in := fixtures.CleanCloseInput()
	policy := fixtures.CleanPolicy()

	first := mustMarshal(t, closequality.Calculate(in, policy))
	for i := 0; i < 20; i++ {
		next := mustMarshal(t, closequality.Calculate(in, policy))
		if first != next {
			t.Fatalf("Calculate produced non-identical JSON on iteration %d", i)
		}
	}
}

// TestDeterminism_WithPrior proves CalculateWithPrior is equally
// deterministic.
func TestDeterminism_WithPrior(t *testing.T) {
	in := fixtures.CleanCloseInput()
	policy := fixtures.CleanPolicy()
	prior := closequality.Calculate(in, policy)

	mutated := fixtures.CleanCloseInput()
	mutated.AR = fixtures.ARResultControlMismatch()

	first := mustMarshal(t, closequality.CalculateWithPrior(mutated, policy, prior))
	for i := 0; i < 10; i++ {
		next := mustMarshal(t, closequality.CalculateWithPrior(mutated, policy, prior))
		if first != next {
			t.Fatalf("CalculateWithPrior produced non-identical JSON on iteration %d", i)
		}
	}
}

func mustMarshal(t *testing.T, v interface{}) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return string(b)
}
