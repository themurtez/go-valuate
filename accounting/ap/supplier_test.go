package ap_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/ap"
)

// TestSupplier_DeterministicSortOrder verifies SupplierSummaries is sorted
// by OpenAmount descending, then SupplierID ascending on ties, and that
// repeated calls against identical input always produce the same order.
func TestSupplier_DeterministicSortOrder(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	payables := []ap.Payable{
		bill("B-1", "S-B", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), 500, 500, ap.StatusOpen),
		bill("B-2", "S-A", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), 500, 500, ap.StatusOpen), // tie with S-B on amount
		bill("B-3", "S-C", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), 9000, 9000, ap.StatusOpen),
	}
	for i := 0; i < 5; i++ {
		result := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf})
		if len(result.SupplierSummaries) != 3 {
			t.Fatalf("run %d: expected 3 suppliers, got %d", i, len(result.SupplierSummaries))
		}
		if result.SupplierSummaries[0].SupplierID != "S-C" {
			t.Errorf("run %d: expected S-C first (largest balance), got %s", i, result.SupplierSummaries[0].SupplierID)
		}
		// Tie between S-A and S-B (both 500) broken alphabetically.
		if result.SupplierSummaries[1].SupplierID != "S-A" || result.SupplierSummaries[2].SupplierID != "S-B" {
			t.Errorf("run %d: expected tie broken alphabetically (S-A, S-B), got (%s, %s)", i, result.SupplierSummaries[1].SupplierID, result.SupplierSummaries[2].SupplierID)
		}
	}
}

func TestSupplier_AggregatesAcrossMultipleBills(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	payables := []ap.Payable{
		bill("B-1", "S-A", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), 1000, 1000, ap.StatusOpen),
		bill("B-2", "S-A", mustDate(t, "2025-05-01"), mustDate(t, "2025-05-15"), 2000, 2000, ap.StatusOpen),
	}
	result := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf})
	if len(result.SupplierSummaries) != 1 {
		t.Fatalf("expected 1 supplier, got %d", len(result.SupplierSummaries))
	}
	s := result.SupplierSummaries[0]
	if s.OpenAmount != 3000 {
		t.Errorf("OpenAmount = %v, want 3000", s.OpenAmount)
	}
	if s.BillCount != 2 {
		t.Errorf("BillCount = %v, want 2", s.BillCount)
	}
}

func TestSupplier_SupplierNameDisplayOnly(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	payables := []ap.Payable{
		{ID: "B-1", SupplierID: "S-A", SupplierName: "Alpha Co", BillDate: mustDate(t, "2025-06-01"), DueDate: mustDate(t, "2025-06-15"),
			OriginalAmount: 1000, OpenAmount: 1000, Currency: "USD", Status: ap.StatusOpen},
		{ID: "B-2", SupplierID: "S-A", SupplierName: "", BillDate: mustDate(t, "2025-06-01"), DueDate: mustDate(t, "2025-06-15"),
			OriginalAmount: 500, OpenAmount: 500, Currency: "USD", Status: ap.StatusOpen},
	}
	result := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf})
	if len(result.SupplierSummaries) != 1 {
		t.Fatalf("expected 1 supplier (SupplierID is identity, not name), got %d", len(result.SupplierSummaries))
	}
	if result.SupplierSummaries[0].OpenAmount != 1500 {
		t.Errorf("OpenAmount = %v, want 1500 (both rows aggregated under same SupplierID)", result.SupplierSummaries[0].OpenAmount)
	}
}
