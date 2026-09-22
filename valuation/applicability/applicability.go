// Package applicability implements deterministic, transparent rules that
// score how well-suited each individual valuation method
// (valuation/sde, ebitda, capitalization, dcf, netassets) is to a
// particular business, described by a valuation/profile.Profile.
//
// Every rule here is a fixed, documented point-scoring heuristic over the
// Profile fields the caller chose to supply — never a statistical model,
// never trained on data, and never a probability. Score's doc comment and
// this package's README section spell out the exact scoring so a reviewer
// can audit why a method landed at a given Level without reverse-engineering
// the code. See the package-level warning against calling this a
// statistical probability: Score is comparable across methods for the same
// Profile (it is the same fixed point scale for all five), but it is not
// comparable to any external/industry benchmark and does not mean
// "probability this method is correct."
//
// This package computes no valuation figures itself and performs no I/O.
package applicability

import "github.com/themurtez/go-valuate/valuation"

// Level is a human-facing applicability tier, derived deterministically
// from a numeric Score via fixed thresholds (see levelForScore). Never a
// statistical confidence interval.
type Level string

const (
	LevelHigh          Level = "HIGH"
	LevelMedium        Level = "MEDIUM"
	LevelLow           Level = "LOW"
	LevelNotApplicable Level = "NOT_APPLICABLE"
)

// ReasonKind distinguishes a reason that reflects the business's
// characteristics (Fit) from one that reflects missing/insufficient input
// data (DataGap) — the same Level can be reached for structurally
// different reasons, and a caller (or future report/UI) may want to treat
// "this method doesn't suit this business" very differently from "this
// method could apply, but the required input was never supplied."
type ReasonKind string

const (
	// ReasonFit means the point applies because of a business
	// characteristic (size, ownership structure, asset intensity, etc.).
	ReasonFit ReasonKind = "fit"
	// ReasonDataGap means the point applies because a Profile field (or
	// Profile.DataAvailability flag) needed to score or run this method
	// confidently was left unset/false.
	ReasonDataGap ReasonKind = "data_gap"
)

// Reason is a single scored contribution (positive or negative) to a
// method's Score, or a non-scoring note. Every rule that changes Score
// emits exactly one Reason, so Score is never a number without an audit
// trail — nothing here is scored silently.
type Reason struct {
	// Kind classifies this reason as reflecting business fit or a data gap.
	Kind ReasonKind `json:"kind"`
	// Detail is a short, fixed, human-readable explanation (e.g. "owner-
	// operated business: SDE reflects the full owner-operator return").
	Detail string `json:"detail"`
	// Points is this reason's signed contribution to Score. Zero for a
	// non-scoring informational Reason (rare; most Reasons carry a
	// nonzero Points).
	Points int `json:"points"`
}

// Result is one method's applicability outcome for a given Profile.
type Result struct {
	// Method is the valuation.Code this result scores.
	Method valuation.Code `json:"method"`
	// Score is the sum of every Reason's Points, clamped to [0, 100] —
	// see Calculate's doc comment for the exact fixed point scale each
	// rule uses. Score is a deterministic heuristic total, not a
	// statistical probability or an industry-standard confidence measure.
	Score int `json:"score"`
	// Level is Score passed through levelForScore's fixed thresholds.
	Level Level `json:"level"`
	// Recommended is true when Level is LevelHigh or LevelMedium — a
	// convenience boolean for a caller (e.g. valuation/orchestrator) that
	// wants a simple include/exclude default without inspecting Level
	// itself. A caller is always free to override this (e.g. force-include
	// a LevelLow method); Recommended is a default, not a constraint.
	Recommended bool `json:"recommended"`
	// Reasons lists every Reason that contributed to Score, in evaluation
	// order, so Score is always fully explained.
	Reasons []Reason `json:"reasons"`
	// Warnings carries non-scoring advisory notes (e.g. "DCF requires an
	// explicit forecast this package does not generate") distinct from
	// Reasons that actually moved Score.
	Warnings []string `json:"warnings,omitempty"`
}
