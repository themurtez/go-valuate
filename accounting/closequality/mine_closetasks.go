package closequality

func closeTaskIssues(tasks []CloseTaskStatus) (map[string]CloseTaskStatus, []Issue) {
	byCode := make(map[string]CloseTaskStatus, len(tasks))
	var issues []Issue
	for _, t := range tasks {
		if t.Code == "" {
			continue
		}
		if _, dup := byCode[t.Code]; dup {
			issues = append(issues, Issue{
				Code:     IssueDuplicateCloseTask,
				Severity: IssueSeverityWarning,
				Message:  "duplicate close task code; only the first is used",
				Ref:      t.Code,
			})
			continue
		}
		byCode[t.Code] = t
	}
	return byCode, issues
}

// mineCloseTaskCompletion translates Input.CloseTasks into
// DimensionCloseTaskCompletion findings and the CloseTaskSummary
// aggregate. This package executes no tasks and manages no
// assignment/workflow — OwnerRef/EvidenceRef are opaque passthroughs.
func mineCloseTaskCompletion(in Input, policy Policy) ([]Finding, DimensionResult, CloseTaskSummary, []Issue) {
	b := newDimensionBuilder(DimensionCloseTaskCompletion)

	if len(in.CloseTasks) == 0 {
		b.unavailable("no close tasks supplied")
		return nil, b.build(), CloseTaskSummary{}, nil
	}
	b.markAssessed()

	byCode, issues := closeTaskIssues(in.CloseTasks)
	codes := make([]string, 0, len(byCode))
	for code := range byCode {
		codes = append(codes, code)
	}
	codes = uniqueSortedStrings(codes)

	summary := CloseTaskSummary{}
	var findings []Finding
	for _, code := range codes {
		t := byCode[code]
		summary.TotalCount++
		if t.Required {
			summary.RequiredCount++
		}
		switch t.Status {
		case CloseTaskCompleted:
			summary.CompletedCount++
			if t.Required {
				summary.RequiredCompletedCount++
			}
		case CloseTaskInProgress:
			summary.InProgressCount++
		case CloseTaskNotStarted:
			summary.NotStartedCount++
		case CloseTaskBlocked:
			summary.BlockedCount++
		case CloseTaskNotApplicable:
			summary.NotApplicableCount++
		}

		if !t.Required {
			continue
		}
		switch t.Status {
		case CloseTaskBlocked:
			f := Finding{
				Code:      FindingRequiredCloseTaskBlocked,
				Dimension: DimensionCloseTaskCompletion,
				Severity:  SeverityBlocking,
				Message:   "required close task is blocked",
				Evidence: Evidence{
					TaskCode:   t.Code,
					Required:   t.Required,
					TaskStatus: string(t.Status),
					Label:      t.Description,
				},
				SourceModule: SourceCloseQuality,
			}
			findings = append(findings, f)
			b.record(f)
		case CloseTaskNotStarted, CloseTaskInProgress:
			sev := SeverityWarning
			if policy.CloseTaskRules.RequiredIncompleteBlocks {
				sev = SeverityBlocking
			}
			f := Finding{
				Code:      FindingRequiredCloseTaskIncomplete,
				Dimension: DimensionCloseTaskCompletion,
				Severity:  sev,
				Message:   "required close task is not yet completed",
				Evidence: Evidence{
					TaskCode:   t.Code,
					Required:   t.Required,
					TaskStatus: string(t.Status),
					Label:      t.Description,
				},
				SourceModule: SourceCloseQuality,
			}
			findings = append(findings, f)
			b.record(f)
		}
	}

	if summary.RequiredCount > 0 {
		summary.CompletionPercent = float64(summary.RequiredCompletedCount) / float64(summary.RequiredCount)
	}
	if len(findings) == 0 {
		b.note("all required close tasks completed")
	}

	return findings, b.build(), summary, issues
}
