package qoe

import "testing"

// TestResolveThresholds_ZeroValueUsesDefaults proves the
// zero-Thresholds-means-defaults contract Calculate relies on.
func TestResolveThresholds_ZeroValueUsesDefaults(t *testing.T) {
	got := resolveThresholds(Thresholds{})
	want := DefaultThresholds()
	if got != want {
		t.Errorf("resolveThresholds(zero value) = %+v, want DefaultThresholds() = %+v", got, want)
	}
}

// TestResolveThresholds_PartialOverridePreserved proves a caller-supplied
// Thresholds with even one non-zero field is used exactly as given, never
// silently merged with defaults field-by-field (matching
// review.resolvePolicy's identical all-or-nothing contract).
func TestResolveThresholds_PartialOverridePreserved(t *testing.T) {
	custom := Thresholds{LargeNormalizationBurdenRatio: 0.75}
	got := resolveThresholds(custom)
	if got != custom {
		t.Errorf("resolveThresholds(custom) = %+v, want unchanged %+v", got, custom)
	}
	if got.VolatileEarningsRatio != 0 {
		t.Errorf("expected VolatileEarningsRatio to remain 0 (not silently defaulted), got %v", got.VolatileEarningsRatio)
	}
}
