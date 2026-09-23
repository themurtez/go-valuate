// Package fixtures provides synthetic accounting/statements data for
// tests and examples, building on accounting/ledger/fixtures' businesses
// (service, retailer, owner-operated, multi-department) with the
// account-mapping layer this package's parent adds. Nothing here is real
// financial data.
package fixtures

import (
	"github.com/themurtez/go-valuate/accounting/ledger"
	ledgerfixtures "github.com/themurtez/go-valuate/accounting/ledger/fixtures"
	"github.com/themurtez/go-valuate/accounting/statements"
	"github.com/themurtez/go-valuate/financial"
)

// ServiceBusinessMappings returns explicit, fully-confirmed mappings for
// ledgerfixtures.ServiceBusinessChart, covering every account.
func ServiceBusinessMappings() []statements.AccountMapping {
	return []statements.AccountMapping{
		{AccountID: "1000", FinancialCode: financial.CodeBsCash, StatementType: financial.StatementBalanceSheet, Source: statements.MappingSourceExplicit, Confirmed: true},
		{AccountID: "1100", FinancialCode: financial.CodeBsAccountsReceivable, StatementType: financial.StatementBalanceSheet, Source: statements.MappingSourceExplicit, Confirmed: true},
		{AccountID: "1500", FinancialCode: financial.CodeBsPrepaid, StatementType: financial.StatementBalanceSheet, Source: statements.MappingSourceExplicit, Confirmed: true},
		{AccountID: "2000", FinancialCode: financial.CodeBsAccountsPayable, StatementType: financial.StatementBalanceSheet, Source: statements.MappingSourceExplicit, Confirmed: true},
		{AccountID: "2100", FinancialCode: financial.CodeBsCurrentLiabilityOther, StatementType: financial.StatementBalanceSheet, Source: statements.MappingSourceExplicit, Confirmed: true},
		{AccountID: "3000", FinancialCode: financial.CodeBsOwnerEquity, StatementType: financial.StatementBalanceSheet, Source: statements.MappingSourceExplicit, Confirmed: true},
		{AccountID: "3900", FinancialCode: financial.CodeBsRetainedEarnings, StatementType: financial.StatementBalanceSheet, Source: statements.MappingSourceExplicit, Confirmed: true},
		{AccountID: "4000", FinancialCode: financial.CodeRevService, StatementType: financial.StatementIncomeStatement, Source: statements.MappingSourceExplicit, Confirmed: true},
		{AccountID: "6000", FinancialCode: financial.CodeOpexPayroll, StatementType: financial.StatementIncomeStatement, Source: statements.MappingSourceExplicit, Confirmed: true},
		{AccountID: "6100", FinancialCode: financial.CodeOpexRent, StatementType: financial.StatementIncomeStatement, Source: statements.MappingSourceExplicit, Confirmed: true},
		{AccountID: "6200", FinancialCode: financial.CodeOpexSoftware, StatementType: financial.StatementIncomeStatement, Source: statements.MappingSourceExplicit, Confirmed: true},
	}
}

// RetailerMappings returns explicit mappings for ledgerfixtures.RetailerChart.
func RetailerMappings() []statements.AccountMapping {
	return []statements.AccountMapping{
		{AccountID: "1000", FinancialCode: financial.CodeBsCash, StatementType: financial.StatementBalanceSheet, Source: statements.MappingSourceExplicit, Confirmed: true},
		{AccountID: "1200", FinancialCode: financial.CodeBsInventory, StatementType: financial.StatementBalanceSheet, Source: statements.MappingSourceExplicit, Confirmed: true},
		{AccountID: "2000", FinancialCode: financial.CodeBsAccountsPayable, StatementType: financial.StatementBalanceSheet, Source: statements.MappingSourceExplicit, Confirmed: true},
		{AccountID: "2200", FinancialCode: financial.CodeBsCurrentLiabilityOther, StatementType: financial.StatementBalanceSheet, Source: statements.MappingSourceExplicit, Confirmed: true},
		{AccountID: "3000", FinancialCode: financial.CodeBsOwnerEquity, StatementType: financial.StatementBalanceSheet, Source: statements.MappingSourceExplicit, Confirmed: true},
		{AccountID: "4000", FinancialCode: financial.CodeRevProduct, StatementType: financial.StatementIncomeStatement, Source: statements.MappingSourceExplicit, Confirmed: true},
		{AccountID: "5000", FinancialCode: financial.CodeCogsMaterial, StatementType: financial.StatementIncomeStatement, Source: statements.MappingSourceExplicit, Confirmed: true},
		{AccountID: "6300", FinancialCode: financial.CodeOpexOffice, StatementType: financial.StatementIncomeStatement, Source: statements.MappingSourceExplicit, Confirmed: true},
	}
}

// OwnerOperatedMappings returns explicit mappings for
// ledgerfixtures.OwnerOperatedChart. Note "Owner's Draw" (3100) is
// deliberately left UNMAPPED here — see UnmappedMaterialAccountMappings
// for a variant that maps it, and the mapping-review integration test for
// how a caller resolves it.
func OwnerOperatedMappings() []statements.AccountMapping {
	return []statements.AccountMapping{
		{AccountID: "1000", FinancialCode: financial.CodeBsCash, StatementType: financial.StatementBalanceSheet, Source: statements.MappingSourceExplicit, Confirmed: true},
		{AccountID: "1400", FinancialCode: financial.CodeBsFixedAssets, StatementType: financial.StatementBalanceSheet, Source: statements.MappingSourceExplicit, Confirmed: true},
		{AccountID: "2000", FinancialCode: financial.CodeBsAccountsPayable, StatementType: financial.StatementBalanceSheet, Source: statements.MappingSourceExplicit, Confirmed: true},
		{AccountID: "3000", FinancialCode: financial.CodeBsOwnerEquity, StatementType: financial.StatementBalanceSheet, Source: statements.MappingSourceExplicit, Confirmed: true},
		{AccountID: "4000", FinancialCode: financial.CodeRevService, StatementType: financial.StatementIncomeStatement, Source: statements.MappingSourceExplicit, Confirmed: true},
		{AccountID: "6400", FinancialCode: financial.CodeOpexVehicle, StatementType: financial.StatementIncomeStatement, Source: statements.MappingSourceExplicit, Confirmed: true},
		{AccountID: "6500", FinancialCode: financial.CodeOpexOffice, StatementType: financial.StatementIncomeStatement, Source: statements.MappingSourceExplicit, Confirmed: true},
	}
}

// UnmappedMaterialAccountMappings returns OwnerOperatedMappings with
// account "3100" (Owner's Draw, a material equity balance) deliberately
// left out — for exercising IssueUnmappedAccount materiality.
func UnmappedMaterialAccountMappings() []statements.AccountMapping {
	return OwnerOperatedMappings()
}

// MultiRevenueAccountMappings returns mappings demonstrating many-to-one
// aggregation (task section 20): three distinct revenue accounts on a
// small synthetic chart all map to the single canonical
// financial.CodeRevService code.
func MultiRevenueChart() []ledger.Account {
	return []ledger.Account{
		{ID: "1000", Number: "1000", Name: "Cash", Type: ledger.AccountAsset, Currency: "USD", Active: true},
		{ID: "3000", Number: "3000", Name: "Common Stock", Type: ledger.AccountEquity, Currency: "USD", Active: true},
		{ID: "4000", Number: "4000", Name: "Product Revenue", Type: ledger.AccountRevenue, Currency: "USD", Active: true},
		{ID: "4010", Number: "4010", Name: "Installation Revenue", Type: ledger.AccountRevenue, Currency: "USD", Active: true},
		{ID: "4020", Number: "4020", Name: "Service Revenue", Type: ledger.AccountRevenue, Currency: "USD", Active: true},
	}
}

// MultiRevenueEntries returns balanced journal entries posting to each of
// MultiRevenueChart's three revenue accounts.
func MultiRevenueEntries() []ledger.JournalEntry {
	return []ledger.JournalEntry{
		{
			ID: "MR-JE-1", Date: "2025-07-01", Period: "2025-07", Status: ledger.StatusPosted,
			Description: "Capitalization", Source: "manual",
			Lines: []ledger.JournalLine{
				{AccountID: "1000", Debit: 10000},
				{AccountID: "3000", Credit: 10000},
			},
		},
		{
			ID: "MR-JE-2", Date: "2025-07-05", Period: "2025-07", Status: ledger.StatusPosted,
			Description: "Product sale", Source: "manual",
			Lines: []ledger.JournalLine{
				{AccountID: "1000", Debit: 5000},
				{AccountID: "4000", Credit: 5000},
			},
		},
		{
			ID: "MR-JE-3", Date: "2025-07-06", Period: "2025-07", Status: ledger.StatusPosted,
			Description: "Installation fee", Source: "manual",
			Lines: []ledger.JournalLine{
				{AccountID: "1000", Debit: 1200},
				{AccountID: "4010", Credit: 1200},
			},
		},
		{
			ID: "MR-JE-4", Date: "2025-07-07", Period: "2025-07", Status: ledger.StatusPosted,
			Description: "Service engagement", Source: "manual",
			Lines: []ledger.JournalLine{
				{AccountID: "1000", Debit: 3000},
				{AccountID: "4020", Credit: 3000},
			},
		},
	}
}

// MultiRevenueMappings maps every one of MultiRevenueChart's three
// revenue accounts to the single canonical financial.CodeRevService
// code — the many-to-one case.
func MultiRevenueMappings() []statements.AccountMapping {
	target := func(id string) statements.AccountMapping {
		return statements.AccountMapping{
			AccountID: id, FinancialCode: financial.CodeRevService, StatementType: financial.StatementIncomeStatement,
			Source: statements.MappingSourceExplicit, Confirmed: true,
		}
	}
	return []statements.AccountMapping{target("4000"), target("4010"), target("4020")}
}

// ContraAccountChart returns a small chart including "Accumulated
// Depreciation" — an ASSET-type ledger account carrying a natural CREDIT
// balance (the contra-asset case task section 13 requires a fixture for).
func ContraAccountChart() []ledger.Account {
	return []ledger.Account{
		{ID: "1000", Number: "1000", Name: "Cash", Type: ledger.AccountAsset, Currency: "USD", Active: true},
		{ID: "1400", Number: "1400", Name: "Fixed Assets", Type: ledger.AccountAsset, Currency: "USD", Active: true},
		{ID: "1450", Number: "1450", Name: "Accumulated Depreciation", Type: ledger.AccountAsset, Currency: "USD", Active: true},
		{ID: "3000", Number: "3000", Name: "Common Stock", Type: ledger.AccountEquity, Currency: "USD", Active: true},
	}
}

// ContraAccountEntries posts a capital contribution, an equipment
// purchase funded from that cash, and a depreciation entry crediting
// Accumulated Depreciation (simplified to a direct equity debit rather
// than routing through a depreciation expense account, since this
// fixture's purpose is only to exercise contra-asset sign handling on the
// balance sheet, not model a full depreciation policy or income
// statement).
func ContraAccountEntries() []ledger.JournalEntry {
	return []ledger.JournalEntry{
		{
			ID: "CA-JE-1", Date: "2025-08-01", Period: "2025-08", Status: ledger.StatusPosted,
			Description: "Owner capital contribution", Source: "manual",
			Lines: []ledger.JournalLine{
				{AccountID: "1000", Debit: 50000},
				{AccountID: "3000", Credit: 50000},
			},
		},
		{
			ID: "CA-JE-2", Date: "2025-08-05", Period: "2025-08", Status: ledger.StatusPosted,
			Description: "Equipment purchase, cash", Source: "manual",
			Lines: []ledger.JournalLine{
				{AccountID: "1400", Debit: 20000},
				{AccountID: "1000", Credit: 20000},
			},
		},
		{
			ID: "CA-JE-3", Date: "2025-08-15", Period: "2025-08", Status: ledger.StatusPosted,
			Description: "Accumulated depreciation, funded by additional paid-in capital for fixture simplicity",
			Source:      "manual",
			Lines: []ledger.JournalLine{
				{AccountID: "3000", Debit: 4000},
				{AccountID: "1450", Credit: 4000},
			},
		},
	}
}

// ContraAccountMappings maps ContraAccountChart, using SignInvert on the
// Accumulated Depreciation account so its natural-credit balance becomes
// the positive magnitude financial.CodeBsAccumDepreciation expects.
func ContraAccountMappings() []statements.AccountMapping {
	return []statements.AccountMapping{
		{AccountID: "1000", FinancialCode: financial.CodeBsCash, StatementType: financial.StatementBalanceSheet, Source: statements.MappingSourceExplicit, Confirmed: true},
		{AccountID: "1400", FinancialCode: financial.CodeBsFixedAssets, StatementType: financial.StatementBalanceSheet, Source: statements.MappingSourceExplicit, Confirmed: true},
		{
			AccountID: "1450", FinancialCode: financial.CodeBsAccumDepreciation, StatementType: financial.StatementBalanceSheet,
			SignTreatment: statements.SignInvert, Source: statements.MappingSourceExplicit, Confirmed: true,
		},
		{AccountID: "3000", FinancialCode: financial.CodeBsOwnerEquity, StatementType: financial.StatementBalanceSheet, Source: statements.MappingSourceExplicit, Confirmed: true},
	}
}

// InvalidMappingExample returns a mapping that fails validation: account
// "4000" (a REVENUE account on ServiceBusinessChart) mapped to a
// balance-sheet code, for exercising IssueIncompatibleAccountType.
func InvalidMappingExample() statements.AccountMapping {
	return statements.AccountMapping{
		AccountID: "4000", FinancialCode: financial.CodeBsCash, StatementType: financial.StatementBalanceSheet,
		Source: statements.MappingSourceExplicit, Confirmed: true,
	}
}

// MultiDepartmentMappings maps ledgerfixtures.MultiDepartmentChart at the
// LEAF level only (task section 14's "map leaf accounts, not hierarchy
// rollup totals" policy): "4000"/"6000" (the parent rollup accounts) are
// deliberately left unmapped, and only their children plus "4000"/"6000"'s
// own direct postings are mapped as ordinary accounts in their own right
// (since ledger.MultiDepartmentEntries posts directly to "6000" as well
// as its children — see that fixture's own doc comment). This
// demonstrates that mapping every account that HAS a balance (parent
// included) without rolling up avoids double counting, since
// ledger.CalculateBalances/aggregateMappedAccounts only ever sum DIRECT
// balances.
func MultiDepartmentMappings() []statements.AccountMapping {
	target := func(id string, code financial.Code, st financial.StatementType) statements.AccountMapping {
		return statements.AccountMapping{AccountID: id, FinancialCode: code, StatementType: st, Source: statements.MappingSourceExplicit, Confirmed: true}
	}
	return []statements.AccountMapping{
		target("1000", financial.CodeBsCash, financial.StatementBalanceSheet),
		target("3000", financial.CodeBsOwnerEquity, financial.StatementBalanceSheet),
		target("4100", financial.CodeRevProduct, financial.StatementIncomeStatement),
		target("4200", financial.CodeRevService, financial.StatementIncomeStatement),
		target("6100", financial.CodeOpexOther, financial.StatementIncomeStatement),
		target("6200", financial.CodeOpexOther, financial.StatementIncomeStatement),
		target("6900", financial.CodeOpexOther, financial.StatementIncomeStatement),
	}
}

// ServiceBusinessMappingTemplate returns a MappingTemplate equivalent to
// ServiceBusinessMappings, expressed as reusable AccountMappingRule
// values matched by exact account number — task section 32/33's
// reusable-template requirement.
func ServiceBusinessMappingTemplate() statements.MappingTemplate {
	rule := func(number string, code financial.Code, st financial.StatementType) statements.AccountMappingRule {
		return statements.AccountMappingRule{
			AccountNumber: number,
			Mapping:       statements.AccountMapping{FinancialCode: code, StatementType: st, Confirmed: true},
		}
	}
	return statements.MappingTemplate{
		Version: "service-business-v1",
		Mappings: []statements.AccountMappingRule{
			rule("1000", financial.CodeBsCash, financial.StatementBalanceSheet),
			rule("1100", financial.CodeBsAccountsReceivable, financial.StatementBalanceSheet),
			rule("1500", financial.CodeBsPrepaid, financial.StatementBalanceSheet),
			rule("2000", financial.CodeBsAccountsPayable, financial.StatementBalanceSheet),
			rule("2100", financial.CodeBsCurrentLiabilityOther, financial.StatementBalanceSheet),
			rule("3000", financial.CodeBsOwnerEquity, financial.StatementBalanceSheet),
			rule("3900", financial.CodeBsRetainedEarnings, financial.StatementBalanceSheet),
			rule("4000", financial.CodeRevService, financial.StatementIncomeStatement),
			rule("6000", financial.CodeOpexPayroll, financial.StatementIncomeStatement),
			rule("6100", financial.CodeOpexRent, financial.StatementIncomeStatement),
			rule("6200", financial.CodeOpexSoftware, financial.StatementIncomeStatement),
		},
	}
}

// ConflictingMappingTemplate returns a template with two rules matching
// the same account number ("1000"), for exercising IssueMappingConflict
// via statements.ResolveTemplate.
func ConflictingMappingTemplate() statements.MappingTemplate {
	return statements.MappingTemplate{
		Version: "conflicting-v1",
		Mappings: []statements.AccountMappingRule{
			{AccountNumber: "1000", Mapping: statements.AccountMapping{FinancialCode: financial.CodeBsCash, StatementType: financial.StatementBalanceSheet}},
			{AccountNumber: "1000", Mapping: statements.AccountMapping{FinancialCode: financial.CodeBsCurrentAssetOther, StatementType: financial.StatementBalanceSheet}},
		},
	}
}

// Re-exported ledger fixtures, so a caller of this package never needs a
// second import of accounting/ledger/fixtures for the underlying
// chart/entries this package's mappings are built against.
var (
	ServiceBusinessChart     = ledgerfixtures.ServiceBusinessChart
	ServiceBusinessEntries   = ledgerfixtures.ServiceBusinessEntries
	RetailerChart            = ledgerfixtures.RetailerChart
	RetailerEntries          = ledgerfixtures.RetailerEntries
	OwnerOperatedChart       = ledgerfixtures.OwnerOperatedChart
	OwnerOperatedEntries     = ledgerfixtures.OwnerOperatedEntries
	MultiDepartmentChart     = ledgerfixtures.MultiDepartmentChart
	MultiDepartmentEntries   = ledgerfixtures.MultiDepartmentEntries
	UnbalancedJournalEntries = ledgerfixtures.UnbalancedJournalEntries
	UnbalancedTrialBalance   = ledgerfixtures.UnbalancedTrialBalance
)
