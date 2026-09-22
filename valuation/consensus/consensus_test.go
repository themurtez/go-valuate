package consensus

import (
	"math"
	"strings"
	"testing"

	"github.com/themurtez/go-valuate/valuation"
)

func approxEqual(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol
}

func TestCalculate_EqualValues_ZeroDispersion(t *testing.T) {
	inputs := []Input{
		{Method: "A", Value: 1_000_000, Weight: 1},
		{Method: "B", Value: 1_000_000, Weight: 1},
		{Method: "C", Value: 1_000_000, Weight: 1},
	}
	res := Calculate(inputs, Options{})
	if !res.Available {
		t.Fatalf("expected Available; errors=%v", res.Errors)
	}
	if res.Statistics.StdDev != 0 {
		t.Errorf("StdDev = %v, want 0", res.Statistics.StdDev)
	}
	if res.Statistics.CoefficientOfVariation != 0 {
		t.Errorf("CV = %v, want 0", res.Statistics.CoefficientOfVariation)
	}
	if res.Dispersion.Score != 100 {
		t.Errorf("Dispersion.Score = %d, want 100 for identical values", res.Dispersion.Score)
	}
	if res.Dispersion.Level != LevelHighConsensus {
		t.Errorf("Dispersion.Level = %v, want HIGH_CONSENSUS", res.Dispersion.Level)
	}
	if res.Statistics.SimpleMean != 1_000_000 {
		t.Errorf("SimpleMean = %v, want 1000000", res.Statistics.SimpleMean)
	}
}

func TestCalculate_DiverseValues(t *testing.T) {
	inputs := []Input{
		{Method: "A", Value: 500_000},
		{Method: "B", Value: 1_000_000},
		{Method: "C", Value: 2_500_000},
	}
	res := Calculate(inputs, Options{})
	if !res.Available {
		t.Fatalf("expected Available")
	}
	wantMean := (500_000.0 + 1_000_000 + 2_500_000) / 3
	if !approxEqual(res.Statistics.SimpleMean, wantMean, 0.01) {
		t.Errorf("SimpleMean = %v, want %v", res.Statistics.SimpleMean, wantMean)
	}
	if res.Statistics.Min != 500_000 || res.Statistics.Max != 2_500_000 {
		t.Errorf("Min/Max = %v/%v, want 500000/2500000", res.Statistics.Min, res.Statistics.Max)
	}
	if res.Statistics.Spread != 2_000_000 {
		t.Errorf("Spread = %v, want 2000000", res.Statistics.Spread)
	}
	if res.Statistics.CoefficientOfVariation <= 0 {
		t.Errorf("CV = %v, want > 0 for diverse values", res.Statistics.CoefficientOfVariation)
	}
	if res.Dispersion.Level == LevelHighConsensus {
		t.Errorf("Dispersion.Level = %v, want less than HIGH_CONSENSUS for widely diverse values", res.Dispersion.Level)
	}
}

func TestCalculate_OneMethod(t *testing.T) {
	res := Calculate([]Input{{Method: "A", Value: 750_000, Weight: 1}}, Options{})
	if !res.Available {
		t.Fatal("expected Available for a single method")
	}
	if res.Statistics.SimpleMean != 750_000 || res.Statistics.Median != 750_000 {
		t.Errorf("mean/median = %v/%v, want 750000/750000", res.Statistics.SimpleMean, res.Statistics.Median)
	}
	if res.Statistics.Spread != 0 {
		t.Errorf("Spread = %v, want 0 for a single value", res.Statistics.Spread)
	}
	if len(res.Warnings) == 0 {
		t.Error("expected a warning that dispersion is zero by definition for a single method")
	}
}

func TestCalculate_OneMethod_DispersionIsZeroByDefinition(t *testing.T) {
	// A single included method cannot disagree with itself: dispersion
	// must read as zero BY DEFINITION (maximum agreement, Score 100), not
	// as "unavailable" or "not meaningful" — see the task requirement that
	// this semantic be explicit and documented.
	res := Calculate([]Input{{Method: "A", Value: 750_000}}, Options{})
	if !res.Available {
		t.Fatal("expected Available for a single method")
	}
	if res.Statistics.CoefficientOfVariation != 0 {
		t.Errorf("CoefficientOfVariation = %v, want 0 for a single value", res.Statistics.CoefficientOfVariation)
	}
	if res.Dispersion.Score != 100 {
		t.Errorf("Dispersion.Score = %d, want 100 (zero dispersion, by definition, for a single method)", res.Dispersion.Score)
	}
	if res.Dispersion.Level != LevelHighConsensus {
		t.Errorf("Dispersion.Level = %v, want HIGH_CONSENSUS", res.Dispersion.Level)
	}
	foundZeroByDefinition := false
	for _, w := range res.Warnings {
		if strings.Contains(w.Message, "zero by definition") {
			foundZeroByDefinition = true
		}
	}
	if !foundZeroByDefinition {
		t.Errorf("Warnings = %+v, want a message naming dispersion as zero by definition, not merely \"not meaningful\"", res.Warnings)
	}
}

func TestCalculate_DuplicateMethodWarns(t *testing.T) {
	res := Calculate([]Input{
		{Method: valuation.CodeSDEMultiple, Value: 1_000_000},
		{Method: valuation.CodeSDEMultiple, Value: 1_200_000},
	}, Options{})
	if !res.Available {
		t.Fatal("expected Available = true; a duplicate method is a warning, not a blocking error")
	}
	found := false
	for _, w := range res.Warnings {
		if w.Code == valuation.IssueDuplicateMethodResult {
			found = true
		}
	}
	if !found {
		t.Errorf("Warnings = %+v, want an IssueDuplicateMethodResult entry", res.Warnings)
	}
}

func TestCalculate_NoMethods(t *testing.T) {
	res := Calculate(nil, Options{})
	if res.Available {
		t.Error("expected Available = false for no inputs")
	}
	if len(res.Errors) == 0 {
		t.Error("expected an error explaining why")
	}
}

func TestCalculate_WeightedMean(t *testing.T) {
	inputs := []Input{
		{Method: "A", Value: 1_000_000, Weight: 3},
		{Method: "B", Value: 2_000_000, Weight: 1},
	}
	res := Calculate(inputs, Options{})
	if !res.WeightsValid {
		t.Fatalf("expected valid weights; errors=%v", res.Errors)
	}
	want := (1_000_000.0*3 + 2_000_000.0*1) / 4
	if !approxEqual(res.Statistics.WeightedMean, want, 0.01) {
		t.Errorf("WeightedMean = %v, want %v", res.Statistics.WeightedMean, want)
	}
	// Weighted mean should pull toward the heavier-weighted method A.
	if res.Statistics.WeightedMean >= res.Statistics.SimpleMean {
		t.Errorf("WeightedMean (%v) should be less than SimpleMean (%v) given A's higher weight", res.Statistics.WeightedMean, res.Statistics.SimpleMean)
	}
}

func TestCalculate_WeightsNormalizedRegardlessOfScale(t *testing.T) {
	// Weights of {30, 10} and {3, 1} and {0.75, 0.25} should all produce
	// the identical weighted mean, since normalization divides by the sum.
	base := []Input{
		{Method: "A", Value: 1_000_000},
		{Method: "B", Value: 4_000_000},
	}
	scales := [][2]float64{{30, 10}, {3, 1}, {0.75, 0.25}}
	var means []float64
	for _, s := range scales {
		inputs := append([]Input(nil), base...)
		inputs[0].Weight = s[0]
		inputs[1].Weight = s[1]
		res := Calculate(inputs, Options{})
		if !res.WeightsValid {
			t.Fatalf("weights %v: expected valid; errors=%v", s, res.Errors)
		}
		means = append(means, res.Statistics.WeightedMean)
	}
	for i := 1; i < len(means); i++ {
		if !approxEqual(means[i], means[0], 0.01) {
			t.Errorf("weighted means differ across equivalent scales: %v", means)
		}
	}
}

func TestCalculate_InvalidWeights_Negative(t *testing.T) {
	inputs := []Input{
		{Method: "A", Value: 1_000_000, Weight: -1},
		{Method: "B", Value: 2_000_000, Weight: 1},
	}
	res := Calculate(inputs, Options{})
	if res.WeightsValid {
		t.Error("expected WeightsValid = false for a negative weight")
	}
	if res.Statistics.WeightedMean != 0 {
		t.Errorf("WeightedMean = %v, want 0 when weights are invalid", res.Statistics.WeightedMean)
	}
	if len(res.Errors) == 0 {
		t.Error("expected an error explaining the invalid weight")
	}
	// Other statistics must still be computed despite invalid weights.
	if !res.Available || res.Statistics.SimpleMean == 0 {
		t.Error("expected SimpleMean and other stats to still be computed despite invalid weights")
	}
}

func TestCalculate_InvalidWeights_AllZero(t *testing.T) {
	inputs := []Input{
		{Method: "A", Value: 1_000_000, Weight: 0},
		{Method: "B", Value: 2_000_000, Weight: 0},
	}
	res := Calculate(inputs, Options{})
	if res.WeightsValid {
		t.Error("expected WeightsValid = false when weights sum to zero")
	}
}

func TestCalculate_InvalidWeights_NonFinite(t *testing.T) {
	inputs := []Input{
		{Method: "A", Value: 1_000_000, Weight: math.NaN()},
		{Method: "B", Value: 2_000_000, Weight: 1},
	}
	res := Calculate(inputs, Options{})
	if res.WeightsValid {
		t.Error("expected WeightsValid = false for a non-finite weight")
	}
}

func TestCalculate_Median_OddCount(t *testing.T) {
	res := Calculate([]Input{
		{Method: "A", Value: 100},
		{Method: "B", Value: 300},
		{Method: "C", Value: 200},
	}, Options{})
	if res.Statistics.Median != 200 {
		t.Errorf("Median = %v, want 200", res.Statistics.Median)
	}
}

func TestCalculate_Median_EvenCount(t *testing.T) {
	res := Calculate([]Input{
		{Method: "A", Value: 100},
		{Method: "B", Value: 200},
		{Method: "C", Value: 300},
		{Method: "D", Value: 400},
	}, Options{})
	if res.Statistics.Median != 250 {
		t.Errorf("Median = %v, want 250", res.Statistics.Median)
	}
}

func TestCalculate_StdDev_KnownValue(t *testing.T) {
	// Population stddev of {2, 4, 4, 4, 5, 5, 7, 9} is 2.0 (textbook example).
	res := Calculate([]Input{
		{Method: "A", Value: 2}, {Method: "B", Value: 4}, {Method: "C", Value: 4}, {Method: "D", Value: 4},
		{Method: "E", Value: 5}, {Method: "F", Value: 5}, {Method: "G", Value: 7}, {Method: "H", Value: 9},
	}, Options{})
	if !approxEqual(res.Statistics.StdDev, 2.0, 0.001) {
		t.Errorf("StdDev = %v, want 2.0", res.Statistics.StdDev)
	}
}

func TestCalculate_CoefficientOfVariation(t *testing.T) {
	res := Calculate([]Input{
		{Method: "A", Value: 800_000},
		{Method: "B", Value: 1_000_000},
		{Method: "C", Value: 1_200_000},
	}, Options{})
	wantCV := res.Statistics.StdDev / res.Statistics.SimpleMean
	if !approxEqual(res.Statistics.CoefficientOfVariation, wantCV, 0.0001) {
		t.Errorf("CV = %v, want %v", res.Statistics.CoefficientOfVariation, wantCV)
	}
}

func TestCalculate_NegativeAndZeroValues(t *testing.T) {
	// Net asset value or SDE-based methods can validly produce zero or
	// negative headline values; consensus must handle this without
	// dividing by zero or producing NaN/Inf.
	res := Calculate([]Input{
		{Method: "A", Value: -100_000},
		{Method: "B", Value: 0},
		{Method: "C", Value: 100_000},
	}, Options{})
	if !res.Available {
		t.Fatal("expected Available for negative/zero values")
	}
	if res.Statistics.SimpleMean != 0 {
		t.Errorf("SimpleMean = %v, want 0", res.Statistics.SimpleMean)
	}
	// SimpleMean is 0, so CV/DeviationPercent must not be NaN/Inf (percentOf's defined zero case).
	if math.IsNaN(res.Statistics.CoefficientOfVariation) || math.IsInf(res.Statistics.CoefficientOfVariation, 0) {
		t.Errorf("CV = %v, want a defined finite value (0) when mean is 0", res.Statistics.CoefficientOfVariation)
	}
	for _, d := range res.Statistics.DeviationsFromMean {
		if math.IsNaN(d.DeviationPercent) || math.IsInf(d.DeviationPercent, 0) {
			t.Errorf("DeviationPercent = %v for method %s, want finite", d.DeviationPercent, d.Method)
		}
	}
}

func TestCalculate_DeviationsFromMean(t *testing.T) {
	res := Calculate([]Input{
		{Method: "A", Value: 900_000},
		{Method: "B", Value: 1_100_000},
	}, Options{})
	mean := res.Statistics.SimpleMean
	for _, d := range res.Statistics.DeviationsFromMean {
		wantDev := d.Value - mean
		if !approxEqual(d.Deviation, wantDev, 0.001) {
			t.Errorf("method %s: Deviation = %v, want %v", d.Method, d.Deviation, wantDev)
		}
	}
}

func TestCalculate_DeviationsFromWeightedMean(t *testing.T) {
	res := Calculate([]Input{
		{Method: "A", Value: 900_000, Weight: 2},
		{Method: "B", Value: 1_300_000, Weight: 1},
	}, Options{})
	if !res.WeightsValid {
		t.Fatal("expected valid weights")
	}
	wm := res.Statistics.WeightedMean
	for _, d := range res.Statistics.DeviationsFromWeightedMean {
		wantDev := d.Value - wm
		if !approxEqual(d.Deviation, wantDev, 0.001) {
			t.Errorf("method %s: Deviation from weighted mean = %v, want %v", d.Method, d.Deviation, wantDev)
		}
	}
}

func TestCalculate_MixedValueTypesWithNoTargetBasis_IsBlockingError(t *testing.T) {
	// Task requirement: a consensus must never silently average an
	// Enterprise Value against an Equity Value. With no explicit
	// Options.TargetBasis and no shared basis to infer, Calculate must
	// refuse to guess rather than average the raw incompatible values.
	res := Calculate([]Input{
		{Method: valuation.CodeEBITDAMultiple, Value: 1_000_000, ValueType: valuation.ValueTypeEnterprise},
		{Method: valuation.CodeSDEMultiple, Value: 900_000, ValueType: valuation.ValueTypeEquity},
	}, Options{})
	if !res.MixedValueTypes {
		t.Error("expected MixedValueTypes = true")
	}
	if res.Available {
		t.Fatal("expected Available = false when inputs carry mixed value types and no TargetBasis was supplied")
	}
	found := false
	for _, e := range res.Errors {
		if e.Code == valuation.IssueIncompatibleValueBasis {
			found = true
		}
	}
	if !found {
		t.Errorf("Errors = %+v, want an IssueIncompatibleValueBasis entry", res.Errors)
	}
}

func TestCalculate_MixedValueTypesWithTargetBasis_ConvertsSuccessfully(t *testing.T) {
	// The same mixed-basis situation, but the caller has explicitly
	// requested an equity-basis consensus and supplied the EBITDA
	// method's own equity bridge — the exact worked example from the task
	// prompt: Enterprise 1,800,000 + 100,000 cash - 300,000 debt = Equity
	// 1,600,000.
	res := Calculate([]Input{
		{
			Method: valuation.CodeEBITDAMultiple, Value: 1_800_000, ValueType: valuation.ValueTypeEnterprise,
			Bridge: valuation.Bridge{Available: true, EnterpriseValue: 1_800_000, ExcessCash: 100_000, TotalDebt: 300_000, EquityValue: 1_600_000},
		},
		{Method: valuation.CodeSDEMultiple, Value: 900_000, ValueType: valuation.ValueTypeEquity},
	}, Options{TargetBasis: valuation.ValueTypeEquity})

	if !res.Available {
		t.Fatalf("expected Available = true once a TargetBasis and equity bridge are supplied; errors=%+v", res.Errors)
	}
	if res.Basis != valuation.ValueTypeEquity {
		t.Errorf("Basis = %v, want equity_value", res.Basis)
	}
	if len(res.Included) != 2 {
		t.Fatalf("Included = %+v, want 2 entries (both methods convert successfully)", res.Included)
	}
	// The EBITDA method's included Value must be the converted 1,600,000
	// equity figure, never the raw 1,800,000 enterprise value.
	var ebitdaValue float64
	for _, in := range res.Included {
		if in.Method == valuation.CodeEBITDAMultiple {
			ebitdaValue = in.Value
		}
	}
	if ebitdaValue != 1_600_000 {
		t.Errorf("included EBITDA value = %v, want the converted 1,600,000, never the raw 1,800,000 enterprise value", ebitdaValue)
	}
	wantMean := (1_600_000.0 + 900_000.0) / 2
	if res.Statistics.SimpleMean != wantMean {
		t.Errorf("SimpleMean = %v, want %v (computed over converted values)", res.Statistics.SimpleMean, wantMean)
	}
	if len(res.BasisExclusions) != 0 {
		t.Errorf("BasisExclusions = %+v, want none (both methods converted successfully)", res.BasisExclusions)
	}
}

func TestCalculate_UnconvertibleInputExcludedNotAveragedRaw(t *testing.T) {
	// An enterprise-value method with no bridge cannot be converted to an
	// equity-basis consensus at all — it must be excluded, never averaged
	// in on its raw (wrong-basis) value.
	res := Calculate([]Input{
		{Method: valuation.CodeSDEMultiple, Value: 1_000_000, ValueType: valuation.ValueTypeEquity},
		{Method: valuation.CodeDCF, Value: 5_000_000, ValueType: valuation.ValueTypeEnterprise}, // no Bridge
	}, Options{TargetBasis: valuation.ValueTypeEquity})

	if !res.Available {
		t.Fatalf("expected Available = true (one method still converts); errors=%+v", res.Errors)
	}
	if len(res.Included) != 1 {
		t.Fatalf("Included = %+v, want exactly 1 (only SDE, which was already equity)", res.Included)
	}
	if res.Statistics.SimpleMean != 1_000_000 {
		t.Errorf("SimpleMean = %v, want 1,000,000 (the raw 5,000,000 DCF enterprise value must never be averaged in)", res.Statistics.SimpleMean)
	}
	if len(res.BasisExclusions) != 1 {
		t.Fatalf("BasisExclusions = %+v, want exactly 1 entry for DCF", res.BasisExclusions)
	}
	if res.BasisExclusions[0].Method != valuation.CodeDCF {
		t.Errorf("BasisExclusions[0].Method = %v, want %v", res.BasisExclusions[0].Method, valuation.CodeDCF)
	}
	if res.BasisExclusions[0].ExclusionReason == "" {
		t.Error("expected a non-empty ExclusionReason")
	}
}

func TestCalculate_AllInputsExcluded_Unavailable(t *testing.T) {
	res := Calculate([]Input{
		{Method: valuation.CodeDCF, Value: 5_000_000, ValueType: valuation.ValueTypeEnterprise}, // no Bridge
	}, Options{TargetBasis: valuation.ValueTypeEquity})
	if res.Available {
		t.Fatal("expected Available = false when every input is excluded by basis conversion")
	}
	if len(res.BasisExclusions) != 1 {
		t.Errorf("BasisExclusions = %+v, want exactly 1", res.BasisExclusions)
	}
}

func TestCalculate_SameValueTypesNoWarning(t *testing.T) {
	res := Calculate([]Input{
		{Method: valuation.CodeSDEMultiple, Value: 1_000_000, ValueType: valuation.ValueTypeEquity},
		{Method: valuation.CodeCapitalizationOfEarnings, Value: 950_000, ValueType: valuation.ValueTypeEquity},
	}, Options{})
	if res.MixedValueTypes {
		t.Error("expected MixedValueTypes = false when all inputs share a value type")
	}
}

func TestCalculate_RangeMatchesMinMax(t *testing.T) {
	res := Calculate([]Input{
		{Method: "A", Value: 300_000},
		{Method: "B", Value: 900_000},
	}, Options{})
	if res.Range.Min != res.Statistics.Min || res.Range.Max != res.Statistics.Max {
		t.Errorf("Range = %+v, want to match Statistics.Min/Max (%v/%v)", res.Range, res.Statistics.Min, res.Statistics.Max)
	}
}

func TestValidateWeights_RejectsNegativeWithoutDroppingOthers(t *testing.T) {
	inputs := []Input{
		{Method: "A", Weight: 1},
		{Method: "B", Weight: -0.5},
		{Method: "C", Weight: 2},
	}
	_, ok, errs := ValidateWeights(inputs)
	if ok {
		t.Fatal("expected ok=false")
	}
	if len(errs) != 1 {
		t.Errorf("expected exactly 1 error (for B), got %d: %v", len(errs), errs)
	}
}

func TestCalculateDispersion_EmptyStatistics(t *testing.T) {
	d := CalculateDispersion(Statistics{})
	if d.Score != 0 {
		t.Errorf("Score = %d, want 0 for empty statistics", d.Score)
	}
}

func TestCalculateDispersion_HighCVProducesLowConsensus(t *testing.T) {
	stats := Statistics{Count: 2, SimpleMean: 1000, StdDev: 800, CoefficientOfVariation: 0.8} // CV = 0.8, well above divisor
	d := CalculateDispersion(stats)
	if d.Level != LevelLowConsensus {
		t.Errorf("Level = %v, want LOW_CONSENSUS for CV=0.8", d.Level)
	}
	if d.Score != 0 {
		t.Errorf("Score = %d, want 0 (floored) for CV beyond the divisor", d.Score)
	}
}

func TestCalculateDispersion_ScoreNeverOutOfRange(t *testing.T) {
	for _, cv := range []float64{0, 0.1, 0.25, 0.5, 1.0, 5.0, 100.0} {
		stats := Statistics{Count: 2, SimpleMean: 1000, StdDev: cv * 1000, CoefficientOfVariation: cv}
		d := CalculateDispersion(stats)
		if d.Score < 0 || d.Score > 100 {
			t.Errorf("CV=%v: Score = %d, want within [0,100]", cv, d.Score)
		}
	}
}
