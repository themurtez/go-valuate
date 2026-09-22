package qoe

import (
	"math"
	"sort"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/adjustments"
	"github.com/themurtez/go-valuate/financial/metrics"
)

// Calculate derives a full Result from in under opts. It never mutates
// in.Dataset or in.Adjustments and performs no I/O.
//
// Calculate recomputes financial/metrics.Snapshot values itself (via
// metrics.Calculate over in.Dataset/in.PeriodMeta) rather than accepting a
// pre-built metrics.Result, so that History's per-period figures and the
// package-level RevenueGrowth/EBITDAMarginTrend/volatility outputs are
// always derived from the exact same metrics.Trend computation — never two
// independently-computed views of the same dataset that could silently
// drift apart. A caller that already ran metrics.Calculate elsewhere is not
// duplicating meaningful work by letting Calculate do it again:
// metrics.Calculate performs no I/O and is cheap, pure arithmetic over an
// in-memory dataset.
func Calculate(in Input, opts Options) Result {
	thresholds := resolveThresholds(opts.Thresholds)
	result := Result{FormulaVersion: FormulaVersion, Thresholds: thresholds}

	periods := in.Dataset.Periods()
	if len(periods) == 0 {
		result.Errors = append(result.Errors, Issue{
			Code:     IssueNoPeriods,
			Severity: SeverityError,
			Message:  "dataset has no periods; quality-of-earnings analysis requires at least one",
		})
		return result
	}
	result.Available = true

	if len(in.PeriodMeta) == 0 {
		result.Warnings = append(result.Warnings, Issue{
			Code:     IssueNoPeriodMeta,
			Severity: SeverityWarning,
			Message:  "no PeriodMeta supplied; revenue growth, margin trend, volatility, and every trend-dependent flag are unavailable",
		})
	}

	metricsResult := metrics.Calculate(in.Dataset, metrics.Options{PeriodMeta: in.PeriodMeta})

	orderedPeriods := periods
	if len(in.PeriodMeta) > 0 {
		if ordered, ok := chronologicalPeriods(periods, in.PeriodMeta); ok {
			orderedPeriods = ordered
		}
	}

	history := make([]PeriodFigures, 0, len(orderedPeriods))
	for _, p := range orderedPeriods {
		snap, ok := metricsResult.SnapshotFor(p)
		if !ok {
			continue
		}
		applied := adjustments.Apply(snap, in.Adjustments)
		history = append(history, PeriodFigures{
			Period:           p,
			ReportedEBITDA:   snap.EBITDA,
			NormalizedEBITDA: bridgeValue(applied.EBITDABridge),
			ReportedSDE:      snap.SDE,
			NormalizedSDE:    bridgeValue(applied.SDEBridge),
			Snapshot:         snap,
			Adjustments:      applied,
		})
	}
	result.History = history

	result.Adjustments = buildAdjustmentBreakdown(history)
	result.Recurrence = buildRecurrence(history, thresholds)
	result.RecurringAdjustments = buildRecurringSummary(history)

	if in.MaintainableEBITDA.Available {
		result.MaintainableEBITDA = in.MaintainableEBITDA
	} else {
		result.Warnings = append(result.Warnings, Issue{
			Code:     IssueMaintainableEBITDAUnavailable,
			Severity: SeverityWarning,
			Message:  "Input.MaintainableEBITDA is not available; maintainable-EBITDA-dependent ratios and flags are skipped",
		})
	}
	if in.MaintainableSDE.Available {
		result.MaintainableSDE = in.MaintainableSDE
	} else {
		result.Warnings = append(result.Warnings, Issue{
			Code:     IssueMaintainableSDEUnavailable,
			Severity: SeverityWarning,
			Message:  "Input.MaintainableSDE is not available; maintainable-SDE-dependent ratios and flags are skipped",
		})
	}

	if metricsResult.Trend != nil {
		result.RevenueGrowth = metricsResult.Trend.RevenueYoYGrowth
		result.RevenueVolatility = metricsResult.Trend.RevenueVolatility
		result.EBITDAMarginTrend = metricsResult.Trend.EBITDAMarginTrend
		result.EBITDAVolatility = metricsResult.Trend.EBITDAVolatility
		result.SDEVolatility = sdeVolatility(history, in.PeriodMeta)
	}

	result.Ratios = buildRatios(history)

	result.Flags = buildFlags(result, thresholds)

	if opts.ComputeScore {
		score := computeScore(result)
		result.Score = &score
	}

	return result
}

// bridgeValue converts an adjustments.Bridge into the equivalent
// metrics.MetricValue, so History's ReportedEBITDA/NormalizedEBITDA (and
// the SDE counterparts) share exactly one availability type throughout this
// package rather than mixing adjustments.Bridge's BaseAvailable/
// NormalizedValue pair with metrics.MetricValue's Available/Value pair.
func bridgeValue(b adjustments.Bridge) metrics.MetricValue {
	if !b.BaseAvailable {
		return metrics.Unavailable()
	}
	return metrics.AvailableValue(b.NormalizedValue)
}

// chronologicalPeriods sorts periods by meta's FiscalYear/Type/SequenceInYear
// ordering, the same fields financial/metrics.sortOrderedPeriods uses. ok is
// false if any period in periods has no meta entry, in which case the
// caller falls back to periods' own (dataset/lexical) order rather than
// guessing a partial sort.
func chronologicalPeriods(periods []financial.Period, meta map[financial.Period]metrics.PeriodInfo) ([]financial.Period, bool) {
	type entry struct {
		period financial.Period
		info   metrics.PeriodInfo
	}
	entries := make([]entry, 0, len(periods))
	for _, p := range periods {
		info, ok := meta[p]
		if !ok {
			return nil, false
		}
		entries = append(entries, entry{period: p, info: info})
	}

	rank := func(t metrics.PeriodType) int {
		switch t {
		case metrics.PeriodTypeFiscalYear, metrics.PeriodTypeYTD:
			return 0
		case metrics.PeriodTypeQuarter:
			return 1
		case metrics.PeriodTypeMonth:
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
	return ordered, true
}

// buildAdjustmentBreakdown sums every applied AppliedLine across every
// period in history, independently for EBITDA and SDE, both in total and
// per adjustments.Type. See AdjustmentBreakdown's doc comment for why
// EBITDA/SDE totals are never combined.
func buildAdjustmentBreakdown(history []PeriodFigures) AdjustmentBreakdown {
	var out AdjustmentBreakdown
	byType := make(map[adjustments.Type]*TypeBreakdown)

	get := func(t adjustments.Type) *TypeBreakdown {
		if tb, ok := byType[t]; ok {
			return tb
		}
		tb := &TypeBreakdown{Type: t}
		byType[t] = tb
		return tb
	}

	for _, pf := range history {
		for _, line := range pf.Adjustments.EBITDABridge.Applied {
			out.TotalEBITDAAdjustment += line.SignedAmount
			tb := get(line.Adjustment.Type)
			tb.EBITDATotal += line.SignedAmount
			tb.EBITDACount++
		}
		for _, line := range pf.Adjustments.SDEBridge.Applied {
			out.TotalSDEAdjustment += line.SignedAmount
			tb := get(line.Adjustment.Type)
			tb.SDETotal += line.SignedAmount
			tb.SDECount++
		}
	}

	types := make([]adjustments.Type, 0, len(byType))
	for t := range byType {
		types = append(types, t)
	}
	sort.Slice(types, func(i, j int) bool { return types[i] < types[j] })
	out.ByType = make([]TypeBreakdown, 0, len(types))
	for _, t := range types {
		out.ByType = append(out.ByType, *byType[t])
	}
	return out
}

// buildRatios computes Ratios from the last period in history (history is
// already in the caller-resolved chronological or dataset order — see
// Calculate). Returns the zero Ratios if history is empty.
func buildRatios(history []PeriodFigures) Ratios {
	if len(history) == 0 {
		return Ratios{}
	}
	last := history[len(history)-1]
	r := Ratios{Period: last.Period}

	if last.ReportedEBITDA.Available && last.ReportedEBITDA.Value != 0 && last.Adjustments.EBITDABridge.BaseAvailable {
		r.AdjustmentToEBITDA = metrics.AvailableValue(absRatio(last.Adjustments.EBITDABridge.TotalAdjustment, last.ReportedEBITDA.Value))
	}
	if last.ReportedSDE.Available && last.ReportedSDE.Value != 0 && last.Adjustments.SDEBridge.BaseAvailable {
		r.AdjustmentToSDE = metrics.AvailableValue(absRatio(last.Adjustments.SDEBridge.TotalAdjustment, last.ReportedSDE.Value))
	}
	return r
}

// absRatio returns |numerator| / |denominator|.
func absRatio(numerator, denominator float64) float64 {
	return math.Abs(numerator) / math.Abs(denominator)
}

// sdeVolatility computes the same year-over-year-growth-based sample
// standard deviation financial/metrics.calculateVolatility uses (see that
// function's doc comment for the exact statistic and its rationale), over
// history's NormalizedSDE series restricted to fiscal-year periods — the
// SDE-basis counterpart to metrics.Trend.EBITDAVolatility, which
// financial/metrics does not itself compute since SDE is out of that
// package's own Trend field set (Trend's volatility fields cover only
// revenue, EBITDA, and net income).
func sdeVolatility(history []PeriodFigures, meta map[financial.Period]metrics.PeriodInfo) metrics.VolatilityResult {
	if len(meta) == 0 {
		return metrics.VolatilityResult{}
	}
	var series []metrics.MetricValue
	for _, pf := range history {
		info, ok := meta[pf.Period]
		if !ok || info.Type != metrics.PeriodTypeFiscalYear {
			continue
		}
		series = append(series, pf.NormalizedSDE)
	}
	return calculateVolatility(series)
}

// calculateVolatility is this package's own copy of
// financial/metrics.calculateVolatility's exact method (sample standard
// deviation of the year-over-year percentage-change series), since that
// function is unexported and SDE volatility is not something
// financial/metrics itself computes (see sdeVolatility's doc comment).
// Duplicated deliberately rather than exported upstream purely for this one
// caller — see the repository's existing precedent of small,
// independently-owned duplicated helpers (e.g. adjustments.HasErrors) over
// growing a shared-utility package for a single reused function.
func calculateVolatility(series []metrics.MetricValue) metrics.VolatilityResult {
	var growthRates []float64
	for i := 1; i < len(series); i++ {
		from, to := series[i-1], series[i]
		if !from.Available || !to.Available || from.Value == 0 {
			continue
		}
		growthRates = append(growthRates, (to.Value-from.Value)/math.Abs(from.Value))
	}
	if len(growthRates) < 2 {
		return metrics.VolatilityResult{SampleSize: len(growthRates)}
	}

	mean := 0.0
	for _, g := range growthRates {
		mean += g
	}
	mean /= float64(len(growthRates))

	sumSquares := 0.0
	for _, g := range growthRates {
		diff := g - mean
		sumSquares += diff * diff
	}
	variance := sumSquares / float64(len(growthRates)-1)

	return metrics.VolatilityResult{Value: metrics.AvailableValue(math.Sqrt(variance)), SampleSize: len(growthRates)}
}
