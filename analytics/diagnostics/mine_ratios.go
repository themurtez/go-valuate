package diagnostics

// ratioSignalMapping mirrors flagMapping but additionally distinguishes a
// positive signal (mined as a Strength, not a Concern) since
// analytics/ratios is the one sibling package whose Signal taxonomy
// includes explicitly positive codes (SignalImprovingProfitability)
// alongside negative ones — every other Flag-shaped sibling this package
// mines (qoe/cashflow/revenuequality/concentration/debt) only ever raises
// a Flag for a negative condition.
type ratioSignalMapping struct {
	Code              FindingCode
	Category          Category
	Title             string
	RecommendedAction string
	Positive          bool
}

var ratioSignalTable = map[string]ratioSignalMapping{
	"WEAKENING_LIQUIDITY": {
		Code: FindingWeakeningLiquidity, Category: CategoryLiquidity,
		Title:             "Weakening liquidity",
		RecommendedAction: "Review the current ratio's decline and its drivers.",
	},
	"RISING_LEVERAGE": {
		Code: FindingRisingLeverage, Category: CategoryLeverage,
		Title:             "Rising leverage",
		RecommendedAction: "Review the increase in debt-to-EBITDA and its drivers.",
	},
	"MARGIN_COMPRESSION": {
		Code: FindingMarginPressure, Category: CategoryProfitability,
		Title:             "EBITDA margin compression",
		RecommendedAction: "Review the drivers of EBITDA margin compression versus the prior period.",
	},
	"SLOWING_COLLECTIONS": {
		Code: FindingSlowingCollections, Category: CategoryLiquidity,
		Title:             "Slowing customer collections",
		RecommendedAction: "Review days-sales-outstanding trends and collection practices.",
	},
	"INVENTORY_BUILDUP": {
		Code: FindingInventoryBuildup, Category: CategoryWorkingCapital,
		Title:             "Inventory buildup",
		RecommendedAction: "Review days-inventory-outstanding trends and inventory management practices.",
	},
	"WEAK_INTEREST_COVERAGE": {
		Code: FindingWeakInterestCoverage, Category: CategoryLeverage,
		Title:             "Weak interest coverage",
		RecommendedAction: "Review interest coverage relative to operating earnings.",
	},
	"IMPROVING_PROFITABILITY": {
		Code: FindingImprovingProfitability, Category: CategoryProfitability,
		Title:             "Improving profitability",
		RecommendedAction: "Confirm the sustainability of the recent EBITDA margin improvement.",
		Positive:          true,
	},
	"DETERIORATING_PROFITABILITY": {
		Code: FindingMarginPressure, Category: CategoryProfitability,
		Title:             "Deteriorating profitability",
		RecommendedAction: "Review the drivers of EBITDA margin deterioration versus the prior period.",
	},
}

// mineRatios mines Input.Ratios.Signals into Findings, splitting positive
// (ratioSignalMapping.Positive) signals into SeverityInfo Strengths from
// negative signals reported at the Signal's own Severity.
func mineRatios(in Input) []Finding {
	if !in.Ratios.Available {
		return nil
	}
	var out []Finding
	for _, s := range in.Ratios.Signals {
		m, ok := ratioSignalTable[string(s.Code)]
		if !ok {
			continue
		}
		severity := Severity(s.Severity)
		if m.Positive {
			severity = SeverityInfo
		}
		comparison := thresholdValue(s.Threshold, false)
		f := Finding{
			Code:              m.Code,
			Category:          m.Category,
			Severity:          severity,
			Title:             m.Title,
			Evidence:          []string{s.Message},
			SourceModule:      SourceRatios,
			SourceCode:        string(s.Code),
			MetricLabel:       string(s.Code),
			Metric:            AvailableValue(s.Value),
			ComparisonLabel:   comparisonLabelForValue(comparison),
			Comparison:        comparison,
			Period:            s.Period,
			Explanation:       s.Message,
			RecommendedAction: m.RecommendedAction,
		}
		out = append(out, f)
	}
	return out
}
