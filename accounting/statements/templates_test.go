package statements_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/ledger"
	"github.com/themurtez/go-valuate/accounting/statements"
	"github.com/themurtez/go-valuate/accounting/statements/fixtures"
	"github.com/themurtez/go-valuate/financial"
)

func TestResolveTemplate_MatchesByAccountNumber(t *testing.T) {
	chart := ledger.BuildChartOfAccounts(fixtures.ServiceBusinessChart())
	template := fixtures.ServiceBusinessMappingTemplate()

	mappings, issues := statements.ResolveTemplate(template, chart)
	if len(issues) != 0 {
		t.Fatalf("unexpected issues: %+v", issues)
	}
	if len(mappings) != 11 {
		t.Fatalf("expected 11 resolved mappings, got %d", len(mappings))
	}

	var cash *statements.AccountMapping
	for i := range mappings {
		if mappings[i].AccountID == "1000" {
			cash = &mappings[i]
		}
	}
	if cash == nil {
		t.Fatal("expected account 1000 resolved from template")
	}
	if cash.FinancialCode != financial.CodeBsCash {
		t.Errorf("expected CodeBsCash, got %v", cash.FinancialCode)
	}
	if cash.Source != statements.MappingSourceExplicit {
		t.Errorf("a resolved template rule should count as explicit, got %v", cash.Source)
	}
}

func TestResolveTemplate_ConflictingRulesReportIssue(t *testing.T) {
	chart := ledger.BuildChartOfAccounts(fixtures.ServiceBusinessChart())
	template := fixtures.ConflictingMappingTemplate()

	mappings, issues := statements.ResolveTemplate(template, chart)

	var found bool
	for _, iss := range issues {
		if iss.Code == statements.IssueMappingConflict && iss.AccountID == "1000" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected IssueMappingConflict for account 1000, got: %+v", issues)
	}

	for _, m := range mappings {
		if m.AccountID == "1000" {
			t.Errorf("a conflicting account must not appear in the resolved mappings, got %+v", m)
		}
	}
}

func TestResolveTemplate_PrecedenceAccountIDBeforeNumberBeforeName(t *testing.T) {
	chart := ledger.BuildChartOfAccounts([]ledger.Account{
		{ID: "acct-1", Number: "1000", Name: "Cash", Type: ledger.AccountAsset, Active: true},
	})

	// Three rules of DIFFERENT precedence kinds all matching the same
	// account: AccountID should win, even though it's listed last in
	// Mappings (precedence is by KIND, not by list order).
	template := statements.MappingTemplate{
		Mappings: []statements.AccountMappingRule{
			{ExactName: "Cash", Mapping: statements.AccountMapping{FinancialCode: financial.CodeBsCurrentAssetOther, StatementType: financial.StatementBalanceSheet}},
			{AccountNumber: "1000", Mapping: statements.AccountMapping{FinancialCode: financial.CodeBsPrepaid, StatementType: financial.StatementBalanceSheet}},
			{AccountID: "acct-1", Mapping: statements.AccountMapping{FinancialCode: financial.CodeBsCash, StatementType: financial.StatementBalanceSheet}},
		},
	}

	mappings, issues := statements.ResolveTemplate(template, chart)
	if len(issues) != 0 {
		t.Fatalf("unexpected issues: %+v", issues)
	}
	if len(mappings) != 1 {
		t.Fatalf("expected exactly 1 resolved mapping, got %d", len(mappings))
	}
	if mappings[0].FinancialCode != financial.CodeBsCash {
		t.Errorf("expected AccountID-precedence rule (CodeBsCash) to win, got %v", mappings[0].FinancialCode)
	}
}

func TestResolveTemplate_ExactNameIsNormalized(t *testing.T) {
	chart := ledger.BuildChartOfAccounts([]ledger.Account{
		{ID: "acct-1", Number: "", Name: "Owner's Draw", Type: ledger.AccountEquity, Active: true},
	})
	template := statements.MappingTemplate{
		Mappings: []statements.AccountMappingRule{
			// Deliberately different casing/punctuation from the account's
			// own Name, to verify classification.NormalizeLabel's
			// comparison is actually used (not a raw string ==).
			{ExactName: "owners draw", Mapping: statements.AccountMapping{FinancialCode: financial.CodeBsOwnerEquity, StatementType: financial.StatementBalanceSheet}},
		},
	}

	mappings, issues := statements.ResolveTemplate(template, chart)
	if len(issues) != 0 {
		t.Fatalf("unexpected issues: %+v", issues)
	}
	if len(mappings) != 1 || mappings[0].FinancialCode != financial.CodeBsOwnerEquity {
		t.Fatalf("expected normalized exact-name match, got %+v", mappings)
	}
}
