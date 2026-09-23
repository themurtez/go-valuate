package ap_test

import (
	"testing"
	"time"

	"github.com/themurtez/go-valuate/accounting/ap"
)

func mustDate(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatalf("mustDate(%q): %v", s, err)
	}
	return d
}

func bill(id, supplierID string, billDate, dueDate time.Time, original, open float64, status ap.PayableStatus) ap.Payable {
	return ap.Payable{
		ID: id, SupplierID: supplierID, BillDate: billDate, DueDate: dueDate,
		OriginalAmount: original, OpenAmount: open, Currency: "USD", Status: status,
	}
}

func hasIssueCode(issues []ap.Issue, code ap.IssueCode) bool {
	for _, i := range issues {
		if i.Code == code {
			return true
		}
	}
	return false
}

func bucketAmount(buckets []ap.BucketAmount, code string) float64 {
	for _, b := range buckets {
		if b.BucketCode == code {
			return b.Amount
		}
	}
	return -1 // sentinel: bucket code not found.
}

// TestAging_DefaultBucketsExactBoundaries exercises every default bucket
// and its exact day-count boundary: 0 (CURRENT), 1 and 30 (1_30), 31 and
// 60 (31_60), 61 and 90 (61_90), 91 (91_PLUS).
func TestAging_DefaultBucketsExactBoundaries(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	cases := []struct {
		daysPastDue int
		wantBucket  string
	}{
		{0, "CURRENT"},
		{1, "1_30"},
		{30, "1_30"},
		{31, "31_60"},
		{60, "31_60"},
		{61, "61_90"},
		{90, "61_90"},
		{91, "91_PLUS"},
		{365, "91_PLUS"},
	}
	for _, c := range cases {
		dueDate := asOf.AddDate(0, 0, -c.daysPastDue)
		payables := []ap.Payable{bill("B-1", "S1", dueDate.AddDate(0, 0, -30), dueDate, 1000, 1000, ap.StatusOpen)}
		result := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf})
		if !result.Available {
			t.Fatalf("daysPastDue=%d: expected Available=true, issues: %+v", c.daysPastDue, result.Issues)
		}
		amt := bucketAmount(result.PortfolioSummary.Buckets, c.wantBucket)
		if amt != 1000 {
			t.Errorf("daysPastDue=%d: expected bucket %s to have amount 1000, got %v (buckets=%+v)", c.daysPastDue, c.wantBucket, amt, result.PortfolioSummary.Buckets)
		}
	}
}

// TestAging_FutureDatedClampedToCurrent verifies a not-yet-due bill (due
// date after AsOfDate) is clamped to DaysPastDue=0, landing in CURRENT.
func TestAging_FutureDatedClampedToCurrent(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	payables := []ap.Payable{bill("B-1", "S1", mustDate(t, "2025-06-01"), mustDate(t, "2025-08-01"), 1000, 1000, ap.StatusOpen)}
	result := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf})
	if bucketAmount(result.PortfolioSummary.Buckets, "CURRENT") != 1000 {
		t.Errorf("expected future-dated bill in CURRENT bucket, got %+v", result.PortfolioSummary.Buckets)
	}
}

// TestAging_CustomBuckets exercises a caller-supplied bucket schema.
func TestAging_CustomBuckets(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	custom := []ap.BucketDefinition{
		{Code: "A", MinDaysPastDue: 0, MaxDaysPastDue: 15, HasMax: true},
		{Code: "B", MinDaysPastDue: 16, HasMax: false},
	}
	payables := []ap.Payable{
		bill("B-1", "S1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-20"), 500, 500, ap.StatusOpen), // 10 days past due -> A
		bill("B-2", "S1", mustDate(t, "2025-05-01"), mustDate(t, "2025-06-01"), 700, 700, ap.StatusOpen), // 29 days past due -> B
	}
	result := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf, Buckets: custom})
	if !result.Available {
		t.Fatalf("expected Available=true, issues: %+v", result.Issues)
	}
	if bucketAmount(result.PortfolioSummary.Buckets, "A") != 500 {
		t.Errorf("bucket A = %v, want 500", bucketAmount(result.PortfolioSummary.Buckets, "A"))
	}
	if bucketAmount(result.PortfolioSummary.Buckets, "B") != 700 {
		t.Errorf("bucket B = %v, want 700", bucketAmount(result.PortfolioSummary.Buckets, "B"))
	}
}

// TestAging_InvalidBucketConfiguration_Overlap verifies overlapping
// buckets are rejected and Result.Available is false.
func TestAging_InvalidBucketConfiguration_Overlap(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	bad := []ap.BucketDefinition{
		{Code: "A", MinDaysPastDue: 0, MaxDaysPastDue: 30, HasMax: true},
		{Code: "B", MinDaysPastDue: 20, HasMax: false}, // overlaps A
	}
	result := ap.Calculate(ap.Input{Payables: []ap.Payable{bill("B-1", "S1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-10"), 100, 100, ap.StatusOpen)}}, ap.Options{AsOfDate: asOf, Buckets: bad})
	if result.Available {
		t.Fatalf("expected Available=false for overlapping buckets")
	}
	if !hasIssueCode(result.Issues, ap.IssueInvalidBucketConfiguration) {
		t.Errorf("expected IssueInvalidBucketConfiguration, got %+v", result.Issues)
	}
}

// TestAging_InvalidBucketConfiguration_Gap verifies a gap between buckets
// is rejected unless AllowBucketGaps is set.
func TestAging_InvalidBucketConfiguration_Gap(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	gapped := []ap.BucketDefinition{
		{Code: "A", MinDaysPastDue: 0, MaxDaysPastDue: 10, HasMax: true},
		{Code: "B", MinDaysPastDue: 20, HasMax: false}, // gap: 11-19 uncovered
	}
	result := ap.Calculate(ap.Input{Payables: []ap.Payable{bill("B-1", "S1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-10"), 100, 100, ap.StatusOpen)}}, ap.Options{AsOfDate: asOf, Buckets: gapped})
	if result.Available {
		t.Fatalf("expected Available=false for gapped buckets without AllowBucketGaps")
	}

	result2 := ap.Calculate(ap.Input{Payables: []ap.Payable{bill("B-1", "S1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-10"), 100, 100, ap.StatusOpen)}}, ap.Options{AsOfDate: asOf, Buckets: gapped, AllowBucketGaps: true})
	if !result2.Available {
		t.Fatalf("expected Available=true when AllowBucketGaps=true, issues: %+v", result2.Issues)
	}
}

// TestAging_MissingAsOfDate verifies AsOfDate is required.
func TestAging_MissingAsOfDate(t *testing.T) {
	result := ap.Calculate(ap.Input{Payables: []ap.Payable{bill("B-1", "S1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-10"), 100, 100, ap.StatusOpen)}}, ap.Options{})
	if result.Available {
		t.Fatalf("expected Available=false with zero-value AsOfDate")
	}
	if !hasIssueCode(result.Issues, ap.IssueInvalidDate) {
		t.Errorf("expected IssueInvalidDate, got %+v", result.Issues)
	}
}

// TestAging_ItemFallsInAllowedGap verifies a payable whose DaysPastDue
// falls inside a gap the caller explicitly allowed (AllowBucketGaps) is
// excluded from bucket totals rather than crashing or being silently
// assigned to the wrong bucket.
func TestAging_ItemFallsInAllowedGap(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	gapped := []ap.BucketDefinition{
		{Code: "A", MinDaysPastDue: 0, MaxDaysPastDue: 10, HasMax: true},
		{Code: "B", MinDaysPastDue: 20, HasMax: false}, // gap: 11-19 uncovered
	}
	// 15 days past due -- falls squarely in the allowed gap.
	payables := []ap.Payable{bill("B-1", "S1", mustDate(t, "2025-06-01"), asOf.AddDate(0, 0, -15), 1000, 1000, ap.StatusOpen)}
	result := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf, Buckets: gapped, AllowBucketGaps: true})
	if !result.Available {
		t.Fatalf("expected Available=true, issues: %+v", result.Issues)
	}
	if bucketAmount(result.PortfolioSummary.Buckets, "A") != 0 || bucketAmount(result.PortfolioSummary.Buckets, "B") != 0 {
		t.Errorf("expected the gap-falling payable in neither bucket, got %+v", result.PortfolioSummary.Buckets)
	}
}

// TestAging_BillDateBasis exercises AgingByBillDate.
func TestAging_BillDateBasis(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	// Bill date 45 days before AsOfDate, due date only 5 days before —
	// bill-date aging should put this in 31_60, due-date aging in 1_30.
	payables := []ap.Payable{bill("B-1", "S1", asOf.AddDate(0, 0, -45), asOf.AddDate(0, 0, -5), 1000, 1000, ap.StatusOpen)}

	byDue := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf, Basis: ap.AgingByDueDate})
	if bucketAmount(byDue.PortfolioSummary.Buckets, "1_30") != 1000 {
		t.Errorf("due-date basis: expected 1_30=1000, got %+v", byDue.PortfolioSummary.Buckets)
	}

	byBill := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf, Basis: ap.AgingByBillDate})
	if bucketAmount(byBill.PortfolioSummary.Buckets, "31_60") != 1000 {
		t.Errorf("bill-date basis: expected 31_60=1000, got %+v", byBill.PortfolioSummary.Buckets)
	}
}
