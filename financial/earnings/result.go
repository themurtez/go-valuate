package earnings

// ExclusionReason explains why an Observation was left out of the
// calculation.
type ExclusionReason string

const (
	// ExclusionUnavailable means Observation.Available was false.
	ExclusionUnavailable ExclusionReason = "unavailable"
	// ExclusionIncomparablePeriodType means the observation's PeriodType
	// did not match the comparable set Calculate selected (see
	// selectComparable / Options.ComparablePeriodType) — e.g. a YTD period
	// excluded from an average of full fiscal years.
	ExclusionIncomparablePeriodType ExclusionReason = "incomparable_period_type"
	// ExclusionNoWeight means StrategyWeightedAverage was used and this
	// period had no entry in Options.Weights.
	ExclusionNoWeight ExclusionReason = "no_weight"
	// ExclusionNotLatest means StrategyLatestPeriod was used and this
	// observation was not the chronologically last comparable one.
	ExclusionNotLatest ExclusionReason = "not_latest"
)

// ExcludedObservation is one Observation that did not participate in the
// final calculation, plus why.
type ExcludedObservation struct {
	Observation Observation     `json:"observation"`
	Reason      ExclusionReason `json:"reason"`
	Detail      string          `json:"detail,omitempty"`
}

// WeightUsed records the resolved weight applied to one included
// observation under StrategyWeightedAverage, both as originally supplied
// and after normalization (if any) — see Options.validateWeights.
type WeightUsed struct {
	Period           string  `json:"period"`
	SuppliedWeight   float64 `json:"supplied_weight"`
	NormalizedWeight float64 `json:"normalized_weight"`
}

// Result is the output of Calculate: a fully explainable trail from the
// input Observations to a single maintainable-earnings figure, suitable
// for direct display in a valuation report.
type Result struct {
	// Strategy is the strategy that was used (echoed from Options).
	Strategy Strategy `json:"strategy"`
	// IncludedPeriods lists, in the order supplied, every Observation that
	// participated in the calculation.
	IncludedPeriods []Observation `json:"included_periods,omitempty"`
	// ExcludedPeriods lists every Observation that did not participate,
	// with its reason.
	ExcludedPeriods []ExcludedObservation `json:"excluded_periods,omitempty"`
	// RawValues is the raw (pre-weighting) value for each included
	// observation, in the same order as IncludedPeriods — a convenience
	// view for a report table, mirroring IncludedPeriods' values without
	// requiring the reader to dig into each Observation.
	RawValues []float64 `json:"raw_values,omitempty"`
	// Weights lists the resolved weight for each included observation,
	// populated only for StrategyWeightedAverage.
	Weights []WeightUsed `json:"weights,omitempty"`
	// Available is false if maintainable earnings could not be computed at
	// all (no comparable observations, invalid weights, unrecognized
	// strategy, etc.) — mirrors metrics.MetricValue's availability
	// convention. Value is always 0 when Available is false.
	Available bool `json:"available"`
	// Value is the final maintainable earnings figure. Meaningful only
	// when Available is true.
	Value float64 `json:"value"`
	// Warnings carries non-fatal notes (e.g. "only one comparable period
	// available; average is not very informative").
	Warnings []string `json:"warnings,omitempty"`
	// Errors carries the reason(s) Available is false, when it is.
	Errors []string `json:"errors,omitempty"`
}
