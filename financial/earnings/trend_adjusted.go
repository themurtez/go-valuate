package earnings

import "math"

// calculateTrendAdjusted fits an ordinary-least-squares linear regression
// (earnings value against period index: 0, 1, 2, ... in the supplied
// chronological order) across every available, comparable observation, and
// uses the fitted line's value at the final period index as maintainable
// earnings.
//
// This is the one deterministic, well-defined trend method: linear
// regression has a single closed-form solution for a given set of points,
// with no subjective parameter (no smoothing factor, no choice of decay
// rate, no discretionary weighting). It requires at least 3 comparable
// available observations — 2 points make any "trend" identical to a
// straight line between them (no more informative than StrategyLatestPeriod
// or a 2-point simple average), so Calculate reports Unavailable below that
// rather than a technically-computable but not meaningfully "trend
// adjusted" result.
//
// Documented limitation: this method assumes the trend is well-approximated
// by a straight line across the full comparable window. It does not detect
// or special-case a level shift (e.g. a one-time acquisition that
// permanently changed the earnings base), regime changes, or
// seasonality — those are subjective judgment calls this package
// deliberately leaves to the caller (e.g. by choosing which observations
// to include, or by not using this strategy at all when a straight-line
// trend isn't a defensible model of the business). Callers needing that
// judgment should treat this strategy's output as one input among several,
// not an authoritative maintainable-earnings figure on its own.
func calculateTrendAdjusted(result Result, comparable []Observation) Result {
	var excluded []ExcludedObservation
	available := partitionAvailable(comparable, &excluded)
	result.ExcludedPeriods = append(result.ExcludedPeriods, excluded...)

	if len(available) < 3 {
		result.Errors = append(result.Errors, "trend_adjusted requires at least 3 comparable available observations to fit a meaningful linear trend")
		return result
	}

	n := float64(len(available))
	var sumX, sumY, sumXY, sumXX float64
	rawValues := make([]float64, 0, len(available))
	for i, obs := range available {
		x := float64(i)
		y := obs.Value
		sumX += x
		sumY += y
		sumXY += x * y
		sumXX += x * x
		rawValues = append(rawValues, y)
	}

	denominator := n*sumXX - sumX*sumX
	if denominator == 0 {
		result.Errors = append(result.Errors, "could not fit a linear trend (degenerate period index spread)")
		return result
	}

	slope := (n*sumXY - sumX*sumY) / denominator
	intercept := (sumY - slope*sumX) / n

	lastIndex := n - 1
	fitted := intercept + slope*lastIndex
	if math.IsNaN(fitted) || math.IsInf(fitted, 0) {
		result.Errors = append(result.Errors, "computed trend value is not a finite number")
		return result
	}

	result.IncludedPeriods = available
	result.RawValues = rawValues
	result.Available = true
	result.Value = fitted
	return result
}
