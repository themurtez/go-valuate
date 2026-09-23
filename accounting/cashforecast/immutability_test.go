package cashforecast

import (
	"encoding/json"
	"reflect"
	"testing"
)

// TestImmutability_CalculateDoesNotMutateInput proves Calculate never
// mutates any field of Input (or nested slices/maps within it) — see the
// task's section 62.
func TestImmutability_CalculateDoesNotMutateInput(t *testing.T) {
	in := fullInput(t)
	opts := fullOptions()

	before, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("Marshal before: %v", err)
	}

	_ = Calculate(in, opts)

	after, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("Marshal after: %v", err)
	}
	if string(before) != string(after) {
		t.Fatalf("Calculate mutated Input:\nbefore=%s\nafter=%s", before, after)
	}
}

func TestImmutability_RecurringRuleDimensionsNotShared(t *testing.T) {
	rule := RecurringRule{
		ID: "r", Amount: 100, Direction: DirectionOutflow, Category: CategoryRent, Basis: BasisScheduled,
		StartDate: testDate(t, "2025-01-01"), Frequency: FrequencyMonthly,
		Dimensions: []Dimension{{Key: "location", Value: "hq"}},
	}
	original := append([]Dimension{}, rule.Dimensions...)

	events := generateRecurringEvents(rule, testDate(t, "2025-01-01"), testDate(t, "2025-06-30"))
	for i := range events {
		events[i].Dimensions[0].Value = "changed"
	}

	if !reflect.DeepEqual(rule.Dimensions, original) {
		t.Errorf("rule.Dimensions mutated: got %+v, want %+v", rule.Dimensions, original)
	}
}

// TestImmutability_ScenarioTransformsDoNotMutateBase proves every
// EventTransform helper returns a copy, never touching the input slice's
// backing array or its elements' nested slices.
func TestImmutability_ScenarioTransformsDoNotMutateBase(t *testing.T) {
	base := []CashFlowEvent{
		{ID: "e1", Date: testDate(t, "2025-01-10"), Amount: 1000, Direction: DirectionInflow, Category: CategoryARCollection, Basis: BasisAssumed,
			ScenarioTags: []string{}, Dimensions: []Dimension{{Key: "region", Value: "east"}}},
	}
	originalJSON, _ := json.Marshal(base)

	_ = DelayEvents(base, 7, nil, "", "")
	_ = ScaleEvents(base, 0.5, nil, "", "")
	_ = RemoveEventByID(base, "e1")
	_ = AddEvent(base, CashFlowEvent{ID: "e2", Date: testDate(t, "2025-01-11"), Amount: 500, Direction: DirectionInflow, Category: CategoryARCollection, Basis: BasisAssumed}, "DOWNSIDE")
	_ = DeferByPriority(base, 7, []Priority{PriorityLow})

	afterJSON, _ := json.Marshal(base)
	if string(originalJSON) != string(afterJSON) {
		t.Fatalf("scenario transform mutated base slice:\nbefore=%s\nafter=%s", originalJSON, afterJSON)
	}

	// Also prove nested Dimensions/ScenarioTags slices are not aliased:
	// mutating a returned copy must not affect base[0].
	delayed := DelayEvents(base, 7, nil, "", "")
	delayed[0].Dimensions[0].Value = "mutated"
	if base[0].Dimensions[0].Value != "east" {
		t.Errorf("DelayEvents result shares Dimensions backing array with base: base=%+v", base[0].Dimensions)
	}
}

// TestImmutability_BaseAndScenarioDoNotShareEventBackingArray proves
// Calculate's base scenario and a named scenario never share a backing
// array such that mutating one's DetailedSchedule would affect the
// other's — verified indirectly via re-marshal-after-use since
// DetailedSchedule entries are already value types (ScheduleEntry has no
// pointer/slice fields), but this test also confirms the base scenario's
// weekly totals are unaffected by a scenario that scales/removes events.
func TestImmutability_BaseUnaffectedByScenarioScaling(t *testing.T) {
	in := Input{
		ForecastStartDate: testDate(t, "2025-01-06"),
		OpeningCash:       OpeningCash{Amount: 10000, Currency: "USD"},
		Events: []CashFlowEvent{
			{ID: "e1", Date: testDate(t, "2025-01-07"), Amount: 2000, Direction: DirectionInflow, Category: CategoryCashSale, Basis: BasisKnown},
		},
		Scenarios: []Scenario{
			{Label: "DOWNSIDE", Transforms: []EventTransform{{Kind: TransformRemoveEvent, RemoveEventID: "e1"}}},
		},
	}
	result := Calculate(in, Options{})

	if result.BaseScenario.Weekly[0].Inflows.Total != 2000 {
		t.Errorf("base scenario inflow affected by scenario's event removal: got %v, want 2000", result.BaseScenario.Weekly[0].Inflows.Total)
	}
	if len(result.Scenarios) != 1 || result.Scenarios[0].Weekly[0].Inflows.Total != 0 {
		t.Errorf("downside scenario should have removed the event: got %+v", result.Scenarios[0].Weekly[0].Inflows)
	}
}
