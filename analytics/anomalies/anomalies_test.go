package anomalies

import (
	"testing"

	"github.com/themurtez/go-valuate/financial"
)

func hasRule(anomalies []Anomaly, code RuleCode) bool {
	for _, a := range anomalies {
		if a.Code == code {
			return true
		}
	}
	return false
}

func countRule(anomalies []Anomaly, code RuleCode) int {
	n := 0
	for _, a := range anomalies {
		if a.Code == code {
			n++
		}
	}
	return n
}

func findRuleForAccount(anomalies []Anomaly, code RuleCode, account financial.Code) (Anomaly, bool) {
	for _, a := range anomalies {
		if a.Code == code && a.Account == account {
			return a, true
		}
	}
	return Anomaly{}, false
}

// TestCalculate_NoPeriods proves the documented Available == false / IssueNoPeriods
// contract for an empty dataset.
func TestCalculate_NoPeriods(t *testing.T) {
	res := Calculate(Input{}, Options{})
	if res.Available {
		t.Fatal("expected Available == false for an empty dataset")
	}
	if !HasErrors(res.Errors) {
		t.Fatal("expected at least one error Issue")
	}
	if res.Errors[0].Code != IssueNoPeriods {
		t.Fatalf("expected IssueNoPeriods, got %s", res.Errors[0].Code)
	}
	if res.FormulaVersion != FormulaVersion {
		t.Fatalf("expected FormulaVersion echoed even when unavailable, got %q", res.FormulaVersion)
	}
}

// TestCalculate_StableDataset proves a flat, unremarkable multi-year dataset
// produces zero anomalies — the "stable dataset" case the task explicitly
// requires coverage for.
func TestCalculate_StableDataset(t *testing.T) {
	res := Calculate(Input{Dataset: stableDataset(), PeriodMeta: fourYearMeta()}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	if len(res.Anomalies) != 0 {
		t.Fatalf("expected zero anomalies for a stable dataset, got %d: %+v", len(res.Anomalies), res.Anomalies)
	}
	if res.Summary.Total != 0 || res.Summary.ReviewRecommended {
		t.Fatalf("expected empty, non-review-recommended Summary, got %+v", res.Summary)
	}
}

// TestCalculate_AbsoluteAmountSpike proves a large dollar-amount jump on one
// account triggers RuleAbsoluteAmountSpike and RulePercentageChangeSpike,
// with every documented Anomaly field populated correctly.
func TestCalculate_AbsoluteAmountSpike(t *testing.T) {
	res := Calculate(Input{Dataset: spikeDataset(), PeriodMeta: fourYearMeta()}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}

	a, ok := findRuleForAccount(res.Anomalies, RuleAbsoluteAmountSpike, financial.CodeOpexMarketing)
	if !ok {
		t.Fatalf("expected RuleAbsoluteAmountSpike for CodeOpexMarketing, got %+v", res.Anomalies)
	}
	if a.Period != "2025" || a.BaselinePeriod != "2024" {
		t.Fatalf("expected Period=2025/BaselinePeriod=2024, got Period=%s BaselinePeriod=%s", a.Period, a.BaselinePeriod)
	}
	if !a.Baseline.Available || a.Baseline.Value != 40_400 {
		t.Fatalf("expected Baseline=40400 (year-4 stable marketing before override), got %+v", a.Baseline)
	}
	if !a.Observed.Available || a.Observed.Value != 200_000 {
		t.Fatalf("expected Observed=200000, got %+v", a.Observed)
	}
	if a.Delta != 200_000-40_400 {
		t.Fatalf("expected Delta=159600, got %v", a.Delta)
	}
	if a.Threshold != DefaultThresholds().AbsoluteAmountSpike {
		t.Fatalf("expected Threshold echoed, got %v", a.Threshold)
	}
	if a.Severity != AnomalySeverityWarning {
		t.Fatalf("expected AnomalySeverityWarning, got %s", a.Severity)
	}
	if a.Explanation == "" {
		t.Fatal("expected non-empty Explanation")
	}

	if !hasRule(res.Anomalies, RulePercentageChangeSpike) {
		t.Fatal("expected the same spike to also clear RulePercentageChangeSpike (350% change)")
	}
}

// TestCalculate_RevenueLinkedExpenseGrowth proves RuleExpenseOutpacingRevenue
// fires when an expense grows much faster than revenue over the same period.
func TestCalculate_RevenueLinkedExpenseGrowth(t *testing.T) {
	meta := map[financial.Period]PeriodInfo{
		"2024": {Type: PeriodTypeFiscalYear, FiscalYear: 2024},
		"2025": {Type: PeriodTypeFiscalYear, FiscalYear: 2025},
	}
	res := Calculate(Input{Dataset: revenueLinkedExpenseGrowthDataset(), PeriodMeta: meta}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}

	a, ok := findRuleForAccount(res.Anomalies, RuleExpenseOutpacingRevenue, financial.CodeOpexMarketing)
	if !ok {
		t.Fatalf("expected RuleExpenseOutpacingRevenue for CodeOpexMarketing, got %+v", res.Anomalies)
	}
	if a.Period != "2025" || a.BaselinePeriod != "2024" {
		t.Fatalf("expected Period=2025/BaselinePeriod=2024, got %s/%s", a.Period, a.BaselinePeriod)
	}
	wantGap := 0.6 - 0.05
	if diffAbs(a.Delta, wantGap) > 1e-9 {
		t.Fatalf("expected Delta (growth gap) ~= %v, got %v", wantGap, a.Delta)
	}
}

// TestCalculate_MarginDeterioration proves RuleMarginDeterioration fires
// when gross margin erodes by more than the configured point threshold,
// with MetricLabel set (not Account, since this is a synthetic figure).
func TestCalculate_MarginDeterioration(t *testing.T) {
	meta := map[financial.Period]PeriodInfo{
		"2024": {Type: PeriodTypeFiscalYear, FiscalYear: 2024},
		"2025": {Type: PeriodTypeFiscalYear, FiscalYear: 2025},
	}
	res := Calculate(Input{Dataset: marginLeakDataset(), PeriodMeta: meta}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}

	var found *Anomaly
	for i := range res.Anomalies {
		if res.Anomalies[i].Code == RuleMarginDeterioration && res.Anomalies[i].MetricLabel == "gross_margin" {
			found = &res.Anomalies[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("expected RuleMarginDeterioration for gross_margin, got %+v", res.Anomalies)
	}
	if found.Account != "" {
		t.Fatalf("expected empty Account for a synthetic-metric anomaly, got %q", found.Account)
	}
	if !found.Baseline.Available || diffAbs(found.Baseline.Value, 0.6) > 1e-9 {
		t.Fatalf("expected Baseline=0.6 (60%% gross margin in 2024), got %+v", found.Baseline)
	}
	if !found.Observed.Available || diffAbs(found.Observed.Value, 0.45) > 1e-9 {
		t.Fatalf("expected Observed=0.45 (45%% gross margin in 2025), got %+v", found.Observed)
	}
}

// TestCalculate_SignFlip proves RuleSignFlip fires when an account's sign
// reverses between adjacent periods with both magnitudes material.
func TestCalculate_SignFlip(t *testing.T) {
	meta := map[financial.Period]PeriodInfo{
		"2024": {Type: PeriodTypeFiscalYear, FiscalYear: 2024},
		"2025": {Type: PeriodTypeFiscalYear, FiscalYear: 2025},
	}
	res := Calculate(Input{Dataset: signFlipDataset(), PeriodMeta: meta}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}

	a, ok := findRuleForAccount(res.Anomalies, RuleSignFlip, financial.CodeOtherIncome)
	if !ok {
		t.Fatalf("expected RuleSignFlip for CodeOtherIncome, got %+v", res.Anomalies)
	}
	if !a.Baseline.Available || a.Baseline.Value != 50_000 {
		t.Fatalf("expected Baseline=50000, got %+v", a.Baseline)
	}
	if !a.Observed.Available || a.Observed.Value != -30_000 {
		t.Fatalf("expected Observed=-30000, got %+v", a.Observed)
	}
}

// TestCalculate_SignFlip_BelowMagnitudeFloor proves a sign change on a
// near-zero balance does NOT trigger RuleSignFlip.
func TestCalculate_SignFlip_BelowMagnitudeFloor(t *testing.T) {
	meta := map[financial.Period]PeriodInfo{
		"2024": {Type: PeriodTypeFiscalYear, FiscalYear: 2024},
		"2025": {Type: PeriodTypeFiscalYear, FiscalYear: 2025},
	}
	ds := datasetOf(
		item(financial.CodeOtherIncome, "2024", 50),
		item(financial.CodeOtherIncome, "2025", -40),
	)
	res := Calculate(Input{Dataset: ds, PeriodMeta: meta}, Options{})
	if hasRule(res.Anomalies, RuleSignFlip) {
		t.Fatalf("expected no RuleSignFlip below SignFlipMinMagnitude, got %+v", res.Anomalies)
	}
}

// TestCalculate_ImmaterialChanges proves small, steady period-over-period
// changes stay under every spike-family threshold and produce zero
// anomalies — the "small immaterial changes" case the task requires.
func TestCalculate_ImmaterialChanges(t *testing.T) {
	meta := map[financial.Period]PeriodInfo{
		"2024": {Type: PeriodTypeFiscalYear, FiscalYear: 2024},
		"2025": {Type: PeriodTypeFiscalYear, FiscalYear: 2025},
	}
	res := Calculate(Input{Dataset: immaterialChangeDataset(), PeriodMeta: meta}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	if len(res.Anomalies) != 0 {
		t.Fatalf("expected zero anomalies for immaterial changes, got %+v", res.Anomalies)
	}
}

// TestCalculate_MissingPeriods covers RuleAccountDisappearedReappeared's
// "disappeared" branch and RuleNewMaterialExpenseCategory together, per the
// task's explicit "missing periods" coverage requirement.
func TestCalculate_MissingPeriods(t *testing.T) {
	res := Calculate(Input{Dataset: missingPeriodDataset(), PeriodMeta: fourYearMeta()}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}

	disappeared, ok := findRuleForAccount(res.Anomalies, RuleAccountDisappearedReappeared, financial.CodeOpexSoftware)
	if !ok {
		t.Fatalf("expected RuleAccountDisappearedReappeared for CodeOpexSoftware, got %+v", res.Anomalies)
	}
	if disappeared.Period != "2024" || disappeared.BaselinePeriod != "2023" {
		t.Fatalf("expected disappeared at Period=2024/BaselinePeriod=2023, got %s/%s", disappeared.Period, disappeared.BaselinePeriod)
	}
	if disappeared.Observed.Available {
		t.Fatalf("expected Observed unavailable for a disappeared account, got %+v", disappeared.Observed)
	}
	if !disappeared.Baseline.Available || disappeared.Baseline.Value != 13_000 {
		t.Fatalf("expected Baseline=13000, got %+v", disappeared.Baseline)
	}

	newCat, ok := findRuleForAccount(res.Anomalies, RuleNewMaterialExpenseCategory, financial.CodeOpexTravel)
	if !ok {
		t.Fatalf("expected RuleNewMaterialExpenseCategory for CodeOpexTravel, got %+v", res.Anomalies)
	}
	if newCat.Period != "2025" {
		t.Fatalf("expected Period=2025, got %s", newCat.Period)
	}
	if newCat.Baseline.Available {
		t.Fatalf("expected Baseline unavailable for a brand-new category, got %+v", newCat.Baseline)
	}
	if !newCat.Observed.Available || newCat.Observed.Value != 25_000 {
		t.Fatalf("expected Observed=25000, got %+v", newCat.Observed)
	}
}

// TestCalculate_ReportedZeroIsNotDisappeared is a regression test: an
// explicitly-reported $0.00 NormalizedItem (e.g. "no ad spend this quarter,"
// but the ingestion pipeline still emits the line item) must NOT be treated
// as an absent account by RuleAccountDisappearedReappeared — only a period
// with no NormalizedItem at all is "disappeared," per that rule's own doc
// comment.
func TestCalculate_ReportedZeroIsNotDisappeared(t *testing.T) {
	ds := datasetOf(
		item(financial.CodeOpexMarketing, "2022", 10_000),
		item(financial.CodeOpexMarketing, "2023", 0), // explicitly reported $0, not absent
		item(financial.CodeOpexMarketing, "2024", 9_500),
		item(financial.CodeOpexMarketing, "2025", 10_200),
	)
	res := Calculate(Input{Dataset: ds, PeriodMeta: fourYearMeta()}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	if hasRule(res.Anomalies, RuleAccountDisappearedReappeared) {
		t.Fatalf("expected no RuleAccountDisappearedReappeared for a reported $0 period, got %+v", res.Anomalies)
	}
}

// TestCalculate_Reappeared covers RuleAccountDisappearedReappeared's
// "reappeared after a gap" branch specifically (a two-period gap).
func TestCalculate_Reappeared(t *testing.T) {
	res := Calculate(Input{Dataset: reappearedDataset(), PeriodMeta: fourYearMeta()}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}

	var disappearedCount, reappearedCount int
	for _, a := range res.Anomalies {
		if a.Code != RuleAccountDisappearedReappeared || a.Account != financial.CodeOpexRepairs {
			continue
		}
		if a.Observed.Available {
			reappearedCount++
			if a.Period != "2025" {
				t.Fatalf("expected reappeared Period=2025, got %s", a.Period)
			}
			if a.Baseline.Available {
				t.Fatal("expected reappeared Baseline unavailable (no single baseline across a gap)")
			}
		} else {
			disappearedCount++
			if a.Period != "2023" {
				t.Fatalf("expected disappeared Period=2023, got %s", a.Period)
			}
		}
	}
	if disappearedCount != 1 {
		t.Fatalf("expected exactly 1 disappeared anomaly, got %d", disappearedCount)
	}
	if reappearedCount != 1 {
		t.Fatalf("expected exactly 1 reappeared anomaly, got %d", reappearedCount)
	}
}

// TestCalculate_RepeatedUnusualValue proves the same nonzero amount
// recurring across at least RepeatedValueMinOccurrences distinct periods
// triggers RuleRepeatedUnusualValue with every referenced period listed.
func TestCalculate_RepeatedUnusualValue(t *testing.T) {
	res := Calculate(Input{Dataset: repeatedValueDataset(), PeriodMeta: fourYearMeta()}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}

	a, ok := findRuleForAccount(res.Anomalies, RuleRepeatedUnusualValue, financial.CodeOpexInsurance)
	if !ok {
		t.Fatalf("expected RuleRepeatedUnusualValue for CodeOpexInsurance, got %+v", res.Anomalies)
	}
	if len(a.RelatedPeriods) != 3 {
		t.Fatalf("expected 3 RelatedPeriods, got %v", a.RelatedPeriods)
	}
	want := []financial.Period{"2022", "2023", "2025"}
	for i, p := range want {
		if a.RelatedPeriods[i] != p {
			t.Fatalf("expected RelatedPeriods=%v, got %v", want, a.RelatedPeriods)
		}
	}
	if !a.Observed.Available || a.Observed.Value != 6_000 {
		t.Fatalf("expected Observed=6000, got %+v", a.Observed)
	}
}

// TestCalculate_RepeatedUnusualValue_NeedsNoPeriodMeta proves this rule
// still runs when PeriodMeta is absent (it needs no chronological order).
func TestCalculate_RepeatedUnusualValue_NeedsNoPeriodMeta(t *testing.T) {
	res := Calculate(Input{Dataset: repeatedValueDataset()}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	if !hasRule(res.Anomalies, RuleRepeatedUnusualValue) {
		t.Fatalf("expected RuleRepeatedUnusualValue without PeriodMeta, got %+v", res.Anomalies)
	}
}

// TestCalculate_DuplicateLikeAmounts proves the same amount recurring across
// two distinct suspicious accounts triggers RuleDuplicateLikeAmounts,
// referencing the other account via RelatedAccounts.
func TestCalculate_DuplicateLikeAmounts(t *testing.T) {
	res := Calculate(Input{Dataset: duplicateAmountsDataset(), PeriodMeta: fourYearMeta()}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}

	if !hasRule(res.Anomalies, RuleDuplicateLikeAmounts) {
		t.Fatalf("expected RuleDuplicateLikeAmounts, got %+v", res.Anomalies)
	}
	var found Anomaly
	for _, a := range res.Anomalies {
		if a.Code == RuleDuplicateLikeAmounts {
			found = a
			break
		}
	}
	if len(found.RelatedAccounts) != 1 {
		t.Fatalf("expected exactly 1 RelatedAccounts entry, got %v", found.RelatedAccounts)
	}
	if len(found.RelatedPeriods) != 2 {
		t.Fatalf("expected 2 RelatedPeriods (2024, 2025), got %v", found.RelatedPeriods)
	}
}

// TestCalculate_DuplicateLikeAmounts_SameAccountNotFlagged proves a value
// repeating on a SINGLE account across periods is RuleRepeatedUnusualValue's
// concern, not RuleDuplicateLikeAmounts' (which requires >= 2 distinct
// accounts).
func TestCalculate_DuplicateLikeAmounts_SameAccountNotFlagged(t *testing.T) {
	ds := datasetOf(
		item(financial.CodeOpexMarketing, "2024", 17_250),
		item(financial.CodeOpexMarketing, "2025", 17_250),
	)
	res := Calculate(Input{Dataset: ds, PeriodMeta: fourYearMeta()}, Options{})
	if hasRule(res.Anomalies, RuleDuplicateLikeAmounts) {
		t.Fatalf("expected no RuleDuplicateLikeAmounts for a single repeated account, got %+v", res.Anomalies)
	}
}

// TestCalculate_HighOwnerDiscretionaryShare proves an outsized owner-comp
// share of revenue in the most recent period triggers
// RuleHighOwnerDiscretionaryShare.
func TestCalculate_HighOwnerDiscretionaryShare(t *testing.T) {
	res := Calculate(Input{Dataset: ownerDiscretionaryDataset(), PeriodMeta: fourYearMeta()}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}

	a, ok := findRuleForAccount(res.Anomalies, RuleHighOwnerDiscretionaryShare, financial.CodeOpexOwnerComp)
	if !ok {
		t.Fatalf("expected RuleHighOwnerDiscretionaryShare, got %+v", res.Anomalies)
	}
	if a.Period != "2025" {
		t.Fatalf("expected most-recent-period=2025, got %s", a.Period)
	}
	if !a.Observed.Available || a.Observed.Value != 200_000 {
		t.Fatalf("expected Observed=200000, got %+v", a.Observed)
	}
}

// TestCalculate_HighOwnerDiscretionaryShare_ExtraCodes proves
// Input.DiscretionaryCodes extends the pool beyond CodeOpexOwnerComp.
func TestCalculate_HighOwnerDiscretionaryShare_ExtraCodes(t *testing.T) {
	ds := datasetOf(
		item(financial.CodeRevProduct, "2025", 1_000_000),
		item(financial.CodeOpexOwnerComp, "2025", 50_000),
		item(financial.CodeOpexVehicle, "2025", 120_000),
	)
	meta := map[financial.Period]PeriodInfo{"2025": {Type: PeriodTypeFiscalYear, FiscalYear: 2025}}

	withoutExtra := Calculate(Input{Dataset: ds, PeriodMeta: meta}, Options{})
	if hasRule(withoutExtra.Anomalies, RuleHighOwnerDiscretionaryShare) {
		t.Fatalf("expected no trigger from owner comp alone (5%%), got %+v", withoutExtra.Anomalies)
	}

	withExtra := Calculate(Input{
		Dataset:            ds,
		PeriodMeta:         meta,
		DiscretionaryCodes: []financial.Code{financial.CodeOpexVehicle},
	}, Options{})
	a, ok := findRuleForAccount(withExtra.Anomalies, RuleHighOwnerDiscretionaryShare, financial.CodeOpexOwnerComp)
	if !ok {
		t.Fatalf("expected RuleHighOwnerDiscretionaryShare once CodeOpexVehicle is included, got %+v", withExtra.Anomalies)
	}
	if a.Observed.Value != 170_000 {
		t.Fatalf("expected Observed=170000 (owner comp + vehicle), got %+v", a.Observed)
	}
}

// TestCalculate_HighOwnerDiscretionaryShare_SkipsZeroRevenuePeriod is a
// regression test: when the chronologically most recent period has an
// explicitly reported $0 Total Revenue, mostRecentAvailableRevenue must skip
// it and fall back to the next-most-recent period with genuine revenue —
// not silently give up and skip the rule entirely, since a $0 period
// shouldn't hide a real anomaly in an earlier period.
func TestCalculate_HighOwnerDiscretionaryShare_SkipsZeroRevenuePeriod(t *testing.T) {
	ds := datasetOf(
		item(financial.CodeRevProduct, "2024", 1_000_000),
		item(financial.CodeOpexOwnerComp, "2024", 500_000), // 50% share, well past the 15% default

		item(financial.CodeRevProduct, "2025", 0), // explicitly reported $0 revenue
		item(financial.CodeOpexOwnerComp, "2025", 10_000),
	)
	meta := map[financial.Period]PeriodInfo{
		"2024": {Type: PeriodTypeFiscalYear, FiscalYear: 2024},
		"2025": {Type: PeriodTypeFiscalYear, FiscalYear: 2025},
	}
	res := Calculate(Input{Dataset: ds, PeriodMeta: meta}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}

	a, ok := findRuleForAccount(res.Anomalies, RuleHighOwnerDiscretionaryShare, financial.CodeOpexOwnerComp)
	if !ok {
		t.Fatalf("expected RuleHighOwnerDiscretionaryShare to fall back to 2024 despite 2025's $0 revenue, got %+v", res.Anomalies)
	}
	if a.Period != "2024" {
		t.Fatalf("expected Period=2024 (the most recent period with nonzero revenue), got %s", a.Period)
	}
}

// TestCalculate_UnexpectedNegativeAmount proves negative revenue/expense
// amounts trigger RuleUnexpectedNegativeAmount, and that this rule needs no
// PeriodMeta (single-period rule).
func TestCalculate_UnexpectedNegativeAmount(t *testing.T) {
	res := Calculate(Input{Dataset: unexpectedNegativeDataset()}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}

	if countRule(res.Anomalies, RuleUnexpectedNegativeAmount) != 2 {
		t.Fatalf("expected 2 RuleUnexpectedNegativeAmount anomalies (revenue + expense), got %+v", res.Anomalies)
	}
	a, ok := findRuleForAccount(res.Anomalies, RuleUnexpectedNegativeAmount, financial.CodeRevProduct)
	if !ok {
		t.Fatal("expected RuleUnexpectedNegativeAmount for CodeRevProduct")
	}
	if a.Severity != AnomalySeverityCritical {
		t.Fatalf("expected AnomalySeverityCritical for unexpected negative revenue, got %s", a.Severity)
	}
	if a.Baseline.Available {
		t.Fatalf("expected Baseline unavailable for a single-period rule, got %+v", a.Baseline)
	}
}

// TestCalculate_AccountGroups proves AccountGroups labels Anomaly.Group
// without changing which anomalies are detected.
func TestCalculate_AccountGroups(t *testing.T) {
	groups := []AccountGroup{
		{Name: "Marketing & Advertising", Codes: []financial.Code{financial.CodeOpexMarketing}},
	}
	withoutGroups := Calculate(Input{Dataset: spikeDataset(), PeriodMeta: fourYearMeta()}, Options{})
	withGroups := Calculate(Input{Dataset: spikeDataset(), PeriodMeta: fourYearMeta(), AccountGroups: groups}, Options{})

	if len(withoutGroups.Anomalies) != len(withGroups.Anomalies) {
		t.Fatalf("expected AccountGroups to not change anomaly count: %d vs %d", len(withoutGroups.Anomalies), len(withGroups.Anomalies))
	}

	a, ok := findRuleForAccount(withGroups.Anomalies, RuleAbsoluteAmountSpike, financial.CodeOpexMarketing)
	if !ok {
		t.Fatal("expected RuleAbsoluteAmountSpike for CodeOpexMarketing")
	}
	if a.Group != "Marketing & Advertising" {
		t.Fatalf("expected Group=%q, got %q", "Marketing & Advertising", a.Group)
	}

	b, _ := findRuleForAccount(withoutGroups.Anomalies, RuleAbsoluteAmountSpike, financial.CodeOpexMarketing)
	if b.Group != "" {
		t.Fatalf("expected empty Group without AccountGroups, got %q", b.Group)
	}
}

// TestCalculate_NoPeriodMeta proves period-over-period/most-recent-period
// rules are skipped (with an advisory Issue) when PeriodMeta is absent,
// while order-independent rules still run.
func TestCalculate_NoPeriodMeta(t *testing.T) {
	res := Calculate(Input{Dataset: spikeDataset()}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	if hasRule(res.Anomalies, RuleAbsoluteAmountSpike) {
		t.Fatal("expected no RuleAbsoluteAmountSpike without PeriodMeta")
	}
	found := false
	for _, w := range res.Warnings {
		if w.Code == IssueNoPeriodMeta {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected IssueNoPeriodMeta warning, got %+v", res.Warnings)
	}
}

// TestCalculate_PeriodMissingFromMeta proves a partially-covered PeriodMeta
// disables ordering entirely rather than guessing a partial sort.
func TestCalculate_PeriodMissingFromMeta(t *testing.T) {
	partial := map[financial.Period]PeriodInfo{
		"2022": {Type: PeriodTypeFiscalYear, FiscalYear: 2022},
		"2023": {Type: PeriodTypeFiscalYear, FiscalYear: 2023},
		// 2024/2025 missing.
	}
	res := Calculate(Input{Dataset: spikeDataset(), PeriodMeta: partial}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	if hasRule(res.Anomalies, RuleAbsoluteAmountSpike) {
		t.Fatal("expected no RuleAbsoluteAmountSpike with a partial PeriodMeta")
	}
	found := false
	for _, w := range res.Warnings {
		if w.Code == IssuePeriodMissingFromMeta {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected IssuePeriodMissingFromMeta warning, got %+v", res.Warnings)
	}
}

// TestCalculate_SortedAnomalies proves Anomalies are ordered by Period, then
// RuleCode declaration order, then Account.
func TestCalculate_SortedAnomalies(t *testing.T) {
	res := Calculate(Input{Dataset: missingPeriodDataset(), PeriodMeta: fourYearMeta()}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	for i := 1; i < len(res.Anomalies); i++ {
		prev, cur := res.Anomalies[i-1], res.Anomalies[i]
		if prev.Period > cur.Period {
			t.Fatalf("Anomalies not sorted by Period at index %d: %s > %s", i, prev.Period, cur.Period)
		}
		if prev.Period == cur.Period && ruleOrder[prev.Code] > ruleOrder[cur.Code] {
			t.Fatalf("Anomalies not sorted by RuleCode within Period=%s at index %d", cur.Period, i)
		}
	}
}

// TestCalculate_Summary proves Summary.Total/ByRule/BySeverity/ReviewRecommended
// are consistent with Anomalies.
func TestCalculate_Summary(t *testing.T) {
	res := Calculate(Input{Dataset: spikeDataset(), PeriodMeta: fourYearMeta()}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	if res.Summary.Total != len(res.Anomalies) {
		t.Fatalf("expected Summary.Total=%d, got %d", len(res.Anomalies), res.Summary.Total)
	}
	if !res.Summary.ReviewRecommended {
		t.Fatal("expected ReviewRecommended=true when anomalies exist")
	}
	var sumByRule int
	for _, rc := range res.Summary.ByRule {
		sumByRule += rc.Count
	}
	if sumByRule != res.Summary.Total {
		t.Fatalf("expected ByRule counts to sum to Total, got %d vs %d", sumByRule, res.Summary.Total)
	}
	var sumBySeverity int
	for _, sc := range res.Summary.BySeverity {
		sumBySeverity += sc.Count
	}
	if sumBySeverity != res.Summary.Total {
		t.Fatalf("expected BySeverity counts to sum to Total, got %d vs %d", sumBySeverity, res.Summary.Total)
	}
}

// TestCalculate_CustomThresholds proves a caller-supplied Thresholds value
// changes trigger points and is echoed on Result.
func TestCalculate_CustomThresholds(t *testing.T) {
	custom := DefaultThresholds()
	custom.AbsoluteAmountSpike = 1_000_000 // raise well above the fixture's spike
	custom.PercentageChangeSpike = 10.0    // raise well above 1000%

	res := Calculate(Input{Dataset: spikeDataset(), PeriodMeta: fourYearMeta()}, Options{Thresholds: custom})
	if hasRule(res.Anomalies, RuleAbsoluteAmountSpike) {
		t.Fatal("expected no RuleAbsoluteAmountSpike once the threshold is raised above the spike")
	}
	if res.Thresholds.AbsoluteAmountSpike != 1_000_000 {
		t.Fatalf("expected Thresholds echoed, got %+v", res.Thresholds)
	}
}
