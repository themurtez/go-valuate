package variance

import (
	"fmt"
	"sort"

	"github.com/themurtez/go-valuate/financial"
)

// periodsOf returns the distinct periods present in lvs, in order of first
// appearance — the fallback ordering used whenever chronological order (via
// PeriodMeta) is unavailable, mirroring
// concentration.orderedPeriodsOf/revenuequality's identical convention.
func periodsOf(lvs []LineVariance) []financial.Period {
	seen := make(map[financial.Period]bool)
	var periods []financial.Period
	for _, lv := range lvs {
		if !seen[lv.Period] {
			seen[lv.Period] = true
			periods = append(periods, lv.Period)
		}
	}
	return periods
}

// chronologicalPeriods sorts periods by meta's FiscalYear/Type/
// SequenceInYear ordering, mirroring
// concentration.chronologicalPeriods/workingcapital.chronologicalPeriods/
// revenuequality.chronologicalPeriods exactly. If meta is empty or any
// period is missing an entry, it returns periods in their original (first-
// appearance) order plus a descriptive warning Issue.
func chronologicalPeriods(periods []financial.Period, meta map[financial.Period]PeriodInfo) ([]financial.Period, *Issue) {
	if len(meta) == 0 {
		return periods, &Issue{
			Code:     IssueNoPeriodMeta,
			Severity: SeverityWarning,
			Message:  "no PeriodMeta supplied; period trends unavailable",
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
			Message:  fmt.Sprintf("one or more periods have no PeriodMeta entry: %v; period trends unavailable", missing),
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

// periodRank builds a lookup from period to its position in ordered, for
// sortLineVariances to sort by without an O(n*m) scan per comparison.
func periodRank(ordered []financial.Period) map[financial.Period]int {
	rank := make(map[financial.Period]int, len(ordered))
	for i, p := range ordered {
		rank[p] = i
	}
	return rank
}

// sortLineVariances sorts lvs in place by Period (chronologically when meta
// covers every period present, else lexically), then AccountCode ascending,
// then stable on original input order for ties — see Result.LineVariances.
func sortLineVariances(lvs []LineVariance, meta map[financial.Period]PeriodInfo) {
	periods := make([]financial.Period, 0, len(lvs))
	seen := make(map[financial.Period]bool)
	for _, lv := range lvs {
		if !seen[lv.Period] {
			seen[lv.Period] = true
			periods = append(periods, lv.Period)
		}
	}
	sort.Slice(periods, func(i, j int) bool { return periods[i] < periods[j] })

	ordered, issue := chronologicalPeriods(periods, meta)
	if issue != nil {
		ordered = periods // lexical fallback
	}
	rank := periodRank(ordered)

	sort.SliceStable(lvs, func(i, j int) bool {
		ri, rj := rank[lvs[i].Period], rank[lvs[j].Period]
		if ri != rj {
			return ri < rj
		}
		return lvs[i].AccountCode < lvs[j].AccountCode
	})
}

// taxonomyCategoryKey returns lv's financial.CodeCategory grouping key (as a
// plain string), or "" if AccountCode was not recognized — used by
// buildCategorySummaries to build Result.CategorySummaries.
func taxonomyCategoryKey(lv LineVariance) string {
	return string(lv.TaxonomyCategory)
}

// customCategoryKey returns lv's caller-supplied Category — used by
// buildCategorySummaries to build Result.CustomCategorySummaries.
func customCategoryKey(lv LineVariance) string {
	return lv.Category
}

// buildCategorySummaries groups lvs by keyFn (skipping an empty key) and
// aggregates each group into a CategorySummary, sorted by Category
// ascending.
func buildCategorySummaries(lvs []LineVariance, keyFn func(LineVariance) string) []CategorySummary {
	type acc struct {
		lineCount        int
		totalActual      float64
		totalBaseline    float64
		haveBaseline     bool
		totalVariance    float64
		favorableCount   int
		unfavorableCount int
		materialCount    int
	}
	byKey := make(map[string]*acc)
	var keys []string

	for _, lv := range lvs {
		key := keyFn(lv)
		if key == "" {
			continue
		}
		a, ok := byKey[key]
		if !ok {
			a = &acc{}
			byKey[key] = a
			keys = append(keys, key)
		}
		a.lineCount++
		a.totalActual += lv.Actual
		if lv.BaselineAvailable {
			a.totalBaseline += lv.Baseline
			a.haveBaseline = true
			if lv.AbsoluteVariance.Available {
				a.totalVariance += lv.AbsoluteVariance.Value
			}
		}
		switch lv.Favorability {
		case FavorabilityFavorable:
			a.favorableCount++
		case FavorabilityUnfavorable:
			a.unfavorableCount++
		}
		if lv.Materiality == MaterialityMaterial {
			a.materialCount++
		}
	}

	sort.Strings(keys)
	summaries := make([]CategorySummary, 0, len(keys))
	for _, key := range keys {
		a := byKey[key]
		cs := CategorySummary{
			Category:         key,
			LineCount:        a.lineCount,
			TotalActual:      a.totalActual,
			FavorableCount:   a.favorableCount,
			UnfavorableCount: a.unfavorableCount,
			MaterialCount:    a.materialCount,
		}
		if a.haveBaseline {
			cs.TotalBaseline = AvailableValue(a.totalBaseline)
			cs.TotalVariance = AvailableValue(a.totalVariance)
			if a.totalBaseline != 0 {
				cs.TotalPercentVariance = AvailableValue(a.totalVariance / absFloat(a.totalBaseline))
			}
		}
		summaries = append(summaries, cs)
	}
	return summaries
}

// topFavorable returns up to n LineVariances classified FavorabilityFavorable,
// sorted by AbsoluteVariance.Value descending (largest favorable dollar
// impact first), ties broken by AccountCode then Period ascending.
func topFavorable(lvs []LineVariance, n int) []LineVariance {
	var favorable []LineVariance
	for _, lv := range lvs {
		if lv.Favorability == FavorabilityFavorable && lv.AbsoluteVariance.Available {
			favorable = append(favorable, lv)
		}
	}
	sort.SliceStable(favorable, func(i, j int) bool {
		vi, vj := favorable[i].AbsoluteVariance.Value, favorable[j].AbsoluteVariance.Value
		if vi != vj {
			return vi > vj
		}
		if favorable[i].AccountCode != favorable[j].AccountCode {
			return favorable[i].AccountCode < favorable[j].AccountCode
		}
		return favorable[i].Period < favorable[j].Period
	})
	if len(favorable) > n {
		favorable = favorable[:n]
	}
	return favorable
}

// topUnfavorable returns up to n LineVariances classified
// FavorabilityUnfavorable, sorted by AbsoluteVariance.Value ascending (most
// negative-impact-magnitude first), ties broken by AccountCode then Period
// ascending.
func topUnfavorable(lvs []LineVariance, n int) []LineVariance {
	var unfavorable []LineVariance
	for _, lv := range lvs {
		if lv.Favorability == FavorabilityUnfavorable && lv.AbsoluteVariance.Available {
			unfavorable = append(unfavorable, lv)
		}
	}
	sort.SliceStable(unfavorable, func(i, j int) bool {
		vi, vj := unfavorable[i].AbsoluteVariance.Value, unfavorable[j].AbsoluteVariance.Value
		if vi != vj {
			return absFloat(vi) > absFloat(vj)
		}
		if unfavorable[i].AccountCode != unfavorable[j].AccountCode {
			return unfavorable[i].AccountCode < unfavorable[j].AccountCode
		}
		return unfavorable[i].Period < unfavorable[j].Period
	})
	if len(unfavorable) > n {
		unfavorable = unfavorable[:n]
	}
	return unfavorable
}

// buildBridge aggregates every LineVariance with an available baseline into
// the total actual-vs-baseline reconciliation.
func buildBridge(lvs []LineVariance) Bridge {
	var b Bridge
	for _, lv := range lvs {
		if !lv.BaselineAvailable || !lv.AbsoluteVariance.Available {
			continue
		}
		b.LineCount++
		b.TotalActual += lv.Actual
		b.TotalBaseline += lv.Baseline
		b.TotalVariance += lv.AbsoluteVariance.Value
		b.TotalAbsoluteVariance += absFloat(lv.AbsoluteVariance.Value)
		switch lv.Favorability {
		case FavorabilityFavorable:
			b.FavorableVariance += lv.AbsoluteVariance.Value
		case FavorabilityUnfavorable:
			b.UnfavorableVariance += lv.AbsoluteVariance.Value
		}
	}
	if b.TotalBaseline != 0 {
		b.TotalPercentVariance = AvailableValue(b.TotalVariance / absFloat(b.TotalBaseline))
	}
	return b
}

// buildMaterialExceptions returns every LineVariance classified
// MaterialityMaterial, in lvs' existing order, each paired with its
// contribution to bridge.TotalAbsoluteVariance.
func buildMaterialExceptions(lvs []LineVariance, bridge Bridge) []MaterialException {
	var exceptions []MaterialException
	for _, lv := range lvs {
		if lv.Materiality != MaterialityMaterial {
			continue
		}
		me := MaterialException{LineVariance: lv}
		if lv.AbsoluteVariance.Available && bridge.TotalAbsoluteVariance != 0 {
			me.ContributionToTotalVariance = AvailableValue(absFloat(lv.AbsoluteVariance.Value) / bridge.TotalAbsoluteVariance)
		}
		exceptions = append(exceptions, me)
	}
	return exceptions
}

// buildPeriodTrends aggregates lvs into one PeriodTrend per period in
// orderedPeriods.
func buildPeriodTrends(lvs []LineVariance, orderedPeriods []financial.Period) []PeriodTrend {
	byPeriod := make(map[financial.Period][]LineVariance)
	for _, lv := range lvs {
		byPeriod[lv.Period] = append(byPeriod[lv.Period], lv)
	}

	trends := make([]PeriodTrend, 0, len(orderedPeriods))
	for _, p := range orderedPeriods {
		rows := byPeriod[p]
		pt := PeriodTrend{Period: p}
		var totalBaseline float64
		var haveBaseline bool
		var totalVariance float64
		for _, lv := range rows {
			pt.TotalActual += lv.Actual
			if lv.BaselineAvailable {
				totalBaseline += lv.Baseline
				haveBaseline = true
				if lv.AbsoluteVariance.Available {
					totalVariance += lv.AbsoluteVariance.Value
				}
			}
		}
		if haveBaseline {
			pt.TotalBaseline = AvailableValue(totalBaseline)
			pt.TotalVariance = AvailableValue(totalVariance)
			if totalBaseline != 0 {
				pt.TotalPercentVariance = AvailableValue(totalVariance / absFloat(totalBaseline))
			}
		}
		trends = append(trends, pt)
	}
	return trends
}

// buildTrendSummary characterizes TotalVariance's overall first-vs-last
// direction across trends, mirroring
// concentration/revenuequality's identical calculateTrend logic.
func buildTrendSummary(trends []PeriodTrend) TrendSummary {
	var first, last *PeriodTrend
	for i := range trends {
		if !trends[i].TotalVariance.Available {
			continue
		}
		if first == nil {
			first = &trends[i]
		}
		last = &trends[i]
	}
	if first == nil || last == nil || first == last {
		if first != nil {
			return TrendSummary{
				Direction:   TrendStable,
				FirstPeriod: first.Period,
				LastPeriod:  first.Period,
				FirstValue:  first.TotalVariance,
				LastValue:   first.TotalVariance,
			}
		}
		return TrendSummary{Direction: TrendUnavailable}
	}

	ts := TrendSummary{
		FirstPeriod: first.Period,
		LastPeriod:  last.Period,
		FirstValue:  first.TotalVariance,
		LastValue:   last.TotalVariance,
	}

	firstVal, lastVal := first.TotalVariance.Value, last.TotalVariance.Value
	if firstVal == 0 {
		if lastVal == 0 {
			ts.Direction = TrendStable
		} else if lastVal > 0 {
			ts.Direction = TrendIncreasing
		} else {
			ts.Direction = TrendDeclining
		}
		return ts
	}

	change := (lastVal - firstVal) / absFloat(firstVal)
	switch {
	case absFloat(change) <= TrendFlatBandPercent:
		ts.Direction = TrendStable
	case change > 0:
		ts.Direction = TrendIncreasing
	default:
		ts.Direction = TrendDeclining
	}
	return ts
}
