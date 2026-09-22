// Package consensus combines multiple included valuation method results
// into a set of descriptive statistics — simple and weighted means,
// median, range, spread, standard deviation, coefficient of variation, and
// a derived agreement/dispersion indicator — over a caller-supplied list
// of (method, value) inputs.
//
// This package computes no valuation figures itself: every Value it
// consumes is a headline figure (e.g. valuation/sde.Result.EquityValue,
// or a Bridge.EquityValue when the caller has chosen to compare methods on
// an equity basis — see the package doc comment on mixed value types
// below) the caller has already produced via valuation/orchestrator or
// individual method packages. It only combines already-produced numbers,
// and does so with a single, explicit, documented set of formulas — see
// Calculate's doc comment.
//
// Consensus is not "true value." A Simple/Weighted Consensus figure is a
// descriptive statistic over whatever methods were included — nothing in
// this package's output, naming, or documentation claims it is the
// correct or actual value of the business; it is one deterministic way of
// summarizing several independent method results. See DispersionScore's
// doc comment for the same caveat applied to the agreement indicator.
//
// Mixed value types. This package does not itself enforce that every
// Input has the same valuation.ValueType (enterprise/equity/asset) — a
// caller comparing results across value types is responsible for bridging
// them to a common basis first (see each method package's EquityBridge)
// before calling Calculate, since averaging an enterprise value against
// an equity value would silently misrepresent both. Input.ValueType is
// carried through to Result so a caller/report can display or validate
// this itself; see Result.MixedValueTypes.
package consensus

import "github.com/themurtez/go-valuate/valuation"

// Input is a single method's headline figure to include in a consensus
// calculation, plus the weight to apply if a weighted mean is requested.
type Input struct {
	// Method identifies which valuation method this Value came from.
	Method valuation.Code `json:"method"`
	// ValueType is the kind of value Value represents (enterprise, equity,
	// or asset) — echoed from the source method's own Result.ValueType, not
	// reinterpreted by this package. See the package doc comment on mixed
	// value types.
	ValueType valuation.ValueType `json:"value_type"`
	// Value is the method's headline figure to include.
	Value float64 `json:"value"`
	// Weight is this method's caller-supplied importance weight, used only
	// for WeightedMean. See Options.Weighting and ValidateWeights for how
	// Weight is validated and normalized.
	Weight float64 `json:"weight"`
}

// DeviationEntry is one included method's Value expressed as a deviation
// from a central figure (SimpleMean or WeightedMean).
type DeviationEntry struct {
	Method valuation.Code `json:"method"`
	// Value is the method's original Value.
	Value float64 `json:"value"`
	// Deviation is Value minus the central figure (signed: positive means
	// above the central figure, negative means below).
	Deviation float64 `json:"deviation"`
	// DeviationPercent is Deviation divided by the central figure,
	// expressed as a decimal (e.g. 0.10 = 10% above). Zero (not NaN/Inf)
	// when the central figure is zero — see percentOf.
	DeviationPercent float64 `json:"deviation_percent"`
}

// Statistics holds every descriptive measure Calculate computes over a set
// of included Input values. Central figures (SimpleMean, WeightedMean,
// Median) are all populated whenever there is at least one included value;
// WeightedMean is additionally gated on valid weights (see
// Result.WeightsValid).
type Statistics struct {
	// Count is the number of included Input values these Statistics were
	// computed over.
	Count int `json:"count"`
	// SimpleMean is the unweighted arithmetic mean of every included Value:
	// sum(values) / count. Called "Simple Consensus" in caller-facing
	// contexts — never "true value" (see the package doc comment).
	SimpleMean float64 `json:"simple_mean"`
	// WeightedMean is the weighted mean of every included Value using
	// normalized weights (see ValidateWeights): sum(value_i * weight_i) /
	// sum(weight_i), with weights already normalized to sum to 1 before
	// this division (so the formula is equivalently sum(value_i *
	// normalized_weight_i)). Meaningful only when Result.WeightsValid is
	// true; zero otherwise. Called "Weighted Consensus" in caller-facing
	// contexts.
	WeightedMean float64 `json:"weighted_mean"`
	// Median is the middle value of every included Value sorted ascending
	// (the average of the two middle values for an even count).
	Median float64 `json:"median"`
	// Min is the smallest included Value.
	Min float64 `json:"min"`
	// Max is the largest included Value.
	Max float64 `json:"max"`
	// Spread is Max - Min: the method range's width, in the same units as
	// Value. See Result.Range for the range as an explicit (Min, Max) pair.
	Spread float64 `json:"spread"`
	// StdDev is the population standard deviation of every included Value:
	// sqrt(sum((value_i - SimpleMean)^2) / Count). Population (dividing by
	// Count), not sample (Count-1), since every included method result is
	// the complete set being summarized, not a sample drawn from a larger
	// population — see CoefficientOfVariation's doc comment for how this
	// feeds the dispersion score.
	StdDev float64 `json:"std_dev"`
	// CoefficientOfVariation is StdDev / |SimpleMean|, a scale-independent
	// dispersion measure. Zero when SimpleMean is zero (see percentOf);
	// this is a defined edge case documented here rather than left as a
	// division-by-zero NaN.
	CoefficientOfVariation float64 `json:"coefficient_of_variation"`
	// DeviationsFromMean is each included method's DeviationEntry relative
	// to SimpleMean, in Input order.
	DeviationsFromMean []DeviationEntry `json:"deviations_from_mean"`
	// DeviationsFromWeightedMean is each included method's DeviationEntry
	// relative to WeightedMean, in Input order. Empty when
	// Result.WeightsValid is false.
	DeviationsFromWeightedMean []DeviationEntry `json:"deviations_from_weighted_mean,omitempty"`
}

// Range is the method range: the explicit (Min, Max) pair Statistics
// derives its Spread from, surfaced as its own named type per the package
// brief's requirement that a range always be returned, distinct from any
// narrower "likely range" a future module might define on top of it (this
// package defines no narrower range — see the package doc comment).
type Range struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

// Result is the output of Calculate.
type Result struct {
	// Included echoes every Input Calculate was given, in order, so a
	// Result is self-contained.
	Included []Input `json:"included"`
	// Available is false if Statistics could not be computed at all (no
	// Input supplied) — see Calculate's doc comment. Every other field is
	// zero-value when Available is false.
	Available bool `json:"available"`
	// Statistics holds every descriptive measure. Meaningful only when
	// Available is true.
	Statistics Statistics `json:"statistics"`
	// Range is the explicit method range (Min, Max). Meaningful only when
	// Available is true.
	Range Range `json:"range"`
	// WeightsValid is true if every Input.Weight passed ValidateWeights and
	// a WeightedMean was actually computed. False means WeightedMean and
	// DeviationsFromWeightedMean are zero-value/empty — see Errors for why.
	WeightsValid bool `json:"weights_valid"`
	// MixedValueTypes is true if Included contains more than one distinct
	// ValueType — a signal (not a blocking error) that the caller may be
	// averaging incompatible bases (e.g. an enterprise value against an
	// equity value) without an explicit bridge. See the package doc
	// comment.
	MixedValueTypes bool `json:"mixed_value_types"`
	// Dispersion is the derived agreement/dispersion indicator — see
	// CalculateDispersion.
	Dispersion Dispersion `json:"dispersion"`
	// Warnings carries non-blocking notes (e.g. MixedValueTypes, or a
	// single-method Input where dispersion is not meaningful).
	Warnings []string `json:"warnings,omitempty"`
	// Errors carries the reason(s) Available or WeightsValid is false.
	Errors []string `json:"errors,omitempty"`
}
