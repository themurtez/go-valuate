package diagnostics

const (
	scorePointsCriticalConcern = 0.0
	scorePointsWarningConcern  = 5.0
	scorePointsNeutral         = 8.0
	scorePointsStrength        = 10.0
)

// computeHealthScore implements OverallHealthScore's documented, fixed
// formula: start from a perfect-health baseline of scorePointsNeutral (8.0)
// points per available category, then let every mined Finding in that
// category pull the category's score toward scorePointsStrength (a
// SeverityInfo Finding classified as a Strength) or down toward
// scorePointsCriticalConcern/scorePointsWarningConcern (a Concern, by
// Severity) — the single worst Concern in a category, if any, sets that
// category's floor (a category with both a critical and a warning Concern
// scores as the critical one; multiple Concerns of the same severity do
// not compound the penalty further, since Severity already reflects how
// bad any one of them is). A category with zero Findings at all keeps the
// neutral baseline — this package never treats "nothing detected" as
// either a Strength or a Concern, since twelve of the fifteen source
// modules can easily have nothing to say about a given category without
// that silence meaning the category is actually healthy.
//
// Only categories backed by at least one available module (per
// moduleCategories) are averaged into Value; a category with zero
// contributing modules available is excluded entirely, mirroring
// salereadiness.computeOverallScore's identical "never disagrees with the
// classification, only weights it" design and its "unassessed contributes
// neither points nor to the divisor" rule.
//
// Returns (Score{}, false) when zero categories have any available
// module, or when policy.MinCoveragePercentForScore is set and
// coverage.CoveragePercent is below it — see Result.OverallHealthScore's
// doc comment.
func computeHealthScore(findings []Finding, coverage Coverage, policy Policy) (Score, bool) {
	if coverage.AvailableModules == 0 {
		return Score{}, false
	}
	if policy.MinCoveragePercentForScore > 0 && coverage.CoveragePercent < policy.MinCoveragePercentForScore {
		return Score{}, false
	}

	availableCategories := categoriesWithAvailableModule(coverage)
	if len(availableCategories) == 0 {
		return Score{}, false
	}

	countsByCategory := make(map[Category]int, len(categoryOrder))
	floorByCategory := make(map[Category]float64, len(categoryOrder))
	hasFloor := make(map[Category]bool, len(categoryOrder))
	hasStrength := make(map[Category]bool, len(categoryOrder))

	for _, f := range findings {
		countsByCategory[f.Category]++
		switch f.Severity {
		case SeverityCritical:
			if !hasFloor[f.Category] || scorePointsCriticalConcern < floorByCategory[f.Category] {
				floorByCategory[f.Category] = scorePointsCriticalConcern
				hasFloor[f.Category] = true
			}
		case SeverityWarning:
			if !hasFloor[f.Category] || scorePointsWarningConcern < floorByCategory[f.Category] {
				floorByCategory[f.Category] = scorePointsWarningConcern
				hasFloor[f.Category] = true
			}
		case SeverityInfo:
			hasStrength[f.Category] = true
		}
	}

	var totalPoints float64
	components := make([]ScoreComponent, 0, len(availableCategories))
	for _, cat := range availableCategories {
		points := scorePointsNeutral
		switch {
		case hasFloor[cat]:
			points = floorByCategory[cat]
		case hasStrength[cat]:
			points = scorePointsStrength
		}
		totalPoints += points
		components = append(components, ScoreComponent{
			Category:     cat,
			FindingCount: countsByCategory[cat],
			Points:       points,
		})
	}

	value := (totalPoints / (float64(len(availableCategories)) * scorePointsStrength)) * 100
	if value < 0 {
		value = 0
	}
	if value > 100 {
		value = 100
	}

	return Score{
		Version:    ScoreVersion,
		Value:      value,
		Components: components,
		Label:      labelForHealthScore(value),
		Heuristic:  true,
	}, true
}

// categoriesWithAvailableModule returns, in categoryOrder, every Category
// backed by at least one currently-available module per moduleCategories —
// the denominator computeHealthScore averages over.
func categoriesWithAvailableModule(coverage Coverage) []Category {
	available := map[SourceModule]bool{
		SourceMetrics:        coverage.WithMetrics,
		SourceRatios:         coverage.WithRatios,
		SourceQoE:            coverage.WithQoE,
		SourceWorkingCapital: coverage.WithWorkingCapital,
		SourceCashFlow:       coverage.WithCashFlow,
		SourceRevenueQuality: coverage.WithRevenueQuality,
		SourceConcentration:  coverage.WithConcentration,
		SourceAnomalies:      coverage.WithAnomalies,
		SourceVariance:       coverage.WithVariance,
		SourceForecast:       coverage.WithForecast,
		SourceDebt:           coverage.WithDebt,
		SourceCovenants:      coverage.WithCovenants,
		SourceBenchmarks:     coverage.WithBenchmarks,
		SourceValueDrivers:   coverage.WithValueDrivers,
		SourceSaleReadiness:  coverage.WithSaleReadiness,
	}

	present := make(map[Category]bool, len(categoryOrder))
	for module, ok := range available {
		if !ok {
			continue
		}
		for _, cat := range moduleCategories[module] {
			present[cat] = true
		}
	}

	var out []Category
	for _, cat := range categoryOrder {
		if present[cat] {
			out = append(out, cat)
		}
	}
	return out
}

// labelForHealthScore returns a short, fixed, human-readable
// characterization of value's range — purely descriptive, mirroring
// salereadiness.labelForScore's identical role and fixed band boundaries.
func labelForHealthScore(value float64) string {
	switch {
	case value >= 85:
		return "Strong"
	case value >= 65:
		return "Stable"
	case value >= 45:
		return "Needs attention"
	default:
		return "Significant concerns"
	}
}
