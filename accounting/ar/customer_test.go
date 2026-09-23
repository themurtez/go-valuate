package ar_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/ar"
)

func TestCustomer_SummariesDeterministicOrder(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	receivables := []ar.Receivable{
		recv("R-1", "C-SMALL", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), 100, 100, ar.StatusOpen),
		recv("R-2", "C-BIG", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), 5000, 5000, ar.StatusOpen),
		recv("R-3", "C-MED-B", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), 1000, 1000, ar.StatusOpen),
		recv("R-4", "C-MED-A", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), 1000, 1000, ar.StatusOpen),
	}
	result := ar.Calculate(ar.Input{Receivables: receivables}, ar.Options{AsOfDate: asOf})

	want := []string{"C-BIG", "C-MED-A", "C-MED-B", "C-SMALL"}
	if len(result.CustomerSummaries) != len(want) {
		t.Fatalf("got %d customer summaries, want %d", len(result.CustomerSummaries), len(want))
	}
	for i, cid := range want {
		if result.CustomerSummaries[i].CustomerID != cid {
			t.Errorf("position %d: got %s, want %s", i, result.CustomerSummaries[i].CustomerID, cid)
		}
	}
}

func TestCustomer_PercentUnavailableWhenTotalZero(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	result := ar.Calculate(ar.Input{Receivables: nil}, ar.Options{AsOfDate: asOf})
	if result.PortfolioSummary.PercentOverdue.Available {
		t.Errorf("expected PercentOverdue.Available=false when total AR is zero")
	}
	if result.PortfolioSummary.TotalOpenReceivables != 0 {
		t.Errorf("expected TotalOpenReceivables=0")
	}
}

func TestConcentration_OverdueRanking(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	receivables := []ar.Receivable{
		// C1: current only, no overdue.
		recv("R-1", "C1", mustDate(t, "2025-06-01"), mustDate(t, "2025-07-01"), 10000, 10000, ar.StatusOpen),
		// C2: large 90+ overdue.
		recv("R-2", "C2", mustDate(t, "2025-01-01"), mustDate(t, "2025-01-31"), 8000, 8000, ar.StatusOpen),
		// C3: small overdue.
		recv("R-3", "C3", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-05"), 100, 100, ar.StatusOpen),
	}
	result := ar.Calculate(ar.Input{Receivables: receivables}, ar.Options{AsOfDate: asOf})

	if len(result.Concentration.Overdue) == 0 {
		t.Fatalf("expected overdue ranking, got none")
	}
	if result.Concentration.Overdue[0].CustomerID != "C2" {
		t.Errorf("top overdue customer = %s, want C2", result.Concentration.Overdue[0].CustomerID)
	}
	if len(result.Concentration.Overdue90Plus) == 0 || result.Concentration.Overdue90Plus[0].CustomerID != "C2" {
		t.Errorf("expected C2 to dominate 90+ ranking, got %+v", result.Concentration.Overdue90Plus)
	}
	// C1 has no overdue, so it should not appear in the overdue ranking at all.
	for _, rc := range result.Concentration.Overdue {
		if rc.CustomerID == "C1" {
			t.Errorf("C1 should not appear in overdue ranking (has no overdue balance)")
		}
	}
}

func TestConcentration_TotalARReusesConcentrationPackage(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	receivables := []ar.Receivable{
		recv("R-1", "C1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), 9000, 9000, ar.StatusOpen),
		recv("R-2", "C2", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), 1000, 1000, ar.StatusOpen),
	}
	result := ar.Calculate(ar.Input{Receivables: receivables}, ar.Options{AsOfDate: asOf})
	if !result.Concentration.TotalAR.Available {
		t.Fatalf("expected TotalAR.Available=true")
	}
	if len(result.Concentration.TotalAR.History) != 1 {
		t.Fatalf("expected one History point (single AsOfDate snapshot), got %d", len(result.Concentration.TotalAR.History))
	}
	share := result.Concentration.TotalAR.History[0].LargestEntityShare
	if !share.Available || share.Value < 0.89 || share.Value > 0.91 {
		t.Errorf("largest entity share = %+v, want ~0.9", share)
	}
}
