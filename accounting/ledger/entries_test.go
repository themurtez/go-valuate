package ledger

import "testing"

func testChart() ChartOfAccounts {
	return BuildChartOfAccounts(validChart())
}

func balancedEntry() JournalEntry {
	return JournalEntry{
		ID:     "JE-1",
		Date:   "2025-01-15",
		Period: "2025",
		Status: StatusPosted,
		Lines: []JournalLine{
			{ID: "L1", AccountID: "1000", Debit: 1000},
			{ID: "L2", AccountID: "4000", Credit: 1000},
		},
	}
}

func TestValidateEntries_Balanced(t *testing.T) {
	issues := ValidateEntries([]JournalEntry{balancedEntry()}, testChart(), ValidateOptions{})
	if HasErrors(issues) {
		t.Fatalf("expected no errors, got %+v", issues)
	}
}

func TestValidateEntries_Unbalanced(t *testing.T) {
	e := balancedEntry()
	e.Lines[1].Credit = 999
	issues := ValidateEntries([]JournalEntry{e}, testChart(), ValidateOptions{})
	if !hasCode(issues, IssueUnbalancedEntry) {
		t.Fatalf("expected IssueUnbalancedEntry, got %+v", issues)
	}
}

func TestValidateEntries_UnbalancedWithinTolerance(t *testing.T) {
	e := balancedEntry()
	e.Lines[1].Credit = 999.999
	issues := ValidateEntries([]JournalEntry{e}, testChart(), ValidateOptions{Tolerance: 0.01})
	if hasCode(issues, IssueUnbalancedEntry) {
		t.Fatalf("expected tolerance to absorb the tiny difference, got %+v", issues)
	}
}

func TestValidateEntries_InvalidSigns(t *testing.T) {
	e := balancedEntry()
	e.Lines[0].Debit = -1000
	issues := ValidateEntries([]JournalEntry{e}, testChart(), ValidateOptions{})
	if !hasCode(issues, IssueNegativeAmount) {
		t.Fatalf("expected IssueNegativeAmount, got %+v", issues)
	}
}

func TestValidateEntries_BothDebitAndCredit(t *testing.T) {
	e := balancedEntry()
	e.Lines[0].Credit = 500
	issues := ValidateEntries([]JournalEntry{e}, testChart(), ValidateOptions{})
	if !hasCode(issues, IssueInvalidDebitCredit) {
		t.Fatalf("expected IssueInvalidDebitCredit, got %+v", issues)
	}
}

func TestValidateEntries_UnknownAccount(t *testing.T) {
	e := balancedEntry()
	e.Lines[0].AccountID = "9999"
	issues := ValidateEntries([]JournalEntry{e}, testChart(), ValidateOptions{})
	if !hasCode(issues, IssueUnknownAccount) {
		t.Fatalf("expected IssueUnknownAccount, got %+v", issues)
	}
}

func TestValidateEntries_DraftPostedVoided(t *testing.T) {
	draft := balancedEntry()
	draft.ID = "JE-DRAFT"
	draft.Status = StatusDraft
	draft.Lines = []JournalLine{{AccountID: "1000", Debit: 500}} // only 1 line, allowed for draft

	posted := balancedEntry()
	posted.ID = "JE-POSTED"
	posted.Status = StatusPosted

	voided := balancedEntry()
	voided.ID = "JE-VOIDED"
	voided.Status = StatusVoided
	voided.Lines[1].Credit = 1 // deliberately unbalanced; voided entries aren't required to balance for our purposes... but let's keep it balanced to isolate the exclusion test
	voided.Lines[1].Credit = 1000

	issues := ValidateEntries([]JournalEntry{draft, posted, voided}, testChart(), ValidateOptions{})
	if HasErrors(issues) {
		t.Fatalf("expected no errors: draft (1 line, warning only), posted (valid), voided (valid): %+v", issues)
	}

	balances := CalculateBalances(testChart(), []JournalEntry{draft, posted, voided}, BalanceOptions{})
	cash := findBalance(t, balances, "1000")
	// Only the posted entry should count (draft never affects balances,
	// voided never affects balances even though it's well-formed).
	if cash.RawBalance != 1000 {
		t.Fatalf("expected only the posted entry (1000) to count, got RawBalance=%v", cash.RawBalance)
	}
}

func TestValidateEntries_Duplicates(t *testing.T) {
	e1 := balancedEntry()
	e2 := balancedEntry() // same ID "JE-1"
	issues := ValidateEntries([]JournalEntry{e1, e2}, testChart(), ValidateOptions{})
	if !hasCode(issues, IssueDuplicateEntry) {
		t.Fatalf("expected IssueDuplicateEntry, got %+v", issues)
	}
}

func TestValidateEntries_DuplicateLineIDs(t *testing.T) {
	e := balancedEntry()
	e.Lines[1].ID = e.Lines[0].ID // force duplicate
	issues := ValidateEntries([]JournalEntry{e}, testChart(), ValidateOptions{})
	if !hasCode(issues, IssueDuplicateLine) {
		t.Fatalf("expected IssueDuplicateLine, got %+v", issues)
	}
}

func TestValidateEntries_TooFewLinesPosted(t *testing.T) {
	e := balancedEntry()
	e.Lines = e.Lines[:1]
	issues := ValidateEntries([]JournalEntry{e}, testChart(), ValidateOptions{})
	found := false
	for _, i := range issues {
		if i.Code == IssueTooFewLines && i.Severity == SeverityError {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected IssueTooFewLines as an error for a posted entry, got %+v", issues)
	}
}

func TestValidateEntries_MissingDateAndPeriod(t *testing.T) {
	e := balancedEntry()
	e.Date = ""
	e.Period = ""
	issues := ValidateEntries([]JournalEntry{e}, testChart(), ValidateOptions{})
	if !hasCode(issues, IssueInvalidPeriod) {
		t.Fatalf("expected IssueInvalidPeriod, got %+v", issues)
	}
}

func TestValidateEntries_InactiveAccountPosting(t *testing.T) {
	chart := BuildChartOfAccounts([]Account{
		{ID: "1000", Name: "Old Cash", Type: AccountAsset, Active: false},
		{ID: "4000", Name: "Revenue", Type: AccountRevenue, Active: true},
	})
	e := JournalEntry{
		ID: "JE-1", Date: "2025-01-01", Status: StatusPosted,
		Lines: []JournalLine{
			{AccountID: "1000", Debit: 100},
			{AccountID: "4000", Credit: 100},
		},
	}
	issuesWarn := ValidateEntries([]JournalEntry{e}, chart, ValidateOptions{})
	if !hasCode(issuesWarn, IssueInactiveAccountPosting) {
		t.Fatalf("expected IssueInactiveAccountPosting, got %+v", issuesWarn)
	}
	if HasErrors(issuesWarn) {
		t.Fatalf("default policy should be warning, not error: %+v", issuesWarn)
	}

	issuesErr := ValidateEntries([]JournalEntry{e}, chart, ValidateOptions{InactivePostingPolicy: InactivePostingError})
	if !HasErrors(issuesErr) {
		t.Fatalf("InactivePostingError policy should produce an error: %+v", issuesErr)
	}
}

func TestJournalEntry_TotalsAndIsBalanced(t *testing.T) {
	e := balancedEntry()
	if e.TotalDebits() != 1000 || e.TotalCredits() != 1000 {
		t.Fatalf("unexpected totals: debits=%v credits=%v", e.TotalDebits(), e.TotalCredits())
	}
	if !e.IsBalanced(0) {
		t.Fatal("expected balanced")
	}
}

func findBalance(t *testing.T, balances []Balance, accountID string) Balance {
	t.Helper()
	for _, b := range balances {
		if b.AccountID == accountID {
			return b
		}
	}
	t.Fatalf("no balance found for account %s", accountID)
	return Balance{}
}
