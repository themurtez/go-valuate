package metrics

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/themurtez/go-valuate/financial"
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

// businessFYMeta builds fiscal-year PeriodInfo for the three consecutive
// years ("2023","2024","2025") every multi-year business archetype fixture
// uses.
func businessFYMeta() map[financial.Period]PeriodInfo {
	return map[financial.Period]PeriodInfo{
		"2023": {Type: PeriodTypeFiscalYear, FiscalYear: 2023},
		"2024": {Type: PeriodTypeFiscalYear, FiscalYear: 2024},
		"2025": {Type: PeriodTypeFiscalYear, FiscalYear: 2025},
	}
}

func TestFixtures_HVACThreeYearMetrics(t *testing.T) {
	ds := loadFixtureDataset(t, "normalized_hvac_multi_year.json")
	result := Calculate(ds, Options{PeriodMeta: businessFYMeta()})

	if len(result.Snapshots) != 3 {
		t.Fatalf("expected 3 snapshots, got %d", len(result.Snapshots))
	}
	s2025, ok := result.SnapshotFor("2025")
	if !ok {
		t.Fatal("expected a 2025 snapshot")
	}
	if !s2025.TotalRevenue.Available || s2025.TotalRevenue.Value != 212000+498000 {
		t.Errorf("2025 TotalRevenue = %+v, want %v", s2025.TotalRevenue, 212000.0+498000.0)
	}
	if !s2025.SDE.Available {
		t.Fatal("expected SDE to be available for an owner-operated business")
	}
	// SDE should exceed EBITDA by exactly owner compensation (92000), since
	// this archetype has a real owner-operator drawing compensation.
	if s2025.SDE.Value-s2025.EBITDA.Value != s2025.OwnerCompensation.Value {
		t.Errorf("SDE - EBITDA = %v, want equal to OwnerCompensation %v", s2025.SDE.Value-s2025.EBITDA.Value, s2025.OwnerCompensation.Value)
	}

	if result.Trend == nil {
		t.Fatal("expected a Trend across 3 fiscal years")
	}
	if result.Trend.Error != nil {
		t.Fatalf("unexpected trend error: %v", result.Trend.Error)
	}
	if len(result.Trend.RevenueYoYGrowth) != 2 {
		t.Errorf("expected 2 YoY growth points across 3 years, got %d", len(result.Trend.RevenueYoYGrowth))
	}
	if !result.Trend.RevenueCAGR.Value.Available {
		t.Error("expected RevenueCAGR to be available for a consistently-positive-revenue business")
	}
}

func TestFixtures_AgencyHasNoInventoryOrCOGSMaterials(t *testing.T) {
	ds := loadFixtureDataset(t, "normalized_agency_multi_year.json")
	result := Calculate(ds, Options{PeriodMeta: businessFYMeta()})
	s, ok := result.SnapshotFor("2025")
	if !ok {
		t.Fatal("expected a 2025 snapshot")
	}
	if s.Inventory.Available {
		t.Error("expected Inventory to be unavailable for a professional services business with no inventory line")
	}
	// COGS here is direct labor only; gross profit should still be
	// calculable.
	if !s.GrossProfit.Available {
		t.Fatal("expected GrossProfit to be available even with COGS limited to direct labor")
	}
}

func TestFixtures_ManufacturerHasSubstantialTangibleAssetValue(t *testing.T) {
	ds := loadFixtureDataset(t, "normalized_manufacturer_multi_year.json")
	result := Calculate(ds, Options{PeriodMeta: businessFYMeta()})
	s, ok := result.SnapshotFor("2025")
	if !ok {
		t.Fatal("expected a 2025 snapshot")
	}
	if !s.TangibleAssetValue.Available {
		t.Fatal("expected TangibleAssetValue to be available for an asset-heavy manufacturer")
	}
	if s.TangibleAssetValue.Value <= 0 {
		t.Errorf("TangibleAssetValue = %v, want a substantial positive figure for an asset-heavy business", s.TangibleAssetValue.Value)
	}
	if !s.TotalDebt.Available || s.TotalDebt.Value <= 0 {
		t.Error("expected the manufacturer to carry meaningful debt")
	}
}

func TestFixtures_SaaSHasNegativeRetainedEarningsButPositiveEBITDATrend(t *testing.T) {
	ds := loadFixtureDataset(t, "normalized_saas_multi_year.json")
	result := Calculate(ds, Options{PeriodMeta: businessFYMeta()})
	s2023, _ := result.SnapshotFor("2023")
	s2025, _ := result.SnapshotFor("2025")

	if !s2023.EBITDA.Available || !s2025.EBITDA.Available {
		t.Fatal("expected EBITDA to be available in both years")
	}
	if s2025.EBITDA.Value <= s2023.EBITDA.Value {
		t.Errorf("expected EBITDA to grow from 2023 (%v) to 2025 (%v) for this growth-stage company", s2023.EBITDA.Value, s2025.EBITDA.Value)
	}
	if s2025.SDE.Value != s2025.EBITDA.Value {
		t.Error("expected SDE to equal EBITDA for a non-owner-operated company with no owner compensation")
	}

	if result.Trend.EBITDAYoYGrowth == nil {
		t.Error("expected EBITDAYoYGrowth to be populated across 3 fiscal years")
	}
}

func TestFixtures_EveryArchetypeProducesExplainableResults(t *testing.T) {
	for _, name := range []string{
		"normalized_hvac_multi_year.json",
		"normalized_agency_multi_year.json",
		"normalized_manufacturer_multi_year.json",
		"normalized_saas_multi_year.json",
	} {
		ds := loadFixtureDataset(t, name)
		result := Calculate(ds, Options{PeriodMeta: businessFYMeta()})
		for _, snap := range result.Snapshots {
			ebitdaResult, ok := snap.Results[MetricEBITDA]
			if !ok {
				t.Fatalf("%s/%s: expected an EBITDA entry in Results", name, snap.Period)
			}
			if ebitdaResult.Formula == "" {
				t.Errorf("%s/%s: expected a non-empty Formula for EBITDA", name, snap.Period)
			}
			if ebitdaResult.Value.Available && len(ebitdaResult.Components) == 0 {
				t.Errorf("%s/%s: expected Components to explain an available EBITDA value", name, snap.Period)
			}
		}
	}
}
