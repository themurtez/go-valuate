package profitability

// RevenueBridge is Gross Revenue less returns/discounts/other-reductions
// plus other revenue — task section 8.
type RevenueBridge struct {
	GrossRevenue          float64 `json:"gross_revenue"`
	Returns               float64 `json:"returns"`
	Discounts             float64 `json:"discounts"`
	OtherRevenueReduction float64 `json:"other_revenue_reduction"`
	OtherRevenue          float64 `json:"other_revenue"`
	NetRevenue            float64 `json:"net_revenue"`
}

// DirectCostBridge is the sum of every direct-cost component — task
// section 8.
type DirectCostBridge struct {
	DirectMaterial      float64 `json:"direct_material"`
	DirectLabor         float64 `json:"direct_labor"`
	DirectSubcontractor float64 `json:"direct_subcontractor"`
	DirectFulfillment   float64 `json:"direct_fulfillment"`
	DirectOther         float64 `json:"direct_other"`
	Total               float64 `json:"total"`
}

// VariableCostBridge is the sum of every variable-operating-cost
// component — task section 8.
type VariableCostBridge struct {
	VariableCommission  float64 `json:"variable_commission"`
	VariablePaymentFees float64 `json:"variable_payment_fees"`
	VariableOther       float64 `json:"variable_other"`
	Total               float64 `json:"total"`
}

// buildRevenueBridge sums amounts's revenue-related components into a
// RevenueBridge.
func buildRevenueBridge(amounts map[Component]float64) RevenueBridge {
	b := RevenueBridge{
		GrossRevenue:          amounts[ComponentGrossRevenue],
		Returns:               amounts[ComponentReturn],
		Discounts:             amounts[ComponentDiscount],
		OtherRevenueReduction: amounts[ComponentOtherRevenueReduction],
		OtherRevenue:          amounts[ComponentOtherRevenue],
	}
	b.NetRevenue = b.GrossRevenue - b.Returns - b.Discounts - b.OtherRevenueReduction + b.OtherRevenue
	return b
}

func buildDirectCostBridge(amounts map[Component]float64) DirectCostBridge {
	b := DirectCostBridge{
		DirectMaterial:      amounts[ComponentDirectMaterial],
		DirectLabor:         amounts[ComponentDirectLabor],
		DirectSubcontractor: amounts[ComponentDirectSubcontractor],
		DirectFulfillment:   amounts[ComponentDirectFulfillment],
		DirectOther:         amounts[ComponentDirectOther],
	}
	b.Total = b.DirectMaterial + b.DirectLabor + b.DirectSubcontractor + b.DirectFulfillment + b.DirectOther
	return b
}

func buildVariableCostBridge(amounts map[Component]float64) VariableCostBridge {
	b := VariableCostBridge{
		VariableCommission:  amounts[ComponentVariableCommission],
		VariablePaymentFees: amounts[ComponentVariablePaymentFee],
		VariableOther:       amounts[ComponentVariableOther],
	}
	b.Total = b.VariableCommission + b.VariablePaymentFees + b.VariableOther
	return b
}

// marginValue computes numerator/denominator as a Value, Unavailable when
// denominator is zero — task section 9 "zero denominator => unavailable,
// no NaN/Inf."
func marginValue(numerator, denominator float64) Value {
	if denominator == 0 {
		return Unavailable()
	}
	return AvailableValue(numerator / denominator)
}
