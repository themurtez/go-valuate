package cashforecast

import "testing"

// TestEventRange_BeforeForecastStartExcludedAndSummarized proves an event
// dated before ForecastStartDate is excluded from every weekly total and
// reported via BeforeStart — opening cash remains authoritative and is
// never adjusted for it.
func TestEventRange_BeforeForecastStartExcludedAndSummarized(t *testing.T) {
	in := Input{
		ForecastStartDate: testDate(t, "2025-01-06"),
		OpeningCash:       OpeningCash{Amount: 1000, Currency: "USD"},
		Events: []CashFlowEvent{
			{ID: "old", Date: testDate(t, "2025-01-01"), Amount: 500, Direction: DirectionInflow, Category: CategoryCashSale, Basis: BasisKnown},
		},
	}
	result := Calculate(in, Options{})

	if !result.Available {
		t.Fatalf("expected Available=true, got %+v", result.Issues)
	}
	if result.BeforeStart.Count != 1 || result.BeforeStart.Amount != 500 {
		t.Errorf("BeforeStart = %+v, want Count=1 Amount=500", result.BeforeStart)
	}
	if result.BaseScenario.Weekly[0].OpeningCash != 1000 {
		t.Errorf("week 1 OpeningCash = %v, want 1000 (unaffected by before-start event)", result.BaseScenario.Weekly[0].OpeningCash)
	}
	if result.BaseScenario.Weekly[0].Inflows.Total != 0 {
		t.Errorf("week 1 inflow = %v, want 0 (before-start event excluded from weekly totals)", result.BaseScenario.Weekly[0].Inflows.Total)
	}
	if !hasIssueCode(result.Issues, IssueEventBeforeForecastStart) {
		t.Errorf("expected IssueEventBeforeForecastStart, got %+v", result.Issues)
	}
}

// TestEventRange_BeyondHorizonExcludedAndSummarized proves an event dated
// after the last forecast week is excluded from weekly totals but
// summarized in BeyondHorizon.
func TestEventRange_BeyondHorizonExcludedAndSummarized(t *testing.T) {
	in := Input{
		ForecastStartDate: testDate(t, "2025-01-06"),
		OpeningCash:       OpeningCash{Amount: 1000, Currency: "USD"},
		Events: []CashFlowEvent{
			{ID: "future", Date: testDate(t, "2025-06-01"), Amount: 700, Direction: DirectionOutflow, Category: CategoryAPPayment, Basis: BasisKnown},
		},
	}
	result := Calculate(in, Options{HorizonWeeks: 13})

	if result.BeyondHorizon.Count != 1 || result.BeyondHorizon.Amount != 700 {
		t.Errorf("BeyondHorizon = %+v, want Count=1 Amount=700", result.BeyondHorizon)
	}
	var totalOutflow float64
	for _, w := range result.BaseScenario.Weekly {
		totalOutflow += w.Outflows.Total
	}
	if totalOutflow != 0 {
		t.Errorf("total weekly outflow = %v, want 0 (beyond-horizon event excluded)", totalOutflow)
	}
	if !hasIssueCode(result.Issues, IssueEventBeyondHorizon) {
		t.Errorf("expected IssueEventBeyondHorizon, got %+v", result.Issues)
	}
}

// TestEventRange_ExactBoundaryDatesIncluded proves an event on exactly
// the forecast's first day or exactly the last day of the horizon is
// included, not excluded (inclusive boundaries).
func TestEventRange_ExactBoundaryDatesIncluded(t *testing.T) {
	in := Input{
		ForecastStartDate: testDate(t, "2025-01-06"),
		OpeningCash:       OpeningCash{Amount: 1000, Currency: "USD"},
		Events: []CashFlowEvent{
			{ID: "first-day", Date: testDate(t, "2025-01-06"), Amount: 100, Direction: DirectionInflow, Category: CategoryCashSale, Basis: BasisKnown},
		},
	}
	result := Calculate(in, Options{HorizonWeeks: 1}) // horizon is exactly [Jan 6, Jan 12].

	// Also test an event on the last day of a 1-week horizon.
	in2 := in
	in2.Events = []CashFlowEvent{
		{ID: "last-day", Date: testDate(t, "2025-01-12"), Amount: 200, Direction: DirectionInflow, Category: CategoryCashSale, Basis: BasisKnown},
	}
	result2 := Calculate(in2, Options{HorizonWeeks: 1})

	if result.BaseScenario.Weekly[0].Inflows.Total != 100 {
		t.Errorf("first-day event not included: week 1 inflow = %v, want 100", result.BaseScenario.Weekly[0].Inflows.Total)
	}
	if result.BeforeStart.Count != 0 {
		t.Errorf("first-day event incorrectly excluded as before-start: %+v", result.BeforeStart)
	}

	if result2.BaseScenario.Weekly[0].Inflows.Total != 200 {
		t.Errorf("last-day event not included: week 1 inflow = %v, want 200", result2.BaseScenario.Weekly[0].Inflows.Total)
	}
	if result2.BeyondHorizon.Count != 0 {
		t.Errorf("last-day event incorrectly excluded as beyond-horizon: %+v", result2.BeyondHorizon)
	}
}
