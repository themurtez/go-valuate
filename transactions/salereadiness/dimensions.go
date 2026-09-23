package salereadiness

import (
	"fmt"

	"github.com/themurtez/go-valuate/analytics/qoe"
)

// unassessed builds the fixed StatusUnassessed Dimension for code, with
// reason as Explanation — the shared shape every classify* function
// returns when its required input(s) are absent.
func unassessed(code DimensionCode, reason string) Dimension {
	return Dimension{Code: code, Status: StatusUnassessed, Explanation: reason}
}

// classifyFinancialRecordQuality assesses record quality from
// Input.DataQuality plus Input.Dataset's period count. Fixed rule (no
// Policy threshold beyond MinYearsHistoryForStrong): starts from
// StatusAcceptable once at least DataQuality is non-zero or Dataset has
// periods; StatusStrong requires reviewed/audited financials AND tax-
// return reconciliation AND (when Policy.MinYearsHistoryForStrong is set)
// at least that many periods; StatusConcerning when
// OpenAccountingIssueCount is confirmed > 0; StatusWeak otherwise when
// neither reviewed/audited nor tax-reconciled.
func classifyFinancialRecordQuality(in Input) Dimension {
	years := len(in.Dataset.Periods())
	hasSignal := in.DataQuality != (DataQuality{}) || years > 0
	if !hasSignal {
		return unassessed(DimensionFinancialRecordQuality,
			"no DataQuality indicators and no Dataset periods supplied")
	}

	dq := in.DataQuality
	if dq.OpenAccountingIssueCount != nil && *dq.OpenAccountingIssueCount > 0 {
		return Dimension{
			Code:        DimensionFinancialRecordQuality,
			Status:      StatusConcerning,
			Value:       AvailableValue(float64(*dq.OpenAccountingIssueCount)),
			Explanation: fmt.Sprintf("%d open accounting issue(s) reported", *dq.OpenAccountingIssueCount),
			Source:      "data_quality",
		}
	}

	strong := dq.HasReviewedOrAuditedFinancials && dq.HasTaxReturnReconciliation
	if strong && in.Policy.MinYearsHistoryForStrong > 0 && years < in.Policy.MinYearsHistoryForStrong {
		strong = false
	}
	if strong {
		return Dimension{
			Code:        DimensionFinancialRecordQuality,
			Status:      StatusStrong,
			Explanation: "financials are reviewed/audited and reconciled to filed tax returns",
			Source:      "data_quality",
		}
	}

	if dq.HasReviewedOrAuditedFinancials || dq.HasTaxReturnReconciliation || dq.HasMultiYearFinancials {
		return Dimension{
			Code:        DimensionFinancialRecordQuality,
			Status:      StatusAcceptable,
			Explanation: "some record-quality indicators confirmed, but not the full reviewed/audited-and-reconciled bar",
			Source:      "data_quality",
		}
	}

	return Dimension{
		Code:        DimensionFinancialRecordQuality,
		Status:      StatusWeak,
		Explanation: "no reviewed/audited financials or tax-return reconciliation confirmed",
		Source:      "data_quality",
	}
}

// classifyEarningsStability assesses earnings volatility from
// Input.QoE.EBITDAVolatility, falling back to
// Input.Metrics.Trend.EBITDAVolatility when QoE is unavailable. Requires
// Policy.MaxAcceptableEarningsVolatility to distinguish StatusStrong from
// StatusAcceptable/StatusWeak/StatusConcerning; without it, any available
// volatility figure still yields StatusAcceptable (assessed, but not
// threshold-graded).
func classifyEarningsStability(in Input) Dimension {
	vol, source, ok := resolveEBITDAVolatility(in)
	if !ok {
		return unassessed(DimensionEarningsStability,
			"neither QoE.EBITDAVolatility nor Metrics.Trend.EBITDAVolatility is available")
	}

	if in.Policy.MaxAcceptableEarningsVolatility <= 0 {
		return Dimension{
			Code:        DimensionEarningsStability,
			Status:      StatusAcceptable,
			Value:       AvailableValue(vol),
			Explanation: fmt.Sprintf("EBITDA volatility of %.1f%% (no Policy threshold supplied to grade it further)", vol*100),
			Source:      source,
		}
	}

	threshold := in.Policy.MaxAcceptableEarningsVolatility
	status := StatusAcceptable
	switch {
	case vol <= threshold/2:
		status = StatusStrong
	case vol <= threshold:
		status = StatusAcceptable
	case vol <= threshold*1.5:
		status = StatusWeak
	default:
		status = StatusConcerning
	}
	return Dimension{
		Code:        DimensionEarningsStability,
		Status:      status,
		Value:       AvailableValue(vol),
		Threshold:   threshold,
		Explanation: fmt.Sprintf("EBITDA volatility of %.1f%% against a %.1f%% threshold", vol*100, threshold*100),
		Source:      source,
	}
}

func resolveEBITDAVolatility(in Input) (value float64, source string, ok bool) {
	if in.QoE.Available && in.QoE.EBITDAVolatility.Value.Available {
		return in.QoE.EBITDAVolatility.Value.Value, "qoe", true
	}
	if in.Metrics.Trend != nil && in.Metrics.Trend.Error == nil && in.Metrics.Trend.EBITDAVolatility.Value.Available {
		return in.Metrics.Trend.EBITDAVolatility.Value.Value, "metrics", true
	}
	return 0, "", false
}

// classifyNormalizationBurden assesses how large the add-back burden is
// relative to reported EBITDA, from Input.QoE.Ratios.AdjustmentToEBITDA.
// Requires QoE; this package computes no adjustment ratio of its own.
func classifyNormalizationBurden(in Input) Dimension {
	if !in.QoE.Available || !in.QoE.Ratios.AdjustmentToEBITDA.Available {
		return unassessed(DimensionNormalizationBurden,
			"QoE.Ratios.AdjustmentToEBITDA is unavailable")
	}
	ratio := in.QoE.Ratios.AdjustmentToEBITDA.Value

	hasLargeBurdenFlag := hasQoEFlag(in.QoE, qoe.FlagLargeNormalizationBurden)

	if in.Policy.MaxAcceptableAdjustmentToEBITDARatio <= 0 {
		status := StatusAcceptable
		if hasLargeBurdenFlag {
			status = StatusWeak
		}
		return Dimension{
			Code:        DimensionNormalizationBurden,
			Status:      status,
			Value:       AvailableValue(ratio),
			Explanation: fmt.Sprintf("adjustment-to-EBITDA ratio of %.1f%% (no Policy threshold supplied to grade it further)", ratio*100),
			Source:      "qoe",
		}
	}

	threshold := in.Policy.MaxAcceptableAdjustmentToEBITDARatio
	status := StatusAcceptable
	switch {
	case ratio <= threshold/2:
		status = StatusStrong
	case ratio <= threshold:
		status = StatusAcceptable
	case ratio <= threshold*1.5:
		status = StatusWeak
	default:
		status = StatusConcerning
	}
	return Dimension{
		Code:        DimensionNormalizationBurden,
		Status:      status,
		Value:       AvailableValue(ratio),
		Threshold:   threshold,
		Explanation: fmt.Sprintf("adjustment-to-EBITDA ratio of %.1f%% against a %.1f%% threshold", ratio*100, threshold*100),
		Source:      "qoe",
	}
}

func hasQoEFlag(r qoe.Result, code qoe.FlagCode) bool {
	for _, f := range r.Flags {
		if f.Code == code {
			return true
		}
	}
	return false
}

// classifyCustomerConcentration assesses the most recent period's largest-
// customer share from Input.Concentration.
func classifyCustomerConcentration(in Input) Dimension {
	if !in.Concentration.Available || len(in.Concentration.History) == 0 {
		return unassessed(DimensionCustomerConcentration,
			"Concentration.History is unavailable or empty")
	}
	latest := in.Concentration.History[len(in.Concentration.History)-1]
	if !latest.LargestEntityShare.Available {
		return unassessed(DimensionCustomerConcentration,
			"most recent period's LargestEntityShare is unavailable")
	}
	share := latest.LargestEntityShare.Value

	if in.Policy.MaxAcceptableLargestCustomerShare <= 0 {
		return Dimension{
			Code:        DimensionCustomerConcentration,
			Status:      StatusAcceptable,
			Value:       AvailableValue(share),
			Explanation: fmt.Sprintf("largest customer is %.1f%% of revenue (no Policy threshold supplied to grade it further)", share*100),
			Source:      "concentration",
		}
	}

	threshold := in.Policy.MaxAcceptableLargestCustomerShare
	status := StatusAcceptable
	switch {
	case share <= threshold/2:
		status = StatusStrong
	case share <= threshold:
		status = StatusAcceptable
	case share <= threshold*1.5:
		status = StatusWeak
	default:
		status = StatusConcerning
	}
	return Dimension{
		Code:        DimensionCustomerConcentration,
		Status:      status,
		Value:       AvailableValue(share),
		Threshold:   threshold,
		Explanation: fmt.Sprintf("largest customer is %.1f%% of revenue against a %.1f%% threshold", share*100, threshold*100),
		Source:      "concentration",
	}
}

// classifyRecurringRevenue assesses recurring-revenue share from the most
// recent period in Input.RevenueQuality.TotalRevenueHistory, falling back
// to Input.Profile.RecurringRevenuePercent when RevenueQuality is
// unavailable.
func classifyRecurringRevenue(in Input) Dimension {
	pct, source, ok := resolveRecurringRevenuePercent(in)
	if !ok {
		return unassessed(DimensionRecurringRevenue,
			"neither RevenueQuality's most recent RecurringPercent nor Profile.RecurringRevenuePercent is available")
	}

	if in.Policy.MinAcceptableRecurringRevenuePercent <= 0 {
		return Dimension{
			Code:        DimensionRecurringRevenue,
			Status:      StatusAcceptable,
			Value:       AvailableValue(pct),
			Explanation: fmt.Sprintf("recurring revenue is %.1f%% of total (no Policy threshold supplied to grade it further)", pct*100),
			Source:      source,
		}
	}

	threshold := in.Policy.MinAcceptableRecurringRevenuePercent
	status := StatusAcceptable
	switch {
	case pct >= threshold*1.5:
		status = StatusStrong
	case pct >= threshold:
		status = StatusAcceptable
	case pct >= threshold*0.5:
		status = StatusWeak
	default:
		status = StatusConcerning
	}
	return Dimension{
		Code:        DimensionRecurringRevenue,
		Status:      status,
		Value:       AvailableValue(pct),
		Threshold:   threshold,
		Explanation: fmt.Sprintf("recurring revenue is %.1f%% of total against a %.1f%% threshold", pct*100, threshold*100),
		Source:      source,
	}
}

func resolveRecurringRevenuePercent(in Input) (value float64, source string, ok bool) {
	if in.RevenueQuality.Available && len(in.RevenueQuality.TotalRevenueHistory) > 0 {
		latest := in.RevenueQuality.TotalRevenueHistory[len(in.RevenueQuality.TotalRevenueHistory)-1]
		if latest.RecurringPercent.Available {
			return latest.RecurringPercent.Value, "revenue_quality", true
		}
	}
	if in.Profile.RecurringRevenuePercent != nil {
		return *in.Profile.RecurringRevenuePercent, "profile", true
	}
	return 0, "", false
}

// classifyOwnerDependence assesses reliance on the current owner from
// Input.Profile.OwnerOperated plus, when available, Input.QoE's
// FlagLargeOwnerDiscretionaryComponent signal. Fixed rule: StatusStrong
// requires an explicit OwnerOperated == false; owner-operated businesses
// are at best StatusAcceptable (a large discretionary component pushes to
// StatusWeak, since large owner-discretionary earnings compound the
// transition risk an owner-operated business already carries).
func classifyOwnerDependence(in Input) Dimension {
	if in.Profile.OwnerOperated == nil {
		return unassessed(DimensionOwnerDependence,
			"Profile.OwnerOperated is not supplied")
	}

	if !*in.Profile.OwnerOperated {
		return Dimension{
			Code:        DimensionOwnerDependence,
			Status:      StatusStrong,
			Explanation: "business is not owner-operated",
			Source:      "profile",
		}
	}

	largeDiscretionary := in.QoE.Available && hasQoEFlag(in.QoE, qoe.FlagLargeOwnerDiscretionaryComponent)
	if largeDiscretionary {
		return Dimension{
			Code:        DimensionOwnerDependence,
			Status:      StatusWeak,
			Explanation: "business is owner-operated with a large owner-discretionary earnings component",
			Source:      "profile+qoe",
		}
	}

	return Dimension{
		Code:        DimensionOwnerDependence,
		Status:      StatusAcceptable,
		Explanation: "business is owner-operated",
		Source:      "profile",
	}
}

// classifyMarginTrend assesses gross/EBITDA margin direction from
// Input.QoE.EBITDAMarginTrend when available, falling back to
// Input.Metrics.Trend.EBITDAMarginTrend. Compares the first vs. last
// available margin point, mirroring workingcapital.TrendDirection's
// first-vs-last convention with the same TrendFlatBandPercent-style 5%
// relative band (fixed, not Policy-configurable, since this is a coarse
// three-way direction call, not a threshold-graded figure).
const marginTrendFlatBandPercent = 0.05

func classifyMarginTrend(in Input) Dimension {
	points, source, ok := resolveEBITDAMarginTrend(in)
	if !ok || len(points) < 2 {
		return unassessed(DimensionMarginTrend,
			"fewer than two EBITDA margin trend points available from QoE or Metrics")
	}

	var first, last float64
	var haveFirst, haveLast bool
	for _, p := range points {
		if !p.Margin.Available {
			continue
		}
		if !haveFirst {
			first = p.Margin.Amount
			haveFirst = true
		}
		last = p.Margin.Amount
		haveLast = true
	}
	if !haveFirst || !haveLast {
		return unassessed(DimensionMarginTrend, "no available margin values in the trend series")
	}

	delta := last - first
	band := marginTrendFlatBandPercent
	var status Status
	var direction string
	switch {
	case first == 0:
		status = StatusAcceptable
		direction = "flat (base-period margin was zero)"
	case delta/absFloat(first) > band:
		status = StatusStrong
		direction = "improving"
	case delta/absFloat(first) < -band:
		status = StatusConcerning
		direction = "eroding"
	default:
		status = StatusAcceptable
		direction = "stable"
	}

	return Dimension{
		Code:        DimensionMarginTrend,
		Status:      status,
		Value:       AvailableValue(delta),
		Explanation: fmt.Sprintf("EBITDA margin is %s: %.1f%% to %.1f%%", direction, first*100, last*100),
		Source:      source,
	}
}

func resolveEBITDAMarginTrend(in Input) (points []marginPointLike, source string, ok bool) {
	if in.QoE.Available && len(in.QoE.EBITDAMarginTrend) > 0 {
		out := make([]marginPointLike, len(in.QoE.EBITDAMarginTrend))
		for i, p := range in.QoE.EBITDAMarginTrend {
			out[i] = marginPointLike{Margin: Value{Available: p.Margin.Available, Amount: p.Margin.Value}}
		}
		return out, "qoe", true
	}
	if in.Metrics.Trend != nil && in.Metrics.Trend.Error == nil && len(in.Metrics.Trend.EBITDAMarginTrend) > 0 {
		out := make([]marginPointLike, len(in.Metrics.Trend.EBITDAMarginTrend))
		for i, p := range in.Metrics.Trend.EBITDAMarginTrend {
			out[i] = marginPointLike{Margin: Value{Available: p.Margin.Available, Amount: p.Margin.Value}}
		}
		return out, "metrics", true
	}
	return nil, "", false
}

// marginPointLike is a source-agnostic view over qoe.MarginPoint/
// metrics.MarginPoint (identical shape, different packages), letting
// classifyMarginTrend compare either series with one code path.
type marginPointLike struct {
	Margin Value
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// classifyWorkingCapitalStability assesses NWC-as-percent-of-revenue
// volatility from Input.WorkingCapital.NWCPercentOfRevenueStatistics.
func classifyWorkingCapitalStability(in Input) Dimension {
	if !in.WorkingCapital.Available || !in.WorkingCapital.NWCPercentOfRevenueStatistics.Volatility.Available {
		return unassessed(DimensionWorkingCapitalStability,
			"WorkingCapital.NWCPercentOfRevenueStatistics.Volatility is unavailable")
	}
	vol := in.WorkingCapital.NWCPercentOfRevenueStatistics.Volatility.Value

	if in.Policy.MaxAcceptableNWCVolatility <= 0 {
		return Dimension{
			Code:        DimensionWorkingCapitalStability,
			Status:      StatusAcceptable,
			Value:       AvailableValue(vol),
			Explanation: fmt.Sprintf("NWC-as-%%-of-revenue volatility of %.1f%% (no Policy threshold supplied to grade it further)", vol*100),
			Source:      "working_capital",
		}
	}

	threshold := in.Policy.MaxAcceptableNWCVolatility
	status := StatusAcceptable
	switch {
	case vol <= threshold/2:
		status = StatusStrong
	case vol <= threshold:
		status = StatusAcceptable
	case vol <= threshold*1.5:
		status = StatusWeak
	default:
		status = StatusConcerning
	}
	return Dimension{
		Code:        DimensionWorkingCapitalStability,
		Status:      status,
		Value:       AvailableValue(vol),
		Threshold:   threshold,
		Explanation: fmt.Sprintf("NWC-as-%%-of-revenue volatility of %.1f%% against a %.1f%% threshold", vol*100, threshold*100),
		Source:      "working_capital",
	}
}

// classifyDebtLeverage assesses NetDebt/EBITDA from the most recent
// Input.Metrics snapshot.
func classifyDebtLeverage(in Input) Dimension {
	snap, ok := mostRecentSnapshot(in.Metrics.Snapshots, in.PeriodMeta)
	if !ok || !snap.NetDebt.Available || !snap.EBITDA.Available {
		return unassessed(DimensionDebtLeverage,
			"most recent Metrics snapshot's NetDebt/EBITDA is unavailable")
	}
	if snap.EBITDA.Value <= 0 {
		return unassessed(DimensionDebtLeverage,
			"most recent Metrics snapshot's EBITDA is zero or negative; leverage multiple is not meaningful")
	}
	leverage := snap.NetDebt.Value / snap.EBITDA.Value

	if in.Policy.MaxAcceptableNetDebtToEBITDA <= 0 {
		return Dimension{
			Code:        DimensionDebtLeverage,
			Status:      StatusAcceptable,
			Value:       AvailableValue(leverage),
			Explanation: fmt.Sprintf("net debt is %.2fx EBITDA (no Policy threshold supplied to grade it further)", leverage),
			Source:      "metrics",
		}
	}

	threshold := in.Policy.MaxAcceptableNetDebtToEBITDA
	status := StatusAcceptable
	switch {
	case leverage <= threshold/2:
		status = StatusStrong
	case leverage <= threshold:
		status = StatusAcceptable
	case leverage <= threshold*1.5:
		status = StatusWeak
	default:
		status = StatusConcerning
	}
	return Dimension{
		Code:        DimensionDebtLeverage,
		Status:      status,
		Value:       AvailableValue(leverage),
		Threshold:   threshold,
		Explanation: fmt.Sprintf("net debt is %.2fx EBITDA against a %.2fx threshold", leverage, threshold),
		Source:      "metrics",
	}
}

// classifyDataCompleteness assesses how many of the five optional
// analytical-module inputs (QoE, WorkingCapital, Concentration,
// RevenueQuality, Consensus) were supplied and available — distinct from
// Coverage (which counts assessed dimensions, a broader superset that also
// includes Profile/DataQuality-only dimensions). Always assessed (never
// StatusUnassessed) since the count itself is always computable, even when
// it is zero.
func classifyDataCompleteness(in Input) Dimension {
	total := 5
	available := 0
	if in.QoE.Available {
		available++
	}
	if in.WorkingCapital.Available {
		available++
	}
	if in.Concentration.Available {
		available++
	}
	if in.RevenueQuality.Available {
		available++
	}
	if in.Consensus.Available {
		available++
	}
	pct := float64(available) / float64(total)

	var status Status
	switch {
	case available == total:
		status = StatusStrong
	case pct >= 0.6:
		status = StatusAcceptable
	case pct >= 0.3:
		status = StatusWeak
	default:
		status = StatusConcerning
	}

	return Dimension{
		Code:        DimensionDataCompleteness,
		Status:      status,
		Value:       AvailableValue(pct),
		Explanation: fmt.Sprintf("%d of %d optional analytical modules supplied and available", available, total),
		Source:      "input",
	}
}

// classifyValuationMethodConsensus assesses valuation-method agreement
// from Input.Consensus.Statistics.CoefficientOfVariation.
func classifyValuationMethodConsensus(in Input) Dimension {
	if !in.Consensus.Available {
		return unassessed(DimensionValuationMethodConsensus,
			"Consensus.Available is false")
	}
	if in.Consensus.Statistics.Count < 2 {
		return unassessed(DimensionValuationMethodConsensus,
			"fewer than two included valuation methods; dispersion is not meaningful")
	}
	cov := in.Consensus.Statistics.CoefficientOfVariation

	if in.Policy.MaxAcceptableValuationDispersion <= 0 {
		return Dimension{
			Code:        DimensionValuationMethodConsensus,
			Status:      StatusAcceptable,
			Value:       AvailableValue(cov),
			Explanation: fmt.Sprintf("valuation-method coefficient of variation is %.1f%% across %d methods (no Policy threshold supplied to grade it further)", cov*100, in.Consensus.Statistics.Count),
			Source:      "consensus",
		}
	}

	threshold := in.Policy.MaxAcceptableValuationDispersion
	status := StatusAcceptable
	switch {
	case cov <= threshold/2:
		status = StatusStrong
	case cov <= threshold:
		status = StatusAcceptable
	case cov <= threshold*1.5:
		status = StatusWeak
	default:
		status = StatusConcerning
	}
	return Dimension{
		Code:        DimensionValuationMethodConsensus,
		Status:      status,
		Value:       AvailableValue(cov),
		Threshold:   threshold,
		Explanation: fmt.Sprintf("valuation-method coefficient of variation is %.1f%% across %d methods against a %.1f%% threshold", cov*100, in.Consensus.Statistics.Count, threshold*100),
		Source:      "consensus",
	}
}

// buildDimensions classifies every dimension in dimensionOrder.
func buildDimensions(in Input) []Dimension {
	dims := make([]Dimension, 0, len(dimensionOrder))
	dims = append(dims, classifyFinancialRecordQuality(in))
	dims = append(dims, classifyEarningsStability(in))
	dims = append(dims, classifyNormalizationBurden(in))
	dims = append(dims, classifyCustomerConcentration(in))
	dims = append(dims, classifyRecurringRevenue(in))
	dims = append(dims, classifyOwnerDependence(in))
	dims = append(dims, classifyMarginTrend(in))
	dims = append(dims, classifyWorkingCapitalStability(in))
	dims = append(dims, classifyDebtLeverage(in))
	dims = append(dims, classifyDataCompleteness(in))
	dims = append(dims, classifyValuationMethodConsensus(in))
	return dims
}
