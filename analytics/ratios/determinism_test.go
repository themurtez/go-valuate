package ratios

import (
	"encoding/json"
	"testing"
)

// TestCalculate_Deterministic proves repeated execution against identical
// input returns byte-for-byte identical output, mirroring
// workingcapital/determinism_test.go's TestCalculate_Deterministic.
func TestCalculate_Deterministic(t *testing.T) {
	ds := loadFixtureDataset(t, "normalized_hvac_multi_year.json")
	meta := threeYearMeta()

	in := Input{Dataset: ds, PeriodMeta: meta}
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

// TestCalculate_DeterministicAcrossFixtures runs every realistic multi-year
// fixture in fixtures/ to prove determinism holds across a variety of
// dataset shapes (SaaS, manufacturer, agency), not just HVAC.
func TestCalculate_DeterministicAcrossFixtures(t *testing.T) {
	names := []string{
		"normalized_hvac_multi_year.json",
		"normalized_saas_multi_year.json",
		"normalized_manufacturer_multi_year.json",
		"normalized_agency_multi_year.json",
	}
	for _, name := range names {
		name := name
		t.Run(name, func(t *testing.T) {
			ds := loadFixtureDataset(t, name)
			meta := threeYearMeta()
			in := Input{Dataset: ds, PeriodMeta: meta}

			first, err := json.Marshal(Calculate(in, Options{}))
			if err != nil {
				t.Fatalf("json.Marshal failed: %v", err)
			}
			for i := 0; i < 5; i++ {
				got, err := json.Marshal(Calculate(in, Options{}))
				if err != nil {
					t.Fatalf("run %d: json.Marshal failed: %v", i, err)
				}
				if string(got) != string(first) {
					t.Fatalf("run %d: output differs from the first run", i)
				}
			}
		})
	}
}
