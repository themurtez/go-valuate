package cashforecast

// FlagCode is a stable identifier for one kind of deterministic liquidity/
// business-result signal — as opposed to an Issue, which is an input/
// configuration/integrity problem (see issues.go). This package never
// derives a hidden risk score; every flag is a simple, documented
// threshold comparison the caller can fully see and override via
// FlagThresholds.
type FlagCode string

const (
	// FlagCashBelowMinimum means at least one week's EndingCash is
	// available and below MinimumCashPolicy's resolved threshold.
	FlagCashBelowMinimum FlagCode = "CASH_BELOW_MINIMUM"
	// FlagNegativeCash means at least one week's EndingCash is negative.
	FlagNegativeCash FlagCode = "NEGATIVE_CASH"
	// FlagMaterialFundingGap means Summary.MaximumFundingGap is available
	// and exceeds FlagThresholds.FundingGapMaterialAmount.
	FlagMaterialFundingGap FlagCode = "MATERIAL_FUNDING_GAP"
	// FlagChronicBelowMinimum means Summary.WeeksBelowMinimum reaches
	// FlagThresholds.WeeksBelowMinimumThreshold.
	FlagChronicBelowMinimum FlagCode = "CHRONIC_BELOW_MINIMUM"
	// FlagUnscheduledAR means UnscheduledAR's percentage of total open AR
	// exceeds FlagThresholds.UnscheduledPercentThreshold.
	FlagUnscheduledAR FlagCode = "UNSCHEDULED_AR"
	// FlagUnscheduledAP means UnscheduledAP's percentage of total open AP
	// exceeds FlagThresholds.UnscheduledPercentThreshold.
	FlagUnscheduledAP FlagCode = "UNSCHEDULED_AP"
	// FlagStaleARSource means ARSnapshotDate's age exceeds
	// StalenessPolicy.MaxARAgeDays.
	FlagStaleARSource FlagCode = "STALE_AR_SOURCE"
	// FlagStaleAPSource means APSnapshotDate's age exceeds
	// StalenessPolicy.MaxAPAgeDays.
	FlagStaleAPSource FlagCode = "STALE_AP_SOURCE"
	// FlagStaleCashSource means OpeningCash.AsOfDate's age exceeds
	// StalenessPolicy.MaxCashAgeDays.
	FlagStaleCashSource FlagCode = "STALE_CASH_SOURCE"
	// FlagFacilityInsufficientForGap means Summary.MaximumFundingGap is
	// available, positive, and exceeds total available CreditFacility
	// capacity.
	FlagFacilityInsufficientForGap FlagCode = "FACILITY_INSUFFICIENT_FOR_GAP"
)

// flagCodeOrder fixes FlagCode declaration order for deterministic Flags
// sorting.
var flagCodeOrder = []FlagCode{
	FlagCashBelowMinimum,
	FlagNegativeCash,
	FlagMaterialFundingGap,
	FlagChronicBelowMinimum,
	FlagUnscheduledAR,
	FlagUnscheduledAP,
	FlagStaleARSource,
	FlagStaleAPSource,
	FlagStaleCashSource,
	FlagFacilityInsufficientForGap,
}

func flagRank(c FlagCode) int {
	for i, fc := range flagCodeOrder {
		if fc == c {
			return i
		}
	}
	return len(flagCodeOrder)
}

// FlagSeverity mirrors debt.FlagSeverity/cashflow.FlagSeverity's role: a
// structured, matchable urgency signal, never inferred from Message text.
type FlagSeverity string

const (
	FlagSeverityInfo     FlagSeverity = "info"
	FlagSeverityWarning  FlagSeverity = "warning"
	FlagSeverityCritical FlagSeverity = "critical"
)

// Flag is one deterministic liquidity signal Calculate triggered.
type Flag struct {
	Code      FlagCode     `json:"code"`
	Severity  FlagSeverity `json:"severity"`
	Scenario  string       `json:"scenario,omitempty"`
	Week      int          `json:"week,omitempty"`
	Message   string       `json:"message"`
	Value     float64      `json:"value,omitempty"`
	Threshold float64      `json:"threshold,omitempty"`
}

// flagInputs bundles everything computeFlags needs.
type flagInputs struct {
	result     Result
	facilities []CreditFacility
	thresholds FlagThresholds
}

// computeFlags evaluates every FlagCode rule against the base scenario and
// every named scenario, returning triggered flags sorted by FlagCode
// declaration order, then Scenario, then Week.
func computeFlags(in flagInputs) []Flag {
	var flags []Flag
	t := in.thresholds

	scenarios := append([]ScenarioResult{in.result.BaseScenario}, in.result.Scenarios...)
	for _, sc := range scenarios {
		scenarioLabel := sc.Label
		if scenarioLabel == BaseScenarioLabel {
			scenarioLabel = "" // base scenario omits Scenario for a cleaner default read.
		}

		for _, w := range sc.Weekly {
			if w.Threshold.Available && w.Threshold.BelowMinimum {
				flags = append(flags, Flag{Code: FlagCashBelowMinimum, Severity: FlagSeverityWarning, Scenario: scenarioLabel,
					Week: w.WeekNumber, Value: w.EndingCash, Threshold: w.Threshold.MinimumCash,
					Message: "weekly ending cash is below the minimum cash threshold"})
			}
			if w.EndingCash < 0 {
				flags = append(flags, Flag{Code: FlagNegativeCash, Severity: FlagSeverityCritical, Scenario: scenarioLabel,
					Week: w.WeekNumber, Value: w.EndingCash,
					Message: "weekly ending cash is negative"})
			}
		}

		if sc.Summary.ThresholdAvailable {
			if sc.Summary.MaximumFundingGap > t.FundingGapMaterialAmount {
				flags = append(flags, Flag{Code: FlagMaterialFundingGap, Severity: FlagSeverityCritical, Scenario: scenarioLabel,
					Value: sc.Summary.MaximumFundingGap, Threshold: t.FundingGapMaterialAmount,
					Message: "maximum funding gap across the horizon is material"})
			}
			if sc.Summary.WeeksBelowMinimum >= t.WeeksBelowMinimumThreshold {
				flags = append(flags, Flag{Code: FlagChronicBelowMinimum, Severity: FlagSeverityWarning, Scenario: scenarioLabel,
					Value: float64(sc.Summary.WeeksBelowMinimum), Threshold: float64(t.WeeksBelowMinimumThreshold),
					Message: "cash is below minimum for a chronic number of weeks"})
			}
			if sc.Summary.MaximumFundingGap > 0 {
				capacity := totalFacilityCapacity(in.facilities)
				if sc.Summary.MaximumFundingGap > capacity {
					flags = append(flags, Flag{Code: FlagFacilityInsufficientForGap, Severity: FlagSeverityWarning, Scenario: scenarioLabel,
						Value: sc.Summary.MaximumFundingGap, Threshold: capacity,
						Message: "maximum funding gap exceeds total available credit facility capacity"})
				}
			}
		}
	}

	if pct := in.result.Coverage.AR.ScheduledPercent; pct.Available {
		unscheduledPct := 1 - pct.Value
		if unscheduledPct > t.UnscheduledPercentThreshold {
			flags = append(flags, Flag{Code: FlagUnscheduledAR, Severity: FlagSeverityWarning,
				Value: unscheduledPct, Threshold: t.UnscheduledPercentThreshold,
				Message: "a material percentage of open AR is not scheduled into the forecast"})
		}
	}
	if pct := in.result.Coverage.AP.ScheduledPercent; pct.Available {
		unscheduledPct := 1 - pct.Value
		if unscheduledPct > t.UnscheduledPercentThreshold {
			flags = append(flags, Flag{Code: FlagUnscheduledAP, Severity: FlagSeverityWarning,
				Value: unscheduledPct, Threshold: t.UnscheduledPercentThreshold,
				Message: "a material percentage of open AP is not scheduled into the forecast"})
		}
	}

	if in.result.Coverage.ARStaleness.Stale {
		flags = append(flags, Flag{Code: FlagStaleARSource, Severity: FlagSeverityWarning,
			Value: float64(in.result.Coverage.ARStaleness.AgeDays), Threshold: float64(in.result.Coverage.ARStaleness.ThresholdDays),
			Message: "AR source data is older than the configured staleness threshold"})
	}
	if in.result.Coverage.APStaleness.Stale {
		flags = append(flags, Flag{Code: FlagStaleAPSource, Severity: FlagSeverityWarning,
			Value: float64(in.result.Coverage.APStaleness.AgeDays), Threshold: float64(in.result.Coverage.APStaleness.ThresholdDays),
			Message: "AP source data is older than the configured staleness threshold"})
	}
	if in.result.Coverage.CashStaleness.Stale {
		flags = append(flags, Flag{Code: FlagStaleCashSource, Severity: FlagSeverityWarning,
			Value: float64(in.result.Coverage.CashStaleness.AgeDays), Threshold: float64(in.result.Coverage.CashStaleness.ThresholdDays),
			Message: "opening cash source data is older than the configured staleness threshold"})
	}

	sortFlags(flags)
	return flags
}
