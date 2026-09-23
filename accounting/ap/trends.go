package ap

import (
	"sort"
	"time"
)

// Snapshot is one historical point-in-time AP aging snapshot: the same
// payable/bill IDs as they existed on an earlier AsOfDate, supplied by the
// caller (never inferred by this package — migration requires the same
// payable ID appearing across snapshots; never inferred from one
// snapshot).
type Snapshot struct {
	// AsOfDate is this snapshot's own analysis date.
	AsOfDate string `json:"as_of_date"`
	// Payables is the open-item list as it existed at AsOfDate. Each
	// Payable.ID is expected to match IDs used in later snapshots or
	// Input.Payables for the same underlying bill, so migration can track
	// it across time.
	Payables []Payable `json:"payables"`
}

// MigrationEntry is one from-bucket -> to-bucket transition observed
// between two chronologically adjacent snapshots (or the latest snapshot
// and the current Input.Payables), for payables present in both.
type MigrationEntry struct {
	FromBucketCode string `json:"from_bucket_code"`
	// ToBucketCode is empty when ToPaid is true (the payable is no longer
	// open in the later snapshot — treated as "paid/cleared").
	ToBucketCode string  `json:"to_bucket_code,omitempty"`
	ToPaid       bool    `json:"to_paid,omitempty"`
	Count        int     `json:"count"`
	Amount       float64 `json:"amount"`
}

// MigrationResult is the aging migration analysis between the most recent
// two chronological aging states available (either two Snapshots, or the
// latest Snapshot vs. current Input.Payables).
type MigrationResult struct {
	Available bool `json:"available"`
	// FromAsOfDate/ToAsOfDate label the two states compared.
	FromAsOfDate string           `json:"from_as_of_date,omitempty"`
	ToAsOfDate   string           `json:"to_as_of_date,omitempty"`
	Entries      []MigrationEntry `json:"entries,omitempty"`
	// CureAmount is the total OpenAmount of payables whose bucket
	// severity decreased or that moved to paid (improvement).
	CureAmount float64 `json:"cure_amount"`
	// DeteriorationAmount is the total OpenAmount of payables whose
	// bucket severity increased.
	DeteriorationAmount float64 `json:"deterioration_amount"`
	// SuppliersImproving/SuppliersDeteriorating count distinct SupplierID
	// values with at least one payable that cured/improved (including
	// moving to paid) or deteriorated between the two compared states.
	SuppliersImproving     int `json:"suppliers_improving"`
	SuppliersDeteriorating int `json:"suppliers_deteriorating"`
}

// calculateMigration compares the most recent snapshot to the current
// aged payables (rows), or, if fewer than the needed states are
// available, returns an unavailable result. Snapshots must be supplied in
// chronological order by the caller; only the LAST snapshot is compared
// against the current state (a caller wanting every adjacent pair can
// call this repeatedly across their own snapshot sequence).
func calculateMigration(snapshots []Snapshot, currentRows []payableAging, sortedBuckets []BucketDefinition, basis AgingBasis, currentAsOf string) MigrationResult {
	if len(snapshots) == 0 {
		return MigrationResult{}
	}
	last := snapshots[len(snapshots)-1]

	fromByID := map[string]payableAging{}
	asOfDate, err := time.Parse("2006-01-02", last.AsOfDate)
	if err != nil {
		return MigrationResult{}
	}
	for _, p := range last.Payables {
		if !includedStatus(p.Status, defaultAgingStatuses()) {
			continue
		}
		days := ageDays(asOfDate, basisDateFor(p, basis))
		code, ok := bucketForDays(sortedBuckets, days)
		if !ok {
			continue
		}
		fromByID[p.ID] = payableAging{p: p, daysPastDue: days, bucketCode: code, includedInAgg: true}
	}

	toByID := map[string]payableAging{}
	for _, row := range currentRows {
		if !row.includedInAgg || row.isCredit {
			continue
		}
		toByID[row.p.ID] = row
	}

	type key struct{ from, to string }
	counts := map[key]int{}
	amounts := map[key]float64{}
	var cure, deterioration float64
	improvedSuppliers := map[string]bool{}
	deterioratedSuppliers := map[string]bool{}

	for id, fromRow := range fromByID {
		toRow, stillOpen := toByID[id]
		fromRank, ok := bucketSeverity(sortedBuckets, fromRow.bucketCode)
		if !ok {
			continue
		}
		if !stillOpen {
			k := key{from: fromRow.bucketCode, to: ""}
			counts[k]++
			amounts[k] += fromRow.p.OpenAmount
			cure += fromRow.p.OpenAmount
			improvedSuppliers[fromRow.p.SupplierID] = true
			continue
		}
		toRank, ok := bucketSeverity(sortedBuckets, toRow.bucketCode)
		if !ok {
			continue
		}
		k := key{from: fromRow.bucketCode, to: toRow.bucketCode}
		counts[k]++
		amounts[k] += toRow.p.OpenAmount
		switch {
		case toRank < fromRank:
			cure += toRow.p.OpenAmount
			improvedSuppliers[toRow.p.SupplierID] = true
		case toRank > fromRank:
			deterioration += toRow.p.OpenAmount
			deterioratedSuppliers[toRow.p.SupplierID] = true
		}
	}

	keys := make([]key, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].from != keys[j].from {
			return keys[i].from < keys[j].from
		}
		return keys[i].to < keys[j].to
	})

	entries := make([]MigrationEntry, 0, len(keys))
	for _, k := range keys {
		entries = append(entries, MigrationEntry{
			FromBucketCode: k.from,
			ToBucketCode:   k.to,
			ToPaid:         k.to == "",
			Count:          counts[k],
			Amount:         amounts[k],
		})
	}

	return MigrationResult{
		Available:              true,
		FromAsOfDate:           last.AsOfDate,
		ToAsOfDate:             currentAsOf,
		Entries:                entries,
		CureAmount:             cure,
		DeteriorationAmount:    deterioration,
		SuppliersImproving:     len(improvedSuppliers),
		SuppliersDeteriorating: len(deterioratedSuppliers),
	}
}

// AgingTrendPoint is one snapshot's aged totals, used by AgingTrend.
type AgingTrendPoint struct {
	AsOfDate      string  `json:"as_of_date"`
	TotalOpen     float64 `json:"total_open"`
	OverdueTotal  float64 `json:"overdue_total"`
	Overdue60Plus float64 `json:"overdue_60_plus"`
	Overdue90Plus float64 `json:"overdue_90_plus"`
}

// AgingTrend is the historical portfolio-level aging series computed
// across Input.Snapshots (each aged independently using the same
// BucketDefinition schema and AgingBasis as the current analysis).
type AgingTrend struct {
	Available bool              `json:"available"`
	Points    []AgingTrendPoint `json:"points,omitempty"`
}

// calculateAgingTrend ages each Snapshot independently (current Input's
// bucket schema/basis, never the current AsOfDate) and returns one
// AgingTrendPoint per snapshot in caller-supplied order.
func calculateAgingTrend(snapshots []Snapshot, sortedBuckets []BucketDefinition, basis AgingBasis, includeStatuses []PayableStatus) AgingTrend {
	if len(snapshots) == 0 {
		return AgingTrend{}
	}
	var points []AgingTrendPoint
	for _, snap := range snapshots {
		asOf, err := time.Parse("2006-01-02", snap.AsOfDate)
		if err != nil {
			continue
		}
		var total, overdue, o60, o90 float64
		for _, p := range snap.Payables {
			if !includedStatus(p.Status, includeStatuses) {
				continue
			}
			if isNonFinite(p.OpenAmount) {
				continue
			}
			days := ageDays(asOf, basisDateFor(p, basis))
			code, ok := bucketForDays(sortedBuckets, days)
			if !ok {
				continue
			}
			total += p.OpenAmount
			if !bucketIsCurrent(sortedBuckets, code) {
				overdue += p.OpenAmount
			}
			if days >= 60 {
				o60 += p.OpenAmount
			}
			if days >= 90 {
				o90 += p.OpenAmount
			}
		}
		points = append(points, AgingTrendPoint{
			AsOfDate:      snap.AsOfDate,
			TotalOpen:     total,
			OverdueTotal:  overdue,
			Overdue60Plus: o60,
			Overdue90Plus: o90,
		})
	}
	if len(points) == 0 {
		return AgingTrend{}
	}
	return AgingTrend{Available: true, Points: points}
}

func trendDirection(points []float64) string {
	if len(points) < 2 {
		return ""
	}
	change := points[len(points)-1] - points[0]
	switch {
	case change > 0:
		return "deteriorating"
	case change < 0:
		return "improving"
	default:
		return "flat"
	}
}
