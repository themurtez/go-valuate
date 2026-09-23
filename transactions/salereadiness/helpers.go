package salereadiness

import (
	"sort"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/metrics"
)

// chronologicalSnapshots orders snapshots by Input.PeriodMeta's FiscalYear/
// Type/SequenceInYear ranking, mirroring analytics/qoe.chronologicalPeriods'
// identical tie-break rule exactly (fiscal-year/YTD rank 0, quarter rank 1,
// month rank 2, unknown rank 3). ok is false if meta does not cover every
// snapshot's period, in which case the caller falls back to snapshots'
// original (dataset/lexical) order rather than guessing a partial sort —
// the same fallback discipline every analytics sibling package's History
// ordering uses.
func chronologicalSnapshots(snapshots []metrics.Snapshot, meta map[financial.Period]metrics.PeriodInfo) ([]metrics.Snapshot, bool) {
	type entry struct {
		snap metrics.Snapshot
		info metrics.PeriodInfo
	}
	entries := make([]entry, 0, len(snapshots))
	for _, s := range snapshots {
		info, ok := meta[s.Period]
		if !ok {
			return nil, false
		}
		entries = append(entries, entry{snap: s, info: info})
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
		ordered[i] = e.snap
	}
	return ordered, true
}

// mostRecentSnapshot returns the chronologically last snapshot in
// snapshots when meta covers every snapshot's period, otherwise the last
// element in snapshots' own (dataset/lexical) order. ok is false when
// snapshots is empty.
func mostRecentSnapshot(snapshots []metrics.Snapshot, meta map[financial.Period]metrics.PeriodInfo) (metrics.Snapshot, bool) {
	if len(snapshots) == 0 {
		return metrics.Snapshot{}, false
	}
	if ordered, ok := chronologicalSnapshots(snapshots, meta); ok {
		return ordered[len(ordered)-1], true
	}
	return snapshots[len(snapshots)-1], true
}
