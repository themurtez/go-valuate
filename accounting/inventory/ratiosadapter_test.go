package inventory_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/inventory"
	"github.com/themurtez/go-valuate/analytics/ratios"
	"github.com/themurtez/go-valuate/financial"
)

// TestAdapter_DIOMatchesRatiosUnderEquivalentInputs proves task section
// 76's DIO invariant, under the ONE case where the two packages' average-
// inventory semantics are genuinely equivalent: analytics/ratios'
// DaysInventoryOutstanding uses a single ending-balance Inventory figure
// (financial.CodeBsInventory, one point-in-time balance per period), never
// a beginning/ending average — see analytics/ratios/efficiency.go's
// daysInventoryOutstanding. This package's own default average basis is
// (Beginning + Ending) / 2 (AverageBasisBeginningEnding), which is NOT the
// same figure whenever beginning != ending. Equality is therefore only
// asserted here for a period where BeginningInventoryValue ==
// EndingInventoryValue (a degenerate case where the average IS the ending
// balance) — this is the "truly equivalent inputs" case task section 76
// requires, not a general claim that these two packages always agree.
// Where they would disagree (non-degenerate periods), this package's
// AverageBasisEndingOnly (used automatically when no beginning balance is
// supplied at all) is the mode that matches ratios' semantics exactly —
// this test exercises the AverageBasisEndingOnly path directly for that
// reason, rather than a same-value degenerate BeginningEnding case, since
// AverageBasisEndingOnly is the mode this package documents as ratios-
// equivalent.
func TestAdapter_DIOMatchesRatiosUnderEquivalentInputs(t *testing.T) {
	const period financial.Period = "2025"
	const endingInventory = 120000.0
	const cogs = 900000.0

	// accounting/inventory: no beginning balance supplied at all, so
	// AverageBasisEndingOnly is used (average == ending balance) — the
	// one mode genuinely equivalent to ratios' single-balance basis.
	invIn := inventory.Input{
		AsOfDate: "2025-12-31",
		Periods: []inventory.PeriodInfo{
			{Period: string(period), StartDate: mustDate(t, "2025-01-01"), EndDate: mustDate(t, "2025-12-31")},
		},
		Financials: []inventory.PeriodFinancials{
			{Period: string(period), EndingInventoryValue: inventory.AvailableValue(endingInventory), COGS: inventory.AvailableValue(cogs)},
		},
	}
	invResult := inventory.Calculate(invIn, inventory.Policy{})
	if len(invResult.Periods) != 1 || !invResult.Periods[0].DIO.Available {
		t.Fatalf("expected 1 available DIO period, got %+v", invResult.Periods)
	}
	if invResult.Periods[0].DIO.AverageBasis != inventory.AverageBasisEndingOnly {
		t.Fatalf("expected AverageBasisEndingOnly, got %q", invResult.Periods[0].DIO.AverageBasis)
	}
	gotDIO := invResult.Periods[0].DIO.Value

	// analytics/ratios: same ending inventory balance and COGS via a
	// synthetic FinancialDataset, using ratios' own fixed 365-day
	// convention (matches this package's default when PeriodInfo.Days
	// resolves to a full calendar year — see resolvedPeriodDays).
	ds := financial.FinancialDataset{
		Currency: "USD",
		Items: []financial.NormalizedItem{
			{Code: financial.CodeBsInventory, Period: period, Amount: endingInventory},
			// analytics/ratios' TotalCOGS is financial/metrics' sum of every
			// individual COGS_* code (see metrics.totalCOGS); there is no
			// single aggregate COGS code in the taxonomy, so this supplies
			// the full COGS figure via one component code.
			{Code: financial.CodeCogsMaterial, Period: period, Amount: cogs},
		},
	}
	ratiosResult := ratios.Calculate(ratios.Input{Dataset: ds}, ratios.Options{})
	if !ratiosResult.Available || len(ratiosResult.History) != 1 {
		t.Fatalf("expected 1 available ratios period, got %+v", ratiosResult)
	}
	ratiosDIO := ratiosResult.History[0].DaysInventoryOutstanding
	if !ratiosDIO.Value.Available {
		t.Fatalf("expected ratios DaysInventoryOutstanding.Value.Available=true, got %+v", ratiosDIO)
	}

	if absDiff(gotDIO, ratiosDIO.Value.Value) > 0.01 {
		t.Errorf("accounting/inventory DIO = %v, analytics/ratios DIO = %v; expected equality under equivalent single-balance inputs", gotDIO, ratiosDIO.Value.Value)
	}
}
