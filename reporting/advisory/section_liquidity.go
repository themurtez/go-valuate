package advisory

import (
	"github.com/themurtez/go-valuate/accounting/cashforecast"
)

// Local metric codes used only within this section.
const (
	metricCodeRequiredFunding  = "required_funding"
	metricCodeFacilityCapacity = "facility_capacity"
	metricCodeMinimumCashWeek  = "minimum_cash_week"
	metricCodeOpeningCash      = "opening_cash"
)

// buildLiquiditySection composes the LIQUIDITY section entirely from
// Input.Operating.CashForecast (accounting/cashforecast.Result) — task
// section 17. This package recomputes no forecast; every Metric here is
// read verbatim from CashForecast.BaseScenario.Summary/OpeningPosition —
// task section 2's "do not rebuild a 13-week forecast" rule.
func buildLiquiditySection(in Input, policy Policy) Section {
	cf := in.Operating.CashForecast
	if !cf.Available {
		status := StatusNotSupplied
		if hasNonZeroCashForecast(cf) {
			status = StatusUnavailable
		}
		return newUnavailableSection(SectionLiquidity, status)
	}

	period := currentPeriodLabel(in)
	summary := cf.BaseScenario.Summary

	metrics := []Metric{
		newMetric(metricCodeOpeningCash, "Opening Cash", AvailableValue(cf.OpeningPosition.UnrestrictedCash), UnitCurrency, period, "cashforecast", "opening_position.unrestricted_cash"),
		newMetric(metricCodeEndingCash, "Ending Cash (13-Week)", AvailableValue(summary.EndingCash), UnitCurrency, period, "cashforecast", "base_scenario.summary.ending_cash"),
		newMetric(metricCodeMinimumCash, "Minimum Cash (13-Week)", AvailableValue(summary.LowestCashBalance), UnitCurrency, period, "cashforecast", "base_scenario.summary.lowest_cash_balance"),
	}
	minWeekMetric := newMetric(metricCodeMinimumCashWeek, "Minimum Cash Week", AvailableValue(float64(summary.LowestCashWeek)), UnitCount, period, "cashforecast", "base_scenario.summary.lowest_cash_week")
	metrics = append(metrics, minWeekMetric)

	if summary.ThresholdAvailable {
		metrics = append(metrics, newMetric(metricCodeRequiredFunding, "Required Funding", AvailableValue(summary.RequiredFunding.RequiredAtStart), UnitCurrency, period, "cashforecast", "base_scenario.summary.required_funding.required_at_start"))
		if summary.FacilityCapacity != 0 {
			metrics = append(metrics, newMetric(metricCodeFacilityCapacity, "Available Facility Capacity", AvailableValue(summary.FacilityCapacity), UnitCurrency, period, "cashforecast", "base_scenario.summary.facility_capacity"))
		}
	}

	var findings []Insight
	var actions []ActionItem
	sourceRef := []SourceRef{{Module: "cashforecast", Period: period}}

	if summary.ThresholdAvailable && summary.NegativeCashReached {
		findings = append(findings, Insight{
			Code: string(StatementMinimumCashBelowThreshold), Category: string(SectionLiquidity),
			Severity: SeverityHigh, Priority: priorityForLiquidity(policy, true),
			Title:     "Minimum cash reaches a negative balance",
			Statement: statementMinimumCash(summary.LowestCashBalance, summary.LowestCashWeek, true),
			Current:   AvailableValue(summary.LowestCashBalance), Period: period,
			SourceModule: "cashforecast", SourceCode: "negative_cash_reached", SourceRefs: sourceRef,
		})
		actions = append(actions, newGeneratedAction(actionTemplates[ActionReviewMinimumCash], priorityForLiquidity(policy, true), "cashforecast", "negative_cash_reached", sourceRef, "", period))
	} else if summary.ThresholdAvailable && summary.WeeksBelowMinimum > 0 {
		findings = append(findings, Insight{
			Code: string(StatementMinimumCashBelowThreshold), Category: string(SectionLiquidity),
			Severity: SeverityMedium, Priority: priorityForLiquidity(policy, false),
			Title:     "Minimum cash falls below threshold",
			Statement: statementMinimumCash(summary.LowestCashBalance, summary.LowestCashWeek, false),
			Current:   AvailableValue(summary.LowestCashBalance), Period: period,
			SourceModule: "cashforecast", SourceCode: "below_minimum_cash", SourceRefs: sourceRef,
		})
		actions = append(actions, newGeneratedAction(actionTemplates[ActionReviewMinimumCash], priorityForLiquidity(policy, false), "cashforecast", "below_minimum_cash", sourceRef, "", period))
	}

	if summary.ThresholdAvailable && summary.MaximumFundingGap > 0 {
		actions = append(actions, newGeneratedAction(actionTemplates[ActionReviewFundingGap], PriorityHigh, "cashforecast", "funding_gap", sourceRef, "", period))
		if summary.FacilityCapacity > 0 {
			actions = append(actions, newGeneratedAction(actionTemplates[ActionReviewFacilityCapacity], PriorityMedium, "cashforecast", "facility_capacity", sourceRef, "", period))
		}
	}

	for _, f := range cf.Flags {
		if f.Scenario != "" && f.Scenario != cf.BaseScenario.Label {
			continue // task section 2: base-scenario facts only for this section's headline figures
		}
		findings = append(findings, insightFromCashForecastFlag(f, period))
	}

	actions = dedupeActions(actions)

	return Section{
		Code: SectionLiquidity, Availability: StatusAvailable,
		Metrics: metrics, Findings: capInsightsForSection(findings, policy), Actions: capActionsForSection(actions, policy),
		Sources: []SourceRef{{Module: "cashforecast", Period: period}},
	}
}

func hasNonZeroCashForecast(cf cashforecast.Result) bool {
	return cf.ForecastStartDate != "" || cf.HorizonWeeks != 0
}

func insightFromCashForecastFlag(f cashforecast.Flag, period string) Insight {
	return Insight{
		Code: string(f.Code), Category: string(SectionLiquidity),
		Severity: severityFromCashForecastFlag(f.Severity),
		Title:    "Cash forecast flag", Statement: f.Message,
		Period:       period,
		SourceModule: "cashforecast", SourceCode: string(f.Code),
		SourceRefs: []SourceRef{{Module: "cashforecast", Code: string(f.Code), Period: period}},
	}
}

func severityFromCashForecastFlag(s cashforecast.FlagSeverity) Severity {
	switch s {
	case cashforecast.FlagSeverityCritical:
		return SeverityHigh
	case cashforecast.FlagSeverityWarning:
		return SeverityMedium
	default:
		return SeverityInfo
	}
}

func priorityForLiquidity(policy Policy, negative bool) Priority {
	if negative {
		return PriorityCritical
	}
	if policy.Synthesis.MinimumCashThreshold > 0 {
		return PriorityHigh
	}
	return PriorityMedium
}
