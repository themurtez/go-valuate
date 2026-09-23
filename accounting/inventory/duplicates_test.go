package inventory_test

import (
	"fmt"
	"testing"

	"github.com/themurtez/go-valuate/accounting/inventory"
)

// TestDuplicates_LargeGroupNeverBlowsUp is a regression test for a real
// O(N^2) bug found via benchmark scaling verification (task section 79):
// findPossibleDuplicateMovements originally reported every pairwise
// combination within one matching-signature group with no bound, so a
// data set containing one very large group of identical-looking
// movements (e.g. a bulk import that legitimately repeats one signature
// thousands of times) made this function's true cost quadratic — 2x
// movements measured ~8.4x runtime before the fix. This test builds
// exactly that adversarial shape (5,000 movements sharing one signature)
// and asserts Calculate completes and the possible-duplicate output
// stays bounded, not ~12.5 million pairs.
func TestDuplicates_LargeGroupNeverBlowsUp(t *testing.T) {
	const groupSize = 5000
	date := mustDate(t, "2025-06-01")
	movements := make([]inventory.Movement, groupSize)
	for i := 0; i < groupSize; i++ {
		movements[i] = inventory.Movement{
			ID: fmt.Sprintf("MV-%d", i), ItemID: "ITEM-1", Date: date, Type: inventory.MovementCustomerShipment,
			Quantity: inventory.AvailableQty(10, "EA"), Amount: inventory.AvailableValue(100), ReferenceID: "SAME-REF",
		}
	}
	in := periodInput(t, movements)
	result := inventory.Calculate(in, inventory.Policy{})

	// C(5000, 2) would be ~12.5 million without a bound; the capped
	// output must stay small regardless of group size.
	if len(result.PossibleDuplicateMovements) > 5000 {
		t.Errorf("expected a bounded possible-duplicate count, got %d pairs from a %d-movement group", len(result.PossibleDuplicateMovements), groupSize)
	}
	if !hasFlagCode(result.Flags, inventory.FlagPossibleDuplicateMovement) {
		t.Errorf("expected FlagPossibleDuplicateMovement to still fire, got %+v", result.Flags)
	}
}

// TestDuplicates_SmallGroupReportsEveryPair verifies normal, realistic-
// sized groups (2-5 movements) still report every pairwise combination —
// the cap must not silently drop legitimate small-group coverage.
func TestDuplicates_SmallGroupReportsEveryPair(t *testing.T) {
	date := mustDate(t, "2025-06-01")
	movements := []inventory.Movement{
		{ID: "MV-1", ItemID: "ITEM-1", Date: date, Type: inventory.MovementCustomerShipment, Quantity: inventory.AvailableQty(10, "EA"), Amount: inventory.AvailableValue(100), ReferenceID: "REF-1"},
		{ID: "MV-2", ItemID: "ITEM-1", Date: date, Type: inventory.MovementCustomerShipment, Quantity: inventory.AvailableQty(10, "EA"), Amount: inventory.AvailableValue(100), ReferenceID: "REF-1"},
		{ID: "MV-3", ItemID: "ITEM-1", Date: date, Type: inventory.MovementCustomerShipment, Quantity: inventory.AvailableQty(10, "EA"), Amount: inventory.AvailableValue(100), ReferenceID: "REF-1"},
	}
	in := periodInput(t, movements)
	result := inventory.Calculate(in, inventory.Policy{})
	// C(3, 2) = 3 pairs.
	if len(result.PossibleDuplicateMovements) != 3 {
		t.Errorf("expected 3 pairs for a 3-movement group, got %d: %+v", len(result.PossibleDuplicateMovements), result.PossibleDuplicateMovements)
	}
}
