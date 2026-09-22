package ai

import (
	"encoding/json"
	"testing"

	"github.com/themurtez/go-valuate/financial/adjustments"
)

func mustMarshalAny(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	return string(data)
}

// TestJSON_ProvenanceRoundTrips proves Provenance survives a JSON
// marshal/unmarshal cycle byte-for-byte in its meaningful fields, and that
// RequestSchemaVersion/OrchestrationVersion are both actually present on the
// wire — the audit trail this package's provenance record exists for
// depends on both surviving serialization intact.
func TestJSON_ProvenanceRoundTrips(t *testing.T) {
	orig := Provenance{
		Provider: "openai", Model: "gpt-4o-mini", AdapterVersion: "1.0.0",
		RequestSchemaVersion: RequestSchemaVersion, OrchestrationVersion: OrchestrationVersion,
		CandidateRowCount: 3, RawSuggestionCount: 2, ValidSuggestionCount: 1, RejectedCount: 1,
	}
	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var got Provenance
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if got != orig {
		t.Fatalf("round-trip mismatch: got %+v, want %+v", got, orig)
	}
}

func TestJSON_SuggestionRoundTrips(t *testing.T) {
	conf := 0.82
	orig := Suggestion{
		SourceRowID: "row-27", Period: "2025", AdjustmentType: adjustments.TypeOneTimeExpense,
		Amount: 42000, Direction: DirectionIncreaseEarnings, Reason: "relocation expense",
		Confidence: &conf, RequiresUserInput: false,
	}
	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var got Suggestion
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if got.SourceRowID != orig.SourceRowID || got.Period != orig.Period || got.AdjustmentType != orig.AdjustmentType ||
		got.Amount != orig.Amount || got.Direction != orig.Direction || got.Reason != orig.Reason ||
		got.RequiresUserInput != orig.RequiresUserInput {
		t.Fatalf("round-trip mismatch: got %+v, want %+v", got, orig)
	}
	if got.Confidence == nil || *got.Confidence != conf {
		t.Fatalf("expected confidence to round-trip, got %v", got.Confidence)
	}
}

// TestJSON_ExampleConceptualResponseShapeParses pins the task brief's
// conceptual response JSON shape (section 8) against this package's actual
// Response type, so a future field rename here would be caught immediately.
func TestJSON_ExampleConceptualResponseShapeParses(t *testing.T) {
	raw := `{
		"suggestions": [
			{
				"source_row_id": "row-27",
				"period": "2025",
				"adjustment_type": "one_time_expense",
				"amount": 42000,
				"direction": "INCREASE_EARNINGS",
				"reason": "The label indicates a one-time relocation expense.",
				"confidence": 0.78,
				"requires_user_input": false
			}
		]
	}`
	var resp Response
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("failed to parse conceptual response shape: %v", err)
	}
	if len(resp.Suggestions) != 1 {
		t.Fatalf("expected 1 suggestion, got %d", len(resp.Suggestions))
	}
	s := resp.Suggestions[0]
	if s.SourceRowID != "row-27" || s.AdjustmentType != adjustments.TypeOneTimeExpense || s.Amount != 42000 {
		t.Fatalf("unexpected parsed suggestion: %+v", s)
	}
}

func TestVersions_AreNonEmpty(t *testing.T) {
	if RequestSchemaVersion == "" {
		t.Error("RequestSchemaVersion must not be empty")
	}
	if OrchestrationVersion == "" {
		t.Error("OrchestrationVersion must not be empty")
	}
}
