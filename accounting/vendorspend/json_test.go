package vendorspend_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/themurtez/go-valuate/accounting/vendorspend"
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

func TestJSON_Supplier(t *testing.T) {
	s := vendorspend.Supplier{
		SupplierID: "SUP-1", Name: "Acme Co", Category: "Materials", Country: "US", Active: true,
		ParentID: "SUP-0", Dependency: vendorspend.DependencyCritical, PreferredSupplier: true, ContractedSupplier: true,
		SourceRef: vendorspend.SourceRef{System: "erp", ID: "s1"},
	}
	got := roundTrip(t, s)
	if got != s {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, s)
	}
}

func TestJSON_Period(t *testing.T) {
	p := vendorspend.Period{Period: "2025-01", StartDate: mustDate(t, "2025-01-01"), EndDate: mustDate(t, "2025-01-31"), SequenceInYear: 1}
	got := roundTrip(t, p)
	if got.Period != p.Period || !got.StartDate.Equal(p.StartDate) || !got.EndDate.Equal(p.EndDate) || got.SequenceInYear != p.SequenceInYear {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, p)
	}
}

func TestJSON_SpendRecord(t *testing.T) {
	r := vendorspend.SpendRecord{
		SpendID: "SP-1", SupplierID: "SUP-1", Period: "2025-01", Date: mustDate(t, "2025-01-15"),
		Amount: 1000, Currency: "USD", Category: "Supplies", Subcategory: "Office",
		Quantity: vendorspend.AvailableValue(10), UnitPrice: vendorspend.AvailableValue(100), UnitOfMeasure: "EA",
		Description: "widgets", ReferenceID: "PO-1", SpendType: vendorspend.SpendTypeGoods, Effect: vendorspend.EffectNormal,
		Recurrence: vendorspend.RecurrenceRecurring, Commitment: vendorspend.CommitmentCommitted, Basis: vendorspend.BasisAccrual,
		ProductID: "PROD-1", Location: "WH1", Department: "Ops", CostCenter: "CC1",
		SourceRef: vendorspend.SourceRef{System: "erp", ID: "r1"},
	}
	got := roundTrip(t, r)
	if got.SpendID != r.SpendID || !got.Date.Equal(r.Date) || got.Amount != r.Amount || got.Quantity != r.Quantity {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, r)
	}
}

func TestJSON_Result_FullRoundTrip(t *testing.T) {
	result := vendorspend.Calculate(fullInput(), fullOptions())
	got := roundTrip(t, result)

	if got.SchemaVersion != result.SchemaVersion || got.FormulaVersion != result.FormulaVersion {
		t.Errorf("version mismatch: got %+v, want %+v", got, result)
	}
	if got.Available != result.Available {
		t.Errorf("Available mismatch: got %v, want %v", got.Available, result.Available)
	}
	if got.Bridge != result.Bridge {
		t.Errorf("Bridge mismatch: got %+v, want %+v", got.Bridge, result.Bridge)
	}
	if len(got.SupplierSummaries) != len(result.SupplierSummaries) {
		t.Errorf("SupplierSummaries length mismatch: got %d, want %d", len(got.SupplierSummaries), len(result.SupplierSummaries))
	}
	if len(got.Flags) != len(result.Flags) {
		t.Errorf("Flags length mismatch: got %d, want %d", len(got.Flags), len(result.Flags))
	}

	// Re-marshal both and compare byte-for-byte — the strongest possible
	// round-trip assertion for a value this large.
	b1, _ := json.Marshal(result)
	b2, _ := json.Marshal(got)
	if string(b1) != string(b2) {
		t.Errorf("re-marshaled JSON differs after round-trip")
	}
}

func mustDate(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatalf("parse date %q: %v", s, err)
	}
	return d
}
