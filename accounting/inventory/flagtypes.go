package inventory

import "sort"

// FlagCode is a stable identifier for one deterministic business/
// accounting condition — task section 66: a Flag is never an input/
// configuration problem (that is an Issue; see issues.go). This package
// never derives a hidden inventory-health score or composite rating —
// task section 57: "no composite inventory-health score" — every flag is
// a simple, documented threshold comparison the caller can fully see and
// override via Policy. Only codes this package actually emits are
// defined.
type FlagCode string

const (
	FlagSlowMovingInventory                     FlagCode = "SLOW_MOVING_INVENTORY"
	FlagNonMovingInventory                      FlagCode = "NON_MOVING_INVENTORY"
	FlagAgedInventoryReview                     FlagCode = "AGED_INVENTORY_REVIEW"
	FlagUnknownInventoryAge                     FlagCode = "UNKNOWN_INVENTORY_AGE"
	FlagNegativeInventory                       FlagCode = "NEGATIVE_INVENTORY"
	FlagAboveCallerMaximum                      FlagCode = "INVENTORY_ABOVE_CALLER_MAXIMUM"
	FlagBelowCallerMinimum                      FlagCode = "INVENTORY_BELOW_CALLER_MINIMUM"
	FlagPurchasesOutpaceUsage                   FlagCode = "PURCHASES_OUTPACE_USAGE"
	FlagInventoryBuildWithoutMatchingCOGSGrowth FlagCode = "INVENTORY_BUILD_WITHOUT_MATCHING_COGS_GROWTH"
	FlagHighInventoryAdjustmentRate             FlagCode = "HIGH_INVENTORY_ADJUSTMENT_RATE"
	FlagLargeWriteOff                           FlagCode = "LARGE_WRITE_OFF"
	FlagRepeatedItemAdjustments                 FlagCode = "REPEATED_ITEM_ADJUSTMENTS"
	FlagPeriodEndInventoryAdjustment            FlagCode = "PERIOD_END_INVENTORY_ADJUSTMENT"
	FlagInventoryGLMismatch                     FlagCode = "INVENTORY_GL_MISMATCH"
	FlagExpiringInventory                       FlagCode = "EXPIRING_INVENTORY"
	FlagExpiredInventoryReview                  FlagCode = "EXPIRED_INVENTORY_REVIEW"
	FlagPossibleDuplicateMovement               FlagCode = "POSSIBLE_DUPLICATE_MOVEMENT"
)

// flagCodeOrder fixes FlagCode declaration order for deterministic Flags
// sorting — task section 70.
var flagCodeOrder = []FlagCode{
	FlagSlowMovingInventory,
	FlagNonMovingInventory,
	FlagAgedInventoryReview,
	FlagUnknownInventoryAge,
	FlagNegativeInventory,
	FlagAboveCallerMaximum,
	FlagBelowCallerMinimum,
	FlagPurchasesOutpaceUsage,
	FlagInventoryBuildWithoutMatchingCOGSGrowth,
	FlagHighInventoryAdjustmentRate,
	FlagLargeWriteOff,
	FlagRepeatedItemAdjustments,
	FlagPeriodEndInventoryAdjustment,
	FlagInventoryGLMismatch,
	FlagExpiringInventory,
	FlagExpiredInventoryReview,
	FlagPossibleDuplicateMovement,
}

func flagRank(c FlagCode) int {
	for i, fc := range flagCodeOrder {
		if fc == c {
			return i
		}
	}
	return len(flagCodeOrder)
}

// Flag is one deterministic condition Calculate triggered. Every Message
// uses neutral, factual language — never "shrinkage," "fraud," or "theft"
// — see safety_test.go's permanent regression test enforcing this (task
// sections 6/35/66).
type Flag struct {
	Code    FlagCode `json:"code"`
	Message string   `json:"message"`

	ItemID     string `json:"item_id,omitempty"`
	Category   string `json:"category,omitempty"`
	Location   string `json:"location,omitempty"`
	Period     string `json:"period,omitempty"`
	MovementID string `json:"movement_id,omitempty"`

	Value     float64 `json:"value,omitempty"`
	Threshold float64 `json:"threshold,omitempty"`
}

// sortFlags sorts flags by FlagCode declaration order, then ItemID, then
// Period — task section 70's "flags severity/code/item" ordering.
func sortFlags(flags []Flag) {
	sort.SliceStable(flags, func(i, j int) bool {
		ri, rj := flagRank(flags[i].Code), flagRank(flags[j].Code)
		if ri != rj {
			return ri < rj
		}
		if flags[i].ItemID != flags[j].ItemID {
			return flags[i].ItemID < flags[j].ItemID
		}
		return flags[i].Period < flags[j].Period
	})
}
