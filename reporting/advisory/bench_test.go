package advisory

import (
	"fmt"
	"os"
	"testing"
)

// benchmarkActionSet builds n synthetic ActionItems with a mix of
// distinct and duplicate identities (roughly 1-in-4 duplicate, mirroring
// a realistic proportion of the same underlying issue surfaced through a
// few sibling adapters) — the shape benchmarkDedupe/benchmarkPriority
// scale against, task section 124's "5,000 metrics / 2,000 findings /
// 2,000 candidate actions" representative sizing (scaled down per
// sub-benchmark per Go's b.N convention).
func benchmarkActionSet(n int) []ActionItem {
	out := make([]ActionItem, n)
	for i := 0; i < n; i++ {
		identityGroup := i / 4 // every 4 consecutive items share one identity
		out[i] = ActionItem{
			ActionCode:   fmt.Sprintf("ACTION_%d", identityGroup),
			EntityRef:    fmt.Sprintf("ENTITY_%d", identityGroup%50),
			Period:       "2026-02",
			Priority:     PriorityMedium,
			SourceModule: fmt.Sprintf("module_%d", i%7),
			SourceRefs:   []SourceRef{{Module: fmt.Sprintf("module_%d", i%7), Period: "2026-02"}},
		}
	}
	return out
}

func BenchmarkDedupeActions(b *testing.B) {
	for _, n := range []int{500, 2000, 5000} {
		items := benchmarkActionSet(n)
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = dedupeActions(items)
			}
		})
	}
}

func benchmarkInsightSet(n int) []Insight {
	out := make([]Insight, n)
	for i := 0; i < n; i++ {
		out[i] = Insight{
			Code: fmt.Sprintf("CODE_%d", i), Category: string(SectionWorkingCapital),
			Severity: SeverityMedium, Priority: PriorityMedium,
			SourceModule: fmt.Sprintf("module_%d", i%7), EntityRef: fmt.Sprintf("ENTITY_%d", i%100),
		}
	}
	return out
}

func BenchmarkSortInsights(b *testing.B) {
	for _, n := range []int{500, 2000, 5000} {
		items := benchmarkInsightSet(n)
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				cp := append([]Insight(nil), items...)
				sortInsights(cp, sectionOrder)
			}
		})
	}
}

// BenchmarkBuild_FullScale is gated behind ADVISORY_FULL_SCALE_BENCH=1 —
// task section 124's "gate larger stress testing" instruction. Not run by
// default; laptop-safe.
func BenchmarkBuild_FullScale(b *testing.B) {
	if os.Getenv("ADVISORY_FULL_SCALE_BENCH") != "1" {
		b.Skip("set ADVISORY_FULL_SCALE_BENCH=1 to run the full-scale Build benchmark")
	}
	in := fullyPopulatedInput()
	policy := ExamplePolicy()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Build(in, policy)
	}
}

func BenchmarkBuild_RepresentativePack(b *testing.B) {
	in := fullyPopulatedInput()
	policy := ExamplePolicy()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Build(in, policy)
	}
}
