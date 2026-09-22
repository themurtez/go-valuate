package settings

import (
	"bytes"
	"encoding/json"
	"testing"
)

// TestResolution_JSONRoundTrip proves Resolution's Values/Sources maps —
// the module's highest-risk JSON-ordering target, since
// map[string]any/map[string]Scope are marshaled directly with no explicit
// sorting in this package's own code — serialize deterministically and
// round-trip marshal -> unmarshal -> marshal to byte-identical output.
func TestResolution_JSONRoundTrip(t *testing.T) {
	system := Settings{
		SDEMultiple:   Float64(2.5),
		DiscountRate:  Float64(0.15),
		TaxRate:       Float64(0.21),
		MethodEnabled: map[Method]*bool{MethodSDE: Bool(true), MethodDCF: Bool(false)},
	}
	account := Settings{ControlPremium: Float64(0.1)}
	client := Settings{MarketabilityDiscount: Float64(0.2)}
	valuation := Settings{EBITDAMultiple: Float64(4.0)}

	res := Resolve(system, account, client, valuation)

	first, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	if !json.Valid(first) {
		t.Fatal("expected valid JSON output")
	}

	var roundTripped Resolution
	if err := json.Unmarshal(first, &roundTripped); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	second, err := json.Marshal(roundTripped)
	if err != nil {
		t.Fatalf("re-marshal failed: %v", err)
	}
	if string(first) != string(second) {
		t.Fatal("Resolution did not round-trip byte-for-byte through marshal -> unmarshal -> marshal")
	}

	// Repeat-marshal determinism: the same Resolution marshaled many times
	// in a row must always produce identical bytes — the map-key-sort
	// guarantee under direct, explicit test rather than left incidental.
	for i := 0; i < 10; i++ {
		got, err := json.Marshal(res)
		if err != nil {
			t.Fatalf("run %d: json.Marshal failed: %v", i, err)
		}
		if string(got) != string(first) {
			t.Fatalf("run %d: repeated marshal of the same Resolution produced different bytes", i)
		}
	}
}

// TestResolution_JSONKeysAreSortedInValuesAndSources proves the two map
// fields' keys actually appear in ascending alphabetical order in the raw
// JSON bytes — not merely that two marshals agree with each other (which
// TestResolution_JSONRoundTrip already proves), but that the order is the
// specific, predictable one a consumer/report would expect.
func TestResolution_JSONKeysAreSortedInValuesAndSources(t *testing.T) {
	system := Settings{
		TaxRate:               Float64(0.21),
		SDEMultiple:           Float64(2.5),
		ControlPremium:        Float64(0.1),
		MarketabilityDiscount: Float64(0.2),
	}
	res := Resolve(system, Settings{}, Settings{}, Settings{})

	valuesData, err := json.Marshal(res.Values)
	if err != nil {
		t.Fatalf("json.Marshal(Values) failed: %v", err)
	}
	sourcesData, err := json.Marshal(res.Sources)
	if err != nil {
		t.Fatalf("json.Marshal(Sources) failed: %v", err)
	}

	assertKeysSorted(t, "values", topLevelObjectKeys(t, valuesData))
	assertKeysSorted(t, "sources", topLevelObjectKeys(t, sourcesData))
}

// topLevelObjectKeys decodes data (expected to be a single JSON object) and
// returns its keys in the order they appeared in the byte stream, using
// json.Decoder's token-by-token API so a key is never mistaken for one
// nested inside a value at a deeper level.
func topLevelObjectKeys(t *testing.T, data []byte) []string {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(data))

	tok, err := dec.Token()
	if err != nil {
		t.Fatalf("decoding opening token: %v", err)
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		t.Fatalf("expected a top-level JSON object, got token %v", tok)
	}

	var keys []string
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			t.Fatalf("decoding key token: %v", err)
		}
		key, ok := keyTok.(string)
		if !ok {
			t.Fatalf("expected a string key, got %v", keyTok)
		}
		keys = append(keys, key)

		var discard json.RawMessage
		if err := dec.Decode(&discard); err != nil {
			t.Fatalf("skipping value for key %q: %v", key, err)
		}
	}
	return keys
}

func assertKeysSorted(t *testing.T, label string, keys []string) {
	t.Helper()
	if len(keys) < 2 {
		t.Fatalf("%s: expected at least 2 keys to meaningfully test ordering, got %v", label, keys)
	}
	for i := 1; i < len(keys); i++ {
		if keys[i-1] > keys[i] {
			t.Errorf("%s: keys not sorted ascending: %q appears before %q in %v", label, keys[i-1], keys[i], keys)
		}
	}
}
