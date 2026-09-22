package earnings

import (
	"fmt"
	"math"
)

// weightSumTolerance is how far a weight sum may drift from 1.0 (or from
// 100, in percentage form — see resolveWeightSum) while still being
// accepted without normalization. This absorbs ordinary floating point
// representation error (e.g. 0.2+0.3+0.5 not landing on exactly 1.0) and
// simple rounding in caller-supplied percentages (e.g. 33.33+33.33+33.34),
// not a genuine mis-specification of weights.
const weightSumTolerance = 0.005

// calculateWeightedAverage computes a caller-weighted average of every
// available, comparable observation, using explicit per-period weights
// from Options.Weights.
//
// Weight validation, applied in order (any failure makes the whole result
// Unavailable — this package never silently drops or reinterprets a
// caller's weights to make an invalid set "work"):
//
//  1. Every available, comparable observation's period must have an entry
//     in Weights (ExclusionNoWeight otherwise — a caller who forgets to
//     weight a period finds out immediately rather than getting a
//     silently-wrong average that treats the missing weight as 0).
//  2. Every supplied weight must be finite.
//  3. Every supplied weight must be non-negative. A negative weight is
//     never a legitimate "mathematically usable" weight for an average
//     (see the package doc comment); it is rejected rather than allowed
//     through, unlike Adjustment.Amount's sign, which this package is
//     unrelated to.
//  4. The weights actually used (i.e. for included periods only) must sum
//     to a positive number, so division by the sum is well-defined.
//
// Normalization. If the included weights' sum is already within
// weightSumTolerance of 1.0 (weights supplied as fractions, e.g.
// 0.2/0.3/0.5) or of 100 (weights supplied as whole percentages, e.g.
// 20/30/50 — the example form in this package's design brief), the
// weighted average is computed directly against that sum with no
// rescaling beyond dividing by it (which is mathematically required
// regardless of form, not a "normalization" in the sense of silently
// fixing an invalid input). Any other sum (e.g. weights that don't come
// close to summing to 1 or 100 — a genuinely mis-specified set) is
// rejected outright rather than silently rescaled to sum to 1, per this
// package's explicit "do not silently normalize wildly invalid weights"
// design rule; the caller must fix their weights and call again.
func calculateWeightedAverage(result Result, comparable []Observation, weights map[string]float64) Result {
	var excluded []ExcludedObservation
	available := partitionAvailable(comparable, &excluded)
	result.ExcludedPeriods = append(result.ExcludedPeriods, excluded...)

	if len(available) == 0 {
		result.Errors = append(result.Errors, "no available comparable observations to average")
		return result
	}

	type weightedObs struct {
		obs    Observation
		weight float64
	}
	var included []weightedObs
	for _, obs := range available {
		w, ok := weights[obs.Period]
		if !ok {
			result.ExcludedPeriods = append(result.ExcludedPeriods, ExcludedObservation{
				Observation: obs, Reason: ExclusionNoWeight,
				Detail: fmt.Sprintf("no weight supplied for period %q", obs.Period),
			})
			continue
		}
		if math.IsNaN(w) || math.IsInf(w, 0) {
			result.Errors = append(result.Errors, fmt.Sprintf("weight for period %q is not finite: %v", obs.Period, w))
			return result
		}
		if w < 0 {
			result.Errors = append(result.Errors, fmt.Sprintf("weight for period %q is negative (%v); negative weights are not mathematically usable for an average", obs.Period, w))
			return result
		}
		included = append(included, weightedObs{obs: obs, weight: w})
	}

	if len(included) == 0 {
		result.Errors = append(result.Errors, "no comparable observation had a supplied weight")
		return result
	}

	sum := 0.0
	for _, wo := range included {
		sum += wo.weight
	}

	divisor, sumErr := resolveWeightSum(sum)
	if sumErr != "" {
		result.Errors = append(result.Errors, sumErr)
		return result
	}

	weightedTotal := 0.0
	rawValues := make([]float64, 0, len(included))
	includedObs := make([]Observation, 0, len(included))
	usedWeights := make([]WeightUsed, 0, len(included))
	for _, wo := range included {
		normalized := wo.weight / divisor
		weightedTotal += wo.obs.Value * normalized
		rawValues = append(rawValues, wo.obs.Value)
		includedObs = append(includedObs, wo.obs)
		usedWeights = append(usedWeights, WeightUsed{Period: wo.obs.Period, SuppliedWeight: wo.weight, NormalizedWeight: normalized})
	}

	result.IncludedPeriods = includedObs
	result.RawValues = rawValues
	result.Weights = usedWeights
	result.Available = true
	result.Value = weightedTotal
	return result
}

// resolveWeightSum checks that sum is usable as a weighted-average
// denominator without silently rescaling a mis-specified set of weights.
// Returns the divisor to use (either sum itself, when it is already
// exactly the mathematically required denominator) and an empty error, or
// a zero divisor and a non-empty error explaining why sum is rejected.
//
// A weight sum near 1.0 or near 100 is accepted as-is (fraction or
// percentage form respectively); the caller's weights are used exactly as
// supplied, divided by their own sum, which is required regardless of
// form and is not itself a "normalization" of invalid input. Any other sum
// is rejected — see calculateWeightedAverage's doc comment.
func resolveWeightSum(sum float64) (divisor float64, errMsg string) {
	if sum <= 0 {
		return 0, fmt.Sprintf("weights sum to %v, which is not positive; cannot compute a weighted average", sum)
	}
	if math.Abs(sum-1.0) <= weightSumTolerance {
		return sum, ""
	}
	if math.Abs(sum-100.0) <= weightSumTolerance*100 {
		return sum, ""
	}
	return 0, fmt.Sprintf("weights sum to %v, which is neither ~1.0 (fractional weights) nor ~100 (percentage weights); supply weights that sum to one of these rather than relying on implicit normalization", sum)
}
