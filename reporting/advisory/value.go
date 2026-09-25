package advisory

import "math"

// Value represents a single figure that may or may not be available,
// distinguishing "computed/reported to be exactly 0" from "unknown because
// a required input was absent or the underlying source metric was itself
// unavailable" — this package's own copy of the convention every sibling
// package in this repository duplicates locally rather than importing a
// shared type (see transactions/salereadiness.Value's doc comment for the
// full rationale, restated identically here).
type Value struct {
	Available bool    `json:"available"`
	Amount    float64 `json:"amount"`
}

// Unavailable is the canonical zero-information Value.
func Unavailable() Value { return Value{} }

// AvailableValue reports a Value for a known figure (which may
// legitimately be zero or negative).
func AvailableValue(v float64) Value { return Value{Available: true, Amount: v} }

// Change is a current-vs-prior comparison over a pair of already-computed
// [Value]s — the only arithmetic this package performs on a source figure
// (task section 9's "change between already-computed metrics" allowance).
// Every field is independently unavailable when its precondition is not
// met; see change's doc comment for exactly which precondition gates
// which field.
type Change struct {
	// Current and Prior are the two Values being compared, carried through
	// unchanged so a caller never has to look elsewhere to see what this
	// Change was computed from.
	Current Value `json:"current"`
	Prior   Value `json:"prior"`
	// AbsoluteChange is Current.Amount - Prior.Amount. Available only when
	// both Current and Prior are Available.
	AbsoluteChange Value `json:"absolute_change"`
	// PercentChange is (Current.Amount - Prior.Amount) / |Prior.Amount|, a
	// *relative* percent change. Available only when both Current and
	// Prior are Available and Prior.Amount is nonzero — task section 10's
	// explicit "do not fabricate percent changes from zero denominator"
	// instruction. Meaningful for any unit, but see PercentagePointChange
	// for why a percent-shaped metric usually wants that field instead —
	// both are always computed when preconditions allow; a caller/adapter
	// picks the one it means, mirroring analytics/kpi.Change's identical
	// "both always computed, never mixed automatically" convention.
	PercentChange Value `json:"percent_change"`
	// PercentagePointChange is Current.Amount - Prior.Amount, for metrics
	// whose Unit is already a percent (e.g. a margin, a DSO-as-percent-of-
	// terms figure never applies here, but a margin or a concentration
	// share does) — the correct comparison for "18% -> 27%" is "+9
	// percentage points," not the relative PercentChange's "+50%". An
	// adapter sets this only for percent-unit metrics; it is left
	// unavailable for currency/day/count/multiple-unit metrics, where
	// PercentagePointChange would not be a meaningful figure.
	PercentagePointChange Value `json:"percentage_point_change"`
}

// computeChange builds a Change from a current/prior Value pair.
// isPercentUnit controls whether PercentagePointChange is populated — see
// Change.PercentagePointChange's doc comment. Never mutates cur or prior.
func computeChange(cur, prior Value, isPercentUnit bool) Change {
	c := Change{Current: cur, Prior: prior}
	if !cur.Available || !prior.Available {
		return c
	}
	c.AbsoluteChange = safeValue(cur.Amount - prior.Amount)
	if prior.Amount != 0 {
		c.PercentChange = safeValue((cur.Amount - prior.Amount) / absFloat(prior.Amount))
	}
	if isPercentUnit {
		c.PercentagePointChange = safeValue(cur.Amount - prior.Amount)
	}
	return c
}

// safeValue returns AvailableValue(f), unless f is NaN or +/-Inf (e.g.
// from subtracting/dividing two extreme-magnitude float64 inputs), in
// which case it returns Unavailable() — this package's own copy of the
// repo-wide convention of never propagating a non-finite result as though
// it were a computed figure (see e.g. valuation/consensus.Statistics'
// documented zero-on-zero-denominator edge cases). computeChange is the
// only place in this package that performs unchecked float64 arithmetic
// on caller-supplied source figures, so this guard lives here rather than
// on every call site.
func safeValue(f float64) Value {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return Unavailable()
	}
	return AvailableValue(f)
}

func absFloat(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

// Unit is a stable identifier for how a Value should be displayed —
// purely descriptive; this package performs no unit conversion or
// formatting itself, mirroring reporting/management.Unit exactly (a
// separate copy, not a shared import, per this package's own-taxonomy
// convention — see IssueCode's doc comment).
type Unit string

const (
	UnitCurrency Unit = "currency"
	UnitPercent  Unit = "percent"
	UnitMultiple Unit = "multiple"
	UnitDays     Unit = "days"
	UnitMonths   Unit = "months"
	UnitCount    Unit = "count"
	UnitWeeks    Unit = "weeks"
	UnitRatio    Unit = "ratio"
)

// isPercentUnit reports whether u is a unit for which Change should
// populate PercentagePointChange rather than (or in addition to)
// PercentChange — see Change.PercentagePointChange's doc comment.
func isPercentUnit(u Unit) bool { return u == UnitPercent }

// Metric is one composed figure in a [Section]: a value plus its current/
// prior comparison, unit, period, and full provenance back to the sibling
// package/field it was read from — task section 47. Metric never
// recomputes its Value; every Metric is either read verbatim from a
// sibling Result (via an adapter_*.go file) or is a generic current/prior
// Change over two already-computed Values (task section 9/48).
type Metric struct {
	// Code is a stable, machine-readable identifier for this metric (e.g.
	// "ar_dso", "minimum_13_week_cash", "ebitda_margin"). Unique within one
	// Section's Metrics slice.
	Code string `json:"code"`
	// Label is a short, fixed, human-readable name (e.g. "AR Days Sales
	// Outstanding", "13-Week Minimum Cash").
	Label string `json:"label"`

	Value  Value  `json:"value"`
	Unit   Unit   `json:"unit"`
	Period string `json:"period,omitempty"`

	Prior  Value  `json:"prior"`
	Change Change `json:"change"`

	// SourceModule/SourceCode/SourceRefs identify exactly which sibling
	// package, field, and (where the source package itself carries one) a
	// caller-opaque SourceRef this Metric's Value was read from — task
	// section 47/87's "no untraceable headline number" requirement. Every
	// Metric populated by an adapter carries a non-empty SourceModule and
	// SourceCode; Metric never has an empty provenance pair except a
	// caller-supplied Metric explicitly marked as such by its own
	// adapter/caller.
	SourceModule string      `json:"source_module,omitempty"`
	SourceCode   string      `json:"source_code,omitempty"`
	SourceRefs   []SourceRef `json:"source_refs,omitempty"`
}

// newMetric builds a Metric from an already-available current Value, with
// no prior/change comparison — the common case for a point-in-time or
// single-period sibling figure.
func newMetric(code, label string, v Value, unit Unit, period, sourceModule, sourceCode string) Metric {
	return Metric{
		Code: code, Label: label,
		Value: v, Unit: unit, Period: period,
		SourceModule: sourceModule, SourceCode: sourceCode,
	}
}

// newMetricWithPrior builds a Metric with a current/prior comparison,
// computing Change per computeChange.
func newMetricWithPrior(code, label string, cur, prior Value, unit Unit, period, sourceModule, sourceCode string) Metric {
	m := newMetric(code, label, cur, unit, period, sourceModule, sourceCode)
	m.Prior = prior
	m.Change = computeChange(cur, prior, isPercentUnit(unit))
	return m
}
