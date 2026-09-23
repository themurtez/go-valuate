package statements_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/themurtez/go-valuate/accounting/ledger"
	"github.com/themurtez/go-valuate/accounting/statements"
	"github.com/themurtez/go-valuate/accounting/statements/fixtures"
	"github.com/themurtez/go-valuate/financial"
)

// deepCopyViaJSON round-trips v through JSON to produce an independent
// snapshot for later reflect.DeepEqual comparison. Safe to use as a
// mutation-detection baseline for any type in this package's public API,
// since every one of them is required to be JSON-safe (task section 41,
// independently verified by json_test.go) — this helper does not itself
// prove JSON-safety, it only leans on it.
func deepCopyViaJSON(t *testing.T, v any, out any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("deepCopyViaJSON: marshal failed: %v", err)
	}
	if err := json.Unmarshal(b, out); err != nil {
		t.Fatalf("deepCopyViaJSON: unmarshal failed: %v", err)
	}
}

// TestBuild_DoesNotMutateInput proves Build does not mutate any part of
// its Input — task section 39's explicit deep-snapshot requirement.
func TestBuild_DoesNotMutateInput(t *testing.T) {
	chart := fixtures.ServiceBusinessChart()
	entries := fixtures.ServiceBusinessEntries()
	mappings := fixtures.ServiceBusinessMappings()
	periods := []financial.Period{"2025-01"}

	var chartBefore []ledger.Account
	var entriesBefore []ledger.JournalEntry
	var mappingsBefore []statements.AccountMapping
	var periodsBefore []financial.Period
	deepCopyViaJSON(t, chart, &chartBefore)
	deepCopyViaJSON(t, entries, &entriesBefore)
	deepCopyViaJSON(t, mappings, &mappingsBefore)
	deepCopyViaJSON(t, periods, &periodsBefore)

	input := statements.Input{
		Source:    statements.SourceLedger,
		Chart:     chart,
		Entries:   entries,
		Periods:   periods,
		Mappings:  mappings,
		Selection: statements.SelectionBoth,
	}
	_ = statements.Build(input, statements.Options{})

	if !reflect.DeepEqual(chart, chartBefore) {
		t.Error("Build mutated input.Chart")
	}
	if !reflect.DeepEqual(entries, entriesBefore) {
		t.Error("Build mutated input.Entries")
	}
	if !reflect.DeepEqual(mappings, mappingsBefore) {
		t.Error("Build mutated input.Mappings")
	}
	if !reflect.DeepEqual(periods, periodsBefore) {
		t.Error("Build mutated input.Periods")
	}
}

// TestBuild_DoesNotMutateImportedTrialBalance mirrors the above for the
// SourceImportedTrialBalance path.
func TestBuild_DoesNotMutateImportedTrialBalance(t *testing.T) {
	chart := fixtures.ServiceBusinessChart()
	ledgerChart := ledger.BuildChartOfAccounts(chart)
	tb := ledger.NormalizeTrialBalance(ledger.TrialBalanceInput{
		Period: "2025-01",
		Lines: []ledger.TrialBalanceInputLine{
			{AccountID: "1000", Debit: 51000},
			{AccountID: "3000", Credit: 50000},
			{AccountID: "4000", Credit: 12000},
			{AccountID: "6000", Debit: 8000},
			{AccountID: "6100", Debit: 3000},
		},
	}, ledgerChart, 0.01)

	tbMap := map[financial.Period]ledger.NormalizedTrialBalance{"2025-01": tb}

	var tbBefore ledger.NormalizedTrialBalance
	deepCopyViaJSON(t, tb, &tbBefore)

	input := statements.Input{
		Source:                statements.SourceImportedTrialBalance,
		ImportedChart:         chart,
		ImportedTrialBalances: tbMap,
		Periods:               []financial.Period{"2025-01"},
		Mappings:              fixtures.ServiceBusinessMappings(),
		Selection:             statements.SelectionIncomeOnly,
	}
	_ = statements.Build(input, statements.Options{})

	if !reflect.DeepEqual(tbMap["2025-01"], tbBefore) {
		t.Error("Build mutated the imported trial balance")
	}
}

// TestResolveTemplate_DoesNotMutateInputs proves ResolveTemplate does not
// mutate its inputs.
func TestResolveTemplate_DoesNotMutateInputs(t *testing.T) {
	chart := ledger.BuildChartOfAccounts(fixtures.ServiceBusinessChart())
	template := fixtures.ServiceBusinessMappingTemplate()

	var templateBefore statements.MappingTemplate
	deepCopyViaJSON(t, template, &templateBefore)

	_, _ = statements.ResolveTemplate(template, chart)

	if !reflect.DeepEqual(template, templateBefore) {
		t.Error("ResolveTemplate mutated its template input")
	}
}
