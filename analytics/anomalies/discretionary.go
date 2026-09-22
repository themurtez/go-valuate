package anomalies

import (
	"fmt"

	"github.com/themurtez/go-valuate/financial"
)

// detectHighOwnerDiscretionaryShare implements RuleHighOwnerDiscretionaryShare:
// evaluated once, against the chronologically most recent period in ordered
// with an available Total Revenue figure. Sums financial.CodeOpexOwnerComp
// plus every discretionaryCodes amount reported in that period and compares
// the result, as a fraction of that period's Total Revenue, against
// Thresholds.OwnerDiscretionaryShareOfRevenue.
func detectHighOwnerDiscretionaryShare(idx codeIndex, ordered []orderedPeriod, discretionaryCodes []financial.Code, thresholds Thresholds) []Anomaly {
	period, revenue, ok := mostRecentAvailableRevenue(idx, ordered)
	if !ok {
		return nil
	}

	codes := discretionaryCodeSet(discretionaryCodes)
	total := 0.0
	anyPresent := false
	for _, code := range codes {
		item, itemOK := idx.lookup(code, period)
		if !itemOK {
			continue
		}
		anyPresent = true
		total += item.Amount
	}
	if !anyPresent {
		return nil
	}

	share := total / abs(revenue)
	if share < thresholds.OwnerDiscretionaryShareOfRevenue {
		return nil
	}

	return []Anomaly{{
		Code:      RuleHighOwnerDiscretionaryShare,
		Severity:  AnomalySeverityInfo,
		Account:   financial.CodeOpexOwnerComp,
		Period:    period,
		Baseline:  Unavailable(),
		Observed:  AvailableValue(total),
		Delta:     total,
		Threshold: thresholds.OwnerDiscretionaryShareOfRevenue,
		Explanation: fmt.Sprintf(
			"owner/discretionary expenses total %.2f in %s, %.1f%% of Total Revenue, exceeding the %.1f%% threshold; unusual pattern, review recommended",
			total, period, share*100, thresholds.OwnerDiscretionaryShareOfRevenue*100,
		),
	}}
}

// discretionaryCodeSet returns financial.CodeOpexOwnerComp plus extra,
// deduplicated, preserving CodeOpexOwnerComp first for deterministic
// summation order.
func discretionaryCodeSet(extra []financial.Code) []financial.Code {
	seen := map[financial.Code]bool{financial.CodeOpexOwnerComp: true}
	codes := []financial.Code{financial.CodeOpexOwnerComp}
	for _, c := range extra {
		if seen[c] {
			continue
		}
		seen[c] = true
		codes = append(codes, c)
	}
	return codes
}

// mostRecentAvailableRevenue walks ordered from the end backward and
// returns the first (chronologically latest) period with an available,
// nonzero Total Revenue figure — a period with an explicitly reported $0
// Total Revenue is skipped (not returned), since a $0 denominator can never
// support a share calculation; the walk continues to the next-most-recent
// period instead of the caller silently getting no result at all.
func mostRecentAvailableRevenue(idx codeIndex, ordered []orderedPeriod) (financial.Period, float64, bool) {
	for i := len(ordered) - 1; i >= 0; i-- {
		period := ordered[i].period
		revenue, ok := totalRevenue(idx, period)
		if ok && revenue != 0 {
			return period, revenue, true
		}
	}
	return "", 0, false
}
