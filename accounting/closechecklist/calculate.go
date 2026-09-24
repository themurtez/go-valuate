package closechecklist

// Calculate evaluates one close [Instance] under [Policy] and returns a
// deterministic [Result]. See doc.go for the package-level contract;
// see readiness.go, checklistreadiness.go, and coverage.go for the
// individual decision rules this function composes.
//
// Calculate performs no I/O, never mutates in or policy, and is safe for
// concurrent/repeated calls against identical input (see
// determinism_test.go and concurrency_test.go).
func Calculate(in Instance, policy Policy) Result {
	return calculateWithPrior(in, policy, nil)
}

// CalculateWithPrior is [Calculate], additionally populating
// [Result.PriorCloseComparison] against prior — section 25. prior is
// never mutated and never changes any field of Result other than
// PriorCloseComparison.
func CalculateWithPrior(in Instance, policy Policy, prior PriorClose) Result {
	return calculateWithPrior(in, policy, &prior)
}

func calculateWithPrior(in Instance, policy Policy, prior *PriorClose) Result {
	v := validateAndIndex(in)

	evalList := evaluateTasks(v, in, policy)
	evals := make(map[string]taskEval, len(evalList))
	for _, e := range evalList {
		evals[e.def.TaskCode] = e
	}

	downstream := computeDownstreamRequiredCounts(v, evals)

	// Attach timing-consistency findings (needs the full evals map for
	// cross-task dependency completion timestamps).
	for i := range evalList {
		e := &evalList[i]
		if e.applicability != ApplicableYes {
			continue
		}
		e.findings = append(e.findings, checkTimingConsistency(e.def, *e, v, evals, policy)...)
	}

	structurallyInvalid := HasErrors(v.issues)

	taskResults := make([]TaskResult, 0, len(evalList))
	var allBlockers []Blocker
	var allFindings []Finding
	var anyApplicableStarted bool
	var anyEffectiveBlocker bool
	var allRequiredSatisfied = true
	blockerTaskCodes := make(map[string]bool)
	overdueTaskCodes := make(map[string]bool)
	currentTaskCodes := make(map[string]bool)

	for _, e := range evalList {
		currentTaskCodes[e.def.TaskCode] = true
		tr := buildTaskResult(e, downstream[e.def.TaskCode])
		taskResults = append(taskResults, tr)

		if e.applicability != ApplicableYes {
			continue
		}
		if e.state.Status != TaskNotStarted {
			anyApplicableStarted = true
		}
		if e.def.Required && !e.satisfied {
			allRequiredSatisfied = false
		}
		if e.effectiveBlocking {
			anyEffectiveBlocker = true
		}
		if len(e.blockers) > 0 {
			blockerTaskCodes[e.def.TaskCode] = true
		}
		if e.dueStatus == DueStatusOverdue {
			overdueTaskCodes[e.def.TaskCode] = true
		}
		for _, b := range e.blockers {
			allBlockers = append(allBlockers, b)
		}
		allFindings = append(allFindings, e.findings...)
	}

	sections := buildSectionResults(v, evalList)

	completion := buildCompletion(evalList)
	evidenceCov := buildEvidenceCoverage(evalList)
	signOffCov := buildSignOffCoverage(evalList)
	gateCov := buildGateCoverage(evalList)

	prepSatisfied, finalReviewSatisfied := splitPreparationVsReview(evalList)

	readiness := deriveChecklistReadiness(checklistReadinessInputs{
		structurallyInvalid:       structurallyInvalid,
		anyApplicableStarted:      anyApplicableStarted,
		allRequiredSatisfied:      allRequiredSatisfied,
		preparationTasksSatisfied: prepSatisfied,
		finalReviewSatisfied:      finalReviewSatisfied,
		anyEffectiveBlocker:       anyEffectiveBlocker,
		periodState:               in.PeriodState,
	})

	periodConsistency := checkPeriodConsistency(in.PeriodState, allRequiredSatisfied && !anyEffectiveBlocker)

	timingAnalytics := buildTimingAnalytics(evalList, in.Period)
	bottlenecks := buildBottleneckSummary(evalList, downstream)

	result := Result{
		PeriodID:          in.Period.PeriodID,
		TemplateID:        in.Template.TemplateID,
		TemplateVersion:   in.Template.Version,
		Readiness:         readiness,
		Sections:          sections,
		Tasks:             taskResults,
		Completion:        completion,
		EvidenceCoverage:  evidenceCov,
		SignOffCoverage:   signOffCov,
		GateCoverage:      gateCov,
		Blockers:          sortBlockers(allBlockers),
		Findings:          sortFindings(allFindings),
		PeriodConsistency: periodConsistency,
		TimingAnalytics:   timingAnalytics,
		Bottlenecks:       bottlenecks,
		Versions:          currentVersions(),
		Issues:            sortIssues(v.issues),
	}

	if prior != nil {
		result.PriorCloseComparison = buildPriorCloseComparison(*prior, currentTaskCodes, blockerTaskCodes, overdueTaskCodes)
		result.TemplateVersionComparison = buildTemplateVersionComparison(prior.Instance.Template, in.Template)
	}

	return result
}

// buildTaskResult assembles the public TaskResult from an internal
// taskEval.
func buildTaskResult(e taskEval, downstreamCount int) TaskResult {
	tr := TaskResult{
		TaskCode:                    e.def.TaskCode,
		SectionCode:                 e.def.SectionCode,
		Applicability:               e.applicability,
		Required:                    e.def.Required,
		ReportedStatus:              e.state.Status,
		ReadyToStart:                e.readyToStart,
		ReadyToComplete:             e.readyToComplete,
		Satisfied:                   e.satisfied,
		EvidenceStatus:              e.evidenceStatus,
		ReviewStatus:                e.reviewStatus,
		DueStatus:                   e.dueStatus,
		Blockers:                    sortBlockers(e.blockers),
		Findings:                    sortFindings(e.findings),
		ExceptionApplied:            e.exceptionApplied,
		EffectiveBlocking:           e.effectiveBlocking,
		DownstreamRequiredTaskCount: downstreamCount,
	}
	if e.dueDate != nil {
		s := e.dueDate.Format("2006-01-02")
		tr.DueDate = &s
	}
	if e.state.CompletedAt != nil {
		s := e.state.CompletedAt.Format("2006-01-02")
		tr.CompletedAt = &s
	}
	return tr
}

// splitPreparationVsReview reports whether every required applicable
// task with no ReviewPolicy (or a NONE/PREPARER_ONLY ReviewPolicy) is
// satisfied ("preparation"), and separately whether every required
// applicable task with a review-requiring policy is satisfied ("final
// review"). This is the concrete rule behind READY_FOR_REVIEW vs
// READY_TO_CLOSE (section 15/18): a task is treated as a "final review"
// task when its ReviewPolicy requires more than just the preparer
// (PREPARER_AND_REVIEWER or SPECIFIC_ROLES).
func splitPreparationVsReview(evals []taskEval) (prepSatisfied, reviewSatisfied bool) {
	prepSatisfied = true
	reviewSatisfied = true
	for _, e := range evals {
		if e.applicability != ApplicableYes || !e.def.Required {
			continue
		}
		if isFinalReviewTask(e.def.ReviewPolicy) {
			if !e.satisfied {
				reviewSatisfied = false
			}
		} else {
			if !e.satisfied {
				prepSatisfied = false
			}
		}
	}
	return prepSatisfied, reviewSatisfied
}

func isFinalReviewTask(p ReviewPolicy) bool {
	return p.Type == ReviewPreparerAndReviewer || p.Type == ReviewSpecificRoles
}
