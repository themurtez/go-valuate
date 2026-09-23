package benchmarks

import "sort"

// benchmarkStats is the set of statistics Calculate can derive from a
// BenchmarkSet, regardless of which BenchmarkForm supplied it — the
// common shape every form-specific deriver below normalizes into so
// evaluateMetric's comparison arithmetic (difference, percentile
// placement, band, range) is written exactly once.
type benchmarkStats struct {
	median Value
	// points is the set of known (percentile, value) pairs this benchmark
	// set implies, sorted ascending by percentile, deduplicated by
	// percentile (last write wins for an exact duplicate). Used for
	// interpolation and range/band derivation. May be empty (FormMedian
	// with only a median known) or contain a single point (a single
	// PercentileBands entry).
	points []PercentilePoint
	// insufficientPoints is true when Form implied point data was
	// supplied (FormPercentileBands, FormQuartiles, or
	// FormPeerObservations) but fewer than two distinct-percentile points
	// resulted, so no interpolation is possible.
	insufficientPoints bool
	// nonMonotonic is true when points has at least two entries but Value
	// is not non-decreasing as Percentile increases — see
	// IssueNonMonotonicBenchmarkPoints. Never true for
	// FormPeerObservations, whose points are built by sorting on Value
	// itself (see peerPercentilePoints), so monotonicity holds by
	// construction.
	nonMonotonic bool
}

// deriveBenchmarkStats normalizes bs into benchmarkStats per its declared
// Form. Only the field(s) documented as belonging to Form are read (see
// BenchmarkForm's doc comment) — a caller populating an unrelated field is
// silently ignored, not merged in as additional evidence.
func deriveBenchmarkStats(bs BenchmarkSet) (benchmarkStats, IssueCode) {
	switch bs.Form {
	case FormMedian:
		if !bs.Median.Available {
			return benchmarkStats{}, IssueBenchmarkDataMissing
		}
		return benchmarkStats{median: bs.Median}, ""

	case FormPercentileBands:
		if len(bs.PercentileBands) == 0 {
			return benchmarkStats{}, IssueBenchmarkDataMissing
		}
		points := dedupSortPoints(bs.PercentileBands)
		stats := benchmarkStats{points: points}
		if len(points) < 2 {
			stats.insufficientPoints = true
		} else if !isNonDecreasing(points) {
			stats.nonMonotonic = true
		}
		stats.median = interpolate(points, 50)
		if bs.Median.Available {
			stats.median = bs.Median
		}
		return stats, ""

	case FormQuartiles:
		var points []PercentilePoint
		if bs.Quartiles.Q1.Available {
			points = append(points, PercentilePoint{Percentile: 25, Value: bs.Quartiles.Q1.Amount})
		}
		if bs.Quartiles.Median.Available {
			points = append(points, PercentilePoint{Percentile: 50, Value: bs.Quartiles.Median.Amount})
		}
		if bs.Quartiles.Q3.Available {
			points = append(points, PercentilePoint{Percentile: 75, Value: bs.Quartiles.Q3.Amount})
		}
		if len(points) == 0 {
			return benchmarkStats{}, IssueBenchmarkDataMissing
		}
		points = dedupSortPoints(points)
		stats := benchmarkStats{points: points}
		if len(points) < 2 {
			stats.insufficientPoints = true
		} else if !isNonDecreasing(points) {
			stats.nonMonotonic = true
		}
		stats.median = interpolate(points, 50)
		if bs.Quartiles.Median.Available {
			stats.median = bs.Quartiles.Median
		} else if bs.Median.Available {
			stats.median = bs.Median
		}
		return stats, ""

	case FormPeerObservations:
		if len(bs.PeerObservations) == 0 {
			return benchmarkStats{}, IssueBenchmarkDataMissing
		}
		points, median := peerPercentilePoints(bs.PeerObservations)
		stats := benchmarkStats{points: points, median: median}
		if len(points) < 2 {
			stats.insufficientPoints = true
		}
		if bs.Median.Available {
			stats.median = bs.Median
		}
		return stats, ""

	default:
		return benchmarkStats{}, IssueInvalidForm
	}
}

// isNonDecreasing reports whether pts (already sorted ascending by
// Percentile) has Value non-decreasing across consecutive points — the
// precondition interpolate/percentileOfValue/benchmarkRange/classifyBand
// all assume (see IssueNonMonotonicBenchmarkPoints).
func isNonDecreasing(pts []PercentilePoint) bool {
	for i := 1; i < len(pts); i++ {
		if pts[i].Value < pts[i-1].Value {
			return false
		}
	}
	return true
}

// dedupSortPoints returns pts sorted ascending by Percentile, with
// duplicate percentiles collapsed (the last occurrence in input order
// wins) — never mutates pts.
func dedupSortPoints(pts []PercentilePoint) []PercentilePoint {
	byPercentile := make(map[float64]float64, len(pts))
	order := make([]float64, 0, len(pts))
	for _, p := range pts {
		if _, seen := byPercentile[p.Percentile]; !seen {
			order = append(order, p.Percentile)
		}
		byPercentile[p.Percentile] = p.Value
	}
	sort.Float64s(order)
	out := make([]PercentilePoint, len(order))
	for i, pct := range order {
		out[i] = PercentilePoint{Percentile: pct, Value: byPercentile[pct]}
	}
	return out
}

// peerPercentilePoints derives a sorted set of (percentile, value) points
// from raw peer observations using the linear-interpolation ("R-7"/
// Excel PERCENTILE.INC) rank convention: the i-th smallest of n sorted
// values (0-indexed) sits at percentile 100*i/(n-1). This is the same
// convention interpolate's inverse (percentileOfValue) assumes, so the
// two stay consistent with each other. A single observation sits at the
// 50th percentile by convention (n-1 == 0), reported as the median but
// left insufficientPoints for interpolation purposes.
func peerPercentilePoints(obs []PeerObservation) (points []PercentilePoint, median Value) {
	values := make([]float64, len(obs))
	for i, o := range obs {
		values[i] = o.Value
	}
	sort.Float64s(values)

	n := len(values)
	points = make([]PercentilePoint, n)
	if n == 1 {
		points[0] = PercentilePoint{Percentile: 50, Value: values[0]}
		return points, AvailableValue(values[0])
	}
	for i, v := range values {
		pct := 100 * float64(i) / float64(n-1)
		points[i] = PercentilePoint{Percentile: pct, Value: v}
	}
	median = interpolate(points, 50)
	return points, median
}

// interpolate returns the value at percentile pct via piecewise-linear
// interpolation across pts (already sorted ascending by Percentile, with
// unique percentiles). Returns Unavailable if pts is empty. A pct below
// the first point or above the last is clamped to the nearest known
// endpoint (extrapolation is not attempted — this package reports a
// value's relationship to the known benchmark range, never a guess beyond
// it).
func interpolate(pts []PercentilePoint, pct float64) Value {
	if len(pts) == 0 {
		return Unavailable()
	}
	if len(pts) == 1 {
		return AvailableValue(pts[0].Value)
	}
	if pct <= pts[0].Percentile {
		return AvailableValue(pts[0].Value)
	}
	last := pts[len(pts)-1]
	if pct >= last.Percentile {
		return AvailableValue(last.Value)
	}
	for i := 0; i < len(pts)-1; i++ {
		lo, hi := pts[i], pts[i+1]
		if pct >= lo.Percentile && pct <= hi.Percentile {
			if hi.Percentile == lo.Percentile {
				return AvailableValue(lo.Value)
			}
			frac := (pct - lo.Percentile) / (hi.Percentile - lo.Percentile)
			return AvailableValue(lo.Value + frac*(hi.Value-lo.Value))
		}
	}
	return AvailableValue(last.Value)
}

// percentileOfValue is interpolate's inverse: given pts (sorted ascending
// by Percentile, unique percentiles, at least two points) and a company
// value v, estimates what percentile v falls at via inverse linear
// interpolation across the same piecewise-linear curve interpolate
// assumes. A v below the lowest known point or above the highest is
// clamped to the nearest known endpoint's percentile (0 or 100 only when
// that endpoint itself sits at the population's true min/max — for
// FormPeerObservations the endpoints are the actual min/max, so clamping
// there is exact; for FormPercentileBands/FormQuartiles the lowest/
// highest supplied point is not necessarily the population's true min/max,
// so a value beyond it is reported at that point's own percentile, not
// forced to 0/100 — see the Band derivation in evaluate.go for how a
// value beyond the known points is still distinguished as BandBelowMin/
// BandAboveMax rather than merely clamped-percentile).
func percentileOfValue(pts []PercentilePoint, v float64) float64 {
	if v <= pts[0].Value {
		return pts[0].Percentile
	}
	last := pts[len(pts)-1]
	if v >= last.Value {
		return last.Percentile
	}
	for i := 0; i < len(pts)-1; i++ {
		lo, hi := pts[i], pts[i+1]
		if v >= lo.Value && v <= hi.Value {
			if hi.Value == lo.Value {
				return lo.Percentile
			}
			frac := (v - lo.Value) / (hi.Value - lo.Value)
			return lo.Percentile + frac*(hi.Percentile-lo.Percentile)
		}
	}
	return last.Percentile
}
