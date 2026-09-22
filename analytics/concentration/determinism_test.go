package concentration

import (
	"encoding/json"
	"testing"

	"github.com/themurtez/go-valuate/financial"
)

// TestCalculate_Deterministic proves repeated execution against identical
// input returns byte-for-byte identical output, mirroring
// revenuequality/determinism_test.go's TestCalculate_Deterministic.
func TestCalculate_Deterministic(t *testing.T) {
	rate := 0.35
	in := Input{
		Basis:        BasisCustomerRevenue,
		Observations: highlyConcentratedObservations(),
		PeriodMeta:   threeYearMeta(),
		Policy: Policy{
			ScenarioTopN:            []int{1, 3},
			DefaultImpactMarginRate: &rate,
		},
	}
	opts := Options{}

	first, err := json.Marshal(Calculate(in, opts))
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	for i := 0; i < 10; i++ {
		got, err := json.Marshal(Calculate(in, opts))
		if err != nil {
			t.Fatalf("run %d: json.Marshal failed: %v", i, err)
		}
		if string(got) != string(first) {
			t.Fatalf("run %d: Calculate output differs from the first run", i)
		}
	}
}

// TestCalculate_DeterministicAcrossMapOrdering proves Calculate's output
// does not depend on Go's randomized map iteration order, using a dataset
// with many entities/categories/periods that surfaces map-order
// dependencies in entityTotalsByKey, groupByPeriod, and
// calculateCategoryShares (all of which internally use maps before sorting
// into deterministic slices) — mirroring
// revenuequality.TestCalculate_DeterministicAcrossMapOrdering.
func TestCalculate_DeterministicAcrossMapOrdering(t *testing.T) {
	meta := map[financial.Period]PeriodInfo{
		"2024": {Type: PeriodTypeFiscalYear, FiscalYear: 2024},
		"2025": {Type: PeriodTypeFiscalYear, FiscalYear: 2025},
	}
	categories := []string{"east", "west", "north", "south", "central"}

	var observations []Observation
	for i := 0; i < 30; i++ {
		key := "entity-" + string(rune('a'+i%26)) + string(rune('0'+i/26))
		cat := categories[i%len(categories)]
		observations = append(observations,
			Observation{EntityKey: key, Period: "2024", Amount: float64(1000 + i*137), Category: cat},
			Observation{EntityKey: key, Period: "2025", Amount: float64(1100 + i*151), Category: cat},
		)
	}

	in := Input{Observations: observations, PeriodMeta: meta}
	opts := Options{}

	first, err := json.Marshal(Calculate(in, opts))
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	for i := 0; i < 20; i++ {
		got, err := json.Marshal(Calculate(in, opts))
		if err != nil {
			t.Fatalf("run %d: json.Marshal failed: %v", i, err)
		}
		if string(got) != string(first) {
			t.Fatalf("run %d: output differs; suspect map-order nondeterminism", i)
		}
	}
}
