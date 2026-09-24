package kpi

import (
	"strconv"
	"testing"
)

// Benchmarks — task sections 46/47/52. Laptop-safe foreground scales by
// default; a much larger stress scale is gated behind
// KPI_FULL_SCALE_BENCH=1 (see BenchmarkFullScale below), never launched
// automatically. Run with:
//
//	go test -bench=. -run='^$' ./analytics/kpi
//
// See the completion report for actual measured results/scaling analysis.

// buildKPIChain returns n Definitions forming a linear KPI-of-KPI chain
// k0 -> k1 -> ... -> k(n-1), rooted at a single "base" source metric.
func buildKPIChain(n int) []Definition {
	defs := make([]Definition, 0, n)
	defs = append(defs, Definition{Code: "k0", Unit: currencyUnit("USD"), Formula: Metric(MetricRef{Code: "base"})})
	for i := 1; i < n; i++ {
		code := "k" + strconv.Itoa(i)
		prev := "k" + strconv.Itoa(i-1)
		defs = append(defs, Definition{Code: code, Unit: currencyUnit("USD"), Formula: KPIExpr(KPIRef{Code: prev})})
	}
	return defs
}

// buildIndependentKPIs returns n independent (no KPI-to-KPI dependency)
// Definitions, each a simple METRIC leaf over its own distinct source
// metric — representative of a realistic "many unrelated KPI
// definitions" workload.
func buildIndependentKPIs(n int) ([]Definition, []MetricValue) {
	defs := make([]Definition, 0, n)
	metricsIn := make([]MetricValue, 0, n)
	for i := 0; i < n; i++ {
		code := "kpi" + strconv.Itoa(i)
		metricCode := "metric" + strconv.Itoa(i)
		defs = append(defs, Definition{Code: code, Unit: currencyUnit("USD"), Formula: Metric(MetricRef{Code: metricCode})})
		metricsIn = append(metricsIn, metricInput(metricCode, "P1", float64(i), true, currencyUnit("USD")))
	}
	return defs, metricsIn
}

func BenchmarkCalculate_1000Definitions(b *testing.B) {
	defs, metricsIn := buildIndependentKPIs(1000)
	in := onePeriodInput(defs, metricsIn)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Calculate(in, Options{})
	}
}

func BenchmarkCalculate_10000Definitions(b *testing.B) {
	defs, metricsIn := buildIndependentKPIs(10000)
	in := onePeriodInput(defs, metricsIn)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Calculate(in, Options{})
	}
}

// BenchmarkCalculate_10000Definitions_ScalingCheck runs the same
// workload at 1x/2x/4x scale in one benchmark to make superlinear
// scaling visible directly in `go test -bench` output (ns/op should
// roughly double when N doubles for a linear algorithm).
func BenchmarkCalculate_ScalingCheck(b *testing.B) {
	for _, n := range []int{1000, 2000, 4000, 8000} {
		defs, metricsIn := buildIndependentKPIs(n)
		in := onePeriodInput(defs, metricsIn)
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = Calculate(in, Options{})
			}
		})
	}
}

func BenchmarkCalculate_100000MetricValues(b *testing.B) {
	const numDefs = 100
	const periodsPerDef = 1000 // 100 * 1000 = 100,000 MetricValues
	defs := make([]Definition, 0, numDefs)
	var metricsIn []MetricValue
	var periods []Period
	for p := 0; p < periodsPerDef; p++ {
		periods = append(periods, Period{Code: "P" + strconv.Itoa(p), Sequence: p + 1})
	}
	for i := 0; i < numDefs; i++ {
		code := "kpi" + strconv.Itoa(i)
		metricCode := "metric" + strconv.Itoa(i)
		defs = append(defs, Definition{Code: code, Unit: currencyUnit("USD"), Formula: Metric(MetricRef{Code: metricCode})})
		for p := 0; p < periodsPerDef; p++ {
			metricsIn = append(metricsIn, metricInput(metricCode, "P"+strconv.Itoa(p), float64(p), true, currencyUnit("USD")))
		}
	}
	in := Input{Definitions: defs, Metrics: metricsIn, Periods: periods}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Calculate(in, Options{})
	}
}

func BenchmarkCalculate_100PeriodDimensionGroups(b *testing.B) {
	def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Metric(MetricRef{Code: "revenue"})}
	var periods []Period
	var metricsIn []MetricValue
	var dims []DimensionKey
	for p := 0; p < 10; p++ {
		periods = append(periods, Period{Code: "P" + strconv.Itoa(p), Sequence: p + 1})
	}
	for d := 0; d < 10; d++ {
		dim := DimensionKey{"segment": "seg" + strconv.Itoa(d)}
		dims = append(dims, dim)
		for p := 0; p < 10; p++ {
			metricsIn = append(metricsIn, MetricValue{Code: "revenue", Period: "P" + strconv.Itoa(p), Dimensions: dim, Value: float64(p), Available: true, Unit: currencyUnit("USD"), Aggregation: AggregationSum})
		}
	}
	in := Input{Definitions: []Definition{def}, Metrics: metricsIn, Periods: periods}
	opts := Options{Dimensions: dims} // 10 periods x 10 dims (+business-level) = 110 groups
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Calculate(in, opts)
	}
}

func BenchmarkDependencyDepth_100(b *testing.B)  { benchmarkDependencyChain(b, 100) }
func BenchmarkDependencyDepth_500(b *testing.B)  { benchmarkDependencyChain(b, 500) }
func BenchmarkDependencyDepth_1000(b *testing.B) { benchmarkDependencyChain(b, 1000) }

func benchmarkDependencyChain(b *testing.B, n int) {
	defs := buildKPIChain(n)
	in := onePeriodInput(defs, []MetricValue{metricInput("base", "P1", 1, true, currencyUnit("USD"))})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Calculate(in, Options{})
	}
}

func BenchmarkValidateExpression_Deep(b *testing.B) {
	expr := Metric(MetricRef{Code: "a"})
	for i := 0; i < MaxExpressionDepth-2; i++ {
		expr = Unary(OpNegate, expr)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validateExpression("k", expr)
	}
}

func BenchmarkTrace_Overhead(b *testing.B) {
	in := buildDeterminismFixture()
	b.Run("without_trace", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = Calculate(in, Options{})
		}
	})
	b.Run("with_trace", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = Calculate(in, Options{IncludeTrace: true})
		}
	})
}
