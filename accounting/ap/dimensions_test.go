package ap_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/ap"
)

func TestDimensions_BreakdownByLocation(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	payables := []ap.Payable{
		{ID: "B-1", SupplierID: "S1", BillDate: mustDate(t, "2025-06-01"), DueDate: mustDate(t, "2025-06-15"),
			OriginalAmount: 1000, OpenAmount: 1000, Currency: "USD", Status: ap.StatusOpen,
			Dimensions: []ap.Dimension{{Key: ap.DimensionLocation, Value: "east"}}},
		{ID: "B-2", SupplierID: "S2", BillDate: mustDate(t, "2025-05-01"), DueDate: mustDate(t, "2025-05-15"),
			OriginalAmount: 2000, OpenAmount: 2000, Currency: "USD", Status: ap.StatusOpen,
			Dimensions: []ap.Dimension{{Key: ap.DimensionLocation, Value: "west"}}},
		{ID: "B-3", SupplierID: "S3", BillDate: mustDate(t, "2025-06-01"), DueDate: mustDate(t, "2025-06-15"),
			OriginalAmount: 500, OpenAmount: 500, Currency: "USD", Status: ap.StatusOpen,
			Dimensions: []ap.Dimension{{Key: ap.DimensionLocation, Value: "east"}}},
		{ID: "B-4", SupplierID: "S4", BillDate: mustDate(t, "2025-06-01"), DueDate: mustDate(t, "2025-06-15"),
			OriginalAmount: 300, OpenAmount: 300, Currency: "USD", Status: ap.StatusOpen}, // no dimension at all
	}
	result := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf, Dimension: ap.DimensionLocation})
	if !result.Dimension.Available {
		t.Fatalf("expected Dimension.Available=true, issues: %+v", result.Issues)
	}
	if result.Dimension.Key != ap.DimensionLocation {
		t.Errorf("Key = %q, want %q", result.Dimension.Key, ap.DimensionLocation)
	}
	if len(result.Dimension.Values) != 2 {
		t.Fatalf("expected 2 distinct dimension values (east, west), got %d: %+v", len(result.Dimension.Values), result.Dimension.Values)
	}
	// west: 2000 (one bill); east: 1000+500=1500 (two bills). Sorted by
	// OpenAmount descending, so west comes first despite fewer bills.
	if result.Dimension.Values[0].Value != "west" || result.Dimension.Values[0].OpenAmount != 2000 {
		t.Errorf("Values[0] = %+v, want west/2000", result.Dimension.Values[0])
	}
	if result.Dimension.Values[1].Value != "east" || result.Dimension.Values[1].OpenAmount != 1500 {
		t.Errorf("Values[1] = %+v, want east/1500", result.Dimension.Values[1])
	}
	// The bill with no dimension at all must be excluded from the
	// breakdown entirely (its amount does not silently spill anywhere).
	var total float64
	for _, v := range result.Dimension.Values {
		total += v.OpenAmount
	}
	if total != 3500 {
		t.Errorf("total dimension breakdown amount = %v, want 3500 (excludes the undimensioned $300 bill)", total)
	}
}

func TestDimensions_EmptyKeyUnavailable(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	payables := []ap.Payable{bill("B-1", "S1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), 1000, 1000, ap.StatusOpen)}
	result := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf})
	if result.Dimension.Available {
		t.Errorf("expected Dimension.Available=false when Options.Dimension is not set")
	}
}

func TestDimensions_KeyNotPresentOnAnyPayable(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	payables := []ap.Payable{bill("B-1", "S1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), 1000, 1000, ap.StatusOpen)}
	result := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf, Dimension: "nonexistent_key"})
	if result.Dimension.Available {
		t.Errorf("expected Dimension.Available=false when no payable has that dimension key")
	}
}
