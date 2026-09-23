package inventory

// PeriodFinancials is optional, portable, per-period business metrics —
// task sections 2B and 44. Used for: (a) turnover/DIO/purchases-vs-usage
// on the summary-input path when no item/movement detail is supplied, and
// (b) inventory-specific relationships (inventory growth vs. COGS growth,
// DIO, turnover) even when item-level detail IS supplied. This package
// never requires financial.FinancialDataset — portable typed values only
// (task section 44); an adapter/test can populate this from
// financial/metrics or analytics/ratios output — see financialsadapter_test.go
// and ratiosadapter_test.go.
//
// This is not a general-purpose margin/profitability model — see the
// package doc's "gross margin boundary" (task section 45): Revenue/COGS
// here exist only to support inventory-specific ratios, not to reimplement
// analytics/revenuequality or analytics/ratios' broader margin analysis.
type PeriodFinancials struct {
	Period string `json:"period"`

	// BeginningInventoryValue/EndingInventoryValue are the summary-path's
	// own inventory balance facts, used directly when no
	// InventorySnapshot/Movement detail is supplied for the period (or as
	// a caller-supplied override average-basis input — see turnover.go).
	// Optional.
	BeginningInventoryValue Value `json:"beginning_inventory_value"`
	EndingInventoryValue    Value `json:"ending_inventory_value"`

	Revenue Value `json:"revenue"`
	COGS    Value `json:"cogs"`

	// PurchaseValue/PurchaseUnits are the summary-path's own purchase
	// facts for the period, used when no PURCHASE_RECEIPT Movement detail
	// is supplied — see usage.go.
	PurchaseValue Value `json:"purchase_value"`
	PurchaseUnits Value `json:"purchase_units"`
	// UnitsSold/UnitsUsed is the summary-path's own outbound-usage fact,
	// used when no outbound Movement detail is supplied.
	UnitsSold Value `json:"units_sold"`

	Currency string `json:"currency,omitempty"`
}

// GLControl is an optional caller-supplied GL inventory control balance
// for one period/as-of point — task section 37. Component is empty for a
// single combined inventory control account, or a caller-defined
// component label (e.g. "RAW_MATERIAL", "FINISHED_GOODS") for multi-
// account reconciliation — task section 38: "use caller-defined
// component/category mapping," never a hard-coded manufacturing
// requirement.
type GLControl struct {
	Period    string  `json:"period,omitempty"`
	AsOfDate  string  `json:"as_of_date,omitempty"`
	Component string  `json:"component,omitempty"`
	Balance   float64 `json:"balance"`
	Currency  string  `json:"currency,omitempty"`
}
