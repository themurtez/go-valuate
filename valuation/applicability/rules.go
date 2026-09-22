package applicability

import (
	"github.com/themurtez/go-valuate/valuation"
	"github.com/themurtez/go-valuate/valuation/profile"
)

// baseScore is every method's starting Score before any rule adjusts it:
// a neutral MEDIUM-range starting point (see levelForScore), so a
// completely empty Profile — no information supplied at all — lands every
// method at LevelMedium rather than defaulting to HIGH (overclaiming
// fitness with no evidence) or LOW/NOT_APPLICABLE (penalizing the caller
// for not having supplied a Profile yet). Every rule below moves a method
// up or down from this shared starting point.
const baseScore = 50

// Score-adjustment sizes, fixed and documented once here so every rule in
// this file draws from the same small vocabulary of point deltas rather
// than inventing ad hoc numbers per rule. Larger magnitudes represent a
// stronger structural signal (e.g. DCF has no forecast at all, a hard
// blocking fact) than a softer one (e.g. moderate asset intensity, a
// matter of degree).
const (
	pointsStrongPositive = 25
	pointsPositive       = 15
	pointsSlightPositive = 8
	pointsSlightNegative = -8
	pointsNegative       = -15
	pointsStrongNegative = -25
	pointsBlocking       = -100 // forces NOT_APPLICABLE regardless of other rules
)

// levelForScore maps a clamped [0,100] Score to a Level via fixed
// thresholds. This is the single source of truth for the Score->Level
// mapping; nothing else in this package hardcodes these numbers.
//
//	80-100 -> HIGH
//	50-79  -> MEDIUM
//	1-49   -> LOW
//	0      -> NOT_APPLICABLE
func levelForScore(score int) Level {
	switch {
	case score >= 80:
		return LevelHigh
	case score >= 50:
		return LevelMedium
	case score >= 1:
		return LevelLow
	default:
		return LevelNotApplicable
	}
}

// clampScore restricts a running point total to [0, 100] so a long chain
// of negative or positive reasons cannot drive Score outside the scale
// levelForScore interprets.
func clampScore(v int) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

// finalize converts an accumulated (method, score, reasons, warnings)
// tuple into a Result: clamps Score, derives Level, and sets Recommended.
func finalize(method valuation.Code, score int, reasons []Reason, warnings []string) Result {
	clamped := clampScore(score)
	level := levelForScore(clamped)
	return Result{
		Method:      method,
		Score:       clamped,
		Level:       level,
		Recommended: level == LevelHigh || level == LevelMedium,
		Reasons:     reasons,
		Warnings:    warnings,
	}
}

// scoreSDE rules — Seller's Discretionary Earnings multiple.
//
// SDE prices a business for a single owner-operator buyer (see
// valuation/sde's package doc comment): it is the best-fit method exactly
// when the business is small, owner-operated, and service/trade-like, and
// a poor fit for a business too large for an individual owner-operator
// buyer pool or one with no owner-operator earnings component to speak of.
func scoreSDE(p profile.Profile) Result {
	score := baseScore
	var reasons []Reason
	var warnings []string

	add := func(kind ReasonKind, detail string, points int) {
		reasons = append(reasons, Reason{Kind: kind, Detail: detail, Points: points})
		score += points
	}

	if p.OwnerOperated != nil {
		if *p.OwnerOperated {
			add(ReasonFit, "owner-operated business: SDE captures the full owner-operator return this method is designed to price", pointsStrongPositive)
		} else {
			add(ReasonFit, "not owner-operated: SDE's owner-operator-return premise does not apply", pointsStrongNegative)
		}
	} else {
		add(ReasonDataGap, "owner-operated status not supplied; SDE applicability cannot be confidently assessed", pointsSlightNegative)
	}

	if p.AnnualRevenue != nil {
		switch {
		case *p.AnnualRevenue <= 2_000_000:
			add(ReasonFit, "revenue is in the small main-street range SDE is conventionally used for", pointsPositive)
		case *p.AnnualRevenue <= 5_000_000:
			add(ReasonFit, "revenue is at the upper end of SDE's conventional main-street range", pointsSlightPositive)
		default:
			add(ReasonFit, "revenue exceeds the main-street range a single owner-operator buyer typically prices", pointsNegative)
		}
	}

	if p.EmployeeCount != nil {
		switch {
		case *p.EmployeeCount <= 10:
			add(ReasonFit, "small headcount consistent with an owner-operator-managed business", pointsSlightPositive)
		case *p.EmployeeCount > 50:
			add(ReasonFit, "headcount implies a management layer beyond a single owner-operator", pointsNegative)
		}
	}

	if p.AssetIntensity != nil && *p.AssetIntensity >= 1.0 {
		add(ReasonFit, "high asset intensity: an asset-heavy business is typically better served by an asset- or EBITDA-based method", pointsSlightNegative)
	}

	if p.Profitability == profile.ProfitabilityUnprofitable {
		add(ReasonFit, "unprofitable: a pure multiple method will price the business at or below zero (see valuation/sde's zero/negative-SDE handling), which is mathematically valid but rarely the most informative method here", pointsNegative)
	}

	return finalize(valuation.CodeSDEMultiple, score, reasons, warnings)
}

// scoreEBITDA rules — EBITDA multiple.
//
// EBITDA multiples price the whole enterprise on a cash-free, debt-free
// basis (see valuation/ebitda's package doc comment): best fit for a
// business large enough to be run by hired management (so EBITDA is not
// dominated by an unstated owner-compensation add-back the way SDE is),
// and a common secondary fit even for a smaller owner-operated business.
func scoreEBITDA(p profile.Profile) Result {
	score := baseScore
	var reasons []Reason
	var warnings []string

	add := func(kind ReasonKind, detail string, points int) {
		reasons = append(reasons, Reason{Kind: kind, Detail: detail, Points: points})
		score += points
	}

	if p.OwnerOperated != nil {
		if *p.OwnerOperated {
			add(ReasonFit, "owner-operated: EBITDA does not add back owner compensation, so it may understate a transferable earnings base unless paired with an SDE view", pointsSlightNegative)
		} else {
			add(ReasonFit, "not owner-operated: EBITDA cleanly represents earnings available to a professional-management buyer", pointsStrongPositive)
		}
	}

	if p.AnnualRevenue != nil {
		switch {
		case *p.AnnualRevenue >= 5_000_000:
			add(ReasonFit, "revenue is large enough that an EBITDA-multiple, enterprise-value framing is conventional", pointsPositive)
		case *p.AnnualRevenue < 1_000_000:
			add(ReasonFit, "revenue is small enough that EBITDA may be dominated by unaddressed owner-related items better handled via SDE", pointsSlightNegative)
		}
	}

	if p.EmployeeCount != nil && *p.EmployeeCount > 10 {
		add(ReasonFit, "headcount implies a management bench beyond a single owner-operator", pointsSlightPositive)
	}

	if p.AssetIntensity != nil {
		switch {
		case *p.AssetIntensity >= 0.75:
			add(ReasonFit, "asset-heavy operations are well represented by an enterprise-value method like EBITDA", pointsPositive)
		case *p.AssetIntensity < 0.15:
			add(ReasonFit, "very low asset intensity is unusual for an EBITDA-multiple comparable set, though not disqualifying", pointsSlightNegative)
		}
	}

	if p.Profitability == profile.ProfitabilityUnprofitable {
		add(ReasonFit, "unprofitable: a pure multiple method will price the business at or below zero, which is mathematically valid but rarely the most informative method here", pointsNegative)
	}

	return finalize(valuation.CodeEBITDAMultiple, score, reasons, warnings)
}

// scoreCapitalization rules — capitalization of earnings.
//
// Capitalization of earnings is a single-period method: it is a poor fit
// for a business whose earnings are volatile or growing/declining quickly
// (a single period cannot represent a moving target), and a good fit for
// a business with stable, mature earnings — regardless of owner-operated
// status, since valuation/capitalization deliberately does not assume
// which earnings base (SDE-like or EBIT-like) the caller supplied.
func scoreCapitalization(p profile.Profile) Result {
	score := baseScore
	var reasons []Reason
	var warnings []string

	add := func(kind ReasonKind, detail string, points int) {
		reasons = append(reasons, Reason{Kind: kind, Detail: detail, Points: points})
		score += points
	}

	switch p.EarningsStability {
	case profile.EarningsStabilityStable:
		add(ReasonFit, "stable historical earnings suit a single-period capitalization", pointsStrongPositive)
	case profile.EarningsStabilityVariable:
		add(ReasonFit, "variable earnings make a single-period capitalization less representative", pointsNegative)
	case profile.EarningsStabilityVolatile:
		add(ReasonFit, "volatile earnings are a poor fit for a single-period capitalization", pointsStrongNegative)
	case profile.EarningsStabilityDeclining:
		add(ReasonFit, "declining earnings are a poor fit for a single-period capitalization, which assumes a representative steady state", pointsStrongNegative)
	default:
		add(ReasonDataGap, "earnings stability not supplied; capitalization applicability cannot be confidently assessed", pointsSlightNegative)
	}

	if p.HistoricalGrowthRate != nil {
		switch {
		case *p.HistoricalGrowthRate > 0.15:
			add(ReasonFit, "high historical growth rate is poorly represented by a flat-capitalization method; a DCF better captures a growth trajectory", pointsNegative)
		case *p.HistoricalGrowthRate < -0.05:
			add(ReasonFit, "declining historical growth is poorly represented by a flat-capitalization method", pointsNegative)
		default:
			add(ReasonFit, "modest/stable historical growth is consistent with a single-period capitalization", pointsSlightPositive)
		}
	}

	if p.Profitability == profile.ProfitabilityUnprofitable {
		add(ReasonFit, "unprofitable: capitalizing zero/negative earnings produces a zero-or-below value, which is mathematically valid but rarely the most informative method here", pointsNegative)
	}

	return finalize(valuation.CodeCapitalizationOfEarnings, score, reasons, warnings)
}

// scoreDCF rules — discounted cash flow.
//
// valuation/dcf never generates a forecast itself (see its package doc
// comment): DCF is only meaningfully applicable when the caller has (or
// intends to supply) an explicit multi-period forecast. Absent that, DCF
// is scored NOT_APPLICABLE outright — a hard data-availability block, not
// a matter of degree — regardless of how well the business's growth
// profile would otherwise suit a DCF. Growth companies with weak current
// earnings are exactly the case a DCF is meant for, but revenue-multiple
// valuation is explicitly out of scope for this repository (see the
// package brief) and is never invented here as a substitute.
func scoreDCF(p profile.Profile) Result {
	if !p.DataAvailability.HasForecast {
		return finalize(valuation.CodeDCF, 0, []Reason{
			{Kind: ReasonDataGap, Detail: "no explicit forecast cash flows available; valuation/dcf never generates a forecast, so DCF cannot run at all for this business", Points: pointsBlocking},
		}, []string{"supply an explicit multi-period free cash flow forecast (valuation/dcf.Input.ForecastPeriods) to make DCF applicable"})
	}

	score := baseScore
	var reasons []Reason
	var warnings []string

	add := func(kind ReasonKind, detail string, points int) {
		reasons = append(reasons, Reason{Kind: kind, Detail: detail, Points: points})
		score += points
	}

	add(ReasonFit, "explicit forecast cash flows are available", pointsStrongPositive)

	if p.RecurringRevenuePercent != nil && *p.RecurringRevenuePercent >= 0.5 {
		add(ReasonFit, "majority recurring revenue supports a more predictable multi-period forecast", pointsPositive)
	}

	if p.HistoricalGrowthRate != nil && *p.HistoricalGrowthRate > 0.15 {
		add(ReasonFit, "high growth trajectory is well represented by an explicit multi-period forecast rather than a single-period multiple", pointsPositive)
	}

	switch p.EarningsStability {
	case profile.EarningsStabilityVolatile:
		add(ReasonFit, "volatile historical earnings add uncertainty to any forecast built on them, though DCF's explicit period-by-period structure can still represent a recovery/growth path better than a single-period method", pointsSlightNegative)
	case profile.EarningsStabilityStable:
		add(ReasonFit, "stable historical earnings support confidence in a supplied forecast", pointsSlightPositive)
	}

	if !p.DataAvailability.HasMultiYearFinancials {
		add(ReasonDataGap, "no multi-year financial history to ground the supplied forecast's assumptions against", pointsSlightNegative)
	}

	return finalize(valuation.CodeDCF, score, reasons, warnings)
}

// scoreNetAssets rules — adjusted net asset value.
//
// Adjusted net asset value is a controlling/liquidation-oriented reference
// value (see valuation/netassets' package doc comment), not a going-
// concern earnings-based value: it is the best fit for an asset-heavy
// business (manufacturing, real estate-holding, equipment-heavy
// operations) and a weak fit for an asset-light, earnings-driven business
// where the balance sheet captures little of the business's real value.
func scoreNetAssets(p profile.Profile) Result {
	score := baseScore
	var reasons []Reason
	var warnings []string

	add := func(kind ReasonKind, detail string, points int) {
		reasons = append(reasons, Reason{Kind: kind, Detail: detail, Points: points})
		score += points
	}

	if !p.DataAvailability.HasBalanceSheet {
		add(ReasonDataGap, "no balance-sheet data available; adjusted net asset value cannot be confidently assessed without it", pointsStrongNegative)
	}

	if p.AssetIntensity != nil {
		switch {
		case *p.AssetIntensity >= 1.0:
			add(ReasonFit, "high asset intensity: the balance sheet captures a large share of the business's value", pointsStrongPositive)
		case *p.AssetIntensity >= 0.5:
			add(ReasonFit, "moderate asset intensity supports a net asset value as a useful reference point alongside an earnings-based method", pointsPositive)
		case *p.AssetIntensity < 0.15:
			add(ReasonFit, "very low asset intensity: the balance sheet likely captures little of this business's real (earnings-driven) value", pointsStrongNegative)
		}
	} else {
		add(ReasonDataGap, "asset intensity not supplied; net asset value applicability cannot be confidently assessed", pointsSlightNegative)
	}

	if p.DataAvailability.HasAppraisedAssetValues {
		add(ReasonFit, "independent appraisal/fair-value overrides are available, strengthening confidence in an adjusted (not merely book) net asset value", pointsPositive)
	}

	if p.Profitability == profile.ProfitabilityUnprofitable || p.Profitability == profile.ProfitabilityMarginal {
		add(ReasonFit, "weak current profitability makes an asset-based reference value especially relevant alongside (or instead of) an earnings-based method", pointsSlightPositive)
	}

	return finalize(valuation.CodeAdjustedNetAssetValue, score, reasons, warnings)
}
