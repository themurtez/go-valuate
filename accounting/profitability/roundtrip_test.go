package profitability_test

import (
	"encoding/json"
	"testing"

	"github.com/themurtez/go-valuate/accounting/profitability"
)

func TestRoundTrip_Result(t *testing.T) {
	r := profitability.Calculate(fullInput(), profitability.DefaultPolicy())
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var r2 profitability.Result
	if err := json.Unmarshal(b, &r2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	b2, err := json.Marshal(r2)
	if err != nil {
		t.Fatalf("re-marshal: %v", err)
	}
	if string(b) != string(b2) {
		t.Error("expected Result to round-trip through JSON byte-for-byte")
	}
}

func TestRoundTrip_Entity(t *testing.T) {
	e := profitability.Entity{Dimension: profitability.DimensionCustomer, EntityID: "C1", Name: "Acme", Group: "G1", Active: true}
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var e2 profitability.Entity
	if err := json.Unmarshal(b, &e2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	b2, _ := json.Marshal(e2)
	if string(b) != string(b2) {
		t.Error("expected Entity to round-trip through JSON byte-for-byte")
	}
}

func TestRoundTrip_Fact(t *testing.T) {
	f := profitability.Fact{FactID: "F1", Period: "2025-01", Component: profitability.ComponentGrossRevenue, Amount: 100,
		Attributions: []profitability.Attribution{{Dimension: profitability.DimensionCustomer, EntityID: "C1", Share: 1}}, Currency: "USD"}
	b, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var f2 profitability.Fact
	if err := json.Unmarshal(b, &f2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	b2, _ := json.Marshal(f2)
	if string(b) != string(b2) {
		t.Error("expected Fact to round-trip through JSON byte-for-byte")
	}
}

func TestRoundTrip_Input(t *testing.T) {
	in := fullInput()
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var in2 profitability.Input
	if err := json.Unmarshal(b, &in2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	b2, _ := json.Marshal(in2)
	if string(b) != string(b2) {
		t.Error("expected Input to round-trip through JSON byte-for-byte")
	}
}

func TestRoundTrip_Policy(t *testing.T) {
	p := profitability.DefaultPolicy()
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var p2 profitability.Policy
	if err := json.Unmarshal(b, &p2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	b2, _ := json.Marshal(p2)
	if string(b) != string(b2) {
		t.Error("expected Policy to round-trip through JSON byte-for-byte")
	}
}
