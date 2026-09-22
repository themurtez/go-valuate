package adjustments

import (
	"encoding/json"
	"testing"

	"github.com/themurtez/go-valuate/financial/metrics"
)

// TestApply_Deterministic proves repeated execution against identical
// input returns byte-for-byte identical output.
func TestApply_Deterministic(t *testing.T) {
	snapshot := metrics.Snapshot{
		Period: "2025",
		EBITDA: metrics.AvailableValue(500000),
		SDE:    metrics.AvailableValue(560000),
	}
	adjs := []Adjustment{
		{
			ID: "adj-1", Period: "2025", Type: TypePersonalVehicle,
			Amount: 7200, Reason: "owner's personal vehicle", Included: true,
		},
		{
			ID: "adj-2", Period: "2025", Type: TypeOwnerCompensationNormalization,
			Amount: 32000, Effect: EffectDecrease, Targets: []Target{TargetSDE},
			Reason: "normalize actual draw to market-rate replacement salary", Included: true,
		},
	}

	first, err := json.Marshal(Apply(snapshot, adjs))
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	for i := 0; i < 10; i++ {
		got, err := json.Marshal(Apply(snapshot, adjs))
		if err != nil {
			t.Fatalf("run %d: json.Marshal failed: %v", i, err)
		}
		if string(got) != string(first) {
			t.Fatalf("run %d: Apply output differs from the first run", i)
		}
	}
}
