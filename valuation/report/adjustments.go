package report

// AdjustmentSummary is the report's normalization-adjustment overview:
// which adjustments were applied, and the resulting normalized-EBITDA/SDE
// bridges — reshaped from financial/adjustments.Result (see build.go) into
// this package's plain, presentation-neutral form.
type AdjustmentSummary struct {
	// Applied lists every adjustment that contributed to at least one
	// bridge, across every period supplied to Build.
	Applied []AppliedAdjustment `json:"applied,omitempty"`
	// EBITDABridge is the normalized-EBITDA bridge line items, when
	// supplied.
	EBITDABridge []BridgeLine `json:"ebitda_bridge,omitempty"`
	// SDEBridge is the normalized-SDE bridge line items, when supplied.
	SDEBridge []BridgeLine `json:"sde_bridge,omitempty"`
}

// AppliedAdjustment is one normalization adjustment that was applied,
// reshaped from financial/adjustments.AppliedLine.
type AppliedAdjustment struct {
	// Period is the period this adjustment applied to.
	Period string `json:"period"`
	// Type echoes the adjustment's financial/adjustments.Type as a plain
	// string.
	Type string `json:"type"`
	// Reason echoes the adjustment's caller-supplied Reason.
	Reason string `json:"reason"`
	// SignedAmount echoes financial/adjustments.AppliedLine.SignedAmount
	// (positive = increased the target metric, negative = decreased it).
	SignedAmount float64 `json:"signed_amount"`
	// Target names which bridge (e.g. "ebitda", "sde") this line
	// contributed to.
	Target string `json:"target"`
}

// BridgeLine is a single labeled step in a normalized-EBITDA or
// normalized-SDE bridge walk, from the starting reported figure through
// every applied adjustment to the final normalized figure — reshaped from
// financial/adjustments.Bridge so a report can render the bridge as a
// simple ordered list without depending on that package's types directly.
type BridgeLine struct {
	// Label describes this line (e.g. "Reported EBITDA",
	// "+ Owner Discretionary Expense", "Normalized EBITDA").
	Label string `json:"label"`
	// Amount is this line's contribution (the starting/ending totals are
	// carried as their own lines with the full running figure; adjustment
	// lines carry their signed delta) — see build.go for exactly how a
	// financial/adjustments.Bridge is flattened into this shape.
	Amount float64 `json:"amount"`
	// IsTotal marks a line as a running total (the starting reported figure
	// or the final normalized figure) rather than an individual adjustment
	// delta, so a renderer can bold/emphasize it distinctly.
	IsTotal bool `json:"is_total"`
}
