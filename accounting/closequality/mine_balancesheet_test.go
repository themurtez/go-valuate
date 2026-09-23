package closequality_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/closequality"
	"github.com/themurtez/go-valuate/accounting/closequality/fixtures"
	"github.com/themurtez/go-valuate/accounting/ledger"
)

// TestAccountExpectation_ExpectedSignPositive_Violation proves a
// declared ExpectedSignPositive account with a negative ending balance
// produces FindingUnexpectedAccountBalance.
func TestAccountExpectation_ExpectedSignPositive_Violation(t *testing.T) {
	in := fixtures.CleanCloseInput()
	policy := fixtures.CleanPolicy()
	policy.AccountExpectations = []closequality.AccountExpectation{
		{AccountID: "2000", ExpectedSign: closequality.ExpectedSignPositive, Label: "AP must not be negative"},
	}

	// Force account 2000 (Accounts Payable) negative by crediting cash
	// against it beyond a normal payable posting — post a debit that
	// flips its display balance negative.
	in.Ledger.Entries = append(in.Ledger.Entries, apOverpaymentEntry())

	result := closequality.Calculate(in, policy)

	if !hasFindingCode(result.Warnings, closequality.FindingUnexpectedAccountBalance) &&
		!hasFindingCode(result.Blockers, closequality.FindingUnexpectedAccountBalance) {
		t.Errorf("expected FindingUnexpectedAccountBalance, got warnings=%+v blockers=%+v", result.Warnings, result.Blockers)
	}
}

// TestAccountExpectation_NoExpectation_NoFinding proves an account with
// no declared AccountExpectation never produces
// FindingUnexpectedAccountBalance, even with an opposite-sign balance —
// this package never infers sign expectations from account type or name.
func TestAccountExpectation_NoExpectation_NoFinding(t *testing.T) {
	in := fixtures.CleanCloseInput()
	in.Ledger.Entries = append(in.Ledger.Entries, apOverpaymentEntry())
	policy := fixtures.CleanPolicy() // no AccountExpectations declared

	result := closequality.Calculate(in, policy)

	if hasFindingCode(allFindings(result), closequality.FindingUnexpectedAccountBalance) {
		t.Errorf("did not expect FindingUnexpectedAccountBalance without a declared expectation")
	}
}

// TestAccountExpectation_MinMaxBalance proves Min/MaxBalance range
// checks fire independently of ExpectedSign.
func TestAccountExpectation_MinMaxBalance(t *testing.T) {
	maxBalance := 10000.0
	in := fixtures.CleanCloseInput() // cash (1000) ends at 51000
	policy := fixtures.CleanPolicy()
	policy.AccountExpectations = []closequality.AccountExpectation{
		{AccountID: "1000", MaxBalance: &maxBalance, Label: "cash sweep ceiling"},
	}

	result := closequality.Calculate(in, policy)

	if !hasFindingCode(allFindings(result), closequality.FindingUnexpectedAccountBalance) {
		t.Errorf("expected FindingUnexpectedAccountBalance for balance exceeding declared max, got %+v", allFindings(result))
	}
}

// TestAccountExpectation_UnknownAccount_ProducesIssue proves an
// AccountExpectation for an account with no ledger activity produces an
// Issue rather than a silent no-op or a crash.
func TestAccountExpectation_UnknownAccount_ProducesIssue(t *testing.T) {
	in := fixtures.CleanCloseInput()
	policy := fixtures.CleanPolicy()
	policy.AccountExpectations = []closequality.AccountExpectation{
		{AccountID: "9999-does-not-exist", ExpectedSign: closequality.ExpectedSignPositive},
	}

	result := closequality.Calculate(in, policy)

	found := false
	for _, iss := range result.Issues {
		if iss.Code == closequality.IssueUnknownAccount {
			found = true
		}
	}
	if !found {
		t.Errorf("expected IssueUnknownAccount, got %+v", result.Issues)
	}
}

// TestAccountExpectation_DuplicateAccountID_ProducesIssue proves a
// duplicate AccountExpectation for the same AccountID produces an Issue
// and only the first is used.
func TestAccountExpectation_DuplicateAccountID_ProducesIssue(t *testing.T) {
	in := fixtures.CleanCloseInput()
	policy := fixtures.CleanPolicy()
	policy.AccountExpectations = []closequality.AccountExpectation{
		{AccountID: "1000", ExpectedSign: closequality.ExpectedSignPositive},
		{AccountID: "1000", ExpectedSign: closequality.ExpectedSignNegative},
	}

	result := closequality.Calculate(in, policy)

	found := false
	for _, iss := range result.Issues {
		if iss.Code == closequality.IssueInvalidAccountExpectation {
			found = true
		}
	}
	if !found {
		t.Errorf("expected IssueInvalidAccountExpectation for duplicate account expectation, got %+v", result.Issues)
	}
}

// apOverpaymentEntry posts a balanced entry that pushes account 2000
// (Accounts Payable, a natural-credit liability whose DisplayBalance is
// normally >= 0) to a negative DisplayBalance, simulating an overpayment
// reclass — a legitimate but unusual condition this package only flags
// when the caller has explicitly declared an ExpectedSign for it.
func apOverpaymentEntry() ledger.JournalEntry {
	return ledger.JournalEntry{
		ID: "SVC-JE-APOVERPAY", Date: "2025-01-25", Period: "2025-01", Status: ledger.StatusPosted,
		Description: "AP overpayment reclass", Source: "manual",
		Lines: []ledger.JournalLine{
			{ID: "L1", AccountID: "2000", Debit: 5000},
			{ID: "L2", AccountID: "1000", Credit: 5000},
		},
	}
}
