package reconciliation

// IssueSeverity mirrors every sibling package's identical two-level Issue
// severity scale (ledger.IssueSeverity, ar.IssueSeverity, closequality.IssueSeverity,
// etc.) — intentionally duplicated per this repository's convention (see
// financial/adjustments.HasErrors's doc comment for the full rationale).
type IssueSeverity string

const (
	IssueSeverityError   IssueSeverity = "error"
	IssueSeverityWarning IssueSeverity = "warning"
)

// IssueCode is a stable identifier for an input/configuration/engine
// problem with this package's own Input or MatchingPolicy — distinct
// from FindingCode, which identifies a reconciliation condition needing
// review in the underlying data. See the package doc comment's "Findings
// vs issues" distinction, mirrored from closequality's identical
// Issue-vs-Finding split.
//
// Only the codes below are ever emitted — this is a closed, stable set
// (task section 56/57: "only define emitted codes").
type IssueCode string

const (
	IssueInvalidReconciliationKey       IssueCode = "INVALID_RECONCILIATION_KEY"
	IssueInvalidAsOfDate                IssueCode = "INVALID_AS_OF_DATE"
	IssueDuplicateBookItem              IssueCode = "DUPLICATE_BOOK_ITEM"
	IssueDuplicateExternalItem          IssueCode = "DUPLICATE_EXTERNAL_ITEM"
	IssueInvalidAmount                  IssueCode = "INVALID_AMOUNT"
	IssueNonFiniteAmount                IssueCode = "NON_FINITE_AMOUNT"
	IssueInvalidDirection               IssueCode = "INVALID_DIRECTION"
	IssueMixedCurrency                  IssueCode = "MIXED_CURRENCY"
	IssueInvalidPolicy                  IssueCode = "INVALID_POLICY"
	IssueInvalidConfirmedMatch          IssueCode = "INVALID_CONFIRMED_MATCH"
	IssueUnknownMatchItem               IssueCode = "UNKNOWN_MATCH_ITEM"
	IssueItemUsedInMultipleMatches      IssueCode = "ITEM_USED_IN_MULTIPLE_MATCHES"
	IssueInvalidReconcilingItem         IssueCode = "INVALID_RECONCILING_ITEM"
	IssueCompositeSearchLimitReached    IssueCode = "COMPOSITE_SEARCH_LIMIT_REACHED"
	IssueSuppliedDerivedBalanceMismatch IssueCode = "SUPPLIED_DERIVED_BALANCE_MISMATCH"
)

// Issue reports a problem with the Input, MatchingPolicy, or engine
// operation this package was given, as opposed to a Finding, which
// reports a reconciliation condition in the underlying data itself.
type Issue struct {
	Code     IssueCode     `json:"code"`
	Severity IssueSeverity `json:"severity"`
	Message  string        `json:"message"`
	// ItemID, when set, identifies the BookItem/ExternalItem/ConfirmedMatch/
	// ReconcilingItem this issue concerns.
	ItemID string `json:"item_id,omitempty"`
	// Ref is an additional free-form reference (e.g. a MatchID, an
	// AccountID) when ItemID alone is insufficient context.
	Ref string `json:"ref,omitempty"`
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
