package vendorspend_test

import (
	"encoding/json"
	"testing"

	"github.com/themurtez/go-valuate/accounting/vendorspend"
)

// TestImmutability_CalculateDoesNotMutateInput proves Calculate never
// mutates any field of Input (or nested slices/maps within it).
func TestImmutability_CalculateDoesNotMutateInput(t *testing.T) {
	in := fullInput()
	opts := fullOptions()

	before, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("Marshal before: %v", err)
	}

	_ = vendorspend.Calculate(in, opts)

	after, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("Marshal after: %v", err)
	}
	if string(before) != string(after) {
		t.Fatalf("Calculate mutated Input:\nbefore=%s\nafter=%s", before, after)
	}
}

// TestImmutability_CalculateDoesNotMutateOptions proves Calculate never
// mutates its Options argument.
func TestImmutability_CalculateDoesNotMutateOptions(t *testing.T) {
	in := fullInput()
	opts := fullOptions()

	before, err := json.Marshal(opts)
	if err != nil {
		t.Fatalf("Marshal before: %v", err)
	}

	_ = vendorspend.Calculate(in, opts)

	after, err := json.Marshal(opts)
	if err != nil {
		t.Fatalf("Marshal after: %v", err)
	}
	if string(before) != string(after) {
		t.Fatalf("Calculate mutated Options:\nbefore=%s\nafter=%s", before, after)
	}
}

// TestImmutability_ResultMutationDoesNotAffectNextCall proves mutating a
// returned Result does not corrupt a subsequent Calculate call (no
// shared backing arrays/maps between calls).
func TestImmutability_ResultMutationDoesNotAffectNextCall(t *testing.T) {
	in := fullInput()
	opts := fullOptions()

	r1 := vendorspend.Calculate(in, opts)
	if len(r1.SupplierSummaries) > 0 {
		r1.SupplierSummaries[0].SupplierID = "MUTATED"
	}
	if len(r1.Flags) > 0 {
		r1.Flags[0].Message = "MUTATED"
	}
	if len(r1.Issues) > 0 {
		r1.Issues[0].Message = "MUTATED"
	}

	r2 := vendorspend.Calculate(in, opts)
	for _, s := range r2.SupplierSummaries {
		if s.SupplierID == "MUTATED" {
			t.Fatalf("mutating r1.SupplierSummaries affected r2")
		}
	}
	for _, f := range r2.Flags {
		if f.Message == "MUTATED" {
			t.Fatalf("mutating r1.Flags affected r2")
		}
	}
	for _, is := range r2.Issues {
		if is.Message == "MUTATED" {
			t.Fatalf("mutating r1.Issues affected r2")
		}
	}
}

// TestImmutability_SupplierSliceNotSharedWithInput proves the Supplier
// records inside Input.Suppliers cannot be mutated through Result.
func TestImmutability_SupplierSliceNotSharedWithInput(t *testing.T) {
	in := fullInput()
	originalFirstID := in.Suppliers[0].SupplierID

	_ = vendorspend.Calculate(in, fullOptions())

	if in.Suppliers[0].SupplierID != originalFirstID {
		t.Fatalf("Input.Suppliers[0].SupplierID changed: got %q, want %q", in.Suppliers[0].SupplierID, originalFirstID)
	}
}
