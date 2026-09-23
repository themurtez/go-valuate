package labor_test

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/themurtez/go-valuate/accounting/labor"
	"github.com/themurtez/go-valuate/accounting/labor/fixtures"
)

func fullInput() labor.Input {
	workers, records := fixtures.StableServiceBusiness()
	_, contractors := fixtures.HighContractorShare()
	return labor.Input{
		Periods:           fixtures.TwoMonthPeriods(),
		Workers:           workers,
		PayrollRecords:    records,
		ContractorRecords: contractors,
		BusinessMetrics: []labor.BusinessMetrics{
			{Period: "2025-05", Revenue: labor.AvailableValue(300000), GrossProfit: labor.AvailableValue(150000), EBITDA: labor.AvailableValue(80000)},
			{Period: "2025-06", Revenue: labor.AvailableValue(310000), GrossProfit: labor.AvailableValue(155000), EBITDA: labor.AvailableValue(82000)},
		},
	}
}

func TestDeterminism_RepeatedCallsProduceIdenticalJSON(t *testing.T) {
	in := fullInput()
	policy := labor.DefaultPolicy()

	first := labor.Calculate(in, policy)
	firstJSON, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	for i := 0; i < 10; i++ {
		result := labor.Calculate(in, policy)
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
	in := fullInput()
	policy := labor.DefaultPolicy()

	want, err := json.Marshal(labor.Calculate(in, policy))
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
			got, err := json.Marshal(labor.Calculate(in, policy))
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

// TestDeterminism_InputOrderIndependence proves that reversing
// PayrollRecords/Workers input order produces an identical Result, since
// every grouping/sort in this package uses a fixed key order, never
// input order.
func TestDeterminism_InputOrderIndependence(t *testing.T) {
	in1 := fullInput()
	in2 := fullInput()
	for i, j := 0, len(in2.PayrollRecords)-1; i < j; i, j = i+1, j-1 {
		in2.PayrollRecords[i], in2.PayrollRecords[j] = in2.PayrollRecords[j], in2.PayrollRecords[i]
	}
	for i, j := 0, len(in2.Workers)-1; i < j; i, j = i+1, j-1 {
		in2.Workers[i], in2.Workers[j] = in2.Workers[j], in2.Workers[i]
	}

	r1 := labor.Calculate(in1, labor.DefaultPolicy())
	r2 := labor.Calculate(in2, labor.DefaultPolicy())

	b1, _ := json.Marshal(r1)
	b2, _ := json.Marshal(r2)
	if string(b1) != string(b2) {
		t.Error("expected identical Result regardless of caller-supplied input order")
	}
}
