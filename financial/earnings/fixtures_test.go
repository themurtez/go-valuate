package earnings

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

// ebitdaObservations builds a chronologically-ordered []Observation from a
// three-fiscal-year normalized dataset fixture's EBITDA values.
func ebitdaObservations(t *testing.T, datasetFixture string) []Observation {
	t.Helper()
	ds := loadFixtureDataset(t, datasetFixture)
	result := metrics.Calculate(ds, metrics.Options{})

	var obs []Observation
	for _, period := range []financial.Period{"2023", "2024", "2025"} {
		snap, ok := result.SnapshotFor(period)
		if !ok {
			t.Fatalf("expected a %s snapshot in %s", period, datasetFixture)
		}
		obs = append(obs, Observation{
			Period:     string(period),
			PeriodType: PeriodTypeFiscalYear,
			Value:      snap.EBITDA.Value,
			Available:  snap.EBITDA.Available,
		})
	}
	return obs
}

func TestFixtures_HVACLatestEarnings(t *testing.T) {
	obs := ebitdaObservations(t, "normalized_hvac_multi_year.json")
	res := Calculate(obs, Options{Strategy: StrategyLatestPeriod})
	if !res.Available {
		t.Fatalf("expected available result, got errors %v", res.Errors)
	}
	if res.Value != obs[2].Value {
		t.Errorf("Value = %v, want the 2025 EBITDA value %v", res.Value, obs[2].Value)
	}
}

func TestFixtures_AgencySimpleAverage(t *testing.T) {
	obs := ebitdaObservations(t, "normalized_agency_multi_year.json")
	res := Calculate(obs, Options{Strategy: StrategySimpleAverage})
	if !res.Available {
		t.Fatalf("expected available result, got errors %v", res.Errors)
	}
	want := (obs[0].Value + obs[1].Value + obs[2].Value) / 3
	if res.Value != want {
		t.Errorf("Value = %v, want %v", res.Value, want)
	}
}

func TestFixtures_ManufacturerWeightedAverage(t *testing.T) {
	obs := ebitdaObservations(t, "normalized_manufacturer_multi_year.json")
	weights := map[string]float64{"2023": 0.2, "2024": 0.3, "2025": 0.5}
	res := Calculate(obs, Options{Strategy: StrategyWeightedAverage, Weights: weights})
	if !res.Available {
		t.Fatalf("expected available result, got errors %v", res.Errors)
	}
	want := obs[0].Value*0.2 + obs[1].Value*0.3 + obs[2].Value*0.5
	if res.Value != want {
		t.Errorf("Value = %v, want %v", res.Value, want)
	}
}

func TestFixtures_SaaSTrendAdjusted(t *testing.T) {
	obs := ebitdaObservations(t, "normalized_saas_multi_year.json")
	res := Calculate(obs, Options{Strategy: StrategyTrendAdjusted})
	if !res.Available {
		t.Fatalf("expected available result for a growth-stage company with 3 fiscal years, got errors %v", res.Errors)
	}
	// SaaS EBITDA is growing year over year per the metrics fixtures test;
	// the trend-adjusted value should exceed the simple average, since a
	// rising trend's fitted endpoint sits above the flat mean.
	avgRes := Calculate(obs, Options{Strategy: StrategySimpleAverage})
	if res.Value <= avgRes.Value {
		t.Errorf("expected trend-adjusted value (%v) to exceed simple average (%v) for a growing series", res.Value, avgRes.Value)
	}
}

func TestFixtures_MixedFullYearAndYTDIsNotBlindlyAveraged(t *testing.T) {
	obs := ebitdaObservations(t, "normalized_hvac_multi_year.json")
	obs = append(obs, Observation{Period: "2026-YTD", PeriodType: PeriodTypeYTD, Value: 40000, Available: true})

	res := Calculate(obs, Options{Strategy: StrategySimpleAverage})
	if !res.Available {
		t.Fatalf("expected available result, got errors %v", res.Errors)
	}
	want := (obs[0].Value + obs[1].Value + obs[2].Value) / 3
	if res.Value != want {
		t.Errorf("Value = %v, want %v (YTD period must be excluded from the fiscal-year average)", res.Value, want)
	}
	foundYTDExcluded := false
	for _, ex := range res.ExcludedPeriods {
		if ex.Observation.Period == "2026-YTD" {
			foundYTDExcluded = true
		}
	}
	if !foundYTDExcluded {
		t.Error("expected the 2026-YTD observation to appear in ExcludedPeriods")
	}
}
