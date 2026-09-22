package financial

import "sort"

// Code is a stable canonical taxonomy identifier for a classified financial
// line item (e.g. CodeOpexMarketing). Codes are the durable contract between
// classification, normalization, and downstream valuation logic — display
// labels and descriptions are metadata (see CodeMeta) and may change
// independently of the code itself.
//
// The taxonomy is currently a flat set of codes. It is designed so a later
// version can introduce hierarchy (e.g. a Parent field in CodeMeta) without
// changing the string value of any existing code.
type Code string

// Revenue codes.
const (
	CodeRevProduct   Code = "REV_PRODUCT"
	CodeRevService   Code = "REV_SERVICE"
	CodeRevRecurring Code = "REV_RECURRING"
	CodeRevOther     Code = "REV_OTHER"
)

// Cost of goods sold codes.
const (
	CodeCogsMaterial    Code = "COGS_MATERIAL"
	CodeCogsDirectLabor Code = "COGS_DIRECT_LABOR"
	CodeCogsFreight     Code = "COGS_FREIGHT"
	CodeCogsOther       Code = "COGS_OTHER"
)

// Operating expense codes.
const (
	CodeOpexPayroll          Code = "OPEX_PAYROLL"
	CodeOpexOwnerComp        Code = "OPEX_OWNER_COMP"
	CodeOpexRent             Code = "OPEX_RENT"
	CodeOpexMarketing        Code = "OPEX_MARKETING"
	CodeOpexInsurance        Code = "OPEX_INSURANCE"
	CodeOpexUtilities        Code = "OPEX_UTILITIES"
	CodeOpexSoftware         Code = "OPEX_SOFTWARE"
	CodeOpexProfessionalFees Code = "OPEX_PROFESSIONAL_FEES"
	CodeOpexRepairs          Code = "OPEX_REPAIRS"
	CodeOpexVehicle          Code = "OPEX_VEHICLE"
	CodeOpexTravel           Code = "OPEX_TRAVEL"
	CodeOpexOffice           Code = "OPEX_OFFICE"
	CodeOpexOther            Code = "OPEX_OTHER"
)

// Other income statement codes.
const (
	CodeDepreciation    Code = "DEPRECIATION"
	CodeAmortization    Code = "AMORTIZATION"
	CodeInterestExpense Code = "INTEREST_EXPENSE"
	CodeInterestIncome  Code = "INTEREST_INCOME"
	CodeIncomeTax       Code = "INCOME_TAX"
	CodeOtherIncome     Code = "OTHER_INCOME"
	CodeOtherExpense    Code = "OTHER_EXPENSE"
)

// Balance sheet codes.
const (
	CodeBsCash                  Code = "BS_CASH"
	CodeBsAccountsReceivable    Code = "BS_ACCOUNTS_RECEIVABLE"
	CodeBsInventory             Code = "BS_INVENTORY"
	CodeBsPrepaid               Code = "BS_PREPAID"
	CodeBsCurrentAssetOther     Code = "BS_CURRENT_ASSET_OTHER"
	CodeBsFixedAssets           Code = "BS_FIXED_ASSETS"
	CodeBsAccumDepreciation     Code = "BS_ACCUM_DEPRECIATION"
	CodeBsIntangibleAssets      Code = "BS_INTANGIBLE_ASSETS"
	CodeBsGoodwill              Code = "BS_GOODWILL"
	CodeBsAccountsPayable       Code = "BS_ACCOUNTS_PAYABLE"
	CodeBsCurrentLiabilityOther Code = "BS_CURRENT_LIABILITY_OTHER"
	CodeBsShortTermDebt         Code = "BS_SHORT_TERM_DEBT"
	CodeBsLongTermDebt          Code = "BS_LONG_TERM_DEBT"
	CodeBsRetainedEarnings      Code = "BS_RETAINED_EARNINGS"
	CodeBsOwnerEquity           Code = "BS_OWNER_EQUITY"
)

// CodeCategory groups codes by broad statement section, useful for display
// grouping and validation.
type CodeCategory string

const (
	CategoryRevenue              CodeCategory = "revenue"
	CategoryCogs                 CodeCategory = "cogs"
	CategoryOpex                 CodeCategory = "opex"
	CategoryOtherIncomeStatement CodeCategory = "other_income_statement"
	CategoryBalanceSheet         CodeCategory = "balance_sheet"
)

// CodeMeta is descriptive metadata about a canonical code: its display
// label, category, and which statement type it belongs to. Metadata is
// intentionally separate from the Code value itself so labels/descriptions
// can be revised without changing the stable identifier used in stored or
// transmitted data.
type CodeMeta struct {
	Code          Code          `json:"code"`
	Label         string        `json:"label"`
	Category      CodeCategory  `json:"category"`
	StatementType StatementType `json:"statement_type"`
}

// codeRegistry is the canonical source of taxonomy metadata. It is built
// once at init time and never mutated, so it is safe for concurrent read
// access from multiple goroutines without synchronization.
var codeRegistry = buildCodeRegistry()

func buildCodeRegistry() map[Code]CodeMeta {
	entries := []CodeMeta{
		{CodeRevProduct, "Product Revenue", CategoryRevenue, StatementIncomeStatement},
		{CodeRevService, "Service Revenue", CategoryRevenue, StatementIncomeStatement},
		{CodeRevRecurring, "Recurring Revenue", CategoryRevenue, StatementIncomeStatement},
		{CodeRevOther, "Other Revenue", CategoryRevenue, StatementIncomeStatement},

		{CodeCogsMaterial, "Materials", CategoryCogs, StatementIncomeStatement},
		{CodeCogsDirectLabor, "Direct Labor", CategoryCogs, StatementIncomeStatement},
		{CodeCogsFreight, "Freight", CategoryCogs, StatementIncomeStatement},
		{CodeCogsOther, "Other COGS", CategoryCogs, StatementIncomeStatement},

		{CodeOpexPayroll, "Payroll", CategoryOpex, StatementIncomeStatement},
		{CodeOpexOwnerComp, "Owner Compensation", CategoryOpex, StatementIncomeStatement},
		{CodeOpexRent, "Rent", CategoryOpex, StatementIncomeStatement},
		{CodeOpexMarketing, "Marketing", CategoryOpex, StatementIncomeStatement},
		{CodeOpexInsurance, "Insurance", CategoryOpex, StatementIncomeStatement},
		{CodeOpexUtilities, "Utilities", CategoryOpex, StatementIncomeStatement},
		{CodeOpexSoftware, "Software", CategoryOpex, StatementIncomeStatement},
		{CodeOpexProfessionalFees, "Professional Fees", CategoryOpex, StatementIncomeStatement},
		{CodeOpexRepairs, "Repairs & Maintenance", CategoryOpex, StatementIncomeStatement},
		{CodeOpexVehicle, "Vehicle Expense", CategoryOpex, StatementIncomeStatement},
		{CodeOpexTravel, "Travel", CategoryOpex, StatementIncomeStatement},
		{CodeOpexOffice, "Office Expense", CategoryOpex, StatementIncomeStatement},
		{CodeOpexOther, "Other Operating Expense", CategoryOpex, StatementIncomeStatement},

		{CodeDepreciation, "Depreciation", CategoryOtherIncomeStatement, StatementIncomeStatement},
		{CodeAmortization, "Amortization", CategoryOtherIncomeStatement, StatementIncomeStatement},
		{CodeInterestExpense, "Interest Expense", CategoryOtherIncomeStatement, StatementIncomeStatement},
		{CodeInterestIncome, "Interest Income", CategoryOtherIncomeStatement, StatementIncomeStatement},
		{CodeIncomeTax, "Income Tax", CategoryOtherIncomeStatement, StatementIncomeStatement},
		{CodeOtherIncome, "Other Income", CategoryOtherIncomeStatement, StatementIncomeStatement},
		{CodeOtherExpense, "Other Expense", CategoryOtherIncomeStatement, StatementIncomeStatement},

		{CodeBsCash, "Cash", CategoryBalanceSheet, StatementBalanceSheet},
		{CodeBsAccountsReceivable, "Accounts Receivable", CategoryBalanceSheet, StatementBalanceSheet},
		{CodeBsInventory, "Inventory", CategoryBalanceSheet, StatementBalanceSheet},
		{CodeBsPrepaid, "Prepaid Expenses", CategoryBalanceSheet, StatementBalanceSheet},
		{CodeBsCurrentAssetOther, "Other Current Assets", CategoryBalanceSheet, StatementBalanceSheet},
		{CodeBsFixedAssets, "Fixed Assets", CategoryBalanceSheet, StatementBalanceSheet},
		{CodeBsAccumDepreciation, "Accumulated Depreciation", CategoryBalanceSheet, StatementBalanceSheet},
		{CodeBsIntangibleAssets, "Intangible Assets", CategoryBalanceSheet, StatementBalanceSheet},
		{CodeBsGoodwill, "Goodwill", CategoryBalanceSheet, StatementBalanceSheet},
		{CodeBsAccountsPayable, "Accounts Payable", CategoryBalanceSheet, StatementBalanceSheet},
		{CodeBsCurrentLiabilityOther, "Other Current Liabilities", CategoryBalanceSheet, StatementBalanceSheet},
		{CodeBsShortTermDebt, "Short-Term Debt", CategoryBalanceSheet, StatementBalanceSheet},
		{CodeBsLongTermDebt, "Long-Term Debt", CategoryBalanceSheet, StatementBalanceSheet},
		{CodeBsRetainedEarnings, "Retained Earnings", CategoryBalanceSheet, StatementBalanceSheet},
		{CodeBsOwnerEquity, "Owner Equity", CategoryBalanceSheet, StatementBalanceSheet},
	}

	registry := make(map[Code]CodeMeta, len(entries))
	for _, e := range entries {
		registry[e.Code] = e
	}
	return registry
}

// LookupCode returns metadata for a canonical code and whether it is known.
func LookupCode(code Code) (CodeMeta, bool) {
	meta, ok := codeRegistry[code]
	return meta, ok
}

// IsValidCode reports whether code is a recognized canonical taxonomy code.
func IsValidCode(code Code) bool {
	_, ok := codeRegistry[code]
	return ok
}

// AllCodes returns metadata for every canonical code, sorted by code value
// for deterministic output.
func AllCodes() []CodeMeta {
	metas := make([]CodeMeta, 0, len(codeRegistry))
	for _, meta := range codeRegistry {
		metas = append(metas, meta)
	}
	sort.Slice(metas, func(i, j int) bool { return metas[i].Code < metas[j].Code })
	return metas
}

// CodesByCategory returns metadata for every canonical code in the given
// category, sorted by code value for deterministic output.
func CodesByCategory(category CodeCategory) []CodeMeta {
	var metas []CodeMeta
	for _, meta := range codeRegistry {
		if meta.Category == category {
			metas = append(metas, meta)
		}
	}
	sort.Slice(metas, func(i, j int) bool { return metas[i].Code < metas[j].Code })
	return metas
}

// sortPeriods sorts periods lexically in place. Extracted as a helper so the
// sorting convention is defined in one place (see FinancialDataset.Periods).
func sortPeriods(periods []Period) {
	sort.Slice(periods, func(i, j int) bool { return periods[i] < periods[j] })
}
