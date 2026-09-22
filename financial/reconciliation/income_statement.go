package reconciliation

import (
	"fmt"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/metrics"
)

// checkGrossProfit compares reconstructed gross profit (Revenue - COGS,
// from the metrics package) against a caller-supplied reported gross
// profit for period, if one was supplied via Options.Reported.
//
// Returns StatusNotApplicable, rather than inventing a reported value, if
// either the reconstructed figure is unavailable (missing revenue/COGS
// data) or no reported gross profit was supplied for this period.
func checkGrossProfit(snapshot metrics.Snapshot, reported ReportedPeriodTotals, tol Tolerance) Check {
	check := Check{
		Code:         CheckGrossProfit,
		Period:       snapshot.Period,
		Tolerance:    tol,
		RelatedCodes: []string{metrics.MetricTotalRevenue, metrics.MetricTotalCOGS, metrics.MetricGrossProfit},
	}

	if reported.GrossProfit == nil {
		check.Status = StatusNotApplicable
		check.Explanation = "no reported gross profit was supplied for this period; nothing to reconcile against"
		return check
	}
	if !snapshot.GrossProfit.Available {
		check.Status = StatusNotApplicable
		check.Explanation = "gross profit could not be reconstructed from the dataset (missing revenue or COGS data)"
		return check
	}

	expected := *reported.GrossProfit
	actual := snapshot.GrossProfit.Value
	status, diff := tol.Evaluate(expected, actual)
	check.Expected = &expected
	check.Actual = &actual
	check.Difference = &diff
	check.Status = status
	check.Explanation = explainComparison(status, "reported gross profit", expected, "reconstructed gross profit (Revenue - COGS)", actual, diff, tol)
	return check
}

// checkOperatingIncome compares reconstructed operating income (EBIT, i.e.
// Gross Profit - Operating Expenses) against a caller-supplied reported
// operating income for period.
func checkOperatingIncome(snapshot metrics.Snapshot, reported ReportedPeriodTotals, tol Tolerance) Check {
	check := Check{
		Code:         CheckOperatingIncome,
		Period:       snapshot.Period,
		Tolerance:    tol,
		RelatedCodes: []string{metrics.MetricGrossProfit, metrics.MetricTotalOpex, metrics.MetricEBIT},
	}

	if reported.OperatingIncome == nil {
		check.Status = StatusNotApplicable
		check.Explanation = "no reported operating income was supplied for this period; nothing to reconcile against"
		return check
	}
	if !snapshot.EBIT.Available {
		check.Status = StatusNotApplicable
		check.Explanation = "operating income could not be reconstructed from the dataset (missing gross profit or operating expense data)"
		return check
	}

	expected := *reported.OperatingIncome
	actual := snapshot.EBIT.Value
	status, diff := tol.Evaluate(expected, actual)
	check.Expected = &expected
	check.Actual = &actual
	check.Difference = &diff
	check.Status = status
	check.Explanation = explainComparison(status, "reported operating income", expected, "reconstructed operating income (Gross Profit - Operating Expenses)", actual, diff, tol)
	return check
}

// checkEBITDABridge compares reconstructed EBITDA (EBIT + Depreciation +
// Amortization) against a caller-supplied reported EBITDA for period, only
// if one was supplied — most source statements never report EBITDA
// directly, so this is expected to be StatusNotApplicable far more often
// than not, which is not itself a problem.
func checkEBITDABridge(snapshot metrics.Snapshot, reported ReportedPeriodTotals, tol Tolerance) Check {
	check := Check{
		Code:         CheckEBITDABridge,
		Period:       snapshot.Period,
		Tolerance:    tol,
		RelatedCodes: []string{metrics.MetricEBIT, string(financial.CodeDepreciation), string(financial.CodeAmortization), metrics.MetricEBITDA},
	}

	if reported.EBITDA == nil {
		check.Status = StatusNotApplicable
		check.Explanation = "no reported EBITDA was supplied for this period (most statements don't report EBITDA directly); nothing to reconcile against"
		return check
	}
	if !snapshot.EBITDA.Available {
		check.Status = StatusNotApplicable
		check.Explanation = "EBITDA could not be reconstructed from the dataset (missing EBIT, i.e. gross profit or operating expense data)"
		return check
	}

	expected := *reported.EBITDA
	actual := snapshot.EBITDA.Value
	status, diff := tol.Evaluate(expected, actual)
	check.Expected = &expected
	check.Actual = &actual
	check.Difference = &diff
	check.Status = status
	check.Explanation = explainComparison(status, "reported EBITDA", expected, "reconstructed EBITDA (EBIT + Depreciation + Amortization)", actual, diff, tol)
	return check
}

// checkNetIncome compares reconstructed net income against a caller-supplied
// reported net income for period. See metrics' netIncome doc comment for
// the exact reconstruction formula and its below-the-line limitations.
func checkNetIncome(snapshot metrics.Snapshot, reported ReportedPeriodTotals, tol Tolerance) Check {
	check := Check{
		Code:      CheckNetIncome,
		Period:    snapshot.Period,
		Tolerance: tol,
		RelatedCodes: []string{
			metrics.MetricEBIT,
			string(financial.CodeOtherIncome),
			string(financial.CodeInterestIncome),
			string(financial.CodeInterestExpense),
			string(financial.CodeOtherExpense),
			string(financial.CodeIncomeTax),
			metrics.MetricNetIncome,
		},
	}

	if reported.NetIncome == nil {
		check.Status = StatusNotApplicable
		check.Explanation = "no reported net income was supplied for this period; nothing to reconcile against"
		return check
	}
	if !snapshot.NetIncome.Available {
		check.Status = StatusNotApplicable
		check.Explanation = "net income could not be reconstructed from the dataset (missing operating income data)"
		return check
	}

	expected := *reported.NetIncome
	actual := snapshot.NetIncome.Value
	status, diff := tol.Evaluate(expected, actual)
	check.Expected = &expected
	check.Actual = &actual
	check.Difference = &diff
	check.Status = status
	check.Explanation = explainComparison(status, "reported net income", expected, "reconstructed net income (Operating Income + Other Income + Interest Income - Interest Expense - Other Expense - Taxes)", actual, diff, tol)
	return check
}

// explainComparison produces a consistent human-readable Explanation string
// for a reported-vs-reconstructed comparison, shared by every income
// statement check so wording stays uniform.
func explainComparison(status Status, expectedLabel string, expected float64, actualLabel string, actual float64, diff float64, tol Tolerance) string {
	switch status {
	case StatusPass:
		return fmt.Sprintf("%s (%.2f) matches %s (%.2f) within tolerance (difference %.2f, absolute tolerance %.2f)",
			actualLabel, actual, expectedLabel, expected, diff, tol.Absolute)
	default:
		return fmt.Sprintf("%s (%.2f) differs from %s (%.2f) by %.2f, outside tolerance (absolute %.2f)",
			actualLabel, actual, expectedLabel, expected, diff, tol.Absolute)
	}
}
