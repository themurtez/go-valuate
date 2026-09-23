package forecast

import (
	"fmt"

	"github.com/themurtez/go-valuate/analytics/workingcapital"
)

// Calculate derives a full Result from in: it identifies the historical
// base period, then projects every Input.Scenarios entry forward across
// Input.Horizon periods. It never mutates Input.Dataset or any Scenario and
// performs no I/O.
func Calculate(in Input) Result {
	wcPolicy := resolveWorkingCapitalPolicy(in.WorkingCapitalPolicy)
	dscSource := resolveDSCSource(in.DebtServiceCoverageSource)
	result := Result{
		FormulaVersion:            FormulaVersion,
		Horizon:                   in.Horizon,
		DebtServiceCoverageSource: dscSource,
		WorkingCapitalPolicy:      wcPolicy,
	}

	periods := in.Dataset.Periods()
	if len(periods) == 0 {
		result.Errors = append(result.Errors, Issue{
			Code:     IssueNoPeriods,
			Severity: SeverityError,
			Message:  "dataset has no periods; forecast requires at least one historical period to project from",
		})
		return result
	}

	basePeriod, baseIssue := resolveBasePeriod(periods, in.PeriodMeta)
	if baseIssue != nil {
		result.Errors = append(result.Errors, *baseIssue)
		return result
	}

	if in.Horizon < 1 {
		result.Errors = append(result.Errors, Issue{
			Code:     IssueInvalidHorizon,
			Severity: SeverityError,
			Message:  fmt.Sprintf("horizon must be at least 1 forecast period, got %d", in.Horizon),
		})
		return result
	}

	if len(in.Scenarios) == 0 {
		result.Errors = append(result.Errors, Issue{
			Code:     IssueNoScenarios,
			Severity: SeverityError,
			Message:  "at least one scenario is required",
		})
		return result
	}

	result.Available = true

	idx := buildCodeIndex(in.Dataset)
	basePL := buildBasePL(idx, basePeriod)
	baseNWC := buildBaseWorkingCapital(idx, basePeriod, wcPolicy)
	result.Base = BaseFinancials{Period: basePeriod, PL: basePL, WorkingCapital: baseNWC}

	forecastLabels := make([]string, in.Horizon)
	for i := 0; i < in.Horizon; i++ {
		forecastLabels[i] = periodLabel(in.ForecastPeriodLabels, i)
	}
	result.ForecastPeriods = forecastLabels

	seenNames := make(map[string]bool, len(in.Scenarios))
	var scenarioResults []ScenarioResult
	var inputWarnings []Issue
	for _, scenario := range in.Scenarios {
		if scenario.Name == "" {
			inputWarnings = append(inputWarnings, Issue{
				Code: IssueEmptyScenarioName, Severity: SeverityWarning,
				Message: "a scenario with an empty Name was skipped",
			})
			continue
		}
		if seenNames[scenario.Name] {
			inputWarnings = append(inputWarnings, Issue{
				Code: IssueDuplicateScenarioName, Severity: SeverityWarning, Scenario: scenario.Name,
				Message: fmt.Sprintf("scenario name %q is duplicated; only the first occurrence was projected", scenario.Name),
			})
			continue
		}
		seenNames[scenario.Name] = true

		sr, _ := projectScenario(scenario, basePL, baseNWC, in.Horizon, in.ForecastPeriodLabels, dscSource)
		scenarioResults = append(scenarioResults, sr)
	}
	result.ScenarioResults = scenarioResults
	result.Warnings = inputWarnings

	return result
}

// resolveWorkingCapitalPolicy returns p if non-empty, otherwise
// workingcapital.DefaultInclusionPolicy() — the same zero-value-means-
// defaults rule cashflow.Input.Policy resolution follows.
func resolveWorkingCapitalPolicy(p workingcapital.InclusionPolicy) workingcapital.InclusionPolicy {
	if len(p.AssetCodes) == 0 && len(p.LiabilityCodes) == 0 {
		return workingcapital.DefaultInclusionPolicy()
	}
	return p
}
