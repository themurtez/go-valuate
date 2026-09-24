package vendorspend

// IssueSeverity distinguishes a problem that excludes a row or blocks some
// portion of the analysis (SeverityError) from one that is advisory only
// (SeverityWarning) — the same two-severity model every sibling package in
// this repository uses.
type IssueSeverity string

const (
	SeverityError   IssueSeverity = "error"
	SeverityWarning IssueSeverity = "warning"
)

// IssueCode is a stable identifier for one kind of input/validation
// problem. This package defines its own separate taxonomy, consistent
// with every other package in this repository. Only codes this package
// actually emits are defined — task section 36 lists this as a
// "potential" set; codes never emitted are not defined at all.
type IssueCode string

const (
	// IssueInvalidPeriod means a Period's StartDate/EndDate/Period label
	// failed Period.Valid().
	IssueInvalidPeriod IssueCode = "INVALID_PERIOD"
	// IssueDuplicatePeriod means the same Period.Period label appeared
	// more than once in Input.Periods. Only the first occurrence is used.
	IssueDuplicatePeriod IssueCode = "DUPLICATE_PERIOD"
	// IssueDuplicateSupplier means the same Supplier.SupplierID appeared
	// more than once in Input.Suppliers. Only the first occurrence is
	// used.
	IssueDuplicateSupplier IssueCode = "DUPLICATE_SUPPLIER"
	// IssueUnknownSupplier means a SpendRecord.SupplierID does not match
	// any Supplier.SupplierID in Input.Suppliers. The record is excluded.
	IssueUnknownSupplier IssueCode = "UNKNOWN_SUPPLIER"
	// IssueInvalidSupplierParent means a Supplier.ParentID does not
	// reference a known SupplierID, or participates in a parent-reference
	// cycle.
	IssueInvalidSupplierParent IssueCode = "INVALID_SUPPLIER_PARENT"
	// IssueDuplicateSpend means the same SpendRecord.SpendID appeared more
	// than once in Input.SpendRecords. Only the first occurrence is used
	// — this is a distinct, structural problem from the optional
	// possible-economic-duplicate detection (see FlagPossibleDuplicateSpend).
	IssueDuplicateSpend IssueCode = "DUPLICATE_SPEND"
	// IssueInvalidSpendDate means SpendRecord.Date is the zero time.Time,
	// or falls outside every supplied Period's [StartDate, EndDate] range
	// when SpendRecord.Period does not resolve the ambiguity.
	IssueInvalidSpendDate IssueCode = "INVALID_SPEND_DATE"
	// IssueUnknownPeriod means SpendRecord.Period does not match any
	// Period.Period label in Input.Periods. The record is excluded from
	// every period-scoped computation.
	IssueUnknownPeriod IssueCode = "UNKNOWN_PERIOD"
	// IssueNonFiniteAmount means SpendRecord.Amount, Quantity.Value, or
	// UnitPrice.Value is NaN or +/-Inf. The record is excluded.
	IssueNonFiniteAmount IssueCode = "NON_FINITE_AMOUNT"
	// IssueNegativeAmount means SpendRecord.Amount is negative — expected
	// non-negative regardless of Effect (see SpendEffect's doc comment).
	// Advisory only; the record is not excluded.
	IssueNegativeAmount IssueCode = "NEGATIVE_AMOUNT"
	// IssueInvalidEffect means SpendRecord.Effect is a non-empty,
	// unrecognized value. The record is excluded, since Effect determines
	// gross/net-spend semantics and a silently-defaulted Effect could
	// misstate whether the record adds to or reduces spend.
	IssueInvalidEffect IssueCode = "INVALID_EFFECT"
	// IssueInvalidSpendType means SpendRecord.SpendType is a non-empty,
	// unrecognized value. Advisory only; resolvedSpendType falls back to
	// SpendTypeOther.
	IssueInvalidSpendType IssueCode = "INVALID_SPEND_TYPE"
	// IssueInvalidSpendBasis means SpendRecord.Basis is a non-empty,
	// unrecognized value. Advisory only; resolvedBasis falls back to
	// BasisUnknown.
	IssueInvalidSpendBasis IssueCode = "INVALID_SPEND_BASIS"
	// IssueMixedSpendBasis means the included records use more than one
	// SpendBasis and Options.RequireSingleBasis is set — see
	// Result.SpendByBasis for the always-computed per-basis breakdown
	// that lets a caller allow mixed bases intentionally (task section 6).
	IssueMixedSpendBasis IssueCode = "MIXED_SPEND_BASIS"
	// IssueMixedCurrency means the included records use more than one
	// currency and Options.ReportingCurrency was not supplied to resolve
	// it.
	IssueMixedCurrency IssueCode = "MIXED_CURRENCY"
	// IssueInvalidQuantity means SpendRecord.Quantity.Available is true
	// but Quantity.Value is negative or non-finite.
	IssueInvalidQuantity IssueCode = "INVALID_QUANTITY"
	// IssueInvalidUnitPrice means SpendRecord.UnitPrice.Available is true
	// but UnitPrice.Value is negative or non-finite.
	IssueInvalidUnitPrice IssueCode = "INVALID_UNIT_PRICE"
	// IssueInvalidUOM means SpendRecord.Quantity or UnitPrice is
	// Available but UnitOfMeasure is empty, making the quantity/price
	// figure unusable for any cross-record comparison (task section 14).
	IssueInvalidUOM IssueCode = "INVALID_UOM"
	// IssueInvalidControlTotal means a ControlTotals figure is
	// non-finite.
	IssueInvalidControlTotal IssueCode = "INVALID_CONTROL_TOTAL"
	// IssueInvalidPolicy means a supplied Policy field is structurally
	// invalid (e.g. a negative Materiality.AbsoluteAmount).
	IssueInvalidPolicy IssueCode = "INVALID_POLICY"
)

// Issue is a single structured validation/analysis finding, returned
// instead of an error so callers can inspect every problem found in one
// pass.
type Issue struct {
	Code     IssueCode     `json:"code"`
	Severity IssueSeverity `json:"severity"`
	Message  string        `json:"message"`
	// SpendID identifies the SpendRecord this Issue relates to, if any.
	SpendID string `json:"spend_id,omitempty"`
	// SupplierID identifies the Supplier this Issue relates to, if any.
	SupplierID string `json:"supplier_id,omitempty"`
	// Period identifies the period label this Issue relates to, if any.
	Period string `json:"period,omitempty"`
}

// HasErrors reports whether any Issue in issues has SeverityError.
//
// Intentionally duplicated from ap.HasErrors/inventory.HasErrors/
// profitability.HasErrors and this repository's other sibling HasErrors
// functions rather than shared — see financial/adjustments.HasErrors's
// doc comment for the full rationale (each package's Issue is a distinct
// Go type with no common interface worth introducing for one boolean
// function).
func HasErrors(issues []Issue) bool {
	for _, i := range issues {
		if i.Severity == SeverityError {
			return true
		}
	}
	return false
}

// issueCodeOrder fixes IssueCode declaration order for deterministic
// Issues sorting — task section 41's "issues code/source ID" rule.
var issueCodeOrder = []IssueCode{
	IssueInvalidPeriod,
	IssueDuplicatePeriod,
	IssueDuplicateSupplier,
	IssueUnknownSupplier,
	IssueInvalidSupplierParent,
	IssueDuplicateSpend,
	IssueInvalidSpendDate,
	IssueUnknownPeriod,
	IssueNonFiniteAmount,
	IssueNegativeAmount,
	IssueInvalidEffect,
	IssueInvalidSpendType,
	IssueInvalidSpendBasis,
	IssueMixedSpendBasis,
	IssueMixedCurrency,
	IssueInvalidQuantity,
	IssueInvalidUnitPrice,
	IssueInvalidUOM,
	IssueInvalidControlTotal,
	IssueInvalidPolicy,
}

func issueRank(c IssueCode) int {
	for i, ic := range issueCodeOrder {
		if ic == c {
			return i
		}
	}
	return len(issueCodeOrder)
}

// sourceIDOf returns the best single "source ID" for an Issue for
// deterministic sorting: SpendID first, then SupplierID, then Period.
func sourceIDOf(i Issue) string {
	if i.SpendID != "" {
		return i.SpendID
	}
	if i.SupplierID != "" {
		return i.SupplierID
	}
	return i.Period
}
