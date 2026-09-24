package vendorspend

// ControlTotals supplies caller-known, independently-sourced control
// figures for component reconciliation — task section 29. Every field is
// a *float64 so "not supplied" (nil) is distinct from "supplied as
// zero" — this package never assumes an absent control total is zero.
type ControlTotals struct {
	// Purchases is a caller-known total purchases figure for the scope
	// being reconciled (e.g. from a purchases journal or GL).
	Purchases *float64 `json:"purchases,omitempty"`
	// ExpenseSpend is a caller-known total operating-expense spend
	// figure (e.g. from the income statement).
	ExpenseSpend *float64 `json:"expense_spend,omitempty"`
	// CapexSpend is a caller-known total capital-expenditure figure.
	CapexSpend *float64 `json:"capex_spend,omitempty"`
	// InventoryPurchases is a caller-known total inventory-purchase
	// figure (e.g. from accounting/inventory's purchases-vs-usage
	// analysis).
	InventoryPurchases *float64 `json:"inventory_purchases,omitempty"`
	// ContractorSpend is a caller-known total contractor-labor spend
	// figure (e.g. from accounting/labor's contractor mix).
	ContractorSpend *float64 `json:"contractor_spend,omitempty"`
}

// Input bundles everything Calculate needs.
type Input struct {
	// Periods is the full set of caller-supplied chronological periods —
	// required for every period-scoped output. Calculate returns
	// Available == false with an error Issue if empty.
	Periods []Period `json:"periods"`
	// Suppliers is the supplier master list. Required; a SpendRecord
	// referencing an unknown SupplierID is excluded (IssueUnknownSupplier).
	Suppliers []Supplier `json:"suppliers"`
	// SpendRecords is the full set of economic spend records to analyze.
	// Required; Calculate returns Available == false with an error Issue
	// if empty.
	SpendRecords []SpendRecord `json:"spend_records"`

	// Controls supplies optional control totals for reconciliation — see
	// ControlTotals and ControlReconciliation. Zero value (all nil
	// fields) means no reconciliation is attempted for any component.
	Controls ControlTotals `json:"controls,omitempty"`

	// Policy configures top-N cutoffs, materiality, tail-spend
	// definition, and every other caller-adjustable, non-flag behavior.
	// If the zero value, DefaultPolicy() is used.
	Policy Policy `json:"policy,omitempty"`
}

// Options controls Calculate's optional flag-trigger behavior. The zero
// Options is valid: DefaultThresholds() is used.
type Options struct {
	// Thresholds configures every deterministic flag trigger point — see
	// Thresholds. If the zero value, DefaultThresholds() is used.
	Thresholds Thresholds `json:"thresholds,omitempty"`
}
