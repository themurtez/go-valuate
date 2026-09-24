package vendorspend

import "sort"

// RecurrenceMix reports the declared-recurrence composition of net spend
// — task section 19's "support caller-declared RECURRING/ONE_TIME/
// IRREGULAR/UNKNOWN" requirement. Built strictly from
// SpendRecord.Recurrence; RecurringSpend below reports the separate,
// pattern-detected signal and never overwrites this.
type RecurrenceMix struct {
	Type  RecurrenceType `json:"type"`
	Spend float64        `json:"spend"`
	Share Value          `json:"share"`
}

// computeRecurrenceMix buckets net spend by resolved Recurrence.
func computeRecurrenceMix(records []SpendRecord, netTotal float64) []RecurrenceMix {
	byType := map[RecurrenceType]float64{}
	for _, r := range records {
		byType[resolvedRecurrence(r.Recurrence)] += netSpendOf(r)
	}
	types := make([]RecurrenceType, 0, len(byType))
	for t := range byType {
		types = append(types, t)
	}
	sort.Slice(types, func(i, j int) bool { return types[i] < types[j] })

	out := make([]RecurrenceMix, 0, len(types))
	for _, t := range types {
		amt := byType[t]
		rm := RecurrenceMix{Type: t, Spend: amt}
		if netTotal != 0 {
			rm.Share = AvailableValue(amt / netTotal)
		}
		out = append(out, rm)
	}
	return out
}

// ObservedRecurringGroup is one supplier/category combination this
// package's own pattern detection (never overwriting caller Recurrence)
// found to show comparable spend across at least
// Policy.ObservedRecurringMinPeriods distinct periods — task section 19's
// "optionally detect an OBSERVED_REPEATED_SPEND pattern separately" rule.
type ObservedRecurringGroup struct {
	SupplierID    string   `json:"supplier_id"`
	Category      string   `json:"category,omitempty"`
	Periods       []string `json:"periods"`
	AverageAmount float64  `json:"average_amount"`
}

// computeObservedRecurring detects supplier/category groups with
// comparable (within tolerance) net spend across minPeriods+ distinct
// chronological periods.
func computeObservedRecurring(orderedPeriods []string, byPeriod map[string][]SpendRecord, minPeriods int, tolerance float64) []ObservedRecurringGroup {
	if minPeriods < 2 {
		minPeriods = 2
	}
	type groupKey struct{ supplierID, category string }
	amountsByGroup := map[groupKey]map[string]float64{}

	for _, p := range orderedPeriods {
		for _, r := range byPeriod[p] {
			gk := groupKey{r.SupplierID, r.Category}
			if amountsByGroup[gk] == nil {
				amountsByGroup[gk] = map[string]float64{}
			}
			amountsByGroup[gk][p] += netSpendOf(r)
		}
	}

	var groupKeys []groupKey
	for gk := range amountsByGroup {
		groupKeys = append(groupKeys, gk)
	}
	sort.Slice(groupKeys, func(i, j int) bool {
		if groupKeys[i].supplierID != groupKeys[j].supplierID {
			return groupKeys[i].supplierID < groupKeys[j].supplierID
		}
		return groupKeys[i].category < groupKeys[j].category
	})

	var out []ObservedRecurringGroup
	for _, gk := range groupKeys {
		byP := amountsByGroup[gk]

		// Build the amounts in chronological order for only the periods
		// this group has activity in.
		var activePeriods []string
		var amounts []float64
		for _, p := range orderedPeriods {
			if amt, ok := byP[p]; ok {
				activePeriods = append(activePeriods, p)
				amounts = append(amounts, amt)
			}
		}
		if len(activePeriods) < minPeriods {
			continue
		}

		// "Comparable" means every amount in the run is within tolerance
		// of the run's average.
		sum := 0.0
		for _, a := range amounts {
			sum += a
		}
		avg := sum / float64(len(amounts))
		if avg == 0 {
			continue
		}
		comparable := true
		for _, a := range amounts {
			if absFloat(a-avg) > tolerance*absFloat(avg) {
				comparable = false
				break
			}
		}
		if !comparable {
			continue
		}

		out = append(out, ObservedRecurringGroup{
			SupplierID: gk.supplierID, Category: gk.category, Periods: activePeriods, AverageAmount: avg,
		})
	}

	return out
}

// RecurringSpend bundles the caller-declared RecurrenceMix and this
// package's own ObservedRecurring pattern detection — task section 19.
type RecurringSpend struct {
	Mix               []RecurrenceMix          `json:"mix,omitempty"`
	ObservedRecurring []ObservedRecurringGroup `json:"observed_recurring,omitempty"`
}

// CommitmentMix reports the declared committed-vs-discretionary
// composition of net spend — task section 20. Never inferred from
// SpendType.
type CommitmentMix struct {
	Type  CommitmentType `json:"type"`
	Spend float64        `json:"spend"`
	Share Value          `json:"share"`
}

// computeCommitmentMix buckets net spend by resolved Commitment.
func computeCommitmentMix(records []SpendRecord, netTotal float64) []CommitmentMix {
	byType := map[CommitmentType]float64{}
	for _, r := range records {
		byType[resolvedCommitment(r.Commitment)] += netSpendOf(r)
	}
	types := make([]CommitmentType, 0, len(byType))
	for t := range byType {
		types = append(types, t)
	}
	sort.Slice(types, func(i, j int) bool { return types[i] < types[j] })

	out := make([]CommitmentMix, 0, len(types))
	for _, t := range types {
		amt := byType[t]
		cm := CommitmentMix{Type: t, Spend: amt}
		if netTotal != 0 {
			cm.Share = AvailableValue(amt / netTotal)
		}
		out = append(out, cm)
	}
	return out
}
