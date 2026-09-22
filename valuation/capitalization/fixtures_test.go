package capitalization

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/metrics"
)

type archetypeAssumptions struct {
	CapitalizationRate float64 `json:"capitalization_rate"`
}

func loadAssumptions(t *testing.T, archetype string) archetypeAssumptions {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "fixtures", "valuation_by_business_type.json"))
	if err != nil {
		t.Fatalf("reading valuation fixture: %v", err)
	}
	var byArchetype map[string]json.RawMessage
	if err := json.Unmarshal(b, &byArchetype); err != nil {
		t.Fatalf("unmarshaling valuation fixture: %v", err)
	}
	raw, ok := byArchetype[archetype]
	if !ok {
		t.Fatalf("no %q entry in valuation fixture", archetype)
	}
	var a archetypeAssumptions
	if err := json.Unmarshal(raw, &a); err != nil {
		t.Fatalf("unmarshaling %q assumptions: %v", archetype, err)
	}
	return a
}

// maintainableEarnings2025 uses SDE as the earnings base for this fixture
// (a small-business-oriented capitalization, consistent with each
// archetype's sde_multiple assumption also being SDE-based) — see the
// package doc comment on why Calculate always reports the result as an
// equity value regardless of which earnings base the caller chooses.
func maintainableEarnings2025(t *testing.T, datasetFixture string) float64 {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "fixtures", datasetFixture))
	if err != nil {
		t.Fatalf("reading dataset fixture %s: %v", datasetFixture, err)
	}
	var ds financial.FinancialDataset
	if err := json.Unmarshal(b, &ds); err != nil {
		t.Fatalf("unmarshaling dataset fixture %s: %v", datasetFixture, err)
	}
	result := metrics.Calculate(ds, metrics.Options{})
	snap, ok := result.SnapshotFor("2025")
	if !ok {
		t.Fatalf("expected a 2025 snapshot in %s", datasetFixture)
	}
	if !snap.SDE.Available {
		t.Fatalf("SDE unavailable for %s 2025", datasetFixture)
	}
	return snap.SDE.Value
}

func TestFixtures_HVAC(t *testing.T) {
	assumptions := loadAssumptions(t, "hvac")
	earnings := maintainableEarnings2025(t, "normalized_hvac_multi_year.json")

	res := Calculate(Input{MaintainableEarnings: earnings, CapitalizationRate: assumptions.CapitalizationRate})

	if !res.Available {
		t.Fatalf("unexpected errors: %+v", res.Errors)
	}
	wantValue := earnings / assumptions.CapitalizationRate
	if res.EquityValue != wantValue {
		t.Errorf("EquityValue = %v, want %v", res.EquityValue, wantValue)
	}
}

func TestFixtures_Agency(t *testing.T) {
	assumptions := loadAssumptions(t, "agency")
	earnings := maintainableEarnings2025(t, "normalized_agency_multi_year.json")

	res := Calculate(Input{MaintainableEarnings: earnings, CapitalizationRate: assumptions.CapitalizationRate})

	if !res.Available {
		t.Fatalf("unexpected errors: %+v", res.Errors)
	}
	wantValue := earnings / assumptions.CapitalizationRate
	if res.EquityValue != wantValue {
		t.Errorf("EquityValue = %v, want %v", res.EquityValue, wantValue)
	}
}

func TestFixtures_Manufacturer(t *testing.T) {
	assumptions := loadAssumptions(t, "manufacturer")
	earnings := maintainableEarnings2025(t, "normalized_manufacturer_multi_year.json")

	res := Calculate(Input{MaintainableEarnings: earnings, CapitalizationRate: assumptions.CapitalizationRate})

	if !res.Available {
		t.Fatalf("unexpected errors: %+v", res.Errors)
	}
	wantValue := earnings / assumptions.CapitalizationRate
	if res.EquityValue != wantValue {
		t.Errorf("EquityValue = %v, want %v", res.EquityValue, wantValue)
	}
	// A lower capitalization rate (manufacturer: 0.18) relative to HVAC's
	// (0.30) reflects lower perceived risk for a larger, more established
	// business — captured here as a smoke check that the fixture's
	// assumptions are directionally sane, not just individually valid.
	hvacAssumptions := loadAssumptions(t, "hvac")
	if assumptions.CapitalizationRate >= hvacAssumptions.CapitalizationRate {
		t.Errorf("expected manufacturer's capitalization rate (%v) to be lower than HVAC's (%v)", assumptions.CapitalizationRate, hvacAssumptions.CapitalizationRate)
	}
}

func TestFixtures_SaaS(t *testing.T) {
	assumptions := loadAssumptions(t, "saas")
	earnings := maintainableEarnings2025(t, "normalized_saas_multi_year.json")

	res := Calculate(Input{MaintainableEarnings: earnings, CapitalizationRate: assumptions.CapitalizationRate})

	if !res.Available {
		t.Fatalf("unexpected errors: %+v", res.Errors)
	}
	wantValue := earnings / assumptions.CapitalizationRate
	if res.EquityValue != wantValue {
		t.Errorf("EquityValue = %v, want %v", res.EquityValue, wantValue)
	}
}

func TestFixtures_EveryArchetypeCalculatesWithNoErrors(t *testing.T) {
	datasets := map[string]string{
		"hvac":         "normalized_hvac_multi_year.json",
		"agency":       "normalized_agency_multi_year.json",
		"manufacturer": "normalized_manufacturer_multi_year.json",
		"saas":         "normalized_saas_multi_year.json",
	}
	for archetype, datasetFixture := range datasets {
		t.Run(archetype, func(t *testing.T) {
			assumptions := loadAssumptions(t, archetype)
			earnings := maintainableEarnings2025(t, datasetFixture)
			res := Calculate(Input{MaintainableEarnings: earnings, CapitalizationRate: assumptions.CapitalizationRate})
			if !res.Available {
				t.Fatalf("unexpected errors: %+v", res.Errors)
			}
			if res.EquityValue <= 0 {
				t.Errorf("%s: EquityValue = %v, expected positive", archetype, res.EquityValue)
			}
		})
	}
}
