package variance

import (
	"testing"

	"github.com/themurtez/go-valuate/financial"
)

func TestCalculate_NoLines(t *testing.T) {
	res := Calculate(Input{})
	if res.Available {
		t.Fatal("expected Available == false for empty Lines")
	}
	if !HasErrors(res.Errors) {
		t.Fatal("expected an error Issue for empty Lines")
	}
	if len(res.Errors) != 1 || res.Errors[0].Code != IssueNoLines {
		t.Fatalf("expected exactly one IssueNoLines, got %+v", res.Errors)
	}
}

// TestCalculate_RevenueFavorableOnIncrease proves a revenue account beating
// its budget is classified favorable, and a revenue account missing budget
// is classified unfavorable.
func TestCalculate_RevenueFavorableOnIncrease(t *testing.T) {
	res := Calculate(Input{
		Lines: []LineObservation{
			line(financial.CodeRevProduct, "2025", 550_000, 500_000, BaselineTypeBudget),
			line(financial.CodeRevService, "2025", 90_000, 100_000, BaselineTypeBudget),
		},
	})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	byCode := lineVarianceByCode(res.LineVariances)

	beat := byCode[financial.CodeRevProduct]
	if beat.Favorability != FavorabilityFavorable {
		t.Errorf("revenue beating budget: expected FavorabilityFavorable, got %s", beat.Favorability)
	}
	if !beat.AbsoluteVariance.Available || beat.AbsoluteVariance.Value != 50_000 {
		t.Errorf("expected AbsoluteVariance 50000, got %+v", beat.AbsoluteVariance)
	}
	if !beat.PercentVariance.Available || beat.PercentVariance.Value != 0.1 {
		t.Errorf("expected PercentVariance 0.1, got %+v", beat.PercentVariance)
	}

	miss := byCode[financial.CodeRevService]
	if miss.Favorability != FavorabilityUnfavorable {
		t.Errorf("revenue missing budget: expected FavorabilityUnfavorable, got %s", miss.Favorability)
	}
	if !miss.AbsoluteVariance.Available || miss.AbsoluteVariance.Value != -10_000 {
		t.Errorf("expected AbsoluteVariance -10000, got %+v", miss.AbsoluteVariance)
	}
}

// TestCalculate_ExpenseUnfavorableOnIncrease proves an OPEX account
// overspending its budget is unfavorable, and underspending is favorable —
// the opposite direction from revenue for the identical variance sign.
func TestCalculate_ExpenseUnfavorableOnIncrease(t *testing.T) {
	res := Calculate(Input{
		Lines: []LineObservation{
			line(financial.CodeOpexMarketing, "2025", 35_000, 30_000, BaselineTypeBudget), // over budget
			line(financial.CodeOpexPayroll, "2025", 140_000, 150_000, BaselineTypeBudget), // under budget
		},
	})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	byCode := lineVarianceByCode(res.LineVariances)

	over := byCode[financial.CodeOpexMarketing]
	if over.Favorability != FavorabilityUnfavorable {
		t.Errorf("expense over budget: expected FavorabilityUnfavorable, got %s", over.Favorability)
	}

	under := byCode[financial.CodeOpexPayroll]
	if under.Favorability != FavorabilityFavorable {
		t.Errorf("expense under budget: expected FavorabilityFavorable, got %s", under.Favorability)
	}
}

// TestCalculate_COGS proves COGS is treated with expense semantics
// (increase is unfavorable), same as OPEX.
func TestCalculate_COGS(t *testing.T) {
	res := Calculate(Input{
		Lines: []LineObservation{
			line(financial.CodeCogsMaterial, "2025", 260_000, 240_000, BaselineTypeBudget),
		},
	})
	lv := res.LineVariances[0]
	if lv.TaxonomyCategory != financial.CategoryCogs {
		t.Fatalf("expected TaxonomyCategory cogs, got %s", lv.TaxonomyCategory)
	}
	if lv.Favorability != FavorabilityUnfavorable {
		t.Errorf("COGS over budget: expected FavorabilityUnfavorable, got %s", lv.Favorability)
	}
}

// TestCalculate_ZeroBaseline proves a zero baseline yields an available
// AbsoluteVariance but an Unavailable PercentVariance, with
// VarianceFromZeroBase set.
func TestCalculate_ZeroBaseline(t *testing.T) {
	res := Calculate(Input{
		Lines: []LineObservation{
			line(financial.CodeOpexSoftware, "2025", 12_000, 0, BaselineTypeBudget),
		},
	})
	lv := res.LineVariances[0]
	if !lv.AbsoluteVariance.Available || lv.AbsoluteVariance.Value != 12_000 {
		t.Fatalf("expected AbsoluteVariance 12000, got %+v", lv.AbsoluteVariance)
	}
	if lv.PercentVariance.Available {
		t.Fatalf("expected PercentVariance unavailable for zero baseline, got %+v", lv.PercentVariance)
	}
	if !lv.VarianceFromZeroBase {
		t.Fatal("expected VarianceFromZeroBase == true")
	}
	// New spend against a zero budget on an expense account is unfavorable.
	if lv.Favorability != FavorabilityUnfavorable {
		t.Errorf("expected FavorabilityUnfavorable, got %s", lv.Favorability)
	}
}

// TestCalculate_NegativeValues proves negative actual/baseline amounts
// (out-of-convention for revenue/expense codes, which financial/metrics'
// sign convention treats as normally non-negative — see
// anomalies.RuleUnexpectedNegativeAmount) still compute a correct signed
// arithmetic variance rather than erroring or panicking: Actual - Baseline
// is a positive 3000 here (-5000 is algebraically greater than -8000), and
// this package's direction rule is a pure sign comparison on that
// arithmetic result, so a positive variance on an expense code is
// classified unfavorable exactly as it would be for positive amounts. A
// caller feeding contra/negative expense amounts is responsible for
// knowing this package does not special-case that convention break.
func TestCalculate_NegativeValues(t *testing.T) {
	res := Calculate(Input{
		Lines: []LineObservation{
			line(financial.CodeOtherExpense, "2025", -5_000, -8_000, BaselineTypeBudget),
		},
	})
	lv := res.LineVariances[0]
	if !lv.AbsoluteVariance.Available || lv.AbsoluteVariance.Value != 3_000 {
		t.Fatalf("expected AbsoluteVariance 3000, got %+v", lv.AbsoluteVariance)
	}
	if lv.Favorability != FavorabilityUnfavorable {
		t.Errorf("expected FavorabilityUnfavorable, got %s", lv.Favorability)
	}
}

// TestCalculate_MissingBaseline proves a line with no baseline leaves every
// variance figure Unavailable and records an advisory IssueMissingBaseline,
// without erroring the whole calculation.
func TestCalculate_MissingBaseline(t *testing.T) {
	res := Calculate(Input{
		Lines: []LineObservation{
			lineNoBaseline(financial.CodeRevProduct, "2025", 100_000),
			line(financial.CodeRevService, "2025", 50_000, 45_000, BaselineTypeBudget),
		},
	})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	byCode := lineVarianceByCode(res.LineVariances)

	missing := byCode[financial.CodeRevProduct]
	if missing.BaselineAvailable {
		t.Fatal("expected BaselineAvailable == false")
	}
	if missing.AbsoluteVariance.Available {
		t.Fatalf("expected AbsoluteVariance unavailable, got %+v", missing.AbsoluteVariance)
	}
	if missing.Favorability != FavorabilityUnknown {
		t.Errorf("expected FavorabilityUnknown, got %s", missing.Favorability)
	}
	if missing.Materiality != MaterialityUnknown {
		t.Errorf("expected MaterialityUnknown, got %s", missing.Materiality)
	}

	var found bool
	for _, w := range res.Warnings {
		if w.Code == IssueMissingBaseline {
			found = true
		}
	}
	if !found {
		t.Error("expected IssueMissingBaseline warning")
	}

	// The line missing a baseline should not appear in the Bridge.
	if res.Bridge.LineCount != 1 {
		t.Errorf("expected Bridge.LineCount == 1 (only the line with a baseline), got %d", res.Bridge.LineCount)
	}
}

// TestCalculate_MissingBaselineType proves BaselineAvailable == true with an
// empty BaselineType still computes variance but records an advisory issue.
func TestCalculate_MissingBaselineType(t *testing.T) {
	res := Calculate(Input{
		Lines: []LineObservation{
			{AccountCode: financial.CodeRevProduct, Period: "2025", Actual: 100_000, BaselineAvailable: true, Baseline: 90_000},
		},
	})
	lv := res.LineVariances[0]
	if !lv.AbsoluteVariance.Available {
		t.Fatal("expected AbsoluteVariance available despite missing BaselineType")
	}
	var found bool
	for _, w := range res.Warnings {
		if w.Code == IssueMissingBaselineType {
			found = true
		}
	}
	if !found {
		t.Error("expected IssueMissingBaselineType warning")
	}
}

// TestCalculate_UnknownAccountCode proves an unrecognized financial.Code
// with no DirectionOverrides leaves Favorability unknown but still computes
// AbsoluteVariance, and that a DirectionOverride resolves it.
func TestCalculate_UnknownAccountCode(t *testing.T) {
	unknown := financial.Code("CUSTOM_LINE")

	res := Calculate(Input{
		Lines: []LineObservation{
			line(unknown, "2025", 12_000, 10_000, BaselineTypeCustom),
		},
	})
	lv := res.LineVariances[0]
	if lv.Favorability != FavorabilityUnknown {
		t.Fatalf("expected FavorabilityUnknown, got %s", lv.Favorability)
	}
	if !lv.AbsoluteVariance.Available || lv.AbsoluteVariance.Value != 2_000 {
		t.Fatalf("expected AbsoluteVariance 2000 even with unknown code, got %+v", lv.AbsoluteVariance)
	}
	var found bool
	for _, w := range res.Warnings {
		if w.Code == IssueUnknownAccountCode {
			found = true
		}
	}
	if !found {
		t.Error("expected IssueUnknownAccountCode warning")
	}

	resOverride := Calculate(Input{
		Lines: []LineObservation{
			line(unknown, "2025", 12_000, 10_000, BaselineTypeCustom),
		},
		Policy: Policy{
			DirectionOverrides: []DirectionOverride{
				{AccountCode: unknown, IncreaseIsFavorable: false},
			},
		},
	})
	lvOverride := resOverride.LineVariances[0]
	if lvOverride.Favorability != FavorabilityUnfavorable {
		t.Fatalf("expected DirectionOverride to force FavorabilityUnfavorable, got %s", lvOverride.Favorability)
	}
}

// TestCalculate_MultiplePeriods_PeriodTrends proves PeriodTrends aggregates
// each period and TrendSummary characterizes the overall direction using
// PeriodMeta-driven chronological order.
func TestCalculate_MultiplePeriods_PeriodTrends(t *testing.T) {
	res := Calculate(Input{
		Lines:      budgetVsActualLines(),
		PeriodMeta: threeYearMeta(),
	})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	if len(res.PeriodTrends) != 3 {
		t.Fatalf("expected 3 PeriodTrends, got %d", len(res.PeriodTrends))
	}
	if res.PeriodTrends[0].Period != "2023" || res.PeriodTrends[2].Period != "2025" {
		t.Fatalf("expected chronological order 2023..2025, got %v, %v, %v",
			res.PeriodTrends[0].Period, res.PeriodTrends[1].Period, res.PeriodTrends[2].Period)
	}
	if res.TrendSummary.Direction == TrendUnavailable {
		t.Fatal("expected a computed TrendSummary.Direction")
	}
}

// TestCalculate_NoPeriodMeta proves PeriodTrends/TrendSummary are left
// unavailable (with an advisory warning) when PeriodMeta is not supplied,
// while every other output is still fully computed.
func TestCalculate_NoPeriodMeta(t *testing.T) {
	res := Calculate(Input{Lines: budgetVsActualLines()})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	if len(res.PeriodTrends) != 0 {
		t.Fatalf("expected no PeriodTrends without PeriodMeta, got %d", len(res.PeriodTrends))
	}
	if res.TrendSummary.Direction != TrendUnavailable {
		t.Fatalf("expected TrendUnavailable, got %s", res.TrendSummary.Direction)
	}
	var found bool
	for _, w := range res.Warnings {
		if w.Code == IssueNoPeriodMeta {
			found = true
		}
	}
	if !found {
		t.Error("expected IssueNoPeriodMeta warning")
	}
	if len(res.LineVariances) == 0 {
		t.Fatal("expected LineVariances still computed without PeriodMeta")
	}
}

// TestCalculate_CategoryRollup proves CategorySummaries aggregates by
// taxonomy category correctly (revenue vs. cogs vs. opex).
func TestCalculate_CategoryRollup(t *testing.T) {
	res := Calculate(Input{
		Lines:      budgetVsActualLines(),
		PeriodMeta: threeYearMeta(),
	})
	summaries := make(map[string]CategorySummary)
	for _, cs := range res.CategorySummaries {
		summaries[cs.Category] = cs
	}

	rev, ok := summaries[string(financial.CategoryRevenue)]
	if !ok {
		t.Fatal("expected a revenue CategorySummary")
	}
	if rev.LineCount != 3 {
		t.Errorf("expected 3 revenue lines, got %d", rev.LineCount)
	}
	wantRevActual := 550_000.0 + 610_000.0 + 590_000.0
	if rev.TotalActual != wantRevActual {
		t.Errorf("expected TotalActual %v, got %v", wantRevActual, rev.TotalActual)
	}

	cogs, ok := summaries[string(financial.CategoryCogs)]
	if !ok {
		t.Fatal("expected a cogs CategorySummary")
	}
	if cogs.LineCount != 3 {
		t.Errorf("expected 3 cogs lines, got %d", cogs.LineCount)
	}

	opex, ok := summaries[string(financial.CategoryOpex)]
	if !ok {
		t.Fatal("expected an opex CategorySummary")
	}
	if opex.LineCount != 6 {
		t.Errorf("expected 6 opex lines (payroll+marketing x3 years), got %d", opex.LineCount)
	}
}

// TestCalculate_CustomCategoryRollup proves CustomCategorySummaries
// aggregates by the caller-supplied Category label, independent of
// taxonomy category.
func TestCalculate_CustomCategoryRollup(t *testing.T) {
	res := Calculate(Input{
		Lines: []LineObservation{
			lineCat(financial.CodeRevProduct, "2025", 100_000, 90_000, BaselineTypeBudget, "east-region"),
			lineCat(financial.CodeRevService, "2025", 50_000, 55_000, BaselineTypeBudget, "east-region"),
			lineCat(financial.CodeRevProduct, "2025", 70_000, 60_000, BaselineTypeBudget, "west-region"),
		},
	})
	if len(res.CustomCategorySummaries) != 2 {
		t.Fatalf("expected 2 custom category summaries, got %d", len(res.CustomCategorySummaries))
	}
	byCategory := make(map[string]CategorySummary)
	for _, cs := range res.CustomCategorySummaries {
		byCategory[cs.Category] = cs
	}
	east, ok := byCategory["east-region"]
	if !ok {
		t.Fatal("expected east-region summary")
	}
	if east.LineCount != 2 {
		t.Errorf("expected 2 lines in east-region, got %d", east.LineCount)
	}
	if east.TotalActual != 150_000 {
		t.Errorf("expected TotalActual 150000, got %v", east.TotalActual)
	}
}

// TestCalculate_TopFavorableAndUnfavorable proves TopFavorable/TopUnfavorable
// rank by dollar magnitude and respect Policy.TopN.
func TestCalculate_TopFavorableAndUnfavorable(t *testing.T) {
	res := Calculate(Input{
		Lines: []LineObservation{
			line(financial.CodeRevProduct, "2025", 600_000, 500_000, BaselineTypeBudget),  // +100k favorable
			line(financial.CodeRevService, "2025", 120_000, 100_000, BaselineTypeBudget),  // +20k favorable
			line(financial.CodeOpexMarketing, "2025", 60_000, 30_000, BaselineTypeBudget), // +30k unfavorable
			line(financial.CodeOpexPayroll, "2025", 200_000, 150_000, BaselineTypeBudget), // +50k unfavorable
		},
		Policy: Policy{TopN: 1},
	})
	if len(res.TopFavorable) != 1 || res.TopFavorable[0].AccountCode != financial.CodeRevProduct {
		t.Fatalf("expected top-1 favorable to be CodeRevProduct, got %+v", res.TopFavorable)
	}
	if len(res.TopUnfavorable) != 1 || res.TopUnfavorable[0].AccountCode != financial.CodeOpexPayroll {
		t.Fatalf("expected top-1 unfavorable to be CodeOpexPayroll (largest unfavorable magnitude), got %+v", res.TopUnfavorable)
	}
}

// TestCalculate_MaterialExceptions proves materiality gating is off by
// default (mirroring review.IsMaterial) and activates only when a caller
// supplies a positive threshold, and that ContributionToTotalVariance sums
// correctly against Bridge.TotalAbsoluteVariance.
func TestCalculate_MaterialExceptions(t *testing.T) {
	lines := []LineObservation{
		line(financial.CodeRevProduct, "2025", 600_000, 500_000, BaselineTypeBudget), // 100k variance
		line(financial.CodeOpexOffice, "2025", 5_100, 5_000, BaselineTypeBudget),     // 100 variance
	}

	resDefault := Calculate(Input{Lines: lines})
	if len(resDefault.MaterialExceptions) != 0 {
		t.Fatalf("expected no material exceptions with default (off) Policy, got %d", len(resDefault.MaterialExceptions))
	}

	resThreshold := Calculate(Input{
		Lines:  lines,
		Policy: Policy{MaterialAmountThreshold: 10_000},
	})
	if len(resThreshold.MaterialExceptions) != 1 {
		t.Fatalf("expected exactly 1 material exception, got %d", len(resThreshold.MaterialExceptions))
	}
	me := resThreshold.MaterialExceptions[0]
	if me.AccountCode != financial.CodeRevProduct {
		t.Fatalf("expected the material exception to be CodeRevProduct, got %s", me.AccountCode)
	}
	if !me.ContributionToTotalVariance.Available {
		t.Fatal("expected ContributionToTotalVariance available")
	}
	wantContribution := 100_000.0 / (100_000.0 + 100.0)
	if diffAbs(me.ContributionToTotalVariance.Value, wantContribution) > 1e-9 {
		t.Errorf("expected ContributionToTotalVariance %v, got %v", wantContribution, me.ContributionToTotalVariance.Value)
	}
}

// TestCalculate_MaterialityByPercent proves the percent-of-baseline leg
// triggers materiality independently of the absolute-dollar leg (OR logic).
func TestCalculate_MaterialityByPercent(t *testing.T) {
	res := Calculate(Input{
		Lines: []LineObservation{
			line(financial.CodeOpexOffice, "2025", 1_200, 1_000, BaselineTypeBudget), // 20% variance, small dollar amount
		},
		Policy: Policy{MaterialPercentOfBaseline: 0.1},
	})
	lv := res.LineVariances[0]
	if lv.Materiality != MaterialityMaterial {
		t.Fatalf("expected MaterialityMaterial via percent leg, got %s", lv.Materiality)
	}
}

// TestCalculate_Bridge proves the Bridge totals reconcile: FavorableVariance
// + UnfavorableVariance == TotalVariance when every line has a known
// Favorability, and TotalVariance == TotalActual - TotalBaseline.
func TestCalculate_Bridge(t *testing.T) {
	res := Calculate(Input{Lines: budgetVsActualLines()})
	b := res.Bridge
	if b.LineCount != 12 {
		t.Fatalf("expected 12 lines in bridge, got %d", b.LineCount)
	}
	if diffAbs(b.TotalVariance, b.TotalActual-b.TotalBaseline) > 1e-9 {
		t.Errorf("expected TotalVariance == TotalActual - TotalBaseline, got %v vs %v", b.TotalVariance, b.TotalActual-b.TotalBaseline)
	}
	if diffAbs(b.FavorableVariance+b.UnfavorableVariance, b.TotalVariance) > 1e-9 {
		t.Errorf("expected FavorableVariance + UnfavorableVariance == TotalVariance, got %v + %v != %v",
			b.FavorableVariance, b.UnfavorableVariance, b.TotalVariance)
	}
}

// TestCalculate_ExactMatchIsNeutral proves an actual exactly equal to
// baseline yields FavorabilityNeutral with a zero AbsoluteVariance.
func TestCalculate_ExactMatchIsNeutral(t *testing.T) {
	res := Calculate(Input{
		Lines: []LineObservation{
			line(financial.CodeOpexRent, "2025", 24_000, 24_000, BaselineTypeBudget),
		},
	})
	lv := res.LineVariances[0]
	if lv.Favorability != FavorabilityNeutral {
		t.Fatalf("expected FavorabilityNeutral, got %s", lv.Favorability)
	}
	if !lv.AbsoluteVariance.Available || lv.AbsoluteVariance.Value != 0 {
		t.Fatalf("expected AbsoluteVariance 0, got %+v", lv.AbsoluteVariance)
	}
}

// TestCalculate_OtherIncomeStatementMixedDirection proves the
// other-income-statement category's mixed income/expense lines each resolve
// individually rather than uniformly.
func TestCalculate_OtherIncomeStatementMixedDirection(t *testing.T) {
	res := Calculate(Input{
		Lines: []LineObservation{
			line(financial.CodeInterestIncome, "2025", 12_000, 10_000, BaselineTypeBudget),  // income, up is favorable
			line(financial.CodeInterestExpense, "2025", 12_000, 10_000, BaselineTypeBudget), // expense, up is unfavorable
		},
	})
	byCode := lineVarianceByCode(res.LineVariances)
	if byCode[financial.CodeInterestIncome].Favorability != FavorabilityFavorable {
		t.Errorf("expected interest income increase to be favorable, got %s", byCode[financial.CodeInterestIncome].Favorability)
	}
	if byCode[financial.CodeInterestExpense].Favorability != FavorabilityUnfavorable {
		t.Errorf("expected interest expense increase to be unfavorable, got %s", byCode[financial.CodeInterestExpense].Favorability)
	}
}

func lineVarianceByCode(lvs []LineVariance) map[financial.Code]LineVariance {
	m := make(map[financial.Code]LineVariance, len(lvs))
	for _, lv := range lvs {
		m[lv.AccountCode] = lv
	}
	return m
}

func diffAbs(a, b float64) float64 {
	d := a - b
	if d < 0 {
		return -d
	}
	return d
}
