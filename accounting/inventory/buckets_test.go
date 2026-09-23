package inventory_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/inventory"
)

// TestBuckets_InvalidConfigurationNoTerminal verifies a bucket schema
// missing an open-ended terminal bucket is rejected.
func TestBuckets_InvalidConfigurationNoTerminal(t *testing.T) {
	in := inventory.Input{AsOfDate: "2025-06-30"}
	policy := inventory.Policy{Buckets: []inventory.BucketDefinition{
		{Code: "A", MinDays: 0, MaxDays: 30, HasMax: true},
	}}
	result := inventory.Calculate(in, policy)
	if !hasIssueCode(result.Issues, inventory.IssueInvalidPolicy) {
		t.Errorf("expected IssueInvalidPolicy for missing terminal bucket, got %+v", result.Issues)
	}
}

// TestBuckets_InvalidConfigurationOverlap verifies overlapping buckets
// are rejected.
func TestBuckets_InvalidConfigurationOverlap(t *testing.T) {
	in := inventory.Input{AsOfDate: "2025-06-30"}
	policy := inventory.Policy{Buckets: []inventory.BucketDefinition{
		{Code: "A", MinDays: 0, MaxDays: 40, HasMax: true},
		{Code: "B", MinDays: 30, HasMax: false},
	}}
	result := inventory.Calculate(in, policy)
	if !hasIssueCode(result.Issues, inventory.IssueInvalidPolicy) {
		t.Errorf("expected IssueInvalidPolicy for overlapping buckets, got %+v", result.Issues)
	}
}

// TestBuckets_ReservedUnknownCodeCollision verifies a caller cannot
// define a bucket named UNKNOWN, since that code is reserved.
func TestBuckets_ReservedUnknownCodeCollision(t *testing.T) {
	in := inventory.Input{AsOfDate: "2025-06-30"}
	policy := inventory.Policy{Buckets: []inventory.BucketDefinition{
		{Code: "UNKNOWN", MinDays: 0, HasMax: false},
	}}
	result := inventory.Calculate(in, policy)
	if !hasIssueCode(result.Issues, inventory.IssueInvalidPolicy) {
		t.Errorf("expected IssueInvalidPolicy for reserved UNKNOWN code collision, got %+v", result.Issues)
	}
}

// TestBuckets_GapAllowed verifies task section 18's doc comment: unlike
// AR/AP, a gap in a caller's custom value-aging schema is legal (items
// falling in the gap resolve to UnknownAgeBucketCode).
func TestBuckets_GapAllowed(t *testing.T) {
	asOf := mustDate(t, "2025-12-31")
	received := asOf.AddDate(0, 0, -45) // falls in the gap between the two buckets below.
	in := inventory.Input{
		AsOfDate: "2025-12-31",
		Items:    []inventory.Item{item("ITEM-1", "Widgets")},
		Snapshots: []inventory.InventorySnapshot{
			{ID: "SNAP-1", ItemID: "ITEM-1", AsOfDate: asOf, ReceivedDate: &received,
				QuantityOnHand: inventory.AvailableQty(10, "EA"), UnitCost: inventory.AvailableValue(1), Currency: "USD"},
		},
	}
	policy := inventory.Policy{Buckets: []inventory.BucketDefinition{
		{Code: "SHORT", MinDays: 0, MaxDays: 10, HasMax: true},
		{Code: "LONG", MinDays: 100, HasMax: false},
	}}
	result := inventory.Calculate(in, policy)
	if hasIssueCode(result.Issues, inventory.IssueInvalidPolicy) {
		t.Errorf("expected a gap to be a legal configuration, got %+v", result.Issues)
	}
	if result.Aging.Rows[0].BucketCode != inventory.UnknownAgeBucketCode {
		t.Errorf("BucketCode = %q, want %q (falls in gap)", result.Aging.Rows[0].BucketCode, inventory.UnknownAgeBucketCode)
	}
}
