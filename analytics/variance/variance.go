package variance

import (
	"fmt"

	"github.com/themurtez/go-valuate/financial"
)

// Calculate derives a full Result from in under in.Policy. It never mutates
// in.Lines and performs no I/O.
func Calculate(in Input) Result {
	policy := resolvePolicy(in.Policy)
	result := Result{FormulaVersion: FormulaVersion, Policy: policy}

	if len(in.Lines) == 0 {
		result.Errors = append(result.Errors, Issue{
			Code:     IssueNoLines,
			Severity: SeverityError,
			Message:  "no lines supplied; variance analysis requires at least one",
		})
		return result
	}
	result.Available = true

	directions := buildDirectionIndex(policy.DirectionOverrides)

	lineVariances := make([]LineVariance, 0, len(in.Lines))
	var warnings []Issue
	for _, line := range in.Lines {
		lv, issues := computeLineVariance(line, directions, policy)
		lineVariances = append(lineVariances, lv)
		warnings = append(warnings, issues...)
	}

	sortLineVariances(lineVariances, in.PeriodMeta)
	result.LineVariances = lineVariances
	result.Warnings = append(result.Warnings, warnings...)

	result.CategorySummaries = buildCategorySummaries(lineVariances, taxonomyCategoryKey)
	result.CustomCategorySummaries = buildCategorySummaries(lineVariances, customCategoryKey)

	result.TopFavorable = topFavorable(lineVariances, policy.TopN)
	result.TopUnfavorable = topUnfavorable(lineVariances, policy.TopN)

	result.Bridge = buildBridge(lineVariances)
	result.MaterialExceptions = buildMaterialExceptions(lineVariances, result.Bridge)

	orderedPeriods, orderIssue := chronologicalPeriods(periodsOf(lineVariances), in.PeriodMeta)
	if orderIssue != nil {
		result.Warnings = append(result.Warnings, *orderIssue)
		result.TrendSummary = TrendSummary{Direction: TrendUnavailable}
	} else {
		periodTrends := buildPeriodTrends(lineVariances, orderedPeriods)
		result.PeriodTrends = periodTrends
		result.TrendSummary = buildTrendSummary(periodTrends)
	}

	return result
}

// computeLineVariance computes one LineObservation's LineVariance plus any
// advisory Issues it produced.
func computeLineVariance(line LineObservation, directions directionIndex, policy Policy) (LineVariance, []Issue) {
	lv := LineVariance{
		AccountCode:       line.AccountCode,
		Label:             resolveLabel(line),
		Period:            line.Period,
		Category:          line.Category,
		Actual:            line.Actual,
		BaselineAvailable: line.BaselineAvailable,
		Baseline:          line.Baseline,
		BaselineType:      line.BaselineType,
	}

	if meta, ok := financial.LookupCode(line.AccountCode); ok {
		lv.TaxonomyCategory = meta.Category
	}

	var issues []Issue

	if !line.BaselineAvailable {
		issues = append(issues, Issue{
			Code:     IssueMissingBaseline,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("account %s period %s: no baseline available; variance figures unavailable", line.AccountCode, line.Period),
		})
		lv.Favorability = FavorabilityUnknown
		lv.Materiality = MaterialityUnknown
		return lv, issues
	}

	if line.BaselineType == "" {
		issues = append(issues, Issue{
			Code:     IssueMissingBaselineType,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("account %s period %s: baseline available but BaselineType is empty", line.AccountCode, line.Period),
		})
	}

	absVariance := line.Actual - line.Baseline
	lv.AbsoluteVariance = AvailableValue(absVariance)

	if line.Baseline == 0 {
		lv.VarianceFromZeroBase = true
	} else {
		lv.PercentVariance = AvailableValue(absVariance / absFloat(line.Baseline))
	}

	increaseFavorable, known := directions.resolve(line.AccountCode)
	if !known {
		issues = append(issues, Issue{
			Code:     IssueUnknownAccountCode,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("account %s: not a recognized taxonomy code and no DirectionOverrides entry; favorability unknown", line.AccountCode),
		})
	}

	switch {
	case absVariance == 0:
		lv.Favorability = FavorabilityNeutral
	case !known:
		lv.Favorability = FavorabilityUnknown
	case (absVariance > 0) == increaseFavorable:
		lv.Favorability = FavorabilityFavorable
	default:
		lv.Favorability = FavorabilityUnfavorable
	}

	if isMaterial(absVariance, line.Baseline, policy) {
		lv.Materiality = MaterialityMaterial
	} else {
		lv.Materiality = MaterialityImmaterial
	}

	return lv, issues
}

// resolveLabel returns line.Label if non-empty, otherwise AccountCode's
// financial.CodeMeta.Label when recognized, otherwise empty.
func resolveLabel(line LineObservation) string {
	if line.Label != "" {
		return line.Label
	}
	if meta, ok := financial.LookupCode(line.AccountCode); ok {
		return meta.Label
	}
	return ""
}

// isMaterial mirrors review.IsMaterial's exact OR-of-two-legs, off-by-default
// design: a variance is material if its unsigned amount is at or above
// Policy.MaterialAmountThreshold OR (Policy.MaterialPercentOfBaseline > 0
// and the unsigned amount is at or above that fraction of |baseline|). Both
// legs default to 0 (off); with both at 0, isMaterial always returns false
// (mirroring review.IsMaterial's inverse convention, adapted here since
// this package's LineVariance always carries an explicit
// MaterialityImmaterial/MaterialityMaterial classification rather than a
// bool gate applied elsewhere).
func isMaterial(variance float64, baseline float64, policy Policy) bool {
	abs := absFloat(variance)
	if policy.MaterialAmountThreshold > 0 && abs >= policy.MaterialAmountThreshold {
		return true
	}
	if policy.MaterialPercentOfBaseline > 0 && baseline != 0 && abs >= policy.MaterialPercentOfBaseline*absFloat(baseline) {
		return true
	}
	return false
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// directionIndex resolves a financial.Code to its favorable-on-increase
// direction, consulting caller overrides before falling back to taxonomy
// category defaults.
type directionIndex struct {
	overrides map[financial.Code]bool
}

func buildDirectionIndex(overrides []DirectionOverride) directionIndex {
	idx := directionIndex{overrides: make(map[financial.Code]bool, len(overrides))}
	for _, o := range overrides {
		if _, exists := idx.overrides[o.AccountCode]; exists {
			continue // first entry wins — deterministic, caller-controlled precedence.
		}
		idx.overrides[o.AccountCode] = o.IncreaseIsFavorable
	}
	return idx
}

// resolve returns whether an increase in code's actual (vs. baseline) is
// favorable, and whether that direction is known at all.
func (idx directionIndex) resolve(code financial.Code) (increaseIsFavorable bool, known bool) {
	if v, ok := idx.overrides[code]; ok {
		return v, true
	}
	meta, ok := financial.LookupCode(code)
	if !ok {
		return false, false
	}
	switch meta.Category {
	case financial.CategoryRevenue:
		return true, true
	case financial.CategoryCogs, financial.CategoryOpex:
		return false, true
	case financial.CategoryOtherIncomeStatement:
		// Other-income-statement mixes true income lines (interest income,
		// other income) with expense lines (depreciation, amortization,
		// interest expense, income tax) — this package resolves each
		// individually by financial.Code rather than treating the whole
		// category uniformly, since "an increase is favorable" is not a
		// safe default for this mixed category as a whole.
		switch code {
		case financial.CodeInterestIncome, financial.CodeOtherIncome:
			return true, true
		case financial.CodeDepreciation, financial.CodeAmortization,
			financial.CodeInterestExpense, financial.CodeIncomeTax, financial.CodeOtherExpense:
			return false, true
		default:
			return false, false
		}
	default:
		// Balance-sheet codes (and any future category) have no inherent
		// income-statement favorability direction; a caller analyzing
		// variance on a balance-sheet line must supply a
		// DirectionOverrides entry.
		return false, false
	}
}
