package report

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/adjustments"
	"github.com/themurtez/go-valuate/financial/metrics"
	"github.com/themurtez/go-valuate/settings"
	"github.com/themurtez/go-valuate/valuation"
	"github.com/themurtez/go-valuate/valuation/capitalization"
	"github.com/themurtez/go-valuate/valuation/consensus"
	"github.com/themurtez/go-valuate/valuation/dcf"
	"github.com/themurtez/go-valuate/valuation/ebitda"
	"github.com/themurtez/go-valuate/valuation/netassets"
	"github.com/themurtez/go-valuate/valuation/orchestrator"
	"github.com/themurtez/go-valuate/valuation/sde"
	"github.com/themurtez/go-valuate/valuation/sensitivity"
)

func sampleRun(t *testing.T) orchestrator.Run {
	t.Helper()
	req := orchestrator.Request{
		SDE: &sde.Input{MaintainableSDE: 300_000, Multiple: 2.5},
		EBITDA: &ebitda.Input{
			MaintainableEBITDA: 400_000, Multiple: 3.5,
			EquityBridge: ebitda.EquityBridgeInput{Requested: true, ExcessCash: 50_000, ShortTermDebt: 20_000, LongTermDebt: 30_000},
		},
		Capitalization: &capitalization.Input{MaintainableEarnings: 300_000, CapitalizationRate: 0.25},
		DCF: &dcf.Input{
			ForecastPeriods: []dcf.ForecastPeriod{
				{Period: "2026", FreeCashFlow: 100_000},
				{Period: "2027", FreeCashFlow: 110_000},
			},
			DiscountRate:       0.18,
			TerminalGrowthRate: 0.03,
			EquityBridge:       dcf.EquityBridgeInput{Requested: true, ExcessCash: 40_000, ShortTermDebt: 10_000, LongTermDebt: 15_000},
		},
	}
	return orchestrator.Execute(req)
}

func TestBuild_MethodsPopulatedFromRun(t *testing.T) {
	run := sampleRun(t)
	rep := Build(BuildInput{Run: &run})

	if len(rep.Methods) != 5 {
		t.Fatalf("expected 5 method rows, got %d", len(rep.Methods))
	}
	// Fixed order: SDE, EBITDA, Capitalization, DCF, NetAssets.
	want := []valuation.Code{
		valuation.CodeSDEMultiple, valuation.CodeEBITDAMultiple,
		valuation.CodeCapitalizationOfEarnings, valuation.CodeDCF, valuation.CodeAdjustedNetAssetValue,
	}
	for i, code := range want {
		if rep.Methods[i].Method != code {
			t.Errorf("Methods[%d] = %v, want %v", i, rep.Methods[i].Method, code)
		}
	}

	netAssetsRow := rep.Methods[4]
	if netAssetsRow.Included {
		t.Error("NetAssets was never supplied an Input; expected Included = false")
	}
	if netAssetsRow.ExclusionReason != string(orchestrator.ExclusionNoInput) {
		t.Errorf("ExclusionReason = %q, want %q", netAssetsRow.ExclusionReason, orchestrator.ExclusionNoInput)
	}

	sdeRow := rep.Methods[0]
	if !sdeRow.Included || sdeRow.Value != 750_000 {
		t.Errorf("SDE row = %+v, want Included=true Value=750000", sdeRow)
	}
	if len(sdeRow.Assumptions) == 0 {
		t.Error("expected SDE row to carry Assumptions")
	}
	if len(sdeRow.Steps) == 0 {
		t.Error("expected SDE row to carry calculation Steps")
	}
}

func TestBuild_SummaryFromConsensus(t *testing.T) {
	run := sampleRun(t)
	inputs := BuildConsensusInputs(run, map[valuation.Code]float64{
		valuation.CodeSDEMultiple:              1,
		valuation.CodeEBITDAMultiple:           1,
		valuation.CodeCapitalizationOfEarnings: 1,
		valuation.CodeDCF:                      1,
	})
	c := consensus.Calculate(inputs, consensus.Options{TargetBasis: valuation.ValueTypeEquity})
	rep := Build(BuildInput{Run: &run, Consensus: &c, ValuationDate: "2026-09-22"})

	if !rep.Summary.ConsensusAvailable {
		t.Fatal("expected ConsensusAvailable = true")
	}
	if rep.Summary.SimpleConsensus != c.Statistics.SimpleMean {
		t.Errorf("SimpleConsensus = %v, want %v", rep.Summary.SimpleConsensus, c.Statistics.SimpleMean)
	}
	if rep.Summary.Median != c.Statistics.Median {
		t.Errorf("Median = %v, want %v", rep.Summary.Median, c.Statistics.Median)
	}
	if rep.Summary.MethodRange != c.Range {
		t.Errorf("MethodRange = %+v, want %+v", rep.Summary.MethodRange, c.Range)
	}
	if rep.Summary.ValuationDate != "2026-09-22" {
		t.Errorf("ValuationDate = %q, want 2026-09-22", rep.Summary.ValuationDate)
	}
	if rep.Summary.IncludedMethodCount != 4 {
		t.Errorf("IncludedMethodCount = %d, want 4", rep.Summary.IncludedMethodCount)
	}
}

func TestBuild_NoConsensus_SummaryUnavailable(t *testing.T) {
	run := sampleRun(t)
	rep := Build(BuildInput{Run: &run})
	if rep.Summary.ConsensusAvailable {
		t.Error("expected ConsensusAvailable = false when no Consensus supplied")
	}
}

func TestBuild_FinancialSummaryFromSnapshots(t *testing.T) {
	snapshots := []metrics.Snapshot{
		{
			Period:       financial.Period("2024"),
			TotalRevenue: metrics.AvailableValue(1_000_000),
			EBITDA:       metrics.AvailableValue(200_000),
			EBITDAMargin: metrics.AvailableValue(0.2),
			SDE:          metrics.AvailableValue(250_000),
		},
		{
			Period:       financial.Period("2025"),
			TotalRevenue: metrics.AvailableValue(1_200_000),
			EBITDA:       metrics.AvailableValue(260_000),
			EBITDAMargin: metrics.AvailableValue(0.2167),
			SDE:          metrics.AvailableValue(310_000),
		},
	}
	rep := Build(BuildInput{Snapshots: snapshots})

	if len(rep.Financial.Periods) != 2 {
		t.Fatalf("expected 2 periods, got %d", len(rep.Financial.Periods))
	}
	if rep.Financial.Periods[0].Revenue.Value != 1_000_000 {
		t.Errorf("Periods[0].Revenue = %v, want 1000000", rep.Financial.Periods[0].Revenue.Value)
	}
	if !rep.Financial.Periods[1].EBITDA.Available || rep.Financial.Periods[1].EBITDA.Value != 260_000 {
		t.Errorf("Periods[1].EBITDA = %+v, want Available=true Value=260000", rep.Financial.Periods[1].EBITDA)
	}
}

func TestBuild_ChartSeries(t *testing.T) {
	run := sampleRun(t)
	snapshots := []metrics.Snapshot{
		{Period: "2024", TotalRevenue: metrics.AvailableValue(1_000_000), EBITDA: metrics.AvailableValue(200_000), SDE: metrics.AvailableValue(220_000), EBITDAMargin: metrics.AvailableValue(0.2)},
		{Period: "2025", TotalRevenue: metrics.AvailableValue(1_100_000), EBITDA: metrics.AvailableValue(230_000), SDE: metrics.AvailableValue(250_000), EBITDAMargin: metrics.AvailableValue(0.209)},
	}
	rep := Build(BuildInput{Run: &run, Snapshots: snapshots})

	if len(rep.Series.RevenueHistory) != 2 {
		t.Errorf("RevenueHistory len = %d, want 2", len(rep.Series.RevenueHistory))
	}
	if rep.Series.RevenueHistory[0].Label != "2024" || rep.Series.RevenueHistory[0].Value != 1_000_000 {
		t.Errorf("RevenueHistory[0] = %+v, want {2024, 1000000}", rep.Series.RevenueHistory[0])
	}
	if len(rep.Series.EBITDAHistory) != 2 || len(rep.Series.SDEHistory) != 2 || len(rep.Series.MarginHistory) != 2 {
		t.Error("expected EBITDA/SDE/Margin history series to each have 2 points")
	}
	// Only 4 of 5 methods actually succeeded (NetAssets excluded, no input).
	if len(rep.Series.ValuationByMethod) != 4 {
		t.Errorf("ValuationByMethod len = %d, want 4 (excludes NetAssets)", len(rep.Series.ValuationByMethod))
	}
	// ValuationHistory is a placeholder shape: always present as a field,
	// empty until a future persistence layer populates it.
	if rep.Series.ValuationHistory != nil {
		t.Errorf("ValuationHistory = %+v, want nil/empty placeholder for a single-date Report", rep.Series.ValuationHistory)
	}
}

func TestBuild_AdjustmentSummary(t *testing.T) {
	snapshot := metrics.Snapshot{
		Period: "2025",
		EBITDA: metrics.AvailableValue(500_000),
		SDE:    metrics.AvailableValue(550_000),
	}
	adjs := []adjustments.Adjustment{
		{
			ID: "adj-1", Period: "2025", Type: adjustments.TypeOwnerDiscretionaryExpense,
			Amount: 20_000, Reason: "Personal insurance run through business", Included: true,
		},
	}
	res := adjustments.Apply(snapshot, adjs)
	rep := Build(BuildInput{AdjustmentsEBITDA: &res})

	if len(rep.Adjustments.EBITDABridge) == 0 {
		t.Fatal("expected EBITDABridge to be populated")
	}
	first := rep.Adjustments.EBITDABridge[0]
	if !first.IsTotal || first.Amount != 500_000 {
		t.Errorf("first bridge line = %+v, want IsTotal=true Amount=500000", first)
	}
	last := rep.Adjustments.EBITDABridge[len(rep.Adjustments.EBITDABridge)-1]
	if !last.IsTotal || last.Amount != 520_000 {
		t.Errorf("last bridge line = %+v, want IsTotal=true Amount=520000", last)
	}
	if len(rep.Adjustments.Applied) != 1 {
		t.Errorf("expected 1 applied adjustment, got %d", len(rep.Adjustments.Applied))
	}
}

func TestBuild_SensitivityData(t *testing.T) {
	ms := sensitivity.MultipleSensitivity(1_000_000, []float64{2, 3, -1})
	matrix := sensitivity.EarningsMultipleMatrix(
		[]sensitivity.EarningsScenario{{Label: "Base", Earnings: 1_000_000}},
		[]float64{2, 3},
	)
	grid := sensitivity.DCFSensitivity(
		[]dcf.ForecastPeriod{{Period: "2026", FreeCashFlow: 100_000}},
		[]float64{0.15, 0.20}, []float64{0.03}, dcf.EquityBridgeInput{},
	)
	rep := Build(BuildInput{
		MultipleSensitivity:    &ms,
		EarningsMultipleMatrix: &matrix,
		DCFSensitivity:         &grid,
	})

	if len(rep.Sensitivity.MultipleSensitivity) != 3 {
		t.Errorf("MultipleSensitivity rows = %d, want 3", len(rep.Sensitivity.MultipleSensitivity))
	}
	if len(rep.Sensitivity.EarningsMultipleMatrix) != 2 {
		t.Errorf("EarningsMultipleMatrix rows = %d, want 2 (1 scenario x 2 multiples)", len(rep.Sensitivity.EarningsMultipleMatrix))
	}
	if len(rep.Sensitivity.DCFSensitivity) != 2 {
		t.Errorf("DCFSensitivity rows = %d, want 2 (2 discount rates x 1 terminal growth)", len(rep.Sensitivity.DCFSensitivity))
	}
}

func TestBuild_EmptyInput_NoPanic(t *testing.T) {
	rep := Build(BuildInput{})
	if rep.Summary.ConsensusAvailable {
		t.Error("expected ConsensusAvailable = false for empty input")
	}
	if len(rep.Methods) != 0 {
		t.Error("expected no method rows for empty input")
	}
}

func TestBuild_SettingsDisabledMethodExcludedInReport(t *testing.T) {
	req := orchestrator.Request{
		SDE: &sde.Input{MaintainableSDE: 300_000, Multiple: 2.5},
		Resolution: settings.Resolve(
			settings.Settings{}, settings.Settings{}, settings.Settings{},
			settings.Settings{MethodEnabled: map[settings.Method]*bool{settings.MethodSDE: settings.Bool(false)}},
		),
	}
	run := orchestrator.Execute(req)
	rep := Build(BuildInput{Run: &run})
	row := rep.Methods[0]
	if row.Included {
		t.Error("expected SDE row Included = false when disabled by settings")
	}
	if row.ExclusionReason != string(orchestrator.ExclusionDisabledBySettings) {
		t.Errorf("ExclusionReason = %q, want %q", row.ExclusionReason, orchestrator.ExclusionDisabledBySettings)
	}
}

func TestBuild_UnavailableMethodReportedNotIncluded(t *testing.T) {
	req := orchestrator.Request{
		NetAssets: &netassets.Input{}, // no assets -> blocked
	}
	run := orchestrator.Execute(req)
	rep := Build(BuildInput{Run: &run})
	row := rep.Methods[4]
	if row.Outcome != string(orchestrator.OutcomeUnavailable) {
		t.Errorf("Outcome = %q, want %q", row.Outcome, orchestrator.OutcomeUnavailable)
	}
	if row.Included {
		t.Error("expected Included = false for an unavailable method")
	}
}

func TestBuildConsensusInputs_NeverIncludesUnavailableOrExcludedMethods(t *testing.T) {
	req := orchestrator.Request{
		SDE:       &sde.Input{MaintainableSDE: 300_000, Multiple: 2.5}, // succeeds
		NetAssets: &netassets.Input{},                                  // no assets -> unavailable
		// EBITDA/Capitalization/DCF: no Input supplied -> excluded
	}
	run := orchestrator.Execute(req)
	inputs := BuildConsensusInputs(run, nil)
	if len(inputs) != 1 {
		t.Fatalf("BuildConsensusInputs returned %d inputs, want exactly 1 (only the successful SDE method); got %+v", len(inputs), inputs)
	}
	if inputs[0].Method != valuation.CodeSDEMultiple {
		t.Errorf("included method = %v, want %v", inputs[0].Method, valuation.CodeSDEMultiple)
	}
}

func TestBuildConsensusInputs_WeightForAbsentMethodHasNoEffect(t *testing.T) {
	req := orchestrator.Request{
		SDE: &sde.Input{MaintainableSDE: 300_000, Multiple: 2.5}, // the only method that will run
	}
	run := orchestrator.Execute(req)
	// Supply a weight for a method that was never even requested — it must
	// be silently ignored, never cause that method to appear as a
	// dead-weight entry in the consensus input set.
	weights := map[valuation.Code]float64{
		valuation.CodeSDEMultiple:    1,
		valuation.CodeEBITDAMultiple: 5,
	}
	inputs := BuildConsensusInputs(run, weights)
	if len(inputs) != 1 {
		t.Fatalf("BuildConsensusInputs returned %d inputs, want exactly 1; a weight for an absent method must not add a phantom entry: %+v", len(inputs), inputs)
	}
	if inputs[0].Method != valuation.CodeSDEMultiple {
		t.Errorf("included method = %v, want %v", inputs[0].Method, valuation.CodeSDEMultiple)
	}
}

// --- JSON serialization tests ---

func TestReport_JSONRoundTrip(t *testing.T) {
	run := sampleRun(t)
	inputs := BuildConsensusInputs(run, nil)
	c := consensus.Calculate(inputs, consensus.Options{TargetBasis: valuation.ValueTypeEquity})
	snapshots := []metrics.Snapshot{
		{Period: "2025", TotalRevenue: metrics.AvailableValue(1_000_000)},
	}
	rep := Build(BuildInput{Run: &run, Consensus: &c, Snapshots: snapshots, ValuationDate: "2026-01-01"})

	data, err := json.Marshal(rep)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	if !json.Valid(data) {
		t.Fatal("expected valid JSON output")
	}

	var roundTripped Report
	if err := json.Unmarshal(data, &roundTripped); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	// Full round-trip contract: marshal -> unmarshal -> marshal must
	// reproduce the exact same bytes, not merely agree on a couple of
	// spot-checked fields.
	data2, err := json.Marshal(roundTripped)
	if err != nil {
		t.Fatalf("re-marshal of round-tripped Report failed: %v", err)
	}
	if string(data) != string(data2) {
		t.Fatal("Report did not round-trip byte-for-byte through marshal -> unmarshal -> marshal")
	}
}

func TestReport_JSONHasNoNaNOrInf(t *testing.T) {
	// A Report built from an empty/degenerate input (e.g. zero mean feeding
	// a percentOf-style ratio somewhere upstream) must never serialize
	// NaN/Inf, which encoding/json rejects outright.
	run := orchestrator.Execute(orchestrator.Request{
		SDE: &sde.Input{MaintainableSDE: 0, Multiple: 1},
	})
	inputs := BuildConsensusInputs(run, nil)
	c := consensus.Calculate(inputs, consensus.Options{TargetBasis: valuation.ValueTypeEquity})
	rep := Build(BuildInput{Run: &run, Consensus: &c})

	if _, err := json.Marshal(rep); err != nil {
		t.Fatalf("json.Marshal failed on a zero-mean Report: %v", err)
	}
}

func TestReport_JSONFieldNamesAreSnakeCase(t *testing.T) {
	run := sampleRun(t)
	rep := Build(BuildInput{Run: &run})
	data, err := json.Marshal(rep)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	// Spot-check a few expected snake_case keys are present, confirming
	// struct tags took effect rather than falling back to Go field names.
	for _, key := range []string{`"simple_consensus"`, `"method_range"`, `"valuation_by_method"`} {
		if !strings.Contains(string(data), key) {
			t.Errorf("expected JSON output to contain %s", key)
		}
	}
}

func TestReport_StableAcrossRepeatedBuilds(t *testing.T) {
	run1 := sampleRun(t)
	run2 := sampleRun(t)
	rep1 := Build(BuildInput{Run: &run1})
	rep2 := Build(BuildInput{Run: &run2})

	data1, _ := json.Marshal(rep1)
	data2, _ := json.Marshal(rep2)
	if string(data1) != string(data2) {
		t.Error("expected two Builds from equivalent input to produce identical JSON")
	}
}
