package ap_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/ap"
	apfixtures "github.com/themurtez/go-valuate/accounting/ap/fixtures"
)

func TestConcentration_OneLargeSupplierDominatesTotalAP(t *testing.T) {
	asOf := mustDate(t, apfixtures.AsOfDate)
	result := ap.Calculate(ap.Input{Payables: apfixtures.OneLargeOverdueSupplier()}, ap.Options{AsOfDate: asOf})
	if !result.Available {
		t.Fatalf("expected Available=true, issues: %+v", result.Issues)
	}
	if !result.Concentration.TotalAP.Available {
		t.Fatalf("expected Concentration.TotalAP.Available=true")
	}
	if len(result.Concentration.TotalAP.History) != 1 {
		t.Fatalf("expected 1 concentration period, got %d", len(result.Concentration.TotalAP.History))
	}
	share := result.Concentration.TotalAP.History[0].LargestEntityShare
	if !share.Available || share.Value < 0.9 {
		t.Errorf("expected largest supplier share > 0.9, got %+v", share)
	}
	if result.Concentration.Label == "" {
		t.Errorf("expected non-empty Concentration.Label disclaiming operational dependency")
	}
}

func TestConcentration_OverdueRankingSeparateFromTotalAP(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	payables := []ap.Payable{
		// S-A has the largest total balance but it's all current.
		bill("B-1", "S-A", mustDate(t, "2025-06-15"), mustDate(t, "2025-08-15"), 50000, 50000, ap.StatusOpen),
		// S-B has a smaller total balance but it's all overdue.
		bill("B-2", "S-B", mustDate(t, "2025-01-01"), mustDate(t, "2025-02-01"), 10000, 10000, ap.StatusOpen),
	}
	result := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf})
	if len(result.Concentration.Overdue) == 0 {
		t.Fatalf("expected non-empty Overdue ranking")
	}
	if result.Concentration.Overdue[0].SupplierID != "S-B" {
		t.Errorf("expected S-B to rank first in overdue concentration (only overdue supplier), got %s", result.Concentration.Overdue[0].SupplierID)
	}
}

func TestConcentration_60And90PlusRankings(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	payables := []ap.Payable{
		bill("B-1", "S-90", mustDate(t, "2025-01-01"), mustDate(t, "2025-01-31"), 5000, 5000, ap.StatusOpen), // ~150 days past due
		bill("B-2", "S-30", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-10"), 3000, 3000, ap.StatusOpen), // ~20 days past due
	}
	result := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf})
	found90 := false
	for _, r := range result.Concentration.Overdue90Plus {
		if r.SupplierID == "S-90" {
			found90 = true
		}
		if r.SupplierID == "S-30" {
			t.Errorf("S-30 should not appear in Overdue90Plus (only ~20 days past due)")
		}
	}
	if !found90 {
		t.Errorf("expected S-90 in Overdue90Plus ranking")
	}
}

func TestConcentration_ManySmallSuppliersLowConcentration(t *testing.T) {
	asOf := mustDate(t, apfixtures.AsOfDate)
	result := ap.Calculate(ap.Input{Payables: apfixtures.ManySmallSuppliers()}, ap.Options{AsOfDate: asOf})
	share := result.Concentration.TotalAP.History[0].LargestEntityShare
	if !share.Available || share.Value > 0.2 {
		t.Errorf("expected low concentration (largest share < 0.2) among 10 similarly-sized suppliers, got %+v", share)
	}
}
