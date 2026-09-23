package ar_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/themurtez/go-valuate/accounting/ar"
)

func generateReceivables(n, customers int) []ar.Receivable {
	base := mustDateAny("2025-01-01")
	out := make([]ar.Receivable, n)
	for i := 0; i < n; i++ {
		cid := fmt.Sprintf("CUST-%d", i%customers)
		invoiceDate := base.AddDate(0, 0, i%180)
		dueDate := invoiceDate.AddDate(0, 0, 30)
		amount := float64(100 + (i % 5000))
		out[i] = ar.Receivable{
			ID: fmt.Sprintf("INV-%d", i), CustomerID: cid,
			InvoiceDate: invoiceDate, DueDate: dueDate,
			OriginalAmount: amount, OpenAmount: amount,
			Currency: "USD", Status: ar.StatusOpen, TermsDays: 30,
		}
	}
	return out
}

func mustDateAny(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func BenchmarkCalculate_100kReceivables_10kCustomers(b *testing.B) {
	receivables := generateReceivables(100000, 10000)
	asOf := mustDateAny("2025-06-30")
	in := ar.Input{Receivables: receivables}
	opts := ar.Options{AsOfDate: asOf}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ar.Calculate(in, opts)
	}
}

func BenchmarkCalculate_10kReceivables_1kCustomers(b *testing.B) {
	receivables := generateReceivables(10000, 1000)
	asOf := mustDateAny("2025-06-30")
	in := ar.Input{Receivables: receivables}
	opts := ar.Options{AsOfDate: asOf}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ar.Calculate(in, opts)
	}
}

func BenchmarkCalculate_WithSnapshotsAndPayments(b *testing.B) {
	receivables := generateReceivables(20000, 2000)
	asOf := mustDateAny("2025-06-30")

	snapshotReceivables := generateReceivables(20000, 2000)
	snapshots := []ar.Snapshot{
		{AsOfDate: "2025-05-31", Receivables: snapshotReceivables},
	}

	payments := make([]ar.Payment, 0, 5000)
	for i := 0; i < 5000; i++ {
		payments = append(payments, ar.Payment{
			ID: fmt.Sprintf("PAY-%d", i), ReceivableID: fmt.Sprintf("INV-%d", i),
			CustomerID: fmt.Sprintf("CUST-%d", i%2000), Date: mustDateAny("2025-06-01"), Amount: 100,
		})
	}

	in := ar.Input{Receivables: receivables, Snapshots: snapshots, Payments: payments}
	opts := ar.Options{AsOfDate: asOf}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ar.Calculate(in, opts)
	}
}

func BenchmarkConcentration_10kCustomers(b *testing.B) {
	receivables := generateReceivables(50000, 10000)
	asOf := mustDateAny("2025-06-30")
	in := ar.Input{Receivables: receivables}
	opts := ar.Options{AsOfDate: asOf}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := ar.Calculate(in, opts)
		_ = result.Concentration
	}
}
