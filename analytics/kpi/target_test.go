package kpi

import "testing"

func TestTarget_Minimum(t *testing.T) {
	// Gross Margin >= 35% -- task section 22's worked example.
	policy := TargetPolicy{Kind: TargetMinimum, Min: 35}
	met := evaluateTarget(policy, true, availableValue(40))
	if !met.TargetAvailable || !met.TargetMet {
		t.Fatalf("40 >= 35 should be met, got %+v", met)
	}
	notMet := evaluateTarget(policy, true, availableValue(30))
	if !notMet.TargetAvailable || notMet.TargetMet {
		t.Fatalf("30 >= 35 should not be met, got %+v", notMet)
	}
}

func TestTarget_Maximum(t *testing.T) {
	// DSO <= 45 -- task section 22's worked example.
	policy := TargetPolicy{Kind: TargetMaximum, Max: 45}
	met := evaluateTarget(policy, true, availableValue(40))
	if !met.TargetAvailable || !met.TargetMet {
		t.Fatalf("40 <= 45 should be met, got %+v", met)
	}
	notMet := evaluateTarget(policy, true, availableValue(50))
	if notMet.TargetMet {
		t.Fatalf("50 <= 45 should not be met, got %+v", notMet)
	}
}

func TestTarget_Range(t *testing.T) {
	// Utilization between 70% and 85% -- task section 22's worked example.
	policy := TargetPolicy{Kind: TargetRange, Min: 70, Max: 85}
	for _, tc := range []struct {
		v    float64
		want bool
	}{{75, true}, {70, true}, {85, true}, {69.9, false}, {85.1, false}} {
		got := evaluateTarget(policy, true, availableValue(tc.v))
		if got.TargetMet != tc.want {
			t.Fatalf("value %v: TargetMet=%v, want %v", tc.v, got.TargetMet, tc.want)
		}
	}
}

func TestTarget_Exact_Boundaries(t *testing.T) {
	policy := TargetPolicy{Kind: TargetExact, Exact: 100, Tolerance: 5}
	if !evaluateTarget(policy, true, availableValue(105)).TargetMet {
		t.Fatalf("105 within tolerance 5 of 100 should be met")
	}
	if evaluateTarget(policy, true, availableValue(106)).TargetMet {
		t.Fatalf("106 outside tolerance 5 of 100 should not be met")
	}
}

func TestTarget_Unavailable_WhenValueUnavailable(t *testing.T) {
	policy := TargetPolicy{Kind: TargetMinimum, Min: 35}
	got := evaluateTarget(policy, true, unavailableValue(AvailabilityMissingMetric))
	if got.TargetAvailable {
		t.Fatalf("target should be unavailable when the KPI's own value is unavailable, got %+v", got)
	}
}

func TestTarget_Unavailable_WhenNotConfigured(t *testing.T) {
	got := evaluateTarget(TargetPolicy{}, false, availableValue(100))
	if got.TargetAvailable {
		t.Fatalf("target should be unavailable when not configured, got %+v", got)
	}
}

func TestTarget_InvalidRange_MinGreaterThanMax(t *testing.T) {
	policy := TargetPolicy{Kind: TargetRange, Min: 90, Max: 10}
	if policy.valid() {
		t.Fatalf("Min > Max range should be invalid")
	}
}

// --- Threshold bands ---

func TestBands_Basic(t *testing.T) {
	bands := []ThresholdBand{
		{Label: "LOW", Min: 0, Max: 50},
		{Label: "MID", Min: 50, Max: 80},
		{Label: "HIGH", Min: 80, Max: 100},
	}
	ok, overlapping, degenerate := validateBands(bands)
	if !ok || overlapping || degenerate {
		t.Fatalf("expected valid non-overlapping bands, got ok=%v overlapping=%v degenerate=%v", ok, overlapping, degenerate)
	}
	if b, found := bandFor(bands, 25); !found || b.Label != "LOW" {
		t.Fatalf("25 should be LOW, got %+v found=%v", b, found)
	}
	if b, found := bandFor(bands, 50); !found || b.Label != "MID" {
		t.Fatalf("50 should be MID (Min inclusive), got %+v found=%v", b, found)
	}
	// Top band's own upper boundary IS reachable (closed on both ends).
	if b, found := bandFor(bands, 100); !found || b.Label != "HIGH" {
		t.Fatalf("100 (top band's own Max) should be HIGH, got %+v found=%v", b, found)
	}
	if b, found := bandFor(bands, 80); !found || b.Label != "HIGH" {
		t.Fatalf("80 should be HIGH (Min inclusive), got %+v found=%v", b, found)
	}
}

func TestBands_OverlappingRejected(t *testing.T) {
	bands := []ThresholdBand{{Label: "A", Min: 0, Max: 60}, {Label: "B", Min: 50, Max: 100}}
	ok, overlapping, _ := validateBands(bands)
	if ok || !overlapping {
		t.Fatalf("expected overlapping bands rejected, got ok=%v overlapping=%v", ok, overlapping)
	}
}

func TestBands_Degenerate(t *testing.T) {
	bands := []ThresholdBand{{Label: "BAD", Min: 50, Max: 50}}
	ok, _, degenerate := validateBands(bands)
	if ok || !degenerate {
		t.Fatalf("expected degenerate band rejected, got ok=%v degenerate=%v", ok, degenerate)
	}
}

func TestBands_KPIResult_Integration(t *testing.T) {
	def := Definition{
		Code: "k", Unit: Unit{Kind: UnitPercent}, Formula: Metric(MetricRef{Code: "margin"}),
		ThresholdBands: []ThresholdBand{{Label: "LOW", Min: 0, Max: 50}, {Label: "HIGH", Min: 50, Max: 100}},
	}
	res := Calculate(onePeriodInput([]Definition{def}, []MetricValue{
		metricInput("margin", "P1", 60, true, Unit{Kind: UnitPercent}),
	}), Options{})
	kr := firstResult(t, res, "k")
	if !kr.BandAvailable || kr.Band.Label != "HIGH" {
		t.Fatalf("got Band=%+v BandAvailable=%v", kr.Band, kr.BandAvailable)
	}
}

func TestBands_OverlappingRejected_AtDefinitionLevel(t *testing.T) {
	def := Definition{
		Code: "k", Unit: Unit{Kind: UnitPercent}, Formula: Const(50),
		ThresholdBands: []ThresholdBand{{Label: "A", Min: 0, Max: 60}, {Label: "B", Min: 50, Max: 100}},
	}
	res := Calculate(onePeriodInput([]Definition{def}, nil), Options{})
	found := false
	for _, i := range res.DefinitionIssues {
		if i.Code == IssueOverlappingThresholdBands {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected IssueOverlappingThresholdBands, got %v", res.DefinitionIssues)
	}
}
