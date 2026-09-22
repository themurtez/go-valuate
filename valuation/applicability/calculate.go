package applicability

import "github.com/themurtez/go-valuate/valuation/profile"

// Results holds one applicability Result per method, in a fixed
// deterministic order (matching valuation's Code declaration order), for
// a single Profile.
type Results struct {
	// Profile echoes the exact profile.Profile Calculate was given, so a
	// Results value is self-contained.
	Profile profile.Profile `json:"profile"`
	// Methods lists every method's Result, in a fixed order: SDE, EBITDA,
	// Capitalization of Earnings, DCF, Adjusted Net Asset Value.
	Methods []Result `json:"methods"`
}

// ForMethod returns the Result for a specific method within r, and
// whether one was found. Results always contains exactly one entry per
// method Calculate knows about, so ok is false only if code is not a
// method this package scores.
func (r Results) ForMethod(code string) (Result, bool) {
	for _, m := range r.Methods {
		if string(m.Method) == code {
			return m, true
		}
	}
	return Result{}, false
}

// Calculate scores every individual valuation method's applicability to p
// using this package's fixed, deterministic rules (see rules.go). Calculate
// never fails and never panics: an empty Profile is valid input and simply
// produces every method's baseline (MEDIUM) score, with DCF as the one
// exception that always starts blocked at NOT_APPLICABLE absent an
// explicit forecast (see scoreDCF's doc comment).
func Calculate(p profile.Profile) Results {
	return Results{
		Profile: p,
		Methods: []Result{
			scoreSDE(p),
			scoreEBITDA(p),
			scoreCapitalization(p),
			scoreDCF(p),
			scoreNetAssets(p),
		},
	}
}
