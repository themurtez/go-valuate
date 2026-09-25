package advisory

// buildRevenueSection composes REVENUE from
// Input.Financial.RevenueQuality/Concentration and
// Input.Operating.Profitability's customer view — task section 20.
func buildRevenueSection(in Input, policy Policy) Section {
	rq := in.Financial.RevenueQuality
	conc := in.Financial.Concentration
	if !rq.Available && !conc.Available {
		return newUnavailableSection(SectionRevenue, StatusNotSupplied)
	}

	var metricsOut []Metric
	var findings []Insight
	var actions []ActionItem
	usedModules := map[string]bool{}
	period := currentPeriodLabel(in)

	if rq.Available && len(rq.TotalRevenueHistory) > 0 {
		usedModules["revenue_quality"] = true
		latest := rq.TotalRevenueHistory[len(rq.TotalRevenueHistory)-1]
		if latest.RecurringPercent.Available {
			metricsOut = append(metricsOut, newMetric("recurring_revenue_percent", "Recurring Revenue %", AvailableValue(latest.RecurringPercent.Value), UnitPercent, string(latest.Period), "revenue_quality", "recurring_percent"))
		}
		if len(rq.CustomerTransitions) > 0 {
			t := rq.CustomerTransitions[len(rq.CustomerTransitions)-1]
			if t.RetainedRevenue.Available {
				metricsOut = append(metricsOut, newMetric("retained_revenue", "Retained Revenue", AvailableValue(t.RetainedRevenue.Value), UnitCurrency, string(t.ToPeriod), "revenue_quality", "customer_transitions.retained_revenue"))
			}
			if t.LostCustomerRevenue.Available && t.LostCustomerRevenue.Value > 0 {
				findings = append(findings, Insight{
					Code: "LOST_CUSTOMER_REVENUE", Category: string(SectionRevenue), Severity: SeverityMedium,
					Title: "Lost customer revenue", Statement: statementValueChanged("Lost customer revenue", 0, t.LostCustomerRevenue.Value),
					Period: string(t.ToPeriod), Current: AvailableValue(t.LostCustomerRevenue.Value),
					SourceModule: "revenue_quality", SourceCode: "customer_transitions.lost_customer_revenue",
					SourceRefs: []SourceRef{{Module: "revenue_quality", Period: string(t.ToPeriod)}},
				})
				actions = append(actions, newGeneratedAction(actionTemplates[ActionReviewCustomerRetentionChange], PriorityMedium, "revenue_quality", "lost_customer_revenue", []SourceRef{{Module: "revenue_quality", Period: string(t.ToPeriod)}}, "", string(t.ToPeriod)))
			}
		}
	}

	if conc.Available && len(conc.History) > 0 {
		usedModules["concentration"] = true
		latest := conc.History[len(conc.History)-1]
		if latest.LargestEntityShare.Available {
			metricsOut = append(metricsOut, newMetric("largest_customer_share", "Largest Customer Share", AvailableValue(latest.LargestEntityShare.Value), UnitPercent, string(latest.Period), "concentration", "largest_entity_share"))
		}
		if latest.HHI.Available {
			metricsOut = append(metricsOut, newMetric("customer_concentration_hhi", "Customer Concentration (HHI)", AvailableValue(latest.HHI.Value), UnitCount, string(latest.Period), "concentration", "hhi"))
		}
		if len(conc.History) >= 2 {
			prior := conc.History[len(conc.History)-2]
			if latest.LargestEntityShare.Available && prior.LargestEntityShare.Available {
				change := computeChange(AvailableValue(latest.LargestEntityShare.Value), AvailableValue(prior.LargestEntityShare.Value), true)
				if policy.Materiality.isMaterial(change) && change.PercentagePointChange.Amount > 0 {
					findings = append(findings, Insight{
						Code: string(StatementCustomerConcentrationIncreased), Category: string(SectionRevenue), Severity: SeverityMedium,
						Title:     "Customer concentration increased",
						Statement: statementPercentIncreased("Largest customer share", prior.LargestEntityShare.Value, latest.LargestEntityShare.Value),
						Period:    string(latest.Period), Current: AvailableValue(latest.LargestEntityShare.Value), Prior: AvailableValue(prior.LargestEntityShare.Value), Change: change,
						SourceModule: "concentration", SourceCode: "largest_entity_share",
						SourceRefs: []SourceRef{{Module: "concentration", Period: string(latest.Period)}},
					})
				}
			}
		}
	}

	if len(metricsOut) == 0 && len(findings) == 0 {
		return newUnavailableSection(SectionRevenue, StatusUnavailable)
	}

	sources := make([]SourceRef, 0, len(usedModules))
	for m := range usedModules {
		sources = append(sources, SourceRef{Module: m, Period: period})
	}

	return Section{
		Code: SectionRevenue, Availability: StatusAvailable,
		Metrics: metricsOut, Findings: capInsightsForSection(findings, policy), Actions: capActionsForSection(dedupeActions(actions), policy),
		Sources: sortedSourceRefs(sources),
	}
}
