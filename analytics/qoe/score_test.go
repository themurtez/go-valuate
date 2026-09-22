package qoe

import "testing"

// TestLabelForScore_Bands proves labelForScore's four fixed bands match
// their documented boundaries exactly, including the boundary values
// themselves (85 and 65 and 40 are each in the higher band, per the >=
// comparisons in labelForScore).
func TestLabelForScore_Bands(t *testing.T) {
	cases := []struct {
		value float64
		want  string
	}{
		{100, "high quality"},
		{85, "high quality"},
		{84.999, "moderate quality"},
		{65, "moderate quality"},
		{64.999, "elevated concern"},
		{40, "elevated concern"},
		{39.999, "low quality"},
		{0, "low quality"},
	}
	for _, c := range cases {
		if got := labelForScore(c.value); got != c.want {
			t.Errorf("labelForScore(%v) = %q, want %q", c.value, got, c.want)
		}
	}
}

// TestScore_NotComputedByDefault proves Options.ComputeScore's opt-in
// behavior: the zero Options leaves Result.Score nil.
func TestScore_NotComputedByDefault(t *testing.T) {
	ds := loadFixtureDataset(t, "normalized_manufacturer_multi_year.json")
	res := Calculate(Input{Dataset: ds, PeriodMeta: threeYearMeta()}, Options{})
	if res.Score != nil {
		t.Errorf("expected Score to be nil when ComputeScore is false, got %+v", res.Score)
	}
}

// TestScore_HeuristicLabelAlwaysTrue proves every computed Score carries
// Heuristic == true, per the package's explicit requirement that this
// score never be presented as an accounting standard.
func TestScore_HeuristicLabelAlwaysTrue(t *testing.T) {
	ds := loadFixtureDataset(t, "normalized_manufacturer_multi_year.json")
	res := Calculate(Input{Dataset: ds, PeriodMeta: threeYearMeta()}, Options{ComputeScore: true})
	if res.Score == nil {
		t.Fatal("expected Score to be populated")
	}
	if !res.Score.Heuristic {
		t.Error("expected Score.Heuristic == true")
	}
	if res.Score.Version != ScoreVersion {
		t.Errorf("Score.Version = %q, want %q", res.Score.Version, ScoreVersion)
	}
}

// TestScore_ClampedToZeroToHundred proves Score.Value never leaves [0,
// 100] even when many severe flags fire (in this test, by feeding
// maintainable earnings at exactly the near-zero floor for both bases,
// which alone would only trigger 2 critical deductions — this test mainly
// guards the clamp logic itself rather than trying to manufacture every
// flag at once).
func TestScore_ClampedToZeroToHundred(t *testing.T) {
	ds := loadFixtureDataset(t, "normalized_hvac_multi_year.json")
	meta := threeYearMeta()
	res := Calculate(Input{
		Dataset:            ds,
		PeriodMeta:         meta,
		MaintainableEBITDA: earningsResultAt(0),
		MaintainableSDE:    earningsResultAt(-5000),
	}, Options{ComputeScore: true})
	if res.Score == nil {
		t.Fatal("expected Score to be populated")
	}
	if res.Score.Value < 0 || res.Score.Value > 100 {
		t.Errorf("Score.Value = %v, want within [0, 100]", res.Score.Value)
	}
}

// TestScore_ComponentsExplainValue proves Score.Components sums exactly to
// Value's deduction from the 100-point baseline (before clamping would
// only matter in extreme cases not exercised here), so a caller can always
// reconstruct Value from Components alone.
func TestScore_ComponentsExplainValue(t *testing.T) {
	ds := loadFixtureDataset(t, "normalized_hvac_multi_year.json")
	meta := threeYearMeta()
	res := runQoE(t, ds, meta, nil, Options{ComputeScore: true})
	if res.Score == nil {
		t.Fatal("expected Score to be populated")
	}

	sum := 100.0
	for _, c := range res.Score.Components {
		sum += c.Points
	}
	if sum < 0 {
		sum = 0
	}
	if sum > 100 {
		sum = 100
	}
	if sum != res.Score.Value {
		t.Errorf("sum of 100 + Components (%v) != Score.Value (%v)", sum, res.Score.Value)
	}
	if len(res.Score.Components) != len(res.Flags) {
		t.Errorf("expected one ScoreComponent per Flag, got %d components for %d flags", len(res.Score.Components), len(res.Flags))
	}
}
