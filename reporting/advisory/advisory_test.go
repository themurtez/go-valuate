package advisory

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/cashforecast"
)

func TestBuild_EmptyInput(t *testing.T) {
	result := Build(Input{}, Policy{})

	if result.Status != BuildInvalid {
		t.Fatalf("Status = %v, want BuildInvalid", result.Status)
	}
	if len(result.Errors) == 0 {
		t.Fatal("expected at least one Error for empty Input")
	}
	if result.SchemaVersion != SchemaVersion {
		t.Errorf("SchemaVersion = %q, want %q", result.SchemaVersion, SchemaVersion)
	}
	if len(result.Sections) != 0 {
		t.Errorf("Sections should be empty for an INVALID build, got %d", len(result.Sections))
	}
}

func TestBuild_ZeroPolicyDoesNotPanic(t *testing.T) {
	// A zero-value Policy must never panic (nil map reads, division by
	// zero in resolveSourcedMetric's tolerance check, etc.).
	in := Input{
		Company: CompanyContext{CompanyName: "Test Co", CurrentPeriod: "2026-01"},
		Operating: OperatingInputs{
			CashForecast: cashforecast.Result{
				Available: true, ForecastStartDate: "2026-01-01", HorizonWeeks: 13,
				BaseScenario: cashforecast.ScenarioResult{
					Summary: cashforecast.LiquiditySummary{EndingCash: 50000, LowestCashBalance: 10000, LowestCashWeek: 6},
				},
			},
		},
	}

	result := Build(in, Policy{})

	if result.Status == BuildInvalid {
		t.Fatalf("Status = BuildInvalid, want COMPLETE or PARTIAL; Errors: %+v", result.Errors)
	}
	liq, ok := sectionByCode(result.Sections, SectionLiquidity)
	if !ok || liq.Availability != StatusAvailable {
		t.Fatalf("LIQUIDITY section not available: %+v", liq)
	}
	found := false
	for _, m := range liq.Metrics {
		if m.Code == metricCodeEndingCash && m.Value.Available && m.Value.Amount == 50000 {
			found = true
		}
	}
	if !found {
		t.Errorf("expected ending_cash metric == 50000 in LIQUIDITY section, got %+v", liq.Metrics)
	}
}

func TestBuild_Determinism(t *testing.T) {
	in := Input{
		Company: CompanyContext{CompanyName: "Test Co", CurrentPeriod: "2026-01"},
		Operating: OperatingInputs{
			CashForecast: cashforecast.Result{
				Available: true, ForecastStartDate: "2026-01-01", HorizonWeeks: 13,
				BaseScenario: cashforecast.ScenarioResult{
					Summary: cashforecast.LiquiditySummary{
						EndingCash: 50000, LowestCashBalance: 10000, LowestCashWeek: 6,
						ThresholdAvailable: true, MinimumCashThreshold: 25000, WeeksBelowMinimum: 3,
					},
				},
			},
		},
	}
	policy := ExamplePolicy()

	r1 := Build(in, policy)
	r2 := Build(in, policy)

	j1 := mustJSON(t, r1)
	j2 := mustJSON(t, r2)
	if j1 != j2 {
		t.Fatalf("Build is not deterministic:\n--- run 1 ---\n%s\n--- run 2 ---\n%s", j1, j2)
	}
}

func TestBuild_NeverMutatesInput(t *testing.T) {
	in := Input{
		Company: CompanyContext{CompanyName: "Test Co"},
		Operating: OperatingInputs{
			CashForecast: cashforecast.Result{
				Available: true, ForecastStartDate: "2026-01-01", HorizonWeeks: 13,
				BaseScenario: cashforecast.ScenarioResult{Summary: cashforecast.LiquiditySummary{EndingCash: 1000}},
			},
		},
		Periods: []PeriodInfo{{Code: "2026-01", Sequence: 1, IsCurrent: true}},
	}
	before := mustJSON(t, in)

	_ = Build(in, ExamplePolicy())

	after := mustJSON(t, in)
	if before != after {
		t.Fatalf("Build mutated its Input:\n--- before ---\n%s\n--- after ---\n%s", before, after)
	}
}

func TestResolvePolicy_FreshMapsNotSharedWithDefault(t *testing.T) {
	// Regression test for the portfolio/diagnostics.DefaultPolicy bug this
	// package was explicitly told to avoid repeating (task section 12).
	p1 := Policy{SeverityWeights: map[Severity]int{SeverityHigh: 1}}
	resolved1 := resolvePolicy(p1)
	resolved1.SeverityWeights[SeverityHigh] = 999

	p2 := Policy{SeverityWeights: map[Severity]int{SeverityHigh: 1}}
	resolved2 := resolvePolicy(p2)

	if resolved2.SeverityWeights[SeverityHigh] != 1 {
		t.Fatalf("mutating one resolvePolicy result's map affected another call's result: got %d, want 1", resolved2.SeverityWeights[SeverityHigh])
	}
}

func TestExamplePolicy_FreshEveryCall(t *testing.T) {
	p1 := ExamplePolicy()
	p1.Materiality.AbsoluteThreshold = 999999

	p2 := ExamplePolicy()
	if p2.Materiality.AbsoluteThreshold == 999999 {
		t.Fatal("ExamplePolicy returned a shared value; mutating one call's result affected another")
	}
}
