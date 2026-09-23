package cashforecast

// MinimumCashPolicy configures the caller's minimum-cash threshold used
// for headroom/funding-gap analysis. This package never invents a
// default minimum — when neither MinimumCashBalance nor any
// CashAccount.MinimumReserve is supplied, threshold-dependent fields
// (Headroom, FundingGap, FirstWeekBelowMinimum, MaximumFundingGap,
// RequiredFunding) are left unavailable while weekly cash balances remain
// fully available — see the task's section 5.
type MinimumCashPolicy struct {
	// MinimumCashBalance is the caller's flat minimum-cash threshold. If
	// zero and Options.CashAccounts carries at least one nonzero
	// MinimumReserve, the sum of those reserves is used instead — see
	// resolvedMinimumCashPolicy. A caller who genuinely wants a zero
	// minimum sets MinimumCashBalanceExplicitZero.
	MinimumCashBalance float64 `json:"minimum_cash_balance,omitempty"`
	// MinimumCashBalanceExplicitZero, when true, means the caller
	// explicitly wants a $0 minimum-cash threshold (distinguishing that
	// from "no threshold supplied at all") — otherwise a zero
	// MinimumCashBalance with no account reserves means the threshold is
	// unavailable, not zero.
	MinimumCashBalanceExplicitZero bool `json:"minimum_cash_balance_explicit_zero,omitempty"`
}

// resolvedMinimumCashThreshold returns the threshold to use and whether
// one is available at all, applying MinimumCashPolicy over the account
// reserve fallback.
func resolvedMinimumCashThreshold(policy MinimumCashPolicy, accountReserveTotal float64) (float64, bool) {
	if policy.MinimumCashBalance != 0 {
		return policy.MinimumCashBalance, true
	}
	if policy.MinimumCashBalanceExplicitZero {
		return 0, true
	}
	if accountReserveTotal != 0 {
		return accountReserveTotal, true
	}
	return 0, false
}

// RequiredInputs lets a caller mark which upstream cash-flow categories
// must be present for Coverage to consider the forecast adequately
// sourced. This package never automatically makes every category
// required — a missing category is only reported via
// IssueMissingRequiredInput when the caller explicitly flagged it as
// required here.
type RequiredInputs struct {
	AR          bool `json:"ar,omitempty"`
	AP          bool `json:"ap,omitempty"`
	Payroll     bool `json:"payroll,omitempty"`
	DebtService bool `json:"debt_service,omitempty"`
	Tax         bool `json:"tax,omitempty"`
}

// StalenessPolicy configures how old source-as-of dates may be before
// Coverage reports them stale. This package never infers a staleness
// threshold — a source-as-of date is reported alongside its raw age only,
// unless the caller supplies the corresponding Max*AgeDays field.
type StalenessPolicy struct {
	MaxARAgeDays   int `json:"max_ar_age_days,omitempty"`
	MaxAPAgeDays   int `json:"max_ap_age_days,omitempty"`
	MaxCashAgeDays int `json:"max_cash_age_days,omitempty"`
}

// FlagThresholds configures every Flag's trigger point. All
// caller-adjustable; the zero value resolves to DefaultFlagThresholds.
type FlagThresholds struct {
	// FundingGapMaterialAmount triggers FlagMaterialFundingGap when
	// MaximumFundingGap exceeds this absolute dollar amount. Zero resolves
	// to defaultFundingGapMaterialAmount (0, meaning any positive gap is
	// material) unless overridden.
	FundingGapMaterialAmount float64 `json:"funding_gap_material_amount,omitempty"`
	// UnscheduledPercentThreshold triggers FlagUnscheduledAR/
	// FlagUnscheduledAP when the corresponding unscheduled percentage
	// exceeds this fraction. Default 0.20 (20%).
	UnscheduledPercentThreshold float64 `json:"unscheduled_percent_threshold,omitempty"`
	// SourceStaleDays is the fallback staleness trigger used only when the
	// corresponding StalenessPolicy.Max*AgeDays is unset for that source
	// — see resolveFlagThresholds. Default 0 (disabled; staleness flags
	// require an explicit policy).
	SourceStaleDays int `json:"source_stale_days,omitempty"`
	// WeeksBelowMinimumThreshold triggers FlagChronicBelowMinimum when
	// WeeksBelowMinimum reaches this count. Default 4.
	WeeksBelowMinimumThreshold int `json:"weeks_below_minimum_threshold,omitempty"`
}

const (
	defaultUnscheduledPercentThreshold = 0.20
	defaultWeeksBelowMinimumThreshold  = 4
)

// DefaultFlagThresholds returns this package's baseline flag trigger
// points.
func DefaultFlagThresholds() FlagThresholds {
	return FlagThresholds{
		UnscheduledPercentThreshold: defaultUnscheduledPercentThreshold,
		WeeksBelowMinimumThreshold:  defaultWeeksBelowMinimumThreshold,
	}
}

func resolveFlagThresholds(t FlagThresholds) FlagThresholds {
	d := DefaultFlagThresholds()
	if t.UnscheduledPercentThreshold == 0 {
		t.UnscheduledPercentThreshold = d.UnscheduledPercentThreshold
	}
	if t.WeeksBelowMinimumThreshold == 0 {
		t.WeeksBelowMinimumThreshold = d.WeeksBelowMinimumThreshold
	}
	return t
}
