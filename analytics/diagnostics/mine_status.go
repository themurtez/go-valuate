package diagnostics

import "fmt"

// workingCapitalVolatilityThreshold is the fixed NWC-as-percent-of-revenue
// volatility decimal value at or above which mineWorkingCapital reports
// FindingWorkingCapitalVolatility — part of FormulaVersion, mirroring
// salereadiness.classifyWorkingCapitalStability's identical figure choice
// for the same underlying statistic, since no Policy-configurable
// threshold exists in this package's own small Policy (see Policy's doc
// comment on why this package mines pre-classified signals rather than
// re-deriving thresholds its sibling packages already own — this is the
// one case where a sibling supplies a raw statistic, not a classification,
// so a fixed threshold is unavoidable to turn it into a Finding at all).
const workingCapitalVolatilityThreshold = 0.20

// mineWorkingCapital mines Input.WorkingCapital into Findings.
// analytics/workingcapital has no Flag/Signal of its own (see the package
// survey) — its Trend.Direction and NWCPercentOfRevenueStatistics.Volatility
// are the only pre-computed classification/statistic available to mine.
func mineWorkingCapital(in Input) []Finding {
	if !in.WorkingCapital.Available {
		return nil
	}
	var out []Finding

	if t := in.WorkingCapital.Trend; t.Direction == "increasing" && t.FirstValue.Available && t.LastValue.Available {
		out = append(out, Finding{
			Code:              FindingWorkingCapitalVolatility,
			Category:          CategoryWorkingCapital,
			Severity:          SeverityWarning,
			Title:             "Rising net working capital",
			Evidence:          []string{fmt.Sprintf("net working capital rose from %.2f to %.2f", t.FirstValue.Value, t.LastValue.Value)},
			SourceModule:      SourceWorkingCapital,
			SourceCode:        "increasing",
			MetricLabel:       "NWC",
			Metric:            Value{Available: t.LastValue.Available, Amount: t.LastValue.Value},
			ComparisonLabel:   "PRIOR_PERIOD",
			Comparison:        Value{Available: t.FirstValue.Available, Amount: t.FirstValue.Value},
			Period:            t.LastPeriod,
			Explanation:       fmt.Sprintf("net working capital increased from %.2f (%s) to %.2f (%s)", t.FirstValue.Value, t.FirstPeriod, t.LastValue.Value, t.LastPeriod),
			RecommendedAction: "Review the drivers of the rising net-working-capital balance.",
		})
	}

	vol := in.WorkingCapital.NWCPercentOfRevenueStatistics.Volatility
	if vol.Available && vol.Value >= workingCapitalVolatilityThreshold {
		out = append(out, Finding{
			Code:              FindingWorkingCapitalVolatility,
			Category:          CategoryWorkingCapital,
			Severity:          SeverityWarning,
			Title:             "Volatile working-capital-to-revenue ratio",
			Evidence:          []string{fmt.Sprintf("NWC-as-percent-of-revenue volatility of %.1f%%", vol.Value*100)},
			SourceModule:      SourceWorkingCapital,
			SourceCode:        "NWC_PERCENT_OF_REVENUE_VOLATILITY",
			MetricLabel:       "NWC_PERCENT_OF_REVENUE_VOLATILITY",
			Metric:            AvailableValue(vol.Value),
			ComparisonLabel:   "THRESHOLD",
			Comparison:        AvailableValue(workingCapitalVolatilityThreshold),
			Explanation:       fmt.Sprintf("NWC-as-percent-of-revenue volatility of %.1f%% is at or above the %.1f%% reference level", vol.Value*100, workingCapitalVolatilityThreshold*100),
			RecommendedAction: "Review the sources of period-to-period working-capital volatility relative to revenue.",
		})
	}

	return out
}

// mineVariance mines Input.Variance.MaterialExceptions into Findings —
// analytics/variance has no Flag of its own; MaterialExceptions is the
// package's own "worth a reviewer's attention" rollup (Materiality ==
// material AND Favorability == unfavorable, per variance.Result's doc
// comment), the closest existing analogue to a Flag list.
func mineVariance(in Input) []Finding {
	if !in.Variance.Available {
		return nil
	}
	var out []Finding
	for _, e := range in.Variance.MaterialExceptions {
		explanation := fmt.Sprintf("%s: actual %.2f vs. baseline %.2f (variance %.2f)",
			lineLabel(string(e.AccountCode), e.Label), e.Actual, e.Baseline, e.AbsoluteVariance.Value)
		out = append(out, Finding{
			Code:              FindingMaterialUnfavorableVariance,
			Category:          CategoryOperationalCostControl,
			Severity:          SeverityWarning,
			Title:             "Material unfavorable variance",
			Evidence:          []string{explanation},
			SourceModule:      SourceVariance,
			SourceCode:        string(e.AccountCode),
			MetricLabel:       lineLabel(string(e.AccountCode), e.Label),
			Metric:            AvailableValue(e.Actual),
			ComparisonLabel:   "BASELINE",
			Comparison:        Value{Available: e.BaselineAvailable, Amount: e.Baseline},
			Period:            e.Period,
			Explanation:       explanation,
			RecommendedAction: "Review the driver of this unfavorable variance versus its baseline.",
		})
	}
	return out
}

func lineLabel(code, label string) string {
	if label != "" {
		return label
	}
	return code
}

// covenantMetricLabel names a CovenantTest's Metric for display: its own
// CustomMetricLabel when Metric == MetricCustom (the one case
// covenants.TestResult documents CustomMetricLabel as naming Metric
// specifically — see covenants.CovenantTest.CustomMetricLabel's doc
// comment), else the test's Label, else the raw Metric string.
func covenantMetricLabel(metric, customMetricLabel, label string) string {
	if metric == "CUSTOM" && customMetricLabel != "" {
		return customMetricLabel
	}
	return lineLabel(metric, label)
}

// mineCovenants mines Input.Covenants.Tests into Findings for every
// StatusFail (breach) and WarningBufferWithinBuffer (near breach).
func mineCovenants(in Input) []Finding {
	if !in.Covenants.Available {
		return nil
	}
	var out []Finding
	for _, t := range in.Covenants.Tests {
		metricLabel := covenantMetricLabel(string(t.Metric), t.CustomMetricLabel, t.Label)
		switch {
		case t.Status == "fail":
			out = append(out, Finding{
				Code:              FindingCovenantBreach,
				Category:          CategoryLeverage,
				Severity:          SeverityCritical,
				Title:             "Covenant breach",
				Evidence:          []string{t.Explanation},
				SourceModule:      SourceCovenants,
				SourceCode:        t.CovenantID,
				MetricLabel:       metricLabel,
				Metric:            Value{Available: t.Actual.Available, Amount: t.Actual.Amount},
				ComparisonLabel:   "THRESHOLD",
				Comparison:        AvailableValue(t.Threshold),
				Period:            t.Period,
				Explanation:       t.Explanation,
				RecommendedAction: "Review the breached covenant and any applicable cure/grace provisions with counsel.",
			})
		case t.WarningBufferStatus == "within_buffer":
			out = append(out, Finding{
				Code:              FindingCovenantNearBreach,
				Category:          CategoryLeverage,
				Severity:          SeverityWarning,
				Title:             "Covenant near breach",
				Evidence:          []string{t.Explanation},
				SourceModule:      SourceCovenants,
				SourceCode:        t.CovenantID,
				MetricLabel:       metricLabel,
				Metric:            Value{Available: t.Actual.Available, Amount: t.Actual.Amount},
				ComparisonLabel:   "THRESHOLD",
				Comparison:        AvailableValue(t.Threshold),
				Period:            t.Period,
				Explanation:       t.Explanation,
				RecommendedAction: "Review this covenant's headroom against its warning buffer.",
			})
		}
	}
	return out
}

// mineBenchmarks mines Input.Benchmarks.Comparisons into Findings for
// every Favorable == "UNFAVORABLE" comparison.
func mineBenchmarks(in Input) []Finding {
	if !in.Benchmarks.Available {
		return nil
	}
	var out []Finding
	for _, c := range in.Benchmarks.Comparisons {
		if c.Favorable != "UNFAVORABLE" {
			continue
		}
		out = append(out, Finding{
			Code:              FindingUnfavorableCostBenchmark,
			Category:          CategoryOperationalCostControl,
			Severity:          SeverityWarning,
			Title:             "Unfavorable benchmark comparison",
			Evidence:          []string{fmt.Sprintf("%s is unfavorable versus the benchmark median", lineLabel(c.MetricID, c.Label))},
			SourceModule:      SourceBenchmarks,
			SourceCode:        c.MetricID,
			MetricLabel:       lineLabel(c.MetricID, c.Label),
			Metric:            Value{Available: c.CompanyValue.Available, Amount: c.CompanyValue.Amount},
			ComparisonLabel:   "BENCHMARK_MEDIAN",
			Comparison:        Value{Available: c.BenchmarkMedian.Available, Amount: c.BenchmarkMedian.Amount},
			Period:            c.Period,
			Explanation:       fmt.Sprintf("%s is classified unfavorable relative to the %s benchmark median", lineLabel(c.MetricID, c.Label), c.Source.Name),
			RecommendedAction: "Review this metric's gap versus the benchmark population and its underlying drivers.",
		})
	}
	return out
}

// valueDriversDownsideSensitivityThreshold is the fixed
// ConsensusPercentDelta magnitude (as a decimal) at or beyond which
// mineValueDrivers reports FindingHighDownsideValueSensitivity for a
// scenario — part of FormulaVersion. analytics/valuedrivers computes no
// severity classification of its own (see the package survey: it is a
// what-if/scenario-delta engine with no Flag/threshold concept), so this
// package applies its own fixed bar to decide which scenario deltas are
// large enough to surface as a Finding at all.
const valueDriversDownsideSensitivityThreshold = 0.15

// mineValueDrivers mines Input.ValueDrivers.Scenarios for any scenario
// whose ConsensusPercentDelta is a large enough decline to indicate high
// downside sensitivity. Only Scenarios (caller-named, typically
// "downside"/"stress" cases) are mined, not OneFactorAtATime (individual
// driver sensitivities are exploratory by design, not diagnostic
// findings).
func mineValueDrivers(in Input) []Finding {
	if !in.ValueDrivers.Available {
		return nil
	}
	var out []Finding
	for _, s := range in.ValueDrivers.Scenarios {
		if !s.Available || !s.ConsensusDeltaAvailable {
			continue
		}
		if s.ConsensusPercentDelta > -valueDriversDownsideSensitivityThreshold {
			continue
		}
		out = append(out, Finding{
			Code:              FindingHighDownsideValueSensitivity,
			Category:          CategoryValuation,
			Severity:          SeverityWarning,
			Title:             "High downside value sensitivity",
			Evidence:          []string{fmt.Sprintf("scenario %q reduces consensus value by %.1f%%", lineLabel(s.ScenarioID, s.Label), -s.ConsensusPercentDelta*100)},
			SourceModule:      SourceValueDrivers,
			SourceCode:        s.ScenarioID,
			MetricLabel:       "CONSENSUS_VALUE",
			Metric:            AvailableValue(s.Consensus.Statistics.SimpleMean),
			ComparisonLabel:   "BASELINE_CONSENSUS_VALUE",
			Comparison:        AvailableValue(in.ValueDrivers.Baseline.Consensus.Statistics.SimpleMean),
			Explanation:       fmt.Sprintf("under scenario %q, consensus value declines %.1f%% versus the baseline", lineLabel(s.ScenarioID, s.Label), -s.ConsensusPercentDelta*100),
			RecommendedAction: "Review the assumptions behind this downside scenario and the resulting valuation sensitivity.",
		})
	}
	return out
}
