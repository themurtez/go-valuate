package journaldiagnostics_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/journaldiagnostics"
	"github.com/themurtez/go-valuate/accounting/journaldiagnostics/fixtures"
)

func TestRoundDollar_DetectsRoundAmount(t *testing.T) {
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	found := findingsWithCode(r.Findings, journaldiagnostics.FindingRoundDollarEntry)
	matched := false
	for _, f := range found {
		if containsEntryID(f, "JE-ROUND-01") {
			matched = true
			if !f.Evidence.RoundBase.Available || f.Evidence.RoundBase.Amount != 10000 {
				t.Errorf("expected RoundBase evidence == 10000, got %+v", f.Evidence.RoundBase)
			}
		}
	}
	if !matched {
		t.Error("expected JE-ROUND-01 ($10,000) to be flagged as round-dollar")
	}
}

func TestRoundDollar_DisabledByDefault(t *testing.T) {
	policy := journaldiagnostics.DefaultPolicy()
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), policy)
	if r.RuleAvailability.RoundDollar != journaldiagnostics.RuleDisabled {
		t.Errorf("expected RoundDollar DISABLED when RoundDollarMinAmount is unset, got %s", r.RuleAvailability.RoundDollar)
	}
}

func TestRoundDollar_DoesNotFlagTinyRoundAmounts(t *testing.T) {
	policy := fixtures.Policy()
	// $10 is round (divisible by 10... but our bases are 100/1000/10000) —
	// use an amount below RoundDollarMinAmount that is still evenly
	// divisible by 100 to prove the minimum gate, not the base list, is
	// what excludes it.
	l := fixtures.Ledger()
	l.Entries = append(l.Entries, journalEntryHelper("JE-TINYROUND-01", "2025-03-19", 500))
	meta := append([]journaldiagnostics.EntryMetadata{}, fixtures.Metadata()...)

	r := journaldiagnostics.Calculate(l, meta, fixtures.Window(), policy)
	for _, f := range findingsWithCode(r.Findings, journaldiagnostics.FindingRoundDollarEntry) {
		if containsEntryID(f, "JE-TINYROUND-01") {
			t.Error("$500 is below RoundDollarMinAmount (1000) and must not be flagged even though it is round")
		}
	}
}

func TestRoundDollar_NonRoundAmountNotFlagged(t *testing.T) {
	l := fixtures.Ledger()
	l.Entries = append(l.Entries, journalEntryHelper("JE-NONROUND-01", "2025-03-19", 12345.67))
	r := journaldiagnostics.Calculate(l, fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	for _, f := range findingsWithCode(r.Findings, journaldiagnostics.FindingRoundDollarEntry) {
		if containsEntryID(f, "JE-NONROUND-01") {
			t.Error("$12,345.67 is not round under bases {100,1000,10000} and must not be flagged")
		}
	}
}

func TestLargeEntryAbsolute_DetectsOverThreshold(t *testing.T) {
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	found := findingsWithCode(r.Findings, journaldiagnostics.FindingLargeEntry)
	matched := false
	for _, f := range found {
		if containsEntryID(f, "JE-LARGE-01") {
			matched = true
		}
	}
	if !matched {
		t.Error("expected JE-LARGE-01 ($50,000 > $40,000 threshold) to be flagged as LARGE_ENTRY")
	}
}

func TestLargeEntryAbsolute_DisabledWhenZero(t *testing.T) {
	policy := fixtures.Policy()
	policy.LargeEntryAbsoluteThreshold = 0
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), policy)
	if r.RuleAvailability.LargeEntryAbsolute != journaldiagnostics.RuleDisabled {
		t.Errorf("expected LargeEntryAbsolute DISABLED when threshold is 0, got %s", r.RuleAvailability.LargeEntryAbsolute)
	}
}

func TestAccountRelativeLargeEntry_DetectsOutlierAgainstBaseline(t *testing.T) {
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	found := findingsWithCode(r.Findings, journaldiagnostics.FindingAccountRelativeLargeEntry)
	matched := false
	for _, f := range found {
		if containsEntryID(f, "JE-LARGE-01") {
			matched = true
			if !f.Evidence.HistoricalMedian.Available || !f.Evidence.MAD.Available || !f.Evidence.ObservationCount.Available {
				t.Errorf("expected full baseline evidence, got %+v", f.Evidence)
			}
		}
	}
	if !matched {
		t.Error("expected JE-LARGE-01 ($50,000 against 6200's small baseline) to be flagged as account-relative outlier")
	}
}

func TestAccountRelativeLargeEntry_InsufficientBaselineSurfacesIssue(t *testing.T) {
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	found := false
	for _, iss := range r.Issues {
		if iss.Code == journaldiagnostics.IssueInsufficientBaseline {
			found = true
		}
	}
	if !found {
		t.Error("expected at least one IssueInsufficientBaseline for a low-activity account in the fixture set")
	}
}

func TestAccountRelativeLargeEntry_UnavailableWithNoQualifyingAccounts(t *testing.T) {
	policy := fixtures.Policy()
	policy.MinBaselineObservations = 1000 // impossibly high, given the fixture population
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), policy)
	if r.RuleAvailability.AccountRelativeLargeEntry != journaldiagnostics.RuleUnavailable {
		t.Errorf("expected AccountRelativeLargeEntry UNAVAILABLE when no account meets the baseline minimum, got %s",
			r.RuleAvailability.AccountRelativeLargeEntry)
	}
}

func TestAccountRelativeLargeEntry_FirstEverActivityNotFlaggedAsOutlier(t *testing.T) {
	// JE-NEWACCT-01 touches two accounts: 6999 (its first-ever activity,
	// n=1) and 1000/Cash (which has a large, unrelated baseline from every
	// other fixture entry). The entry legitimately CAN be flagged against
	// Cash — this test's actual claim is narrower: account 6999 itself must
	// never be the AccountIDs target of an ACCOUNT_RELATIVE_LARGE_ENTRY
	// finding, since its own baseline (n=1) is below MinBaselineObservations.
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	for _, f := range findingsWithCode(r.Findings, journaldiagnostics.FindingAccountRelativeLargeEntry) {
		for _, acct := range f.AccountIDs {
			if acct == "6999" {
				t.Errorf("account 6999 has only n=1 historical observation (its first-ever activity) and must never itself be the flagged account for ACCOUNT_RELATIVE_LARGE_ENTRY; got finding %+v", f)
			}
		}
	}

	foundInsufficientFor6999 := false
	for _, iss := range r.Issues {
		if iss.Code == journaldiagnostics.IssueInsufficientBaseline && iss.AccountID == "6999" {
			foundInsufficientFor6999 = true
		}
	}
	if !foundInsufficientFor6999 {
		t.Error("expected IssueInsufficientBaseline for account 6999 (n=1 < MinBaselineObservations)")
	}
}
