package reconciliation

import (
	"encoding/json"
	"math"
	"testing"
)

// TestSafety_NonFiniteItemAmountsExcludedNotCrashed confirms a NaN/Inf
// BookItem/ExternalItem amount is reported as an Issue and excluded,
// never propagated into arithmetic (task section 72).
func TestSafety_NonFiniteItemAmountsExcludedNotCrashed(t *testing.T) {
	in := Input{
		AccountID: "A",
		AsOfDate:  "2025-01-01",
		BookItems: []BookItem{
			{ItemID: "B1", Date: "2025-01-01", Amount: math.NaN(), Direction: DirectionOutflow},
			{ItemID: "B2", Date: "2025-01-01", Amount: math.Inf(1), Direction: DirectionOutflow},
			{ItemID: "B3", Date: "2025-01-01", Amount: 100, Direction: DirectionOutflow},
		},
		ExternalItems: []ExternalItem{
			{ItemID: "E1", Date: "2025-01-01", Amount: math.Inf(-1), Direction: DirectionOutflow},
			{ItemID: "E1b", Date: "2025-01-01", Amount: 100, Direction: DirectionOutflow},
		},
	}
	r := Calculate(in)
	assertNoNaNOrInf(t, r)
	// B1/B2 should be excluded (2 error issues); B3 should still match
	// E1b.
	errorCount := 0
	for _, iss := range r.Issues {
		if iss.Code == IssueNonFiniteAmount {
			errorCount++
		}
	}
	if errorCount != 3 {
		t.Fatalf("expected 3 non-finite-amount issues (B1, B2, E1), got %d: %+v", errorCount, r.Issues)
	}
	if len(r.MatchedGroups) != 1 {
		t.Fatalf("expected B3/E1b to still match despite other invalid items, got %+v", r.MatchedGroups)
	}
}

// TestSafety_NonFiniteBalancesExcluded confirms NaN/Inf opening/ending
// balances never reach the equation.
func TestSafety_NonFiniteBalancesExcluded(t *testing.T) {
	bad := math.NaN()
	in := Input{
		AccountID:       "A",
		AsOfDate:        "2025-01-01",
		BookBalance:     BalanceInput{EndingBalance: &bad},
		ExternalBalance: BalanceInput{EndingBalance: &bad},
	}
	r := Calculate(in)
	assertNoNaNOrInf(t, r)
	if r.Equation.Available {
		t.Fatalf("expected equation unavailable with non-finite balances excluded, got %+v", r.Equation)
	}
}

// TestSafety_NonFinitePolicyThresholdsFlagged confirms a NaN/Inf policy
// threshold is reported, not silently used in arithmetic.
func TestSafety_NonFinitePolicyThresholdsFlagged(t *testing.T) {
	in := Input{
		AccountID: "A",
		AsOfDate:  "2025-01-01",
		Policy:    MatchingPolicy{AmountTolerance: math.Inf(1)},
	}
	r := Calculate(in)
	assertNoNaNOrInf(t, r)
	found := false
	for _, iss := range r.Issues {
		if iss.Code == IssueInvalidPolicy {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected IssueInvalidPolicy for non-finite tolerance, got %+v", r.Issues)
	}
}

// TestSafety_ExtremeValuesDoNotOverflowSilently confirms very large
// finite amounts do not panic and stay finite in Difference computation.
func TestSafety_ExtremeValuesDoNotOverflowSilently(t *testing.T) {
	big := math.MaxFloat64 / 2
	in := Input{
		AccountID: "A",
		AsOfDate:  "2025-01-01",
		BookItems: []BookItem{
			{ItemID: "B1", Date: "2025-01-01", Amount: big, Direction: DirectionOutflow},
		},
		ExternalItems: []ExternalItem{
			{ItemID: "E1", Date: "2025-01-01", Amount: big, Direction: DirectionOutflow},
		},
	}
	r := Calculate(in) // must not panic
	assertNoNaNOrInf(t, r)
}

func assertNoNaNOrInf(t *testing.T, r Result) {
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal error (likely NaN/Inf leaked into JSON): %v", err)
	}
	// encoding/json itself refuses to marshal NaN/Inf float64 values
	// (returns an UnsupportedValueError), so a successful Marshal above
	// already proves no NaN/Inf reached any float64 field in r. This
	// second pass is a defensive re-confirmation via round-trip.
	var round Result
	if err := json.Unmarshal(b, &round); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
}
