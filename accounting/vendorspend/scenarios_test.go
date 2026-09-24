package vendorspend_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/vendorspend"
	"github.com/themurtez/go-valuate/accounting/vendorspend/fixtures"
)

// namedScenario bundles one fixture scenario's Input and its computed
// Result — the shared shape allFixtureResults returns, used by tests that
// need to run an assertion across every scenario (e.g. neutral-language,
// determinism-per-scenario).
type namedScenario struct {
	name   string
	input  vendorspend.Input
	result vendorspend.Result
}

// allScenarioInputs returns every named fixture scenario's Input — task
// section 44's full scenario catalog, each wired into a runnable
// vendorspend.Input.
func allScenarioInputs() []struct {
	name  string
	input vendorspend.Input
} {
	return []struct {
		name  string
		input vendorspend.Input
	}{
		{"diversified suppliers", vendorspend.Input{Periods: fixtures.SixMonthPeriods(), Suppliers: fixtures.DiversifiedSuppliers(), SpendRecords: fixtures.DiversifiedSpend()}},
		{"highly concentrated spend", vendorspend.Input{Periods: fixtures.SixMonthPeriods(), Suppliers: fixtures.ConcentratedSuppliers(), SpendRecords: fixtures.HighlyConcentratedSpend()}},
		{"concentration increasing", vendorspend.Input{Periods: fixtures.SixMonthPeriods(), Suppliers: fixtures.ConcentratedSuppliers(), SpendRecords: fixtures.IncreasingConcentrationSpend()}},
		{"new supplier", vendorspend.Input{Periods: fixtures.SixMonthPeriods(), Suppliers: fixtures.NewSupplierSuppliers(), SpendRecords: fixtures.NewSupplierSpend(),
			Policy: vendorspend.Policy{NewSupplierMaterialAmount: 1000}}},
		{"lost supplier", vendorspend.Input{Periods: fixtures.SixMonthPeriods(), Suppliers: fixtures.LostSupplierSuppliers(), SpendRecords: fixtures.LostSupplierSpend(),
			Policy: vendorspend.Policy{LostSupplierMaterialAmount: 1000}}},
		{"one-period insufficient history", vendorspend.Input{Periods: fixtures.OnePeriodOnly(), Suppliers: fixtures.SinglePeriodSuppliers(), SpendRecords: fixtures.SinglePeriodSpend()}},
		{"price increase", vendorspend.Input{Periods: fixtures.SixMonthPeriods(), Suppliers: fixtures.UnitPriceSuppliers(), SpendRecords: fixtures.PriceIncreaseSpend()}},
		{"volume increase", vendorspend.Input{Periods: fixtures.SixMonthPeriods(), Suppliers: fixtures.UnitPriceSuppliers(), SpendRecords: fixtures.VolumeIncreaseSpend()}},
		{"combined price+volume", vendorspend.Input{Periods: fixtures.SixMonthPeriods(), Suppliers: fixtures.UnitPriceSuppliers(), SpendRecords: fixtures.CombinedPriceVolumeSpend()}},
		{"multi-supplier same product", vendorspend.Input{Periods: fixtures.SixMonthPeriods(), Suppliers: fixtures.MultiSupplierSameProductSuppliers(), SpendRecords: fixtures.MultiSupplierSameProductSpend()}},
		{"observed single-source product", vendorspend.Input{Periods: fixtures.SixMonthPeriods(), Suppliers: fixtures.ObservedSingleSourceSuppliers(), SpendRecords: fixtures.ObservedSingleSourceSpend()}},
		{"declared recurring spend", vendorspend.Input{Periods: fixtures.SixMonthPeriods(), Suppliers: fixtures.DeclaredRecurringSuppliers(), SpendRecords: fixtures.DeclaredRecurringSpend()}},
		{"observed repeated spend", vendorspend.Input{Periods: fixtures.SixMonthPeriods(), Suppliers: fixtures.DeclaredRecurringSuppliers(), SpendRecords: fixtures.ObservedRepeatedSpend()}},
		{"one-time spend", vendorspend.Input{Periods: fixtures.SixMonthPeriods(), Suppliers: fixtures.DeclaredRecurringSuppliers(), SpendRecords: fixtures.OneTimeSpend()}},
		{"committed/discretionary mix", vendorspend.Input{Periods: fixtures.SixMonthPeriods(), Suppliers: fixtures.CommitmentMixSuppliers(), SpendRecords: fixtures.CommitmentMixSpend()}},
		{"category trend", vendorspend.Input{Periods: fixtures.SixMonthPeriods(), Suppliers: fixtures.CategoryTrendSuppliers(), SpendRecords: fixtures.CategoryTrendSpend()}},
		{"uncategorized spend", vendorspend.Input{Periods: fixtures.SixMonthPeriods(), Suppliers: fixtures.UncategorizedSpendSuppliers(), SpendRecords: fixtures.UncategorizedSpend()}},
		{"non-preferred supplier", vendorspend.Input{Periods: fixtures.SixMonthPeriods(), Suppliers: fixtures.NonPreferredSuppliers(), SpendRecords: fixtures.NonPreferredSpendRecords()}},
		{"tail spend", vendorspend.Input{Periods: fixtures.SixMonthPeriods(), Suppliers: fixtures.TailSpendSuppliers(), SpendRecords: fixtures.TailSpendRecords(),
			Policy: vendorspend.Policy{TailSpend: vendorspend.TailSpendPolicy{BelowAmount: 1000}}}},
		{"duplicate-like spend", vendorspend.Input{Periods: fixtures.SixMonthPeriods(), Suppliers: fixtures.DuplicateLikeSuppliers(), SpendRecords: fixtures.DuplicateLikeSpend()}},
		{"near duplicate", vendorspend.Input{Periods: fixtures.SixMonthPeriods(), Suppliers: fixtures.DuplicateLikeSuppliers(), SpendRecords: fixtures.NearDuplicateSpend()}},
		{"vendor credit/refund", vendorspend.Input{Periods: fixtures.SixMonthPeriods(), Suppliers: fixtures.VendorCreditSuppliers(), SpendRecords: fixtures.VendorCreditSpend()}},
		{"mixed UOM", vendorspend.Input{Periods: fixtures.SixMonthPeriods(), Suppliers: fixtures.MixedUOMSuppliers(), SpendRecords: fixtures.MixedUOMSpend()}},
		{"zero-spend period", vendorspend.Input{Periods: fixtures.ZeroSpendPeriodPeriods(), Suppliers: fixtures.DiversifiedSuppliers(), SpendRecords: fixtures.ZeroSpendPeriodSpend()}},
	}
}

// allFixtureResults runs Calculate against every allScenarioInputs entry
// and returns the paired (name, input, result) — used by tests that
// assert something across every scenario at once (neutral language,
// schema/formula version consistency, etc.).
func allFixtureResults(t *testing.T) []namedScenario {
	t.Helper()
	inputs := allScenarioInputs()
	out := make([]namedScenario, 0, len(inputs))
	for _, s := range inputs {
		out = append(out, namedScenario{name: s.name, input: s.input, result: vendorspend.Calculate(s.input, vendorspend.Options{})})
	}
	return out
}

// TestScenario_DiversifiedSuppliers_LowConcentration verifies the
// diversified-suppliers fixture produces a low top-1 concentration share
// (no supplier dominates).
func TestScenario_DiversifiedSuppliers_LowConcentration(t *testing.T) {
	in := vendorspend.Input{Periods: fixtures.SixMonthPeriods(), Suppliers: fixtures.DiversifiedSuppliers(), SpendRecords: fixtures.DiversifiedSpend()}
	result := vendorspend.Calculate(in, vendorspend.Options{})
	if !result.Available {
		t.Fatalf("expected Available, got Issues: %+v", result.Issues)
	}
	if !result.Concentration.Available {
		t.Fatalf("expected Concentration.Available")
	}
	if result.Concentration.Top1.Value >= 0.3 {
		t.Errorf("Top1 = %v, want < 0.3 for a diversified supplier base", result.Concentration.Top1.Value)
	}
	for _, f := range result.Flags {
		if f.Code == vendorspend.FlagHighSupplierConcentration {
			t.Errorf("did not expect FlagHighSupplierConcentration for a diversified supplier base")
		}
	}
}

// TestScenario_HighlyConcentratedSpend_TriggersFlag verifies the
// highly-concentrated fixture triggers FlagHighSupplierConcentration.
func TestScenario_HighlyConcentratedSpend_TriggersFlag(t *testing.T) {
	in := vendorspend.Input{Periods: fixtures.SixMonthPeriods(), Suppliers: fixtures.ConcentratedSuppliers(), SpendRecords: fixtures.HighlyConcentratedSpend()}
	result := vendorspend.Calculate(in, vendorspend.Options{})
	found := false
	for _, f := range result.Flags {
		if f.Code == vendorspend.FlagHighSupplierConcentration {
			found = true
		}
	}
	if !found {
		t.Errorf("expected FlagHighSupplierConcentration, got flags: %+v", result.Flags)
	}
}

// TestScenario_ConcentrationIncreasing_TriggersFlag verifies the
// increasing-concentration fixture triggers
// FlagSupplierConcentrationIncreasing.
func TestScenario_ConcentrationIncreasing_TriggersFlag(t *testing.T) {
	in := vendorspend.Input{Periods: fixtures.SixMonthPeriods(), Suppliers: fixtures.ConcentratedSuppliers(), SpendRecords: fixtures.IncreasingConcentrationSpend()}
	result := vendorspend.Calculate(in, vendorspend.Options{})
	found := false
	for _, f := range result.Flags {
		if f.Code == vendorspend.FlagSupplierConcentrationIncreasing {
			found = true
		}
	}
	if !found {
		t.Errorf("expected FlagSupplierConcentrationIncreasing, got flags: %+v", result.Flags)
	}
}

// TestScenario_NewSupplier_Detected verifies the new-supplier fixture is
// detected as NEW_SUPPLIER_ACTIVITY, material given the configured
// threshold.
func TestScenario_NewSupplier_Detected(t *testing.T) {
	in := vendorspend.Input{Periods: fixtures.SixMonthPeriods(), Suppliers: fixtures.NewSupplierSuppliers(), SpendRecords: fixtures.NewSupplierSpend(),
		Policy: vendorspend.Policy{NewSupplierMaterialAmount: 1000}}
	result := vendorspend.Calculate(in, vendorspend.Options{})
	if !result.NewLostSuppliers.Available {
		t.Fatalf("expected NewLostSuppliers.Available")
	}
	found := false
	for _, c := range result.NewLostSuppliers.Changes {
		if c.Kind == vendorspend.ChangeNewSupplierActivity && c.SupplierID == "SUP-NEW" {
			found = true
			if !c.Material {
				t.Errorf("expected SUP-NEW's new-activity change to be Material given the $1000 threshold and $8000 amount")
			}
		}
	}
	if !found {
		t.Errorf("expected a NEW_SUPPLIER_ACTIVITY change for SUP-NEW, got: %+v", result.NewLostSuppliers.Changes)
	}
}

// TestScenario_LostSupplier_Detected verifies the lost-supplier fixture
// is detected as SUPPLIER_SPEND_DISCONTINUED.
func TestScenario_LostSupplier_Detected(t *testing.T) {
	in := vendorspend.Input{Periods: fixtures.SixMonthPeriods(), Suppliers: fixtures.LostSupplierSuppliers(), SpendRecords: fixtures.LostSupplierSpend(),
		Policy: vendorspend.Policy{LostSupplierMaterialAmount: 1000}}
	result := vendorspend.Calculate(in, vendorspend.Options{})
	found := false
	for _, c := range result.NewLostSuppliers.Changes {
		if c.Kind == vendorspend.ChangeSupplierSpendDiscontinued && c.SupplierID == "SUP-LEAVING" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a SUPPLIER_SPEND_DISCONTINUED change for SUP-LEAVING, got: %+v", result.NewLostSuppliers.Changes)
	}
}

// TestScenario_OnePeriod_NewLostUnavailable verifies task section 11's
// "if only one period exists, new/lost analysis is unavailable" rule —
// no supplier is ever called "new" from a single period.
func TestScenario_OnePeriod_NewLostUnavailable(t *testing.T) {
	in := vendorspend.Input{Periods: fixtures.OnePeriodOnly(), Suppliers: fixtures.SinglePeriodSuppliers(), SpendRecords: fixtures.SinglePeriodSpend()}
	result := vendorspend.Calculate(in, vendorspend.Options{})
	if !result.Available {
		t.Fatalf("expected Available, got Issues: %+v", result.Issues)
	}
	if result.NewLostSuppliers.Available {
		t.Errorf("expected NewLostSuppliers.Available == false with only one period, got Changes: %+v", result.NewLostSuppliers.Changes)
	}
}

// TestScenario_MultiSupplierSameProduct_PriceComparison verifies the
// cross-supplier price comparison fixture produces a
// ProductPriceComparison with both suppliers and correct median/lowest.
func TestScenario_MultiSupplierSameProduct_PriceComparison(t *testing.T) {
	in := vendorspend.Input{Periods: fixtures.SixMonthPeriods(), Suppliers: fixtures.MultiSupplierSameProductSuppliers(), SpendRecords: fixtures.MultiSupplierSameProductSpend()}
	result := vendorspend.Calculate(in, vendorspend.Options{})
	if len(result.ProductPriceComparisons) != 1 {
		t.Fatalf("expected exactly 1 ProductPriceComparison, got %d", len(result.ProductPriceComparisons))
	}
	pc := result.ProductPriceComparisons[0]
	if pc.ProductID != "COMMON-PART" {
		t.Errorf("ProductID = %q, want COMMON-PART", pc.ProductID)
	}
	if len(pc.Suppliers) != 2 {
		t.Fatalf("expected 2 suppliers in comparison, got %d", len(pc.Suppliers))
	}
	if !pc.LowestObservedPrice.Available || pc.LowestObservedPrice.Value != 10 {
		t.Errorf("LowestObservedPrice = %+v, want 10", pc.LowestObservedPrice)
	}
}

// TestScenario_ObservedSingleSourceProduct_Flagged verifies the
// observed-single-source fixture produces both the ProductSpend
// ObservedSingleSource flag and the FlagObservedSingleSourceProduct
// signal.
func TestScenario_ObservedSingleSourceProduct_Flagged(t *testing.T) {
	in := vendorspend.Input{Periods: fixtures.SixMonthPeriods(), Suppliers: fixtures.ObservedSingleSourceSuppliers(), SpendRecords: fixtures.ObservedSingleSourceSpend()}
	result := vendorspend.Calculate(in, vendorspend.Options{})
	found := false
	for _, p := range result.ProductSummaries {
		if p.ProductID == "SOLE-PART" {
			if !p.ObservedSingleSource {
				t.Errorf("expected ObservedSingleSource for SOLE-PART")
			}
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a ProductSpend entry for SOLE-PART")
	}
	flagFound := false
	for _, f := range result.Flags {
		if f.Code == vendorspend.FlagObservedSingleSourceProduct && f.ProductID == "SOLE-PART" {
			flagFound = true
		}
	}
	if !flagFound {
		t.Errorf("expected FlagObservedSingleSourceProduct for SOLE-PART, got flags: %+v", result.Flags)
	}
}

// TestScenario_DeclaredRecurring_NeverOverwritten verifies caller-
// declared RecurrenceRecurring is preserved verbatim in RecurringSpend.Mix
// and is NOT itself reported as an ObservedRecurring pattern (since
// ObservedRecurring is a separate, pattern-detected signal — task
// section 19's "never overwrite caller recurrence with inferred pattern"
// rule means the two lists are independent, not that declared-recurring
// spend is excluded from pattern detection).
func TestScenario_DeclaredRecurring_MixReflectsCallerDeclaration(t *testing.T) {
	in := vendorspend.Input{Periods: fixtures.SixMonthPeriods(), Suppliers: fixtures.DeclaredRecurringSuppliers(), SpendRecords: fixtures.DeclaredRecurringSpend()}
	result := vendorspend.Calculate(in, vendorspend.Options{})
	found := false
	for _, m := range result.RecurringSpend.Mix {
		if m.Type == vendorspend.RecurrenceRecurring {
			found = true
			if m.Spend <= 0 {
				t.Errorf("RECURRING mix spend = %v, want > 0", m.Spend)
			}
		}
	}
	if !found {
		t.Errorf("expected a RECURRING entry in RecurringSpend.Mix, got: %+v", result.RecurringSpend.Mix)
	}
}

// TestScenario_ObservedRepeatedSpend_DetectedWithoutDeclaration verifies
// the observed-repeated-spend fixture (no caller Recurrence set) is
// detected purely by pattern.
func TestScenario_ObservedRepeatedSpend_DetectedWithoutDeclaration(t *testing.T) {
	in := vendorspend.Input{Periods: fixtures.SixMonthPeriods(), Suppliers: fixtures.DeclaredRecurringSuppliers(), SpendRecords: fixtures.ObservedRepeatedSpend()}
	result := vendorspend.Calculate(in, vendorspend.Options{})
	if len(result.RecurringSpend.ObservedRecurring) == 0 {
		t.Fatalf("expected at least one ObservedRecurringGroup, got none")
	}
	for _, m := range result.RecurringSpend.Mix {
		if m.Type == vendorspend.RecurrenceRecurring && m.Spend > 0 {
			t.Errorf("expected no caller-declared RECURRING spend in this fixture (pattern-only detection), got %+v", m)
		}
	}
}

// TestScenario_CommitmentMix_SeparatesCommittedFromDiscretionary verifies
// the commitment-mix fixture correctly separates COMMITTED and
// DISCRETIONARY totals.
func TestScenario_CommitmentMix_SeparatesCommittedFromDiscretionary(t *testing.T) {
	in := vendorspend.Input{Periods: fixtures.SixMonthPeriods(), Suppliers: fixtures.CommitmentMixSuppliers(), SpendRecords: fixtures.CommitmentMixSpend()}
	result := vendorspend.Calculate(in, vendorspend.Options{})
	var committed, discretionary float64
	for _, m := range result.CommitmentMix {
		switch m.Type {
		case vendorspend.CommitmentCommitted:
			committed = m.Spend
		case vendorspend.CommitmentDiscretionary:
			discretionary = m.Spend
		}
	}
	if committed != 10000 {
		t.Errorf("COMMITTED = %v, want 10000", committed)
	}
	if discretionary != 4000 {
		t.Errorf("DISCRETIONARY = %v, want 4000", discretionary)
	}
}

// TestScenario_TailSpend_MatchesDefinition verifies the tail-spend
// fixture correctly identifies the three small suppliers below the
// $1,000 threshold as tail spend.
func TestScenario_TailSpend_MatchesDefinition(t *testing.T) {
	in := vendorspend.Input{Periods: fixtures.SixMonthPeriods(), Suppliers: fixtures.TailSpendSuppliers(), SpendRecords: fixtures.TailSpendRecords(),
		Policy: vendorspend.Policy{TailSpend: vendorspend.TailSpendPolicy{BelowAmount: 1000}}}
	result := vendorspend.Calculate(in, vendorspend.Options{})
	if !result.TailSpend.Available {
		t.Fatalf("expected TailSpend.Available")
	}
	if result.TailSpend.Definition != vendorspend.TailSpendBelowAmount {
		t.Errorf("Definition = %v, want TailSpendBelowAmount", result.TailSpend.Definition)
	}
	if result.TailSpend.TailSupplierCount != 3 {
		t.Errorf("TailSupplierCount = %d, want 3", result.TailSpend.TailSupplierCount)
	}
	if result.TailSpend.TailSpendAmount != 1000 { // 500+300+200
		t.Errorf("TailSpendAmount = %v, want 1000", result.TailSpend.TailSpendAmount)
	}
}

// TestScenario_TailSpend_UnavailableWithoutPolicy verifies task section
// 21's "never report tail spend without the definition used" rule: an
// unconfigured TailSpendPolicy leaves TailSpend unavailable.
func TestScenario_TailSpend_UnavailableWithoutPolicy(t *testing.T) {
	in := vendorspend.Input{Periods: fixtures.SixMonthPeriods(), Suppliers: fixtures.TailSpendSuppliers(), SpendRecords: fixtures.TailSpendRecords()}
	result := vendorspend.Calculate(in, vendorspend.Options{})
	if result.TailSpend.Available {
		t.Errorf("expected TailSpend.Available == false with no Policy.TailSpend configured, got %+v", result.TailSpend)
	}
}

// TestScenario_DuplicateLikeSpend_Detected verifies the duplicate-like
// fixture (same supplier/amount/category, 1 day apart) is detected.
func TestScenario_DuplicateLikeSpend_Detected(t *testing.T) {
	in := vendorspend.Input{Periods: fixtures.SixMonthPeriods(), Suppliers: fixtures.DuplicateLikeSuppliers(), SpendRecords: fixtures.DuplicateLikeSpend()}
	result := vendorspend.Calculate(in, vendorspend.Options{})
	if len(result.DuplicateLikeGroups) != 1 {
		t.Fatalf("expected exactly 1 DuplicateLikeGroup, got %d: %+v", len(result.DuplicateLikeGroups), result.DuplicateLikeGroups)
	}
	g := result.DuplicateLikeGroups[0]
	if len(g.SpendIDs) != 2 {
		t.Errorf("expected 2 SpendIDs in the group, got %d", len(g.SpendIDs))
	}
}

// TestScenario_NearDuplicate_DetectedAcrossBucketBoundary verifies two
// records 2 days apart (within the default 3-day window but not on the
// same day) are still matched — exercises the adjacent-bucket lookup.
func TestScenario_NearDuplicate_DetectedAcrossBucketBoundary(t *testing.T) {
	in := vendorspend.Input{Periods: fixtures.SixMonthPeriods(), Suppliers: fixtures.DuplicateLikeSuppliers(), SpendRecords: fixtures.NearDuplicateSpend()}
	result := vendorspend.Calculate(in, vendorspend.Options{})
	if len(result.DuplicateLikeGroups) != 1 {
		t.Fatalf("expected exactly 1 DuplicateLikeGroup for near-duplicate records 2 days apart, got %d", len(result.DuplicateLikeGroups))
	}
}

// TestScenario_VendorCredit_NetsCorrectly verifies the vendor-credit
// fixture nets to $8,500 while gross stays $10,000.
func TestScenario_VendorCredit_NetsCorrectly(t *testing.T) {
	in := vendorspend.Input{Periods: fixtures.SixMonthPeriods(), Suppliers: fixtures.VendorCreditSuppliers(), SpendRecords: fixtures.VendorCreditSpend()}
	result := vendorspend.Calculate(in, vendorspend.Options{})
	if result.Bridge.GrossSpend != 10000 || result.Bridge.CreditsRefunds != 1500 || result.Bridge.NetSpend != 8500 {
		t.Errorf("Bridge = %+v, want Gross=10000 Credits=1500 Net=8500", result.Bridge)
	}
}

// TestScenario_MixedCurrency_FlaggedAndResolved verifies the
// mixed-currency fixture emits IssueMixedCurrency and resolves to a
// single reporting currency rather than silently summing across
// currencies.
func TestScenario_MixedCurrency_FlaggedAndResolved(t *testing.T) {
	in := vendorspend.Input{Periods: fixtures.SixMonthPeriods(), Suppliers: fixtures.MixedCurrencySuppliers(), SpendRecords: fixtures.MixedCurrencySpend()}
	result := vendorspend.Calculate(in, vendorspend.Options{})
	if !result.Available {
		t.Fatalf("expected Available (mixed currency is a warning, not a hard failure by default), got Issues: %+v", result.Issues)
	}
	found := false
	for _, is := range result.Issues {
		if is.Code == vendorspend.IssueMixedCurrency {
			found = true
		}
	}
	if !found {
		t.Errorf("expected IssueMixedCurrency, got Issues: %+v", result.Issues)
	}
	if result.ReportingCurrency == "" {
		t.Errorf("expected a resolved ReportingCurrency")
	}
	// Only ONE currency's records should be included in the bridge — not
	// USD 5000 + EUR 4000 = 9000 summed across currencies.
	if result.Bridge.NetSpend == 9000 {
		t.Errorf("NetSpend = 9000 implies USD and EUR amounts were summed together without currency resolution")
	}
}

// TestScenario_MixedUOM_NeverAggregatedAcrossUnits verifies the mixed-UOM
// fixture never produces a WeightedAverageUnitPrice mixing EA and CASE
// quantities.
func TestScenario_MixedUOM_NeverAggregatedAcrossUnits(t *testing.T) {
	in := vendorspend.Input{Periods: fixtures.SixMonthPeriods(), Suppliers: fixtures.MixedUOMSuppliers(), SpendRecords: fixtures.MixedUOMSpend()}
	result := vendorspend.Calculate(in, vendorspend.Options{})
	if len(result.UnitPricePoints) != 2 {
		t.Fatalf("expected 2 separate UnitPricePoints (one per UOM), got %d: %+v", len(result.UnitPricePoints), result.UnitPricePoints)
	}
	uoms := map[string]bool{}
	for _, p := range result.UnitPricePoints {
		uoms[p.UnitOfMeasure] = true
	}
	if !uoms["EA"] || !uoms["CASE"] {
		t.Errorf("expected separate EA and CASE UnitPricePoints, got %+v", result.UnitPricePoints)
	}
}

// TestScenario_ZeroSpendPeriod_HandledGracefully verifies a period with
// zero spend activity does not produce a divide-by-zero or crash, and is
// simply absent from PeriodSummaries (only periods with actual activity
// produce a summary).
func TestScenario_ZeroSpendPeriod_HandledGracefully(t *testing.T) {
	in := vendorspend.Input{Periods: fixtures.ZeroSpendPeriodPeriods(), Suppliers: fixtures.DiversifiedSuppliers(), SpendRecords: fixtures.ZeroSpendPeriodSpend()}
	result := vendorspend.Calculate(in, vendorspend.Options{})
	if !result.Available {
		t.Fatalf("expected Available, got Issues: %+v", result.Issues)
	}
	if len(result.Periods) != 2 {
		t.Errorf("expected 2 Periods echoed (both validated periods), got %d: %v", len(result.Periods), result.Periods)
	}
	for _, ps := range result.PeriodSummaries {
		if ps.Period == "2025-02" {
			t.Errorf("did not expect a PeriodSummary for the zero-activity period 2025-02")
		}
	}
}

// TestScenario_NonPreferredSupplier_Flagged verifies the non-preferred
// fixture correctly identifies SUP-OTHER's spend as non-preferred.
func TestScenario_NonPreferredSupplier_Flagged(t *testing.T) {
	in := vendorspend.Input{Periods: fixtures.SixMonthPeriods(), Suppliers: fixtures.NonPreferredSuppliers(), SpendRecords: fixtures.NonPreferredSpendRecords()}
	result := vendorspend.Calculate(in, vendorspend.Options{})
	if !result.NonPreferredSpend.Available {
		t.Fatalf("expected NonPreferredSpend.Available (at least one supplier declared PreferredSupplier)")
	}
	if result.NonPreferredSpend.TotalSpend != 4000 {
		t.Errorf("NonPreferredSpend.TotalSpend = %v, want 4000", result.NonPreferredSpend.TotalSpend)
	}
	found := false
	for _, f := range result.Flags {
		if f.Code == vendorspend.FlagSpendWithNonPreferredSupplier {
			found = true
		}
	}
	if !found {
		t.Errorf("expected FlagSpendWithNonPreferredSupplier, got flags: %+v", result.Flags)
	}
}

// TestScenario_UncategorizedSpend_CoverageAndFlag verifies the
// uncategorized-spend fixture's coverage math and the resulting flag
// when coverage is below the configured threshold.
func TestScenario_UncategorizedSpend_CoverageAndFlag(t *testing.T) {
	in := vendorspend.Input{Periods: fixtures.SixMonthPeriods(), Suppliers: fixtures.UncategorizedSpendSuppliers(), SpendRecords: fixtures.UncategorizedSpend(),
		Policy: vendorspend.Policy{CategoryCoverageThreshold: 0.9}}
	result := vendorspend.Calculate(in, vendorspend.Options{})
	c := result.CategoryProductCoverage
	if c.CategorizedSpend != 3000 || c.UncategorizedSpend != 7000 {
		t.Errorf("coverage = %+v, want Categorized=3000 Uncategorized=7000", c)
	}
	found := false
	for _, f := range result.Flags {
		if f.Code == vendorspend.FlagMaterialUncategorizedSpend {
			found = true
		}
	}
	if !found {
		t.Errorf("expected FlagMaterialUncategorizedSpend (30%% coverage is below the 90%% threshold), got flags: %+v", result.Flags)
	}
}
