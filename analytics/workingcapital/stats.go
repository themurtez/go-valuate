package workingcapital

import (
	"math"
	"sort"
)

// nwcSeries extracts the NWC.Value series from history, in the same order
// as history (assumed chronological, or dataset order if chronological
// order was unavailable).
func nwcSeries(history []PeriodNWC) []NWCValue {
	series := make([]NWCValue, len(history))
	for i, p := range history {
		series[i] = p.NWC
	}
	return series
}

// nwcPercentSeries extracts the NWCPercentOfRevenue series from history.
func nwcPercentSeries(history []PeriodNWC) []NWCValue {
	series := make([]NWCValue, len(history))
	for i, p := range history {
		series[i] = p.NWCPercentOfRevenue
	}
	return series
}

// calculateStatistics computes Average/Median/Min/Max/Volatility over every
// Available value in series, preserving series' order for the
// period-over-period volatility calculation (see calculateVolatility).
func calculateStatistics(series []NWCValue) Statistics {
	var available []float64
	for _, v := range series {
		if v.Available {
			available = append(available, v.Value)
		}
	}

	stats := Statistics{SampleSize: len(available)}
	if len(available) == 0 {
		return stats
	}

	sum := 0.0
	minV, maxV := available[0], available[0]
	for _, v := range available {
		sum += v
		if v < minV {
			minV = v
		}
		if v > maxV {
			maxV = v
		}
	}
	stats.Average = AvailableValue(sum / float64(len(available)))
	stats.Median = AvailableValue(median(available))
	stats.Min = AvailableValue(minV)
	stats.Max = AvailableValue(maxV)

	vol, sampleSize := calculateVolatility(series)
	stats.Volatility = vol
	stats.VolatilitySampleSize = sampleSize

	return stats
}

// median returns the median of values, which is mutated (sorted) as a
// side effect — callers must pass a slice they own (calculateStatistics
// always builds `available` fresh, so this is safe).
func median(values []float64) float64 {
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	n := len(sorted)
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}

// calculateVolatility computes the sample standard deviation of the
// period-over-period percentage-change series derived from series (in
// series' own chronological order), mirroring
// financial/metrics.calculateVolatility's exact method (see that
// function's doc comment for the full rationale: this measures how much
// the period-over-period change itself swings, which is more meaningful
// for stability assessment than the standard deviation of raw dollar
// levels). Requires at least 2 valid growth observations (3 periods with
// every value Available and no zero-base transition); otherwise Value is
// Unavailable.
func calculateVolatility(series []NWCValue) (NWCValue, int) {
	var growthRates []float64
	for i := 1; i < len(series); i++ {
		from, to := series[i-1], series[i]
		if !from.Available || !to.Available || from.Value == 0 {
			continue
		}
		growthRates = append(growthRates, (to.Value-from.Value)/math.Abs(from.Value))
	}
	if len(growthRates) < 2 {
		return Unavailable(), len(growthRates)
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

	return AvailableValue(math.Sqrt(variance)), len(growthRates)
}

// calculateTrend characterizes history's NWC.Value series by comparing the
// first vs. last available observation (in history's own order, assumed
// chronological). Returns TrendUnavailable if fewer than two available
// observations exist.
//
// When the first observation is exactly 0, a percentage change is
// undefined (division by zero), so Direction falls back to a direct sign
// comparison of the raw delta against TrendFlatBandPercent applied to
// |LastValue| instead of |FirstValue| — still deterministic, just using
// the only nonzero scale reference available. PercentChange itself is left
// Unavailable in that case, mirroring
// financial/metrics.GrowthPoint.GrowthFromZeroBase's identical handling of
// zero-base growth.
func calculateTrend(history []PeriodNWC) Trend {
	var withValue []PeriodNWC
	for _, p := range history {
		if p.NWC.Available {
			withValue = append(withValue, p)
		}
	}
	if len(withValue) < 2 {
		return Trend{Direction: TrendUnavailable}
	}

	first := withValue[0]
	last := withValue[len(withValue)-1]

	t := Trend{
		FirstPeriod: first.Period,
		LastPeriod:  last.Period,
		FirstValue:  first.NWC,
		LastValue:   last.NWC,
	}

	delta := last.NWC.Value - first.NWC.Value
	if first.NWC.Value == 0 {
		if last.NWC.Value == 0 {
			t.Direction = TrendStable
			return t
		}
		t.Direction = classifyDirection(delta / math.Abs(last.NWC.Value))
		return t
	}

	percentChange := delta / math.Abs(first.NWC.Value)
	t.PercentChange = AvailableValue(percentChange)
	t.Direction = classifyDirection(percentChange)
	return t
}

// classifyDirection maps a percent change to a TrendDirection using
// TrendFlatBandPercent as the stable/non-stable boundary.
func classifyDirection(percentChange float64) TrendDirection {
	if percentChange > TrendFlatBandPercent {
		return TrendIncreasing
	}
	if percentChange < -TrendFlatBandPercent {
		return TrendDeclining
	}
	return TrendStable
}
