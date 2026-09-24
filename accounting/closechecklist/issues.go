package closechecklist

// IssueSeverity mirrors every sibling package's identical two-level
// Issue severity scale (ledger.IssueSeverity, closequality.IssueSeverity,
// reconciliation's equivalent, etc.).
type IssueSeverity string

const (
	IssueSeverityError   IssueSeverity = "error"
	IssueSeverityWarning IssueSeverity = "warning"
)

// IssueCode is a stable identifier for a structural problem with this
// package's own Template/Instance/Policy input — distinct from
// FindingCode, which identifies a valid close condition needing
// attention (see findings.go and the package's task spec section 37's
// "Issues vs Findings" distinction).
type IssueCode string

const (
	IssueInvalidPeriod            IssueCode = "INVALID_PERIOD"
	IssueDuplicateTaskDefinition  IssueCode = "DUPLICATE_TASK_DEFINITION"
	IssueUnknownSection           IssueCode = "UNKNOWN_SECTION"
	IssueUnknownDependency        IssueCode = "UNKNOWN_DEPENDENCY"
	IssueSelfDependency           IssueCode = "SELF_DEPENDENCY"
	IssueDuplicateDependency      IssueCode = "DUPLICATE_DEPENDENCY"
	IssueInvalidDependency        IssueCode = "INVALID_DEPENDENCY"
	IssueDependencyCycle          IssueCode = "DEPENDENCY_CYCLE"
	IssueDuplicateTaskState       IssueCode = "DUPLICATE_TASK_STATE"
	IssueUnknownTaskState         IssueCode = "UNKNOWN_TASK_STATE"
	IssueDuplicateEvidence        IssueCode = "DUPLICATE_EVIDENCE"
	IssueDuplicateSignOff         IssueCode = "DUPLICATE_SIGNOFF"
	IssueInvalidTimestamp         IssueCode = "INVALID_TIMESTAMP"
	IssueInvalidDueRule           IssueCode = "INVALID_DUE_RULE"
	IssueInvalidGateRule          IssueCode = "INVALID_GATE_RULE"
	IssueInvalidApplicabilityRule IssueCode = "INVALID_APPLICABILITY_RULE"
	IssueInvalidException         IssueCode = "INVALID_EXCEPTION"
	IssueUnknownExceptionTask     IssueCode = "UNKNOWN_EXCEPTION_TASK"
	IssueDuplicateException       IssueCode = "DUPLICATE_EXCEPTION"
	IssueInvalidTemplateVersion   IssueCode = "INVALID_TEMPLATE_VERSION"
	IssueInvalidPolicy            IssueCode = "INVALID_POLICY"
	IssueDuplicateGateFact        IssueCode = "DUPLICATE_GATE_FACT"
	IssueInvalidGateFact          IssueCode = "INVALID_GATE_FACT"
	IssueNonFiniteValue           IssueCode = "NON_FINITE_VALUE"
)

// Issue reports a structural problem with the Template/Instance/Policy
// this package was given, as opposed to a Finding, which reports a valid
// close condition in the checklist itself.
type Issue struct {
	Code     IssueCode     `json:"code"`
	Severity IssueSeverity `json:"severity"`
	Message  string        `json:"message"`
	TaskCode string        `json:"task_code,omitempty"`
	Ref      string        `json:"ref,omitempty"`
}

// HasErrors reports whether any issue in issues is error-severity.
//
// Intentionally duplicated from ledger.HasErrors and this repository's
// other sibling HasErrors functions rather than shared — see
// financial/adjustments.HasErrors's doc comment for the full rationale.
func HasErrors(issues []Issue) bool {
	for _, i := range issues {
		if i.Severity == IssueSeverityError {
			return true
		}
	}
	return false
}
