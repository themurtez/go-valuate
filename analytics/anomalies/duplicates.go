package anomalies

import (
	"fmt"
	"math"
	"sort"

	"github.com/themurtez/go-valuate/financial"
)

// roundKey buckets a float64 amount into a cents-precision string key, so
// RuleRepeatedUnusualValue/RuleDuplicateLikeAmounts can group observations
// by amount via a Go map — see FloatEqualityTolerance's doc comment for why
// this package treats "same value" as a fixed-tolerance comparison rather
// than exact float64 equality. Cents precision is finer than any realistic
// financial amount's meaningful precision, so two values a caller would
// consider genuinely distinct dollar amounts never collide into the same
// key, while two values that differ only by floating-point noise (e.g. from
// upstream aggregation arithmetic) reliably land in the same bucket.
func roundKey(amount float64) string {
	return fmt.Sprintf("%.2f", math.Round(amount*100)/100)
}

// detectRepeatedUnusualValues implements RuleRepeatedUnusualValue: for
// every account, every distinct nonzero amount (grouped via roundKey) that
// appears in at least Thresholds.RepeatedValueMinOccurrences distinct
// periods produces one Anomaly, referencing every period it appeared in.
// Needs no chronological order; periods is sorted (dataset lexical order)
// when PeriodMeta was unavailable, or the caller may pass a chronologically
// sorted slice — either way RelatedPeriods is re-sorted by meta when
// available (see anomalies.go's sortAnomalies, which sorts the returned
// slice, not this function's internal grouping order).
func detectRepeatedUnusualValues(idx codeIndex, periods []financial.Period, thresholds Thresholds) []Anomaly {
	minOccurrences := thresholds.RepeatedValueMinOccurrences
	if minOccurrences <= 0 {
		minOccurrences = DefaultThresholds().RepeatedValueMinOccurrences
	}

	var out []Anomaly
	for _, code := range idx.codes() {
		byAmount := make(map[string][]financial.NormalizedItem)
		for _, period := range periods {
			item, ok := idx.lookup(code, period)
			if !ok || item.Amount == 0 {
				continue
			}
			key := roundKey(item.Amount)
			byAmount[key] = append(byAmount[key], item)
		}

		keys := make([]string, 0, len(byAmount))
		for k := range byAmount {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, key := range keys {
			items := byAmount[key]
			if len(items) < minOccurrences {
				continue
			}
			relatedPeriods := make([]financial.Period, 0, len(items))
			for _, it := range items {
				relatedPeriods = append(relatedPeriods, it.Period)
			}
			sort.Slice(relatedPeriods, func(i, j int) bool { return relatedPeriods[i] < relatedPeriods[j] })

			last := items[len(items)-1]
			out = append(out, Anomaly{
				Code:      RuleRepeatedUnusualValue,
				Severity:  AnomalySeverityInfo,
				Account:   code,
				Period:    last.Period,
				Baseline:  Unavailable(),
				Observed:  AvailableValue(last.Amount),
				Delta:     0,
				Threshold: float64(minOccurrences),
				Explanation: fmt.Sprintf(
					"%s reports the same amount of %.2f in %d periods (%v); unusual pattern, review recommended",
					code, last.Amount, len(items), relatedPeriods,
				),
				RelatedPeriods: relatedPeriods,
				Provenance:     provenanceFor(last, financial.NormalizedItem{}, false),
			})
		}
	}
	return out
}

// suspiciousDuplicateCodes is cogsCodes plus opexCodes — the accounts
// RuleDuplicateLikeAmounts scans for cross-account duplicate amounts. See
// RuleDuplicateLikeAmounts's doc comment for why balance-sheet/revenue
// codes are excluded (a shared control total or an intentionally identical
// recurring revenue figure is a far less notable coincidence there).
var suspiciousDuplicateCodes = expenseCodes

// duplicateObservation is one (account, period, amount) triple considered
// by detectDuplicateLikeAmounts.
type duplicateObservation struct {
	code   financial.Code
	period financial.Period
	item   financial.NormalizedItem
}

// detectDuplicateLikeAmounts implements RuleDuplicateLikeAmounts: across
// every (account, period) pair among suspiciousDuplicateCodes, this groups
// observations by amount (via roundKey) and produces one Anomaly per
// distinct amount shared by two or more DISTINCT accounts (a repeated value
// on a single account across periods is RuleRepeatedUnusualValue's concern,
// not this rule's — see that rule's doc comment).
func detectDuplicateLikeAmounts(idx codeIndex, periods []financial.Period, thresholds Thresholds) []Anomaly {
	byAmount := make(map[string][]duplicateObservation)
	for _, code := range suspiciousDuplicateCodes {
		for _, period := range periods {
			item, ok := idx.lookup(code, period)
			if !ok || item.Amount == 0 {
				continue
			}
			key := roundKey(item.Amount)
			byAmount[key] = append(byAmount[key], duplicateObservation{code: code, period: period, item: item})
		}
	}

	keys := make([]string, 0, len(byAmount))
	for k := range byAmount {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var out []Anomaly
	for _, key := range keys {
		obs := byAmount[key]
		distinctAccounts := make(map[financial.Code]bool)
		for _, o := range obs {
			distinctAccounts[o.code] = true
		}
		if len(distinctAccounts) < 2 {
			continue
		}

		sort.Slice(obs, func(i, j int) bool {
			if obs[i].code != obs[j].code {
				return obs[i].code < obs[j].code
			}
			return obs[i].period < obs[j].period
		})

		accounts := make([]financial.Code, 0, len(distinctAccounts))
		for c := range distinctAccounts {
			accounts = append(accounts, c)
		}
		sort.Slice(accounts, func(i, j int) bool { return accounts[i] < accounts[j] })

		relatedPeriodSet := make(map[financial.Period]bool)
		for _, o := range obs {
			relatedPeriodSet[o.period] = true
		}
		relatedPeriods := make([]financial.Period, 0, len(relatedPeriodSet))
		for p := range relatedPeriodSet {
			relatedPeriods = append(relatedPeriods, p)
		}
		sort.Slice(relatedPeriods, func(i, j int) bool { return relatedPeriods[i] < relatedPeriods[j] })

		anchor := obs[0]
		otherAccounts := make([]financial.Code, 0, len(accounts)-1)
		for _, a := range accounts {
			if a != anchor.code {
				otherAccounts = append(otherAccounts, a)
			}
		}

		out = append(out, Anomaly{
			Code:      RuleDuplicateLikeAmounts,
			Severity:  AnomalySeverityInfo,
			Account:   anchor.code,
			Period:    anchor.period,
			Baseline:  Unavailable(),
			Observed:  AvailableValue(anchor.item.Amount),
			Delta:     0,
			Threshold: float64(len(distinctAccounts)),
			Explanation: fmt.Sprintf(
				"amount %.2f repeats identically across %d distinct accounts (%v) in periods %v; unusual pattern, review recommended",
				anchor.item.Amount, len(distinctAccounts), accounts, relatedPeriods,
			),
			RelatedPeriods:  relatedPeriods,
			RelatedAccounts: otherAccounts,
			Provenance:      provenanceFor(anchor.item, financial.NormalizedItem{}, false),
		})
	}
	return out
}
