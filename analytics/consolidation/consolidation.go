package consolidation

import "sort"

// Calculate combines in.Entities into a single consolidated
// financial.FinancialDataset under opts. It never mutates any
// EntityDataset.Dataset and performs no I/O.
func Calculate(in Input) Result {
	mode := resolveMode(in.Mode)
	policy := resolvePolicy(in.Policy, in.Entities)
	result := Result{FormulaVersion: FormulaVersion, Mode: mode, Policy: policy}

	if len(in.Entities) == 0 {
		result.Errors = append(result.Errors, Issue{
			Code:     IssueNoEntities,
			Severity: SeverityError,
			Message:  "no entities supplied; consolidation requires at least one",
		})
		result.ReconciliationIssues = result.Errors
		return result
	}
	if len(in.Periods) == 0 {
		result.Errors = append(result.Errors, Issue{
			Code:     IssueNoPeriods,
			Severity: SeverityError,
			Message:  "no common reporting periods supplied; consolidation never infers which periods to combine",
		})
		result.ReconciliationIssues = result.Errors
		return result
	}
	if dup, ok := firstDuplicateEntityID(in.Entities); ok {
		result.Errors = append(result.Errors, Issue{
			Code:     IssueDuplicateEntityID,
			Severity: SeverityError,
			EntityID: dup,
			Message:  "entity ID \"" + dup + "\" appears more than once in Input.Entities",
		})
		result.ReconciliationIssues = result.Errors
		return result
	}

	var issues []Issue
	entityByID := make(map[string]EntityDataset, len(in.Entities))
	for _, e := range in.Entities {
		entityByID[e.EntityID] = e
	}

	selected, selIssues := resolveSelectedEntities(policy.SelectedEntityIDs, entityByID)
	issues = append(issues, selIssues...)

	targetCurrency, curIssue, ok := resolveTargetCurrency(policy.TargetCurrency, selected)
	if !ok {
		issues = append(issues, curIssue)
		result.Errors = append(result.Errors, filterSeverity(issues, SeverityError)...)
		result.Warnings = append(result.Warnings, filterSeverity(issues, SeverityWarning)...)
		result.ReconciliationIssues = issues
		return result
	}
	result.Available = true

	elimByEntity, elimIssues, applied := resolveEliminations(in.Eliminations, entityByID, selected, in.Periods)
	issues = append(issues, elimIssues...)

	contributions, conversions, contribIssues := buildEntityContributions(selected, in.Periods, mode, elimByEntity, targetCurrency, in.CurrencyRates)
	issues = append(issues, contribIssues...)

	periodIssues := validatePeriodCoverage(selected, in.Periods)
	issues = append(issues, periodIssues...)

	sort.Slice(contributions, func(i, j int) bool { return contributions[i].EntityID < contributions[j].EntityID })
	result.EntityContributions = contributions

	result.Consolidated = buildConsolidatedDataset(contributions, targetCurrency)

	sort.Slice(applied, func(i, j int) bool {
		if applied[i].EntityID != applied[j].EntityID {
			return applied[i].EntityID < applied[j].EntityID
		}
		if applied[i].Code != applied[j].Code {
			return applied[i].Code < applied[j].Code
		}
		return applied[i].Period < applied[j].Period
	})
	result.EliminationsApplied = applied

	result.CurrencyConversions = conversions

	sort.SliceStable(issues, func(i, j int) bool {
		if issues[i].EntityID != issues[j].EntityID {
			return issues[i].EntityID < issues[j].EntityID
		}
		return issues[i].Period < issues[j].Period
	})
	result.ReconciliationIssues = issues
	result.Errors = filterSeverity(issues, SeverityError)
	result.Warnings = filterSeverity(issues, SeverityWarning)

	return result
}

// firstDuplicateEntityID returns the first EntityID that appears more than
// once in entities, in input order, and whether one was found.
func firstDuplicateEntityID(entities []EntityDataset) (string, bool) {
	seen := make(map[string]struct{}, len(entities))
	for _, e := range entities {
		if _, ok := seen[e.EntityID]; ok {
			return e.EntityID, true
		}
		seen[e.EntityID] = struct{}{}
	}
	return "", false
}

// resolveSelectedEntities looks up each Policy.SelectedEntityIDs entry
// against entityByID, in the order supplied, skipping and reporting any ID
// with no match.
func resolveSelectedEntities(ids []string, entityByID map[string]EntityDataset) ([]EntityDataset, []Issue) {
	var selected []EntityDataset
	var issues []Issue
	for _, id := range ids {
		e, ok := entityByID[id]
		if !ok {
			issues = append(issues, Issue{
				Code:     IssueUnknownSelectedEntity,
				Severity: SeverityWarning,
				EntityID: id,
				Message:  "Policy.SelectedEntityIDs named \"" + id + "\", which has no matching Input.Entities entry",
			})
			continue
		}
		selected = append(selected, e)
	}
	return selected, issues
}

// resolveTargetCurrency determines the consolidated dataset's currency. If
// Policy.TargetCurrency is set, it is used as-is. Otherwise, every selected
// entity must share exactly one non-empty currency, which is then used.
// Resolution fails — never silently returning an empty string, since
// financial.FinancialDataset.Currency is "required and never empty" — if
// more than one distinct currency is present, or if no selected entity has
// a non-empty currency at all (e.g. every EntityDataset.Dataset.Currency
// was left empty, or no entity was selected in the first place).
func resolveTargetCurrency(targetCurrency string, selected []EntityDataset) (string, Issue, bool) {
	if targetCurrency != "" {
		return targetCurrency, Issue{}, true
	}

	seen := make(map[string]struct{})
	for _, e := range selected {
		if e.Dataset.Currency == "" {
			continue
		}
		seen[e.Dataset.Currency] = struct{}{}
	}
	if len(seen) == 1 {
		for c := range seen {
			return c, Issue{}, true
		}
	}

	msg := "no selected entity has a non-empty currency and Policy.TargetCurrency was not supplied"
	if len(seen) > 1 {
		msg = "selected entities report more than one currency and Policy.TargetCurrency was not supplied"
	}
	return "", Issue{
		Code:     IssueMissingTargetCurrency,
		Severity: SeverityError,
		Message:  msg,
	}, false
}

// filterSeverity returns the subset of issues with the given severity, in
// their existing relative order.
func filterSeverity(issues []Issue, sev IssueSeverity) []Issue {
	var out []Issue
	for _, i := range issues {
		if i.Severity == sev {
			out = append(out, i)
		}
	}
	return out
}
