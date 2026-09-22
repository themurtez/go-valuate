// Package earnings selects or derives a single "maintainable earnings"
// figure from a caller-supplied series of period/value observations —
// e.g. normalized EBITDA or normalized SDE values produced by
// financial/adjustments across several historical periods — for use as an
// input to future valuation formulas (SDE/EBITDA multiples, DCF terminal
// value, etc.).
//
// This package has no idea where its observations came from (a
// metrics.Snapshot, an adjustments.Bridge, a hand-entered figure) and does
// not compute EBITDA/SDE itself; it only knows how to combine an ordered
// series of (period, value) pairs into one defensible number, under a
// caller-selected strategy, with a fully explainable trail of what was
// included, excluded, and why. Every function here is pure: no I/O, no
// mutation of its inputs.
//
// Comparable periods only. This package never averages observations from
// incompatible period types (e.g. a full fiscal year against a
// partial-year YTD figure) — see Observation.PeriodType and
// Calculate's doc comment.
package earnings

import "github.com/themurtez/go-valuate/financial/metrics"

// PeriodType mirrors metrics.PeriodType (fiscal year, YTD, quarter, month)
// so this package can reason about comparability without importing
// metrics.PeriodInfo's full ordering machinery — earnings only needs to
// know whether two periods are the same granularity, not their relative
// chronological order beyond what the caller has already sorted.
type PeriodType = metrics.PeriodType

// Re-exported PeriodType constants, so callers building an Observation
// slice don't need to import financial/metrics solely for these.
const (
	PeriodTypeFiscalYear = metrics.PeriodTypeFiscalYear
	PeriodTypeYTD        = metrics.PeriodTypeYTD
	PeriodTypeQuarter    = metrics.PeriodTypeQuarter
	PeriodTypeMonth      = metrics.PeriodTypeMonth
)

// Observation is a single period's earnings figure (e.g. normalized
// EBITDA or normalized SDE for one fiscal year), supplied by the caller in
// chronological order (oldest first) — this package does not re-derive
// order from Period's string value, consistent with financial/metrics'
// explicit no-guessing rule for period order.
type Observation struct {
	// Period is a display label for this observation (e.g. "2025",
	// "2026-YTD"). Purely for explainability in Result; not parsed.
	Period string `json:"period"`
	// PeriodType is this observation's granularity, used to determine
	// comparability — see Calculate's doc comment on incomparable periods.
	PeriodType PeriodType `json:"period_type"`
	// Value is the earnings figure for this period (e.g. normalized
	// EBITDA). Ignored if Available is false.
	Value float64 `json:"value"`
	// Available is false if this period's earnings figure could not be
	// computed (mirrors metrics.MetricValue.Available) — Calculate excludes
	// such observations rather than treating a missing figure as 0.
	Available bool `json:"available"`
}
