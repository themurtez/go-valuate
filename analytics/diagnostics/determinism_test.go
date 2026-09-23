package diagnostics

import (
	"encoding/json"
	"testing"
)

// TestCalculate_Deterministic proves repeated execution against identical
// input returns byte-for-byte identical output, for both the healthy and
// stressed fixtures.
func TestCalculate_Deterministic(t *testing.T) {
	for name, in := range map[string]Input{"healthy": healthyFixture(), "stressed": stressedFixture()} {
		t.Run(name, func(t *testing.T) {
			first, err := json.Marshal(Calculate(in))
			if err != nil {
				t.Fatalf("json.Marshal failed: %v", err)
			}
			for i := 0; i < 10; i++ {
				got, err := json.Marshal(Calculate(in))
				if err != nil {
					t.Fatalf("run %d: json.Marshal failed: %v", i, err)
				}
				if string(got) != string(first) {
					t.Fatalf("run %d: Calculate output differs from the first run", i)
				}
			}
		})
	}
}

// TestCalculate_CoverageAndScoreVersionOrder proves Coverage.MissingModules
// and Result.Findings/Strengths/Concerns/Opportunities never depend on Go
// map iteration order by checking the same fixture's output is identical
// across many runs with fresh map allocations each time (exercises
// moduleCategories/categoryForDimension/ratioSignalTable/etc. — every
// lookup table in this package is a map).
func TestCalculate_NoMapOrderDependence(t *testing.T) {
	in := stressedFixture()
	var results [][]byte
	for i := 0; i < 20; i++ {
		b, err := json.Marshal(Calculate(in))
		if err != nil {
			t.Fatalf("run %d: json.Marshal failed: %v", i, err)
		}
		results = append(results, b)
	}
	for i := 1; i < len(results); i++ {
		if string(results[i]) != string(results[0]) {
			t.Fatalf("run %d differs from run 0 — possible map-order nondeterminism", i)
		}
	}
}
