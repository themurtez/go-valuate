package ap_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/themurtez/go-valuate/accounting/ap"
)

func generatePayables(n, suppliers int) []ap.Payable {
	base := mustDateAny("2025-01-01")
	out := make([]ap.Payable, n)
	for i := 0; i < n; i++ {
		sid := fmt.Sprintf("SUP-%d", i%suppliers)
		billDate := base.AddDate(0, 0, i%180)
		dueDate := billDate.AddDate(0, 0, 30)
		amount := float64(100 + (i % 5000))
		out[i] = ap.Payable{
			ID: fmt.Sprintf("BILL-%d", i), SupplierID: sid,
			BillDate: billDate, DueDate: dueDate,
			OriginalAmount: amount, OpenAmount: amount,
			Currency: "USD", Status: ap.StatusOpen, TermsDays: 30,
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

func BenchmarkCalculate_100kPayables_10kSuppliers(b *testing.B) {
	payables := generatePayables(100000, 10000)
	asOf := mustDateAny("2025-06-30")
	in := ap.Input{Payables: payables}
	opts := ap.Options{AsOfDate: asOf}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ap.Calculate(in, opts)
	}
}

func BenchmarkCalculate_10kPayables_1kSuppliers(b *testing.B) {
	payables := generatePayables(10000, 1000)
	asOf := mustDateAny("2025-06-30")
	in := ap.Input{Payables: payables}
	opts := ap.Options{AsOfDate: asOf}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ap.Calculate(in, opts)
	}
}

func BenchmarkCalculate_WithSnapshotsAndPayments(b *testing.B) {
	payables := generatePayables(20000, 2000)
	asOf := mustDateAny("2025-06-30")

	snapshotPayables := generatePayables(20000, 2000)
	snapshots := []ap.Snapshot{
		{AsOfDate: "2025-05-31", Payables: snapshotPayables},
	}

	payments := make([]ap.SupplierPayment, 0, 5000)
	for i := 0; i < 5000; i++ {
		payments = append(payments, ap.SupplierPayment{
			ID: fmt.Sprintf("PMT-%d", i), PayableID: fmt.Sprintf("BILL-%d", i),
			SupplierID: fmt.Sprintf("SUP-%d", i%2000), Date: mustDateAny("2025-06-01"), Amount: 100,
		})
	}

	in := ap.Input{Payables: payables, Snapshots: snapshots, Payments: payments}
	opts := ap.Options{AsOfDate: asOf}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ap.Calculate(in, opts)
	}
}

func BenchmarkConcentration_10kSuppliers(b *testing.B) {
	payables := generatePayables(50000, 10000)
	asOf := mustDateAny("2025-06-30")
	in := ap.Input{Payables: payables}
	opts := ap.Options{AsOfDate: asOf}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := ap.Calculate(in, opts)
		_ = result.Concentration
	}
}

func BenchmarkDueSchedule_100kPayables(b *testing.B) {
	payables := generatePayables(100000, 10000)
	asOf := mustDateAny("2025-06-30")
	in := ap.Input{Payables: payables}
	opts := ap.Options{AsOfDate: asOf}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := ap.Calculate(in, opts)
		_ = result.DueSchedule
	}
}

func BenchmarkHistoricalSnapshots_10Snapshots_10kPayablesEach(b *testing.B) {
	current := generatePayables(10000, 1000)
	asOf := mustDateAny("2025-12-31")

	snapshots := make([]ap.Snapshot, 0, 10)
	for m := 1; m <= 10; m++ {
		snapshots = append(snapshots, ap.Snapshot{
			AsOfDate: fmt.Sprintf("2025-%02d-28", m),
			Payables: generatePayables(10000, 1000),
		})
	}

	in := ap.Input{Payables: current, Snapshots: snapshots}
	opts := ap.Options{AsOfDate: asOf}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := ap.Calculate(in, opts)
		_ = result.AgingTrend
		_ = result.Migration
	}
}
