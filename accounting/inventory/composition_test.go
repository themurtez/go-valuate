package inventory_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/inventory"
)

// TestComposition_ByClass verifies task section 46: value/share by
// caller-defined InventoryClass, with unclassified items reported
// separately.
func TestComposition_ByClass(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	in := inventory.Input{
		AsOfDate: "2025-06-30",
		Items: []inventory.Item{
			{ID: "RM-1", Category: "Materials", Class: inventory.ClassRawMaterial, Active: true, Currency: "USD", UnitOfMeasure: "EA"},
			{ID: "FG-1", Category: "Products", Class: inventory.ClassFinishedGood, Active: true, Currency: "USD", UnitOfMeasure: "EA"},
			{ID: "UNK-1", Category: "Other", Active: true, Currency: "USD", UnitOfMeasure: "EA"}, // no Class.
		},
		Snapshots: []inventory.InventorySnapshot{
			snapshot("SNAP-1", "RM-1", asOf, 100, 1), // $100
			snapshot("SNAP-2", "FG-1", asOf, 50, 2),  // $100
			snapshot("SNAP-3", "UNK-1", asOf, 25, 4), // $100, unclassified.
		},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	if !result.Composition.Available {
		t.Fatalf("expected Composition.Available=true, got %+v", result.Composition)
	}
	if len(result.Composition.ByClass) != 2 {
		t.Fatalf("expected 2 classified classes, got %+v", result.Composition.ByClass)
	}
	for _, cs := range result.Composition.ByClass {
		if cs.Value != 100 {
			t.Errorf("class %s Value = %v, want 100", cs.Class, cs.Value)
		}
		if !cs.Share.Available || absDiff(cs.Share.Amount, 1.0/3) > 0.001 {
			t.Errorf("class %s Share = %+v, want ~0.333", cs.Class, cs.Share)
		}
	}
	if result.Composition.UnclassifiedValue != 100 {
		t.Errorf("UnclassifiedValue = %v, want 100", result.Composition.UnclassifiedValue)
	}
}

// TestComposition_NoClassInference verifies task section 46: this
// package never infers a class from an item's name.
func TestComposition_NoClassInference(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	in := inventory.Input{
		AsOfDate: "2025-06-30",
		Items: []inventory.Item{
			{ID: "ITEM-1", Name: "Raw Steel Bar", Category: "Materials", Active: true, Currency: "USD", UnitOfMeasure: "EA"}, // name suggests RAW_MATERIAL but Class is unset.
		},
		Snapshots: []inventory.InventorySnapshot{snapshot("SNAP-1", "ITEM-1", asOf, 10, 5)},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	if len(result.Composition.ByClass) != 0 {
		t.Errorf("expected no class breakdown when Item.Class is unset regardless of Name, got %+v", result.Composition.ByClass)
	}
	if result.Composition.UnclassifiedValue != 50 {
		t.Errorf("UnclassifiedValue = %v, want 50 (never inferred from name)", result.Composition.UnclassifiedValue)
	}
}
