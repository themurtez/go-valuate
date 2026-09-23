package covenants

import (
	"encoding/json"
	"testing"
)

// TestCalculate_Deterministic proves repeated execution against identical
// input returns byte-for-byte identical output, mirroring
// analytics/debt/determinism_test.go's TestCalculate_Deterministic.
func TestCalculate_Deterministic(t *testing.T) {
	in := Input{Tests: []CovenantTest{
		{
			CovenantID:           "MIN_DSCR",
			Label:                "Minimum DSCR",
			Metric:               MetricDSCR,
			Operator:             OperatorGTE,
			Threshold:            1.25,
			Actual:               AvailableValue(1.30),
			Period:               "2025-Q3",
			WarningBufferPercent: 0.10,
			CureGrace:            &CureGrace{Description: "10 business days", CureDays: 10, GraceDays: 5},
		},
		{
			CovenantID: "MAX_LEVERAGE",
			Metric:     MetricDebtToEBITDA,
			Operator:   OperatorLTE,
			Threshold:  4.0,
			Actual:     AvailableValue(4.5),
			Period:     "2025-Q3",
		},
		{
			CovenantID: "MIN_NET_WORTH",
			Metric:     MetricMinimumNetWorth,
			Operator:   OperatorGTE,
			Threshold:  1_000_000,
			Actual:     Unavailable(),
			Period:     "2025-Q3",
		},
		{
			CovenantID:        "MAX_OWNER_DIST",
			Metric:            MetricCustom,
			CustomMetricLabel: "Maximum Owner Distributions",
			Operator:          OperatorLTE,
			Threshold:         250_000,
			Actual:            AvailableValue(200_000),
			Period:            "2025-Q2",
		},
		{
			CovenantID: "BAD_OP",
			Operator:   Operator("~="),
			Threshold:  1,
			Actual:     AvailableValue(1),
			Period:     "2025-Q3",
		},
	}}

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
