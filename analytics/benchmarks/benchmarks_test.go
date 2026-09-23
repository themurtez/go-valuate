package benchmarks

import (
	"math"
	"testing"
)

func approxEqual(t *testing.T, got, want, tol float64, label string) {
	t.Helper()
	diff := got - want
	if diff < 0 {
		diff = -diff
	}
	if diff > tol {
		t.Errorf("%s: got %v, want %v (tol %v)", label, got, want, tol)
	}
}

func TestCalculate_NoMetrics(t *testing.T) {
	res := Calculate(Input{})
	if res.Available {
		t.Fatal("expected Available == false with no metrics at all")
	}
	if !HasErrors(res.Errors) {
		t.Fatal("expected an error when no metrics are supplied")
	}
}

// TestCalculate_MedianOnly covers FormMedian: only difference/relative
// difference are calculable, no percentile or band.
func TestCalculate_MedianOnly(t *testing.T) {
	res := Calculate(Input{Metrics: []MetricRequest{
		{
			MetricID:     "GROSS_MARGIN",
			Label:        "Gross Margin %",
			CompanyValue: AvailableValue(0.42),
			Direction:    DirectionHigherIsBetter,
			Benchmark: BenchmarkSet{
				Form:   FormMedian,
				Median: AvailableValue(0.38),
				Source: BenchmarkSource{Name: "Industry Survey"},
			},
		},
	}})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	c := res.Comparisons[0]
	if !c.Available {
		t.Fatal("expected comparison available")
	}
	if !c.BenchmarkMedian.Available || c.BenchmarkMedian.Amount != 0.38 {
		t.Fatalf("expected benchmark median 0.38, got %+v", c.BenchmarkMedian)
	}
	if c.Percentile.Available {
		t.Error("expected percentile unavailable for FormMedian")
	}
	if c.Band != BandUnavailable {
		t.Errorf("expected BandUnavailable for FormMedian, got %s", c.Band)
	}
	if !c.Difference.Available {
		t.Fatal("expected difference available")
	}
	approxEqual(t, c.Difference.Amount, 0.04, 1e-9, "difference")
	approxEqual(t, c.RelativeDifference.Amount, 0.04/0.38, 1e-9, "relative difference")
	if c.Favorable != FavorableYes {
		t.Errorf("expected FavorableYes (higher is better, above median), got %s", c.Favorable)
	}
}

// TestCalculate_PercentileBands_Interpolation covers linear interpolation
// across percentile bands, both for the median and for the company
// value's own percentile estimate.
func TestCalculate_PercentileBands_Interpolation(t *testing.T) {
	bands := []PercentilePoint{
		{Percentile: 10, Value: 0.10},
		{Percentile: 25, Value: 0.20},
		{Percentile: 50, Value: 0.30},
		{Percentile: 75, Value: 0.40},
		{Percentile: 90, Value: 0.50},
	}
	res := Calculate(Input{Metrics: []MetricRequest{
		{
			MetricID:     "EBITDA_MARGIN",
			CompanyValue: AvailableValue(0.35), // halfway between P50 (0.30) and P75 (0.40) -> P62.5
			Direction:    DirectionHigherIsBetter,
			Benchmark: BenchmarkSet{
				Form:            FormPercentileBands,
				PercentileBands: bands,
				Source:          BenchmarkSource{Name: "Peer Study"},
			},
		},
	}})
	c := res.Comparisons[0]
	if !c.BenchmarkMedian.Available {
		t.Fatal("expected interpolated median available")
	}
	approxEqual(t, c.BenchmarkMedian.Amount, 0.30, 1e-9, "interpolated median")

	if !c.Percentile.Available {
		t.Fatal("expected percentile estimate available")
	}
	approxEqual(t, c.Percentile.Amount, 62.5, 1e-6, "company percentile")

	if c.Band != BandQ3 {
		t.Errorf("expected BandQ3 (between 50th and 75th percentile), got %s", c.Band)
	}
	if !c.BenchmarkRange.Min.Available || c.BenchmarkRange.Min.Amount != 0.10 {
		t.Errorf("expected range min 0.10, got %+v", c.BenchmarkRange.Min)
	}
	if !c.BenchmarkRange.Max.Available || c.BenchmarkRange.Max.Amount != 0.50 {
		t.Errorf("expected range max 0.50, got %+v", c.BenchmarkRange.Max)
	}
}

// TestCalculate_PercentileBands_BandQ4 covers a company value strictly
// above the 75th percentile but still within the known range (BandQ4).
func TestCalculate_PercentileBands_BandQ4(t *testing.T) {
	bands := []PercentilePoint{
		{Percentile: 25, Value: 0.20},
		{Percentile: 50, Value: 0.30},
		{Percentile: 75, Value: 0.40},
		{Percentile: 90, Value: 0.50},
	}
	res := Calculate(Input{Metrics: []MetricRequest{
		{
			MetricID:     "M1",
			CompanyValue: AvailableValue(0.45), // between P75 (0.40) and P90 (0.50) -> P82.5
			Benchmark:    BenchmarkSet{Form: FormPercentileBands, PercentileBands: bands, Source: BenchmarkSource{Name: "X"}},
		},
	}})
	c := res.Comparisons[0]
	if c.Band != BandQ4 {
		t.Errorf("expected BandQ4, got %s", c.Band)
	}
	approxEqual(t, c.Percentile.Amount, 82.5, 1e-6, "company percentile")
}

// TestCalculate_PercentileBands_OutOfRange covers a company value beyond
// the known percentile band extent: clamped percentile, but classified
// BandBelowMin/BandAboveMax rather than merely Q1/Q4.
func TestCalculate_PercentileBands_OutOfRange(t *testing.T) {
	bands := []PercentilePoint{
		{Percentile: 25, Value: 0.20},
		{Percentile: 75, Value: 0.40},
	}

	below := Calculate(Input{Metrics: []MetricRequest{{
		MetricID:     "M1",
		CompanyValue: AvailableValue(0.05),
		Benchmark:    BenchmarkSet{Form: FormPercentileBands, PercentileBands: bands, Source: BenchmarkSource{Name: "X"}},
	}}}).Comparisons[0]
	if below.Band != BandBelowMin {
		t.Errorf("expected BandBelowMin, got %s", below.Band)
	}
	if !below.Percentile.Available || below.Percentile.Amount != 25 {
		t.Errorf("expected clamped percentile 25, got %+v", below.Percentile)
	}

	above := Calculate(Input{Metrics: []MetricRequest{{
		MetricID:     "M2",
		CompanyValue: AvailableValue(0.90),
		Benchmark:    BenchmarkSet{Form: FormPercentileBands, PercentileBands: bands, Source: BenchmarkSource{Name: "X"}},
	}}}).Comparisons[0]
	if above.Band != BandAboveMax {
		t.Errorf("expected BandAboveMax, got %s", above.Band)
	}
	if !above.Percentile.Available || above.Percentile.Amount != 75 {
		t.Errorf("expected clamped percentile 75, got %+v", above.Percentile)
	}
}

// TestCalculate_Quartiles covers FormQuartiles: Q1/Median/Q3 imply a
// three-point interpolation table.
func TestCalculate_Quartiles(t *testing.T) {
	res := Calculate(Input{Metrics: []MetricRequest{
		{
			MetricID:     "DSO",
			CompanyValue: AvailableValue(45),
			Direction:    DirectionLowerIsBetter,
			Benchmark: BenchmarkSet{
				Form: FormQuartiles,
				Quartiles: Quartiles{
					Q1:     AvailableValue(30),
					Median: AvailableValue(40),
					Q3:     AvailableValue(55),
				},
				Source: BenchmarkSource{Name: "Trade Association"},
			},
		},
	}})
	c := res.Comparisons[0]
	if !c.BenchmarkMedian.Available || c.BenchmarkMedian.Amount != 40 {
		t.Fatalf("expected median 40, got %+v", c.BenchmarkMedian)
	}
	// 45 is between Q2(40) and Q3(55) -> band Q3, and company value above
	// median is *unfavorable* for a lower-is-better metric like DSO.
	if c.Band != BandQ3 {
		t.Errorf("expected BandQ3, got %s", c.Band)
	}
	if c.Favorable != FavorableNo {
		t.Errorf("expected FavorableNo (lower is better, DSO above median), got %s", c.Favorable)
	}
}

// TestCalculate_Quartiles_Sparse covers a Quartiles value with only Q1 and
// Median populated (Q3 unavailable) — still enough for a two-point
// interpolation, but IssueInsufficientPercentileBands must NOT fire.
func TestCalculate_Quartiles_Sparse(t *testing.T) {
	res := Calculate(Input{Metrics: []MetricRequest{
		{
			MetricID:     "M1",
			CompanyValue: AvailableValue(35),
			Benchmark: BenchmarkSet{
				Form: FormQuartiles,
				Quartiles: Quartiles{
					Q1:     AvailableValue(30),
					Median: AvailableValue(40),
				},
				Source: BenchmarkSource{Name: "X"},
			},
		},
	}})
	c := res.Comparisons[0]
	if !c.Percentile.Available {
		t.Fatal("expected percentile available with 2 known quartile points")
	}
	for _, w := range res.Warnings {
		if w.Code == IssueInsufficientPercentileBands {
			t.Errorf("did not expect IssueInsufficientPercentileBands with 2 points, got warnings=%+v", res.Warnings)
		}
	}
}

// TestCalculate_Quartiles_SinglePoint covers a Quartiles value with only
// one field populated: insufficient for interpolation.
func TestCalculate_Quartiles_SinglePoint(t *testing.T) {
	res := Calculate(Input{Metrics: []MetricRequest{
		{
			MetricID:     "M1",
			CompanyValue: AvailableValue(35),
			Benchmark: BenchmarkSet{
				Form:      FormQuartiles,
				Quartiles: Quartiles{Median: AvailableValue(40)},
				Source:    BenchmarkSource{Name: "X"},
			},
		},
	}})
	c := res.Comparisons[0]
	if c.Percentile.Available {
		t.Error("expected percentile unavailable with only 1 known quartile point")
	}
	if !c.BenchmarkMedian.Available || c.BenchmarkMedian.Amount != 40 {
		t.Errorf("expected median still available from the single known point, got %+v", c.BenchmarkMedian)
	}
	found := false
	for _, w := range res.Warnings {
		if w.Code == IssueInsufficientPercentileBands {
			found = true
		}
	}
	if !found {
		t.Error("expected IssueInsufficientPercentileBands warning")
	}
}

// TestCalculate_NonMonotonicQuartiles covers a caller-supplied Quartiles
// table where Value decreases as Percentile increases (Q1=80 > Median=50
// > Q3=20) — a plausible mistake for a "lower is better" metric like DSO
// entered in the wrong direction. Percentile/Band/Range must all be left
// unavailable rather than silently reporting an inverted range or a band
// derived from a self-contradictory table.
func TestCalculate_NonMonotonicQuartiles(t *testing.T) {
	res := Calculate(Input{Metrics: []MetricRequest{
		{
			MetricID:     "M1",
			CompanyValue: AvailableValue(40),
			Benchmark: BenchmarkSet{
				Form: FormQuartiles,
				Quartiles: Quartiles{
					Q1:     AvailableValue(80),
					Median: AvailableValue(50),
					Q3:     AvailableValue(20),
				},
				Source: BenchmarkSource{Name: "X"},
			},
		},
	}})
	c := res.Comparisons[0]
	if c.Percentile.Available {
		t.Errorf("expected percentile unavailable for a non-monotonic points table, got %+v", c.Percentile)
	}
	if c.Band != BandUnavailable {
		t.Errorf("expected BandUnavailable for a non-monotonic points table, got %s", c.Band)
	}
	if c.BenchmarkRange.Min.Available || c.BenchmarkRange.Max.Available {
		t.Errorf("expected range unavailable for a non-monotonic points table, got %+v", c.BenchmarkRange)
	}
	// The explicit Quartiles.Median is still trustworthy on its own (it
	// doesn't depend on point-table ordering), so difference/relative
	// difference against it should still be reported.
	if !c.BenchmarkMedian.Available || c.BenchmarkMedian.Amount != 50 {
		t.Fatalf("expected median 50 still available, got %+v", c.BenchmarkMedian)
	}
	if !c.Difference.Available {
		t.Error("expected difference still available from the explicit median")
	}
	found := false
	for _, w := range res.Warnings {
		if w.Code == IssueNonMonotonicBenchmarkPoints {
			found = true
		}
	}
	if !found {
		t.Error("expected IssueNonMonotonicBenchmarkPoints warning")
	}
}

// TestCalculate_NonMonotonicPercentileBands mirrors
// TestCalculate_NonMonotonicQuartiles for FormPercentileBands.
func TestCalculate_NonMonotonicPercentileBands(t *testing.T) {
	res := Calculate(Input{Metrics: []MetricRequest{
		{
			MetricID:     "M1",
			CompanyValue: AvailableValue(0.30),
			Benchmark: BenchmarkSet{
				Form: FormPercentileBands,
				PercentileBands: []PercentilePoint{
					{Percentile: 10, Value: 0.60},
					{Percentile: 50, Value: 0.40},
					{Percentile: 90, Value: 0.20},
				},
				Source: BenchmarkSource{Name: "X"},
			},
		},
	}})
	c := res.Comparisons[0]
	if c.Percentile.Available {
		t.Errorf("expected percentile unavailable, got %+v", c.Percentile)
	}
	if c.Band != BandUnavailable {
		t.Errorf("expected BandUnavailable, got %s", c.Band)
	}
	if c.BenchmarkRange.Min.Available || c.BenchmarkRange.Max.Available {
		t.Errorf("expected range unavailable, got %+v", c.BenchmarkRange)
	}
	found := false
	for _, w := range res.Warnings {
		if w.Code == IssueNonMonotonicBenchmarkPoints {
			found = true
		}
	}
	if !found {
		t.Error("expected IssueNonMonotonicBenchmarkPoints warning")
	}
}

// TestCalculate_PeerObservations covers FormPeerObservations: Calculate
// derives median/percentile/range directly from raw peer values.
func TestCalculate_PeerObservations(t *testing.T) {
	peers := []PeerObservation{
		{PeerKey: "peer-1", Value: 10},
		{PeerKey: "peer-2", Value: 20},
		{PeerKey: "peer-3", Value: 30},
		{PeerKey: "peer-4", Value: 40},
		{PeerKey: "peer-5", Value: 50},
	}
	res := Calculate(Input{Metrics: []MetricRequest{
		{
			MetricID:     "REVENUE_PER_EMPLOYEE",
			CompanyValue: AvailableValue(30),
			Direction:    DirectionHigherIsBetter,
			Benchmark: BenchmarkSet{
				Form:             FormPeerObservations,
				PeerObservations: peers,
				Source:           BenchmarkSource{Name: "Peer Set", SampleSize: 5},
			},
		},
	}})
	c := res.Comparisons[0]
	if !c.BenchmarkMedian.Available {
		t.Fatal("expected median available")
	}
	approxEqual(t, c.BenchmarkMedian.Amount, 30, 1e-9, "peer median")
	if !c.Percentile.Available {
		t.Fatal("expected percentile available")
	}
	approxEqual(t, c.Percentile.Amount, 50, 1e-9, "peer percentile")
	if c.Favorable != FavorableEqual {
		t.Errorf("expected FavorableEqual (company value == median), got %s", c.Favorable)
	}
	if !c.BenchmarkRange.Min.Available || c.BenchmarkRange.Min.Amount != 10 {
		t.Errorf("expected range min 10, got %+v", c.BenchmarkRange.Min)
	}
	if !c.BenchmarkRange.Max.Available || c.BenchmarkRange.Max.Amount != 50 {
		t.Errorf("expected range max 50, got %+v", c.BenchmarkRange.Max)
	}
}

// TestCalculate_PeerObservations_UnsortedInput proves peer order in input
// doesn't matter — Calculate sorts before computing rank-based percentile.
func TestCalculate_PeerObservations_UnsortedInput(t *testing.T) {
	peers := []PeerObservation{
		{Value: 50}, {Value: 10}, {Value: 30}, {Value: 40}, {Value: 20},
	}
	res := Calculate(Input{Metrics: []MetricRequest{
		{
			MetricID:     "M1",
			CompanyValue: AvailableValue(20),
			Benchmark:    BenchmarkSet{Form: FormPeerObservations, PeerObservations: peers, Source: BenchmarkSource{Name: "X"}},
		},
	}})
	c := res.Comparisons[0]
	approxEqual(t, c.Percentile.Amount, 25, 1e-9, "sorted peer percentile")
}

// TestCalculate_PeerObservations_SingleObservation covers a lone peer
// observation: reported as the median but insufficient for percentile
// interpolation.
func TestCalculate_PeerObservations_SingleObservation(t *testing.T) {
	res := Calculate(Input{Metrics: []MetricRequest{
		{
			MetricID:     "M1",
			CompanyValue: AvailableValue(99),
			Benchmark: BenchmarkSet{
				Form:             FormPeerObservations,
				PeerObservations: []PeerObservation{{Value: 42}},
				Source:           BenchmarkSource{Name: "X"},
			},
		},
	}})
	c := res.Comparisons[0]
	if !c.BenchmarkMedian.Available || c.BenchmarkMedian.Amount != 42 {
		t.Fatalf("expected median 42 from the single observation, got %+v", c.BenchmarkMedian)
	}
	if c.Percentile.Available {
		t.Error("expected percentile unavailable with a single peer observation")
	}
}

// TestCalculate_ExplicitMedianOverridesDerived proves a caller-supplied
// BenchmarkSet.Median takes precedence over a derived median even when
// Form implies richer data.
func TestCalculate_ExplicitMedianOverridesDerived(t *testing.T) {
	res := Calculate(Input{Metrics: []MetricRequest{
		{
			MetricID:     "M1",
			CompanyValue: AvailableValue(10),
			Benchmark: BenchmarkSet{
				Form:   FormPercentileBands,
				Median: AvailableValue(0.99), // deliberately inconsistent with bands, to prove precedence
				PercentileBands: []PercentilePoint{
					{Percentile: 25, Value: 1},
					{Percentile: 75, Value: 3},
				},
				Source: BenchmarkSource{Name: "X"},
			},
		},
	}})
	c := res.Comparisons[0]
	if !c.BenchmarkMedian.Available || c.BenchmarkMedian.Amount != 0.99 {
		t.Fatalf("expected explicit median 0.99 to take precedence, got %+v", c.BenchmarkMedian)
	}
}

// TestCalculate_UnavailableCompanyValue proves benchmark-side fields are
// still reported when CompanyValue is unavailable, but comparison fields
// are not.
func TestCalculate_UnavailableCompanyValue(t *testing.T) {
	res := Calculate(Input{Metrics: []MetricRequest{
		{
			MetricID:     "M1",
			CompanyValue: Unavailable(),
			Direction:    DirectionHigherIsBetter,
			Benchmark: BenchmarkSet{
				Form:            FormPercentileBands,
				PercentileBands: []PercentilePoint{{Percentile: 25, Value: 1}, {Percentile: 75, Value: 3}},
				Source:          BenchmarkSource{Name: "X"},
			},
		},
	}})
	c := res.Comparisons[0]
	if !c.Available {
		t.Fatal("expected Comparison.Available true even with unavailable company value")
	}
	if !c.BenchmarkMedian.Available {
		t.Error("expected benchmark median still available")
	}
	if c.Percentile.Available {
		t.Error("expected percentile unavailable without a company value")
	}
	if c.Difference.Available {
		t.Error("expected difference unavailable without a company value")
	}
	if c.Favorable != FavorableNotApplicable {
		t.Errorf("expected FavorableNotApplicable, got %s", c.Favorable)
	}
	found := false
	for _, w := range res.Warnings {
		if w.Code == IssueCompanyValueUnavailable {
			found = true
		}
	}
	if !found {
		t.Error("expected IssueCompanyValueUnavailable warning")
	}
}

// TestCalculate_MissingMetricID proves a request with no MetricID
// produces an entirely unavailable Comparison, distinct from an
// unavailable CompanyValue (which still reports benchmark-side fields).
func TestCalculate_MissingMetricID(t *testing.T) {
	res := Calculate(Input{Metrics: []MetricRequest{
		{
			CompanyValue: AvailableValue(10),
			Benchmark:    BenchmarkSet{Form: FormMedian, Median: AvailableValue(5), Source: BenchmarkSource{Name: "X"}},
		},
	}})
	c := res.Comparisons[0]
	if c.Available {
		t.Error("expected Comparison.Available false without a metric ID")
	}
	found := false
	for _, w := range res.Warnings {
		if w.Code == IssueMissingMetricID {
			found = true
		}
	}
	if !found {
		t.Error("expected IssueMissingMetricID warning")
	}
}

// TestCalculate_InvalidForm covers an empty/unrecognized BenchmarkForm.
func TestCalculate_InvalidForm(t *testing.T) {
	res := Calculate(Input{Metrics: []MetricRequest{
		{
			MetricID:     "M1",
			CompanyValue: AvailableValue(10),
			Benchmark:    BenchmarkSet{Form: BenchmarkForm("BOGUS"), Source: BenchmarkSource{Name: "X"}},
		},
	}})
	c := res.Comparisons[0]
	if !c.Available {
		t.Error("expected Comparison.Available true (the row was identifiable, just its form was invalid)")
	}
	if c.BenchmarkMedian.Available {
		t.Error("expected benchmark median unavailable for an invalid form")
	}
	found := false
	for _, w := range res.Warnings {
		if w.Code == IssueInvalidForm {
			found = true
		}
	}
	if !found {
		t.Error("expected IssueInvalidForm warning")
	}
}

// TestCalculate_BenchmarkDataMissing covers a declared Form whose data
// field was left empty.
func TestCalculate_BenchmarkDataMissing(t *testing.T) {
	res := Calculate(Input{Metrics: []MetricRequest{
		{
			MetricID:     "M1",
			CompanyValue: AvailableValue(10),
			Benchmark:    BenchmarkSet{Form: FormMedian, Source: BenchmarkSource{Name: "X"}},
		},
	}})
	c := res.Comparisons[0]
	if c.BenchmarkMedian.Available {
		t.Error("expected benchmark median unavailable when Median was never populated")
	}
	found := false
	for _, w := range res.Warnings {
		if w.Code == IssueBenchmarkDataMissing {
			found = true
		}
	}
	if !found {
		t.Error("expected IssueBenchmarkDataMissing warning")
	}
}

// TestCalculate_BenchmarkMedianZero covers RelativeDifference's
// division-by-zero guard.
func TestCalculate_BenchmarkMedianZero(t *testing.T) {
	res := Calculate(Input{Metrics: []MetricRequest{
		{
			MetricID:     "M1",
			CompanyValue: AvailableValue(5),
			Benchmark:    BenchmarkSet{Form: FormMedian, Median: AvailableValue(0), Source: BenchmarkSource{Name: "X"}},
		},
	}})
	c := res.Comparisons[0]
	if !c.Difference.Available || c.Difference.Amount != 5 {
		t.Fatalf("expected difference 5, got %+v", c.Difference)
	}
	if c.RelativeDifference.Available {
		t.Error("expected relative difference unavailable when median is exactly 0")
	}
	found := false
	for _, w := range res.Warnings {
		if w.Code == IssueBenchmarkMedianZero {
			found = true
		}
	}
	if !found {
		t.Error("expected IssueBenchmarkMedianZero warning")
	}
}

// TestCalculate_NaNCompanyValue is a regression test for a bug where an
// Available CompanyValue with a NaN/Inf Amount (distinct from
// !Available, which IssueCompanyValueUnavailable already guards) flowed
// unguarded into Difference/RelativeDifference's arithmetic, producing a
// NaN/Inf Comparison field. Caught by FuzzCalculate_CompanyValueAndMedian.
func TestCalculate_NaNCompanyValue(t *testing.T) {
	res := Calculate(Input{Metrics: []MetricRequest{
		{
			MetricID:     "M1",
			CompanyValue: AvailableValue(math.NaN()),
			Benchmark:    BenchmarkSet{Form: FormMedian, Median: AvailableValue(5), Source: BenchmarkSource{Name: "X"}},
		},
	}})
	c := res.Comparisons[0]
	if c.Difference.Available {
		t.Errorf("expected Difference unavailable for a NaN company value, got %+v", c.Difference)
	}
	found := false
	for _, w := range res.Warnings {
		if w.Code == IssueInvalidCompanyValue {
			found = true
		}
	}
	if !found {
		t.Error("expected IssueInvalidCompanyValue warning")
	}
}

// TestCalculate_InfBenchmarkMedian is a regression test for the same
// class of bug as TestCalculate_NaNCompanyValue, on the benchmark side:
// an Available Median with a NaN/Inf Amount flowed unguarded into
// Difference/RelativeDifference. Caught by
// FuzzCalculate_CompanyValueAndMedian.
func TestCalculate_InfBenchmarkMedian(t *testing.T) {
	res := Calculate(Input{Metrics: []MetricRequest{
		{
			MetricID:     "M1",
			CompanyValue: AvailableValue(5),
			Benchmark:    BenchmarkSet{Form: FormMedian, Median: AvailableValue(math.Inf(1)), Source: BenchmarkSource{Name: "X"}},
		},
	}})
	c := res.Comparisons[0]
	if c.BenchmarkMedian.Available {
		t.Errorf("expected BenchmarkMedian unavailable for an infinite median, got %+v", c.BenchmarkMedian)
	}
	found := false
	for _, w := range res.Warnings {
		if w.Code == IssueInvalidBenchmarkMedian {
			found = true
		}
	}
	if !found {
		t.Error("expected IssueInvalidBenchmarkMedian warning")
	}
}

// TestCalculate_RelativeDifferenceOverflow is a regression test for a
// bug where CompanyValue and BenchmarkMedian were each individually
// finite, but Difference's subtraction (or RelativeDifference's
// division) overflowed float64's range, producing a +/-Inf Comparison
// field — distinct from IssueBenchmarkMedianZero's exactly-zero-
// denominator case. Caught by FuzzCalculate_CompanyValueAndMedian.
func TestCalculate_RelativeDifferenceOverflow(t *testing.T) {
	res := Calculate(Input{Metrics: []MetricRequest{
		{
			MetricID:     "M1",
			CompanyValue: AvailableValue(math.MaxFloat64),
			Benchmark:    BenchmarkSet{Form: FormMedian, Median: AvailableValue(-math.MaxFloat64), Source: BenchmarkSource{Name: "X"}},
		},
	}})
	c := res.Comparisons[0]
	if c.Difference.Available {
		t.Errorf("expected Difference unavailable when the subtraction overflows, got %+v", c.Difference)
	}
	if c.RelativeDifference.Available {
		t.Errorf("expected RelativeDifference unavailable when Difference overflowed, got %+v", c.RelativeDifference)
	}
	found := false
	for _, w := range res.Warnings {
		if w.Code == IssueRelativeDifferenceOverflow {
			found = true
		}
	}
	if !found {
		t.Error("expected IssueRelativeDifferenceOverflow warning")
	}
}

// TestCalculate_DirectionNeutralAndUnspecified prove neither ever
// produces a Favorable/Unfavorable verdict.
func TestCalculate_DirectionNeutralAndUnspecified(t *testing.T) {
	for _, dir := range []Direction{DirectionNeutral, DirectionUnspecified} {
		res := Calculate(Input{Metrics: []MetricRequest{
			{
				MetricID:     "M1",
				CompanyValue: AvailableValue(10),
				Direction:    dir,
				Benchmark:    BenchmarkSet{Form: FormMedian, Median: AvailableValue(5), Source: BenchmarkSource{Name: "X"}},
			},
		}})
		c := res.Comparisons[0]
		if c.Favorable != FavorableNotApplicable {
			t.Errorf("direction %q: expected FavorableNotApplicable, got %s", dir, c.Favorable)
		}
	}
}

// TestCalculate_DuplicateMetricID covers the advisory duplicate-detection
// path: both rows are still evaluated independently.
func TestCalculate_DuplicateMetricID(t *testing.T) {
	res := Calculate(Input{Metrics: []MetricRequest{
		{MetricID: "M1", Period: "2025-Q1", CompanyValue: AvailableValue(1), Benchmark: BenchmarkSet{Form: FormMedian, Median: AvailableValue(1), Source: BenchmarkSource{Name: "X"}}},
		{MetricID: "M1", Period: "2025-Q1", CompanyValue: AvailableValue(2), Benchmark: BenchmarkSet{Form: FormMedian, Median: AvailableValue(1), Source: BenchmarkSource{Name: "X"}}},
	}})
	if len(res.Comparisons) != 2 {
		t.Fatalf("expected both duplicate rows evaluated, got %d", len(res.Comparisons))
	}
	found := false
	for _, w := range res.Warnings {
		if w.Code == IssueDuplicateMetricID {
			found = true
		}
	}
	if !found {
		t.Error("expected IssueDuplicateMetricID warning")
	}
}

// TestCalculate_Summary covers Summary aggregation across a batch mixing
// favorable, unfavorable, and unavailable comparisons.
func TestCalculate_Summary(t *testing.T) {
	res := Calculate(Input{Metrics: []MetricRequest{
		{MetricID: "FAV", CompanyValue: AvailableValue(10), Direction: DirectionHigherIsBetter, Benchmark: BenchmarkSet{Form: FormMedian, Median: AvailableValue(5), Source: BenchmarkSource{Name: "X"}}},
		{MetricID: "UNFAV", CompanyValue: AvailableValue(1), Direction: DirectionHigherIsBetter, Benchmark: BenchmarkSet{Form: FormMedian, Median: AvailableValue(5), Source: BenchmarkSource{Name: "X"}}},
		{MetricID: "", CompanyValue: AvailableValue(1), Benchmark: BenchmarkSet{Form: FormMedian, Median: AvailableValue(5), Source: BenchmarkSource{Name: "X"}}},
	}})
	if res.Summary.MetricCount != 3 {
		t.Errorf("expected metric count 3, got %d", res.Summary.MetricCount)
	}
	if res.Summary.FavorableCount != 1 {
		t.Errorf("expected favorable count 1, got %d", res.Summary.FavorableCount)
	}
	if res.Summary.UnfavorableCount != 1 {
		t.Errorf("expected unfavorable count 1, got %d", res.Summary.UnfavorableCount)
	}
	if res.Summary.UnavailableCount != 1 {
		t.Errorf("expected unavailable count 1, got %d", res.Summary.UnavailableCount)
	}
	if len(res.Summary.UnfavorableMetricIDs) != 1 || res.Summary.UnfavorableMetricIDs[0] != "UNFAV" {
		t.Errorf("expected UnfavorableMetricIDs = [UNFAV], got %v", res.Summary.UnfavorableMetricIDs)
	}
}

// TestCalculate_NoMutationOfInput proves Calculate never mutates
// caller-owned input slices.
func TestCalculate_NoMutationOfInput(t *testing.T) {
	bands := []PercentilePoint{
		{Percentile: 75, Value: 3},
		{Percentile: 25, Value: 1},
	}
	in := Input{Metrics: []MetricRequest{
		{
			MetricID:     "M1",
			CompanyValue: AvailableValue(2),
			Benchmark:    BenchmarkSet{Form: FormPercentileBands, PercentileBands: bands, Source: BenchmarkSource{Name: "X"}},
		},
	}}
	_ = Calculate(in)
	if bands[0].Percentile != 75 || bands[1].Percentile != 25 {
		t.Fatalf("expected input bands left unsorted/unmutated, got %+v", bands)
	}
}

// TestCalculate_SourceProvenancePreserved proves every BenchmarkSource
// field is echoed verbatim onto Comparison.
func TestCalculate_SourceProvenancePreserved(t *testing.T) {
	src := BenchmarkSource{
		Name:          "RMA Annual Statement Studies",
		EffectiveDate: "FY2024",
		Population:    "US manufacturing, $10M-$50M revenue",
		SampleSize:    128,
		SourceID:      "RMA-2024-SUB-882",
	}
	res := Calculate(Input{Metrics: []MetricRequest{
		{
			MetricID:     "M1",
			CompanyValue: AvailableValue(1),
			Benchmark: BenchmarkSet{
				Form:           FormMedian,
				Median:         AvailableValue(1),
				IndustryLabel:  "Manufacturing",
				SizeLabel:      "$10M-$50M",
				GeographyLabel: "US",
				Source:         src,
			},
		},
	}})
	c := res.Comparisons[0]
	if c.Source != src {
		t.Fatalf("expected source echoed verbatim, got %+v", c.Source)
	}
	if c.SourceIndustryLabel != "Manufacturing" || c.SourceSizeLabel != "$10M-$50M" || c.SourceGeographyLabel != "US" {
		t.Errorf("expected segment labels echoed, got industry=%q size=%q geo=%q", c.SourceIndustryLabel, c.SourceSizeLabel, c.SourceGeographyLabel)
	}
}
