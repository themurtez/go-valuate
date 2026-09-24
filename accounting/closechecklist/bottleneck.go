package closechecklist

import "math/bits"

// computeDownstreamRequiredCounts computes, for every applicable task,
// how many other Required applicable tasks transitively depend on it
// (section 28's optional DownstreamRequiredTaskCount).
//
// A correct exact count needs each task's full downstream *set*, not
// just a number: naively summing direct dependents' own counts
// double-counts any task reachable through more than one path (a
// diamond-shaped dependency graph). This function still runs in
// O(V*W + E) time and O(V*W) space, where W = ceil(V/64) machine words,
// by representing each task's downstream set as a bitset (one bit per
// task index) instead of a map[string]bool: unioning two bitsets is a
// word-at-a-time OR rather than a per-element map insert, and counting
// set bits is a hardware popcount. Tasks are processed in reverse
// dependency-respecting order (evaluationOrder, direct dependents before
// the tasks they depend on) via dynamic programming — a task's
// downstream bitset is the OR of its direct dependents' own bitsets,
// with those dependents' own bits set. Every task's bitset is computed
// exactly once.
//
// A naive BFS-per-source version of this (retained in git history) was
// O(V^2) on a long dependency chain — see benchmark_test.go's scaling
// sweep, which exists specifically to catch a regression back to that.
func computeDownstreamRequiredCounts(v validated, evals map[string]taskEval) map[string]int {
	n := len(v.taskOrder)
	index := make(map[string]int, n)
	for i, code := range v.taskOrder {
		index[code] = i
	}

	// Build reverse adjacency: task -> tasks that depend on it.
	reverse := make(map[string][]string, n)
	for _, taskCode := range v.taskOrder {
		for _, dep := range v.graph.edges[taskCode] {
			reverse[dep.DependsOnTaskCode] = append(reverse[dep.DependsOnTaskCode], taskCode)
		}
	}

	words := (n + 63) / 64
	bitset := func() []uint64 { return make([]uint64, words) }
	setBit := func(bs []uint64, i int) { bs[i/64] |= 1 << uint(i%64) }
	unionInto := func(dst, src []uint64) {
		for i := range dst {
			dst[i] |= src[i]
		}
	}
	popcount := func(bs []uint64) int {
		c := 0
		for _, w := range bs {
			c += bits.OnesCount64(w)
		}
		return c
	}

	order := evaluationOrder(v) // dependency-respecting: dependencies appear before dependents
	downstream := make(map[string][]uint64, n)

	for i := len(order) - 1; i >= 0; i-- {
		code := order[i]
		set := bitset()
		for _, dependent := range reverse[code] {
			setBit(set, index[dependent])
			if depSet, ok := downstream[dependent]; ok {
				unionInto(set, depSet)
			}
		}
		downstream[code] = set
	}

	// Only Required+APPLICABLE tasks count toward the total — mask those
	// out with one shared bitset rather than re-checking eligibility bit
	// by bit for every task.
	eligible := bitset()
	for code, e := range evals {
		if e.def.Required && e.applicability == ApplicableYes {
			setBit(eligible, index[code])
		}
	}

	counts := make(map[string]int, n)
	masked := bitset()
	for _, code := range v.taskOrder {
		for i := range masked {
			masked[i] = downstream[code][i] & eligible[i]
		}
		counts[code] = popcount(masked)
	}
	return counts
}

// buildBottleneckSummary computes section 28's factual counts from the
// final evaluations.
func buildBottleneckSummary(evals []taskEval, downstream map[string]int) BottleneckSummary {
	var s BottleneckSummary
	for _, e := range evals {
		if e.applicability != ApplicableYes {
			continue
		}
		if e.state.Status == TaskBlocked || e.effectiveBlocking {
			s.BlockedTaskCount++
		}
		if downstream[e.def.TaskCode] > 0 && e.effectiveBlocking {
			s.TasksBlockingOthers++
		}
		for _, b := range e.blockers {
			if !b.EffectiveBlocking {
				continue
			}
			switch b.ReasonCode {
			case BlockerGateFailed, BlockerGateUnavailable:
				s.ExternalGateBlockers++
			case BlockerEvidenceMissing:
				s.EvidenceBlockers++
			case BlockerSignOffMissing:
				s.SignOffBlockers++
			}
		}
	}
	return s
}
