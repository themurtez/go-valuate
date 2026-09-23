package statements

import (
	"github.com/themurtez/go-valuate/accounting/ledger"
	"github.com/themurtez/go-valuate/financial"
)

// compatibleCategories maps a ledger.AccountType to the financial.
// CodeCategory values a mapping to that account type may legitimately
// target. This is a fixed table, not a heuristic, mirroring
// ledger.NormalBalance's own "fixed table, never inspects Name" design —
// see the task's explicit account-type-constraint examples (a REVENUE
// account must not map to a balance-sheet asset code; an ASSET account
// must not map to an operating expense code).
//
// A category absent from a given type's list is impossible for that
// account, not merely discouraged — isAccountTypeCompatible below rejects
// it outright (IssueIncompatibleAccountType), it never silently coerces
// the mapping (per the task's explicit "reject, don't coerce" rule).
var compatibleCategories = map[ledger.AccountType][]financial.CodeCategory{
	ledger.AccountRevenue:   {financial.CategoryRevenue},
	ledger.AccountExpense:   {financial.CategoryCogs, financial.CategoryOpex, financial.CategoryOtherIncomeStatement},
	ledger.AccountAsset:     {financial.CategoryBalanceSheet},
	ledger.AccountLiability: {financial.CategoryBalanceSheet},
	ledger.AccountEquity:    {financial.CategoryBalanceSheet},
}

// isAccountTypeCompatible reports whether financial.Code's own registered
// financial.CodeCategory is one acctType may legitimately map to, per
// compatibleCategories. An unrecognized code or account type is treated
// as incompatible (fails closed) — callers should already have validated
// code/account-type validity separately (see IssueInvalidFinancialCode/
// IssueUnknownAccount) and treat an incompatibility finding from THIS
// function as specifically an account-type/category mismatch, not a
// missing-lookup problem.
func isAccountTypeCompatible(acctType ledger.AccountType, code financial.Code) bool {
	meta, ok := financial.LookupCode(code)
	if !ok {
		return false
	}
	allowed, ok := compatibleCategories[acctType]
	if !ok {
		return false
	}
	for _, c := range allowed {
		if c == meta.Category {
			return true
		}
	}
	return false
}

// statementTypeForAccountType returns the financial.StatementType a
// synthetic financial.RawLineItem built from a ledger.Account of the
// given AccountType should declare — used only by the optional
// deterministic-suggestion path (mapping.go) to give
// financial/classification's context-aware rules (which read
// RuleInput.StatementType) an accurate signal, since ledger.Account has
// no StatementType field of its own (ledger is independent of the
// financial domain — see accounting/ledger's package doc comment).
// REVENUE/EXPENSE accounts belong to the income statement;
// ASSET/LIABILITY/EQUITY accounts belong to the balance sheet.
// ledger.AccountType has exactly five values (see ledger.NormalBalance's
// doc comment), so this covers every case without a default fallback.
func statementTypeForAccountType(t ledger.AccountType) financial.StatementType {
	switch t {
	case ledger.AccountRevenue, ledger.AccountExpense:
		return financial.StatementIncomeStatement
	default: // AccountAsset, AccountLiability, AccountEquity
		return financial.StatementBalanceSheet
	}
}
