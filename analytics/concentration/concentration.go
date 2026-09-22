package concentration

// Calculate derives a full Result from in under opts. It never mutates
// in.Observations and performs no I/O.
func Calculate(in Input, opts Options) Result {
	policy := resolvePolicy(in.Policy)
	thresholds := resolveThresholds(opts.Thresholds)
	basis := in.Basis
	if basis == "" {
		basis = BasisOther
	}
	result := Result{FormulaVersion: FormulaVersion, Basis: basis, Policy: policy, Thresholds: thresholds}

	if len(in.Observations) == 0 {
		result.Errors = append(result.Errors, Issue{
			Code:     IssueNoObservations,
			Severity: SeverityError,
			Message:  "no observations supplied; concentration analysis requires at least one",
		})
		return result
	}
	result.Available = true

	result.Warnings = append(result.Warnings, validateObservations(in.Observations)...)
	validObs := filterValidObservations(in.Observations)

	fallbackOrder := orderedPeriodsOf(validObs)
	orderedPeriods, orderIssue := chronologicalPeriods(fallbackOrder, in.PeriodMeta)
	if orderIssue != nil {
		result.Warnings = append(result.Warnings, *orderIssue)
	}
	chronological := orderIssue == nil

	byPeriod := groupByPeriod(validObs)

	history := make([]PeriodConcentration, 0, len(orderedPeriods))
	for _, p := range orderedPeriods {
		rows, ok := byPeriod[p]
		if !ok {
			continue
		}
		history = append(history, computePeriodConcentration(p, rows, policy.TopN))
	}
	result.History = history

	result.LargestShareTrend = Trend{Direction: TrendUnavailable}
	result.HHITrend = Trend{Direction: TrendUnavailable}
	if chronological {
		result.LargestShareTrend = calculateTrend(history, func(p PeriodConcentration) ConcentrationValue { return p.LargestEntityShare })
		result.HHITrend = calculateTrend(history, func(p PeriodConcentration) ConcentrationValue { return p.HHI })
		result.DependencyChanges = calculateDependencyChanges(orderedPeriods, byPeriod)
		result.Scenarios = calculateScenarios(history, policy)

		if !everyScenarioEntityHasImpact(result.Scenarios) {
			result.Warnings = append(result.Warnings, Issue{
				Code:     IssueNoImpactAssumption,
				Severity: SeverityWarning,
				Message:  "no margin assumption (Policy.DefaultImpactMarginRate or a matching EntityImpactAssumption) covers every entity removed by at least one Scenario; those Scenarios' earnings-impact figures are unavailable",
			})
		}
	}

	result.Flags = buildFlags(result, thresholds)

	return result
}

// everyScenarioEntityHasImpact reports whether every EntityImpact across
// every Scenario has an available EarningsImpact — used only to decide
// whether to emit the advisory IssueNoImpactAssumption. Checked against the
// entities Scenarios actually removed (rather than every entity in the
// period) so a caller who only supplied margin data for its largest
// entities does not get a spurious warning when no configured scenario
// reaches beyond them.
func everyScenarioEntityHasImpact(scenarios []Scenario) bool {
	for _, s := range scenarios {
		for _, ei := range s.EntityImpacts {
			if !ei.EarningsImpact.Available {
				return false
			}
		}
	}
	return true
}
