package cashforecast

import "testing"

// TestScenarioValidation_OneInvalidScenarioDoesNotDropOthers is a
// regression test for a real bug found by code review: a single invalid
// scenario (e.g. an empty Label) used to gate the ENTIRE scenario loop,
// silently discarding every other, otherwise-valid, scenario from the
// Result — not just the bad one.
func TestScenarioValidation_OneInvalidScenarioDoesNotDropOthers(t *testing.T) {
	in := Input{
		ForecastStartDate: testDate(t, "2025-01-06"),
		OpeningCash:       OpeningCash{Amount: 10000, Currency: "USD"},
		Scenarios: []Scenario{
			{Label: "UPSIDE", Transforms: []EventTransform{{Kind: TransformScaleInflows, ScaleFactor: 1.1}}},
			{Label: ""}, // invalid: empty label.
		},
	}
	result := Calculate(in, Options{})

	if !result.Available {
		t.Fatalf("expected Available=true, got %+v", result.Issues)
	}
	if !hasIssueCode(result.Issues, IssueInvalidScenario) {
		t.Errorf("expected IssueInvalidScenario for the empty-label scenario, got %+v", result.Issues)
	}
	if len(result.Scenarios) != 1 {
		t.Fatalf("expected the valid UPSIDE scenario to still be computed, got %d scenarios: %+v", len(result.Scenarios), result.Scenarios)
	}
	if result.Scenarios[0].Label != "UPSIDE" {
		t.Errorf("expected the surviving scenario to be UPSIDE, got %s", result.Scenarios[0].Label)
	}
}

// TestScenarioValidation_DuplicateLabelDoesNotDropUnrelatedScenario
// proves the same fix for the duplicate-label case: a third, unrelated,
// valid scenario must survive even when two other scenarios collide.
func TestScenarioValidation_DuplicateLabelDoesNotDropUnrelatedScenario(t *testing.T) {
	in := Input{
		ForecastStartDate: testDate(t, "2025-01-06"),
		OpeningCash:       OpeningCash{Amount: 10000, Currency: "USD"},
		Scenarios: []Scenario{
			{Label: "DOWNSIDE"},
			{Label: "DOWNSIDE"}, // duplicate.
			{Label: "STRESS_TEST"},
		},
	}
	result := Calculate(in, Options{})

	if !hasIssueCode(result.Issues, IssueInvalidScenario) {
		t.Errorf("expected IssueInvalidScenario, got %+v", result.Issues)
	}
	// Only the FIRST occurrence of DOWNSIDE is valid (the second is the
	// duplicate rejection), plus STRESS_TEST — so 2 scenarios total.
	if len(result.Scenarios) != 2 {
		t.Fatalf("expected 2 surviving scenarios (first DOWNSIDE + STRESS_TEST), got %d: %+v", len(result.Scenarios), result.Scenarios)
	}
}

// TestScenarioValidation_BeyondHorizonEventInScenarioIsReported is a
// regression test for a real bug found by code review: an event a
// scenario transform pushes beyond the forecast horizon used to be
// silently discarded with no diagnostic (the scenario path discarded
// partitionEventsByRange's before/beyond return values via `_, _`),
// unlike the base path which always emits
// IssueEventBeforeForecastStart/IssueEventBeyondHorizon.
func TestScenarioValidation_BeyondHorizonEventInScenarioIsReported(t *testing.T) {
	in := Input{
		ForecastStartDate: testDate(t, "2025-01-06"),
		OpeningCash:       OpeningCash{Amount: 10000, Currency: "USD"},
		Events: []CashFlowEvent{
			// Lands in the last week of the default 13-week horizon
			// (2025-03-31..2025-04-06); delaying 30 days pushes it well
			// beyond the horizon end.
			{ID: "near-end", Date: testDate(t, "2025-04-01"), Amount: 1000, Direction: DirectionInflow,
				Category: CategoryCashSale, Basis: BasisKnown},
		},
		Scenarios: []Scenario{
			{Label: "DELAYED", Transforms: []EventTransform{
				{Kind: TransformDelayInflows, DelayDays: 30, Category: CategoryCashSale},
			}},
		},
	}
	result := Calculate(in, Options{})

	if !result.Available {
		t.Fatalf("expected Available=true, got %+v", result.Issues)
	}
	found := false
	for _, i := range result.Issues {
		if i.Code == IssueEventBeyondHorizon && i.SourceID == "DELAYED" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected IssueEventBeyondHorizon with SourceID=DELAYED, got %+v", result.Issues)
	}
	if len(result.Scenarios) != 1 || result.Scenarios[0].Weekly[len(result.Scenarios[0].Weekly)-1].Inflows.Total != 0 {
		t.Errorf("expected the delayed event to be excluded from the scenario's weekly totals")
	}
}
