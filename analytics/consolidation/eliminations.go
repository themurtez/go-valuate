package consolidation

import "github.com/themurtez/go-valuate/financial"

// elimKey identifies one (EntityID, Code, Period) cell an Elimination
// targets.
type elimKey struct {
	entityID string
	code     financial.Code
	period   financial.Period
}

// resolveEliminations validates in against entityByID/selected/periods and
// returns a lookup from elimKey to the total amount to remove at that cell
// (an entity/code/period may be targeted by more than one Elimination, in
// which case their Amounts are summed), the issues raised, and the flat
// list of Eliminations actually applied (known, selected EntityID).
func resolveEliminations(in []Elimination, entityByID map[string]EntityDataset, selected []EntityDataset, periods []financial.Period) (map[elimKey]float64, []Issue, []Elimination) {
	selectedIDs := make(map[string]struct{}, len(selected))
	for _, e := range selected {
		selectedIDs[e.EntityID] = struct{}{}
	}
	periodSet := make(map[financial.Period]struct{}, len(periods))
	for _, p := range periods {
		periodSet[p] = struct{}{}
	}

	byCell := make(map[elimKey]float64)
	var issues []Issue
	var applied []Elimination

	for _, elim := range in {
		entity, ok := entityByID[elim.EntityID]
		if !ok {
			issues = append(issues, Issue{
				Code:     IssueUnknownEliminationEntity,
				Severity: SeverityWarning,
				EntityID: elim.EntityID,
				Period:   elim.Period,
				Message:  "elimination names entity \"" + elim.EntityID + "\", which has no matching Input.Entities entry",
			})
			continue
		}
		if _, ok := selectedIDs[elim.EntityID]; !ok {
			issues = append(issues, Issue{
				Code:     IssueEliminationEntityNotSelected,
				Severity: SeverityWarning,
				EntityID: elim.EntityID,
				Period:   elim.Period,
				Message:  "elimination names entity \"" + elim.EntityID + "\", which is not among the selected entities",
			})
			continue
		}
		if _, ok := periodSet[elim.Period]; !ok {
			issues = append(issues, Issue{
				Code:     IssueEliminationPeriodOutOfScope,
				Severity: SeverityWarning,
				EntityID: elim.EntityID,
				Period:   elim.Period,
				Message:  "elimination for entity \"" + elim.EntityID + "\" targets period \"" + string(elim.Period) + "\", which is not among Input.Periods and can never be subtracted from anything",
			})
		}
		if !hasMatchingItem(entity.Dataset.Items, elim.Code, elim.Period) {
			issues = append(issues, Issue{
				Code:     IssueEliminationTargetNotFound,
				Severity: SeverityWarning,
				EntityID: elim.EntityID,
				Period:   elim.Period,
				Message:  "elimination for entity \"" + elim.EntityID + "\" targets " + string(elim.Code) + "/" + string(elim.Period) + ", which has no matching item in that entity's Dataset",
			})
		}
		k := elimKey{entityID: elim.EntityID, code: elim.Code, period: elim.Period}
		byCell[k] += elim.Amount
		applied = append(applied, elim)
	}

	return byCell, issues, applied
}

// hasMatchingItem reports whether items contains an entry for (code,
// period).
func hasMatchingItem(items []financial.NormalizedItem, code financial.Code, period financial.Period) bool {
	for _, it := range items {
		if it.Code == code && it.Period == period {
			return true
		}
	}
	return false
}
