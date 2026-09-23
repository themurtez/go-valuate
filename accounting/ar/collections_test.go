package ar_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/ar"
)

func TestWriteOffs_SummaryByPeriodAndCustomer(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	writeOffs := []ar.WriteOff{
		{ID: "WO-1", CustomerID: "C1", Date: mustDate(t, "2025-03-15"), Amount: 1000, Period: "2025-Q1"},
		{ID: "WO-2", CustomerID: "C2", Date: mustDate(t, "2025-04-10"), Amount: 500, Period: "2025-Q2"},
		{ID: "WO-3", CustomerID: "C1", Date: mustDate(t, "2025-05-01"), Amount: 250, Period: "2025-Q2"},
	}
	result := ar.Calculate(ar.Input{Receivables: nil, WriteOffs: writeOffs}, ar.Options{AsOfDate: asOf})

	if !result.WriteOffs.Available {
		t.Fatalf("expected WriteOffs.Available=true")
	}
	if result.WriteOffs.TotalAmount != 1750 {
		t.Errorf("TotalAmount = %v, want 1750", result.WriteOffs.TotalAmount)
	}
	if len(result.WriteOffs.ByPeriod) != 2 {
		t.Errorf("expected 2 periods, got %d: %+v", len(result.WriteOffs.ByPeriod), result.WriteOffs.ByPeriod)
	}
	if len(result.WriteOffs.ByCustomer) != 2 {
		t.Errorf("expected 2 customers, got %d: %+v", len(result.WriteOffs.ByCustomer), result.WriteOffs.ByCustomer)
	}
	// C1 has 1250 total (1000+250), should rank first.
	if result.WriteOffs.ByCustomer[0].CustomerID != "C1" || result.WriteOffs.ByCustomer[0].Amount != 1250 {
		t.Errorf("expected C1 with 1250 first, got %+v", result.WriteOffs.ByCustomer[0])
	}
}

func TestWriteOffs_UnavailableWhenEmpty(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	result := ar.Calculate(ar.Input{Receivables: nil}, ar.Options{AsOfDate: asOf})
	if result.WriteOffs.Available {
		t.Errorf("expected WriteOffs.Available=false with no write-off records")
	}
}

func TestWriteOffs_PortfolioSummaryTracksWrittenOffStatus(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	receivables := []ar.Receivable{
		recv("R-1", "C1", mustDate(t, "2025-01-01"), mustDate(t, "2025-01-31"), 5000, 5000, ar.StatusWrittenOff),
	}
	result := ar.Calculate(ar.Input{Receivables: receivables}, ar.Options{AsOfDate: asOf})
	if result.PortfolioSummary.WrittenOffAmount != 5000 {
		t.Errorf("WrittenOffAmount = %v, want 5000", result.PortfolioSummary.WrittenOffAmount)
	}
	// Written-off receivables are excluded from aging totals by default.
	if result.PortfolioSummary.TotalOpenReceivables != 0 {
		t.Errorf("TotalOpenReceivables = %v, want 0 (written-off excluded from aging)", result.PortfolioSummary.TotalOpenReceivables)
	}
}

func TestTermsAnalysis_AverageAndWeighted(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	receivables := []ar.Receivable{
		{ID: "R-1", CustomerID: "C1", InvoiceDate: mustDate(t, "2025-06-01"), DueDate: mustDate(t, "2025-06-15"),
			OriginalAmount: 1000, OpenAmount: 1000, Currency: "USD", Status: ar.StatusOpen, TermsDays: 14},
		{ID: "R-2", CustomerID: "C1", InvoiceDate: mustDate(t, "2025-06-01"), DueDate: mustDate(t, "2025-07-01"),
			OriginalAmount: 3000, OpenAmount: 3000, Currency: "USD", Status: ar.StatusOpen, TermsDays: 30},
	}
	result := ar.Calculate(ar.Input{Receivables: receivables}, ar.Options{AsOfDate: asOf})

	if !result.TermsAnalysis.Available {
		t.Fatalf("expected TermsAnalysis.Available=true")
	}
	// simple average = (14+30)/2 = 22
	if result.TermsAnalysis.AverageTermsDays.Value != 22 {
		t.Errorf("AverageTermsDays = %v, want 22", result.TermsAnalysis.AverageTermsDays.Value)
	}
	// weighted by original amount: (14*1000 + 30*3000) / 4000 = (14000+90000)/4000 = 26
	if result.TermsAnalysis.WeightedAverageTermsDays.Value != 26 {
		t.Errorf("WeightedAverageTermsDays = %v, want 26", result.TermsAnalysis.WeightedAverageTermsDays.Value)
	}
}

func TestDimensionSummary_Breakdown(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	receivables := []ar.Receivable{
		{ID: "R-1", CustomerID: "C1", InvoiceDate: mustDate(t, "2025-06-01"), DueDate: mustDate(t, "2025-06-15"),
			OriginalAmount: 1000, OpenAmount: 1000, Currency: "USD", Status: ar.StatusOpen,
			Dimensions: []ar.Dimension{{Key: ar.DimensionRegion, Value: "west"}}},
		{ID: "R-2", CustomerID: "C2", InvoiceDate: mustDate(t, "2025-06-01"), DueDate: mustDate(t, "2025-06-15"),
			OriginalAmount: 500, OpenAmount: 500, Currency: "USD", Status: ar.StatusOpen,
			Dimensions: []ar.Dimension{{Key: ar.DimensionRegion, Value: "east"}}},
	}
	result := ar.Calculate(ar.Input{Receivables: receivables}, ar.Options{AsOfDate: asOf, Dimension: ar.DimensionRegion})

	if !result.Dimension.Available {
		t.Fatalf("expected Dimension.Available=true")
	}
	if len(result.Dimension.Values) != 2 {
		t.Fatalf("expected 2 dimension values, got %d", len(result.Dimension.Values))
	}
	if result.Dimension.Values[0].Value != "west" || result.Dimension.Values[0].OpenAmount != 1000 {
		t.Errorf("expected west=1000 first, got %+v", result.Dimension.Values[0])
	}
}

func TestDimensionSummary_UnavailableWhenNotSelected(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	result := ar.Calculate(ar.Input{Receivables: nil}, ar.Options{AsOfDate: asOf})
	if result.Dimension.Available {
		t.Errorf("expected Dimension.Available=false when Options.Dimension is empty")
	}
}

func TestCollectionMetrics_OverdueTrendDirections(t *testing.T) {
	// Two snapshots with growing overdue balances (a second receivable
	// added between snapshots), so OverdueTotal itself increases —
	// distinct from which bucket an unchanged balance falls into.
	snapshots := []ar.Snapshot{
		{AsOfDate: "2025-04-30", Receivables: []ar.Receivable{
			recv("R-1", "C1", mustDate(t, "2025-01-01"), mustDate(t, "2025-01-31"), 1000, 1000, ar.StatusOpen),
		}},
		{AsOfDate: "2025-05-31", Receivables: []ar.Receivable{
			recv("R-1", "C1", mustDate(t, "2025-01-01"), mustDate(t, "2025-01-31"), 1000, 1000, ar.StatusOpen),
			recv("R-2", "C2", mustDate(t, "2025-01-01"), mustDate(t, "2025-01-31"), 2000, 2000, ar.StatusOpen),
		}},
	}
	current := []ar.Receivable{
		recv("R-1", "C1", mustDate(t, "2025-01-01"), mustDate(t, "2025-01-31"), 1000, 1000, ar.StatusOpen),
		recv("R-2", "C2", mustDate(t, "2025-01-01"), mustDate(t, "2025-01-31"), 2000, 2000, ar.StatusOpen),
		recv("R-3", "C3", mustDate(t, "2025-01-01"), mustDate(t, "2025-01-31"), 3000, 3000, ar.StatusOpen),
	}
	asOf := mustDate(t, "2025-06-30")

	result := ar.Calculate(ar.Input{Receivables: current, Snapshots: snapshots}, ar.Options{AsOfDate: asOf})
	if !result.CollectionMetrics.Available {
		t.Fatalf("expected CollectionMetrics.Available=true")
	}
	if result.CollectionMetrics.OverdueTrend != "deteriorating" {
		t.Errorf("OverdueTrend = %v, want deteriorating", result.CollectionMetrics.OverdueTrend)
	}
}

func TestAmountValue_UnavailableConstructor(t *testing.T) {
	v := ar.Unavailable()
	if v.Available {
		t.Errorf("expected Unavailable() to have Available=false")
	}
	if v.Value != 0 {
		t.Errorf("expected Unavailable() to have Value=0, got %v", v.Value)
	}
}
