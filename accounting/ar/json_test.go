package ar_test

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/themurtez/go-valuate/accounting/ar"
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

func TestJSON_Receivable(t *testing.T) {
	r := ar.Receivable{
		ID: "R-1", CustomerID: "C1", CustomerName: "Test Co", InvoiceNumber: "INV-1",
		DocumentType: ar.DocumentTypeInvoice,
		InvoiceDate:  mustDate(t, "2025-06-01"), DueDate: mustDate(t, "2025-06-15"),
		OriginalAmount: 1000, OpenAmount: 500, Currency: "USD", Status: ar.StatusPartiallyPaid,
		TermsDays: 14, Dimensions: []ar.Dimension{{Key: "region", Value: "west"}},
		SourceRef: ar.SourceRef{System: "qbo", ID: "abc123"},
	}
	got := roundTrip(t, r)
	if got.ID != r.ID || got.CustomerID != r.CustomerID || got.OpenAmount != r.OpenAmount {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, r)
	}
	if len(got.Dimensions) != 1 || got.Dimensions[0].Value != "west" {
		t.Errorf("Dimensions round-trip failed: %+v", got.Dimensions)
	}
}

func TestJSON_Payment(t *testing.T) {
	p := ar.Payment{ID: "P-1", ReceivableID: "R-1", CustomerID: "C1", Date: mustDate(t, "2025-06-10"), Amount: 500}
	got := roundTrip(t, p)
	if got.ID != p.ID || got.Amount != p.Amount {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, p)
	}
}

func TestJSON_BucketDefinition(t *testing.T) {
	got := roundTrip(t, ar.DefaultBuckets())
	if len(got) != len(ar.DefaultBuckets()) {
		t.Fatalf("round-trip length mismatch")
	}
}

func TestJSON_FullResult_NoNaNInf(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	receivables := []ar.Receivable{
		recv("R-1", "C1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), 1000, 1000, ar.StatusOpen),
	}
	result := ar.Calculate(ar.Input{Receivables: receivables}, ar.Options{AsOfDate: asOf})
	got := roundTrip(t, result)
	if got.SchemaVersion != result.SchemaVersion {
		t.Errorf("SchemaVersion round-trip mismatch")
	}
	if got.PortfolioSummary.TotalOpenReceivables != result.PortfolioSummary.TotalOpenReceivables {
		t.Errorf("PortfolioSummary round-trip mismatch")
	}
}

func TestJSON_EmptyResult_NoNaNInf(t *testing.T) {
	// Zero total AR: every Percent/AmountValue should be Unavailable, never
	// serialize as NaN/Inf.
	asOf := mustDate(t, "2025-06-30")
	result := ar.Calculate(ar.Input{Receivables: nil}, ar.Options{AsOfDate: asOf})
	_ = roundTrip(t, result)

	if result.PortfolioSummary.PercentOverdue.Available {
		t.Errorf("expected PercentOverdue unavailable for zero AR")
	}
}

// TestJSON_NaNRejectedByEncoding documents that Go's encoding/json itself
// refuses to marshal a NaN/Inf float64 (returns an error rather than
// emitting invalid JSON) — the underlying safety net beneath this
// package's own "unavailable, never NaN/Inf" convention (AmountValue.
// Available distinguishes "computed as exactly 0" from "not computed" so
// call sites never need to construct a NaN AvailableAmount in the first
// place).
func TestJSON_NaNRejectedByEncoding(t *testing.T) {
	v := ar.AvailableAmount(math.NaN())
	if _, err := json.Marshal(v); err == nil {
		t.Fatalf("expected json.Marshal to reject NaN")
	}
}

func TestJSON_CustomerSummary(t *testing.T) {
	cs := ar.CustomerSummary{
		CustomerID: "C1", CustomerName: "Test", OpenAmount: 100,
		Buckets: []ar.BucketAmount{{BucketCode: "CURRENT", Amount: 100, Percent: ar.AvailableAmount(1), InvoiceCount: 1}},
	}
	got := roundTrip(t, cs)
	if got.CustomerID != cs.CustomerID || got.OpenAmount != cs.OpenAmount {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, cs)
	}
}

func TestJSON_Issue(t *testing.T) {
	iss := ar.Issue{Code: ar.IssueInvalidAmount, Severity: ar.SeverityWarning, Message: "test", ReceivableID: "R-1"}
	got := roundTrip(t, iss)
	if got != iss {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, iss)
	}
}

func TestJSON_MigrationResult(t *testing.T) {
	mr := ar.MigrationResult{
		Available: true, FromAsOfDate: "2025-05-31", ToAsOfDate: "2025-06-30",
		Entries: []ar.MigrationEntry{{FromBucketCode: "CURRENT", ToBucketCode: "1_30", Count: 2, Amount: 500}},
	}
	got := roundTrip(t, mr)
	if len(got.Entries) != 1 || got.Entries[0].Amount != 500 {
		t.Errorf("round-trip mismatch: got %+v", got)
	}
}
