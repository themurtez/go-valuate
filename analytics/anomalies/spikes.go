package anomalies

import (
	"fmt"
	"math"

	"github.com/themurtez/go-valuate/financial"
)

// detectAbsoluteAmountSpikes implements RuleAbsoluteAmountSpike: for every
// account and every chronologically adjacent period pair where both periods
// have a reported amount, an unsigned change at or above
// Thresholds.AbsoluteAmountSpike triggers one Anomaly.
func detectAbsoluteAmountSpikes(idx codeIndex, ordered []orderedPeriod, thresholds Thresholds) []Anomaly {
	var out []Anomaly
	for _, code := range idx.codes() {
		for i := 1; i < len(ordered); i++ {
			fromPeriod, toPeriod := ordered[i-1].period, ordered[i].period
			from, fromOK := idx.lookup(code, fromPeriod)
			to, toOK := idx.lookup(code, toPeriod)
			if !fromOK || !toOK {
				continue
			}
			delta := to.Amount - from.Amount
			if math.Abs(delta) < thresholds.AbsoluteAmountSpike {
				continue
			}
			out = append(out, Anomaly{
				Code:           RuleAbsoluteAmountSpike,
				Severity:       AnomalySeverityWarning,
				Account:        code,
				Period:         toPeriod,
				BaselinePeriod: fromPeriod,
				Baseline:       AvailableValue(from.Amount),
				Observed:       AvailableValue(to.Amount),
				Delta:          delta,
				Threshold:      thresholds.AbsoluteAmountSpike,
				Explanation: fmt.Sprintf(
					"%s changed by %.2f from %s to %s (%.2f to %.2f), exceeding the %.2f absolute-change threshold; unusual pattern, review recommended",
					code, delta, fromPeriod, toPeriod, from.Amount, to.Amount, thresholds.AbsoluteAmountSpike,
				),
				Provenance: provenanceFor(to, from, true),
			})
		}
	}
	return out
}

// detectPercentageChangeSpikes implements RulePercentageChangeSpike: for
// every account and every chronologically adjacent period pair where both
// periods have a reported amount and the baseline (prior) amount is
// nonzero, a |change| / |baseline| ratio at or above
// Thresholds.PercentageChangeSpike triggers one Anomaly.
func detectPercentageChangeSpikes(idx codeIndex, ordered []orderedPeriod, thresholds Thresholds) []Anomaly {
	var out []Anomaly
	for _, code := range idx.codes() {
		for i := 1; i < len(ordered); i++ {
			fromPeriod, toPeriod := ordered[i-1].period, ordered[i].period
			from, fromOK := idx.lookup(code, fromPeriod)
			to, toOK := idx.lookup(code, toPeriod)
			if !fromOK || !toOK || from.Amount == 0 {
				continue
			}
			delta := to.Amount - from.Amount
			pctChange := math.Abs(delta) / math.Abs(from.Amount)
			if pctChange < thresholds.PercentageChangeSpike {
				continue
			}
			out = append(out, Anomaly{
				Code:           RulePercentageChangeSpike,
				Severity:       AnomalySeverityWarning,
				Account:        code,
				Period:         toPeriod,
				BaselinePeriod: fromPeriod,
				Baseline:       AvailableValue(from.Amount),
				Observed:       AvailableValue(to.Amount),
				Delta:          delta,
				Threshold:      thresholds.PercentageChangeSpike,
				Explanation: fmt.Sprintf(
					"%s changed %.1f%% from %s to %s (%.2f to %.2f), exceeding the %.1f%% variance threshold; unusual pattern, review recommended",
					code, pctChange*100, fromPeriod, toPeriod, from.Amount, to.Amount, thresholds.PercentageChangeSpike*100,
				),
				Provenance: provenanceFor(to, from, true),
			})
		}
	}
	return out
}

// detectSignFlips implements RuleSignFlip: for every account and every
// chronologically adjacent period pair where both periods have a reported
// nonzero amount, a strict sign change (positive to negative or vice versa)
// where both magnitudes are at least Thresholds.SignFlipMinMagnitude
// triggers one Anomaly.
func detectSignFlips(idx codeIndex, ordered []orderedPeriod, thresholds Thresholds) []Anomaly {
	var out []Anomaly
	for _, code := range idx.codes() {
		for i := 1; i < len(ordered); i++ {
			fromPeriod, toPeriod := ordered[i-1].period, ordered[i].period
			from, fromOK := idx.lookup(code, fromPeriod)
			to, toOK := idx.lookup(code, toPeriod)
			if !fromOK || !toOK {
				continue
			}
			if from.Amount == 0 || to.Amount == 0 {
				continue
			}
			sameSign := (from.Amount > 0) == (to.Amount > 0)
			if sameSign {
				continue
			}
			if math.Abs(from.Amount) < thresholds.SignFlipMinMagnitude || math.Abs(to.Amount) < thresholds.SignFlipMinMagnitude {
				continue
			}
			out = append(out, Anomaly{
				Code:           RuleSignFlip,
				Severity:       AnomalySeverityWarning,
				Account:        code,
				Period:         toPeriod,
				BaselinePeriod: fromPeriod,
				Baseline:       AvailableValue(from.Amount),
				Observed:       AvailableValue(to.Amount),
				Delta:          to.Amount - from.Amount,
				Threshold:      thresholds.SignFlipMinMagnitude,
				Explanation: fmt.Sprintf(
					"%s flipped sign from %.2f in %s to %.2f in %s; unusual pattern, review recommended",
					code, from.Amount, fromPeriod, to.Amount, toPeriod,
				),
				Provenance: provenanceFor(to, from, true),
			})
		}
	}
	return out
}

// unexpectedNegativeCodes is revenueCodes plus expenseCodes — the accounts
// RuleUnexpectedNegativeAmount checks, since this repository's sign
// convention (see financial/metrics' package doc comment) treats revenue
// and expense lines as conventionally non-negative.
var unexpectedNegativeCodes = append(append([]financial.Code{}, revenueCodes...), expenseCodes...)

// detectUnexpectedNegativeAmounts implements RuleUnexpectedNegativeAmount: a
// single-period rule needing no chronological order. For every period and
// every revenue/expense account with a reported negative amount whose
// magnitude is at least Thresholds.UnexpectedNegativeMinMagnitude, one
// Anomaly is produced.
func detectUnexpectedNegativeAmounts(idx codeIndex, periods []financial.Period, thresholds Thresholds) []Anomaly {
	var out []Anomaly
	for _, code := range unexpectedNegativeCodes {
		for _, period := range periods {
			item, ok := idx.lookup(code, period)
			if !ok || item.Amount >= 0 {
				continue
			}
			if math.Abs(item.Amount) < thresholds.UnexpectedNegativeMinMagnitude {
				continue
			}
			out = append(out, Anomaly{
				Code:      RuleUnexpectedNegativeAmount,
				Severity:  AnomalySeverityCritical,
				Account:   code,
				Period:    period,
				Baseline:  Unavailable(),
				Observed:  AvailableValue(item.Amount),
				Delta:     item.Amount,
				Threshold: thresholds.UnexpectedNegativeMinMagnitude,
				Explanation: fmt.Sprintf(
					"%s reported a negative amount of %.2f in %s; revenue/expense accounts are conventionally non-negative, unusual pattern, review recommended",
					code, item.Amount, period,
				),
				Provenance: provenanceFor(item, financial.NormalizedItem{}, false),
			})
		}
	}
	return out
}
