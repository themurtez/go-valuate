package earnings

import (
	"math"
	"testing"
)

func fy(period string, value float64) Observation {
	return Observation{Period: period, PeriodType: PeriodTypeFiscalYear, Value: value, Available: true}
}

func TestCalculate_MultiplePeriods_LatestPeriod(t *testing.T) {
	obs := []Observation{fy("2023", 300000), fy("2024", 350000), fy("2025", 420000)}
	res := Calculate(obs, Options{Strategy: StrategyLatestPeriod})
	if !res.Available || res.Value != 420000 {
		t.Fatalf("Result = %+v, want available 420000", res)
	}
	if len(res.IncludedPeriods) != 1 || res.IncludedPeriods[0].Period != "2025" {
		t.Errorf("IncludedPeriods = %+v, want just 2025", res.IncludedPeriods)
	}
	if len(res.ExcludedPeriods) != 2 {
		t.Errorf("expected 2023/2024 excluded as not-latest, got %+v", res.ExcludedPeriods)
	}
}

func TestCalculate_LatestEarnings_SinglePeriod(t *testing.T) {
	obs := []Observation{fy("2025", 500000)}
	res := Calculate(obs, Options{Strategy: StrategyLatestPeriod})
	if !res.Available || res.Value != 500000 {
		t.Fatalf("Result = %+v, want available 500000", res)
	}
}

func TestCalculate_SimpleAverage(t *testing.T) {
	obs := []Observation{fy("2023", 300000), fy("2024", 350000), fy("2025", 400000)}
	res := Calculate(obs, Options{Strategy: StrategySimpleAverage})
	want := (300000.0 + 350000 + 400000) / 3
	if !res.Available || res.Value != want {
		t.Fatalf("Result = %+v, want available %v", res, want)
	}
	if len(res.IncludedPeriods) != 3 {
		t.Errorf("expected all 3 periods included, got %+v", res.IncludedPeriods)
	}
}

func TestCalculate_WeightedAverage_Percentages(t *testing.T) {
	obs := []Observation{fy("2023", 200000), fy("2024", 300000), fy("2025", 500000)}
	weights := map[string]float64{"2023": 20, "2024": 30, "2025": 50}
	res := Calculate(obs, Options{Strategy: StrategyWeightedAverage, Weights: weights})
	want := 200000*0.20 + 300000*0.30 + 500000*0.50
	if !res.Available {
		t.Fatalf("expected available result, got errors %v", res.Errors)
	}
	if math.Abs(res.Value-want) > 0.001 {
		t.Errorf("Value = %v, want %v", res.Value, want)
	}
	if len(res.Weights) != 3 {
		t.Fatalf("expected 3 weight entries, got %+v", res.Weights)
	}
}

func TestCalculate_WeightedAverage_Fractions(t *testing.T) {
	obs := []Observation{fy("2023", 200000), fy("2024", 300000), fy("2025", 500000)}
	weights := map[string]float64{"2023": 0.2, "2024": 0.3, "2025": 0.5}
	res := Calculate(obs, Options{Strategy: StrategyWeightedAverage, Weights: weights})
	want := 200000*0.20 + 300000*0.30 + 500000*0.50
	if !res.Available || math.Abs(res.Value-want) > 0.001 {
		t.Fatalf("Result = %+v, want available %v", res, want)
	}
}

func TestCalculate_InvalidWeights_DoNotSumToOneOrHundred(t *testing.T) {
	obs := []Observation{fy("2023", 200000), fy("2024", 300000), fy("2025", 500000)}
	weights := map[string]float64{"2023": 1, "2024": 2, "2025": 3} // sums to 6
	res := Calculate(obs, Options{Strategy: StrategyWeightedAverage, Weights: weights})
	if res.Available {
		t.Fatalf("expected weights summing to 6 to be rejected, got %+v", res)
	}
	if len(res.Errors) == 0 {
		t.Error("expected an explanatory error")
	}
}

func TestCalculate_InvalidWeights_Negative(t *testing.T) {
	obs := []Observation{fy("2023", 200000), fy("2024", 300000), fy("2025", 500000)}
	weights := map[string]float64{"2023": -0.2, "2024": 0.5, "2025": 0.7}
	res := Calculate(obs, Options{Strategy: StrategyWeightedAverage, Weights: weights})
	if res.Available {
		t.Fatalf("expected negative weight to be rejected, got %+v", res)
	}
}

func TestCalculate_InvalidWeights_MissingPeriod(t *testing.T) {
	obs := []Observation{fy("2023", 200000), fy("2024", 300000), fy("2025", 500000)}
	weights := map[string]float64{"2023": 0.5, "2024": 0.5} // missing 2025
	res := Calculate(obs, Options{Strategy: StrategyWeightedAverage, Weights: weights})
	if !res.Available {
		t.Fatalf("expected result still available from the two weighted periods, got errors %v", res.Errors)
	}
	found := false
	for _, ex := range res.ExcludedPeriods {
		if ex.Observation.Period == "2025" && ex.Reason == ExclusionNoWeight {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 2025 excluded with ExclusionNoWeight, got %+v", res.ExcludedPeriods)
	}
}

func TestCalculate_IncomparablePeriods_YTDExcludedFromFiscalYearAverage(t *testing.T) {
	obs := []Observation{
		fy("2024", 400000),
		fy("2025", 450000),
		{Period: "2026-YTD", PeriodType: PeriodTypeYTD, Value: 200000, Available: true},
	}
	res := Calculate(obs, Options{Strategy: StrategySimpleAverage})
	want := (400000.0 + 450000) / 2
	if !res.Available || res.Value != want {
		t.Fatalf("Result = %+v, want available %v (YTD period must be excluded)", res, want)
	}
	found := false
	for _, ex := range res.ExcludedPeriods {
		if ex.Observation.Period == "2026-YTD" && ex.Reason == ExclusionIncomparablePeriodType {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 2026-YTD excluded as incomparable, got %+v", res.ExcludedPeriods)
	}
}

func TestCalculate_IncomparablePeriods_ExplicitComparableTypeOverride(t *testing.T) {
	// If the caller explicitly wants YTD comparisons only, they can select
	// that via ComparablePeriodType rather than relying on the inferred
	// "last observation's type" default.
	obs := []Observation{
		{Period: "2025-YTD", PeriodType: PeriodTypeYTD, Value: 180000, Available: true},
		{Period: "2026-YTD", PeriodType: PeriodTypeYTD, Value: 200000, Available: true},
		fy("2024", 400000),
	}
	res := Calculate(obs, Options{Strategy: StrategySimpleAverage, ComparablePeriodType: PeriodTypeYTD})
	want := (180000.0 + 200000) / 2
	if !res.Available || res.Value != want {
		t.Fatalf("Result = %+v, want available %v", res, want)
	}
}

func TestCalculate_NegativeEarnings(t *testing.T) {
	obs := []Observation{fy("2023", -50000), fy("2024", -20000), fy("2025", 10000)}
	res := Calculate(obs, Options{Strategy: StrategySimpleAverage})
	want := (-50000.0 - 20000 + 10000) / 3
	if !res.Available || res.Value != want {
		t.Fatalf("Result = %+v, want available %v", res, want)
	}
}

func TestCalculate_ZeroEarnings(t *testing.T) {
	obs := []Observation{fy("2024", 0), fy("2025", 0)}
	res := Calculate(obs, Options{Strategy: StrategySimpleAverage})
	if !res.Available || res.Value != 0 {
		t.Fatalf("Result = %+v, want available 0", res)
	}
}

func TestCalculate_ExcludedPeriods_Unavailable(t *testing.T) {
	obs := []Observation{
		fy("2023", 300000),
		{Period: "2024", PeriodType: PeriodTypeFiscalYear, Available: false},
		fy("2025", 400000),
	}
	res := Calculate(obs, Options{Strategy: StrategySimpleAverage})
	want := (300000.0 + 400000) / 2
	if !res.Available || res.Value != want {
		t.Fatalf("Result = %+v, want available %v", res, want)
	}
	found := false
	for _, ex := range res.ExcludedPeriods {
		if ex.Observation.Period == "2024" && ex.Reason == ExclusionUnavailable {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 2024 excluded as unavailable, got %+v", res.ExcludedPeriods)
	}
}

func TestCalculate_TrendAdjusted(t *testing.T) {
	// Perfectly linear series: 100, 200, 300, 400 -> next fitted point is 400
	// at the last index (index 3), matching the last observed value exactly
	// since the fit is exact for a perfectly linear series.
	obs := []Observation{fy("2022", 100000), fy("2023", 200000), fy("2024", 300000), fy("2025", 400000)}
	res := Calculate(obs, Options{Strategy: StrategyTrendAdjusted})
	if !res.Available {
		t.Fatalf("expected available result, got errors %v", res.Errors)
	}
	if math.Abs(res.Value-400000) > 0.01 {
		t.Errorf("Value = %v, want ~400000 for a perfectly linear series", res.Value)
	}
}

func TestCalculate_TrendAdjusted_RequiresThreePoints(t *testing.T) {
	obs := []Observation{fy("2024", 100000), fy("2025", 200000)}
	res := Calculate(obs, Options{Strategy: StrategyTrendAdjusted})
	if res.Available {
		t.Fatalf("expected trend_adjusted to be unavailable with only 2 points, got %+v", res)
	}
}

func TestCalculate_EmptyObservations(t *testing.T) {
	res := Calculate(nil, Options{Strategy: StrategySimpleAverage})
	if res.Available {
		t.Fatalf("expected unavailable result for empty observations, got %+v", res)
	}
}

func TestCalculate_UnrecognizedStrategy(t *testing.T) {
	obs := []Observation{fy("2025", 100000)}
	res := Calculate(obs, Options{Strategy: "not_a_real_strategy"})
	if res.Available {
		t.Fatalf("expected unavailable result for unrecognized strategy, got %+v", res)
	}
}
