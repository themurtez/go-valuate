// Package consensus combines multiple included valuation method results
// into a set of descriptive statistics — simple and weighted means,
// median, range, spread, standard deviation, coefficient of variation, and
// a derived agreement/dispersion indicator — over a caller-supplied list
// of (method, value) inputs.
//
// This package computes no valuation figures itself: every Value it
// consumes is a headline figure (e.g. valuation/sde.Result.EquityValue)
// the caller has already produced via valuation/orchestrator or
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
// Value basis. A consensus must never silently average an Enterprise
// Value against an Equity Value against a net asset value. Every
// Calculate call resolves to exactly one basis (Result.Basis) that every
// included figure is expressed on:
//
//   - Options.TargetBasis explicit: every Input is converted to that basis
//     via valuation/basis before any statistic is computed. An Input that
//     cannot be converted (see valuation/basis.Convert's doc comment for
//     exactly which basis pairs have no deterministic conversion) is
//     excluded from Included/Statistics and reported in
//     Result.BasisExclusions with a structured reason — never averaged in
//     on its original, incompatible basis.
//   - Options.TargetBasis left empty ("infer"): if every Input already
//     shares one ValueType, that becomes Result.Basis and every Input is
//     included as-is (Outcome OutcomeDirect for each, no conversion
//     needed). If Inputs do not already share a basis, Calculate refuses
//     to guess which one the caller meant: Result.Available is false and
//     Result.Errors carries IssueIncompatibleValueBasis — this is the one
//     case Calculate fails outright rather than computing a partial
//     result, since there is no default basis to fall back to.
//
// Result.Conversions always lists every Input's outcome (Direct/
// Converted/Excluded), in order, so a caller/report can show exactly how
// each method's figure got to the basis actually used — see
// valuation/basis.Conversion.
package consensus

import (
	"github.com/themurtez/go-valuate/valuation"
	"github.com/themurtez/go-valuate/valuation/basis"
)

// Input is a single method's headline figure to include in a consensus
// calculation, plus the weight to apply if a weighted mean is requested.
type Input struct {
	// Method identifies which valuation method this Value came from.
	Method valuation.Code `json:"method"`
	// ValueType is the kind of value Value represents (enterprise, equity,
	// or asset) — echoed from the source method's own Result.ValueType, not
	// reinterpreted by this package. See the package doc comment on value
	// basis.
	ValueType valuation.ValueType `json:"value_type"`
	// Value is the method's headline figure to include, on ValueType's
	// basis.
	Value float64 `json:"value"`
	// Weight is this method's caller-supplied importance weight, used only
	// for WeightedMean. See Options.Weighting and ValidateWeights for how
	// Weight is validated and normalized.
	Weight float64 `json:"weight"`
	// Bridge is the source method's own already-computed Enterprise-Value-
	// to-Equity-Value bridge (echoing e.g. valuation/ebitda.Result.Bridge),
	// if any — used only when Options.TargetBasis requires converting this
	// Input's ValueTypeEnterprise value to equity. The zero Bridge
	// (Available == false) means "no bridge available," matching every
	// method package's own convention; see valuation/basis.Convert.
	Bridge valuation.Bridge `json:"bridge,omitempty"`
}

// Options controls how Calculate combines its inputs.
type Options struct {
	// TargetBasis is the value basis every included Input must be
	// expressed on before Calculate computes any statistic. Left empty,
	// Calculate infers a basis only when every Input already shares one —
	// see the package doc comment on value basis for the full rule,
	// including the incompatible-basis failure case.
	TargetBasis valuation.ValueType `json:"target_basis,omitempty"`
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
	// feeds the dispersion score. Mathematically 0 when Count == 1 (a
	// single value has nothing to differ from) — see
	// CalculateDispersion's doc comment for why this reads as "dispersion
	// is zero by definition," not "unavailable."
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
	// Requested echoes every Input Calculate was given, in original order
	// and on each Input's own original ValueType/Value — never converted —
	// so a Result is self-contained and a caller can always see exactly
	// what was asked for, independent of what ended up usable.
	Requested []Input `json:"requested"`
	// Included is the subset of Requested that ended up expressed on
	// Basis and actually fed into Statistics — every Value here is on
	// Basis, in the same order as Requested with any BasisExclusions
	// entries removed. Statistics, Range, and ValidateWeights are all
	// computed over exactly this slice, so a caller re-deriving normalized
	// weights (see valuation/report.BuildConsensusInputs) gets figures
	// consistent with what Calculate itself used.
	Included []Input `json:"included"`
	// Basis is the value basis every Value in Included is expressed on —
	// either Options.TargetBasis (if set) or the single ValueType every
	// Requested Input already shared (if Options.TargetBasis was left
	// empty and inference succeeded). Empty when Available is false.
	Basis valuation.ValueType `json:"basis,omitempty"`
	// Conversions lists, for every Requested Input in order, what
	// basis.Convert did with it — Direct (already on Basis), Converted
	// (bridged or identity-converted to Basis), or Excluded (no
	// deterministic conversion existed, see BasisExclusions). Always
	// populated whenever Available is true, even when every Input was
	// already on the same basis (all Direct) — so a caller/report can
	// always show the full basis story, not just the excluded cases.
	Conversions []basis.Conversion `json:"conversions,omitempty"`
	// BasisExclusions is the subset of Conversions with Outcome ==
	// basis.OutcomeExcluded — the methods that could not be expressed on
	// Basis at all and were consequently left out of Included/Statistics.
	// A structured, non-empty ExclusionReason accompanies every entry —
	// see basis.Conversion. Never silently dropped without a trace: every
	// exclusion here is also present in Conversions and explains why a
	// method a caller may have expected to see in Included is absent.
	BasisExclusions []basis.Conversion `json:"basis_exclusions,omitempty"`
	// Available is false if Statistics could not be computed at all — no
	// Input supplied, every Input excluded by basis conversion, or
	// Options.TargetBasis was left empty and Requested did not already
	// share one basis (see the package doc comment). Every other field is
	// zero-value when Available is false.
	Available bool `json:"available"`
	// Statistics holds every descriptive measure, computed over Included.
	// Meaningful only when Available is true.
	Statistics Statistics `json:"statistics"`
	// Range is the explicit method range (Min, Max) over Included.
	// Meaningful only when Available is true.
	Range Range `json:"range"`
	// WeightsValid is true if every Input.Weight in Included passed
	// ValidateWeights and a WeightedMean was actually computed. False
	// means WeightedMean and DeviationsFromWeightedMean are zero-value/
	// empty — see Errors for why.
	WeightsValid bool `json:"weights_valid"`
	// MixedValueTypes is true if Requested contained more than one
	// distinct ValueType before conversion — purely informational once
	// Basis/Conversions exist (it no longer determines whether Calculate
	// blocks or excludes anything by itself; see the package doc comment
	// on value basis for what actually does).
	MixedValueTypes bool `json:"mixed_value_types"`
	// Dispersion is the derived agreement/dispersion indicator, computed
	// over Included — see CalculateDispersion.
	Dispersion Dispersion `json:"dispersion"`
	// FormulaVersion identifies which version of this package's fixed
	// formula set (see Calculate's doc comment) produced this Result —
	// see the repository README's versioning-strategy section.
	FormulaVersion string `json:"formula_version"`
	// Warnings carries non-blocking notes (e.g. a single-method Input
	// where dispersion is not meaningful by definition).
	Warnings []valuation.Issue `json:"warnings,omitempty"`
	// Errors carries the reason(s) Available or WeightsValid is false.
	Errors []valuation.Issue `json:"errors,omitempty"`
}
