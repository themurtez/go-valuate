package closechecklist

import (
	"container/heap"
	"time"
)

// taskEval holds one task's full computed evaluation before assembly
// into a TaskResult — kept separate so downstream aggregation (sections,
// bottlenecks, blockers) can consult raw booleans without re-parsing
// TaskResult.
type taskEval struct {
	def   TaskDefinition
	state TaskState

	applicability Applicability

	readyToStart    bool
	readyToComplete bool
	satisfied       bool

	dependencyBlocked bool
	blockedByTaskCode string

	evidenceStatus TaskEvidenceStatus
	reviewStatus   TaskReviewStatus
	dueStatus      DueStatus
	dueDate        *time.Time

	gateResults []gateEvalResult

	blockers []Blocker
	findings []Finding

	exceptionApplied  bool
	effectiveBlocking bool
}

type gateEvalResult struct {
	rule      GateRule
	fact      GateFact
	available bool
	satisfied bool
}

// evaluateTasks runs the per-task readiness computation for every task
// in v. It performs one pass per task using only the pre-built indexes
// in v (section 45: no rescanning), but the pass order itself follows a
// dependency-respecting topological order rather than raw template
// order: a task's Satisfied depends on whether its own dependencies are
// already resolved, so a dependency must be fully evaluated (including
// its own dependency resolution) before any task depending on it is
// evaluated — see evaluationOrder. A task inside a cycle (or depending,
// even transitively, only on tasks inside a cycle) is evaluated last,
// with its unresolved dependencies correctly reported as unresolved —
// see section 7's "never silently break a cycle."
func evaluateTasks(v validated, in Instance, policy Policy) []taskEval {
	overrides := v.overrides
	evals := make(map[string]taskEval, len(v.tasks))

	// Intrinsic (non-dependency) evaluation for every task first — cheap,
	// and independent of evaluation order.
	for _, t := range v.tasks {
		evals[t.TaskCode] = evaluateTaskIntrinsic(t, v, in, policy, overrides)
	}

	order := evaluationOrder(v)
	for _, taskCode := range order {
		t, ok := v.taskByCode[taskCode]
		if !ok {
			continue
		}
		e := evals[taskCode]
		if e.applicability != ApplicableYes {
			continue
		}
		applyDependencies(&e, t, v, evals)
		evals[taskCode] = e
	}

	out := make([]taskEval, 0, len(v.tasks))
	for _, t := range v.tasks {
		out = append(out, evals[t.TaskCode])
	}
	return out
}

// evaluationOrder returns v.taskOrder's task codes reordered so every
// task appears after all tasks it (non-cyclically) depends on — a
// standard iterative topological sort (Kahn's algorithm), which never
// recurses (section 47's adversarial-safety requirement) and is stable
// under v.taskOrder for any remaining ties, so evaluation order (and
// therefore every downstream Satisfied computation) never depends on Go
// map iteration. Tasks that remain unplaced once every task with
// in-degree zero has been consumed (i.e. every task inside, or
// depending only on, a dependency cycle) are appended at the end in
// their original v.taskOrder position — applyDependencies still
// evaluates them correctly (their unresolved cyclic dependency is simply
// reported as unresolved via v.inCycle), just without a "correct"
// topological position, which does not exist for a cycle.
func evaluationOrder(v validated) []string {
	indegree := make(map[string]int, len(v.taskOrder))
	dependents := make(map[string][]string, len(v.taskOrder))
	position := make(map[string]int, len(v.taskOrder))
	for i, code := range v.taskOrder {
		indegree[code] = 0
		position[code] = i
	}
	for _, code := range v.taskOrder {
		for _, dep := range v.graph.edges[code] {
			indegree[code]++
			dependents[dep.DependsOnTaskCode] = append(dependents[dep.DependsOnTaskCode], code)
		}
	}

	// ready is a min-heap ordered by original v.taskOrder position, so
	// among several simultaneously-ready tasks we always advance the
	// earliest one in template order — evaluation order (and therefore
	// every downstream Satisfied computation) is deterministic and
	// independent of Go map/edge iteration order (section 43).
	ready := &taskHeap{}
	for _, code := range v.taskOrder {
		if indegree[code] == 0 {
			heap.Push(ready, heapItem{code: code, pos: position[code]})
		}
	}

	placed := make(map[string]bool, len(v.taskOrder))
	order := make([]string, 0, len(v.taskOrder))
	for ready.Len() > 0 {
		next := heap.Pop(ready).(heapItem)
		if placed[next.code] {
			continue
		}
		placed[next.code] = true
		order = append(order, next.code)
		for _, code := range dependents[next.code] {
			if placed[code] {
				continue
			}
			indegree[code]--
			if indegree[code] == 0 {
				heap.Push(ready, heapItem{code: code, pos: position[code]})
			}
		}
	}

	// Anything left unplaced participates in a cycle (or depends, even
	// transitively, only on a cycle) — append in original order; see the
	// function doc comment.
	for _, code := range v.taskOrder {
		if !placed[code] {
			order = append(order, code)
		}
	}
	return order
}

// evaluateTaskIntrinsic computes everything about one task that does not
// require knowledge of other tasks: applicability, evidence, review,
// gates, due status, and the caller-reported-complete-with-missing-
// requirements check.
func evaluateTaskIntrinsic(t TaskDefinition, v validated, in Instance, policy Policy, overrides map[string]Applicability) taskEval {
	state, hasState := v.states[t.TaskCode]
	if !hasState {
		state = defaultTaskState(t.TaskCode)
	}

	app := resolveApplicability(t.TaskCode, t.Applicability, overrides, in.ApplicabilityFlags, v.gates)

	e := taskEval{def: t, state: state, applicability: app}
	if app != ApplicableYes {
		e.evidenceStatus = EvidenceStatusNotRequired
		e.reviewStatus = ReviewStatusNotRequired
		e.dueStatus = DueStatusUnavailable
		return e
	}

	// Evidence.
	evReq := t.EvidencePolicy.Type != "" && t.EvidencePolicy.Type != EvidenceNone
	evOK := evidenceSatisfied(t.EvidencePolicy, state.Evidence)
	switch {
	case !evReq:
		e.evidenceStatus = EvidenceStatusNotRequired
	case evOK:
		e.evidenceStatus = EvidenceStatusSatisfied
	default:
		e.evidenceStatus = EvidenceStatusMissing
	}

	// Review / sign-off.
	review := evaluateReview(t.ReviewPolicy, state.SignOffs)
	reviewRequired := len(t.ReviewPolicy.requiredRoles()) > 0
	switch {
	case !reviewRequired:
		e.reviewStatus = ReviewStatusNotRequired
	case review.roleConflict:
		e.reviewStatus = ReviewStatusConflict
	case review.satisfied:
		e.reviewStatus = ReviewStatusSatisfied
	default:
		e.reviewStatus = ReviewStatusMissing
	}
	if review.roleConflict {
		e.findings = append(e.findings, Finding{
			Code:     FindingSignOffRoleConflict,
			Severity: FindingSeverityWarning,
			Message:  "Sign-off role conflict on task " + t.TaskCode + ": actor " + review.conflictActor + " recorded multiple required roles.",
			TaskCode: t.TaskCode,
		})
	}

	// Gates.
	for _, gr := range t.GateRules {
		fact, ok := v.gates[gr.GateCode]
		sat := gateSatisfied(gr, fact, ok)
		e.gateResults = append(e.gateResults, gateEvalResult{rule: gr, fact: fact, available: ok, satisfied: sat})
		if ok && fact.Status == GateWarning {
			e.findings = append(e.findings, Finding{
				Code:     FindingExternalGateWarning,
				Severity: FindingSeverityInfo,
				Message:  "Gate " + gr.GateCode + " for task " + t.TaskCode + " reports WARNING.",
				TaskCode: t.TaskCode,
				Ref:      gr.GateCode,
			})
		}
	}

	// Due date / due status.
	due, resolved := resolveDueDate(t.DueRule, in.Period)
	if resolved {
		e.dueDate = &due
	}
	e.dueStatus = computeDueStatus(due, resolved, state.CompletedAt, in.EvaluationDate)
	if e.dueStatus == DueStatusOverdue {
		e.findings = append(e.findings, Finding{
			Code:     FindingTaskOverdue,
			Severity: FindingSeverityWarning,
			Message:  "Task " + t.TaskCode + " is overdue.",
			TaskCode: t.TaskCode,
		})
	}
	if e.dueStatus == DueStatusCompletedLate {
		e.findings = append(e.findings, Finding{
			Code:     FindingTaskCompletedLate,
			Severity: FindingSeverityInfo,
			Message:  "Task " + t.TaskCode + " was completed after its due date.",
			TaskCode: t.TaskCode,
		})
	}

	return e
}

// applyDependencies fills in the dependency-aware fields of e
// (ReadyToStart, ReadyToComplete, Satisfied, and their Blockers/
// Findings), then applies the required-task/skip/evidence/review/gate
// rules to determine final Satisfied — mirrors section 15/16/17's
// combined semantics. evals must already hold every task's intrinsic
// evaluation.
func applyDependencies(e *taskEval, t TaskDefinition, v validated, evals map[string]taskEval) {
	depsOK := true
	var depBlockers []Blocker
	for _, dep := range v.graph.edges[t.TaskCode] {
		depEval, ok := evals[dep.DependsOnTaskCode]
		if !ok {
			continue
		}
		resolved := dependencyResolved(dep.Type, depEval)
		if !resolved {
			depsOK = false
			depBlockers = append(depBlockers, Blocker{
				TaskCode:           t.TaskCode,
				ReasonCode:         BlockerDependencyIncomplete,
				Message:            "Task " + t.TaskCode + " is blocked because dependency " + dep.DependsOnTaskCode + " is not resolved.",
				DependencyTaskCode: dep.DependsOnTaskCode,
				EffectiveBlocking:  true,
			})
		}
	}
	if v.inCycle[t.TaskCode] {
		depsOK = false
	}

	e.readyToStart = depsOK
	e.readyToComplete = depsOK

	// Gate satisfaction for required gate rules.
	gatesOK := true
	var gateBlockers []Blocker
	for _, gr := range e.gateResults {
		if !gr.rule.Required {
			continue
		}
		if !gr.satisfied {
			gatesOK = false
			reason := BlockerGateFailed
			if !gr.available {
				reason = BlockerGateUnavailable
			}
			gateBlockers = append(gateBlockers, Blocker{
				TaskCode:          t.TaskCode,
				ReasonCode:        reason,
				Message:           gateBlockerMessage(t.TaskCode, gr),
				GateCode:          gr.rule.GateCode,
				EffectiveBlocking: true,
			})
		}
	}
	if !gatesOK {
		e.readyToComplete = false
	}

	evidenceOK := e.evidenceStatus != EvidenceStatusMissing
	var evidenceBlockers []Blocker
	if !evidenceOK {
		missingTypes := missingEvidenceTypes(t.EvidencePolicy, e.state.Evidence)
		mt := ""
		if len(missingTypes) > 0 {
			mt = missingTypes[0]
		}
		evidenceBlockers = append(evidenceBlockers, Blocker{
			TaskCode:            t.TaskCode,
			ReasonCode:          BlockerEvidenceMissing,
			Message:             "Task " + t.TaskCode + " is missing required evidence.",
			MissingEvidenceType: mt,
			EffectiveBlocking:   true,
		})
	}

	reviewOK := e.reviewStatus != ReviewStatusMissing && e.reviewStatus != ReviewStatusConflict
	var reviewBlockers []Blocker
	if !reviewOK {
		role := ""
		review := evaluateReview(t.ReviewPolicy, e.state.SignOffs)
		if len(review.missingRoles) > 0 {
			role = string(review.missingRoles[0])
		}
		reason := BlockerSignOffMissing
		msg := "Reviewer sign-off is missing for task " + t.TaskCode + "."
		if e.reviewStatus == ReviewStatusConflict {
			msg = "Sign-off role conflict is unresolved for task " + t.TaskCode + "."
		}
		reviewBlockers = append(reviewBlockers, Blocker{
			TaskCode:           t.TaskCode,
			ReasonCode:         reason,
			Message:            msg,
			MissingSignOffRole: role,
			EffectiveBlocking:  true,
		})
	}

	callerComplete := e.state.Status == TaskCompleted
	callerBlocked := e.state.Status == TaskBlocked
	callerSkipped := e.state.Status == TaskSkipped

	allRequirementsMet := depsOK && gatesOK && evidenceOK && reviewOK

	var blockers []Blocker
	blockers = append(blockers, depBlockers...)
	blockers = append(blockers, gateBlockers...)
	blockers = append(blockers, evidenceBlockers...)
	blockers = append(blockers, reviewBlockers...)
	if callerBlocked {
		blockers = append(blockers, Blocker{
			TaskCode:          t.TaskCode,
			ReasonCode:        BlockerCallerMarkedBlocked,
			Message:           "Task " + t.TaskCode + " is marked BLOCKED by the caller.",
			EffectiveBlocking: true,
		})
	}

	// Resolve exceptions against every blocker before deciding Satisfied,
	// so a valid permitted (non-expired) exception can make an otherwise
	// unmet requirement effectively non-blocking — section 22/42: the
	// underlying blocker always remains visible (ExceptionApplied=true,
	// EffectiveBlocking=false), it is never deleted.
	if len(blockers) > 0 {
		applyExceptions(e, t, v, blockers)
	} else {
		e.blockers = nil
	}
	effectivelyBlocked := false
	for _, b := range e.blockers {
		if b.EffectiveBlocking {
			effectivelyBlocked = true
			break
		}
	}
	e.effectiveBlocking = effectivelyBlocked

	satisfied := false
	switch {
	case callerSkipped:
		satisfied = t.AllowSkip || !t.Required
		if t.Required && !t.AllowSkip {
			e.findings = append(e.findings, Finding{
				Code:     FindingRequiredTaskSkipped,
				Severity: FindingSeverityBlocking,
				Message:  "Required task " + t.TaskCode + " was skipped but skipping is not permitted.",
				TaskCode: t.TaskCode,
			})
		}
	case callerComplete:
		satisfied = allRequirementsMet || (!effectivelyBlocked && !callerBlocked)
		if !satisfied {
			e.findings = append(e.findings, Finding{
				Code:     FindingTaskMarkedCompleteWithMissingRequirements,
				Severity: FindingSeverityBlocking,
				Message:  "Task " + t.TaskCode + " is marked COMPLETED but required dependencies/gates/evidence/sign-offs are not all satisfied.",
				TaskCode: t.TaskCode,
			})
		}
	default:
		satisfied = false
	}

	e.satisfied = satisfied

	if !satisfied && t.Required {
		e.findings = append(e.findings, Finding{
			Code:     FindingRequiredTaskIncomplete,
			Severity: FindingSeverityBlocking,
			Message:  "Required task " + t.TaskCode + " is not yet satisfied.",
			TaskCode: t.TaskCode,
		})
	}
	for _, b := range depBlockers {
		e.findings = append(e.findings, Finding{
			Code:     FindingTaskBlockedByDependency,
			Severity: FindingSeverityWarning,
			Message:  b.Message,
			TaskCode: t.TaskCode,
			Ref:      b.DependencyTaskCode,
		})
	}
	for _, b := range gateBlockers {
		e.findings = append(e.findings, Finding{
			Code:     FindingTaskBlockedByGate,
			Severity: FindingSeverityWarning,
			Message:  b.Message,
			TaskCode: t.TaskCode,
			Ref:      b.GateCode,
		})
	}
	if !evidenceOK {
		e.findings = append(e.findings, Finding{
			Code:     FindingRequiredEvidenceMissing,
			Severity: FindingSeverityWarning,
			Message:  "Task " + t.TaskCode + " is missing required evidence.",
			TaskCode: t.TaskCode,
		})
	}
	if !reviewOK && e.reviewStatus == ReviewStatusMissing {
		e.findings = append(e.findings, Finding{
			Code:     FindingRequiredSignOffMissing,
			Severity: FindingSeverityWarning,
			Message:  "Reviewer sign-off is missing for task " + t.TaskCode + ".",
			TaskCode: t.TaskCode,
		})
	}
}

func dependencyResolved(depType DependencyType, dep taskEval) bool {
	switch depType {
	case DependencyMustBeResolved:
		return dep.satisfied || dep.applicability == ApplicableNo || (dep.state.Status == TaskSkipped && dep.def.AllowSkip)
	default: // DependencyMustBeCompleted and unrecognized fall back to the strict rule
		return dep.satisfied
	}
}

func gateBlockerMessage(taskCode string, gr gateEvalResult) string {
	if !gr.available {
		return "Task " + taskCode + " is blocked because gate " + gr.rule.GateCode + " is unavailable."
	}
	return "Task " + taskCode + " is blocked because gate " + gr.rule.GateCode + " is " + string(gr.fact.Status) + "."
}

// applyExceptions resolves any Exception recorded against t against
// blockers, marking EffectiveBlocking=false on a blocker only when a
// permitted, non-expired exception covers this task and the task's
// ExceptionPolicy allows it. The underlying blocker itself always
// remains in e.blockers — see exceptions.go.
func applyExceptions(e *taskEval, t TaskDefinition, v validated, blockers []Blocker) {
	excs := v.exceptions[t.TaskCode]
	if len(excs) == 0 || t.ExceptionPolicy == "" || t.ExceptionPolicy == ExceptionNotAllowed {
		e.blockers = blockers
		return
	}

	var bestEffective *exceptionState
	var bestExpired *exceptionState
	for _, exc := range excs {
		st := evaluateException(exc, t.ExceptionPolicy, v.evaluationDate())
		if st.effective() {
			s := st
			bestEffective = &s
		} else if st.permitted && st.expired {
			s := st
			bestExpired = &s
		}
	}

	out := make([]Blocker, len(blockers))
	copy(out, blockers)
	if bestEffective != nil {
		for i := range out {
			out[i].ExceptionApplied = true
			out[i].EffectiveBlocking = false
		}
		e.exceptionApplied = true
		e.findings = append(e.findings, Finding{
			Code:     FindingValidExceptionApplied,
			Severity: FindingSeverityInfo,
			Message:  "A valid exception is applied to task " + t.TaskCode + "; the underlying condition remains visible but is not effectively blocking.",
			TaskCode: t.TaskCode,
			Ref:      bestEffective.exception.ExceptionID,
		})
	} else if bestExpired != nil {
		e.findings = append(e.findings, Finding{
			Code:     FindingExpiredException,
			Severity: FindingSeverityWarning,
			Message:  "An exception for task " + t.TaskCode + " has expired and no longer suppresses blocking.",
			TaskCode: t.TaskCode,
			Ref:      bestExpired.exception.ExceptionID,
		})
	}
	e.blockers = out
}

func (v validated) evaluationDate() time.Time {
	return v.evalDate
}

// heapItem/taskHeap implement container/heap.Interface for
// evaluationOrder's deterministic "earliest in template order among
// ready tasks" tie-break.
type heapItem struct {
	code string
	pos  int
}

type taskHeap []heapItem

func (h taskHeap) Len() int            { return len(h) }
func (h taskHeap) Less(i, j int) bool  { return h[i].pos < h[j].pos }
func (h taskHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *taskHeap) Push(x interface{}) { *h = append(*h, x.(heapItem)) }
func (h *taskHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}
