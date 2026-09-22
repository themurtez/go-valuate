package concentration

import "github.com/themurtez/go-valuate/financial"

// calculateDependencyChanges computes one DependencyChange per entity
// active in either side of every chronologically adjacent pair in
// orderedPeriods that both have an entry in byPeriod (periods with no
// observations at all are simply skipped — there is nothing to compare).
// Mirrors revenuequality.calculateCustomerTransitions' identical
// adjacent-pair iteration.
func calculateDependencyChanges(orderedPeriods []financial.Period, byPeriod map[financial.Period][]Observation) []DependencyChange {
	var periodsWithData []financial.Period
	for _, p := range orderedPeriods {
		if _, ok := byPeriod[p]; ok {
			periodsWithData = append(periodsWithData, p)
		}
	}
	if len(periodsWithData) < 2 {
		return nil
	}

	var changes []DependencyChange
	for i := 1; i < len(periodsWithData); i++ {
		fromPeriod, toPeriod := periodsWithData[i-1], periodsWithData[i]
		fromTotals, fromGrandTotal := entityTotalsByKey(byPeriod[fromPeriod])
		toTotals, toGrandTotal := entityTotalsByKey(byPeriod[toPeriod])
		changes = append(changes, computeDependencyChanges(fromPeriod, toPeriod, fromTotals, toTotals, fromGrandTotal, toGrandTotal)...)
	}
	return changes
}

// computeDependencyChanges builds one DependencyChange per distinct entity
// key present in fromTotals or toTotals, iterating in sorted-key order for
// deterministic output — mirrors
// revenuequality.computeCustomerTransition's identical sorted-iteration
// requirement (float64 addition is not associative, so a stable iteration
// order is required for Calculate's documented byte-for-byte-identical-
// output guarantee, not merely for readability; here each entity's own
// FromAmount/ToAmount/shares are computed independently with no shared
// accumulator, but the *output slice order* must still be deterministic).
func computeDependencyChanges(fromPeriod, toPeriod financial.Period, fromTotals, toTotals map[string]float64, fromGrandTotal, toGrandTotal float64) []DependencyChange {
	allKeysSet := make(map[string]bool, len(fromTotals)+len(toTotals))
	for k := range fromTotals {
		allKeysSet[k] = true
	}
	for k := range toTotals {
		allKeysSet[k] = true
	}
	allKeys := sortedKeys(allKeysSet)

	changes := make([]DependencyChange, 0, len(allKeys))
	for _, key := range allKeys {
		dc := DependencyChange{
			FromPeriod: fromPeriod,
			ToPeriod:   toPeriod,
			EntityKey:  key,
		}

		if fromAmt, ok := fromTotals[key]; ok {
			dc.FromAmount = AvailableValue(fromAmt)
			if fromGrandTotal != 0 {
				dc.FromShare = AvailableValue(fromAmt / fromGrandTotal)
			}
		}
		if toAmt, ok := toTotals[key]; ok {
			dc.ToAmount = AvailableValue(toAmt)
			if toGrandTotal != 0 {
				dc.ToShare = AvailableValue(toAmt / toGrandTotal)
			}
		}
		if dc.FromShare.Available && dc.ToShare.Available {
			dc.ShareChange = AvailableValue(dc.ToShare.Value - dc.FromShare.Value)
		}

		changes = append(changes, dc)
	}
	return changes
}
