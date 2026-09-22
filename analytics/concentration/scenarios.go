package concentration

// marginRateFor resolves the applicable margin rate for entityKey: an
// EntityImpactAssumption covering it takes precedence over
// Policy.DefaultImpactMarginRate. Returns (rate, true) if one applies,
// (0, false) otherwise — this package never invents a margin assumption a
// caller did not supply (see Policy.DefaultImpactMarginRate's doc comment).
func marginRateFor(entityKey string, policy Policy) (float64, bool) {
	for _, a := range policy.EntityImpactAssumptions {
		if a.EntityKey == entityKey {
			return a.MarginRate, true
		}
	}
	if policy.DefaultImpactMarginRate != nil {
		return *policy.DefaultImpactMarginRate, true
	}
	return 0, false
}

// buildScenario constructs a Scenario removing the top n entities (by
// Amount) from period's RankedEntities.
func buildScenario(kind ScenarioKind, n int, period PeriodConcentration, policy Policy) Scenario {
	cutoff := n
	if cutoff > len(period.RankedEntities) {
		cutoff = len(period.RankedEntities)
	}
	removed := period.RankedEntities[:cutoff]

	sc := Scenario{
		Kind:   kind,
		N:      n,
		Period: period.Period,
	}

	var totalRevenue float64
	var totalEarnings float64
	allHaveEarnings := len(removed) > 0
	impacts := make([]EntityImpact, 0, len(removed))
	for _, re := range removed {
		impact := EntityImpact{
			EntityKey:     re.EntityKey,
			RevenueImpact: AvailableValue(re.Amount),
		}
		totalRevenue += re.Amount
		if rate, ok := marginRateFor(re.EntityKey, policy); ok {
			impact.EarningsImpact = AvailableValue(re.Amount * rate)
			impact.MarginRateUsed = rate
			totalEarnings += re.Amount * rate
		} else {
			allHaveEarnings = false
		}
		impacts = append(impacts, impact)
	}
	sc.EntityImpacts = impacts
	sc.TotalRevenueImpact = AvailableValue(totalRevenue)
	if allHaveEarnings {
		sc.TotalEarningsImpact = AvailableValue(totalEarnings)
	}

	if period.TotalAmount.Available {
		sc.RemainingRevenue = AvailableValue(period.TotalAmount.Value - totalRevenue)
		if period.TotalAmount.Value != 0 {
			sc.RevenueImpactPercent = AvailableValue(totalRevenue / period.TotalAmount.Value)
		}
	}

	return sc
}

// calculateScenarios builds ScenarioLostLargestEntity plus one
// ScenarioTopNLoss per policy.ScenarioTopN, all against the chronologically
// last period in history (history is expected already in chronological
// order — Calculate only calls this when chronological order was
// established). Returns nil if history is empty or its last period has no
// entities.
func calculateScenarios(history []PeriodConcentration, policy Policy) []Scenario {
	if len(history) == 0 {
		return nil
	}
	period := history[len(history)-1]
	if len(period.RankedEntities) == 0 {
		return nil
	}

	scenarios := make([]Scenario, 0, 1+len(policy.ScenarioTopN))
	scenarios = append(scenarios, buildScenario(ScenarioLostLargestEntity, 1, period, policy))
	for _, n := range sortedTopN(policy.ScenarioTopN) {
		scenarios = append(scenarios, buildScenario(ScenarioTopNLoss, n, period, policy))
	}
	return scenarios
}
