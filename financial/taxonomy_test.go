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
