package closechecklist

// DependencyType is the fixed set of task-to-task dependency kinds.
type DependencyType string

const (
	// DependencyMustBeCompleted: the referenced task must be Satisfied
	// (see readiness.go) before this task is ReadyToStart/ReadyToComplete.
	DependencyMustBeCompleted DependencyType = "MUST_BE_COMPLETED"
	// DependencyMustBeResolved: the referenced task must be either
	// Satisfied or resolved to NOT_APPLICABLE/SKIPPED-with-allowance —
	// i.e. no longer an open requirement, even if not literally
	// "completed." Useful for a task that only needs a prior task to be
	// off the table (e.g. "inventory adjustment review" depending on
	// "inventory reconciliation," which may resolve NOT_APPLICABLE for a
	// service business).
	DependencyMustBeResolved DependencyType = "MUST_BE_RESOLVED"
)

func isRecognizedDependencyType(t DependencyType) bool {
	switch t {
	case "", DependencyMustBeCompleted, DependencyMustBeResolved:
		return true
	default:
		return false
	}
}

// TaskDependency declares that one task depends on another, within the
// same Template.
type TaskDependency struct {
	DependsOnTaskCode string         `json:"depends_on_task_code"`
	Type              DependencyType `json:"type,omitempty"`
}

// dependencyGraph is the validated, indexed adjacency built once per
// Calculate call from a Template's task definitions — never rebuilt per
// task, and never dependent on slice order (section 43).
type dependencyGraph struct {
	// edges maps a task code to its (deduplicated, validated) outgoing
	// dependencies.
	edges map[string][]TaskDependency
}

// buildDependencyGraph indexes every TaskDefinition's Dependencies,
// reporting Issues for unknown/self/duplicate dependencies. Cycle
// detection is separate (see detectCycles) so a cycle never silently
// breaks the graph — every edge, even one inside a cycle, is preserved
// as-is (section 7: "Do not silently break cycles").
func buildDependencyGraph(tasks []TaskDefinition) (dependencyGraph, []Issue) {
	var issues []Issue
	known := make(map[string]bool, len(tasks))
	for _, t := range tasks {
		known[t.TaskCode] = true
	}

	g := dependencyGraph{edges: make(map[string][]TaskDependency, len(tasks))}
	for _, t := range tasks {
		seen := make(map[string]bool, len(t.Dependencies))
		var kept []TaskDependency
		for _, d := range t.Dependencies {
			if d.DependsOnTaskCode == t.TaskCode {
				issues = append(issues, Issue{
					Code:     IssueSelfDependency,
					Severity: IssueSeverityError,
					Message:  "task " + t.TaskCode + " declares a dependency on itself",
					TaskCode: t.TaskCode,
				})
				continue
			}
			if !known[d.DependsOnTaskCode] {
				issues = append(issues, Issue{
					Code:     IssueUnknownDependency,
					Severity: IssueSeverityError,
					Message:  "task " + t.TaskCode + " depends on unknown task " + d.DependsOnTaskCode,
					TaskCode: t.TaskCode,
					Ref:      d.DependsOnTaskCode,
				})
				continue
			}
			if !isRecognizedDependencyType(d.Type) {
				issues = append(issues, Issue{
					Code:     IssueInvalidDependency,
					Severity: IssueSeverityWarning,
					Message:  "task " + t.TaskCode + " has an unrecognized dependency type for " + d.DependsOnTaskCode,
					TaskCode: t.TaskCode,
					Ref:      d.DependsOnTaskCode,
				})
			}
			key := d.DependsOnTaskCode
			if seen[key] {
				issues = append(issues, Issue{
					Code:     IssueDuplicateDependency,
					Severity: IssueSeverityWarning,
					Message:  "task " + t.TaskCode + " declares a duplicate dependency on " + d.DependsOnTaskCode,
					TaskCode: t.TaskCode,
					Ref:      d.DependsOnTaskCode,
				})
				continue
			}
			seen[key] = true
			kept = append(kept, d)
		}
		g.edges[t.TaskCode] = kept
	}
	return g, issues
}

// detectCycles runs an iterative (non-recursive, per section 47's
// "prefer iterative graph algorithms" adversarial-safety requirement)
// three-color DFS over g, returning the set of task codes that
// participate in at least one dependency cycle, plus one Issue per
// distinct cycle found. Task order of iteration follows taskOrder (the
// Template's own Tasks slice order) solely to make which cycle is
// "found first" deterministic for Issue ordering — the graph semantics
// themselves never depend on this order (section 43).
func detectCycles(g dependencyGraph, taskOrder []string) (map[string]bool, []Issue) {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int, len(taskOrder))
	inCycle := make(map[string]bool)
	var issues []Issue

	type frame struct {
		node    string
		edgeIdx int
		path    []string
	}

	for _, start := range taskOrder {
		if color[start] != white {
			continue
		}
		stack := []frame{{node: start, path: []string{start}}}
		color[start] = gray
		for len(stack) > 0 {
			top := &stack[len(stack)-1]
			deps := g.edges[top.node]
			if top.edgeIdx >= len(deps) {
				color[top.node] = black
				stack = stack[:len(stack)-1]
				continue
			}
			next := deps[top.edgeIdx].DependsOnTaskCode
			top.edgeIdx++
			switch color[next] {
			case white:
				color[next] = gray
				path := append(append([]string{}, top.path...), next)
				stack = append(stack, frame{node: next, path: path})
			case gray:
				// Found a cycle: next is an ancestor on the current path.
				cycleStart := 0
				for i, n := range top.path {
					if n == next {
						cycleStart = i
						break
					}
				}
				cycle := append(append([]string{}, top.path[cycleStart:]...), next)
				for _, n := range cycle {
					inCycle[n] = true
				}
				issues = append(issues, Issue{
					Code:     IssueDependencyCycle,
					Severity: IssueSeverityError,
					Message:  "dependency cycle detected: " + joinTaskCodes(cycle),
					TaskCode: cycle[0],
				})
			case black:
				// already fully processed, not part of a new cycle via this edge
			}
		}
	}
	return inCycle, issues
}

func joinTaskCodes(codes []string) string {
	out := ""
	for i, c := range codes {
		if i > 0 {
			out += " -> "
		}
		out += c
	}
	return out
}
