package ratios

import (
	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/metrics"
)

// Calculate derives a full Result from in under opts. It never mutates
// in.Dataset and performs no I/O.
func Calculate(in Input, opts Options) Result {
	thresholds := resolveThresholds(opts.Thresholds)
	result := Result{FormulaVersion: FormulaVersion, SignalRulesVersion: SignalRulesVersion, Thresholds: thresholds}

	periods := in.Dataset.Periods()
	if len(periods) == 0 {
		result.Errors = append(result.Errors, Issue{
			Code:     IssueNoPeriods,
			Severity: SeverityError,
			Message:  "dataset has no periods; ratio analysis requires at least one",
		})
		return result
	}
	result.Available = true

	orderedPeriods, orderIssue := chronologicalPeriods(periods, in.PeriodMeta)
	if orderIssue != nil {
		result.Warnings = append(result.Warnings, *orderIssue)
	}

	idx := buildIndex(in.Dataset)

	// financial/metrics.Calculate recomputes every Snapshot this package
	// needs. Options.PeriodMeta is intentionally left empty here — this
	// package does its own chronological ordering (chronologicalPeriods
	// above, using its own PeriodInfo type) rather than also feeding
	// PeriodMeta into metrics.Calculate, since metrics.Trend's fiscal-
	// year-only calculations are not used by this package at all (see
	// growth.go's doc comment on why this package computes its own,
	// unrestricted growth series instead of reading metrics.Trend).
	metricsResult := metrics.Calculate(in.Dataset, metrics.Options{})
	snapshotByPeriod := make(map[financial.Period]metrics.Snapshot, len(metricsResult.Snapshots))
	for _, s := range metricsResult.Snapshots {
		snapshotByPeriod[s.Period] = s
	}

	history := make([]PeriodRatios, 0, len(orderedPeriods))
	for _, p := range orderedPeriods {
		history = append(history, computePeriodRatios(idx, snapshotByPeriod[p], p))
	}
	result.History = history

	if len(in.PeriodMeta) > 0 && orderIssue == nil {
		result.Trends = calculateTrends(history)
		result.Comparisons = calculateComparisons(history)
		result.Growth = buildGrowth(orderedPeriods, history)
	}

	result.Signals = buildSignals(result.Comparisons, history, thresholds)

	return result
}

// computePeriodRatios computes every ratio for a single period from its
// already-computed metrics.Snapshot.
func computePeriodRatios(idx codeIndex, s metrics.Snapshot, period financial.Period) PeriodRatios {
	totalAssetsSum := sumCodes(idx, period, totalAssetCodes)
	var totalAssets metrics.MetricValue
	if totalAssetsSum.anyPresent {
		totalAssets = metrics.AvailableValue(totalAssetsSum.total)
	}

	totalEquitySum := sumCodes(idx, period, totalEquityCodes)
	var totalEquity metrics.MetricValue
	if totalEquitySum.anyPresent {
		totalEquity = metrics.AvailableValue(totalEquitySum.total)
	}

	gm := grossMargin(s, period)
	om := operatingMargin(s, period)
	em := ebitdaMargin(s, period)
	nm := netMargin(s, period)
	roa := returnOnAssets(s, totalAssets, period)
	roe := returnOnEquity(s, totalEquity, period)

	cr := currentRatio(s, period)
	qr := quickRatio(idx, s, period)
	cashR := cashRatio(s, period)

	dte := debtToEquity(s, totalEquity, period)
	dta := debtToAssets(s, totalAssets, period)
	dteb := debtToEBITDA(s, period)
	ndteb := netDebtToEBITDA(s, period)
	ic := interestCoverage(idx, s, period)

	at := assetTurnover(s, totalAssets, period)
	rt := receivablesTurnover(s, period)
	it := inventoryTurnover(s, period)
	dso := daysSalesOutstanding(s, period)
	dio := daysInventoryOutstanding(s, period)
	dpo := daysPayableOutstanding(s, period)
	ccc := cashConversionCycle(dso, dio, dpo, period)

	p := PeriodRatios{
		Period: period,

		GrossMargin:     gm,
		OperatingMargin: om,
		EBITDAMargin:    em,
		NetMargin:       nm,
		ReturnOnAssets:  roa,
		ReturnOnEquity:  roe,

		CurrentRatio: cr,
		QuickRatio:   qr,
		CashRatio:    cashR,

		DebtToEquity:     dte,
		DebtToAssets:     dta,
		DebtToEBITDA:     dteb,
		NetDebtToEBITDA:  ndteb,
		InterestCoverage: ic,

		AssetTurnover:            at,
		ReceivablesTurnover:      rt,
		InventoryTurnover:        it,
		DaysSalesOutstanding:     dso,
		DaysInventoryOutstanding: dio,
		DaysPayableOutstanding:   dpo,
		CashConversionCycle:      ccc,

		Snapshot:    s,
		TotalAssets: totalAssets,
		TotalEquity: totalEquity,
	}

	results := []Ratio{
		gm, om, em, nm, roa, roe,
		cr, qr, cashR,
		dte, dta, dteb, ndteb, ic,
		at, rt, it, dso, dio, dpo, ccc,
	}
	p.Results = make(map[string]Ratio, len(results))
	for _, r := range results {
		p.Results[r.Metric] = r
	}

	return p
}
