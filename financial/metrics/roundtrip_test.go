package metrics

import (
	"encoding/json"
	"testing"

	"github.com/themurtez/go-valuate/financial"
)

// TestResult_JSONRoundTrip proves metrics.Result marshals, unmarshals, and
// re-marshals to byte-identical output — the same Marshal-Unmarshal-Marshal
// pattern every sibling package's own roundtrip_test.go uses. This package
// previously only had determinism_test.go (Marshal-succeeds plus map-key-
// ordering), never a full round trip through Unmarshal.
func TestResult_JSONRoundTrip(t *testing.T) {
	ds := financial.FinancialDataset{
		Currency: "USD",
		Items: []financial.NormalizedItem{
			{Code: financial.CodeRevProduct, Period: "2024", Amount: 1000000},
			{Code: financial.CodeRevProduct, Period: "2025", Amount: 1100000},
			{Code: financial.CodeCogsMaterial, Period: "2024", Amount: 400000},
			{Code: financial.CodeCogsMaterial, Period: "2025", Amount: 440000},
			{Code: financial.CodeOpexPayroll, Period: "2024", Amount: 200000},
			{Code: financial.CodeOpexPayroll, Period: "2025", Amount: 220000},
			{Code: financial.CodeOpexOwnerComp, Period: "2025", Amount: 90000},
		},
	}
	periodMeta := map[financial.Period]PeriodInfo{
		"2024": {Type: PeriodTypeFiscalYear, FiscalYear: 2024},
		"2025": {Type: PeriodTypeFiscalYear, FiscalYear: 2025},
	}
	res := Calculate(ds, Options{PeriodMeta: periodMeta})

	first, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	if !json.Valid(first) {
		t.Fatal("expected valid JSON output")
	}

	var roundTripped Result
	if err := json.Unmarshal(first, &roundTripped); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	second, err := json.Marshal(roundTripped)
	if err != nil {
		t.Fatalf("re-marshal failed: %v", err)
	}
	if string(first) != string(second) {
		t.Fatal("Result did not round-trip byte-for-byte through marshal -> unmarshal -> marshal")
	}
}

// TestSnapshot_JSONRoundTrip proves a single Snapshot (the type most
// callers actually persist per-period, via Result.SnapshotFor) round-trips
// independently of the enclosing Result — including its Results
// map[string]MetricResult field.
func TestSnapshot_JSONRoundTrip(t *testing.T) {
	ds := financial.FinancialDataset{
		Currency: "USD",
		Items: []financial.NormalizedItem{
			{Code: financial.CodeRevProduct, Period: "2025", Amount: 1000000},
			{Code: financial.CodeCogsMaterial, Period: "2025", Amount: 400000},
			{Code: financial.CodeOpexPayroll, Period: "2025", Amount: 200000},
			{Code: financial.CodeDepreciation, Period: "2025", Amount: 20000},
		},
	}
	res := Calculate(ds, Options{})
	snap, ok := res.SnapshotFor("2025")
	if !ok {
		t.Fatalf("test setup error: expected a 2025 snapshot, got %+v", res)
	}

	first, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var roundTripped Snapshot
	if err := json.Unmarshal(first, &roundTripped); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	second, err := json.Marshal(roundTripped)
	if err != nil {
		t.Fatalf("re-marshal failed: %v", err)
	}
	if string(first) != string(second) {
		t.Fatal("Snapshot did not round-trip byte-for-byte through marshal -> unmarshal -> marshal")
	}
}
