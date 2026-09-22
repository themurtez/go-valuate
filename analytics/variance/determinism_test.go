package variance

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
		Lines:      budgetVsActualLines(),
		PeriodMeta: threeYearMeta(),
		Policy: Policy{
			MaterialAmountThreshold:   10_000,
			MaterialPercentOfBaseline: 0.05,
			TopN:                      3,
		},
	}

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
}

// TestCalculate_DeterministicAcrossMapOrdering proves Calculate's output
// does not depend on Go's randomized map iteration order, using many
// accounts/categories/periods that surface map-order dependencies in
// buildCategorySummaries/buildPeriodTrends/buildDirectionIndex (all of
// which internally use maps before sorting into deterministic slices).
func TestCalculate_DeterministicAcrossMapOrdering(t *testing.T) {
	codes := []financial.Code{
		financial.CodeRevProduct, financial.CodeRevService, financial.CodeRevRecurring, financial.CodeRevOther,
		financial.CodeCogsMaterial, financial.CodeCogsDirectLabor, financial.CodeCogsFreight,
		financial.CodeOpexPayroll, financial.CodeOpexMarketing, financial.CodeOpexRent, financial.CodeOpexSoftware,
	}
	categories := []string{"east", "west", "north", "south", "central"}
	meta := map[financial.Period]PeriodInfo{
		"2024": {Type: PeriodTypeFiscalYear, FiscalYear: 2024},
		"2025": {Type: PeriodTypeFiscalYear, FiscalYear: 2025},
	}

	var lines []LineObservation
	for i, code := range codes {
		cat := categories[i%len(categories)]
		lines = append(lines,
			lineCat(code, "2024", float64(10_000+i*137), float64(9_500+i*130), BaselineTypeBudget, cat),
			lineCat(code, "2025", float64(11_000+i*151), float64(10_200+i*140), BaselineTypeBudget, cat),
		)
	}

	in := Input{Lines: lines, PeriodMeta: meta}

	first, err := json.Marshal(Calculate(in))
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	for i := 0; i < 20; i++ {
		got, err := json.Marshal(Calculate(in))
		if err != nil {
			t.Fatalf("run %d: json.Marshal failed: %v", i, err)
		}
		if string(got) != string(first) {
			t.Fatalf("run %d: output differs; suspect map-order nondeterminism", i)
		}
	}
}
