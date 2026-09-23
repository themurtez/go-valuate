package dealstructure

import (
	"encoding/json"
	"testing"
)

// TestBuild_Deterministic proves repeated execution against identical
// input returns byte-for-byte identical output, mirroring
// acquisition/determinism_test.go's TestCalculate_Deterministic.
func TestBuild_Deterministic(t *testing.T) {
	in := buildFullInput()

	first, err := json.Marshal(Build(in))
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	for i := 0; i < 10; i++ {
		got, err := json.Marshal(Build(in))
		if err != nil {
			t.Fatalf("run %d: json.Marshal failed: %v", i, err)
		}
		if string(got) != string(first) {
			t.Fatalf("run %d: Build output differs from the first run", i)
		}
	}
}

// TestBuild_NoMutationOfInput proves Build never mutates caller-owned
// slices (DebtTranches, SellerNote.Terms, Earnout.Payments).
func TestBuild_NoMutationOfInput(t *testing.T) {
	in := buildFullInput()

	before, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("json.Marshal(in) failed: %v", err)
	}

	_ = Build(in)
	_ = Build(in)

	after, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("json.Marshal(in) failed: %v", err)
	}
	if string(before) != string(after) {
		t.Fatalf("Build mutated caller-owned Input:\nbefore: %s\nafter:  %s", before, after)
	}
}

// TestBuild_NoMutationOfInput_UnsortedEarnout specifically proves the
// earnout-sorting logic in buildEarnoutSchedule does not sort
// Input.Earnout.Payments in place (Result.EarnoutSchedule is a new,
// sorted slice).
func TestBuild_NoMutationOfInput_UnsortedEarnout(t *testing.T) {
	payments := []EarnoutPayment{
		{PeriodNumber: 3, Amount: 300_000},
		{PeriodNumber: 1, Amount: 100_000},
		{PeriodNumber: 2, Amount: 200_000},
	}
	in := Input{
		PurchasePrice: AvailableValue(1_000_000),
		BuyerEquity:   AvailableValue(1_000_000),
		Earnout:       Earnout{Included: true, Payments: payments},
	}

	_ = Build(in)

	if payments[0].PeriodNumber != 3 || payments[1].PeriodNumber != 1 || payments[2].PeriodNumber != 2 {
		t.Fatalf("Build mutated the original Payments slice order: %+v", payments)
	}
}
