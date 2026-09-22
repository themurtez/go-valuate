package anomalies

import (
	"encoding/json"
	"testing"

	"github.com/themurtez/go-valuate/financial"
)

// TestResult_JSONRoundTrip proves Result marshals, unmarshals, and
// re-marshals to byte-identical output, exercising every nested type
// (Anomalies with every optional field populated, Summary, Thresholds,
// Warnings) — mirroring concentration/roundtrip_test.go.
func TestResult_JSONRoundTrip(t *testing.T) {
	in := Input{
		Dataset: unionDataset(
			missingPeriodDataset(),
			repeatedValueDataset(),
			duplicateAmountsDataset(),
			ownerDiscretionaryDataset(),
			unexpectedNegativeDataset(),
		),
		PeriodMeta:         fourYearMeta(),
		AccountGroups:      []AccountGroup{{Name: "G&A", Codes: []financial.Code{financial.CodeOpexSoftware}}},
		DiscretionaryCodes: []financial.Code{financial.CodeOpexVehicle},
	}
	res := Calculate(in, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	if len(res.Anomalies) == 0 {
		t.Fatal("expected at least one anomaly to exercise every field in this round-trip")
	}

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

// TestResult_JSONRoundTrip_Unavailable proves the zero-anomaly
// Available == false shape also round-trips cleanly.
func TestResult_JSONRoundTrip_Unavailable(t *testing.T) {
	res := Calculate(Input{}, Options{})
	first, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
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
		t.Fatal("unavailable Result did not round-trip byte-for-byte")
	}
}

// unionDataset merges several FinancialDataset fixtures into one, for tests
// that want a single dataset exercising several rules at once. Later
// datasets' items for the same (code, period) win, matching how
// financial.FinancialDataset.ByCodeAndPeriod's first-match semantics would
// otherwise behave if duplicates were left in place — this helper instead
// de-duplicates outright so no (code, period) pair appears twice, keeping
// the merged dataset a valid non-ambiguous FinancialDataset.
func unionDataset(datasets ...financial.FinancialDataset) financial.FinancialDataset {
	type key struct {
		code   financial.Code
		period financial.Period
	}
	byKey := make(map[key]financial.NormalizedItem)
	var order []key
	for _, ds := range datasets {
		for _, it := range ds.Items {
			k := key{it.Code, it.Period}
			if _, ok := byKey[k]; !ok {
				order = append(order, k)
			}
			byKey[k] = it
		}
	}
	items := make([]financial.NormalizedItem, 0, len(order))
	for _, k := range order {
		items = append(items, byKey[k])
	}
	return financial.FinancialDataset{Currency: "USD", Items: items}
}
