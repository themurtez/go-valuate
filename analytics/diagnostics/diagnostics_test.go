package diagnostics

import (
	"testing"

	"github.com/themurtez/go-valuate/analytics/anomalies"
	"github.com/themurtez/go-valuate/analytics/qoe"
	"github.com/themurtez/go-valuate/analytics/ratios"
	"github.com/themurtez/go-valuate/analytics/workingcapital"
	"github.com/themurtez/go-valuate/financial/metrics"
)

func findFinding(findings []Finding, code FindingCode) (Finding, bool) {
	for _, f := range findings {
		if f.Code == code {
			return f, true
		}
	}
	return Finding{}, false
}

func countBySeverity(findings []Finding, sev Severity) int {
	n := 0
	for _, f := range findings {
		if f.Severity == sev {
			n++
		}
	}
	return n
}

// TestCalculate_EmptyInput covers the degenerate zero-Input case: no
// module supplied at all. Expect Available == false and a single Errors
// entry, mirroring salereadiness.Result.Available's identical convention.
func TestCalculate_EmptyInput(t *testing.T) {
	res := Calculate(Input{})
	if res.Available {
		t.Fatal("expected Available == false for zero-value Input")
	}
	if len(res.Errors) != 1 || res.Errors[0].Code != IssueNoInputSupplied {
		t.Fatalf("expected exactly one IssueNoInputSupplied error, got %+v", res.Errors)
	}
	if res.FormulaVersion != FormulaVersion {
		t.Fatalf("expected FormulaVersion echoed even when unavailable, got %q", res.FormulaVersion)
	}
	if len(res.Findings) != 0 || res.OverallHealthScore != nil {
		t.Fatal("expected every other field zero-value when unavailable")
	}
}

// TestCalculate_Healthy covers a well-performing business (healthyFixture):
// expect Available, at least one Strength, zero critical Concerns, and a
// high OverallHealthScore.
func TestCalculate_Healthy(t *testing.T) {
	res := Calculate(healthyFixture())
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	if len(res.Strengths) == 0 {
		t.Fatal("expected at least one Strength for a healthy business")
	}
	if countBySeverity(res.Concerns, SeverityCritical) != 0 {
		t.Fatalf("expected zero critical Concerns for a healthy business, got %+v", res.Concerns)
	}
	if res.OverallHealthScore == nil {
		t.Fatal("expected OverallHealthScore to be present")
	}
	if res.OverallHealthScore.Value < 70 {
		t.Fatalf("expected a high health score for a healthy business, got %.1f", res.OverallHealthScore.Value)
	}
	if res.Coverage.AvailableModules != res.Coverage.TotalModules {
		t.Fatalf("expected full coverage (every module supplied), got %d/%d", res.Coverage.AvailableModules, res.Coverage.TotalModules)
	}
	if len(res.MissingDataAreas) != 0 {
		t.Fatalf("expected no missing data areas with full coverage, got %+v", res.MissingDataAreas)
	}
}

// TestCalculate_Stressed covers a business under financial stress
// (stressedFixture): expect Available, multiple critical Concerns across
// several categories, a low OverallHealthScore, and specific expected
// Finding codes present.
func TestCalculate_Stressed(t *testing.T) {
	res := Calculate(stressedFixture())
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	if got := countBySeverity(res.Concerns, SeverityCritical); got == 0 {
		t.Fatal("expected at least one critical Concern for a stressed business")
	}
	if res.OverallHealthScore == nil {
		t.Fatal("expected OverallHealthScore to be present")
	}
	if res.OverallHealthScore.Value > 40 {
		t.Fatalf("expected a low health score for a stressed business, got %.1f", res.OverallHealthScore.Value)
	}

	expectedCodes := []FindingCode{
		FindingMarginPressure,
		FindingRisingLeverage,
		FindingWeakeningLiquidity,
		FindingLargeNormalizationBurden,
		FindingWorkingCapitalVolatility,
		FindingWeakCashConversion,
		FindingLowCashRunway,
		FindingDecliningRecurringMix,
		FindingHighCustomerConcentration,
		FindingExpenseSpike,
		FindingMaterialUnfavorableVariance,
		FindingBelowMinimumDSCR,
		FindingCovenantBreach,
		FindingUnfavorableCostBenchmark,
		FindingHighDownsideValueSensitivity,
		FindingSaleReadinessBlocker,
		FindingSaleReadinessRisk,
	}
	for _, code := range expectedCodes {
		if _, ok := findFinding(res.Findings, code); !ok {
			t.Errorf("expected Finding code %s to be present for the stressed fixture", code)
		}
	}

	// Categories touched should span most of the fixed taxonomy for a
	// business this broadly stressed.
	seen := make(map[Category]bool)
	for _, f := range res.Findings {
		seen[f.Category] = true
	}
	if len(seen) < 8 {
		t.Fatalf("expected findings across at least 8 categories for a broadly stressed business, got %d: %+v", len(seen), seen)
	}
}

// TestCalculate_PartialInput covers a caller supplying only one module
// (Ratios) with everything else left at zero value: expect Available,
// Coverage reflecting exactly one available module, and MissingDataAreas
// covering the other fourteen.
func TestCalculate_PartialInput(t *testing.T) {
	in := Input{
		Ratios: ratios.Result{
			Available: true,
			Signals: []ratios.Signal{
				{Code: "MARGIN_COMPRESSION", Severity: "warning", Period: "FY2024", Message: "margin compression", Value: 0.10, Threshold: 0.15},
			},
		},
	}
	res := Calculate(in)
	if !res.Available {
		t.Fatalf("expected Available with one module supplied, errors=%+v", res.Errors)
	}
	if res.Coverage.AvailableModules != 1 || !res.Coverage.WithRatios {
		t.Fatalf("expected exactly one available module (ratios), got %+v", res.Coverage)
	}
	if len(res.MissingDataAreas) != res.Coverage.TotalModules-1 {
		t.Fatalf("expected %d missing data areas, got %d", res.Coverage.TotalModules-1, len(res.MissingDataAreas))
	}
	if len(res.Findings) != 1 {
		t.Fatalf("expected exactly one Finding from the single supplied signal, got %d", len(res.Findings))
	}
	if len(res.Warnings) != res.Coverage.TotalModules-1 {
		t.Fatalf("expected one IssueModuleUnavailable warning per missing module, got %d", len(res.Warnings))
	}
}

// TestCalculate_ContradictorySignals covers a case where two sibling
// modules disagree: analytics/ratios reports improving profitability
// (info/positive) for the same period analytics/qoe reports a critical
// earnings-quality flag. Expect both Findings to survive independently —
// this package never suppresses or reconciles a contradiction, only
// reports what each source said (see the package doc comment: this is not
// a narrative layer that resolves disagreement).
func TestCalculate_ContradictorySignals(t *testing.T) {
	in := Input{
		Ratios: ratios.Result{
			Available: true,
			Signals: []ratios.Signal{
				{Code: "IMPROVING_PROFITABILITY", Severity: "info", Period: "FY2024", Message: "EBITDA margin improved", Value: 0.25},
			},
		},
		QoE: qoe.Result{
			Available: true,
			Flags: []qoe.Flag{
				{Code: qoe.FlagNegativeOrNearZeroMaintainableEarnings, Severity: "critical", Period: "FY2024", Message: "maintainable earnings are negative", Value: -50_000},
			},
		},
	}
	res := Calculate(in)
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	if _, ok := findFinding(res.Strengths, FindingImprovingProfitability); !ok {
		t.Error("expected the positive ratios signal to still appear as a Strength")
	}
	if _, ok := findFinding(res.Concerns, FindingNegativeMaintainableEarnings); !ok {
		t.Error("expected the critical QoE flag to still appear as a Concern")
	}
	if len(res.Findings) != 2 {
		t.Fatalf("expected both contradictory findings to survive independently, got %d: %+v", len(res.Findings), res.Findings)
	}
}

// TestCalculate_UnrecognizedSignalCodeIgnored proves an unrecognized
// sibling Flag/Signal Code (e.g. from a future sibling package version
// this package's tables do not yet know about) is silently skipped rather
// than crashing or producing a malformed Finding.
func TestCalculate_UnrecognizedSignalCodeIgnored(t *testing.T) {
	in := Input{
		Ratios: ratios.Result{
			Available: true,
			Signals:   []ratios.Signal{{Code: "SOME_FUTURE_SIGNAL_CODE", Severity: "warning", Message: "unrecognized"}},
		},
	}
	res := Calculate(in)
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	if len(res.Findings) != 0 {
		t.Fatalf("expected zero findings for an unrecognized signal code, got %+v", res.Findings)
	}
}

// TestCalculate_MinCoveragePercentForScore proves Policy.MinCoveragePercentForScore
// gates OverallHealthScore when set and not met.
func TestCalculate_MinCoveragePercentForScore(t *testing.T) {
	in := Input{
		Ratios: ratios.Result{Available: true, Signals: []ratios.Signal{{Code: "MARGIN_COMPRESSION", Severity: "warning", Message: "x", Value: 0.1}}},
		Policy: Policy{MinCoveragePercentForScore: 0.5},
	}
	res := Calculate(in)
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	if res.OverallHealthScore != nil {
		t.Fatalf("expected OverallHealthScore to be nil below the coverage minimum, got %+v", res.OverallHealthScore)
	}
}

// TestCalculate_FindingsSortedDeterministically proves Result.Findings is
// always ordered by Category (categoryOrder), then Severity descending,
// regardless of the order mine* functions happened to append them in.
func TestCalculate_FindingsSortedDeterministically(t *testing.T) {
	res := Calculate(stressedFixture())
	for i := 1; i < len(res.Findings); i++ {
		prev, cur := res.Findings[i-1], res.Findings[i]
		prevRank, curRank := categoryRank[prev.Category], categoryRank[cur.Category]
		if prevRank > curRank {
			t.Fatalf("findings not sorted by category at index %d: %s (rank %d) before %s (rank %d)", i, prev.Category, prevRank, cur.Category, curRank)
		}
		if prevRank == curRank {
			if severityRank[prev.Severity] > severityRank[cur.Severity] {
				t.Fatalf("findings not sorted by severity within category at index %d: %s before %s", i, prev.Severity, cur.Severity)
			}
		}
	}
}

// TestCalculate_WorkingCapitalFallbackNoDoubleCount proves QoE's own
// EBITDA-volatility flag suppresses financial/metrics's fallback mining
// (mineMetrics) so the same underlying observation is never reported
// twice — see mineMetrics's doc comment.
func TestCalculate_MetricsFallbackSuppressedByQoE(t *testing.T) {
	in := healthyFixture()
	in.QoE = qoe.Result{Available: false}
	in.Metrics.Trend = nil
	res := Calculate(in)
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	// With QoE unavailable and no Metrics.Trend, mineMetrics should not
	// panic or contribute a spurious finding.
	if _, ok := findFinding(res.Findings, FindingEarningsVolatility); ok {
		t.Error("did not expect FindingEarningsVolatility with no Metrics.Trend and no QoE")
	}
}

// TestBuildCoverage_AllUnavailable proves buildCoverage reports zero
// AvailableModules and every module listed in MissingModules when every
// Input field is left at its zero value.
func TestBuildCoverage_AllUnavailable(t *testing.T) {
	c := buildCoverage(Input{})
	if c.AvailableModules != 0 {
		t.Fatalf("expected 0 available modules, got %d", c.AvailableModules)
	}
	if len(c.MissingModules) != c.TotalModules {
		t.Fatalf("expected every module listed as missing, got %d of %d", len(c.MissingModules), c.TotalModules)
	}
}

// TestMineWorkingCapital_RequiresAvailable proves mineWorkingCapital
// returns nil (not a panic) for an unavailable WorkingCapital Result, even
// with a non-zero-value Trend embedded (a caller populating Trend without
// setting Available is a legitimate zero-information state, per every
// sibling package's convention).
func TestMineWorkingCapital_RequiresAvailable(t *testing.T) {
	in := Input{WorkingCapital: workingcapital.Result{
		Trend: workingcapital.Trend{Direction: workingcapital.TrendIncreasing},
	}}
	if got := mineWorkingCapital(in); got != nil {
		t.Fatalf("expected nil findings for an unavailable WorkingCapital Result, got %+v", got)
	}
}

// TestMineAnomalies_UnrecognizedRuleCodeIgnored proves an unrecognized
// anomalies.RuleCode is skipped rather than producing a zero-value
// Finding.
func TestMineAnomalies_UnrecognizedRuleCodeIgnored(t *testing.T) {
	in := Input{Anomalies: anomalies.Result{
		Available: true,
		Anomalies: []anomalies.Anomaly{{Code: "SOME_FUTURE_RULE", Severity: "warning"}},
	}}
	if got := mineAnomalies(in); got != nil {
		t.Fatalf("expected nil findings for an unrecognized rule code, got %+v", got)
	}
}

// TestMineMetrics_FallbackFires proves mineMetrics reports
// FindingEarningsVolatility from financial/metrics.Trend when QoE is
// unavailable and EBITDAVolatility is at or above
// metricsEBITDAVolatilityThreshold.
func TestMineMetrics_FallbackFires(t *testing.T) {
	in := Input{
		Metrics: metrics.Result{
			Snapshots: []metrics.Snapshot{{Period: "FY2024"}},
			Trend: &metrics.Trend{
				EBITDAVolatility: metrics.VolatilityResult{Value: metrics.AvailableValue(0.40), SampleSize: 3},
			},
		},
	}
	got := mineMetrics(in)
	if len(got) != 1 {
		t.Fatalf("expected exactly one finding, got %d: %+v", len(got), got)
	}
	if got[0].Code != FindingEarningsVolatility {
		t.Fatalf("expected FindingEarningsVolatility, got %s", got[0].Code)
	}
	if !got[0].Metric.Available || got[0].Metric.Amount != 0.40 {
		t.Fatalf("expected Metric to echo the volatility figure, got %+v", got[0].Metric)
	}
}

// TestMineMetrics_BelowThresholdNoFinding proves mineMetrics reports
// nothing when EBITDAVolatility is below metricsEBITDAVolatilityThreshold.
func TestMineMetrics_BelowThresholdNoFinding(t *testing.T) {
	in := Input{
		Metrics: metrics.Result{
			Snapshots: []metrics.Snapshot{{Period: "FY2024"}},
			Trend: &metrics.Trend{
				EBITDAVolatility: metrics.VolatilityResult{Value: metrics.AvailableValue(0.05)},
			},
		},
	}
	if got := mineMetrics(in); got != nil {
		t.Fatalf("expected nil findings below threshold, got %+v", got)
	}
}

// TestHasErrors proves HasErrors distinguishes an Issue slice containing
// at least one IssueSeverityError from one that does not.
func TestHasErrors(t *testing.T) {
	if HasErrors(nil) {
		t.Error("HasErrors(nil) = true, want false")
	}
	if HasErrors([]Issue{{Severity: IssueSeverityWarning}}) {
		t.Error("HasErrors with only a warning = true, want false")
	}
	if !HasErrors([]Issue{{Severity: IssueSeverityWarning}, {Severity: IssueSeverityError}}) {
		t.Error("HasErrors with an error present = false, want true")
	}
}

// TestMineDebt_NoDebtServiceDistinctFromBreach is a regression test: a
// debt-free business's debt.FlagNoDebtService must not be mapped to
// FindingBelowMinimumDSCR (which every other producer means "coverage is
// below the required minimum," the opposite of "no debt at all").
func TestMineDebt_NoDebtServiceDistinctFromBreach(t *testing.T) {
	in := Input{Debt: debtResultNoDebtService()}
	got := mineDebt(in)
	if len(got) != 1 {
		t.Fatalf("expected exactly one finding, got %d: %+v", len(got), got)
	}
	if got[0].Code != FindingNoDebtService {
		t.Fatalf("expected FindingNoDebtService, got %s", got[0].Code)
	}
	if got[0].Code == FindingBelowMinimumDSCR {
		t.Fatal("FlagNoDebtService must never map to FindingBelowMinimumDSCR")
	}
}

// TestMineSaleReadiness_StrengthAndOpportunityCodesDistinct is a
// regression test: a Strength and an Opportunity must not share a
// FindingCode, since a caller grouping Result.Findings by Code should be
// able to tell them apart without inspecting which Result slice they came
// from.
func TestMineSaleReadiness_StrengthAndOpportunityCodesDistinct(t *testing.T) {
	in := healthyFixture()
	_, strengths, _ := mineSaleReadiness(in)
	if len(strengths) == 0 {
		t.Fatal("expected at least one sale-readiness Strength from healthyFixture")
	}
	for _, s := range strengths {
		if s.Code == FindingSaleReadinessOpportunity {
			t.Fatalf("sale-readiness Strength must not share FindingSaleReadinessOpportunity's code, got %+v", s)
		}
		if s.Code != FindingSaleReadinessStrength {
			t.Fatalf("expected FindingSaleReadinessStrength, got %s", s.Code)
		}
	}
}

// TestMineSaleReadiness_OpportunitySourceCodeIsOpportunityCode is a
// regression test: Finding.SourceCode for a mined Opportunity must echo
// salereadiness.Opportunity's own Code (OpportunityCode), not just its
// Dimension, so two different opportunity types under the same Dimension
// remain distinguishable.
func TestMineSaleReadiness_OpportunitySourceCodeIsOpportunityCode(t *testing.T) {
	in := Input{SaleReadiness: salereadinessResultWithOpportunity()}
	_, _, opportunities := mineSaleReadiness(in)
	if len(opportunities) != 1 {
		t.Fatalf("expected exactly one opportunity, got %d", len(opportunities))
	}
	if opportunities[0].SourceCode != "REDUCE_LEVERAGE" {
		t.Fatalf("expected SourceCode to echo the Opportunity's own Code, got %q", opportunities[0].SourceCode)
	}
}

// TestMineCovenants_CustomMetricLabel is a regression test: a custom
// covenant's MetricLabel must come from CustomMetricLabel, not the raw
// "CUSTOM" literal, when no separate Label was supplied.
func TestMineCovenants_CustomMetricLabel(t *testing.T) {
	in := Input{Covenants: covenantsResultCustomMetric()}
	got := mineCovenants(in)
	if len(got) != 1 {
		t.Fatalf("expected exactly one finding, got %d", len(got))
	}
	if got[0].MetricLabel != "Minimum Liquidity" {
		t.Fatalf("expected MetricLabel %q, got %q", "Minimum Liquidity", got[0].MetricLabel)
	}
}

// TestMineQoE_ZeroThresholdPreserved is a regression test:
// qoe.FlagDecliningEBITDADespiteRevenueGrowth's deliberate Threshold: 0
// must survive as an Available Comparison, not be erased by the
// zero-means-absent heuristic that applies to every other Flag code.
func TestMineQoE_ZeroThresholdPreserved(t *testing.T) {
	in := Input{QoE: qoeResultDecliningEBITDA()}
	got := mineQoE(in)
	if len(got) != 1 {
		t.Fatalf("expected exactly one finding, got %d", len(got))
	}
	if !got[0].Comparison.Available {
		t.Fatal("expected Comparison to be Available for a deliberate zero threshold")
	}
	if got[0].Comparison.Amount != 0 {
		t.Fatalf("expected Comparison amount 0, got %v", got[0].Comparison.Amount)
	}
}

// TestMineWorkingCapital_RisingNWCRequiresBothValuesAvailable is a
// regression test: the "rising net working capital" finding must not fire
// with fabricated 0.00 evidence when Trend reports Direction ==
// "increasing" but FirstValue/LastValue are not both Available.
func TestMineWorkingCapital_RisingNWCRequiresBothValuesAvailable(t *testing.T) {
	in := Input{WorkingCapital: workingcapitalResultIncreasingNoValues()}
	got := mineWorkingCapital(in)
	if len(got) != 0 {
		t.Fatalf("expected zero findings without both FirstValue/LastValue available, got %+v", got)
	}
}
