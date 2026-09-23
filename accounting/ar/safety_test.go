package ar_test

import (
	"reflect"
	"testing"

	"github.com/themurtez/go-valuate/accounting/ar"
)

func TestSafety_MixedCurrencyFlaggedAndResolved(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	receivables := []ar.Receivable{
		recv("R-1", "C1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), 1000, 1000, ar.StatusOpen),
		{ID: "R-2", CustomerID: "C2", InvoiceDate: mustDate(t, "2025-06-01"), DueDate: mustDate(t, "2025-06-15"),
			OriginalAmount: 500, OpenAmount: 500, Currency: "EUR", Status: ar.StatusOpen},
	}
	receivables[0].Currency = "USD"

	result := ar.Calculate(ar.Input{Receivables: receivables}, ar.Options{AsOfDate: asOf})
	if !hasIssueCode(result.Issues, ar.IssueMixedCurrency) {
		t.Errorf("expected IssueMixedCurrency, got %+v", result.Issues)
	}
	// Only USD (more common by count, tie -> alphabetical also picks EUR<USD... verify explicit currency wins when set)
	if result.PortfolioSummary.TotalOpenReceivables != 1000 && result.PortfolioSummary.TotalOpenReceivables != 500 {
		t.Errorf("expected single-currency total, got %v", result.PortfolioSummary.TotalOpenReceivables)
	}
}

func TestSafety_MixedCurrencyExplicitReportingCurrency(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	receivables := []ar.Receivable{
		{ID: "R-1", CustomerID: "C1", InvoiceDate: mustDate(t, "2025-06-01"), DueDate: mustDate(t, "2025-06-15"),
			OriginalAmount: 1000, OpenAmount: 1000, Currency: "USD", Status: ar.StatusOpen},
		{ID: "R-2", CustomerID: "C2", InvoiceDate: mustDate(t, "2025-06-01"), DueDate: mustDate(t, "2025-06-15"),
			OriginalAmount: 500, OpenAmount: 500, Currency: "EUR", Status: ar.StatusOpen},
	}
	result := ar.Calculate(ar.Input{Receivables: receivables}, ar.Options{AsOfDate: asOf, ReportingCurrency: "EUR"})
	if result.PortfolioSummary.TotalOpenReceivables != 500 {
		t.Errorf("TotalOpenReceivables = %v, want 500 (EUR only)", result.PortfolioSummary.TotalOpenReceivables)
	}
}

func TestSafety_NoMutationOfInput(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	original := []ar.Receivable{
		recv("R-1", "C1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), 1000, 1000, ar.StatusOpen),
	}
	snapshot := make([]ar.Receivable, len(original))
	copy(snapshot, original)

	buckets := ar.DefaultBuckets()
	bucketsSnapshot := make([]ar.BucketDefinition, len(buckets))
	copy(bucketsSnapshot, buckets)

	_ = ar.Calculate(ar.Input{Receivables: original}, ar.Options{AsOfDate: asOf, Buckets: buckets})

	for i := range original {
		if !reflect.DeepEqual(original[i], snapshot[i]) {
			t.Errorf("Receivable[%d] was mutated: got %+v, want %+v", i, original[i], snapshot[i])
		}
	}
	for i := range buckets {
		if buckets[i] != bucketsSnapshot[i] {
			t.Errorf("BucketDefinition[%d] was mutated: got %+v, want %+v", i, buckets[i], bucketsSnapshot[i])
		}
	}
}

func TestSafety_Determinism(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	receivables := []ar.Receivable{
		recv("R-1", "C1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), 1000, 1000, ar.StatusOpen),
		recv("R-2", "C2", mustDate(t, "2025-04-01"), mustDate(t, "2025-04-30"), 2000, 2000, ar.StatusOpen),
		recv("R-3", "C3", mustDate(t, "2025-01-01"), mustDate(t, "2025-01-31"), 3000, 3000, ar.StatusOpen),
	}
	in := ar.Input{Receivables: receivables}
	opts := ar.Options{AsOfDate: asOf}

	first := ar.Calculate(in, opts)
	for i := 0; i < 20; i++ {
		next := ar.Calculate(in, opts)
		if next.PortfolioSummary.TotalOpenReceivables != first.PortfolioSummary.TotalOpenReceivables {
			t.Fatalf("nondeterministic TotalOpenReceivables at iteration %d", i)
		}
		if len(next.CustomerSummaries) != len(first.CustomerSummaries) {
			t.Fatalf("nondeterministic CustomerSummaries length at iteration %d", i)
		}
		for j := range next.CustomerSummaries {
			if next.CustomerSummaries[j].CustomerID != first.CustomerSummaries[j].CustomerID {
				t.Fatalf("nondeterministic CustomerSummaries order at iteration %d", i)
			}
		}
	}
}

func TestSafety_ConcurrentCalls(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	receivables := []ar.Receivable{
		recv("R-1", "C1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), 1000, 1000, ar.StatusOpen),
		recv("R-2", "C2", mustDate(t, "2025-04-01"), mustDate(t, "2025-04-30"), 2000, 2000, ar.StatusOpen),
	}
	in := ar.Input{Receivables: receivables}
	opts := ar.Options{AsOfDate: asOf}

	done := make(chan ar.Result, 50)
	for i := 0; i < 50; i++ {
		go func() {
			done <- ar.Calculate(in, opts)
		}()
	}
	var results []ar.Result
	for i := 0; i < 50; i++ {
		results = append(results, <-done)
	}
	for _, r := range results {
		if r.PortfolioSummary.TotalOpenReceivables != 3000 {
			t.Errorf("concurrent call got TotalOpenReceivables=%v, want 3000", r.PortfolioSummary.TotalOpenReceivables)
		}
	}
}
