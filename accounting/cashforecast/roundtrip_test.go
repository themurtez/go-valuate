package cashforecast

import (
	"encoding/json"
	"strings"
	"testing"
)

func roundTrip[T any](t *testing.T, v T) T {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(data), "NaN") || strings.Contains(string(data), "Inf") {
		t.Fatalf("JSON output contains NaN/Inf: %s", data)
	}
	var out T
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	return out
}

func TestJSON_CashFlowEvent(t *testing.T) {
	e := CashFlowEvent{
		ID: "evt-1", Date: testDate(t, "2025-01-10"), Amount: 5000, Direction: DirectionInflow,
		Category: CategoryARCollection, Subcategory: "retainer", Description: "Client payment",
		CounterpartyID: "cust-1", SourceType: SourceARReceivable, SourceID: "recv-1",
		Basis: BasisAssumed, Certainty: CertaintyMedium, Commitment: CommitmentRequired, Priority: PriorityHigh,
		ScenarioTags: []string{"DOWNSIDE"}, Dimensions: []Dimension{{Key: "region", Value: "west"}},
	}
	got := roundTrip(t, e)
	if got.ID != e.ID || got.Amount != e.Amount || got.Direction != e.Direction || got.Category != e.Category {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, e)
	}
	if len(got.ScenarioTags) != 1 || got.ScenarioTags[0] != "DOWNSIDE" {
		t.Errorf("ScenarioTags round-trip failed: %+v", got.ScenarioTags)
	}
	if len(got.Dimensions) != 1 || got.Dimensions[0].Value != "west" {
		t.Errorf("Dimensions round-trip failed: %+v", got.Dimensions)
	}
}

func TestJSON_OpeningCashAndAccounts(t *testing.T) {
	oc := OpeningCash{Amount: 100000, Currency: "USD", AsOfDate: testDate(t, "2025-01-01"), SourceRef: "bank-1"}
	got := roundTrip(t, oc)
	if got.Amount != oc.Amount || got.Currency != oc.Currency {
		t.Errorf("OpeningCash round-trip mismatch: got %+v, want %+v", got, oc)
	}

	ca := CashAccount{AccountID: "acct-1", Name: "Operating", Balance: 50000, Currency: "USD", Restricted: true, MinimumReserve: 10000}
	gotCA := roundTrip(t, ca)
	if gotCA.AccountID != ca.AccountID || gotCA.Restricted != ca.Restricted {
		t.Errorf("CashAccount round-trip mismatch: got %+v, want %+v", gotCA, ca)
	}
}

func TestJSON_RecurringRule(t *testing.T) {
	rule := RecurringRule{
		ID: "rent-1", Amount: 5000, Direction: DirectionOutflow, Category: CategoryRent, Basis: BasisScheduled,
		StartDate: testDate(t, "2025-01-01"), EndDate: testDate(t, "2025-12-31"), Frequency: FrequencyMonthly,
	}
	got := roundTrip(t, rule)
	if got.ID != rule.ID || got.Amount != rule.Amount || got.Frequency != rule.Frequency {
		t.Errorf("RecurringRule round-trip mismatch: got %+v, want %+v", got, rule)
	}
}

func TestJSON_ARCollectionAssumptionAndAPPaymentPlan(t *testing.T) {
	ar := ARCollectionAssumption{ID: "c1", ReceivableID: "r1", ExpectedReceiptDate: testDate(t, "2025-01-15"), ExpectedAmount: 5000}
	gotAR := roundTrip(t, ar)
	if gotAR.ReceivableID != ar.ReceivableID || gotAR.ExpectedAmount != ar.ExpectedAmount {
		t.Errorf("ARCollectionAssumption round-trip mismatch: got %+v, want %+v", gotAR, ar)
	}

	ap := APPaymentPlan{ID: "p1", PayableID: "b1", PaymentDate: testDate(t, "2025-01-20"), Amount: 3000}
	gotAP := roundTrip(t, ap)
	if gotAP.PayableID != ap.PayableID || gotAP.Amount != ap.Amount {
		t.Errorf("APPaymentPlan round-trip mismatch: got %+v, want %+v", gotAP, ap)
	}
}

func TestJSON_WeeklyForecastAndScenarioResult(t *testing.T) {
	in := Input{
		ForecastStartDate: testDate(t, "2025-01-06"),
		OpeningCash:       OpeningCash{Amount: 50000, Currency: "USD"},
		Events: []CashFlowEvent{
			{ID: "e1", Date: testDate(t, "2025-01-07"), Amount: 1000, Direction: DirectionInflow, Category: CategoryCashSale, Basis: BasisKnown},
		},
	}
	result := Calculate(in, Options{MinimumCash: MinimumCashPolicy{MinimumCashBalance: 10000}})

	gotWeek := roundTrip(t, result.BaseScenario.Weekly[0])
	if gotWeek.WeekNumber != result.BaseScenario.Weekly[0].WeekNumber || gotWeek.EndingCash != result.BaseScenario.Weekly[0].EndingCash {
		t.Errorf("WeeklyForecast round-trip mismatch: got %+v, want %+v", gotWeek, result.BaseScenario.Weekly[0])
	}

	gotScenario := roundTrip(t, result.BaseScenario)
	if gotScenario.Label != result.BaseScenario.Label || len(gotScenario.Weekly) != len(result.BaseScenario.Weekly) {
		t.Errorf("ScenarioResult round-trip mismatch: got %+v, want %+v", gotScenario.Label, result.BaseScenario.Label)
	}
}

func TestJSON_FullResult(t *testing.T) {
	in := Input{
		ForecastStartDate: testDate(t, "2025-01-06"),
		OpeningCash:       OpeningCash{Amount: 50000, Currency: "USD"},
		ARSources:         []ARReceivableSource{{ReceivableID: "r1", OpenAmount: 10000}},
		ARCollections:     []ARCollectionAssumption{{ID: "c1", ReceivableID: "r1", ExpectedReceiptDate: testDate(t, "2025-01-10"), ExpectedAmount: 6000}},
		Scenarios:         []Scenario{{Label: "DOWNSIDE", Transforms: []EventTransform{{Kind: TransformScaleInflows, ScaleFactor: 0.8}}}},
	}
	result := Calculate(in, Options{MinimumCash: MinimumCashPolicy{MinimumCashBalance: 5000}})

	got := roundTrip(t, result)
	if got.Available != result.Available || got.SchemaVersion != result.SchemaVersion {
		t.Errorf("Result round-trip mismatch: got Available=%v SchemaVersion=%v", got.Available, got.SchemaVersion)
	}
	if len(got.Scenarios) != len(result.Scenarios) {
		t.Errorf("Result.Scenarios round-trip length mismatch: got %d, want %d", len(got.Scenarios), len(result.Scenarios))
	}
	if !got.UnscheduledAR.Available || got.UnscheduledAR.Amount != result.UnscheduledAR.Amount {
		t.Errorf("Result.UnscheduledAR round-trip mismatch: got %+v, want %+v", got.UnscheduledAR, result.UnscheduledAR)
	}
}

func TestJSON_IssuesAndFlags(t *testing.T) {
	issue := Issue{Code: IssueDuplicateEvent, Severity: SeverityWarning, Message: "dup", EventID: "e1"}
	gotIssue := roundTrip(t, issue)
	if gotIssue.Code != issue.Code || gotIssue.EventID != issue.EventID {
		t.Errorf("Issue round-trip mismatch: got %+v, want %+v", gotIssue, issue)
	}

	flag := Flag{Code: FlagCashBelowMinimum, Severity: FlagSeverityWarning, Week: 3, Value: 1000, Threshold: 5000}
	gotFlag := roundTrip(t, flag)
	if gotFlag.Code != flag.Code || gotFlag.Week != flag.Week {
		t.Errorf("Flag round-trip mismatch: got %+v, want %+v", gotFlag, flag)
	}
}
