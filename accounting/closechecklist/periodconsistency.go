package closechecklist

// checkPeriodConsistency evaluates section 23's factual period-state
// consistency check: a caller-reported CLOSED/LOCKED period whose
// required applicable tasks are not all satisfied is an inconsistency,
// surfaced as a Finding (not enforced/mutated). An OPEN period that
// happens to be fully ready is not itself an error.
func checkPeriodConsistency(periodState PeriodState, allRequiredSatisfied bool) []PeriodConsistencyFinding {
	if isClosedOrLocked(periodState) && !allRequiredSatisfied {
		return []PeriodConsistencyFinding{{
			Code:    string(FindingPeriodMarkedClosedWithIncompleteRequiredTasks),
			Message: "The period is marked " + string(periodState) + " but one or more required applicable tasks remain unsatisfied.",
		}}
	}
	return nil
}
