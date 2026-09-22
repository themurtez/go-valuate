package consensus

import (
	"math"
	"sort"

	"github.com/themurtez/go-valuate/valuation"
	"github.com/themurtez/go-valuate/valuation/basis"
)

// FormulaVersion identifies this package's fixed statistics/dispersion
// formula set (see Calculate's and CalculateDispersion's doc comments).
// Bump this whenever a formula or its documented rounding/edge-case
// behavior changes in a way that could make a historical Result not
// reproduce identically under the new code — see the repository README's
// versioning-strategy section.
const FormulaVersion = "1.0.0"

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
// wants in the consensus) — expressed on a single value basis (see the
// package doc comment on value basis and Options.TargetBasis).
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
// Every formula above is computed over Result.Included — the subset of
// inputs successfully expressed on Result.Basis — never over a raw mix of
// incompatible bases.
//
// Calculate never panics. Result.Available is false in exactly two cases:
// inputs is empty, or every Input was excluded by basis conversion (see
// basis.Convert) leaving nothing to compute over — including the
// TargetBasis-left-empty "inputs do not already share one basis" case,
// where Calculate refuses to guess a default rather than silently picking
// one. A weighted mean additionally requires every Included Input's
// Weight to pass ValidateWeights; if it does not, WeightedMean and
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
func Calculate(inputs []Input, opts Options) Result {
	result := Result{Requested: inputs, FormulaVersion: FormulaVersion}

	if len(inputs) == 0 {
		result.Errors = []valuation.Issue{{
			Code: valuation.IssueMissingRequiredData, Severity: valuation.SeverityError,
			Message: "no method results supplied to compute consensus from",
		}}
		return result
	}

	result.MixedValueTypes = hasMixedValueTypes(inputs)

	targetBasis, ok := resolveBasis(inputs, opts)
	if !ok {
		result.Errors = []valuation.Issue{{
			Code: valuation.IssueIncompatibleValueBasis, Severity: valuation.SeverityError,
			Message: "included method results carry more than one value type (enterprise/equity/asset) and no Options.TargetBasis was supplied to convert them to a common basis before averaging",
		}}
		return result
	}
	result.Basis = targetBasis

	conversions := basis.ConvertAll(conversionInputs(inputs), targetBasis)
	result.Conversions = conversions

	included, excluded := splitByOutcome(inputs, conversions)
	result.BasisExclusions = excluded
	if len(included) == 0 {
		result.Errors = []valuation.Issue{{
			Code: valuation.IssueIncompatibleValueBasis, Severity: valuation.SeverityError,
			Message: "no included method result could be expressed on the target basis " + string(targetBasis),
		}}
		return result
	}
	result.Included = included

	values := make([]float64, len(included))
	for i, in := range included {
		values[i] = in.Value
	}

	stats := Statistics{Count: len(values)}
	stats.SimpleMean = mean(values)
	stats.Median = median(values)
	stats.Min, stats.Max = minMax(values)
	stats.Spread = stats.Max - stats.Min
	stats.StdDev = populationStdDev(values, stats.SimpleMean)
	stats.CoefficientOfVariation = percentOf(stats.StdDev, stats.SimpleMean)
	stats.DeviationsFromMean = deviationsFrom(included, stats.SimpleMean)

	normalizedWeights, weightsOK, weightErrs := ValidateWeights(included)
	if weightsOK {
		stats.WeightedMean = weightedMean(values, normalizedWeights)
		stats.DeviationsFromWeightedMean = deviationsFrom(included, stats.WeightedMean)
		result.WeightsValid = true
	} else {
		result.Errors = append(result.Errors, weightErrs...)
	}

	result.Available = true
	result.Statistics = stats
	result.Range = Range{Min: stats.Min, Max: stats.Max}
	result.Dispersion = CalculateDispersion(stats)

	if dup := duplicateMethodIssue(included); dup != nil {
		result.Warnings = append(result.Warnings, *dup)
	}

	if len(excluded) > 0 {
		result.Warnings = append(result.Warnings, valuation.Issue{
			Code: valuation.IssueIncompatibleValueBasis, Severity: valuation.SeverityWarning,
			Message: "one or more method results could not be converted to the target basis and were excluded from this consensus; see BasisExclusions",
		})
	}

	if len(included) == 1 {
		result.Warnings = append(result.Warnings, valuation.Issue{
			Code: valuation.IssueMissingRequiredData, Severity: valuation.SeverityWarning,
			Message: "only one method included; dispersion is zero by definition (a single value cannot disagree with itself) and the method range has zero width, not \"not meaningful\"",
		})
	}

	return result
}

// resolveBasis determines the single value basis Calculate should convert
// every Input onto. If opts.TargetBasis is set, it is used unconditionally
// — every Input, including ones already on that basis, still goes through
// basis.Convert for a uniform, always-populated Conversions trail. If left
// empty, a basis is inferred only when every Input already shares one
// ValueType; ok is false if inputs is non-empty but mixed, signaling
// Calculate should fail rather than guess.
func resolveBasis(inputs []Input, opts Options) (target valuation.ValueType, ok bool) {
	if opts.TargetBasis != "" {
		return opts.TargetBasis, true
	}
	if !hasMixedValueTypes(inputs) {
		return inputs[0].ValueType, true
	}
	return "", false
}

// conversionInputs adapts consensus.Input to basis.ConversionInput.
func conversionInputs(inputs []Input) []basis.ConversionInput {
	out := make([]basis.ConversionInput, len(inputs))
	for i, in := range inputs {
		out[i] = basis.ConversionInput{
			Method: in.Method, ValueType: in.ValueType, Value: in.Value, Bridge: in.Bridge,
		}
	}
	return out
}

// splitByOutcome walks inputs and their parallel conversions (same order,
// same length — see basis.ConvertAll) and returns the Included slice
// (every non-excluded Input, with Value/ValueType replaced by the
// converted figure/target basis so every downstream statistic reads a
// single consistent basis) alongside the excluded Conversions.
func splitByOutcome(inputs []Input, conversions []basis.Conversion) (included []Input, excluded []basis.Conversion) {
	included = make([]Input, 0, len(inputs))
	for i, in := range inputs {
		c := conversions[i]
		if c.Outcome == basis.OutcomeExcluded {
			excluded = append(excluded, c)
			continue
		}
		converted := in
		converted.Value = c.ConvertedValue
		converted.ValueType = c.TargetBasis
		included = append(included, converted)
	}
	return included, excluded
}

// duplicateMethodIssue returns a SeverityWarning Issue if any Method
// appears more than once in included, or nil if every Method is unique.
// A duplicate is not blocked outright (a caller may deliberately include
// the same method computed under two different assumption sets), but it
// is surfaced — an unnoticed duplicate would silently overweight one
// method's view relative to the others in every mean/dispersion figure.
func duplicateMethodIssue(included []Input) *valuation.Issue {
	seen := make(map[valuation.Code]bool, len(included))
	for _, in := range included {
		if seen[in.Method] {
			return &valuation.Issue{
				Code: valuation.IssueDuplicateMethodResult, Severity: valuation.SeverityWarning,
				Message: "method " + string(in.Method) + " appears more than once in the included results; this will overweight it in every mean/dispersion figure unless deliberate",
			}
		}
		seen[in.Method] = true
	}
	return nil
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
