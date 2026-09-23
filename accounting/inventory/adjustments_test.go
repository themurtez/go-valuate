package inventory_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/inventory"
)

// TestAdjustments_HighRate verifies task section 35: adjustment rate
// beyond threshold triggers FlagHighInventoryAdjustmentRate.
func TestAdjustments_HighRate(t *testing.T) {
	in := inventory.Input{
		AsOfDate: "2025-01-31",
		Items:    []inventory.Item{item("ITEM-1", "Widgets")},
		Periods: []inventory.PeriodInfo{
			{Period: "2025-01", StartDate: mustDate(t, "2025-01-01"), EndDate: mustDate(t, "2025-01-31")},
		},
		Financials: []inventory.PeriodFinancials{
			{Period: "2025-01", BeginningInventoryValue: inventory.AvailableValue(10000), EndingInventoryValue: inventory.AvailableValue(10000)},
		},
		Movements: []inventory.Movement{
			movement("MV-1", "ITEM-1", mustDate(t, "2025-01-15"), inventory.MovementAdjustmentDecrease, 100, 5), // $500 adjustment vs $10000 average = 5%.
		},
	}
	result := inventory.Calculate(in, inventory.Policy{AdjustmentRateThreshold: 0.02})
	if !hasFlagCode(result.Flags, inventory.FlagHighInventoryAdjustmentRate) {
		t.Errorf("expected FlagHighInventoryAdjustmentRate, got %+v", result.Flags)
	}
}

// TestAdjustments_RepeatedItemAdjustments verifies task section 35.
func TestAdjustments_RepeatedItemAdjustments(t *testing.T) {
	in := inventory.Input{
		AsOfDate: "2025-01-31",
		Items:    []inventory.Item{item("ITEM-1", "Widgets")},
		Periods: []inventory.PeriodInfo{
			{Period: "2025-01", StartDate: mustDate(t, "2025-01-01"), EndDate: mustDate(t, "2025-01-31")},
		},
		Movements: []inventory.Movement{
			movement("MV-1", "ITEM-1", mustDate(t, "2025-01-05"), inventory.MovementAdjustmentDecrease, 1, 1),
			movement("MV-2", "ITEM-1", mustDate(t, "2025-01-10"), inventory.MovementAdjustmentDecrease, 1, 1),
			movement("MV-3", "ITEM-1", mustDate(t, "2025-01-15"), inventory.MovementAdjustmentIncrease, 1, 1),
		},
	}
	result := inventory.Calculate(in, inventory.Policy{RepeatedItemAdjustmentCount: 3})
	if !hasFlagCode(result.Flags, inventory.FlagRepeatedItemAdjustments) {
		t.Errorf("expected FlagRepeatedItemAdjustments, got %+v", result.Flags)
	}
}

// TestAdjustments_LargeWriteOff verifies task section 35.
func TestAdjustments_LargeWriteOff(t *testing.T) {
	in := periodInput(t, []inventory.Movement{
		movement("MV-1", "ITEM-1", mustDate(t, "2025-01-15"), inventory.MovementWriteOff, 1000, 10), // $10,000 write-off.
	})
	result := inventory.Calculate(in, inventory.Policy{LargeWriteOffThreshold: 5000})
	if !hasFlagCode(result.Flags, inventory.FlagLargeWriteOff) {
		t.Errorf("expected FlagLargeWriteOff, got %+v", result.Flags)
	}
	f, _ := findFlag(result.Flags, inventory.FlagLargeWriteOff)
	if f.Value != 10000 {
		t.Errorf("Flag.Value = %v, want 10000", f.Value)
	}
}

// TestAdjustments_PeriodEndAdjustment verifies task section 35's neutral
// timing observation.
func TestAdjustments_PeriodEndAdjustment(t *testing.T) {
	in := periodInput(t, []inventory.Movement{
		movement("MV-1", "ITEM-1", mustDate(t, "2025-01-30"), inventory.MovementAdjustmentDecrease, 5, 5),
	})
	result := inventory.Calculate(in, inventory.Policy{PeriodEndAdjustmentWindowDays: 3})
	if !hasFlagCode(result.Flags, inventory.FlagPeriodEndInventoryAdjustment) {
		t.Errorf("expected FlagPeriodEndInventoryAdjustment, got %+v", result.Flags)
	}
}

// TestAdjustments_NotFlaggedWhenAdjustmentIsMidPeriod is a regression
// test for a real bug found during code review: this flag originally
// fired for ANY adjustment activity anywhere in the period, regardless
// of how close it actually was to PeriodInfo.EndDate — defeating the
// flag's own "period-end timing" meaning. An adjustment dated in the
// middle of a 31-day period, far outside any reasonable window, must
// never trigger FlagPeriodEndInventoryAdjustment.
func TestAdjustments_NotFlaggedWhenAdjustmentIsMidPeriod(t *testing.T) {
	in := periodInput(t, []inventory.Movement{
		movement("MV-1", "ITEM-1", mustDate(t, "2025-01-15"), inventory.MovementAdjustmentDecrease, 5, 5), // 16 days before period end.
	})
	result := inventory.Calculate(in, inventory.Policy{PeriodEndAdjustmentWindowDays: 3})
	if hasFlagCode(result.Flags, inventory.FlagPeriodEndInventoryAdjustment) {
		t.Errorf("expected no FlagPeriodEndInventoryAdjustment for a mid-period adjustment, got %+v", result.Flags)
	}
	// The adjustment must still be counted in the ordinary
	// AdjustmentCount/rate — only the period-end-specific flag is
	// affected.
	if len(result.Periods) != 1 || result.Periods[0].Adjustments.AdjustmentCount != 1 {
		t.Errorf("expected AdjustmentCount=1 regardless of period-end proximity, got %+v", result.Periods)
	}
}

// TestAdjustments_PeriodEndWindowBoundary verifies the exact boundary:
// an adjustment exactly PeriodEndAdjustmentWindowDays before EndDate is
// included (inclusive), one day further out is not.
func TestAdjustments_PeriodEndWindowBoundary(t *testing.T) {
	// Period ends 2025-01-31; window is 3 days.
	atBoundary := inventory.Calculate(periodInput(t, []inventory.Movement{
		movement("MV-1", "ITEM-1", mustDate(t, "2025-01-28"), inventory.MovementAdjustmentDecrease, 5, 5), // exactly 3 days before end.
	}), inventory.Policy{PeriodEndAdjustmentWindowDays: 3})
	if !hasFlagCode(atBoundary.Flags, inventory.FlagPeriodEndInventoryAdjustment) {
		t.Errorf("expected FlagPeriodEndInventoryAdjustment at exactly the window boundary, got %+v", atBoundary.Flags)
	}

	justOutside := inventory.Calculate(periodInput(t, []inventory.Movement{
		movement("MV-1", "ITEM-1", mustDate(t, "2025-01-27"), inventory.MovementAdjustmentDecrease, 5, 5), // 4 days before end.
	}), inventory.Policy{PeriodEndAdjustmentWindowDays: 3})
	if hasFlagCode(justOutside.Flags, inventory.FlagPeriodEndInventoryAdjustment) {
		t.Errorf("expected no FlagPeriodEndInventoryAdjustment just outside the window, got %+v", justOutside.Flags)
	}
}

// TestAdjustments_NeutralLanguage verifies task sections 6/35/66: no
// shrinkage/fraud/theft language appears anywhere in generated
// messages, even when a caller supplies ReasonCode=SHRINKAGE as raw
// source data.
func TestAdjustments_NeutralLanguage(t *testing.T) {
	in := periodInput(t, []inventory.Movement{
		{ID: "MV-1", ItemID: "ITEM-1", Date: mustDate(t, "2025-01-15"), Type: inventory.MovementAdjustmentDecrease,
			Quantity: inventory.AvailableQty(1000, "EA"), UnitCost: inventory.AvailableValue(10), ReasonCode: "SHRINKAGE"},
	})
	result := inventory.Calculate(in, inventory.Policy{LargeWriteOffThreshold: 100, AdjustmentRateThreshold: 0.001})
	forbidden := []string{"shrinkage", "fraud", "theft", "stolen", "steal"}
	checkNoForbiddenLanguage(t, result, forbidden)
}

// checkNoForbiddenLanguage scans every Flag.Message and Issue.Message for
// forbidden substrings (case-insensitive).
func checkNoForbiddenLanguage(t *testing.T, result inventory.Result, forbidden []string) {
	t.Helper()
	lower := func(s string) string {
		out := make([]byte, len(s))
		for i := 0; i < len(s); i++ {
			c := s[i]
			if c >= 'A' && c <= 'Z' {
				c = c - 'A' + 'a'
			}
			out[i] = c
		}
		return string(out)
	}
	contains := func(haystack, needle string) bool {
		return len(needle) == 0 || (len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0)
	}
	for _, f := range result.Flags {
		msg := lower(f.Message)
		for _, word := range forbidden {
			if contains(msg, word) {
				t.Errorf("Flag %s message contains forbidden word %q: %q", f.Code, word, f.Message)
			}
		}
	}
	for _, i := range result.Issues {
		msg := lower(i.Message)
		for _, word := range forbidden {
			if contains(msg, word) {
				t.Errorf("Issue %s message contains forbidden word %q: %q", i.Code, word, i.Message)
			}
		}
	}
}

func indexOf(haystack, needle string) int {
	n, m := len(haystack), len(needle)
	for i := 0; i+m <= n; i++ {
		if haystack[i:i+m] == needle {
			return i
		}
	}
	return -1
}
