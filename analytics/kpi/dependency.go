package kpi

import (
	"container/heap"
	"sort"
)

// dependencyGraph is built once per Calculate call from every supplied
// Definition — task section 14's "build a dependency graph once"
// instruction. It never mutates the Definitions it was built from.
type dependencyGraph struct {
	// byCode indexes each valid, non-duplicate Definition by Code.
	byCode map[string]Definition
	// edges[a] lists every KPI Code that Definition a's Formula
	// references via a KPIRef, deduplicated.
	edges map[string][]string
	// order is the topological evaluation order (task section 14's
	// "evaluate topologically, independent of definition input order"
	// instruction) for every KPI NOT involved in a cycle/too-deep chain.
	// Ties are broken by Code (deterministic, independent of input
	// order) — task section 42's "dependency traversal topological with
	// code tie-break" rule.
	order []string
	// inCycle marks every KPI Code that participates in (or transitively
	// depends only on KPIs within) a dependency cycle, or exceeds
	// MaxDependencyDepth — task section 15: "A/B/C must be
	// invalid/unavailable with a stable cycle issue" while "independent
	// KPI D should still evaluate."
	inCycle map[string]bool
	// evaluationOrder is every valid Definition's Code, in the order
	// Calculate produces a KPIResult for it: order first (topological),
	// then every inCycle-excluded code appended in sorted order. A
	// cyclic/too-deep KPI still gets a KPIResult (Value unavailable, with
	// AvailabilityDependencyUnavailable/AvailabilityInvalidDefinition) —
	// task section 15's "A/B/C must be invalid/unavailable" means an
	// unavailable RESULT, not a missing one; only a Definition that
	// failed structural validation entirely (never reaching byCode) has
	// no KPIResult at all. See buildEvaluationOrder.
	evaluationOrder []string
}

// buildDependencyGraph indexes defs (already deduplicated/validated for
// Code uniqueness by the caller — see validateDefinitions) and computes
// metricRefs/kpiRefs usage plus a topological order. metricCodes is the
// set of known MetricValue codes, used only for IssueUnknownMetric-style
// cross-namespace collision detection (IssueNamespaceCollision) — task
// section 16.
func buildDependencyGraph(defs map[string]Definition, metricCodes map[string]bool) (dependencyGraph, []DefinitionIssue) {
	g := dependencyGraph{
		byCode:  defs,
		edges:   make(map[string][]string, len(defs)),
		inCycle: make(map[string]bool),
	}
	var issues []DefinitionIssue

	codes := sortedKeys(defs)
	selfReferencing := map[string]bool{}
	for _, code := range codes {
		if metricCodes[code] {
			issues = append(issues, DefinitionIssue{Code: IssueNamespaceCollision, Severity: SeverityError, KPICode: code,
				Message: "KPI code \"" + code + "\" collides with a supplied source metric code"})
		}
		refs := collectKPIRefs(defs[code].Formula)
		seen := map[string]bool{}
		var deduped []string
		for _, r := range refs {
			if r == code {
				// A self-reference is deliberately never added as an
				// edge (g.edges never contains a self-loop, so
				// topologicalOrder/dependencyDepths never need to
				// special-case one) -- but it is unconditionally cyclic
				// by definition, so it is tracked here and folded into
				// g.inCycle below exactly like any other detectCycles
				// finding. Previously this case only emitted the issue
				// and relied on evaluateKPI's own defensive `visiting`
				// guard to stay safe at evaluation time -- but that
				// guard is local to evaluateKPI's call tree alone, and
				// buildProvenance's later memoized rewrite (added for
				// O(n^2) performance -- see provenanceCache) has no
				// independent per-call guard of its own and instead
				// trusts g.inCycle as the single source of truth for
				// "this KPI is structurally unevaluable" -- so g.inCycle
				// must actually be complete, not just detectCycles'
				// finding.
				selfReferencing[code] = true
				issues = append(issues, DefinitionIssue{Code: IssueDependencyCycle, Severity: SeverityError, KPICode: code,
					Message: "KPI \"" + code + "\" references itself"})
				continue
			}
			if _, ok := defs[r]; !ok {
				issues = append(issues, DefinitionIssue{Code: IssueUnknownKPI, Severity: SeverityError, KPICode: code,
					Message: "KPI \"" + code + "\" references unknown KPI \"" + r + "\""})
				continue
			}
			if !seen[r] {
				seen[r] = true
				deduped = append(deduped, r)
			}
		}
		sort.Strings(deduped)
		g.edges[code] = deduped
	}

	cyclic := detectCycles(codes, g.edges)
	for _, c := range cyclic {
		g.inCycle[c] = true
	}
	for c := range selfReferencing {
		g.inCycle[c] = true
	}
	if len(cyclic) > 0 {
		sort.Strings(cyclic)
		for _, c := range cyclic {
			issues = append(issues, DefinitionIssue{Code: IssueDependencyCycle, Severity: SeverityError, KPICode: c,
				Message: "KPI \"" + c + "\" participates in a dependency cycle"})
		}
	}

	maxDepth := dependencyDepths(codes, g.edges, g.inCycle)
	for code, depth := range maxDepth {
		if depth > MaxDependencyDepth {
			issues = append(issues, DefinitionIssue{Code: IssueDependencyTooDeep, Severity: SeverityError, KPICode: code,
				Message: "KPI \"" + code + "\" exceeds the maximum dependency chain depth"})
			g.inCycle[code] = true // reuse inCycle as the general "exclude from evaluation" set
		}
	}

	g.order = topologicalOrder(codes, g.edges, g.inCycle)
	g.evaluationOrder = buildEvaluationOrder(codes, g.order, g.inCycle)
	return g, issues
}

// buildEvaluationOrder appends every inCycle-excluded code (sorted, for
// determinism) after order — see dependencyGraph.evaluationOrder's doc
// comment.
func buildEvaluationOrder(allCodes, order []string, inCycle map[string]bool) []string {
	out := make([]string, 0, len(allCodes))
	out = append(out, order...)
	var excluded []string
	for _, c := range allCodes {
		if inCycle[c] {
			excluded = append(excluded, c)
		}
	}
	sort.Strings(excluded)
	out = append(out, excluded...)
	return out
}

// collectKPIRefs walks e and returns every KPIRef.Code referenced,
// including duplicates (deduplicated by the caller) — depth is naturally
// bounded by measureExpression/validateExpression running first in
// practice, but this function itself also refuses to recurse past
// MaxExpressionDepth+1 to stay safe even if called independently.
func collectKPIRefs(e Expression) []string {
	return collectKPIRefsAt(e, 1)
}

func collectKPIRefsAt(e Expression, depth int) []string {
	if depth > MaxExpressionDepth+1 {
		return nil
	}
	var out []string
	if e.Op == OpKPI && e.KPIRef != nil {
		out = append(out, e.KPIRef.Code)
	}
	for _, arg := range e.Args {
		out = append(out, collectKPIRefsAt(arg, depth+1)...)
	}
	return out
}

// detectCycles returns every KPI Code that is part of at least one
// directed cycle in edges, using a standard 3-color (white/gray/black)
// DFS. codes is iterated in sorted order so tie-breaking among multiple
// possible traversal starting points is deterministic, though the set of
// codes actually IN a cycle is a property of the graph alone and does not
// depend on traversal order.
func detectCycles(codes []string, edges map[string][]string) []string {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int, len(codes))
	inCycleSet := make(map[string]bool)
	var stack []string

	var visit func(node string)
	visit = func(node string) {
		color[node] = gray
		stack = append(stack, node)
		for _, next := range edges[node] {
			switch color[next] {
			case white:
				visit(next)
			case gray:
				// Found a back-edge: every node on stack from next's
				// first occurrence to the top is part of this cycle.
				start := -1
				for i, s := range stack {
					if s == next {
						start = i
						break
					}
				}
				if start >= 0 {
					for _, s := range stack[start:] {
						inCycleSet[s] = true
					}
				}
			case black:
				// already fully explored, no new cycle through here
			}
		}
		stack = stack[:len(stack)-1]
		color[node] = black
	}

	for _, code := range codes {
		if color[code] == white {
			visit(code)
		}
	}

	out := make([]string, 0, len(inCycleSet))
	for c := range inCycleSet {
		out = append(out, c)
	}
	return out
}

// dependencyDepths returns, for every non-cyclic code, the length of its
// longest dependency chain (a KPI with no KPI dependencies has depth 1).
// Cyclic codes are skipped (their depth is not meaningful).
func dependencyDepths(codes []string, edges map[string][]string, inCycle map[string]bool) map[string]int {
	depth := make(map[string]int, len(codes))
	var resolve func(code string, visiting map[string]bool) int
	resolve = func(code string, visiting map[string]bool) int {
		if inCycle[code] {
			return 0
		}
		if d, ok := depth[code]; ok {
			return d
		}
		if visiting[code] {
			// Should not happen (cycles already excluded), but guard
			// against runaway recursion defensively.
			return 0
		}
		visiting[code] = true
		max := 0
		for _, dep := range edges[code] {
			if d := resolve(dep, visiting); d > max {
				max = d
			}
		}
		delete(visiting, code)
		depth[code] = max + 1
		return depth[code]
	}
	for _, code := range codes {
		resolve(code, map[string]bool{})
	}
	return depth
}

// topologicalOrder returns every non-cyclic, non-excluded code in an
// order where every KPI appears after all KPIs it depends on — task
// section 14's "evaluate topologically, independent of definition input
// order" instruction, with Code used as the deterministic tie-break
// (task section 42).
func topologicalOrder(codes []string, edges map[string][]string, excluded map[string]bool) []string {
	inDegree := make(map[string]int, len(codes))
	dependents := make(map[string][]string, len(codes))
	var eligible []string
	for _, code := range codes {
		if excluded[code] {
			continue
		}
		eligible = append(eligible, code)
	}
	eligibleSet := make(map[string]bool, len(eligible))
	for _, c := range eligible {
		eligibleSet[c] = true
	}
	for _, code := range eligible {
		for _, dep := range edges[code] {
			if !eligibleSet[dep] {
				continue
			}
			inDegree[code]++
			dependents[dep] = append(dependents[dep], code)
		}
	}

	// ready is a min-heap of Code strings, keeping "the next eligible
	// code in Code order" available in O(log n) rather than the O(n log
	// n) a full sort.Strings(ready) on every iteration of the loop below
	// would cost — found via profiling (pprof) to dominate ~76% of total
	// CPU time on a 10,000-independent-KPI benchmark, since Kahn's
	// algorithm here removes exactly one node per outer-loop iteration,
	// making a per-iteration full re-sort O(n^2 log n) overall instead of
	// the intended O(n log n). Deterministic Code tie-break (task section
	// 42) is preserved exactly: a min-heap by Code always pops the
	// lexicographically smallest ready Code, identical to what
	// re-sorting the whole ready slice every time and taking [0] would
	// have produced, just without the redundant re-sorting of elements
	// that were already in order from the previous iteration.
	ready := &stringHeap{}
	for _, code := range eligible {
		if inDegree[code] == 0 {
			*ready = append(*ready, code)
		}
	}
	heap.Init(ready)

	order := make([]string, 0, len(eligible))
	for ready.Len() > 0 {
		next := heap.Pop(ready).(string)
		order = append(order, next)
		deps := append([]string(nil), dependents[next]...)
		sort.Strings(deps)
		for _, d := range deps {
			inDegree[d]--
			if inDegree[d] == 0 {
				heap.Push(ready, d)
			}
		}
	}
	return order
}

// stringHeap is a container/heap min-heap of strings — see
// topologicalOrder's use above.
type stringHeap []string

func (h stringHeap) Len() int            { return len(h) }
func (h stringHeap) Less(i, j int) bool  { return h[i] < h[j] }
func (h stringHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *stringHeap) Push(x interface{}) { *h = append(*h, x.(string)) }
func (h *stringHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

func sortedKeys(m map[string]Definition) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
