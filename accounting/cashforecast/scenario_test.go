package cashforecast

import "testing"

func TestScenario_AccelerateEvents(t *testing.T) {
	events := []CashFlowEvent{
		{ID: "e1", Date: testDate(t, "2025-01-20"), Amount: 100, Direction: DirectionOutflow, Category: CategoryAPPayment, Basis: BasisKnown},
	}
	got := AccelerateEvents(events, 7, nil, "", "")
	if got[0].Date.Format("2006-01-02") != "2025-01-13" {
		t.Errorf("accelerated date = %s, want 2025-01-13", got[0].Date.Format("2006-01-02"))
	}
	// Original must be unaffected.
	if events[0].Date.Format("2006-01-02") != "2025-01-20" {
		t.Errorf("AccelerateEvents mutated original: %v", events[0].Date)
	}
}

func TestScenario_AccelerateOutflowsTransformDispatch(t *testing.T) {
	in := Input{
		ForecastStartDate: testDate(t, "2025-01-06"),
		OpeningCash:       OpeningCash{Amount: 100000, Currency: "USD"},
		Events: []CashFlowEvent{
			{ID: "bill1", Date: testDate(t, "2025-01-25"), Amount: 5000, Direction: DirectionOutflow, Category: CategoryAPPayment, Basis: BasisKnown},
		},
		Scenarios: []Scenario{
			{Label: "ACCELERATED", Transforms: []EventTransform{
				{Kind: TransformAccelerateOutflows, DelayDays: 14, Category: CategoryAPPayment},
			}},
		},
	}
	result := Calculate(in, Options{})
	if !result.Available {
		t.Fatalf("expected Available=true, got %+v", result.Issues)
	}
	// Original date Jan 25 is week 4 (Jan 6 + 3*7 = Jan 27 start... let's
	// just confirm week 4 has 0 and week 2 has the accelerated outflow,
	// since Jan 25 - 14 = Jan 11 (week 1).
	accelerated := result.Scenarios[0]
	if accelerated.Weekly[0].Outflows.Total != 5000 {
		t.Errorf("week 1 accelerated outflow = %v, want 5000 (Jan 25 - 14d = Jan 11)", accelerated.Weekly[0].Outflows.Total)
	}
}

func TestScenario_ScaleCategoryTransformDispatch(t *testing.T) {
	in := Input{
		ForecastStartDate: testDate(t, "2025-01-06"),
		OpeningCash:       OpeningCash{Amount: 1000, Currency: "USD"},
		Events: []CashFlowEvent{
			{ID: "e1", Date: testDate(t, "2025-01-08"), Amount: 1000, Direction: DirectionInflow, Category: CategoryCashSale, Basis: BasisKnown},
		},
		Scenarios: []Scenario{
			{Label: "SCALED", Transforms: []EventTransform{
				{Kind: TransformScaleCategory, ScaleFactor: 0.75, Category: CategoryCashSale, Direction: DirectionInflow},
			}},
		},
	}
	result := Calculate(in, Options{})
	if result.Scenarios[0].Weekly[0].Inflows.Total != 750 {
		t.Errorf("scaled inflow = %v, want 750", result.Scenarios[0].Weekly[0].Inflows.Total)
	}
}

func TestScenario_DeferByPriorityTransformDispatch(t *testing.T) {
	in := Input{
		ForecastStartDate: testDate(t, "2025-01-06"),
		OpeningCash:       OpeningCash{Amount: 100000, Currency: "USD"},
		Events: []CashFlowEvent{
			{ID: "low1", Date: testDate(t, "2025-01-08"), Amount: 2000, Direction: DirectionOutflow, Category: CategoryOtherOperatingOutflow, Basis: BasisKnown, Priority: PriorityLow},
			{ID: "crit1", Date: testDate(t, "2025-01-08"), Amount: 3000, Direction: DirectionOutflow, Category: CategoryOtherOperatingOutflow, Basis: BasisKnown, Priority: PriorityCritical},
		},
		Scenarios: []Scenario{
			{Label: "DEFER_LOW", Transforms: []EventTransform{
				{Kind: TransformDeferByPriority, DelayDays: 7, Priorities: []Priority{PriorityLow}},
			}},
		},
	}
	result := Calculate(in, Options{})
	deferred := result.Scenarios[0]
	// Week 1: only the critical outflow remains (low was pushed to week 2).
	if deferred.Weekly[0].Outflows.Total != 3000 {
		t.Errorf("week 1 outflow after deferral = %v, want 3000 (only critical remains)", deferred.Weekly[0].Outflows.Total)
	}
	if deferred.Weekly[1].Outflows.Total != 2000 {
		t.Errorf("week 2 outflow after deferral = %v, want 2000 (deferred low-priority item)", deferred.Weekly[1].Outflows.Total)
	}
}

func TestScenario_UnrecognizedTransformKindIsNoOp(t *testing.T) {
	events := []CashFlowEvent{
		{ID: "e1", Date: testDate(t, "2025-01-08"), Amount: 100, Direction: DirectionInflow, Category: CategoryCashSale, Basis: BasisKnown},
	}
	got := applyTransform(events, EventTransform{Kind: "BOGUS"}, "SCENARIO")
	if len(got) != 1 || got[0].Amount != 100 || !got[0].Date.Equal(events[0].Date) {
		t.Errorf("unrecognized transform kind should be a no-op copy, got %+v", got)
	}
}

func TestScenario_AppliesToScenario(t *testing.T) {
	untagged := CashFlowEvent{ID: "e1"}
	if !appliesToScenario(untagged, "ANYTHING") {
		t.Errorf("untagged event should apply to every scenario")
	}
	tagged := CashFlowEvent{ID: "e2", ScenarioTags: []string{"DOWNSIDE"}}
	if !appliesToScenario(tagged, "DOWNSIDE") {
		t.Errorf("tagged event should apply to its own scenario")
	}
	if appliesToScenario(tagged, "UPSIDE") {
		t.Errorf("tagged event should not apply to a different scenario")
	}
}

func TestScenario_UnavailableAmountZeroValue(t *testing.T) {
	v := UnavailableAmount()
	if v.Available || v.Value != 0 {
		t.Errorf("UnavailableAmount() = %+v, want zero value", v)
	}
}

func TestScenario_InvalidScenarioLabelCollisionRejected(t *testing.T) {
	in := Input{
		ForecastStartDate: testDate(t, "2025-01-06"),
		OpeningCash:       OpeningCash{Amount: 1000, Currency: "USD"},
		Scenarios: []Scenario{
			{Label: BaseScenarioLabel},
		},
	}
	result := Calculate(in, Options{})
	if !hasIssueCode(result.Issues, IssueInvalidScenario) {
		t.Errorf("expected IssueInvalidScenario for a scenario colliding with the reserved base label, got %+v", result.Issues)
	}
	if len(result.Scenarios) != 0 {
		t.Errorf("expected 0 scenarios computed, got %d", len(result.Scenarios))
	}
}

func TestScenario_DuplicateScenarioLabelRejected(t *testing.T) {
	in := Input{
		ForecastStartDate: testDate(t, "2025-01-06"),
		OpeningCash:       OpeningCash{Amount: 1000, Currency: "USD"},
		Scenarios: []Scenario{
			{Label: "DOWNSIDE"},
			{Label: "DOWNSIDE"},
		},
	}
	result := Calculate(in, Options{})
	if !hasIssueCode(result.Issues, IssueInvalidScenario) {
		t.Errorf("expected IssueInvalidScenario for duplicate labels, got %+v", result.Issues)
	}
}
