package diagnostics

import "github.com/themurtez/go-valuate/financial"

// genericFlag is a source-agnostic view over any sibling package's Flag
// type — qoe.Flag, cashflow.Flag, revenuequality.Flag, concentration.Flag,
// and debt.Flag all share this exact {Code, Severity, Period, Message,
// Value, Threshold} shape (a deliberate repository-wide convention, not a
// coincidence this package invented), letting mapFlag translate any of
// them with one function instead of five near-identical copies. Each
// sibling's own Flag type is still a distinct Go type (no shared
// interface exists, matching this repository's "each package owns its
// types" convention), so each mine* function below builds a genericFlag
// from its source Flag by hand before calling mapFlag.
type genericFlag struct {
	Code      string
	Severity  Severity
	Period    financial.Period
	Message   string
	Value     float64
	Threshold float64
}

// flagMapping names, for one sibling Flag Code, the normalized FindingCode/
// Category/RecommendedAction this package reports it as. severity/isStrength
// are not part of the mapping: a Flag's own Severity is used as-is (mapped
// 1:1: qoe.FlagSeverityInfo/Warning/Critical etc. use the identical string
// values as this package's own Severity, so no translation table is
// needed), and every Flag-derived Finding is a Concern (an active Flag is
// never itself the mechanism this package uses to report a Strength — see
// mineStrengths for how positive signals are mined instead).
type flagMapping struct {
	Code              FindingCode
	Category          Category
	Title             string
	RecommendedAction string
	// ZeroThresholdMeaningful opts a Flag code out of thresholdValue's
	// default "Threshold == 0 means no threshold was supplied" heuristic.
	// Every Flag emitter in this repository sets Threshold from a
	// caller-configured Policy field that fires only when actually
	// crossed (never genuinely 0 in practice) except
	// qoe.FlagDecliningEBITDADespiteRevenueGrowth, which deliberately sets
	// Threshold: 0 as a real EBITDA-change-vs-breakeven comparison point
	// (analytics/qoe/flags.go) — set true only for that entry.
	ZeroThresholdMeaningful bool
}

// mapFlag builds a Finding from one genericFlag, when code is a recognized
// key in table. Returns (Finding{}, false) for an unrecognized code (never
// silently drops a real Flag without a caller-visible signal — see each
// mine* function's fallback handling) so callers can decide how to report
// the gap.
func mapFlag(f genericFlag, source SourceModule, table map[string]flagMapping) (Finding, bool) {
	m, ok := table[f.Code]
	if !ok {
		return Finding{}, false
	}
	comparison := thresholdValue(f.Threshold, m.ZeroThresholdMeaningful)
	return Finding{
		Code:              m.Code,
		Category:          m.Category,
		Severity:          Severity(f.Severity),
		Title:             m.Title,
		Evidence:          []string{f.Message},
		SourceModule:      source,
		SourceCode:        f.Code,
		Metric:            AvailableValue(f.Value),
		ComparisonLabel:   comparisonLabelForValue(comparison),
		Comparison:        comparison,
		Period:            f.Period,
		Explanation:       f.Message,
		RecommendedAction: m.RecommendedAction,
	}, true
}

// thresholdValue reports threshold as an AvailableValue, unless threshold
// is exactly 0 and zeroMeaningful is false — the default "Threshold == 0
// means no threshold was supplied" heuristic every sibling Flag emitter's
// own convention implies (a Flag whose rule has no threshold at all, e.g.
// debt.FlagNoDebtService, leaves Threshold at its float64 zero value),
// except the handful of codes flagMapping.ZeroThresholdMeaningful opts out
// of that heuristic because they deliberately compare against a real zero
// (e.g. qoe.FlagDecliningEBITDADespiteRevenueGrowth) — see
// flagMapping.ZeroThresholdMeaningful's doc comment.
func thresholdValue(threshold float64, zeroMeaningful bool) Value {
	if threshold == 0 && !zeroMeaningful {
		return Unavailable()
	}
	return AvailableValue(threshold)
}

// comparisonLabelForValue reports "THRESHOLD" only when comparison is
// itself Available, keeping ComparisonLabel empty alongside an Unavailable
// Comparison.
func comparisonLabelForValue(comparison Value) string {
	if !comparison.Available {
		return ""
	}
	return "THRESHOLD"
}

var qoeFlagTable = map[string]flagMapping{
	"LARGE_NORMALIZATION_BURDEN": {
		Code: FindingLargeNormalizationBurden, Category: CategoryEarningsQuality,
		Title:             "Large earnings normalization burden",
		RecommendedAction: "Review the size and documentation of proposed EBITDA add-backs relative to reported earnings.",
	},
	"DECLINING_EBITDA_DESPITE_REVENUE_GROWTH": {
		Code: FindingMarginPressure, Category: CategoryProfitability,
		Title:                   "EBITDA declining despite revenue growth",
		RecommendedAction:       "Investigate cost structure changes driving margin compression alongside top-line growth.",
		ZeroThresholdMeaningful: true,
	},
	"VOLATILE_EARNINGS": {
		Code: FindingEarningsVolatility, Category: CategoryProfitability,
		Title:             "Volatile historical earnings",
		RecommendedAction: "Review the sources of period-to-period earnings volatility.",
	},
	"INCONSISTENT_MARGINS": {
		Code: FindingMarginPressure, Category: CategoryProfitability,
		Title:             "Inconsistent margins across periods",
		RecommendedAction: "Review the drivers of margin inconsistency across the historical period set.",
	},
	"LARGE_OWNER_DISCRETIONARY_COMPONENT": {
		Code: FindingLargeOwnerDiscretionaryComponent, Category: CategoryEarningsQuality,
		Title:             "Large owner-discretionary earnings component",
		RecommendedAction: "Review the size and documentation of owner-discretionary add-backs.",
	},
	"REPEATED_ONE_TIME_ADJUSTMENTS": {
		Code: FindingRepeatedOneTimeAdjustments, Category: CategoryEarningsQuality,
		Title:             "Repeated \"one-time\" adjustments",
		RecommendedAction: "Review whether adjustments labeled one-time have recurred across multiple periods.",
	},
	"NON_OPERATING_INCOME_SUPPORTING_EARNINGS": {
		Code: FindingNonOperatingIncomeReliance, Category: CategoryEarningsQuality,
		Title:             "Non-operating income supporting reported earnings",
		RecommendedAction: "Review how much of reported earnings depends on non-operating income sources.",
	},
	"NEGATIVE_OR_NEAR_ZERO_MAINTAINABLE_EARNINGS": {
		Code: FindingNegativeMaintainableEarnings, Category: CategoryEarningsQuality,
		Title:             "Negative or near-zero maintainable earnings",
		RecommendedAction: "Review the normalization adjustments underlying maintainable earnings.",
	},
}

// mineQoE mines Input.QoE.Flags into Findings, plus a positive Finding
// when QoE.Score reports a strong result — see mineStrengths for the
// broader positive-signal convention this augments.
func mineQoE(in Input) []Finding {
	if !in.QoE.Available {
		return nil
	}
	var out []Finding
	for _, f := range in.QoE.Flags {
		gf := genericFlag{Code: string(f.Code), Severity: Severity(f.Severity), Period: f.Period, Message: f.Message, Value: f.Value, Threshold: f.Threshold}
		if finding, ok := mapFlag(gf, SourceQoE, qoeFlagTable); ok {
			out = append(out, finding)
		}
	}
	return out
}

var cashflowFlagTable = map[string]flagMapping{
	"WEAK_CASH_CONVERSION": {
		Code: FindingWeakCashConversion, Category: CategoryCashConversion,
		Title:             "Weak EBITDA-to-cash conversion",
		RecommendedAction: "Review the drivers of the gap between reported EBITDA and operating/free cash flow.",
	},
	"HIGH_CAPEX_BURDEN": {
		Code: FindingHighCapexBurden, Category: CategoryCashConversion,
		Title:             "High capital expenditure burden",
		RecommendedAction: "Review capital expenditure levels relative to cash flow generation.",
	},
	"HIGH_WORKING_CAPITAL_BURDEN": {
		Code: FindingHighWorkingCapitalBurden, Category: CategoryWorkingCapital,
		Title:             "High working-capital burden on cash flow",
		RecommendedAction: "Review working-capital changes' effect on cash generation.",
	},
	"LOW_DEBT_SERVICE_COVERAGE": {
		Code: FindingWeakInterestCoverage, Category: CategoryLeverage,
		Title:             "Low debt-service coverage from cash flow",
		RecommendedAction: "Review debt-service coverage computed from actual cash flow rather than EBITDA alone.",
	},
	"DISTRIBUTIONS_EXCEED_FREE_CASH_FLOW": {
		Code: FindingDistributionsExceedFCF, Category: CategoryCashConversion,
		Title:             "Distributions exceed free cash flow",
		RecommendedAction: "Review the sustainability of distributions relative to free cash flow generation.",
	},
	"LOW_CASH_RUNWAY": {
		Code: FindingLowCashRunway, Category: CategoryCashConversion,
		Title:             "Low cash runway",
		RecommendedAction: "Review the cash burn rate and remaining runway.",
	},
	"DECLINING_CONVERSION_TREND": {
		Code: FindingDecliningCashConversion, Category: CategoryCashConversion,
		Title:             "Declining cash-conversion trend",
		RecommendedAction: "Review the trend in EBITDA-to-cash-flow conversion across recent periods.",
	},
}

// mineCashFlow mines Input.CashFlow.Flags into Findings.
func mineCashFlow(in Input) []Finding {
	if !in.CashFlow.Available {
		return nil
	}
	var out []Finding
	for _, f := range in.CashFlow.Flags {
		gf := genericFlag{Code: string(f.Code), Severity: Severity(f.Severity), Period: f.Period, Message: f.Message, Value: f.Value, Threshold: f.Threshold}
		if finding, ok := mapFlag(gf, SourceCashFlow, cashflowFlagTable); ok {
			out = append(out, finding)
		}
	}
	return out
}

var revenueQualityFlagTable = map[string]flagMapping{
	"DECLINING_RECURRING_MIX": {
		Code: FindingDecliningRecurringMix, Category: CategoryRevenueQuality,
		Title:             "Declining recurring-revenue mix",
		RecommendedAction: "Review the shift away from recurring/contractual revenue.",
	},
	"GROWTH_DEPENDENT_ON_NEW_CUSTOMERS": {
		Code: FindingGrowthDependency, Category: CategoryGrowth,
		Title:             "Growth dependent on new customer acquisition",
		RecommendedAction: "Review how much of revenue growth depends on new-customer acquisition versus the existing base.",
	},
	"HIGH_LOST_CUSTOMER_REVENUE": {
		Code: FindingHighLostCustomerRevenue, Category: CategoryRevenueQuality,
		Title:             "High lost-customer revenue",
		RecommendedAction: "Review the causes of customer attrition and the resulting revenue loss.",
	},
	"VOLATILE_REVENUE": {
		Code: FindingRevenueVolatility, Category: CategoryGrowth,
		Title:             "Volatile historical revenue",
		RecommendedAction: "Review the sources of period-to-period revenue volatility.",
	},
	"ONE_PERIOD_SPIKE": {
		Code: FindingOnePeriodRevenueSpike, Category: CategoryGrowth,
		Title:             "One-period revenue spike",
		RecommendedAction: "Review whether a single-period revenue spike reflects a recurring or non-recurring event.",
	},
	"SHRINKING_EXISTING_CUSTOMER_BASE": {
		Code: FindingShrinkingExistingCustomers, Category: CategoryRevenueQuality,
		Title:             "Shrinking existing customer base",
		RecommendedAction: "Review revenue trends within the existing customer base, independent of new-customer additions.",
	},
}

// mineRevenueQuality mines Input.RevenueQuality.Flags into Findings.
func mineRevenueQuality(in Input) []Finding {
	if !in.RevenueQuality.Available {
		return nil
	}
	var out []Finding
	for _, f := range in.RevenueQuality.Flags {
		gf := genericFlag{Code: string(f.Code), Severity: Severity(f.Severity), Period: f.Period, Message: f.Message, Value: f.Value, Threshold: f.Threshold}
		if finding, ok := mapFlag(gf, SourceRevenueQuality, revenueQualityFlagTable); ok {
			out = append(out, finding)
		}
	}
	return out
}

var concentrationFlagTable = map[string]flagMapping{
	"HIGH_LARGEST_ENTITY_CONCENTRATION": {
		Code: FindingHighCustomerConcentration, Category: CategoryConcentration,
		Title:             "High largest-customer concentration",
		RecommendedAction: "Review dependence on the single largest customer/counterparty.",
	},
	"HIGH_TOP_5_CONCENTRATION": {
		Code: FindingHighTop5Concentration, Category: CategoryConcentration,
		Title:             "High top-5 customer concentration",
		RecommendedAction: "Review dependence on the top five customers/counterparties combined.",
	},
	"HIGH_HHI": {
		Code: FindingHighCustomerConcentration, Category: CategoryConcentration,
		Title:             "High revenue concentration (HHI)",
		RecommendedAction: "Review overall customer/counterparty concentration as measured by the Herfindahl-Hirschman Index.",
	},
	"INCREASING_CONCENTRATION": {
		Code: FindingIncreasingConcentration, Category: CategoryConcentration,
		Title:             "Increasing customer concentration",
		RecommendedAction: "Review the trend toward greater customer/counterparty concentration.",
	},
	"HIGH_SCENARIO_IMPACT": {
		Code: FindingHighConcentrationScenarioImpact, Category: CategoryConcentration,
		Title:             "High revenue impact from a customer-loss scenario",
		RecommendedAction: "Review the modeled revenue impact of losing a top customer/counterparty.",
	},
}

// mineConcentration mines Input.Concentration.Flags into Findings.
func mineConcentration(in Input) []Finding {
	if !in.Concentration.Available {
		return nil
	}
	var out []Finding
	for _, f := range in.Concentration.Flags {
		gf := genericFlag{Code: string(f.Code), Severity: Severity(f.Severity), Period: f.Period, Message: f.Message, Value: f.Value, Threshold: f.Threshold}
		if finding, ok := mapFlag(gf, SourceConcentration, concentrationFlagTable); ok {
			out = append(out, finding)
		}
	}
	return out
}

var debtFlagTable = map[string]flagMapping{
	"BELOW_MINIMUM_DSCR": {
		Code: FindingBelowMinimumDSCR, Category: CategoryLeverage,
		Title:             "Debt-service coverage below policy minimum",
		RecommendedAction: "Review debt-service coverage against the applicable minimum DSCR requirement.",
	},
	"ABOVE_LEVERAGE_CAP": {
		Code: FindingAboveLeverageCap, Category: CategoryLeverage,
		Title:             "Leverage above policy cap",
		RecommendedAction: "Review total/net debt relative to EBITDA against the applicable leverage cap.",
	},
	"BELOW_MINIMUM_FIXED_CHARGE_COVERAGE": {
		Code: FindingWeakInterestCoverage, Category: CategoryLeverage,
		Title:             "Fixed-charge coverage below policy minimum",
		RecommendedAction: "Review fixed-charge coverage (debt service plus leases/capex/taxes) against the applicable minimum.",
	},
	"NEGATIVE_HEADROOM": {
		Code: FindingNegativeDebtHeadroom, Category: CategoryLeverage,
		Title:             "Negative debt capacity headroom",
		RecommendedAction: "Review current debt levels against the modeled maximum debt capacity.",
	},
	"SCENARIO_BREACHES_DSCR": {
		Code: FindingBelowMinimumDSCR, Category: CategoryLeverage,
		Title:             "Downside scenario breaches minimum DSCR",
		RecommendedAction: "Review debt-service coverage under the modeled downside scenario(s).",
	},
	"NO_DEBT_SERVICE": {
		Code: FindingNoDebtService, Category: CategoryLeverage,
		Title:             "No debt service to evaluate",
		RecommendedAction: "Confirm whether debt data was omitted or the business genuinely carries no debt.",
	},
}

// mineDebt mines Input.Debt.Flags into Findings. FlagNoDebtService is
// intentionally not filed as a Concern's typical negative-signal severity
// override — it inherits whatever Severity debt.Calculate itself assigned
// (see debt.Flag's doc comment), since "no debt" is informational rather
// than automatically concerning.
func mineDebt(in Input) []Finding {
	if !in.Debt.Available {
		return nil
	}
	var out []Finding
	for _, f := range in.Debt.Flags {
		gf := genericFlag{Code: string(f.Code), Severity: Severity(f.Severity), Period: "", Message: f.Message, Value: f.Value, Threshold: f.Threshold}
		if finding, ok := mapFlag(gf, SourceDebt, debtFlagTable); ok {
			out = append(out, finding)
		}
	}
	return out
}
