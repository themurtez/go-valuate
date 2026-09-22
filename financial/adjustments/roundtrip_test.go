package adjustments

import (
	"encoding/json"
	"testing"

	"github.com/themurtez/go-valuate/financial/metrics"
)

// TestResult_JSONRoundTrip proves adjustments.Result marshals, unmarshals,
// and re-marshals to byte-identical output. OriginalSnapshot embeds
// metrics.Snapshot (which carries the module's Results map[string]MetricResult
// field — see financial/metrics' own key-sorting test), so this also
// exercises that map field one level deeper than its own package's tests.
func TestResult_JSONRoundTrip(t *testing.T) {
	snapshot := metrics.Snapshot{
		Period: "2025",
		EBITDA: metrics.AvailableValue(500000),
		SDE:    metrics.AvailableValue(560000),
	}
	adjs := []Adjustment{
		{ID: "adj-1", Period: "2025", Type: TypePersonalVehicle, Amount: 7200, Reason: "owner's personal vehicle", Included: true},
	}
	res := Apply(snapshot, adjs)

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
