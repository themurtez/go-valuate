package vendorspend_test

import (
	"strings"
	"testing"

	"github.com/themurtez/go-valuate/accounting/vendorspend"
)

// forbiddenPhrases are the words/phrases task section 38 explicitly
// prohibits anywhere in a Flag/Issue Message. Checked case-insensitively
// so "Overcharging" or "FRAUD" are caught too.
var forbiddenPhrases = []string{
	"overcharg",
	"bad vendor",
	"replace vendor",
	"negotiate hard",
	"is risky",
	"supplier is risky",
	"fraud",
}

// TestNeutralLanguage_NoForbiddenWordsInFlagsOrIssues is a permanent
// regression test — task section 38. Runs across every fixture scenario
// this package ships, not just one Calculate call, since a future flag
// added by a different section of the codebase could reintroduce
// forbidden language without this test's coverage broadening too.
func TestNeutralLanguage_NoForbiddenWordsInFlagsOrIssues(t *testing.T) {
	for _, scenario := range allFixtureResults(t) {
		for _, f := range scenario.result.Flags {
			checkNeutral(t, scenario.name, "Flag["+string(f.Code)+"]", f.Message)
		}
		for _, is := range scenario.result.Issues {
			checkNeutral(t, scenario.name, "Issue["+string(is.Code)+"]", is.Message)
		}
	}
}

func checkNeutral(t *testing.T, scenario, label, message string) {
	t.Helper()
	lower := strings.ToLower(message)
	for _, forbidden := range forbiddenPhrases {
		if strings.Contains(lower, forbidden) {
			t.Errorf("%s: %s message contains forbidden phrase %q: %q", scenario, label, forbidden, message)
		}
	}
}

// TestNeutralLanguage_DuplicateFlagDoesNotAccuseFraud specifically checks
// FlagPossibleDuplicateSpend's own message never implies fraud or error —
// task section 22's "no fraud language" rule, the single flag in this
// package most likely to be miswritten toward accusatory language.
func TestNeutralLanguage_DuplicateFlagDoesNotAccuseFraud(t *testing.T) {
	for _, scenario := range allFixtureResults(t) {
		for _, f := range scenario.result.Flags {
			if f.Code != vendorspend.FlagPossibleDuplicateSpend {
				continue
			}
			lower := strings.ToLower(f.Message)
			for _, bad := range []string{"fraud", "error", "mistake", "duplicate entry error", "accidental"} {
				if strings.Contains(lower, bad) {
					t.Errorf("%s: POSSIBLE_DUPLICATE_SPEND message contains %q: %q", scenario.name, bad, f.Message)
				}
			}
		}
	}
}
