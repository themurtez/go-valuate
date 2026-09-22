package netassets

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type archetypeAssumptions struct {
	NetAssetValue struct {
		Assets      []AssetItem     `json:"assets"`
		Liabilities []LiabilityItem `json:"liabilities"`
	} `json:"net_asset_value"`
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

func sumAmounts[T interface{ AssetItem | LiabilityItem }](items []T, amount func(T) float64) float64 {
	total := 0.0
	for _, item := range items {
		total += amount(item)
	}
	return total
}

func TestFixtures_HVAC(t *testing.T) {
	a := loadAssumptions(t, "hvac")
	res := Calculate(Input{Assets: a.NetAssetValue.Assets, Liabilities: a.NetAssetValue.Liabilities})

	if !res.Available {
		t.Fatalf("unexpected errors: %+v", res.Errors)
	}
	wantAssets := sumAmounts(a.NetAssetValue.Assets, func(i AssetItem) float64 { return i.Amount })
	wantLiabilities := sumAmounts(a.NetAssetValue.Liabilities, func(i LiabilityItem) float64 { return i.Amount })
	if res.TotalAdjustedAssets != wantAssets {
		t.Errorf("TotalAdjustedAssets = %v, want %v", res.TotalAdjustedAssets, wantAssets)
	}
	if res.TotalAdjustedLiabilities != wantLiabilities {
		t.Errorf("TotalAdjustedLiabilities = %v, want %v", res.TotalAdjustedLiabilities, wantLiabilities)
	}
	if res.AdjustedNetAssetValue != wantAssets-wantLiabilities {
		t.Errorf("AdjustedNetAssetValue = %v, want %v", res.AdjustedNetAssetValue, wantAssets-wantLiabilities)
	}
	// This fixture's HVAC assets are exactly the book-value 2025 balance
	// sheet (see fixtures/valuation_by_business_type.json's hvac
	// net_asset_value comment) with no override items — verifying the NAV
	// therefore equals book equity independently confirms this package
	// applies no hidden adjustment of its own.
	const wantBookEquity = 128400 // liabilities+equity from the HVAC 2025 fixture, per completion-report verification
	if res.AdjustedNetAssetValue != wantBookEquity {
		t.Errorf("AdjustedNetAssetValue = %v, want %v (book equity, since no override items are present)", res.AdjustedNetAssetValue, wantBookEquity)
	}
}

func TestFixtures_Agency(t *testing.T) {
	a := loadAssumptions(t, "agency")
	res := Calculate(Input{Assets: a.NetAssetValue.Assets, Liabilities: a.NetAssetValue.Liabilities})

	if !res.Available {
		t.Fatalf("unexpected errors: %+v", res.Errors)
	}
	const wantBookEquity = 305500
	if res.AdjustedNetAssetValue != wantBookEquity {
		t.Errorf("AdjustedNetAssetValue = %v, want %v", res.AdjustedNetAssetValue, wantBookEquity)
	}
}

func TestFixtures_Manufacturer_AppraisalOverrideExceedsBookEquity(t *testing.T) {
	a := loadAssumptions(t, "manufacturer")
	res := Calculate(Input{Assets: a.NetAssetValue.Assets, Liabilities: a.NetAssetValue.Liabilities})

	if !res.Available {
		t.Fatalf("unexpected errors: %+v", res.Errors)
	}

	var overrideCount int
	var overrideAmount float64
	for _, item := range a.NetAssetValue.Assets {
		if item.IsOverride {
			overrideCount++
			overrideAmount += item.Amount
		}
	}
	if overrideCount != 1 {
		t.Fatalf("expected exactly 1 override asset item in the manufacturer fixture, got %d", overrideCount)
	}

	const bookEquity = 2041800 // manufacturer 2025 book equity, per completion-report verification
	wantNAV := bookEquity + overrideAmount
	if res.AdjustedNetAssetValue != wantNAV {
		t.Errorf("AdjustedNetAssetValue = %v, want %v (book equity %v + appraisal override %v)", res.AdjustedNetAssetValue, wantNAV, bookEquity, overrideAmount)
	}
	if res.AdjustedNetAssetValue <= bookEquity {
		t.Errorf("expected the appraisal override to push NAV (%v) above unadjusted book equity (%v)", res.AdjustedNetAssetValue, bookEquity)
	}
}

func TestFixtures_SaaS(t *testing.T) {
	a := loadAssumptions(t, "saas")
	res := Calculate(Input{Assets: a.NetAssetValue.Assets, Liabilities: a.NetAssetValue.Liabilities})

	if !res.Available {
		t.Fatalf("unexpected errors: %+v", res.Errors)
	}
	const wantBookEquity = 1858000
	if res.AdjustedNetAssetValue != wantBookEquity {
		t.Errorf("AdjustedNetAssetValue = %v, want %v", res.AdjustedNetAssetValue, wantBookEquity)
	}
}

func TestFixtures_EveryArchetypeCalculatesWithNoErrors(t *testing.T) {
	for _, archetype := range []string{"hvac", "agency", "manufacturer", "saas"} {
		t.Run(archetype, func(t *testing.T) {
			a := loadAssumptions(t, archetype)
			res := Calculate(Input{Assets: a.NetAssetValue.Assets, Liabilities: a.NetAssetValue.Liabilities})
			if !res.Available {
				t.Fatalf("unexpected errors: %+v", res.Errors)
			}
			if res.AdjustedNetAssetValue <= 0 {
				t.Errorf("%s: AdjustedNetAssetValue = %v, expected positive for a solvent archetype", archetype, res.AdjustedNetAssetValue)
			}
			if len(res.AssetComponents) != len(a.NetAssetValue.Assets) {
				t.Errorf("%s: expected %d asset components, got %d", archetype, len(a.NetAssetValue.Assets), len(res.AssetComponents))
			}
		})
	}
}
