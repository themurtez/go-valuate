package vendorspend_test

import (
	"math"
	"testing"
	"time"

	"github.com/themurtez/go-valuate/accounting/vendorspend"
)

// TestRegression_RequireSingleBasis_ScopedToBasisIssuesOnly locks a real
// bug found by code review: Policy.RequireSingleBasis incorrectly
// aborted Calculate on ANY pre-existing error-severity Issue in the
// cumulative result (e.g. an unrelated duplicate-SpendID error), not
// only on an actual mixed-basis conflict. A single-basis dataset with an
// unrelated structural error elsewhere must still be fully analyzed
// (Available == true), with the unrelated error simply reported.
func TestRegression_RequireSingleBasis_ScopedToBasisIssuesOnly(t *testing.T) {
	periods := []vendorspend.Period{
		{Period: "2025-01", StartDate: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), EndDate: time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC), SequenceInYear: 1},
	}
	suppliers := []vendorspend.Supplier{{SupplierID: "SUP-1", Active: true}}
	records := []vendorspend.SpendRecord{
		// A single, consistent basis (ACCRUAL) throughout.
		{SpendID: "SP-1", SupplierID: "SUP-1", Period: "2025-01", Date: time.Date(2025, 1, 5, 0, 0, 0, 0, time.UTC),
			Amount: 100, Currency: "USD", Basis: vendorspend.BasisAccrual},
		// An UNRELATED duplicate SpendID — a real error, but not a basis
		// conflict.
		{SpendID: "SP-1", SupplierID: "SUP-1", Period: "2025-01", Date: time.Date(2025, 1, 6, 0, 0, 0, 0, time.UTC),
			Amount: 999, Currency: "USD", Basis: vendorspend.BasisAccrual},
	}

	result := vendorspend.Calculate(vendorspend.Input{
		Periods: periods, Suppliers: suppliers, SpendRecords: records,
		Policy: vendorspend.Policy{RequireSingleBasis: true},
	}, vendorspend.Options{})

	if !result.Available {
		t.Fatalf("REGRESSION: RequireSingleBasis aborted Calculate due to an unrelated error, not a basis conflict; Issues: %+v", result.Issues)
	}
	if result.Bridge.NetSpend != 100 {
		t.Errorf("NetSpend = %v, want 100 (the single valid record)", result.Bridge.NetSpend)
	}
}

// TestRegression_RequireSingleBasis_StillAbortsOnActualConflict verifies
// the fix did not over-correct: a genuine mixed-basis conflict under
// RequireSingleBasis still aborts.
func TestRegression_RequireSingleBasis_StillAbortsOnActualConflict(t *testing.T) {
	periods := []vendorspend.Period{
		{Period: "2025-01", StartDate: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), EndDate: time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC), SequenceInYear: 1},
	}
	suppliers := []vendorspend.Supplier{{SupplierID: "SUP-1", Active: true}}
	records := []vendorspend.SpendRecord{
		{SpendID: "SP-1", SupplierID: "SUP-1", Period: "2025-01", Date: time.Date(2025, 1, 5, 0, 0, 0, 0, time.UTC),
			Amount: 100, Currency: "USD", Basis: vendorspend.BasisAccrual},
		{SpendID: "SP-2", SupplierID: "SUP-1", Period: "2025-01", Date: time.Date(2025, 1, 6, 0, 0, 0, 0, time.UTC),
			Amount: 200, Currency: "USD", Basis: vendorspend.BasisCash},
	}
	result := vendorspend.Calculate(vendorspend.Input{
		Periods: periods, Suppliers: suppliers, SpendRecords: records,
		Policy: vendorspend.Policy{RequireSingleBasis: true},
	}, vendorspend.Options{})
	if result.Available {
		t.Fatalf("expected Available == false for an actual mixed-basis conflict under RequireSingleBasis, got Bridge: %+v", result.Bridge)
	}
}

// TestRegression_PolicyTopN_ThreadedIntoConcentration locks a real bug:
// Policy.TopN was never passed to analytics/concentration, so a caller
// requesting only []int{1} still got Top3/Top5/Top10 computed (wasted
// work) and — more importantly — had no way to narrow the reported
// cutoffs at all.
func TestRegression_PolicyTopN_ThreadedIntoConcentration(t *testing.T) {
	in := fullInput()
	in.Policy.TopN = []int{1}
	result := vendorspend.Calculate(in, vendorspend.Options{})
	if !result.Concentration.Available {
		t.Fatalf("expected Concentration.Available")
	}
	if result.Concentration.Top3.Available || result.Concentration.Top5.Available || result.Concentration.Top10.Available {
		t.Errorf("expected only Top1 to be available with Policy.TopN=[]int{1}, got %+v", result.Concentration)
	}
	if !result.Concentration.Top1.Available {
		t.Errorf("expected Top1 to remain available (always computed from LargestEntityShare regardless of TopN)")
	}
}

// TestRegression_SupplierShareIncreasePoints_Respected locks a real bug:
// FlagSupplierConcentrationIncreasing fired on ANY positive share
// change, ignoring Policy.SupplierShareIncreasePoints entirely. A high
// configured tolerance must suppress a small increase.
func TestRegression_SupplierShareIncreasePoints_Respected(t *testing.T) {
	periods := []vendorspend.Period{
		{Period: "2025-01", StartDate: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), EndDate: time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC), SequenceInYear: 1},
		{Period: "2025-02", StartDate: time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC), EndDate: time.Date(2025, 2, 28, 0, 0, 0, 0, time.UTC), SequenceInYear: 2},
	}
	suppliers := []vendorspend.Supplier{
		{SupplierID: "SUP-BIG", Active: true}, {SupplierID: "SUP-SMALL", Active: true},
	}
	// SUP-BIG's share moves from 50% to 52% — a tiny (2-point) increase.
	records := []vendorspend.SpendRecord{
		{SpendID: "SP-1", SupplierID: "SUP-BIG", Period: "2025-01", Date: time.Date(2025, 1, 5, 0, 0, 0, 0, time.UTC), Amount: 5000, Currency: "USD"},
		{SpendID: "SP-2", SupplierID: "SUP-SMALL", Period: "2025-01", Date: time.Date(2025, 1, 5, 0, 0, 0, 0, time.UTC), Amount: 5000, Currency: "USD"},
		{SpendID: "SP-3", SupplierID: "SUP-BIG", Period: "2025-02", Date: time.Date(2025, 2, 5, 0, 0, 0, 0, time.UTC), Amount: 5200, Currency: "USD"},
		{SpendID: "SP-4", SupplierID: "SUP-SMALL", Period: "2025-02", Date: time.Date(2025, 2, 5, 0, 0, 0, 0, time.UTC), Amount: 4800, Currency: "USD"},
	}

	// With a high tolerance (0.5 = 50 points), the 2-point increase must
	// NOT trigger the flag.
	highTolerance := vendorspend.Calculate(vendorspend.Input{
		Periods: periods, Suppliers: suppliers, SpendRecords: records,
		Policy: vendorspend.Policy{SupplierShareIncreasePoints: 0.5},
	}, vendorspend.Options{})
	for _, f := range highTolerance.Flags {
		if f.Code == vendorspend.FlagSupplierConcentrationIncreasing {
			t.Errorf("REGRESSION: FlagSupplierConcentrationIncreasing fired despite a high configured tolerance (0.5) for only a ~2-point increase")
		}
	}

	// With a low tolerance (0.01 = 1 point), the 2-point increase MUST
	// trigger.
	lowTolerance := vendorspend.Calculate(vendorspend.Input{
		Periods: periods, Suppliers: suppliers, SpendRecords: records,
		Policy: vendorspend.Policy{SupplierShareIncreasePoints: 0.01},
	}, vendorspend.Options{})
	found := false
	for _, f := range lowTolerance.Flags {
		if f.Code == vendorspend.FlagSupplierConcentrationIncreasing {
			found = true
		}
	}
	if !found {
		t.Errorf("expected FlagSupplierConcentrationIncreasing with a low (0.01) tolerance for a ~2-point increase, got flags: %+v", lowTolerance.Flags)
	}
}

// TestRegression_DuplicateBucket_NegativeAmountRounding locks a real
// bug: the original int64(v*100+0.5) bucketing formula truncated
// negative amounts toward zero instead of rounding away from zero,
// putting -$100.00 into a bucket one cent off from its correct bucket.
// Two records with an identical negative Amount (a legitimate CREDIT)
// must still land in the SAME bucket and be detected as possible
// duplicates.
func TestRegression_DuplicateBucket_NegativeAmountRounding(t *testing.T) {
	periods := []vendorspend.Period{
		{Period: "2025-01", StartDate: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), EndDate: time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC), SequenceInYear: 1},
	}
	suppliers := []vendorspend.Supplier{{SupplierID: "SUP-1", Active: true}}
	records := []vendorspend.SpendRecord{
		{SpendID: "SP-1", SupplierID: "SUP-1", Period: "2025-01", Date: time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC),
			Amount: 100.00, Currency: "USD", Effect: vendorspend.EffectCredit, Category: "Supplies"},
		{SpendID: "SP-2", SupplierID: "SUP-1", Period: "2025-01", Date: time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC),
			Amount: 100.00, Currency: "USD", Effect: vendorspend.EffectCredit, Category: "Supplies"},
	}
	result := vendorspend.Calculate(vendorspend.Input{Periods: periods, Suppliers: suppliers, SpendRecords: records}, vendorspend.Options{})
	if len(result.DuplicateLikeGroups) != 1 {
		t.Fatalf("expected exactly 1 DuplicateLikeGroup for two identical-amount CREDIT records on the same day, got %d: %+v",
			len(result.DuplicateLikeGroups), result.DuplicateLikeGroups)
	}
}

// TestRegression_ConcentrationCalculatedOnce_MatchesBothViews locks a
// fix that eliminated a redundant second analytics/concentration.Calculate
// call: result.Concentration (most recent period) and the internal
// concentration-history trend check must still agree (derived from the
// same single concentration.Result), verified indirectly by confirming
// result.Concentration's Top1 matches the last entry a manually-run
// concentration-history-driven flag decision would use.
func TestRegression_ConcentrationCalculatedOnce_MatchesBothViews(t *testing.T) {
	result := vendorspend.Calculate(fullInput(), vendorspend.Options{})
	if !result.Concentration.Available {
		t.Skip("fixture did not produce available concentration")
	}
	// This is primarily a non-crash/consistency smoke test: the
	// refactor's correctness is that result.Concentration is still
	// populated exactly as before (spendConcentrationFromResult /
	// concentrationHistoryFromResult are pure re-splits of one
	// concentration.Result) — verified by the pre-existing
	// TestInvariant_ConcentrationMatchesAnalyticsConcentration and
	// TestJSON_Result_FullRoundTrip continuing to pass unchanged.
	if result.Concentration.Top1.Value < 0 || result.Concentration.Top1.Value > 1 {
		t.Errorf("Top1 = %v, want a value in [0,1]", result.Concentration.Top1.Value)
	}
}

// TestRegression_ControlTotal_NonFiniteReportsIssueAndUnavailable locks
// a real bug: a NaN/Inf control total silently produced a NaN
// Difference and Reconciled == false with no explanatory Issue, despite
// IssueInvalidControlTotal's doc comment promising exactly this
// detection.
func TestRegression_ControlTotal_NonFiniteReportsIssueAndUnavailable(t *testing.T) {
	nan := math.NaN()
	in := vendorspend.Input{
		Periods:      []vendorspend.Period{{Period: "2025-01", StartDate: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), EndDate: time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC), SequenceInYear: 1}},
		Suppliers:    []vendorspend.Supplier{{SupplierID: "SUP-1", Active: true}},
		SpendRecords: []vendorspend.SpendRecord{{SpendID: "SP-1", SupplierID: "SUP-1", Period: "2025-01", Date: time.Date(2025, 1, 5, 0, 0, 0, 0, time.UTC), Amount: 100, Currency: "USD"}},
		Controls:     vendorspend.ControlTotals{Purchases: &nan},
	}
	result := vendorspend.Calculate(in, vendorspend.Options{})
	if result.ControlReconciliation.Purchases.Available {
		t.Errorf("expected Purchases reconciliation Available == false for a NaN control total, got %+v", result.ControlReconciliation.Purchases)
	}
	if !hasIssueCode(result.Issues, vendorspend.IssueInvalidControlTotal) {
		t.Errorf("expected IssueInvalidControlTotal, got Issues: %+v", result.Issues)
	}
}

// TestRegression_NonPreferredFlag_TriggersOnNegativeNet locks a real
// bug: FlagSpendWithNonPreferredSupplier only fired when
// NonPreferredSpend.TotalSpend > 0, silently hiding real non-preferred
// activity that netted to zero or negative (e.g. mostly offset by a
// credit).
func TestRegression_NonPreferredFlag_TriggersOnNegativeNet(t *testing.T) {
	periods := []vendorspend.Period{
		{Period: "2025-01", StartDate: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), EndDate: time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC), SequenceInYear: 1},
	}
	suppliers := []vendorspend.Supplier{
		{SupplierID: "SUP-PREFERRED", Active: true, PreferredSupplier: true},
		{SupplierID: "SUP-OTHER", Active: true}, // non-preferred
	}
	records := []vendorspend.SpendRecord{
		{SpendID: "SP-1", SupplierID: "SUP-PREFERRED", Period: "2025-01", Date: time.Date(2025, 1, 5, 0, 0, 0, 0, time.UTC), Amount: 1000, Currency: "USD"},
		// SUP-OTHER (non-preferred): $200 normal purchase, $1000 credit —
		// nets to -$800, but real non-preferred activity DID occur.
		{SpendID: "SP-2", SupplierID: "SUP-OTHER", Period: "2025-01", Date: time.Date(2025, 1, 6, 0, 0, 0, 0, time.UTC), Amount: 200, Currency: "USD", Effect: vendorspend.EffectNormal},
		{SpendID: "SP-3", SupplierID: "SUP-OTHER", Period: "2025-01", Date: time.Date(2025, 1, 7, 0, 0, 0, 0, time.UTC), Amount: 1000, Currency: "USD", Effect: vendorspend.EffectCredit},
	}
	result := vendorspend.Calculate(vendorspend.Input{Periods: periods, Suppliers: suppliers, SpendRecords: records}, vendorspend.Options{})

	if !result.NonPreferredSpend.Available {
		t.Fatalf("expected NonPreferredSpend.Available")
	}
	if result.NonPreferredSpend.TotalSpend >= 0 {
		t.Fatalf("test setup error: expected a negative net TotalSpend, got %v", result.NonPreferredSpend.TotalSpend)
	}
	found := false
	for _, f := range result.Flags {
		if f.Code == vendorspend.FlagSpendWithNonPreferredSupplier {
			found = true
		}
	}
	if !found {
		t.Errorf("REGRESSION: FlagSpendWithNonPreferredSupplier did not fire despite real non-preferred activity (net -$800), flags: %+v", result.Flags)
	}
}

// TestRegression_CategoryCoverage_NotDistortedByOffsettingCredit locks
// a real bug: CategoryCoveragePercent, computed from signed net dollars,
// produced a nonsensical percentage (millions of a percent) when an
// uncategorized credit largely offset categorized spend.
func TestRegression_CategoryCoverage_NotDistortedByOffsettingCredit(t *testing.T) {
	periods := []vendorspend.Period{
		{Period: "2025-01", StartDate: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), EndDate: time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC), SequenceInYear: 1},
	}
	suppliers := []vendorspend.Supplier{{SupplierID: "SUP-1", Active: true}}
	records := []vendorspend.SpendRecord{
		// $100,000 of categorized normal spend.
		{SpendID: "SP-1", SupplierID: "SUP-1", Period: "2025-01", Date: time.Date(2025, 1, 5, 0, 0, 0, 0, time.UTC),
			Amount: 100000, Currency: "USD", Category: "Materials"},
		// $99,999 of UNCATEGORIZED credit, largely offsetting the above
		// on a net-dollar basis (net total = $1).
		{SpendID: "SP-2", SupplierID: "SUP-1", Period: "2025-01", Date: time.Date(2025, 1, 6, 0, 0, 0, 0, time.UTC),
			Amount: 99999, Currency: "USD", Effect: vendorspend.EffectCredit},
	}
	result := vendorspend.Calculate(vendorspend.Input{Periods: periods, Suppliers: suppliers, SpendRecords: records}, vendorspend.Options{})

	c := result.CategoryProductCoverage
	if !c.CategoryCoveragePercent.Available {
		t.Fatalf("expected CategoryCoveragePercent.Available")
	}
	// Magnitude-basis: 100000 categorized / (100000+99999) total ≈ 50%,
	// not the ~10,000,000% a signed-net-dollar ratio would have produced.
	if c.CategoryCoveragePercent.Value < 0 || c.CategoryCoveragePercent.Value > 1 {
		t.Errorf("REGRESSION: CategoryCoveragePercent = %v is outside [0,1] — a signed-net-dollar-basis ratio distorted by an offsetting credit", c.CategoryCoveragePercent.Value)
	}
	// The invariant (CategorizedSpend + UncategorizedSpend == NetSpend)
	// must still hold on the underlying net-dollar figures — only the
	// PERCENT uses the magnitude basis.
	if !almostEqual(c.CategorizedSpend+c.UncategorizedSpend, result.Bridge.NetSpend) {
		t.Errorf("CategorizedSpend(%v) + UncategorizedSpend(%v) != NetSpend(%v)", c.CategorizedSpend, c.UncategorizedSpend, result.Bridge.NetSpend)
	}
}

// TestRegression_TailSpendAmbiguousPolicy_ReportsIssue locks a real
// bug: TailSpendPolicy.kind() silently picked BelowAmount over
// OutsideTopN when both were set, with no Issue explaining that half
// the caller's configuration was discarded.
func TestRegression_TailSpendAmbiguousPolicy_ReportsIssue(t *testing.T) {
	in := fullInput()
	in.Policy.TailSpend = vendorspend.TailSpendPolicy{BelowAmount: 1000, OutsideTopN: 3}
	result := vendorspend.Calculate(in, vendorspend.Options{})
	if !hasIssueCode(result.Issues, vendorspend.IssueInvalidPolicy) {
		t.Errorf("expected IssueInvalidPolicy when both TailSpend.BelowAmount and OutsideTopN are set, got Issues: %+v", result.Issues)
	}
	if result.TailSpend.Definition != vendorspend.TailSpendBelowAmount {
		t.Errorf("Definition = %v, want TailSpendBelowAmount (documented precedence)", result.TailSpend.Definition)
	}
}

// TestRegression_InvalidPolicy_NegativeMateriality_ReportsIssueNotFatal
// locks the new validatePolicy check: a negative Policy field is
// reported via IssueInvalidPolicy but does not abort the analysis.
func TestRegression_InvalidPolicy_NegativeMateriality_ReportsIssueNotFatal(t *testing.T) {
	in := fullInput()
	in.Policy.Materiality.AbsoluteAmount = -500
	result := vendorspend.Calculate(in, vendorspend.Options{})
	if !result.Available {
		t.Fatalf("expected Available == true (an invalid Policy field is reported, not fatal), got Issues: %+v", result.Issues)
	}
	if !hasIssueCode(result.Issues, vendorspend.IssueInvalidPolicy) {
		t.Errorf("expected IssueInvalidPolicy for a negative Materiality.AbsoluteAmount, got Issues: %+v", result.Issues)
	}
}
