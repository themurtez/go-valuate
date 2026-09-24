package kpi

import (
	"encoding/json"
	"reflect"
	"strconv"
	"sync"
	"testing"
)

// TestImmutability_Input proves Calculate never mutates the caller's
// Input.Definitions/Metrics/Periods (including nested Expression/
// MetricValue/Period values) — task section 43.
func TestImmutability_Input(t *testing.T) {
	def := Definition{
		Code: "k", Unit: currencyUnit("USD"),
		Formula:        Binary(OpDivide, Metric(MetricRef{Code: "a"}), Metric(MetricRef{Code: "b"})),
		Target:         &TargetPolicy{Kind: TargetMinimum, Min: 10},
		ThresholdBands: []ThresholdBand{{Label: "LOW", Min: 0, Max: 100}},
	}
	metrics := []MetricValue{
		metricInput("a", "P1", 10, true, currencyUnit("USD")),
		metricInput("b", "P1", 2, true, currencyUnit("USD")),
	}
	periods := []Period{{Code: "P1", Sequence: 1}}

	in := Input{Definitions: []Definition{def}, Metrics: metrics, Periods: periods}
	before, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	_ = Calculate(in, Options{IncludeTrace: true, IncludeTrend: true, Dimensions: []DimensionKey{{"x": "y"}}})

	after, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(before) != string(after) {
		t.Fatalf("Input was mutated:\nbefore=%s\nafter=%s", before, after)
	}
}

// TestImmutability_DimensionKeyNotAliased proves a returned
// KPIResult.Dimensions is not the same backing map as any caller-supplied
// DimensionKey (a caller mutating one must never affect the other).
func TestImmutability_DimensionKeyNotAliased(t *testing.T) {
	dim := DimensionKey{"department": "service"}
	def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Metric(MetricRef{Code: "revenue"})}
	in := onePeriodInput([]Definition{def}, []MetricValue{
		{Code: "revenue", Period: "P1", Dimensions: dim, Value: 100, Available: true, Unit: currencyUnit("USD"), Aggregation: AggregationSum},
	})
	res := Calculate(in, Options{Dimensions: []DimensionKey{dim}})
	kr := firstResult(t, res, "k")
	// This should be the business-level result since dim wasn't matched
	// with an explicit reference -- but regardless, mutate dim afterward
	// and prove nothing in res changed.
	_ = kr
	dim["department"] = "MUTATED"
	for _, r := range res.KPIResults {
		if r.Dimensions["department"] == "MUTATED" {
			t.Fatalf("KPIResult.Dimensions was aliased to caller's DimensionKey")
		}
	}
}

// TestDeterminism_RepeatedCalls proves identical input produces
// byte-for-byte identical JSON across repeated Calculate calls — task
// section 42/43.
func TestDeterminism_RepeatedCalls(t *testing.T) {
	in := buildDeterminismFixture()
	opts := Options{IncludeTrace: true, IncludeTrend: true, Dimensions: []DimensionKey{{"department": "service"}, {"department": "sales"}}}

	var results [][]byte
	for i := 0; i < 5; i++ {
		res := Calculate(in, opts)
		b, err := json.Marshal(res)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		results = append(results, b)
	}
	for i := 1; i < len(results); i++ {
		if string(results[i]) != string(results[0]) {
			t.Fatalf("run %d differs from run 0", i)
		}
	}
}

// TestConcurrency_ParallelCalculate proves concurrent Calculate calls
// against identical immutable input are safe — task section 43. Run with
// -race.
func TestConcurrency_ParallelCalculate(t *testing.T) {
	in := buildDeterminismFixture()
	opts := Options{IncludeTrace: true, IncludeTrend: true, Dimensions: []DimensionKey{{"department": "service"}}}

	const goroutines = 20
	results := make([][]byte, goroutines)
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			res := Calculate(in, opts)
			b, err := json.Marshal(res)
			if err != nil {
				t.Errorf("marshal: %v", err)
				return
			}
			results[idx] = b
		}(i)
	}
	wg.Wait()

	for i := 1; i < goroutines; i++ {
		if string(results[i]) != string(results[0]) {
			t.Fatalf("goroutine %d produced a different result than goroutine 0", i)
		}
	}
}

func buildDeterminismFixture() Input {
	kpiA := Definition{Code: "revenue_per_fte", Unit: Unit{Kind: UnitCustom, CustomLabel: "USD_PER_COUNT"},
		Formula: Binary(OpDivide, Metric(MetricRef{Code: "revenue"}), Metric(MetricRef{Code: "fte"})),
		Target:  &TargetPolicy{Kind: TargetMinimum, Min: 50000}}
	kpiB := Definition{Code: "margin", Unit: Unit{Kind: UnitPercent},
		Formula:        Binary(OpPercent, Metric(MetricRef{Code: "profit"}), Metric(MetricRef{Code: "revenue"})),
		ThresholdBands: []ThresholdBand{{Label: "LOW", Min: 0, Max: 20}, {Label: "HIGH", Min: 20, Max: 100}}}
	kpiC := Definition{Code: "composite", Unit: Unit{Kind: UnitRatio}, Formula: Binary(OpDivide, KPIExpr(KPIRef{Code: "revenue_per_fte"}), Const(1))}

	return Input{
		Definitions: []Definition{kpiA, kpiB, kpiC},
		Metrics: []MetricValue{
			{Code: "revenue", Period: "2024", Value: 1000000, Available: true, Unit: currencyUnit("USD"), Aggregation: AggregationSum},
			{Code: "revenue", Period: "2025", Value: 1500000, Available: true, Unit: currencyUnit("USD"), Aggregation: AggregationSum},
			{Code: "fte", Period: "2024", Value: 10, Available: true, Unit: Unit{Kind: UnitCount}, Aggregation: AggregationSum},
			{Code: "fte", Period: "2025", Value: 12, Available: true, Unit: Unit{Kind: UnitCount}, Aggregation: AggregationSum},
			{Code: "profit", Period: "2024", Value: 200000, Available: true, Unit: currencyUnit("USD"), Aggregation: AggregationSum},
			{Code: "profit", Period: "2025", Value: 350000, Available: true, Unit: currencyUnit("USD"), Aggregation: AggregationSum},
			{Code: "revenue", Period: "2024", Dimensions: DimensionKey{"department": "service"}, Value: 400000, Available: true, Unit: currencyUnit("USD"), Aggregation: AggregationSum},
			{Code: "fte", Period: "2024", Dimensions: DimensionKey{"department": "service"}, Value: 4, Available: true, Unit: Unit{Kind: UnitCount}, Aggregation: AggregationSum},
			{Code: "revenue", Period: "2025", Dimensions: DimensionKey{"department": "service"}, Value: 500000, Available: true, Unit: currencyUnit("USD"), Aggregation: AggregationSum},
			{Code: "fte", Period: "2025", Dimensions: DimensionKey{"department": "service"}, Value: 5, Available: true, Unit: Unit{Kind: UnitCount}, Aggregation: AggregationSum},
		},
		Periods: []Period{
			{Code: "2024", Sequence: 1, FiscalYear: "FY24", PositionInYear: "ANNUAL"},
			{Code: "2025", Sequence: 2, FiscalYear: "FY25", PositionInYear: "ANNUAL"},
		},
	}
}

// --- Complexity limits ---

func TestComplexity_MaxExpressionDepth(t *testing.T) {
	// Build a chain of MaxExpressionDepth+10 nested NEGATE nodes.
	expr := Metric(MetricRef{Code: "a"})
	for i := 0; i < MaxExpressionDepth+10; i++ {
		expr = Unary(OpNegate, expr)
	}
	issues := validateExpression("k", expr)
	found := false
	for _, i := range issues {
		if i.Code == IssueExpressionTooComplex {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected IssueExpressionTooComplex for over-deep expression, got %v", issues)
	}
}

func TestComplexity_MaxExpressionNodes(t *testing.T) {
	// A wide SUM with far more nodes than MaxExpressionNodes.
	var args []Expression
	for i := 0; i < MaxExpressionNodes+100; i++ {
		args = append(args, Const(1))
	}
	expr := Variadic(OpSum, args...)
	issues := validateExpression("k", expr)
	found := false
	for _, i := range issues {
		if i.Code == IssueExpressionTooComplex {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected IssueExpressionTooComplex for over-wide expression, got %v", issues)
	}
}

func TestComplexity_ExpressionWithinLimitsIsFine(t *testing.T) {
	expr := Binary(OpAdd, Metric(MetricRef{Code: "a"}), Metric(MetricRef{Code: "b"}))
	issues := validateExpression("k", expr)
	if len(issues) != 0 {
		t.Fatalf("expected no issues for a small valid expression, got %v", issues)
	}
}

func TestComplexity_MaxDependencyDepth(t *testing.T) {
	var defs []Definition
	defs = append(defs, Definition{Code: "k0", Unit: currencyUnit("USD"), Formula: Metric(MetricRef{Code: "base"})})
	for i := 1; i <= MaxDependencyDepth+10; i++ {
		code := "k" + strconv.Itoa(i)
		prev := "k" + strconv.Itoa(i-1)
		defs = append(defs, Definition{Code: code, Unit: currencyUnit("USD"), Formula: KPIExpr(KPIRef{Code: prev})})
	}
	_, issues := buildDependencyGraph(toDefMap(defs), nil)
	found := false
	for _, i := range issues {
		if i.Code == IssueDependencyTooDeep {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected IssueDependencyTooDeep for over-deep dependency chain, got %v", issues)
	}
}

func toDefMap(defs []Definition) map[string]Definition {
	m := make(map[string]Definition, len(defs))
	for _, d := range defs {
		m[d.Code] = d
	}
	return m
}

// TestSafety_NoPanicOnEmptyInput proves Calculate never panics on the
// zero-value Input/Options.
func TestSafety_NoPanicOnEmptyInput(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Calculate panicked on empty input: %v", r)
		}
	}()
	res := Calculate(Input{}, Options{})
	if res.SchemaVersion != SchemaVersion {
		t.Fatalf("expected SchemaVersion to still be set on empty input")
	}
}

// TestSafety_DeepEqualAcrossRuns is a stronger structural check than the
// JSON-string determinism test: proves the Result VALUE (not just its
// JSON encoding) is identical across repeated runs.
func TestSafety_DeepEqualAcrossRuns(t *testing.T) {
	in := buildDeterminismFixture()
	opts := Options{IncludeTrace: true}
	r1 := Calculate(in, opts)
	r2 := Calculate(in, opts)
	if !reflect.DeepEqual(r1, r2) {
		t.Fatalf("results differ across identical runs")
	}
}
