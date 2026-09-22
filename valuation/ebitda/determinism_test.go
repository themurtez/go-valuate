package ebitda

import (
	"encoding/json"
	"testing"
)

// TestCalculate_Deterministic proves repeated execution against identical
// input returns byte-for-byte identical output.
func TestCalculate_Deterministic(t *testing.T) {
	in := Input{
		MaintainableEBITDA: 126800, Multiple: 3.5,
		EquityBridge: EquityBridgeInput{Requested: true, ExcessCash: 63000, ShortTermDebt: 8000, LongTermDebt: 47000},
	}
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
