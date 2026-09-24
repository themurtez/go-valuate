package profitability

// EntityPeriodSummaryInput is one caller-pre-aggregated entity/period
// profitability row — task section 16, for callers who already have
// entity-period totals rather than fact-level detail. Every money field
// is a Value so "not supplied" (Unavailable) is distinguishable from
// "supplied and exactly zero" (AvailableValue(0)) — task section 16 "use
// availability-aware fields where omitted-vs-zero matters."
//
// # Mutually exclusive with the detailed Fact path
//
// A caller supplies EITHER Input.Facts (detailed) OR
// Input.EntityPeriodSummaries (pre-aggregated) for a given
// (Dimension, EntityID, Period) — never both, since combining them would
// double count. Supplying both for the same key is flagged as
// IssueInputModeConflict and the summary row is excluded — see the
// task's section 17 "explicit mutually-exclusive input mode."
type EntityPeriodSummaryInput struct {
	Dimension Dimension `json:"dimension"`
	EntityID  string    `json:"entity_id"`
	Period    string    `json:"period"`

	GrossRevenue Value `json:"gross_revenue"`
	Returns      Value `json:"returns"`
	Discounts    Value `json:"discounts"`
	OtherRevenue Value `json:"other_revenue"`

	DirectMaterial      Value `json:"direct_material"`
	DirectLabor         Value `json:"direct_labor"`
	DirectSubcontractor Value `json:"direct_subcontractor"`
	DirectFulfillment   Value `json:"direct_fulfillment"`
	DirectOther         Value `json:"direct_other"`

	VariableCommission  Value `json:"variable_commission"`
	VariablePaymentFees Value `json:"variable_payment_fees"`
	VariableOther       Value `json:"variable_other"`

	SourceRef SourceRef `json:"source_ref,omitempty"`
}

// summaryKey identifies one EntityPeriodSummaryInput row's slot in the
// same (Dimension, EntityID, Period) space Facts' Attributions populate —
// used to detect Fact/summary overlap (IssueInputModeConflict) and to
// build EntityPeriodResult directly from summary rows when no Facts
// attribute to that slot.
type summaryKey struct {
	Dimension Dimension
	EntityID  string
	Period    string
}

// componentBridgeFromSummary converts one EntityPeriodSummaryInput into
// the same internal per-component amount map buildEntityPeriodResult uses
// for the detailed Fact path, so both input paths share one formula
// implementation — task section 17 "both input paths should use the same
// profitability formulas."
func componentBridgeFromSummary(s EntityPeriodSummaryInput) map[Component]float64 {
	amounts := map[Component]float64{}
	add := func(c Component, v Value) {
		if v.Available {
			amounts[c] = v.Amount
		}
	}
	add(ComponentGrossRevenue, s.GrossRevenue)
	add(ComponentReturn, s.Returns)
	add(ComponentDiscount, s.Discounts)
	add(ComponentOtherRevenue, s.OtherRevenue)
	add(ComponentDirectMaterial, s.DirectMaterial)
	add(ComponentDirectLabor, s.DirectLabor)
	add(ComponentDirectSubcontractor, s.DirectSubcontractor)
	add(ComponentDirectFulfillment, s.DirectFulfillment)
	add(ComponentDirectOther, s.DirectOther)
	add(ComponentVariableCommission, s.VariableCommission)
	add(ComponentVariablePaymentFee, s.VariablePaymentFees)
	add(ComponentVariableOther, s.VariableOther)
	return amounts
}
