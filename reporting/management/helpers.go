package management

import (
	"fmt"
	"sort"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/metrics"
)

// orderedSnapshots returns snapshots reordered chronologically per meta,
// falling back to snapshots' own original order when meta is empty or any
// Snapshot's Period is missing from it — in which case it also returns a
// non-nil *Issue describing the fallback, mirroring
// analytics/ratios.chronologicalPeriods' (trend.go) identical
// all-or-nothing fallback-plus-warning rule: this package's own copy of
// the same convention, per the repository's established
// per-package-duplication pattern for period-ordering helpers
// (analytics/workingcapital, analytics/concentration,
// analytics/revenuequality, analytics/variance each keep their own copy
// too).
//
// This package cannot assume snapshots already arrives chronologically
// ordered: financial/metrics.Calculate builds Snapshots from
// financial.FinancialDataset.Periods(), which sorts lexically, not
// chronologically, regardless of whether PeriodMeta was supplied to that
// call — PeriodMeta there only affects Result.Trend, never Result.Snapshots
// itself.
func orderedSnapshots(snapshots []metrics.Snapshot, meta map[financial.Period]metrics.PeriodInfo) ([]metrics.Snapshot, *Issue) {
	if len(meta) == 0 {
		return snapshots, &Issue{
			Code:     IssueNoPeriodMetaForHistoricalOrder,
			Severity: IssueSeverityWarning,
			Message:  "no PeriodMeta supplied; historical series uses Snapshots' own order, which is not guaranteed chronological",
		}
	}

	type entry struct {
		snapshot metrics.Snapshot
		info     metrics.PeriodInfo
	}
	entries := make([]entry, 0, len(snapshots))
	var missing []financial.Period
	for _, s := range snapshots {
		info, ok := meta[s.Period]
		if !ok {
			missing = append(missing, s.Period)
			continue
		}
		entries = append(entries, entry{snapshot: s, info: info})
	}
	if len(missing) > 0 {
		return snapshots, &Issue{
			Code:     IssueNoPeriodMetaForHistoricalOrder,
			Severity: IssueSeverityWarning,
			Message:  fmt.Sprintf("one or more periods have no PeriodMeta entry: %v; historical series uses Snapshots' own order, which is not guaranteed chronological", missing),
		}
	}

	rank := func(t metrics.PeriodType) int {
		switch t {
		case metrics.PeriodTypeFiscalYear, metrics.PeriodTypeYTD:
			return 0
		case metrics.PeriodTypeQuarter:
			return 1
		case metrics.PeriodTypeMonth:
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

	ordered := make([]metrics.Snapshot, len(entries))
	for i, e := range entries {
		ordered[i] = e.snapshot
	}
	return ordered, nil
}
