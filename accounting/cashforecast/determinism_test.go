package cashforecast

import (
	"encoding/json"
	"sync"
	"testing"
)

func fullInput(t *testing.T) Input {
	t.Helper()
	return Input{
		ForecastStartDate: testDate(t, "2025-01-06"),
		OpeningCash:       OpeningCash{Amount: 100000, Currency: "USD", AsOfDate: testDate(t, "2025-01-05")},
		CashAccounts: []CashAccount{
			{AccountID: "op-1", Balance: 80000, Currency: "USD"},
			{AccountID: "reserve-1", Balance: 20000, Currency: "USD", Restricted: true},
		},
		RecurringRules: []RecurringRule{
			{ID: "rent", Amount: 5000, Direction: DirectionOutflow, Category: CategoryRent, Basis: BasisScheduled,
				StartDate: testDate(t, "2025-01-01"), Frequency: FrequencyMonthly},
			{ID: "software", Amount: 500, Direction: DirectionOutflow, Category: CategoryOtherOperatingOutflow, Basis: BasisScheduled,
				StartDate: testDate(t, "2025-01-01"), Frequency: FrequencyMonthly},
		},
		ARSources: []ARReceivableSource{
			{ReceivableID: "r1", OpenAmount: 10000}, {ReceivableID: "r2", OpenAmount: 5000},
		},
		ARCollections: []ARCollectionAssumption{
			{ID: "c1", ReceivableID: "r1", ExpectedReceiptDate: testDate(t, "2025-01-15"), ExpectedAmount: 8000},
		},
		APSources: []APPayableSource{
			{PayableID: "b1", OpenAmount: 3000, DueDate: testDate(t, "2025-01-20")},
		},
		APPlans: []APPaymentPlan{
			{ID: "p1", PayableID: "b1", PaymentDate: testDate(t, "2025-01-18"), Amount: 3000},
		},
		Payroll: []PayrollEvent{
			{ID: "pr1", Date: testDate(t, "2025-01-10"), EmployeeNetCash: 8000, EmployerTaxes: 800},
		},
		Tax:        []TaxEvent{{ID: "t1", Date: testDate(t, "2025-01-25"), Amount: 1200, Category: TaxSales}},
		Capex:      []CapexEvent{{ID: "cap1", Date: testDate(t, "2025-02-01"), Amount: 15000}},
		Facilities: []CreditFacility{{FacilityID: "loc-1", AvailableToDraw: 50000}},
		Scenarios: []Scenario{
			{Label: "DOWNSIDE", Transforms: []EventTransform{{Kind: TransformDelayInflows, DelayDays: 7, Category: CategoryARCollection}}},
			{Label: "UPSIDE", Transforms: []EventTransform{{Kind: TransformScaleInflows, ScaleFactor: 1.1, Category: CategoryARCollection}}},
		},
	}
}

func fullOptions() Options {
	return Options{
		MinimumCash:    MinimumCashPolicy{MinimumCashBalance: 20000},
		RequiredInputs: RequiredInputs{AR: true, AP: true},
	}
}

func TestDeterminism_RepeatedCallsProduceIdenticalJSON(t *testing.T) {
	in := fullInput(t)
	opts := fullOptions()

	first := Calculate(in, opts)
	firstJSON, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	for i := 0; i < 10; i++ {
		result := Calculate(in, opts)
		gotJSON, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("Marshal iteration %d: %v", i, err)
		}
		if string(gotJSON) != string(firstJSON) {
			t.Fatalf("iteration %d produced different JSON output", i)
		}
	}
}

func TestDeterminism_ConcurrentCallsProduceIdenticalResults(t *testing.T) {
	in := fullInput(t)
	opts := fullOptions()

	want, err := json.Marshal(Calculate(in, opts))
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	const n = 20
	var wg sync.WaitGroup
	errs := make(chan string, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := json.Marshal(Calculate(in, opts))
			if err != nil {
				errs <- err.Error()
				return
			}
			if string(got) != string(want) {
				errs <- "mismatch"
			}
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Errorf("concurrent call failure: %s", e)
	}
}

// TestDeterminism_ScenarioOrderIndependence proves that recomputing the
// same scenarios in reverse input order produces byte-identical
// per-scenario results — see the task's section 60.
func TestDeterminism_ScenarioOrderIndependence(t *testing.T) {
	in := fullInput(t)
	opts := fullOptions()

	result := Calculate(in, opts)

	reversed := in
	reversed.Scenarios = []Scenario{in.Scenarios[1], in.Scenarios[0]}
	reversedResult := Calculate(reversed, opts)

	byLabel := func(scenarios []ScenarioResult) map[string]ScenarioResult {
		m := make(map[string]ScenarioResult, len(scenarios))
		for _, s := range scenarios {
			m[s.Label] = s
		}
		return m
	}
	want := byLabel(result.Scenarios)
	got := byLabel(reversedResult.Scenarios)

	for label, w := range want {
		g, ok := got[label]
		if !ok {
			t.Fatalf("scenario %s missing after reordering", label)
		}
		wantJSON, _ := json.Marshal(w)
		gotJSON, _ := json.Marshal(g)
		if string(wantJSON) != string(gotJSON) {
			t.Errorf("scenario %s differs after reordering inputs", label)
		}
	}
}
