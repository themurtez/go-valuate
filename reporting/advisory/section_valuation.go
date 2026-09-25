package advisory

// buildValuationSection composes VALUATION_AND_VALUE_DRIVERS from
// Input.Valuation.Consensus and Input.Financial.ValueDrivers — task
// section 28. Never recomputes valuation; basis (Enterprise/Equity/Asset)
// is preserved verbatim from consensus.Result.Basis, never combined
// across an incompatible basis — task section 91.
func buildValuationSection(in Input, policy Policy) Section {
	c := in.Valuation.Consensus
	vd := in.Financial.ValueDrivers
	if !c.Available && !vd.Available {
		return newUnavailableSection(SectionValuation, StatusNotSupplied)
	}

	period := currentPeriodLabel(in)
	var metricsOut []Metric
	var findings []Insight
	var actions []ActionItem
	usedModules := map[string]bool{}

	if c.Available {
		usedModules["consensus"] = true
		amount := c.Statistics.WeightedMean
		if !c.WeightsValid {
			amount = c.Statistics.SimpleMean
		}
		metricsOut = append(metricsOut, Metric{
			Code: metricCodeIndicatedValue, Label: "Indicated Value (" + string(c.Basis) + ")",
			Value: AvailableValue(amount), Unit: UnitCurrency, Period: period,
			SourceModule: "consensus", SourceCode: "statistics.weighted_mean",
		})
		metricsOut = append(metricsOut,
			newMetric("valuation_range_min", "Valuation Range (Min)", AvailableValue(c.Range.Min), UnitCurrency, period, "consensus", "range.min"),
			newMetric("valuation_range_max", "Valuation Range (Max)", AvailableValue(c.Range.Max), UnitCurrency, period, "consensus", "range.max"),
		)
		if c.Dispersion.Score != 0 || c.Statistics.Count > 0 {
			metricsOut = append(metricsOut, newMetric("valuation_dispersion_score", "Valuation Dispersion Score", AvailableValue(float64(c.Dispersion.Score)), UnitCount, period, "consensus", "dispersion.score"))
		}
		if c.Statistics.Count > 1 {
			actions = append(actions, newGeneratedAction(actionTemplates[ActionReviewValuationSensitivity], PriorityLow, "consensus", "range", []SourceRef{{Module: "consensus", Period: period}}, "", period))
		}
	}

	if vd.Available {
		usedModules["value_drivers"] = true
		for _, s := range vd.Scenarios {
			if !s.ConsensusDeltaAvailable {
				continue
			}
			findings = append(findings, Insight{
				Code: "VALUE_DRIVER_SCENARIO_MOVEMENT", Category: string(SectionValuation), Severity: SeverityInfo,
				Title:     "Value driver scenario: " + s.Label,
				Statement: statementValueChanged("Consensus value under scenario "+s.Label, 0, s.ConsensusValueDelta),
				Period:    period, Change: Change{AbsoluteChange: AvailableValue(s.ConsensusValueDelta), PercentChange: AvailableValue(s.ConsensusPercentDelta / 100)},
				SourceModule: "value_drivers", SourceCode: s.ScenarioID,
				SourceRefs: []SourceRef{{Module: "value_drivers", Code: s.ScenarioID, Period: period}},
			})
			actions = append(actions, newGeneratedAction(actionTemplates[ActionReviewValueDriverChange], PriorityLow, "value_drivers", s.ScenarioID, []SourceRef{{Module: "value_drivers", Code: s.ScenarioID, Period: period}}, "", period))
		}
	}

	sources := make([]SourceRef, 0, len(usedModules))
	for m := range usedModules {
		sources = append(sources, SourceRef{Module: m, Period: period})
	}

	return Section{
		Code: SectionValuation, Availability: StatusAvailable,
		Metrics: metricsOut, Findings: capInsightsForSection(findings, policy), Actions: capActionsForSection(dedupeActions(actions), policy),
		Sources: sortedSourceRefs(sources),
	}
}
