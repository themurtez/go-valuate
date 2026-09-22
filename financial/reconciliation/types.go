// Package reconciliation evaluates the internal consistency of a normalized
// financial.FinancialDataset: whether reported subtotals match their
// reconstructed components, whether the balance sheet balances, and whether
// the dataset is structurally well-formed.
//
// This package never mutates the dataset it is given and performs no I/O.
// It returns a flat, deterministic list of Check results rather than a
// single pass/fail boolean, since "is this dataset internally consistent"
// is rarely a single yes/no answer — some checks genuinely don't apply to
// every dataset (see StatusNotApplicable), and callers need to see
// differences and tolerances, not just a verdict.
//
// financial.FinancialDataset has no representation for a reported subtotal
// (gross profit, operating income, EBITDA, net income): financial.Normalize
// deliberately excludes subtotal/total rows to avoid double counting (see
// financial.RowStatusSubtotal/RowStatusTotal). Checks that compare a
// reconstructed figure against a "reported" one therefore take reported
// values as a separate, optional, caller-supplied ReportedTotals argument
// rather than reading them from the dataset — see Run and ReportedTotals.
package reconciliation

import "github.com/themurtez/go-valuate/financial"

// Status is the outcome of a single reconciliation Check. Deliberately not a
// bool: a check can genuinely not apply to a given dataset (StatusNotApplicable),
// and a numeric mismatch can be within tolerance (StatusPass) or outside it
// at different severities (StatusWarning vs. StatusFail).
type Status string

const (
	// StatusPass means the check ran and its expected/actual values matched
	// within tolerance (or the integrity condition held).
	StatusPass Status = "PASS"
	// StatusWarning means the check found something worth a human's
	// attention that is not necessarily wrong — e.g. a difference outside
	// tolerance but small relative to the statement, or a structural
	// condition (like an income-statement-only dataset) that is unusual but
	// not invalid.
	StatusWarning Status = "WARNING"
	// StatusFail means the check ran and found a difference or condition
	// that indicates a real problem — e.g. a balance sheet that does not
	// balance by a material amount, or a malformed value.
	StatusFail Status = "FAIL"
	// StatusNotApplicable means the check could not run at all because a
	// required input was absent — e.g. no reported gross profit was
	// supplied, or the dataset has no balance sheet data. This is
	// deliberately distinct from StatusPass: the check makes no claim about
	// correctness when it has nothing to check.
	StatusNotApplicable Status = "NOT_APPLICABLE"
)

// CheckCode is a stable identifier for one kind of reconciliation check,
// analogous to financial.Code: durable, meant to be matched on by callers
// (e.g. to filter or group), independent of the human-readable Explanation
// text, which may be reworded freely.
type CheckCode string

// Income statement check codes.
const (
	CheckGrossProfit     CheckCode = "GROSS_PROFIT_RECONCILES"
	CheckOperatingIncome CheckCode = "OPERATING_INCOME_RECONCILES"
	CheckEBITDABridge    CheckCode = "EBITDA_BRIDGE_RECONCILES"
	CheckNetIncome       CheckCode = "NET_INCOME_RECONCILES"
)

// Balance sheet check codes.
const (
	CheckBalanceSheetBalances       CheckCode = "BALANCE_SHEET_BALANCES"
	CheckCurrentAssetsSubtotal      CheckCode = "CURRENT_ASSETS_SUBTOTAL"
	CheckCurrentLiabilitiesSubtotal CheckCode = "CURRENT_LIABILITIES_SUBTOTAL"
	CheckWorkingCapitalCalculated   CheckCode = "WORKING_CAPITAL_CALCULATED"
	CheckDebtTotalsCalculated       CheckCode = "DEBT_TOTALS_CALCULATED"
)

// Dataset-level integrity check codes.
const (
	CheckNoDuplicateEntries                 CheckCode = "NO_DUPLICATE_ENTRIES"
	CheckNoUnknownPeriods                   CheckCode = "NO_UNKNOWN_PERIOD_REFERENCES"
	CheckFiniteValues                       CheckCode = "FINITE_NUMERIC_VALUES"
	CheckValidCurrency                      CheckCode = "VALID_CURRENCY"
	CheckDatasetNotEmpty                    CheckCode = "DATASET_NOT_EMPTY"
	CheckIncomeStatementHasRevenue          CheckCode = "INCOME_STATEMENT_HAS_REVENUE"
	CheckBalanceSheetHasLiabilitiesOrEquity CheckCode = "BALANCE_SHEET_HAS_LIABILITIES_OR_EQUITY"
	CheckNoSuspiciousDuplicateSources       CheckCode = "NO_SUSPICIOUS_DUPLICATE_SOURCES"
)

// Check is the result of a single reconciliation check for a single period
// (or for the dataset as a whole, for checks that are not period-scoped —
// see the Period field's doc comment below).
type Check struct {
	// Code is the stable identifier for this kind of check. See the
	// Check* constants.
	Code CheckCode `json:"code"`
	// Status is the outcome. See the Status constants; never represented as
	// a bare bool.
	Status Status `json:"status"`
	// Period is the reporting period this check applies to. Empty for
	// dataset-wide checks that are not scoped to a single period (e.g.
	// CheckDatasetNotEmpty, CheckValidCurrency).
	Period financial.Period `json:"period,omitempty"`
	// Expected is the reported/reference value being checked against, when
	// applicable (e.g. a caller-supplied reported gross profit, or the
	// liabilities+equity side of the balance sheet equation). Nil when the
	// check has no single "expected" value (e.g. most integrity checks).
	Expected *float64 `json:"expected,omitempty"`
	// Actual is the reconstructed/observed value being checked, when
	// applicable (e.g. Revenue - COGS, or the assets side of the balance
	// sheet equation). Nil alongside Expected for checks with no numeric
	// comparison.
	Actual *float64 `json:"actual,omitempty"`
	// Difference is Actual - Expected, when both are present. Provided
	// pre-computed so callers never need to reimplement the subtraction (or
	// risk getting the sign backwards) themselves.
	Difference *float64 `json:"difference,omitempty"`
	// Tolerance is the tolerance this check was evaluated against, when
	// applicable. Zero-value Tolerance for checks with no numeric
	// comparison.
	Tolerance Tolerance `json:"tolerance,omitempty"`
	// Explanation is a human-readable description of what this check found,
	// suitable for display to a reviewer.
	Explanation string `json:"explanation"`
	// RelatedCodes lists the canonical financial.Code and/or metrics values
	// this check's calculation drew from, for traceability (e.g.
	// ["REV_PRODUCT", "REV_SERVICE", "COGS_MATERIAL"] for a gross profit
	// check).
	RelatedCodes []string `json:"related_codes,omitempty"`
}

// Result is the output of Run: every Check evaluated, plus the dataset-wide
// summary counts by Status.
type Result struct {
	// Checks is every Check evaluated, in a fixed deterministic order:
	// dataset-level integrity checks first, then per-period checks ordered
	// by period (financial.FinancialDataset.Periods order), then by check
	// code within a period.
	Checks []Check `json:"checks"`
}

// CountByStatus returns how many Checks in the result have the given
// Status, a convenience for callers that want a quick summary (e.g. "are
// there any FAILs") without iterating Checks themselves.
func (r Result) CountByStatus(status Status) int {
	n := 0
	for _, c := range r.Checks {
		if c.Status == status {
			n++
		}
	}
	return n
}

// HasFailures reports whether any Check in the result has StatusFail.
func (r Result) HasFailures() bool {
	return r.CountByStatus(StatusFail) > 0
}
