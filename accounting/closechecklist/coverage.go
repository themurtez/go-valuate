package closechecklist

// buildSectionResults aggregates evals into one SectionResult per
// Template section, in template declaration order (section 19/39).
func buildSectionResults(v validated, evals []taskEval) []SectionResult {
	bySection := make(map[string]*SectionResult, len(v.sectionOrder))
	order := make([]string, 0, len(v.sectionOrder))
	for _, code := range v.sectionOrder {
		if _, ok := bySection[code]; !ok {
			bySection[code] = &SectionResult{SectionCode: code, Name: v.sections[code].Name}
			order = append(order, code)
		}
	}

	for _, e := range evals {
		code := e.def.SectionCode
		sr, ok := bySection[code]
		if !ok {
			// Task references an unknown section (already flagged as an
			// Issue by validate.go) — still surface it under its own
			// synthetic section bucket so no task silently vanishes from
			// every section total.
			sr = &SectionResult{SectionCode: code}
			bySection[code] = sr
			order = append(order, code)
		}
		if e.applicability != ApplicableYes {
			continue
		}
		sr.ApplicableTasks++
		if e.def.Required {
			sr.RequiredTasks++
		}
		if e.satisfied {
			sr.SatisfiedTasks++
		}
		if e.state.Status == TaskInProgress {
			sr.InProgressTasks++
		}
		if e.state.Status == TaskBlocked || e.effectiveBlocking {
			sr.BlockedTasks++
		}
		if e.dueStatus == DueStatusOverdue {
			sr.OverdueTasks++
		}
	}

	out := make([]SectionResult, 0, len(order))
	for _, code := range order {
		sr := bySection[code]
		if sr.RequiredTasks > 0 {
			sr.CompletionPercent = round2(100 * float64(sr.SatisfiedTasks) / float64(sr.RequiredTasks))
		}
		sr.Status = deriveSectionStatus(*sr)
		out = append(out, *sr)
	}
	return out
}

func deriveSectionStatus(sr SectionResult) SectionStatus {
	switch {
	case sr.ApplicableTasks == 0:
		return SectionNotStarted
	case sr.RequiredTasks > 0 && sr.SatisfiedTasks >= sr.RequiredTasks:
		return SectionComplete
	case sr.SatisfiedTasks > 0 || sr.InProgressTasks > 0:
		return SectionInProgress
	default:
		return SectionNotStarted
	}
}

// buildCompletion computes section 19's top-level Completion summary.
// NOT_APPLICABLE tasks are excluded from every denominator (core
// invariant, section 42).
func buildCompletion(evals []taskEval) Completion {
	var c Completion
	for _, e := range evals {
		if e.applicability != ApplicableYes {
			continue
		}
		c.ApplicableTaskCount++
		if e.def.Required {
			c.RequiredTaskCount++
			if e.satisfied {
				c.SatisfiedRequiredTaskCount++
			} else {
				c.UnsatisfiedRequiredTaskCount++
			}
		} else {
			c.OptionalTaskCount++
		}
		if e.state.Status == TaskBlocked || e.effectiveBlocking {
			c.BlockedTaskCount++
		}
		if e.dueStatus == DueStatusOverdue {
			c.OverdueTaskCount++
		}
	}
	if c.RequiredTaskCount > 0 {
		c.RequiredCompletionPercent = round2(100 * float64(c.SatisfiedRequiredTaskCount) / float64(c.RequiredTaskCount))
	}
	if c.ApplicableTaskCount > 0 {
		satisfiedTotal := c.SatisfiedRequiredTaskCount
		for _, e := range evals {
			if e.applicability == ApplicableYes && !e.def.Required && e.satisfied {
				satisfiedTotal++
			}
		}
		c.OverallCompletionPercent = round2(100 * float64(satisfiedTotal) / float64(c.ApplicableTaskCount))
	}
	return c
}

// buildEvidenceCoverage computes section 20's factual evidence coverage.
func buildEvidenceCoverage(evals []taskEval) EvidenceCoverage {
	var c EvidenceCoverage
	for _, e := range evals {
		if e.applicability != ApplicableYes {
			continue
		}
		required := e.def.EvidencePolicy.requiredCount()
		if required == 0 {
			continue
		}
		c.RequiredEvidenceCount += required
		present := len(e.state.Evidence)
		if present > required {
			present = required
		}
		c.PresentEvidenceCount += present
		if e.evidenceStatus == EvidenceStatusMissing {
			c.MissingEvidenceCount++
		}
	}
	if c.RequiredEvidenceCount > 0 {
		c.EvidenceCoveragePercent = round2(100 * float64(c.PresentEvidenceCount) / float64(c.RequiredEvidenceCount))
	}
	return c
}

// buildSignOffCoverage computes section 20's factual sign-off coverage.
func buildSignOffCoverage(evals []taskEval) SignOffCoverage {
	var c SignOffCoverage
	for _, e := range evals {
		if e.applicability != ApplicableYes {
			continue
		}
		required := len(e.def.ReviewPolicy.requiredRoles())
		if required == 0 {
			continue
		}
		c.RequiredSignOffCount += required
		switch e.reviewStatus {
		case ReviewStatusSatisfied:
			c.ValidSignOffCount += required
		case ReviewStatusMissing:
			c.MissingSignOffCount++
		case ReviewStatusConflict:
			c.ConflictingSignOffCount++
		}
	}
	if c.RequiredSignOffCount > 0 {
		c.SignOffCoveragePercent = round2(100 * float64(c.ValidSignOffCount) / float64(c.RequiredSignOffCount))
	}
	return c
}

// buildGateCoverage computes section 20's factual gate coverage across
// every applicable task's GateRules (required and optional alike, since
// this is a coverage report, not a satisfaction count).
func buildGateCoverage(evals []taskEval) GateCoverage {
	var c GateCoverage
	for _, e := range evals {
		if e.applicability != ApplicableYes {
			continue
		}
		for _, gr := range e.gateResults {
			c.RequiredGateCount++
			if !gr.available {
				c.UnavailableCount++
				continue
			}
			switch gr.fact.Status {
			case GatePass:
				c.PassCount++
			case GateWarning:
				c.WarningCount++
			case GateFail:
				c.FailCount++
			case GateNotApplicable:
				c.NotApplicableCount++
			case GateUnavailable:
				c.UnavailableCount++
			}
		}
	}
	return c
}

func round2(f float64) float64 {
	return float64(int(f*100+0.5)) / 100
}
