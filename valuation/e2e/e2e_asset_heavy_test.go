package e2e

import (
	"encoding/json"
	"testing"

	"github.com/themurtez/go-valuate/financial/adjustments"
	"github.com/themurtez/go-valuate/financial/earnings"
	"github.com/themurtez/go-valuate/financial/metrics"
	"github.com/themurtez/go-valuate/financial/reconciliation"
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

// TestEndToEnd_Manufacturer runs the complete deterministic pipeline for
// an asset-heavy, professionally-managed (not owner-operated) manufacturer
// — the second required e2e archetype alongside TestEndToEnd_HVAC's
// owner-operated service business, exercising:
//
//   - a high asset-intensity applicability profile (Adjusted Net Asset
//     Value scoring HIGH/MEDIUM rather than LOW, the opposite of the HVAC
//     fixture's SDE-dominant profile),
//   - the value-basis conversion path for a caller who wants Adjusted Net
//     Asset Value included in an equity-basis consensus alongside
//     earnings-based methods (see valuation/basis's asset->equity identity
//     conversion),
//   - a fixture whose adjustments only cover a single period (2025, not
//     HVAC's three), so maintainable earnings uses
//     earnings.StrategyLatestPeriod rather than StrategySimpleAverage —
//     a legitimate strategy choice for this data shape, not a fixture gap.
//
// Uses the same fixtures/normalized_manufacturer_multi_year.json,
// fixtures/adjustments_by_business_type.json ("manufacturer"), and
// fixtures/valuation_by_business_type.json ("manufacturer") every other
// package's own manufacturer-archetype tests already use — no new fixture
// data was invented for this test.
func TestEndToEnd_Manufacturer(t *testing.T) {
	// 1. Normalized financial data -> financial/metrics.
	ds := loadDataset(t, "normalized_manufacturer_multi_year.json")
	metricsResult := metrics.Calculate(ds, metrics.Options{PeriodMeta: fyMeta(2023, 2024, 2025)})
	if len(metricsResult.Snapshots) != 3 {
		t.Fatalf("expected 3 snapshots, got %d", len(metricsResult.Snapshots))
	}
	snap2025, ok := metricsResult.SnapshotFor("2025")
	if !ok {
		t.Fatal("expected a 2025 snapshot")
	}
	if !snap2025.EBITDA.Available {
		t.Fatal("expected 2025 EBITDA to be available")
	}
	if !snap2025.TangibleAssetValue.Available || !snap2025.TotalRevenue.Available {
		t.Fatal("expected 2025 TangibleAssetValue and TotalRevenue to be available (needed for the asset-intensity profile below)")
	}
	assetIntensity := snap2025.TangibleAssetValue.Value / snap2025.TotalRevenue.Value
	if assetIntensity < 0.3 {
		t.Fatalf("expected this archetype's asset intensity to be meaningfully higher than a typical service business (>=0.3), got %v — fixture may no longer represent an asset-heavy business", assetIntensity)
	}

	// 1b. financial/reconciliation: the same normalized dataset must be
	// internally consistent.
	reconResult := reconciliation.Run(ds, reconciliation.Options{})
	if reconResult.HasFailures() {
		for _, check := range reconResult.Checks {
			if check.Status == reconciliation.StatusFail {
				t.Errorf("reconciliation check %s unexpectedly FAILed: %s", check.Code, check.Explanation)
			}
		}
	}

	// 2. financial/adjustments: normalized EBITDA/SDE bridges for 2025 —
	// this archetype's fixture only has 2025 adjustments (unlike HVAC's
	// three years), reflected in the earnings strategy chosen in step 3.
	adjs := loadAdjustments(t, "manufacturer")
	adjResult := adjustments.Apply(snap2025, adjs)
	if adjustments.HasErrors(adjResult.Errors) {
		t.Fatalf("unexpected adjustment errors: %+v", adjResult.Errors)
	}
	if !adjResult.EBITDABridge.BaseAvailable {
		t.Fatal("expected the EBITDA bridge to have an available base metric")
	}

	// 3. financial/earnings: a single normalized-EBITDA observation ->
	// StrategyLatestPeriod, since only one period's adjustments exist for
	// this archetype (a genuine, deliberate strategy choice for this data
	// shape — see financial/earnings' StrategyLatestPeriod doc comment).
	ebitdaObs := []earnings.Observation{
		{Period: "2025", Value: adjResult.EBITDABridge.NormalizedValue, PeriodType: metrics.PeriodTypeFiscalYear, Available: true},
	}
	maintainableEBITDA := earnings.Calculate(ebitdaObs, earnings.Options{Strategy: earnings.StrategyLatestPeriod})
	if !maintainableEBITDA.Available {
		t.Fatalf("expected maintainable EBITDA to be available: %+v", maintainableEBITDA.Errors)
	}

	// 4. Individual valuation methods, using caller-supplied assumptions
	// from the golden fixture. This archetype is not owner-operated (see
	// the fixture's own _comment: "SDE == EBITDA in the underlying
	// fixture"), so SDE reuses the same maintainable EBITDA figure as its
	// earnings base, exactly as the fixture intends.
	assumptions := loadValuationAssumptions(t, "manufacturer")

	sdeInput := sde.Input{MaintainableSDE: maintainableEBITDA.Value, Multiple: assumptions.SDEMultiple}
	ebitdaInput := ebitda.Input{
		MaintainableEBITDA: maintainableEBITDA.Value, Multiple: assumptions.EBITDAMultiple,
		EquityBridge: ebitda.EquityBridgeInput{
			Requested: true, ExcessCash: 238000, ShortTermDebt: 140000, LongTermDebt: 825000,
		},
	}
	capInput := capitalization.Input{MaintainableEarnings: maintainableEBITDA.Value, CapitalizationRate: assumptions.CapitalizationRate}
	dcfInput := dcf.Input{
		ForecastPeriods:    assumptions.DCF.ForecastPeriods,
		DiscountRate:       assumptions.DCF.DiscountRate,
		TerminalGrowthRate: assumptions.DCF.TerminalGrowthRate,
		EquityBridge: dcf.EquityBridgeInput{
			Requested: true, ExcessCash: 238000, ShortTermDebt: 140000, LongTermDebt: 825000,
		},
	}
	netAssetsInput := netassets.Input{Assets: assumptions.NetAssetValue.Assets, Liabilities: assumptions.NetAssetValue.Liabilities}

	// 5. valuation/applicability: score every method for this business
	// profile (asset-heavy manufacturer, not owner-operated).
	ownerOperated := false
	revenue := snap2025.TotalRevenue.Value
	prof := profile.Profile{
		Industry:       profile.IndustryManufacturing,
		OwnerOperated:  &ownerOperated,
		AnnualRevenue:  &revenue,
		AssetIntensity: &assetIntensity,
		Profitability:  profile.ProfitabilityModerate,
		DataAvailability: profile.DataAvailability{
			HasMultiYearFinancials:  true,
			HasBalanceSheet:         true,
			HasForecast:             true,
			HasAppraisedAssetValues: true,
		},
	}
	applicabilityResults := applicability.Calculate(prof)
	netAssetsApplicability, ok := applicabilityResults.ForMethod(string(valuation.CodeAdjustedNetAssetValue))
	if !ok || (netAssetsApplicability.Level != applicability.LevelHigh && netAssetsApplicability.Level != applicability.LevelMedium) {
		t.Errorf("expected Adjusted Net Asset Value applicability HIGH or MEDIUM for a high-asset-intensity manufacturer, got %v (score %d)", netAssetsApplicability.Level, netAssetsApplicability.Score)
	}
	ebitdaApplicability, ok := applicabilityResults.ForMethod(string(valuation.CodeEBITDAMultiple))
	if !ok || ebitdaApplicability.Level == applicability.LevelNotApplicable {
		t.Errorf("expected EBITDA Multiple applicability to be a real (non-NOT_APPLICABLE) score for a professionally-managed manufacturer, got %v", ebitdaApplicability.Level)
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

	// 7. valuation/consensus: equal-weighted across all 5 methods, on an
	// explicit equity basis — the same value-basis conversion path
	// TestEndToEnd_HVAC exercises, here specifically proving Adjusted Net
	// Asset Value (this archetype's most applicable method) converts onto
	// the equity basis via the documented identity conversion (see
	// valuation/basis), not excluded or silently averaged on its native
	// asset_value basis.
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
	consensusResult := consensus.Calculate(consensusInputs, consensus.Options{TargetBasis: valuation.ValueTypeEquity})
	if !consensusResult.Available {
		t.Fatalf("expected consensus to be available: %+v", consensusResult.Errors)
	}
	if !consensusResult.WeightsValid {
		t.Fatalf("expected equal weights to be valid: %+v", consensusResult.Errors)
	}
	if consensusResult.Statistics.SimpleMean <= 0 {
		t.Errorf("expected a positive SimpleMean, got %v", consensusResult.Statistics.SimpleMean)
	}
	if len(consensusResult.Included) != 5 {
		t.Fatalf("expected all 5 methods to convert successfully onto the equity basis, got %d included; conversions=%+v", len(consensusResult.Included), consensusResult.Conversions)
	}
	if len(consensusResult.BasisExclusions) != 0 {
		t.Errorf("expected no basis exclusions, got %+v", consensusResult.BasisExclusions)
	}
	var netAssetsConverted bool
	for _, c := range consensusResult.Conversions {
		if c.Method == valuation.CodeAdjustedNetAssetValue {
			netAssetsConverted = true
			if c.Outcome != "converted" {
				t.Errorf("expected NetAssets conversion Outcome = converted, got %v", c.Outcome)
			}
			if c.ConvertedValue != c.OriginalValue {
				t.Errorf("asset->equity is an identity conversion: ConvertedValue (%v) should equal OriginalValue (%v)", c.ConvertedValue, c.OriginalValue)
			}
		}
	}
	if !netAssetsConverted {
		t.Fatal("expected a Conversion entry for the NetAssets method")
	}

	// 8. valuation/sensitivity: multiple sensitivity over the EBITDA
	// multiple (the applicable method for this archetype, unlike HVAC's
	// SDE-based sensitivity), and a DCF discount-rate grid.
	multiplePoints := sensitivity.MultipleSensitivity(maintainableEBITDA.Value, []float64{4.5, 5.5, 6.5})
	if len(multiplePoints.Points) != 3 {
		t.Fatalf("expected 3 multiple sensitivity points, got %d", len(multiplePoints.Points))
	}
	for _, p := range multiplePoints.Points {
		if !p.Valid {
			t.Errorf("expected multiple sensitivity point %+v to be valid", p)
		}
	}
	dcfGrid := sensitivity.DCFSensitivity(
		dcfInput.ForecastPeriods,
		[]float64{dcfInput.DiscountRate - 0.02, dcfInput.DiscountRate, dcfInput.DiscountRate + 0.02},
		[]float64{dcfInput.TerminalGrowthRate},
		dcf.EquityBridgeInput{Requested: true, ExcessCash: 238000, ShortTermDebt: 140000, LongTermDebt: 825000},
	)
	if len(dcfGrid.Rows) != 3 || len(dcfGrid.Rows[0]) != 1 {
		t.Fatalf("expected a 3x1 DCF sensitivity grid, got %d rows", len(dcfGrid.Rows))
	}

	// 9. valuation/report: assemble the final report.
	rep := report.Build(report.BuildInput{
		ValuationDate:       "2026-01-01",
		Snapshots:           metricsResult.Snapshots,
		Run:                 &run,
		Consensus:           &consensusResult,
		MultipleSensitivity: &multiplePoints,
		DCFSensitivity:      &dcfGrid,
		AdjustmentsEBITDA:   &adjResult,
	})
	if !rep.Summary.ConsensusAvailable {
		t.Fatal("expected report Summary.ConsensusAvailable = true")
	}
	if len(rep.Methods) != 5 {
		t.Fatalf("expected 5 method rows, got %d", len(rep.Methods))
	}
	for _, row := range rep.Methods {
		if !row.Included {
			t.Errorf("method %v: expected Included = true (all 5 succeeded)", row.Method)
		}
		if row.Value == 0 {
			t.Errorf("method %v: expected a nonzero Value", row.Method)
		}
		if row.Applicability == nil {
			t.Errorf("method %v: expected Applicability to be populated", row.Method)
		}
	}

	// Final: the whole Report must serialize cleanly to JSON (no NaN/Inf)
	// and round-trip.
	repJSON, err := json.Marshal(rep)
	if err != nil {
		t.Fatalf("report failed to serialize to JSON: %v", err)
	}
	var roundTripped report.Report
	if err := json.Unmarshal(repJSON, &roundTripped); err != nil {
		t.Fatalf("report failed to round-trip from JSON: %v", err)
	}
	repJSON2, err := json.Marshal(roundTripped)
	if err != nil {
		t.Fatalf("round-tripped report failed to re-serialize: %v", err)
	}
	if string(repJSON) != string(repJSON2) {
		t.Fatal("report JSON did not round-trip byte-for-byte through marshal -> unmarshal -> marshal")
	}
}
