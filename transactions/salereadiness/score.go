package salereadiness

const (
	scorePointsStrong     = 10.0
	scorePointsAcceptable = 7.0
	scorePointsWeak       = 3.0
	scorePointsConcerning = 0.0
)

// computeOverallScore implements Score's documented, fixed formula: each
// assessed (non-StatusUnassessed) Dimension in dims contributes a fixed
// point value by its Status (StatusStrong=10, StatusAcceptable=7,
// StatusWeak=3, StatusConcerning=0), averaged over the number of assessed
// dimensions and scaled to [0, 100] — so Score.Value always reflects only
// the dimensions actually assessed, never diluted by StatusUnassessed
// entries (which contribute neither points nor to the divisor). Returns
// (Score{}, false) when zero dimensions were assessed, since an average
// over zero terms has nothing to report — see Result.OverallScore's doc
// comment.
//
// This formula never disagrees with Dimensions about which dimensions are
// concerning: it only weights already-classified Statuses into one
// composite number, mirroring qoe.Score's identical "never introduces a
// new threshold or computation of its own" design.
func computeOverallScore(dims []Dimension) (Score, bool) {
	var totalPoints float64
	components := make([]ScoreComponent, 0, len(dims))
	assessed := 0

	for _, d := range dims {
		var points float64
		switch d.Status {
		case StatusStrong:
			points = scorePointsStrong
		case StatusAcceptable:
			points = scorePointsAcceptable
		case StatusWeak:
			points = scorePointsWeak
		case StatusConcerning:
			points = scorePointsConcerning
		case StatusUnassessed:
			continue
		default:
			continue
		}
		assessed++
		totalPoints += points
		components = append(components, ScoreComponent{Dimension: d.Code, Status: d.Status, Points: points})
	}

	if assessed == 0 {
		return Score{}, false
	}

	maxPoints := scorePointsStrong
	value := (totalPoints / (float64(assessed) * maxPoints)) * 100
	if value < 0 {
		value = 0
	}
	if value > 100 {
		value = 100
	}

	return Score{
		Version:    ScoreVersion,
		Value:      value,
		Components: components,
		Label:      labelForScore(value),
		Heuristic:  true,
	}, true
}

// labelForScore returns a short, fixed, human-readable characterization of
// value's range — purely descriptive, mirroring qoe.labelForScore's
// identical role and fixed band boundaries.
func labelForScore(value float64) string {
	switch {
	case value >= 85:
		return "Sale ready"
	case value >= 70:
		return "Mostly ready"
	case value >= 50:
		return "Needs preparation"
	default:
		return "Significant preparation needed"
	}
}
