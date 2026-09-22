package revenuequality

import (
	"math"
	"sort"

	"github.com/themurtez/go-valuate/financial"
)

// totalRevenueSeries extracts the TotalRevenue series from history, in the
// same order as history (assumed chronological, or dataset order if
// chronological order was unavailable).
func totalRevenueSeries(history []PeriodRevenue) []RevenueValue {
	series := make([]RevenueValue, len(history))
	for i, p := range history {
		series[i] = p.TotalRevenue
	}
	return series
}

// calculateStatistics computes Average/Median/Min/Max over every Available
// value in series, mirroring workingcapital.calculateStatistics's method
// exactly (minus Volatility, which this package reports separately as
// VolatilityResult — see calculateVolatility below — since
// RevenueVolatility carries its own SampleSize field at the Result level
// rather than nested inside Statistics).
func calculateStatistics(series []RevenueValue) Statistics {
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

	return stats
}

// median returns the median of values. Does not mutate values.
func median(values []float64) float64 {
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	n := len(sorted)
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}

// calculateTrend characterizes history's TotalRevenue series by comparing
// the first vs. last available observation (in history's own order,
// assumed chronological). Returns TrendUnavailable if fewer than two
// available observations exist. Mirrors
// workingcapital.calculateTrend/cashflow.calculateTrend's identical method
// exactly, including the zero-first-value fallback (see either for the
// full rationale).
func calculateTrend(history []PeriodRevenue) Trend {
	var withValue []PeriodRevenue
	for _, p := range history {
		if p.TotalRevenue.Available {
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
		FirstValue:  first.TotalRevenue,
		LastValue:   last.TotalRevenue,
	}

	delta := last.TotalRevenue.Value - first.TotalRevenue.Value
	if first.TotalRevenue.Value == 0 {
		if last.TotalRevenue.Value == 0 {
			t.Direction = TrendStable
			return t
		}
		t.Direction = classifyDirection(delta / math.Abs(last.TotalRevenue.Value))
		return t
	}

	percentChange := delta / math.Abs(first.TotalRevenue.Value)
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

// calculateGrowthSeries computes one GrowthPoint per chronologically
// adjacent pair in history, mirroring
// financial/metrics.calculateYoYGrowth's exact formula
// ((to-from)/|from|, zero-base handling) but, like
// analytics/ratios.calculateGrowthSeries, not restricted to
// fiscal-year-to-fiscal-year comparisons — see CAGRResult's doc comment for
// the shared rationale.
func calculateGrowthSeries(history []PeriodRevenue) []GrowthPoint {
	if len(history) < 2 {
		return nil
	}
	points := make([]GrowthPoint, 0, len(history)-1)
	for i := 1; i < len(history); i++ {
		from, to := history[i-1], history[i]
		point := GrowthPoint{FromPeriod: from.Period, ToPeriod: to.Period}
		switch {
		case !from.TotalRevenue.Available || !to.TotalRevenue.Available:
			// Growth left Unavailable.
		case from.TotalRevenue.Value == 0:
			point.GrowthFromZeroBase = true
		default:
			point.Growth = AvailableValue((to.TotalRevenue.Value - from.TotalRevenue.Value) / math.Abs(from.TotalRevenue.Value))
		}
		points = append(points, point)
	}
	return points
}

// calculateCAGR computes the compound annual growth rate across history's
// full available span, mirroring metrics.calculateCAGR's exact formula and
// validity rules (see CAGRResult's doc comment for why this is duplicated
// rather than calling metrics' unexported version). meta supplies each
// period's FiscalYear for the year-span calculation; if a needed period is
// missing from meta, CAGR is left with Invalid set rather than guessing a
// span.
func calculateCAGR(history []PeriodRevenue, meta map[financial.Period]PeriodInfo) CAGRResult {
	var withValue []PeriodRevenue
	for _, p := range history {
		if p.TotalRevenue.Available {
			withValue = append(withValue, p)
		}
	}
	if len(withValue) < 2 {
		return CAGRResult{}
	}

	first := withValue[0]
	last := withValue[len(withValue)-1]

	firstInfo, firstOK := meta[first.Period]
	lastInfo, lastOK := meta[last.Period]
	if !firstOK || !lastOK {
		return CAGRResult{
			FirstPeriod: first.Period,
			LastPeriod:  last.Period,
			Invalid:     "first or last period missing from PeriodMeta; fiscal-year span could not be determined",
		}
	}

	years := lastInfo.FiscalYear - firstInfo.FiscalYear
	result := CAGRResult{FirstPeriod: first.Period, LastPeriod: last.Period, Years: years}

	if years <= 0 {
		result.Invalid = "fiscal year span is not positive"
		return result
	}
	if first.TotalRevenue.Value <= 0 {
		result.Invalid = "starting value must be positive to compute a meaningful CAGR"
		return result
	}

	cagr := math.Pow(last.TotalRevenue.Value/first.TotalRevenue.Value, 1.0/float64(years)) - 1
	if math.IsNaN(cagr) || math.IsInf(cagr, 0) {
		result.Invalid = "computed CAGR is not a finite real number"
		return result
	}

	result.Value = AvailableValue(cagr)
	return result
}

// calculateVolatility computes the sample standard deviation of history's
// TotalRevenue period-over-period percentage-change series (in history's
// own chronological order), mirroring
// financial/metrics.calculateVolatility/workingcapital.calculateVolatility's
// exact method. Requires at least 2 valid growth observations (3 periods
// with every value Available and no zero-base transition); otherwise Value
// is Unavailable.
func calculateVolatility(history []PeriodRevenue) VolatilityResult {
	series := totalRevenueSeries(history)

	var growthRates []float64
	for i := 1; i < len(series); i++ {
		from, to := series[i-1], series[i]
		if !from.Available || !to.Available || from.Value == 0 {
			continue
		}
		growthRates = append(growthRates, (to.Value-from.Value)/math.Abs(from.Value))
	}
	if len(growthRates) < 2 {
		return VolatilityResult{SampleSize: len(growthRates)}
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

	return VolatilityResult{Value: AvailableValue(math.Sqrt(variance)), SampleSize: len(growthRates)}
}
