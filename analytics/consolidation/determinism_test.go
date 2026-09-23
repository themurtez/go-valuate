package consolidation

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
		Entities: []EntityDataset{
			{EntityID: "parent", EntityLabel: "Parent Co", Dataset: parentUSD()},
			{EntityID: "sub", EntityLabel: "Subsidiary Co", Dataset: subsidiaryEUR()},
		},
		Periods:       twoYearPeriods(),
		CurrencyRates: eurToUSDRates(),
		Eliminations:  managementFeeEliminations(),
		Policy:        Policy{TargetCurrency: "USD"},
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
// does not depend on Go's randomized map iteration order, using a dataset
// with many entities/codes/periods/currencies/eliminations that surfaces
// map-order dependencies in entityByID, elimByEntity, rateIndex, and the
// totals/sources maps inside buildConsolidatedDataset — mirroring
// concentration.TestCalculate_DeterministicAcrossMapOrdering.
func TestCalculate_DeterministicAcrossMapOrdering(t *testing.T) {
	codes := []financial.Code{
		financial.CodeRevProduct, financial.CodeRevService, financial.CodeRevRecurring, financial.CodeRevOther,
		financial.CodeCogsMaterial, financial.CodeOpexPayroll, financial.CodeOpexMarketing, financial.CodeOpexOther,
	}
	periods := []financial.Period{"2023", "2024", "2025"}
	currencies := []string{"USD", "EUR", "GBP"}

	var entities []EntityDataset
	var rates []CurrencyRate
	var eliminations []Elimination

	for i := 0; i < 15; i++ {
		id := "entity-" + string(rune('a'+i%26)) + string(rune('0'+i/26))
		cur := currencies[i%len(currencies)]

		var items []financial.NormalizedItem
		for j, code := range codes {
			for k, p := range periods {
				items = append(items, financial.NormalizedItem{
					Code:   code,
					Period: p,
					Amount: float64(1000 + i*137 + j*29 + k*11),
				})
			}
		}
		entities = append(entities, EntityDataset{
			EntityID:    id,
			EntityLabel: "Entity " + id,
			Dataset:     financial.FinancialDataset{Currency: cur, Items: items},
		})

		if cur != "USD" {
			for _, p := range periods {
				rates = append(rates, CurrencyRate{FromCurrency: cur, ToCurrency: "USD", Period: p, Rate: 1.0 + float64(i%5)*0.02})
			}
		}
		eliminations = append(eliminations, Elimination{
			EntityID: id, Code: codes[i%len(codes)], Period: periods[i%len(periods)], Amount: float64(50 + i*3),
		})
	}

	in := Input{
		Entities:      entities,
		Periods:       periods,
		CurrencyRates: rates,
		Eliminations:  eliminations,
		Policy:        Policy{TargetCurrency: "USD"},
	}

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
