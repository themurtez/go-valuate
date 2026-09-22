package cashflow

import "github.com/themurtez/go-valuate/financial/metrics"

// buildRecurringDrains aggregates capex, working-capital build, debt
// service, and owner distributions across history into four
// RecurringDrain entries, in RecurringDrainCategory's fixed declaration
// order (never Go map order). A category with zero periods present is
// still included (TotalAmount 0, PeriodsPresent 0) so a caller iterating
// Result.RecurringDrains always sees all four categories rather than
// having to know which ones happened to have data.
func buildRecurringDrains(history []Bridge) []RecurringDrain {
	drains := []RecurringDrain{
		{Category: RecurringDrainCapex},
		{Category: RecurringDrainWorkingCapital},
		{Category: RecurringDrainDebtService},
		{Category: RecurringDrainOwnerDistributions},
	}

	var capexEBITDASum, wcEBITDASum, dsEBITDASum, distEBITDASum float64
	var capexEBITDAOK, wcEBITDAOK, dsEBITDAOK, distEBITDAOK bool

	for _, b := range history {
		ebitdaAvailable := b.EBITDA.Available

		if b.Capex.Available {
			drains[0].TotalAmount += b.Capex.Value
			if b.Capex.Value != 0 {
				drains[0].PeriodsPresent++
			}
			if ebitdaAvailable {
				capexEBITDASum += b.EBITDA.Value
				capexEBITDAOK = true
			}
		}

		if b.ChangeInNWC.Available {
			drains[1].TotalAmount += b.ChangeInNWC.Value
			if b.ChangeInNWC.Value != 0 {
				drains[1].PeriodsPresent++
			}
			if ebitdaAvailable {
				wcEBITDASum += b.EBITDA.Value
				wcEBITDAOK = true
			}
		}

		if total := b.DebtService.Total(); total.Available {
			drains[2].TotalAmount += total.Value
			if total.Value != 0 {
				drains[2].PeriodsPresent++
			}
			if ebitdaAvailable {
				dsEBITDASum += b.EBITDA.Value
				dsEBITDAOK = true
			}
		}

		if b.OwnerDistributions.Available {
			drains[3].TotalAmount += b.OwnerDistributions.Value
			if b.OwnerDistributions.Value != 0 {
				drains[3].PeriodsPresent++
			}
			if ebitdaAvailable {
				distEBITDASum += b.EBITDA.Value
				distEBITDAOK = true
			}
		}
	}

	drains[0].AverageOfEBITDA = ratioOfSum(drains[0].TotalAmount, capexEBITDASum, capexEBITDAOK)
	drains[1].AverageOfEBITDA = ratioOfSum(drains[1].TotalAmount, wcEBITDASum, wcEBITDAOK)
	drains[2].AverageOfEBITDA = ratioOfSum(drains[2].TotalAmount, dsEBITDASum, dsEBITDAOK)
	drains[3].AverageOfEBITDA = ratioOfSum(drains[3].TotalAmount, distEBITDASum, distEBITDAOK)

	return drains
}

// ratioOfSum returns total / ebitdaSum, available only if ok and
// ebitdaSum != 0.
func ratioOfSum(total, ebitdaSum float64, ok bool) metrics.MetricValue {
	if !ok || ebitdaSum == 0 {
		return metrics.Unavailable()
	}
	return metrics.AvailableValue(total / ebitdaSum)
}
