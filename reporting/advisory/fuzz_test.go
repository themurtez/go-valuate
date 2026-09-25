package advisory

import (
	"math"
	"strconv"
	"testing"
)

// FuzzComputeChange targets task section 126's "current/prior comparison,
// metric change math" goal: no panic, no NaN/Inf, for any float64 pair.
func FuzzComputeChange(f *testing.F) {
	f.Add(100.0, 50.0, true)
	f.Add(0.0, 0.0, false)
	f.Add(math.MaxFloat64, 1.0, true)
	f.Add(-100.0, 100.0, false)
	f.Add(1.0, -0.0000001, true)

	f.Fuzz(func(t *testing.T, cur, prior float64, isPercent bool) {
		if math.IsNaN(cur) || math.IsInf(cur, 0) || math.IsNaN(prior) || math.IsInf(prior, 0) {
			t.Skip("advisory.Value never carries a NaN/Inf Amount in practice; this fuzz target verifies computeChange's own output given finite input")
		}
		c := computeChange(AvailableValue(cur), AvailableValue(prior), isPercent)

		if c.AbsoluteChange.Available && (math.IsNaN(c.AbsoluteChange.Amount) || math.IsInf(c.AbsoluteChange.Amount, 0)) {
			t.Errorf("AbsoluteChange is NaN/Inf for cur=%v prior=%v: %v", cur, prior, c.AbsoluteChange.Amount)
		}
		if c.PercentChange.Available && (math.IsNaN(c.PercentChange.Amount) || math.IsInf(c.PercentChange.Amount, 0)) {
			t.Errorf("PercentChange is NaN/Inf for cur=%v prior=%v: %v", cur, prior, c.PercentChange.Amount)
		}
		if c.PercentagePointChange.Available && (math.IsNaN(c.PercentagePointChange.Amount) || math.IsInf(c.PercentagePointChange.Amount, 0)) {
			t.Errorf("PercentagePointChange is NaN/Inf for cur=%v prior=%v: %v", cur, prior, c.PercentagePointChange.Amount)
		}
		// task section 10's explicit "do not fabricate percent changes
		// from zero denominator" rule.
		if prior == 0 && c.PercentChange.Available {
			t.Errorf("PercentChange must be unavailable when prior == 0, got %v", c.PercentChange)
		}
	})
}

// FuzzResolveSourcedMetric targets task section 126's "source precedence"
// goal: no panic, no nondeterministic ordering, for arbitrary candidate
// values/tolerances.
func FuzzResolveSourcedMetric(f *testing.F) {
	f.Add(47.2, 49.8, 0.01, false)
	f.Add(0.0, 0.0, 0.0, false)
	f.Add(-5.0, 5.0, 100.0, true)

	f.Fuzz(func(t *testing.T, valA, valB, tolerance float64, disableDefault bool) {
		if math.IsNaN(valA) || math.IsInf(valA, 0) || math.IsNaN(valB) || math.IsInf(valB, 0) || math.IsNaN(tolerance) || math.IsInf(tolerance, 0) {
			t.Skip("finite inputs only")
		}
		candidates := map[string]candidateValue{
			"ar":     {Value: AvailableValue(valA), SourceCode: "dso"},
			"ratios": {Value: AvailableValue(valB), SourceCode: "days_sales_outstanding"},
		}
		policy := Policy{ConflictTolerance: math.Abs(tolerance), DisableDefaultSourceOrder: disableDefault}

		// Must never panic, and must be deterministic across repeated
		// calls with identical input.
		m1, iss1 := resolveSourcedMetric(metricCodeDSO, "DSO", UnitDays, "2026-01", candidates, defaultOrder(SourceOrderDSO), policy)
		m2, iss2 := resolveSourcedMetric(metricCodeDSO, "DSO", UnitDays, "2026-01", candidates, defaultOrder(SourceOrderDSO), policy)

		if (m1 == nil) != (m2 == nil) || (iss1 == nil) != (iss2 == nil) {
			t.Fatalf("nondeterministic result across identical calls: (%v,%v) vs (%v,%v)", m1, iss1, m2, iss2)
		}
		if m1 != nil && m2 != nil && mustJSON(t, *m1) != mustJSON(t, *m2) {
			t.Fatalf("nondeterministic Metric across identical calls: %+v vs %+v", *m1, *m2)
		}
		if m1 != nil && (math.IsNaN(m1.Value.Amount) || math.IsInf(m1.Value.Amount, 0)) {
			t.Errorf("resolved Metric.Value.Amount is NaN/Inf: %v", m1.Value.Amount)
		}
	})
}

// FuzzDedupeActions targets task section 126's "action deduplication" goal:
// no panic, no duplicate action identity in output, no loss of
// provenance count.
func FuzzDedupeActions(f *testing.F) {
	f.Add(3, 2)
	f.Add(0, 0)
	f.Add(10, 1)

	f.Fuzz(func(t *testing.T, count, identityMod int) {
		if count < 0 || count > 200 || identityMod <= 0 {
			t.Skip("bounded input only — this fuzz target checks dedup invariants, not large-N performance (see bench_test.go for that)")
		}
		items := make([]ActionItem, count)
		totalRefsIn := 0
		for i := 0; i < count; i++ {
			items[i] = ActionItem{
				ActionCode: "CODE", EntityRef: strconv.Itoa(i % identityMod), Period: "P",
				SourceRefs: []SourceRef{{Module: "m", Code: strconv.Itoa(i)}},
			}
			totalRefsIn += len(items[i].SourceRefs)
		}

		out := dedupeActions(items)

		seen := make(map[ActionIdentity]bool)
		totalRefsOut := 0
		for _, a := range out {
			id := a.identity()
			if seen[id] {
				t.Fatalf("duplicate ActionIdentity in dedupeActions output: %+v", id)
			}
			seen[id] = true
			totalRefsOut += len(a.SourceRefs)
		}
		if totalRefsOut < totalRefsIn && count > 0 {
			// Every contributing SourceRef must be preserved somewhere —
			// task section 36's provenance rule. (Additional
			// SourceModule-derived refs may be appended by dedupeActions
			// itself, so totalRefsOut >= totalRefsIn is the invariant, not
			// equality.)
			t.Errorf("provenance loss: %d SourceRefs went in, only %d came out", totalRefsIn, totalRefsOut)
		}
	})
}
