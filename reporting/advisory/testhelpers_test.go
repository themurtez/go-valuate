package advisory

import (
	"encoding/json"
	"testing"
)

// mustJSON marshals v to a stable JSON string for equality comparison in
// determinism/immutability tests, failing the test on marshal error.
func mustJSON(t *testing.T, v interface{}) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	return string(b)
}
