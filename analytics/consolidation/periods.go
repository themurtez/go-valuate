package consolidation

import "github.com/themurtez/go-valuate/financial"

// validatePeriodCoverage reports, for every selected entity and every
// period in periods, an IssuePeriodMissingForEntity when that entity's
// Dataset has no item at all for that period — the period-alignment
// validation the task brief requires. This never blocks consolidation
// (the entity simply contributes nothing for that period); it is purely
// advisory so a caller can see at a glance which entities have gaps in
// the common period set.
func validatePeriodCoverage(selected []EntityDataset, periods []financial.Period) []Issue {
	var issues []Issue
	for _, e := range selected {
		present := make(map[financial.Period]struct{})
		for _, item := range e.Dataset.Items {
			present[item.Period] = struct{}{}
		}
		for _, p := range periods {
			if _, ok := present[p]; ok {
				continue
			}
			issues = append(issues, Issue{
				Code:     IssuePeriodMissingForEntity,
				Severity: SeverityWarning,
				EntityID: e.EntityID,
				Period:   p,
				Message:  "entity \"" + e.EntityID + "\" has no data for period \"" + string(p) + "\"",
			})
		}
	}
	return issues
}
