package vendorspend

// ComponentReconciliation is one control total's reconciliation against
// this package's own VendorSpend figure — task section 29. Available
// only when the corresponding ControlTotals field was supplied (non-nil)
// AND is finite — this package never assumes an absent control total is
// zero, and never silently reconciles against a NaN/Inf control (see
// IssueInvalidControlTotal).
type ComponentReconciliation struct {
	Available bool `json:"available"`
	// VendorSpend is this package's own computed figure for the
	// component being reconciled (always NetSpend for the matching
	// scope).
	VendorSpend float64 `json:"vendor_spend"`
	// ControlAmount is the caller-supplied ControlTotals figure.
	ControlAmount float64 `json:"control_amount"`
	Difference    float64 `json:"difference"`
	Tolerance     float64 `json:"tolerance"`
	Reconciled    bool    `json:"reconciled"`
}

// ControlReconciliation bundles every available ComponentReconciliation —
// task section 29. Offsetting differences remain visible: each component
// is reconciled independently against its own VendorSpend scope, never
// summed/netted against another component before comparison — task
// section 29's "offsetting component differences must remain visible"
// rule.
type ControlReconciliation struct {
	Purchases          ComponentReconciliation `json:"purchases"`
	ExpenseSpend       ComponentReconciliation `json:"expense_spend"`
	CapexSpend         ComponentReconciliation `json:"capex_spend"`
	InventoryPurchases ComponentReconciliation `json:"inventory_purchases"`
	ContractorSpend    ComponentReconciliation `json:"contractor_spend"`
}

// componentRecon builds one ComponentReconciliation for one named
// component, plus an Issue (IssueInvalidControlTotal) if control was
// supplied but is NaN/Inf. Returns the zero (Available == false) value,
// with no Issue, if control is nil (simply not supplied — not an error).
func componentRecon(componentName string, vendorSpend float64, control *float64, tolerance float64) (ComponentReconciliation, *Issue) {
	if control == nil {
		return ComponentReconciliation{}, nil
	}
	if isNonFinite(*control) {
		return ComponentReconciliation{}, &Issue{Code: IssueInvalidControlTotal, Severity: SeverityError,
			Message: "control total for " + componentName + " is non-finite; that component's reconciliation is unavailable"}
	}
	diff := vendorSpend - *control
	return ComponentReconciliation{
		Available: true, VendorSpend: vendorSpend, ControlAmount: *control,
		Difference: diff, Tolerance: tolerance, Reconciled: absFloat(diff) <= tolerance,
	}, nil
}

// computeControlReconciliation builds ControlReconciliation plus any
// IssueInvalidControlTotal findings. Every component is reconciled
// against the SAME overall NetSpend figure by default — task section 29
// does not define a mechanism for a caller to scope, say, CapexSpend
// control to only CAPEX-type records, so a caller wanting a scoped
// reconciliation instead pre-filters Input.SpendRecords into a separate
// Calculate call for that scope (this package's Calculate is stateless
// and cheap to call repeatedly for exactly this reason).
func computeControlReconciliation(netSpend float64, controls ControlTotals, tolerance float64) (ControlReconciliation, []Issue) {
	var cr ControlReconciliation
	var issues []Issue

	components := []struct {
		name    string
		control *float64
		dest    *ComponentReconciliation
	}{
		{"purchases", controls.Purchases, &cr.Purchases},
		{"expense spend", controls.ExpenseSpend, &cr.ExpenseSpend},
		{"capex spend", controls.CapexSpend, &cr.CapexSpend},
		{"inventory purchases", controls.InventoryPurchases, &cr.InventoryPurchases},
		{"contractor spend", controls.ContractorSpend, &cr.ContractorSpend},
	}
	for _, c := range components {
		recon, issue := componentRecon(c.name, netSpend, c.control, tolerance)
		*c.dest = recon
		if issue != nil {
			issues = append(issues, *issue)
		}
	}

	return cr, issues
}
