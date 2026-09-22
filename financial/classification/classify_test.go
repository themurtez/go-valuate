package classification

import (
	"testing"

	"github.com/themurtez/go-valuate/financial"
)

func rawRow(id, label, parent string) financial.RawLineItem {
	return financial.RawLineItem{
		ID:            id,
		StatementType: financial.StatementIncomeStatement,
		Label:         label,
		ParentLabel:   parent,
		Values:        map[financial.Period]float64{"2025": 1000},
	}
}

// --- Exact aliases -----------------------------------------------------

func TestClassify_ExactAliases(t *testing.T) {
	cfg := Config{
		AliasLayers: []AliasLayer{
			{Name: "global", Aliases: []Alias{
				{Label: "Advertising", Code: financial.CodeOpexMarketing},
				{Label: "Advertising & Promotion", Code: financial.CodeOpexMarketing},
				{Label: "Marketing Expense", Code: financial.CodeOpexMarketing},
				{Label: "Bank Charges", Code: financial.CodeOpexProfessionalFees},
				{Label: "Merchant Fees", Code: financial.CodeOpexProfessionalFees},
			}},
		},
	}

	tests := []struct {
		label string
		want  financial.Code
	}{
		{"Advertising", financial.CodeOpexMarketing},
		{"Advertising & Promotion", financial.CodeOpexMarketing},
		{"Marketing Expense", financial.CodeOpexMarketing},
		{"Bank Charges", financial.CodeOpexProfessionalFees},
		{"Merchant Fees", financial.CodeOpexProfessionalFees},
	}

	for _, tt := range tests {
		row := rawRow("row-1", tt.label, "Operating Expenses")
		result := Classify(row, cfg)
		if result.Code != tt.want {
			t.Errorf("Classify(%q).Code = %v, want %v", tt.label, result.Code, tt.want)
		}
		if result.Source != SourceAlias {
			t.Errorf("Classify(%q).Source = %v, want %v", tt.label, result.Source, SourceAlias)
		}
		if result.Confidence != ConfidenceAlias {
			t.Errorf("Classify(%q).Confidence = %v, want %v", tt.label, result.Confidence, ConfidenceAlias)
		}
		if result.ReviewRequired {
			t.Errorf("Classify(%q).ReviewRequired = true, want false (alias confidence exceeds default threshold)", tt.label)
		}
	}
}

func TestClassify_ExplicitMappingBeatsAlias(t *testing.T) {
	cfg := Config{
		Explicits: []Explicit{
			{RowID: "row-1", Code: financial.CodeOpexOther},
		},
		AliasLayers: []AliasLayer{
			{Name: "global", Aliases: []Alias{
				{Label: "Advertising", Code: financial.CodeOpexMarketing},
			}},
		},
	}

	row := rawRow("row-1", "Advertising", "")
	result := Classify(row, cfg)
	if result.Code != financial.CodeOpexOther {
		t.Errorf("Code = %v, want %v (explicit should beat alias)", result.Code, financial.CodeOpexOther)
	}
	if result.Source != SourceExplicit {
		t.Errorf("Source = %v, want %v", result.Source, SourceExplicit)
	}
	if result.Confidence != ConfidenceExplicit {
		t.Errorf("Confidence = %v, want %v", result.Confidence, ConfidenceExplicit)
	}
}

// --- Context-sensitive cases --------------------------------------------

func TestClassify_LaborUnderCostOfSales(t *testing.T) {
	cfg := Config{Rules: DefaultRules()}
	row := rawRow("row-1", "Field Installers", "Cost of Sales")
	result := Classify(row, cfg)

	if result.Code != financial.CodeCogsDirectLabor {
		t.Errorf("Code = %v, want %v", result.Code, financial.CodeCogsDirectLabor)
	}
	if result.Source != SourceContextRule {
		t.Errorf("Source = %v, want %v", result.Source, SourceContextRule)
	}
}

func TestClassify_LaborUnderOperatingExpenses(t *testing.T) {
	cfg := Config{Rules: DefaultRules()}
	row := rawRow("row-1", "Field Installers", "Operating Expenses")
	result := Classify(row, cfg)

	if result.Code != financial.CodeOpexPayroll {
		t.Errorf("Code = %v, want %v", result.Code, financial.CodeOpexPayroll)
	}
	if result.Source != SourceContextRule {
		t.Errorf("Source = %v, want %v", result.Source, SourceContextRule)
	}
}

func TestClassify_LaborWithoutParentIsAmbiguous(t *testing.T) {
	// Without any parent context, the labor rule must not guess.
	cfg := Config{Rules: DefaultRules()}
	row := rawRow("row-1", "Field Installers", "")
	result := Classify(row, cfg)

	if result.Code == financial.CodeCogsDirectLabor || result.Code == financial.CodeOpexPayroll {
		t.Errorf("expected no confident guess without parent context, got %v (source=%v)", result.Code, result.Source)
	}
}

func TestClassify_DepreciationExpense(t *testing.T) {
	cfg := Config{Rules: DefaultRules()}
	row := rawRow("row-1", "Depreciation Expense", "Operating Expenses")
	result := Classify(row, cfg)

	if result.Code != financial.CodeDepreciation {
		t.Errorf("Code = %v, want %v", result.Code, financial.CodeDepreciation)
	}
}

func TestClassify_InterestExpense(t *testing.T) {
	cfg := Config{Rules: DefaultRules()}
	row := rawRow("row-1", "Interest Expense", "Other Expenses")
	result := Classify(row, cfg)

	if result.Code != financial.CodeInterestExpense {
		t.Errorf("Code = %v, want %v", result.Code, financial.CodeInterestExpense)
	}
}

func TestClassify_InterestIncomeDistinctFromExpense(t *testing.T) {
	cfg := Config{Rules: DefaultRules()}
	row := rawRow("row-1", "Interest Income", "Other Income")
	result := Classify(row, cfg)

	if result.Code != financial.CodeInterestIncome {
		t.Errorf("Code = %v, want %v", result.Code, financial.CodeInterestIncome)
	}
}

func TestClassify_AccountsReceivableOnBalanceSheet(t *testing.T) {
	cfg := Config{Rules: DefaultRules()}
	row := financial.RawLineItem{
		ID:            "row-1",
		StatementType: financial.StatementBalanceSheet,
		Label:         "Accounts Receivable",
		Values:        map[financial.Period]float64{"2025": 1000},
	}
	result := Classify(row, cfg)

	if result.Code != financial.CodeBsAccountsReceivable {
		t.Errorf("Code = %v, want %v", result.Code, financial.CodeBsAccountsReceivable)
	}
}

func TestClassify_AccountsReceivableRequiresBalanceSheet(t *testing.T) {
	// The same label on an income statement should not match the
	// balance-sheet-specific rule.
	cfg := Config{Rules: DefaultRules()}
	row := rawRow("row-1", "Accounts Receivable", "")
	result := Classify(row, cfg)

	if result.Code == financial.CodeBsAccountsReceivable {
		t.Errorf("expected accounts-receivable rule to require a balance sheet statement type")
	}
}

// --- Precedence: valuation > client > account > global ------------------

func TestClassify_AliasPrecedence_ValuationOverridesAll(t *testing.T) {
	cfg := Config{
		AliasLayers: []AliasLayer{
			{Name: "global", Aliases: []Alias{{Label: "Field Installers", Code: financial.CodeOpexOther}}},
			{Name: "account", Aliases: []Alias{{Label: "Field Installers", Code: financial.CodeCogsDirectLabor}}},
			{Name: "client", Aliases: []Alias{{Label: "Field Installers", Code: financial.CodeOpexPayroll}}},
			{Name: "valuation", Aliases: []Alias{{Label: "Field Installers", Code: financial.CodeOpexOwnerComp}}},
		},
	}
	row := rawRow("row-1", "Field Installers", "")
	result := Classify(row, cfg)

	if result.Code != financial.CodeOpexOwnerComp {
		t.Errorf("Code = %v, want %v (valuation should win)", result.Code, financial.CodeOpexOwnerComp)
	}
}

func TestClassify_AliasPrecedence_ClientOverridesAccountAndGlobal(t *testing.T) {
	cfg := Config{
		AliasLayers: []AliasLayer{
			{Name: "global", Aliases: []Alias{{Label: "Shop Supplies", Code: financial.CodeOpexOther}}},
			{Name: "account", Aliases: []Alias{{Label: "Shop Supplies", Code: financial.CodeOpexOffice}}},
			{Name: "client", Aliases: []Alias{{Label: "Shop Supplies", Code: financial.CodeCogsMaterial}}},
			{Name: "valuation", Aliases: nil},
		},
	}
	row := rawRow("row-1", "Shop Supplies", "")
	result := Classify(row, cfg)

	if result.Code != financial.CodeCogsMaterial {
		t.Errorf("Code = %v, want %v (client should win over account/global)", result.Code, financial.CodeCogsMaterial)
	}
}

func TestClassify_AliasPrecedence_AccountOverridesGlobal(t *testing.T) {
	cfg := Config{
		AliasLayers: []AliasLayer{
			{Name: "global", Aliases: []Alias{{Label: "Merchant Fees", Code: financial.CodeOpexOther}}},
			{Name: "account", Aliases: []Alias{{Label: "Merchant Fees", Code: financial.CodeOpexProfessionalFees}}},
		},
	}
	row := rawRow("row-1", "Merchant Fees", "")
	result := Classify(row, cfg)

	if result.Code != financial.CodeOpexProfessionalFees {
		t.Errorf("Code = %v, want %v (account should win over global)", result.Code, financial.CodeOpexProfessionalFees)
	}
}

func TestClassify_AliasPrecedence_GlobalFallback(t *testing.T) {
	cfg := Config{
		AliasLayers: []AliasLayer{
			{Name: "global", Aliases: []Alias{{Label: "Bank Charges", Code: financial.CodeOpexProfessionalFees}}},
		},
	}
	row := rawRow("row-1", "Bank Charges", "")
	result := Classify(row, cfg)

	if result.Code != financial.CodeOpexProfessionalFees {
		t.Errorf("Code = %v, want %v (global should apply with no overrides)", result.Code, financial.CodeOpexProfessionalFees)
	}
}

// --- UNKNOWN behavior -----------------------------------------------------

func TestClassify_UnknownForAmbiguousLabels(t *testing.T) {
	cfg := Config{Rules: DefaultRules()}
	ambiguous := []string{"Misc", "General", "Adjustment", "Other"}

	for _, label := range ambiguous {
		row := rawRow("row-1", label, "Operating Expenses")
		result := Classify(row, cfg)
		if !result.IsUnknown() {
			t.Errorf("Classify(%q) = %v (source=%v), want SourceUnknown", label, result.Code, result.Source)
		}
		if result.Confidence != ConfidenceUnknown {
			t.Errorf("Classify(%q).Confidence = %v, want 0", label, result.Confidence)
		}
		if !result.ReviewRequired {
			t.Errorf("Classify(%q).ReviewRequired = false, want true", label)
		}
		if result.Code != "" {
			t.Errorf("Classify(%q).Code = %v, want empty for UNKNOWN", label, result.Code)
		}
	}
}

func TestClassify_UnknownDoesNotFallBackToOpexOther(t *testing.T) {
	cfg := Config{Rules: DefaultRules()}
	row := rawRow("row-1", "Miscellaneous Adjustment", "Operating Expenses")
	result := Classify(row, cfg)

	if result.Code == financial.CodeOpexOther {
		t.Error("UNKNOWN classification must never silently fall back to OPEX_OTHER")
	}
	if !result.IsUnknown() {
		t.Errorf("expected UNKNOWN, got code=%v source=%v", result.Code, result.Source)
	}
}

func TestClassify_EmptyConfigYieldsUnknown(t *testing.T) {
	row := rawRow("row-1", "Advertising", "Operating Expenses")
	result := Classify(row, Config{})

	if !result.IsUnknown() {
		t.Errorf("expected UNKNOWN with zero-value Config, got %v", result.Code)
	}
}

// --- Totals/subtotals -----------------------------------------------------

func TestClassify_TotalRevenueIsTotal(t *testing.T) {
	row := rawRow("row-3", "Total Revenue", "")
	result := Classify(row, Config{Rules: DefaultRules()})

	if result.Status != financial.RowStatusTotal {
		t.Errorf("Status = %v, want %v", result.Status, financial.RowStatusTotal)
	}
	if result.Code != "" {
		t.Errorf("Code = %v, want empty for a total row", result.Code)
	}
	if result.Source != SourceStructural {
		t.Errorf("Source = %v, want %v", result.Source, SourceStructural)
	}
}

func TestClassify_TotalOperatingExpensesIsSubtotal(t *testing.T) {
	row := rawRow("row-50", "Total Operating Expenses", "Operating Expenses")
	result := Classify(row, Config{Rules: DefaultRules()})

	if result.Status != financial.RowStatusSubtotal {
		t.Errorf("Status = %v, want %v", result.Status, financial.RowStatusSubtotal)
	}
	if result.Code != "" {
		t.Errorf("Code = %v, want empty for a subtotal row", result.Code)
	}
}

func TestClassify_NetIncomeIsTotal(t *testing.T) {
	row := rawRow("row-99", "Net Income", "")
	result := Classify(row, Config{Rules: DefaultRules()})

	if result.Status != financial.RowStatusTotal {
		t.Errorf("Status = %v, want %v", result.Status, financial.RowStatusTotal)
	}
}

func TestClassify_StructuralDetectionBeatsAliasesAndRules(t *testing.T) {
	// Even if an alias exists that happens to match a total-like label, the
	// structural check must win, since normalization/aggregation
	// correctness depends on totals never being double-counted as if they
	// were an ordinary account.
	cfg := Config{
		AliasLayers: []AliasLayer{
			{Name: "global", Aliases: []Alias{{Label: "Total Operating Expenses", Code: financial.CodeOpexOther}}},
		},
		Rules: DefaultRules(),
	}
	row := rawRow("row-50", "Total Operating Expenses", "Operating Expenses")
	result := Classify(row, cfg)

	if result.Source != SourceStructural {
		t.Errorf("Source = %v, want %v", result.Source, SourceStructural)
	}
	if result.Code != "" {
		t.Error("expected no code for a structural row even when an alias exists")
	}
}

// --- RawLineItem.Kind precedence ---------------------------------------
//
// These tests exercise the fix for the documented "Gross Profit" gap (see
// the README's former "Known deterministic ingestion gaps" entry): before
// RawLineItem.Kind existed, Classify's own structuralTotalTokens check
// (only "total"/"subtotal"/"net") did not recognize "Gross Profit" as
// structural, so it fell through to ordinary classification. Now, when an
// upstream adapter supplies Kind directly, that read wins outright.

func TestClassify_KindSubtotalWinsEvenWhenLabelHeuristicWouldNotMatch(t *testing.T) {
	// "Gross Profit" contains none of structuralTotalTokens
	// ("total"/"subtotal"/"net"), so without Kind supplied, Classify's own
	// label heuristic would NOT recognize it as structural (see the
	// contrasting case in TestClassify_GrossProfitWithoutKindFallsThroughToOrdinaryClassification
	// below) — proving raw.Kind, not the label, is what makes this pass.
	row := rawRow("row-1", "Gross Profit", "")
	row.Kind = financial.RowKindSubtotal

	result := Classify(row, Config{Rules: DefaultRules()})

	if result.Source != SourceStructural {
		t.Errorf("Source = %v, want %v", result.Source, SourceStructural)
	}
	if result.Status != financial.RowStatusSubtotal {
		t.Errorf("Status = %v, want %v", result.Status, financial.RowStatusSubtotal)
	}
	if result.Code != "" {
		t.Errorf("Code = %v, want empty for a structural row", result.Code)
	}
	if result.Kind != financial.RowKindSubtotal {
		t.Errorf("Kind = %v, want %v", result.Kind, financial.RowKindSubtotal)
	}
	if result.ReviewRequired {
		t.Error("ReviewRequired = true, want false for a structural result")
	}
}

func TestClassify_GrossProfitWithoutKindFallsThroughToOrdinaryClassification(t *testing.T) {
	// Confirms the premise of the test above: with Kind left at its zero
	// value (no upstream signal), "Gross Profit" is NOT recognized as
	// structural by Classify's own label heuristic alone — it has no
	// "total"/"subtotal"/"net" token. This is the exact historical bug:
	// callers relying on ingestion's structural read (via Kind) now avoid
	// it, but a bare hand-built RawLineItem with no Kind reproduces the old
	// behavior unchanged, which is the documented backward-compatibility
	// contract for the zero value.
	row := rawRow("row-1", "Gross Profit", "")
	result := Classify(row, Config{Rules: DefaultRules()})

	if result.Source == SourceStructural {
		t.Error("expected Gross Profit with no Kind supplied to NOT be classified as structural (label heuristic alone does not recognize it) — if this now passes, the label heuristic changed and this test's premise needs updating")
	}
}

func TestClassify_KindTotalWinsOverLabelHeuristic(t *testing.T) {
	// A label with no total/subtotal/net token, and no special-cased
	// phrase, but an explicit upstream Kind of total.
	row := rawRow("row-1", "Bottom Line Result", "")
	row.Kind = financial.RowKindTotal

	result := Classify(row, Config{Rules: DefaultRules()})

	if result.Status != financial.RowStatusTotal {
		t.Errorf("Status = %v, want %v", result.Status, financial.RowStatusTotal)
	}
	if result.Source != SourceStructural {
		t.Errorf("Source = %v, want %v", result.Source, SourceStructural)
	}
}

func TestClassify_KindHeadingProducesIgnoredStatusAndNoCode(t *testing.T) {
	row := rawRow("row-1", "Operating Expenses", "")
	row.Kind = financial.RowKindHeading
	row.Values = map[financial.Period]float64{} // headings carry no amounts

	result := Classify(row, Config{Rules: DefaultRules()})

	if result.Status != financial.RowStatusIgnored {
		t.Errorf("Status = %v, want %v", result.Status, financial.RowStatusIgnored)
	}
	if result.Source != SourceStructural {
		t.Errorf("Source = %v, want %v", result.Source, SourceStructural)
	}
	if result.Code != "" {
		t.Errorf("Code = %v, want empty for a heading row", result.Code)
	}
	if result.Kind != financial.RowKindHeading {
		t.Errorf("Kind = %v, want %v", result.Kind, financial.RowKindHeading)
	}
	if result.Reason == "" {
		t.Error("expected a non-empty Reason explaining the heading classification")
	}
	if financial.RowStatusIgnored != result.Status {
		t.Fatalf("sanity: RowStatusIgnored constant mismatch")
	}
}

func TestClassify_KindNormalDoesNotShortCircuitOrdinaryClassification(t *testing.T) {
	// RowKindNormal is the zero value and means "no upstream signal" — an
	// ordinary account row explicitly carrying it (as opposed to simply
	// omitting Kind) must classify exactly as if Kind were never set.
	cfg := Config{
		AliasLayers: []AliasLayer{
			{Name: "global", Aliases: []Alias{{Label: "Advertising & Promotion", Code: financial.CodeOpexMarketing}}},
		},
	}
	row := rawRow("row-1", "Advertising & Promotion", "")
	row.Kind = financial.RowKindNormal

	result := Classify(row, cfg)

	if result.Source != SourceAlias {
		t.Errorf("Source = %v, want %v", result.Source, SourceAlias)
	}
	if result.Code != financial.CodeOpexMarketing {
		t.Errorf("Code = %v, want %v", result.Code, financial.CodeOpexMarketing)
	}
}

// --- Batch classification ---------------------------------------------

func TestClassifyBatch_PreservesOrderAndCoversEveryRow(t *testing.T) {
	cfg := Config{Rules: DefaultRules()}
	rows := []financial.RawLineItem{
		rawRow("row-1", "Advertising", ""),
		rawRow("row-2", "Depreciation Expense", ""),
		rawRow("row-3", "Misc", ""),
		rawRow("row-4", "Total Revenue", ""),
		rawRow("row-5", "Interest Expense", ""),
	}

	results := ClassifyBatch(rows, cfg)

	if len(results) != len(rows) {
		t.Fatalf("got %d results, want %d (one per input row)", len(results), len(rows))
	}
	for i, r := range results {
		if r.RowID != rows[i].ID {
			t.Errorf("result[%d].RowID = %q, want %q (order must match input)", i, r.RowID, rows[i].ID)
		}
	}

	if results[2].Source != SourceUnknown {
		t.Errorf("results[2] (Misc) source = %v, want SourceUnknown", results[2].Source)
	}
	if results[3].Status != financial.RowStatusTotal {
		t.Errorf("results[3] (Total Revenue) status = %v, want total", results[3].Status)
	}
}

func TestClassifyBatch_Empty(t *testing.T) {
	results := ClassifyBatch(nil, Config{Rules: DefaultRules()})
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty input, got %d", len(results))
	}
}

// --- Result conversion ---------------------------------------------------

func TestResult_ToMappedLineItem(t *testing.T) {
	row := rawRow("row-1", "Advertising", "Operating Expenses")
	cfg := Config{AliasLayers: []AliasLayer{
		{Name: "global", Aliases: []Alias{{Label: "Advertising", Code: financial.CodeOpexMarketing}}},
	}}
	result := Classify(row, cfg)
	mapped := result.ToMappedLineItem(row)

	if mapped.SourceID != row.ID {
		t.Errorf("SourceID = %v, want %v", mapped.SourceID, row.ID)
	}
	if mapped.Code != financial.CodeOpexMarketing {
		t.Errorf("Code = %v, want %v", mapped.Code, financial.CodeOpexMarketing)
	}
	if mapped.Status != financial.RowStatusNormal {
		t.Errorf("Status = %v, want normal", mapped.Status)
	}
	if mapped.Values["2025"] != 1000 {
		t.Errorf("Values[2025] = %v, want 1000", mapped.Values["2025"])
	}

	// Mutating the returned map must not affect the original row.
	mapped.Values["2025"] = 999
	if row.Values["2025"] != 1000 {
		t.Error("ToMappedLineItem must not alias the original row's Values map")
	}
}

func TestClassify_DoesNotMutateRawInput(t *testing.T) {
	row := rawRow("row-1", "Advertising", "Operating Expenses")
	original := row
	_ = Classify(row, Config{Rules: DefaultRules()})

	if row.Label != original.Label || row.ParentLabel != original.ParentLabel {
		t.Error("Classify must not mutate its input row")
	}
}

// --- Alternatives -----------------------------------------------------

func TestClassify_AlternativesOrderedByConfidence(t *testing.T) {
	cfg := Config{Rules: DefaultRules()}
	// "Field Installers" under Cost of Sales matches the labor rule
	// strongly; it should not also fire weaker phrase rules for the same
	// label in this case, but we verify alternatives are sorted whenever
	// more than one rule matches by using a label that hits multiple
	// phrase rules.
	row := rawRow("row-1", "Office Supplies", "Operating Expenses")
	result := Classify(row, cfg)

	for i := 1; i < len(result.Alternatives); i++ {
		if result.Alternatives[i].Confidence > result.Alternatives[i-1].Confidence {
			t.Errorf("Alternatives not sorted by descending confidence: %+v", result.Alternatives)
		}
	}
}

func TestClassify_ReviewRequiredBelowThreshold(t *testing.T) {
	cfg := Config{Rules: DefaultRules(), ReviewThreshold: 0.90}
	row := rawRow("row-1", "Office Supplies", "")
	result := Classify(row, cfg)

	if result.Confidence >= cfg.ReviewThreshold && result.ReviewRequired {
		t.Errorf("ReviewRequired should be false when confidence %v >= threshold %v", result.Confidence, cfg.ReviewThreshold)
	}
	if result.Confidence < cfg.ReviewThreshold && !result.ReviewRequired {
		t.Errorf("ReviewRequired should be true when confidence %v < threshold %v", result.Confidence, cfg.ReviewThreshold)
	}
}
