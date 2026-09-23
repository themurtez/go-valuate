package labor

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
// problem — never a labor-analytics/business-result signal, which is a
// Flag instead (see flags.go). This package defines its own separate
// taxonomy, consistent with every other package in this repository. Only
// codes this package actually emits are defined.
type IssueCode string

const (
	// IssueInvalidPeriod means a PeriodInfo has an empty Period label, a
	// zero-value StartDate/EndDate, or EndDate before StartDate.
	IssueInvalidPeriod IssueCode = "INVALID_PERIOD"
	// IssueDuplicatePeriod means the same PeriodInfo.Period appeared more
	// than once in Input.Periods. Only the first occurrence (input order)
	// is used.
	IssueDuplicatePeriod IssueCode = "DUPLICATE_PERIOD"
	// IssueDuplicateWorker means the same Worker.WorkerID appeared more
	// than once in Input.Workers. Only the first occurrence is used.
	IssueDuplicateWorker IssueCode = "DUPLICATE_WORKER"
	// IssueDuplicatePayrollRecord means the same PayrollRecord.ID appeared
	// more than once. Only the first occurrence is used.
	IssueDuplicatePayrollRecord IssueCode = "DUPLICATE_PAYROLL_RECORD"
	// IssueDuplicateContractorRecord means the same
	// ContractorLaborRecord.ID appeared more than once. Only the first
	// occurrence is used.
	IssueDuplicateContractorRecord IssueCode = "DUPLICATE_CONTRACTOR_RECORD"
	// IssueUnknownWorker means a PayrollRecord.WorkerID (or
	// ContractorLaborRecord.ContractorID checked against the worker
	// roster) does not match any Worker.WorkerID, and a roster was
	// actually supplied (Input.Workers non-empty) — this check is skipped
	// entirely when no roster is supplied at all, per the task's "worker
	// roster is optional" design.
	IssueUnknownWorker IssueCode = "UNKNOWN_WORKER"
	// IssueInvalidDate means a required date field (PayrollRecord.PayDate,
	// ContractorLaborRecord.Date) is the zero time.Time.
	IssueInvalidDate IssueCode = "INVALID_DATE"
	// IssueInvalidWorkerDates means Worker.TerminationDate is before
	// Worker.HireDate (when both are supplied).
	IssueInvalidWorkerDates IssueCode = "INVALID_WORKER_DATES"
	// IssueNonFiniteAmount means a monetary or hours field is NaN or
	// +/-Inf. The row is excluded from every computation.
	IssueNonFiniteAmount IssueCode = "NON_FINITE_AMOUNT"
	// IssueNegativeAmount means a labor-cost component that this package
	// requires to be a non-negative magnitude (see the task's section 46)
	// is negative on a RecordTypeNormal record. A REVERSAL/ADJUSTMENT
	// record is exempt (its explicit RecordType already signals the
	// component is a correction) — see resolvedRecordType.
	IssueNegativeAmount IssueCode = "NEGATIVE_AMOUNT"
	// IssueInvalidHours means HoursRegular/HoursOvertime/Hours is
	// Available but negative or non-finite.
	IssueInvalidHours IssueCode = "INVALID_HOURS"
	// IssueMixedCurrency means the included records use more than one
	// currency and no single reporting currency could be resolved.
	IssueMixedCurrency IssueCode = "MIXED_CURRENCY"
	// IssueInvalidFTE means a caller-supplied FTE value (see fte.go) is
	// negative or non-finite.
	IssueInvalidFTE IssueCode = "INVALID_FTE"
	// IssueInvalidFinancialMetric means a BusinessMetrics.Revenue/
	// GrossProfit/EBITDA Value is Available but non-finite.
	IssueInvalidFinancialMetric IssueCode = "INVALID_FINANCIAL_METRIC"
	// IssueInvalidGLControl means a GLPayrollControl Value is Available
	// but non-finite.
	IssueInvalidGLControl IssueCode = "INVALID_GL_CONTROL"
	// IssueInvalidPolicy means a Policy threshold is non-finite or
	// otherwise structurally invalid — see validatePolicy.
	IssueInvalidPolicy IssueCode = "INVALID_POLICY"
	// IssueMissingWorkerID means a PayrollRecord/ContractorLaborRecord has
	// an empty WorkerID/ContractorID.
	IssueMissingWorkerID IssueCode = "MISSING_WORKER_ID"
)

// Issue is a single structured validation/analysis finding, returned
// instead of an error so callers can inspect every problem found in one
// pass.
type Issue struct {
	Code     IssueCode     `json:"code"`
	Severity IssueSeverity `json:"severity"`
	Message  string        `json:"message"`

	// RecordID identifies the PayrollRecord/ContractorLaborRecord this
	// Issue relates to, if any.
	RecordID string `json:"record_id,omitempty"`
	// WorkerID identifies the Worker/ContractorID this Issue relates to,
	// if any.
	WorkerID string `json:"worker_id,omitempty"`
	// Period identifies the period this Issue relates to, if any.
	Period string `json:"period,omitempty"`
}

// HasErrors reports whether any Issue in issues has SeverityError.
//
// Intentionally duplicated from ar.HasErrors/ap.HasErrors/
// cashforecast.HasErrors and this repository's other sibling HasErrors
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
// Issues sorting.
var issueCodeOrder = []IssueCode{
	IssueInvalidPeriod,
	IssueDuplicatePeriod,
	IssueDuplicateWorker,
	IssueDuplicatePayrollRecord,
	IssueDuplicateContractorRecord,
	IssueUnknownWorker,
	IssueInvalidDate,
	IssueInvalidWorkerDates,
	IssueNonFiniteAmount,
	IssueNegativeAmount,
	IssueInvalidHours,
	IssueMixedCurrency,
	IssueInvalidFTE,
	IssueInvalidFinancialMetric,
	IssueInvalidGLControl,
	IssueInvalidPolicy,
	IssueMissingWorkerID,
}

func issueRank(c IssueCode) int {
	for i, ic := range issueCodeOrder {
		if ic == c {
			return i
		}
	}
	return len(issueCodeOrder)
}
