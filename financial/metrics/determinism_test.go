package metrics

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/themurtez/go-valuate/financial"
)

// TestCalculate_Deterministic proves repeated execution against identical
// input returns byte-for-byte identical output — including Snapshot.Results
// map[string]MetricResult, whose keys must sort consistently on every
// marshal (encoding/json sorts string map keys automatically, but this
// test asserts that guarantee holds rather than relying on it silently).
func TestCalculate_Deterministic(t *testing.T) {
	ds := financial.FinancialDataset{
		Currency: "USD",
		Items: []financial.NormalizedItem{
			{Code: financial.CodeRevProduct, Period: "2023", Amount: 900000},
			{Code: financial.CodeRevProduct, Period: "2024", Amount: 1000000},
			{Code: financial.CodeRevProduct, Period: "2025", Amount: 1100000},
			{Code: financial.CodeCogsMaterial, Period: "2023", Amount: 400000},
			{Code: financial.CodeCogsMaterial, Period: "2024", Amount: 420000},
			{Code: financial.CodeCogsMaterial, Period: "2025", Amount: 440000},
			{Code: financial.CodeOpexPayroll, Period: "2023", Amount: 200000},
			{Code: financial.CodeOpexPayroll, Period: "2024", Amount: 210000},
			{Code: financial.CodeOpexPayroll, Period: "2025", Amount: 220000},
		},
	}
	periodMeta := map[financial.Period]PeriodInfo{
		"2023": {Type: PeriodTypeFiscalYear, FiscalYear: 2023},
		"2024": {Type: PeriodTypeFiscalYear, FiscalYear: 2024},
		"2025": {Type: PeriodTypeFiscalYear, FiscalYear: 2025},
	}

	first, err := json.Marshal(Calculate(ds, Options{PeriodMeta: periodMeta}))
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	for i := 0; i < 10; i++ {
		got, err := json.Marshal(Calculate(ds, Options{PeriodMeta: periodMeta}))
		if err != nil {
			t.Fatalf("run %d: json.Marshal failed: %v", i, err)
		}
		if string(got) != string(first) {
			t.Fatalf("run %d: Calculate output differs from the first run", i)
		}
	}
}

// TestSnapshot_ResultsMapKeysAreSorted proves Snapshot.Results
// (map[string]MetricResult, keyed by metric name — see the README's
// Explainability section) serializes with its top-level keys in a fixed,
// predictable (ascending alphabetical) order — the one raw map field in
// this package's exported output. Uses json.Decoder's token stream rather
// than a naive substring/regex scan so nested object keys inside each
// MetricResult (e.g. "available", "formula") are never mistaken for
// top-level Results keys.
func TestSnapshot_ResultsMapKeysAreSorted(t *testing.T) {
	ds := financial.FinancialDataset{
		Currency: "USD",
		Items: []financial.NormalizedItem{
			{Code: financial.CodeRevProduct, Period: "2025", Amount: 1000000},
			{Code: financial.CodeCogsMaterial, Period: "2025", Amount: 400000},
			{Code: financial.CodeOpexPayroll, Period: "2025", Amount: 200000},
			{Code: financial.CodeDepreciation, Period: "2025", Amount: 20000},
		},
	}
	res := Calculate(ds, Options{})
	if len(res.Snapshots) != 1 || len(res.Snapshots[0].Results) < 2 {
		t.Fatalf("expected at least 1 snapshot with 2+ Results entries to meaningfully test ordering, got %+v", res)
	}

	data, err := json.Marshal(res.Snapshots[0].Results)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	keys := topLevelObjectKeys(t, data)
	if len(keys) < 2 {
		t.Fatalf("expected at least 2 top-level keys, got %v", keys)
	}
	for i := 1; i < len(keys); i++ {
		if keys[i-1] > keys[i] {
			t.Errorf("Results map keys not sorted ascending: %q appears before %q in %v", keys[i-1], keys[i], keys)
		}
	}
}

// topLevelObjectKeys decodes data (expected to be a single JSON object) and
// returns its keys in the order they appeared in the byte stream, using
// json.Decoder's token-by-token API so nested object keys at deeper levels
// are correctly skipped rather than mistaken for top-level keys.
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

		// Skip this key's value entirely (whatever shape it is) by
		// decoding it into a discarded json.RawMessage.
		var discard json.RawMessage
		if err := dec.Decode(&discard); err != nil {
			t.Fatalf("skipping value for key %q: %v", key, err)
		}
	}
	return keys
}
