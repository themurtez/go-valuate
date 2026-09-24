package profitability

// ControlTotals is one period's optional caller-supplied business/GL
// control figures, used only for business-level reconciliation — task
// section 37. Availability-aware: a caller can supply just NetRevenue and
// leave the rest Unavailable.
type ControlTotals struct {
	Period       string `json:"period"`
	NetRevenue   Value  `json:"net_revenue"`
	DirectCost   Value  `json:"direct_cost"`
	VariableCost Value  `json:"variable_cost"`
	SharedCost   Value  `json:"shared_cost"`
}

// ControlComponentReconciliation is one layer's control-vs-computed
// comparison — task section 38 "reconcile each layer separately."
type ControlComponentReconciliation struct {
	Label      string `json:"label"`
	Computed   Value  `json:"computed"`
	Control    Value  `json:"control"`
	Difference Value  `json:"difference"`
	Reconciled bool   `json:"reconciled"`
}

// ControlReconciliation is one period's full control-total reconciliation
// — task sections 37-38.
type ControlReconciliation struct {
	Period     string                           `json:"period"`
	Available  bool                             `json:"available"`
	Components []ControlComponentReconciliation `json:"components,omitempty"`
}

func compareControlComponent(label string, computed Value, control Value, tolerance float64) ControlComponentReconciliation {
	c := ControlComponentReconciliation{Label: label, Computed: computed, Control: control}
	if !computed.Available || !control.Available {
		return c
	}
	diff := computed.Amount - control.Amount
	c.Difference = AvailableValue(diff)
	c.Reconciled = absFloat(diff) <= tolerance
	return c
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// buildControlReconciliation compares one period's business-level
// computed totals against caller-supplied ControlTotals, one layer at a
// time so offsetting differences never hide behind one matching grand
// total — task section 38.
func buildControlReconciliation(period string, computedNetRevenue, computedDirectCost, computedVariableCost, computedSharedCost Value, control *ControlTotals, tolerance float64) ControlReconciliation {
	r := ControlReconciliation{Period: period}
	if control == nil {
		return r
	}
	r.Available = true
	r.Components = []ControlComponentReconciliation{
		compareControlComponent("net_revenue", computedNetRevenue, control.NetRevenue, tolerance),
		compareControlComponent("direct_cost", computedDirectCost, control.DirectCost, tolerance),
		compareControlComponent("variable_cost", computedVariableCost, control.VariableCost, tolerance),
		compareControlComponent("shared_cost", computedSharedCost, control.SharedCost, tolerance),
	}
	return r
}
