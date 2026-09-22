package ingestion_test

import (
	"strings"
	"testing"

	"github.com/themurtez/go-valuate/ingestion"
	"github.com/themurtez/go-valuate/ingestion/csv"
)

// TestZeroValue_Options_CSVParseDoesNotPanic proves ingestion.Options{}
// (no Locale, no DashTreatment, no Limits) is a safe, usable zero value —
// csv.Parse fills every unset field with a documented default
// (DefaultLimits() etc.) rather than requiring a caller to always build a
// fully-populated Options.
func TestZeroValue_Options_CSVParseDoesNotPanic(t *testing.T) {
	result, err := csv.Parse(strings.NewReader("Label,2025\nRevenue,100\n"), ingestion.Options{})
	if err != nil {
		t.Fatalf("expected a zero-value Options to parse cleanly, got error: %+v", err)
	}
	if result == nil {
		t.Fatal("expected a non-nil Result")
	}
}

// TestZeroValue_Limits_AllFieldsDefaulted proves ingestion.Limits{} (every
// bound left at 0) does not silently produce a zero-row-limit/zero-size-
// limit parse — csv.Parse must apply DefaultLimits()-equivalent bounds
// rather than treating a zero limit as "allow nothing."
func TestZeroValue_Limits_AllFieldsDefaulted(t *testing.T) {
	result, err := csv.Parse(strings.NewReader("Label,2025\nRevenue,100\nCOGS,50\n"), ingestion.Options{Limits: ingestion.Limits{}})
	if err != nil {
		t.Fatalf("expected zero-value Limits to fall back to defaults, got error: %+v", err)
	}
	if len(result.Rows) != 2 {
		t.Fatalf("expected 2 parsed rows under zero-value Limits, got %d", len(result.Rows))
	}
}
