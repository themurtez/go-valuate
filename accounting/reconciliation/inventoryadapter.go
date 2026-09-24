package reconciliation

import "github.com/themurtez/go-valuate/accounting/inventory"

// InventorySubledgerBalance reports one component's subledger inventory
// value from result.Reconciliation (already computed by inventory's own
// buildReconciliationSummary — task section 36: "Support inventory
// subledger total vs GL inventory control... Do not recompute aging").
// component matches inventory.ComponentReconciliation.Component ("" for
// the single combined total when the caller supplied no component
// labels). Returns (0, false) if inventory reconciliation is
// unavailable or component was not found among result.Reconciliation.Components.
func InventorySubledgerBalance(result inventory.Result, component string) (float64, bool) {
	if !result.Available || !result.Reconciliation.Available {
		return 0, false
	}
	for _, c := range result.Reconciliation.Components {
		if c.Component == component {
			return c.SubledgerInventoryValue, true
		}
	}
	return 0, false
}

// InventorySubledgerBookBalance builds a BalanceInput for the book side
// of an inventory-control reconciliation from InventorySubledgerBalance.
func InventorySubledgerBookBalance(result inventory.Result, component, asOfDate string) BalanceInput {
	bal, ok := InventorySubledgerBalance(result, component)
	if !ok {
		return BalanceInput{}
	}
	return BalanceInput{EndingBalance: &bal, EndingBalanceDate: asOfDate}
}

// InventoryControlExternalBalance builds a BalanceInput for the external
// (GL-control) side of an inventory-control reconciliation, from
// result.Reconciliation's matching component's GLInventoryBalance — this
// republishes the same GL control balance inventory.Result was already
// given, so a caller does not need to pass that figure twice.
func InventoryControlExternalBalance(result inventory.Result, component, asOfDate string) BalanceInput {
	if !result.Available || !result.Reconciliation.Available {
		return BalanceInput{}
	}
	for _, c := range result.Reconciliation.Components {
		if c.Component == component {
			bal := c.GLInventoryBalance
			return BalanceInput{EndingBalance: &bal, EndingBalanceDate: asOfDate}
		}
	}
	return BalanceInput{}
}
