package qoe

import (
	"sort"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/adjustments"
)

// nonRecurringTypes is the set of adjustments.Type values that are, by
// their own built-in definition (see adjustments.buildTypeRegistry's
// doc comments), meant to describe something non-recurring: a one-time
// expense, a non-recurring professional fee, or a one-time gain/loss.
// buildRecurrence restricts its "may not be truly non-recurring" signal to
// exactly this set — a Type like TypeOwnerCompensationNormalization or
// TypePersonalVehicle recurring every year is expected and not suspicious
// (an owner's under-market salary or personal vehicle use does not stop
// being real just because it appears in three consecutive years), so
// flagging it here would be noise, not signal.
var nonRecurringTypes = map[adjustments.Type]bool{
	adjustments.TypeOneTimeExpense:               true,
	adjustments.TypeNonRecurringProfessionalFees: true,
	adjustments.TypeUnusualGain:                  true,
	adjustments.TypeUnusualLoss:                  true,
}

// recurringTypes is the set of adjustments.Type values that are, by their
// own built-in definition, expected to recur period over period rather
// than being one-off items: an owner's compensation, personal vehicle, or
// personal travel expense, and related-party rent, are all ongoing
// arrangements a caller should generally expect to see again in a future
// period, not isolated events. Used by buildRecurringSummary — see
// RecurringSummary's doc comment for how this differs from
// nonRecurringTypes/RecurrencePattern.
//
// TypeNonOperatingIncome and TypeCustom are deliberately in neither this
// set nor nonRecurringTypes: TypeNonOperatingIncome's own definition (see
// adjustments.buildTypeRegistry) covers both a recurring investment-income
// stream and a one-off asset-sale gain without distinguishing which,
// and TypeCustom is caller-defined with no inherent nature this package
// can infer. Both fall into RecurringSummary's Unclassified bucket instead
// of being guessed into one side or the other.
var recurringTypes = map[adjustments.Type]bool{
	adjustments.TypeOwnerCompensationNormalization: true,
	adjustments.TypeOwnerDiscretionaryExpense:      true,
	adjustments.TypePersonalVehicle:                true,
	adjustments.TypePersonalTravel:                 true,
	adjustments.TypeRelatedPartyRentAdjustment:     true,
}

// buildRecurringSummary implements RecurringSummary: every applied AppliedLine
// across history's EBITDA and SDE bridges is classified by its Type into
// exactly one of three buckets (recurring/non-recurring/unclassified) and
// summed, independently per bridge. Unlike buildRecurrence, no cross-bridge
// ID dedup is needed here: EBITDATotal and SDETotal are two independently
// meaningful sums (mirroring AdjustmentBreakdown's own EBITDA/SDE
// separation), not a single combined total that dual-counting would
// corrupt.
func buildRecurringSummary(history []PeriodFigures) RecurringSummary {
	var s RecurringSummary

	classify := func(t adjustments.Type, amount float64, ebitdaBridge bool) {
		switch {
		case recurringTypes[t]:
			if ebitdaBridge {
				s.RecurringEBITDATotal += amount
			} else {
				s.RecurringSDETotal += amount
			}
			s.RecurringCount++
		case nonRecurringTypes[t]:
			if ebitdaBridge {
				s.NonRecurringEBITDATotal += amount
			} else {
				s.NonRecurringSDETotal += amount
			}
			s.NonRecurringCount++
		default:
			if ebitdaBridge {
				s.UnclassifiedEBITDATotal += amount
			} else {
				s.UnclassifiedSDETotal += amount
			}
			s.UnclassifiedCount++
		}
	}

	for _, pf := range history {
		for _, line := range pf.Adjustments.EBITDABridge.Applied {
			classify(line.Adjustment.Type, line.SignedAmount, true)
		}
		for _, line := range pf.Adjustments.SDEBridge.Applied {
			classify(line.Adjustment.Type, line.SignedAmount, false)
		}
	}

	return s
}

// buildRecurrence groups every applied, confirmed adjustment line across
// history by adjustments.Type, restricted to nonRecurringTypes, and reports
// how many distinct periods each Type appeared in. See RecurrencePattern
// and the package doc comment's "Repeated one-time adjustment detection"
// section: this function only surfaces the signal — it never removes or
// reclassifies an adjustment itself.
//
// A Type counts as "appearing" in a period if at least one applied line of
// that Type contributed to either bridge (EBITDA or SDE) in that period; a
// Type applied to both bridges in the same period still counts that period
// once, not twice, since RecurrencePattern.Count answers "how many distinct
// periods," not "how many applied lines."
func buildRecurrence(history []PeriodFigures, thresholds Thresholds) []RecurrencePattern {
	type accum struct {
		periods     map[financial.Period]bool
		order       []financial.Period
		totalAmount float64
	}
	byType := make(map[adjustments.Type]*accum)

	recordType := func(t adjustments.Type, period financial.Period, amount float64) {
		if !nonRecurringTypes[t] {
			return
		}
		a, ok := byType[t]
		if !ok {
			a = &accum{periods: make(map[financial.Period]bool)}
			byType[t] = a
		}
		if !a.periods[period] {
			a.periods[period] = true
			a.order = append(a.order, period)
		}
		a.totalAmount += amount
	}

	for _, pf := range history {
		// Dedupe by the adjustment's own ID (unique within a single Apply
		// call, enforced by adjustments.Validate's IssueDuplicateID check)
		// before feeding recordType, not by Type: a dual-target
		// adjustment (e.g. one TypeOneTimeExpense targeting both EBITDA
		// and SDE) must contribute its Amount exactly once, but two
		// DIFFERENT adjustments of the same Type in the same period — one
		// targeting EBITDA only, another targeting SDE only — are two
		// distinct real-world amounts and must both be counted. A dedup
		// keyed on Type alone would silently drop the second adjustment's
		// Amount in that second case.
		seenThisPeriod := make(map[adjustments.ID]bool)
		recordLine := func(line adjustments.AppliedLine) {
			if seenThisPeriod[line.Adjustment.ID] {
				return
			}
			seenThisPeriod[line.Adjustment.ID] = true
			recordType(line.Adjustment.Type, pf.Period, line.Adjustment.Amount)
		}
		for _, line := range pf.Adjustments.EBITDABridge.Applied {
			recordLine(line)
		}
		for _, line := range pf.Adjustments.SDEBridge.Applied {
			recordLine(line)
		}
	}

	types := make([]adjustments.Type, 0, len(byType))
	for t := range byType {
		types = append(types, t)
	}
	sort.Slice(types, func(i, j int) bool { return types[i] < types[j] })

	minPeriods := thresholds.RepeatedOneTimeMinPeriods
	if minPeriods <= 0 {
		minPeriods = DefaultThresholds().RepeatedOneTimeMinPeriods
	}

	patterns := make([]RecurrencePattern, 0, len(types))
	for _, t := range types {
		a := byType[t]
		// a.order was appended while iterating history, which Calculate
		// already placed in chronological order (when PeriodMeta was
		// supplied) or dataset order otherwise — preserved as-is here
		// rather than re-sorted lexically, since financial.Period's
		// string value has no guaranteed chronological sort order (e.g.
		// "2025-Q4" < "2025-Q10" is not how a caller would want these
		// read).
		periods := append([]financial.Period(nil), a.order...)
		patterns = append(patterns, RecurrencePattern{
			Type:                  t,
			Periods:               periods,
			Count:                 len(periods),
			TotalAmount:           a.totalAmount,
			LikelyNotNonRecurring: len(periods) >= minPeriods,
		})
	}
	return patterns
}
