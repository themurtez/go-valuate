package settings

import "testing"

// TestZeroValue_AllFourLayers_ResolvesToEmptyNotPanic proves
// Resolve(Settings{}, Settings{}, Settings{}, Settings{}) — every pointer
// field nil at every scope — is a safe, meaningful zero value: an empty
// Resolution (nothing resolved anywhere), not a panic. This is the
// documented "no scope has an opinion" case.
func TestZeroValue_AllFourLayers_ResolvesToEmptyNotPanic(t *testing.T) {
	res := Resolve(Settings{}, Settings{}, Settings{}, Settings{})
	if len(res.Values) != 0 {
		t.Errorf("expected an empty Values map when every scope is entirely unset, got %+v", res.Values)
	}
	if len(res.Sources) != 0 {
		t.Errorf("expected an empty Sources map when every scope is entirely unset, got %+v", res.Sources)
	}
	if res.SchemaVersion == "" {
		t.Error("expected SchemaVersion to be populated even for an all-empty resolution")
	}
}

// TestZeroValue_MethodEnabled_NilMapMeansEveryMethodEnabled proves a nil
// Settings.MethodEnabled map (the zero value) resolves as "no method is
// explicitly disabled" rather than panicking on a nil-map read.
func TestZeroValue_MethodEnabled_NilMapMeansEveryMethodEnabled(t *testing.T) {
	res := Resolve(Settings{MethodEnabled: nil}, Settings{}, Settings{}, Settings{})
	if _, ok := res.Sources[MethodFieldKey(MethodSDE)]; ok {
		t.Error("expected no resolved value for a method key nobody set at any scope")
	}
}
