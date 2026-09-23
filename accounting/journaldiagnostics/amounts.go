package journaldiagnostics

import (
	"math"
	"sort"
)

// roundDollarEpsilon is the tolerance (in dollars) a magnitude's remainder
// against a round-dollar base must fall within to still count as "evenly
// divisible" — absorbs ordinary floating-point representation error (e.g.
// an entry summed to 999.9999999999999 instead of exactly 1000) without
// treating a genuinely non-round amount (e.g. 1000.50) as round.
const roundDollarEpsilon = 0.005

// largestDivisor returns the largest base in bases that amount is evenly
// divisible by (within roundDollarEpsilon), and true if any base matched.
func largestDivisor(amount float64, bases []float64) (base float64, ok bool) {
	sorted := sortedFloat64s(bases)
	for i := len(sorted) - 1; i >= 0; i-- {
		b := sorted[i]
		if b <= 0 {
			continue
		}
		rem := math.Mod(amount, b)
		if rem > b/2 {
			rem -= b
		}
		if math.Abs(rem) <= roundDollarEpsilon {
			return b, true
		}
	}
	return 0, false
}

// findRoundDollarFindings implements section 14. RoundDollarMinAmount == 0
// disables the rule entirely (never "flag everything").
func findRoundDollarFindings(all []analyzedEntry, policy Policy) (findings []Finding, state RuleState) {
	if policy.RoundDollarMinAmount <= 0 || len(policy.RoundDollarBases) == 0 {
		return nil, RuleDisabled
	}
	for _, a := range all {
		if a.magnitude < policy.RoundDollarMinAmount {
			continue
		}
		base, ok := largestDivisor(a.magnitude, policy.RoundDollarBases)
		if !ok {
			continue
		}
		findings = append(findings, Finding{
			Code: FindingRoundDollarEntry, Severity: SeverityInfo,
			Period: a.period, EntryIDs: []string{a.entry.ID}, AccountIDs: entryAccountIDs(a.entry),
			Amount:   AvailableValue(a.magnitude),
			Evidence: Evidence{EntryAmount: AvailableValue(a.magnitude), RoundBase: AvailableValue(base), EntryDate: a.date},
			Message:  msgRoundDollarEntry,
		})
	}
	return findings, RuleAvailable
}

// findLargeEntryAbsoluteFindings implements the fixed-threshold half of
// section 15: LargeEntryAbsoluteThreshold == 0 disables the rule.
func findLargeEntryAbsoluteFindings(all []analyzedEntry, policy Policy) (findings []Finding, state RuleState) {
	if policy.LargeEntryAbsoluteThreshold <= 0 {
		return nil, RuleDisabled
	}
	for _, a := range all {
		if a.magnitude <= policy.LargeEntryAbsoluteThreshold {
			continue
		}
		findings = append(findings, Finding{
			Code: FindingLargeEntry, Severity: SeverityWarning,
			Period: a.period, EntryIDs: []string{a.entry.ID}, AccountIDs: entryAccountIDs(a.entry),
			Amount: AvailableValue(a.magnitude),
			Evidence: Evidence{
				EntryAmount: AvailableValue(a.magnitude), EntryDate: a.date,
				Threshold: AvailableValue(policy.LargeEntryAbsoluteThreshold),
			},
			Message: msgLargeEntry,
		})
	}
	return findings, RuleAvailable
}

// accountBaseline is one account's historical-magnitude baseline (every
// prior analyzed entry touching that account, before the entry currently
// being tested), used by the median+MAD account-relative large-entry rule.
type accountBaseline struct {
	median float64
	mad    float64
	n      int
}

// buildAccountBaselines computes, for every account touched by all, the
// full-period baseline of per-entry magnitudes for entries touching that
// account — see the package doc's "account-relative unusual amount"
// section (17). A single entry touching multiple accounts (e.g. a 3-line
// entry) contributes its EntryMagnitude to each touched account's
// distribution; this mirrors "the entry's size relative to this account's
// normal transaction size," which is what the account actually
// experienced, not a per-line split.
//
// This is a whole-period baseline (not strictly point-in-time "before this
// entry"): with journal-entry-level data (not a running ledger feed), the
// task's own median+MAD method is inherently a batch statistic over the
// supplied history, and excluding "the entry itself" from its own
// account's baseline for every comparison would require an O(n) baseline
// recompute per entry per account — the same tradeoff
// analytics/benchmarks and analytics/anomalies' peer-comparison baselines
// already make (see their FormulaVersion-governed batch-statistic
// convention). Documented here once rather than repeated per caller.
func buildAccountBaselines(all []analyzedEntry) map[string]accountBaseline {
	byAccount := make(map[string][]float64)
	for _, a := range all {
		seen := make(map[string]bool, len(a.entry.Lines))
		for _, l := range a.entry.Lines {
			if seen[l.AccountID] {
				continue
			}
			seen[l.AccountID] = true
			byAccount[l.AccountID] = append(byAccount[l.AccountID], a.magnitude)
		}
	}
	out := make(map[string]accountBaseline, len(byAccount))
	for acct, vals := range byAccount {
		med := median(vals)
		out[acct] = accountBaseline{median: med, mad: mad(vals, med), n: len(vals)}
	}
	return out
}

// median returns the median of vs (does not mutate vs).
func median(vs []float64) float64 {
	if len(vs) == 0 {
		return 0
	}
	sorted := sortedFloat64s(vs)
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[mid]
	}
	return (sorted[mid-1] + sorted[mid]) / 2
}

// mad returns the median absolute deviation of vs around med.
func mad(vs []float64, med float64) float64 {
	if len(vs) == 0 {
		return 0
	}
	devs := make([]float64, len(vs))
	for i, v := range vs {
		d := v - med
		if d < 0 {
			d = -d
		}
		devs[i] = d
	}
	return median(devs)
}

// findAccountRelativeLargeEntryFindings implements sections 15-17: for each
// account with at least policy.MinBaselineObservations historical entries,
// flag an entry whose magnitude exceeds median + MADMultiplier*scaledMAD.
// MAD is scaled by 1.4826 (the standard consistency constant that makes MAD
// comparable to a standard deviation under a normal-distribution
// assumption) so MADMultiplier reads on roughly the same scale a
// stddev-based multiplier would. An account's first-ever activity is never
// flagged here (n would be 0, which is always < MinBaselineObservations) —
// see IssueInsufficientBaseline and NewAccountActivity in accounts.go for
// that case instead.
func findAccountRelativeLargeEntryFindings(all []analyzedEntry, policy Policy) (findings []Finding, state RuleState, insufficientAccounts []string) {
	baselines := buildAccountBaselines(all)
	anyBaseline := false
	for _, b := range baselines {
		if b.n >= policy.MinBaselineObservations {
			anyBaseline = true
			break
		}
	}
	if !anyBaseline {
		return nil, RuleUnavailable, nil
	}

	const madScale = 1.4826
	insufficientSet := make(map[string]bool)
	for _, a := range all {
		seen := make(map[string]bool, len(a.entry.Lines))
		for _, l := range a.entry.Lines {
			if seen[l.AccountID] {
				continue
			}
			seen[l.AccountID] = true
			b, ok := baselines[l.AccountID]
			if !ok || b.n < policy.MinBaselineObservations {
				insufficientSet[l.AccountID] = true
				continue
			}
			threshold := b.median + policy.MADMultiplier*b.mad*madScale
			if b.mad == 0 || a.magnitude <= threshold {
				continue
			}
			findings = append(findings, Finding{
				Code: FindingAccountRelativeLargeEntry, Severity: SeverityWarning,
				Period: a.period, EntryIDs: []string{a.entry.ID}, AccountIDs: []string{l.AccountID},
				Amount: AvailableValue(a.magnitude),
				Evidence: Evidence{
					EntryAmount: AvailableValue(a.magnitude), EntryDate: a.date,
					HistoricalMedian: AvailableValue(b.median), MAD: AvailableValue(b.mad),
					ObservationCount: AvailableValue(float64(b.n)), Threshold: AvailableValue(threshold),
				},
				Message: msgAccountRelativeLargeEntry,
			})
		}
	}
	for acct := range insufficientSet {
		insufficientAccounts = append(insufficientAccounts, acct)
	}
	sort.Strings(insufficientAccounts)
	return findings, RuleAvailable, insufficientAccounts
}
