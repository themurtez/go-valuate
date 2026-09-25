package advisory

// validateInput checks in/the already-resolved policy for structural
// problems Build should report — task section 94's taxonomy. Advisory
// only in every case except when in supplies nothing at all (see
// hasAnySupplied), matching every sibling composition package's identical
// "no input at all" hard-stop convention (reporting/management.Calculate,
// transactions/salereadiness.Calculate).
func validateInput(in Input, policy Policy) []Issue {
	var issues []Issue

	if !hasAnySupplied(in) {
		issues = append(issues, Issue{
			Code: IssueInvalidInput, Severity: IssueSeverityError,
			Message: "no financial, operating, close, transaction, valuation, or KPI input supplied; nothing to compose",
		})
	}

	seenCodes := make(map[string]bool, len(in.Periods))
	seenCurrent, seenPrior := false, false
	for _, p := range in.Periods {
		if p.Code == "" {
			issues = append(issues, Issue{Code: IssueInvalidPeriod, Severity: IssueSeverityWarning, Message: "a Periods entry has an empty Code"})
			continue
		}
		if seenCodes[p.Code] {
			issues = append(issues, Issue{Code: IssueInvalidPeriod, Severity: IssueSeverityWarning, Message: "duplicate period code: " + p.Code, Period: p.Code})
		}
		seenCodes[p.Code] = true
		if p.IsCurrent {
			if seenCurrent {
				issues = append(issues, Issue{Code: IssueInvalidPeriod, Severity: IssueSeverityWarning, Message: "more than one Periods entry marked IsCurrent; the first is used"})
			}
			seenCurrent = true
		}
		if p.IsPrior {
			if seenPrior {
				issues = append(issues, Issue{Code: IssueInvalidPeriod, Severity: IssueSeverityWarning, Message: "more than one Periods entry marked IsPrior; the first is used"})
			}
			seenPrior = true
		}
	}

	for _, sp := range policy.SourcePreferences {
		if sp.MetricCode == "" {
			issues = append(issues, Issue{Code: IssueInvalidSourcePreference, Severity: IssueSeverityWarning, Message: "a SourcePreference entry has an empty MetricCode"})
		}
		if len(sp.OrderedSources) == 0 {
			issues = append(issues, Issue{Code: IssueInvalidSourcePreference, Severity: IssueSeverityWarning, Message: "SourcePreference for " + sp.MetricCode + " has no OrderedSources", SourceCode: sp.MetricCode})
		}
	}

	for _, pr := range policy.PriorityRules {
		if pr.ForcePriority == "" {
			issues = append(issues, Issue{Code: IssueInvalidPriorityRule, Severity: IssueSeverityWarning, Message: "a PriorityRule has an empty ForcePriority"})
		}
		if pr.MatchSourceModule == "" && pr.MatchSourceCode == "" && pr.MatchEntityRef == "" {
			issues = append(issues, Issue{Code: IssueInvalidPriorityRule, Severity: IssueSeverityWarning, Message: "a PriorityRule matches every fact (every Match field empty)"})
		}
	}

	for _, a := range in.CallerActions {
		if a.ActionCode == "" {
			issues = append(issues, Issue{Code: IssueInvalidActionOverride, Severity: IssueSeverityWarning, Message: "a caller-supplied ActionItem has an empty ActionCode"})
			continue
		}
		if !isKnownActionStatus(a.Status) && a.Status != "" {
			issues = append(issues, Issue{Code: IssueUnknownActionStatus, Severity: IssueSeverityWarning, Message: "caller-supplied ActionItem " + a.ActionCode + " has an unrecognized Status", SourceCode: a.ActionCode})
		}
	}

	if in.Prior != nil && in.Prior.SchemaVersion == "" {
		issues = append(issues, Issue{Code: IssueInvalidPriorResult, Severity: IssueSeverityWarning, Message: "Input.Prior has an empty SchemaVersion; prior-comparison sections may be incomplete"})
	}

	return issues
}

func isKnownActionStatus(s ActionStatus) bool {
	switch s {
	case ActionStatusOpen, ActionStatusInProgress, ActionStatusResolved, ActionStatusDeferred, ActionStatusNotRequired:
		return true
	default:
		return false
	}
}

// hasAnySupplied reports whether in has any usable content at all —
// mirrors every sibling composition package's identical "at least one
// optional field non-zero" gate (reporting/management.Calculate,
// transactions/salereadiness.Calculate, portfolio/diagnostics.Calculate).
func hasAnySupplied(in Input) bool {
	return len(suppliedModuleNames(in)) > 0 || len(in.KPIValues) > 0 || len(in.CallerActions) > 0
}
