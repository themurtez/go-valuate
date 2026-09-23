package cashforecast

import "testing"

// TestCoverageSourceAmount_NegativeOpenAmountExcludedFromScheduledPercent
// is a regression test for a real bug found by code review:
// buildCoverage summed ARReceivableSource/APPayableSource.OpenAmount with
// no guard against a negative or non-finite value (unlike CashFlowEvent.
// Amount, which validate.go explicitly rejects), so a single bad row
// (e.g. a credit memo mis-mapped as a receivable with a negative
// OpenAmount) could silently drive Coverage.AR.ScheduledPercent above
// 100% or negative while still reporting Available:true.
func TestCoverageSourceAmount_NegativeOpenAmountExcludedFromScheduledPercent(t *testing.T) {
	in := Input{
		ForecastStartDate: testDate(t, "2025-01-06"),
		OpeningCash:       OpeningCash{Amount: 10000, Currency: "USD"},
		ARSources: []ARReceivableSource{
			{ReceivableID: "r1", OpenAmount: 10000},
			{ReceivableID: "r2", OpenAmount: -5000}, // bad row: negative.
		},
		ARCollections: []ARCollectionAssumption{
			{ID: "c1", ReceivableID: "r1", ExpectedReceiptDate: testDate(t, "2025-01-10"), ExpectedAmount: 10000},
		},
	}
	result := Calculate(in, Options{})

	if !result.Available {
		t.Fatalf("expected Available=true, got %+v", result.Issues)
	}
	if !hasIssueCode(result.Issues, IssueNonFiniteAmount) {
		t.Errorf("expected IssueNonFiniteAmount for the negative OpenAmount row, got %+v", result.Issues)
	}
	// With r2 excluded, totalOpen = 10000 (r1 only), scheduled = 10000 ->
	// 100%, never > 1.0 or negative.
	if !result.Coverage.AR.ScheduledPercent.Available {
		t.Fatalf("expected AR.ScheduledPercent.Available=true")
	}
	if got := result.Coverage.AR.ScheduledPercent.Value; got < 0 || got > 1.0 {
		t.Errorf("AR.ScheduledPercent = %v, want a value in [0, 1] (bad row must be excluded)", got)
	}
	if abs(result.Coverage.AR.ScheduledPercent.Value-1.0) > 1e-9 {
		t.Errorf("AR.ScheduledPercent = %v, want 1.0 (only the valid r1 row counted)", result.Coverage.AR.ScheduledPercent.Value)
	}
}

func TestCoverageSourceAmount_APNegativeOpenAmountExcluded(t *testing.T) {
	in := Input{
		ForecastStartDate: testDate(t, "2025-01-06"),
		OpeningCash:       OpeningCash{Amount: 10000, Currency: "USD"},
		APSources: []APPayableSource{
			{PayableID: "b1", OpenAmount: 8000, DueDate: testDate(t, "2025-01-20")},
			{PayableID: "b2", OpenAmount: -2000, DueDate: testDate(t, "2025-01-20")}, // bad row.
		},
		APPlans: []APPaymentPlan{
			{ID: "p1", PayableID: "b1", PaymentDate: testDate(t, "2025-01-18"), Amount: 4000},
		},
	}
	result := Calculate(in, Options{})

	if !hasIssueCode(result.Issues, IssueNonFiniteAmount) {
		t.Errorf("expected IssueNonFiniteAmount for the negative AP OpenAmount row, got %+v", result.Issues)
	}
	if got := result.Coverage.AP.ScheduledPercent.Value; got < 0 || got > 1.0 {
		t.Errorf("AP.ScheduledPercent = %v, want a value in [0, 1] (bad row must be excluded)", got)
	}
}
