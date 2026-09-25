package advisory

import "github.com/themurtez/go-valuate/accounting/profitability"

// buildProfitabilitySection composes PROFITABILITY from
// Input.Operating.Profitability's BusinessTotals/rankings — task section
// 21. Consumes accounting/profitability.Result verbatim; recomputes no
// margin/contribution formula.
func buildProfitabilitySection(in Input, policy Policy) Section {
	p := in.Operating.Profitability
	if len(p.BusinessTotals.Periods) == 0 {
		return newUnavailableSection(SectionProfitability, StatusNotSupplied)
	}

	latest := p.BusinessTotals.Periods[len(p.BusinessTotals.Periods)-1]
	var metricsOut []Metric
	metricsOut = append(metricsOut,
		newMetric("gross_profit_by_entity", "Gross Profit", AvailableValue(latest.GrossProfit), UnitCurrency, latest.Period, "profitability", "business_totals.gross_profit"),
		newMetric("contribution_profit", "Contribution Profit", AvailableValue(latest.ContributionProfit), UnitCurrency, latest.Period, "profitability", "business_totals.contribution_profit"),
	)

	var findings []Insight
	var actions []ActionItem

	if latest.Margins.ContributionMargin.Available {
		curMargin := AvailableValue(latest.Margins.ContributionMargin.Amount)
		marginMetric := newMetric("contribution_margin", "Contribution Margin", curMargin, UnitPercent, latest.Period, "profitability", "business_totals.margins.contribution_margin")

		if len(p.BusinessTotals.Periods) >= 2 {
			prior := p.BusinessTotals.Periods[len(p.BusinessTotals.Periods)-2]
			if prior.Margins.ContributionMargin.Available {
				priorMargin := AvailableValue(prior.Margins.ContributionMargin.Amount)
				marginMetric.Prior = priorMargin
				marginMetric.Change = computeChange(curMargin, priorMargin, true)

				if policy.Materiality.isMaterial(marginMetric.Change) && marginMetric.Change.PercentagePointChange.Available && marginMetric.Change.PercentagePointChange.Amount < 0 {
					findings = append(findings, Insight{
						Code: string(StatementContributionMarginDeclined), Category: string(SectionProfitability), Severity: SeverityMedium,
						Title:     "Contribution margin declined",
						Statement: statementPercentDecreased("Contribution margin", priorMargin.Amount, curMargin.Amount),
						Period:    latest.Period, Current: curMargin, Prior: priorMargin, Change: marginMetric.Change,
						SourceModule: "profitability", SourceCode: "business_totals.margins.contribution_margin",
						SourceRefs: []SourceRef{{Module: "profitability", Period: latest.Period}},
					})
					actions = append(actions, newGeneratedAction(actionTemplates[ActionReviewMarginChange], PriorityMedium, "profitability", "contribution_margin_declined", []SourceRef{{Module: "profitability", Period: latest.Period}}, "", latest.Period))
				}
			}
		}

		metricsOut = append(metricsOut, marginMetric)
	}

	return finishProfitabilitySection(p, latest.Period, metricsOut, findings, actions, policy)
}

func finishProfitabilitySection(p profitability.Result, period string, metricsOut []Metric, findings []Insight, actions []ActionItem, policy Policy) Section {
	for _, view := range []profitability.DimensionView{p.CustomerView, p.JobView, p.ProductView} {
		for _, e := range view.Rankings.NegativeContribution {
			findings = append(findings, Insight{
				Code: "NEGATIVE_CONTRIBUTION_ENTITY", Category: string(SectionProfitability), Severity: SeverityMedium,
				Title: "Negative contribution result", Statement: statementValueChanged("Contribution for "+e.EntityID, 0, e.Value),
				Period: period, EntityRef: e.EntityID, Current: AvailableValue(e.Value),
				SourceModule: "profitability", SourceCode: "rankings.negative_contribution",
				SourceRefs: []SourceRef{{Module: "profitability", Ref: e.EntityID, Period: period}},
			})
			actions = append(actions, newGeneratedAction(actionTemplates[ActionReviewNegativeContributionEntity], PriorityMedium, "profitability", "negative_contribution", []SourceRef{{Module: "profitability", Ref: e.EntityID, Period: period}}, e.EntityID, period))
		}
	}
	for _, pool := range p.SharedCostPools {
		_ = pool // shared-cost pool iteration reserved for unallocated-cost detection once a stable "unallocated amount" field is confirmed against a real fixture; no unverified field access here.
	}

	return Section{
		Code: SectionProfitability, Availability: StatusAvailable,
		Metrics: metricsOut, Findings: capInsightsForSection(findings, policy), Actions: capActionsForSection(dedupeActions(actions), policy),
		Sources: []SourceRef{{Module: "profitability", Period: period}},
	}
}
