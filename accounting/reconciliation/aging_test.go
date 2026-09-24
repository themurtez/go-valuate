package reconciliation

import "testing"

func TestAging_DaysOutstandingComputation(t *testing.T) {
	d, ok := daysOutstanding("2025-01-01", "2025-01-15")
	if !ok || d != 14 {
		t.Fatalf("expected 14 days, got %d ok=%v", d, ok)
	}
}

func TestAging_FutureDateReturnsUnavailable(t *testing.T) {
	_, ok := daysOutstanding("2025-02-01", "2025-01-15")
	if ok {
		t.Fatalf("expected unavailable for a date after AsOfDate")
	}
}

func TestAging_BucketAssignment(t *testing.T) {
	buckets := []AgingBucket{
		{Label: "0-30", MinDays: 0, MaxDays: 30},
		{Label: "31-60", MinDays: 31, MaxDays: 60},
		{Label: "61+", MinDays: 61},
	}
	if got := bucketFor(15, buckets); got != "0-30" {
		t.Fatalf("expected '0-30', got %q", got)
	}
	if got := bucketFor(45, buckets); got != "31-60" {
		t.Fatalf("expected '31-60', got %q", got)
	}
	if got := bucketFor(200, buckets); got != "61+" {
		t.Fatalf("expected '61+' (open-ended), got %q", got)
	}
}

func TestAging_StaleFindingsRequireExplicitThreshold(t *testing.T) {
	aged := []AgedItem{{ItemID: "B1", DaysOutstanding: 90}}
	findings := staleUnmatchedFindings(aged, nil, 0)
	if findings != nil {
		t.Fatalf("expected no stale findings with a zero threshold (no universal default), got %+v", findings)
	}
	findings2 := staleUnmatchedFindings(aged, nil, 60)
	if len(findings2) != 1 || findings2[0].Code != FindingStaleUnmatchedItem {
		t.Fatalf("expected 1 FindingStaleUnmatchedItem, got %+v", findings2)
	}
}

func TestAging_ReconcilingItemStaleness(t *testing.T) {
	aged := []AgedItem{{ItemID: "R1", DaysOutstanding: 100}}
	findings := staleReconcilingFindings(aged, 30)
	if len(findings) != 1 || findings[0].Code != FindingStaleReconcilingItem {
		t.Fatalf("expected 1 FindingStaleReconcilingItem, got %+v", findings)
	}
}
