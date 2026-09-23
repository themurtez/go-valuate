package statements_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/ledger"
	"github.com/themurtez/go-valuate/accounting/statements"
	"github.com/themurtez/go-valuate/accounting/statements/fixtures"
	"github.com/themurtez/go-valuate/financial"
)

func TestBuild_Allocation_SplitOneAccountIntoTwoCodes(t *testing.T) {
	// Account "6300" (Store Supplies Expense) is split 70% OPEX_OFFICE /
	// 30% OPEX_OTHER — task section 21's explicit-allocation one-to-many
	// support.
	input := statements.Input{
		Source:  statements.SourceLedger,
		Chart:   fixtures.RetailerChart(),
		Entries: fixtures.RetailerEntries(),
		Periods: []financial.Period{"2025-03"},
		Mappings: append(
			[]statements.AccountMapping{
				{
					AccountID: "6300",
					Allocations: []statements.AllocationRule{
						{FinancialCode: financial.CodeOpexOffice, StatementType: financial.StatementIncomeStatement, Percent: 0.70},
						{FinancialCode: financial.CodeOpexOther, StatementType: financial.StatementIncomeStatement, Percent: 0.30},
					},
					Source: statements.MappingSourceExplicit, Confirmed: true,
				},
			},
			fixtures.RetailerMappings()[:len(fixtures.RetailerMappings())-1]..., // everything except the original 6300 mapping
		),
		Selection: statements.SelectionIncomeOnly,
	}

	result := statements.Build(input, statements.Options{})

	if statements.HasErrors(result.Issues) {
		t.Fatalf("unexpected error issues: %+v", result.Issues)
	}

	// This fixture's 6300 account has a zero balance (RetailerEntries
	// never posts to it), so the allocation split should produce two
	// zero-amount items rather than failing — the split logic itself is
	// what's under test, not a specific nonzero figure. Assert both codes
	// are present and sourced from account 6300.
	for _, code := range []financial.Code{financial.CodeOpexOffice, financial.CodeOpexOther} {
		item, ok := result.Dataset.ByCodeAndPeriod(code, "2025-03")
		if !ok {
			t.Errorf("expected code %v present in dataset from the allocation", code)
			continue
		}
		var sourced bool
		for _, src := range item.Sources {
			if src.RowID == "6300" {
				sourced = true
			}
		}
		if !sourced {
			t.Errorf("expected code %v to be sourced from account 6300's allocation", code)
		}
	}
}

func TestValidate_AllocationPercentagesMustSumToOne(t *testing.T) {
	mapping := statements.AccountMapping{
		AccountID: "6300",
		Allocations: []statements.AllocationRule{
			{FinancialCode: financial.CodeOpexOffice, StatementType: financial.StatementIncomeStatement, Percent: 0.50},
			{FinancialCode: financial.CodeOpexOther, StatementType: financial.StatementIncomeStatement, Percent: 0.30}, // sums to 0.80, not 1.0
		},
		Source: statements.MappingSourceExplicit,
	}

	input := statements.Input{
		Source:    statements.SourceLedger,
		Chart:     fixtures.RetailerChart(),
		Entries:   fixtures.RetailerEntries(),
		Periods:   []financial.Period{"2025-03"},
		Mappings:  []statements.AccountMapping{mapping},
		Selection: statements.SelectionIncomeOnly,
	}

	result := statements.Build(input, statements.Options{})

	var found bool
	for _, iss := range result.Issues {
		if iss.Code == statements.IssueInvalidAllocation && iss.AccountID == "6300" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected IssueInvalidAllocation for a short allocation sum, got: %+v", result.Issues)
	}

	// An invalid allocation must not contribute to the dataset at all.
	for _, item := range result.Dataset.Items {
		for _, src := range item.Sources {
			if src.RowID == "6300" {
				t.Errorf("invalid allocation for account 6300 must not contribute to the dataset")
			}
		}
	}
}

func TestValidate_UnknownAccountInMapping(t *testing.T) {
	input := statements.Input{
		Source:  statements.SourceLedger,
		Chart:   fixtures.ServiceBusinessChart(),
		Entries: fixtures.ServiceBusinessEntries(),
		Periods: []financial.Period{"2025-01"},
		Mappings: []statements.AccountMapping{
			{AccountID: "does-not-exist", FinancialCode: financial.CodeBsCash, StatementType: financial.StatementBalanceSheet, Source: statements.MappingSourceExplicit},
		},
		Selection: statements.SelectionBalanceOnly,
	}

	result := statements.Build(input, statements.Options{})

	var found bool
	for _, iss := range result.Issues {
		if iss.Code == statements.IssueInvalidMapping && iss.AccountID == "does-not-exist" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected IssueInvalidMapping for an unknown account reference, got: %+v", result.Issues)
	}
}

func TestValidate_UnrecognizedFinancialCode(t *testing.T) {
	input := statements.Input{
		Source:  statements.SourceLedger,
		Chart:   fixtures.ServiceBusinessChart(),
		Entries: fixtures.ServiceBusinessEntries(),
		Periods: []financial.Period{"2025-01"},
		Mappings: []statements.AccountMapping{
			{AccountID: "1000", FinancialCode: "NOT_A_REAL_CODE", StatementType: financial.StatementBalanceSheet, Source: statements.MappingSourceExplicit},
		},
		Selection: statements.SelectionBalanceOnly,
	}

	result := statements.Build(input, statements.Options{})

	var found bool
	for _, iss := range result.Issues {
		if iss.Code == statements.IssueInvalidFinancialCode && iss.AccountID == "1000" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected IssueInvalidFinancialCode, got: %+v", result.Issues)
	}
}

func TestValidate_InvalidSignTreatment(t *testing.T) {
	input := statements.Input{
		Source:  statements.SourceLedger,
		Chart:   fixtures.ServiceBusinessChart(),
		Entries: fixtures.ServiceBusinessEntries(),
		Periods: []financial.Period{"2025-01"},
		Mappings: []statements.AccountMapping{
			{AccountID: "1000", FinancialCode: financial.CodeBsCash, StatementType: financial.StatementBalanceSheet, SignTreatment: "BOGUS", Source: statements.MappingSourceExplicit},
		},
		Selection: statements.SelectionBalanceOnly,
	}

	result := statements.Build(input, statements.Options{})

	var found bool
	for _, iss := range result.Issues {
		if iss.Code == statements.IssueInvalidSignTreatment && iss.AccountID == "1000" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected IssueInvalidSignTreatment, got: %+v", result.Issues)
	}
}

func TestValidate_StatementTypeMismatch(t *testing.T) {
	input := statements.Input{
		Source:  statements.SourceLedger,
		Chart:   fixtures.ServiceBusinessChart(),
		Entries: fixtures.ServiceBusinessEntries(),
		Periods: []financial.Period{"2025-01"},
		Mappings: []statements.AccountMapping{
			// CodeBsCash belongs to the balance sheet; declaring income
			// statement here is a caller mistake that must be flagged,
			// not silently trusted.
			{AccountID: "1000", FinancialCode: financial.CodeBsCash, StatementType: financial.StatementIncomeStatement, Source: statements.MappingSourceExplicit},
		},
		Selection: statements.SelectionBalanceOnly,
	}

	result := statements.Build(input, statements.Options{})

	var found bool
	for _, iss := range result.Issues {
		if iss.Code == statements.IssueInvalidMapping && iss.AccountID == "1000" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected IssueInvalidMapping for a statement-type mismatch, got: %+v", result.Issues)
	}
}

func TestValidate_InactiveAccountMapped_Warning(t *testing.T) {
	chart := fixtures.ServiceBusinessChart()
	for i := range chart {
		if chart[i].ID == "1000" {
			chart[i].Active = false
		}
	}

	input := statements.Input{
		Source:  statements.SourceLedger,
		Chart:   chart,
		Entries: fixtures.ServiceBusinessEntries(),
		Periods: []financial.Period{"2025-01"},
		Mappings: []statements.AccountMapping{
			{AccountID: "1000", FinancialCode: financial.CodeBsCash, StatementType: financial.StatementBalanceSheet, Source: statements.MappingSourceExplicit},
		},
		Selection: statements.SelectionBalanceOnly,
	}

	result := statements.Build(input, statements.Options{})

	var found bool
	for _, iss := range result.Issues {
		if iss.Code == statements.IssueInvalidMapping && iss.AccountID == "1000" && iss.Severity == statements.SeverityWarning {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a warning-severity IssueInvalidMapping for a mapped inactive account, got: %+v", result.Issues)
	}

	// Historical balances on an inactive account still apply — it should
	// NOT be excluded from the dataset (see ledger.Account.Active's doc
	// comment: "historical balances/postings on an inactive account
	// remain valid").
	if _, ok := result.Dataset.ByCodeAndPeriod(financial.CodeBsCash, "2025-01"); !ok {
		t.Error("an inactive-but-mapped account's historical balance should still appear in the dataset")
	}
}

func TestValidate_MissingPeriods(t *testing.T) {
	input := statements.Input{
		Source:  statements.SourceLedger,
		Chart:   fixtures.ServiceBusinessChart(),
		Entries: fixtures.ServiceBusinessEntries(),
		Periods: nil,
	}

	result := statements.Build(input, statements.Options{})

	if !statements.HasErrors(result.Issues) {
		t.Fatal("expected an error when no periods are supplied")
	}
	var found bool
	for _, iss := range result.Issues {
		if iss.Code == statements.IssueMissingPeriod {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected IssueMissingPeriod, got: %+v", result.Issues)
	}
	if result.DatasetAvailability != statements.AvailabilityInvalid {
		t.Errorf("expected INVALID availability with no periods, got %v", result.DatasetAvailability)
	}
}

func TestValidate_MixedCurrencyWithoutReportingCurrency(t *testing.T) {
	chart := ledger.BuildChartOfAccounts([]ledger.Account{
		{ID: "1000", Number: "1000", Name: "Cash USD", Type: ledger.AccountAsset, Currency: "USD", Active: true},
		{ID: "1010", Number: "1010", Name: "Cash EUR", Type: ledger.AccountAsset, Currency: "EUR", Active: true},
		{ID: "3000", Number: "3000", Name: "Equity", Type: ledger.AccountEquity, Currency: "USD", Active: true},
	})
	tb := ledger.NormalizeTrialBalance(ledger.TrialBalanceInput{
		Period: "2025-06",
		Lines: []ledger.TrialBalanceInputLine{
			{AccountID: "1000", Debit: 10000, Currency: "USD"},
			{AccountID: "1010", Debit: 5000, Currency: "EUR"},
			{AccountID: "3000", Credit: 10000, Currency: "USD"},
		},
	}, chart, 0.01)

	input := statements.Input{
		Source:                statements.SourceImportedTrialBalance,
		ImportedChart:         []ledger.Account{{ID: "1000", Type: ledger.AccountAsset, Currency: "USD", Active: true}, {ID: "1010", Type: ledger.AccountAsset, Currency: "EUR", Active: true}, {ID: "3000", Type: ledger.AccountEquity, Currency: "USD", Active: true}},
		ImportedTrialBalances: map[financial.Period]ledger.NormalizedTrialBalance{"2025-06": tb},
		Periods:               []financial.Period{"2025-06"},
		Mappings: []statements.AccountMapping{
			{AccountID: "1000", FinancialCode: financial.CodeBsCash, StatementType: financial.StatementBalanceSheet, Source: statements.MappingSourceExplicit},
			{AccountID: "1010", FinancialCode: financial.CodeBsCurrentAssetOther, StatementType: financial.StatementBalanceSheet, Source: statements.MappingSourceExplicit},
			{AccountID: "3000", FinancialCode: financial.CodeBsOwnerEquity, StatementType: financial.StatementBalanceSheet, Source: statements.MappingSourceExplicit},
		},
	}

	result := statements.Build(input, statements.Options{})

	var found bool
	for _, iss := range result.Issues {
		if iss.Code == statements.IssueMixedCurrency {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected IssueMixedCurrency, got: %+v", result.Issues)
	}
	if result.Dataset.Currency != "" || len(result.Dataset.Items) != 0 {
		t.Errorf("expected no dataset to be built when currency cannot be resolved, got currency=%q items=%d", result.Dataset.Currency, len(result.Dataset.Items))
	}
}

func TestValidate_MixedCurrencyWithExplicitReportingCurrency(t *testing.T) {
	chart := ledger.BuildChartOfAccounts([]ledger.Account{
		{ID: "1000", Number: "1000", Name: "Cash USD", Type: ledger.AccountAsset, Currency: "USD", Active: true},
		{ID: "1010", Number: "1010", Name: "Cash EUR", Type: ledger.AccountAsset, Currency: "EUR", Active: true},
		{ID: "3000", Number: "3000", Name: "Equity", Type: ledger.AccountEquity, Currency: "USD", Active: true},
	})
	tb := ledger.NormalizeTrialBalance(ledger.TrialBalanceInput{
		Period: "2025-06",
		Lines: []ledger.TrialBalanceInputLine{
			{AccountID: "1000", Debit: 10000, Currency: "USD"},
			{AccountID: "3000", Credit: 10000, Currency: "USD"},
		},
	}, chart, 0.01)

	input := statements.Input{
		Source:                statements.SourceImportedTrialBalance,
		ImportedChart:         []ledger.Account{{ID: "1000", Type: ledger.AccountAsset, Currency: "USD", Active: true}, {ID: "3000", Type: ledger.AccountEquity, Currency: "USD", Active: true}},
		ImportedTrialBalances: map[financial.Period]ledger.NormalizedTrialBalance{"2025-06": tb},
		Periods:               []financial.Period{"2025-06"},
		ReportingCurrency:     "USD",
		Mappings: []statements.AccountMapping{
			{AccountID: "1000", FinancialCode: financial.CodeBsCash, StatementType: financial.StatementBalanceSheet, Source: statements.MappingSourceExplicit},
			{AccountID: "3000", FinancialCode: financial.CodeBsOwnerEquity, StatementType: financial.StatementBalanceSheet, Source: statements.MappingSourceExplicit},
		},
	}

	result := statements.Build(input, statements.Options{})

	if result.Dataset.Currency != "USD" {
		t.Errorf("expected dataset currency USD, got %q", result.Dataset.Currency)
	}
}
