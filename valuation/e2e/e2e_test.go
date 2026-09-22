// Package e2e contains a single, complete end-to-end deterministic fixture
// exercising the whole pipeline this repository implements, front to back,
// for one realistic small private business:
//
//	normalized financial data
//	-> financial/metrics (Calculate)
//	-> financial/adjustments (Apply: normalized EBITDA/SDE bridges)
//	-> financial/earnings (Calculate: maintainable earnings)
//	-> individual valuation methods (valuation/sde, ebitda, capitalization,
//	   dcf, netassets)
//	-> valuation/applicability (Calculate)
//	-> valuation/orchestrator (Execute)
//	-> valuation/consensus (Calculate)
//	-> valuation/sensitivity (MultipleSensitivity, DCFSensitivity)
//	-> valuation/report (Build)
//
// No database, no AI, no PDF, no network I/O anywhere in this chain — every
// stage is a pure function over the previous stage's output, using the
// repository's existing fixtures (fixtures/normalized_hvac_multi_year.json,
// fixtures/adjustments_by_business_type.json,
// fixtures/valuation_by_business_type.json) for a small owner-operated HVAC
// service business, the same archetype used throughout this repository's
// other fixture-based tests.
//
// This package lives outside financial/valuation's own directory trees
// specifically because it is the one place allowed to import nearly every
// package in the module at once; no other package should need to.
package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/adjustments"
	"github.com/themurtez/go-valuate/financial/earnings"
	"github.com/themurtez/go-valuate/financial/metrics"
	"github.com/themurtez/go-valuate/valuation"
	"github.com/themurtez/go-valuate/valuation/applicability"
	"github.com/themurtez/go-valuate/valuation/capitalization"
	"github.com/themurtez/go-valuate/valuation/consensus"
	"github.com/themurtez/go-valuate/valuation/dcf"
	"github.com/themurtez/go-valuate/valuation/ebitda"
	"github.com/themurtez/go-valuate/valuation/netassets"
	"github.com/themurtez/go-valuate/valuation/orchestrator"
	"github.com/themurtez/go-valuate/valuation/profile"
	"github.com/themurtez/go-valuate/valuation/report"
	"github.com/themurtez/go-valuate/valuation/sde"
	"github.com/themurtez/go-valuate/valuation/sensitivity"
)

func fixturePath(elems ...string) string {
	return filepath.Join(append([]string{"..", "..", "fixtures"}, elems...)...)
}

func loadDataset(t *testing.T, name string) financial.FinancialDataset {
	t.Helper()
	b, err := os.ReadFile(fixturePath(name))
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	var ds financial.FinancialDataset
	if err := json.Unmarshal(b, &ds); err != nil {
		t.Fatalf("unmarshaling fixture %s: %v", name, err)
	}
	return ds
}

func loadAdjustments(t *testing.T, archetype string) []adjustments.Adjustment {
	t.Helper()
	b, err := os.ReadFile(fixturePath("adjustments_by_business_type.json"))
	if err != nil {
		t.Fatalf("reading adjustments fixture: %v", err)
	}
	var byArchetype map[string][]adjustments.Adjustment
	if err := json.Unmarshal(b, &byArchetype); err != nil {
		t.Fatalf("unmarshaling adjustments fixture: %v", err)
	}
	adjs, ok := byArchetype[archetype]
	if !ok {
		t.Fatalf("no adjustments for archetype %q", archetype)
	}
	return adjs
}

type valuationAssumptions struct {
	SDEMultiple        float64 `json:"sde_multiple"`
	EBITDAMultiple     float64 `json:"ebitda_multiple"`
	CapitalizationRate float64 `json:"capitalization_rate"`
	DCF                struct {
		ForecastPeriods    []dcf.ForecastPeriod `json:"forecast_periods"`
		DiscountRate       float64              `json:"discount_rate"`
		TerminalGrowthRate float64              `json:"terminal_growth_rate"`
	} `json:"dcf"`
	NetAssetValue struct {
		Assets      []netassets.AssetItem     `json:"assets"`
		Liabilities []netassets.LiabilityItem `json:"liabilities"`
	} `json:"net_asset_value"`
}

func loadValuationAssumptions(t *testing.T, archetype string) valuationAssumptions {
	t.Helper()
	b, err := os.ReadFile(fixturePath("valuation_by_business_type.json"))
	if err != nil {
		t.Fatalf("reading valuation fixture: %v", err)
	}
	var byArchetype map[string]json.RawMessage
	if err := json.Unmarshal(b, &byArchetype); err != nil {
		t.Fatalf("unmarshaling valuation fixture: %v", err)
	}
	raw, ok := byArchetype[archetype]
	if !ok {
		t.Fatalf("no valuation assumptions for archetype %q", archetype)
	}
	var a valuationAssumptions
	if err := json.Unmarshal(raw, &a); err != nil {
		t.Fatalf("unmarshaling %q valuation assumptions: %v", archetype, err)
	}
	return a
}

func fyMeta(years ...int) map[financial.Period]metrics.PeriodInfo {
	meta := make(map[financial.Period]metrics.PeriodInfo, len(years))
	for _, y := range years {
		p := financial.Period(itoa(y))
		meta[p] = metrics.PeriodInfo{Type: metrics.PeriodTypeFiscalYear, FiscalYear: y}
	}
	return meta
}

func itoa(y int) string {
	// Small local helper to avoid importing strconv solely for this.
	digits := [10]byte{}
	i := len(digits)
	n := y
	for n > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}
	return string(digits[i:])
}

// TestEndToEnd_HVAC runs the complete deterministic pipeline for a small
// owner-operated HVAC service business and asserts on the result at every
// stage, not only the final report — see the package doc comment for the
// full pipeline this exercises.
func TestEndToEnd_HVAC(t *testing.T) {
	// 1. Normalized financial data -> financial/metrics.
	ds := loadDataset(t, "normalized_hvac_multi_year.json")
	metricsResult := metrics.Calculate(ds, metrics.Options{PeriodMeta: fyMeta(2023, 2024, 2025)})
	if len(metricsResult.Snapshots) != 3 {
		t.Fatalf("expected 3 snapshots, got %d", len(metricsResult.Snapshots))
	}
	snap2025, ok := metricsResult.SnapshotFor("2025")
	if !ok {
		t.Fatal("expected a 2025 snapshot")
	}
	if !snap2025.EBITDA.Available || !snap2025.SDE.Available {
		t.Fatal("expected 2025 EBITDA and SDE to be available")
	}

	// 2. financial/adjustments: normalized EBITDA/SDE bridges for 2025.
	adjs := loadAdjustments(t, "hvac")
	adjResult := adjustments.Apply(snap2025, adjs)
	if adjustments.HasErrors(adjResult.Errors) {
		t.Fatalf("unexpected adjustment errors: %+v", adjResult.Errors)
	}
	if !adjResult.EBITDABridge.BaseAvailable || !adjResult.SDEBridge.BaseAvailable {
		t.Fatal("expected both bridges to have an available base metric")
	}

	// 3. financial/earnings: maintainable earnings across all 3 normalized
	// years (using each year's own bridge so the earnings strategy sees a
	// genuine multi-year series, not one repeated figure).
	var ebitdaObs, sdeObs []earnings.Observation
	for _, period := range []string{"2023", "2024", "2025"} {
		snap, ok := metricsResult.SnapshotFor(financial.Period(period))
		if !ok {
			t.Fatalf("missing snapshot for %s", period)
		}
		yearAdjs := adjustmentsForPeriod(adjs, period)
		res := adjustments.Apply(snap, yearAdjs)
		ebitdaObs = append(ebitdaObs, earnings.Observation{
			Period: period, PeriodType: earnings.PeriodTypeFiscalYear,
			Value: res.EBITDABridge.NormalizedValue, Available: res.EBITDABridge.BaseAvailable,
		})
		sdeObs = append(sdeObs, earnings.Observation{
			Period: period, PeriodType: earnings.PeriodTypeFiscalYear,
			Value: res.SDEBridge.NormalizedValue, Available: res.SDEBridge.BaseAvailable,
		})
	}
	maintainableEBITDA := earnings.Calculate(ebitdaObs, earnings.Options{Strategy: earnings.StrategySimpleAverage})
	maintainableSDE := earnings.Calculate(sdeObs, earnings.Options{Strategy: earnings.StrategySimpleAverage})
	if !maintainableEBITDA.Available || !maintainableSDE.Available {
		t.Fatalf("expected maintainable earnings to be available: ebitda=%+v sde=%+v", maintainableEBITDA.Errors, maintainableSDE.Errors)
	}

	// 4. Individual valuation methods, using caller-supplied assumptions
	// from the golden fixture.
	assumptions := loadValuationAssumptions(t, "hvac")

	sdeInput := sde.Input{MaintainableSDE: maintainableSDE.Value, Multiple: assumptions.SDEMultiple}
	ebitdaInput := ebitda.Input{
		MaintainableEBITDA: maintainableEBITDA.Value, Multiple: assumptions.EBITDAMultiple,
		EquityBridge: ebitda.EquityBridgeInput{
			Requested: true, ExcessCash: 63000, ShortTermDebt: 8000, LongTermDebt: 47000,
		},
	}
	capInput := capitalization.Input{MaintainableEarnings: maintainableSDE.Value, CapitalizationRate: assumptions.CapitalizationRate}
	dcfInput := dcf.Input{
		ForecastPeriods:    assumptions.DCF.ForecastPeriods,
		DiscountRate:       assumptions.DCF.DiscountRate,
		TerminalGrowthRate: assumptions.DCF.TerminalGrowthRate,
		EquityBridge: dcf.EquityBridgeInput{
			Requested: true, ExcessCash: 63000, ShortTermDebt: 8000, LongTermDebt: 47000,
		},
	}
	netAssetsInput := netassets.Input{Assets: assumptions.NetAssetValue.Assets, Liabilities: assumptions.NetAssetValue.Liabilities}

	// 5. valuation/applicability: score every method for this business
	// profile (small owner-operated HVAC service business).
	ownerOperated := true
	revenue := 900000.0 // approximate 2025 total revenue for this fixture archetype
	employees := 8
	assetIntensity := 0.25
	prof := profile.Profile{
		Industry:       profile.IndustryTrades,
		OwnerOperated:  &ownerOperated,
		AnnualRevenue:  &revenue,
		EmployeeCount:  &employees,
		AssetIntensity: &assetIntensity,
		Profitability:  profile.ProfitabilityModerate,
		DataAvailability: profile.DataAvailability{
			HasMultiYearFinancials: true,
			HasBalanceSheet:        true,
			HasForecast:            true,
		},
	}
	applicabilityResults := applicability.Calculate(prof)
	sdeApplicability, ok := applicabilityResults.ForMethod(string(valuation.CodeSDEMultiple))
	if !ok || sdeApplicability.Level != applicability.LevelHigh {
		t.Errorf("expected SDE applicability HIGH for a small owner-operated HVAC business, got %v", sdeApplicability.Level)
	}

	// 6. valuation/orchestrator: run every method.
	run := orchestrator.Execute(orchestrator.Request{
		Applicability:  &applicabilityResults,
		SDE:            &sdeInput,
		EBITDA:         &ebitdaInput,
		Capitalization: &capInput,
		DCF:            &dcfInput,
		NetAssets:      &netAssetsInput,
	})
	if len(run.Successful()) != 5 {
		t.Fatalf("expected all 5 methods to succeed, got %d successful; run=%+v", len(run.Successful()), run.Methods)
	}

	// 7. valuation/consensus: equal-weighted across all 5 methods.
	weights := map[valuation.Code]float64{
		valuation.CodeSDEMultiple:              1,
		valuation.CodeEBITDAMultiple:           1,
		valuation.CodeCapitalizationOfEarnings: 1,
		valuation.CodeDCF:                      1,
		valuation.CodeAdjustedNetAssetValue:    1,
	}
	consensusInputs := report.BuildConsensusInputs(run, weights)
	if len(consensusInputs) != 5 {
		t.Fatalf("expected 5 consensus inputs, got %d", len(consensusInputs))
	}
	consensusResult := consensus.Calculate(consensusInputs)
	if !consensusResult.Available {
		t.Fatalf("expected consensus to be available: %+v", consensusResult.Errors)
	}
	if !consensusResult.WeightsValid {
		t.Fatalf("expected equal weights to be valid: %+v", consensusResult.Errors)
	}
	if consensusResult.Statistics.SimpleMean <= 0 {
		t.Errorf("expected a positive SimpleMean, got %v", consensusResult.Statistics.SimpleMean)
	}
	// Simple mean of 5 mixed enterprise/equity/asset values is a real,
	// deliberately-flagged number here (see MixedValueTypes); the fixture's
	// point is that the pipeline runs end to end, not that mixing value
	// types is the recommended real-world usage.
	if !consensusResult.MixedValueTypes {
		t.Error("expected MixedValueTypes = true (SDE/Capitalization are equity, EBITDA/DCF are enterprise, NetAssets is asset)")
	}

	// 8. valuation/sensitivity: multiple sensitivity over the SDE multiple,
	// and a DCF discount-rate x terminal-growth grid.
	multiplePoints := sensitivity.MultipleSensitivity(maintainableSDE.Value, []float64{2.0, 2.5, 3.0})
	if len(multiplePoints.Points) != 3 {
		t.Fatalf("expected 3 multiple sensitivity points, got %d", len(multiplePoints.Points))
	}
	for _, p := range multiplePoints.Points {
		if !p.Valid {
			t.Errorf("expected multiple %v to be valid", p.Multiple)
		}
	}

	dcfGrid := sensitivity.DCFSensitivity(
		assumptions.DCF.ForecastPeriods,
		[]float64{assumptions.DCF.DiscountRate - 0.02, assumptions.DCF.DiscountRate, assumptions.DCF.DiscountRate + 0.02},
		[]float64{assumptions.DCF.TerminalGrowthRate},
		dcf.EquityBridgeInput{},
	)
	for i, row := range dcfGrid.Rows {
		for j, cell := range row {
			if !cell.Valid {
				t.Errorf("dcf grid cell (%d,%d) unexpectedly invalid: %+v", i, j, cell.Result.Errors)
			}
		}
	}

	// 9. valuation/report: assemble the final presentation-neutral report.
	rep := report.Build(report.BuildInput{
		ValuationDate:       "2026-01-15",
		Snapshots:           metricsResult.Snapshots,
		NormalizedEBITDA:    report.FinancialFigure{Available: true, Value: maintainableEBITDA.Value},
		NormalizedSDE:       report.FinancialFigure{Available: true, Value: maintainableSDE.Value},
		AdjustmentsEBITDA:   &adjResult,
		AdjustmentsSDE:      &adjResult,
		Run:                 &run,
		Consensus:           &consensusResult,
		MultipleSensitivity: &multiplePoints,
		DCFSensitivity:      &dcfGrid,
	})

	// Full-report assertions.
	if !rep.Summary.ConsensusAvailable {
		t.Fatal("expected report Summary.ConsensusAvailable = true")
	}
	if rep.Summary.ValuationDate != "2026-01-15" {
		t.Errorf("ValuationDate = %q, want 2026-01-15", rep.Summary.ValuationDate)
	}
	if rep.Summary.IncludedMethodCount != 5 {
		t.Errorf("IncludedMethodCount = %d, want 5", rep.Summary.IncludedMethodCount)
	}
	if len(rep.Methods) != 5 {
		t.Fatalf("expected 5 method rows, got %d", len(rep.Methods))
	}
	for _, row := range rep.Methods {
		if !row.Included {
			t.Errorf("method %s: expected Included = true", row.Method)
		}
		if row.Value == 0 {
			t.Errorf("method %s: expected a nonzero Value", row.Method)
		}
		if row.Applicability == nil {
			t.Errorf("method %s: expected Applicability to be populated", row.Method)
		}
	}
	if len(rep.Financial.Periods) != 3 {
		t.Errorf("expected 3 financial periods in the report, got %d", len(rep.Financial.Periods))
	}
	if len(rep.Adjustments.EBITDABridge) == 0 || len(rep.Adjustments.SDEBridge) == 0 {
		t.Error("expected both bridges to be populated in the report")
	}
	if len(rep.Sensitivity.MultipleSensitivity) != 3 {
		t.Errorf("expected 3 multiple sensitivity rows in the report, got %d", len(rep.Sensitivity.MultipleSensitivity))
	}
	if len(rep.Sensitivity.DCFSensitivity) != 3 {
		t.Errorf("expected 3 DCF sensitivity rows in the report, got %d", len(rep.Sensitivity.DCFSensitivity))
	}
	if len(rep.Series.ValuationByMethod) != 5 {
		t.Errorf("expected 5 points in ValuationByMethod series, got %d", len(rep.Series.ValuationByMethod))
	}
	if len(rep.Series.RevenueHistory) != 3 {
		t.Errorf("expected 3 points in RevenueHistory series, got %d", len(rep.Series.RevenueHistory))
	}

	// Final: the whole Report must serialize cleanly to JSON (no NaN/Inf,
	// no unsupported types) — the deliverable a future API/UI/export
	// pipeline actually consumes.
	if _, err := json.Marshal(rep); err != nil {
		t.Fatalf("report failed to serialize to JSON: %v", err)
	}
}

func adjustmentsForPeriod(all []adjustments.Adjustment, period string) []adjustments.Adjustment {
	var out []adjustments.Adjustment
	for _, a := range all {
		if string(a.Period) == period {
			out = append(out, a)
		}
	}
	return out
}
