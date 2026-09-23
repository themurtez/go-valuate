package journaldiagnostics

import (
	"sort"
	"strings"

	"github.com/themurtez/go-valuate/accounting/ledger"
)

// accountCombinationKey builds a deterministic representation of e's
// account combination per section 34: sorted debit account IDs, then
// sorted credit account IDs. Two entries with the same accounts but
// different amounts share a key; an entry line with both/neither debit and
// credit set (a ledger-validation error already caught separately) does not
// contribute to either side.
func accountCombinationKey(e ledger.JournalEntry) string {
	var debits, credits []string
	for _, l := range e.Lines {
		switch {
		case l.Debit > 0 && l.Credit == 0:
			debits = append(debits, l.AccountID)
		case l.Credit > 0 && l.Debit == 0:
			credits = append(credits, l.AccountID)
		}
	}
	sort.Strings(debits)
	sort.Strings(credits)
	return strings.Join(debits, ",") + ">" + strings.Join(credits, ",")
}

// findRareAccountCombinationFindings implements section 34: identifies
// entries using an account combination that occurs rarely within the
// supplied population. Requires policy.MinBaselineObservations worth of
// combination history to exist at all before this rule considers itself
// available (a small population makes every combination look "rare" by
// construction, which is not a meaningful signal — see the package doc's
// "do not flag first-use combination as statistically unusual unless
// policy explicitly requests it" instruction: this V1 rule requires
// materiality too, so first-use alone is never sufficient).
func findRareAccountCombinationFindings(all []analyzedEntry, policy Policy) (findings []Finding, state RuleState) {
	if policy.MaterialAmount <= 0 {
		return nil, RuleDisabled
	}
	if len(all) < policy.MinBaselineObservations {
		return nil, RuleUnavailable
	}

	counts := make(map[string]int)
	for _, a := range all {
		counts[accountCombinationKey(a.entry)]++
	}

	for _, a := range all {
		if a.magnitude < policy.MaterialAmount {
			continue
		}
		key := accountCombinationKey(a.entry)
		if key == ">" {
			continue // no clean debit/credit lines to key on
		}
		if counts[key] > policy.RareAccountMaxHistoricalEntries {
			continue
		}
		findings = append(findings, Finding{
			Code: FindingRareAccountCombination, Severity: SeverityInfo,
			Period: a.period, EntryIDs: []string{a.entry.ID}, AccountIDs: entryAccountIDs(a.entry),
			Amount: AvailableValue(a.magnitude),
			Evidence: Evidence{
				EntryAmount: AvailableValue(a.magnitude), EntryDate: a.date,
				HistoricalUseCount: AvailableValue(float64(counts[key])),
				DebitAccountIDs:    combinationSide(key, 0),
				CreditAccountIDs:   combinationSide(key, 1),
			},
			Message: msgRareAccountCombination,
		})
	}
	return findings, RuleAvailable
}

// combinationSide extracts the debit (side==0) or credit (side==1) account
// list from an accountCombinationKey-formatted string.
func combinationSide(key string, side int) []string {
	parts := strings.SplitN(key, ">", 2)
	if len(parts) != 2 {
		return nil
	}
	s := parts[side]
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}
