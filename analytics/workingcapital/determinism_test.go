package workingcapital

import (
	"encoding/json"
	"testing"
)

// TestCalculate_Deterministic proves repeated execution against identical
// input returns byte-for-byte identical output, mirroring
// analytics/qoe/determinism_test.go's TestCalculate_Deterministic.
func TestCalculate_Deterministic(t *testing.T) {
	ds := loadFixtureDataset(t, "normalized_saas_multi_year.json")
	meta := threeYearMeta()

	in := Input{Dataset: ds, PeriodMeta: meta, AsOf: "2025"}
	opts := Options{PegMethod: PegMethodTrailingAverage, TrailingPeriods: 2}

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
// does not depend on Go's randomized map iteration order, using a large
// InclusionPolicy plus a seasonal quarterly dataset that surfaces map-order
// dependencies in buildIndex, excludedCodes, and calculateSeasonalProfile
// (all of which internally use maps before sorting into deterministic
// slices).
func TestCalculate_DeterministicAcrossMapOrdering(t *testing.T) {
	ds, meta := seasonalRetailDataset()
	in := Input{Dataset: ds, PeriodMeta: meta, AsOf: "2025-Q4"}
	opts := Options{PegMethod: PegMethodMedian}

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
