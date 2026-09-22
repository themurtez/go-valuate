package classification

import (
	"testing"

	"github.com/themurtez/go-valuate/financial"
)

// TestZeroValue_Config_ClassifiesAsUnknownNotPanic proves classification.
// Config{} (no AliasLayers, no Rules, no ReviewThreshold) is a safe zero
// value: Classify never panics on it, and — per the package's own
// documented behavior — resolves an ordinary (non-structural) row to
// SourceUnknown rather than guessing.
func TestZeroValue_Config_ClassifiesAsUnknownNotPanic(t *testing.T) {
	result := Classify(financial.RawLineItem{ID: "r1", Label: "Advertising"}, Config{})
	if !result.IsUnknown() {
		t.Errorf("expected a zero-value Config to leave an ordinary row UNKNOWN, got Source=%s Code=%s", result.Source, result.Code)
	}
}

// TestZeroValue_Config_StructuralDetectionStillWorks proves the one
// exception the package documents: structural detection (via
// RawLineItem.Kind or the built-in label heuristic) still runs correctly
// even under a zero-value Config, since it does not depend on
// AliasLayers/Rules at all.
func TestZeroValue_Config_StructuralDetectionStillWorks(t *testing.T) {
	result := Classify(financial.RawLineItem{ID: "r1", Label: "Total Operating Expenses"}, Config{})
	if result.Source != SourceStructural {
		t.Errorf("expected zero-value Config to still detect a structural row by label heuristic, got Source=%s", result.Source)
	}
}

// TestZeroValue_Config_ClassifyBatchNilRules proves a nil Config.Rules
// slice (as opposed to DefaultRules()) is a safe no-op range in
// ClassifyBatch, not a nil-dereference.
func TestZeroValue_Config_ClassifyBatchNilRules(t *testing.T) {
	raws := []financial.RawLineItem{{ID: "r1", Label: "Something Ambiguous"}}
	results := ClassifyBatch(raws, Config{Rules: nil})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}
