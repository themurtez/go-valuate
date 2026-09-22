package sensitivity

import (
	"encoding/json"
	"testing"

	"github.com/themurtez/go-valuate/valuation/dcf"
)

// TestMultipleSensitivityResult_JSONRoundTrip proves
// MultipleSensitivityResult (including its per-point Valid=false/Reason
// case for a non-positive multiple) marshals, unmarshals, and re-marshals
// to byte-identical output. This package previously had no serialization
// test coverage at all, despite every one of its result types being
// JSON-serializable output meant to flow into valuation/report's
// SensitivityData.
func TestMultipleSensitivityResult_JSONRoundTrip(t *testing.T) {
	result := MultipleSensitivity(500000, []float64{2.0, 2.5, 3.0, -1.0, 0})

	first, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	if !json.Valid(first) {
		t.Fatal("expected valid JSON output")
	}

	var roundTripped MultipleSensitivityResult
	if err := json.Unmarshal(first, &roundTripped); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	second, err := json.Marshal(roundTripped)
	if err != nil {
		t.Fatalf("re-marshal failed: %v", err)
	}
	if string(first) != string(second) {
		t.Fatal("MultipleSensitivityResult did not round-trip byte-for-byte through marshal -> unmarshal -> marshal")
	}
}

// TestMatrix_JSONRoundTrip proves EarningsMultipleMatrix's Matrix
// round-trips, including a column made entirely invalid by a non-positive
// multiple (Valid=false cells alongside valid ones in the same Matrix).
func TestMatrix_JSONRoundTrip(t *testing.T) {
	matrix := EarningsMultipleMatrix(
		[]EarningsScenario{
			{Label: "Downside", Earnings: 400000},
			{Label: "Base Case", Earnings: 500000},
			{Label: "Upside", Earnings: 600000},
		},
		[]float64{2.0, 3.0, -1.0},
	)

	first, err := json.Marshal(matrix)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var roundTripped Matrix
	if err := json.Unmarshal(first, &roundTripped); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	second, err := json.Marshal(roundTripped)
	if err != nil {
		t.Fatalf("re-marshal failed: %v", err)
	}
	if string(first) != string(second) {
		t.Fatal("Matrix did not round-trip byte-for-byte through marshal -> unmarshal -> marshal")
	}
}

// TestDCFGrid_JSONRoundTrip proves DCFSensitivity's DCFGrid round-trips,
// including a cell invalidated by discountRate <= terminalGrowthRate
// (Valid=false, no embedded dcf.Result) alongside valid cells that carry a
// full nested dcf.Result.
func TestDCFGrid_JSONRoundTrip(t *testing.T) {
	forecast := []dcf.ForecastPeriod{
		{Period: "2026", FreeCashFlow: 130000},
		{Period: "2027", FreeCashFlow: 140000},
		{Period: "2028", FreeCashFlow: 150000},
	}
	grid := DCFSensitivity(
		forecast,
		[]float64{0.18, 0.22, 0.02}, // 0.02 combined with a 0.03 terminal growth rate below is invalid
		[]float64{0.02, 0.03},
		dcf.EquityBridgeInput{Requested: true, ExcessCash: 50000, ShortTermDebt: 10000},
	)

	first, err := json.Marshal(grid)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var roundTripped DCFGrid
	if err := json.Unmarshal(first, &roundTripped); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	second, err := json.Marshal(roundTripped)
	if err != nil {
		t.Fatalf("re-marshal failed: %v", err)
	}
	if string(first) != string(second) {
		t.Fatal("DCFGrid did not round-trip byte-for-byte through marshal -> unmarshal -> marshal")
	}
}
