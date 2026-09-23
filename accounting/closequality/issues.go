package closequality

import "math"

// IssueSeverity mirrors every sibling package's identical two-level Issue
// severity scale (ledger.IssueSeverity, ar.IssueSeverity, etc.).
type IssueSeverity string

const (
	IssueSeverityError   IssueSeverity = "error"
	IssueSeverityWarning IssueSeverity = "warning"
)

// IssueCode is a stable identifier for an input/analysis problem with
// this package's own Input or Policy — distinct from FindingCode, which
// identifies a close/bookkeeping condition in the underlying books. See
// the package doc comment's "Issue vs Finding" distinction.
type IssueCode string

const (
	IssueInvalidPeriod                 IssueCode = "INVALID_PERIOD"
	IssueInvalidPolicy                 IssueCode = "INVALID_POLICY"
	IssueUnknownAccount                IssueCode = "UNKNOWN_ACCOUNT"
	IssueDuplicateReconciliationStatus IssueCode = "DUPLICATE_RECONCILIATION_STATUS"
	IssueDuplicateCloseTask            IssueCode = "DUPLICATE_CLOSE_TASK"
	IssueInvalidReconciliationValue    IssueCode = "INVALID_RECONCILIATION_VALUE"
	IssueNonFiniteValue                IssueCode = "NON_FINITE_VALUE"
	IssueInvalidAccountExpectation     IssueCode = "INVALID_ACCOUNT_EXPECTATION"
	IssueInvalidExpectedActivityRule   IssueCode = "INVALID_EXPECTED_ACTIVITY_RULE"
)

// Issue reports a problem with the Input or Policy this package was
// given, as opposed to a Finding, which reports a condition in the books
// themselves.
type Issue struct {
	Code      IssueCode     `json:"code"`
	Severity  IssueSeverity `json:"severity"`
	Message   string        `json:"message"`
	AccountID string        `json:"account_id,omitempty"`
	Ref       string        `json:"ref,omitempty"`
}

// HasErrors reports whether any issue in issues is error-severity.
//
// Intentionally duplicated from ledger.HasErrors and this repository's
// other sibling HasErrors functions rather than shared — see
// financial/adjustments.HasErrors's doc comment for the full rationale
// (each package's Issue is a distinct Go type with no common interface
// worth introducing for one boolean function).
func HasErrors(issues []Issue) bool {
	for _, i := range issues {
		if i.Severity == IssueSeverityError {
			return true
		}
	}
	return false
}

func isNonFinite(v float64) bool {
	return math.IsNaN(v) || math.IsInf(v, 0)
}
