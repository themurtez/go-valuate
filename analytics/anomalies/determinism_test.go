package anomalies

import (
	"encoding/json"
	"testing"

	"github.com/themurtez/go-valuate/financial"
)

// TestCalculate_Deterministic proves repeated execution against identical
// input returns byte-for-byte identical output, mirroring
// concentration/determinism_test.go's TestCalculate_Deterministic.
func TestCalculate_Deterministic(t *testing.T) {
	in := Input{
		Dataset:       missingPeriodDataset(),
		PeriodMeta:    fourYearMeta(),
		AccountGroups: []AccountGroup{{Name: "G&A", Codes: []financial.Code{financial.CodeOpexSoftware, financial.CodeOpexTravel}}},
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
// with many accounts/periods/repeated-and-duplicate amounts that surfaces
// map-order dependencies in buildIndex, detectRepeatedUnusualValues'/
// detectDuplicateLikeAmounts' byAmount grouping, and groupIndex — mirroring
// concentration.TestCalculate_DeterministicAcrossMapOrdering.
func TestCalculate_DeterministicAcrossMapOrdering(t *testing.T) {
	meta := map[financial.Period]PeriodInfo{
		"2022": {Type: PeriodTypeFiscalYear, FiscalYear: 2022},
		"2023": {Type: PeriodTypeFiscalYear, FiscalYear: 2023},
		"2024": {Type: PeriodTypeFiscalYear, FiscalYear: 2024},
		"2025": {Type: PeriodTypeFiscalYear, FiscalYear: 2025},
	}
	years := []string{"2022", "2023", "2024", "2025"}
	codes := financial.CodesByCategory(financial.CategoryOpex)

	var items []financial.NormalizedItem
	for i, codeMeta := range codes {
		for j, y := range years {
			amount := float64(1000 + i*137 + j*211)
			if i%5 == 0 {
				// Force some duplicate-like/repeated amounts to stress the
				// byAmount map-keyed grouping in duplicates.go.
				amount = 5000
			}
			items = append(items, item(codeMeta.Code, y, amount))
		}
	}
	items = append(items,
		item(financial.CodeRevProduct, "2022", 2_000_000),
		item(financial.CodeRevProduct, "2023", 2_100_000),
		item(financial.CodeRevProduct, "2024", 2_050_000),
		item(financial.CodeRevProduct, "2025", 2_300_000),
	)

	in := Input{Dataset: datasetOf(items...), PeriodMeta: meta}
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
