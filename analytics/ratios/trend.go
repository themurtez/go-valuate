package ratios

import (
	"fmt"
	"math"
	"sort"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/metrics"
)

// chronologicalPeriods sorts periods by meta's FiscalYear/Type/
// SequenceInYear ordering, mirroring
// workingcapital.chronologicalPeriods/analytics/qoe's identical helper and
// financial/metrics.sortOrderedPeriods. If meta is empty or any period is
// missing an entry, it returns periods in their original (lexical) order
// plus a descriptive warning Issue — this package never guesses
// chronological order from financial.Period's string value, but a missing
// order does not prevent per-period History from being computed (only
// Trend/Comparisons/Growth depend on true chronological order).
func chronologicalPeriods(periods []financial.Period, meta map[financial.Period]PeriodInfo) ([]financial.Period, *Issue) {
	if len(meta) == 0 {
		return periods, &Issue{
			Code:     IssueNoPeriodMeta,
			Severity: SeverityWarning,
			Message:  "no PeriodMeta supplied; History uses dataset lexical order and Trends/Comparisons/Growth are unavailable",
		}
	}

	type entry struct {
		period financial.Period
		info   PeriodInfo
	}
	entries := make([]entry, 0, len(periods))
	var missing []financial.Period
	for _, p := range periods {
		info, ok := meta[p]
		if !ok {
			missing = append(missing, p)
			continue
		}
		entries = append(entries, entry{period: p, info: info})
	}
	if len(missing) > 0 {
		return periods, &Issue{
			Code:     IssuePeriodMissingFromMeta,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("one or more periods have no PeriodMeta entry: %v; History uses dataset lexical order and Trends/Comparisons/Growth are unavailable", missing),
		}
	}

	rank := func(t PeriodType) int {
		switch t {
		case PeriodTypeFiscalYear, PeriodTypeYTD:
			return 0
		case PeriodTypeQuarter:
			return 1
		case PeriodTypeMonth:
			return 2
		default:
			return 3
		}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i].info, entries[j].info
		if a.FiscalYear != b.FiscalYear {
			return a.FiscalYear < b.FiscalYear
		}
		if rank(a.Type) != rank(b.Type) {
			return rank(a.Type) < rank(b.Type)
		}
		return a.SequenceInYear < b.SequenceInYear
	})

	ordered := make([]financial.Period, len(entries))
	for i, e := range entries {
		ordered[i] = e.period
	}
	return ordered, nil
}

// ratioSeriesGetter extracts one metric's series from a chronologically
// ordered History.
type ratioSeriesGetter func(PeriodRatios) Ratio

// ratioGetters lists every ratio metric this package computes Trends/
// Comparisons for, in RatioXxx declaration order — the single source of
// truth both calculateTrends and calculateComparisons iterate, so the two
// outputs can never drift out of sync about which metrics they cover.
var ratioGetters = []struct {
	metric string
	get    ratioSeriesGetter
}{
	{RatioGrossMargin, func(p PeriodRatios) Ratio { return p.GrossMargin }},
	{RatioOperatingMargin, func(p PeriodRatios) Ratio { return p.OperatingMargin }},
	{RatioEBITDAMargin, func(p PeriodRatios) Ratio { return p.EBITDAMargin }},
	{RatioNetMargin, func(p PeriodRatios) Ratio { return p.NetMargin }},
	{RatioReturnOnAssets, func(p PeriodRatios) Ratio { return p.ReturnOnAssets }},
	{RatioReturnOnEquity, func(p PeriodRatios) Ratio { return p.ReturnOnEquity }},
	{RatioCurrentRatio, func(p PeriodRatios) Ratio { return p.CurrentRatio }},
	{RatioQuickRatio, func(p PeriodRatios) Ratio { return p.QuickRatio }},
	{RatioCashRatio, func(p PeriodRatios) Ratio { return p.CashRatio }},
	{RatioDebtToEquity, func(p PeriodRatios) Ratio { return p.DebtToEquity }},
	{RatioDebtToAssets, func(p PeriodRatios) Ratio { return p.DebtToAssets }},
	{RatioDebtToEBITDA, func(p PeriodRatios) Ratio { return p.DebtToEBITDA }},
	{RatioNetDebtToEBITDA, func(p PeriodRatios) Ratio { return p.NetDebtToEBITDA }},
	{RatioInterestCoverage, func(p PeriodRatios) Ratio { return p.InterestCoverage }},
	{RatioAssetTurnover, func(p PeriodRatios) Ratio { return p.AssetTurnover }},
	{RatioReceivablesTurnover, func(p PeriodRatios) Ratio { return p.ReceivablesTurnover }},
	{RatioInventoryTurnover, func(p PeriodRatios) Ratio { return p.InventoryTurnover }},
	{RatioDaysSalesOutstanding, func(p PeriodRatios) Ratio { return p.DaysSalesOutstanding }},
	{RatioDaysInventoryOutstanding, func(p PeriodRatios) Ratio { return p.DaysInventoryOutstanding }},
	{RatioDaysPayableOutstanding, func(p PeriodRatios) Ratio { return p.DaysPayableOutstanding }},
	{RatioCashConversionCycle, func(p PeriodRatios) Ratio { return p.CashConversionCycle }},
}

// calculateTrends computes one RatioTrend per metric in ratioGetters that
// has at least one available observation in orderedHistory (which must
// already be chronologically ordered).
func calculateTrends(orderedHistory []PeriodRatios) []RatioTrend {
	var trends []RatioTrend
	for _, g := range ratioGetters {
		trend, ok := calculateRatioTrend(g.metric, orderedHistory, g.get)
		if ok {
			trends = append(trends, trend)
		}
	}
	return trends
}

// calculateRatioTrend finds the first and last available observation of
// one metric across orderedHistory and characterizes the direction between
// them, mirroring workingcapital.calculateTrend's identical
// first-vs-last/TrendFlatBandPercent method. ok is false if no observation
// in orderedHistory was available for this metric at all (in which case no
// RatioTrend entry is produced — a metric wholly absent from a dataset,
// e.g. balance-sheet ratios for an income-statement-only dataset, need not
// clutter Result.Trends with an all-Unavailable placeholder).
func calculateRatioTrend(metric string, orderedHistory []PeriodRatios, get ratioSeriesGetter) (RatioTrend, bool) {
	var first, last Ratio
	found := false
	for i := range orderedHistory {
		r := get(orderedHistory[i])
		if !r.Value.Available {
			continue
		}
		if !found {
			first = r
			found = true
		}
		last = r
	}
	if !found {
		return RatioTrend{}, false
	}

	t := RatioTrend{
		Metric:      metric,
		FirstPeriod: first.Period,
		LastPeriod:  last.Period,
		FirstValue:  first.Value,
		LastValue:   last.Value,
	}

	if first.Period == last.Period {
		t.Direction = TrendStable
		return t, true
	}

	if first.Value.Value != 0 {
		t.PercentChange = metrics.AvailableValue((last.Value.Value - first.Value.Value) / math.Abs(first.Value.Value))
	}

	change := last.Value.Value - first.Value.Value
	band := math.Abs(first.Value.Value) * TrendFlatBandPercent
	switch {
	case math.Abs(change) <= band:
		t.Direction = TrendStable
	case change > 0:
		t.Direction = TrendIncreasing
	default:
		t.Direction = TrendDeclining
	}
	return t, true
}

// calculateComparisons computes every adjacent-period Comparison for every
// metric in ratioGetters that has at least one available observation in
// orderedHistory, ordered by metric (ratioGetters' declaration order), then
// chronologically within a metric.
func calculateComparisons(orderedHistory []PeriodRatios) []Comparison {
	var comparisons []Comparison
	for _, g := range ratioGetters {
		for i := 1; i < len(orderedHistory); i++ {
			from := g.get(orderedHistory[i-1])
			to := g.get(orderedHistory[i])
			if !from.Value.Available && !to.Value.Available {
				continue
			}
			c := Comparison{
				Metric:     g.metric,
				FromPeriod: from.Period,
				ToPeriod:   to.Period,
				FromValue:  from.Value,
				ToValue:    to.Value,
			}
			if from.Value.Available && to.Value.Available {
				c.Change = metrics.AvailableValue(to.Value.Value - from.Value.Value)
			}
			comparisons = append(comparisons, c)
		}
	}
	return comparisons
}
