package qoe

// ScoreVersion identifies the exact formula computeScore implements. A
// caller persisting Score alongside a historical Result should treat a
// ScoreVersion change the same way the README's versioning-strategy section
// treats any other formula version bump: a historical Score computed under
// an older ScoreVersion is not directly comparable to one computed under a
// newer one. Distinct from FormulaVersion because a caller may reasonably
// want to change how the flags/ratios themselves are computed
// (FormulaVersion) independently of how those already-computed inputs are
// weighted into one composite number (ScoreVersion) — though in practice
// this package bumps both together today; the separate constant exists so
// that need not remain true forever.
const ScoreVersion = "1.0.0"

// Score is the optional deterministic 0-100 heuristic earnings-quality
// score. It is explicitly a heuristic composite, not an accounting
// standard or a statistically calibrated probability — see the package doc
// comment and ScoreVersion.
//
// Formula (exact, fixed): Score starts at a 100-point baseline and
// Components lists every deduction subtracted from it, clamped to [0, 100].
// Every deduction is driven entirely by Result.Flags (already-computed,
// already-explainable rule-based signals — see flags.go) plus
// Result.Ratios — Score never introduces a new threshold or computation of
// its own beyond re-reading what Calculate already determined. This means
// Score can never disagree with Flags about whether something is a
// problem; it only weights problems already identified into one number.
//
//	100 points, baseline
//	- 15 points  per FlagSeverityCritical flag in Result.Flags
//	-  8 points  per FlagSeverityWarning flag in Result.Flags
//	-  3 points  per FlagSeverityInfo flag in Result.Flags
//	clamped to [0, 100]
type Score struct {
	// Version echoes ScoreVersion.
	Version string `json:"version"`
	// Value is the final clamped [0, 100] score.
	Value float64 `json:"value"`
	// Components lists every deduction that contributed to Value, in the
	// same order Result.Flags is in (never Go map order), so a caller can
	// reconstruct exactly how Value was reached from Result.Flags alone.
	Components []ScoreComponent `json:"components"`
	// Label is a short, fixed, human-readable characterization of Value's
	// range — see labelForScore. Purely descriptive; a caller should
	// branch on Value's numeric range directly if it needs to make a
	// decision, never parse Label.
	Label string `json:"label"`
	// Heuristic is always true and always present in the JSON output —
	// see the package doc comment's explicit requirement that this score
	// be "clearly labeled heuristic, not an accounting standard." A
	// caller deserializing this type without reading its documentation
	// still sees the field name in the payload itself.
	Heuristic bool `json:"heuristic"`
}

// ScoreComponent is one named deduction (or, in principle, addition)
// contributing to Score.Value.
type ScoreComponent struct {
	// Label identifies which flag (by FlagCode and, when applicable,
	// Period) this component corresponds to.
	Label string `json:"label"`
	// Points is this component's contribution to Score.Value — negative
	// for a deduction, the only kind this package's fixed formula
	// currently produces.
	Points float64 `json:"points"`
}

const (
	scoreBaseline          = 100.0
	scoreDeductionCritical = 15.0
	scoreDeductionWarning  = 8.0
	scoreDeductionInfo     = 3.0
)

// computeScore implements Score's documented formula over result.Flags.
// result must already have Flags populated (see Calculate's call order:
// buildFlags always runs before computeScore). Takes no Thresholds of its
// own, by design — see Score's doc comment: every deduction reads only
// Result.Flags, which Calculate has already evaluated against Thresholds.
func computeScore(result Result) Score {
	value := scoreBaseline
	components := make([]ScoreComponent, 0, len(result.Flags))

	for _, f := range result.Flags {
		var deduction float64
		switch f.Severity {
		case FlagSeverityCritical:
			deduction = scoreDeductionCritical
		case FlagSeverityWarning:
			deduction = scoreDeductionWarning
		case FlagSeverityInfo:
			deduction = scoreDeductionInfo
		default:
			continue
		}
		value -= deduction
		label := string(f.Code)
		if f.Period != "" {
			label += " (" + string(f.Period) + ")"
		}
		components = append(components, ScoreComponent{Label: label, Points: -deduction})
	}

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
	}
}

// labelForScore returns a fixed, documented characterization band for
// value. Bands: [85, 100] "high quality", [65, 85) "moderate quality",
// [40, 65) "elevated concern", [0, 40) "low quality". These bounds are part
// of ScoreVersion's fixed formula — changing them requires bumping
// ScoreVersion.
func labelForScore(value float64) string {
	switch {
	case value >= 85:
		return "high quality"
	case value >= 65:
		return "moderate quality"
	case value >= 40:
		return "elevated concern"
	default:
		return "low quality"
	}
}
