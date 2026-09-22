package workingcapital

import (
	"math"
	"testing"
)

func TestCalculateStatistics_Empty(t *testing.T) {
	stats := calculateStatistics(nil)
	if stats.SampleSize != 0 {
		t.Errorf("expected SampleSize 0, got %d", stats.SampleSize)
	}
	if stats.Average.Available || stats.Median.Available || stats.Min.Available || stats.Max.Available {
		t.Error("expected every statistic unavailable for an empty series")
	}
}

func TestCalculateStatistics_SkipsUnavailable(t *testing.T) {
	series := []NWCValue{
		AvailableValue(10),
		Unavailable(),
		AvailableValue(20),
		AvailableValue(30),
	}
	stats := calculateStatistics(series)
	if stats.SampleSize != 3 {
		t.Fatalf("expected SampleSize 3, got %d", stats.SampleSize)
	}
	if stats.Average.Value != 20 {
		t.Errorf("Average = %v, want 20", stats.Average.Value)
	}
	if stats.Median.Value != 20 {
		t.Errorf("Median = %v, want 20", stats.Median.Value)
	}
	if stats.Min.Value != 10 {
		t.Errorf("Min = %v, want 10", stats.Min.Value)
	}
	if stats.Max.Value != 30 {
		t.Errorf("Max = %v, want 30", stats.Max.Value)
	}
}

func TestCalculateStatistics_MedianEvenCount(t *testing.T) {
	series := []NWCValue{
		AvailableValue(10),
		AvailableValue(20),
		AvailableValue(30),
		AvailableValue(40),
	}
	stats := calculateStatistics(series)
	want := (20.0 + 30.0) / 2
	if stats.Median.Value != want {
		t.Errorf("Median = %v, want %v", stats.Median.Value, want)
	}
}

func TestCalculateVolatility_RequiresAtLeastTwoGrowthPoints(t *testing.T) {
	// Only 2 periods -> 1 growth point -> insufficient.
	series := []NWCValue{AvailableValue(100), AvailableValue(110)}
	vol, n := calculateVolatility(series)
	if vol.Available {
		t.Errorf("expected unavailable volatility with only 1 growth point, got %+v", vol)
	}
	if n != 1 {
		t.Errorf("expected sample size 1, got %d", n)
	}
}

func TestCalculateVolatility_StableSeriesIsLow(t *testing.T) {
	series := []NWCValue{
		AvailableValue(100000),
		AvailableValue(101000),
		AvailableValue(99500),
		AvailableValue(100200),
	}
	vol, n := calculateVolatility(series)
	if !vol.Available {
		t.Fatal("expected volatility available")
	}
	if n != 3 {
		t.Errorf("expected 3 growth observations, got %d", n)
	}
	if vol.Value > 0.05 {
		t.Errorf("expected low volatility for a stable series, got %v", vol.Value)
	}
}

func TestCalculateVolatility_VolatileSeriesIsHigh(t *testing.T) {
	series := []NWCValue{
		AvailableValue(100000),
		AvailableValue(20000),
		AvailableValue(150000),
		AvailableValue(10000),
	}
	vol, _ := calculateVolatility(series)
	if !vol.Available {
		t.Fatal("expected volatility available")
	}
	if vol.Value < 0.5 {
		t.Errorf("expected high volatility for an erratic series, got %v", vol.Value)
	}
}

func TestCalculateVolatility_SkipsZeroBaseTransition(t *testing.T) {
	series := []NWCValue{
		AvailableValue(0),
		AvailableValue(100),
		AvailableValue(110),
		AvailableValue(105),
	}
	// The 0 -> 100 transition is a zero-base growth and must be skipped,
	// leaving only 2 valid growth points (100->110, 110->105).
	_, n := calculateVolatility(series)
	if n != 2 {
		t.Errorf("expected 2 valid growth observations (zero-base transition skipped), got %d", n)
	}
}

func TestCalculateTrend_FewerThanTwoObservations(t *testing.T) {
	trend := calculateTrend([]PeriodNWC{{Period: "2025", NWC: AvailableValue(100)}})
	if trend.Direction != TrendUnavailable {
		t.Errorf("expected TrendUnavailable with 1 observation, got %v", trend.Direction)
	}
}

func TestCalculateTrend_ZeroBaseNonZeroEnd(t *testing.T) {
	history := []PeriodNWC{
		{Period: "2023", NWC: AvailableValue(0)},
		{Period: "2024", NWC: AvailableValue(50000)},
	}
	trend := calculateTrend(history)
	if trend.Direction != TrendIncreasing {
		t.Errorf("expected TrendIncreasing from a zero base to a positive value, got %v", trend.Direction)
	}
	if trend.PercentChange.Available {
		t.Errorf("expected PercentChange unavailable from a zero base, got %+v", trend.PercentChange)
	}
}

func TestCalculateTrend_ZeroBaseZeroEnd(t *testing.T) {
	history := []PeriodNWC{
		{Period: "2023", NWC: AvailableValue(0)},
		{Period: "2024", NWC: AvailableValue(0)},
	}
	trend := calculateTrend(history)
	if trend.Direction != TrendStable {
		t.Errorf("expected TrendStable when both endpoints are 0, got %v", trend.Direction)
	}
}

func TestCalculateTrend_SkipsUnavailableObservations(t *testing.T) {
	history := []PeriodNWC{
		{Period: "2022", NWC: AvailableValue(10000)},
		{Period: "2023", NWC: Unavailable()},
		{Period: "2024", NWC: AvailableValue(20000)},
	}
	trend := calculateTrend(history)
	if trend.FirstPeriod != "2022" || trend.LastPeriod != "2024" {
		t.Errorf("expected first/last to skip the unavailable middle period, got %v/%v", trend.FirstPeriod, trend.LastPeriod)
	}
}

func TestMedian_OddAndEven(t *testing.T) {
	if got := median([]float64{3, 1, 2}); got != 2 {
		t.Errorf("median([3,1,2]) = %v, want 2", got)
	}
	if got := median([]float64{4, 1, 3, 2}); got != 2.5 {
		t.Errorf("median([4,1,3,2]) = %v, want 2.5", got)
	}
}

func TestCalculateVolatility_NoNaN(t *testing.T) {
	series := []NWCValue{AvailableValue(1), AvailableValue(1), AvailableValue(1)}
	vol, _ := calculateVolatility(series)
	if vol.Available && math.IsNaN(vol.Value) {
		t.Error("volatility must never be NaN")
	}
}
