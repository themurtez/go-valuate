package inventory_test

import (
	"math"
	"testing"
	"time"

	"github.com/themurtez/go-valuate/accounting/inventory"
)

func mustDate(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatalf("mustDate(%q): %v", s, err)
	}
	return d
}

func absDiff(a, b float64) float64 {
	return math.Abs(a - b)
}

func isInfOrNaN(v float64) bool {
	return math.IsInf(v, 0) || math.IsNaN(v)
}

func mathNaN() float64 { return math.NaN() }
func mathInf() float64 { return math.Inf(1) }

func hasIssueCode(issues []inventory.Issue, code inventory.IssueCode) bool {
	for _, i := range issues {
		if i.Code == code {
			return true
		}
	}
	return false
}

func hasFlagCode(flags []inventory.Flag, code inventory.FlagCode) bool {
	for _, f := range flags {
		if f.Code == code {
			return true
		}
	}
	return false
}

func findFlag(flags []inventory.Flag, code inventory.FlagCode) (inventory.Flag, bool) {
	for _, f := range flags {
		if f.Code == code {
			return f, true
		}
	}
	return inventory.Flag{}, false
}

// item is a terse constructor for the common test-item shape.
func item(id, category string) inventory.Item {
	return inventory.Item{ID: id, Category: category, Active: true, Currency: "USD", UnitOfMeasure: "EA"}
}

// snapshot is a terse constructor for a quantity x unit-cost snapshot.
func snapshot(id, itemID string, asOf time.Time, qty, unitCost float64) inventory.InventorySnapshot {
	return inventory.InventorySnapshot{
		ID: id, ItemID: itemID, AsOfDate: asOf,
		QuantityOnHand: inventory.AvailableQty(qty, "EA"),
		UnitCost:       inventory.AvailableValue(unitCost),
		Currency:       "USD",
	}
}

// movement is a terse constructor for a quantity x unit-cost movement.
func movement(id, itemID string, date time.Time, typ inventory.MovementType, qty, unitCost float64) inventory.Movement {
	return inventory.Movement{
		ID: id, ItemID: itemID, Date: date, Type: typ,
		Quantity: inventory.AvailableQty(qty, "EA"),
		UnitCost: inventory.AvailableValue(unitCost),
		Currency: "USD",
	}
}
