package kpi

import "testing"

func TestTrend_AbsoluteAndRelativeChange(t *testing.T) {
	def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Metric(MetricRef{Code: "revenue"})}
	res := Calculate(multiPeriodInput([]Definition{def}, []MetricValue{
		metricInput("revenue", "2024", 100, true, currencyUnit("USD")),
		metricInput("revenue", "2025", 150, true, currencyUnit("USD")),
		metricInput("revenue", "2026", 180, true, currencyUnit("USD")),
	}), Options{IncludeTrend: true, Periods: []string{"2026"}})
	kr := firstResult(t, res, "k")
	if kr.Trend == nil {
		t.Fatalf("expected Trend to be populated")
	}
	if len(kr.Trend.Points) != 3 {
		t.Fatalf("expected 3 points, got %d", len(kr.Trend.Points))
	}
	if len(kr.Trend.Adjacent) != 2 {
		t.Fatalf("expected 2 adjacent changes, got %d", len(kr.Trend.Adjacent))
	}
	if kr.Trend.Adjacent[0].AbsoluteChange.Amount != 50 {
		t.Fatalf("first adjacent change got %+v, want 50", kr.Trend.Adjacent[0].AbsoluteChange)
	}
	if kr.Trend.Adjacent[1].AbsoluteChange.Amount != 30 {
		t.Fatalf("second adjacent change got %+v, want 30", kr.Trend.Adjacent[1].AbsoluteChange)
	}
}

func TestTrend_FirstVsLast(t *testing.T) {
	def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Metric(MetricRef{Code: "revenue"})}
	res := Calculate(multiPeriodInput([]Definition{def}, []MetricValue{
		metricInput("revenue", "2024", 100, true, currencyUnit("USD")),
		metricInput("revenue", "2025", 150, true, currencyUnit("USD")),
		metricInput("revenue", "2026", 180, true, currencyUnit("USD")),
	}), Options{IncludeTrend: true, Periods: []string{"2026"}})
	kr := firstResult(t, res, "k")
	if kr.Trend.FirstVsLast.AbsoluteChange.Amount != 80 {
		t.Fatalf("first-vs-last got %+v, want 80 (100 -> 180, skipping the middle point)", kr.Trend.FirstVsLast.AbsoluteChange)
	}
}

func TestTrend_Direction(t *testing.T) {
	cases := []struct {
		name      string
		values    []float64
		tolerance float64
		want      TrendDirection
	}{
		{"increasing", []float64{100, 200}, 0, TrendIncreasing},
		{"decreasing", []float64{200, 100}, 0, TrendDecreasing},
		{"stable_exact", []float64{100, 100}, 0, TrendStable},
		{"stable_within_tolerance", []float64{100, 105}, 10, TrendStable},
		{"increasing_beyond_tolerance", []float64{100, 120}, 10, TrendIncreasing},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Metric(MetricRef{Code: "revenue"})}
			res := Calculate(multiPeriodInput([]Definition{def}, []MetricValue{
				metricInput("revenue", "2024", c.values[0], true, currencyUnit("USD")),
				metricInput("revenue", "2025", c.values[1], true, currencyUnit("USD")),
			}), Options{IncludeTrend: true, TrendStabilityTolerance: c.tolerance, Periods: []string{"2025"}})
			kr := firstResult(t, res, "k")
			if kr.Trend.Direction != c.want {
				t.Fatalf("got %v, want %v", kr.Trend.Direction, c.want)
			}
		})
	}
}

func TestTrend_UnavailableWithOnePoint(t *testing.T) {
	def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Metric(MetricRef{Code: "revenue"})}
	res := Calculate(multiPeriodInput([]Definition{def}, []MetricValue{
		metricInput("revenue", "2024", 100, true, currencyUnit("USD")),
	}), Options{IncludeTrend: true, Periods: []string{"2024"}})
	kr := firstResult(t, res, "k")
	if kr.Trend.Direction != TrendUnavailable {
		t.Fatalf("single-point trend direction should be UNAVAILABLE, got %v", kr.Trend.Direction)
	}
}

// --- Trace / provenance ---

func TestTrace_NotPopulatedByDefault(t *testing.T) {
	def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Metric(MetricRef{Code: "revenue"})}
	res := Calculate(onePeriodInput([]Definition{def}, []MetricValue{
		metricInput("revenue", "P1", 100, true, currencyUnit("USD")),
	}), Options{})
	kr := firstResult(t, res, "k")
	if kr.Trace != nil {
		t.Fatalf("Trace should be nil unless IncludeTrace is set")
	}
}

func TestTrace_MetricSources(t *testing.T) {
	def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Binary(OpAdd, Metric(MetricRef{Code: "a"}), Metric(MetricRef{Code: "b"}))}
	res := Calculate(onePeriodInput([]Definition{def}, []MetricValue{
		metricInput("a", "P1", 10, true, currencyUnit("USD")),
		metricInput("b", "P1", 20, true, currencyUnit("USD")),
	}), Options{IncludeTrace: true})
	kr := firstResult(t, res, "k")
	if kr.Trace == nil {
		t.Fatalf("expected Trace to be populated")
	}
	if kr.Trace.Operator != OpAdd {
		t.Fatalf("got operator %v", kr.Trace.Operator)
	}
	if len(kr.Trace.Args) != 2 {
		t.Fatalf("expected 2 trace args, got %d: %+v", len(kr.Trace.Args), kr.Trace.Args)
	}
	if kr.Trace.Result.Amount != 30 {
		t.Fatalf("trace result got %+v", kr.Trace.Result)
	}
}

func TestTrace_KPIDependencies(t *testing.T) {
	inner := Definition{Code: "inner", Unit: currencyUnit("USD"), Formula: Metric(MetricRef{Code: "revenue"})}
	outer := Definition{Code: "outer", Unit: currencyUnit("USD"), Formula: KPIExpr(KPIRef{Code: "inner"})}
	res := Calculate(onePeriodInput([]Definition{inner, outer}, []MetricValue{
		metricInput("revenue", "P1", 500, true, currencyUnit("USD")),
	}), Options{IncludeTrace: true})
	kr := firstResult(t, res, "outer")
	if kr.Trace == nil || kr.Trace.Operator != OpKPI {
		t.Fatalf("got trace %+v", kr.Trace)
	}
}

func TestProvenance_SourceMetricsAndDependencies(t *testing.T) {
	inner := Definition{Code: "inner", Unit: currencyUnit("USD"), Formula: Metric(MetricRef{Code: "revenue"})}
	outer := Definition{Code: "outer", Unit: currencyUnit("USD"), Formula: Binary(OpAdd, KPIExpr(KPIRef{Code: "inner"}), Metric(MetricRef{Code: "bonus"}))}
	res := Calculate(onePeriodInput([]Definition{inner, outer}, []MetricValue{
		metricInput("revenue", "P1", 500, true, currencyUnit("USD")),
		metricInput("bonus", "P1", 100, true, currencyUnit("USD")),
	}), Options{}) // no IncludeTrace -- provenance is always populated
	kr := firstResult(t, res, "outer")
	if len(kr.Provenance.SourceMetricCodes) != 2 {
		t.Fatalf("expected 2 source metrics (revenue, bonus), got %v", kr.Provenance.SourceMetricCodes)
	}
	if len(kr.Provenance.DependencyKPICodes) != 1 || kr.Provenance.DependencyKPICodes[0] != "inner" {
		t.Fatalf("expected [inner], got %v", kr.Provenance.DependencyKPICodes)
	}
}

func TestProvenance_SourceRefs(t *testing.T) {
	def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Metric(MetricRef{Code: "revenue"})}
	in := onePeriodInput([]Definition{def}, []MetricValue{
		{Code: "revenue", Period: "P1", Value: 100, Available: true, Unit: currencyUnit("USD"), Aggregation: AggregationSum, SourceRef: "ERP-INV-001"},
	})
	res := Calculate(in, Options{})
	kr := firstResult(t, res, "k")
	if len(kr.Provenance.SourceRefs) != 1 || kr.Provenance.SourceRefs[0] != "ERP-INV-001" {
		t.Fatalf("got %v", kr.Provenance.SourceRefs)
	}
}
