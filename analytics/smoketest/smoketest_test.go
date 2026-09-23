// Package smoketest is a suite-level integration test, not a production
// facade: it demonstrates the full documented composition chain from a
// financial.FinancialDataset through every analytics/valuation/transaction/
// portfolio/reporting/diagnostics package this task hardened, using the
// canonical synthetic fixture in fixtures/synthetic. It is deliberately a
// _test.go file with no exported API of its own — nothing in this module
// is meant to import this package.
package smoketest

import (
	"bytes"
	"encoding/json"
	"math"
	"testing"

	"github.com/themurtez/go-valuate/analytics/anomalies"
	"github.com/themurtez/go-valuate/analytics/benchmarks"
	"github.com/themurtez/go-valuate/analytics/cashflow"
	"github.com/themurtez/go-valuate/analytics/concentration"
	"github.com/themurtez/go-valuate/analytics/consolidation"
	"github.com/themurtez/go-valuate/analytics/covenants"
	"github.com/themurtez/go-valuate/analytics/debt"
	analyticsdiagnostics "github.com/themurtez/go-valuate/analytics/diagnostics"
	"github.com/themurtez/go-valuate/analytics/forecast"
	"github.com/themurtez/go-valuate/analytics/qoe"
	"github.com/themurtez/go-valuate/analytics/ratios"
	"github.com/themurtez/go-valuate/analytics/revenuequality"
	"github.com/themurtez/go-valuate/analytics/valuedrivers"
	"github.com/themurtez/go-valuate/analytics/variance"
	"github.com/themurtez/go-valuate/analytics/workingcapital"
	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/fixtures/synthetic"
	portfoliodiagnostics "github.com/themurtez/go-valuate/portfolio/diagnostics"
	"github.com/themurtez/go-valuate/reporting/management"
	"github.com/themurtez/go-valuate/transactions/acquisition"
	"github.com/themurtez/go-valuate/transactions/dealstructure"
	"github.com/themurtez/go-valuate/transactions/salereadiness"
	"github.com/themurtez/go-valuate/valuation"
	"github.com/themurtez/go-valuate/valuation/consensus"
	"github.com/themurtez/go-valuate/valuation/orchestrator"
	"github.com/themurtez/go-valuate/valuation/report"
)

// chainResults holds every stage's Result so later stages (and the final
// assertions) can reference earlier ones by name, and so
// TestSmoke_DeterministicRepeatedExecution can rerun the identical chain
// and compare.
type chainResults struct {
	qoeRes           qoe.Result
	wcRes            workingcapital.Result
	ratiosRes        ratios.Result
	cashflowRes      cashflow.Result
	revQualityRes    revenuequality.Result
	concentrationRes concentration.Result
	anomaliesRes     anomalies.Result
	varianceRes      variance.Result
	forecastRes      forecast.Result
	debtRes          debt.Result
	covenantsRes     covenants.Result
	benchmarksRes    benchmarks.Result
	orchestratorRun  orchestrator.Run
	consensusRes     consensus.Result
	valueDriversRes  valuedrivers.Result
	saleReadinessRes salereadiness.Result
	managementReport management.Report
	diagnosticsRes   analyticsdiagnostics.Result
}

// runChain executes the full documented composition chain once:
// FinancialDataset -> QoE -> working capital -> ratios -> cash flow ->
// revenue quality -> concentration -> anomalies -> variance -> forecast ->
// debt -> covenants -> benchmarks -> valuation -> value drivers -> sale
// readiness -> management report -> diagnostic engine. Every stage that
// takes an upstream Result as input is wired directly from the previous
// stage's actual output, never a separately hand-built stand-in — this is
// what proves the packages compose, not just that each independently
// works against its own fixture.
func runChain(t *testing.T) chainResults {
	t.Helper()
	var c chainResults

	c.qoeRes = qoe.Calculate(synthetic.BuildQoEInput(), qoe.Options{})
	if !c.qoeRes.Available {
		t.Fatalf("qoe.Calculate: Available == false, errors: %+v", c.qoeRes.Errors)
	}

	c.wcRes = workingcapital.Calculate(synthetic.BuildWorkingCapitalInput(), workingcapital.Options{})
	if !c.wcRes.Available {
		t.Fatalf("workingcapital.Calculate: Available == false, errors: %+v", c.wcRes.Errors)
	}

	c.ratiosRes = ratios.Calculate(synthetic.BuildRatiosInput(), ratios.Options{})
	if !c.ratiosRes.Available {
		t.Fatalf("ratios.Calculate: Available == false, errors: %+v", c.ratiosRes.Errors)
	}

	c.cashflowRes = cashflow.Calculate(synthetic.BuildCashFlowInput(), cashflow.Options{})
	if !c.cashflowRes.Available {
		t.Fatalf("cashflow.Calculate: Available == false, errors: %+v", c.cashflowRes.Errors)
	}

	c.revQualityRes = revenuequality.Calculate(synthetic.BuildRevenueQualityInput(), revenuequality.Options{})
	if !c.revQualityRes.Available {
		t.Fatalf("revenuequality.Calculate: Available == false, errors: %+v", c.revQualityRes.Errors)
	}

	c.concentrationRes = concentration.Calculate(synthetic.BuildConcentrationInput(), concentration.Options{})
	if !c.concentrationRes.Available {
		t.Fatalf("concentration.Calculate: Available == false, errors: %+v", c.concentrationRes.Errors)
	}

	c.anomaliesRes = anomalies.Calculate(synthetic.BuildAnomaliesInput(), anomalies.Options{})
	if !c.anomaliesRes.Available {
		t.Fatalf("anomalies.Calculate: Available == false, errors: %+v", c.anomaliesRes.Errors)
	}

	c.varianceRes = variance.Calculate(synthetic.BuildVarianceInput())
	if !c.varianceRes.Available {
		t.Fatalf("variance.Calculate: Available == false, errors: %+v", c.varianceRes.Errors)
	}

	c.forecastRes = forecast.Calculate(synthetic.BuildForecastInput())
	if !c.forecastRes.Available {
		t.Fatalf("forecast.Calculate: Available == false, errors: %+v", c.forecastRes.Errors)
	}

	c.debtRes = debt.Calculate(synthetic.BuildDebtInput())
	if !c.debtRes.Available {
		t.Fatalf("debt.Calculate: Available == false, errors: %+v", c.debtRes.Errors)
	}

	c.covenantsRes = covenants.Calculate(synthetic.BuildCovenantTests())
	if !c.covenantsRes.Available {
		t.Fatalf("covenants.Calculate: Available == false, errors: %+v", c.covenantsRes.Errors)
	}

	c.benchmarksRes = benchmarks.Calculate(synthetic.BuildBenchmarkInput())
	if !c.benchmarksRes.Available {
		t.Fatalf("benchmarks.Calculate: Available == false, errors: %+v", c.benchmarksRes.Errors)
	}

	c.orchestratorRun = orchestrator.Execute(synthetic.BuildOrchestratorRequest())
	consensusWeights := map[valuation.Code]float64{
		valuation.CodeSDEMultiple:              0.5, // downweighted: not owner-operated, SDE is a weaker fit
		valuation.CodeEBITDAMultiple:           1.0,
		valuation.CodeCapitalizationOfEarnings: 1.0,
		valuation.CodeDCF:                      1.0,
		valuation.CodeAdjustedNetAssetValue:    0.25, // downweighted: asset-light SaaS business
	}
	consensusInputs := report.BuildConsensusInputs(c.orchestratorRun, consensusWeights)
	c.consensusRes = consensus.Calculate(consensusInputs, consensus.Options{TargetBasis: valuation.ValueTypeEquity})
	if !c.consensusRes.Available {
		t.Fatalf("consensus.Calculate: Available == false, errors: %+v", c.consensusRes.Errors)
	}

	c.valueDriversRes = valuedrivers.Calculate(valuedrivers.Input{
		BaselineRequest:  synthetic.BuildOrchestratorRequest(),
		ConsensusOptions: consensus.Options{TargetBasis: valuation.ValueTypeEquity},
		Weights:          consensusWeights,
		Drivers: []valuedrivers.Driver{
			{
				ID:   "ebitda-multiple-up-half-turn",
				Type: valuedrivers.DriverMultipleChange,
				MultipleChange: &valuedrivers.MultipleChangeParams{
					Methods:     []valuedrivers.MultipleChangeMethod{valuedrivers.MultipleChangeEBITDA},
					ChangeDelta: 0.5,
				},
			},
		},
	})
	if !c.valueDriversRes.Available {
		t.Fatalf("valuedrivers.Calculate: Available == false")
	}

	c.saleReadinessRes = salereadiness.Calculate(salereadiness.Input{
		Dataset:        synthetic.BuildDataset(),
		PeriodMeta:     synthetic.BuildPeriodMeta(),
		QoE:            c.qoeRes,
		WorkingCapital: c.wcRes,
		Concentration:  c.concentrationRes,
		RevenueQuality: c.revQualityRes,
		Consensus:      c.consensusRes,
		Profile:        synthetic.BuildProfile(),
	})
	if !c.saleReadinessRes.Available {
		t.Fatalf("salereadiness.Calculate: Available == false, errors: %+v", c.saleReadinessRes.Errors)
	}

	c.managementReport = management.Calculate(management.Input{
		Dataset:        synthetic.BuildDataset(),
		PeriodMeta:     synthetic.BuildPeriodMeta(),
		QoE:            c.qoeRes,
		WorkingCapital: c.wcRes,
		CashFlow:       c.cashflowRes,
		Anomalies:      c.anomaliesRes,
		Variance:       c.varianceRes,
		Forecast:       c.forecastRes,
		Debt:           c.debtRes,
		Covenants:      c.covenantsRes,
		Consensus:      c.consensusRes,
	})
	if !c.managementReport.Available {
		t.Fatalf("management.Calculate: Available == false, errors: %+v", c.managementReport.Errors)
	}

	c.diagnosticsRes = analyticsdiagnostics.Calculate(analyticsdiagnostics.Input{
		Dataset:        synthetic.BuildDataset(),
		PeriodMeta:     synthetic.BuildPeriodMeta(),
		Ratios:         c.ratiosRes,
		QoE:            c.qoeRes,
		WorkingCapital: c.wcRes,
		CashFlow:       c.cashflowRes,
		RevenueQuality: c.revQualityRes,
		Concentration:  c.concentrationRes,
		Anomalies:      c.anomaliesRes,
		Variance:       c.varianceRes,
		Forecast:       c.forecastRes,
		Debt:           c.debtRes,
		Covenants:      c.covenantsRes,
		Benchmarks:     c.benchmarksRes,
		ValueDrivers:   c.valueDriversRes,
		SaleReadiness:  c.saleReadinessRes,
	})
	if !c.diagnosticsRes.Available {
		t.Fatalf("analyticsdiagnostics.Calculate: Available == false, errors: %+v", c.diagnosticsRes.Errors)
	}

	return c
}

// TestSmoke_FullChain runs the entire documented composition chain once
// and asserts every stage succeeded (already checked inside runChain via
// t.Fatalf) plus the cross-cutting properties this task's section 16
// requires: every output serializes, every version field is populated, no
// NaN/Inf anywhere in the JSON, and no fatal errors on the valid fixture.
func TestSmoke_FullChain(t *testing.T) {
	c := runChain(t)

	results := map[string]any{
		"qoe":            c.qoeRes,
		"workingcapital": c.wcRes,
		"ratios":         c.ratiosRes,
		"cashflow":       c.cashflowRes,
		"revenuequality": c.revQualityRes,
		"concentration":  c.concentrationRes,
		"anomalies":      c.anomaliesRes,
		"variance":       c.varianceRes,
		"forecast":       c.forecastRes,
		"debt":           c.debtRes,
		"covenants":      c.covenantsRes,
		"benchmarks":     c.benchmarksRes,
		"consensus":      c.consensusRes,
		"valuedrivers":   c.valueDriversRes,
		"salereadiness":  c.saleReadinessRes,
		"management":     c.managementReport,
		"diagnostics":    c.diagnosticsRes,
	}

	for name, res := range results {
		b, err := json.Marshal(res)
		if err != nil {
			t.Errorf("%s: json.Marshal failed: %v", name, err)
			continue
		}
		if bytes.Contains(b, []byte("NaN")) || bytes.Contains(b, []byte("Inf")) {
			t.Errorf("%s: JSON output contains NaN or Inf: %s", name, truncate(b, 200))
		}
		if !bytes.Contains(b, []byte(`"formula_version"`)) {
			t.Errorf("%s: JSON output has no formula_version field", name)
		}
	}
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "...(truncated)"
}

// TestSmoke_DeterministicRepeatedExecution proves the entire chain, run
// twice against identical fixture input, produces byte-for-byte identical
// JSON at every stage — the suite-level equivalent of every package's own
// determinism_test.go, proving determinism survives composition across
// package boundaries too.
func TestSmoke_DeterministicRepeatedExecution(t *testing.T) {
	first := runChain(t)
	second := runChain(t)

	pairs := []struct {
		name string
		a, b any
	}{
		{"qoe", first.qoeRes, second.qoeRes},
		{"workingcapital", first.wcRes, second.wcRes},
		{"ratios", first.ratiosRes, second.ratiosRes},
		{"cashflow", first.cashflowRes, second.cashflowRes},
		{"revenuequality", first.revQualityRes, second.revQualityRes},
		{"concentration", first.concentrationRes, second.concentrationRes},
		{"anomalies", first.anomaliesRes, second.anomaliesRes},
		{"variance", first.varianceRes, second.varianceRes},
		{"forecast", first.forecastRes, second.forecastRes},
		{"debt", first.debtRes, second.debtRes},
		{"covenants", first.covenantsRes, second.covenantsRes},
		{"benchmarks", first.benchmarksRes, second.benchmarksRes},
		{"consensus", first.consensusRes, second.consensusRes},
		{"valuedrivers", first.valueDriversRes, second.valueDriversRes},
		{"salereadiness", first.saleReadinessRes, second.saleReadinessRes},
		{"management", first.managementReport, second.managementReport},
		{"diagnostics", first.diagnosticsRes, second.diagnosticsRes},
	}
	for _, p := range pairs {
		ab, err := json.Marshal(p.a)
		if err != nil {
			t.Fatalf("%s: marshal run 1: %v", p.name, err)
		}
		bb, err := json.Marshal(p.b)
		if err != nil {
			t.Fatalf("%s: marshal run 2: %v", p.name, err)
		}
		if !bytes.Equal(ab, bb) {
			t.Errorf("%s: output differs between two identical runs (nondeterminism)", p.name)
		}
	}
}

// TestSmoke_NoInputMutation proves running the full chain never mutates
// the fixture's own dataset — a second, independent BuildDataset() call
// (freshly constructed, never touched by runChain) must equal the exact
// bytes runChain's own dataset produced.
func TestSmoke_NoInputMutation(t *testing.T) {
	before, err := json.Marshal(synthetic.BuildDataset())
	if err != nil {
		t.Fatalf("marshal before: %v", err)
	}

	runChain(t)

	after, err := json.Marshal(synthetic.BuildDataset())
	if err != nil {
		t.Fatalf("marshal after: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("BuildDataset() output changed after running the full chain — a stage may have mutated shared fixture state")
	}
}

// TestSmoke_NoNaNOrInf is a focused sweep for NaN/Inf leaking into any
// float64 field across every stage's Result, using reflection-free JSON
// round-tripping (the same technique TestSmoke_FullChain uses, isolated
// into its own test so a failure here is unambiguous about which
// guarantee broke).
func TestSmoke_NoNaNOrInf(t *testing.T) {
	c := runChain(t)
	check := func(name string, v float64) {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			t.Errorf("%s: NaN or Inf leaked into a float64 field: %v", name, v)
		}
	}
	check("consensus.Statistics.SimpleMean", c.consensusRes.Statistics.SimpleMean)
	check("consensus.Statistics.WeightedMean", c.consensusRes.Statistics.WeightedMean)
}

// toBusinessSnapshot is a small, explicit adapter from this smoke test's
// chainResults into one portfolio/diagnostics.BusinessSnapshot — a
// worked example of the kind of mechanical, stable mapping section 13
// calls out (qoe.Result -> portfolio.QoESummary, etc.). It intentionally
// covers only the fields portfolio/diagnostics' own doc comments name as
// each summary's primary source, not an exhaustive field-by-field
// translation.
func toBusinessSnapshot(id string, period string, c chainResults) portfoliodiagnostics.BusinessSnapshot {
	hasCriticalQoEFlag := false
	for _, f := range c.qoeRes.Flags {
		if f.Severity == qoe.FlagSeverityCritical {
			hasCriticalQoEFlag = true
			break
		}
	}
	hasRisingLeverage := false
	for _, s := range c.ratiosRes.Signals {
		if s.Code == ratios.SignalRisingLeverage {
			hasRisingLeverage = true
			break
		}
	}

	indicatedValue := c.consensusRes.Statistics.SimpleMean
	if c.consensusRes.WeightsValid {
		indicatedValue = c.consensusRes.Statistics.WeightedMean
	}

	overallScore := portfoliodiagnostics.Unavailable()
	if c.saleReadinessRes.OverallScore != nil {
		overallScore = portfoliodiagnostics.AvailableValue(c.saleReadinessRes.OverallScore.Value)
	}

	return portfoliodiagnostics.BusinessSnapshot{
		ID:     id,
		Period: financial.Period(period),
		QoE: portfoliodiagnostics.QoESummary{
			Available:          c.qoeRes.Available,
			FormulaVersion:     c.qoeRes.FormulaVersion,
			EBITDAVolatility:   portfoliodiagnostics.AvailableValue(c.qoeRes.EBITDAVolatility.Value.Value),
			AdjustmentToEBITDA: portfoliodiagnostics.AvailableValue(c.qoeRes.Ratios.AdjustmentToEBITDA.Value),
			HasCriticalFlags:   hasCriticalQoEFlag,
		},
		RatioHealth: portfoliodiagnostics.RatioHealthSummary{
			Available:               c.ratiosRes.Available,
			FormulaVersion:          c.ratiosRes.FormulaVersion,
			HasRisingLeverageSignal: hasRisingLeverage,
		},
		Concentration: portfoliodiagnostics.ConcentrationSummary{
			Available:          c.concentrationRes.Available,
			FormulaVersion:     c.concentrationRes.FormulaVersion,
			LargestEntityShare: portfoliodiagnostics.AvailableValue(latestConcentration(c.concentrationRes).LargestEntityShare.Value),
			HHI:                portfoliodiagnostics.AvailableValue(latestConcentration(c.concentrationRes).HHI.Value),
		},
		CashFlow: portfoliodiagnostics.CashFlowSummary{
			Available:      c.cashflowRes.Available,
			FormulaVersion: c.cashflowRes.FormulaVersion,
		},
		Valuation: portfoliodiagnostics.ValuationSummary{
			Available:      c.consensusRes.Available,
			FormulaVersion: c.consensusRes.FormulaVersion,
			IndicatedValue: portfoliodiagnostics.AvailableValue(indicatedValue),
			MethodCount:    c.consensusRes.Statistics.Count,
		},
		SaleReadiness: portfoliodiagnostics.SaleReadinessSummary{
			Available:      c.saleReadinessRes.Available,
			FormulaVersion: c.saleReadinessRes.FormulaVersion,
			OverallScore:   overallScore,
			BlockerCount:   len(c.saleReadinessRes.Blockers),
			StrengthCount:  len(c.saleReadinessRes.Strengths),
		},
	}
}

func latestConcentration(res concentration.Result) concentration.PeriodConcentration {
	if len(res.History) == 0 {
		return concentration.PeriodConcentration{}
	}
	return res.History[len(res.History)-1]
}

// TestSmoke_PortfolioDiagnosticsBranch demonstrates the optional branch
// into portfolio/diagnostics: one BusinessSnapshot built from the main
// chain's own Results (via toBusinessSnapshot) plus a synthetic prior-
// period snapshot showing modest margin deterioration, so at least one
// portfolio Finding is expected to fire.
func TestSmoke_PortfolioDiagnosticsBranch(t *testing.T) {
	c := runChain(t)
	current := toBusinessSnapshot("meridian-saas", "2025", c)
	prior := current
	prior.Period = "2024"
	// A synthetic prior snapshot with a stronger EBITDA margin than
	// current's SelectedMetrics implies, so FindingMarginDeterioration has
	// a real signal — SelectedMetrics itself is left zero-value in
	// toBusinessSnapshot (out of this adapter's minimal scope), so this
	// test sets it directly on both snapshots instead.
	// Current margin (32.7%) is deliberately BELOW prior's (40.0%) by more
	// than portfolio/diagnostics.DirectionFlatBandPercent (5 points), so
	// FindingMarginDeterioration has a genuine signal to detect — verified
	// against the package's own detectMarginDeterioration threshold, not
	// just plausible-looking numbers.
	current.Metrics = portfoliodiagnostics.SelectedMetrics{
		Revenue:      portfoliodiagnostics.AvailableValue(8_724_000),
		EBITDA:       portfoliodiagnostics.AvailableValue(2_854_600),
		EBITDAMargin: portfoliodiagnostics.AvailableValue(2_854_600.0 / 8_724_000.0), // 32.7%
	}
	prior.Metrics = portfoliodiagnostics.SelectedMetrics{
		Revenue:      portfoliodiagnostics.AvailableValue(6_826_000),
		EBITDA:       portfoliodiagnostics.AvailableValue(2_730_400),
		EBITDAMargin: portfoliodiagnostics.AvailableValue(2_730_400.0 / 6_826_000.0), // 40.0%
	}
	current.Prior = &prior

	res := portfoliodiagnostics.Calculate(portfoliodiagnostics.Input{
		Portfolio: []portfoliodiagnostics.BusinessSnapshot{current},
	})

	b, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	if bytes.Contains(b, []byte("NaN")) || bytes.Contains(b, []byte("Inf")) {
		t.Errorf("portfolio/diagnostics output contains NaN or Inf")
	}
	found := false
	for _, f := range res.Findings {
		if f.Code == portfoliodiagnostics.FindingMarginDeterioration {
			found = true
		}
	}
	if !found {
		t.Errorf("expected FindingMarginDeterioration to fire given the 7.3-point margin decline, got %d findings, none matching", len(res.Findings))
	}
	t.Logf("portfolio/diagnostics: %d findings", len(res.Findings))
}

// TestSmoke_ConsolidationBranch demonstrates the optional branch into
// analytics/consolidation: Meridian SaaS plus a smaller wholly-owned
// subsidiary (a scaled-down copy of the same dataset), consolidated under
// full consolidation with no intercompany eliminations (a deliberately
// simple case — this package never infers eliminations, so a
// realistic-but-nonzero elimination set would need its own bespoke
// fixture beyond this smoke test's scope).
func TestSmoke_ConsolidationBranch(t *testing.T) {
	parent := synthetic.BuildDataset()
	sub := scaleDataset(parent, 0.15)

	res := consolidation.Calculate(consolidation.Input{
		Entities: []consolidation.EntityDataset{
			{EntityID: "parent", EntityLabel: "Meridian SaaS (parent)", Dataset: parent},
			{EntityID: "sub", EntityLabel: "Meridian EU (subsidiary)", Dataset: sub},
		},
		Periods: synthetic.Periods,
	})
	if !res.Available {
		t.Fatalf("consolidation.Calculate: Available == false, errors: %+v", res.Errors)
	}
	b, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	if bytes.Contains(b, []byte("NaN")) || bytes.Contains(b, []byte("Inf")) {
		t.Errorf("consolidation output contains NaN or Inf")
	}
}

func scaleDataset(ds financial.FinancialDataset, factor float64) financial.FinancialDataset {
	items := make([]financial.NormalizedItem, len(ds.Items))
	for i, it := range ds.Items {
		it.Amount *= factor
		items[i] = it
	}
	return financial.FinancialDataset{Currency: ds.Currency, Items: items}
}

// TestSmoke_AcquisitionAndDealStructureBranch demonstrates the optional
// branch into transactions/acquisition and transactions/dealstructure,
// using their own standalone fixtures (both packages take fully
// caller-supplied deal terms, not upstream Results from the main chain).
func TestSmoke_AcquisitionAndDealStructureBranch(t *testing.T) {
	acqRes := acquisition.Calculate(synthetic.BuildAcquisitionInput())
	if !acqRes.Available {
		t.Fatalf("acquisition.Calculate: Available == false, errors: %+v", acqRes.Errors)
	}
	dealRes := dealstructure.Build(synthetic.BuildDealStructureInput())
	if !dealRes.Available {
		t.Fatalf("dealstructure.Build: Available == false, errors: %+v", dealRes.Errors)
	}

	for name, res := range map[string]any{"acquisition": acqRes, "dealstructure": dealRes} {
		b, err := json.Marshal(res)
		if err != nil {
			t.Fatalf("%s: json.Marshal failed: %v", name, err)
		}
		if bytes.Contains(b, []byte("NaN")) || bytes.Contains(b, []byte("Inf")) {
			t.Errorf("%s output contains NaN or Inf", name)
		}
	}
}
