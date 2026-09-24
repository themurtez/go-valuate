package closechecklist

import "time"

// validated is every index/lookup structure built once from a validated
// Instance, consumed by readiness.go's per-task evaluation — section 45's
// "validate/index template once" performance design.
type validated struct {
	sections     map[string]SectionDefinition
	sectionOrder []string
	tasks        []TaskDefinition // deduplicated, in template order
	taskByCode   map[string]TaskDefinition
	taskOrder    []string

	graph   dependencyGraph
	inCycle map[string]bool

	states     map[string]TaskState
	overrides  map[string]Applicability
	gates      map[string]GateFact
	exceptions map[string][]Exception // by TaskCode

	evalDate time.Time

	issues []Issue
}

// validateAndIndex performs section 44's structural validation and
// builds every index readiness.go needs, exactly once per Calculate
// call.
func validateAndIndex(in Instance) validated {
	v := validated{
		sections:   make(map[string]SectionDefinition),
		taskByCode: make(map[string]TaskDefinition),
		states:     make(map[string]TaskState),
		overrides:  make(map[string]Applicability),
		gates:      make(map[string]GateFact),
		exceptions: make(map[string][]Exception),
		evalDate:   in.EvaluationDate,
	}

	// Period.
	if isZeroTime(in.Period.StartDate) || isZeroTime(in.Period.EndDate) || in.Period.PeriodID == "" {
		v.issues = append(v.issues, Issue{Code: IssueInvalidPeriod, Severity: IssueSeverityError, Message: "period requires a non-empty period_id, start_date, and end_date"})
	} else if in.Period.EndDate.Before(in.Period.StartDate) {
		v.issues = append(v.issues, Issue{Code: IssueInvalidPeriod, Severity: IssueSeverityError, Message: "period end_date is before start_date"})
	}
	if !isRecognizedPeriodState(in.PeriodState) {
		v.issues = append(v.issues, Issue{Code: IssueInvalidPeriod, Severity: IssueSeverityWarning, Message: "unrecognized period_state"})
	}

	// Template identity.
	if in.Template.TemplateID == "" || in.Template.Version == "" {
		v.issues = append(v.issues, Issue{Code: IssueInvalidTemplateVersion, Severity: IssueSeverityError, Message: "template requires a non-empty template_id and version"})
	}

	// Sections.
	for _, s := range in.Template.Sections {
		if s.SectionCode == "" {
			continue
		}
		v.sections[s.SectionCode] = s
		v.sectionOrder = append(v.sectionOrder, s.SectionCode)
	}

	// Tasks: dedup by TaskCode (first wins), validate SectionCode refs.
	seenTask := make(map[string]bool, len(in.Template.Tasks))
	for _, t := range in.Template.Tasks {
		if t.TaskCode == "" {
			continue
		}
		if seenTask[t.TaskCode] {
			v.issues = append(v.issues, Issue{Code: IssueDuplicateTaskDefinition, Severity: IssueSeverityError, Message: "duplicate task definition for task_code " + t.TaskCode, TaskCode: t.TaskCode})
			continue
		}
		seenTask[t.TaskCode] = true
		if t.SectionCode != "" {
			if _, ok := v.sections[t.SectionCode]; !ok {
				v.issues = append(v.issues, Issue{Code: IssueUnknownSection, Severity: IssueSeverityError, Message: "task " + t.TaskCode + " references unknown section_code " + t.SectionCode, TaskCode: t.TaskCode, Ref: t.SectionCode})
			}
		}
		if !isRecognizedApplicabilityRuleType(t.Applicability.Type) {
			v.issues = append(v.issues, Issue{Code: IssueInvalidApplicabilityRule, Severity: IssueSeverityWarning, Message: "task " + t.TaskCode + " has an unrecognized applicability rule type", TaskCode: t.TaskCode})
		}
		if !isRecognizedEvidencePolicyType(t.EvidencePolicy.Type) {
			v.issues = append(v.issues, Issue{Code: IssueInvalidPolicy, Severity: IssueSeverityWarning, Message: "task " + t.TaskCode + " has an unrecognized evidence policy type", TaskCode: t.TaskCode})
		}
		if !isRecognizedReviewPolicyType(t.ReviewPolicy.Type) {
			v.issues = append(v.issues, Issue{Code: IssueInvalidPolicy, Severity: IssueSeverityWarning, Message: "task " + t.TaskCode + " has an unrecognized review policy type", TaskCode: t.TaskCode})
		}
		if !isRecognizedDueRuleType(t.DueRule.Type) {
			v.issues = append(v.issues, Issue{Code: IssueInvalidDueRule, Severity: IssueSeverityWarning, Message: "task " + t.TaskCode + " has an unrecognized due rule type", TaskCode: t.TaskCode})
		}
		if !isRecognizedExceptionPolicy(t.ExceptionPolicy) {
			v.issues = append(v.issues, Issue{Code: IssueInvalidPolicy, Severity: IssueSeverityWarning, Message: "task " + t.TaskCode + " has an unrecognized exception policy", TaskCode: t.TaskCode})
		}
		for _, gr := range t.GateRules {
			if !isRecognizedGateRequirement(gr.Require) || gr.GateCode == "" {
				v.issues = append(v.issues, Issue{Code: IssueInvalidGateRule, Severity: IssueSeverityWarning, Message: "task " + t.TaskCode + " has an invalid gate rule", TaskCode: t.TaskCode, Ref: gr.GateCode})
			}
		}
		v.taskByCode[t.TaskCode] = t
		v.taskOrder = append(v.taskOrder, t.TaskCode)
		v.tasks = append(v.tasks, t)
	}

	// Dependency graph + cycle detection.
	graph, depIssues := buildDependencyGraph(v.tasks)
	v.graph = graph
	v.issues = append(v.issues, depIssues...)
	inCycle, cycleIssues := detectCycles(graph, v.taskOrder)
	v.inCycle = inCycle
	v.issues = append(v.issues, cycleIssues...)

	// Task states: dedup by TaskCode, flag unknown task codes.
	seenState := make(map[string]bool, len(in.TaskStates))
	for _, s := range in.TaskStates {
		if s.TaskCode == "" {
			continue
		}
		if seenState[s.TaskCode] {
			v.issues = append(v.issues, Issue{Code: IssueDuplicateTaskState, Severity: IssueSeverityError, Message: "duplicate task state for task_code " + s.TaskCode, TaskCode: s.TaskCode})
			continue
		}
		seenState[s.TaskCode] = true
		if _, ok := v.taskByCode[s.TaskCode]; !ok {
			v.issues = append(v.issues, Issue{Code: IssueUnknownTaskState, Severity: IssueSeverityWarning, Message: "task state references unknown task_code " + s.TaskCode, TaskCode: s.TaskCode})
			continue
		}
		if !isRecognizedTaskStatus(s.Status) {
			v.issues = append(v.issues, Issue{Code: IssueUnknownTaskState, Severity: IssueSeverityWarning, Message: "task " + s.TaskCode + " has an unrecognized status", TaskCode: s.TaskCode})
		}
		if s.StartedAt != nil && s.CompletedAt != nil && s.CompletedAt.Before(*s.StartedAt) {
			v.issues = append(v.issues, Issue{Code: IssueInvalidTimestamp, Severity: IssueSeverityWarning, Message: "task " + s.TaskCode + " completed_at is before started_at", TaskCode: s.TaskCode})
		}
		// Evidence: dedup by EvidenceID within this task's state.
		seenEvidence := make(map[string]bool, len(s.Evidence))
		var evidence []EvidenceRef
		for _, e := range s.Evidence {
			if e.EvidenceID != "" && seenEvidence[e.EvidenceID] {
				v.issues = append(v.issues, Issue{Code: IssueDuplicateEvidence, Severity: IssueSeverityWarning, Message: "task " + s.TaskCode + " has duplicate evidence_id " + e.EvidenceID, TaskCode: s.TaskCode, Ref: e.EvidenceID})
				continue
			}
			if e.EvidenceID != "" {
				seenEvidence[e.EvidenceID] = true
			}
			evidence = append(evidence, e)
		}
		s.Evidence = evidence
		// Sign-offs: dedup by SignOffID within this task's state.
		seenSignOff := make(map[string]bool, len(s.SignOffs))
		var signOffs []SignOff
		for _, so := range s.SignOffs {
			if so.SignOffID != "" && seenSignOff[so.SignOffID] {
				v.issues = append(v.issues, Issue{Code: IssueDuplicateSignOff, Severity: IssueSeverityWarning, Message: "task " + s.TaskCode + " has duplicate sign_off_id " + so.SignOffID, TaskCode: s.TaskCode, Ref: so.SignOffID})
				continue
			}
			if so.SignOffID != "" {
				seenSignOff[so.SignOffID] = true
			}
			if isZeroTime(so.SignedAt) {
				v.issues = append(v.issues, Issue{Code: IssueInvalidTimestamp, Severity: IssueSeverityWarning, Message: "task " + s.TaskCode + " sign-off " + so.SignOffID + " has no signed_at", TaskCode: s.TaskCode, Ref: so.SignOffID})
			}
			signOffs = append(signOffs, so)
		}
		// Sign-offs are stored in the fixed, deterministic
		// role/actor/ID order (section 39) so any future consumer that
		// iterates TaskState.SignOffs directly (e.g. a caller inspecting
		// v.states, or a later feature surfacing them on TaskResult)
		// never depends on caller-supplied slice order or Go map order.
		s.SignOffs = sortSignOffs(signOffs)
		v.states[s.TaskCode] = s
	}

	// Applicability overrides.
	for _, o := range in.ApplicabilityOverrides {
		if _, ok := v.taskByCode[o.TaskCode]; !ok {
			v.issues = append(v.issues, Issue{Code: IssueUnknownTaskState, Severity: IssueSeverityWarning, Message: "applicability override references unknown task_code " + o.TaskCode, TaskCode: o.TaskCode})
			continue
		}
		v.overrides[o.TaskCode] = o.Applicability
	}

	// Gate facts: dedup by GateCode.
	for _, g := range in.Gates {
		if g.GateCode == "" {
			continue
		}
		if !isRecognizedGateStatus(g.Status) {
			v.issues = append(v.issues, Issue{Code: IssueInvalidGateFact, Severity: IssueSeverityWarning, Message: "gate fact " + g.GateCode + " has an unrecognized status", Ref: g.GateCode})
			continue
		}
		if _, ok := v.gates[g.GateCode]; ok {
			v.issues = append(v.issues, Issue{Code: IssueDuplicateGateFact, Severity: IssueSeverityWarning, Message: "duplicate gate fact for gate_code " + g.GateCode, Ref: g.GateCode})
			continue
		}
		v.gates[g.GateCode] = g
	}

	// Exceptions: validate task ref, dedup by ExceptionID.
	seenException := make(map[string]bool, len(in.Exceptions))
	for _, e := range in.Exceptions {
		if e.ExceptionID != "" && seenException[e.ExceptionID] {
			v.issues = append(v.issues, Issue{Code: IssueDuplicateException, Severity: IssueSeverityWarning, Message: "duplicate exception_id " + e.ExceptionID, Ref: e.ExceptionID})
			continue
		}
		if e.ExceptionID != "" {
			seenException[e.ExceptionID] = true
		}
		if _, ok := v.taskByCode[e.TaskCode]; !ok {
			v.issues = append(v.issues, Issue{Code: IssueUnknownExceptionTask, Severity: IssueSeverityWarning, Message: "exception " + e.ExceptionID + " references unknown task_code " + e.TaskCode, TaskCode: e.TaskCode, Ref: e.ExceptionID})
			continue
		}
		if isZeroTime(e.ApprovedAt) {
			v.issues = append(v.issues, Issue{Code: IssueInvalidException, Severity: IssueSeverityWarning, Message: "exception " + e.ExceptionID + " has no approved_at", TaskCode: e.TaskCode, Ref: e.ExceptionID})
		}
		if e.ExpiresAt != nil && e.ExpiresAt.Before(e.ApprovedAt) {
			v.issues = append(v.issues, Issue{Code: IssueInvalidException, Severity: IssueSeverityWarning, Message: "exception " + e.ExceptionID + " expires before it was approved", TaskCode: e.TaskCode, Ref: e.ExceptionID})
		}
		v.exceptions[e.TaskCode] = append(v.exceptions[e.TaskCode], e)
	}

	return v
}
