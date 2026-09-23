package ap

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
// problem. This package defines its own separate taxonomy, consistent with
// every other package in this repository. Only codes this package actually
// emits are defined.
type IssueCode string

const (
	// IssueDuplicatePayable means the same Payable.ID appeared more than
	// once in Input.Payables. Only the first occurrence (input order) is
	// used.
	IssueDuplicatePayable IssueCode = "DUPLICATE_PAYABLE"
	// IssueInvalidStatus means a Payable.Status value is not one of the
	// recognized PayableStatus constants, or Payable.ID is empty.
	IssueInvalidStatus IssueCode = "INVALID_STATUS"
	// IssueInvalidDocumentType is reserved for a future stricter
	// DocumentType check; currently unrecognized values resolve silently
	// to DocumentTypeBill (see resolvedDocumentType), matching
	// accounting/ar's identical DocumentType-defaulting convention, so
	// this code is defined but not emitted by the current validation
	// rules. It exists so IssueInvalidAmount is not overloaded to also
	// mean "document type looked wrong."
	IssueInvalidDocumentType IssueCode = "INVALID_DOCUMENT_TYPE"
	// IssueInvalidAmount means a Payable's OriginalAmount/OpenAmount sign
	// is inconsistent with its DocumentType (e.g. a positive
	// OriginalAmount on a vendor credit, or a negative OriginalAmount on
	// an ordinary bill without vendor-credit semantics).
	IssueInvalidAmount IssueCode = "INVALID_AMOUNT"
	// IssueNonFiniteAmount means a monetary field (OriginalAmount,
	// OpenAmount, or a SupplierPayment.Amount) is NaN or +/-Inf. The row
	// is excluded from every computation.
	IssueNonFiniteAmount IssueCode = "NON_FINITE_AMOUNT"
	// IssueOpenExceedsOriginal means abs(OpenAmount) > abs(OriginalAmount)
	// for an ordinary (non-vendor-credit) payable.
	IssueOpenExceedsOriginal IssueCode = "OPEN_EXCEEDS_ORIGINAL"
	// IssuePaidWithOpenBalance means Status == StatusPaid but OpenAmount
	// is nonzero.
	IssuePaidWithOpenBalance IssueCode = "PAID_WITH_OPEN_BALANCE"
	// IssueInvalidDate means BillDate or DueDate is the zero time.Time, or
	// DueDate is before BillDate.
	IssueInvalidDate IssueCode = "INVALID_DATE"
	// IssueFutureBill means BillDate is after AsOfDate — a payable that,
	// per the analysis snapshot date, should not exist yet.
	IssueFutureBill IssueCode = "FUTURE_BILL"
	// IssueInvalidBucketConfiguration means the supplied (or default)
	// BucketDefinition slice fails structural validation — see
	// validateBucketDefinitions.
	IssueInvalidBucketConfiguration IssueCode = "INVALID_BUCKET_CONFIGURATION"
	// IssueMixedCurrency means the included payables use more than one
	// currency and Options.ReportingCurrency was not supplied to resolve
	// it.
	IssueMixedCurrency IssueCode = "MIXED_CURRENCY"
	// IssueMissingSupplier means Payable.SupplierID is empty.
	IssueMissingSupplier IssueCode = "MISSING_SUPPLIER"
	// IssueControlAccountMismatch means Options.ControlAccountBalance was
	// supplied and differs from PortfolioSummary.TotalOpenPayables by more
	// than the resolved tolerance — see ControlAccountReconciliation.
	IssueControlAccountMismatch IssueCode = "CONTROL_ACCOUNT_MISMATCH"
	// IssueUnbalancedAging means the sum of bucket amounts does not equal
	// PortfolioSummary.TotalOpenPayables within floating-point tolerance
	// — see AgingReconciliation. This should never occur for valid input;
	// it exists as an explicit, always-checked invariant rather than a
	// silent assumption.
	IssueUnbalancedAging IssueCode = "UNBALANCED_AGING"
	// IssueMissingDenominatorForDPO means DPO could not be computed
	// because no valid Options.DPODenominator was supplied.
	IssueMissingDenominatorForDPO IssueCode = "MISSING_DENOMINATOR_FOR_DPO"
	// IssueInvalidPayment means a SupplierPayment has a non-finite or
	// non-positive Amount, or a zero-value Date.
	IssueInvalidPayment IssueCode = "INVALID_PAYMENT"
	// IssueUnknownPayablePayment means a SupplierPayment.PayableID does
	// not match any Payable.ID in Input.Payables.
	IssueUnknownPayablePayment IssueCode = "UNKNOWN_PAYABLE_PAYMENT"
)

// Issue is a single structured validation/analysis finding, returned
// instead of an error so callers can inspect every problem found in one
// pass.
type Issue struct {
	Code     IssueCode     `json:"code"`
	Severity IssueSeverity `json:"severity"`
	Message  string        `json:"message"`
	// PayableID identifies the Payable this Issue relates to, if any.
	PayableID string `json:"payable_id,omitempty"`
	// SupplierID identifies the supplier this Issue relates to, if any.
	SupplierID string `json:"supplier_id,omitempty"`
	// PaymentID identifies the SupplierPayment this Issue relates to, if
	// any.
	PaymentID string `json:"payment_id,omitempty"`
	// BucketCode identifies the BucketDefinition this Issue relates to, if
	// any.
	BucketCode string `json:"bucket_code,omitempty"`
}

// HasErrors reports whether any Issue in issues has SeverityError.
//
// Intentionally duplicated from ar.HasErrors/ledger.HasErrors/
// statements.HasErrors and this repository's other sibling HasErrors
// functions rather than shared — see financial/adjustments.HasErrors's doc
// comment for the full rationale (each package's Issue is a distinct Go
// type with no common interface worth introducing for one boolean
// function).
func HasErrors(issues []Issue) bool {
	for _, i := range issues {
		if i.Severity == SeverityError {
			return true
		}
	}
	return false
}
