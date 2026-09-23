package diagnostics

import "fmt"

// metricsEBITDAVolatilityThreshold mirrors
// salereadiness.classifyEarningsStability's own fixed reference point for
// the same underlying statistic (financial/metrics.Trend.EBITDAVolatility)
// — part of FormulaVersion. Only applied as a fallback when QoE is
// unavailable (QoE.Flags already carries FlagVolatileEarnings from a
// richer, normalized-earnings-based computation — see mineQoE); mining the
// same underlying volatility statistic from both sources when both are
// available would double-report one observation as two Findings.
const metricsEBITDAVolatilityThreshold = 0.25

// mineMetrics mines Input.Metrics.Trend into Findings. financial/metrics
// has no Flag/Signal of its own (see the package survey) — Trend is the
// only pre-computed statistic available to mine, and only as a fallback
// when the richer QoE/Ratios sources are unavailable.
func mineMetrics(in Input) []Finding {
	if len(in.Metrics.Snapshots) == 0 || in.Metrics.Trend == nil || in.Metrics.Trend.Error != nil {
		return nil
	}
	if in.QoE.Available {
		return nil
	}

	var out []Finding
	vol := in.Metrics.Trend.EBITDAVolatility.Value
	if vol.Available && vol.Value >= metricsEBITDAVolatilityThreshold {
		out = append(out, Finding{
			Code:              FindingEarningsVolatility,
			Category:          CategoryProfitability,
			Severity:          SeverityWarning,
			Title:             "Volatile EBITDA history",
			Evidence:          []string{fmt.Sprintf("EBITDA volatility of %.1f%% across %d fiscal years", vol.Value*100, in.Metrics.Trend.EBITDAVolatility.SampleSize)},
			SourceModule:      SourceMetrics,
			SourceCode:        "EBITDA_VOLATILITY",
			MetricLabel:       "EBITDA_VOLATILITY",
			Metric:            AvailableValue(vol.Value),
			ComparisonLabel:   "REFERENCE_LEVEL",
			Comparison:        AvailableValue(metricsEBITDAVolatilityThreshold),
			Explanation:       fmt.Sprintf("EBITDA volatility of %.1f%% is at or above the %.1f%% reference level", vol.Value*100, metricsEBITDAVolatilityThreshold*100),
			RecommendedAction: "Review the sources of period-to-period EBITDA volatility.",
		})
	}
	return out
}
