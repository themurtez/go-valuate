package advisory

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/themurtez/go-valuate/accounting/cashforecast"
)

// TestJSONRoundTrip_Result confirms Result marshals and unmarshals back
// to an equal value — task section 101. Uses a populated Result (built
// via Build) rather than a hand-constructed literal, so every nested
// type (Section, Metric, Insight, Evidence, ActionItem, ExecutiveSummary,
// Snapshot, Coverage) is actually exercised.
func TestJSONRoundTrip_Result(t *testing.T) {
	in := Input{
		Company: CompanyContext{CompanyName: "Fixture Co", CurrentPeriod: "2026-02"},
		Operating: OperatingInputs{
			CashForecast: cashforecast.Result{
				Available: true, ForecastStartDate: "2026-02-01", HorizonWeeks: 13,
				BaseScenario: cashforecast.ScenarioResult{
					Summary: cashforecast.LiquiditySummary{
						EndingCash: 50000, LowestCashBalance: 10000, LowestCashWeek: 6,
						ThresholdAvailable: true, MinimumCashThreshold: 25000, WeeksBelowMinimum: 2,
					},
				},
			},
		},
	}
	original := Build(in, ExamplePolicy())

	b, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded Result
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	b2, err := json.Marshal(decoded)
	if err != nil {
		t.Fatalf("re-Marshal failed: %v", err)
	}

	if string(b) != string(b2) {
		t.Fatalf("round-trip is not stable:\n--- original ---\n%s\n--- after round-trip ---\n%s", b, b2)
	}
}

// TestJSONRoundTrip_NoNaNOrInf scans every float64 this package could
// plausibly emit for NaN/Inf via a battery of edge-case inputs (zero
// denominators, single-candidate tolerance checks, empty histories) —
// task section 101's "no NaN/Inf" requirement, and task section 126's
// fuzz goal restated as a fixed battery.
func TestJSONRoundTrip_NoNaNOrInf(t *testing.T) {
	cases := []Input{
		{}, // zero value
		{
			Company: CompanyContext{CurrentPeriod: "2026-01"},
			Operating: OperatingInputs{
				CashForecast: cashforecast.Result{
					Available: true, ForecastStartDate: "2026-01-01", HorizonWeeks: 1,
					BaseScenario: cashforecast.ScenarioResult{Summary: cashforecast.LiquiditySummary{EndingCash: 0, LowestCashBalance: 0}},
				},
			},
		},
	}

	for i, in := range cases {
		result := Build(in, Policy{})
		b, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("case %d: Marshal failed: %v", i, err)
		}
		var generic map[string]interface{}
		if err := json.Unmarshal(b, &generic); err != nil {
			t.Fatalf("case %d: Unmarshal failed: %v", i, err)
		}
		assertNoNaNOrInf(t, i, "$", generic)
	}
}

func assertNoNaNOrInf(t *testing.T, caseIdx int, path string, v interface{}) {
	t.Helper()
	switch val := v.(type) {
	case float64:
		if math.IsNaN(val) || math.IsInf(val, 0) {
			t.Errorf("case %d: NaN/Inf found at %s: %v", caseIdx, path, val)
		}
	case map[string]interface{}:
		for k, sub := range val {
			assertNoNaNOrInf(t, caseIdx, path+"."+k, sub)
		}
	case []interface{}:
		for i, sub := range val {
			assertNoNaNOrInf(t, caseIdx, path+"[]", sub)
			_ = i
		}
	}
}
