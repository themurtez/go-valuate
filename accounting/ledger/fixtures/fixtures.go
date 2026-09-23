// Package fixtures provides synthetic accounting/ledger data for tests and
// examples: a handful of small, hand-built businesses plus a set of
// deliberately-broken scenarios (unbalanced journal, unbalanced imported
// TB, mixed currency) used to exercise this package's validation paths.
// Nothing here is real financial data — every figure is invented for
// illustration.
package fixtures

import "github.com/themurtez/go-valuate/accounting/ledger"

// ServiceBusinessChart returns a small chart of accounts for a
// professional-services business (no inventory/COGS).
func ServiceBusinessChart() []ledger.Account {
	return []ledger.Account{
		{ID: "1000", Number: "1000", Name: "Cash", Type: ledger.AccountAsset, Currency: "USD", Active: true},
		{ID: "1100", Number: "1100", Name: "Accounts Receivable", Type: ledger.AccountAsset, Currency: "USD", Active: true},
		{ID: "1500", Number: "1500", Name: "Prepaid Expenses", Type: ledger.AccountAsset, Currency: "USD", Active: true},
		{ID: "2000", Number: "2000", Name: "Accounts Payable", Type: ledger.AccountLiability, Currency: "USD", Active: true},
		{ID: "2100", Number: "2100", Name: "Accrued Payroll", Type: ledger.AccountLiability, Currency: "USD", Active: true},
		{ID: "3000", Number: "3000", Name: "Owner's Equity", Type: ledger.AccountEquity, Currency: "USD", Active: true},
		{ID: "3900", Number: "3900", Name: "Retained Earnings", Type: ledger.AccountEquity, Currency: "USD", Active: true},
		{ID: "4000", Number: "4000", Name: "Consulting Revenue", Type: ledger.AccountRevenue, Currency: "USD", Active: true},
		{ID: "6000", Number: "6000", Name: "Salaries Expense", Type: ledger.AccountExpense, Currency: "USD", Active: true},
		{ID: "6100", Number: "6100", Name: "Rent Expense", Type: ledger.AccountExpense, Currency: "USD", Active: true},
		{ID: "6200", Number: "6200", Name: "Software Expense", Type: ledger.AccountExpense, Currency: "USD", Active: true},
	}
}

// ServiceBusinessEntries returns a small, internally-balanced set of
// journal entries for ServiceBusinessChart covering a single month.
func ServiceBusinessEntries() []ledger.JournalEntry {
	return []ledger.JournalEntry{
		{
			ID: "SVC-JE-1", Date: "2025-01-05", Period: "2025-01", Status: ledger.StatusPosted,
			Description: "Owner capital contribution", Source: "manual",
			Lines: []ledger.JournalLine{
				{ID: "L1", AccountID: "1000", Debit: 50000},
				{ID: "L2", AccountID: "3000", Credit: 50000},
			},
		},
		{
			ID: "SVC-JE-2", Date: "2025-01-10", Period: "2025-01", Status: ledger.StatusPosted,
			Description: "Consulting invoice #101", Source: "billing", Reference: "INV-101",
			Lines: []ledger.JournalLine{
				{ID: "L1", AccountID: "1100", Debit: 12000},
				{ID: "L2", AccountID: "4000", Credit: 12000},
			},
		},
		{
			ID: "SVC-JE-3", Date: "2025-01-15", Period: "2025-01", Status: ledger.StatusPosted,
			Description: "January payroll", Source: "payroll",
			Lines: []ledger.JournalLine{
				{ID: "L1", AccountID: "6000", Debit: 8000},
				{ID: "L2", AccountID: "1000", Credit: 8000},
			},
		},
		{
			ID: "SVC-JE-4", Date: "2025-01-20", Period: "2025-01", Status: ledger.StatusPosted,
			Description: "Office rent", Source: "accounts_payable",
			Lines: []ledger.JournalLine{
				{ID: "L1", AccountID: "6100", Debit: 3000},
				{ID: "L2", AccountID: "1000", Credit: 3000},
			},
		},
		{
			ID: "SVC-JE-5", Date: "2025-01-25", Period: "2025-01", Status: ledger.StatusPosted,
			Description: "Cash collected on invoice #101", Source: "billing", Reference: "INV-101",
			Lines: []ledger.JournalLine{
				{ID: "L1", AccountID: "1000", Debit: 12000},
				{ID: "L2", AccountID: "1100", Credit: 12000},
			},
		},
	}
}

// RetailerChart returns a chart of accounts for a small retailer,
// including inventory/COGS accounts a service business doesn't need.
func RetailerChart() []ledger.Account {
	return []ledger.Account{
		{ID: "1000", Number: "1000", Name: "Cash", Type: ledger.AccountAsset, Currency: "USD", Active: true},
		{ID: "1200", Number: "1200", Name: "Inventory", Type: ledger.AccountAsset, Currency: "USD", Active: true},
		{ID: "2000", Number: "2000", Name: "Accounts Payable", Type: ledger.AccountLiability, Currency: "USD", Active: true},
		{ID: "2200", Number: "2200", Name: "Sales Tax Payable", Type: ledger.AccountLiability, Currency: "USD", Active: true},
		{ID: "3000", Number: "3000", Name: "Common Stock", Type: ledger.AccountEquity, Currency: "USD", Active: true},
		{ID: "4000", Number: "4000", Name: "Merchandise Sales", Type: ledger.AccountRevenue, Currency: "USD", Active: true},
		{ID: "5000", Number: "5000", Name: "Cost of Goods Sold", Type: ledger.AccountExpense, Currency: "USD", Active: true},
		{ID: "6300", Number: "6300", Name: "Store Supplies Expense", Type: ledger.AccountExpense, Currency: "USD", Active: true},
	}
}

// RetailerEntries returns a small, internally-balanced set of journal
// entries for RetailerChart, including an inventory purchase and a sale
// with its matching cost-of-goods-sold entry.
func RetailerEntries() []ledger.JournalEntry {
	return []ledger.JournalEntry{
		{
			ID: "RTL-JE-1", Date: "2025-03-01", Period: "2025-03", Status: ledger.StatusPosted,
			Description: "Initial capitalization", Source: "manual",
			Lines: []ledger.JournalLine{
				{AccountID: "1000", Debit: 100000},
				{AccountID: "3000", Credit: 100000},
			},
		},
		{
			ID: "RTL-JE-2", Date: "2025-03-03", Period: "2025-03", Status: ledger.StatusPosted,
			Description: "Inventory purchase on account", Source: "accounts_payable",
			Lines: []ledger.JournalLine{
				{AccountID: "1200", Debit: 40000},
				{AccountID: "2000", Credit: 40000},
			},
		},
		{
			ID: "RTL-JE-3", Date: "2025-03-10", Period: "2025-03", Status: ledger.StatusPosted,
			Description: "Cash sale, including sales tax collected", Source: "pos",
			Lines: []ledger.JournalLine{
				{AccountID: "1000", Debit: 21600},
				{AccountID: "4000", Credit: 20000},
				{AccountID: "2200", Credit: 1600},
			},
		},
		{
			ID: "RTL-JE-4", Date: "2025-03-10", Period: "2025-03", Status: ledger.StatusPosted,
			Description: "Cost of goods sold for the above sale", Source: "pos",
			Lines: []ledger.JournalLine{
				{AccountID: "5000", Debit: 9000},
				{AccountID: "1200", Credit: 9000},
			},
		},
	}
}

// OwnerOperatedChart returns a chart of accounts for a small owner-operated
// business (sole proprietor / SDE-style), with an owner's draw account
// rather than payroll for the owner.
func OwnerOperatedChart() []ledger.Account {
	return []ledger.Account{
		{ID: "1000", Number: "1000", Name: "Cash", Type: ledger.AccountAsset, Currency: "USD", Active: true},
		{ID: "1400", Number: "1400", Name: "Equipment", Type: ledger.AccountAsset, Currency: "USD", Active: true},
		{ID: "2000", Number: "2000", Name: "Accounts Payable", Type: ledger.AccountLiability, Currency: "USD", Active: true},
		{ID: "3000", Number: "3000", Name: "Owner's Capital", Type: ledger.AccountEquity, Currency: "USD", Active: true},
		{ID: "3100", Number: "3100", Name: "Owner's Draw", Type: ledger.AccountEquity, Currency: "USD", Active: true},
		{ID: "4000", Number: "4000", Name: "Service Revenue", Type: ledger.AccountRevenue, Currency: "USD", Active: true},
		{ID: "6400", Number: "6400", Name: "Fuel & Vehicle Expense", Type: ledger.AccountExpense, Currency: "USD", Active: true},
		{ID: "6500", Number: "6500", Name: "Supplies Expense", Type: ledger.AccountExpense, Currency: "USD", Active: true},
	}
}

// OwnerOperatedEntries returns a small, internally-balanced set of journal
// entries for OwnerOperatedChart, including an owner's draw (equity debit
// — the only account in this fixture set exercising a natural-credit
// account being debited in the ordinary course of business, not an error).
func OwnerOperatedEntries() []ledger.JournalEntry {
	return []ledger.JournalEntry{
		{
			ID: "OWN-JE-1", Date: "2025-02-01", Period: "2025-02", Status: ledger.StatusPosted,
			Description: "Job revenue, cash", Source: "manual",
			Lines: []ledger.JournalLine{
				{AccountID: "1000", Debit: 6000},
				{AccountID: "4000", Credit: 6000},
			},
		},
		{
			ID: "OWN-JE-2", Date: "2025-02-05", Period: "2025-02", Status: ledger.StatusPosted,
			Description: "Fuel purchase", Source: "manual",
			Lines: []ledger.JournalLine{
				{AccountID: "6400", Debit: 400},
				{AccountID: "1000", Credit: 400},
			},
		},
		{
			ID: "OWN-JE-3", Date: "2025-02-10", Period: "2025-02", Status: ledger.StatusPosted,
			Description: "Owner draw for personal use", Source: "manual",
			Lines: []ledger.JournalLine{
				{AccountID: "3100", Debit: 2000},
				{AccountID: "1000", Credit: 2000},
			},
		},
	}
}

// MultiDepartmentChart returns a chart of accounts with a parent/child
// hierarchy (rollup accounts) for a business with two operating
// departments, used to exercise hierarchy/rollup behavior. "6000" and
// "6100" (both Revenue's Total Operating Expenses, allowing a direct
// posting to a parent) demonstrate the DirectBalance/ChildBalance/
// TotalBalance distinction rollups.go documents.
func MultiDepartmentChart() []ledger.Account {
	return []ledger.Account{
		{ID: "1000", Number: "1000", Name: "Cash", Type: ledger.AccountAsset, Currency: "USD", Active: true},
		{ID: "3000", Number: "3000", Name: "Common Stock", Type: ledger.AccountEquity, Currency: "USD", Active: true},

		{ID: "4000", Number: "4000", Name: "Total Revenue", Type: ledger.AccountRevenue, Currency: "USD", Active: true},
		{ID: "4100", Number: "4100", Name: "Product Department Revenue", Type: ledger.AccountRevenue, ParentID: "4000", Currency: "USD", Active: true},
		{ID: "4200", Number: "4200", Name: "Services Department Revenue", Type: ledger.AccountRevenue, ParentID: "4000", Currency: "USD", Active: true},

		{ID: "6000", Number: "6000", Name: "Total Operating Expenses", Type: ledger.AccountExpense, Currency: "USD", Active: true},
		{ID: "6100", Number: "6100", Name: "Product Department Expenses", Type: ledger.AccountExpense, ParentID: "6000", Currency: "USD", Active: true},
		{ID: "6200", Number: "6200", Name: "Services Department Expenses", Type: ledger.AccountExpense, ParentID: "6000", Currency: "USD", Active: true},
		{ID: "6900", Number: "6900", Name: "Shared Corporate Overhead", Type: ledger.AccountExpense, ParentID: "6000", Currency: "USD", Active: true},
	}
}

// MultiDepartmentEntries returns journal entries for MultiDepartmentChart,
// posting to department-level leaf accounts and, deliberately, one entry
// posting directly to the "6000" parent (shared overhead allocated at the
// rollup level) so a caller can observe DirectBalance vs. ChildBalance.
func MultiDepartmentEntries() []ledger.JournalEntry {
	return []ledger.JournalEntry{
		{
			ID: "DEPT-JE-1", Date: "2025-04-01", Period: "2025-04", Status: ledger.StatusPosted,
			Description: "Product department sale", Source: "manual",
			Lines: []ledger.JournalLine{
				{AccountID: "1000", Debit: 15000, Dimensions: []ledger.Dimension{{Key: ledger.DimensionDepartment, Value: "Product"}}},
				{AccountID: "4100", Credit: 15000, Dimensions: []ledger.Dimension{{Key: ledger.DimensionDepartment, Value: "Product"}}},
			},
		},
		{
			ID: "DEPT-JE-2", Date: "2025-04-02", Period: "2025-04", Status: ledger.StatusPosted,
			Description: "Services department engagement", Source: "manual",
			Lines: []ledger.JournalLine{
				{AccountID: "1000", Debit: 9000, Dimensions: []ledger.Dimension{{Key: ledger.DimensionDepartment, Value: "Services"}}},
				{AccountID: "4200", Credit: 9000, Dimensions: []ledger.Dimension{{Key: ledger.DimensionDepartment, Value: "Services"}}},
			},
		},
		{
			ID: "DEPT-JE-3", Date: "2025-04-05", Period: "2025-04", Status: ledger.StatusPosted,
			Description: "Product department cost", Source: "manual",
			Lines: []ledger.JournalLine{
				{AccountID: "6100", Debit: 4000, Dimensions: []ledger.Dimension{{Key: ledger.DimensionDepartment, Value: "Product"}}},
				{AccountID: "1000", Credit: 4000},
			},
		},
		{
			ID: "DEPT-JE-4", Date: "2025-04-06", Period: "2025-04", Status: ledger.StatusPosted,
			Description: "Services department cost", Source: "manual",
			Lines: []ledger.JournalLine{
				{AccountID: "6200", Debit: 2500, Dimensions: []ledger.Dimension{{Key: ledger.DimensionDepartment, Value: "Services"}}},
				{AccountID: "1000", Credit: 2500},
			},
		},
		{
			ID: "DEPT-JE-5", Date: "2025-04-10", Period: "2025-04", Status: ledger.StatusPosted,
			Description: "Shared corporate overhead, posted directly to the rollup parent",
			Source:      "manual",
			Lines: []ledger.JournalLine{
				{AccountID: "6000", Debit: 1000},
				{AccountID: "1000", Credit: 1000},
			},
		},
	}
}

// UnbalancedJournalEntries returns a small journal where one entry does
// not balance (total debits != total credits), for exercising
// IssueUnbalancedEntry.
func UnbalancedJournalEntries() []ledger.JournalEntry {
	return []ledger.JournalEntry{
		{
			ID: "BAD-JE-1", Date: "2025-05-01", Period: "2025-05", Status: ledger.StatusPosted,
			Description: "Deliberately unbalanced entry", Source: "manual",
			Lines: []ledger.JournalLine{
				{AccountID: "1000", Debit: 5000},
				{AccountID: "4000", Credit: 4500}, // short by 500
			},
		},
	}
}

// UnbalancedTrialBalance returns an imported TrialBalanceInput whose totals
// do not agree, for exercising IssueUnbalancedTrialBalance via
// NormalizeTrialBalance.
func UnbalancedTrialBalance() ledger.TrialBalanceInput {
	return ledger.TrialBalanceInput{
		Period: "2025-05",
		Lines: []ledger.TrialBalanceInputLine{
			{AccountID: "1000", Debit: 20000},
			{AccountID: "2000", Credit: 8000},
			{AccountID: "3000", Credit: 11000}, // total credits 19000 != 20000 debits
		},
	}
}

// OpeningBalancesFixture returns opening balances for ServiceBusinessChart,
// representing a mid-year cutover where the caller supplies starting
// positions instead of replaying prior-year journal history.
func OpeningBalancesFixture() []ledger.OpeningBalance {
	return []ledger.OpeningBalance{
		{AccountID: "1000", Debit: 25000, Currency: "USD", SourceRef: "2024-closing-tb"},
		{AccountID: "1100", Debit: 6000, Currency: "USD", SourceRef: "2024-closing-tb"},
		{AccountID: "2000", Credit: 3000, Currency: "USD", SourceRef: "2024-closing-tb"},
		{AccountID: "3000", Credit: 20000, Currency: "USD", SourceRef: "2024-closing-tb"},
		{AccountID: "3900", Credit: 8000, Currency: "USD", SourceRef: "2024-closing-tb"},
	}
}

// ReversalPairEntries returns a JournalEntry and its explicit reversing
// entry (see ledger.Reversal), for exercising the reversal-consistency
// checks and confirming StatusReversed still affects balances until offset
// by its reversal.
func ReversalPairEntries() []ledger.JournalEntry {
	return []ledger.JournalEntry{
		{
			ID: "REV-JE-1", Date: "2025-06-01", Period: "2025-06", Status: ledger.StatusReversed,
			Description: "Mis-posted rent expense (later reversed)", Source: "manual",
			Reversal: ledger.Reversal{ReversedByEntryID: "REV-JE-2"},
			Lines: []ledger.JournalLine{
				{AccountID: "6100", Debit: 3000},
				{AccountID: "1000", Credit: 3000},
			},
		},
		{
			ID: "REV-JE-2", Date: "2025-06-02", Period: "2025-06", Status: ledger.StatusPosted,
			Description: "Reversal of REV-JE-1", Source: "manual",
			Reversal: ledger.Reversal{ReversalOfEntryID: "REV-JE-1"},
			Lines: []ledger.JournalLine{
				{AccountID: "1000", Debit: 3000},
				{AccountID: "6100", Credit: 3000},
			},
		},
	}
}

// MixedCurrencyChart returns two accounts denominated in different
// currencies, for exercising IssueMixedCurrency.
func MixedCurrencyChart() []ledger.Account {
	return []ledger.Account{
		{ID: "1000", Number: "1000", Name: "Cash - USD", Type: ledger.AccountAsset, Currency: "USD", Active: true},
		{ID: "1010", Number: "1010", Name: "Cash - EUR", Type: ledger.AccountAsset, Currency: "EUR", Active: true},
		{ID: "3000", Number: "3000", Name: "Common Stock", Type: ledger.AccountEquity, Currency: "USD", Active: true},
	}
}

// MixedCurrencyTrialBalance returns an imported TB mixing USD and EUR
// rows with no conversion supplied, for exercising IssueMixedCurrency via
// NormalizeTrialBalance.
func MixedCurrencyTrialBalance() ledger.TrialBalanceInput {
	return ledger.TrialBalanceInput{
		Period: "2025-06",
		Lines: []ledger.TrialBalanceInputLine{
			{AccountID: "1000", Debit: 10000, Currency: "USD"},
			{AccountID: "1010", Debit: 5000, Currency: "EUR"},
			{AccountID: "3000", Credit: 10000, Currency: "USD"},
		},
	}
}
