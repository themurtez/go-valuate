package revenuequality

import (
	"fmt"
	"math"
	"sort"

	"github.com/themurtez/go-valuate/financial"
)

// validateCustomerRevenue checks every row in rows against ds and returns
// one advisory Issue per problem class found (not one Issue per row, to
// avoid flooding Result.Warnings when many rows share the same problem —
// mirroring workingcapital.chronologicalPeriods' single-Issue-listing-every-
// affected-period convention).
func validateCustomerRevenue(rows []CustomerPeriodRevenue, ds financial.FinancialDataset) []Issue {
	validPeriods := make(map[financial.Period]bool, len(ds.Items))
	for _, p := range ds.Periods() {
		validPeriods[p] = true
	}

	var missing []financial.Period
	seenMissing := make(map[financial.Period]bool)
	for _, row := range rows {
		if !validPeriods[row.Period] && !seenMissing[row.Period] {
			seenMissing[row.Period] = true
			missing = append(missing, row.Period)
		}
	}

	var issues []Issue
	if len(missing) > 0 {
		sort.Slice(missing, func(i, j int) bool { return missing[i] < missing[j] })
		issues = append(issues, Issue{
			Code:     IssueCustomerPeriodNotInDataset,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("one or more CustomerPeriodRevenue rows reference a period not present in Dataset: %v; those rows are excluded from every customer-level output", missing),
		})
	}

	return issues
}

// filterValidCustomerRevenue returns only the rows whose Period is present
// in ds — see IssueCustomerPeriodNotInDataset.
func filterValidCustomerRevenue(rows []CustomerPeriodRevenue, ds financial.FinancialDataset) []CustomerPeriodRevenue {
	validPeriods := make(map[financial.Period]bool, len(ds.Items))
	for _, p := range ds.Periods() {
		validPeriods[p] = true
	}
	valid := make([]CustomerPeriodRevenue, 0, len(rows))
	for _, row := range rows {
		if validPeriods[row.Period] {
			valid = append(valid, row)
		}
	}
	return valid
}

// groupCustomerRevenueByPeriod groups rows by Period.
func groupCustomerRevenueByPeriod(rows []CustomerPeriodRevenue) map[financial.Period][]CustomerPeriodRevenue {
	byPeriod := make(map[financial.Period][]CustomerPeriodRevenue)
	for _, row := range rows {
		byPeriod[row.Period] = append(byPeriod[row.Period], row)
	}
	return byPeriod
}

// customerTotalsByKey sums rows' Amount per distinct CustomerKey, returning
// the per-customer totals plus the grand total.
func customerTotalsByKey(rows []CustomerPeriodRevenue) (map[string]float64, float64) {
	totals := make(map[string]float64, len(rows))
	var grandTotal float64
	for _, row := range rows {
		totals[row.CustomerKey] += row.Amount
		grandTotal += row.Amount
	}
	return totals, grandTotal
}

// computeCustomerPeriodTotal aggregates rows (all for the same Period) into
// a CustomerPeriodTotal.
func computeCustomerPeriodTotal(period financial.Period, rows []CustomerPeriodRevenue) CustomerPeriodTotal {
	_, total := customerTotalsByKey(rows)

	customers := make(map[string]bool, len(rows))
	var recurringTotal float64
	var flaggedAmount float64
	var anyFlagged bool
	for _, row := range rows {
		customers[row.CustomerKey] = true
		if row.RecurringFlag != nil {
			anyFlagged = true
			flaggedAmount += row.Amount
			if *row.RecurringFlag {
				recurringTotal += row.Amount
			}
		}
	}

	ct := CustomerPeriodTotal{
		Period:               period,
		CustomerCount:        len(customers),
		TotalCustomerRevenue: AvailableValue(total),
	}
	if anyFlagged {
		ct.RecurringCustomerRevenue = AvailableValue(recurringTotal)
	}
	if total != 0 {
		ct.RecurringFlagCoverage = flaggedAmount / total
	}
	return ct
}

// calculateCustomerTransitions computes one CustomerTransition per
// chronologically adjacent pair in orderedPeriods that both have an entry
// in byPeriod (periods with no customer data at all are simply skipped —
// there is nothing to compare).
func calculateCustomerTransitions(orderedPeriods []financial.Period, byPeriod map[financial.Period][]CustomerPeriodRevenue) []CustomerTransition {
	var periodsWithData []financial.Period
	for _, p := range orderedPeriods {
		if _, ok := byPeriod[p]; ok {
			periodsWithData = append(periodsWithData, p)
		}
	}
	if len(periodsWithData) < 2 {
		return nil
	}

	transitions := make([]CustomerTransition, 0, len(periodsWithData)-1)
	for i := 1; i < len(periodsWithData); i++ {
		fromPeriod, toPeriod := periodsWithData[i-1], periodsWithData[i]
		fromTotals, fromGrandTotal := customerTotalsByKey(byPeriod[fromPeriod])
		toTotals, toGrandTotal := customerTotalsByKey(byPeriod[toPeriod])
		transitions = append(transitions, computeCustomerTransition(fromPeriod, toPeriod, fromTotals, toTotals, fromGrandTotal, toGrandTotal))
	}
	return transitions
}

// computeCustomerTransition classifies every customer key present in
// fromTotals or toTotals into new/lost/retained-with-expansion/
// retained-with-contraction/retained-flat, and sums each category.
// fromGrandTotal/toGrandTotal are customerTotalsByKey's own grand totals
// for the same two periods, passed through rather than re-derived from
// fromTotals/toTotals here — see CustomerTransition.FromPeriodTotalRevenue's
// doc comment for why every "FromPeriod total" figure this package reports
// must trace back to this single source rather than being reconstructed
// from the categorized new/lost/retained/expansion/contraction fields,
// which are NOT algebraically sufficient to recover it (RetainedRevenue is
// already min(from,to) per customer, so ExpansionRevenue is additional
// revenue on top of it, not a component to subtract back out).
//
// Every accumulation below iterates keys in sorted order (via sortedKeys),
// never raw Go map order: float64 addition is not associative, so summing
// the same set of amounts in a different order can produce a different
// last-bit result (see determinism_test.go's
// TestCalculate_DeterministicAcrossMapOrdering, which caught exactly this
// class of bug during development). A stable iteration order is required
// for Calculate's documented byte-for-byte-identical-output guarantee, not
// merely for readability.
func computeCustomerTransition(fromPeriod, toPeriod financial.Period, fromTotals, toTotals map[string]float64, fromGrandTotal, toGrandTotal float64) CustomerTransition {
	t := CustomerTransition{
		FromPeriod:             fromPeriod,
		ToPeriod:               toPeriod,
		FromPeriodTotalRevenue: AvailableValue(fromGrandTotal),
		ToPeriodTotalRevenue:   AvailableValue(toGrandTotal),
		TotalRevenueGrowth:     AvailableValue(toGrandTotal - fromGrandTotal),
	}

	allKeysSet := make(map[string]bool, len(fromTotals)+len(toTotals))
	for k := range fromTotals {
		allKeysSet[k] = true
	}
	for k := range toTotals {
		allKeysSet[k] = true
	}
	allKeys := sortedKeys(allKeysSet)

	var newRevenue, lostRevenue, retainedRevenue, expansionRevenue, contractionRevenue float64
	var newCount, lostCount, retainedCount, expandedCount, contractedCount int

	for _, key := range allKeys {
		fromAmt, hadFrom := fromTotals[key]
		toAmt, hasTo := toTotals[key]

		switch {
		case !hadFrom && hasTo:
			newRevenue += toAmt
			newCount++
		case hadFrom && !hasTo:
			lostRevenue += fromAmt
			lostCount++
		case hadFrom && hasTo:
			retainedCount++
			retainedRevenue += math.Min(fromAmt, toAmt)
			switch {
			case toAmt > fromAmt:
				expansionRevenue += toAmt - fromAmt
				expandedCount++
			case toAmt < fromAmt:
				contractionRevenue += fromAmt - toAmt
				contractedCount++
			}
		}
	}

	t.NewCustomerRevenue = AvailableValue(newRevenue)
	t.NewCustomerCount = newCount
	t.LostCustomerRevenue = AvailableValue(lostRevenue)
	t.LostCustomerCount = lostCount
	t.RetainedRevenue = AvailableValue(retainedRevenue)
	t.RetainedCustomerCount = retainedCount
	t.ExpansionRevenue = AvailableValue(expansionRevenue)
	t.ExpandedCustomerCount = expandedCount
	t.ContractionRevenue = AvailableValue(contractionRevenue)
	t.ContractedCustomerCount = contractedCount

	existingBaseChange := (retainedRevenue + expansionRevenue - contractionRevenue) - fromGrandTotal
	t.ExistingCustomerBaseChange = AvailableValue(existingBaseChange)

	return t
}

// sortedKeys returns m's keys sorted ascending, for deterministic iteration
// over a map whose accumulation order can affect float64 output (see
// computeCustomerTransition's doc comment). Accepts either map value type
// used in this file via a type parameter rather than two near-identical
// copies.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// calculateConcentrationSummary computes ConcentrationSummary for the most
// recent period (by orderedPeriods' order) that has an entry in byPeriod.
// revenueHistory supplies Dataset's own total revenue for
// UnallocatedRevenue.
func calculateConcentrationSummary(orderedPeriods []financial.Period, byPeriod map[financial.Period][]CustomerPeriodRevenue, revenueHistory []PeriodRevenue, policy Policy) ConcentrationSummary {
	var targetPeriod financial.Period
	var found bool
	for i := len(orderedPeriods) - 1; i >= 0; i-- {
		if _, ok := byPeriod[orderedPeriods[i]]; ok {
			targetPeriod = orderedPeriods[i]
			found = true
			break
		}
	}
	if !found {
		return ConcentrationSummary{}
	}

	rows := byPeriod[targetPeriod]
	totals, grandTotal := customerTotalsByKey(rows)

	summary := ConcentrationSummary{
		Period:               targetPeriod,
		CustomerCount:        len(totals),
		TotalCustomerRevenue: AvailableValue(grandTotal),
	}

	for _, pr := range revenueHistory {
		if pr.Period == targetPeriod && pr.TotalRevenue.Available {
			summary.UnallocatedRevenue = AvailableValue(pr.TotalRevenue.Value - grandTotal)
			break
		}
	}

	sortedAmounts := make([]float64, 0, len(totals))
	for _, amt := range totals {
		sortedAmounts = append(sortedAmounts, amt)
	}
	sort.Sort(sort.Reverse(sort.Float64Slice(sortedAmounts)))

	for _, n := range sortedTopN(policy.ConcentrationTopN) {
		share := TopNShare{N: n}
		cutoff := n
		if cutoff > len(sortedAmounts) {
			cutoff = len(sortedAmounts)
		}
		var topSum float64
		for i := 0; i < cutoff; i++ {
			topSum += sortedAmounts[i]
		}
		share.Revenue = AvailableValue(topSum)
		if grandTotal != 0 {
			share.Percent = AvailableValue(topSum / grandTotal)
		}
		summary.TopNShares = append(summary.TopNShares, share)
	}

	if len(totals) > 0 && grandTotal != 0 {
		var hhi float64
		for _, key := range sortedKeys(totals) {
			share := totals[key] / grandTotal
			hhi += share * share
		}
		summary.HHI = AvailableValue(hhi * 10000)
	}

	summary.Segments = calculateSegmentShares(rows, grandTotal)

	return summary
}

// sortedTopN returns a deduplicated, ascending-sorted copy of ns.
func sortedTopN(ns []int) []int {
	seen := make(map[int]bool, len(ns))
	out := make([]int, 0, len(ns))
	for _, n := range ns {
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	sort.Ints(out)
	return out
}

// calculateSegmentShares groups rows by Segment (skipping empty Segment
// values) and computes each segment's share of grandTotal.
func calculateSegmentShares(rows []CustomerPeriodRevenue, grandTotal float64) []SegmentShare {
	bySegment := make(map[string]float64)
	for _, row := range rows {
		if row.Segment == "" {
			continue
		}
		bySegment[row.Segment] += row.Amount
	}
	if len(bySegment) == 0 {
		return nil
	}

	segments := make([]string, 0, len(bySegment))
	for s := range bySegment {
		segments = append(segments, s)
	}
	sort.Strings(segments)

	shares := make([]SegmentShare, 0, len(segments))
	for _, s := range segments {
		amt := bySegment[s]
		share := SegmentShare{Segment: s, Revenue: AvailableValue(amt)}
		if grandTotal != 0 {
			share.Percent = AvailableValue(amt / grandTotal)
		}
		shares = append(shares, share)
	}
	return shares
}
