package review

// IssueSeverity distinguishes a Decision validation problem that blocks
// application (SeverityIssueError) from one worth surfacing but non-blocking
// (SeverityIssueWarning) — the same two-severity model
// adjustments.IssueSeverity uses (SeverityError/SeverityWarning), renamed
// here only to avoid colliding with this package's own Severity type (a
// ReviewItem's urgency), which is a completely different concept sharing
// nothing but a similar-sounding name.
type IssueSeverity string

const (
	// SeverityIssueError means the Decision this Issue was found on is
	// invalid and was not applied (see ApplyResult.Invalid).
	SeverityIssueError IssueSeverity = "error"
	// SeverityIssueWarning means the Decision is applied but something
	// about it is worth a human's attention.
	SeverityIssueWarning IssueSeverity = "warning"
)

// IssueCode is a stable identifier for one kind of Decision validation
// problem, analogous to adjustments.IssueCode and reconciliation.CheckCode.
//
// This package defines its own IssueCode system rather than reusing
// valuation.IssueCode or adjustments.IssueCode: review's problem domain
// (unknown item IDs, wrong decision-payload-for-item-kind, invalid RowKind
// overrides, duplicate decisions targeting one item) doesn't overlap with
// either valuation's (rate/weight/value-basis validity) or adjustments'
// (adjustment-set internal consistency) domain, and forcing one of those
// types to also serve review would either leak review-specific codes into
// financial/valuation or vice versa — the same reasoning the repository
// README's error-taxonomy section already applies to keep valuation.Issue
// and adjustments.Issue separate from each other.
type IssueCode string

const (
	// IssueUnknownItemID means Decision.ItemID does not match any
	// ReviewItem.ID in the Plan.
	IssueUnknownItemID IssueCode = "UNKNOWN_ITEM_ID"
	// IssueWrongPayloadKind means Decision's populated payload field does
	// not match the targeted ReviewItem's Kind (e.g. an OCRNumeric payload
	// on a KindClassification item).
	IssueWrongPayloadKind IssueCode = "WRONG_PAYLOAD_KIND"
	// IssueInvalidAction means Decision.Action is not a legal Action for
	// the targeted ReviewItem's Kind (e.g. ActionOverride on a Kind that
	// only accepts accept/ignore).
	IssueInvalidAction IssueCode = "INVALID_ACTION"
	// IssueMissingPayload means Decision.Action requires a typed payload
	// (e.g. ActionOverride always does) but none was supplied.
	IssueMissingPayload IssueCode = "MISSING_PAYLOAD"
	// IssueInvalidCode means a classification override's Code fails
	// financial.IsValidCode.
	IssueInvalidCode IssueCode = "INVALID_CODE"
	// IssueInvalidRowKind means a structure override's RowKind is not one
	// of the four known financial.RowKind constants.
	IssueInvalidRowKind IssueCode = "INVALID_ROW_KIND"
	// IssueNonFiniteAmount means a numeric override (an OCR numeric replace,
	// or an adjustment amount modification) is NaN or +/-Inf.
	IssueNonFiniteAmount IssueCode = "NON_FINITE_AMOUNT"
	// IssueMalformedPeriod means a period override's Period is empty.
	IssueMalformedPeriod IssueCode = "MALFORMED_PERIOD"
	// IssueDuplicateItemID means the same Decision.ItemID appears more than
	// once within one Apply call. Detected the same way
	// adjustments.IssueDuplicateID is: a seen-map keyed by ID, with every
	// colliding Decision index reported.
	IssueDuplicateItemID IssueCode = "DUPLICATE_ITEM_ID"
	// IssueConflictingDecision means the same Decision.ItemID appears more
	// than once with different Action/payload values — a stricter subCase
	// of IssueDuplicateItemID reported when the duplicates are not even
	// identical repeats of each other.
	IssueConflictingDecision IssueCode = "CONFLICTING_DECISION"
	// IssueIncompatibleDecision means Action and payload are individually
	// well-formed but incompatible with each other or with the item's
	// current Status (e.g. deciding on an item Build never produced as
	// Required == true with an Action this package still permits, but the
	// combination is otherwise nonsensical for this Kind).
	IssueIncompatibleDecision IssueCode = "INCOMPATIBLE_DECISION"
)

// Issue is a single Decision validation finding, mirroring adjustments.Issue's
// shape exactly (AdjustmentID -> ItemID, IssueCode/IssueSeverity local to
// this package).
type Issue struct {
	// ItemID identifies the ReviewItem.ID (or attempted ItemID) the
	// offending Decision targeted. Never empty: even IssueUnknownItemID
	// reports the (invalid) ID that was supplied.
	ItemID   string        `json:"item_id,omitempty"`
	Code     IssueCode     `json:"code"`
	Severity IssueSeverity `json:"severity"`
	Message  string        `json:"message"`
}

// HasErrors reports whether any Issue in issues has SeverityIssueError,
// mirroring adjustments.HasErrors.
func HasErrors(issues []Issue) bool {
	for _, i := range issues {
		if i.Severity == SeverityIssueError {
			return true
		}
	}
	return false
}
