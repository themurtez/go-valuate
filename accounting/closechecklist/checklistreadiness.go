package closechecklist

// checklistReadinessInputs is every fact deriveChecklistReadiness needs,
// gathered so the decision rule itself reads as one flat function —
// mirrors closequality's readiness.go and reconciliation's status.go
// precedent of isolating the status decision into one small, documented
// function.
type checklistReadinessInputs struct {
	structurallyInvalid bool

	anyApplicableStarted bool

	allRequiredSatisfied      bool
	preparationTasksSatisfied bool // required tasks outside final review/approval sections/roles
	finalReviewSatisfied      bool

	anyEffectiveBlocker bool

	periodState PeriodState
}

// deriveChecklistReadiness applies the fixed decision table (section
// 18):
//
//	INVALID:
//	  the template/instance is structurally invalid (HasErrors(Issues)).
//
//	NOT_STARTED:
//	  no applicable task has started.
//
//	IN_PROGRESS:
//	  required applicable tasks remain unsatisfied.
//
//	READY_FOR_REVIEW:
//	  every required applicable task outside final review/approval is
//	  satisfied, but required final review/sign-off tasks remain
//	  unsatisfied.
//
//	READY_TO_CLOSE:
//	  every required applicable task/gate/evidence/sign-off is satisfied
//	  and no effective blocker remains.
//
//	CLOSED / CLOSED_WITH_EXCEPTIONS:
//	  only reachable when the caller's own PeriodState is CLOSED/LOCKED
//	  — CLOSED when requirements are satisfied, CLOSED_WITH_EXCEPTIONS
//	  when the caller declared the period closed/locked while effective
//	  unresolved requirements remain.
//
// This function never mutates its input and performs no I/O.
func deriveChecklistReadiness(in checklistReadinessInputs) ChecklistReadiness {
	if in.structurallyInvalid {
		return ChecklistInvalid
	}

	closedByCaller := isClosedOrLocked(in.periodState)

	switch {
	case !in.anyApplicableStarted && !closedByCaller:
		return ChecklistNotStarted
	case in.allRequiredSatisfied && !in.anyEffectiveBlocker:
		if closedByCaller {
			return ChecklistClosed
		}
		return ChecklistReadyToClose
	case in.preparationTasksSatisfied && !in.finalReviewSatisfied:
		if closedByCaller {
			return ChecklistClosedWithExceptions
		}
		return ChecklistReadyForReview
	default:
		if closedByCaller {
			return ChecklistClosedWithExceptions
		}
		return ChecklistInProgress
	}
}
