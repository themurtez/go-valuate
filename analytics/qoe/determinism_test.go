package qoe

import (
	"encoding/json"
	"testing"

	"github.com/themurtez/go-valuate/financial/adjustments"
)

// TestCalculate_Deterministic proves repeated execution against identical
// input returns byte-for-byte identical output, mirroring
// financial/adjustments/determinism_test.go's TestApply_Deterministic.
func TestCalculate_Deterministic(t *testing.T) {
	ds := loadFixtureDataset(t, "normalized_saas_multi_year.json")
	meta := threeYearMeta()
	adjs := loadAdjustmentFixtures(t)["saas"]
	maintainableEBITDA := earningsResultAt(450000)
	maintainableSDE := earningsResultAt(450000)

	in := Input{
		Dataset:            ds,
		PeriodMeta:         meta,
		Adjustments:        adjs,
		MaintainableEBITDA: maintainableEBITDA,
		MaintainableSDE:    maintainableSDE,
	}
	opts := Options{ComputeScore: true}

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
// does not depend on Go's randomized map iteration order, by running many
// times with a large Adjustments/Thresholds set that would surface any
// hidden map-order dependency in buildAdjustmentBreakdown/buildRecurrence
// (both of which internally use maps keyed by adjustments.Type before
// sorting into a deterministic slice).
func TestCalculate_DeterministicAcrossMapOrdering(t *testing.T) {
	ds := loadFixtureDataset(t, "normalized_manufacturer_multi_year.json")
	meta := threeYearMeta()

	adjs := loadAdjustmentFixtures(t)["manufacturer"]
	adjs = append(adjs,
		adjustmentAt("extra-1", "2023", adjustments.TypeOneTimeExpense, 1000),
		adjustmentAt("extra-2", "2024", adjustments.TypeOneTimeExpense, 2000),
		adjustmentAt("extra-3", "2025", adjustments.TypeUnusualGain, 3000, adjustments.EffectDecrease),
		adjustmentAt("extra-4", "2023", adjustments.TypeNonRecurringProfessionalFees, 500),
	)

	in := Input{Dataset: ds, PeriodMeta: meta, Adjustments: adjs}
	first, err := json.Marshal(Calculate(in, Options{}))
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	for i := 0; i < 20; i++ {
		got, err := json.Marshal(Calculate(in, Options{}))
		if err != nil {
			t.Fatalf("run %d: json.Marshal failed: %v", i, err)
		}
		if string(got) != string(first) {
			t.Fatalf("run %d: output differs; suspect map-order nondeterminism", i)
		}
	}
}
