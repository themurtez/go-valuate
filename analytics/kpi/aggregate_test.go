package kpi

import "testing"

func TestAggregation_Sum(t *testing.T) {
	vals := []MetricValue{
		{Value: 10, Available: true}, {Value: 20, Available: true}, {Value: 30, Available: true},
	}
	got := aggregateMetricValues(vals, AggregationSum)
	if !got.Available || got.Amount != 60 {
		t.Fatalf("got %+v", got)
	}
}

func TestAggregation_Average(t *testing.T) {
	vals := []MetricValue{{Value: 10, Available: true}, {Value: 20, Available: true}}
	got := aggregateMetricValues(vals, AggregationAverage)
	if !got.Available || got.Amount != 15 {
		t.Fatalf("got %+v", got)
	}
}

func TestAggregation_WeightedAverage_RequiresExplicitOperator(t *testing.T) {
	// A metric tagged AggregationWeightedAverage rolled up via the bare
	// aggregation path (not the explicit WEIGHTED_AVERAGE Expression
	// operator) has no weight source -- unavailable, never a silent
	// plain-average fallback (task section 37).
	vals := []MetricValue{{Value: 10, Available: true}, {Value: 90, Available: true}}
	got := aggregateMetricValues(vals, AggregationWeightedAverage)
	if got.Available {
		t.Fatalf("expected unavailable (no weight source), got %+v", got)
	}
}

func TestAggregation_NotAggregatable(t *testing.T) {
	// A margin percentage is not automatically averaged unless explicitly
	// configured -- task section 37/17.
	vals := []MetricValue{{Value: 25, Available: true}, {Value: 35, Available: true}}
	got := aggregateMetricValues(vals, AggregationNotAggregatable)
	if got.Available {
		t.Fatalf("NOT_AGGREGATABLE should never silently average, got %+v", got)
	}
}

func TestAggregation_DefaultIsNotAggregatable(t *testing.T) {
	// An empty/unrecognized Aggregation must resolve to
	// NOT_AGGREGATABLE, never SUM -- task section 17's explicit
	// instruction.
	if got := resolvedAggregation(""); got != AggregationNotAggregatable {
		t.Fatalf("empty Aggregation resolved to %q, want NOT_AGGREGATABLE", got)
	}
	if got := resolvedAggregation("BOGUS"); got != AggregationNotAggregatable {
		t.Fatalf("unrecognized Aggregation resolved to %q, want NOT_AGGREGATABLE", got)
	}
}

// TestAggregation_RecomputeRatioFromTotals proves the recommended pattern
// (task section 37's "prove... a margin percentage is not automatically
// averaged unless explicitly configured" plus 17's "prefer recomputing
// ratios from underlying totals rather than averaging percentages")
// actually works end to end: SUM the numerator/denominator, then DIVIDE,
// rather than averaging a pre-computed margin.
func TestAggregation_RecomputeRatioFromTotals(t *testing.T) {
	def := Definition{Code: "blended_margin", Unit: Unit{Kind: UnitPercent}, Formula: Binary(OpPercent,
		Metric(MetricRef{Code: "profit", Time: TimeRefTrailingN, TrailingN: 2}),
		Metric(MetricRef{Code: "revenue", Time: TimeRefTrailingN, TrailingN: 2}))}
	res := Calculate(multiPeriodInput([]Definition{def}, []MetricValue{
		metricInput("revenue", "2024", 100, true, currencyUnit("USD")),
		metricInput("profit", "2024", 50, true, currencyUnit("USD")), // 50%
		metricInput("revenue", "2025", 300, true, currencyUnit("USD")),
		metricInput("profit", "2025", 60, true, currencyUnit("USD")), // 20%
	}), Options{Periods: []string{"2025"}})
	kr := firstResult(t, res, "blended_margin")
	// Blended: (50+60)/(100+300)*100 = 27.5%, NOT (50+20)/2 = 35%.
	want := (50.0 + 60.0) / (100.0 + 300.0) * 100
	if !kr.Value.Available || abs(kr.Value.Amount-want) > 1e-9 {
		t.Fatalf("got %+v, want %v (recomputed from totals, not averaged percentages)", kr.Value, want)
	}
}

func TestAggregation_TrailingN_UsesDeclaredRule(t *testing.T) {
	def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Metric(MetricRef{Code: "revenue", Time: TimeRefTrailingN, TrailingN: 2})}
	res := Calculate(multiPeriodInput([]Definition{def}, []MetricValue{
		{Code: "revenue", Period: "2024", Value: 100, Available: true, Unit: currencyUnit("USD"), Aggregation: AggregationSum},
		{Code: "revenue", Period: "2025", Value: 200, Available: true, Unit: currencyUnit("USD"), Aggregation: AggregationSum},
	}), Options{Periods: []string{"2025"}})
	kr := firstResult(t, res, "k")
	if !kr.Value.Available || kr.Value.Amount != 300 {
		t.Fatalf("got %+v, want SUM-aggregated 300", kr.Value)
	}
}
