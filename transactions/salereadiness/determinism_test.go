package salereadiness

import (
	"encoding/json"
	"testing"

	"github.com/themurtez/go-valuate/analytics/concentration"
	"github.com/themurtez/go-valuate/analytics/qoe"
	"github.com/themurtez/go-valuate/analytics/revenuequality"
	"github.com/themurtez/go-valuate/analytics/workingcapital"
	"github.com/themurtez/go-valuate/financial/metrics"
	"github.com/themurtez/go-valuate/valuation/consensus"
	"github.com/themurtez/go-valuate/valuation/profile"
)

// fullFixture builds an Input exercising every dimension and every
// optional module, reused by the determinism/round-trip tests.
func fullFixture() Input {
	return Input{
		DataQuality: DataQuality{
			HasReviewedOrAuditedFinancials: true,
			HasTaxReturnReconciliation:     true,
			OpenAccountingIssueCount:       intPtr(0),
		},
		QoE: qoe.Result{
			Available: true,
			History: []qoe.PeriodFigures{
				{Period: "FY2024", NormalizedEBITDA: metrics.AvailableValue(500_000)},
			},
			EBITDAVolatility: metrics.VolatilityResult{Value: metrics.AvailableValue(0.12)},
			EBITDAMarginTrend: []metrics.MarginPoint{
				{Period: "FY2023", Margin: metrics.AvailableValue(0.19)},
				{Period: "FY2024", Margin: metrics.AvailableValue(0.21)},
			},
			Ratios: qoe.Ratios{
				Period:             "FY2024",
				AdjustmentToEBITDA: metrics.AvailableValue(0.12),
			},
			Flags: []qoe.Flag{
				{Code: qoe.FlagLargeOwnerDiscretionaryComponent, Severity: qoe.FlagSeverityWarning, Message: "large owner-discretionary component"},
			},
		},
		WorkingCapital: workingcapital.Result{
			Available: true,
			NWCPercentOfRevenueStatistics: workingcapital.Statistics{
				Volatility: workingcapital.NWCValue{Available: true, Value: 0.09},
			},
		},
		Concentration: concentration.Result{
			Available: true,
			History: []concentration.PeriodConcentration{
				{Period: "FY2024", LargestEntityShare: concentration.ConcentrationValue{Available: true, Value: 0.30}},
			},
		},
		RevenueQuality: revenuequality.Result{
			Available: true,
			TotalRevenueHistory: []revenuequality.PeriodRevenue{
				{Period: "FY2024", RecurringPercent: revenuequality.RevenueValue{Available: true, Value: 0.35}},
			},
		},
		Consensus: consensus.Result{
			Available: true,
			Statistics: consensus.Statistics{
				Count:                  3,
				CoefficientOfVariation: 0.18,
			},
		},
		Metrics: metrics.Result{
			Snapshots: []metrics.Snapshot{
				{Period: "FY2024", NetDebt: metrics.AvailableValue(1_200_000), EBITDA: metrics.AvailableValue(500_000)},
			},
		},
		Profile: profile.Profile{
			OwnerOperated:           boolPtr(true),
			RecurringRevenuePercent: floatPtr(0.35),
		},
		Policy: wholeInputPolicy(),
	}
}

// TestCalculate_Deterministic proves repeated execution against identical
// input returns byte-for-byte identical output.
func TestCalculate_Deterministic(t *testing.T) {
	in := fullFixture()

	first, err := json.Marshal(Calculate(in))
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	for i := 0; i < 10; i++ {
		got, err := json.Marshal(Calculate(in))
		if err != nil {
			t.Fatalf("run %d: json.Marshal failed: %v", i, err)
		}
		if string(got) != string(first) {
			t.Fatalf("run %d: Calculate output differs from the first run", i)
		}
	}
}

// TestCalculate_DimensionOrder proves Result.Dimensions is always the same
// fixed 11-entry order whenever Available is true, regardless of which
// Input fields were supplied.
func TestCalculate_DimensionOrder(t *testing.T) {
	for _, in := range []Input{fullFixture(), {Profile: profile.Profile{OwnerOperated: boolPtr(false)}}} {
		res := Calculate(in)
		if !res.Available {
			t.Fatalf("expected Available, errors=%+v", res.Errors)
		}
		if len(res.Dimensions) != len(dimensionOrder) {
			t.Fatalf("expected %d dimensions, got %d", len(dimensionOrder), len(res.Dimensions))
		}
		for i, code := range dimensionOrder {
			if res.Dimensions[i].Code != code {
				t.Fatalf("dimension %d: expected %s, got %s", i, code, res.Dimensions[i].Code)
			}
		}
	}
}
