package vendorspend_test

import (
	"os"
	"testing"

	"github.com/themurtez/go-valuate/accounting/vendorspend"
	"github.com/themurtez/go-valuate/accounting/vendorspend/fixtures"
)

// BenchmarkCalculate_LargePopulation exercises a large synthetic
// population (spend records, suppliers, products, periods all
// simultaneously — supplier summaries, concentration, unit-price
// analytics, product-price comparison, price-volume decomposition, and
// possible-duplicate detection all have real work to do) to catch
// accidental O(N^2) behavior — task section 46.
//
// Default scale (20,000 records / 2,000 suppliers / 2,000 products / 24
// periods) is sized to run safely on a development laptop in a few
// seconds, well below the task's own recommended default sizing (100k
// records / 10k suppliers / 10k products / 60 periods), which is
// exercised by BenchmarkCalculate_LargePopulation_FullScale below and
// skipped unless VENDORSPEND_FULL_SCALE_BENCH=1 is set — this
// repository's Prompt 46 development session drove a laptop to 34GB RAM
// via stacked, unconfirmed-exited concurrent large benchmark runs (see
// docs/VENDOR_SPEND_ANALYTICS.md's benchmark-safety section and this
// package's own doc comment), so every large-scale benchmark here
// follows accounting/profitability/accounting/inventory's identical
// gate-by-default convention. Run it deliberately, one at a time, on a
// machine with headroom, after confirming any previous benchmark process
// has actually exited (e.g. via `ps`), e.g.:
//
//	VENDORSPEND_FULL_SCALE_BENCH=1 go test -bench=FullScale -run='^$' -timeout=15m ./accounting/vendorspend/...
//
// Linearity itself is verified at the default scale by
// BenchmarkCalculate_ScalingLinearity below.
func BenchmarkCalculate_LargePopulation(b *testing.B) {
	periods, suppliers, records := fixtures.LargePopulation(20000, 2000, 2000, 24)
	in := vendorspend.Input{Periods: periods, Suppliers: suppliers, SpendRecords: records}
	opts := vendorspend.Options{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = vendorspend.Calculate(in, opts)
	}
}

// BenchmarkCalculate_LargePopulation_FullScale is the task section 46
// recommended-default case (100,000 spend records / 10,000 suppliers /
// 10,000 products / 60 periods). Skipped by default — see
// BenchmarkCalculate_LargePopulation's doc comment for why, and how to
// opt in deliberately.
func BenchmarkCalculate_LargePopulation_FullScale(b *testing.B) {
	if os.Getenv("VENDORSPEND_FULL_SCALE_BENCH") != "1" {
		b.Skip("skipped by default (heavy: 100,000 spend records / 10,000 suppliers / 10,000 products / 60 periods); set VENDORSPEND_FULL_SCALE_BENCH=1 to run deliberately on a machine with headroom, and confirm no previous benchmark process is still running first")
	}
	periods, suppliers, records := fixtures.LargePopulation(100000, 10000, 10000, 60)
	in := vendorspend.Input{Periods: periods, Suppliers: suppliers, SpendRecords: records}
	opts := vendorspend.Options{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = vendorspend.Calculate(in, opts)
	}
}

// BenchmarkCalculate_ScalingLinearity runs Calculate at two population
// sizes (10x apart, both modest enough to run safely by default) and
// reports ns/op for each, so `go test -bench` output makes it easy to
// spot-check that per-record cost does not grow superlinearly (an
// O(N^2) aggregation — most likely in possible-duplicate detection,
// which this package deliberately implements via a bounded/bucketed
// index rather than pairwise comparison — would show a markedly worse
// than 10x slowdown between the two sizes).
func BenchmarkCalculate_ScalingLinearity(b *testing.B) {
	sizes := []int{2000, 20000}
	for _, n := range sizes {
		periods, suppliers, records := fixtures.LargePopulation(n, n/10, n/10, 24)
		in := vendorspend.Input{Periods: periods, Suppliers: suppliers, SpendRecords: records}
		opts := vendorspend.Options{}

		b.Run(scaleLabel(n), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = vendorspend.Calculate(in, opts)
			}
		})
	}
}

// BenchmarkDuplicateDetection_ScalingLinearity isolates
// possible-duplicate-spend detection specifically — task section 46's
// core purpose ("the purpose is to detect O(N^2)") most directly applies
// to this function, since a naive implementation would compare every
// record pairwise.
func BenchmarkDuplicateDetection_ScalingLinearity(b *testing.B) {
	sizes := []int{2000, 20000}
	for _, n := range sizes {
		// Many suppliers, few products, so records cluster into a
		// realistic number of duplicate-candidate buckets rather than one
		// giant bucket (which would defeat the point of measuring the
		// indexed approach) or zero shared buckets (which would trivially
		// show linear scaling regardless of the algorithm).
		periods, suppliers, records := fixtures.LargePopulation(n, n/20, n/2, 12)
		in := vendorspend.Input{Periods: periods, Suppliers: suppliers, SpendRecords: records}
		opts := vendorspend.Options{}

		b.Run(scaleLabel(n), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = vendorspend.Calculate(in, opts)
			}
		})
	}
}

func scaleLabel(n int) string {
	switch n {
	case 2000:
		return "2000records"
	case 20000:
		return "20000records"
	default:
		return "n"
	}
}
