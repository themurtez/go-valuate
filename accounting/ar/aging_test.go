package ar_test

import (
	"testing"
	"time"

	"github.com/themurtez/go-valuate/accounting/ar"
)

func mustDate(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatalf("bad date %q: %v", s, err)
	}
	return d
}

func recv(id, custID string, invoiceDate, dueDate time.Time, original, open float64, status ar.ReceivableStatus) ar.Receivable {
	return ar.Receivable{
		ID: id, CustomerID: custID,
		InvoiceDate: invoiceDate, DueDate: dueDate,
		OriginalAmount: original, OpenAmount: open,
		Currency: "USD", Status: status,
	}
}

func bucketAmount(t *testing.T, buckets []ar.BucketAmount, code string) float64 {
	t.Helper()
	for _, b := range buckets {
		if b.BucketCode == code {
			return b.Amount
		}
	}
	t.Fatalf("bucket %q not found", code)
	return 0
}

func TestAging_DefaultBuckets_ExactBoundaries(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")

	receivables := []ar.Receivable{
		recv("R-CURRENT", "C1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-30"), 100, 100, ar.StatusOpen), // due today -> CURRENT
		recv("R-1", "C1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-29"), 100, 100, ar.StatusOpen),       // 1 day past due -> 1_30
		recv("R-30", "C1", mustDate(t, "2025-06-01"), mustDate(t, "2025-05-31"), 100, 100, ar.StatusOpen),      // 30 days past due -> 1_30
		recv("R-31", "C1", mustDate(t, "2025-06-01"), mustDate(t, "2025-05-30"), 100, 100, ar.StatusOpen),      // 31 days past due -> 31_60
		recv("R-60", "C1", mustDate(t, "2025-06-01"), mustDate(t, "2025-05-01"), 100, 100, ar.StatusOpen),      // 60 days past due -> 31_60
		recv("R-61", "C1", mustDate(t, "2025-06-01"), mustDate(t, "2025-04-30"), 100, 100, ar.StatusOpen),      // 61 days -> 61_90
		recv("R-90", "C1", mustDate(t, "2025-06-01"), mustDate(t, "2025-04-01"), 100, 100, ar.StatusOpen),      // 90 days -> 61_90
		recv("R-91", "C1", mustDate(t, "2025-06-01"), mustDate(t, "2025-03-31"), 100, 100, ar.StatusOpen),      // 91 days -> 91_PLUS
	}

	result := ar.Calculate(ar.Input{Receivables: receivables}, ar.Options{AsOfDate: asOf})
	if !result.Available {
		t.Fatalf("expected Available, issues: %+v", result.Issues)
	}

	if got := bucketAmount(t, result.PortfolioSummary.Buckets, "CURRENT"); got != 100 {
		t.Errorf("CURRENT = %v, want 100", got)
	}
	if got := bucketAmount(t, result.PortfolioSummary.Buckets, "1_30"); got != 200 {
		t.Errorf("1_30 = %v, want 200", got)
	}
	if got := bucketAmount(t, result.PortfolioSummary.Buckets, "31_60"); got != 200 {
		t.Errorf("31_60 = %v, want 200", got)
	}
	if got := bucketAmount(t, result.PortfolioSummary.Buckets, "61_90"); got != 200 {
		t.Errorf("61_90 = %v, want 200", got)
	}
	if got := bucketAmount(t, result.PortfolioSummary.Buckets, "91_PLUS"); got != 100 {
		t.Errorf("91_PLUS = %v, want 100", got)
	}
}

func TestAging_CustomBuckets(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	receivables := []ar.Receivable{
		recv("R-1", "C1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-25"), 500, 500, ar.StatusOpen), // 5 days
		recv("R-2", "C1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-01"), 500, 500, ar.StatusOpen), // 29 days
	}
	buckets := []ar.BucketDefinition{
		{Code: "A", MinDaysPastDue: 0, MaxDaysPastDue: 10, HasMax: true},
		{Code: "B", MinDaysPastDue: 11, HasMax: false},
	}

	result := ar.Calculate(ar.Input{Receivables: receivables}, ar.Options{AsOfDate: asOf, Buckets: buckets})
	if !result.Available {
		t.Fatalf("expected Available, issues: %+v", result.Issues)
	}
	if got := bucketAmount(t, result.PortfolioSummary.Buckets, "A"); got != 500 {
		t.Errorf("A = %v, want 500", got)
	}
	if got := bucketAmount(t, result.PortfolioSummary.Buckets, "B"); got != 500 {
		t.Errorf("B = %v, want 500", got)
	}
}

func TestAging_InvoiceDateBasis(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	receivables := []ar.Receivable{
		// invoice 40 days ago, but due date only 5 days ago.
		recv("R-1", "C1", mustDate(t, "2025-05-21"), mustDate(t, "2025-06-25"), 100, 100, ar.StatusOpen),
	}

	byDue := ar.Calculate(ar.Input{Receivables: receivables}, ar.Options{AsOfDate: asOf, Basis: ar.AgingByDueDate})
	if got := bucketAmount(t, byDue.PortfolioSummary.Buckets, "1_30"); got != 100 {
		t.Errorf("due-date basis: 1_30 = %v, want 100", got)
	}

	byInvoice := ar.Calculate(ar.Input{Receivables: receivables}, ar.Options{AsOfDate: asOf, Basis: ar.AgingByInvoiceDate})
	if got := bucketAmount(t, byInvoice.PortfolioSummary.Buckets, "31_60"); got != 100 {
		t.Errorf("invoice-date basis: 31_60 = %v, want 100", got)
	}
}

func TestAging_InvalidBucketConfiguration(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	cases := []struct {
		name    string
		buckets []ar.BucketDefinition
	}{
		{"overlap", []ar.BucketDefinition{
			{Code: "A", MinDaysPastDue: 0, MaxDaysPastDue: 30, HasMax: true},
			{Code: "B", MinDaysPastDue: 20, HasMax: false},
		}},
		{"no terminal bucket", []ar.BucketDefinition{
			{Code: "A", MinDaysPastDue: 0, MaxDaysPastDue: 30, HasMax: true},
		}},
		{"duplicate code", []ar.BucketDefinition{
			{Code: "A", MinDaysPastDue: 0, MaxDaysPastDue: 30, HasMax: true},
			{Code: "A", MinDaysPastDue: 31, HasMax: false},
		}},
		{"negative min", []ar.BucketDefinition{
			{Code: "A", MinDaysPastDue: -5, HasMax: false},
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			result := ar.Calculate(ar.Input{Receivables: nil}, ar.Options{AsOfDate: asOf, Buckets: c.buckets})
			if result.Available {
				t.Fatalf("expected Available=false for invalid bucket config %q", c.name)
			}
			if !ar.HasErrors(result.Issues) {
				t.Fatalf("expected error issues for %q, got %+v", c.name, result.Issues)
			}
		})
	}
}

func TestAging_EmptyBucketsResolveToDefaults(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	result := ar.Calculate(ar.Input{Receivables: nil}, ar.Options{AsOfDate: asOf, Buckets: []ar.BucketDefinition{}})
	if !result.Available {
		t.Fatalf("expected Available=true (empty Buckets resolves to DefaultBuckets), issues: %+v", result.Issues)
	}
	if len(result.Buckets) != len(ar.DefaultBuckets()) {
		t.Errorf("got %d buckets, want %d default buckets", len(result.Buckets), len(ar.DefaultBuckets()))
	}
}

func TestAging_GapsAllowedWhenOptedIn(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	buckets := []ar.BucketDefinition{
		{Code: "A", MinDaysPastDue: 0, MaxDaysPastDue: 10, HasMax: true},
		{Code: "B", MinDaysPastDue: 20, HasMax: false}, // gap 11-19
	}
	result := ar.Calculate(ar.Input{Receivables: nil}, ar.Options{AsOfDate: asOf, Buckets: buckets, AllowBucketGaps: true})
	if !result.Available {
		t.Fatalf("expected Available with AllowBucketGaps, issues: %+v", result.Issues)
	}

	resultNoOptIn := ar.Calculate(ar.Input{Receivables: nil}, ar.Options{AsOfDate: asOf, Buckets: buckets})
	if resultNoOptIn.Available {
		t.Fatalf("expected Available=false without AllowBucketGaps")
	}
}

func TestAging_MissingAsOfDate(t *testing.T) {
	result := ar.Calculate(ar.Input{Receivables: nil}, ar.Options{})
	if result.Available {
		t.Fatalf("expected Available=false with zero AsOfDate")
	}
	if !ar.HasErrors(result.Issues) {
		t.Fatalf("expected error issue for missing AsOfDate")
	}
}

func TestAging_FutureInvoiceDate(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	receivables := []ar.Receivable{
		recv("R-1", "C1", mustDate(t, "2025-07-01"), mustDate(t, "2025-08-01"), 100, 100, ar.StatusOpen),
	}
	result := ar.Calculate(ar.Input{Receivables: receivables}, ar.Options{AsOfDate: asOf})
	found := false
	for _, iss := range result.Issues {
		if iss.Code == ar.IssueFutureInvoice {
			found = true
		}
	}
	if !found {
		t.Errorf("expected IssueFutureInvoice, got %+v", result.Issues)
	}
	// Future-dated item should be treated as DaysPastDue = 0 (CURRENT), never negative.
	if got := bucketAmount(t, result.PortfolioSummary.Buckets, "CURRENT"); got != 100 {
		t.Errorf("CURRENT = %v, want 100 for future-dated item", got)
	}
}
