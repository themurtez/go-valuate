package closequality

// Calculate assesses one accounting period's bookkeeping quality and
// close readiness from whichever upstream results in in are available,
// under the rules in policy. See the package doc comment for the full
// scope and non-goals, and docs/CLOSE_QUALITY.md for the complete
// dimension/finding/readiness reference.
//
// Calculate is pure: it never mutates in, policy, or any of their
// nested slices/maps, performs no I/O, and returns byte-for-byte
// identical JSON for identical input on every call.
func Calculate(in Input, policy Policy) Result {
	return calculate(in, policy, nil)
}

// CalculateWithPrior is Calculate plus a pure comparison against a prior
// period's Result (Result.Comparison) — the prior Result is never used
// to change current-period calculations, only to report deltas. See
// ComparisonResult.
func CalculateWithPrior(in Input, policy Policy, prior Result) Result {
	return calculate(in, policy, &prior)
}

func calculate(in Input, policy Policy, prior *Result) Result {
	var issues []Issue
	if !in.Period.valid() {
		issues = append(issues, Issue{
			Code:     IssueInvalidPeriod,
			Severity: IssueSeverityError,
			Message:  "period is missing required fields or has an invalid/inconsistent date range",
		})
	}
	issues = append(issues, validatePolicy(policy)...)

	coverage := buildCoverage(in, policy, prior != nil)

	var allFindings []Finding
	var dims []DimensionResult

	appendDim := func(fs []Finding, d DimensionResult) {
		allFindings = append(allFindings, fs...)
		dims = append(dims, d)
	}

	fs, d := mineLedgerIntegrity(in)
	appendDim(fs, d)

	fs, d = mineTrialBalanceIntegrity(in)
	appendDim(fs, d)

	fs, d = mineFinancialStatementIntegrity(in, policy)
	appendDim(fs, d)

	fs, d = mineARControl(in, policy)
	appendDim(fs, d)

	fs, d = mineAPControl(in, policy)
	appendDim(fs, d)

	fs, d = mineJournalReview(in, policy)
	appendDim(fs, d)

	fs, d = mineBalanceSheetQuality(in, policy)
	appendDim(fs, d)

	plausFindings, plausDim, plausIssues := mineAccountBalancePlausibility(in, policy)
	issues = append(issues, plausIssues...)
	clearingFindings, _ := mineClearingAndStaleBalances(in, policy)
	plausDim = mergeClearingIntoDimension(plausDim, clearingFindings)
	appendDim(append(plausFindings, clearingFindings...), plausDim)

	recFindings, recDim, _, recIssues := mineReconciliationCoverage(in, policy)
	issues = append(issues, recIssues...)
	appendDim(recFindings, recDim)

	taskFindings, taskDim, taskSummary, taskIssues := mineCloseTaskCompletion(in, policy)
	issues = append(issues, taskIssues...)
	appendDim(taskFindings, taskDim)

	fs, d = minePeriodLockPostCloseActivity(in, policy)
	appendDim(fs, d)

	completenessFindings, completenessDim := mineDataCompleteness(coverage, policy)
	extraCompleteness := append(mineExpectedActivity(in, policy), mineExpectedPeriodEntries(in, policy)...)
	completenessDim = mergeFindingsIntoDimension(completenessDim, extraCompleteness)
	appendDim(append(completenessFindings, extraCompleteness...), completenessDim)

	sortDimensionsInPlace(dims)
	allFindings = dedupFindings(allFindings)
	sortFindings(allFindings)

	var blockers, warnings, info []Finding
	for _, f := range allFindings {
		switch f.Severity {
		case SeverityBlocking:
			blockers = append(blockers, f)
		case SeverityWarning:
			warnings = append(warnings, f)
		default:
			info = append(info, f)
		}
	}

	dimCoverage := buildDimensionCoverage(dims)
	status := deriveStatus(dimCoverage, blockers, warnings)

	result := Result{
		Period:            in.Period,
		Status:            status,
		Coverage:          coverage,
		DimensionCoverage: dimCoverage,
		Dimensions:        dims,
		Blockers:          blockers,
		Warnings:          warnings,
		Information:       info,
		MissingInputs:     coverage.MissingModules,
		UnresolvedItems:   append(append([]Finding{}, blockers...), warnings...),
		CloseTaskSummary:  taskSummary,
		Versions:          currentVersions(),
		Issues:            issues,
	}

	if prior != nil {
		cmp := compareToPrior(result, *prior)
		result.Comparison = &cmp
	}

	return result
}

// mergeClearingIntoDimension folds clearing/stale-balance findings
// (which are evaluated by a separate helper from the rest of
// AccountBalancePlausibility because they route through ShouldClear
// rather than sign/range checks) into the same DimensionResult.
func mergeClearingIntoDimension(dim DimensionResult, extra []Finding) DimensionResult {
	return mergeFindingsIntoDimension(dim, extra)
}

// mergeFindingsIntoDimension folds additional findings (produced by a
// helper that does not build its own DimensionResult) into an
// already-built DimensionResult, marking it assessed and widening its
// status to the worst of its existing status and the new findings'.
func mergeFindingsIntoDimension(dim DimensionResult, extra []Finding) DimensionResult {
	if len(extra) == 0 {
		return dim
	}
	dim.Assessed = true
	if dim.Status == DimensionUnassessed {
		dim.Status = DimensionPass
	}
	for _, f := range extra {
		dim.FindingCodes = append(dim.FindingCodes, f.Code)
		if s := severityToDimensionStatus(f.Severity); dimensionStatusRank[s] < dimensionStatusRank[dim.Status] {
			dim.Status = s
		}
	}
	return dim
}
