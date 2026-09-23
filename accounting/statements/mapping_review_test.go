package statements_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/statements"
	"github.com/themurtez/go-valuate/accounting/statements/fixtures"
	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/classification"
)

// TestMappingReview_UnmappedThenSuggestThenConfirmThenRebuild
// demonstrates task section 45's full mapping-review cycle end to end,
// entirely through this package's own domain types — no UI, no
// persistence, exactly as the task requires:
//
//  1. an account starts unmapped (no explicit mapping supplied),
//  2. a deterministic suggestion is produced for it,
//  3. a caller (standing in for a future application's review screen)
//     "confirms" the suggestion by promoting it into an explicit,
//     Confirmed mapping,
//  4. Build is called again with that confirmed mapping added to
//     Input.Mappings,
//  5. the account is now MAPPED and the dataset is complete for it.
func TestMappingReview_UnmappedThenSuggestThenConfirmThenRebuild(t *testing.T) {
	chart := fixtures.ServiceBusinessChart()
	entries := fixtures.ServiceBusinessEntries()
	periods := []financial.Period{"2025-01"}

	// Step 1 & 2: build with NO explicit mapping for "6100" (Rent
	// Expense), but with suggestions enabled — it should come back
	// SUGGESTED, not UNMAPPED, and specifically NOT Confirmed.
	initialInput := statements.Input{
		Source:    statements.SourceLedger,
		Chart:     chart,
		Entries:   entries,
		Periods:   periods,
		Selection: statements.SelectionIncomeOnly,
		Mappings:  nil, // nothing mapped at all yet
	}
	suggestOpts := statements.Options{
		Mapping: statements.MappingOptions{
			Mode:                 statements.MappingSuggestDeterministic,
			ClassificationConfig: classification.Config{Rules: classification.DefaultRules()},
		},
	}

	firstResult := statements.Build(initialInput, suggestOpts)

	var rentMapping *statements.AccountMappingResult
	for i := range firstResult.Mappings {
		if firstResult.Mappings[i].AccountID == "6100" {
			rentMapping = &firstResult.Mappings[i]
		}
	}
	if rentMapping == nil {
		t.Fatal("expected account 6100 in the first build's mapping results")
	}
	if rentMapping.MappingStatus != statements.MappingStatusSuggested {
		t.Fatalf("expected account 6100 to start SUGGESTED, got %v", rentMapping.MappingStatus)
	}
	if rentMapping.Mapping.Confirmed {
		t.Fatal("a fresh suggestion must never already be Confirmed")
	}
	suggestedCode := rentMapping.Mapping.FinancialCode
	if suggestedCode == "" {
		t.Fatal("expected a non-empty suggested financial code")
	}

	// Before confirmation, the dataset must NOT silently include the
	// suggestion as if it were accepted — a suggestion is still just a
	// proposal (task section 5's "suggestions are not confirmations").
	// Rent Expense should therefore be absent from the very first
	// result's dataset only if the caller had disabled suggestions; but
	// this package's own rule (mapping.go) is that a SUGGESTED mapping
	// DOES already contribute to the dataset once produced (it is a
	// resolved mapping, just not yet human-confirmed) — assert that
	// behavior explicitly here, since it is the actual contract, rather
	// than assuming the stricter "excluded until confirmed" alternative
	// design this test's own comment could otherwise be misread as
	// requiring.
	if _, ok := firstResult.Dataset.ByCodeAndPeriod(suggestedCode, "2025-01"); !ok {
		t.Fatalf("expected the suggested code %v to already contribute to the dataset (a suggestion is a resolved mapping, not excluded pending confirmation)", suggestedCode)
	}

	// Step 3: the caller confirms the suggestion — promotes it to an
	// explicit, Confirmed mapping (standing in for a human clicking
	// "accept" in a future review screen).
	confirmed := rentMapping.Mapping
	confirmed.Source = statements.MappingSourceExplicit
	confirmed.Confirmed = true

	// Step 4: rebuild with the now-confirmed mapping supplied explicitly,
	// suggestions disabled this time (the review cycle for THIS account
	// is done; a real caller would typically still enable suggestions for
	// every OTHER still-unmapped account, but disabling here isolates
	// what this test is checking).
	rebuildInput := initialInput
	rebuildInput.Mappings = []statements.AccountMapping{confirmed}

	secondResult := statements.Build(rebuildInput, statements.Options{})

	var rentMappingAfter *statements.AccountMappingResult
	for i := range secondResult.Mappings {
		if secondResult.Mappings[i].AccountID == "6100" {
			rentMappingAfter = &secondResult.Mappings[i]
		}
	}
	if rentMappingAfter == nil {
		t.Fatal("expected account 6100 in the second build's mapping results")
	}

	// Step 5: now MAPPED (not merely SUGGESTED), Confirmed, and the
	// dataset reflects it under the confirmed code with identical
	// figures to what the suggestion had already produced.
	if rentMappingAfter.MappingStatus != statements.MappingStatusMapped {
		t.Errorf("expected MappingStatusMapped after confirmation, got %v", rentMappingAfter.MappingStatus)
	}
	if rentMappingAfter.Mapping.Source != statements.MappingSourceExplicit {
		t.Errorf("expected MappingSourceExplicit after confirmation, got %v", rentMappingAfter.Mapping.Source)
	}
	if !rentMappingAfter.Mapping.Confirmed {
		t.Error("expected Confirmed == true after confirmation")
	}
	if rentMappingAfter.Mapping.FinancialCode != suggestedCode {
		t.Errorf("expected the confirmed code to match the original suggestion (%v), got %v", suggestedCode, rentMappingAfter.Mapping.FinancialCode)
	}

	item, ok := secondResult.Dataset.ByCodeAndPeriod(suggestedCode, "2025-01")
	if !ok {
		t.Fatalf("expected code %v present in the rebuilt dataset", suggestedCode)
	}
	if item.Amount != 3000 { // Rent Expense's actual balance in ServiceBusinessEntries
		t.Errorf("rebuilt dataset amount = %v, want 3000", item.Amount)
	}
}
