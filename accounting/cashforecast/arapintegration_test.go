package cashforecast

import "testing"

// TestARIntegration_CoverageAndUnscheduled locks the task's section 52
// worked example: AR open amount 100,000, scheduled collections 80,000 ->
// 80% scheduled, 20,000 unscheduled.
func TestARIntegration_CoverageAndUnscheduled(t *testing.T) {
	in := Input{
		ForecastStartDate: testDate(t, "2025-01-06"),
		OpeningCash:       OpeningCash{Amount: 10000, Currency: "USD"},
		ARSources: []ARReceivableSource{
			{ReceivableID: "r1", OpenAmount: 60000},
			{ReceivableID: "r2", OpenAmount: 40000},
		},
		ARCollections: []ARCollectionAssumption{
			{ID: "c1", ReceivableID: "r1", ExpectedReceiptDate: testDate(t, "2025-01-10"), ExpectedAmount: 60000},
			{ID: "c2", ReceivableID: "r2", ExpectedReceiptDate: testDate(t, "2025-01-12"), ExpectedAmount: 20000},
		},
	}
	result := Calculate(in, Options{})

	if !result.Available {
		t.Fatalf("expected Available=true, got Issues=%+v", result.Issues)
	}
	if !result.Coverage.AR.ScheduledPercent.Available {
		t.Fatalf("expected AR.ScheduledPercent.Available=true")
	}
	if got := result.Coverage.AR.ScheduledPercent.Value; abs(got-0.8) > 1e-9 {
		t.Errorf("AR.ScheduledPercent = %v, want 0.8", got)
	}
	if !result.UnscheduledAR.Available || result.UnscheduledAR.Amount != 20000 {
		t.Errorf("UnscheduledAR = %+v, want Available=true Amount=20000", result.UnscheduledAR)
	}
	if result.UnscheduledAR.Count != 1 {
		t.Errorf("UnscheduledAR.Count = %d, want 1", result.UnscheduledAR.Count)
	}
}

// TestAPIntegration_CoverageAndUnscheduled locks the task's section 53
// worked example: AP open amount 75,000, planned payments 60,000 ->
// 60,000 scheduled, 15,000 unscheduled.
func TestAPIntegration_CoverageAndUnscheduled(t *testing.T) {
	in := Input{
		ForecastStartDate: testDate(t, "2025-01-06"),
		OpeningCash:       OpeningCash{Amount: 100000, Currency: "USD"},
		APSources: []APPayableSource{
			{PayableID: "b1", OpenAmount: 45000, DueDate: testDate(t, "2025-01-20")},
			{PayableID: "b2", OpenAmount: 30000, DueDate: testDate(t, "2025-01-25")},
		},
		APPlans: []APPaymentPlan{
			{ID: "p1", PayableID: "b1", PaymentDate: testDate(t, "2025-01-18"), Amount: 45000},
			{ID: "p2", PayableID: "b2", PaymentDate: testDate(t, "2025-01-22"), Amount: 15000},
		},
	}
	result := Calculate(in, Options{})

	if !result.Available {
		t.Fatalf("expected Available=true, got Issues=%+v", result.Issues)
	}
	if !result.UnscheduledAP.Available || result.UnscheduledAP.Amount != 15000 {
		t.Errorf("UnscheduledAP = %+v, want Available=true Amount=15000", result.UnscheduledAP)
	}
	// 60000 scheduled / 75000 total = 0.8.
	if got := result.Coverage.AP.ScheduledPercent.Value; abs(got-0.8) > 1e-9 {
		t.Errorf("AP.ScheduledPercent = %v, want 0.8", got)
	}
}

// TestAPIntegration_DueDateAdapterOptIn proves that with
// Options.PayAPOnDueDate=true, open bills not otherwise scheduled become
// outflow events on their own due dates — see the task's section 54.
func TestAPIntegration_DueDateAdapterOptIn(t *testing.T) {
	in := Input{
		ForecastStartDate: testDate(t, "2025-01-06"),
		OpeningCash:       OpeningCash{Amount: 100000, Currency: "USD"},
		APSources: []APPayableSource{
			{PayableID: "b1", OpenAmount: 5000, DueDate: testDate(t, "2025-01-09")}, // week 1
		},
	}
	result := Calculate(in, Options{PayAPOnDueDate: true})

	if !result.Available {
		t.Fatalf("expected Available=true, got Issues=%+v", result.Issues)
	}
	if result.BaseScenario.Weekly[0].Outflows.Total != 5000 {
		t.Errorf("week 1 outflow = %v, want 5000 (due-date adapter should have scheduled it)", result.BaseScenario.Weekly[0].Outflows.Total)
	}
	// UnscheduledAP must be empty in due-date-adapter mode — every open
	// payable is covered by construction.
	if result.UnscheduledAP.Amount != 0 {
		t.Errorf("UnscheduledAP.Amount = %v, want 0 when PayAPOnDueDate=true", result.UnscheduledAP.Amount)
	}
}

// TestAPIntegration_DueDateAdapterOff proves no AP outflow event is
// generated when PayAPOnDueDate is left at its default false — the
// default-safe behavior.
func TestAPIntegration_DueDateAdapterOff(t *testing.T) {
	in := Input{
		ForecastStartDate: testDate(t, "2025-01-06"),
		OpeningCash:       OpeningCash{Amount: 100000, Currency: "USD"},
		APSources: []APPayableSource{
			{PayableID: "b1", OpenAmount: 5000, DueDate: testDate(t, "2025-01-09")},
		},
	}
	result := Calculate(in, Options{})

	if result.BaseScenario.Weekly[0].Outflows.Total != 0 {
		t.Errorf("week 1 outflow = %v, want 0 (no automatic AP scheduling by default)", result.BaseScenario.Weekly[0].Outflows.Total)
	}
	if result.UnscheduledAP.Amount != 5000 {
		t.Errorf("UnscheduledAP.Amount = %v, want 5000", result.UnscheduledAP.Amount)
	}
}

// TestARIntegration_DueDateNeverAutoBecomesReceiptDate is a regression
// test proving a receivable's due date is never automatically treated as
// its cash receipt date — see the task's section 55. There is
// deliberately no AR equivalent of PayAPOnDueDate; ARSources alone
// (without an explicit ARCollectionAssumption) produces zero AR inflow
// events.
func TestARIntegration_DueDateNeverAutoBecomesReceiptDate(t *testing.T) {
	in := Input{
		ForecastStartDate: testDate(t, "2025-01-06"),
		OpeningCash:       OpeningCash{Amount: 1000, Currency: "USD"},
		ARSources: []ARReceivableSource{
			{ReceivableID: "r1", OpenAmount: 20000},
		},
		// No ARCollections supplied at all.
	}
	result := Calculate(in, Options{})

	for i, w := range result.BaseScenario.Weekly {
		if w.Inflows.Total != 0 {
			t.Errorf("week %d inflow = %v, want 0 (no automatic AR receipt scheduling)", i+1, w.Inflows.Total)
		}
	}
	if !result.UnscheduledAR.Available || result.UnscheduledAR.Amount != 20000 {
		t.Errorf("UnscheduledAR = %+v, want Available=true Amount=20000 (entire receivable unscheduled)", result.UnscheduledAR)
	}
}

func TestARIntegration_OverSchedulingRejected(t *testing.T) {
	in := Input{
		ForecastStartDate: testDate(t, "2025-01-06"),
		OpeningCash:       OpeningCash{Amount: 1000, Currency: "USD"},
		ARSources: []ARReceivableSource{
			{ReceivableID: "r1", OpenAmount: 10000},
		},
		ARCollections: []ARCollectionAssumption{
			{ID: "c1", ReceivableID: "r1", ExpectedReceiptDate: testDate(t, "2025-01-10"), ExpectedAmount: 6000},
			{ID: "c2", ReceivableID: "r1", ExpectedReceiptDate: testDate(t, "2025-01-15"), ExpectedAmount: 6000}, // total 12000 > 10000 open
		},
	}
	result := Calculate(in, Options{})

	if !hasIssueCode(result.Issues, IssueARScheduleExceedsOpen) {
		t.Errorf("expected IssueARScheduleExceedsOpen, got %+v", result.Issues)
	}
	// The over-scheduling second assumption should be excluded; only the
	// first 6000 should appear as an inflow.
	var totalInflow float64
	for _, w := range result.BaseScenario.Weekly {
		totalInflow += w.Inflows.Total
	}
	if totalInflow != 6000 {
		t.Errorf("total AR inflow = %v, want 6000 (over-scheduled portion excluded)", totalInflow)
	}
}

func TestAPIntegration_OverSchedulingRejected(t *testing.T) {
	in := Input{
		ForecastStartDate: testDate(t, "2025-01-06"),
		OpeningCash:       OpeningCash{Amount: 100000, Currency: "USD"},
		APSources: []APPayableSource{
			{PayableID: "b1", OpenAmount: 5000, DueDate: testDate(t, "2025-01-20")},
		},
		APPlans: []APPaymentPlan{
			{ID: "p1", PayableID: "b1", PaymentDate: testDate(t, "2025-01-10"), Amount: 4000},
			{ID: "p2", PayableID: "b1", PaymentDate: testDate(t, "2025-01-15"), Amount: 4000}, // total 8000 > 5000 open
		},
	}
	result := Calculate(in, Options{})

	if !hasIssueCode(result.Issues, IssueAPScheduleExceedsOpen) {
		t.Errorf("expected IssueAPScheduleExceedsOpen, got %+v", result.Issues)
	}
}

// TestARAPIntegration_PartialAndMultipleReceipts proves multiple partial
// scheduled receipts/payments against the same receivable/payable are
// supported as long as their total does not exceed the open amount.
func TestARAPIntegration_PartialAndMultipleReceipts(t *testing.T) {
	in := Input{
		ForecastStartDate: testDate(t, "2025-01-06"),
		OpeningCash:       OpeningCash{Amount: 1000, Currency: "USD"},
		ARSources: []ARReceivableSource{
			{ReceivableID: "r1", OpenAmount: 10000},
		},
		ARCollections: []ARCollectionAssumption{
			{ID: "c1", ReceivableID: "r1", ExpectedReceiptDate: testDate(t, "2025-01-08"), ExpectedAmount: 4000},
			{ID: "c2", ReceivableID: "r1", ExpectedReceiptDate: testDate(t, "2025-01-15"), ExpectedAmount: 6000},
		},
	}
	result := Calculate(in, Options{})

	if hasIssueCode(result.Issues, IssueARScheduleExceedsOpen) {
		t.Errorf("unexpected IssueARScheduleExceedsOpen for exactly-matching partial receipts")
	}
	var totalInflow float64
	for _, w := range result.BaseScenario.Weekly {
		totalInflow += w.Inflows.Total
	}
	if totalInflow != 10000 {
		t.Errorf("total AR inflow = %v, want 10000 (both partial receipts included)", totalInflow)
	}
	if result.UnscheduledAR.Amount != 0 {
		t.Errorf("UnscheduledAR.Amount = %v, want 0 (fully scheduled)", result.UnscheduledAR.Amount)
	}
}
