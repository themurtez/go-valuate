package financial

import (
	"encoding/json"
	"testing"
)

// TestRawLineItem_KindJSONRoundTrip proves RawLineItem.Kind marshals,
// unmarshals, and re-marshals to byte-identical output, for both a
// zero-value Kind (must be omitted entirely per omitempty) and each
// non-zero RowKind value.
func TestRawLineItem_KindJSONRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		kind RowKind
	}{
		{"zero value", RowKindNormal},
		{"heading", RowKindHeading},
		{"subtotal", RowKindSubtotal},
		{"total", RowKindTotal},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			item := RawLineItem{
				ID:            "row-1",
				StatementType: StatementIncomeStatement,
				Label:         "Gross Profit",
				Kind:          tc.kind,
				Values:        map[Period]float64{"2025": 100},
			}

			first, err := json.Marshal(item)
			if err != nil {
				t.Fatalf("json.Marshal failed: %v", err)
			}

			var decoded RawLineItem
			if err := json.Unmarshal(first, &decoded); err != nil {
				t.Fatalf("json.Unmarshal failed: %v", err)
			}
			second, err := json.Marshal(decoded)
			if err != nil {
				t.Fatalf("re-marshal failed: %v", err)
			}
			if string(first) != string(second) {
				t.Fatalf("RawLineItem did not round-trip byte-for-byte:\nfirst:  %s\nsecond: %s", first, second)
			}
			if decoded.Kind != tc.kind {
				t.Errorf("decoded Kind = %q, want %q", decoded.Kind, tc.kind)
			}
		})
	}
}

// TestRawLineItem_KindZeroValueOmittedFromJSON confirms the zero-value
// Kind (RowKindNormal, meaning "no upstream structural signal supplied")
// never appears in marshaled JSON, matching every other zero-value-omitted
// field in this package (e.g. MappedLineItem.Status) and preserving
// backward compatibility for any existing consumer that deserializes a
// RawLineItem produced before this field existed.
func TestRawLineItem_KindZeroValueOmittedFromJSON(t *testing.T) {
	item := RawLineItem{ID: "row-1", Label: "Revenue", Values: map[Period]float64{"2025": 100}}
	b, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	if _, present := raw["kind"]; present {
		t.Errorf("expected \"kind\" to be omitted from JSON for zero-value Kind, got %s", b)
	}
}

// TestMappedLineItem_KindJSONRoundTrip mirrors
// TestRawLineItem_KindJSONRoundTrip for MappedLineItem.Kind.
func TestMappedLineItem_KindJSONRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		kind RowKind
	}{
		{"zero value", RowKindNormal},
		{"heading", RowKindHeading},
		{"subtotal", RowKindSubtotal},
		{"total", RowKindTotal},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			item := MappedLineItem{
				SourceID:      "row-1",
				Label:         "Gross Profit",
				StatementType: StatementIncomeStatement,
				Status:        RowStatusSubtotal,
				Kind:          tc.kind,
				Values:        map[Period]float64{"2025": 100},
			}

			first, err := json.Marshal(item)
			if err != nil {
				t.Fatalf("json.Marshal failed: %v", err)
			}

			var decoded MappedLineItem
			if err := json.Unmarshal(first, &decoded); err != nil {
				t.Fatalf("json.Unmarshal failed: %v", err)
			}
			second, err := json.Marshal(decoded)
			if err != nil {
				t.Fatalf("re-marshal failed: %v", err)
			}
			if string(first) != string(second) {
				t.Fatalf("MappedLineItem did not round-trip byte-for-byte:\nfirst:  %s\nsecond: %s", first, second)
			}
			if decoded.Kind != tc.kind {
				t.Errorf("decoded Kind = %q, want %q", decoded.Kind, tc.kind)
			}
		})
	}
}

// TestMappedLineItem_KindZeroValueOmittedFromJSON mirrors
// TestRawLineItem_KindZeroValueOmittedFromJSON for MappedLineItem.
func TestMappedLineItem_KindZeroValueOmittedFromJSON(t *testing.T) {
	item := MappedLineItem{SourceID: "row-1", Code: CodeRevProduct, Values: map[Period]float64{"2025": 100}}
	b, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	if _, present := raw["kind"]; present {
		t.Errorf("expected \"kind\" to be omitted from JSON for zero-value Kind, got %s", b)
	}
}

// TestNormalize_HeadingKindIgnoredRowExcluded confirms a MappedLineItem
// carrying Kind == RowKindHeading (via Status == RowStatusIgnored, the
// normalize-time directive classification.Classify derives from a heading
// Kind) is excluded from aggregation exactly like any other ignored row —
// Normalize itself required no code change for this, since it already
// switches on Status, not Kind; this test exists to pin that contract in
// place for the RowKind field specifically.
func TestNormalize_HeadingKindIgnoredRowExcluded(t *testing.T) {
	items := []MappedLineItem{
		{SourceID: "row-1", Code: CodeOpexMarketing, Status: RowStatusNormal, Values: map[Period]float64{"2025": 5000}},
		{SourceID: "row-2", Label: "Operating Expenses", Status: RowStatusIgnored, Kind: RowKindHeading, Values: map[Period]float64{}},
	}

	ds, err := Normalize(items, NormalizeOptions{Currency: "USD"})
	if err != nil {
		t.Fatalf("Normalize returned error: %v", err)
	}
	if len(ds.Items) != 1 {
		t.Fatalf("expected 1 normalized item (heading excluded), got %d: %+v", len(ds.Items), ds.Items)
	}
	item, ok := ds.ByCodeAndPeriod(CodeOpexMarketing, "2025")
	if !ok || item.Amount != 5000 {
		t.Errorf("OPEX_MARKETING/2025 = %+v, want amount 5000", item)
	}
}
