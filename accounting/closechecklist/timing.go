package closechecklist

import "time"

// checkTimingConsistency evaluates section 24's factual timing checks
// for one task against its dependencies. Findings are always
// non-blocking (INFO/WARNING) unless Policy explicitly opts a specific
// FindingCode into blocking via Policy.BlockingTimingFindingCodes.
func checkTimingConsistency(t TaskDefinition, e taskEval, v validated, evals map[string]taskEval, policy Policy) []Finding {
	var findings []Finding

	if e.state.CompletedAt != nil {
		for _, dep := range v.graph.edges[t.TaskCode] {
			depEval, ok := evals[dep.DependsOnTaskCode]
			if !ok || depEval.state.CompletedAt == nil {
				continue
			}
			if e.state.CompletedAt.Before(*depEval.state.CompletedAt) {
				findings = append(findings, Finding{
					Code:     FindingTaskCompletedBeforeDependency,
					Severity: timingSeverity(policy, FindingTaskCompletedBeforeDependency),
					Message:  "Task " + t.TaskCode + " was completed before its dependency " + dep.DependsOnTaskCode + ".",
					TaskCode: t.TaskCode,
					Ref:      dep.DependsOnTaskCode,
				})
			}
		}
	}

	for _, so := range e.state.SignOffs {
		if e.state.CompletedAt != nil && so.SignedAt.Before(*e.state.CompletedAt) && so.Role != SignOffPreparer {
			findings = append(findings, Finding{
				Code:     FindingSignOffBeforeTaskCompletion,
				Severity: timingSeverity(policy, FindingSignOffBeforeTaskCompletion),
				Message:  "A sign-off for task " + t.TaskCode + " was recorded before the task's completion.",
				TaskCode: t.TaskCode,
				Ref:      so.SignOffID,
			})
		}
	}

	return findings
}

func timingSeverity(policy Policy, code FindingCode) FindingSeverity {
	if policy.timingIsBlocking(code) {
		return FindingSeverityBlocking
	}
	return FindingSeverityWarning
}

// buildTimingAnalytics computes section 27's factual on-time/late
// summary across every applicable task, plus DaysToClose (from the
// earliest StartedAt to the latest CompletedAt among applicable tasks,
// when both exist) and DaysLateVsTarget (the checklist's overall
// completion date vs Period.TargetCloseDate, when the checklist is
// fully satisfied).
func buildTimingAnalytics(evals []taskEval, period Period) TimingAnalytics {
	var ta TimingAnalytics
	var totalLateDays int
	var earliestStart, latestComplete *time.Time

	for _, e := range evals {
		if e.applicability != ApplicableYes {
			continue
		}
		switch e.dueStatus {
		case DueStatusCompletedOnTime:
			ta.OnTimeTaskCount++
		case DueStatusCompletedLate:
			ta.LateTaskCount++
			if e.dueDate != nil && e.state.CompletedAt != nil {
				days := int(truncateDay(*e.state.CompletedAt).Sub(truncateDay(*e.dueDate)).Hours() / 24)
				if days > 0 {
					totalLateDays += days
					if days > ta.MaxDaysLate {
						ta.MaxDaysLate = days
					}
				}
			}
		}
		if e.state.StartedAt != nil && (earliestStart == nil || e.state.StartedAt.Before(*earliestStart)) {
			earliestStart = e.state.StartedAt
		}
		if e.state.CompletedAt != nil && (latestComplete == nil || e.state.CompletedAt.After(*latestComplete)) {
			latestComplete = e.state.CompletedAt
		}
	}

	if ta.LateTaskCount > 0 {
		ta.AverageDaysLate = float64(totalLateDays) / float64(ta.LateTaskCount)
	}

	if earliestStart != nil && latestComplete != nil {
		days := int(truncateDay(*latestComplete).Sub(truncateDay(*earliestStart)).Hours() / 24)
		ta.DaysToClose = &days
	}

	allSatisfied := true
	for _, e := range evals {
		if e.applicability == ApplicableYes && e.def.Required && !e.satisfied {
			allSatisfied = false
			break
		}
	}
	if allSatisfied && latestComplete != nil && !isZeroTime(period.TargetCloseDate) {
		days := int(truncateDay(*latestComplete).Sub(truncateDay(period.TargetCloseDate)).Hours() / 24)
		ta.DaysLateVsTarget = &days
	}

	return ta
}
