package cashforecast

// IssueSeverity distinguishes a problem that excludes an event or blocks
// some portion of the analysis (SeverityError) from one that is advisory
// only (SeverityWarning) — the same two-severity model every sibling
// package in this repository uses.
type IssueSeverity string

const (
	SeverityError   IssueSeverity = "error"
	SeverityWarning IssueSeverity = "warning"
)

// IssueCode is a stable identifier for one kind of input/configuration/
// integrity problem — never a liquidity/business result, which is a Flag
// instead (see flags.go's doc comment for the Issue-vs-Flag distinction).
// This package defines its own separate taxonomy, consistent with every
// other package in this repository. Only codes this package actually
// emits are defined.
type IssueCode string

const (
	// IssueInvalidForecastStart means Input.ForecastStartDate is the zero
	// time.Time. Calculate cannot proceed at all.
	IssueInvalidForecastStart IssueCode = "INVALID_FORECAST_START"
	// IssueInvalidHorizon means Options.HorizonWeeks is negative or
	// exceeds MaxHorizonWeeks. Calculate cannot proceed at all.
	IssueInvalidHorizon IssueCode = "INVALID_HORIZON"
	// IssueInvalidOpeningCash means Input.OpeningCash.Currency is empty,
	// or CashAccounts was supplied with an empty AccountID or a currency
	// mismatch against ReportingCurrency.
	IssueInvalidOpeningCash IssueCode = "INVALID_OPENING_CASH"
	// IssueOpeningCashMismatch means Input.CashAccounts was supplied and
	// its balances sum to a different total than Input.OpeningCash.Amount
	// (beyond floating-point tolerance). The CashAccounts-derived total is
	// used; OpeningCash.Amount is not silently corrected.
	IssueOpeningCashMismatch IssueCode = "OPENING_CASH_MISMATCH"
	// IssueInvalidEvent means a CashFlowEvent has an empty ID, a zero-value
	// Date, an unrecognized Direction/Category/Basis, or a Category whose
	// fixed CashDirection does not match Direction. The event is excluded.
	IssueInvalidEvent IssueCode = "INVALID_EVENT"
	// IssueDuplicateEvent means the same CashFlowEvent.ID appeared more
	// than once (across Input.Events and every generated event). Only the
	// first occurrence (generation order — see calculate.go) is used.
	IssueDuplicateEvent IssueCode = "DUPLICATE_EVENT"
	// IssueInvalidEventDate is reserved for a future stricter date check;
	// IssueInvalidEvent currently covers every zero-date case, so this
	// code exists but is not emitted by the current validation rules,
	// matching accounting/ap.IssueInvalidDocumentType's identical
	// reserved-code precedent.
	IssueInvalidEventDate IssueCode = "INVALID_EVENT_DATE"
	// IssueNonFiniteAmount means a CashFlowEvent.Amount (or any monetary
	// field in a recurring rule, AR/AP plan, or scenario transform) is NaN
	// or +/-Inf. The event/row is excluded.
	IssueNonFiniteAmount IssueCode = "NON_FINITE_AMOUNT"
	// IssueNegativeAmount means a CashFlowEvent.Amount is negative. The
	// event is excluded — see CashFlowEvent.Amount's doc comment.
	IssueNegativeAmount IssueCode = "NEGATIVE_AMOUNT"
	// IssueInvalidDirection means a CashFlowEvent.Direction is not one of
	// the recognized CashDirection values. The event is excluded.
	IssueInvalidDirection IssueCode = "INVALID_DIRECTION"
	// IssueInvalidCategory means a CashFlowEvent.Category is not one of
	// the recognized CashCategory values. The event is excluded.
	IssueInvalidCategory IssueCode = "INVALID_CATEGORY"
	// IssueCategoryDirectionMismatch means a CashFlowEvent.Category's
	// fixed expected CashDirection does not match its Direction (e.g.
	// CategoryAPPayment with Direction=INFLOW). The event is excluded.
	IssueCategoryDirectionMismatch IssueCode = "CATEGORY_DIRECTION_MISMATCH"
	// IssueMixedCurrency means Input.ReportingCurrency was not supplied
	// and OpeningCash/CashAccounts imply more than one currency, or an
	// event/adapter input carried an explicit currency different from the
	// resolved reporting currency. This package never fetches FX or sums
	// mixed currencies — mismatched-currency rows are excluded.
	IssueMixedCurrency IssueCode = "MIXED_CURRENCY"
	// IssueInvalidRecurringRule means a RecurringRule has a non-positive
	// Amount, a zero-value StartDate, an EndDate before StartDate, or an
	// unrecognized Frequency. The rule generates no events.
	IssueInvalidRecurringRule IssueCode = "INVALID_RECURRING_RULE"
	// IssueARScheduleExceedsOpen means the sum of ARCollectionAssumption/
	// ARReceiptEvent amounts scheduled against one ReceivableID exceeds
	// that receivable's open amount (from the supplied ar.Result or
	// caller-declared open amount) beyond tolerance.
	IssueARScheduleExceedsOpen IssueCode = "AR_SCHEDULE_EXCEEDS_OPEN"
	// IssueAPScheduleExceedsOpen means the sum of APPaymentPlan amounts
	// scheduled against one PayableID exceeds that payable's open amount
	// beyond tolerance.
	IssueAPScheduleExceedsOpen IssueCode = "AP_SCHEDULE_EXCEEDS_OPEN"
	// IssueUnknownReceivable means an ARCollectionAssumption/
	// ARReceiptEvent's ReceivableID does not match any receivable in the
	// supplied ar.Result/ARSource.
	IssueUnknownReceivable IssueCode = "UNKNOWN_RECEIVABLE"
	// IssueUnknownPayable means an APPaymentPlan's PayableID does not
	// match any payable in the supplied ap.Result/APSource.
	IssueUnknownPayable IssueCode = "UNKNOWN_PAYABLE"
	// IssueInvalidScenario means a Scenario has an empty Label, or two
	// Scenarios share the same Label.
	IssueInvalidScenario IssueCode = "INVALID_SCENARIO"
	// IssueInvalidFacility means a CreditFacility has an empty FacilityID,
	// a negative AvailableToDraw, or MinimumDraw > MaximumDraw when both
	// are set. That facility is excluded.
	IssueInvalidFacility IssueCode = "INVALID_FACILITY"
	// IssueMissingRequiredInput means Options.RequiredInputs marked a
	// category (AR/AP/Payroll/DebtService/Tax) required but Input supplied
	// nothing for it (no source data and no directly-supplied events of
	// that category).
	IssueMissingRequiredInput IssueCode = "MISSING_REQUIRED_INPUT"
	// IssueEventBeforeForecastStart means a CashFlowEvent's Date is before
	// Input.ForecastStartDate. The event is excluded from every weekly
	// total; opening cash is authoritative and is never adjusted for it —
	// see the task's section 11.
	IssueEventBeforeForecastStart IssueCode = "EVENT_BEFORE_FORECAST_START"
	// IssueEventBeyondHorizon means a CashFlowEvent's Date is after the
	// last forecast week's EndDate. The event is excluded from weekly
	// totals but summarized in BeyondHorizonSummary.
	IssueEventBeyondHorizon IssueCode = "EVENT_BEYOND_HORIZON"
)

// Issue is a single structured validation/analysis finding, returned
// instead of an error so callers can inspect every problem found in one
// pass.
type Issue struct {
	Code     IssueCode     `json:"code"`
	Severity IssueSeverity `json:"severity"`
	Message  string        `json:"message"`
	// EventID identifies the CashFlowEvent this Issue relates to, if any.
	EventID string `json:"event_id,omitempty"`
	// SourceID identifies a non-event source record (a receivable ID,
	// payable ID, recurring rule ID, scenario label, or facility ID) this
	// Issue relates to, if any.
	SourceID string `json:"source_id,omitempty"`
}

// HasErrors reports whether any Issue in issues has SeverityError.
//
// Intentionally duplicated from ar.HasErrors/ap.HasErrors/debt.HasErrors
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
