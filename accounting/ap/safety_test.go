package ap_test

import (
	"encoding/json"
	"reflect"
	"sync"
	"testing"

	"github.com/themurtez/go-valuate/accounting/ap"
	apfixtures "github.com/themurtez/go-valuate/accounting/ap/fixtures"
)

func TestSafety_MixedCurrencyFlaggedAndResolved(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	payables := []ap.Payable{
		bill("B-1", "S1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), 1000, 1000, ap.StatusOpen),
		{ID: "B-2", SupplierID: "S2", BillDate: mustDate(t, "2025-06-01"), DueDate: mustDate(t, "2025-06-15"),
			OriginalAmount: 500, OpenAmount: 500, Currency: "EUR", Status: ap.StatusOpen},
	}
	payables[0].Currency = "USD"

	result := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf})
	if !hasIssueCode(result.Issues, ap.IssueMixedCurrency) {
		t.Errorf("expected IssueMixedCurrency, got %+v", result.Issues)
	}
	if result.PortfolioSummary.TotalOpenPayables != 1000 && result.PortfolioSummary.TotalOpenPayables != 500 {
		t.Errorf("expected single-currency total, got %v", result.PortfolioSummary.TotalOpenPayables)
	}
}

func TestSafety_MixedCurrencyExplicitReportingCurrency(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	payables := []ap.Payable{
		bill("B-1", "S1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), 1000, 1000, ap.StatusOpen),
		{ID: "B-2", SupplierID: "S2", BillDate: mustDate(t, "2025-06-01"), DueDate: mustDate(t, "2025-06-15"),
			OriginalAmount: 500, OpenAmount: 500, Currency: "EUR", Status: ap.StatusOpen},
	}
	result := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf, ReportingCurrency: "EUR"})
	if result.PortfolioSummary.TotalOpenPayables != 500 {
		t.Errorf("TotalOpenPayables = %v, want 500 (EUR only)", result.PortfolioSummary.TotalOpenPayables)
	}
}

func TestSafety_MixedCurrencyIssueOrReject(t *testing.T) {
	asOf := mustDate(t, apfixtures.AsOfDate)
	result := ap.Calculate(ap.Input{Payables: apfixtures.MixedCurrency()}, ap.Options{AsOfDate: asOf})
	if !result.Available {
		t.Fatalf("expected Available=true (issue, not reject, per task section 19's 'issue/reject' option)")
	}
	if !hasIssueCode(result.Issues, ap.IssueMixedCurrency) {
		t.Errorf("expected IssueMixedCurrency, got %+v", result.Issues)
	}
}

func TestSafety_NoMutationOfInput(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	original := []ap.Payable{
		bill("B-1", "S1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), 1000, 1000, ap.StatusOpen),
	}
	snapshot := make([]ap.Payable, len(original))
	copy(snapshot, original)

	buckets := ap.DefaultBuckets()
	bucketsSnapshot := make([]ap.BucketDefinition, len(buckets))
	copy(bucketsSnapshot, buckets)

	horizons := ap.DefaultDueScheduleHorizons()
	horizonsSnapshot := make([]ap.DueScheduleHorizon, len(horizons))
	copy(horizonsSnapshot, horizons)

	payments := []ap.SupplierPayment{{ID: "P-1", PayableID: "B-1", SupplierID: "S1", Date: mustDate(t, "2025-06-10"), Amount: 500}}
	paymentsSnapshot := make([]ap.SupplierPayment, len(payments))
	copy(paymentsSnapshot, payments)

	_ = ap.Calculate(ap.Input{Payables: original, Payments: payments}, ap.Options{AsOfDate: asOf, Buckets: buckets, DueScheduleHorizons: horizons})

	for i := range original {
		if !reflect.DeepEqual(original[i], snapshot[i]) {
			t.Errorf("Payable[%d] was mutated: got %+v, want %+v", i, original[i], snapshot[i])
		}
	}
	for i := range buckets {
		if !reflect.DeepEqual(buckets[i], bucketsSnapshot[i]) {
			t.Errorf("BucketDefinition[%d] was mutated: got %+v, want %+v", i, buckets[i], bucketsSnapshot[i])
		}
	}
	for i := range horizons {
		if !reflect.DeepEqual(horizons[i], horizonsSnapshot[i]) {
			t.Errorf("DueScheduleHorizon[%d] was mutated: got %+v, want %+v", i, horizons[i], horizonsSnapshot[i])
		}
	}
	for i := range payments {
		if !reflect.DeepEqual(payments[i], paymentsSnapshot[i]) {
			t.Errorf("SupplierPayment[%d] was mutated: got %+v, want %+v", i, payments[i], paymentsSnapshot[i])
		}
	}
}

func TestSafety_NoMutationOfSnapshots(t *testing.T) {
	asOf := mustDate(t, apfixtures.AsOfDate)
	snapshots, current := apfixtures.DeterioratingSnapshots()

	before, err := json.Marshal(snapshots)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	_ = ap.Calculate(ap.Input{Payables: current, Snapshots: snapshots}, ap.Options{AsOfDate: asOf})

	after, err := json.Marshal(snapshots)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(before) != string(after) {
		t.Errorf("Snapshots were mutated:\nbefore: %s\nafter:  %s", before, after)
	}
}

// TestSafety_DeterministicRepeatedExecution proves identical input always
// produces byte-for-byte identical JSON output across repeated calls.
func TestSafety_DeterministicRepeatedExecution(t *testing.T) {
	asOf := mustDate(t, apfixtures.AsOfDate)
	in := ap.Input{
		Payables: apfixtures.AgingHeavyPayables(),
		Payments: []ap.SupplierPayment{{ID: "P-1", PayableID: "BILL-2001", SupplierID: "SUP-X", Date: mustDate(t, "2025-06-20"), Amount: 1000}},
	}
	opts := ap.Options{AsOfDate: asOf}

	first := ap.Calculate(in, opts)
	firstJSON, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	for i := 0; i < 10; i++ {
		result := ap.Calculate(in, opts)
		resultJSON, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		if string(resultJSON) != string(firstJSON) {
			t.Fatalf("run %d: output diverged from first run", i)
		}
	}
}

// TestSafety_ConcurrentCalls proves Calculate can be called concurrently
// against identical input without data races or inconsistent results (run
// with -race in verification).
func TestSafety_ConcurrentCalls(t *testing.T) {
	asOf := mustDate(t, apfixtures.AsOfDate)
	in := ap.Input{Payables: apfixtures.AgingHeavyPayables()}
	opts := ap.Options{AsOfDate: asOf}

	const workers = 20
	var wg sync.WaitGroup
	results := make([]ap.Result, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx] = ap.Calculate(in, opts)
		}(i)
	}
	wg.Wait()

	first, err := json.Marshal(results[0])
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	for i := 1; i < workers; i++ {
		other, err := json.Marshal(results[i])
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		if string(other) != string(first) {
			t.Errorf("worker %d produced a different result under concurrent execution", i)
		}
	}
}
