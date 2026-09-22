package concentration

import "github.com/themurtez/go-valuate/financial"

// threeYearMeta builds a PeriodInfo map for three consecutive fiscal years
// "2023", "2024", "2025", mirroring
// analytics/revenuequality/fixtures_test.go's threeYearMeta.
func threeYearMeta() map[financial.Period]PeriodInfo {
	return map[financial.Period]PeriodInfo{
		"2023": {Type: PeriodTypeFiscalYear, FiscalYear: 2023},
		"2024": {Type: PeriodTypeFiscalYear, FiscalYear: 2024},
		"2025": {Type: PeriodTypeFiscalYear, FiscalYear: 2025},
	}
}

// obs is a terse constructor for a valid Observation, for test readability.
func obs(entityKey, period string, amount float64) Observation {
	return Observation{EntityKey: entityKey, Period: financial.Period(period), Amount: amount}
}

// obsCat is obs plus a Category.
func obsCat(entityKey, period string, amount float64, category string) Observation {
	return Observation{EntityKey: entityKey, Period: financial.Period(period), Amount: amount, Category: category}
}

// diffAbs returns the absolute difference between a and b, for
// tolerance-based float64 comparisons in tests (float64 summation order can
// produce last-bit differences from a hand-computed expected value).
func diffAbs(a, b float64) float64 {
	d := a - b
	if d < 0 {
		return -d
	}
	return d
}

// highlyConcentratedObservations models a business earning nearly all of
// its revenue from a single customer across three years, with the
// concentration worsening over time (from 70% to 85%).
func highlyConcentratedObservations() []Observation {
	return []Observation{
		obs("whale", "2023", 700_000),
		obs("small-1", "2023", 150_000),
		obs("small-2", "2023", 150_000),

		obs("whale", "2024", 800_000),
		obs("small-1", "2024", 120_000),
		obs("small-2", "2024", 80_000),

		obs("whale", "2025", 850_000),
		obs("small-1", "2025", 100_000),
		obs("small-2", "2025", 50_000),
	}
}

// diversifiedObservations models a business with ten roughly equal-sized
// customers in a single period — low concentration, low HHI.
func diversifiedObservations() []Observation {
	var out []Observation
	for i := 1; i <= 10; i++ {
		out = append(out, obs(entityKeyOf(i), "2025", 100_000))
	}
	return out
}

func entityKeyOf(i int) string {
	names := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}
	return "cust-" + names[i-1]
}
