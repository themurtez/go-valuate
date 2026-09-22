package consensus

// Level is a human-facing agreement tier, derived deterministically from a
// numeric Score via fixed thresholds — see levelForDispersionScore.
// Mirrors valuation/applicability.Level's role: never a statistical
// confidence interval, never an industry-standard measure, always
// traceable back to CoefficientOfVariation via a documented formula (see
// CalculateDispersion's doc comment).
type Level string

const (
	// LevelHighConsensus means included methods produced closely agreeing
	// values (low dispersion).
	LevelHighConsensus Level = "HIGH_CONSENSUS"
	// LevelModerateConsensus means included methods produced moderately
	// dispersed values.
	LevelModerateConsensus Level = "MODERATE_CONSENSUS"
	// LevelLowConsensus means included methods produced widely dispersed
	// values.
	LevelLowConsensus Level = "LOW_CONSENSUS"
)

// dispersionScoreCVDivisor is the CoefficientOfVariation value at which
// Score reaches 0 (maximum disagreement on this package's fixed scale).
// Chosen so that a CV of 0 (every method agrees exactly) scores 100, a CV
// of 0.5 (spread comparable in magnitude to the mean — methods disagree by
// roughly half the central value on average) scores 0, and CV values in
// between are linearly interpolated. A CV at or above this divisor floors
// at Score 0 rather than going negative — see CalculateDispersion.
const dispersionScoreCVDivisor = 0.5

// levelForDispersionScore maps a clamped [0,100] Score to a Level via
// fixed thresholds, the single source of truth for this mapping:
//
//	70-100 -> HIGH_CONSENSUS
//	40-69  -> MODERATE_CONSENSUS
//	0-39   -> LOW_CONSENSUS
func levelForDispersionScore(score int) Level {
	switch {
	case score >= 70:
		return LevelHighConsensus
	case score >= 40:
		return LevelModerateConsensus
	default:
		return LevelLowConsensus
	}
}

// Dispersion is the agreement/dispersion indicator: the raw measures it
// was derived from, plus the derived Score/Level.
type Dispersion struct {
	// CoefficientOfVariation echoes Statistics.CoefficientOfVariation — the
	// single raw measure Score is derived from.
	CoefficientOfVariation float64 `json:"coefficient_of_variation"`
	// RelativeSpread is Spread / |SimpleMean| — a second raw, independently
	// interpretable dispersion measure returned alongside
	// CoefficientOfVariation for transparency, but not itself part of the
	// Score formula (see CalculateDispersion's doc comment on why Score
	// uses CV alone). Zero when SimpleMean is zero.
	RelativeSpread float64 `json:"relative_spread"`
	// Score is a deterministic [0,100] transformation of
	// CoefficientOfVariation: Score = round(100 * (1 -
	// CV/dispersionScoreCVDivisor)), clamped to [0,100]. 100 means every
	// included method agreed exactly (CV == 0); 0 means CV is at or beyond
	// dispersionScoreCVDivisor. This is a fixed, documented heuristic
	// transformation for human-facing display — not an industry-standard
	// statistic, and not a probability that the methods "agree."
	Score int `json:"score"`
	// Level is Score passed through levelForDispersionScore's fixed
	// thresholds.
	Level Level `json:"level"`
}

// CalculateDispersion derives a Dispersion from already-computed
// Statistics. Returns the zero Dispersion (Score 0, Level
// LevelLowConsensus) if stats.Count == 0, since there is nothing to
// measure agreement over — callers should treat this the same as
// Result.Available == false rather than reading it as "methods
// disagreed."
//
// A single-value Statistics (Count == 1) is the opposite edge case:
// StdDev and CoefficientOfVariation are mathematically 0 for any
// one-element set (there is no other value to differ from), so Score is
// exactly 100 and Level is LevelHighConsensus. This is "dispersion is
// zero BY DEFINITION," not "dispersion is unavailable" or "not
// meaningful" — a single included method cannot disagree with itself,
// which is a genuinely different statement from "we don't know how much
// the methods agree." Result.Warnings still flags the single-method case
// (see Calculate), since a Score of 100 from one method carries far less
// evidentiary weight than the same Score from several independently
// agreeing methods — but the Score/Level themselves are not a special
// case in the formula below.
func CalculateDispersion(stats Statistics) Dispersion {
	if stats.Count == 0 {
		return Dispersion{}
	}

	cv := stats.CoefficientOfVariation
	relativeSpread := percentOf(stats.Spread, stats.SimpleMean)

	ratio := cv / dispersionScoreCVDivisor
	rawScore := 100 * (1 - ratio)
	score := clampDispersionScore(roundHalfAwayFromZero(rawScore))

	return Dispersion{
		CoefficientOfVariation: cv,
		RelativeSpread:         relativeSpread,
		Score:                  score,
		Level:                  levelForDispersionScore(score),
	}
}

// roundHalfAwayFromZero rounds a float to the nearest int, with .5 rounding
// away from zero. rawScore is negative whenever CV exceeds
// dispersionScoreCVDivisor, so this must round correctly in both
// directions rather than assuming a non-negative input.
func roundHalfAwayFromZero(v float64) int {
	if v < 0 {
		return -int(-v + 0.5)
	}
	return int(v + 0.5)
}

func clampDispersionScore(v int) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}
