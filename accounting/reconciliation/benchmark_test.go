package reconciliation

import (
	"os"
	"strconv"
	"testing"
)

// buildBenchmarkPopulation constructs n book items and n external items:
// ~95% straightforward one-to-one matches (unique amount/date/reference
// per pair), plus a deliberate repeated-amount ambiguity cluster and a
// bounded composite-candidate bucket — task section 73's recommended
// default shape ("10,000 book items, 10,000 external items, ~95%
// straightforward matches, repeated-amount ambiguity, bounded composite
// buckets").
func buildBenchmarkPopulation(n int) ([]BookItem, []ExternalItem) {
	book := make([]BookItem, 0, n)
	external := make([]ExternalItem, 0, n)

	straightforward := n * 95 / 100
	remaining := n - straightforward

	for i := 0; i < straightforward; i++ {
		id := strconv.Itoa(i)
		amount := float64(100 + i%5000)
		book = append(book, BookItem{
			ItemID: "B" + id, Date: "2025-01-15", Amount: amount, Direction: DirectionOutflow, Reference: "REF-" + id,
		})
		external = append(external, ExternalItem{
			ItemID: "E" + id, Date: "2025-01-15", Amount: amount, Direction: DirectionOutflow, Reference: "REF-" + id,
		})
	}

	// Repeated-amount ambiguity cluster: 20 items on each side sharing
	// one amount with no reference — bounded, not O(N) of the total
	// population, so it stresses ambiguity detection without dominating
	// runtime.
	ambiguousCount := 20
	if remaining < ambiguousCount {
		ambiguousCount = remaining
	}
	for i := 0; i < ambiguousCount; i++ {
		id := strconv.Itoa(straightforward + i)
		book = append(book, BookItem{ItemID: "B" + id, Date: "2025-01-20", Amount: 777, Direction: DirectionOutflow})
		external = append(external, ExternalItem{ItemID: "E" + id, Date: "2025-01-20", Amount: 777, Direction: DirectionOutflow})
	}
	remaining -= ambiguousCount

	// Bounded composite-candidate bucket: a handful of book items whose
	// amount equals the sum of 2 external items dated on the same day —
	// only reachable/matched when EnableCompositeMatching is on; present
	// here purely to give composite search real (bounded) candidates to
	// examine in the composite-focused benchmark below.
	for i := 0; i < remaining; i++ {
		id := strconv.Itoa(straightforward + ambiguousCount + i)
		book = append(book, BookItem{ItemID: "B" + id, Date: "2025-01-25", Amount: 300, Direction: DirectionOutflow})
		external = append(external, ExternalItem{ItemID: "E" + id + "a", Date: "2025-01-25", Amount: 100, Direction: DirectionOutflow})
		external = append(external, ExternalItem{ItemID: "E" + id + "b", Date: "2025-01-25", Amount: 200, Direction: DirectionOutflow})
	}

	return book, external
}

func benchmarkPolicy() MatchingPolicy {
	return MatchingPolicy{
		AmountTolerance:        0.01,
		DateWindowDays:         3,
		ReferenceNormalization: ReferenceNormalization{Trim: true, CaseFold: true},
	}
}

// BenchmarkCalculate_10k is the task section 73 recommended default
// scale (10,000 book items / 10,000 external items), sized to run safely
// on a development laptop in a few seconds.
func BenchmarkCalculate_10k(b *testing.B) {
	book, external := buildBenchmarkPopulation(10000)
	in := Input{AccountID: "BENCH", AsOfDate: "2025-02-01", BookItems: book, ExternalItems: external, Policy: benchmarkPolicy()}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Calculate(in)
	}
}

// BenchmarkCalculate_ScalingLinearity runs 1k -> 5k -> 10k (task section
// 73: "enough to detect O(N^2)") so `go test -bench=ScalingLinearity`
// output can be visually compared for roughly-linear ns/op growth.
func BenchmarkCalculate_ScalingLinearity(b *testing.B) {
	for _, n := range []int{1000, 5000, 10000} {
		n := n
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			book, external := buildBenchmarkPopulation(n)
			in := Input{AccountID: "BENCH", AsOfDate: "2025-02-01", BookItems: book, ExternalItems: external, Policy: benchmarkPolicy()}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = Calculate(in)
			}
		})
	}
}

// BenchmarkCalculate_CompositeEnabled exercises the same population with
// composite matching turned on, to separately verify the bounded
// composite search does not dominate runtime at this scale.
func BenchmarkCalculate_CompositeEnabled(b *testing.B) {
	book, external := buildBenchmarkPopulation(10000)
	policy := benchmarkPolicy()
	policy.EnableCompositeMatching = true
	policy.MaxCompositeGroupSize = 3
	policy.MaxCandidatesPerItem = 10
	policy.MaxCompositeSearchCombinations = 500
	in := Input{AccountID: "BENCH", AsOfDate: "2025-02-01", BookItems: book, ExternalItems: external, Policy: policy}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Calculate(in)
	}
}

// BenchmarkCalculate_FullScale is a heavier stress scale beyond the
// recommended default, gated behind RECONCILIATION_FULL_SCALE_BENCH=1
// per task section 73 ("Any larger stress benchmark must be env-gated")
// and this repository's established laptop-safety convention (see e.g.
// accounting/vendorspend/benchmark_test.go's identical gate, adopted
// after Prompt 46's stacked-concurrent-benchmark incident recorded in
// docs/VENDOR_SPEND_ANALYTICS.md). Run it deliberately, one at a time, on
// a machine with headroom, after confirming any previous benchmark
// process has actually exited (e.g. via `ps`):
//
//	RECONCILIATION_FULL_SCALE_BENCH=1 go test -bench=FullScale -run='^$' -timeout=15m ./accounting/reconciliation
func BenchmarkCalculate_FullScale(b *testing.B) {
	if os.Getenv("RECONCILIATION_FULL_SCALE_BENCH") != "1" {
		b.Skip("skipped by default (heavy: 100,000 book items / 100,000 external items); set RECONCILIATION_FULL_SCALE_BENCH=1 to run deliberately on a machine with headroom")
	}
	book, external := buildBenchmarkPopulation(100000)
	in := Input{AccountID: "BENCH", AsOfDate: "2025-02-01", BookItems: book, ExternalItems: external, Policy: benchmarkPolicy()}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Calculate(in)
	}
}
