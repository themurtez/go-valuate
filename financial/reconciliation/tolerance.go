package reconciliation

import "math"

// Tolerance controls how large a difference between an expected and actual
// value can be while still counting as StatusPass, absorbing ordinary
// accounting rounding differences (penny rounding across many summed line
// items, rounding of percentages, etc.) without masking real discrepancies.
//
// A Tolerance is supplied per-call via Options, never hard-coded as a single
// global inside this package: different callers reconcile datasets of very
// different scale (a $50k HVAC business vs. a $50M manufacturer), and a
// fixed absolute tolerance appropriate for one is meaningless for the other.
type Tolerance struct {
	// Absolute is the maximum allowed |difference| in the dataset's
	// currency units, regardless of the size of the values being compared.
	Absolute float64 `json:"absolute,omitempty"`
	// RelativePercent is the maximum allowed |difference| / |expected|,
	// expressed as a decimal (0.01 = 1%). Optional: zero means no relative
	// tolerance is applied, only Absolute. Ignored when Expected is 0 (a
	// relative tolerance against a zero base is undefined; see
	// Tolerance.Evaluate).
	RelativePercent float64 `json:"relative_percent,omitempty"`
}

// DefaultTolerance is a small absolute tolerance suitable for typical
// small-business financials, used only when a caller supplies a zero-value
// Tolerance to Options — see Options.tolerance. Callers reconciling
// larger-scale statements should supply their own Tolerance explicitly
// rather than relying on this default.
var DefaultTolerance = Tolerance{Absolute: 1.0}

// Evaluate compares actual against expected under this tolerance and
// returns the resulting Status (StatusPass or StatusFail — Evaluate never
// returns StatusWarning or StatusNotApplicable; callers upgrade a
// borderline Pass to a Warning themselves when they have context Evaluate
// doesn't, such as materiality relative to the whole statement) along with
// the raw difference.
//
// A difference passes if it is within Absolute OR, when RelativePercent is
// nonzero and expected is nonzero, within RelativePercent of |expected| —
// whichever is more permissive. This "OR" combination (rather than
// "AND") is deliberate: a tiny absolute tolerance would otherwise reject
// small rounding noise on very large statements, while a tiny relative
// tolerance would reject any noise at all on near-zero expected values;
// combining them with OR lets either one independently absorb the
// rounding differences it's suited for.
func (t Tolerance) Evaluate(expected, actual float64) (status Status, difference float64) {
	difference = actual - expected
	abs := math.Abs(difference)

	if abs <= t.Absolute {
		return StatusPass, difference
	}
	if t.RelativePercent > 0 && expected != 0 {
		relativeLimit := math.Abs(expected) * t.RelativePercent
		if abs <= relativeLimit {
			return StatusPass, difference
		}
	}
	return StatusFail, difference
}
