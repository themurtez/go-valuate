package inventory_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/inventory"
)

// TestTurnoverHistory_TrendImproving verifies task section 15: DIO
// decreasing first-vs-last is reported as "improving."
func TestTurnoverHistory_TrendImproving(t *testing.T) {
	in := inventory.Input{
		AsOfDate: "2025-12-31",
		Periods: []inventory.PeriodInfo{
			{Period: "2025-Q1", StartDate: mustDate(t, "2025-01-01"), EndDate: mustDate(t, "2025-03-31")},
			{Period: "2025-Q2", StartDate: mustDate(t, "2025-04-01"), EndDate: mustDate(t, "2025-06-30")},
		},
		Financials: []inventory.PeriodFinancials{
			{Period: "2025-Q1", BeginningInventoryValue: inventory.AvailableValue(200000), EndingInventoryValue: inventory.AvailableValue(200000), COGS: inventory.AvailableValue(400000)},
			{Period: "2025-Q2", BeginningInventoryValue: inventory.AvailableValue(200000), EndingInventoryValue: inventory.AvailableValue(200000), COGS: inventory.AvailableValue(900000)}, // COGS way up -> DIO way down.
		},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	if !result.TurnoverHistory.Available {
		t.Fatalf("expected TurnoverHistory.Available=true, got %+v", result.TurnoverHistory)
	}
	if result.TurnoverHistory.DIOTrend != "improving" {
		t.Errorf("DIOTrend = %q, want improving", result.TurnoverHistory.DIOTrend)
	}
	if !result.TurnoverHistory.DIOFirstVsLastChange.Available || result.TurnoverHistory.DIOFirstVsLastChange.Amount >= 0 {
		t.Errorf("DIOFirstVsLastChange = %+v, want a negative change", result.TurnoverHistory.DIOFirstVsLastChange)
	}
}

// TestTurnoverHistory_AdjacentChanges verifies the per-adjacent-pair
// change series across 3+ periods. Periods are given an explicit, equal
// Days (rather than left to derive from calendar-quarter length, which
// varies 90/91/92 days and would itself introduce a small DIO change even
// with identical Average/COGS figures) so a genuinely flat trend is
// unambiguous.
func TestTurnoverHistory_AdjacentChanges(t *testing.T) {
	in := inventory.Input{
		AsOfDate: "2025-12-31",
		Periods: []inventory.PeriodInfo{
			{Period: "2025-Q1", StartDate: mustDate(t, "2025-01-01"), EndDate: mustDate(t, "2025-03-31"), Days: 90},
			{Period: "2025-Q2", StartDate: mustDate(t, "2025-04-01"), EndDate: mustDate(t, "2025-06-30"), Days: 90},
			{Period: "2025-Q3", StartDate: mustDate(t, "2025-07-01"), EndDate: mustDate(t, "2025-09-30"), Days: 90},
		},
		Financials: []inventory.PeriodFinancials{
			{Period: "2025-Q1", BeginningInventoryValue: inventory.AvailableValue(100000), EndingInventoryValue: inventory.AvailableValue(100000), COGS: inventory.AvailableValue(300000)},
			{Period: "2025-Q2", BeginningInventoryValue: inventory.AvailableValue(100000), EndingInventoryValue: inventory.AvailableValue(100000), COGS: inventory.AvailableValue(300000)},
			{Period: "2025-Q3", BeginningInventoryValue: inventory.AvailableValue(100000), EndingInventoryValue: inventory.AvailableValue(100000), COGS: inventory.AvailableValue(300000)},
		},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	if len(result.TurnoverHistory.DIOAdjacentChanges) != 2 {
		t.Fatalf("expected 2 adjacent changes for 3 periods, got %d", len(result.TurnoverHistory.DIOAdjacentChanges))
	}
	if result.TurnoverHistory.DIOTrend != "flat" {
		t.Errorf("DIOTrend = %q, want flat (identical figures and identical explicit Days each period)", result.TurnoverHistory.DIOTrend)
	}
}

// TestTurnoverHistory_DaysVaryByCalendarPeriodLength documents an
// intentional, non-obvious behavior: when PeriodInfo.Days is left
// unspecified, it is derived from each period's own calendar length
// (task section 11's resolvedPeriodDays), so DIO can legitimately differ
// slightly across quarters of different lengths even with identical
// Average Inventory and COGS — this is correct days-outstanding math
// (more days in the period means the same average balance represents
// slightly more "days of COGS"), not a defect.
func TestTurnoverHistory_DaysVaryByCalendarPeriodLength(t *testing.T) {
	in := inventory.Input{
		AsOfDate: "2025-12-31",
		Periods: []inventory.PeriodInfo{
			{Period: "2025-Q1", StartDate: mustDate(t, "2025-01-01"), EndDate: mustDate(t, "2025-03-31")}, // 90 days, Days unset.
			{Period: "2025-Q3", StartDate: mustDate(t, "2025-07-01"), EndDate: mustDate(t, "2025-09-30")}, // 92 days, Days unset.
		},
		Financials: []inventory.PeriodFinancials{
			{Period: "2025-Q1", BeginningInventoryValue: inventory.AvailableValue(100000), EndingInventoryValue: inventory.AvailableValue(100000), COGS: inventory.AvailableValue(300000)},
			{Period: "2025-Q3", BeginningInventoryValue: inventory.AvailableValue(100000), EndingInventoryValue: inventory.AvailableValue(100000), COGS: inventory.AvailableValue(300000)},
		},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	if len(result.Periods) != 2 {
		t.Fatalf("expected 2 periods, got %d", len(result.Periods))
	}
	q1DIO := result.Periods[0].DIO
	q3DIO := result.Periods[1].DIO
	if q1DIO.Days != 90 || q3DIO.Days != 92 {
		t.Fatalf("expected derived Days 90/92, got %d/%d", q1DIO.Days, q3DIO.Days)
	}
	if q1DIO.Value == q3DIO.Value {
		t.Errorf("expected DIO to differ slightly given different period lengths, both were %v", q1DIO.Value)
	}
}

// TestTurnoverHistory_HistoryUnavailableWithoutPeriods verifies
// TurnoverHistory is simply Unavailable, not an error, when no periods
// were supplied at all.
func TestTurnoverHistory_HistoryUnavailableWithoutPeriods(t *testing.T) {
	in := inventory.Input{AsOfDate: "2025-12-31"}
	result := inventory.Calculate(in, inventory.Policy{})
	if result.TurnoverHistory.Available {
		t.Errorf("expected TurnoverHistory.Available=false without any periods, got %+v", result.TurnoverHistory)
	}
}
