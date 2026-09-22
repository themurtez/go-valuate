package cashflow

import (
	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/metrics"
)

// calculateCashRunway computes CashRunway from in.CashBalance and
// history's OperatingCashFlow series, using orderedPeriods for
// chronological order. Available only when the average monthly burn rate
// is itself available and negative — see CashRunway.Available's doc
// comment.
func calculateCashRunway(in Input, orderedPeriods []financial.Period, history []Bridge) CashRunway {
	burn := monthlyBurnRate(history)

	var runway CashRunway
	runway.MonthlyBurnRate = burn

	if current, ok := latestAvailableCashBalance(in.CashBalance, orderedPeriods); ok {
		runway.CurrentCashBalance = metrics.AvailableValue(current)
	}

	if !burn.Available || burn.Value >= 0 {
		return runway
	}
	runway.Available = true

	if runway.CurrentCashBalance.Available {
		runway.MonthsOfRunway = metrics.AvailableValue(runway.CurrentCashBalance.Value / -burn.Value)
	}

	return runway
}

// monthlyBurnRate averages history's available OperatingCashFlow
// observations. This package has no monthly/quarterly-to-monthly
// conversion input (Options carries no periods-per-year figure), so this
// is a simple mean over whatever periods are available in History — a
// caller with quarterly or annual data should divide/scale
// CurrentCashBalance and the resulting MonthsOfRunway externally if a
// true monthly figure is required; the field name documents the
// assumption rather than silently guessing a period length.
func monthlyBurnRate(history []Bridge) metrics.MetricValue {
	var sum float64
	var count int
	for _, b := range history {
		if b.OperatingCashFlow.Available {
			sum += b.OperatingCashFlow.Value
			count++
		}
	}
	if count == 0 {
		return metrics.Unavailable()
	}
	return metrics.AvailableValue(sum / float64(count))
}

// latestAvailableCashBalance returns the most recent (per orderedPeriods)
// Available entry in balances.
func latestAvailableCashBalance(balances map[financial.Period]CashFlowValue, orderedPeriods []financial.Period) (float64, bool) {
	for i := len(orderedPeriods) - 1; i >= 0; i-- {
		if v, ok := balances[orderedPeriods[i]]; ok && v.Available {
			return v.Value, true
		}
	}
	return 0, false
}
