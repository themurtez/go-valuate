package profitability

// MarginLeakage reports the cost-share/rate breakdown used to spot
// margin leakage for one entity/period — task section 47. Every field is
// a Value, Unavailable when NetRevenue is zero.
type MarginLeakage struct {
	ReturnRate                       Value `json:"return_rate"`
	DiscountRate                     Value `json:"discount_rate"`
	DirectMaterialPercentRevenue     Value `json:"direct_material_percent_revenue"`
	DirectLaborPercentRevenue        Value `json:"direct_labor_percent_revenue"`
	SubcontractorPercentRevenue      Value `json:"subcontractor_percent_revenue"`
	FulfillmentPercentRevenue        Value `json:"fulfillment_percent_revenue"`
	VariableCommissionPercentRevenue Value `json:"variable_commission_percent_revenue"`
	PaymentFeePercentRevenue         Value `json:"payment_fee_percent_revenue"`
}

// buildMarginLeakage computes MarginLeakage for one EntityPeriodResult.
// Rates use GrossRevenue as the denominator (the pre-reduction base a
// return/discount rate is conventionally expressed against); cost shares
// use NetRevenue (the bridge's own downstream base) — both Unavailable
// when their respective denominator is zero.
func buildMarginLeakage(r EntityPeriodResult) MarginLeakage {
	gross := r.RevenueBridge.GrossRevenue
	net := r.RevenueBridge.NetRevenue
	return MarginLeakage{
		ReturnRate:                       marginValue(r.RevenueBridge.Returns, gross),
		DiscountRate:                     marginValue(r.RevenueBridge.Discounts, gross),
		DirectMaterialPercentRevenue:     marginValue(r.DirectCostBridge.DirectMaterial, net),
		DirectLaborPercentRevenue:        marginValue(r.DirectCostBridge.DirectLabor, net),
		SubcontractorPercentRevenue:      marginValue(r.DirectCostBridge.DirectSubcontractor, net),
		FulfillmentPercentRevenue:        marginValue(r.DirectCostBridge.DirectFulfillment, net),
		VariableCommissionPercentRevenue: marginValue(r.VariableCostBridge.VariableCommission, net),
		PaymentFeePercentRevenue:         marginValue(r.VariableCostBridge.VariablePaymentFees, net),
	}
}

// TrendPoint is one chronological period's value for a trend series.
type TrendPoint struct {
	Period    string  `json:"period"`
	Available bool    `json:"available"`
	Value     float64 `json:"value"`
}

// AdjacentChange is the most recent two available TrendPoints' change —
// task section 48. Unavailable if fewer than two points have data.
type AdjacentChange struct {
	Available      bool    `json:"available"`
	FromPeriod     string  `json:"from_period,omitempty"`
	ToPeriod       string  `json:"to_period,omitempty"`
	AbsoluteChange float64 `json:"absolute_change,omitempty"`
	PercentChange  Value   `json:"percent_change"`
}

func buildAdjacentChange(points []TrendPoint) AdjacentChange {
	var available []TrendPoint
	for _, p := range points {
		if p.Available {
			available = append(available, p)
		}
	}
	if len(available) < 2 {
		return AdjacentChange{}
	}
	from := available[len(available)-2]
	to := available[len(available)-1]
	change := AdjacentChange{Available: true, FromPeriod: from.Period, ToPeriod: to.Period, AbsoluteChange: to.Value - from.Value}
	change.PercentChange = marginValue(to.Value-from.Value, absFloat(from.Value))
	return change
}

// EntityTrend bundles every adjacent-period change series for one entity
// — task section 48. No prediction — every field is a factual historical
// comparison.
type EntityTrend struct {
	Dimension Dimension `json:"dimension"`
	EntityID  string    `json:"entity_id"`

	NetRevenue            []TrendPoint `json:"net_revenue,omitempty"`
	GrossProfit           []TrendPoint `json:"gross_profit,omitempty"`
	GrossMargin           []TrendPoint `json:"gross_margin,omitempty"`
	ContributionProfit    []TrendPoint `json:"contribution_profit,omitempty"`
	ContributionMargin    []TrendPoint `json:"contribution_margin,omitempty"`
	AllocatedProfit       []TrendPoint `json:"allocated_profit,omitempty"`
	AllocatedMargin       []TrendPoint `json:"allocated_margin,omitempty"`
	DirectLaborPercent    []TrendPoint `json:"direct_labor_percent,omitempty"`
	DirectMaterialPercent []TrendPoint `json:"direct_material_percent,omitempty"`
	ReturnsPercent        []TrendPoint `json:"returns_percent,omitempty"`
	DiscountPercent       []TrendPoint `json:"discount_percent,omitempty"`

	NetRevenueChange            AdjacentChange `json:"net_revenue_change"`
	GrossProfitChange           AdjacentChange `json:"gross_profit_change"`
	GrossMarginChange           AdjacentChange `json:"gross_margin_change"`
	ContributionProfitChange    AdjacentChange `json:"contribution_profit_change"`
	ContributionMarginChange    AdjacentChange `json:"contribution_margin_change"`
	AllocatedProfitChange       AdjacentChange `json:"allocated_profit_change"`
	AllocatedMarginChange       AdjacentChange `json:"allocated_margin_change"`
	DirectLaborPercentChange    AdjacentChange `json:"direct_labor_percent_change"`
	DirectMaterialPercentChange AdjacentChange `json:"direct_material_percent_change"`
	ReturnsPercentChange        AdjacentChange `json:"returns_percent_change"`
	DiscountPercentChange       AdjacentChange `json:"discount_percent_change"`
}

func valueToPoint(period string, v Value) TrendPoint {
	return TrendPoint{Period: period, Available: v.Available, Value: v.Amount}
}

func floatToPoint(period string, v float64) TrendPoint {
	return TrendPoint{Period: period, Available: true, Value: v}
}

// buildEntityTrend computes EntityTrend from one entity's chronologically
// ordered EntityPeriodResults (periods must already be sorted).
func buildEntityTrend(dim Dimension, entityID string, periods []EntityPeriodResult, minPeriods int) EntityTrend {
	t := EntityTrend{Dimension: dim, EntityID: entityID}
	if len(periods) < minPeriods {
		return t
	}
	for _, p := range periods {
		leak := buildMarginLeakage(p)
		t.NetRevenue = append(t.NetRevenue, floatToPoint(p.Period, p.RevenueBridge.NetRevenue))
		t.GrossProfit = append(t.GrossProfit, floatToPoint(p.Period, p.GrossProfit))
		t.GrossMargin = append(t.GrossMargin, valueToPoint(p.Period, p.Margins.GrossMargin))
		t.ContributionProfit = append(t.ContributionProfit, floatToPoint(p.Period, p.ContributionProfit))
		t.ContributionMargin = append(t.ContributionMargin, valueToPoint(p.Period, p.Margins.ContributionMargin))
		t.AllocatedProfit = append(t.AllocatedProfit, floatToPoint(p.Period, p.AllocatedProfit))
		t.AllocatedMargin = append(t.AllocatedMargin, valueToPoint(p.Period, p.Margins.AllocatedMargin))
		t.DirectLaborPercent = append(t.DirectLaborPercent, valueToPoint(p.Period, leak.DirectLaborPercentRevenue))
		t.DirectMaterialPercent = append(t.DirectMaterialPercent, valueToPoint(p.Period, leak.DirectMaterialPercentRevenue))
		t.ReturnsPercent = append(t.ReturnsPercent, valueToPoint(p.Period, leak.ReturnRate))
		t.DiscountPercent = append(t.DiscountPercent, valueToPoint(p.Period, leak.DiscountRate))
	}
	t.NetRevenueChange = buildAdjacentChange(t.NetRevenue)
	t.GrossProfitChange = buildAdjacentChange(t.GrossProfit)
	t.GrossMarginChange = buildAdjacentChange(t.GrossMargin)
	t.ContributionProfitChange = buildAdjacentChange(t.ContributionProfit)
	t.ContributionMarginChange = buildAdjacentChange(t.ContributionMargin)
	t.AllocatedProfitChange = buildAdjacentChange(t.AllocatedProfit)
	t.AllocatedMarginChange = buildAdjacentChange(t.AllocatedMargin)
	t.DirectLaborPercentChange = buildAdjacentChange(t.DirectLaborPercent)
	t.DirectMaterialPercentChange = buildAdjacentChange(t.DirectMaterialPercent)
	t.ReturnsPercentChange = buildAdjacentChange(t.ReturnsPercent)
	t.DiscountPercentChange = buildAdjacentChange(t.DiscountPercent)
	return t
}
