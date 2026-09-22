package metrics

import (
	"math"
	"testing"

	"github.com/themurtez/go-valuate/financial"
)

func fyMeta(years ...int) map[financial.Period]PeriodInfo {
	meta := make(map[financial.Period]PeriodInfo, len(years))
	for _, y := range years {
		period := financial.Period(intToString(y))
		meta[period] = PeriodInfo{Type: PeriodTypeFiscalYear, FiscalYear: y}
	}
	return meta
}

func intToString(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		return "-" + string(digits)
	}
	return string(digits)
}

func TestCalculate_NoTrendWithFewerThanTwoPeriods(t *testing.T) {
	ds := dataset(item(financial.CodeRevProduct, "2025", 100000))
	result := Calculate(ds, Options{})
	if result.Trend != nil {
		t.Error("expected no Trend with only one period")
	}
}

func TestCalculate_TrendErrorWithoutPeriodMeta(t *testing.T) {
	ds := dataset(
		item(financial.CodeRevProduct, "2024", 100000),
		item(financial.CodeRevProduct, "2025", 120000),
	)
	result := Calculate(ds, Options{})
	if result.Trend == nil {
		t.Fatal("expected a non-nil Trend even on error, to carry the explanation")
	}
	if result.Trend.Error == nil {
		t.Error("expected Trend.Error to be set when no PeriodMeta was supplied")
	}
}

func TestCalculate_TrendErrorWhenPeriodMissingFromMeta(t *testing.T) {
	ds := dataset(
		item(financial.CodeRevProduct, "2024", 100000),
		item(financial.CodeRevProduct, "2025", 120000),
	)
	meta := fyMeta(2024) // 2025 deliberately omitted
	result := Calculate(ds, Options{PeriodMeta: meta})
	if result.Trend.Error == nil {
		t.Fatal("expected Trend.Error when a period has no PeriodMeta entry")
	}
	if len(result.Trend.Error.MissingPeriods) != 1 || result.Trend.Error.MissingPeriods[0] != "2025" {
		t.Errorf("MissingPeriods = %v, want [2025]", result.Trend.Error.MissingPeriods)
	}
}

func TestCalculate_DoesNotGuessOrderFromLexicalStrings(t *testing.T) {
	// Deliberately give periods whose lexical order is the OPPOSITE of
	// their real chronological order via FiscalYear, to prove ordering
	// comes from PeriodInfo and not string comparison.
	ds := dataset(
		item(financial.CodeRevProduct, "period-b", 100000), // chronologically first
		item(financial.CodeRevProduct, "period-a", 150000), // chronologically second
	)
	meta := map[financial.Period]PeriodInfo{
		"period-b": {Type: PeriodTypeFiscalYear, FiscalYear: 2024},
		"period-a": {Type: PeriodTypeFiscalYear, FiscalYear: 2025},
	}
	result := Calculate(ds, Options{PeriodMeta: meta})
	if result.Trend.Error != nil {
		t.Fatalf("unexpected trend error: %v", result.Trend.Error)
	}
	if len(result.Trend.FiscalYearsUsed) != 2 || result.Trend.FiscalYearsUsed[0] != "period-b" || result.Trend.FiscalYearsUsed[1] != "period-a" {
		t.Errorf("FiscalYearsUsed = %v, want [period-b period-a] (chronological via FiscalYear, not lexical)", result.Trend.FiscalYearsUsed)
	}
	growth := result.Trend.RevenueYoYGrowth[0]
	if growth.FromPeriod != "period-b" || growth.ToPeriod != "period-a" {
		t.Errorf("growth = %+v, want from period-b to period-a", growth)
	}
	if !growth.Growth.Available || growth.Growth.Value != 0.5 {
		t.Errorf("growth.Growth = %+v, want Available=true Value=0.5 (100000->150000)", growth.Growth)
	}
}

func TestYoYGrowth_PositiveGrowth(t *testing.T) {
	ds := dataset(
		item(financial.CodeRevProduct, "2024", 100000),
		item(financial.CodeRevProduct, "2025", 120000),
	)
	result := Calculate(ds, Options{PeriodMeta: fyMeta(2024, 2025)})
	if len(result.Trend.RevenueYoYGrowth) != 1 {
		t.Fatalf("expected 1 growth point, got %d", len(result.Trend.RevenueYoYGrowth))
	}
	g := result.Trend.RevenueYoYGrowth[0]
	if !g.Growth.Available || g.Growth.Value != 0.2 {
		t.Errorf("Growth = %+v, want Available=true Value=0.2", g.Growth)
	}
}

func TestYoYGrowth_NegativeGrowth(t *testing.T) {
	ds := dataset(
		item(financial.CodeRevProduct, "2024", 100000),
		item(financial.CodeRevProduct, "2025", 80000),
	)
	result := Calculate(ds, Options{PeriodMeta: fyMeta(2024, 2025)})
	g := result.Trend.RevenueYoYGrowth[0]
	if !g.Growth.Available || g.Growth.Value != -0.2 {
		t.Errorf("Growth = %+v, want Available=true Value=-0.2", g.Growth)
	}
}

func TestYoYGrowth_FromZeroBaseIsExplicitlyFlagged(t *testing.T) {
	ds := dataset(
		item(financial.CodeRevProduct, "2024", 0),
		item(financial.CodeRevProduct, "2025", 50000),
	)
	result := Calculate(ds, Options{PeriodMeta: fyMeta(2024, 2025)})
	g := result.Trend.RevenueYoYGrowth[0]
	if g.Growth.Available {
		t.Error("expected Growth to be unavailable when growing from a zero base")
	}
	if !g.GrowthFromZeroBase {
		t.Error("expected GrowthFromZeroBase to be true")
	}
}

func TestYoYGrowth_UnavailableWhenEitherEndpointUnavailable(t *testing.T) {
	ds := dataset(
		item(financial.CodeOpexPayroll, "2024", 1000), // no revenue in 2024
		item(financial.CodeRevProduct, "2025", 50000),
	)
	result := Calculate(ds, Options{PeriodMeta: fyMeta(2024, 2025)})
	g := result.Trend.RevenueYoYGrowth[0]
	if g.Growth.Available {
		t.Error("expected Growth to be unavailable when the prior period's revenue is unavailable")
	}
	if g.GrowthFromZeroBase {
		t.Error("expected GrowthFromZeroBase to be false when unavailability is due to missing data, not a zero base")
	}
}

func TestCAGR_ExactTenPercentOverFourYears(t *testing.T) {
	ds := dataset(
		item(financial.CodeRevProduct, "2021", 100000),
		item(financial.CodeRevProduct, "2022", 110000),
		item(financial.CodeRevProduct, "2023", 121000),
		item(financial.CodeRevProduct, "2024", 133100),
		item(financial.CodeRevProduct, "2025", 146410),
	)
	result := Calculate(ds, Options{PeriodMeta: fyMeta(2021, 2022, 2023, 2024, 2025)})
	cagr := result.Trend.RevenueCAGR
	if !cagr.Value.Available {
		t.Fatalf("expected CAGR to be available, Invalid=%q", cagr.Invalid)
	}
	if math.Abs(cagr.Value.Value-0.10) > 1e-9 {
		t.Errorf("CAGR = %v, want ~0.10 (10%%)", cagr.Value.Value)
	}
	if cagr.Years != 4 {
		t.Errorf("Years = %v, want 4", cagr.Years)
	}
}

func TestCAGR_InvalidWhenStartingValueIsZero(t *testing.T) {
	ds := dataset(
		item(financial.CodeRevProduct, "2024", 0),
		item(financial.CodeRevProduct, "2025", 50000),
	)
	result := Calculate(ds, Options{PeriodMeta: fyMeta(2024, 2025)})
	cagr := result.Trend.RevenueCAGR
	if cagr.Value.Available {
		t.Error("expected CAGR to be unavailable when starting value is zero")
	}
	if cagr.Invalid == "" {
		t.Error("expected a non-empty Invalid reason")
	}
}

func TestCAGR_InvalidWhenStartingValueIsNegative(t *testing.T) {
	// Revenue is realistically never negative, but earnings can be, so use
	// net income to exercise CAGR's negative-starting-value guard.
	ds := dataset(
		item(financial.CodeRevProduct, "2024", 100000),
		item(financial.CodeCogsMaterial, "2024", 60000),
		item(financial.CodeOpexPayroll, "2024", 90000), // EBIT = -50000
		item(financial.CodeRevProduct, "2025", 200000),
		item(financial.CodeCogsMaterial, "2025", 80000),
		item(financial.CodeOpexPayroll, "2025", 60000), // EBIT = 60000
	)
	result := Calculate(ds, Options{PeriodMeta: fyMeta(2024, 2025)})
	cagr := result.Trend.EarningsCAGR
	if cagr.Value.Available {
		t.Error("expected EarningsCAGR to be unavailable when starting net income is negative")
	}
	if cagr.Invalid == "" {
		t.Error("expected a non-empty Invalid reason for a negative starting value")
	}
}

func TestCAGR_UnavailableWhenEndpointDataMissing(t *testing.T) {
	ds := dataset(
		item(financial.CodeOpexPayroll, "2024", 1000), // no revenue
		item(financial.CodeRevProduct, "2025", 50000),
	)
	result := Calculate(ds, Options{PeriodMeta: fyMeta(2024, 2025)})
	cagr := result.Trend.RevenueCAGR
	if cagr.Value.Available {
		t.Error("expected CAGR to be unavailable when an endpoint is unavailable")
	}
	if cagr.Invalid != "" {
		t.Errorf("expected no Invalid reason for ordinary missing-data unavailability, got %q", cagr.Invalid)
	}
}

func TestVolatility_ComputesSampleStdDevOfGrowthRates(t *testing.T) {
	ds := dataset(
		item(financial.CodeRevProduct, "2022", 100000),
		item(financial.CodeRevProduct, "2023", 120000),
		item(financial.CodeRevProduct, "2024", 108000),
		item(financial.CodeRevProduct, "2025", 135000),
	)
	result := Calculate(ds, Options{PeriodMeta: fyMeta(2022, 2023, 2024, 2025)})
	vol := result.Trend.RevenueVolatility
	if !vol.Value.Available {
		t.Fatal("expected RevenueVolatility to be available")
	}
	want := 0.18929694486000914
	if math.Abs(vol.Value.Value-want) > 1e-9 {
		t.Errorf("Volatility = %v, want %v", vol.Value.Value, want)
	}
	if vol.SampleSize != 3 {
		t.Errorf("SampleSize = %v, want 3", vol.SampleSize)
	}
}

func TestVolatility_UnavailableWithFewerThanTwoObservations(t *testing.T) {
	ds := dataset(
		item(financial.CodeRevProduct, "2024", 100000),
		item(financial.CodeRevProduct, "2025", 120000),
	)
	result := Calculate(ds, Options{PeriodMeta: fyMeta(2024, 2025)})
	vol := result.Trend.RevenueVolatility
	if vol.Value.Available {
		t.Error("expected Volatility to be unavailable with only 1 growth observation")
	}
	if vol.SampleSize != 1 {
		t.Errorf("SampleSize = %v, want 1", vol.SampleSize)
	}
}

func TestVolatility_ZeroForPerfectlySteadyGrowth(t *testing.T) {
	ds := dataset(
		item(financial.CodeRevProduct, "2023", 100000),
		item(financial.CodeRevProduct, "2024", 110000),
		item(financial.CodeRevProduct, "2025", 121000),
	)
	result := Calculate(ds, Options{PeriodMeta: fyMeta(2023, 2024, 2025)})
	vol := result.Trend.RevenueVolatility
	if !vol.Value.Available {
		t.Fatal("expected Volatility to be available")
	}
	if math.Abs(vol.Value.Value) > 1e-9 {
		t.Errorf("Volatility = %v, want ~0 for perfectly steady 10%% growth", vol.Value.Value)
	}
}

func TestGrossMarginTrend_ListsMarginPerYear(t *testing.T) {
	ds := dataset(
		item(financial.CodeRevProduct, "2024", 100000),
		item(financial.CodeCogsMaterial, "2024", 40000),
		item(financial.CodeRevProduct, "2025", 200000),
		item(financial.CodeCogsMaterial, "2025", 60000),
	)
	result := Calculate(ds, Options{PeriodMeta: fyMeta(2024, 2025)})
	trend := result.Trend.GrossMarginTrend
	if len(trend) != 2 {
		t.Fatalf("expected 2 margin points, got %d", len(trend))
	}
	if !trend[0].Margin.Available || trend[0].Margin.Value != 0.6 {
		t.Errorf("2024 margin = %+v, want 0.6", trend[0].Margin)
	}
	if !trend[1].Margin.Available || trend[1].Margin.Value != 0.7 {
		t.Errorf("2025 margin = %+v, want 0.7", trend[1].Margin)
	}
}

func TestTrend_RestrictsToFiscalYearPeriodsOnly(t *testing.T) {
	// A quarter mixed in with fiscal years should be excluded from
	// fiscal-year-to-fiscal-year comparisons per the MVP restriction.
	ds := dataset(
		item(financial.CodeRevProduct, "2024", 100000),
		item(financial.CodeRevProduct, "2025-Q1", 30000),
		item(financial.CodeRevProduct, "2025", 130000),
	)
	meta := map[financial.Period]PeriodInfo{
		"2024":    {Type: PeriodTypeFiscalYear, FiscalYear: 2024},
		"2025-Q1": {Type: PeriodTypeQuarter, FiscalYear: 2025, SequenceInYear: 1},
		"2025":    {Type: PeriodTypeFiscalYear, FiscalYear: 2025},
	}
	result := Calculate(ds, Options{PeriodMeta: meta})
	if result.Trend.Error != nil {
		t.Fatalf("unexpected error: %v", result.Trend.Error)
	}
	if len(result.Trend.FiscalYearsUsed) != 2 {
		t.Fatalf("FiscalYearsUsed = %v, want exactly the 2 fiscal-year periods (quarter excluded)", result.Trend.FiscalYearsUsed)
	}
	for _, y := range result.Trend.FiscalYearsUsed {
		if y == "2025-Q1" {
			t.Error("quarter period must not appear in FiscalYearsUsed")
		}
	}
}

func TestTrend_ErrorWhenFewerThanTwoFiscalYearsAvailable(t *testing.T) {
	ds := dataset(
		item(financial.CodeRevProduct, "2025", 100000),
		item(financial.CodeRevProduct, "2025-Q1", 30000),
	)
	meta := map[financial.Period]PeriodInfo{
		"2025":    {Type: PeriodTypeFiscalYear, FiscalYear: 2025},
		"2025-Q1": {Type: PeriodTypeQuarter, FiscalYear: 2025, SequenceInYear: 1},
	}
	result := Calculate(ds, Options{PeriodMeta: meta})
	if result.Trend.Error == nil {
		t.Fatal("expected Trend.Error when fewer than 2 fiscal-year periods are available")
	}
}
