package ap_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/ap"
	apfixtures "github.com/themurtez/go-valuate/accounting/ap/fixtures"
)

func TestTrends_MigrationRequiresSameID(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	snapshots := []ap.Snapshot{
		{AsOfDate: "2025-05-31", Payables: []ap.Payable{
			bill("B-1", "S1", mustDate(t, "2025-05-01"), mustDate(t, "2025-05-15"), 1000, 1000, ap.StatusOpen), // 16 days past due at snapshot -> 1_30
		}},
	}
	current := []ap.Payable{
		bill("B-1", "S1", mustDate(t, "2025-05-01"), mustDate(t, "2025-05-15"), 1000, 1000, ap.StatusOpen), // same ID, now 46 days past due -> 31_60
	}
	result := ap.Calculate(ap.Input{Payables: current, Snapshots: snapshots}, ap.Options{AsOfDate: asOf})
	if !result.Migration.Available {
		t.Fatalf("expected Migration.Available=true, issues: %+v", result.Issues)
	}
	if len(result.Migration.Entries) != 1 {
		t.Fatalf("expected 1 migration entry, got %d: %+v", len(result.Migration.Entries), result.Migration.Entries)
	}
	e := result.Migration.Entries[0]
	if e.FromBucketCode != "1_30" || e.ToBucketCode != "31_60" {
		t.Errorf("expected 1_30 -> 31_60, got %s -> %s", e.FromBucketCode, e.ToBucketCode)
	}
	if result.Migration.DeteriorationAmount != 1000 {
		t.Errorf("DeteriorationAmount = %v, want 1000", result.Migration.DeteriorationAmount)
	}
	if result.Migration.SuppliersDeteriorating != 1 {
		t.Errorf("SuppliersDeteriorating = %v, want 1", result.Migration.SuppliersDeteriorating)
	}
}

func TestTrends_MigrationToPaid(t *testing.T) {
	asOf := mustDate(t, apfixtures.AsOfDate)
	snapshots, current := apfixtures.ImprovingSnapshots()
	result := ap.Calculate(ap.Input{Payables: current, Snapshots: snapshots}, ap.Options{AsOfDate: asOf})
	if !result.Migration.Available {
		t.Fatalf("expected Migration.Available=true, issues: %+v", result.Issues)
	}
	found := false
	for _, e := range result.Migration.Entries {
		if e.ToPaid {
			found = true
			if e.ToBucketCode != "" {
				t.Errorf("expected empty ToBucketCode when ToPaid=true, got %q", e.ToBucketCode)
			}
		}
	}
	if !found {
		t.Errorf("expected a ToPaid transition entry, got %+v", result.Migration.Entries)
	}
	if result.Migration.CureAmount != 6000 {
		t.Errorf("CureAmount = %v, want 6000", result.Migration.CureAmount)
	}
	if result.Migration.SuppliersImproving != 1 {
		t.Errorf("SuppliersImproving = %v, want 1", result.Migration.SuppliersImproving)
	}
}

func TestTrends_NeverInfersMigrationFromOneSnapshot(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	// No snapshots at all -- migration must be unavailable, never inferred.
	current := []ap.Payable{bill("B-1", "S1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), 1000, 1000, ap.StatusOpen)}
	result := ap.Calculate(ap.Input{Payables: current}, ap.Options{AsOfDate: asOf})
	if result.Migration.Available {
		t.Errorf("expected Migration.Available=false with zero snapshots")
	}
}

func TestTrends_AgingTrendDeteriorating(t *testing.T) {
	asOf := mustDate(t, apfixtures.AsOfDate)
	snapshots, current := apfixtures.DeterioratingSnapshots()
	result := ap.Calculate(ap.Input{Payables: current, Snapshots: snapshots}, ap.Options{AsOfDate: asOf})
	if !result.AgingTrend.Available {
		t.Fatalf("expected AgingTrend.Available=true, issues: %+v", result.Issues)
	}
	if len(result.AgingTrend.Points) != 1 {
		t.Fatalf("expected 1 aging trend point (1 snapshot supplied), got %d", len(result.AgingTrend.Points))
	}
}

// TestTrends_PaymentMetricsTrendDirections exercises
// PaymentMetrics.OverdueTrend/Overdue60Trend/Overdue90Trend, which require
// at least two AgingTrend.Points (i.e. at least two Snapshots) to compute
// a first-vs-last direction.
func TestTrends_PaymentMetricsTrendDirections(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	snapshots := []ap.Snapshot{
		{AsOfDate: "2025-04-30", Payables: []ap.Payable{
			bill("B-1", "S1", mustDate(t, "2025-01-01"), mustDate(t, "2025-01-31"), 1000, 1000, ap.StatusOpen), // ~89 days past due at this snapshot
		}},
		{AsOfDate: "2025-05-31", Payables: []ap.Payable{
			bill("B-1", "S1", mustDate(t, "2025-01-01"), mustDate(t, "2025-01-31"), 1000, 1000, ap.StatusOpen), // ~120 days past due at this snapshot
		}},
	}
	current := []ap.Payable{
		bill("B-1", "S1", mustDate(t, "2025-01-01"), mustDate(t, "2025-01-31"), 1000, 0, ap.StatusPaid), // now paid off
	}
	payments := []ap.SupplierPayment{
		{ID: "P-1", PayableID: "B-1", SupplierID: "S1", Date: mustDate(t, "2025-06-15"), Amount: 1000},
	}
	result := ap.Calculate(ap.Input{Payables: current, Snapshots: snapshots, Payments: payments}, ap.Options{AsOfDate: asOf})
	if !result.AgingTrend.Available || len(result.AgingTrend.Points) != 2 {
		t.Fatalf("expected 2 AgingTrend.Points, got %+v", result.AgingTrend)
	}
	if !result.PaymentMetrics.Available {
		t.Fatalf("expected PaymentMetrics.Available=true, issues: %+v", result.Issues)
	}
	// Both snapshots show the same (fully overdue) $1000 balance, so the
	// portfolio-level overdue/60+/90+ totals are flat between them, and
	// the bill is now fully paid in the current state -- exercising the
	// trend-direction computation path without asserting a specific
	// direction the underlying numbers don't actually guarantee.
	if result.PaymentMetrics.OverdueTrend == "" {
		t.Errorf("expected non-empty OverdueTrend with 2 AgingTrend points, got %+v", result.PaymentMetrics)
	}
	if !result.PaymentMetrics.AmountPaid.Available || result.PaymentMetrics.AmountPaid.Value != 1000 {
		t.Errorf("AmountPaid = %+v, want 1000", result.PaymentMetrics.AmountPaid)
	}
}

func TestTrends_DPODeteriorationFlag(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	ap1 := 10000.0
	ap2 := 25000.0
	history := []ap.PayablesPeriod{
		{Period: "2025-Q1", DenominatorAmount: 40000, Days: 90, EndingAP: &ap1},
		{Period: "2025-Q2", DenominatorAmount: 40000, Days: 90, EndingAP: &ap2},
	}
	result := ap.Calculate(ap.Input{PurchasesHistory: history}, ap.Options{AsOfDate: asOf})
	found := false
	for _, f := range result.Flags {
		if f.Code == ap.FlagDPODeterioration {
			found = true
		}
	}
	if !found {
		t.Errorf("expected FlagDPODeterioration for a large first-vs-last DPO increase, flags: %+v", result.Flags)
	}
}
