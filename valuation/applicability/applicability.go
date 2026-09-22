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

// FilterPolicy is a caller-selected, deterministic policy for how an
// orchestrator (or any other caller consuming applicability scores)
// should use Results to decide which methods to actually run. Every
// policy is fully specified by its name alone — none of them consult
// anything beyond Results and, for PolicyMinimumLevel/
// PolicyExplicitSelection, the caller's own accompanying threshold/
// selection — so a caller can choose a policy without touching how
// applicability itself is scored. See valuation/orchestrator.Request for
// where this is consumed; this package defines the policy vocabulary
// without depending on orchestrator.
type FilterPolicy string

const (
	// PolicyIncludeAllEnabled means applicability is purely informational:
	// every settings-enabled method with a supplied Input runs regardless
	// of its Score/Level. This is the zero-value policy — a caller that
	// never opts into filtering gets exactly today's behavior.
	PolicyIncludeAllEnabled FilterPolicy = "INCLUDE_ALL_ENABLED"
	// PolicyExcludeNotApplicable excludes only a method whose Level is
	// exactly LevelNotApplicable (a hard block, e.g. DCF with no
	// forecast — see Result.HardBlockReason) — the least aggressive
	// filtering policy that still respects a genuine hard block.
	PolicyExcludeNotApplicable FilterPolicy = "EXCLUDE_NOT_APPLICABLE"
	// PolicyMinimumLevel excludes any method whose Level ranks below a
	// caller-supplied minimum threshold (see levelRank's ordering:
	// NOT_APPLICABLE < LOW < MEDIUM < HIGH). This is today's
	// MinApplicabilityLevel behavior, now named as one policy among
	// several rather than the only option.
	PolicyMinimumLevel FilterPolicy = "MINIMUM_LEVEL"
	// PolicyExplicitSelection ignores Score/Level for inclusion entirely:
	// only methods a caller explicitly names run, regardless of how they
	// scored. Applicability is still echoed on every MethodOutcome for
	// display, but plays no role in whether a method executes under this
	// policy.
	PolicyExplicitSelection FilterPolicy = "EXPLICIT_METHOD_SELECTION"
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

// Result is one method's applicability outcome for a given Profile. Every
// field here exists so an accountant reviewing a recommendation can
// reconstruct exactly how Score was reached without reading this
// package's source — see Calculate's doc comment for the full worked
// example this shape is designed to support:
//
//	base score:                         50
//	owner-operated service business:  +25
//	low asset intensity:              +15
//	stable positive earnings:         +15
//	--------------------------------------
//	raw score:                        105
//	clamped score:                     100
type Result struct {
	// Method is the valuation.Code this result scores.
	Method valuation.Code `json:"method"`
	// BaseScore is the starting point every rule's Reasons were added to
	// or subtracted from — always baseScore (50) today, but exposed
	// explicitly (rather than left as an unstated constant only visible in
	// source) so the worked example above is fully reproducible from
	// Result alone.
	BaseScore int `json:"base_score"`
	// RawScore is BaseScore plus every Reason's Points, before clamping —
	// i.e. what Score would be if it were allowed to fall outside [0,100].
	// Equal to Score whenever Clamped is false.
	RawScore int `json:"raw_score"`
	// Score is RawScore clamped to [0, 100] — see Calculate's doc comment
	// for the exact fixed point scale each rule uses. Score is a
	// deterministic heuristic total, not a statistical probability or an
	// industry-standard confidence measure.
	Score int `json:"score"`
	// Clamped is true if RawScore fell outside [0, 100] and Score is
	// consequently not equal to RawScore — a visible flag rather than
	// requiring a caller to compare the two fields themselves to notice
	// clamping occurred.
	Clamped bool `json:"clamped"`
	// Level is Score passed through levelForScore's fixed thresholds.
	Level Level `json:"level"`
	// Recommended is true when Level is LevelHigh or LevelMedium — a
	// convenience boolean for a caller (e.g. valuation/orchestrator) that
	// wants a simple include/exclude default without inspecting Level
	// itself. A caller is always free to override this (e.g. force-include
	// a LevelLow method); Recommended is a default, not a constraint.
	Recommended bool `json:"recommended"`
	// HardBlockReason is non-empty only when a rule short-circuited
	// straight to NOT_APPLICABLE regardless of every other rule (today,
	// only scoreDCF's absent-forecast case — see pointsBlocking), as
	// opposed to a low Score reached by the ordinary accumulation of
	// Reasons. Distinguishing the two matters to a reviewer: a hard block
	// means "this method cannot run at all without more data," while a low
	// accumulated Score means "this method could run, but is a poor fit."
	HardBlockReason string `json:"hard_block_reason,omitempty"`
	// Reasons lists every Reason that contributed to Score, in evaluation
	// order, so Score is always fully explained.
	Reasons []Reason `json:"reasons"`
	// Warnings carries non-scoring advisory notes (e.g. "DCF requires an
	// explicit forecast this package does not generate") distinct from
	// Reasons that actually moved Score.
	Warnings []string `json:"warnings,omitempty"`
}
