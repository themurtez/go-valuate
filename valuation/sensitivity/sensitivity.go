// Package sensitivity provides reusable, deterministic sensitivity
// analysis over already-defined valuation calculations: how a method's
// value changes across a caller-supplied grid of multiples, earnings
// scenarios, or DCF discount-rate/terminal-growth-rate combinations.
//
// This package generates no scenarios itself — every multiple, earnings
// adjustment, discount rate, and terminal growth rate analyzed here is
// caller-supplied, mirroring every other package in this repository's
// "no invented figures" rule. It only re-runs the relevant method's own
// Calculate (valuation/sde, valuation/dcf) or applies the same multiple
// formula those packages use, once per grid cell, and reports every cell
// — including invalid combinations, which are marked invalid rather than
// silently skipped or computed under an unstated fallback. Every function
// here is pure: no I/O, no mutation of its inputs.
package sensitivity

import "math"

// MultiplePoint is one multiple's resulting value in a MultipleSensitivity
// analysis.
type MultiplePoint struct {
	// Multiple is the multiple analyzed.
	Multiple float64 `json:"multiple"`
	// Value is Earnings * Multiple. Meaningful only when Valid is true.
	Value float64 `json:"value"`
	// Valid is false if Multiple was non-finite or <= 0 — the same
	// validation valuation/sde and valuation/ebitda apply to a multiple,
	// applied here per-point instead of failing the whole analysis.
	Valid bool `json:"valid"`
	// Reason explains why Valid is false, when it is.
	Reason string `json:"reason,omitempty"`
}

// MultipleSensitivityResult is the output of MultipleSensitivity.
type MultipleSensitivityResult struct {
	// Earnings is the single earnings figure (e.g. maintainable SDE or
	// EBITDA) every Points entry was computed against.
	Earnings float64 `json:"earnings"`
	// Points lists one MultiplePoint per caller-supplied multiple, in
	// input order.
	Points []MultiplePoint `json:"points"`
}

// MultipleSensitivity computes Earnings * multiple for every multiple in
// multiples, in order, marking non-finite or non-positive multiples
// invalid rather than computing a nonsensical value for them. Earnings
// itself is not validated here (a non-finite Earnings would make every
// point non-finite; this package leaves that to the caller, mirroring
// this analysis being a thin re-application of the same formula
// valuation/sde.Calculate and valuation/ebitda.Calculate already
// validate).
func MultipleSensitivity(earnings float64, multiples []float64) MultipleSensitivityResult {
	points := make([]MultiplePoint, 0, len(multiples))
	for _, m := range multiples {
		if !isFinite(m) {
			points = append(points, MultiplePoint{Multiple: m, Valid: false, Reason: "multiple is not a finite number"})
			continue
		}
		if m <= 0 {
			points = append(points, MultiplePoint{Multiple: m, Valid: false, Reason: "multiple must be greater than zero"})
			continue
		}
		points = append(points, MultiplePoint{Multiple: m, Value: earnings * m, Valid: true})
	}
	return MultipleSensitivityResult{Earnings: earnings, Points: points}
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}
