package reconciliation

import (
	"testing"
	"time"
)

// TestScaling_MatcherIsSubQuadratic locks the real O(N^2) bug found via
// benchmarking during this package's own build (task section 71/73):
// matchStage originally re-scanned the entire opposite population,
// including re-normalizing every external item's Reference from
// scratch, for every single book item — confirmed via profiling to make
// runtime scale roughly as N^2 (1k -> 5k -> 10k measured at
// ~107ms/2.8s/12.1s before the fix, i.e. a ~26x jump for a 5x input
// increase). Fixed by indexing both sides once (by normalized reference
// and by amount bucket — see itemIndex in matcher.go) before any
// matching stage runs.
//
// This test is a coarse, CI-safe guard (not a precise complexity proof):
// it measures wall-clock time at two population sizes with a 4x ratio
// and asserts the LARGER size's runtime is well under quadratic growth
// (a strict O(N^2) match would show ~16x; this asserts under 8x, a
// generous middle ground that fails loudly if the O(N^2) shape returns
// but tolerates ordinary test-machine variance). It intentionally stays
// far below benchmark_test.go's own 1k/5k/10k sweep in absolute size (500/
// 2000) so it runs in well under a second even on a slow CI machine and
// never risks flaking on timing noise at larger scale.
func TestScaling_MatcherIsSubQuadratic(t *testing.T) {
	small := timeCalculate(t, 500)
	large := timeCalculate(t, 2000) // 4x population

	if small <= 0 {
		t.Skip("measured duration too small to compare reliably on this machine")
	}
	ratio := float64(large) / float64(small)
	if ratio > 8.0 {
		t.Fatalf("runtime grew %.1fx for a 4x population increase (500->2000 items each side); expected well under the ~16x a true O(N^2) shape would show — this suggests the matcher regressed back to a full pairwise scan. small=%v large=%v", ratio, small, large)
	}
}

func timeCalculate(t *testing.T, n int) time.Duration {
	t.Helper()
	book, external := buildBenchmarkPopulation(n)
	in := Input{AccountID: "SCALE", AsOfDate: "2025-02-01", BookItems: book, ExternalItems: external, Policy: benchmarkPolicy()}

	start := time.Now()
	_ = Calculate(in)
	return time.Since(start)
}
