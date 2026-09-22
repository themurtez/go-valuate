// Package report defines a presentation-neutral, JSON-serializable
// report data model over the outputs of every other package in this
// repository: financial metrics, adjustments, individual valuation
// methods, applicability, orchestration, consensus, and sensitivity.
//
// This package computes nothing new itself — every figure in a Report was
// already produced by an upstream package's Calculate/Run/Apply; Build
// (see build.go) only reshapes those outputs into a single structure
// suitable for a future Vue UI, a JSON API response, PDF generation, or a
// CSV/export pipeline, none of which this package implements. Report
// contains no chart library types, no HTML, no PDF bytes — only plain
// structs, strings, and numbers that serialize cleanly with
// encoding/json (see the package's serialization tests).
//
// Consensus is not "true value," and applicability/dispersion scores are
// deterministic heuristics, never statistical probabilities or AI/external
// data — see valuation/consensus and valuation/applicability's package doc
// comments, which this package's output faithfully carries through
// without softening or losing that framing.
package report

import (
	"github.com/themurtez/go-valuate/valuation"
	"github.com/themurtez/go-valuate/valuation/applicability"
	"github.com/themurtez/go-valuate/valuation/consensus"
)

// SchemaVersion identifies this package's fixed Report shape (every
// section/field Build populates). Bump this whenever a field is added,
// removed, renamed, or reinterpreted in a way that could make a
// historically persisted Report not deserialize/interpret identically
// under the new code — see the repository README's versioning-strategy
// section. Distinct from any upstream package's own version (e.g.
// consensus.FormulaVersion, a method's MethodVersion): SchemaVersion
// versions only this package's own reshaping/presentation layer.
const SchemaVersion = "1.0.0"

// Report is the complete presentation-neutral valuation report: every
// section a future UI, API response, or export would need, assembled from
// upstream package outputs by Build.
type Report struct {
	// SchemaVersion identifies which version of this package's Report
	// shape produced this value — see the SchemaVersion constant's doc
	// comment.
	SchemaVersion string `json:"schema_version"`
	// Summary is the top-of-report consensus/range overview.
	Summary Summary `json:"summary"`
	// Financial is the historical financial summary (revenue, EBITDA, SDE,
	// margins, growth).
	Financial FinancialSummary `json:"financial"`
	// Methods lists every method's comparison row (value, value type,
	// included/excluded, weight, applicability, assumptions, warnings).
	Methods []MethodComparisonRow `json:"methods"`
	// Adjustments is the adjustment/normalization summary, when supplied to
	// Build.
	Adjustments AdjustmentSummary `json:"adjustments"`
	// Sensitivity is the structured sensitivity data (rows/matrices only —
	// no charts), when supplied to Build.
	Sensitivity SensitivityData `json:"sensitivity"`
	// Series holds every chart-ready data series (see series.go). Entirely
	// presentation-neutral: plain (label, value) pairs a future charting
	// library of the caller's choice can consume directly, with no
	// dependency on any specific library here.
	Series ChartSeries `json:"series"`
}

// Summary is the report's top-level consensus/range overview.
type Summary struct {
	// ValuationDate is the caller-supplied as-of date for this valuation,
	// in RFC 3339 date form (e.g. "2026-09-22"), or empty if not supplied
	// — this package neither generates nor requires one; a report is still
	// complete without it (see Build's doc comment).
	ValuationDate string `json:"valuation_date,omitempty"`
	// SimpleConsensus echoes consensus.Statistics.SimpleMean, under the
	// caller-facing name the package brief specifies. Meaningful only when
	// ConsensusAvailable is true.
	SimpleConsensus float64 `json:"simple_consensus"`
	// WeightedConsensus echoes consensus.Statistics.WeightedMean.
	// Meaningful only when ConsensusAvailable && WeightsValid.
	WeightedConsensus float64 `json:"weighted_consensus"`
	// WeightsValid echoes consensus.Result.WeightsValid — whether
	// WeightedConsensus is meaningful.
	WeightsValid bool `json:"weights_valid"`
	// Median echoes consensus.Statistics.Median.
	Median float64 `json:"median"`
	// MethodRange echoes consensus.Result.Range: the (min, max) span of
	// every included method's value — never a narrower "likely range"; see
	// valuation/consensus' package doc comment on why this package invents
	// no such range either.
	MethodRange consensus.Range `json:"method_range"`
	// ConsensusLevel echoes consensus.Dispersion.Level — see
	// valuation/consensus' package doc comment: a deterministic heuristic
	// agreement indicator, not a statistical confidence measure.
	ConsensusLevel consensus.Level `json:"consensus_level"`
	// ConsensusScore echoes consensus.Dispersion.Score.
	ConsensusScore int `json:"consensus_score"`
	// ConsensusAvailable is false if no consensus.Result was supplied to
	// Build, or the supplied one had Available == false (e.g. zero included
	// methods) — every other Summary field is zero-value in that case.
	ConsensusAvailable bool `json:"consensus_available"`
	// IncludedMethodCount is the number of methods that contributed to
	// consensus — a convenience count mirroring
	// consensus.Statistics.Count.
	IncludedMethodCount int `json:"included_method_count"`
}

// MethodComparisonRow is one method's row in the report's method
// comparison table.
type MethodComparisonRow struct {
	// Method is the method's stable valuation.Code.
	Method valuation.Code `json:"method"`
	// MethodVersion echoes the method's own Result.MethodVersion, empty if
	// the method never ran (excluded).
	MethodVersion string `json:"method_version,omitempty"`
	// ValueType is the kind of value Value represents, empty if the method
	// never ran.
	ValueType valuation.ValueType `json:"value_type,omitempty"`
	// Value is the method's headline figure. Zero if Included is false.
	Value float64 `json:"value"`
	// Included is true only if the method ran and produced an Available
	// result (mirrors orchestrator.OutcomeSuccess) — an unavailable or
	// excluded method is never silently shown as if it contributed a
	// value.
	Included bool `json:"included"`
	// Outcome echoes orchestrator.Outcome (success/unavailable/excluded),
	// for a caller that wants the precise reason beyond the coarser
	// Included boolean.
	Outcome string `json:"outcome"`
	// ExclusionReason echoes orchestrator.ExclusionReason when Outcome is
	// "excluded".
	ExclusionReason string `json:"exclusion_reason,omitempty"`
	// Weight is this method's normalized weight as used in
	// WeightedConsensus (consensus.Input.Weight after normalization — see
	// valuation/consensus.ValidateWeights), zero if not included or no
	// weight was supplied.
	Weight float64 `json:"weight,omitempty"`
	// Applicability echoes this method's applicability.Result, if supplied
	// to Build.
	Applicability *applicability.Result `json:"applicability,omitempty"`
	// Assumptions lists the method's key caller-supplied assumptions as
	// plain (label, value) pairs (e.g. "Multiple" -> "3.5x",
	// "Discount Rate" -> "18%") — see build.go's per-method assumption
	// extraction. Deliberately plain strings rather than each method's own
	// strongly-typed Input, so this package's JSON shape does not require
	// a consumer to know five different Input schemas just to render a
	// table.
	Assumptions []Assumption `json:"assumptions,omitempty"`
	// Steps echoes the method's own calculation Steps (valuation.Step),
	// when Included, for a report that wants the full calculation trace
	// inline rather than only the headline Value.
	Steps []valuation.Step `json:"steps,omitempty"`
	// Warnings lists every warning message from the method's own Result,
	// plus (when Outcome is "excluded") a synthesized note explaining the
	// exclusion.
	Warnings []string `json:"warnings,omitempty"`
}

// Assumption is one named input assumption behind a method's calculated
// value, e.g. {Label: "SDE Multiple", Value: "2.5x"}.
type Assumption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}
