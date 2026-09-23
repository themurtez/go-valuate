package cashforecast

import "testing"

// TestFundingGap_LockedFormula proves the task's section 59 worked
// example: minimum cash 50,000, minimum projected cash -20,000 ->
// single-upfront funding requirement 70,000.
func TestFundingGap_LockedFormula(t *testing.T) {
	in := Input{
		ForecastStartDate: testDate(t, "2025-01-06"),
		OpeningCash:       OpeningCash{Amount: 30000, Currency: "USD"},
		Events: []CashFlowEvent{
			// Opening 30,000 - 50,000 outflow in week 1 = -20,000 ending
			// cash, which is also the minimum across the horizon (no
			// further events).
			{ID: "big-outflow", Date: testDate(t, "2025-01-08"), Amount: 50000, Direction: DirectionOutflow, Category: CategoryAPPayment, Basis: BasisKnown},
		},
	}
	opts := Options{MinimumCash: MinimumCashPolicy{MinimumCashBalance: 50000}}
	result := Calculate(in, opts)

	if !result.Available {
		t.Fatalf("expected Available=true, got Issues=%+v", result.Issues)
	}
	summary := result.BaseScenario.Summary
	if !summary.ThresholdAvailable {
		t.Fatalf("expected ThresholdAvailable=true")
	}
	if summary.LowestCashBalance != -20000 {
		t.Fatalf("expected LowestCashBalance=-20000, got %v", summary.LowestCashBalance)
	}
	if summary.MaximumFundingGap != 70000 {
		t.Errorf("MaximumFundingGap = %v, want 70000", summary.MaximumFundingGap)
	}
	if !summary.RequiredFunding.Available {
		t.Fatalf("expected RequiredFunding.Available=true")
	}
	if summary.RequiredFunding.RequiredAtStart != 70000 {
		t.Errorf("RequiredFunding.RequiredAtStart = %v, want 70000", summary.RequiredFunding.RequiredAtStart)
	}
	if summary.RequiredFunding.PeakCumulativeGap != 70000 {
		t.Errorf("RequiredFunding.PeakCumulativeGap = %v, want 70000", summary.RequiredFunding.PeakCumulativeGap)
	}
}

func TestFundingGap_NoGapWhenAboveMinimum(t *testing.T) {
	in := Input{
		ForecastStartDate: testDate(t, "2025-01-06"),
		OpeningCash:       OpeningCash{Amount: 100000, Currency: "USD"},
	}
	opts := Options{MinimumCash: MinimumCashPolicy{MinimumCashBalance: 10000}}
	result := Calculate(in, opts)

	summary := result.BaseScenario.Summary
	if summary.MaximumFundingGap != 0 {
		t.Errorf("MaximumFundingGap = %v, want 0", summary.MaximumFundingGap)
	}
	if summary.RequiredFunding.RequiredAtStart != 0 {
		t.Errorf("RequiredFunding.RequiredAtStart = %v, want 0", summary.RequiredFunding.RequiredAtStart)
	}
	for _, w := range result.BaseScenario.Weekly {
		if w.Threshold.BelowMinimum {
			t.Errorf("week %d unexpectedly below minimum", w.WeekNumber)
		}
	}
}

func TestFundingGap_UnavailableWithoutMinimumThreshold(t *testing.T) {
	in := Input{
		ForecastStartDate: testDate(t, "2025-01-06"),
		OpeningCash:       OpeningCash{Amount: 1000, Currency: "USD"},
		Events: []CashFlowEvent{
			{ID: "e1", Date: testDate(t, "2025-01-08"), Amount: 5000, Direction: DirectionOutflow, Category: CategoryAPPayment, Basis: BasisKnown},
		},
	}
	result := Calculate(in, Options{})

	summary := result.BaseScenario.Summary
	if summary.ThresholdAvailable {
		t.Errorf("expected ThresholdAvailable=false with no minimum-cash policy supplied")
	}
	if summary.RequiredFunding.Available {
		t.Errorf("expected RequiredFunding.Available=false with no minimum-cash policy supplied")
	}
	// Weekly cash balances should remain fully available even without a
	// threshold — see the task's section 5. Jan 8 falls in week 1 (Jan
	// 6-12), so week 1 already reflects the 5000 outflow: 1000 - 5000 =
	// -4000.
	if len(result.BaseScenario.Weekly) == 0 {
		t.Fatalf("expected weekly forecast to still be computed")
	}
	if result.BaseScenario.Weekly[0].EndingCash != -4000 {
		t.Errorf("week 1 EndingCash = %v, want -4000", result.BaseScenario.Weekly[0].EndingCash)
	}
}

func TestFundingGap_ExplicitZeroMinimumDistinctFromUnsupplied(t *testing.T) {
	in := Input{
		ForecastStartDate: testDate(t, "2025-01-06"),
		OpeningCash:       OpeningCash{Amount: 5000, Currency: "USD"},
	}
	result := Calculate(in, Options{MinimumCash: MinimumCashPolicy{MinimumCashBalanceExplicitZero: true}})

	summary := result.BaseScenario.Summary
	if !summary.ThresholdAvailable {
		t.Fatalf("expected ThresholdAvailable=true for an explicit $0 minimum")
	}
	if summary.MinimumCashThreshold != 0 {
		t.Errorf("MinimumCashThreshold = %v, want 0", summary.MinimumCashThreshold)
	}
}

// TestFundingGap_FacilityCoverage proves GapAfterFacility reports the
// remaining gap after available facility capacity without asserting a
// draw occurred.
func TestFundingGap_FacilityCoverage(t *testing.T) {
	in := Input{
		ForecastStartDate: testDate(t, "2025-01-06"),
		OpeningCash:       OpeningCash{Amount: 10000, Currency: "USD"},
		Events: []CashFlowEvent{
			{ID: "big", Date: testDate(t, "2025-01-08"), Amount: 60000, Direction: DirectionOutflow, Category: CategoryAPPayment, Basis: BasisKnown},
		},
		Facilities: []CreditFacility{{FacilityID: "loc-1", AvailableToDraw: 30000}},
	}
	result := Calculate(in, Options{MinimumCash: MinimumCashPolicy{MinimumCashBalance: 0, MinimumCashBalanceExplicitZero: true}})

	summary := result.BaseScenario.Summary
	// Gap = 0 - (10000 - 60000) = 50000.
	if summary.MaximumFundingGap != 50000 {
		t.Fatalf("MaximumFundingGap = %v, want 50000", summary.MaximumFundingGap)
	}
	if summary.FacilityCapacity != 30000 {
		t.Errorf("FacilityCapacity = %v, want 30000", summary.FacilityCapacity)
	}
	if !summary.GapAfterFacility.Available || summary.GapAfterFacility.Value != 20000 {
		t.Errorf("GapAfterFacility = %+v, want Available=true Value=20000", summary.GapAfterFacility)
	}

	hasFacilityFlag := false
	for _, f := range result.Flags {
		if f.Code == FlagFacilityInsufficientForGap {
			hasFacilityFlag = true
		}
	}
	if !hasFacilityFlag {
		t.Errorf("expected FlagFacilityInsufficientForGap, got flags %+v", result.Flags)
	}
}
