package financial

import "testing"

func TestLookupCode_KnownCode(t *testing.T) {
	meta, ok := LookupCode(CodeOpexMarketing)
	if !ok {
		t.Fatal("expected OPEX_MARKETING to be a known code")
	}
	if meta.Label == "" {
		t.Error("expected non-empty label")
	}
	if meta.Category != CategoryOpex {
		t.Errorf("category = %q, want %q", meta.Category, CategoryOpex)
	}
	if meta.StatementType != StatementIncomeStatement {
		t.Errorf("statement type = %q, want %q", meta.StatementType, StatementIncomeStatement)
	}
}

func TestLookupCode_UnknownCode(t *testing.T) {
	_, ok := LookupCode(Code("NOT_A_REAL_CODE"))
	if ok {
		t.Error("expected unknown code to return ok=false")
	}
}

func TestIsValidCode(t *testing.T) {
	if !IsValidCode(CodeBsCash) {
		t.Error("expected BS_CASH to be valid")
	}
	if IsValidCode(Code("")) {
		t.Error("expected empty code to be invalid")
	}
}

func TestAllCodes_NoDuplicatesAndStableCount(t *testing.T) {
	metas := AllCodes()
	seen := make(map[Code]bool)
	for _, m := range metas {
		if seen[m.Code] {
			t.Errorf("duplicate code in registry: %s", m.Code)
		}
		seen[m.Code] = true
	}
	// 4 revenue + 4 cogs + 13 opex + 7 other-income-statement + 15 balance sheet = 43
	const want = 43
	if len(metas) != want {
		t.Errorf("AllCodes() returned %d codes, want %d", len(metas), want)
	}
}

func TestCodesByCategory(t *testing.T) {
	revenue := CodesByCategory(CategoryRevenue)
	if len(revenue) != 4 {
		t.Errorf("revenue codes = %d, want 4", len(revenue))
	}
	for _, m := range revenue {
		if m.Category != CategoryRevenue {
			t.Errorf("got category %q in revenue lookup", m.Category)
		}
	}
}

// allDeclaredCodes lists every Code-typed exported constant declared in
// taxonomy.go, hand-maintained so a code that's declared but never added
// to buildCodeRegistry's entries slice is caught — TestAllCodes_NoDuplicatesAndStableCount
// alone only proves the registry has no internal duplicates, not that
// every declared constant made it into the registry at all.
var allDeclaredCodes = []Code{
	CodeRevProduct, CodeRevService, CodeRevRecurring, CodeRevOther,
	CodeCogsMaterial, CodeCogsDirectLabor, CodeCogsFreight, CodeCogsOther,
	CodeOpexPayroll, CodeOpexOwnerComp, CodeOpexRent, CodeOpexMarketing,
	CodeOpexInsurance, CodeOpexUtilities, CodeOpexSoftware, CodeOpexProfessionalFees,
	CodeOpexRepairs, CodeOpexVehicle, CodeOpexTravel, CodeOpexOffice, CodeOpexOther,
	CodeDepreciation, CodeAmortization, CodeInterestExpense, CodeInterestIncome,
	CodeIncomeTax, CodeOtherIncome, CodeOtherExpense,
	CodeBsCash, CodeBsAccountsReceivable, CodeBsInventory, CodeBsPrepaid,
	CodeBsCurrentAssetOther, CodeBsFixedAssets, CodeBsAccumDepreciation,
	CodeBsIntangibleAssets, CodeBsGoodwill, CodeBsAccountsPayable,
	CodeBsCurrentLiabilityOther, CodeBsShortTermDebt, CodeBsLongTermDebt,
	CodeBsRetainedEarnings, CodeBsOwnerEquity,
}

func TestTaxonomy_EveryDeclaredConstantIsRegistered(t *testing.T) {
	if len(allDeclaredCodes) != len(AllCodes()) {
		t.Fatalf("allDeclaredCodes has %d entries but AllCodes() returns %d; a declared Code constant may be missing from buildCodeRegistry, or this test's list is stale", len(allDeclaredCodes), len(AllCodes()))
	}
	for _, code := range allDeclaredCodes {
		if !IsValidCode(code) {
			t.Errorf("declared constant %s is not registered in codeRegistry", code)
		}
	}
}

func TestTaxonomy_EveryCodeHasValidMetadata(t *testing.T) {
	validCategories := map[CodeCategory]bool{
		CategoryRevenue: true, CategoryCogs: true, CategoryOpex: true,
		CategoryOtherIncomeStatement: true, CategoryBalanceSheet: true,
	}
	validStatements := map[StatementType]bool{
		StatementIncomeStatement: true, StatementBalanceSheet: true, StatementCashFlow: true,
	}
	for _, meta := range AllCodes() {
		if meta.Label == "" {
			t.Errorf("%s: empty Label", meta.Code)
		}
		if !validCategories[meta.Category] {
			t.Errorf("%s: invalid Category %q", meta.Code, meta.Category)
		}
		if !validStatements[meta.StatementType] {
			t.Errorf("%s: invalid StatementType %q", meta.Code, meta.StatementType)
		}
	}
}

func TestTaxonomyVersion_NotEmpty(t *testing.T) {
	if TaxonomyVersion == "" {
		t.Error("TaxonomyVersion constant must not be empty")
	}
}
