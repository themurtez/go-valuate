package closequality_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/closequality"
	"github.com/themurtez/go-valuate/accounting/closequality/fixtures"
	"github.com/themurtez/go-valuate/accounting/ledger"
)

// TestExpectedActivity_MissingActivity_ProducesFinding proves an
// ExpectedActivityRule naming an account with no period activity
// produces FindingExpectedPeriodEntryMissing.
func TestExpectedActivity_MissingActivity_ProducesFinding(t *testing.T) {
	in := fixtures.CleanCloseInput()
	policy := fixtures.CleanPolicy()
	policy.ExpectedActivityRules = []closequality.ExpectedActivityRule{
		{Label: "payroll clearing must move", AccountIDs: []string{"2100"}, MinEntryCount: 1},
	}

	result := closequality.Calculate(in, policy)

	if !hasFindingCode(result.Warnings, closequality.FindingExpectedPeriodEntryMissing) {
		t.Errorf("expected FindingExpectedPeriodEntryMissing among warnings, got %+v", result.Warnings)
	}
}

// TestExpectedActivity_ActivityPresent_NoFinding proves a rule
// satisfied by real activity in the fixture ledger produces no finding.
func TestExpectedActivity_ActivityPresent_NoFinding(t *testing.T) {
	in := fixtures.CleanCloseInput()
	policy := fixtures.CleanPolicy()
	policy.ExpectedActivityRules = []closequality.ExpectedActivityRule{
		{Label: "cash must move", AccountIDs: []string{"1000"}, MinEntryCount: 1},
	}

	result := closequality.Calculate(in, policy)

	if hasFindingCode(result.Warnings, closequality.FindingExpectedPeriodEntryMissing) {
		t.Errorf("did not expect FindingExpectedPeriodEntryMissing, got %+v", result.Warnings)
	}
}

// TestExpectedActivity_Unavailable_NoLedger proves the rule produces no
// finding when no ledger is supplied (the check is genuinely
// unavailable, not "missing").
func TestExpectedActivity_Unavailable_NoLedger(t *testing.T) {
	in := fixtures.CleanCloseInput()
	in.LedgerProvided = false
	in.Ledger = ledger.Ledger{}
	in.LedgerValidation = []ledger.Issue{} // keep ledger dimensions assessed-but-empty, unrelated to this check
	policy := fixtures.CleanPolicy()
	policy.ExpectedActivityRules = []closequality.ExpectedActivityRule{
		{Label: "payroll clearing must move", AccountIDs: []string{"2100"}, MinEntryCount: 1},
	}

	result := closequality.Calculate(in, policy)

	if hasFindingCode(result.Warnings, closequality.FindingExpectedPeriodEntryMissing) {
		t.Errorf("did not expect a finding when no ledger is supplied, got %+v", result.Warnings)
	}
}

// TestExpectedPeriodEntry_Found proves an ExpectedPeriodEntry rule
// matched by real ledger activity (account + exact normalized
// description) produces no finding.
func TestExpectedPeriodEntry_Found(t *testing.T) {
	in := fixtures.CleanCloseInput()
	policy := fixtures.CleanPolicy()
	policy.ExpectedPeriodEntries = []closequality.ExpectedPeriodEntry{
		{Label: "month-end close", AccountIDs: []string{"3900"}, DescriptionExact: "  Month-end Close: Net Income To Retained Earnings  "},
	}

	result := closequality.Calculate(in, policy)

	if hasFindingCode(result.Warnings, closequality.FindingExpectedPeriodEntryMissing) {
		t.Errorf("did not expect FindingExpectedPeriodEntryMissing, got %+v", result.Warnings)
	}
}

// TestExpectedPeriodEntry_NotFound proves a rule with no matching entry
// produces FindingExpectedPeriodEntryMissing.
func TestExpectedPeriodEntry_NotFound(t *testing.T) {
	in := fixtures.CleanCloseInput()
	policy := fixtures.CleanPolicy()
	policy.ExpectedPeriodEntries = []closequality.ExpectedPeriodEntry{
		{Label: "monthly depreciation", AccountIDs: []string{"1600"}, DescriptionExact: "Monthly depreciation"},
	}

	result := closequality.Calculate(in, policy)

	if !hasFindingCode(result.Warnings, closequality.FindingExpectedPeriodEntryMissing) {
		t.Errorf("expected FindingExpectedPeriodEntryMissing among warnings, got %+v", result.Warnings)
	}
}

// TestExpectedPeriodEntry_UnevaluableRule_NoFinding proves a rule with
// no usable criteria (no account IDs, no source, no description) is
// unavailable rather than treated as unmet.
func TestExpectedPeriodEntry_UnevaluableRule_NoFinding(t *testing.T) {
	in := fixtures.CleanCloseInput()
	policy := fixtures.CleanPolicy()
	policy.ExpectedPeriodEntries = []closequality.ExpectedPeriodEntry{
		{Label: "no criteria at all"},
	}

	result := closequality.Calculate(in, policy)

	if hasFindingCode(result.Warnings, closequality.FindingExpectedPeriodEntryMissing) {
		t.Errorf("did not expect a finding for an unevaluable rule, got %+v", result.Warnings)
	}
}

// TestExpectedPeriodEntry_AmountRange proves MinAmount/MaxAmount
// filtering participates in matching.
func TestExpectedPeriodEntry_AmountRange(t *testing.T) {
	minAmt, maxAmt := 100.0, 500.0
	in := fixtures.CleanCloseInput()
	policy := fixtures.CleanPolicy()
	policy.ExpectedPeriodEntries = []closequality.ExpectedPeriodEntry{
		{Label: "small owner draw", AccountIDs: []string{"3000"}, MinAmount: &minAmt, MaxAmount: &maxAmt},
	}

	result := closequality.Calculate(in, policy)

	// The fixture's only entry touching account 3000 is the 50000 owner
	// contribution, well outside [100, 500], so this should be reported
	// missing.
	if !hasFindingCode(result.Warnings, closequality.FindingExpectedPeriodEntryMissing) {
		t.Errorf("expected FindingExpectedPeriodEntryMissing for out-of-range amount, got %+v", result.Warnings)
	}
}
