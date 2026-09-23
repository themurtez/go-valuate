package cashforecast

import "testing"

// TestSmoke_MinimalHealthyForecast exercises Calculate end-to-end with the
// smallest input that should produce a fully Available Result: an
// opening cash balance, a forecast start date, and one recurring rule.
// This is a wiring smoke test, not a correctness suite — see the other
// *_test.go files for full coverage.
func TestSmoke_MinimalHealthyForecast(t *testing.T) {
	in := Input{
		ForecastStartDate: testDate(t, "2025-01-06"),
		OpeningCash:       OpeningCash{Amount: 100000, Currency: "USD", AsOfDate: testDate(t, "2025-01-05")},
		RecurringRules: []RecurringRule{
			{ID: "rent", Amount: 5000, Direction: DirectionOutflow, Category: CategoryRent, Basis: BasisScheduled,
				StartDate: testDate(t, "2025-01-01"), Frequency: FrequencyMonthly},
		},
		Events: []CashFlowEvent{
			{ID: "sale-1", Date: testDate(t, "2025-01-10"), Amount: 20000, Direction: DirectionInflow,
				Category: CategoryCashSale, Basis: BasisKnown},
		},
	}
	result := Calculate(in, Options{})

	if !result.Available {
		t.Fatalf("expected Available=true, got Issues=%+v", result.Issues)
	}
	if result.HorizonWeeks != DefaultHorizonWeeks {
		t.Errorf("HorizonWeeks = %d, want %d", result.HorizonWeeks, DefaultHorizonWeeks)
	}
	if len(result.BaseScenario.Weekly) != DefaultHorizonWeeks {
		t.Fatalf("expected %d weeks, got %d", DefaultHorizonWeeks, len(result.BaseScenario.Weekly))
	}
	if result.BaseScenario.Weekly[0].OpeningCash != 100000 {
		t.Errorf("week 1 OpeningCash = %v, want 100000", result.BaseScenario.Weekly[0].OpeningCash)
	}
	if HasErrors(result.Issues) {
		t.Errorf("unexpected error-level issues: %+v", result.Issues)
	}
}

func TestSmoke_MissingForecastStartDate(t *testing.T) {
	result := Calculate(Input{}, Options{})
	if result.Available {
		t.Fatalf("expected Available=false for zero ForecastStartDate")
	}
	if !hasIssueCode(result.Issues, IssueInvalidForecastStart) {
		t.Errorf("expected IssueInvalidForecastStart, got %+v", result.Issues)
	}
}

func hasIssueCode(issues []Issue, code IssueCode) bool {
	for _, i := range issues {
		if i.Code == code {
			return true
		}
	}
	return false
}

func TestSmoke_ScenarioProducesDelta(t *testing.T) {
	// Week 1 spans 2025-01-06..2025-01-12, week 2 spans 01-13..01-19,
	// week 3 spans 01-20..01-26 (CALLER_START_DATE alignment). The base
	// event lands in week 2; delaying it 14 days pushes it into week 4.
	in := Input{
		ForecastStartDate: testDate(t, "2025-01-06"),
		OpeningCash:       OpeningCash{Amount: 50000, Currency: "USD"},
		Events: []CashFlowEvent{
			{ID: "collect-1", Date: testDate(t, "2025-01-15"), Amount: 30000, Direction: DirectionInflow,
				Category: CategoryARCollection, Basis: BasisAssumed},
		},
		Scenarios: []Scenario{
			{Label: "DOWNSIDE", Transforms: []EventTransform{
				{Kind: TransformDelayInflows, DelayDays: 14, Category: CategoryARCollection},
			}},
		},
	}
	result := Calculate(in, Options{})
	if !result.Available {
		t.Fatalf("expected Available=true, got Issues=%+v", result.Issues)
	}
	if len(result.Scenarios) != 1 {
		t.Fatalf("expected 1 scenario, got %d", len(result.Scenarios))
	}
	downside := result.Scenarios[0]
	if !downside.DeltaVsBase.Available {
		t.Fatalf("expected DeltaVsBase.Available=true")
	}

	// Base scenario: $30k inflow lands in week 2 (index 1).
	if result.BaseScenario.Weekly[0].Inflows.Total != 0 {
		t.Errorf("base week 1 inflow = %v, want 0 (event lands in week 2)", result.BaseScenario.Weekly[0].Inflows.Total)
	}
	if result.BaseScenario.Weekly[1].Inflows.Total != 30000 {
		t.Errorf("base week 2 inflow = %v, want 30000", result.BaseScenario.Weekly[1].Inflows.Total)
	}

	// Downside scenario: same event delayed 14 days lands in week 4
	// (index 3), so week 2 shows no inflow.
	if downside.Weekly[1].Inflows.Total != 0 {
		t.Errorf("downside week 2 inflow = %v, want 0 (delayed into week 4)", downside.Weekly[1].Inflows.Total)
	}
	if downside.Weekly[3].Inflows.Total != 30000 {
		t.Errorf("downside week 4 inflow = %v, want 30000", downside.Weekly[3].Inflows.Total)
	}

	// Ending cash should differ between base and downside starting at
	// week 2 (base has collected, downside hasn't yet).
	if downside.Weekly[1].EndingCash >= result.BaseScenario.Weekly[1].EndingCash {
		t.Errorf("expected downside week 2 ending cash to lag base: downside=%v base=%v",
			downside.Weekly[1].EndingCash, result.BaseScenario.Weekly[1].EndingCash)
	}

	// By the end of the horizon (both events have landed by week 13),
	// ending cash should converge back to equal.
	lastIdx := len(result.BaseScenario.Weekly) - 1
	if downside.Weekly[lastIdx].EndingCash != result.BaseScenario.Weekly[lastIdx].EndingCash {
		t.Errorf("expected ending cash to converge by end of horizon: downside=%v base=%v",
			downside.Weekly[lastIdx].EndingCash, result.BaseScenario.Weekly[lastIdx].EndingCash)
	}
}
