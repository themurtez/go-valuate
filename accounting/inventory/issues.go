package inventory

// IssueSeverity distinguishes a problem that excludes a row or blocks some
// portion of the analysis (SeverityError) from one that is advisory only
// (SeverityWarning) — the same two-severity model every sibling package in
// this repository uses.
type IssueSeverity string

const (
	SeverityError   IssueSeverity = "error"
	SeverityWarning IssueSeverity = "warning"
)

// IssueCode is a stable identifier for one kind of input/configuration/
// integrity problem — task section 66: an Issue is never a business or
// accounting condition (that is a Flag; see flags.go). Only codes this
// package actually emits are defined — task section 67.
type IssueCode string

const (
	IssueInvalidAsOfDate IssueCode = "INVALID_AS_OF_DATE"
	IssueInvalidPeriod   IssueCode = "INVALID_PERIOD"

	IssueDuplicateItem     IssueCode = "DUPLICATE_ITEM"
	IssueDuplicateSnapshot IssueCode = "DUPLICATE_SNAPSHOT"
	IssueDuplicateMovement IssueCode = "DUPLICATE_MOVEMENT"
	IssueUnknownItem       IssueCode = "UNKNOWN_ITEM"

	IssueNonFiniteQuantity IssueCode = "NON_FINITE_QUANTITY"
	IssueNonFiniteAmount   IssueCode = "NON_FINITE_AMOUNT"
	IssueInvalidQuantity   IssueCode = "INVALID_QUANTITY"
	IssueInvalidCost       IssueCode = "INVALID_COST"
	IssueInvalidMovement   IssueCode = "INVALID_MOVEMENT"
	IssueInvalidUOM        IssueCode = "INVALID_UOM"
	IssueMixedCurrency     IssueCode = "MIXED_CURRENCY"

	// IssueValueQuantityCostMismatch means an InventorySnapshot (or
	// Movement) supplied both an explicit extended value AND
	// Quantity x UnitCost, and the two disagree beyond amountTolerance —
	// see value.go's resolveSnapshotValue.
	IssueValueQuantityCostMismatch IssueCode = "VALUE_QUANTITY_COST_MISMATCH"

	IssueInvalidStockPolicy     IssueCode = "INVALID_STOCK_POLICY"
	IssueInvalidFinancialMetric IssueCode = "INVALID_FINANCIAL_METRIC"
	IssueInvalidGLControl       IssueCode = "INVALID_GL_CONTROL"
	IssueInvalidAgeDate         IssueCode = "INVALID_AGE_DATE"
	IssueInvalidExpiryDate      IssueCode = "INVALID_EXPIRY_DATE"
	IssueInvalidPolicy          IssueCode = "INVALID_POLICY"
)

// Issue is a single structured validation/analysis finding, returned
// instead of an error so callers can inspect every problem found in one
// pass.
type Issue struct {
	Code     IssueCode     `json:"code"`
	Severity IssueSeverity `json:"severity"`
	Message  string        `json:"message"`

	ItemID     string `json:"item_id,omitempty"`
	SnapshotID string `json:"snapshot_id,omitempty"`
	MovementID string `json:"movement_id,omitempty"`
	BucketCode string `json:"bucket_code,omitempty"`
	Period     string `json:"period,omitempty"`
}

// HasErrors reports whether any Issue in issues has SeverityError.
//
// Intentionally duplicated from ar.HasErrors/ap.HasErrors/labor.HasErrors
// and this repository's other sibling HasErrors functions rather than
// shared — see financial/adjustments.HasErrors's doc comment for the full
// rationale (each package's Issue is a distinct Go type with no common
// interface worth introducing for one boolean function).
func HasErrors(issues []Issue) bool {
	for _, i := range issues {
		if i.Severity == SeverityError {
			return true
		}
	}
	return false
}
