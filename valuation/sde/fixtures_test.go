package sde

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/metrics"
)

// archetypeAssumptions mirrors the subset of
// fixtures/valuation_by_business_type.json this package's tests need: just
// the SDE multiple. Other methods' fixtures_test.go define their own
// narrower or wider view of the same file, mirroring how
// financial/adjustments' fixtures_test.go only decodes the Adjustment
// fields it needs from a shared fixture file.
type archetypeAssumptions struct {
	SDEMultiple float64 `json:"sde_multiple"`
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

// maintainableSDE2025 derives the archetype's maintainable SDE the same way
// a real caller would: financial/metrics.Calculate over the archetype's
// three-fiscal-year normalized dataset, taking the 2025 snapshot's baseline
// SDE directly (no financial/adjustments/financial/earnings layering — this
// keeps the fixture focused on exercising valuation/sde in isolation, the
// same simplification financial/adjustments' fixture tests make when they
// go straight from a metrics.Snapshot to Apply).
func maintainableSDE2025(t *testing.T, datasetFixture string) float64 {
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
	maintainableSDE := maintainableSDE2025(t, "normalized_hvac_multi_year.json")

	res := Calculate(Input{MaintainableSDE: maintainableSDE, Multiple: assumptions.SDEMultiple})

	if !res.Available {
		t.Fatalf("unexpected errors: %+v", res.Errors)
	}
	wantEquity := maintainableSDE * assumptions.SDEMultiple
	if res.EquityValue != wantEquity {
		t.Errorf("EquityValue = %v, want %v (SDE %v x multiple %v)", res.EquityValue, wantEquity, maintainableSDE, assumptions.SDEMultiple)
	}
	if len(res.Warnings) != 0 {
		t.Errorf("expected no warnings for a healthy positive-SDE business, got %+v", res.Warnings)
	}
}

func TestFixtures_Agency(t *testing.T) {
	assumptions := loadAssumptions(t, "agency")
	maintainableSDE := maintainableSDE2025(t, "normalized_agency_multi_year.json")

	res := Calculate(Input{MaintainableSDE: maintainableSDE, Multiple: assumptions.SDEMultiple})

	if !res.Available {
		t.Fatalf("unexpected errors: %+v", res.Errors)
	}
	wantEquity := maintainableSDE * assumptions.SDEMultiple
	if res.EquityValue != wantEquity {
		t.Errorf("EquityValue = %v, want %v", res.EquityValue, wantEquity)
	}
}

func TestFixtures_Manufacturer(t *testing.T) {
	assumptions := loadAssumptions(t, "manufacturer")
	maintainableSDE := maintainableSDE2025(t, "normalized_manufacturer_multi_year.json")

	// Manufacturer has no owner-operator, so baseline SDE == baseline
	// EBITDA (owner compensation contributes 0) — see
	// financial/reconciliation's TestFixtures_ManufacturerHasNoOwnerCompensation.
	res := Calculate(Input{MaintainableSDE: maintainableSDE, Multiple: assumptions.SDEMultiple})
	if !res.Available {
		t.Fatalf("unexpected errors: %+v", res.Errors)
	}
	wantEquity := maintainableSDE * assumptions.SDEMultiple
	if res.EquityValue != wantEquity {
		t.Errorf("EquityValue = %v, want %v", res.EquityValue, wantEquity)
	}
}

func TestFixtures_SaaS(t *testing.T) {
	assumptions := loadAssumptions(t, "saas")
	maintainableSDE := maintainableSDE2025(t, "normalized_saas_multi_year.json")

	res := Calculate(Input{MaintainableSDE: maintainableSDE, Multiple: assumptions.SDEMultiple})
	if !res.Available {
		t.Fatalf("unexpected errors: %+v", res.Errors)
	}
	wantEquity := maintainableSDE * assumptions.SDEMultiple
	if res.EquityValue != wantEquity {
		t.Errorf("EquityValue = %v, want %v", res.EquityValue, wantEquity)
	}
}

func TestFixtures_EveryArchetypeProducesAPositiveEquityValue(t *testing.T) {
	datasets := map[string]string{
		"hvac":         "normalized_hvac_multi_year.json",
		"agency":       "normalized_agency_multi_year.json",
		"manufacturer": "normalized_manufacturer_multi_year.json",
		"saas":         "normalized_saas_multi_year.json",
	}
	for archetype, datasetFixture := range datasets {
		t.Run(archetype, func(t *testing.T) {
			assumptions := loadAssumptions(t, archetype)
			maintainableSDE := maintainableSDE2025(t, datasetFixture)
			res := Calculate(Input{MaintainableSDE: maintainableSDE, Multiple: assumptions.SDEMultiple})
			if !res.Available {
				t.Fatalf("unexpected errors: %+v", res.Errors)
			}
			if res.EquityValue <= 0 {
				t.Errorf("%s: EquityValue = %v, expected positive for a healthy profitable archetype", archetype, res.EquityValue)
			}
		})
	}
}
