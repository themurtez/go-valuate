package closechecklist

import "time"

// ExceptionPolicy is the fixed set of exception-allowance kinds a task
// may declare. The zero value is ExceptionNotAllowed.
type ExceptionPolicy string

const (
	ExceptionNotAllowed          ExceptionPolicy = "NOT_ALLOWED"
	ExceptionAllowed             ExceptionPolicy = "ALLOWED"
	ExceptionAllowedWithApproval ExceptionPolicy = "ALLOWED_WITH_APPROVAL"
)

func isRecognizedExceptionPolicy(p ExceptionPolicy) bool {
	switch p {
	case "", ExceptionNotAllowed, ExceptionAllowed, ExceptionAllowedWithApproval:
		return true
	default:
		return false
	}
}

// Exception is one caller-approved exception against a specific task's
// blocking condition. A valid Exception never deletes the underlying
// blocker — see [applyException] and section 22.
type Exception struct {
	ExceptionID   string     `json:"exception_id"`
	TaskCode      string     `json:"task_code"`
	ReasonCode    string     `json:"reason_code"`
	Description   string     `json:"description,omitempty"`
	ApprovedByRef string     `json:"approved_by_ref,omitempty"`
	ApprovedAt    time.Time  `json:"approved_at"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
}

// exceptionState is one Exception's resolved state as of EvaluationDate,
// used by readiness.go to decide effective blocking.
type exceptionState struct {
	exception Exception
	// permitted is true when the task's ExceptionPolicy allows an
	// exception at all (ALLOWED or ALLOWED_WITH_APPROVAL, with
	// ALLOWED_WITH_APPROVAL additionally requiring ApprovedByRef to be
	// non-empty).
	permitted bool
	// expired is true when ExpiresAt is set and before EvaluationDate.
	expired bool
}

// evaluateException resolves one Exception against the owning task's
// ExceptionPolicy and the evaluation date. It never mutates exc.
func evaluateException(exc Exception, policy ExceptionPolicy, evaluationDate time.Time) exceptionState {
	permitted := false
	switch policy {
	case ExceptionAllowed:
		permitted = true
	case ExceptionAllowedWithApproval:
		permitted = exc.ApprovedByRef != ""
	default:
		permitted = false
	}

	expired := false
	if exc.ExpiresAt != nil && !isZeroTime(evaluationDate) && exc.ExpiresAt.Before(evaluationDate) {
		expired = true
	}

	return exceptionState{exception: exc, permitted: permitted, expired: expired}
}

// effective reports whether this exception is currently suppressing
// blocking (permitted, not expired). The underlying blocker itself must
// remain visible regardless — see readiness.go's blocker construction,
// which never omits a blocker solely because an exception applies.
func (e exceptionState) effective() bool {
	return e.permitted && !e.expired
}
