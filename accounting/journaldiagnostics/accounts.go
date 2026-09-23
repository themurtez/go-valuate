package journaldiagnostics

import "github.com/themurtez/go-valuate/accounting/ledger"

// accountHistoryCount returns, per account touched by all, how many
// distinct analyzed entries touch it — the "historical use count" rare/new-
// account detection reads. Same full-period batch convention as
// buildAccountBaselines (see its doc comment) — this package works from a
// supplied historical slice, not a running feed, so "prior" means "within
// the supplied population" throughout.
func accountHistoryCount(all []analyzedEntry) map[string]int {
	out := make(map[string]int)
	for _, a := range all {
		seen := make(map[string]bool, len(a.entry.Lines))
		for _, l := range a.entry.Lines {
			if seen[l.AccountID] {
				continue
			}
			seen[l.AccountID] = true
			out[l.AccountID]++
		}
	}
	return out
}

// findRareAndNewAccountFindings implements sections 18-19: a material entry
// (>= policy.MaterialAmount) touching an account whose *other* historical
// entry count (this account's total count minus the current entry) is 0
// counts as FindingNewAccountActivity; an account whose other-historical
// count is > 0 but <= policy.RareAccountMaxHistoricalEntries counts as
// FindingRareAccountActivity. Kept as two separate findings per the task's
// explicit "keep separate from rare-account logic" instruction. Unavailable
// when policy.MaterialAmount is not configured (never "flag every first-use
// account" without a materiality gate).
func findRareAndNewAccountFindings(all []analyzedEntry, policy Policy) (findings []Finding, state RuleState) {
	if policy.MaterialAmount <= 0 {
		return nil, RuleDisabled
	}
	counts := accountHistoryCount(all)

	for _, a := range all {
		if a.magnitude < policy.MaterialAmount {
			continue
		}
		seen := make(map[string]bool, len(a.entry.Lines))
		for _, l := range a.entry.Lines {
			if seen[l.AccountID] {
				continue
			}
			seen[l.AccountID] = true
			otherCount := counts[l.AccountID] - 1 // exclude this entry's own contribution
			switch {
			case otherCount <= 0:
				findings = append(findings, Finding{
					Code: FindingNewAccountActivity, Severity: SeverityWarning,
					Period: a.period, EntryIDs: []string{a.entry.ID}, AccountIDs: []string{l.AccountID},
					Amount: AvailableValue(a.magnitude),
					Evidence: Evidence{
						EntryAmount: AvailableValue(a.magnitude), EntryDate: a.date,
						HistoricalUseCount: AvailableValue(0),
					},
					Message: msgNewAccountActivity,
				})
			case otherCount <= policy.RareAccountMaxHistoricalEntries:
				findings = append(findings, Finding{
					Code: FindingRareAccountActivity, Severity: SeverityInfo,
					Period: a.period, EntryIDs: []string{a.entry.ID}, AccountIDs: []string{l.AccountID},
					Amount: AvailableValue(a.magnitude),
					Evidence: Evidence{
						EntryAmount: AvailableValue(a.magnitude), EntryDate: a.date,
						HistoricalUseCount: AvailableValue(float64(otherCount)),
					},
					Message: msgRareAccountActivity,
				})
			}
		}
	}
	return findings, RuleAvailable
}

// lineMovesAgainstNormalBalance reports whether line's net movement on
// account.Type opposes NormalBalance(account.Type) — a debit on a natural-
// credit account (LIABILITY/EQUITY/REVENUE) or a credit on a natural-debit
// account (ASSET/EXPENSE). Ignores a line where both/neither of
// Debit/Credit is set (ledger.ValidateEntries already flags that
// structurally; this function only judges lines that clearly move one way).
func lineMovesAgainstNormalBalance(line ledger.JournalLine, acctType ledger.AccountType) bool {
	normal, ok := ledger.NormalBalance(acctType)
	if !ok {
		return false
	}
	movesDebit := line.Debit > 0 && line.Credit == 0
	movesCredit := line.Credit > 0 && line.Debit == 0
	switch normal {
	case ledger.Debit:
		return movesCredit
	case ledger.Credit:
		return movesDebit
	default:
		return false
	}
}

// findOppositeNormalBalanceFindings implements section 20: flags a material
// line that moves an account against its normal balance side. This is a
// review indicator only — legitimate returns, corrections, reversals, and
// reclasses regularly produce this pattern (see msgOppositeNormalBalanceMovement's
// neutral wording). An entry that is itself an explicit reversal (per
// ledger.Reversal) is excluded, since a reversal moving against normal
// balance is expected by construction, not unusual — see the package doc's
// "exclude or downgrade known reversal entries" instruction.
func findOppositeNormalBalanceFindings(all []analyzedEntry, chart ledger.ChartOfAccounts, policy Policy) (findings []Finding, state RuleState) {
	if policy.MaterialAmount <= 0 {
		return nil, RuleDisabled
	}
	for _, a := range all {
		if a.magnitude < policy.MaterialAmount {
			continue
		}
		if a.entry.Reversal.IsReversal() {
			continue
		}
		var flaggedAccounts []string
		for _, l := range a.entry.Lines {
			acct, ok := chart.Lookup(l.AccountID)
			if !ok {
				continue
			}
			if lineMovesAgainstNormalBalance(l, acct.Type) {
				flaggedAccounts = append(flaggedAccounts, l.AccountID)
			}
		}
		if len(flaggedAccounts) == 0 {
			continue
		}
		findings = append(findings, Finding{
			Code: FindingOppositeNormalBalanceMovement, Severity: SeverityInfo,
			Period: a.period, EntryIDs: []string{a.entry.ID}, AccountIDs: sortStrings(flaggedAccounts),
			Amount:   AvailableValue(a.magnitude),
			Evidence: Evidence{EntryAmount: AvailableValue(a.magnitude), EntryDate: a.date},
			Message:  msgOppositeNormalBalanceMovement,
		})
	}
	return findings, RuleAvailable
}

// findManualRevenueEquityFindings implements section 21: a manual entry
// (per EntryMetadata.Source == SourceManual) with a line touching a
// REVENUE or EQUITY account. Two separate finding codes so a caller can
// adopt one policy without the other — this package never hard-codes
// either as inherently wrong (see msgManualRevenueEntry/
// msgManualEquityEntry's neutral wording).
func findManualRevenueEquityFindings(all []analyzedEntry, chart ledger.ChartOfAccounts) (findings []Finding, state RuleState) {
	anyMeta := false
	for _, a := range all {
		if a.hasMeta && a.meta.Source != "" {
			anyMeta = true
			break
		}
	}
	if !anyMeta {
		return nil, RuleUnavailable
	}

	for _, a := range manualEntries(all) {
		var revenueAccounts, equityAccounts []string
		for _, l := range a.entry.Lines {
			acct, ok := chart.Lookup(l.AccountID)
			if !ok {
				continue
			}
			switch acct.Type {
			case ledger.AccountRevenue:
				revenueAccounts = append(revenueAccounts, l.AccountID)
			case ledger.AccountEquity:
				equityAccounts = append(equityAccounts, l.AccountID)
			}
		}
		if len(revenueAccounts) > 0 {
			findings = append(findings, Finding{
				Code: FindingManualRevenueEntry, Severity: SeverityWarning,
				Period: a.period, EntryIDs: []string{a.entry.ID}, AccountIDs: sortStrings(revenueAccounts),
				Amount:   AvailableValue(a.magnitude),
				Evidence: Evidence{EntryAmount: AvailableValue(a.magnitude), EntryDate: a.date, Source: SourceManual},
				Message:  msgManualRevenueEntry,
			})
		}
		if len(equityAccounts) > 0 {
			findings = append(findings, Finding{
				Code: FindingManualEquityEntry, Severity: SeverityWarning,
				Period: a.period, EntryIDs: []string{a.entry.ID}, AccountIDs: sortStrings(equityAccounts),
				Amount:   AvailableValue(a.magnitude),
				Evidence: Evidence{EntryAmount: AvailableValue(a.magnitude), EntryDate: a.date, Source: SourceManual},
				Message:  msgManualEquityEntry,
			})
		}
	}
	return findings, RuleAvailable
}

// findSensitiveAccountFindings implements section 22: an entry touching any
// account listed in one of policy.SensitiveAccounts. Never inferred from
// account Name/Number — only the caller-supplied AccountReviewPolicy sets
// are consulted.
func findSensitiveAccountFindings(all []analyzedEntry, policy Policy) (findings []Finding, state RuleState) {
	if len(policy.SensitiveAccounts) == 0 {
		return nil, RuleDisabled
	}
	labelByAccount := make(map[string]string)
	for _, rp := range policy.SensitiveAccounts {
		for _, id := range rp.AccountIDs {
			if _, exists := labelByAccount[id]; !exists {
				labelByAccount[id] = rp.Label
			}
		}
	}

	for _, a := range all {
		matchedByLabel := make(map[string][]string) // label -> account IDs
		for _, l := range a.entry.Lines {
			label, ok := labelByAccount[l.AccountID]
			if !ok {
				continue
			}
			matchedByLabel[label] = append(matchedByLabel[label], l.AccountID)
		}
		var labels []string
		for label := range matchedByLabel {
			labels = append(labels, label)
		}
		labels = sortStrings(labels)
		for _, label := range labels {
			findings = append(findings, Finding{
				Code: FindingSensitiveAccountEntry, Severity: SeverityInfo,
				Period: a.period, EntryIDs: []string{a.entry.ID}, AccountIDs: sortStrings(matchedByLabel[label]),
				Amount: AvailableValue(a.magnitude),
				Evidence: Evidence{
					EntryAmount: AvailableValue(a.magnitude), EntryDate: a.date,
					SensitiveAccountLabel: label,
				},
				Message: msgSensitiveAccountEntry,
			})
		}
	}
	return findings, RuleAvailable
}
