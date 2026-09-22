package anomalies

import (
	"fmt"

	"github.com/themurtez/go-valuate/financial"
)

// detectExpenseOutpacingRevenue implements RuleExpenseOutpacingRevenue: for
// every COGS/OPEX account and every chronologically adjacent period pair
// where Total Revenue is available and positive in both periods and the
// account has a reported amount in both periods, this compares the
// account's period-over-period growth rate against revenue's growth rate.
func detectExpenseOutpacingRevenue(idx codeIndex, ordered []orderedPeriod, thresholds Thresholds) []Anomaly {
	var out []Anomaly
	for i := 1; i < len(ordered); i++ {
		fromPeriod, toPeriod := ordered[i-1].period, ordered[i].period

		fromRevenue, fromRevOK := totalRevenue(idx, fromPeriod)
		toRevenue, toRevOK := totalRevenue(idx, toPeriod)
		if !fromRevOK || !toRevOK || fromRevenue <= 0 || toRevenue <= 0 {
			continue
		}
		revenueGrowth := (toRevenue - fromRevenue) / fromRevenue

		for _, code := range expenseCodes {
			from, fromOK := idx.lookup(code, fromPeriod)
			to, toOK := idx.lookup(code, toPeriod)
			if !fromOK || !toOK || from.Amount == 0 {
				continue
			}
			expenseGrowth := (to.Amount - from.Amount) / from.Amount
			if expenseGrowth <= 0 {
				continue
			}
			gap := expenseGrowth - revenueGrowth
			if gap < thresholds.ExpenseOutpacingRevenueGap {
				continue
			}
			out = append(out, Anomaly{
				Code:           RuleExpenseOutpacingRevenue,
				Severity:       AnomalySeverityWarning,
				Account:        code,
				Period:         toPeriod,
				BaselinePeriod: fromPeriod,
				Baseline:       AvailableValue(from.Amount),
				Observed:       AvailableValue(to.Amount),
				Delta:          expenseGrowth - revenueGrowth,
				Threshold:      thresholds.ExpenseOutpacingRevenueGap,
				Explanation: fmt.Sprintf(
					"%s grew %.1f%% from %s to %s while Total Revenue grew %.1f%%, a %.1f-point gap exceeding the %.1f-point threshold; unusual pattern, review recommended",
					code, expenseGrowth*100, fromPeriod, toPeriod, revenueGrowth*100, gap*100, thresholds.ExpenseOutpacingRevenueGap*100,
				),
				Provenance: provenanceFor(to, from, true),
			})
		}
	}
	return out
}

// marginKind identifies which synthetic margin figure
// detectMarginDeterioration is evaluating, used only to select the right
// compute function and MetricLabel.
type marginKind struct {
	label   string
	compute func(codeIndex, financial.Period) (float64, bool)
}

var marginKinds = []marginKind{
	{label: "gross_margin", compute: grossMargin},
	{label: "operating_margin", compute: operatingMargin},
}

// detectMarginDeterioration implements RuleMarginDeterioration: for every
// chronologically adjacent period pair, gross margin and operating margin
// are each independently checked for a period-over-period decline of at
// least Thresholds.MarginDeteriorationPoints raw percentage points.
func detectMarginDeterioration(idx codeIndex, ordered []orderedPeriod, thresholds Thresholds) []Anomaly {
	var out []Anomaly
	for i := 1; i < len(ordered); i++ {
		fromPeriod, toPeriod := ordered[i-1].period, ordered[i].period

		for _, mk := range marginKinds {
			from, fromOK := mk.compute(idx, fromPeriod)
			to, toOK := mk.compute(idx, toPeriod)
			if !fromOK || !toOK {
				continue
			}
			decline := from - to
			if decline < thresholds.MarginDeteriorationPoints {
				continue
			}
			out = append(out, Anomaly{
				Code:           RuleMarginDeterioration,
				Severity:       AnomalySeverityWarning,
				MetricLabel:    mk.label,
				Period:         toPeriod,
				BaselinePeriod: fromPeriod,
				Baseline:       AvailableValue(from),
				Observed:       AvailableValue(to),
				Delta:          to - from,
				Threshold:      thresholds.MarginDeteriorationPoints,
				Explanation: fmt.Sprintf(
					"%s declined %.1f percentage points from %s (%.1f%%) to %s (%.1f%%), exceeding the %.1f-point threshold; unusual pattern, review recommended",
					mk.label, decline*100, fromPeriod, from*100, toPeriod, to*100, thresholds.MarginDeteriorationPoints*100,
				),
			})
		}
	}
	return out
}
