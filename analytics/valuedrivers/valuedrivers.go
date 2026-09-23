package valuedrivers

import (
	"fmt"
	"math"

	"github.com/themurtez/go-valuate/valuation"
	"github.com/themurtez/go-valuate/valuation/consensus"
	"github.com/themurtez/go-valuate/valuation/orchestrator"
	"github.com/themurtez/go-valuate/valuation/report"
)

// percentOf mirrors consensus.percentOf exactly (numerator / |denominator|,
// or 0 if denominator is 0) — duplicated rather than imported since
// consensus does not export it; the same defined-zero-not-NaN rule
// applies here for the same reason (see consensus.percentOf's doc
// comment).
func percentOf(numerator, denominator float64) float64 {
	if denominator == 0 {
		return 0
	}
	return numerator / math.Abs(denominator)
}

// Calculate runs Input.BaselineRequest once to establish Result.Baseline,
// then re-runs a freshly-cloned copy of that same Request under every
// Input.Drivers entry independently (Result.OneFactorAtATime) and every
// Input.Scenarios entry's full Drivers list together (Result.Scenarios),
// computing method-by-method and consensus deltas against Baseline for
// each.
//
// Every recalculation goes through orchestrator.Execute and
// consensus.Calculate exactly as a caller invoking them directly would —
// see the package doc comment. Calculate never mutates Input or any value
// reachable from it (every Request is deep-copied — see cloneRequest —
// before a Driver's mutation is applied).
func Calculate(in Input) Result {
	result := Result{FormulaVersion: FormulaVersion}

	baselineRun := orchestrator.Execute(in.BaselineRequest)
	baselineConsensus := calculateConsensus(baselineRun, in)
	result.Baseline = Baseline{Run: baselineRun, Consensus: baselineConsensus}
	result.Available = true

	for _, driver := range in.Drivers {
		scenario := Scenario{ID: driver.ID, Label: driver.Label, Drivers: []Driver{driver}}
		result.OneFactorAtATime = append(result.OneFactorAtATime, runScenario(in, scenario, result.Baseline))
	}

	for _, scenario := range in.Scenarios {
		result.Scenarios = append(result.Scenarios, runScenario(in, scenario, result.Baseline))
	}

	return result
}

// calculateConsensus builds a consensus.Input slice from run's successful
// methods (identical to valuation/report.BuildConsensusInputs, duplicated
// as a thin private call here rather than depending on valuation/report
// for a single three-line helper — see buildConsensusInputs) and runs
// consensus.Calculate with in.ConsensusOptions.
func calculateConsensus(run orchestrator.Run, in Input) consensus.Result {
	inputs := report.BuildConsensusInputs(run, in.Weights)
	return consensus.Calculate(inputs, in.ConsensusOptions)
}

// runScenario is the shared engine behind both Result.OneFactorAtATime
// (a single-Driver Scenario) and Result.Scenarios (a multi-Driver
// Scenario): clone the baseline Request, apply every Driver in order,
// re-run orchestrator/consensus, and diff against baseline.
func runScenario(in Input, scenario Scenario, baseline Baseline) ScenarioResult {
	result := ScenarioResult{ScenarioID: scenario.ID, Label: scenario.Label}

	if scenario.ID == "" {
		result.Issues = append(result.Issues, DriverIssue{
			Code: IssueMissingScenarioID, Severity: DriverSeverityError,
			Message: "scenario has an empty ID; it was not run",
		})
		return result
	}
	if len(scenario.Drivers) == 0 {
		result.Issues = append(result.Issues, DriverIssue{
			Code: IssueEmptyScenario, Severity: DriverSeverityError, DriverID: scenario.ID,
			Message: fmt.Sprintf("scenario %q has no drivers; it was not run", scenario.ID),
		})
		return result
	}

	if dup := duplicateDriverIDIssue(scenario.Drivers); dup != nil {
		result.Issues = append(result.Issues, *dup)
	}

	req := cloneRequest(in.BaselineRequest)

	linkagesByMethod := map[valuation.Code][]Linkage{}
	for _, driver := range scenario.Drivers {
		outcomes, issues := applyDriver(&req, driver)
		result.Issues = append(result.Issues, issues...)
		if len(outcomes) == 0 {
			continue
		}
		linkedAny := false
		for _, oc := range outcomes {
			linkagesByMethod[oc.linkage.Method] = append(linkagesByMethod[oc.linkage.Method], oc.linkage)
			result.ChangedInputs = append(result.ChangedInputs, oc.changed...)
			if oc.linkage.Status == LinkageApplied {
				linkedAny = true
			}
		}
		if !linkedAny {
			result.Issues = append(result.Issues, DriverIssue{
				Code: IssueNoLinkedMethod, Severity: DriverSeverityWarning, DriverID: driver.ID,
				Message: fmt.Sprintf("driver %q ran without error but did not apply to any method (every targeted method was excluded or not applicable)", driver.ID),
			})
		}
		result.Assumptions = append(result.Assumptions, driverAssumption(driver))
	}

	result.Available = true
	result.Run = orchestrator.Execute(req)
	result.Consensus = calculateConsensus(result.Run, in)
	result.MethodDeltas = buildMethodDeltas(baseline.Run, result.Run, linkagesByMethod)
	result.ConsensusValueDelta, result.ConsensusPercentDelta, result.ConsensusDeltaAvailable = consensusDelta(baseline.Consensus, result.Consensus)

	return result
}

// duplicateDriverIDIssue returns a SeverityWarning DriverIssue if any
// non-empty Driver.ID appears more than once in drivers, or nil if every
// ID is unique — mirroring consensus.duplicateMethodIssue's identical
// "advisory, not blocking" duplicate-detection pattern.
func duplicateDriverIDIssue(drivers []Driver) *DriverIssue {
	seen := make(map[string]bool, len(drivers))
	for _, d := range drivers {
		if d.ID == "" {
			continue
		}
		if seen[d.ID] {
			return &DriverIssue{
				Code: IssueDuplicateDriverID, Severity: DriverSeverityWarning, DriverID: d.ID,
				Message: fmt.Sprintf("driver ID %q appears more than once in this scenario", d.ID),
			}
		}
		seen[d.ID] = true
	}
	return nil
}

// buildMethodDeltas produces one MethodDelta per methodOrder entry,
// comparing baselineRun and scenarioRun's headline figure for that
// method, and attaching whatever Linkages this scenario's drivers
// recorded for it.
func buildMethodDeltas(baselineRun, scenarioRun orchestrator.Run, linkagesByMethod map[valuation.Code][]Linkage) []MethodDelta {
	deltas := make([]MethodDelta, 0, len(methodOrder))
	for _, code := range methodOrder {
		baseVal := methodValue(baselineRun, code)
		scenVal := methodValue(scenarioRun, code)

		delta := MethodDelta{
			Method: code, Baseline: baseVal, Scenario: scenVal,
			Linkages: linkagesByMethod[code],
		}
		if baseVal.Available && scenVal.Available {
			delta.DeltaAvailable = true
			delta.ValueDelta = scenVal.Value - baseVal.Value
			delta.PercentDelta = percentOf(delta.ValueDelta, baseVal.Value)
		}
		deltas = append(deltas, delta)
	}
	return deltas
}

// methodValue extracts one method's MethodValue from run, mirroring
// valuation/report.headlineValue's per-method switch but returning this
// package's own JSON-safe MethodValue rather than report's private
// multi-return shape, and reporting Available: false (rather than
// omitting the method) when run has no outcome for code at all.
func methodValue(run orchestrator.Run, code valuation.Code) MethodValue {
	for _, m := range run.Methods {
		if m.Method != code {
			continue
		}
		switch {
		case m.SDE != nil:
			return MethodValue{Available: m.SDE.Available, ValueType: m.SDE.ValueType, Value: m.SDE.EquityValue}
		case m.EBITDA != nil:
			return MethodValue{Available: m.EBITDA.Available, ValueType: m.EBITDA.ValueType, Value: m.EBITDA.EnterpriseValue}
		case m.Capitalization != nil:
			return MethodValue{Available: m.Capitalization.Available, ValueType: m.Capitalization.ValueType, Value: m.Capitalization.EquityValue}
		case m.DCF != nil:
			return MethodValue{Available: m.DCF.Available, ValueType: m.DCF.ValueType, Value: m.DCF.EnterpriseValue}
		case m.NetAssets != nil:
			return MethodValue{Available: m.NetAssets.Available, ValueType: m.NetAssets.ValueType, Value: m.NetAssets.AdjustedNetAssetValue}
		default:
			return MethodValue{}
		}
	}
	return MethodValue{}
}

// consensusDelta compares two consensus.Results' SimpleMean, gated on
// both being Available — mirroring MethodDelta's identical
// availability-gated pattern.
func consensusDelta(baseline, scenario consensus.Result) (valueDelta, percentDelta float64, available bool) {
	if !baseline.Available || !scenario.Available {
		return 0, 0, false
	}
	valueDelta = scenario.Statistics.SimpleMean - baseline.Statistics.SimpleMean
	percentDelta = percentOf(valueDelta, baseline.Statistics.SimpleMean)
	return valueDelta, percentDelta, true
}

// driverAssumption renders one Driver as a single (label, value)
// Assumption line for ScenarioResult.Assumptions, using whichever params
// field driver.Type selects. A driver whose params were nil (invalid —
// already recorded as a DriverIssue by applyDriver) renders with an empty
// Value rather than panicking.
func driverAssumption(d Driver) Assumption {
	label := d.Label
	if label == "" {
		label = string(d.Type)
	}
	value := "n/a"
	switch d.Type {
	case DriverRevenueGrowth:
		if p := d.RevenueGrowth; p != nil {
			value = fmt.Sprintf("%.2f%% revenue growth", p.GrowthPercent*100)
		}
	case DriverMarginChange:
		if p := d.MarginChange; p != nil {
			value = fmt.Sprintf("%+.2f margin points on %.2f revenue base", p.MarginPointsDelta*100, p.RevenueBase)
		}
	case DriverSDEChange:
		if p := d.SDEChange; p != nil {
			value = fmt.Sprintf("amount %+.2f, percent %+.2f%%", p.AmountDelta, p.PercentDelta*100)
		}
	case DriverMultipleChange:
		if p := d.MultipleChange; p != nil {
			value = multipleChangeAssumption(p.ChangeDelta, p.NewValue)
		}
	case DriverCapRateChange:
		if p := d.CapRateChange; p != nil {
			value = multipleChangeAssumption(p.ChangeDelta, p.NewValue)
		}
	case DriverDiscountRateChange:
		if p := d.DiscountRateChange; p != nil {
			value = fmt.Sprintf("discount rate delta %+.4f/new %.4f, terminal growth delta %+.4f/new %.4f",
				p.DiscountRateDelta, p.DiscountRateNewValue, p.TerminalGrowthRateDelta, p.TerminalGrowthRateNewValue)
		}
	case DriverDebtChange:
		if p := d.DebtChange; p != nil {
			value = fmt.Sprintf("excess cash %+.2f, short-term debt %+.2f, long-term debt %+.2f, other debt %+.2f",
				p.ExcessCashDelta, p.ShortTermDebtDelta, p.LongTermDebtDelta, p.OtherDebtDelta)
		}
	case DriverWorkingCapitalChange:
		if p := d.WorkingCapitalChange; p != nil {
			value = fmt.Sprintf("%s %+.2f", p.Field, p.Amount)
		}
	case DriverOwnerCompensationAdjustment:
		if p := d.OwnerCompensationAdjustment; p != nil {
			value = fmt.Sprintf("%+.2f (applies beyond SDE: %v)", p.Amount, p.AppliesBeyondSDE)
		}
	case DriverCustomerLossImpact:
		if p := d.CustomerLossImpact; p != nil {
			value = fmt.Sprintf("earnings margin %.2f%% on revenue at risk", p.EarningsMarginOnLostRevenue*100)
		}
	case DriverMethodMultipleRule:
		if p := d.MethodMultipleRule; p != nil {
			value = fmt.Sprintf("%s multiple -> %.4fx", p.Method, p.NewMultiple)
		}
	}
	return Assumption{Label: label, Value: value}
}

func multipleChangeAssumption(delta, newValue float64) string {
	if newValue != 0 {
		return fmt.Sprintf("replaced with %.4f", newValue)
	}
	return fmt.Sprintf("delta %+.4f", delta)
}
