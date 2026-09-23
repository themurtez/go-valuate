package statements_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/ledger"
	"github.com/themurtez/go-valuate/accounting/statements"
	"github.com/themurtez/go-valuate/accounting/statements/fixtures"
	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/classification"
)

func TestBuild_ExplicitMapping_TakesPrecedenceOverSuggestion(t *testing.T) {
	// Deliberately map "4000" (Consulting Revenue) to a DIFFERENT valid
	// revenue code than the classifier would likely suggest, to prove the
	// explicit mapping is used verbatim even under
	// MappingSuggestDeterministic — task section 4's "explicit mappings
	// are authoritative... do not override explicit mappings using
	// account-name heuristics" rule.
	explicit := []statements.AccountMapping{
		{AccountID: "4000", FinancialCode: financial.CodeRevOther, StatementType: financial.StatementIncomeStatement, Source: statements.MappingSourceExplicit, Confirmed: true},
	}

	input := statements.Input{
		Source:    statements.SourceLedger,
		Chart:     fixtures.ServiceBusinessChart(),
		Entries:   fixtures.ServiceBusinessEntries(),
		Periods:   []financial.Period{"2025-01"},
		Mappings:  explicit,
		Selection: statements.SelectionIncomeOnly,
	}
	opts := statements.Options{
		Mapping: statements.MappingOptions{
			Mode:                 statements.MappingSuggestDeterministic,
			ClassificationConfig: classification.Config{Rules: classification.DefaultRules()},
		},
	}

	result := statements.Build(input, opts)

	var mr *statements.AccountMappingResult
	for i := range result.Mappings {
		if result.Mappings[i].AccountID == "4000" {
			mr = &result.Mappings[i]
		}
	}
	if mr == nil {
		t.Fatal("expected account 4000 in mapping results")
	}
	if mr.Mapping.Source != statements.MappingSourceExplicit {
		t.Errorf("expected MappingSourceExplicit, got %v", mr.Mapping.Source)
	}
	if mr.Mapping.FinancialCode != financial.CodeRevOther {
		t.Errorf("expected the explicit code REV_OTHER to win, got %v", mr.Mapping.FinancialCode)
	}

	item, ok := result.Dataset.ByCodeAndPeriod(financial.CodeRevOther, "2025-01")
	if !ok || item.Amount != 12000 {
		t.Errorf("dataset should reflect the explicit mapping: item=%+v ok=%v", item, ok)
	}
}

func TestBuild_DeterministicSuggestion_WhenModeEnabled(t *testing.T) {
	input := statements.Input{
		Source:    statements.SourceLedger,
		Chart:     fixtures.ServiceBusinessChart(),
		Entries:   fixtures.ServiceBusinessEntries(),
		Periods:   []financial.Period{"2025-01"},
		Selection: statements.SelectionIncomeOnly,
		// No explicit Mappings at all.
	}
	opts := statements.Options{
		Mapping: statements.MappingOptions{
			Mode:                 statements.MappingSuggestDeterministic,
			ClassificationConfig: classification.Config{Rules: classification.DefaultRules()},
		},
	}

	result := statements.Build(input, opts)

	// "Rent Expense" (6100) should be recognized by DefaultRules as an
	// operating expense with a strong deterministic rule (rentPhraseRule),
	// producing a suggestion rather than staying unmapped.
	var mr *statements.AccountMappingResult
	for i := range result.Mappings {
		if result.Mappings[i].AccountID == "6100" {
			mr = &result.Mappings[i]
		}
	}
	if mr == nil {
		t.Fatal("expected account 6100 in mapping results")
	}
	if mr.Mapping.Source != statements.MappingSourceDeterministicSuggestion {
		t.Fatalf("expected a deterministic suggestion for Rent Expense, got source=%v issues=%+v", mr.Mapping.Source, mr.Issues)
	}
	if mr.Mapping.FinancialCode != financial.CodeOpexRent {
		t.Errorf("expected suggested code OPEX_RENT, got %v", mr.Mapping.FinancialCode)
	}
	if mr.Mapping.Confirmed {
		t.Error("a suggestion must never be Confirmed automatically")
	}
	if mr.MappingStatus != statements.MappingStatusSuggested {
		t.Errorf("expected MappingStatusSuggested, got %v", mr.MappingStatus)
	}
}

func TestBuild_NoSuggestions_ExplicitOnlyByDefault(t *testing.T) {
	input := statements.Input{
		Source:    statements.SourceLedger,
		Chart:     fixtures.ServiceBusinessChart(),
		Entries:   fixtures.ServiceBusinessEntries(),
		Periods:   []financial.Period{"2025-01"},
		Selection: statements.SelectionIncomeOnly,
		// No explicit Mappings, and Options.Mapping.Mode left at its zero value.
	}

	result := statements.Build(input, statements.Options{})

	for _, mr := range result.Mappings {
		if mr.Mapping.Source == statements.MappingSourceDeterministicSuggestion {
			t.Fatalf("expected no suggestions under the default MappingExplicitOnly mode, got one for account %s", mr.AccountID)
		}
	}
}

func TestBuild_UnknownSuggestion_NeverSilentlyAccepted(t *testing.T) {
	// A chart with an account whose name gives the classifier no signal
	// at all — DefaultRules should not match it, so it must remain
	// MappingSourceUnmapped even under MappingSuggestDeterministic.
	chart := []ledger.Account{
		{ID: "9999", Number: "9999", Name: "Miscellaneous Widget Account", Type: ledger.AccountExpense, Currency: "USD", Active: true},
		{ID: "1000", Number: "1000", Name: "Cash", Type: ledger.AccountAsset, Currency: "USD", Active: true},
		{ID: "3000", Number: "3000", Name: "Equity", Type: ledger.AccountEquity, Currency: "USD", Active: true},
	}
	entries := []ledger.JournalEntry{
		{
			ID: "X-1", Date: "2025-01-01", Period: "2025-01", Status: ledger.StatusPosted,
			Lines: []ledger.JournalLine{
				{AccountID: "1000", Debit: 1000},
				{AccountID: "3000", Credit: 1000},
			},
		},
		{
			ID: "X-2", Date: "2025-01-02", Period: "2025-01", Status: ledger.StatusPosted,
			Lines: []ledger.JournalLine{
				{AccountID: "9999", Debit: 500},
				{AccountID: "1000", Credit: 500},
			},
		},
	}

	input := statements.Input{
		Source:    statements.SourceLedger,
		Chart:     chart,
		Entries:   entries,
		Periods:   []financial.Period{"2025-01"},
		Selection: statements.SelectionIncomeOnly,
	}
	opts := statements.Options{
		Mapping: statements.MappingOptions{
			Mode:                 statements.MappingSuggestDeterministic,
			ClassificationConfig: classification.Config{Rules: classification.DefaultRules()},
		},
	}

	result := statements.Build(input, opts)

	var mr *statements.AccountMappingResult
	for i := range result.Mappings {
		if result.Mappings[i].AccountID == "9999" {
			mr = &result.Mappings[i]
		}
	}
	if mr == nil {
		t.Fatal("expected account 9999 in mapping results")
	}
	if mr.Mapping.Source != statements.MappingSourceUnmapped {
		t.Errorf("expected MappingSourceUnmapped for an unrecognizable account, got %v (code=%v)", mr.Mapping.Source, mr.Mapping.FinancialCode)
	}
}

func TestBuild_AccountTypeIncompatibleSuggestion_Downgraded(t *testing.T) {
	// A REVENUE account whose name happens to look like a balance-sheet
	// term should never end up mapped to a balance-sheet code, even via
	// suggestion — account-type compatibility constrains suggestions too
	// (task section 5's "account type should constrain impossible
	// statement mappings").
	chart := []ledger.Account{
		{ID: "4900", Number: "4900", Name: "Cash Revenue", Type: ledger.AccountRevenue, Currency: "USD", Active: true},
		{ID: "1000", Number: "1000", Name: "Bank", Type: ledger.AccountAsset, Currency: "USD", Active: true},
		{ID: "3000", Number: "3000", Name: "Equity", Type: ledger.AccountEquity, Currency: "USD", Active: true},
	}
	entries := []ledger.JournalEntry{
		{
			ID: "Y-1", Date: "2025-01-01", Period: "2025-01", Status: ledger.StatusPosted,
			Lines: []ledger.JournalLine{
				{AccountID: "1000", Debit: 1000},
				{AccountID: "4900", Credit: 1000},
			},
		},
		{
			ID: "Y-2", Date: "2025-01-02", Period: "2025-01", Status: ledger.StatusPosted,
			Lines: []ledger.JournalLine{
				{AccountID: "3000", Debit: 100},
				{AccountID: "1000", Credit: 100},
			},
		},
	}

	input := statements.Input{
		Source:    statements.SourceLedger,
		Chart:     chart,
		Entries:   entries,
		Periods:   []financial.Period{"2025-01"},
		Selection: statements.SelectionIncomeOnly,
	}
	opts := statements.Options{
		Mapping: statements.MappingOptions{
			Mode:                 statements.MappingSuggestDeterministic,
			ClassificationConfig: classification.Config{Rules: classification.DefaultRules()},
		},
	}

	result := statements.Build(input, opts)

	for _, mr := range result.Mappings {
		if mr.AccountID != "4900" {
			continue
		}
		if mr.Mapping.Source == statements.MappingSourceDeterministicSuggestion {
			meta, _ := financial.LookupCode(mr.Mapping.FinancialCode)
			if meta.Category == financial.CategoryBalanceSheet {
				t.Fatalf("a REVENUE account must never be suggested a balance-sheet code, got %v", mr.Mapping.FinancialCode)
			}
		}
	}
}

func TestBuild_MultipleExplicitMappingsForSameAccount_Conflict(t *testing.T) {
	input := statements.Input{
		Source:  statements.SourceLedger,
		Chart:   fixtures.ServiceBusinessChart(),
		Entries: fixtures.ServiceBusinessEntries(),
		Periods: []financial.Period{"2025-01"},
		Mappings: []statements.AccountMapping{
			{AccountID: "4000", FinancialCode: financial.CodeRevService, StatementType: financial.StatementIncomeStatement, Source: statements.MappingSourceExplicit},
			{AccountID: "4000", FinancialCode: financial.CodeRevOther, StatementType: financial.StatementIncomeStatement, Source: statements.MappingSourceExplicit},
		},
		Selection: statements.SelectionIncomeOnly,
	}

	result := statements.Build(input, statements.Options{})

	var found bool
	for _, iss := range result.Issues {
		if iss.Code == statements.IssueMappingConflict && iss.AccountID == "4000" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected IssueMappingConflict for account 4000, got: %+v", result.Issues)
	}

	// Neither conflicting definition should silently win.
	for _, item := range result.Dataset.Items {
		if item.Code == financial.CodeRevService || item.Code == financial.CodeRevOther {
			for _, src := range item.Sources {
				if src.RowID == "4000" {
					t.Errorf("account 4000's conflicting mapping must not contribute to the dataset, found in code=%v", item.Code)
				}
			}
		}
	}
}
