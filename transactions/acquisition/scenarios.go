package acquisition

import "github.com/themurtez/go-valuate/analytics/debt"

// computeScenarios evaluates every Input.Scenarios entry against
// schedules/totalDebtFinancing held fixed, stressing only
// Revenue/NormalizedEBITDA/NormalizedSDE per ScenarioAdjustment's haircut
// percentages. AskingPrice, Financing terms, Fees, and Capex/
// BuyerCompensation assumptions are held fixed — a scenario stresses the
// target's financial performance, not the deal structure.
func computeScenarios(in Input, schedules []debt.AmortizationSchedule, totalDebtFinancing, baseCashContribution Value) []ScenarioResult {
	if len(in.Scenarios) == 0 {
		return nil
	}

	annualDebtService := sumAnnualDebtService(schedules)

	results := make([]ScenarioResult, 0, len(in.Scenarios))
	for _, sc := range in.Scenarios {
		stressedRevenue := applyHaircut(in.Target.Revenue, sc.RevenueHaircutPercent)
		stressedEBITDA := applyHaircut(in.Target.NormalizedEBITDA, sc.EBITDAHaircutPercent)
		stressedSDE := applyHaircut(in.Target.NormalizedSDE, sc.SDEHaircutPercent)

		stressedTarget := TargetFinancials{
			Revenue:          stressedRevenue,
			NormalizedEBITDA: stressedEBITDA,
			NormalizedSDE:    stressedSDE,
		}

		multiples := computeMultiples(in.AskingPrice, stressedTarget)
		coverage := computeCoverage(stressedEBITDA, stressedSDE, annualDebtService, totalDebtFinancing, in.Capex, in.BuyerCompensation)
		returns := computeReturns(baseCashContribution, coverage.PostDebtCashFlow)

		breach := in.RedFlags.MinimumDSCR > 0 && coverage.DSCR.Available && coverage.DSCR.Amount < in.RedFlags.MinimumDSCR
		negative := coverage.PostDebtCashFlow.Available && coverage.PostDebtCashFlow.Amount < 0

		results = append(results, ScenarioResult{
			Scenario:            sc,
			Multiples:           multiples,
			Coverage:            coverage,
			Returns:             returns,
			BreachesMinimumDSCR: breach,
			HasNegativeCashFlow: negative,
		})
	}
	return results
}

// applyHaircut returns v reduced by haircutPercent (v.Amount x (1 -
// haircutPercent)) when v is available, otherwise Unavailable()
// unchanged. A haircutPercent of 0.15 reduces v by 15%; negative values
// increase v (modeling an upside case) — mirroring
// debt.DownsideScenario's identical convention.
func applyHaircut(v Value, haircutPercent float64) Value {
	if !v.Available {
		return Unavailable()
	}
	return AvailableValue(v.Amount * (1 - haircutPercent))
}
