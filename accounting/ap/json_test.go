package ap_test

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/themurtez/go-valuate/accounting/ap"
	apfixtures "github.com/themurtez/go-valuate/accounting/ap/fixtures"
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

func TestJSON_Payable(t *testing.T) {
	p := ap.Payable{
		ID: "B-1", SupplierID: "S1", SupplierName: "Test Supplier", BillNumber: "BILL-1",
		DocumentType: ap.DocumentTypeBill,
		BillDate:     mustDate(t, "2025-06-01"), DueDate: mustDate(t, "2025-06-15"),
		OriginalAmount: 1000, OpenAmount: 500, Currency: "USD", Status: ap.StatusPartiallyPaid,
		TermsDays: 14, Dimensions: []ap.Dimension{{Key: "region", Value: "west"}},
		SourceRef: ap.SourceRef{System: "qbo", ID: "abc123"},
	}
	got := roundTrip(t, p)
	if got.ID != p.ID || got.SupplierID != p.SupplierID || got.OpenAmount != p.OpenAmount {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, p)
	}
	if len(got.Dimensions) != 1 || got.Dimensions[0].Value != "west" {
		t.Errorf("Dimensions round-trip failed: %+v", got.Dimensions)
	}
}

func TestJSON_SupplierPayment(t *testing.T) {
	sp := ap.SupplierPayment{ID: "P-1", PayableID: "B-1", SupplierID: "S1", Date: mustDate(t, "2025-06-10"), Amount: 500}
	got := roundTrip(t, sp)
	if got.ID != sp.ID || got.Amount != sp.Amount {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, sp)
	}
}

func TestJSON_SupplierSummary(t *testing.T) {
	asOf := mustDate(t, apfixtures.AsOfDate)
	result := ap.Calculate(ap.Input{Payables: apfixtures.HealthyPayables()}, ap.Options{AsOfDate: asOf})
	if len(result.SupplierSummaries) == 0 {
		t.Fatalf("expected non-empty SupplierSummaries")
	}
	got := roundTrip(t, result.SupplierSummaries)
	if len(got) != len(result.SupplierSummaries) {
		t.Fatalf("SupplierSummaries length mismatch after round-trip")
	}
	if got[0].SupplierID != result.SupplierSummaries[0].SupplierID {
		t.Errorf("SupplierID mismatch after round-trip")
	}
}

func TestJSON_DPO(t *testing.T) {
	dpo := ap.DPOResult{Available: true, Value: 42.5, Basis: ap.DPOBasisCOGS, Period: "2025"}
	got := roundTrip(t, dpo)
	if got != dpo {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, dpo)
	}
}

func TestJSON_DueSchedule(t *testing.T) {
	asOf := mustDate(t, apfixtures.AsOfDate)
	result := ap.Calculate(ap.Input{Payables: apfixtures.HealthyPayables()}, ap.Options{AsOfDate: asOf})
	got := roundTrip(t, result.DueSchedule)
	if got.Available != result.DueSchedule.Available {
		t.Errorf("Available mismatch after round-trip")
	}
	if len(got.Entries) != len(result.DueSchedule.Entries) {
		t.Errorf("Entries length mismatch after round-trip")
	}
}

func TestJSON_PaymentPressure(t *testing.T) {
	asOf := mustDate(t, apfixtures.AsOfDate)
	cash := 50000.0
	result := ap.Calculate(ap.Input{Payables: apfixtures.HealthyPayables()}, ap.Options{
		AsOfDate:        asOf,
		PaymentPressure: &ap.PaymentPressureInput{CashAvailable: &cash},
	})
	got := roundTrip(t, result.PaymentPressure)
	if got.Available != result.PaymentPressure.Available {
		t.Errorf("Available mismatch after round-trip")
	}
	if len(got.Windows) != len(result.PaymentPressure.Windows) {
		t.Errorf("Windows length mismatch after round-trip")
	}
}

func TestJSON_Reconciliations(t *testing.T) {
	asOf := mustDate(t, apfixtures.AsOfDate)
	payables, controlBalance := apfixtures.GLSubledgerMismatch()
	result := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf, ControlAccountBalance: &controlBalance})

	gotAging := roundTrip(t, result.AgingReconciliation)
	if gotAging != result.AgingReconciliation {
		t.Errorf("AgingReconciliation round-trip mismatch: got %+v, want %+v", gotAging, result.AgingReconciliation)
	}
	gotControl := roundTrip(t, result.ControlAccountReconciliation)
	if gotControl != result.ControlAccountReconciliation {
		t.Errorf("ControlAccountReconciliation round-trip mismatch: got %+v, want %+v", gotControl, result.ControlAccountReconciliation)
	}
}

func TestJSON_Issues(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	payables := []ap.Payable{bill("B-1", "S1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), -500, -500, ap.StatusOpen)}
	result := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf})
	if len(result.Issues) == 0 {
		t.Fatalf("expected non-empty Issues")
	}
	got := roundTrip(t, result.Issues)
	if len(got) != len(result.Issues) {
		t.Fatalf("Issues length mismatch after round-trip")
	}
}

func TestJSON_FullResult_NoNaNInf(t *testing.T) {
	asOf := mustDate(t, apfixtures.AsOfDate)
	// Deliberately construct a scenario with zero denominators everywhere
	// possible (zero total AP, no purchases history) to prove percentages
	// resolve to Unavailable rather than NaN/Inf.
	result := ap.Calculate(ap.Input{Payables: apfixtures.ZeroAP()}, ap.Options{AsOfDate: asOf})
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(data), "NaN") || strings.Contains(string(data), "Inf") {
		t.Fatalf("full Result JSON contains NaN/Inf: %s", data)
	}

	// Also verify encoding/json's own hard refusal never fires: no field
	// anywhere holds an actual NaN/Inf float64 in Go memory either.
	if math.IsNaN(result.PortfolioSummary.TotalOpenPayables) || math.IsInf(result.PortfolioSummary.TotalOpenPayables, 0) {
		t.Errorf("TotalOpenPayables is NaN/Inf")
	}
}

func TestJSON_SnakeCaseTags(t *testing.T) {
	p := ap.Payable{ID: "B-1", SupplierID: "S1", BillDate: mustDate(t, "2025-06-01"), DueDate: mustDate(t, "2025-06-15"), Currency: "USD", Status: ap.StatusOpen}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	s := string(data)
	for _, field := range []string{`"id"`, `"supplier_id"`, `"bill_date"`, `"due_date"`, `"currency"`, `"status"`} {
		if !strings.Contains(s, field) {
			t.Errorf("expected snake_case field %s in JSON output: %s", field, s)
		}
	}
}
