package statements

// IssueSeverity distinguishes a problem that blocks some portion of the
// build (SeverityError) from one that is advisory only (SeverityWarning)
// — the same two-severity model every sibling package in this repository
// uses (ledger.IssueSeverity, consolidation.IssueSeverity, ...).
type IssueSeverity string

const (
	SeverityError   IssueSeverity = "error"
	SeverityWarning IssueSeverity = "warning"
)

// IssueCode is a stable identifier for one kind of mapping/statement-
// building problem. This package defines its own separate taxonomy,
// consistent with every other package in this repository, rather than
// reusing ledger.IssueCode or classification's error handling — a mapping/
// statement-building problem is a distinct problem domain from a ledger
// posting problem or a row-classification problem. Only codes this
// package actually emits are defined, per the task's "use only the codes
// actually required" instruction.
type IssueCode string

const (
	// IssueUnmappedAccount means an account had no explicit mapping and
	// (if MappingSuggestDeterministic was used) no deterministic
	// suggestion either, or the suggestion was UNKNOWN. Severity depends
	// on UnmappedPolicy and materiality — see issueForUnmapped in
	// mapping.go.
	IssueUnmappedAccount IssueCode = "UNMAPPED_ACCOUNT"
	// IssueInvalidMapping means an AccountMapping is structurally invalid:
	// references an unknown AccountID, has an empty FinancialCode/
	// StatementType with no Allocations, or names an unrecognized
	// financial.Code — see validateMapping in validate.go.
	IssueInvalidMapping IssueCode = "INVALID_MAPPING"
	// IssueMappingConflict means the same AccountID appears in more than
	// one caller-supplied AccountMapping (Input.Mappings), or a
	// MappingTemplate ambiguously matches an account via more than one
	// same-precedence rule — see validateMapping and resolveTemplate.
	IssueMappingConflict IssueCode = "MAPPING_CONFLICT"
	// IssueIncompatibleAccountType means a mapping's FinancialCode
	// belongs to a financial.CodeCategory that the mapped account's
	// ledger.AccountType can never legitimately produce (e.g. a REVENUE
	// account mapped to a balance-sheet asset code) — see
	// accounttype.go's compatibility table.
	IssueIncompatibleAccountType IssueCode = "INCOMPATIBLE_ACCOUNT_TYPE"
	// IssueInvalidFinancialCode means a mapping's FinancialCode is not a
	// recognized entry in the financial taxonomy (financial.IsValidCode
	// returns false).
	IssueInvalidFinancialCode IssueCode = "INVALID_FINANCIAL_CODE"
	// IssueMixedCurrency means the accounts contributing to this build use
	// more than one currency and Input.ReportingCurrency was not supplied
	// to resolve it — see currency handling in builder.go. This package
	// never fetches FX; see the package doc comment.
	IssueMixedCurrency IssueCode = "MIXED_CURRENCY"
	// IssueUnbalancedBalanceSheet means the built balance sheet's Assets
	// do not equal Liabilities + Equity within the reconciliation
	// tolerance — see reconcile.go. Never forces balance; always reports
	// what was actually computed.
	IssueUnbalancedBalanceSheet IssueCode = "UNBALANCED_BALANCE_SHEET"
	// IssueIncompleteStatement means a requested statement could not be
	// built at all — e.g. no mapped accounts existed for that
	// StatementType/period.
	IssueIncompleteStatement IssueCode = "INCOMPLETE_STATEMENT"
	// IssueInvalidSignTreatment means an AccountMapping or AllocationRule
	// specified a SignTreatment value that is not one of NATURAL, NORMAL,
	// or INVERT.
	IssueInvalidSignTreatment IssueCode = "INVALID_SIGN_TREATMENT"
	// IssueInvalidAllocation means an AccountMapping.Allocations slice is
	// malformed: empty Percent values that don't sum to 1.0 within
	// allocationTolerance, a negative/non-finite Percent, or an
	// allocation naming an unrecognized financial.Code — see
	// validateAllocations.
	IssueInvalidAllocation IssueCode = "INVALID_ALLOCATION"
	// IssueMissingPeriod means Input.Periods was empty, or a
	// caller-supplied period matched no data in the source ledger/trial
	// balance.
	IssueMissingPeriod IssueCode = "MISSING_PERIOD"
	// IssueSourceTrialBalanceUnbalanced means the source
	// ledger.TrialBalance/NormalizedTrialBalance this build was derived
	// from was itself out of balance (echoing the upstream
	// ledger.IssueUnbalancedTrialBalance as a statements-level issue so a
	// caller inspecting only Result.Issues still sees it, without this
	// package silently building on top of known-bad source data).
	IssueSourceTrialBalanceUnbalanced IssueCode = "SOURCE_TRIAL_BALANCE_UNBALANCED"
)

// Issue is a single structured mapping/statement-building finding,
// returned instead of an error so callers can inspect every problem found
// in one pass.
type Issue struct {
	Code     IssueCode     `json:"code"`
	Severity IssueSeverity `json:"severity"`
	Message  string        `json:"message"`
	// AccountID identifies the ledger.Account.ID this Issue relates to, if
	// any.
	AccountID string `json:"account_id,omitempty"`
	// FinancialCode identifies the canonical code this Issue relates to,
	// if any.
	FinancialCode string `json:"financial_code,omitempty"`
	// Period identifies the reporting period this Issue relates to, if
	// any.
	Period string `json:"period,omitempty"`
}

// HasErrors reports whether any Issue in issues has SeverityError.
//
// Intentionally duplicated from ledger.HasErrors and this repository's
// other sibling HasErrors functions rather than shared — see
// financial/adjustments.HasErrors's doc comment for the full rationale
// (each package's Issue is a distinct Go type with no common interface
// worth introducing for one boolean function).
func HasErrors(issues []Issue) bool {
	for _, i := range issues {
		if i.Severity == SeverityError {
			return true
		}
	}
	return false
}
