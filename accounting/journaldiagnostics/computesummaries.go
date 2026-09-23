package journaldiagnostics

import "sort"

// computeSourceSummary implements section 33: count/amount/percentage by
// EntrySource, plus an unknown-source bucket for entries with no metadata
// or an unset Source.
func computeSourceSummary(all []analyzedEntry, totalAmount float64) SourceSummary {
	counts := make(map[EntrySource]int)
	amounts := make(map[EntrySource]float64)
	var unknownCount int
	var unknownAmount float64

	for _, a := range all {
		if a.hasMeta && a.meta.Source != "" {
			counts[a.meta.Source]++
			amounts[a.meta.Source] += a.magnitude
		} else {
			unknownCount++
			unknownAmount += a.magnitude
		}
	}

	var buckets []SourceBucket
	for _, s := range sourceOrder {
		if counts[s] == 0 {
			continue
		}
		b := SourceBucket{Source: s, Count: counts[s], Amount: amounts[s]}
		if totalAmount != 0 {
			b.PercentOfTotal = amounts[s] / totalAmount
		}
		buckets = append(buckets, b)
	}

	return SourceSummary{Buckets: buckets, UnknownSourceCount: unknownCount, UnknownSourceAmount: unknownAmount}
}

// computeAccountActivitySummary implements the account-inventory half of
// section 17/34: per-account entry count and debit/credit totals across the
// analyzed population, sorted by AccountID.
func computeAccountActivitySummary(all []analyzedEntry) AccountActivitySummary {
	type accum struct {
		count           int
		debits, credits float64
	}
	byAccount := make(map[string]*accum)
	for _, a := range all {
		for _, l := range a.entry.Lines {
			acc, ok := byAccount[l.AccountID]
			if !ok {
				acc = &accum{}
				byAccount[l.AccountID] = acc
			}
			acc.debits += l.Debit
			acc.credits += l.Credit
		}
		seen := make(map[string]bool, len(a.entry.Lines))
		for _, l := range a.entry.Lines {
			if seen[l.AccountID] {
				continue
			}
			seen[l.AccountID] = true
			byAccount[l.AccountID].count++
		}
	}

	var ids []string
	for id := range byAccount {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	out := make([]AccountActivityBucket, 0, len(ids))
	for _, id := range ids {
		acc := byAccount[id]
		out = append(out, AccountActivityBucket{
			AccountID: id, EntryCount: acc.count,
			TotalDebits: acc.debits, TotalCredits: acc.credits,
		})
	}
	return AccountActivitySummary{Accounts: out}
}

// CrossPeriodComparison implements section 35: a caller-supplied Result
// from a prior period, compared against the current Result's own
// population/finding counts. This package never fabricates a conclusion
// from the delta — it reports the raw comparison only; interpretation is
// left to the caller.
type CrossPeriodComparison struct {
	Available bool `json:"available"`
	// PriorPeriod is the prior Result's own Period label, for display.
	PriorPeriod string `json:"prior_period,omitempty"`

	ManualEntryCountDelta    int     `json:"manual_entry_count_delta"`
	ManualEntryAmountDelta   float64 `json:"manual_entry_amount_delta"`
	PeriodEndCountDelta      int     `json:"period_end_count_delta"`
	PeriodEndAmountDelta     float64 `json:"period_end_amount_delta"`
	ReversalCountDelta       int     `json:"reversal_count_delta"`
	DuplicateGroupCountDelta int     `json:"duplicate_group_count_delta"`
	RoundDollarCountDelta    int     `json:"round_dollar_count_delta"`
	AfterHoursCountDelta     int     `json:"after_hours_count_delta"`
	UnusualAccountCountDelta int     `json:"unusual_account_count_delta"`
}

// countFindingsByCode returns how many findings in fs have code c.
func countFindingsByCode(fs []Finding, codes ...FindingCode) int {
	set := make(map[FindingCode]bool, len(codes))
	for _, c := range codes {
		set[c] = true
	}
	n := 0
	for _, f := range fs {
		if set[f.Code] {
			n++
		}
	}
	return n
}

// compareToPrior implements section 35's delta computation given the
// current Result's own already-computed fields and a caller-supplied prior
// Result. Never fabricates a trend conclusion — see CrossPeriodComparison's
// doc comment.
func compareToPrior(current, prior Result) CrossPeriodComparison {
	unusualAccountCodes := []FindingCode{FindingRareAccountActivity, FindingNewAccountActivity, FindingRareAccountCombination}
	roundDollarCodes := []FindingCode{FindingRoundDollarEntry}
	afterHoursCodes := []FindingCode{FindingOutsideBusinessHours}

	return CrossPeriodComparison{
		Available:   true,
		PriorPeriod: prior.Period,

		ManualEntryCountDelta:  current.PopulationSummary.ManualEntryCount - prior.PopulationSummary.ManualEntryCount,
		ManualEntryAmountDelta: current.PopulationSummary.ManualEntryAmount - prior.PopulationSummary.ManualEntryAmount,

		PeriodEndCountDelta:  current.PeriodEndSummary.PeriodEndEntryCount - prior.PeriodEndSummary.PeriodEndEntryCount,
		PeriodEndAmountDelta: current.PeriodEndSummary.PeriodEndAmount - prior.PeriodEndSummary.PeriodEndAmount,

		ReversalCountDelta:       current.ReversalSummary.ReversalCount - prior.ReversalSummary.ReversalCount,
		DuplicateGroupCountDelta: len(current.DuplicateGroups) - len(prior.DuplicateGroups),

		RoundDollarCountDelta:    countFindingsByCode(current.Findings, roundDollarCodes...) - countFindingsByCode(prior.Findings, roundDollarCodes...),
		AfterHoursCountDelta:     countFindingsByCode(current.Findings, afterHoursCodes...) - countFindingsByCode(prior.Findings, afterHoursCodes...),
		UnusualAccountCountDelta: countFindingsByCode(current.Findings, unusualAccountCodes...) - countFindingsByCode(prior.Findings, unusualAccountCodes...),
	}
}
