package labor_test

import (
	"encoding/json"
	"testing"

	"github.com/themurtez/go-valuate/accounting/labor"
)

// TestImmutability_CalculateDoesNotMutateInput proves Calculate never
// mutates any field of Input (or nested slices/maps within it).
func TestImmutability_CalculateDoesNotMutateInput(t *testing.T) {
	in := fullInput()
	policy := labor.DefaultPolicy()

	before, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("Marshal before: %v", err)
	}

	_ = labor.Calculate(in, policy)

	after, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("Marshal after: %v", err)
	}
	if string(before) != string(after) {
		t.Fatalf("Calculate mutated Input:\nbefore=%s\nafter=%s", before, after)
	}
}

func TestImmutability_PolicyNotMutated(t *testing.T) {
	in := fullInput()
	policy := labor.DefaultPolicy()
	before, _ := json.Marshal(policy)

	_ = labor.Calculate(in, policy)

	after, _ := json.Marshal(policy)
	if string(before) != string(after) {
		t.Fatalf("Calculate mutated Policy:\nbefore=%s\nafter=%s", before, after)
	}
}

// TestImmutability_ResultIndependentOfSubsequentInputMutation proves that
// mutating the caller's Input slices after calling Calculate does not
// retroactively change the already-returned Result (i.e. this package
// never aliases caller-owned backing arrays into its Result).
func TestImmutability_ResultIndependentOfSubsequentInputMutation(t *testing.T) {
	in := fullInput()
	policy := labor.DefaultPolicy()

	result := labor.Calculate(in, policy)
	before, _ := json.Marshal(result)

	// Mutate caller's slices after the call.
	if len(in.PayrollRecords) > 0 {
		in.PayrollRecords[0].RegularPay = 999999
	}
	if len(in.Workers) > 0 {
		in.Workers[0].Department = "MUTATED"
	}

	after, _ := json.Marshal(result)
	if string(before) != string(after) {
		t.Error("Result changed after mutating caller's Input slices post-Calculate; Result must not alias caller-owned backing arrays")
	}
}
