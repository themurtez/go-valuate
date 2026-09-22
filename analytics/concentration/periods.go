package concentration

import (
	"fmt"
	"math"
	"sort"

	"github.com/themurtez/go-valuate/financial"
)

// validateObservations checks every row in obs and returns one advisory
// Issue per problem class found (not one Issue per row, to avoid flooding
// Result.Warnings when many rows share the same problem — mirroring
// revenuequality.validateCustomerRevenue's identical single-Issue-listing-
// every-affected-row convention).
func validateObservations(obs []Observation) []Issue {
	var invalidCount int
	for _, o := range obs {
		if o.Amount < 0 || math.IsNaN(o.Amount) || math.IsInf(o.Amount, 0) {
			invalidCount++
		}
	}

	var issues []Issue
	if invalidCount > 0 {
		issues = append(issues, Issue{
			Code:     IssueInvalidObservation,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("%d observation(s) had a negative, NaN, or infinite amount and were excluded from every computation", invalidCount),
		})
	}
	return issues
}

// filterValidObservations returns only the rows with a valid (non-negative,
// finite) Amount — see IssueInvalidObservation.
func filterValidObservations(obs []Observation) []Observation {
	valid := make([]Observation, 0, len(obs))
	for _, o := range obs {
		if o.Amount < 0 || math.IsNaN(o.Amount) || math.IsInf(o.Amount, 0) {
			continue
		}
		valid = append(valid, o)
	}
	return valid
}

// orderedPeriodsOf returns the distinct periods present in obs, in order of
// first appearance — the fallback ordering used whenever chronological
// order (via PeriodMeta) is unavailable.
func orderedPeriodsOf(obs []Observation) []financial.Period {
	seen := make(map[financial.Period]bool)
	var periods []financial.Period
	for _, o := range obs {
		if !seen[o.Period] {
			seen[o.Period] = true
			periods = append(periods, o.Period)
		}
	}
	return periods
}

// chronologicalPeriods sorts periods by meta's FiscalYear/Type/
// SequenceInYear ordering, mirroring workingcapital.chronologicalPeriods/
// revenuequality.chronologicalPeriods exactly. If meta is empty or any
// period is missing an entry, it returns periods in their original (first-
// appearance) order plus a descriptive warning Issue.
func chronologicalPeriods(periods []financial.Period, meta map[financial.Period]PeriodInfo) ([]financial.Period, *Issue) {
	if len(meta) == 0 {
		return periods, &Issue{
			Code:     IssueNoPeriodMeta,
			Severity: SeverityWarning,
			Message:  "no PeriodMeta supplied; history uses order of first appearance and every ordering-dependent output is unavailable",
		}
	}

	type entry struct {
		period financial.Period
		info   PeriodInfo
	}
	entries := make([]entry, 0, len(periods))
	var missing []financial.Period
	for _, p := range periods {
		info, ok := meta[p]
		if !ok {
			missing = append(missing, p)
			continue
		}
		entries = append(entries, entry{period: p, info: info})
	}
	if len(missing) > 0 {
		sort.Slice(missing, func(i, j int) bool { return missing[i] < missing[j] })
		return periods, &Issue{
			Code:     IssuePeriodMissingFromMeta,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("one or more periods have no PeriodMeta entry: %v; history uses order of first appearance and every ordering-dependent output is unavailable", missing),
		}
	}

	rank := func(t PeriodType) int {
		switch t {
		case PeriodTypeFiscalYear, PeriodTypeYTD:
			return 0
		case PeriodTypeQuarter:
			return 1
		case PeriodTypeMonth:
			return 2
		default:
			return 3
		}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i].info, entries[j].info
		if a.FiscalYear != b.FiscalYear {
			return a.FiscalYear < b.FiscalYear
		}
		if rank(a.Type) != rank(b.Type) {
			return rank(a.Type) < rank(b.Type)
		}
		return a.SequenceInYear < b.SequenceInYear
	})

	ordered := make([]financial.Period, len(entries))
	for i, e := range entries {
		ordered[i] = e.period
	}
	return ordered, nil
}

// groupByPeriod groups obs by Period.
func groupByPeriod(obs []Observation) map[financial.Period][]Observation {
	byPeriod := make(map[financial.Period][]Observation)
	for _, o := range obs {
		byPeriod[o.Period] = append(byPeriod[o.Period], o)
	}
	return byPeriod
}

// entityTotalsByKey sums rows' Amount per distinct EntityKey, returning the
// per-entity totals plus the grand total.
func entityTotalsByKey(rows []Observation) (map[string]float64, float64) {
	totals := make(map[string]float64, len(rows))
	var grandTotal float64
	for _, row := range rows {
		totals[row.EntityKey] += row.Amount
		grandTotal += row.Amount
	}
	return totals, grandTotal
}

// sortedKeys returns m's keys sorted ascending, for deterministic iteration
// over a map whose accumulation order can affect float64 output (float64
// addition is not associative) — mirrors
// revenuequality.sortedKeys/workingcapital's identical determinism
// safeguard.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// rankEntities ranks totals descending by amount, breaking ties by
// EntityKey ascending for deterministic output regardless of Go's map
// iteration order.
func rankEntities(totals map[string]float64, grandTotal float64) []RankedEntity {
	keys := sortedKeys(totals)
	sort.SliceStable(keys, func(i, j int) bool {
		return totals[keys[i]] > totals[keys[j]]
	})

	ranked := make([]RankedEntity, 0, len(keys))
	for i, key := range keys {
		amt := totals[key]
		re := RankedEntity{Rank: i + 1, EntityKey: key, Amount: amt}
		if grandTotal != 0 {
			re.Share = AvailableValue(amt / grandTotal)
		}
		ranked = append(ranked, re)
	}
	return ranked
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

// computeTopNShares computes one TopNShare per cutoff in ns against ranked
// (already sorted descending by Amount) and grandTotal.
func computeTopNShares(ranked []RankedEntity, grandTotal float64, ns []int) []TopNShare {
	shares := make([]TopNShare, 0, len(ns))
	for _, n := range sortedTopN(ns) {
		cutoff := n
		if cutoff > len(ranked) {
			cutoff = len(ranked)
		}
		var topSum float64
		for i := 0; i < cutoff; i++ {
			topSum += ranked[i].Amount
		}
		share := TopNShare{N: n, Amount: AvailableValue(topSum)}
		if grandTotal != 0 {
			share.Share = AvailableValue(topSum / grandTotal)
		}
		shares = append(shares, share)
	}
	return shares
}

// calculateCategoryShares groups rows by Category (skipping empty Category
// values) and computes each category's share of grandTotal.
func calculateCategoryShares(rows []Observation, grandTotal float64) []CategoryShare {
	byCategory := make(map[string]float64)
	for _, row := range rows {
		if row.Category == "" {
			continue
		}
		byCategory[row.Category] += row.Amount
	}
	if len(byCategory) == 0 {
		return nil
	}

	categories := sortedKeys(byCategory)
	shares := make([]CategoryShare, 0, len(categories))
	for _, c := range categories {
		amt := byCategory[c]
		share := CategoryShare{Category: c, Amount: AvailableValue(amt)}
		if grandTotal != 0 {
			share.Share = AvailableValue(amt / grandTotal)
		}
		shares = append(shares, share)
	}
	return shares
}

// computePeriodConcentration aggregates rows (all for the same Period) into
// a full PeriodConcentration.
func computePeriodConcentration(period financial.Period, rows []Observation, topN []int) PeriodConcentration {
	totals, grandTotal := entityTotalsByKey(rows)
	ranked := rankEntities(totals, grandTotal)

	pc := PeriodConcentration{
		Period:         period,
		EntityCount:    len(totals),
		TotalAmount:    AvailableValue(grandTotal),
		RankedEntities: ranked,
	}
	if len(ranked) > 0 {
		pc.LargestEntityShare = ranked[0].Share
	}
	pc.TopNShares = computeTopNShares(ranked, grandTotal, topN)

	if len(totals) > 0 && grandTotal != 0 {
		var hhi float64
		for _, key := range sortedKeys(totals) {
			share := totals[key] / grandTotal
			hhi += share * share
		}
		pc.HHI = AvailableValue(hhi * 10000)
	}

	pc.Categories = calculateCategoryShares(rows, grandTotal)

	return pc
}
