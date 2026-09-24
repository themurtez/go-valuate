package closechecklist_test

import (
	"encoding/json"
	"testing"

	"github.com/themurtez/go-valuate/accounting/closechecklist"
	"github.com/themurtez/go-valuate/accounting/closechecklist/fixtures"
)

// TestRoundTrip_Result marshals then unmarshals a Result and checks the
// re-marshaled bytes match — section 41's "round-trip all public
// contracts."
func TestRoundTrip_Result(t *testing.T) {
	res := closechecklist.Calculate(fixtures.CleanInstance(), closechecklist.DefaultPolicy())
	roundTripJSON(t, res)
}

func TestRoundTrip_Instance(t *testing.T) {
	roundTripJSON(t, fixtures.CleanInstance())
}

func TestRoundTrip_Template(t *testing.T) {
	roundTripJSON(t, closechecklist.ServiceBusinessMonthlyClose())
	roundTripJSON(t, closechecklist.InventoryBusinessMonthlyClose())
}

func TestRoundTrip_Policy(t *testing.T) {
	roundTripJSON(t, closechecklist.DefaultPolicy())
}

func roundTripJSON[T any](t *testing.T, v T) {
	t.Helper()
	b1, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var v2 T
	if err := json.Unmarshal(b1, &v2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	b2, err := json.Marshal(v2)
	if err != nil {
		t.Fatalf("re-marshal: %v", err)
	}
	if string(b1) != string(b2) {
		t.Errorf("round-trip mismatch:\nfirst:  %s\nsecond: %s", b1, b2)
	}
}
