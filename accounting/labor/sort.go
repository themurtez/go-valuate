package labor

import "sort"

// sortedStringKeys returns the keys of m sorted ascending by normalized
// (case-folded) comparison, but returns the original key strings — the
// fixed deterministic order every grouping/breakdown in this package
// uses instead of relying on Go map iteration order. See the task's
// section 53 "departments by normalized name, locations by normalized
// name, cost centers by normalized name" instruction.
//
// Each key's normalized form is computed exactly once up front (not
// inside the sort comparator) — sort.Slice/sort.SliceStable call the
// comparator O(N log N) times, and re-normalizing both operands on every
// comparison turned this into the dominant cost at large N (this was a
// real bug caught by benchmark_test.go's large-population benchmark:
// 600,000 duplicate-detection signature keys took over 20 seconds before
// this fix, dominated by runtime.slicerunetostring/encoderune allocating
// a fresh normalized string on every comparator call).
func sortedStringKeys[V any](m map[string]V) []string {
	type keyed struct {
		raw  string
		norm string
	}
	pairs := make([]keyed, 0, len(m))
	for k := range m {
		pairs = append(pairs, keyed{raw: k, norm: normalizeGroupName(k)})
	}
	sort.SliceStable(pairs, func(i, j int) bool {
		if pairs[i].norm != pairs[j].norm {
			return pairs[i].norm < pairs[j].norm
		}
		return pairs[i].raw < pairs[j].raw
	})
	out := make([]string, len(pairs))
	for i, p := range pairs {
		out[i] = p.raw
	}
	return out
}

// normalizeGroupName lower-cases a grouping key for stable comparison
// without altering the displayed value.
func normalizeGroupName(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			r = r - 'A' + 'a'
		}
		out = append(out, r)
	}
	return string(out)
}

// sortFlags sorts flags by FlagCode declaration order, then Period.
func sortFlags(flags []Flag) {
	sort.SliceStable(flags, func(i, j int) bool {
		ri, rj := flagRank(flags[i].Code), flagRank(flags[j].Code)
		if ri != rj {
			return ri < rj
		}
		return flags[i].Period < flags[j].Period
	})
}

// sortWorkerDateFindings sorts by WorkerID then RecordID — never Go map
// order (findings are built by a single ordered pass over payroll, but
// this sort makes the order explicit/stable regardless of caller input
// order).
func sortWorkerDateFindings(findings []WorkerDateFinding) {
	sort.SliceStable(findings, func(i, j int) bool {
		a, b := findings[i], findings[j]
		if a.WorkerID != b.WorkerID {
			return a.WorkerID < b.WorkerID
		}
		return a.RecordID < b.RecordID
	})
}

// sortDuplicateGroups sorts by the first (lower) RecordID in each pair.
func sortDuplicateGroups(groups []DuplicateGroup) {
	sort.SliceStable(groups, func(i, j int) bool {
		a, b := groups[i], groups[j]
		if a.RecordIDA != b.RecordIDA {
			return a.RecordIDA < b.RecordIDA
		}
		return a.RecordIDB < b.RecordIDB
	})
}
