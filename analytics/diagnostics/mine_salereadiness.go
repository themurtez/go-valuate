package diagnostics

// categoryForDimension maps a salereadiness.DimensionCode to the diagnostic
// Category its Blocker/Risk/Strength/Opportunity most directly concerns.
var categoryForDimension = map[string]Category{
	"FINANCIAL_RECORD_QUALITY":   CategoryEarningsQuality,
	"EARNINGS_STABILITY":         CategoryProfitability,
	"NORMALIZATION_BURDEN":       CategoryEarningsQuality,
	"CUSTOMER_CONCENTRATION":     CategoryConcentration,
	"RECURRING_REVENUE":          CategoryRevenueQuality,
	"OWNER_DEPENDENCE":           CategoryTransactionReadiness,
	"MARGIN_TREND":               CategoryProfitability,
	"WORKING_CAPITAL_STABILITY":  CategoryWorkingCapital,
	"DEBT_LEVERAGE":              CategoryLeverage,
	"DATA_COMPLETENESS":          CategoryTransactionReadiness,
	"VALUATION_METHOD_CONSENSUS": CategoryValuation,
}

// categoryForDimensionCode reports categoryForDimension[code], falling back
// to CategoryTransactionReadiness for an unrecognized code (every Finding
// mined from transactions/salereadiness is, at minimum, about transaction
// readiness — see mineSaleReadiness).
func categoryForDimensionCode(code string) Category {
	if c, ok := categoryForDimension[code]; ok {
		return c
	}
	return CategoryTransactionReadiness
}

// mineSaleReadiness mines Input.SaleReadiness's Blockers/Risks/Strengths/
// Opportunities into Findings — the sole source of
// CategoryTransactionReadiness Findings (beyond the fallback above), since
// no other sibling module in Input assesses transaction readiness as such.
func mineSaleReadiness(in Input) (findings []Finding, strengths []Finding, opportunities []Finding) {
	if !in.SaleReadiness.Available {
		return nil, nil, nil
	}

	for _, b := range in.SaleReadiness.Blockers {
		findings = append(findings, Finding{
			Code:              FindingSaleReadinessBlocker,
			Category:          categoryForDimensionCode(string(b.Dimension)),
			Severity:          Severity(b.Severity),
			Title:             "Sale-readiness blocker",
			Evidence:          []string{b.Message},
			SourceModule:      SourceSaleReadiness,
			SourceCode:        string(b.Dimension),
			Explanation:       b.Message,
			RecommendedAction: "Address this blocker before beginning a sale process.",
		})
	}
	for _, r := range in.SaleReadiness.Risks {
		findings = append(findings, Finding{
			Code:              FindingSaleReadinessRisk,
			Category:          categoryForDimensionCode(string(r.Dimension)),
			Severity:          Severity(r.Severity),
			Title:             "Sale-readiness risk",
			Evidence:          []string{r.Message},
			SourceModule:      SourceSaleReadiness,
			SourceCode:        string(r.Dimension),
			Explanation:       r.Message,
			RecommendedAction: "Review this risk ahead of any future sale process.",
		})
	}
	for _, s := range in.SaleReadiness.Strengths {
		strengths = append(strengths, Finding{
			Code:              FindingSaleReadinessStrength,
			Category:          categoryForDimensionCode(string(s.Dimension)),
			Severity:          SeverityInfo,
			Title:             "Sale-readiness strength",
			Evidence:          []string{s.Message},
			SourceModule:      SourceSaleReadiness,
			SourceCode:        string(s.Dimension),
			Explanation:       s.Message,
			RecommendedAction: "No action needed; maintain current practice.",
		})
	}
	for _, o := range in.SaleReadiness.Opportunities {
		opportunities = append(opportunities, Finding{
			Code:              FindingSaleReadinessOpportunity,
			Category:          categoryForDimensionCode(string(o.Dimension)),
			Severity:          SeverityInfo,
			Title:             "Sale-readiness opportunity",
			Evidence:          []string{o.Message},
			SourceModule:      SourceSaleReadiness,
			SourceCode:        string(o.Code),
			Explanation:       o.Message,
			RecommendedAction: o.Message,
		})
	}

	return findings, strengths, opportunities
}
