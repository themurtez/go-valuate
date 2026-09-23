package ap_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/ap"
	"github.com/themurtez/go-valuate/analytics/ratios"
	"github.com/themurtez/go-valuate/financial"
)

// TestAdapter_DPOMatchesRatiosPackage_WhenDefinitionsAlign compares DPO
// from accounting/ap (this package's own AP-aging-based DPO) against DPO
// from analytics/ratios (financial.FinancialDataset-based: (AP/Total
// COGS)*365 — see analytics/ratios/efficiency.go), ONLY when both are fed
// equivalent inputs and equivalent definitions: this package's Ending AP
// and COGS values equal to the exact AP/COGS figures in the
// FinancialDataset, both computed over the same 365-day period, and both
// using COGS (not purchases) as the denominator — analytics/ratios has no
// separate purchases-basis DPO, so this adapter is only valid on the
// COGS basis. This is a test adapter, not a hard package dependency —
// accounting/ap never imports analytics/ratios in its own source, per the
// task's "document deliberate differences instead of forcing equality"
// instruction. Mirrors accounting/ar/ratios_adapter_test.go's identical
// pattern.
func TestAdapter_DPOMatchesRatiosPackage_WhenDefinitionsAlign(t *testing.T) {
	const period financial.Period = "2025"
	const endingAP = 20000.0
	const cogs = 200000.0

	ds := financial.FinancialDataset{
		Currency: "USD",
		Items: []financial.NormalizedItem{
			{Code: financial.CodeBsAccountsPayable, Period: period, Amount: endingAP},
			// CodeCogsOther is one of four raw taxonomy codes
			// financial/metrics sums into the single derived TotalCOGS
			// figure analytics/ratios' DPO reads (see
			// financial/metrics/income_statement.go's cogsCodes) — there is
			// no single "total COGS" financial.Code itself.
			{Code: financial.CodeCogsOther, Period: period, Amount: cogs},
		},
	}
	ratiosResult := ratios.Calculate(ratios.Input{Dataset: ds}, ratios.Options{})
	if !ratiosResult.Available || len(ratiosResult.History) != 1 {
		t.Fatalf("expected ratios.Result.Available with 1 period, got %+v", ratiosResult)
	}
	ratiosDPO := ratiosResult.History[0].DaysPayableOutstanding
	if !ratiosDPO.Value.Available {
		t.Fatalf("expected analytics/ratios DPO to be available")
	}

	// accounting/ap's own simple DPO, fed the exact same AP/COGS figures
	// and the same 365-day period length analytics/ratios uses for an
	// annual period, explicitly on the COGS basis.
	asOf := mustDate(t, "2025-12-31")
	apResult := ap.Calculate(ap.Input{
		Payables: []ap.Payable{
			bill("BILL-1", "S1", mustDate(t, "2025-01-01"), mustDate(t, "2025-01-31"), endingAP, endingAP, ap.StatusOpen),
		},
		PurchasesHistory: []ap.PayablesPeriod{
			{Period: period, DenominatorAmount: cogs, Days: 365, Basis: ap.DPOBasisCOGS},
		},
	}, ap.Options{AsOfDate: asOf})

	if !apResult.DPO.Available {
		t.Fatalf("expected accounting/ap DPO to be available")
	}

	const tolerance = 0.01
	diff := apResult.DPO.Value - ratiosDPO.Value.Value
	if diff < -tolerance || diff > tolerance {
		t.Errorf("DPO mismatch: accounting/ap=%v, analytics/ratios=%v", apResult.DPO.Value, ratiosDPO.Value.Value)
	}
}

// TestAdapter_DPODiffersFromRatios_OnPurchasesBasis documents (rather than
// forces equality of) the deliberate difference: when accounting/ap uses
// the textbook-correct purchases basis instead of the COGS proxy,
// analytics/ratios has no equivalent purchases-basis DPO to compare
// against at all — analytics/ratios.RatioDaysPayableOutstanding is always
// COGS-based. This test exists so a future reader does not "fix" the two
// into forced equality.
func TestAdapter_DPODiffersFromRatios_OnPurchasesBasis(t *testing.T) {
	asOf := mustDate(t, "2025-12-31")
	apResult := ap.Calculate(ap.Input{
		Payables: []ap.Payable{
			bill("BILL-1", "S1", mustDate(t, "2025-01-01"), mustDate(t, "2025-01-31"), 20000, 20000, ap.StatusOpen),
		},
		PurchasesHistory: []ap.PayablesPeriod{
			{Period: "2025", DenominatorAmount: 150000, Days: 365, Basis: ap.DPOBasisPurchases},
		},
	}, ap.Options{AsOfDate: asOf})
	if !apResult.DPO.Available {
		t.Fatalf("expected accounting/ap DPO to be available")
	}
	if apResult.DPO.Basis != ap.DPOBasisPurchases {
		t.Errorf("expected DPO.Basis=PURCHASES to be echoed, not silently reported as COGS-equivalent")
	}
}
