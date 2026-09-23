package cashforecast

import "testing"

// TestScenarioTag_PreTaggedEventNeverLeaksIntoOtherScenariosOrBase is a
// regression test for a real bug found by code review: applyScenario and
// the base-scenario build path never filtered events by ScenarioTags, so
// a caller-pre-tagged CashFlowEvent (or one produced by AddEvent) applied
// to every scenario and the base case, contradicting
// CashFlowEvent.ScenarioTags's documented "empty means applies
// everywhere" semantics (which implies non-empty tags should scope the
// event to only the named scenario(s)).
func TestScenarioTag_PreTaggedEventNeverLeaksIntoOtherScenariosOrBase(t *testing.T) {
	in := Input{
		ForecastStartDate: testDate(t, "2025-01-06"),
		OpeningCash:       OpeningCash{Amount: 10000, Currency: "USD"},
		Events: []CashFlowEvent{
			// A caller-pre-tagged event, scoped only to DOWNSIDE — must
			// never appear in base or in UPSIDE.
			{ID: "downside-only", Date: testDate(t, "2025-01-08"), Amount: 5000, Direction: DirectionOutflow,
				Category: CategoryOtherOperatingOutflow, Basis: BasisScenario, ScenarioTags: []string{"DOWNSIDE"}},
		},
		Scenarios: []Scenario{
			{Label: "DOWNSIDE"},
			{Label: "UPSIDE"},
		},
	}
	result := Calculate(in, Options{})

	if !result.Available {
		t.Fatalf("expected Available=true, got %+v", result.Issues)
	}
	if result.BaseScenario.Weekly[0].Outflows.Total != 0 {
		t.Errorf("base scenario outflow = %v, want 0 (DOWNSIDE-tagged event must not appear in base)", result.BaseScenario.Weekly[0].Outflows.Total)
	}

	var downside, upside ScenarioResult
	for _, sc := range result.Scenarios {
		switch sc.Label {
		case "DOWNSIDE":
			downside = sc
		case "UPSIDE":
			upside = sc
		}
	}
	if downside.Weekly[0].Outflows.Total != 5000 {
		t.Errorf("DOWNSIDE scenario outflow = %v, want 5000 (tagged event should appear here)", downside.Weekly[0].Outflows.Total)
	}
	if upside.Weekly[0].Outflows.Total != 0 {
		t.Errorf("UPSIDE scenario outflow = %v, want 0 (DOWNSIDE-tagged event must not leak into UPSIDE)", upside.Weekly[0].Outflows.Total)
	}
}

// TestScenarioTag_AddEventTaggedEventOnlyAppliesToItsOwnScenario proves
// the AddEvent transform helper's automatic tagging (scenario.go's
// AddEvent) actually scopes the added event correctly end-to-end through
// Calculate, not just at the helper-function level.
func TestScenarioTag_AddEventTaggedEventOnlyAppliesToItsOwnScenario(t *testing.T) {
	in := Input{
		ForecastStartDate: testDate(t, "2025-01-06"),
		OpeningCash:       OpeningCash{Amount: 10000, Currency: "USD"},
		Scenarios: []Scenario{
			{Label: "DOWNSIDE", Transforms: []EventTransform{
				{Kind: TransformAddEvent, EventToAdd: CashFlowEvent{
					ID: "emergency-outflow", Date: testDate(t, "2025-01-08"), Amount: 8000,
					Direction: DirectionOutflow, Category: CategoryOtherOperatingOutflow, Basis: BasisScenario,
				}},
			}},
			{Label: "UPSIDE"},
		},
	}
	result := Calculate(in, Options{})

	if result.BaseScenario.Weekly[0].Outflows.Total != 0 {
		t.Errorf("base outflow = %v, want 0 (added-in-scenario event must not appear in base)", result.BaseScenario.Weekly[0].Outflows.Total)
	}

	var downside, upside ScenarioResult
	for _, sc := range result.Scenarios {
		switch sc.Label {
		case "DOWNSIDE":
			downside = sc
		case "UPSIDE":
			upside = sc
		}
	}
	if downside.Weekly[0].Outflows.Total != 8000 {
		t.Errorf("DOWNSIDE outflow = %v, want 8000", downside.Weekly[0].Outflows.Total)
	}
	if upside.Weekly[0].Outflows.Total != 0 {
		t.Errorf("UPSIDE outflow = %v, want 0 (must not see DOWNSIDE's added event)", upside.Weekly[0].Outflows.Total)
	}
}
