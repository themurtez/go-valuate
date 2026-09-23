package journaldiagnostics_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/journaldiagnostics"
	"github.com/themurtez/go-valuate/accounting/journaldiagnostics/fixtures"
	"github.com/themurtez/go-valuate/accounting/ledger"
)

// TestFixtures_LedgerValidatesCleanly sanity-checks that fixtures.Ledger()
// itself is a structurally valid ledger (no SeverityError from
// ledger.ValidateEntries) — a prerequisite for every other fixture-based
// test in this package to be testing diagnostic rules rather than tripping
// over broken fixture data.
func TestFixtures_LedgerValidatesCleanly(t *testing.T) {
	l := fixtures.Ledger()
	issues := l.Validate(ledger.ValidateOptions{})
	if ledger.HasErrors(issues) {
		t.Errorf("fixtures.Ledger() has structural validation errors: %+v", issues)
	}
}

// TestFixtures_AllScenariosCoveredByAtLeastOneFinding cross-checks that
// every FindingCode this package defines is actually exercised by the
// fixture set under fixtures.Policy() — guarding against a fixture (or a
// rule) silently regressing to producing zero findings.
func TestFixtures_AllScenariosCoveredByAtLeastOneFinding(t *testing.T) {
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())

	seen := make(map[journaldiagnostics.FindingCode]bool)
	for _, f := range r.Findings {
		seen[f.Code] = true
	}

	// Codes NOT expected to fire under this fixture set/policy combination
	// (e.g. rules gated by data this fixture set deliberately omits, or
	// codes that are genuinely absent because nothing in the fixture set
	// matches).
	notExpected := map[journaldiagnostics.FindingCode]bool{}

	allCodes := []journaldiagnostics.FindingCode{
		journaldiagnostics.FindingMaterialManualEntry, journaldiagnostics.FindingMaterialPeriodEndEntry,
		journaldiagnostics.FindingPostCloseEntry, journaldiagnostics.FindingWeekendEntry,
		journaldiagnostics.FindingOutsideBusinessHours, journaldiagnostics.FindingRoundDollarEntry,
		journaldiagnostics.FindingLargeEntry, journaldiagnostics.FindingAccountRelativeLargeEntry,
		journaldiagnostics.FindingRareAccountActivity, journaldiagnostics.FindingNewAccountActivity,
		journaldiagnostics.FindingOppositeNormalBalanceMovement, journaldiagnostics.FindingManualEquityEntry,
		journaldiagnostics.FindingSensitiveAccountEntry, journaldiagnostics.FindingExactDuplicateEntry,
		journaldiagnostics.FindingPossibleDuplicateEntry, journaldiagnostics.FindingRepeatedIdenticalAmount,
		journaldiagnostics.FindingRapidReversal, journaldiagnostics.FindingCrossPeriodReversal,
		journaldiagnostics.FindingPeriodEndEntryWithEarlyReversal, journaldiagnostics.FindingThresholdCluster,
		journaldiagnostics.FindingSplitEntryCluster, journaldiagnostics.FindingBlankDescription,
		journaldiagnostics.FindingGenericDescription, journaldiagnostics.FindingMissingReference,
		journaldiagnostics.FindingSamePreparerApprover, journaldiagnostics.FindingMissingApprover,
		journaldiagnostics.FindingHighVolumeByPreparer, journaldiagnostics.FindingRareAccountCombination,
	}

	for _, code := range allCodes {
		if notExpected[code] {
			continue
		}
		if !seen[code] {
			t.Errorf("expected fixture set to exercise FindingCode %s at least once; got zero", code)
		}
	}
}

// TestFixtures_BrokenLedgerHasNoAdditionalPurposeBeyondValidation confirms
// the one fixture whose entire purpose is exercising the invalid-entry
// exclusion path actually differs from the clean baseline only in that one
// respect.
func TestFixtures_BrokenLedgerHasNoAdditionalPurposeBeyondValidation(t *testing.T) {
	broken := fixtures.BrokenLedgerUnbalancedEntry()
	clean := fixtures.Ledger()
	if len(broken.Entries) != len(clean.Entries)+1 {
		t.Errorf("expected BrokenLedgerUnbalancedEntry to have exactly one more entry than the clean fixture, got %d vs %d",
			len(broken.Entries), len(clean.Entries))
	}
}
