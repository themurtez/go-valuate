package journaldiagnostics_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/journaldiagnostics"
	"github.com/themurtez/go-valuate/accounting/journaldiagnostics/fixtures"
)

func TestRareAccount_DetectsLowHistoryMaterialPosting(t *testing.T) {
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	found := findingsWithCode(r.Findings, journaldiagnostics.FindingRareAccountActivity)
	matched := false
	for _, f := range found {
		if containsEntryID(f, "JE-RAREACCT-01") {
			matched = true
			if !f.Evidence.HistoricalUseCount.Available {
				t.Error("expected HistoricalUseCount evidence to be available")
			}
		}
	}
	if !matched {
		t.Error("expected JE-RAREACCT-01 to be flagged as rare-account activity")
	}
}

func TestNewAccount_DetectsFirstEverMaterialPosting(t *testing.T) {
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	found := findingsWithCode(r.Findings, journaldiagnostics.FindingNewAccountActivity)
	matched := false
	for _, f := range found {
		if containsEntryID(f, "JE-NEWACCT-01") {
			matched = true
			if f.Evidence.HistoricalUseCount.Amount != 0 {
				t.Errorf("expected HistoricalUseCount == 0 for new-account activity, got %v", f.Evidence.HistoricalUseCount.Amount)
			}
		}
	}
	if !matched {
		t.Error("expected JE-NEWACCT-01 to be flagged as new-account activity")
	}
}

func TestRareAndNewAccount_KeptAsSeparateCodes(t *testing.T) {
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	for _, f := range findingsWithCode(r.Findings, journaldiagnostics.FindingRareAccountActivity) {
		if containsEntryID(f, "JE-NEWACCT-01") {
			t.Error("JE-NEWACCT-01 should be NEW_ACCOUNT_ACTIVITY, not also RARE_ACCOUNT_ACTIVITY")
		}
	}
	for _, f := range findingsWithCode(r.Findings, journaldiagnostics.FindingNewAccountActivity) {
		if containsEntryID(f, "JE-RAREACCT-01") {
			t.Error("JE-RAREACCT-01 should be RARE_ACCOUNT_ACTIVITY, not also NEW_ACCOUNT_ACTIVITY")
		}
	}
}

func TestRareNewAccount_DisabledWithoutMaterialAmount(t *testing.T) {
	policy := fixtures.Policy()
	policy.MaterialAmount = 0
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), policy)
	if r.RuleAvailability.RareAccountActivity != journaldiagnostics.RuleDisabled {
		t.Errorf("expected RareAccountActivity DISABLED without MaterialAmount, got %s", r.RuleAvailability.RareAccountActivity)
	}
}

func TestOppositeNormalBalance_DetectsRevenueAccountDebited(t *testing.T) {
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	found := findingsWithCode(r.Findings, journaldiagnostics.FindingOppositeNormalBalanceMovement)
	matched := false
	for _, f := range found {
		if containsEntryID(f, "JE-OPPOSITE-01") {
			matched = true
			hasRevenue := false
			for _, acct := range f.AccountIDs {
				if acct == "4000" {
					hasRevenue = true
				}
			}
			if !hasRevenue {
				t.Errorf("expected account 4000 (revenue, debited) in AccountIDs, got %v", f.AccountIDs)
			}
		}
	}
	if !matched {
		t.Error("expected JE-OPPOSITE-01 (revenue account debited) to be flagged")
	}
}

func TestOppositeNormalBalance_ExcludesExplicitReversals(t *testing.T) {
	// JE-REV-REV-01 (an explicit reversal) credits 6200 (an EXPENSE
	// account), which is a credit movement against EXPENSE's normal DEBIT
	// side — but since it's an explicit reversal, it must be excluded from
	// this rule.
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	for _, f := range findingsWithCode(r.Findings, journaldiagnostics.FindingOppositeNormalBalanceMovement) {
		if containsEntryID(f, "JE-REV-REV-01") {
			t.Error("an explicit reversal entry must be excluded from OPPOSITE_NORMAL_BALANCE_MOVEMENT")
		}
	}
}

func TestManualRevenueEquity_DetectsManualEquityEntry(t *testing.T) {
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	found := findingsWithCode(r.Findings, journaldiagnostics.FindingManualEquityEntry)
	matched := false
	for _, f := range found {
		if containsEntryID(f, "JE-EQUITY-01") {
			matched = true
		}
	}
	if !matched {
		t.Error("expected JE-EQUITY-01 (manual entry touching equity account 3900) to be flagged as MANUAL_EQUITY_ENTRY")
	}
}

func TestManualRevenueEquity_NotHardCodedAsWrong(t *testing.T) {
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	for _, f := range findingsWithCode(r.Findings, journaldiagnostics.FindingManualEquityEntry) {
		if f.Severity == journaldiagnostics.SeverityHigh {
			t.Errorf("MANUAL_EQUITY_ENTRY must not be treated as inherently high-severity/wrong by default, got severity %s", f.Severity)
		}
	}
}

func TestSensitiveAccount_DetectsCallerDesignatedAccount(t *testing.T) {
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	found := findingsWithCode(r.Findings, journaldiagnostics.FindingSensitiveAccountEntry)
	matched := false
	for _, f := range found {
		if containsEntryID(f, "JE-ISOLATED-01") {
			matched = true
			if f.Evidence.SensitiveAccountLabel != "Suspense accounts" {
				t.Errorf("expected SensitiveAccountLabel 'Suspense accounts', got %q", f.Evidence.SensitiveAccountLabel)
			}
		}
	}
	if !matched {
		t.Error("expected JE-ISOLATED-01 (touches suspense account 1500) to be flagged as sensitive-account entry")
	}
}

func TestSensitiveAccount_NeverInferredFromName(t *testing.T) {
	// No SensitiveAccounts policy supplied at all: even though account 1500
	// is literally named "Suspense Account", it must not be flagged.
	policy := fixtures.Policy()
	policy.SensitiveAccounts = nil
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), policy)
	if r.RuleAvailability.SensitiveAccountEntry != journaldiagnostics.RuleDisabled {
		t.Errorf("expected SensitiveAccountEntry DISABLED without a policy, got %s", r.RuleAvailability.SensitiveAccountEntry)
	}
	if len(findingsWithCode(r.Findings, journaldiagnostics.FindingSensitiveAccountEntry)) != 0 {
		t.Error("expected zero SENSITIVE_ACCOUNT_ENTRY findings without an explicit policy, regardless of account naming")
	}
}
