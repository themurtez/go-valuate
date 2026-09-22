package adjustments

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/metrics"
)

func loadFixtureDataset(t *testing.T, name string) financial.FinancialDataset {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "fixtures", name))
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	var ds financial.FinancialDataset
	if err := json.Unmarshal(b, &ds); err != nil {
		t.Fatalf("unmarshaling fixture %s: %v", name, err)
	}
	return ds
}

// loadAdjustmentFixtures loads fixtures/adjustments_by_business_type.json,
// keyed by business archetype ("hvac", "agency", "manufacturer", "saas").
func loadAdjustmentFixtures(t *testing.T) map[string][]Adjustment {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "fixtures", "adjustments_by_business_type.json"))
	if err != nil {
		t.Fatalf("reading adjustments fixture: %v", err)
	}
	var byArchetype map[string][]Adjustment
	if err := json.Unmarshal(b, &byArchetype); err != nil {
		t.Fatalf("unmarshaling adjustments fixture: %v", err)
	}
	return byArchetype
}

func snapshot2025(t *testing.T, datasetFixture string) metrics.Snapshot {
	t.Helper()
	ds := loadFixtureDataset(t, datasetFixture)
	result := metrics.Calculate(ds, metrics.Options{})
	snap, ok := result.SnapshotFor("2025")
	if !ok {
		t.Fatalf("expected a 2025 snapshot in %s", datasetFixture)
	}
	return snap
}

func TestFixtures_HVAC_OwnerCompVehicleOneTimeRepair(t *testing.T) {
	snap := snapshot2025(t, "normalized_hvac_multi_year.json")
	adjs := loadAdjustmentFixtures(t)["hvac"]
	if len(adjs) != 3 {
		t.Fatalf("expected 3 HVAC adjustments, got %d", len(adjs))
	}

	res := Apply(snap, adjs)
	if HasErrors(res.Errors) {
		t.Fatalf("unexpected errors applying HVAC fixture adjustments: %+v", res.Errors)
	}

	// EBITDA bridge: owner comp normalization is SDE-only, so only
	// personal vehicle (+7200) and one-time repair (+5100) apply.
	wantEBITDA := snap.EBITDA.Value + 7200 + 5100
	if res.EBITDABridge.NormalizedValue != wantEBITDA {
		t.Errorf("normalized EBITDA = %v, want %v", res.EBITDABridge.NormalizedValue, wantEBITDA)
	}
	if len(res.EBITDABridge.Applied) != 2 {
		t.Errorf("expected 2 applied EBITDA lines, got %d: %+v", len(res.EBITDABridge.Applied), res.EBITDABridge.Applied)
	}

	// SDE bridge: all three apply, including the owner comp normalization
	// (-32000).
	wantSDE := snap.SDE.Value - 32000 + 7200 + 5100
	if res.SDEBridge.NormalizedValue != wantSDE {
		t.Errorf("normalized SDE = %v, want %v", res.SDEBridge.NormalizedValue, wantSDE)
	}
	if len(res.SDEBridge.Applied) != 3 {
		t.Errorf("expected 3 applied SDE lines, got %d: %+v", len(res.SDEBridge.Applied), res.SDEBridge.Applied)
	}
}

func TestFixtures_Agency_OwnerSalaryAndRebranding(t *testing.T) {
	snap := snapshot2025(t, "normalized_agency_multi_year.json")
	adjs := loadAdjustmentFixtures(t)["agency"]
	if len(adjs) != 2 {
		t.Fatalf("expected 2 agency adjustments, got %d", len(adjs))
	}

	res := Apply(snap, adjs)
	if HasErrors(res.Errors) {
		t.Fatalf("unexpected errors: %+v", res.Errors)
	}

	wantEBITDA := snap.EBITDA.Value + 18500
	if res.EBITDABridge.NormalizedValue != wantEBITDA {
		t.Errorf("normalized EBITDA = %v, want %v", res.EBITDABridge.NormalizedValue, wantEBITDA)
	}
	wantSDE := snap.SDE.Value - 55000 + 18500
	if res.SDEBridge.NormalizedValue != wantSDE {
		t.Errorf("normalized SDE = %v, want %v", res.SDEBridge.NormalizedValue, wantSDE)
	}
}

func TestFixtures_Manufacturer_UnusualRepairAndRelatedPartyRent(t *testing.T) {
	snap := snapshot2025(t, "normalized_manufacturer_multi_year.json")
	adjs := loadAdjustmentFixtures(t)["manufacturer"]
	if len(adjs) != 2 {
		t.Fatalf("expected 2 manufacturer adjustments, got %d", len(adjs))
	}

	res := Apply(snap, adjs)
	if HasErrors(res.Errors) {
		t.Fatalf("unexpected errors: %+v", res.Errors)
	}

	// Manufacturer has no owner-operator, so SDE == EBITDA before
	// adjustments; both adjustments apply to both bridges equally.
	wantEBITDA := snap.EBITDA.Value + 22000 + 14000
	if res.EBITDABridge.NormalizedValue != wantEBITDA {
		t.Errorf("normalized EBITDA = %v, want %v", res.EBITDABridge.NormalizedValue, wantEBITDA)
	}
	wantSDE := snap.SDE.Value + 22000 + 14000
	if res.SDEBridge.NormalizedValue != wantSDE {
		t.Errorf("normalized SDE = %v, want %v", res.SDEBridge.NormalizedValue, wantSDE)
	}
}

func TestFixtures_SaaS_UnusualFeesAndNonOperatingIncomeRemoval(t *testing.T) {
	snap := snapshot2025(t, "normalized_saas_multi_year.json")
	adjs := loadAdjustmentFixtures(t)["saas"]
	if len(adjs) != 2 {
		t.Fatalf("expected 2 SaaS adjustments, got %d", len(adjs))
	}

	res := Apply(snap, adjs)
	if HasErrors(res.Errors) {
		t.Fatalf("unexpected errors: %+v", res.Errors)
	}

	// +24000 for non-recurring fees, -30000 for non-operating income
	// removal: net decrease of 6000.
	wantEBITDA := snap.EBITDA.Value + 24000 - 30000
	if res.EBITDABridge.NormalizedValue != wantEBITDA {
		t.Errorf("normalized EBITDA = %v, want %v", res.EBITDABridge.NormalizedValue, wantEBITDA)
	}
	if res.EBITDABridge.NormalizedValue >= snap.EBITDA.Value {
		t.Error("expected normalized EBITDA to be lower than reported EBITDA once non-operating income is removed net of the smaller fee add-back")
	}
}

func TestFixtures_EveryArchetypeAppliesWithNoValidationErrors(t *testing.T) {
	byArchetype := loadAdjustmentFixtures(t)
	datasets := map[string]string{
		"hvac":         "normalized_hvac_multi_year.json",
		"agency":       "normalized_agency_multi_year.json",
		"manufacturer": "normalized_manufacturer_multi_year.json",
		"saas":         "normalized_saas_multi_year.json",
	}
	for archetype, datasetFixture := range datasets {
		t.Run(archetype, func(t *testing.T) {
			snap := snapshot2025(t, datasetFixture)
			adjs := byArchetype[archetype]
			issues := Validate(adjs, snap)
			if HasErrors(issues) {
				t.Errorf("unexpected validation errors for %s: %+v", archetype, issues)
			}
		})
	}
}
