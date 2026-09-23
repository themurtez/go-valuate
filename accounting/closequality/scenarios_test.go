package closequality_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/ar"
	"github.com/themurtez/go-valuate/accounting/closequality"
	"github.com/themurtez/go-valuate/accounting/closequality/fixtures"
	"github.com/themurtez/go-valuate/accounting/ledger"
)

// TestScenario_MaterialUnmappedAccounts proves a material unmapped
// account produces a blocking finding and NOT_READY, using a real
// statements.Build output built from a deliberately-incomplete mapping
// set.
func TestScenario_MaterialUnmappedAccounts(t *testing.T) {
	in := fixtures.CleanCloseInput()
	in.Statements = fixtures.UnmappedMaterialStatementsResult()

	result := closequality.Calculate(in, fixtures.CleanPolicy())

	if !hasFindingCode(result.Blockers, closequality.FindingMaterialUnmappedAccounts) &&
		!hasFindingCode(result.Warnings, closequality.FindingMaterialUnmappedAccounts) {
		t.Errorf("expected FindingMaterialUnmappedAccounts among blockers or warnings, got blockers=%+v warnings=%+v",
			result.Blockers, result.Warnings)
	}
}

// TestScenario_CloseChecklistIncomplete proves a required, not-started
// close task produces FindingRequiredCloseTaskIncomplete and, under the
// default policy (RequiredIncompleteBlocks=false), a WARNING rather than
// a blocker.
func TestScenario_CloseChecklistIncomplete(t *testing.T) {
	in := fixtures.CleanCloseInput()
	in.CloseTasks = fixtures.IncompleteCloseTasks()

	result := closequality.Calculate(in, fixtures.CleanPolicy())

	if !hasFindingCode(result.Warnings, closequality.FindingRequiredCloseTaskIncomplete) {
		t.Errorf("expected FindingRequiredCloseTaskIncomplete among warnings, got %+v", result.Warnings)
	}
	if result.CloseTaskSummary.RequiredCount != 1 || result.CloseTaskSummary.RequiredCompletedCount != 0 {
		t.Errorf("unexpected close task summary: %+v", result.CloseTaskSummary)
	}
}

// TestScenario_CloseChecklistBlocked proves a BLOCKED required close
// task always produces a BLOCKING finding regardless of
// RequiredIncompleteBlocks.
func TestScenario_CloseChecklistBlocked(t *testing.T) {
	in := fixtures.CleanCloseInput()
	in.CloseTasks = fixtures.BlockedCloseTasks()

	result := closequality.Calculate(in, fixtures.CleanPolicy())

	if result.Status != closequality.StatusNotReady {
		t.Errorf("status = %s, want NOT_READY", result.Status)
	}
	if !hasFindingCode(result.Blockers, closequality.FindingRequiredCloseTaskBlocked) {
		t.Errorf("expected FindingRequiredCloseTaskBlocked among blockers, got %+v", result.Blockers)
	}
}

// TestScenario_SuspenseAccountUncleared proves a ShouldClear account with
// a nonzero ending balance produces FindingClearingAccountNotCleared.
func TestScenario_SuspenseAccountUncleared(t *testing.T) {
	in := fixtures.CleanCloseInput()
	policy := fixtures.CleanPolicy()
	policy.AccountExpectations = fixtures.SuspenseAccountExpectations()

	// Account 2100 (declared ShouldClear) has a zero ending balance in
	// the clean ledger, so add activity to leave it nonzero.
	in.Ledger.Entries = append(in.Ledger.Entries, unclearedSuspenseEntry())

	result := closequality.Calculate(in, policy)

	if !hasFindingCode(result.Warnings, closequality.FindingClearingAccountNotCleared) &&
		!hasFindingCode(result.Blockers, closequality.FindingClearingAccountNotCleared) {
		t.Errorf("expected FindingClearingAccountNotCleared, got warnings=%+v blockers=%+v", result.Warnings, result.Blockers)
	}
}

// TestScenario_StaleClearingBalance proves a ShouldClear account with a
// matching stale BalanceAge produces FindingStaleAccountBalance.
func TestScenario_StaleClearingBalance(t *testing.T) {
	in := fixtures.CleanCloseInput()
	policy := fixtures.CleanPolicy()
	maxAge := 30
	policy.AccountExpectations = []closequality.AccountExpectation{
		{AccountID: "2100", ShouldClear: true, MaxAgeDays: &maxAge, Label: "suspense"},
	}
	policy.BalanceAges = fixtures.StaleSuspenseBalanceAge()
	in.Ledger.Entries = append(in.Ledger.Entries, unclearedSuspenseEntry())

	result := closequality.Calculate(in, policy)

	if !hasFindingCode(result.Warnings, closequality.FindingStaleAccountBalance) {
		t.Errorf("expected FindingStaleAccountBalance among warnings, got %+v", result.Warnings)
	}
}

// TestScenario_MissingRequiredAR proves ARRequired=true with no AR
// result supplied produces NOT_READY via FindingMissingRequiredInput.
func TestScenario_MissingRequiredAR(t *testing.T) {
	in := fixtures.CleanCloseInput()
	in.AR = arZeroResult()
	policy := fixtures.CleanPolicy()

	result := closequality.Calculate(in, policy)

	if result.Status != closequality.StatusNotReady {
		t.Errorf("status = %s, want NOT_READY", result.Status)
	}
}

// TestScenario_ARNotApplicable proves ARRequired=false with no AR result
// supplied never penalizes readiness.
func TestScenario_ARNotApplicable(t *testing.T) {
	in := fixtures.CleanCloseInput()
	in.AR = arZeroResult()
	policy := fixtures.CleanPolicy()
	policy.Applicability.ARRequired = false

	result := closequality.Calculate(in, policy)

	if hasFindingCode(result.Blockers, closequality.FindingMissingRequiredInput) {
		t.Errorf("did not expect FindingMissingRequiredInput blocker when AR is not required, got %+v", result.Blockers)
	}
}

// TestScenario_WarningsOnly_IsReadyWithWarnings proves a period with only
// non-blocking findings reports READY_WITH_WARNINGS, not READY or
// NOT_READY.
func TestScenario_WarningsOnly_IsReadyWithWarnings(t *testing.T) {
	in := fixtures.CleanCloseInput()
	in.CloseTasks = fixtures.IncompleteCloseTasks() // required, not started -> WARNING by default policy

	result := closequality.Calculate(in, fixtures.CleanPolicy())

	if result.Status != closequality.StatusReadyWithWarnings {
		t.Errorf("status = %s, want READY_WITH_WARNINGS; blockers=%+v warnings=%+v", result.Status, result.Blockers, result.Warnings)
	}
}

// unclearedSuspenseEntry posts a balanced entry that leaves account 2100
// (Accrued Payroll, repurposed as a suspense/clearing account in these
// fixtures) with a nonzero ending balance.
func unclearedSuspenseEntry() ledger.JournalEntry {
	return ledger.JournalEntry{
		ID: "SVC-JE-SUSPENSE", Date: "2025-01-20", Period: "2025-01", Status: ledger.StatusPosted,
		Description: "Payroll accrual not yet cleared", Source: "manual",
		Lines: []ledger.JournalLine{
			{ID: "L1", AccountID: "6000", Debit: 500},
			{ID: "L2", AccountID: "2100", Credit: 500},
		},
	}
}

// arZeroResult returns the zero-value ar.Result, representing "AR not
// supplied" (Available is false).
func arZeroResult() ar.Result { return ar.Result{} }
