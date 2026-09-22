package consensus

import (
	"math"

	"github.com/themurtez/go-valuate/valuation"
)

// ValidateWeights checks every Input's Weight for use in a weighted mean,
// and returns the normalized weight to use for each (in the same order as
// inputs), or ok=false with the reason(s) validation failed.
//
// Validation, applied to every included Input's Weight:
//
//  1. Every Weight must be finite (not NaN/+-Inf).
//  2. Every Weight must be non-negative. A negative weight has no
//     defensible meaning for a weighted average of valuation method
//     results (unlike, say, a signed adjustment) and is rejected outright.
//  3. The weights actually supplied must sum to a strictly positive
//     number, so division by the sum is well-defined (an all-zero-weight
//     set is rejected for the same reason: dividing by a zero sum is
//     undefined, and a caller who weights every method at 0 clearly did
//     not intend a weighted mean).
//
// Any failure means ok=false and every weight is invalid for this call —
// this package never silently drops the offending Input and proceeds with
// the rest, and never silently substitutes equal weights for an invalid
// set: an invalid weight set is a caller error to fix and resubmit, not
// something to paper over (mirroring
// financial/earnings.calculateWeightedAverage's "do not silently
// reinterpret a caller's weights" rule).
//
// Normalization. Weights are always normalized by dividing each by the
// sum of every supplied weight, so they sum to exactly 1 regardless of
// what scale the caller supplied them in (fractions, percentages, or
// arbitrary positive numbers like "3 stars out of 5") — this is a
// deliberate, simpler choice than financial/earnings' fraction-or-
// percentage-near-1-or-100 acceptance window: these are per-method
// importance weights being combined into a single weighted average across
// as few as two methods, not a caller-declared percentage allocation
// where an unnoticed scale error would be a red flag worth rejecting.
// Normalizing by the actual sum is always mathematically correct and
// removes an entire class of "my weights summed to 99.9, is that OK?"
// caller error.
func ValidateWeights(inputs []Input) (normalized []float64, ok bool, errs []valuation.Issue) {
	if len(inputs) == 0 {
		return nil, false, []valuation.Issue{{
			Code: valuation.IssueMissingRequiredData, Severity: valuation.SeverityError,
			Message: "no inputs supplied",
		}}
	}

	sum := 0.0
	for _, in := range inputs {
		w := in.Weight
		if math.IsNaN(w) || math.IsInf(w, 0) {
			errs = append(errs, valuation.Issue{
				Code: valuation.IssueInvalidWeight, Severity: valuation.SeverityError,
				Message: "weight for method " + string(in.Method) + " is not a finite number",
			})
			continue
		}
		if w < 0 {
			errs = append(errs, valuation.Issue{
				Code: valuation.IssueInvalidWeight, Severity: valuation.SeverityError,
				Message: "weight for method " + string(in.Method) + " is negative; negative weights are not usable for a weighted mean",
			})
			continue
		}
		sum += w
	}
	if len(errs) > 0 {
		return nil, false, errs
	}
	if sum <= 0 {
		return nil, false, []valuation.Issue{{
			Code: valuation.IssueInvalidWeight, Severity: valuation.SeverityError,
			Message: "weights sum to zero; cannot compute a weighted mean (supply at least one positive weight)",
		}}
	}

	normalized = make([]float64, len(inputs))
	for i, in := range inputs {
		normalized[i] = in.Weight / sum
	}
	return normalized, true, nil
}
