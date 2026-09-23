package debt

// computeScenarios evaluates every Input.DownsideScenario against schedules/
// totalDebtBalance/interestExpense held fixed, stressing only EBITDA/cash
// flow per DownsideScenario's haircut percentages.
func computeScenarios(in Input, schedules []AmortizationSchedule, totalDebtBalance, interestExpense Value) []ScenarioResult {
	if len(in.DownsideScenarios) == 0 {
		return nil
	}

	results := make([]ScenarioResult, 0, len(in.DownsideScenarios))
	for _, sc := range in.DownsideScenarios {
		stressedEBITDA := applyHaircut(in.EBITDA, sc.EBITDAHaircutPercent)
		stressedCashFlow := applyHaircut(in.CashFlow, sc.CashFlowHaircutPercent)

		coverage := computeCoverage(stressedEBITDA, stressedCashFlow, schedules, totalDebtBalance, interestExpense, in.CashAndEquivalents, in.FixedCharges)

		breach := in.Policy.MinimumDSCR > 0 && coverage.DSCR.Available && coverage.DSCR.Amount < in.Policy.MinimumDSCR

		results = append(results, ScenarioResult{
			Scenario:            sc,
			Coverage:            coverage,
			BreachesMinimumDSCR: breach,
		})
	}
	return results
}

// applyHaircut returns v reduced by haircutPercent (v.Amount x (1 -
// haircutPercent)) when v is available, otherwise Unavailable() unchanged.
// A haircutPercent of 0.15 reduces v by 15%; negative values increase v
// (modeling an upside case) — see DownsideScenario's doc comment.
func applyHaircut(v Value, haircutPercent float64) Value {
	if !v.Available {
		return Unavailable()
	}
	return AvailableValue(v.Amount * (1 - haircutPercent))
}
