package profitability_test

import (
	"encoding/json"
	"testing"

	"github.com/themurtez/go-valuate/accounting/profitability"
)

// TestImmutability_CalculateDoesNotMutateInput proves Calculate never
// mutates any field of Input (or nested slices/maps within it).
func TestImmutability_CalculateDoesNotMutateInput(t *testing.T) {
	in := fullInput()
	policy := profitability.DefaultPolicy()

	before, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("Marshal before: %v", err)
	}

	_ = profitability.Calculate(in, policy)

	after, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("Marshal after: %v", err)
	}
	if string(before) != string(after) {
		t.Fatalf("Calculate mutated Input:\nbefore=%s\nafter=%s", before, after)
	}
}

// TestImmutability_CalculateDoesNotMutatePolicy proves Calculate never
// mutates its Policy argument.
func TestImmutability_CalculateDoesNotMutatePolicy(t *testing.T) {
	in := fullInput()
	policy := profitability.DefaultPolicy()

	before, err := json.Marshal(policy)
	if err != nil {
		t.Fatalf("Marshal before: %v", err)
	}

	_ = profitability.Calculate(in, policy)

	after, err := json.Marshal(policy)
	if err != nil {
		t.Fatalf("Marshal after: %v", err)
	}
	if string(before) != string(after) {
		t.Fatalf("Calculate mutated Policy:\nbefore=%s\nafter=%s", before, after)
	}
}

// TestImmutability_ResultMutationDoesNotAffectNextCall proves mutating a
// returned Result does not corrupt a subsequent Calculate call (no
// shared backing arrays/maps between calls).
func TestImmutability_ResultMutationDoesNotAffectNextCall(t *testing.T) {
	in := fullInput()
	policy := profitability.DefaultPolicy()

	r1 := profitability.Calculate(in, policy)
	if len(r1.CustomerView.AllPeriod) > 0 {
		r1.CustomerView.AllPeriod[0].EntityID = "MUTATED"
	}
	if len(r1.Flags) > 0 {
		r1.Flags[0].Message = "MUTATED"
	}

	r2 := profitability.Calculate(in, policy)
	for _, e := range r2.CustomerView.AllPeriod {
		if e.EntityID == "MUTATED" {
			t.Fatal("mutating a previous Result's slice affected a new Calculate call's output")
		}
	}
	for _, f := range r2.Flags {
		if f.Message == "MUTATED" {
			t.Fatal("mutating a previous Result's Flags affected a new Calculate call's output")
		}
	}
}
