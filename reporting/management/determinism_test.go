package management

import (
	"encoding/json"
	"testing"
)

// TestCalculate_Deterministic proves repeated execution against identical
// input returns byte-for-byte identical output.
func TestCalculate_Deterministic(t *testing.T) {
	in := fullFixture()

	first, err := json.Marshal(Calculate(in))
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	for i := 0; i < 10; i++ {
		got, err := json.Marshal(Calculate(in))
		if err != nil {
			t.Fatalf("run %d: json.Marshal failed: %v", i, err)
		}
		if string(got) != string(first) {
			t.Fatalf("run %d: Calculate output differs from the first run", i)
		}
	}
}

// TestCalculate_DeterministicPartialInput proves determinism also holds
// for a partially-populated Input, not just the full fixture.
func TestCalculate_DeterministicPartialInput(t *testing.T) {
	in := Input{Metrics: fullFixture().Metrics, PeriodMeta: fullFixture().PeriodMeta}

	first, err := json.Marshal(Calculate(in))
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	for i := 0; i < 10; i++ {
		got, err := json.Marshal(Calculate(in))
		if err != nil {
			t.Fatalf("run %d: json.Marshal failed: %v", i, err)
		}
		if string(got) != string(first) {
			t.Fatalf("run %d: Calculate output differs from the first run", i)
		}
	}
}

// TestCalculate_NeverMutatesInput proves Calculate does not mutate any
// slice/map field of its Input.
func TestCalculate_NeverMutatesInput(t *testing.T) {
	in := fullFixture()
	before, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("json.Marshal(in) failed: %v", err)
	}

	_ = Calculate(in)

	after, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("json.Marshal(in) after Calculate failed: %v", err)
	}
	if string(before) != string(after) {
		t.Fatalf("Calculate mutated its Input:\nbefore=%s\nafter=%s", before, after)
	}
}

// TestCalculate_SectionOrder proves Report's fields reflect sectionOrder's
// fixed order and every Section is populated when the full fixture is
// supplied.
func TestCalculate_SectionOrder(t *testing.T) {
	report := Calculate(fullFixture())
	if !report.Available {
		t.Fatalf("expected Available, errors=%+v", report.Errors)
	}

	sections := []struct {
		code      SectionCode
		available bool
	}{
		{SectionExecutiveSummary, report.ExecutiveSummary.Available},
		{SectionHistoricalSeries, report.HistoricalSeries.Available},
		{SectionProfitabilitySeries, report.ProfitabilitySeries.Available},
		{SectionLiquidityLeverageSeries, report.LiquidityLeverageSeries.Available},
		{SectionCashFlowSeries, report.CashFlowSeries.Available},
		{SectionWorkingCapitalSeries, report.WorkingCapitalSeries.Available},
		{SectionVarianceTables, report.VarianceTables.Available},
		{SectionForecastTables, report.ForecastTables.Available},
		{SectionTopIssues, report.TopIssues.Available},
		{SectionChartSeries, report.ChartSeries.Available},
	}
	if len(sections) != len(sectionOrder) {
		t.Fatalf("test's section list (%d) is out of sync with sectionOrder (%d)", len(sections), len(sectionOrder))
	}
	for i, s := range sections {
		if sectionOrder[i] != s.code {
			t.Fatalf("sectionOrder[%d] = %s, want %s", i, sectionOrder[i], s.code)
		}
		if !s.available {
			t.Errorf("section %s: expected Available with the full fixture", s.code)
		}
	}
}
