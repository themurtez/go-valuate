package ebitda

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/metrics"
)

type archetypeAssumptions struct {
	EBITDAMultiple float64 `json:"ebitda_multiple"`
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

// snapshot2025 loads an archetype's three-fiscal-year dataset and returns
// its 2025 metrics.Snapshot, the same live-derivation approach
// valuation/sde's fixtures_test.go and financial/adjustments' use.
func snapshot2025(t *testing.T, datasetFixture string) metrics.Snapshot {
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
	if !snap.EBITDA.Available {
		t.Fatalf("EBITDA unavailable for %s 2025", datasetFixture)
	}
	return snap
}

func TestFixtures_HVAC_EnterpriseValue(t *testing.T) {
	assumptions := loadAssumptions(t, "hvac")
	snap := snapshot2025(t, "normalized_hvac_multi_year.json")

	res := Calculate(Input{MaintainableEBITDA: snap.EBITDA.Value, Multiple: assumptions.EBITDAMultiple})

	if !res.Available {
		t.Fatalf("unexpected errors: %+v", res.Errors)
	}
	wantEV := snap.EBITDA.Value * assumptions.EBITDAMultiple
	if res.EnterpriseValue != wantEV {
		t.Errorf("EnterpriseValue = %v, want %v", res.EnterpriseValue, wantEV)
	}
}

func TestFixtures_HVAC_EquityBridgeWithDebtAndCash(t *testing.T) {
	assumptions := loadAssumptions(t, "hvac")
	snap := snapshot2025(t, "normalized_hvac_multi_year.json")

	if !snap.Cash.Available || !snap.ShortTermDebt.Available || !snap.LongTermDebt.Available {
		t.Fatal("expected cash/debt to be available in the HVAC 2025 snapshot")
	}

	res := Calculate(Input{
		MaintainableEBITDA: snap.EBITDA.Value, Multiple: assumptions.EBITDAMultiple,
		EquityBridge: EquityBridgeInput{
			Requested:     true,
			ExcessCash:    snap.Cash.Value,
			ShortTermDebt: snap.ShortTermDebt.Value,
			LongTermDebt:  snap.LongTermDebt.Value,
		},
	})

	if !res.Available || !res.Bridge.Available {
		t.Fatalf("unexpected errors: %+v", res.Errors)
	}
	wantEV := snap.EBITDA.Value * assumptions.EBITDAMultiple
	wantTotalDebt := snap.ShortTermDebt.Value + snap.LongTermDebt.Value
	wantEquity := wantEV + snap.Cash.Value - wantTotalDebt
	if res.Bridge.EquityValue != wantEquity {
		t.Errorf("Bridge.EquityValue = %v, want %v", res.Bridge.EquityValue, wantEquity)
	}
	// HVAC's 2025 cash ($63,000) exceeds its total debt ($55,000) per the
	// underlying fixture, so equity value should exceed enterprise value.
	if res.Bridge.EquityValue <= res.EnterpriseValue {
		t.Errorf("expected equity value (%v) to exceed enterprise value (%v) given HVAC's net-cash position", res.Bridge.EquityValue, res.EnterpriseValue)
	}
}

func TestFixtures_Agency_DebtFreeBridge(t *testing.T) {
	assumptions := loadAssumptions(t, "agency")
	snap := snapshot2025(t, "normalized_agency_multi_year.json")

	// The agency archetype's fixture reports no debt codes at all (see
	// financial/metrics' TotalDebt.Available == false for this archetype),
	// so a caller bridging to equity supplies 0 for debt fields explicitly
	// rather than reading an unavailable metric.
	res := Calculate(Input{
		MaintainableEBITDA: snap.EBITDA.Value, Multiple: assumptions.EBITDAMultiple,
		EquityBridge: EquityBridgeInput{Requested: true, ExcessCash: snap.Cash.Value},
	})

	if !res.Available || !res.Bridge.Available {
		t.Fatalf("unexpected errors: %+v", res.Errors)
	}
	if res.Bridge.TotalDebt != 0 {
		t.Errorf("TotalDebt = %v, want 0 for a debt-free business", res.Bridge.TotalDebt)
	}
	wantEquity := snap.EBITDA.Value*assumptions.EBITDAMultiple + snap.Cash.Value
	if res.Bridge.EquityValue != wantEquity {
		t.Errorf("Bridge.EquityValue = %v, want %v", res.Bridge.EquityValue, wantEquity)
	}
}

func TestFixtures_Manufacturer_LeveredBridge(t *testing.T) {
	assumptions := loadAssumptions(t, "manufacturer")
	snap := snapshot2025(t, "normalized_manufacturer_multi_year.json")

	res := Calculate(Input{
		MaintainableEBITDA: snap.EBITDA.Value, Multiple: assumptions.EBITDAMultiple,
		EquityBridge: EquityBridgeInput{
			Requested:     true,
			ExcessCash:    snap.Cash.Value,
			ShortTermDebt: snap.ShortTermDebt.Value,
			LongTermDebt:  snap.LongTermDebt.Value,
		},
	})

	if !res.Available || !res.Bridge.Available {
		t.Fatalf("unexpected errors: %+v", res.Errors)
	}
	// Manufacturer carries $965,000 of total debt against $238,000 cash in
	// 2025 — a net-debt position, so equity value must be materially below
	// enterprise value.
	if res.Bridge.EquityValue >= res.EnterpriseValue {
		t.Errorf("expected equity value (%v) to be below enterprise value (%v) given the manufacturer's net-debt position", res.Bridge.EquityValue, res.EnterpriseValue)
	}
	if res.Bridge.TotalDebt != 965000 {
		t.Errorf("TotalDebt = %v, want 965000", res.Bridge.TotalDebt)
	}
}

func TestFixtures_SaaS_NegativeEBITDANotAssumed(t *testing.T) {
	// The SaaS archetype's 2025 EBITDA is strongly positive per the
	// underlying fixture; this test documents that expectation explicitly
	// so a future change to the fixture that turned it negative would be
	// caught here rather than only failing an unrelated assertion deep in
	// another test.
	assumptions := loadAssumptions(t, "saas")
	snap := snapshot2025(t, "normalized_saas_multi_year.json")

	res := Calculate(Input{MaintainableEBITDA: snap.EBITDA.Value, Multiple: assumptions.EBITDAMultiple})
	if !res.Available {
		t.Fatalf("unexpected errors: %+v", res.Errors)
	}
	if len(res.Warnings) != 0 {
		t.Errorf("expected no warnings for a healthy positive-EBITDA business, got %+v", res.Warnings)
	}
	if res.EnterpriseValue <= 0 {
		t.Errorf("EnterpriseValue = %v, expected positive", res.EnterpriseValue)
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
			snap := snapshot2025(t, datasetFixture)
			res := Calculate(Input{MaintainableEBITDA: snap.EBITDA.Value, Multiple: assumptions.EBITDAMultiple})
			if !res.Available {
				t.Fatalf("unexpected errors: %+v", res.Errors)
			}
		})
	}
}
