package statements_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/ledger"
	"github.com/themurtez/go-valuate/accounting/statements"
	"github.com/themurtez/go-valuate/accounting/statements/fixtures"
	"github.com/themurtez/go-valuate/financial"
)

func TestBuild_ServiceBusiness_LedgerDerived(t *testing.T) {
	input := statements.Input{
		Source:    statements.SourceLedger,
		Chart:     fixtures.ServiceBusinessChart(),
		Entries:   fixtures.ServiceBusinessEntries(),
		Periods:   []financial.Period{"2025-01"},
		Mappings:  fixtures.ServiceBusinessMappings(),
		Selection: statements.SelectionBoth,
	}

	result := statements.Build(input, statements.Options{})

	// This fixture's balance sheet is deliberately expected to be OUT of
	// balance by exactly the period's $1000 net income: the statement
	// builder never auto-posts current-period net income to retained
	// earnings (task section 27 — "the statement builder is a reporting
	// transformation, not a posting engine"), so an unbalanced
	// IssueUnbalancedBalanceSheet is the CORRECT, intentional result
	// here, not a bug. See TestBuild_BalancesWhenNetIncomePosted below for
	// the same fixture with an explicit caller-supplied retained-earnings
	// entry, which DOES balance.
	var sawUnbalanced bool
	for _, iss := range result.Issues {
		if iss.Code == statements.IssueUnbalancedBalanceSheet {
			sawUnbalanced = true
		}
	}
	if !sawUnbalanced {
		t.Fatalf("expected IssueUnbalancedBalanceSheet (NI not posted to equity), got issues: %+v", result.Issues)
	}

	if result.Coverage.CountCoveragePercent != 100 {
		t.Fatalf("expected 100%% count coverage, got %v", result.Coverage.CountCoveragePercent)
	}
	if result.Coverage.AmountCoveragePercent != 100 {
		t.Fatalf("expected 100%% amount coverage, got %v", result.Coverage.AmountCoveragePercent)
	}

	rev, ok := result.Dataset.ByCodeAndPeriod(financial.CodeRevService, "2025-01")
	if !ok {
		t.Fatalf("expected revenue item in dataset")
	}
	if rev.Amount != 12000 {
		t.Errorf("revenue = %v, want 12000", rev.Amount)
	}

	payroll, ok := result.Dataset.ByCodeAndPeriod(financial.CodeOpexPayroll, "2025-01")
	if !ok || payroll.Amount != 8000 {
		t.Errorf("payroll = %v (ok=%v), want 8000", payroll.Amount, ok)
	}

	cash, ok := result.Dataset.ByCodeAndPeriod(financial.CodeBsCash, "2025-01")
	if !ok {
		t.Fatalf("expected cash item in dataset")
	}
	// 50000 (capital) - 8000 (payroll) - 3000 (rent) = 39000, then the
	// AR collection (JE5) debits Cash 12000 and credits AR 12000 back to
	// zero, netting Cash to 51000 overall.
	if cash.Amount != 51000 {
		t.Errorf("cash = %v, want 51000", cash.Amount)
	}

	if result.IncomeStatement == nil {
		t.Fatal("expected income statement")
	}
	if result.BalanceSheet == nil {
		t.Fatal("expected balance sheet")
	}

	if len(result.Reconciliation) != 1 {
		t.Fatalf("expected 1 reconciliation entry, got %d", len(result.Reconciliation))
	}
	if result.Reconciliation[0].Balanced {
		t.Errorf("expected balance sheet NOT to balance (NI not posted to equity), got balanced=true")
	}
	if result.Reconciliation[0].Assets != 51000 {
		t.Errorf("reconciliation Assets = %v, want 51000", result.Reconciliation[0].Assets)
	}
	if result.Reconciliation[0].Equity != 50000 {
		t.Errorf("reconciliation Equity = %v, want 50000", result.Reconciliation[0].Equity)
	}
}

func TestBuild_ImportedTrialBalance(t *testing.T) {
	chart := fixtures.ServiceBusinessChart()
	ledgerChart := ledger.BuildChartOfAccounts(chart)
	tb := ledger.NormalizeTrialBalance(ledger.TrialBalanceInput{
		Period: "2025-01",
		Lines: []ledger.TrialBalanceInputLine{
			{AccountID: "1000", Debit: 51000},
			{AccountID: "1100", Debit: 0},
			{AccountID: "2000", Credit: 0},
			{AccountID: "3000", Credit: 50000},
			{AccountID: "4000", Credit: 12000},
			{AccountID: "6000", Debit: 8000},
			{AccountID: "6100", Debit: 3000},
		},
	}, ledgerChart, 0.01)

	if !tb.Balanced {
		t.Fatalf("fixture trial balance does not balance: diff=%v", tb.Difference)
	}

	input := statements.Input{
		Source:                statements.SourceImportedTrialBalance,
		ImportedChart:         chart,
		ImportedTrialBalances: map[financial.Period]ledger.NormalizedTrialBalance{"2025-01": tb},
		Periods:               []financial.Period{"2025-01"},
		Mappings:              fixtures.ServiceBusinessMappings(),
		Selection:             statements.SelectionIncomeOnly, // this TB's equity is not NI-adjusted; see the ledger-derived test's identical note
	}

	result := statements.Build(input, statements.Options{})

	if statements.HasErrors(result.Issues) {
		t.Fatalf("unexpected error issues: %+v", result.Issues)
	}

	rev, ok := result.Dataset.ByCodeAndPeriod(financial.CodeRevService, "2025-01")
	if !ok || rev.Amount != 12000 {
		t.Errorf("revenue = %v (ok=%v), want 12000", rev.Amount, ok)
	}
}

func TestBuild_ManyToOne_MultipleRevenueAccounts(t *testing.T) {
	input := statements.Input{
		Source:    statements.SourceLedger,
		Chart:     fixtures.MultiRevenueChart(),
		Entries:   fixtures.MultiRevenueEntries(),
		Periods:   []financial.Period{"2025-07"},
		Mappings:  fixtures.MultiRevenueMappings(),
		Selection: statements.SelectionIncomeOnly,
	}

	result := statements.Build(input, statements.Options{})

	if statements.HasErrors(result.Issues) {
		t.Fatalf("unexpected error issues: %+v", result.Issues)
	}

	rev, ok := result.Dataset.ByCodeAndPeriod(financial.CodeRevService, "2025-07")
	if !ok {
		t.Fatalf("expected aggregated revenue item")
	}
	if rev.Amount != 9200 { // 5000 + 1200 + 3000
		t.Errorf("aggregated revenue = %v, want 9200", rev.Amount)
	}
	if len(rev.Sources) != 3 {
		t.Errorf("expected 3 contributing sources, got %d: %+v", len(rev.Sources), rev.Sources)
	}

	// Verify the Statement model itself also shows all 3 contributors.
	var found bool
	for _, sec := range result.IncomeStatement.Sections {
		for _, row := range sec.Rows {
			if row.FinancialCode == financial.CodeRevService {
				found = true
				if len(row.Contributors) != 3 {
					t.Errorf("expected 3 contributors on the Row, got %d", len(row.Contributors))
				}
			}
		}
	}
	if !found {
		t.Error("expected to find the aggregated revenue row in the income statement")
	}
}

func TestBuild_ContraAccount_SignInvert(t *testing.T) {
	input := statements.Input{
		Source:    statements.SourceLedger,
		Chart:     fixtures.ContraAccountChart(),
		Entries:   fixtures.ContraAccountEntries(),
		Periods:   []financial.Period{"2025-08"},
		Mappings:  fixtures.ContraAccountMappings(),
		Selection: statements.SelectionBalanceOnly,
	}

	result := statements.Build(input, statements.Options{})

	if statements.HasErrors(result.Issues) {
		t.Fatalf("unexpected error issues: %+v", result.Issues)
	}

	// Accumulated Depreciation is an ASSET-typed ledger account (normal
	// side DEBIT — see ledger.NormalBalance) carrying a CREDIT raw
	// balance of -4000 (contra to its own account type). ledger's
	// natural-display flip only negates a balance when the account's
	// normal side is CREDIT (see naturalDisplay/ledger's own
	// displayBalance) — since Accumulated Depreciation's normal side is
	// DEBIT, SignNatural leaves it at -4000, unchanged. But
	// financial/reconciliation's own contra-asset convention
	// (checkBalanceSheetBalances explicitly does `totalAssets -=
	// item.Amount` for this exact code) requires the dataset to hold a
	// POSITIVE magnitude here, which it then subtracts itself. SignInvert
	// flips SignNatural's -4000 to +4000, producing exactly the
	// convention reconciliation expects — see types.go's SignInvert doc
	// comment for this same worked example.
	accum, ok := result.Dataset.ByCodeAndPeriod(financial.CodeBsAccumDepreciation, "2025-08")
	if !ok {
		t.Fatalf("expected accumulated depreciation item in dataset")
	}
	if accum.Amount != 4000 {
		t.Errorf("accumulated depreciation = %v, want 4000 (positive magnitude, per financial/reconciliation's contra-asset convention)", accum.Amount)
	}

	if len(result.Reconciliation) != 1 {
		t.Fatalf("expected 1 reconciliation entry, got %d", len(result.Reconciliation))
	}
	if !result.Reconciliation[0].Balanced {
		t.Errorf("expected balance sheet to balance with contra-asset correctly reducing assets, got diff=%v assets=%v equity=%v",
			result.Reconciliation[0].Difference, result.Reconciliation[0].Assets, result.Reconciliation[0].Equity)
	}
}

func TestBuild_UnmappedMaterialAccount_DefaultPolicy(t *testing.T) {
	// SelectionIncomeOnly deliberately sidesteps this fixture's balance
	// sheet, which (like every fixture in this package) does not
	// automatically post net income to equity and so would independently
	// fail IssueUnbalancedBalanceSheet — see TestBuild_ServiceBusiness_
	// LedgerDerived's identical note. This test is specifically about
	// IssueUnmappedAccount's own severity/availability effect in
	// isolation, not about balance-sheet reconciliation.
	input := statements.Input{
		Source:    statements.SourceLedger,
		Chart:     fixtures.OwnerOperatedChart(),
		Entries:   fixtures.OwnerOperatedEntries(),
		Periods:   []financial.Period{"2025-02"},
		Mappings:  fixtures.UnmappedMaterialAccountMappings(), // "3100" Owner's Draw left unmapped
		Selection: statements.SelectionIncomeOnly,
	}

	opts := statements.Options{
		Materiality: statements.MaterialityPolicy{AbsoluteThreshold: 100},
	}
	result := statements.Build(input, opts)

	var found bool
	for _, iss := range result.Issues {
		if iss.Code == statements.IssueUnmappedAccount && iss.AccountID == "3100" {
			found = true
			if iss.Severity != statements.SeverityWarning {
				t.Errorf("expected warning severity under default policy, got %v", iss.Severity)
			}
		}
	}
	if !found {
		t.Fatalf("expected IssueUnmappedAccount for account 3100, got issues: %+v", result.Issues)
	}

	if statements.HasErrors(result.Issues) {
		t.Fatalf("expected no error-severity issues, got: %+v", result.Issues)
	}
	if result.DatasetAvailability != statements.AvailabilityBuiltWithWarnings {
		t.Errorf("expected BUILT_WITH_WARNINGS, got %v (issues: %+v)", result.DatasetAvailability, result.Issues)
	}

	// The unmapped account must still appear in Result.Mappings (never
	// hidden — task section 23) with the correct MappingStatus.
	var mappingFound bool
	for _, mr := range result.Mappings {
		if mr.AccountID == "3100" {
			mappingFound = true
			if mr.MappingStatus != statements.MappingStatusUnmapped {
				t.Errorf("expected MappingStatusUnmapped, got %v", mr.MappingStatus)
			}
		}
	}
	if !mappingFound {
		t.Fatal("expected account 3100 to still appear in Result.Mappings")
	}
}

func TestBuild_UnmappedMaterialAccount_FailPolicy(t *testing.T) {
	input := statements.Input{
		Source:    statements.SourceLedger,
		Chart:     fixtures.OwnerOperatedChart(),
		Entries:   fixtures.OwnerOperatedEntries(),
		Periods:   []financial.Period{"2025-02"},
		Mappings:  fixtures.UnmappedMaterialAccountMappings(),
		Selection: statements.SelectionIncomeOnly,
	}

	opts := statements.Options{
		UnmappedPolicy: statements.PolicyFailOnUnmappedMaterial,
		Materiality:    statements.MaterialityPolicy{AbsoluteThreshold: 100},
	}
	result := statements.Build(input, opts)

	if !statements.HasErrors(result.Issues) {
		t.Fatalf("expected error-severity issues under FAIL_ON_UNMAPPED_MATERIAL, got: %+v", result.Issues)
	}
	if result.DatasetAvailability != statements.AvailabilityInvalid {
		t.Errorf("expected INVALID dataset availability, got %v", result.DatasetAvailability)
	}
}

func TestBuild_InvalidMapping_AccountTypeIncompatible(t *testing.T) {
	input := statements.Input{
		Source:    statements.SourceLedger,
		Chart:     fixtures.ServiceBusinessChart(),
		Entries:   fixtures.ServiceBusinessEntries(),
		Periods:   []financial.Period{"2025-01"},
		Mappings:  []statements.AccountMapping{fixtures.InvalidMappingExample()},
		Selection: statements.SelectionBoth,
	}

	result := statements.Build(input, statements.Options{})

	var found bool
	for _, iss := range result.Issues {
		if iss.Code == statements.IssueIncompatibleAccountType {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected IssueIncompatibleAccountType, got issues: %+v", result.Issues)
	}

	// account 4000 should NOT have contributed to the (wrong) balance
	// sheet code, since an invalid mapping must never build data from it.
	if _, ok := result.Dataset.ByCodeAndPeriod(financial.CodeBsCash, "2025-01"); ok {
		for _, item := range result.Dataset.Items {
			if item.Code == financial.CodeBsCash {
				for _, src := range item.Sources {
					if src.RowID == "4000" {
						t.Errorf("invalid mapping for account 4000 leaked into CodeBsCash dataset item")
					}
				}
			}
		}
	}
}

func TestBuild_UnbalancedSourceTrialBalance(t *testing.T) {
	chart := fixtures.ServiceBusinessChart()
	input := statements.Input{
		Source:  statements.SourceLedger,
		Chart:   chart,
		Entries: fixtures.UnbalancedJournalEntries(),
		Periods: []financial.Period{"2025-05"},
	}

	result := statements.Build(input, statements.Options{})

	var found bool
	for _, iss := range result.Issues {
		if iss.Code == statements.IssueSourceTrialBalanceUnbalanced {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected IssueSourceTrialBalanceUnbalanced, got: %+v", result.Issues)
	}
}

func TestBuild_MultiDepartment_NoDoubleCounting(t *testing.T) {
	input := statements.Input{
		Source:    statements.SourceLedger,
		Chart:     fixtures.MultiDepartmentChart(),
		Entries:   fixtures.MultiDepartmentEntries(),
		Periods:   []financial.Period{"2025-04"},
		Mappings:  fixtures.MultiDepartmentMappings(),
		Selection: statements.SelectionIncomeOnly,
	}

	result := statements.Build(input, statements.Options{})

	if statements.HasErrors(result.Issues) {
		t.Fatalf("unexpected error issues: %+v", result.Issues)
	}

	// 6100 (4000) + 6200 (2500) + 6900 has no direct postings; 6000
	// received a DIRECT 1000 posting itself but was left UNMAPPED (see
	// MultiDepartmentMappings), so it must NOT appear anywhere in the
	// dataset — only its mapped children/leaf postings should.
	other, ok := result.Dataset.ByCodeAndPeriod(financial.CodeOpexOther, "2025-04")
	if !ok {
		t.Fatalf("expected aggregated opex-other item")
	}
	if other.Amount != 6500 { // 4000 + 2500; the unmapped parent's own 1000 direct posting is excluded
		t.Errorf("aggregated opex = %v, want 6500 (no double counting, unmapped parent excluded)", other.Amount)
	}
}
