package earnings

// Strategy identifies which method Calculate uses to derive maintainable
// earnings from a series of Observations. Strategy is a plain string
// (mirroring financial.Code/adjustments.Type) so new strategies can be
// added later without breaking existing callers.
type Strategy string

const (
	// StrategyLatestPeriod uses the single most recent comparable
	// observation. See calculateLatestPeriod.
	StrategyLatestPeriod Strategy = "latest_period"
	// StrategySimpleAverage is the unweighted arithmetic mean across every
	// comparable, available observation. See calculateSimpleAverage.
	StrategySimpleAverage Strategy = "simple_average"
	// StrategyWeightedAverage applies caller-supplied explicit weights per
	// period. See calculateWeightedAverage and Options.Weights.
	StrategyWeightedAverage Strategy = "weighted_average"
	// StrategyTrendAdjusted projects a deterministic linear trend across
	// comparable observations and uses the trend line's value at the final
	// period as maintainable earnings, rather than a flat average — see
	// calculateTrendAdjusted for the exact method and its documented
	// limitations.
	StrategyTrendAdjusted Strategy = "trend_adjusted"
)

// Options controls Calculate's behavior. Which fields are required depends
// on Strategy — see each field's doc comment.
type Options struct {
	// Strategy selects the calculation method. Required; Calculate returns
	// a Result with a populated Warnings/no Value if Strategy is empty or
	// unrecognized.
	Strategy Strategy
	// Weights supplies explicit per-period weights for
	// StrategyWeightedAverage, keyed by Observation.Period. Ignored by
	// every other strategy. See Options.validateWeights for the rules
	// weights must satisfy.
	Weights map[string]float64
	// ComparablePeriodType restricts Calculate to only the observations
	// whose PeriodType equals this value, excluding every other period type
	// as incomparable. If empty, Calculate infers the comparable set as
	// every observation sharing the PeriodType of the chronologically last
	// observation (see selectComparable) — the common case of "average the
	// full fiscal years, exclude the trailing YTD stub."
	ComparablePeriodType PeriodType
}
