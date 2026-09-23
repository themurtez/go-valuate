package ledger

import "testing"

func validChart() []Account {
	return []Account{
		{ID: "1000", Number: "1000", Name: "Cash", Type: AccountAsset, Active: true},
		{ID: "1100", Number: "1100", Name: "Accounts Receivable", Type: AccountAsset, Active: true},
		{ID: "2000", Number: "2000", Name: "Accounts Payable", Type: AccountLiability, Active: true},
		{ID: "3000", Number: "3000", Name: "Common Stock", Type: AccountEquity, Active: true},
		{ID: "4000", Number: "4000", Name: "Revenue", Type: AccountRevenue, Active: true},
		{ID: "5000", Number: "5000", Name: "Expenses", Type: AccountExpense, Active: true},
	}
}

func TestValidateAccounts_Valid(t *testing.T) {
	issues := ValidateAccounts(validChart())
	if HasErrors(issues) {
		t.Fatalf("expected no errors for a valid chart, got %+v", issues)
	}
}

func TestValidateAccounts_Duplicate(t *testing.T) {
	accounts := []Account{
		{ID: "1000", Name: "Cash", Type: AccountAsset},
		{ID: "1000", Name: "Cash Duplicate", Type: AccountAsset},
	}
	issues := ValidateAccounts(accounts)
	found := false
	for _, i := range issues {
		if i.Code == IssueDuplicateAccount {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected IssueDuplicateAccount, got %+v", issues)
	}
}

func TestValidateAccounts_UnrecognizedType(t *testing.T) {
	accounts := []Account{{ID: "1000", Name: "Mystery", Type: AccountType("BOGUS")}}
	issues := ValidateAccounts(accounts)
	if !HasErrors(issues) {
		t.Fatalf("expected an error for unrecognized type, got %+v", issues)
	}
}

func TestValidateAccounts_Hierarchy(t *testing.T) {
	accounts := []Account{
		{ID: "1000", Name: "Current Assets", Type: AccountAsset},
		{ID: "1010", Name: "Cash", Type: AccountAsset, ParentID: "1000"},
		{ID: "1020", Name: "AR", Type: AccountAsset, ParentID: "1000"},
	}
	issues := ValidateAccounts(accounts)
	if HasErrors(issues) {
		t.Fatalf("expected valid hierarchy to have no errors, got %+v", issues)
	}
}

func TestValidateAccounts_MissingParent(t *testing.T) {
	accounts := []Account{
		{ID: "1010", Name: "Cash", Type: AccountAsset, ParentID: "9999"},
	}
	issues := ValidateAccounts(accounts)
	if !hasCode(issues, IssueMissingParentAccount) {
		t.Fatalf("expected IssueMissingParentAccount, got %+v", issues)
	}
}

func TestValidateAccounts_SelfParentCycle(t *testing.T) {
	accounts := []Account{
		{ID: "1010", Name: "Cash", Type: AccountAsset, ParentID: "1010"},
	}
	issues := ValidateAccounts(accounts)
	if !hasCode(issues, IssueAccountHierarchyCycle) {
		t.Fatalf("expected IssueAccountHierarchyCycle for self-parent, got %+v", issues)
	}
}

func TestValidateAccounts_IndirectCycle(t *testing.T) {
	accounts := []Account{
		{ID: "A", Name: "A", Type: AccountAsset, ParentID: "C"},
		{ID: "B", Name: "B", Type: AccountAsset, ParentID: "A"},
		{ID: "C", Name: "C", Type: AccountAsset, ParentID: "B"},
	}
	issues := ValidateAccounts(accounts)
	count := 0
	for _, i := range issues {
		if i.Code == IssueAccountHierarchyCycle {
			count++
		}
	}
	if count != 3 {
		t.Fatalf("expected each of the 3 accounts in the cycle to be individually flagged, got %d cycle issues: %+v", count, issues)
	}
}

func TestValidateAccounts_InactiveAllowed(t *testing.T) {
	accounts := []Account{{ID: "1000", Name: "Old Cash", Type: AccountAsset, Active: false}}
	issues := ValidateAccounts(accounts)
	if HasErrors(issues) {
		t.Fatalf("an inactive account by itself (no posting) should not be an error: %+v", issues)
	}
}

func TestChartOfAccounts_LookupAndChildren(t *testing.T) {
	chart := BuildChartOfAccounts([]Account{
		{ID: "1000", Name: "Current Assets", Type: AccountAsset},
		{ID: "1010", Name: "Cash", Type: AccountAsset, ParentID: "1000"},
		{ID: "1020", Name: "AR", Type: AccountAsset, ParentID: "1000"},
	})
	if _, ok := chart.Lookup("1000"); !ok {
		t.Fatal("expected to find account 1000")
	}
	if _, ok := chart.Lookup("nope"); ok {
		t.Fatal("expected not to find unknown account")
	}
	children := chart.Children("1000")
	if len(children) != 2 || children[0] != "1010" || children[1] != "1020" {
		t.Fatalf("expected sorted children [1010 1020], got %v", children)
	}
}

func TestChartOfAccounts_Descendants(t *testing.T) {
	chart := BuildChartOfAccounts([]Account{
		{ID: "1000", Name: "Assets", Type: AccountAsset},
		{ID: "1100", Name: "Current Assets", Type: AccountAsset, ParentID: "1000"},
		{ID: "1110", Name: "Cash", Type: AccountAsset, ParentID: "1100"},
		{ID: "1120", Name: "AR", Type: AccountAsset, ParentID: "1100"},
	})
	desc := chart.Descendants("1000")
	want := []string{"1100", "1110", "1120"}
	if len(desc) != len(want) {
		t.Fatalf("got %v, want %v", desc, want)
	}
	for i := range want {
		if desc[i] != want[i] {
			t.Fatalf("got %v, want %v", desc, want)
		}
	}
}

func hasCode(issues []Issue, code IssueCode) bool {
	for _, i := range issues {
		if i.Code == code {
			return true
		}
	}
	return false
}
