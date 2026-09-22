package consensus

import (
	"math"
	"sort"
)

// percentOf returns numerator / |denominator|, or 0 if denominator is 0 —
// the one shared rule this package uses everywhere a ratio could
// otherwise divide by zero (CoefficientOfVariation, RelativeSpread,
// DeviationEntry.DeviationPercent): a defined, documented zero rather than
// a silently propagating NaN/Inf that would corrupt every figure derived
// from it and break JSON serialization (encoding/json rejects NaN/Inf).
func percentOf(numerator, denominator float64) float64 {
	if denominator == 0 {
		return 0
	}
	return numerator / math.Abs(denominator)
}

// Calculate computes Statistics, Range, and Dispersion over inputs — every
// method result a caller has chosen to include (e.g.
// valuation/orchestrator.Run.Successful(), narrowed to methods the caller
// wants in the consensus).
//
// Formulas (see each Statistics field's doc comment for the precise
// definition):
//
//	Simple Consensus (SimpleMean)   = sum(values) / count
//	Weighted Consensus (WeightedMean) = sum(value_i * normalized_weight_i)
//	Median                           = middle value(s) of sorted values
//	Spread                           = max - min
//	StdDev (population)              = sqrt(sum((v_i - mean)^2) / count)
//	CoefficientOfVariation           = StdDev / |SimpleMean|
//
// Calculate never panics and never fails outright unless inputs is empty
// (Result.Available == false). A weighted mean additionally requires
// every Weight to pass ValidateWeights; if it does not, WeightedMean and
// DeviationsFromWeightedMean are left zero-value/empty and
// Result.WeightsValid is false, with the validation failure(s) recorded in
// Errors — Calculate still computes and returns every other statistic
// (SimpleMean, Median, etc.) rather than failing the whole Result over an
// invalid weight set, since those statistics do not depend on weights at
// all.
//
// Consensus is not "true value" — see the package doc comment. Nothing
// here privileges SimpleMean or WeightedMean as more "correct" than the
// individual method results they were computed from.
func Calculate(inputs []Input) Result {
	result := Result{Included: inputs}

	if len(inputs) == 0 {
		result.Errors = []string{"no method results supplied to compute consensus from"}
		return result
	}

	values := make([]float64, len(inputs))
	for i, in := range inputs {
		values[i] = in.Value
	}

	result.MixedValueTypes = hasMixedValueTypes(inputs)
	if result.MixedValueTypes {
		result.Warnings = append(result.Warnings, "included methods carry more than one value type (enterprise/equity/asset); consider bridging to a common basis before comparing")
	}

	stats := Statistics{Count: len(values)}
	stats.SimpleMean = mean(values)
	stats.Median = median(values)
	stats.Min, stats.Max = minMax(values)
	stats.Spread = stats.Max - stats.Min
	stats.StdDev = populationStdDev(values, stats.SimpleMean)
	stats.CoefficientOfVariation = percentOf(stats.StdDev, stats.SimpleMean)
	stats.DeviationsFromMean = deviationsFrom(inputs, stats.SimpleMean)

	normalizedWeights, weightsOK, weightErrs := ValidateWeights(inputs)
	if weightsOK {
		stats.WeightedMean = weightedMean(values, normalizedWeights)
		stats.DeviationsFromWeightedMean = deviationsFrom(inputs, stats.WeightedMean)
		result.WeightsValid = true
	} else {
		result.Errors = append(result.Errors, weightErrs...)
	}

	result.Available = true
	result.Statistics = stats
	result.Range = Range{Min: stats.Min, Max: stats.Max}
	result.Dispersion = CalculateDispersion(stats)

	if len(inputs) == 1 {
		result.Warnings = append(result.Warnings, "only one method included; dispersion and range are not meaningful across a single value")
	}

	return result
}

func mean(values []float64) float64 {
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func weightedMean(values, normalizedWeights []float64) float64 {
	sum := 0.0
	for i, v := range values {
		sum += v * normalizedWeights[i]
	}
	return sum
}

func median(values []float64) float64 {
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	n := len(sorted)
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}

func minMax(values []float64) (min, max float64) {
	min, max = values[0], values[0]
	for _, v := range values[1:] {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	return min, max
}

func populationStdDev(values []float64, mean float64) float64 {
	sumSq := 0.0
	for _, v := range values {
		d := v - mean
		sumSq += d * d
	}
	return math.Sqrt(sumSq / float64(len(values)))
}

func deviationsFrom(inputs []Input, central float64) []DeviationEntry {
	out := make([]DeviationEntry, 0, len(inputs))
	for _, in := range inputs {
		dev := in.Value - central
		out = append(out, DeviationEntry{
			Method:           in.Method,
			Value:            in.Value,
			Deviation:        dev,
			DeviationPercent: percentOf(dev, central),
		})
	}
	return out
}

func hasMixedValueTypes(inputs []Input) bool {
	if len(inputs) == 0 {
		return false
	}
	first := inputs[0].ValueType
	for _, in := range inputs[1:] {
		if in.ValueType != first {
			return true
		}
	}
	return false
}
