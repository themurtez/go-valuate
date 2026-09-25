package advisory

import "testing"

// TestBuild_NeverMutatesPolicy confirms Build/resolvePolicy never mutate
// the caller's own Policy value (its maps/slices) — task section 100.
func TestBuild_NeverMutatesPolicy(t *testing.T) {
	policy := Policy{
		SeverityWeights:   map[Severity]int{SeverityHigh: 5},
		PriorityRules:     []PriorityRule{{MatchSourceModule: "ar", ForcePriority: PriorityHigh}},
		SourcePreferences: []SourcePreference{{MetricCode: metricCodeDSO, OrderedSources: []string{"ar"}}},
		CategoryOrder:     []SectionCode{SectionLiquidity},
	}
	before := mustJSON(t, policy)

	_ = Build(Input{Company: CompanyContext{CurrentPeriod: "2026-01"}}, policy)

	after := mustJSON(t, policy)
	if before != after {
		t.Fatalf("Build mutated its Policy:\n--- before ---\n%s\n--- after ---\n%s", before, after)
	}
}

// TestDedupeActions_NeverMutatesInputSlice confirms dedupeActions returns
// a fresh slice/maps, never aliasing the caller's own ActionItem.SourceRefs
// backing array (a later append onto the merged record must never corrupt
// the original caller-owned item).
func TestDedupeActions_NeverMutatesInputSlice(t *testing.T) {
	original := ActionItem{
		ActionCode: "X", EntityRef: "E", Period: "P",
		SourceRefs: []SourceRef{{Module: "a"}},
	}
	items := []ActionItem{original, {ActionCode: "X", EntityRef: "E", Period: "P", SourceModule: "b"}}

	_ = dedupeActions(items)

	if len(original.SourceRefs) != 1 || original.SourceRefs[0].Module != "a" {
		t.Fatalf("dedupeActions mutated the original caller-owned SourceRefs slice: %+v", original.SourceRefs)
	}
}

// TestResolvePolicy_NeverAliasesCallerSlices confirms resolvePolicy
// returns fresh backing arrays for every slice field, not aliases into
// the caller's own Policy.
func TestResolvePolicy_NeverAliasesCallerSlices(t *testing.T) {
	rules := []PriorityRule{{MatchSourceModule: "ar", ForcePriority: PriorityHigh}}
	policy := Policy{PriorityRules: rules}

	resolved := resolvePolicy(policy)
	resolved.PriorityRules[0].ForcePriority = PriorityCritical

	if rules[0].ForcePriority != PriorityHigh {
		t.Fatalf("resolvePolicy aliased the caller's PriorityRules backing array: mutation leaked back, got %v", rules[0].ForcePriority)
	}
}
