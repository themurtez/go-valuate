package kpi

// AggregationRule declares how one source metric may be rolled up across
// dimensions or periods when a KPI expression asks for that — task
// section 17. This package never defaults every metric to SUM (task
// section 17's explicit instruction); a MetricValue with an unrecognized
// or empty Aggregation is treated as NotAggregatable (see
// resolvedAggregation), the conservative default that blocks silent
// rollup of a figure whose rollup semantics were never declared.
type AggregationRule string

const (
	AggregationSum             AggregationRule = "SUM"
	AggregationAverage         AggregationRule = "AVERAGE"
	AggregationWeightedAverage AggregationRule = "WEIGHTED_AVERAGE"
	AggregationLast            AggregationRule = "LAST"
	AggregationMin             AggregationRule = "MIN"
	AggregationMax             AggregationRule = "MAX"
	// AggregationNotAggregatable marks a metric (typically a ratio or
	// percentage, e.g. a margin %) that must never be summed/averaged
	// directly across dimensions or periods — task section 17's "Margin %
	// -> NOT_AGGREGATABLE unless explicitly defined" example and "prefer
	// recomputing ratios from underlying totals rather than averaging
	// percentages" guidance. A KPI wanting a rolled-up margin composes it
	// from the underlying SUM-aggregatable numerator/denominator metrics
	// instead (e.g. DIVIDE(SUM(profit), SUM(revenue)), not
	// AVERAGE(margin_pct)).
	AggregationNotAggregatable AggregationRule = "NOT_AGGREGATABLE"
)

func isRecognizedAggregation(a AggregationRule) bool {
	switch a {
	case AggregationSum, AggregationAverage, AggregationWeightedAverage,
		AggregationLast, AggregationMin, AggregationMax, AggregationNotAggregatable:
		return true
	default:
		return false
	}
}

// resolvedAggregation returns a if recognized and non-empty, otherwise
// AggregationNotAggregatable — the safe, non-silent-SUM default (task
// section 17).
func resolvedAggregation(a AggregationRule) AggregationRule {
	if isRecognizedAggregation(a) && a != "" {
		return a
	}
	return AggregationNotAggregatable
}

// MetricValue is one source fact this engine can reference from a KPI
// Expression's METRIC operator — task section 3. A caller assembles a
// slice of these from an existing module's Result, an app-level custom
// fact table, or anywhere else; this package never fetches or infers
// metric data itself (see doc.go's "no external metric fetching"
// non-goal).
//
// Known zero and unavailable are always kept distinct: a MetricValue with
// Available == true and Value == 0 means "this figure was computed and is
// exactly zero" (e.g. $0 of marketing spend this period); a MetricValue
// that was never supplied at all, or supplied with Available == false,
// means "this figure could not be computed" — task section 3's "known
// zero must remain distinct from unavailable" instruction, and the same
// distinction task section 35 locks with explicit tests.
type MetricValue struct {
	// Code identifies this metric (e.g. "financial.revenue", "ar.dso",
	// "labor.fte"). See "Adapter metric namespaces" in doc.go/
	// docs/KPI_ENGINE.md — this package does not enforce any particular
	// namespace convention; Code is an opaque caller-chosen string
	// compared for exact equality only.
	Code string `json:"code"`
	// Period matches a Period.Code in Input.Periods. Required.
	Period string `json:"period"`
	// Dimensions is this MetricValue's exact dimension scope — the zero
	// value means business-level (task section 5). Compared via
	// DimensionKey.Equal (canonical, order-independent).
	Dimensions DimensionKey `json:"dimensions,omitempty"`
	// Value is this metric's figure, meaningful only when Available is
	// true.
	Value float64 `json:"value"`
	// Available distinguishes a successfully computed (possibly zero)
	// figure from one that could not be computed — see the type doc
	// comment's "known zero must remain distinct from unavailable" rule.
	Available bool `json:"available"`
	// Unit is this metric's unit of measure — required (Unit.Valid())
	// for any MetricValue that will participate in an arithmetic
	// Expression; see unit.go.
	Unit Unit `json:"unit"`
	// Aggregation declares this metric's rollup behavior across
	// dimensions/periods — see AggregationRule. Left empty/unrecognized,
	// it resolves to AggregationNotAggregatable (never silently SUM).
	Aggregation AggregationRule `json:"aggregation,omitempty"`
	// Source is a caller-chosen label for where this fact came from (e.g.
	// "accounting/ar", "custom"), carried through for provenance/trace
	// only — never interpreted.
	Source string `json:"source,omitempty"`
	// SourceRef is an opaque caller-defined pointer back to the
	// originating record, mirroring accounting/vendorspend.SourceRef's
	// identical "opaque, never interpreted" convention. Carried through
	// for provenance/trace only.
	SourceRef string `json:"source_ref,omitempty"`
}

// sourceKey is the exact-match identity (metric code + period + canonical
// dimension key) task section 3 requires duplicate detection over — "do
// not pick first/last arbitrarily."
func (m MetricValue) sourceKey() string {
	return m.Code + "\x00" + m.Period + "\x00" + m.Dimensions.hashKey()
}
