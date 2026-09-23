package ratios

// Thresholds configures every deterministic health-signal trigger point
// this package evaluates. Every field has a conservative, documented
// default — see DefaultThresholds — matching the
// qoe.Thresholds/review.Policy/workingcapital's identical pattern: a caller
// passing the zero Thresholds gets sane defaults rather than every
// threshold effectively disabled/zeroed.
type Thresholds struct {
	// LiquidityDeclineThreshold is the absolute decline in CurrentRatio
	// (raw ratio units, e.g. 0.20 = a drop from 1.50 to 1.30) between two
	// chronologically adjacent periods at or beyond which
	// SignalWeakeningLiquidity triggers. Defaults to 0.20.
	LiquidityDeclineThreshold float64 `json:"liquidity_decline_threshold"`
	// LeverageIncreaseThreshold is the absolute increase in DebtToEBITDA
	// (raw ratio units, e.g. 0.50 = a rise from 2.0x to 2.5x) between two
	// chronologically adjacent periods at or beyond which
	// SignalRisingLeverage triggers. Defaults to 0.50.
	LeverageIncreaseThreshold float64 `json:"leverage_increase_threshold"`
	// MarginCompressionThreshold is the absolute decline in EBITDAMargin
	// (raw decimal, e.g. 0.03 = 3 percentage points) between two
	// chronologically adjacent periods at or beyond which
	// SignalMarginCompression triggers. Defaults to 0.03.
	MarginCompressionThreshold float64 `json:"margin_compression_threshold"`
	// CollectionsSlowdownDays is the increase in DaysSalesOutstanding (in
	// days) between two chronologically adjacent periods at or beyond which
	// SignalSlowingCollections triggers. Defaults to 10.
	CollectionsSlowdownDays float64 `json:"collections_slowdown_days"`
	// InventoryBuildupDays is the increase in DaysInventoryOutstanding (in
	// days) between two chronologically adjacent periods at or beyond which
	// SignalInventoryBuildup triggers. Defaults to 10.
	InventoryBuildupDays float64 `json:"inventory_buildup_days"`
	// WeakInterestCoverageRatio is the InterestCoverage level (raw ratio,
	// e.g. 1.5 = EBIT covers interest expense 1.5x) at or below which
	// SignalWeakInterestCoverage triggers for the most recent period.
	// Defaults to 1.5 — the common minimum-covenant-style threshold below
	// which debt service capacity is considered strained.
	WeakInterestCoverageRatio float64 `json:"weak_interest_coverage_ratio"`
	// ProfitabilityChangeThreshold is the absolute change in EBITDAMargin
	// (raw decimal, e.g. 0.02 = 2 percentage points) between two
	// chronologically adjacent periods at or beyond which
	// SignalImprovingProfitability/SignalDeterioratingProfitability
	// trigger, checked against the signed change (positive triggers
	// improving, negative triggers deteriorating). Defaults to 0.02 — a
	// smaller, more sensitive threshold than MarginCompressionThreshold
	// since these two signals ask different questions: compression flags a
	// specific concerning decline, improving/deteriorating flags any
	// directional move worth noting.
	ProfitabilityChangeThreshold float64 `json:"profitability_change_threshold"`
}

// DefaultThresholds returns this package's conservative default
// Thresholds, applied whenever a caller passes a zero-value Thresholds to
// Calculate (see resolveThresholds). Mirrors
// qoe.DefaultThresholds/review.DefaultPolicy's identical role.
func DefaultThresholds() Thresholds {
	return Thresholds{
		LiquidityDeclineThreshold:    0.20,
		LeverageIncreaseThreshold:    0.50,
		MarginCompressionThreshold:   0.03,
		CollectionsSlowdownDays:      10,
		InventoryBuildupDays:         10,
		WeakInterestCoverageRatio:    1.5,
		ProfitabilityChangeThreshold: 0.02,
	}
}

// resolveThresholds returns t if any field differs from the zero value,
// otherwise DefaultThresholds() — the same zero-value-means-defaults rule
// qoe.resolveThresholds/review.resolvePolicy use.
func resolveThresholds(t Thresholds) Thresholds {
	if t == (Thresholds{}) {
		return DefaultThresholds()
	}
	return t
}
