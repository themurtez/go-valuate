package kpi

import "testing"

func multiPeriodInput(defs []Definition, metrics []MetricValue) Input {
	return Input{
		Definitions: defs,
		Metrics:     metrics,
		Periods: []Period{
			{Code: "2024", Sequence: 1, FiscalYear: "FY24", PositionInYear: "ANNUAL"},
			{Code: "2025", Sequence: 2, FiscalYear: "FY25", PositionInYear: "ANNUAL"},
			{Code: "2026", Sequence: 3, FiscalYear: "FY26", PositionInYear: "ANNUAL"},
		},
	}
}

func TestPeriods_MultiPeriod_IndependentEvaluation(t *testing.T) {
	def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Metric(MetricRef{Code: "revenue"})}
	res := Calculate(multiPeriodInput([]Definition{def}, []MetricValue{
		metricInput("revenue", "2024", 100, true, currencyUnit("USD")),
		metricInput("revenue", "2025", 200, true, currencyUnit("USD")),
		// 2026 deliberately has no revenue metric.
	}), Options{})

	got := map[string]KPIResult{}
	for _, kr := range res.KPIResults {
		got[kr.Period] = kr
	}
	if got["2024"].Value.Amount != 100 {
		t.Fatalf("2024 got %+v", got["2024"].Value)
	}
	if got["2025"].Value.Amount != 200 {
		t.Fatalf("2025 got %+v", got["2025"].Value)
	}
	if got["2026"].Value.Available {
		t.Fatalf("2026 should be unavailable (no forward-fill), got %+v", got["2026"].Value)
	}
}

func TestPeriods_PriorPeriod(t *testing.T) {
	def := Definition{Code: "k", Unit: Unit{Kind: UnitPercent}, Formula: Binary(OpPercentChange,
		Metric(MetricRef{Code: "revenue"}), Metric(MetricRef{Code: "revenue", Time: TimeRefPriorPeriod}))}
	res := Calculate(multiPeriodInput([]Definition{def}, []MetricValue{
		metricInput("revenue", "2024", 100, true, currencyUnit("USD")),
		metricInput("revenue", "2025", 150, true, currencyUnit("USD")),
	}), Options{Periods: []string{"2025"}})
	kr := firstResult(t, res, "k")
	if !kr.Value.Available || kr.Value.Amount != 50 {
		t.Fatalf("got %+v, want 50%% growth", kr.Value)
	}
}

func TestPeriods_NoPrior_Unavailable(t *testing.T) {
	def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Metric(MetricRef{Code: "revenue", Time: TimeRefPriorPeriod})}
	res := Calculate(multiPeriodInput([]Definition{def}, []MetricValue{
		metricInput("revenue", "2024", 100, true, currencyUnit("USD")),
	}), Options{Periods: []string{"2024"}})
	kr := firstResult(t, res, "k")
	if kr.Value.Available || kr.Value.Reason != AvailabilityPeriodUnavailable {
		t.Fatalf("first period has no prior, expected PERIOD_UNAVAILABLE, got %+v", kr.Value)
	}
}

func TestPeriods_Change_CurrentAndPrior(t *testing.T) {
	def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Metric(MetricRef{Code: "revenue"})}
	res := Calculate(multiPeriodInput([]Definition{def}, []MetricValue{
		metricInput("revenue", "2024", 100, true, currencyUnit("USD")),
		metricInput("revenue", "2025", 150, true, currencyUnit("USD")),
	}), Options{Periods: []string{"2025"}})
	kr := firstResult(t, res, "k")
	if !kr.Change.Current.Available || kr.Change.Current.Amount != 150 {
		t.Fatalf("current got %+v", kr.Change.Current)
	}
	if !kr.Change.Prior.Available || kr.Change.Prior.Amount != 100 {
		t.Fatalf("prior got %+v", kr.Change.Prior)
	}
	if !kr.Change.AbsoluteChange.Available || kr.Change.AbsoluteChange.Amount != 50 {
		t.Fatalf("absolute change got %+v", kr.Change.AbsoluteChange)
	}
}

func TestPeriods_PercentagePointChange(t *testing.T) {
	// 30% -> 35% is +5 percentage points, not +16.7% -- task section 13's
	// worked example.
	def := Definition{Code: "margin", Unit: Unit{Kind: UnitPercent}, Formula: Metric(MetricRef{Code: "margin_pct"})}
	res := Calculate(multiPeriodInput([]Definition{def}, []MetricValue{
		metricInput("margin_pct", "2024", 30, true, Unit{Kind: UnitPercent}),
		metricInput("margin_pct", "2025", 35, true, Unit{Kind: UnitPercent}),
	}), Options{Periods: []string{"2025"}})
	kr := firstResult(t, res, "margin")
	if !kr.Change.PercentagePointChange.Available || kr.Change.PercentagePointChange.Amount != 5 {
		t.Fatalf("percentage-point change got %+v, want 5", kr.Change.PercentagePointChange)
	}
	wantRelative := (35.0 - 30.0) / 30.0 * 100
	if !kr.Change.RelativeChangePercent.Available || abs(kr.Change.RelativeChangePercent.Amount-wantRelative) > 1e-9 {
		t.Fatalf("relative change got %+v, want %v", kr.Change.RelativeChangePercent, wantRelative)
	}
}

func TestPeriods_PriorYearSamePeriod(t *testing.T) {
	def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Metric(MetricRef{Code: "revenue", Time: TimeRefPriorYearSamePeriod})}
	in := Input{
		Definitions: []Definition{def},
		Metrics: []MetricValue{
			metricInput("revenue", "FY24-Q2", 100, true, currencyUnit("USD")),
			metricInput("revenue", "FY25-Q2", 150, true, currencyUnit("USD")),
		},
		Periods: []Period{
			{Code: "FY24-Q2", Sequence: 1, FiscalYear: "FY24", PositionInYear: "Q2"},
			{Code: "FY25-Q2", Sequence: 2, FiscalYear: "FY25", PositionInYear: "Q2"},
		},
	}
	res := Calculate(in, Options{Periods: []string{"FY25-Q2"}})
	kr := firstResult(t, res, "k")
	if !kr.Value.Available || kr.Value.Amount != 100 {
		t.Fatalf("got %+v, want prior-year-same-period value 100", kr.Value)
	}
}

// TestPeriods_PriorYearSamePeriod_NearestNotEarliest is a permanent
// regression test: resolveTimeTarget's TimeRefPriorYearSamePeriod branch
// was found (via a targeted probe test written after profiling flagged
// resolveTimeTarget as a performance hotspot) to return the EARLIEST
// fiscal year sharing PositionInYear before base, not the NEAREST one,
// whenever more than one prior fiscal year shared that position — a real
// correctness bug independent of the performance issue that prompted
// looking at this function at all. Locked here with 3 fiscal years
// sharing "Q2" so a regression back to forward-scan-return-first would
// be caught immediately.
func TestPeriods_PriorYearSamePeriod_NearestNotEarliest(t *testing.T) {
	def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Metric(MetricRef{Code: "revenue", Time: TimeRefPriorYearSamePeriod})}
	in := Input{
		Definitions: []Definition{def},
		Metrics: []MetricValue{
			metricInput("revenue", "FY23-Q2", 50, true, currencyUnit("USD")),
			metricInput("revenue", "FY24-Q2", 100, true, currencyUnit("USD")),
			metricInput("revenue", "FY25-Q2", 150, true, currencyUnit("USD")),
		},
		Periods: []Period{
			{Code: "FY23-Q2", Sequence: 1, FiscalYear: "FY23", PositionInYear: "Q2"},
			{Code: "FY24-Q2", Sequence: 2, FiscalYear: "FY24", PositionInYear: "Q2"},
			{Code: "FY25-Q2", Sequence: 3, FiscalYear: "FY25", PositionInYear: "Q2"},
		},
	}
	res := Calculate(in, Options{Periods: []string{"FY25-Q2"}})
	kr := firstResult(t, res, "k")
	if !kr.Value.Available || kr.Value.Amount != 100 {
		t.Fatalf("expected nearest prior-year-same-position (FY24-Q2=100), got %+v (a regression would return the earliest, FY23-Q2=50)", kr.Value)
	}
}

func TestPeriods_TrailingN(t *testing.T) {
	def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Metric(MetricRef{Code: "revenue", Time: TimeRefTrailingN, TrailingN: 3})}
	res := Calculate(multiPeriodInput([]Definition{def}, []MetricValue{
		metricInput("revenue", "2024", 100, true, currencyUnit("USD")),
		metricInput("revenue", "2025", 200, true, currencyUnit("USD")),
		metricInput("revenue", "2026", 300, true, currencyUnit("USD")),
	}), Options{Periods: []string{"2026"}})
	kr := firstResult(t, res, "k")
	if !kr.Value.Available || kr.Value.Amount != 600 {
		t.Fatalf("trailing-3 sum got %+v, want 600", kr.Value)
	}
}

func TestPeriods_TrailingN_InsufficientHistory(t *testing.T) {
	def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Metric(MetricRef{Code: "revenue", Time: TimeRefTrailingN, TrailingN: 3})}
	res := Calculate(multiPeriodInput([]Definition{def}, []MetricValue{
		metricInput("revenue", "2024", 100, true, currencyUnit("USD")),
	}), Options{Periods: []string{"2024"}})
	kr := firstResult(t, res, "k")
	if kr.Value.Available || kr.Value.Reason != AvailabilityPeriodUnavailable {
		t.Fatalf("expected PERIOD_UNAVAILABLE with insufficient history, got %+v", kr.Value)
	}
}

func TestPeriods_Ordering(t *testing.T) {
	def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Metric(MetricRef{Code: "revenue"})}
	res := Calculate(multiPeriodInput([]Definition{def}, []MetricValue{
		metricInput("revenue", "2024", 1, true, currencyUnit("USD")),
		metricInput("revenue", "2025", 2, true, currencyUnit("USD")),
		metricInput("revenue", "2026", 3, true, currencyUnit("USD")),
	}), Options{})
	if len(res.KPIResults) != 3 {
		t.Fatalf("expected 3 results, got %d", len(res.KPIResults))
	}
	wantOrder := []string{"2024", "2025", "2026"}
	for i, kr := range res.KPIResults {
		if kr.Period != wantOrder[i] {
			t.Fatalf("result[%d].Period = %q, want %q (chronological order)", i, kr.Period, wantOrder[i])
		}
	}
}

func TestPeriods_InvalidPeriod_EmptyCode(t *testing.T) {
	in := Input{
		Definitions: []Definition{{Code: "k", Unit: currencyUnit("USD"), Formula: Const(1)}},
		Periods:     []Period{{Code: "", Sequence: 1}},
	}
	res := Calculate(in, Options{})
	found := false
	for _, i := range res.EvaluationIssues {
		if i.Code == IssueInvalidPeriod {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected IssueInvalidPeriod for empty Code, got %v", res.EvaluationIssues)
	}
}

func TestPeriods_DuplicateSequence(t *testing.T) {
	in := Input{
		Definitions: []Definition{{Code: "k", Unit: currencyUnit("USD"), Formula: Const(1)}},
		Periods:     []Period{{Code: "A", Sequence: 1}, {Code: "B", Sequence: 1}},
	}
	res := Calculate(in, Options{})
	found := false
	for _, i := range res.EvaluationIssues {
		if i.Code == IssueInvalidPeriod {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected IssueInvalidPeriod for tied Sequence, got %v", res.EvaluationIssues)
	}
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
