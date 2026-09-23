package ar

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
// emits are defined, per the task's "use only the codes actually required"
// instruction.
type IssueCode string

const (
	// IssueDuplicateReceivable means the same Receivable.ID appeared more
	// than once in Input.Receivables. Only the first occurrence (input
	// order) is used.
	IssueDuplicateReceivable IssueCode = "DUPLICATE_RECEIVABLE"
	// IssueInvalidStatus means a Receivable.Status value is not one of the
	// recognized ReceivableStatus constants.
	IssueInvalidStatus IssueCode = "INVALID_STATUS"
	// IssueInvalidAmount means a Receivable's OriginalAmount/OpenAmount
	// sign is inconsistent with its DocumentType (e.g. a positive
	// OriginalAmount on a credit memo, or a negative OriginalAmount on an
	// ordinary invoice without credit-memo semantics).
	IssueInvalidAmount IssueCode = "INVALID_AMOUNT"
	// IssueNonFiniteAmount means a monetary field (OriginalAmount,
	// OpenAmount, or a Payment.Amount) is NaN or +/-Inf. The row is
	// excluded from every computation.
	IssueNonFiniteAmount IssueCode = "NON_FINITE_AMOUNT"
	// IssueOpenExceedsOriginal means abs(OpenAmount) > abs(OriginalAmount)
	// for an ordinary (non-credit-memo) receivable.
	IssueOpenExceedsOriginal IssueCode = "OPEN_EXCEEDS_ORIGINAL"
	// IssuePaidWithOpenBalance means Status == StatusPaid but OpenAmount is
	// nonzero.
	IssuePaidWithOpenBalance IssueCode = "PAID_WITH_OPEN_BALANCE"
	// IssueInvalidDate means InvoiceDate or DueDate is the zero time.Time,
	// or DueDate is before InvoiceDate.
	IssueInvalidDate IssueCode = "INVALID_DATE"
	// IssueFutureInvoice means InvoiceDate is after AsOfDate — a receivable
	// that, per the analysis snapshot date, should not exist yet.
	IssueFutureInvoice IssueCode = "FUTURE_INVOICE"
	// IssueInvalidBucketConfiguration means the supplied (or default)
	// BucketDefinition slice fails structural validation — see
	// validateBucketDefinitions.
	IssueInvalidBucketConfiguration IssueCode = "INVALID_BUCKET_CONFIGURATION"
	// IssueMixedCurrency means the included receivables use more than one
	// currency and Options.ReportingCurrency was not supplied to resolve
	// it.
	IssueMixedCurrency IssueCode = "MIXED_CURRENCY"
	// IssueMissingCustomer means Receivable.CustomerID is empty.
	IssueMissingCustomer IssueCode = "MISSING_CUSTOMER"
	// IssueControlAccountMismatch means Options.ControlAccountBalance was
	// supplied and differs from PortfolioSummary.TotalOpenReceivables by
	// more than the resolved tolerance — see ControlAccountReconciliation.
	IssueControlAccountMismatch IssueCode = "CONTROL_ACCOUNT_MISMATCH"
	// IssueUnbalancedAging means the sum of bucket amounts does not equal
	// PortfolioSummary.TotalOpenReceivables within floating-point
	// tolerance — see AgingReconciliation. This should never occur for
	// valid input; it exists as an explicit, always-checked invariant
	// rather than a silent assumption.
	IssueUnbalancedAging IssueCode = "UNBALANCED_AGING"
	// IssueMissingSalesForDSO means DSO could not be computed because no
	// SalesPeriod input was supplied.
	IssueMissingSalesForDSO IssueCode = "MISSING_SALES_FOR_DSO"
	// IssueInvalidPayment means a Payment has a non-finite or non-positive
	// Amount, or a zero-value Date.
	IssueInvalidPayment IssueCode = "INVALID_PAYMENT"
	// IssueUnknownReceivablePayment means a Payment.ReceivableID does not
	// match any Receivable.ID in Input.Receivables.
	IssueUnknownReceivablePayment IssueCode = "UNKNOWN_RECEIVABLE_PAYMENT"
)

// Issue is a single structured validation/analysis finding, returned
// instead of an error so callers can inspect every problem found in one
// pass.
type Issue struct {
	Code     IssueCode     `json:"code"`
	Severity IssueSeverity `json:"severity"`
	Message  string        `json:"message"`
	// ReceivableID identifies the Receivable this Issue relates to, if any.
	ReceivableID string `json:"receivable_id,omitempty"`
	// CustomerID identifies the customer this Issue relates to, if any.
	CustomerID string `json:"customer_id,omitempty"`
	// PaymentID identifies the Payment this Issue relates to, if any.
	PaymentID string `json:"payment_id,omitempty"`
	// BucketCode identifies the BucketDefinition this Issue relates to, if
	// any.
	BucketCode string `json:"bucket_code,omitempty"`
}

// HasErrors reports whether any Issue in issues has SeverityError.
//
// Intentionally duplicated from ledger.HasErrors/statements.HasErrors and
// this repository's other sibling HasErrors functions rather than shared
// — see financial/adjustments.HasErrors's doc comment for the full
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
