package kpi

import (
	"math"
	"testing"
)

func usd(v float64) Value { return availableValue(v) }

func currencyUnit(code string) Unit { return Unit{Kind: UnitCurrency, CurrencyCode: code} }

func metricInput(code, period string, value float64, available bool, unit Unit) MetricValue {
	return MetricValue{Code: code, Period: period, Value: value, Available: available, Unit: unit, Aggregation: AggregationSum}
}

func onePeriodInput(defs []Definition, metrics []MetricValue) Input {
	return Input{
		Definitions: defs,
		Metrics:     metrics,
		Periods:     []Period{{Code: "P1", Sequence: 1}},
	}
}

func firstResult(t *testing.T, res Result, code string) KPIResult {
	t.Helper()
	for _, kr := range res.KPIResults {
		if kr.Code == code {
			return kr
		}
	}
	t.Fatalf("no KPIResult for code %q; DefinitionIssues=%v EvaluationIssues=%v", code, res.DefinitionIssues, res.EvaluationIssues)
	return KPIResult{}
}

// --- Every implemented operator ---

func TestOperator_Add(t *testing.T) {
	def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Binary(OpAdd, Metric(MetricRef{Code: "a"}), Metric(MetricRef{Code: "b"}))}
	res := Calculate(onePeriodInput([]Definition{def}, []MetricValue{
		metricInput("a", "P1", 10, true, currencyUnit("USD")),
		metricInput("b", "P1", 5, true, currencyUnit("USD")),
	}), Options{})
	kr := firstResult(t, res, "k")
	if !kr.Value.Available || kr.Value.Amount != 15 {
		t.Fatalf("got %+v", kr.Value)
	}
}

func TestOperator_Subtract(t *testing.T) {
	def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Binary(OpSubtract, Metric(MetricRef{Code: "a"}), Metric(MetricRef{Code: "b"}))}
	res := Calculate(onePeriodInput([]Definition{def}, []MetricValue{
		metricInput("a", "P1", 10, true, currencyUnit("USD")),
		metricInput("b", "P1", 5, true, currencyUnit("USD")),
	}), Options{})
	kr := firstResult(t, res, "k")
	if kr.Value.Amount != 5 {
		t.Fatalf("got %+v", kr.Value)
	}
}

func TestOperator_Multiply(t *testing.T) {
	def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Binary(OpMultiply, Metric(MetricRef{Code: "a"}), Metric(MetricRef{Code: "b"}))}
	res := Calculate(onePeriodInput([]Definition{def}, []MetricValue{
		metricInput("a", "P1", 10, true, currencyUnit("USD")),
		metricInput("b", "P1", 2, true, Unit{Kind: UnitRatio}),
	}), Options{})
	kr := firstResult(t, res, "k")
	if kr.Value.Amount != 20 {
		t.Fatalf("got %+v", kr.Value)
	}
}

func TestOperator_Divide(t *testing.T) {
	def := Definition{Code: "k", Unit: Unit{Kind: UnitRatio}, Formula: Binary(OpDivide, Metric(MetricRef{Code: "a"}), Metric(MetricRef{Code: "b"}))}
	res := Calculate(onePeriodInput([]Definition{def}, []MetricValue{
		metricInput("a", "P1", 10, true, currencyUnit("USD")),
		metricInput("b", "P1", 5, true, currencyUnit("USD")),
	}), Options{})
	kr := firstResult(t, res, "k")
	if kr.Value.Amount != 2 {
		t.Fatalf("got %+v", kr.Value)
	}
}

func TestOperator_Negate(t *testing.T) {
	def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Unary(OpNegate, Metric(MetricRef{Code: "a"}))}
	res := Calculate(onePeriodInput([]Definition{def}, []MetricValue{metricInput("a", "P1", 10, true, currencyUnit("USD"))}), Options{})
	kr := firstResult(t, res, "k")
	if kr.Value.Amount != -10 {
		t.Fatalf("got %+v", kr.Value)
	}
}

func TestOperator_Abs(t *testing.T) {
	def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Unary(OpAbs, Metric(MetricRef{Code: "a"}))}
	res := Calculate(onePeriodInput([]Definition{def}, []MetricValue{metricInput("a", "P1", -10, true, currencyUnit("USD"))}), Options{})
	kr := firstResult(t, res, "k")
	if kr.Value.Amount != 10 {
		t.Fatalf("got %+v", kr.Value)
	}
}

func TestOperator_MinMax(t *testing.T) {
	metrics := []MetricValue{
		metricInput("a", "P1", 3, true, currencyUnit("USD")),
		metricInput("b", "P1", 7, true, currencyUnit("USD")),
		metricInput("c", "P1", 5, true, currencyUnit("USD")),
	}
	minDef := Definition{Code: "kmin", Unit: currencyUnit("USD"), Formula: Variadic(OpMin, Metric(MetricRef{Code: "a"}), Metric(MetricRef{Code: "b"}), Metric(MetricRef{Code: "c"}))}
	maxDef := Definition{Code: "kmax", Unit: currencyUnit("USD"), Formula: Variadic(OpMax, Metric(MetricRef{Code: "a"}), Metric(MetricRef{Code: "b"}), Metric(MetricRef{Code: "c"}))}
	res := Calculate(onePeriodInput([]Definition{minDef, maxDef}, metrics), Options{})
	if kr := firstResult(t, res, "kmin"); kr.Value.Amount != 3 {
		t.Fatalf("min got %+v", kr.Value)
	}
	if kr := firstResult(t, res, "kmax"); kr.Value.Amount != 7 {
		t.Fatalf("max got %+v", kr.Value)
	}
}

func TestOperator_Sum(t *testing.T) {
	def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Variadic(OpSum, Metric(MetricRef{Code: "a"}), Metric(MetricRef{Code: "b"}), Metric(MetricRef{Code: "c"}))}
	res := Calculate(onePeriodInput([]Definition{def}, []MetricValue{
		metricInput("a", "P1", 1, true, currencyUnit("USD")),
		metricInput("b", "P1", 2, true, currencyUnit("USD")),
		metricInput("c", "P1", 3, true, currencyUnit("USD")),
	}), Options{})
	kr := firstResult(t, res, "k")
	if kr.Value.Amount != 6 {
		t.Fatalf("got %+v", kr.Value)
	}
}

func TestOperator_Average(t *testing.T) {
	def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Variadic(OpAverage, Metric(MetricRef{Code: "a"}), Metric(MetricRef{Code: "b"}))}
	res := Calculate(onePeriodInput([]Definition{def}, []MetricValue{
		metricInput("a", "P1", 10, true, currencyUnit("USD")),
		metricInput("b", "P1", 20, true, currencyUnit("USD")),
	}), Options{})
	kr := firstResult(t, res, "k")
	if kr.Value.Amount != 15 {
		t.Fatalf("got %+v", kr.Value)
	}
}

func TestOperator_WeightedAverage(t *testing.T) {
	// (90*10 + 70*30) / (10+30) = (900+2100)/40 = 75
	def := Definition{Code: "k", Unit: Unit{Kind: UnitPercent}, Formula: Variadic(OpWeightedAverage,
		Metric(MetricRef{Code: "score1"}), Metric(MetricRef{Code: "weight1"}),
		Metric(MetricRef{Code: "score2"}), Metric(MetricRef{Code: "weight2"}))}
	res := Calculate(onePeriodInput([]Definition{def}, []MetricValue{
		metricInput("score1", "P1", 90, true, Unit{Kind: UnitPercent}),
		metricInput("weight1", "P1", 10, true, Unit{Kind: UnitCount}),
		metricInput("score2", "P1", 70, true, Unit{Kind: UnitPercent}),
		metricInput("weight2", "P1", 30, true, Unit{Kind: UnitCount}),
	}), Options{})
	kr := firstResult(t, res, "k")
	if !kr.Value.Available || math.Abs(kr.Value.Amount-75) > 1e-9 {
		t.Fatalf("got %+v", kr.Value)
	}
}

func TestOperator_WeightedAverage_ZeroTotalWeight(t *testing.T) {
	def := Definition{Code: "k", Unit: Unit{Kind: UnitPercent}, Formula: Variadic(OpWeightedAverage,
		Metric(MetricRef{Code: "score1"}), Metric(MetricRef{Code: "weight1"}))}
	res := Calculate(onePeriodInput([]Definition{def}, []MetricValue{
		metricInput("score1", "P1", 90, true, Unit{Kind: UnitPercent}),
		metricInput("weight1", "P1", 0, true, Unit{Kind: UnitCount}),
	}), Options{})
	kr := firstResult(t, res, "k")
	if kr.Value.Available || kr.Value.Reason != AvailabilityDivideByZero {
		t.Fatalf("got %+v, want unavailable DIVIDE_BY_ZERO", kr.Value)
	}
}

func TestOperator_Percent(t *testing.T) {
	def := Definition{Code: "k", Unit: Unit{Kind: UnitPercent}, Formula: Binary(OpPercent, Metric(MetricRef{Code: "a"}), Metric(MetricRef{Code: "b"}))}
	res := Calculate(onePeriodInput([]Definition{def}, []MetricValue{
		metricInput("a", "P1", 25, true, currencyUnit("USD")),
		metricInput("b", "P1", 100, true, currencyUnit("USD")),
	}), Options{})
	kr := firstResult(t, res, "k")
	if kr.Value.Amount != 25 {
		t.Fatalf("got %+v", kr.Value)
	}
}

func TestOperator_PercentChange(t *testing.T) {
	// 30 -> 35 relative change = 16.666...%
	def := Definition{Code: "k", Unit: Unit{Kind: UnitPercent}, Formula: Binary(OpPercentChange, Metric(MetricRef{Code: "cur"}), Metric(MetricRef{Code: "prior"}))}
	res := Calculate(onePeriodInput([]Definition{def}, []MetricValue{
		metricInput("cur", "P1", 35, true, Unit{Kind: UnitPercent}),
		metricInput("prior", "P1", 30, true, Unit{Kind: UnitPercent}),
	}), Options{})
	kr := firstResult(t, res, "k")
	want := (35.0 - 30.0) / 30.0 * 100
	if !kr.Value.Available || math.Abs(kr.Value.Amount-want) > 1e-9 {
		t.Fatalf("got %+v want %v", kr.Value, want)
	}
}

func TestOperator_Coalesce(t *testing.T) {
	def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Variadic(OpCoalesce, Metric(MetricRef{Code: "missing"}), Metric(MetricRef{Code: "fallback"}))}
	res := Calculate(onePeriodInput([]Definition{def}, []MetricValue{
		metricInput("fallback", "P1", 42, true, currencyUnit("USD")),
	}), Options{})
	kr := firstResult(t, res, "k")
	if kr.Value.Amount != 42 {
		t.Fatalf("got %+v", kr.Value)
	}
}

func TestOperator_Coalesce_NoneAvailable(t *testing.T) {
	def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Variadic(OpCoalesce, Metric(MetricRef{Code: "missing1"}), Metric(MetricRef{Code: "missing2"}))}
	res := Calculate(onePeriodInput([]Definition{def}, nil), Options{})
	kr := firstResult(t, res, "k")
	if kr.Value.Available {
		t.Fatalf("expected unavailable, got %+v", kr.Value)
	}
}

func TestOperator_Constant(t *testing.T) {
	def := Definition{Code: "k", Unit: Unit{Kind: UnitUnitless}, Formula: Const(3.14)}
	res := Calculate(onePeriodInput([]Definition{def}, nil), Options{})
	kr := firstResult(t, res, "k")
	if kr.Value.Amount != 3.14 {
		t.Fatalf("got %+v", kr.Value)
	}
}

// --- Argument validation ---

func TestArgumentValidation_WrongArity(t *testing.T) {
	cases := []struct {
		name string
		expr Expression
	}{
		{"add_1_arg", Unary(OpAdd, Const(1))},
		{"add_3_args", Variadic(OpAdd, Const(1), Const(2), Const(3))},
		{"negate_2_args", Binary(OpNegate, Const(1), Const(2))},
		{"min_1_arg", Unary(OpMin, Const(1))},
		{"sum_0_args", Variadic(OpSum)},
		{"weighted_avg_odd", Variadic(OpWeightedAverage, Const(1), Const(2), Const(3))},
		{"coalesce_0_args", Variadic(OpCoalesce)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			issues := validateExpression("k", c.expr)
			found := false
			for _, i := range issues {
				if i.Code == IssueInvalidArgumentCount {
					found = true
				}
			}
			if !found {
				t.Fatalf("expected IssueInvalidArgumentCount, got %v", issues)
			}
		})
	}
}

func TestArgumentValidation_UnknownOperator(t *testing.T) {
	issues := validateExpression("k", Expression{Op: "BOGUS"})
	found := false
	for _, i := range issues {
		if i.Code == IssueUnknownOperator {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected IssueUnknownOperator, got %v", issues)
	}
}

// --- Nested expressions ---

func TestNestedExpression_CustomComposite(t *testing.T) {
	// (A + B) / C
	def := Definition{Code: "composite", Unit: Unit{Kind: UnitRatio}, Formula: Binary(OpDivide,
		Binary(OpAdd, Metric(MetricRef{Code: "A"}), Metric(MetricRef{Code: "B"})),
		Metric(MetricRef{Code: "C"}))}
	res := Calculate(onePeriodInput([]Definition{def}, []MetricValue{
		metricInput("A", "P1", 10, true, currencyUnit("USD")),
		metricInput("B", "P1", 20, true, currencyUnit("USD")),
		metricInput("C", "P1", 5, true, currencyUnit("USD")),
	}), Options{})
	kr := firstResult(t, res, "composite")
	if !kr.Value.Available || kr.Value.Amount != 6 {
		t.Fatalf("got %+v", kr.Value)
	}
}

// --- Divide by zero ---

func TestDivideByZero(t *testing.T) {
	def := Definition{Code: "k", Unit: Unit{Kind: UnitRatio}, Formula: Binary(OpDivide, Metric(MetricRef{Code: "a"}), Metric(MetricRef{Code: "b"}))}
	res := Calculate(onePeriodInput([]Definition{def}, []MetricValue{
		metricInput("a", "P1", 100, true, currencyUnit("USD")),
		metricInput("b", "P1", 0, true, currencyUnit("USD")),
	}), Options{})
	kr := firstResult(t, res, "k")
	if kr.Value.Available || kr.Value.Reason != AvailabilityDivideByZero {
		t.Fatalf("got %+v", kr.Value)
	}
}

// --- Non-finite input/result ---

func TestNonFiniteInput_Constant(t *testing.T) {
	nan := math.NaN()
	def := Definition{Code: "k", Unit: Unit{Kind: UnitUnitless}, Formula: Expression{Op: OpConstant, Constant: &nan}}
	issues := validateExpression("k", def.Formula)
	found := false
	for _, i := range issues {
		if i.Code == IssueNonFiniteInput {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected IssueNonFiniteInput, got %v", issues)
	}
}

func TestNonFiniteResult_Overflow(t *testing.T) {
	def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Binary(OpMultiply, Metric(MetricRef{Code: "a"}), Metric(MetricRef{Code: "b"}))}
	res := Calculate(onePeriodInput([]Definition{def}, []MetricValue{
		metricInput("a", "P1", math.MaxFloat64, true, currencyUnit("USD")),
		metricInput("b", "P1", math.MaxFloat64, true, Unit{Kind: UnitRatio}),
	}), Options{})
	kr := firstResult(t, res, "k")
	if kr.Value.Available || kr.Value.Reason != AvailabilityNonFiniteResult {
		t.Fatalf("got %+v", kr.Value)
	}
}
