package reconciliation

import (
	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/metrics"
)

// Run evaluates every reconciliation check this package implements against
// dataset and returns the full set of results. Run never mutates dataset.
//
// Run internally uses financial/metrics to reconstruct income-statement and
// balance-sheet subtotals from dataset, so callers get identical figures
// (and identical missing-vs-zero semantics) whether they call
// reconciliation.Run or metrics.Calculate directly. See ReportedTotals for
// how a caller supplies the "reported" side of a reconciliation comparison,
// since dataset itself cannot carry reported subtotals (see this package's
// doc comment).
//
// Checks run in a fixed order: dataset-level integrity checks first
// (regardless of Options), then for each period in dataset.Periods()
// order: balance sheet checks, then income statement checks.
func Run(dataset financial.FinancialDataset, opts Options) Result {
	tol := opts.tolerance()

	var checks []Check
	checks = append(checks, runIntegrityChecks(dataset)...)

	metricsResult := metrics.Calculate(dataset, metrics.Options{})

	for _, period := range dataset.Periods() {
		checks = append(checks, checkBalanceSheetBalances(dataset, period, tol))

		snapshot, ok := metricsResult.SnapshotFor(period)
		if !ok {
			// dataset.Periods() and metricsResult's snapshots are always
			// built from the same dataset, so this cannot happen in
			// practice; skip defensively rather than panic.
			continue
		}
		checks = append(checks, checkCurrentAssetsSubtotal(snapshot))
		checks = append(checks, checkCurrentLiabilitiesSubtotal(snapshot))
		checks = append(checks, checkWorkingCapitalCalculated(snapshot))
		checks = append(checks, checkDebtTotalsCalculated(snapshot))

		reported := opts.reportedFor(period)
		checks = append(checks, checkGrossProfit(snapshot, reported, tol))
		checks = append(checks, checkOperatingIncome(snapshot, reported, tol))
		checks = append(checks, checkEBITDABridge(snapshot, reported, tol))
		checks = append(checks, checkNetIncome(snapshot, reported, tol))
	}

	return Result{Checks: checks}
}
