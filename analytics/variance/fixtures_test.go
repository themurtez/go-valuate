package variance

import "github.com/themurtez/go-valuate/financial"

// threeYearMeta builds a PeriodInfo map for three consecutive fiscal years
// "2023", "2024", "2025", mirroring
// analytics/concentration/fixtures_test.go's threeYearMeta.
func threeYearMeta() map[financial.Period]PeriodInfo {
	return map[financial.Period]PeriodInfo{
		"2023": {Type: PeriodTypeFiscalYear, FiscalYear: 2023},
		"2024": {Type: PeriodTypeFiscalYear, FiscalYear: 2024},
		"2025": {Type: PeriodTypeFiscalYear, FiscalYear: 2025},
	}
}

// line is a terse constructor for a valid LineObservation with an available
// baseline, for test readability.
func line(code financial.Code, period string, actual, baseline float64, baselineType BaselineType) LineObservation {
	return LineObservation{
		AccountCode:       code,
		Period:            financial.Period(period),
		Actual:            actual,
		BaselineAvailable: true,
		Baseline:          baseline,
		BaselineType:      baselineType,
	}
}

// lineNoBaseline is a terse constructor for a LineObservation with no
// baseline available.
func lineNoBaseline(code financial.Code, period string, actual float64) LineObservation {
	return LineObservation{
		AccountCode: code,
		Period:      financial.Period(period),
		Actual:      actual,
	}
}

// lineCat is line plus a caller-supplied Category.
func lineCat(code financial.Code, period string, actual, baseline float64, baselineType BaselineType, category string) LineObservation {
	l := line(code, period, actual, baseline, baselineType)
	l.Category = category
	return l
}

// budgetVsActualLines models a small SMB's annual budget-vs-actual: revenue
// beat budget, COGS came in over budget, and OPEX came in under budget,
// across three fiscal years.
func budgetVsActualLines() []LineObservation {
	return []LineObservation{
		line(financial.CodeRevProduct, "2023", 550_000, 500_000, BaselineTypeBudget),
		line(financial.CodeCogsMaterial, "2023", 220_000, 200_000, BaselineTypeBudget),
		line(financial.CodeOpexPayroll, "2023", 140_000, 150_000, BaselineTypeBudget),
		line(financial.CodeOpexMarketing, "2023", 30_000, 25_000, BaselineTypeBudget),

		line(financial.CodeRevProduct, "2024", 610_000, 560_000, BaselineTypeBudget),
		line(financial.CodeCogsMaterial, "2024", 235_000, 230_000, BaselineTypeBudget),
		line(financial.CodeOpexPayroll, "2024", 155_000, 165_000, BaselineTypeBudget),
		line(financial.CodeOpexMarketing, "2024", 28_000, 30_000, BaselineTypeBudget),

		line(financial.CodeRevProduct, "2025", 590_000, 630_000, BaselineTypeBudget),
		line(financial.CodeCogsMaterial, "2025", 260_000, 240_000, BaselineTypeBudget),
		line(financial.CodeOpexPayroll, "2025", 170_000, 170_000, BaselineTypeBudget),
		line(financial.CodeOpexMarketing, "2025", 35_000, 30_000, BaselineTypeBudget),
	}
}
