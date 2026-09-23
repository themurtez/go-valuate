package inventory_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/themurtez/go-valuate/accounting/inventory"
)

func mustDateAny(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

// generateItems builds n items spread across nCategories/nLocations.
func generateItems(n, nCategories, nLocations int) []inventory.Item {
	out := make([]inventory.Item, n)
	for i := 0; i < n; i++ {
		out[i] = inventory.Item{
			ID: fmt.Sprintf("ITEM-%d", i), SKU: fmt.Sprintf("SKU-%d", i),
			Category: fmt.Sprintf("CAT-%d", i%nCategories), Active: true,
			Currency: "USD", UnitOfMeasure: "EA", Location: fmt.Sprintf("LOC-%d", i%nLocations),
		}
	}
	return out
}

// generateSnapshots builds one snapshot per item, at that item's own
// Location.
func generateSnapshots(items []inventory.Item, asOf time.Time) []inventory.InventorySnapshot {
	out := make([]inventory.InventorySnapshot, len(items))
	for i, it := range items {
		out[i] = inventory.InventorySnapshot{
			ID: fmt.Sprintf("SNAP-%d", i), ItemID: it.ID, AsOfDate: asOf, Location: it.Location,
			QuantityOnHand: inventory.AvailableQty(float64(100+i%500), "EA"),
			UnitCost:       inventory.AvailableValue(float64(1 + i%100)),
			Currency:       "USD",
		}
	}
	return out
}

// generateMovements builds n movements spread across the given items,
// alternating purchase receipts and customer shipments over a 180-day
// window ending at asOf. Field values use different, co-prime-ish moduli
// (and a per-movement ReferenceID) deliberately so that the
// (ItemID, date, Type, Quantity, Amount, ReferenceID) signature
// findPossibleDuplicateMovements groups by stays realistically diverse at
// scale, rather than an artificial, tightly correlated cycle that would
// otherwise pack an unrealistic number of movements into one matching
// signature group and defeat the point of exercising this package at
// realistic scale (task section 79's "avoid O(N²)" — a signature group's
// pairwise-comparison cost is intentionally bounded regardless, see
// maxDuplicateGroupSize, but a benchmark should still reflect data an
// actual caller would plausibly supply).
func generateMovements(n int, items []inventory.Item, asOf time.Time) []inventory.Movement {
	out := make([]inventory.Movement, n)
	base := asOf.AddDate(0, 0, -180)
	for i := 0; i < n; i++ {
		item := items[i%len(items)]
		date := base.AddDate(0, 0, (i*37)%180)
		typ := inventory.MovementPurchaseReceipt
		if i%2 == 1 {
			typ = inventory.MovementCustomerShipment
		}
		out[i] = inventory.Movement{
			ID: fmt.Sprintf("MV-%d", i), ItemID: item.ID, Date: date, Type: typ,
			Quantity:    inventory.AvailableQty(float64(1+(i*7)%997), "EA"),
			UnitCost:    inventory.AvailableValue(float64(1 + (i*11)%89)),
			ReferenceID: fmt.Sprintf("REF-%d", i%(n/10+1)),
			Currency:    "USD",
		}
	}
	return out
}

// BenchmarkCalculate_100kItems exercises task section 79's "100,000
// items" scale, with one snapshot per item (100,000 snapshots) and a
// modest movement set, across 100 categories/locations.
func BenchmarkCalculate_100kItems(b *testing.B) {
	asOf := mustDateAny("2025-06-30")
	items := generateItems(100000, 100, 100)
	snapshots := generateSnapshots(items, asOf)
	movements := generateMovements(50000, items, asOf)

	in := inventory.Input{AsOfDate: "2025-06-30", Items: items, Snapshots: snapshots, Movements: movements}
	policy := inventory.Policy{SlowMovingDays: 60, NonMovingDays: 180}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = inventory.Calculate(in, policy)
	}
}

// BenchmarkCalculate_1MMovements exercises task section 79's "1,000,000
// movements" scale against a smaller, realistic item count (10,000) —
// focuses on movement-aggregation and last-movement-lookup cost.
func BenchmarkCalculate_1MMovements(b *testing.B) {
	asOf := mustDateAny("2025-06-30")
	items := generateItems(10000, 100, 100)
	snapshots := generateSnapshots(items, asOf)
	movements := generateMovements(1000000, items, asOf)

	in := inventory.Input{AsOfDate: "2025-06-30", Items: items, Snapshots: snapshots, Movements: movements}
	policy := inventory.Policy{SlowMovingDays: 60, NonMovingDays: 180}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = inventory.Calculate(in, policy)
	}
}

// BenchmarkCalculate_100kSnapshots exercises task section 79's "100,000
// snapshots" scale directly: multiple lots per item so snapshot count
// exceeds item count, stressing per-item (location, lot) aggregation.
func BenchmarkCalculate_100kSnapshots(b *testing.B) {
	asOf := mustDateAny("2025-06-30")
	items := generateItems(20000, 100, 100)
	snapshots := make([]inventory.InventorySnapshot, 0, 100000)
	for i, it := range items {
		for lot := 0; lot < 5; lot++ { // 20,000 items x 5 lots = 100,000 snapshots.
			snapshots = append(snapshots, inventory.InventorySnapshot{
				ID: fmt.Sprintf("SNAP-%d-%d", i, lot), ItemID: it.ID, AsOfDate: asOf, Location: it.Location,
				LotID:          fmt.Sprintf("LOT-%d", lot),
				QuantityOnHand: inventory.AvailableQty(float64(10+lot), "EA"),
				UnitCost:       inventory.AvailableValue(float64(1 + lot)),
				Currency:       "USD",
			})
		}
	}

	in := inventory.Input{AsOfDate: "2025-06-30", Items: items, Snapshots: snapshots}
	policy := inventory.Policy{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = inventory.Calculate(in, policy)
	}
}

// BenchmarkCalculate_60Periods exercises task section 79's "60 periods"
// scale: 60 monthly periods of summary-path financials plus a moderate
// item/movement set, stressing per-period turnover/DIO/rollforward
// computation.
func BenchmarkCalculate_60Periods(b *testing.B) {
	asOf := mustDateAny("2025-12-31")
	items := generateItems(5000, 50, 50)
	snapshots := generateSnapshots(items, asOf)
	movements := generateMovements(200000, items, asOf)

	var periods []inventory.PeriodInfo
	var financials []inventory.PeriodFinancials
	start := mustDateAny("2021-01-01")
	for m := 0; m < 60; m++ {
		periodStart := start.AddDate(0, m, 0)
		periodEnd := periodStart.AddDate(0, 1, -1)
		label := fmt.Sprintf("%04d-%02d", periodStart.Year(), periodStart.Month())
		periods = append(periods, inventory.PeriodInfo{Period: label, StartDate: periodStart, EndDate: periodEnd, Days: 30})
		financials = append(financials, inventory.PeriodFinancials{
			Period: label, BeginningInventoryValue: inventory.AvailableValue(float64(1000000 + m*1000)),
			EndingInventoryValue: inventory.AvailableValue(float64(1010000 + m*1000)),
			COGS:                 inventory.AvailableValue(float64(2000000 + m*5000)),
		})
	}

	in := inventory.Input{AsOfDate: "2025-12-31", Items: items, Snapshots: snapshots, Movements: movements, Periods: periods, Financials: financials}
	policy := inventory.Policy{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = inventory.Calculate(in, policy)
	}
}

// BenchmarkAging_100kItems isolates aging-summary cost at scale.
func BenchmarkAging_100kItems(b *testing.B) {
	asOf := mustDateAny("2025-06-30")
	items := generateItems(100000, 100, 100)
	snapshots := generateSnapshots(items, asOf)

	in := inventory.Input{AsOfDate: "2025-06-30", Items: items, Snapshots: snapshots}
	policy := inventory.Policy{SlowMovingDays: 60, NonMovingDays: 180}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := inventory.Calculate(in, policy)
		_ = result.Aging
	}
}

// BenchmarkConcentration_100kItems isolates concentration cost at scale
// (100 categories/locations).
func BenchmarkConcentration_100kItems(b *testing.B) {
	asOf := mustDateAny("2025-06-30")
	items := generateItems(100000, 100, 100)
	snapshots := generateSnapshots(items, asOf)

	in := inventory.Input{AsOfDate: "2025-06-30", Items: items, Snapshots: snapshots}
	policy := inventory.Policy{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := inventory.Calculate(in, policy)
		_ = result.Concentration
	}
}

// BenchmarkRollforward_60Periods_5kItems isolates quantity/value
// rollforward cost across many periods.
func BenchmarkRollforward_60Periods_5kItems(b *testing.B) {
	asOf := mustDateAny("2025-12-31")
	items := generateItems(5000, 50, 50)
	snapshots := generateSnapshots(items, asOf)
	movements := generateMovements(200000, items, asOf)

	var periods []inventory.PeriodInfo
	start := mustDateAny("2021-01-01")
	for m := 0; m < 60; m++ {
		periodStart := start.AddDate(0, m, 0)
		periodEnd := periodStart.AddDate(0, 1, -1)
		periods = append(periods, inventory.PeriodInfo{Period: fmt.Sprintf("P%d", m), StartDate: periodStart, EndDate: periodEnd, Days: 30})
	}

	in := inventory.Input{AsOfDate: "2025-12-31", Items: items, Snapshots: snapshots, Movements: movements, Periods: periods}
	policy := inventory.Policy{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := inventory.Calculate(in, policy)
		_ = result.Periods
	}
}

// BenchmarkCalculate_2MMovements is a scaling-verification companion to
// BenchmarkCalculate_1MMovements (not part of the task's required scale
// list). Comparing this benchmark's time against BenchmarkCalculate_1MMovements
// is exactly how a real O(N^2) bug in findPossibleDuplicateMovements was
// found during this package's development: 2x movements measured ~8.4x
// runtime before maxDuplicateGroupSize was added (see
// TestDuplicates_LargeGroupNeverBlowsUp for the permanent correctness
// regression test) and this benchmark's generator was made to produce
// realistically diverse data. Kept in the suite as an ongoing scaling
// guard: a future change to movement-aggregation code that reintroduces
// super-linear cost should show up here as a >~2.2x jump relative to
// BenchmarkCalculate_1MMovements.
func BenchmarkCalculate_2MMovements(b *testing.B) {
	asOf := mustDateAny("2025-06-30")
	items := generateItems(10000, 100, 100)
	snapshots := generateSnapshots(items, asOf)
	movements := generateMovements(2000000, items, asOf)

	in := inventory.Input{AsOfDate: "2025-06-30", Items: items, Snapshots: snapshots, Movements: movements}
	policy := inventory.Policy{SlowMovingDays: 60, NonMovingDays: 180}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = inventory.Calculate(in, policy)
	}
}
