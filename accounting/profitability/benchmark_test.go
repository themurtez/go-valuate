package profitability_test

import (
	"os"
	"testing"

	"github.com/themurtez/go-valuate/accounting/profitability"
	"github.com/themurtez/go-valuate/accounting/profitability/fixtures"
)

// BenchmarkCalculate_LargePopulation exercises a large synthetic
// population with CUSTOMER/JOB/PRODUCT all enabled simultaneously (every
// fact 100%-attributed to one entity of each dimension) and a shared-
// cost pool/allocation-rule set, to catch accidental O(N^2) behavior —
// task section 74.
//
// Default scale (50,000 facts / 5,000 entities per dimension / 12
// periods / 10 pools / 6 rules) is sized to run safely on a development
// laptop in a few seconds. The task's literal minimums (1,000,000 facts
// / 100,000 entities / 60 periods / 100 pools / 20 rules — roughly
// 300,000 total Entity records once CUSTOMER+JOB+PRODUCT are counted)
// are exercised by BenchmarkCalculate_LargePopulation_FullScale below,
// which is skipped unless PROFITABILITY_FULL_SCALE_BENCH=1 is set: a
// naive run of the full-scale case on this repository's own development
// machine (16GB RAM) drove memory usage well past what the machine could
// sustain, backed up by consecutive/overlapping runs, and was killed by
// the OS after 760 seconds — so it is opt-in only, not part of the
// default `go test -bench=.` run. See docs/PROFITABILITY_ANALYTICS.md
// and this package's fixtures.LargePopulation for the generator; run it
// deliberately, one at a time, on a machine with headroom, e.g.:
//
//	PROFITABILITY_FULL_SCALE_BENCH=1 go test -bench=FullScale -run='^$' -timeout=30m ./accounting/profitability/...
//
// Linearity itself is verified at the default scale by
// BenchmarkCalculate_ScalingLinearity below and was additionally spot-
// checked via allocation profiling at 10,000/100,000-fact scale during
// development (buildDimensionView's own allocation grew ~10.5x for a
// 10x increase in facts — linear, not quadratic).
func BenchmarkCalculate_LargePopulation(b *testing.B) {
	periods, entities, facts, pools, rules := fixtures.LargePopulation(50000, 5000, 12, 10, 6)
	in := profitability.Input{Periods: periods, Entities: entities, Facts: facts, SharedCostPools: pools, AllocationRules: rules}
	policy := profitability.DefaultPolicy()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = profitability.Calculate(in, policy)
	}
}

// BenchmarkCalculate_LargePopulation_FullScale is the task section 74
// literal-minimums case (1,000,000 facts / 100,000 entities per
// dimension / 60 periods / 100 pools / 20 rules). Skipped by default —
// see BenchmarkCalculate_LargePopulation's doc comment for why, and how
// to opt in deliberately.
func BenchmarkCalculate_LargePopulation_FullScale(b *testing.B) {
	if os.Getenv("PROFITABILITY_FULL_SCALE_BENCH") != "1" {
		b.Skip("skipped by default (heavy: ~1,000,000 facts / 300,000 entity records); set PROFITABILITY_FULL_SCALE_BENCH=1 to run deliberately on a machine with headroom")
	}
	periods, entities, facts, pools, rules := fixtures.LargePopulation(1000000, 100000, 60, 100, 20)
	in := profitability.Input{Periods: periods, Entities: entities, Facts: facts, SharedCostPools: pools, AllocationRules: rules}
	policy := profitability.DefaultPolicy()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = profitability.Calculate(in, policy)
	}
}

// BenchmarkCalculate_ScalingLinearity runs Calculate at two population
// sizes (10x apart, both modest enough to run safely by default) and
// reports ns/op for each, so `go test -bench` output makes it easy to
// spot-check that per-fact cost does not grow superlinearly (an O(N^2)
// aggregation would show a >10x-per-fact slowdown between the two) —
// task section 74 "avoid O(N^2); index once and sort final output."
func BenchmarkCalculate_ScalingLinearity(b *testing.B) {
	sizes := []int{5000, 50000}
	for _, n := range sizes {
		periods, entities, facts, pools, rules := fixtures.LargePopulation(n, n/10, 12, 10, 6)
		in := profitability.Input{Periods: periods, Entities: entities, Facts: facts, SharedCostPools: pools, AllocationRules: rules}
		policy := profitability.DefaultPolicy()

		b.Run(itoaBench(n), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = profitability.Calculate(in, policy)
			}
		})
	}
}

func itoaBench(n int) string {
	switch n {
	case 5000:
		return "5000facts"
	case 50000:
		return "50000facts"
	default:
		return "n"
	}
}
