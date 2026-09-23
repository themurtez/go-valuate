package cashforecast

import "testing"

// TestScenarioRevalidation_NegativeScaleFactorExcludedNotSilentlyCorrupted
// is a regression test for a real bug found by code review: a scenario's
// ScaleEvents transform (e.g. a caller-supplied ScaleFactor of -1.0,
// perhaps a typo intending "reduce to zero") could produce a
// CashFlowEvent with a negative Amount, and that scenario-transformed
// event was never re-validated the way base events are — silently
// corrupting Inflows.Total/Outflows.Total with no Issue raised.
func TestScenarioRevalidation_NegativeScaleFactorExcludedNotSilentlyCorrupted(t *testing.T) {
	in := Input{
		ForecastStartDate: testDate(t, "2025-01-06"),
		OpeningCash:       OpeningCash{Amount: 10000, Currency: "USD"},
		Events: []CashFlowEvent{
			{ID: "e1", Date: testDate(t, "2025-01-08"), Amount: 1000, Direction: DirectionInflow,
				Category: CategoryCashSale, Basis: BasisKnown},
		},
		Scenarios: []Scenario{
			{Label: "BAD_SCALE", Transforms: []EventTransform{
				{Kind: TransformScaleInflows, ScaleFactor: -1.0},
			}},
		},
	}
	result := Calculate(in, Options{})

	if !result.Available {
		t.Fatalf("expected Available=true, got %+v", result.Issues)
	}
	if len(result.Scenarios) != 1 {
		t.Fatalf("expected 1 scenario, got %d", len(result.Scenarios))
	}
	bad := result.Scenarios[0]

	// The negative-amount event must be excluded, not silently summed
	// with a negative sign.
	if bad.Weekly[0].Inflows.Total != 0 {
		t.Errorf("scenario inflow = %v, want 0 (negative-amount event must be excluded)", bad.Weekly[0].Inflows.Total)
	}

	found := false
	for _, i := range result.Issues {
		if i.Code == IssueNegativeAmount && i.SourceID == "BAD_SCALE" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected IssueNegativeAmount with SourceID=BAD_SCALE, got %+v", result.Issues)
	}
}

// TestScenarioRevalidation_ValidScaleFactorUnaffected proves the
// re-validation fix does not reject ordinary, valid scenario output.
func TestScenarioRevalidation_ValidScaleFactorUnaffected(t *testing.T) {
	in := Input{
		ForecastStartDate: testDate(t, "2025-01-06"),
		OpeningCash:       OpeningCash{Amount: 10000, Currency: "USD"},
		Events: []CashFlowEvent{
			{ID: "e1", Date: testDate(t, "2025-01-08"), Amount: 1000, Direction: DirectionInflow,
				Category: CategoryCashSale, Basis: BasisKnown},
		},
		Scenarios: []Scenario{
			{Label: "SCALED_DOWN", Transforms: []EventTransform{
				{Kind: TransformScaleInflows, ScaleFactor: 0.5},
			}},
		},
	}
	result := Calculate(in, Options{})

	if result.Scenarios[0].Weekly[0].Inflows.Total != 500 {
		t.Errorf("scenario inflow = %v, want 500", result.Scenarios[0].Weekly[0].Inflows.Total)
	}
	if hasIssueCode(result.Issues, IssueNegativeAmount) {
		t.Errorf("did not expect IssueNegativeAmount for a valid 0.5 scale factor")
	}
}
