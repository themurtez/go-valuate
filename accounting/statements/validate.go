package statements

import (
	"math"
	"strconv"

	"github.com/themurtez/go-valuate/accounting/ledger"
	"github.com/themurtez/go-valuate/financial"
)

// allocationTolerance is the maximum allowed absolute deviation of an
// AccountMapping.Allocations' Percent sum from 1.0 before
// IssueInvalidAllocation fires — a small fixed tolerance for float
// accumulation noise, not a business-meaningful rounding allowance (see
// the task's "verify allocations sum to 100%" instruction).
const allocationTolerance = 1e-9

// validateMapping checks a single resolved AccountMapping against the
// rules the task's mapping-validation section enumerates: referenced
// account exists, financial code is valid, statement type is compatible,
// account-type safety, sign-treatment validity, allocation validity, and
// inactive-account posting policy. It does not mutate acct or mapping.
// Structural problems shared across all mappings for the same account
// (duplicate/conflicting explicit mappings) are handled separately by
// indexExplicitMappings in mapping.go, since those require comparing
// across mappings rather than validating one in isolation.
func validateMapping(acct ledger.Account, mapping AccountMapping, chart ledger.ChartOfAccounts) []Issue {
	var issues []Issue

	if _, ok := chart.Lookup(mapping.AccountID); mapping.AccountID != "" && !ok {
		issues = append(issues, Issue{
			Code:      IssueInvalidMapping,
			Severity:  SeverityError,
			Message:   "mapping references unknown account: " + mapping.AccountID,
			AccountID: mapping.AccountID,
		})
		return issues // nothing further to check without a real account
	}

	if mapping.Source == MappingSourceUnmapped {
		return issues // nothing to validate for an intentionally-unmapped account
	}

	if !acct.Active {
		issues = append(issues, Issue{
			Code:      IssueInvalidMapping,
			Severity:  SeverityWarning,
			Message:   "account " + acct.ID + " is inactive but has a mapping; historical balances still apply",
			AccountID: acct.ID,
		})
	}

	if mapping.isAllocated() {
		issues = append(issues, validateAllocations(acct, mapping.Allocations)...)
		return issues
	}

	issues = append(issues, validateSingleTarget(acct, mapping.AccountID, mapping.FinancialCode, mapping.StatementType, mapping.SignTreatment)...)
	return issues
}

// validateSingleTarget validates one (FinancialCode, StatementType,
// SignTreatment) target for acct — shared by validateMapping's
// single-code path and validateAllocations' per-allocation path so the
// two never drift apart on what "a valid target" means.
func validateSingleTarget(acct ledger.Account, accountID string, code financial.Code, stType financial.StatementType, sign SignTreatment) []Issue {
	var issues []Issue

	if code == "" {
		issues = append(issues, Issue{
			Code:      IssueInvalidMapping,
			Severity:  SeverityError,
			Message:   "mapping for account " + accountID + " has an empty financial code",
			AccountID: accountID,
		})
		return issues
	}

	meta, ok := financial.LookupCode(code)
	if !ok {
		issues = append(issues, Issue{
			Code:          IssueInvalidFinancialCode,
			Severity:      SeverityError,
			Message:       "mapping for account " + accountID + " references unrecognized financial code: " + string(code),
			AccountID:     accountID,
			FinancialCode: string(code),
		})
		return issues
	}

	if stType != "" && stType != meta.StatementType {
		issues = append(issues, Issue{
			Code:          IssueInvalidMapping,
			Severity:      SeverityError,
			Message:       "mapping for account " + accountID + " declares statement type " + string(stType) + " but code " + string(code) + " belongs to " + string(meta.StatementType),
			AccountID:     accountID,
			FinancialCode: string(code),
		})
	}

	if !isAccountTypeCompatible(acct.Type, code) {
		issues = append(issues, Issue{
			Code:          IssueIncompatibleAccountType,
			Severity:      SeverityError,
			Message:       "account " + acct.ID + " (" + string(acct.Type) + ") cannot map to " + string(code) + " (" + string(meta.Category) + ")",
			AccountID:     acct.ID,
			FinancialCode: string(code),
		})
	}

	// Checked against the RAW sign value, not resolvedSignTreatment(sign)
	// — resolvedSignTreatment always returns a valid value (silently
	// falling back to SignNatural for anything it doesn't recognize,
	// including the empty zero value, which is a legitimate "not set"
	// case), so validating its OUTPUT could never detect a genuinely
	// invalid caller-supplied value in the first place. The empty string
	// (zero value, "not set") is not itself an error; only a non-empty,
	// unrecognized value is.
	switch sign {
	case "", SignNatural, SignNormal, SignInvert:
		// valid (or unset, which resolves to SignNatural)
	default:
		issues = append(issues, Issue{
			Code:      IssueInvalidSignTreatment,
			Severity:  SeverityError,
			Message:   "mapping for account " + accountID + " has an invalid sign treatment: " + string(sign),
			AccountID: accountID,
		})
	}

	return issues
}

// validateAllocations checks a one-to-many AccountMapping.Allocations
// slice: every allocation individually valid (via validateSingleTarget),
// Percent values finite and non-negative, and the sum of Percent across
// all allocations equal to 1.0 within allocationTolerance — see the
// task's explicit "verify allocations sum to 100%" instruction. Never
// infers or auto-normalizes a short/over sum; always flags it.
func validateAllocations(acct ledger.Account, allocations []AllocationRule) []Issue {
	var issues []Issue
	var sum float64

	for i, a := range allocations {
		issues = append(issues, validateSingleTarget(acct, acct.ID, a.FinancialCode, a.StatementType, a.SignTreatment)...)

		if math.IsNaN(a.Percent) || math.IsInf(a.Percent, 0) || a.Percent < 0 {
			issues = append(issues, Issue{
				Code:      IssueInvalidAllocation,
				Severity:  SeverityError,
				Message:   "allocation [" + strconv.Itoa(i) + "] for account " + acct.ID + " has an invalid percent",
				AccountID: acct.ID,
			})
			continue
		}
		sum += a.Percent
	}

	if len(allocations) > 0 && math.Abs(sum-1.0) > allocationTolerance {
		issues = append(issues, Issue{
			Code:      IssueInvalidAllocation,
			Severity:  SeverityError,
			Message:   "allocations for account " + acct.ID + " sum to " + strconv.FormatFloat(sum*100, 'f', 4, 64) + "%, not 100%",
			AccountID: acct.ID,
		})
	}

	return issues
}
