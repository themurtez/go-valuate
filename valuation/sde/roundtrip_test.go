package sde

import (
	"encoding/json"
	"testing"
)

// TestResult_JSONRoundTrip proves Result marshals, unmarshals, and
// re-marshals to byte-identical output.
func TestResult_JSONRoundTrip(t *testing.T) {
	res := Calculate(Input{
		MaintainableSDE: 218800, Multiple: 2.5,
		EquityBridge: EquityBridgeInput{Requested: true, ExcessCash: 50000, ShortTermDebt: 10000, LongTermDebt: 90000},
	})

	first, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	if !json.Valid(first) {
		t.Fatal("expected valid JSON output")
	}

	var roundTripped Result
	if err := json.Unmarshal(first, &roundTripped); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	second, err := json.Marshal(roundTripped)
	if err != nil {
		t.Fatalf("re-marshal failed: %v", err)
	}
	if string(first) != string(second) {
		t.Fatal("Result did not round-trip byte-for-byte through marshal -> unmarshal -> marshal")
	}
}
