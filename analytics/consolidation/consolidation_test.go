package consolidation

import (
	"testing"

	"github.com/themurtez/go-valuate/financial"
)

func TestCalculate_TwoEntitiesSameCurrency_FullConsolidation(t *testing.T) {
	result := Calculate(twoEntityUSDInput())

	if !result.Available {
		t.Fatalf("expected Available true, errors=%v", result.Errors)
	}
	if result.Mode != ModeFullConsolidation {
		t.Fatalf("expected default Mode to resolve to ModeFullConsolidation, got %q", result.Mode)
	}
	if result.Consolidated.Currency != "USD" {
		t.Fatalf("expected consolidated currency USD, got %q", result.Consolidated.Currency)
	}

	// Parent product revenue (1,000,000) + subsidiary "service" revenue
	// reused as CodeRevService (500,000) for 2024 = 1,500,000.
	revItem, ok := result.Consolidated.ByCodeAndPeriod(financial.CodeRevService, "2024")
	if !ok {
		t.Fatalf("expected a CodeRevService item for 2024")
	}
	if diffAbs(revItem.Amount, 500_000) > 0.0001 {
		t.Errorf("expected CodeRevService 2024 = 500000, got %v", revItem.Amount)
	}

	payroll2025, ok := result.Consolidated.ByCodeAndPeriod(financial.CodeOpexPayroll, "2025")
	if !ok {
		t.Fatalf("expected a CodeOpexPayroll item for 2025")
	}
	// parent 450,000 + sub 220,000 = 670,000.
	if diffAbs(payroll2025.Amount, 670_000) > 0.0001 {
		t.Errorf("expected CodeOpexPayroll 2025 = 670000 (parent 450000 + sub 220000), got %v", payroll2025.Amount)
	}

	if len(result.EntityContributions) != 2 {
		t.Fatalf("expected 2 entity contributions, got %d", len(result.EntityContributions))
	}
	if result.EntityContributions[0].EntityID != "parent" || result.EntityContributions[1].EntityID != "sub" {
		t.Errorf("expected EntityContributions sorted by EntityID (parent, sub), got %q, %q",
			result.EntityContributions[0].EntityID, result.EntityContributions[1].EntityID)
	}

	// Provenance: every consolidated item should carry one SourceRef per
	// contributing entity.
	for _, it := range result.Consolidated.Items {
		if len(it.Sources) == 0 {
			t.Errorf("item %s/%s has no provenance Sources", it.Code, it.Period)
		}
	}
}

func TestCalculate_Eliminations_RemovedFromBothSides(t *testing.T) {
	in := twoEntityUSDInput()
	in.Eliminations = managementFeeEliminations()
	result := Calculate(in)

	if !result.Available {
		t.Fatalf("expected Available true, errors=%v", result.Errors)
	}

	// Parent's CodeRevOther (intercompany fee) should be fully eliminated:
	// 50,000 - 50,000 = 0 for 2024. Since it's the only contributor to that
	// cell, buildConsolidatedDataset should show it as a zero-amount item
	// still present via the totals map.
	revOther2024, ok := result.Consolidated.ByCodeAndPeriod(financial.CodeRevOther, "2024")
	if !ok {
		t.Fatalf("expected a CodeRevOther item for 2024 even after full elimination (net zero, not absent)")
	}
	if diffAbs(revOther2024.Amount, 0) > 0.0001 {
		t.Errorf("expected CodeRevOther 2024 = 0 after eliminating the full intercompany fee, got %v", revOther2024.Amount)
	}

	opexOther2025, ok := result.Consolidated.ByCodeAndPeriod(financial.CodeOpexOther, "2025")
	if !ok {
		t.Fatalf("expected a CodeOpexOther item for 2025")
	}
	if diffAbs(opexOther2025.Amount, 0) > 0.0001 {
		t.Errorf("expected CodeOpexOther 2025 = 0 after eliminating the full intercompany fee, got %v", opexOther2025.Amount)
	}

	if len(result.EliminationsApplied) != 4 {
		t.Fatalf("expected 4 eliminations applied, got %d: %+v", len(result.EliminationsApplied), result.EliminationsApplied)
	}
}

func TestCalculate_Elimination_UnknownEntity_SkippedAndWarned(t *testing.T) {
	in := twoEntityUSDInput()
	in.Eliminations = []Elimination{
		{EntityID: "ghost", Code: financial.CodeRevOther, Period: "2024", Amount: 1000},
	}
	result := Calculate(in)

	if !result.Available {
		t.Fatalf("expected Available true, errors=%v", result.Errors)
	}
	if len(result.EliminationsApplied) != 0 {
		t.Fatalf("expected the unknown-entity elimination not to be applied, got %+v", result.EliminationsApplied)
	}
	if !hasIssue(result.Warnings, IssueUnknownEliminationEntity) {
		t.Errorf("expected IssueUnknownEliminationEntity warning, got %+v", result.Warnings)
	}
}

func TestCalculate_Elimination_TargetNotFound_WarnedButStillApplied(t *testing.T) {
	in := twoEntityUSDInput()
	// parentUSD has no CodeCogsMaterial item at all for any period.
	in.Eliminations = []Elimination{
		{EntityID: "parent", Code: financial.CodeCogsMaterial, Period: "2024", Amount: 1000},
	}
	result := Calculate(in)

	if !result.Available {
		t.Fatalf("expected Available true, errors=%v", result.Errors)
	}
	if !hasIssueForEntityPeriod(result.Warnings, IssueEliminationTargetNotFound, "parent", "2024") {
		t.Errorf("expected IssueEliminationTargetNotFound for parent/2024, got %+v", result.Warnings)
	}
	// Still recorded as "applied" since the entity itself is known/selected.
	if len(result.EliminationsApplied) != 1 {
		t.Errorf("expected the elimination to still be recorded in EliminationsApplied, got %+v", result.EliminationsApplied)
	}
	// It must not have fabricated a new CodeCogsMaterial item out of thin air.
	if _, ok := result.Consolidated.ByCodeAndPeriod(financial.CodeCogsMaterial, "2024"); ok {
		t.Errorf("expected no CodeCogsMaterial item to be fabricated by a no-op elimination")
	}
}

func TestCalculate_Elimination_PeriodOutOfScope_WarnedAndInert(t *testing.T) {
	in := twoEntityUSDInput()
	in.Periods = []financial.Period{"2024"} // 2025 is deliberately out of scope
	in.Eliminations = []Elimination{
		{EntityID: "parent", Code: financial.CodeRevOther, Period: "2025", Amount: 999_999},
	}
	result := Calculate(in)

	if !result.Available {
		t.Fatalf("expected Available true, errors=%v", result.Errors)
	}
	if !hasIssueForEntityPeriod(result.Warnings, IssueEliminationPeriodOutOfScope, "parent", "2025") {
		t.Errorf("expected IssueEliminationPeriodOutOfScope for parent/2025, got %+v", result.Warnings)
	}
	// It is still recorded as "applied" (known, selected entity)...
	if len(result.EliminationsApplied) != 1 {
		t.Fatalf("expected the out-of-scope elimination to still be recorded in EliminationsApplied, got %+v", result.EliminationsApplied)
	}
	// ...but since 2025 is out of scope entirely, no 2025 item exists to
	// have been affected by it one way or the other.
	if _, ok := result.Consolidated.ByCodeAndPeriod(financial.CodeRevOther, "2025"); ok {
		t.Errorf("expected no 2025 items in Result.Consolidated at all, since 2025 is outside Input.Periods")
	}
}

func TestCalculate_AllSelectedEntitiesEmptyCurrency_Unavailable(t *testing.T) {
	in := Input{
		Entities: []EntityDataset{
			{EntityID: "parent", Dataset: financial.FinancialDataset{Currency: "", Items: []financial.NormalizedItem{item(financial.CodeRevProduct, "2024", 1000)}}},
		},
		Periods: twoYearPeriods(),
	}
	result := Calculate(in)

	if result.Available {
		t.Fatalf("expected Available false when no selected entity has a non-empty currency and no TargetCurrency is set")
	}
	if !hasIssue(result.Errors, IssueMissingTargetCurrency) {
		t.Errorf("expected IssueMissingTargetCurrency error, got %+v", result.Errors)
	}
	if result.Consolidated.Currency != "" || len(result.Consolidated.Items) != 0 {
		t.Errorf("expected a zero-value Consolidated dataset when Available is false, got %+v", result.Consolidated)
	}
}

func TestCalculate_DifferentPeriods_MissingPeriodReported(t *testing.T) {
	// Subsidiary only has 2024 data; parent has both 2024 and 2025.
	sub := subsidiaryEUR()
	sub.Currency = "USD"
	sub.Items = []financial.NormalizedItem{item(financial.CodeRevService, "2024", 500_000)}

	in := Input{
		Entities: []EntityDataset{
			{EntityID: "parent", Dataset: parentUSD()},
			{EntityID: "sub", Dataset: sub},
		},
		Periods: twoYearPeriods(),
	}
	result := Calculate(in)

	if !result.Available {
		t.Fatalf("expected Available true, errors=%v", result.Errors)
	}
	if !hasIssueForEntityPeriod(result.Warnings, IssuePeriodMissingForEntity, "sub", "2025") {
		t.Errorf("expected IssuePeriodMissingForEntity for sub/2025, got %+v", result.Warnings)
	}

	// The consolidated 2025 revenue should be parent-only (no sub
	// contribution), not a fabricated zero for sub.
	rev2025, ok := result.Consolidated.ByCodeAndPeriod(financial.CodeRevProduct, "2025")
	if !ok {
		t.Fatalf("expected CodeRevProduct for 2025")
	}
	if diffAbs(rev2025.Amount, 1_200_000) > 0.0001 {
		t.Errorf("expected CodeRevProduct 2025 = 1200000 (parent only), got %v", rev2025.Amount)
	}

	// Sub should contribute nothing for 2025 at all.
	for _, ec := range result.EntityContributions {
		if ec.EntityID != "sub" {
			continue
		}
		for _, it := range ec.Items {
			if it.Period == "2025" {
				t.Errorf("expected sub to contribute nothing for 2025, found %+v", it)
			}
		}
	}
}

func TestCalculate_DifferentCurrencies_WithRate(t *testing.T) {
	in := Input{
		Entities: []EntityDataset{
			{EntityID: "parent", Dataset: parentUSD()},
			{EntityID: "sub", Dataset: subsidiaryEUR()},
		},
		Periods:       twoYearPeriods(),
		CurrencyRates: eurToUSDRates(),
		Policy:        Policy{TargetCurrency: "USD"},
	}
	result := Calculate(in)

	if !result.Available {
		t.Fatalf("expected Available true, errors=%v", result.Errors)
	}
	if result.Consolidated.Currency != "USD" {
		t.Fatalf("expected consolidated currency USD, got %q", result.Consolidated.Currency)
	}

	// Sub's 2024 service revenue (500,000 EUR) at rate 1.10 = 550,000 USD;
	// parent has no CodeRevService, so the consolidated total is exactly
	// the converted sub figure.
	rev2024, ok := result.Consolidated.ByCodeAndPeriod(financial.CodeRevService, "2024")
	if !ok {
		t.Fatalf("expected CodeRevService for 2024")
	}
	if diffAbs(rev2024.Amount, 550_000) > 0.0001 {
		t.Errorf("expected CodeRevService 2024 = 550000 (500000 EUR * 1.10), got %v", rev2024.Amount)
	}

	if len(result.CurrencyConversions) != 2 {
		t.Fatalf("expected 2 currency conversions (one per period), got %d: %+v", len(result.CurrencyConversions), result.CurrencyConversions)
	}
	for _, c := range result.CurrencyConversions {
		if c.EntityID != "sub" || c.FromCurrency != "EUR" || c.ToCurrency != "USD" {
			t.Errorf("unexpected conversion entry: %+v", c)
		}
	}
}

func TestCalculate_DifferentCurrencies_NoRate_ExcludedAndWarned(t *testing.T) {
	in := Input{
		Entities: []EntityDataset{
			{EntityID: "parent", Dataset: parentUSD()},
			{EntityID: "sub", Dataset: subsidiaryEUR()},
		},
		Periods: twoYearPeriods(),
		Policy:  Policy{TargetCurrency: "USD"},
		// No CurrencyRates supplied at all.
	}
	result := Calculate(in)

	if !result.Available {
		t.Fatalf("expected Available true, errors=%v", result.Errors)
	}

	// Sub contributed nothing since no rate was available for either period.
	for _, ec := range result.EntityContributions {
		if ec.EntityID == "sub" {
			t.Errorf("expected sub to contribute nothing with no currency rate available, got %+v", ec)
		}
	}
	if !hasIssueForEntityPeriod(result.Warnings, IssueMissingCurrencyRate, "sub", "2024") {
		t.Errorf("expected IssueMissingCurrencyRate for sub/2024, got %+v", result.Warnings)
	}
	if !hasIssueForEntityPeriod(result.Warnings, IssueMissingCurrencyRate, "sub", "2025") {
		t.Errorf("expected IssueMissingCurrencyRate for sub/2025, got %+v", result.Warnings)
	}

	// Consolidated should be parent-only figures (no EUR items ever
	// converted in unconverted).
	rev2024, ok := result.Consolidated.ByCodeAndPeriod(financial.CodeRevProduct, "2024")
	if !ok || diffAbs(rev2024.Amount, 1_000_000) > 0.0001 {
		t.Errorf("expected CodeRevProduct 2024 = 1000000 (parent only), got %v (ok=%v)", rev2024.Amount, ok)
	}
}

func TestCalculate_MultipleCurrencies_NoTargetCurrency_Unavailable(t *testing.T) {
	in := Input{
		Entities: []EntityDataset{
			{EntityID: "parent", Dataset: parentUSD()},
			{EntityID: "sub", Dataset: subsidiaryEUR()},
		},
		Periods: twoYearPeriods(),
		// No Policy.TargetCurrency, and entities disagree on currency.
	}
	result := Calculate(in)

	if result.Available {
		t.Fatalf("expected Available false when currencies disagree and no TargetCurrency is set")
	}
	if !hasIssue(result.Errors, IssueMissingTargetCurrency) {
		t.Errorf("expected IssueMissingTargetCurrency error, got %+v", result.Errors)
	}
}

func TestCalculate_OwnershipWeighted_ExplicitMode(t *testing.T) {
	in := twoEntityUSDInput()
	in.Mode = ModeOwnershipWeighted
	in.Entities[0].OwnershipPercent = ptr(1.0) // parent, wholly owned
	in.Entities[1].OwnershipPercent = ptr(0.6) // sub, 60% owned

	result := Calculate(in)

	if !result.Available {
		t.Fatalf("expected Available true, errors=%v", result.Errors)
	}
	if result.Mode != ModeOwnershipWeighted {
		t.Fatalf("expected Mode ModeOwnershipWeighted, got %q", result.Mode)
	}

	// Sub's 2024 "service" revenue (reused as CodeRevService, 500,000) at
	// 60% ownership = 300,000; parent contributes nothing to this code.
	rev2024, ok := result.Consolidated.ByCodeAndPeriod(financial.CodeRevService, "2024")
	if !ok {
		t.Fatalf("expected CodeRevService for 2024")
	}
	if diffAbs(rev2024.Amount, 300_000) > 0.0001 {
		t.Errorf("expected CodeRevService 2024 = 300000 (500000 * 0.6), got %v", rev2024.Amount)
	}
}

func TestCalculate_OwnershipWeighted_MissingPercent_EntityExcluded(t *testing.T) {
	in := twoEntityUSDInput()
	in.Mode = ModeOwnershipWeighted
	in.Entities[0].OwnershipPercent = ptr(1.0)
	// sub's OwnershipPercent left nil.

	result := Calculate(in)

	if !result.Available {
		t.Fatalf("expected Available true, errors=%v", result.Errors)
	}
	for _, ec := range result.EntityContributions {
		if ec.EntityID == "sub" {
			t.Errorf("expected sub excluded with missing OwnershipPercent under ModeOwnershipWeighted, got %+v", ec)
		}
	}
	if !hasIssueForEntityPeriod(result.Warnings, IssueMissingOwnershipPercent, "sub", "") {
		t.Errorf("expected IssueMissingOwnershipPercent for sub, got %+v", result.Warnings)
	}
}

func TestCalculate_OwnershipWeighted_InvalidPercent_EntityExcluded(t *testing.T) {
	in := twoEntityUSDInput()
	in.Mode = ModeOwnershipWeighted
	in.Entities[0].OwnershipPercent = ptr(1.0)
	in.Entities[1].OwnershipPercent = ptr(1.5) // out of [0, 1]

	result := Calculate(in)

	if !result.Available {
		t.Fatalf("expected Available true, errors=%v", result.Errors)
	}
	if !hasIssueForEntityPeriod(result.Warnings, IssueInvalidOwnershipPercent, "sub", "") {
		t.Errorf("expected IssueInvalidOwnershipPercent for sub, got %+v", result.Warnings)
	}
}

func TestCalculate_FullConsolidation_IgnoresOwnershipPercent(t *testing.T) {
	// Even with a low OwnershipPercent supplied, ModeFullConsolidation
	// (the default) must consolidate 100% of every selected entity.
	in := twoEntityUSDInput()
	in.Entities[1].OwnershipPercent = ptr(0.1)

	result := Calculate(in)

	payroll2025, ok := result.Consolidated.ByCodeAndPeriod(financial.CodeOpexPayroll, "2025")
	if !ok || diffAbs(payroll2025.Amount, 670_000) > 0.0001 {
		t.Errorf("expected full consolidation regardless of OwnershipPercent: CodeOpexPayroll 2025 = 670000, got %v (ok=%v)", payroll2025.Amount, ok)
	}
}

func TestCalculate_NoEntities_Unavailable(t *testing.T) {
	result := Calculate(Input{Periods: twoYearPeriods()})
	if result.Available {
		t.Fatalf("expected Available false with no entities")
	}
	if !hasIssue(result.Errors, IssueNoEntities) {
		t.Errorf("expected IssueNoEntities, got %+v", result.Errors)
	}
}

func TestCalculate_NoPeriods_Unavailable(t *testing.T) {
	in := twoEntityUSDInput()
	in.Periods = nil
	result := Calculate(in)
	if result.Available {
		t.Fatalf("expected Available false with no periods")
	}
	if !hasIssue(result.Errors, IssueNoPeriods) {
		t.Errorf("expected IssueNoPeriods, got %+v", result.Errors)
	}
}

func TestCalculate_DuplicateEntityID_Unavailable(t *testing.T) {
	in := Input{
		Entities: []EntityDataset{
			{EntityID: "parent", Dataset: parentUSD()},
			{EntityID: "parent", Dataset: parentUSD()},
		},
		Periods: twoYearPeriods(),
	}
	result := Calculate(in)
	if result.Available {
		t.Fatalf("expected Available false with duplicate EntityID")
	}
	if !hasIssue(result.Errors, IssueDuplicateEntityID) {
		t.Errorf("expected IssueDuplicateEntityID, got %+v", result.Errors)
	}
}

func TestCalculate_SelectedEntityIDs_NarrowsParticipation(t *testing.T) {
	in := twoEntityUSDInput()
	in.Policy = Policy{SelectedEntityIDs: []string{"parent"}}

	result := Calculate(in)
	if !result.Available {
		t.Fatalf("expected Available true, errors=%v", result.Errors)
	}
	if len(result.EntityContributions) != 1 || result.EntityContributions[0].EntityID != "parent" {
		t.Fatalf("expected only parent selected, got %+v", result.EntityContributions)
	}

	// Sub's CodeRevService should not appear at all.
	if _, ok := result.Consolidated.ByCodeAndPeriod(financial.CodeRevService, "2024"); ok {
		t.Errorf("expected no CodeRevService item when sub is not selected")
	}
}

func TestCalculate_UnknownSelectedEntity_WarnedAndIgnored(t *testing.T) {
	in := twoEntityUSDInput()
	in.Policy = Policy{SelectedEntityIDs: []string{"parent", "sub", "ghost"}}

	result := Calculate(in)
	if !result.Available {
		t.Fatalf("expected Available true, errors=%v", result.Errors)
	}
	if !hasIssue(result.Warnings, IssueUnknownSelectedEntity) {
		t.Errorf("expected IssueUnknownSelectedEntity warning, got %+v", result.Warnings)
	}
	if len(result.EntityContributions) != 2 {
		t.Errorf("expected exactly parent+sub to contribute, got %+v", result.EntityContributions)
	}
}

func TestCalculate_DoesNotMutateInput(t *testing.T) {
	in := twoEntityUSDInput()
	in.Eliminations = managementFeeEliminations()

	originalParentLen := len(in.Entities[0].Dataset.Items)
	originalSubLen := len(in.Entities[1].Dataset.Items)

	_ = Calculate(in)

	if len(in.Entities[0].Dataset.Items) != originalParentLen {
		t.Errorf("Calculate mutated parent's Dataset.Items length")
	}
	if len(in.Entities[1].Dataset.Items) != originalSubLen {
		t.Errorf("Calculate mutated sub's Dataset.Items length")
	}
	if in.Entities[0].Dataset.Items[0].Amount != parentUSD().Items[0].Amount {
		t.Errorf("Calculate mutated parent's Dataset.Items contents")
	}
}

func TestHasErrors(t *testing.T) {
	if HasErrors(nil) {
		t.Errorf("HasErrors(nil) should be false")
	}
	if HasErrors([]Issue{{Severity: SeverityWarning}}) {
		t.Errorf("HasErrors with only warnings should be false")
	}
	if !HasErrors([]Issue{{Severity: SeverityWarning}, {Severity: SeverityError}}) {
		t.Errorf("HasErrors with an error present should be true")
	}
}

// hasIssue reports whether issues contains an entry with the given Code.
func hasIssue(issues []Issue, code IssueCode) bool {
	for _, i := range issues {
		if i.Code == code {
			return true
		}
	}
	return false
}

// hasIssueForEntityPeriod reports whether issues contains an entry with
// the given Code, EntityID, and Period (Period "" matches any period).
func hasIssueForEntityPeriod(issues []Issue, code IssueCode, entityID string, period financial.Period) bool {
	for _, i := range issues {
		if i.Code != code || i.EntityID != entityID {
			continue
		}
		if period != "" && i.Period != period {
			continue
		}
		return true
	}
	return false
}
