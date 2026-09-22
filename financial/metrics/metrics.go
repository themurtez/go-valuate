package metrics

import "github.com/themurtez/go-valuate/financial"

// Options controls optional behavior of Calculate. The zero Options is
// valid: it computes per-period snapshots for every period in the dataset
// and skips trend calculation.
type Options struct {
	// PeriodMeta supplies ordering/type metadata for the dataset's periods,
	// required for any trend calculation (growth, CAGR, volatility, margin
	// trends). If nil or if a period present in the dataset has no entry
	// here, Result.Trend will carry a TrendError explaining that trends
	// could not be computed, rather than guessing an order from the Period
	// string. See PeriodInfo.
	PeriodMeta map[financial.Period]PeriodInfo
}

// FormulaVersion identifies this package's fixed metric formula set (the
// exact formula table in the repository README's financial/metrics
// section — TotalRevenue, GrossProfit, EBITDA, SDE, WorkingCapital, etc.).
// Bump this whenever a formula, an availability rule, or a sign convention
// changes in a way that could make a historical Result not reproduce
// identically under the new code — see the repository README's
// versioning-strategy section. Echoed on every Result so a persisted
// historical calculation remains self-describing about exactly which
// formula set produced it.
const FormulaVersion = "1.0.0"

// Result is the output of Calculate: one Snapshot per period present in the
// dataset, plus an optional Trend computed across comparable periods.
type Result struct {
	// FormulaVersion identifies which version of this package's fixed
	// metric formula set produced this Result — see the FormulaVersion
	// constant's doc comment.
	FormulaVersion string `json:"formula_version"`
	// Snapshots holds one Snapshot per distinct financial.Period in the
	// dataset, in the same chronological order as Periods (when PeriodMeta
	// was supplied) or dataset lexical order otherwise.
	Snapshots []Snapshot `json:"snapshots"`
	// Trend holds historical/trend metrics computed across Snapshots, when
	// enough ordered, comparable periods were available. Nil if trends were
	// not computed at all (e.g. fewer than two periods); see Trend.Error for
	// the case where trends were attempted but could not be computed
	// reliably.
	Trend *Trend `json:"trend,omitempty"`
}

// Calculate computes derived financial metrics for every period present in
// dataset. It never mutates dataset.
//
// Each Snapshot field is independently a MetricValue: missing required
// inputs make that specific metric Unavailable rather than failing the
// whole calculation or silently substituting 0. See the package doc comment
// for why this distinction matters.
//
// Calculate performs no I/O and returns no error for ordinary missing-data
// cases — that is exactly what MetricValue.Available communicates. It can
// still be called with an empty dataset, in which case Result.Snapshots is
// empty and Result.Trend is nil.
func Calculate(dataset financial.FinancialDataset, opts Options) Result {
	idx := buildCodeIndex(dataset)
	periods := dataset.Periods()

	snapshots := make([]Snapshot, 0, len(periods))
	for _, period := range periods {
		snapshots = append(snapshots, calculateSnapshot(idx, period))
	}

	result := Result{FormulaVersion: FormulaVersion, Snapshots: snapshots}
	if len(snapshots) >= 2 {
		trend := calculateTrend(snapshots, opts.PeriodMeta)
		result.Trend = &trend
	}
	return result
}

// calculateSnapshot computes every metric for a single period.
func calculateSnapshot(idx codeIndex, period financial.Period) Snapshot {
	totalRevenue, productRev, serviceRev, recurringRev, otherRev := revenueBreakdown(idx, period)
	cogs := totalCOGS(idx, period)
	gp := grossProfit(totalRevenue, cogs, period)
	gm := grossMargin(gp, totalRevenue, period)
	opex := totalOpex(idx, period)
	ebitRes := ebit(gp, opex, period)
	depreciation := singleCodeMetric(idx, period, MetricDepreciation, "Depreciation (as reported)", financial.CodeDepreciation)
	amortization := singleCodeMetric(idx, period, MetricAmortization, "Amortization (as reported)", financial.CodeAmortization)
	ebitdaRes := ebitda(ebitRes, depreciation, amortization, period)
	ebitdaMarginRes := ebitdaMargin(ebitdaRes, totalRevenue, period)
	ownerComp := singleCodeMetric(idx, period, MetricOwnerCompensation, "Owner Compensation (as reported)", financial.CodeOpexOwnerComp)
	sdeRes := sde(ebitdaRes, ownerComp, period)
	netIncomeRes := netIncome(idx, ebitRes, period)

	cash := singleCodeMetric(idx, period, MetricCash, "Cash (as reported)", financial.CodeBsCash)
	ar := singleCodeMetric(idx, period, MetricAccountsReceivable, "Accounts Receivable (as reported)", financial.CodeBsAccountsReceivable)
	inventory := singleCodeMetric(idx, period, MetricInventory, "Inventory (as reported)", financial.CodeBsInventory)
	ca := currentAssets(idx, period)
	ap := singleCodeMetric(idx, period, MetricAccountsPayable, "Accounts Payable (as reported)", financial.CodeBsAccountsPayable)
	cl := currentLiabilities(idx, period)
	wc := workingCapital(ca, cl, period)
	shortTermDebt, longTermDebt, totalDebt, netDebt := debtMetrics(idx, period)
	tangible := tangibleAssetValue(idx, period)

	snapshot := Snapshot{
		Period: period,

		TotalRevenue:      totalRevenue.Value,
		ProductRevenue:    productRev.Value,
		ServiceRevenue:    serviceRev.Value,
		RecurringRevenue:  recurringRev.Value,
		OtherRevenue:      otherRev.Value,
		TotalCOGS:         cogs.Value,
		GrossProfit:       gp.Value,
		GrossMargin:       gm.Value,
		TotalOpex:         opex.Value,
		EBIT:              ebitRes.Value,
		Depreciation:      depreciation.Value,
		Amortization:      amortization.Value,
		EBITDA:            ebitdaRes.Value,
		EBITDAMargin:      ebitdaMarginRes.Value,
		OwnerCompensation: ownerComp.Value,
		SDE:               sdeRes.Value,
		NetIncome:         netIncomeRes.Value,

		Cash:               cash.Value,
		AccountsReceivable: ar.Value,
		Inventory:          inventory.Value,
		CurrentAssets:      ca.Value,
		AccountsPayable:    ap.Value,
		CurrentLiabilities: cl.Value,
		WorkingCapital:     wc.Value,
		ShortTermDebt:      shortTermDebt.Value,
		LongTermDebt:       longTermDebt.Value,
		TotalDebt:          totalDebt.Value,
		NetDebt:            netDebt.Value,
		TangibleAssetValue: tangible.Value,
	}

	results := []MetricResult{
		totalRevenue, productRev, serviceRev, recurringRev, otherRev,
		cogs, gp, gm, opex, ebitRes, depreciation, amortization, ebitdaRes,
		ebitdaMarginRes, ownerComp, sdeRes, netIncomeRes,
		cash, ar, inventory, ca, ap, cl, wc,
		shortTermDebt, longTermDebt, totalDebt, netDebt, tangible,
	}
	snapshot.Results = make(map[string]MetricResult, len(results))
	for _, r := range results {
		snapshot.Results[r.Metric] = r
	}

	return snapshot
}

// SnapshotFor returns the Snapshot for a given period from a Result, and
// whether it was found. Convenience lookup mirroring
// financial.FinancialDataset.ByCodeAndPeriod's style.
func (r Result) SnapshotFor(period financial.Period) (Snapshot, bool) {
	for _, s := range r.Snapshots {
		if s.Period == period {
			return s, true
		}
	}
	return Snapshot{}, false
}
