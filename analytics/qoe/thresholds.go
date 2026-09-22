package qoe

// Thresholds configures every deterministic quality-flag trigger point this
// package evaluates. Every field has a conservative, documented default —
// see DefaultThresholds — matching the
// review.Policy/DefaultPolicy/ingestion.Limits/DefaultLimits pattern used
// throughout this repository: a caller passing the zero Thresholds gets
// sane defaults rather than every threshold effectively disabled/zeroed.
type Thresholds struct {
	// LargeNormalizationBurdenRatio is the |adjustment total| / |reported
	// base| ratio at or above which FlagLargeNormalizationBurden triggers,
	// checked independently against Ratios.AdjustmentToEBITDA and
	// Ratios.AdjustmentToSDE (either crossing it is enough). Expressed as a
	// decimal (0.30 = 30%). Defaults to 0.30.
	LargeNormalizationBurdenRatio float64
	// VolatileEarningsRatio is the volatility statistic (sample standard
	// deviation of year-over-year growth — see
	// metrics.VolatilityResult/calculateVolatility) at or above which
	// FlagVolatileEarnings triggers, checked independently against
	// EBITDAVolatility and SDEVolatility. Expressed as a decimal (0.35 =
	// 35% swing). Defaults to 0.35.
	VolatileEarningsRatio float64
	// InconsistentMarginSwing is the absolute range (max - min, in raw
	// margin decimal terms, e.g. 0.15 = 15 percentage points) across
	// EBITDAMarginTrend's fiscal-year margin values at or above which
	// FlagInconsistentMargins triggers. Defaults to 0.15.
	InconsistentMarginSwing float64
	// OwnerDiscretionaryShareOfSDE is the owner-related SDE-bridge
	// adjustment total (see ownerDiscretionaryTypes and
	// largeOwnerDiscretionaryComponentFlag in flags.go) as a fraction of
	// normalized SDE, for the most recent period, at or above which
	// FlagLargeOwnerDiscretionaryComponent triggers. Expressed as a
	// decimal (0.25 = 25%). Defaults to 0.25.
	OwnerDiscretionaryShareOfSDE float64
	// RepeatedOneTimeMinPeriods is the minimum number of distinct periods
	// (RecurrencePattern.Count) a nominally non-recurring adjustments.Type
	// must appear in, with at least one applied/confirmed line, before
	// RecurrencePattern.LikelyNotNonRecurring is set and
	// FlagRepeatedOneTimeAdjustments triggers. Defaults to 2 — the same
	// type recurring in 2 or more separate periods is the deterministic
	// signal this package uses for "may not be truly non-recurring," per
	// the task's explicit requirement.
	RepeatedOneTimeMinPeriods int
	// NonOperatingIncomeShareOfEBITDA is the non-operating-income-removal
	// adjustment total (TypeNonOperatingIncome + TypeUnusualGain) as a
	// fraction of reported EBITDA, for the most recent period, at or above
	// which FlagNonOperatingIncomeSupportingEarnings triggers. Expressed
	// as a decimal (0.20 = 20%). Defaults to 0.20.
	NonOperatingIncomeShareOfEBITDA float64
	// NearZeroMaintainableEarnings is an absolute dollar floor (in the
	// dataset's currency): MaintainableEBITDA.Value/MaintainableSDE.Value
	// at or below this value triggers
	// FlagNegativeOrNearZeroMaintainableEarnings regardless of scale.
	// Defaults to 0, so a caller supplying no threshold at least always
	// catches literal zero-or-negative maintainable earnings even when
	// NearZeroMaintainableEarningsPercentOfRevenue cannot be evaluated
	// (see that field). Combined with the percent-of-revenue leg via OR —
	// either crossing triggers the flag, mirroring
	// review.Policy.MaterialAmountThreshold/MaterialPercentOfRevenue's
	// identical two-leg materiality test (see review.IsMaterial).
	NearZeroMaintainableEarnings float64
	// NearZeroMaintainableEarningsPercentOfRevenue is a fraction of the
	// most recent period's reported total revenue (e.g. 0.02 = 2%) at or
	// below which maintainable earnings is considered "near zero,"
	// evaluated only when that period's TotalRevenue is
	// Available — "near zero" is inherently scale-dependent (a $5,000
	// maintainable EBITDA is unremarkable for a $50,000-revenue business
	// and alarming for a $50M one), and revenue is the one scale reference
	// this package's History already carries on every Snapshot without
	// requiring a caller to supply a separate figure. Defaults to 0.02
	// (2% of revenue).
	NearZeroMaintainableEarningsPercentOfRevenue float64
}

// DefaultThresholds returns the conservative default Thresholds every field
// documented above uses, applied whenever a caller passes a zero-value
// Thresholds to Calculate (see resolveThresholds). Mirrors
// review.DefaultPolicy's role for review.Policy.
func DefaultThresholds() Thresholds {
	return Thresholds{
		LargeNormalizationBurdenRatio:                0.30,
		VolatileEarningsRatio:                        0.35,
		InconsistentMarginSwing:                      0.15,
		OwnerDiscretionaryShareOfSDE:                 0.25,
		RepeatedOneTimeMinPeriods:                    2,
		NonOperatingIncomeShareOfEBITDA:              0.20,
		NearZeroMaintainableEarnings:                 0,
		NearZeroMaintainableEarningsPercentOfRevenue: 0.02,
	}
}

// resolveThresholds returns t if any field differs from the zero value,
// otherwise DefaultThresholds() — the same zero-value-means-defaults rule
// review.resolvePolicy uses for review.Policy.
func resolveThresholds(t Thresholds) Thresholds {
	if t == (Thresholds{}) {
		return DefaultThresholds()
	}
	return t
}
