package diagnostics

import "fmt"

// percentChange computes (current - prior) / |prior|, when both are
// Available and prior.Amount != 0. Returns (Unavailable(), false)
// otherwise — the shared building block for both changeDirection (which
// additionally classifies the change against a metric's polarity) and
// detectValuationMovement (which cares only about magnitude, not
// polarity — see that function's doc comment).
func percentChange(current, prior Value) (change Value, ok bool) {
	if !current.Available || !prior.Available || prior.Amount == 0 {
		return Unavailable(), false
	}
	return AvailableValue((current.Amount - prior.Amount) / absFloat(prior.Amount)), true
}

// changeDirection computes percentChange(current, prior) and classifies it
// against policy under the convention "not DirectionStable when magnitude
// exceeds policy.MaterialChangePercent." When either value is unavailable
// or prior.Amount is exactly 0, it returns (Unavailable(), DirectionUnavailable).
//
// worseningOnIncrease is the polarity of the metric being compared: true
// when a rise in the raw value is the bad direction (e.g. leverage,
// concentration), false when a rise is the good direction (e.g. margin,
// revenue, cash conversion) — callers pass whichever polarity fits the
// metric, mirroring how every sibling package's own increasing/declining
// trend classification is metric-specific.
func changeDirection(current, prior Value, policy Policy, worseningOnIncrease bool) (change Value, dir Direction) {
	change, ok := percentChange(current, prior)
	if !ok {
		return Unavailable(), DirectionUnavailable
	}

	if absFloat(change.Amount) < policy.MaterialChangePercent {
		return change, DirectionStable
	}
	increased := change.Amount > 0
	if increased == worseningOnIncrease {
		return change, DirectionDeteriorating
	}
	return change, DirectionImproving
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// severityForDirection maps a Direction to a Finding Severity for a
// change-based finding: DirectionDeteriorating with a large enough
// magnitude escalates to SeverityCritical (2x policy's material-change
// threshold), otherwise SeverityWarning. Fixed, part of FormulaVersion.
func severityForDirection(change Value, policy Policy) Severity {
	if change.Available && absFloat(change.Amount) >= 2*policy.MaterialChangePercent {
		return SeverityCritical
	}
	return SeverityWarning
}

// detectMarginDeterioration implements FindingMarginDeterioration: fires
// when EBITDAMargin (RatioHealth, falling back to Metrics) declined by at
// least policy.MaterialChangePercent versus Prior, or RatioHealth carries
// an active margin-compression/deteriorating-profitability signal (fires
// even without a usable Prior comparison in that case, at SeverityWarning
// unless a critical ratio signal count is also present).
func detectMarginDeterioration(b BusinessSnapshot, policy Policy) []Finding {
	var findings []Finding

	current := b.RatioHealth.EBITDAMargin
	if !current.Available {
		current = b.Metrics.EBITDAMargin
	}

	if b.Prior != nil {
		prior := b.Prior.RatioHealth.EBITDAMargin
		if !prior.Available {
			prior = b.Prior.Metrics.EBITDAMargin
		}
		change, dir := changeDirection(current, prior, policy, false)
		if dir == DirectionDeteriorating {
			findings = append(findings, Finding{
				Code:                    FindingMarginDeterioration,
				Severity:                severityForDirection(change, policy),
				Metric:                  "EBITDA_MARGIN",
				CurrentValue:            current,
				PriorValue:              prior,
				Change:                  change,
				Reason:                  fmt.Sprintf("EBITDA margin moved from %.1f%% to %.1f%%, a decline of %.1f%%", prior.Amount*100, current.Amount*100, change.Amount*100),
				SuggestedReviewCategory: ReviewCategoryProfitability,
			})
			return findings
		}
	}

	if b.RatioHealth.Available && b.RatioHealth.HasMarginCompressionSignal {
		findings = append(findings, Finding{
			Code:                    FindingMarginDeterioration,
			Severity:                SeverityWarning,
			Metric:                  "EBITDA_MARGIN",
			CurrentValue:            current,
			PriorValue:              Unavailable(),
			Change:                  Unavailable(),
			Reason:                  "ratio health signals active margin compression or deteriorating profitability for the most recent period",
			SuggestedReviewCategory: ReviewCategoryProfitability,
		})
	}

	return findings
}

// detectRevenueDecline implements FindingRevenueDecline: fires when
// Metrics.Revenue declined by at least policy.MaterialChangePercent versus
// Prior. Purely change-based; no fixed-threshold single-period variant,
// since "revenue decline" has no meaning without a comparison point.
func detectRevenueDecline(b BusinessSnapshot, policy Policy) []Finding {
	if b.Prior == nil {
		return nil
	}
	current := b.Metrics.Revenue
	prior := b.Prior.Metrics.Revenue
	change, dir := changeDirection(current, prior, policy, false)
	if dir != DirectionDeteriorating {
		return nil
	}
	return []Finding{{
		Code:                    FindingRevenueDecline,
		Severity:                severityForDirection(change, policy),
		Metric:                  "REVENUE",
		CurrentValue:            current,
		PriorValue:              prior,
		Change:                  change,
		Reason:                  fmt.Sprintf("revenue declined %.1f%% versus the prior period", -change.Amount*100),
		SuggestedReviewCategory: ReviewCategoryProfitability,
	}}
}

// detectCashConversionWeakening implements FindingCashConversionWeakening:
// fires when CashFlow.ConversionTrend is DirectionDeteriorating,
// EBITDAToFreeCashFlow declined by at least policy.MaterialChangePercent
// versus Prior, or CashFlow carries an active declining-conversion flag.
func detectCashConversionWeakening(b BusinessSnapshot, policy Policy) []Finding {
	if b.Prior != nil {
		current := b.CashFlow.EBITDAToFreeCashFlow
		prior := b.Prior.CashFlow.EBITDAToFreeCashFlow
		change, dir := changeDirection(current, prior, policy, false)
		if dir == DirectionDeteriorating {
			return []Finding{{
				Code:                    FindingCashConversionWeakening,
				Severity:                severityForDirection(change, policy),
				Metric:                  "EBITDA_TO_FREE_CASH_FLOW",
				CurrentValue:            current,
				PriorValue:              prior,
				Change:                  change,
				Reason:                  fmt.Sprintf("EBITDA-to-free-cash-flow conversion declined %.1f%% versus the prior period", -change.Amount*100),
				SuggestedReviewCategory: ReviewCategoryCashFlow,
			}}
		}
	}

	if b.CashFlow.Available && (b.CashFlow.ConversionTrend == DirectionDeteriorating || b.CashFlow.HasDecliningConversionFlag) {
		return []Finding{{
			Code:                    FindingCashConversionWeakening,
			Severity:                SeverityWarning,
			Metric:                  "EBITDA_TO_FREE_CASH_FLOW",
			CurrentValue:            b.CashFlow.EBITDAToFreeCashFlow,
			PriorValue:              Unavailable(),
			Change:                  Unavailable(),
			Reason:                  "cash flow analysis shows a declining EBITDA-to-cash conversion trend",
			SuggestedReviewCategory: ReviewCategoryCashFlow,
		}}
	}

	return nil
}

// detectLeverageIncrease implements FindingLeverageIncrease: fires when
// RatioHealth.NetDebtToEBITDA increased by at least
// policy.MaterialChangePercent versus Prior, or RatioHealth carries an
// active rising-leverage signal.
func detectLeverageIncrease(b BusinessSnapshot, policy Policy) []Finding {
	if b.Prior != nil {
		current := b.RatioHealth.NetDebtToEBITDA
		prior := b.Prior.RatioHealth.NetDebtToEBITDA
		change, dir := changeDirection(current, prior, policy, true)
		if dir == DirectionDeteriorating {
			return []Finding{{
				Code:                    FindingLeverageIncrease,
				Severity:                severityForDirection(change, policy),
				Metric:                  "NET_DEBT_TO_EBITDA",
				CurrentValue:            current,
				PriorValue:              prior,
				Change:                  change,
				Reason:                  fmt.Sprintf("net debt / EBITDA rose from %.2fx to %.2fx", prior.Amount, current.Amount),
				SuggestedReviewCategory: ReviewCategoryRisk,
			}}
		}
	}

	if b.RatioHealth.Available && b.RatioHealth.HasRisingLeverageSignal {
		return []Finding{{
			Code:                    FindingLeverageIncrease,
			Severity:                SeverityWarning,
			Metric:                  "NET_DEBT_TO_EBITDA",
			CurrentValue:            b.RatioHealth.NetDebtToEBITDA,
			PriorValue:              Unavailable(),
			Change:                  Unavailable(),
			Reason:                  "ratio health signals rising leverage for the most recent period",
			SuggestedReviewCategory: ReviewCategoryRisk,
		}}
	}

	return nil
}

// detectConcentrationIncrease implements FindingConcentrationIncrease:
// fires when Concentration.LargestEntityShare increased by at least
// policy.MaterialChangePercent versus Prior, or
// Concentration.LargestShareTrend is DirectionDeteriorating.
func detectConcentrationIncrease(b BusinessSnapshot, policy Policy) []Finding {
	if b.Prior != nil {
		current := b.Concentration.LargestEntityShare
		prior := b.Prior.Concentration.LargestEntityShare
		change, dir := changeDirection(current, prior, policy, true)
		if dir == DirectionDeteriorating {
			return []Finding{{
				Code:                    FindingConcentrationIncrease,
				Severity:                severityForDirection(change, policy),
				Metric:                  "LARGEST_ENTITY_SHARE",
				CurrentValue:            current,
				PriorValue:              prior,
				Change:                  change,
				Reason:                  fmt.Sprintf("largest customer/counterparty share rose from %.1f%% to %.1f%%", prior.Amount*100, current.Amount*100),
				SuggestedReviewCategory: ReviewCategoryRisk,
			}}
		}
	}

	if b.Concentration.Available && b.Concentration.LargestShareTrend == DirectionDeteriorating {
		return []Finding{{
			Code:                    FindingConcentrationIncrease,
			Severity:                SeverityWarning,
			Metric:                  "LARGEST_ENTITY_SHARE",
			CurrentValue:            b.Concentration.LargestEntityShare,
			PriorValue:              Unavailable(),
			Change:                  Unavailable(),
			Reason:                  "concentration analysis shows an increasing largest-entity-share trend",
			SuggestedReviewCategory: ReviewCategoryRisk,
		}}
	}

	return nil
}

// detectValuationMovement implements FindingValuationMovement: fires when
// Valuation.IndicatedValue changed (either direction) by at least
// policy.MaterialChangePercent versus Prior — the one finding that reports
// a rise as well as a decline, since a large swing either way is itself
// the signal. Reuses percentChange/severityForDirection for the same
// magnitude/materiality/severity math every other change-based detect*
// function uses; only the "which direction counts as worsening" step is
// skipped, since both directions are equally reportable here.
func detectValuationMovement(b BusinessSnapshot, policy Policy) []Finding {
	if b.Prior == nil {
		return nil
	}
	change, ok := percentChange(b.Valuation.IndicatedValue, b.Prior.Valuation.IndicatedValue)
	if !ok || absFloat(change.Amount) < policy.MaterialChangePercent {
		return nil
	}

	direction := "declined"
	if change.Amount > 0 {
		direction = "increased"
	}

	return []Finding{{
		Code:                    FindingValuationMovement,
		Severity:                severityForDirection(change, policy),
		Metric:                  "INDICATED_VALUE",
		CurrentValue:            b.Valuation.IndicatedValue,
		PriorValue:              b.Prior.Valuation.IndicatedValue,
		Change:                  change,
		Reason:                  fmt.Sprintf("indicated value %s %.1f%% versus the prior valuation", direction, absFloat(change.Amount)*100),
		SuggestedReviewCategory: ReviewCategoryValuation,
	}}
}

// unresolvedAdjustmentBurdenThreshold is the fixed AdjustmentToEBITDA
// decimal value at or above which FindingUnresolvedFinancialQuality fires
// from QoE alone (independent of QoE.HasCriticalFlags) — part of
// FormulaVersion.
const unresolvedAdjustmentBurdenThreshold = 0.5

// detectUnresolvedFinancialQuality implements
// FindingUnresolvedFinancialQuality: a single-period reading (fires with no
// Prior) when QoE.HasCriticalFlags is true, QoE.AdjustmentToEBITDA is at or
// above unresolvedAdjustmentBurdenThreshold, or
// RatioHealth.CriticalSignalCount > 0.
func detectUnresolvedFinancialQuality(b BusinessSnapshot) []Finding {
	var reasons []string
	severity := SeverityWarning

	if b.QoE.Available && b.QoE.HasCriticalFlags {
		reasons = append(reasons, "quality-of-earnings analysis has open critical flags")
		severity = SeverityCritical
	}
	if b.QoE.Available && b.QoE.AdjustmentToEBITDA.Available && b.QoE.AdjustmentToEBITDA.Amount >= unresolvedAdjustmentBurdenThreshold {
		reasons = append(reasons, fmt.Sprintf("adjustments total %.0f%% of reported EBITDA", b.QoE.AdjustmentToEBITDA.Amount*100))
	}
	if b.RatioHealth.Available && b.RatioHealth.CriticalSignalCount > 0 {
		reasons = append(reasons, fmt.Sprintf("ratio health has %d critical signal(s) open", b.RatioHealth.CriticalSignalCount))
		severity = SeverityCritical
	}

	if len(reasons) == 0 {
		return nil
	}

	reason := reasons[0]
	for _, r := range reasons[1:] {
		reason += "; " + r
	}

	return []Finding{{
		Code:                    FindingUnresolvedFinancialQuality,
		Severity:                severity,
		Metric:                  "ADJUSTMENT_TO_EBITDA",
		CurrentValue:            b.QoE.AdjustmentToEBITDA,
		PriorValue:              Unavailable(),
		Change:                  Unavailable(),
		Reason:                  reason,
		SuggestedReviewCategory: ReviewCategoryFinancialQuality,
	}}
}

// saleReadinessOpportunityScoreThreshold is the fixed
// SaleReadiness.OverallScore decimal value below which
// FindingSaleReadinessOpportunity fires from score alone (independent of
// BlockerCount) — part of FormulaVersion.
const saleReadinessOpportunityScoreThreshold = 70.0

// detectSaleReadinessOpportunity implements
// FindingSaleReadinessOpportunity: a single-period reading (fires with no
// Prior) when SaleReadiness.BlockerCount > 0 or
// SaleReadiness.OverallScore is below saleReadinessOpportunityScoreThreshold.
// Framed as a factual observation only — never a recommendation to pursue a
// sale, consistent with the package doc comment's no-sales-pitch rule.
func detectSaleReadinessOpportunity(b BusinessSnapshot) []Finding {
	if !b.SaleReadiness.Available {
		return nil
	}

	if b.SaleReadiness.BlockerCount > 0 {
		return []Finding{{
			Code:                    FindingSaleReadinessOpportunity,
			Severity:                SeverityWarning,
			Metric:                  "SALE_READINESS_BLOCKER_COUNT",
			CurrentValue:            AvailableValue(float64(b.SaleReadiness.BlockerCount)),
			PriorValue:              Unavailable(),
			Change:                  Unavailable(),
			Reason:                  fmt.Sprintf("sale-readiness assessment has %d unresolved blocker(s)", b.SaleReadiness.BlockerCount),
			SuggestedReviewCategory: ReviewCategoryTransactionReadiness,
		}}
	}

	if b.SaleReadiness.OverallScore.Available && b.SaleReadiness.OverallScore.Amount < saleReadinessOpportunityScoreThreshold {
		return []Finding{{
			Code:                    FindingSaleReadinessOpportunity,
			Severity:                SeverityInfo,
			Metric:                  "SALE_READINESS_OVERALL_SCORE",
			CurrentValue:            b.SaleReadiness.OverallScore,
			PriorValue:              Unavailable(),
			Change:                  Unavailable(),
			Reason:                  fmt.Sprintf("sale-readiness overall score of %.0f is below %.0f", b.SaleReadiness.OverallScore.Amount, saleReadinessOpportunityScoreThreshold),
			SuggestedReviewCategory: ReviewCategoryTransactionReadiness,
		}}
	}

	return nil
}

// detectFindings runs every detect* rule for one business and returns every
// Finding raised, unordered (buildResult sorts the full cross-portfolio
// list) with BusinessID/BusinessLabel already stamped.
func detectFindings(b BusinessSnapshot, policy Policy) []Finding {
	var findings []Finding
	findings = append(findings, detectMarginDeterioration(b, policy)...)
	findings = append(findings, detectRevenueDecline(b, policy)...)
	findings = append(findings, detectCashConversionWeakening(b, policy)...)
	findings = append(findings, detectLeverageIncrease(b, policy)...)
	findings = append(findings, detectConcentrationIncrease(b, policy)...)
	findings = append(findings, detectValuationMovement(b, policy)...)
	findings = append(findings, detectUnresolvedFinancialQuality(b)...)
	findings = append(findings, detectSaleReadinessOpportunity(b)...)

	for i := range findings {
		findings[i].BusinessID = b.ID
		findings[i].BusinessLabel = b.Label
	}
	return findings
}
