package diagnostics

// anomalyRuleMapping names, for one anomalies.RuleCode, the normalized
// FindingCode/Category/Title/RecommendedAction this package reports it as.
// Every anomaly is mined as a Concern (RuleCode has no positive-signal
// variant, unlike ratios.SignalCode) at the Anomaly's own Severity.
type anomalyRuleMapping struct {
	Code              FindingCode
	Category          Category
	Title             string
	RecommendedAction string
}

var anomalyRuleTable = map[string]anomalyRuleMapping{
	"ABSOLUTE_AMOUNT_SPIKE": {
		Code: FindingExpenseSpike, Category: CategoryOperationalCostControl,
		Title:             "Absolute amount spike",
		RecommendedAction: "Review the account for a large dollar-amount spike versus its baseline.",
	},
	"PERCENTAGE_CHANGE_SPIKE": {
		Code: FindingExpenseSpike, Category: CategoryOperationalCostControl,
		Title:             "Percentage change spike",
		RecommendedAction: "Review the account for a large percentage-change spike versus its baseline.",
	},
	"EXPENSE_OUTPACING_REVENUE": {
		Code: FindingRevenueOutpacedByExpense, Category: CategoryProfitability,
		Title:             "Expense growth outpacing revenue growth",
		RecommendedAction: "Review whether expense growth is outpacing revenue growth for a specific cost category.",
	},
	"MARGIN_DETERIORATION": {
		Code: FindingMarginPressure, Category: CategoryProfitability,
		Title:             "Margin deterioration",
		RecommendedAction: "Review the drivers of gross or operating margin deterioration.",
	},
	"NEW_MATERIAL_EXPENSE_CATEGORY": {
		Code: FindingNewMaterialExpense, Category: CategoryOperationalCostControl,
		Title:             "New material expense category",
		RecommendedAction: "Review the newly appearing expense category and its business justification.",
	},
	"ACCOUNT_DISAPPEARED_REAPPEARED": {
		Code: FindingUnusualAccountActivity, Category: CategoryOperationalCostControl,
		Title:             "Account disappeared or reappeared",
		RecommendedAction: "Review why an account stopped or resumed appearing across periods.",
	},
	"REPEATED_UNUSUAL_VALUE": {
		Code: FindingRepeatedUnusualValue, Category: CategoryOperationalCostControl,
		Title:             "Repeated unusual value",
		RecommendedAction: "Review why the same unusual amount repeats across multiple periods.",
	},
	"SIGN_FLIP": {
		Code: FindingUnusualAccountActivity, Category: CategoryOperationalCostControl,
		Title:             "Account sign flip",
		RecommendedAction: "Review the account for an unexpected sign change (e.g. an expense becoming a credit).",
	},
	"DUPLICATE_LIKE_AMOUNTS": {
		Code: FindingRepeatedUnusualValue, Category: CategoryOperationalCostControl,
		Title:             "Duplicate-like amounts across accounts",
		RecommendedAction: "Review whether the same amount was recorded in error across multiple accounts.",
	},
	"HIGH_OWNER_DISCRETIONARY_SHARE": {
		Code: FindingHighOwnerDiscretionaryShare, Category: CategoryEarningsQuality,
		Title:             "High owner-discretionary expense share",
		RecommendedAction: "Review the share of expenses classified as owner-discretionary.",
	},
	"UNEXPECTED_NEGATIVE_AMOUNT": {
		Code: FindingUnusualAccountActivity, Category: CategoryOperationalCostControl,
		Title:             "Unexpected negative amount",
		RecommendedAction: "Review the account for an unexpected negative balance.",
	},
}

// mineAnomalies mines Input.Anomalies.Anomalies into Findings.
func mineAnomalies(in Input) []Finding {
	if !in.Anomalies.Available {
		return nil
	}
	var out []Finding
	for _, a := range in.Anomalies.Anomalies {
		m, ok := anomalyRuleTable[string(a.Code)]
		if !ok {
			continue
		}
		metricLabel := string(a.Account)
		if metricLabel == "" {
			metricLabel = a.MetricLabel
		}
		f := Finding{
			Code:              m.Code,
			Category:          m.Category,
			Severity:          Severity(a.Severity),
			Title:             m.Title,
			Evidence:          []string{a.Explanation},
			SourceModule:      SourceAnomalies,
			SourceCode:        string(a.Code),
			MetricLabel:       metricLabel,
			Metric:            Value{Available: a.Observed.Available, Amount: a.Observed.Value},
			ComparisonLabel:   baselineLabel(a.Baseline.Available),
			Comparison:        Value{Available: a.Baseline.Available, Amount: a.Baseline.Value},
			Period:            a.Period,
			Explanation:       a.Explanation,
			RecommendedAction: m.RecommendedAction,
		}
		out = append(out, f)
	}
	return out
}

// baselineLabel reports "BASELINE" only when a baseline figure is actually
// available, keeping ComparisonLabel empty alongside an Unavailable
// Comparison — mirroring comparisonLabelForThreshold's identical rule for
// Flag-shaped sources.
func baselineLabel(available bool) string {
	if !available {
		return ""
	}
	return "BASELINE"
}
