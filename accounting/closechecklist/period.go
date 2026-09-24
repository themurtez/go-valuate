package closechecklist

import "time"

// Period is the caller-supplied close period this checklist evaluates.
// Dates are explicit; this package never infers a fiscal period from a
// label or calendar convention.
type Period struct {
	PeriodID        string    `json:"period_id"`
	Label           string    `json:"label,omitempty"`
	StartDate       time.Time `json:"start_date"`
	EndDate         time.Time `json:"end_date"`
	TargetCloseDate time.Time `json:"target_close_date"`

	// FiscalYear/FiscalPeriodNumber are optional caller-supplied fiscal
	// metadata, carried through for display/provenance only — never used
	// to infer StartDate/EndDate or any due-date computation.
	FiscalYear         int    `json:"fiscal_year,omitempty"`
	FiscalPeriodNumber int    `json:"fiscal_period_number,omitempty"`
	FiscalPeriodLabel  string `json:"fiscal_period_label,omitempty"`
}

// PeriodState is the caller-tracked lifecycle state of a close period.
// This package evaluates consistency between PeriodState and computed
// checklist readiness (see [Result.PeriodConsistency]); it never enforces
// or mutates PeriodState itself.
type PeriodState string

const (
	PeriodStateOpen           PeriodState = "OPEN"
	PeriodStateInProgress     PeriodState = "IN_PROGRESS"
	PeriodStateReadyForReview PeriodState = "READY_FOR_REVIEW"
	PeriodStateReadyToClose   PeriodState = "READY_TO_CLOSE"
	PeriodStateClosed         PeriodState = "CLOSED"
	PeriodStateLocked         PeriodState = "LOCKED"
	PeriodStateReopened       PeriodState = "REOPENED"
)

func isRecognizedPeriodState(s PeriodState) bool {
	switch s {
	case "", PeriodStateOpen, PeriodStateInProgress, PeriodStateReadyForReview,
		PeriodStateReadyToClose, PeriodStateClosed, PeriodStateLocked, PeriodStateReopened:
		return true
	default:
		return false
	}
}

// isClosedOrLocked reports whether s is one of the two terminal
// caller-reported states this package treats as "the caller believes the
// period is finalized" for period-state-consistency checks (section 23).
func isClosedOrLocked(s PeriodState) bool {
	return s == PeriodStateClosed || s == PeriodStateLocked
}

func isZeroTime(t time.Time) bool { return t.IsZero() }
