package diagnostics

import "testing"

func hasFindingCode(findings []Finding, id string, code FindingCode) bool {
	for _, f := range findings {
		if f.BusinessID == id && f.Code == code {
			return true
		}
	}
	return false
}

func TestCalculate_EmptyPortfolio(t *testing.T) {
	res := Calculate(Input{})
	if res.Available {
		t.Fatalf("expected Available == false for empty portfolio")
	}
	if !HasErrors(res.Errors) {
		t.Fatalf("expected an error for empty portfolio, got %+v", res.Errors)
	}
	if res.Errors[0].Code != IssueEmptyPortfolio {
		t.Fatalf("expected IssueEmptyPortfolio, got %s", res.Errors[0].Code)
	}
}

func TestCalculate_SkipsEmptyBusinessID(t *testing.T) {
	res := Calculate(Input{Portfolio: []BusinessSnapshot{
		{ID: "", Period: "FY2024"},
		stableBusiness("b1"),
	}})
	if !res.Available {
		t.Fatalf("expected Available == true, errors=%+v", res.Errors)
	}
	if res.Coverage.TotalBusinesses != 1 {
		t.Fatalf("expected 1 scanned business, got %d", res.Coverage.TotalBusinesses)
	}
	found := false
	for _, w := range res.Warnings {
		if w.Code == IssueEmptyBusinessID {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected IssueEmptyBusinessID warning, got %+v", res.Warnings)
	}
}

func TestCalculate_SkipsDuplicateBusinessID(t *testing.T) {
	b1 := stableBusiness("dup")
	b2 := stableBusiness("dup")
	b2.Label = "Second copy"
	res := Calculate(Input{Portfolio: []BusinessSnapshot{b1, b2}})
	if res.Coverage.TotalBusinesses != 1 {
		t.Fatalf("expected 1 scanned business after dedup, got %d", res.Coverage.TotalBusinesses)
	}
	found := false
	for _, w := range res.Warnings {
		if w.Code == IssueDuplicateBusinessID {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected IssueDuplicateBusinessID warning, got %+v", res.Warnings)
	}
}

// TestCalculate_AllFieldsZero proves a portfolio where every business is
// entirely empty (only ID/Period set) still succeeds with zero findings and
// zero coverage, never a crash or a spurious finding — the base case for
// "missing modules reduce coverage, not silently score zero."
func TestCalculate_AllFieldsZero(t *testing.T) {
	res := Calculate(Input{Portfolio: []BusinessSnapshot{
		emptyBusiness("empty1"),
		emptyBusiness("empty2"),
	}})
	if !res.Available {
		t.Fatalf("expected Available == true, errors=%+v", res.Errors)
	}
	if len(res.Findings) != 0 {
		t.Fatalf("expected zero findings for entirely empty snapshots, got %d: %+v", len(res.Findings), res.Findings)
	}
	if res.Coverage.WithQoE != 0 || res.Coverage.WithRatioHealth != 0 || res.Coverage.WithPrior != 0 {
		t.Fatalf("expected zero coverage across the board, got %+v", res.Coverage)
	}
	if res.Coverage.TotalBusinesses != 2 {
		t.Fatalf("expected 2 total businesses, got %d", res.Coverage.TotalBusinesses)
	}
}

// TestCalculate_StableBusinessHasNoFindings proves a healthy business with
// no material changes produces zero findings.
func TestCalculate_StableBusinessHasNoFindings(t *testing.T) {
	res := Calculate(Input{Portfolio: []BusinessSnapshot{stableBusiness("stable1")}})
	if !res.Available {
		t.Fatalf("expected Available == true, errors=%+v", res.Errors)
	}
	if len(res.Findings) != 0 {
		t.Fatalf("expected zero findings for a stable business, got %+v", res.Findings)
	}
}

// TestCalculate_DecliningBusinessTriggersEveryChangeFinding proves a
// business with material adverse changes on every tracked dimension raises
// every corresponding FindingCode, each pointed at the right business.
func TestCalculate_DecliningBusinessTriggersEveryChangeFinding(t *testing.T) {
	res := Calculate(Input{Portfolio: []BusinessSnapshot{decliningBusiness("d1")}})
	if !res.Available {
		t.Fatalf("expected Available == true, errors=%+v", res.Errors)
	}

	want := []FindingCode{
		FindingMarginDeterioration,
		FindingRevenueDecline,
		FindingCashConversionWeakening,
		FindingLeverageIncrease,
		FindingConcentrationIncrease,
		FindingValuationMovement,
		FindingUnresolvedFinancialQuality,
		FindingSaleReadinessOpportunity,
	}
	for _, code := range want {
		if !hasFindingCode(res.Findings, "d1", code) {
			t.Errorf("expected finding %s for business d1, got %+v", code, res.Findings)
		}
	}
}

// TestCalculate_NoPriorStillFindsSignalOnlyIssues proves single-period
// signal-driven findings still fire without a Prior, while pure
// change-based findings (revenue decline, valuation movement) do not.
func TestCalculate_NoPriorStillFindsSignalOnlyIssues(t *testing.T) {
	res := Calculate(Input{Portfolio: []BusinessSnapshot{noPriorBusiness("np1")}})
	if !res.Available {
		t.Fatalf("expected Available == true, errors=%+v", res.Errors)
	}

	wantPresent := []FindingCode{
		FindingMarginDeterioration,
		FindingCashConversionWeakening,
		FindingLeverageIncrease,
		FindingConcentrationIncrease,
		FindingUnresolvedFinancialQuality,
		FindingSaleReadinessOpportunity,
	}
	for _, code := range wantPresent {
		if !hasFindingCode(res.Findings, "np1", code) {
			t.Errorf("expected finding %s for business np1 (signal-only), got %+v", code, res.Findings)
		}
	}

	wantAbsent := []FindingCode{FindingRevenueDecline, FindingValuationMovement}
	for _, code := range wantAbsent {
		if hasFindingCode(res.Findings, "np1", code) {
			t.Errorf("did not expect finding %s for business np1 with no Prior", code)
		}
	}
}

// TestCalculate_ValuationMovementFiresOnIncrease proves
// FindingValuationMovement fires on a large increase, not just a decline —
// the one finding code with symmetric direction.
func TestCalculate_ValuationMovementFiresOnIncrease(t *testing.T) {
	b := stableBusiness("v1")
	b.Prior.Valuation.IndicatedValue = AvailableValue(2_000_000)
	b.Valuation.IndicatedValue = AvailableValue(3_000_000) // +50%
	res := Calculate(Input{Portfolio: []BusinessSnapshot{b}})
	if !hasFindingCode(res.Findings, "v1", FindingValuationMovement) {
		t.Fatalf("expected FindingValuationMovement on a large increase, got %+v", res.Findings)
	}
	for _, f := range res.Findings {
		if f.Code == FindingValuationMovement {
			if !f.Change.Available || f.Change.Amount <= 0 {
				t.Fatalf("expected a positive Change for an increase, got %+v", f.Change)
			}
		}
	}
}

// TestCalculate_MultiClientPortfolio proves a multi-business portfolio
// scans every business independently, with findings correctly attributed.
func TestCalculate_MultiClientPortfolio(t *testing.T) {
	res := Calculate(Input{Portfolio: []BusinessSnapshot{
		stableBusiness("stable1"),
		decliningBusiness("declining1"),
		emptyBusiness("empty1"),
		noPriorBusiness("noprior1"),
	}})
	if !res.Available {
		t.Fatalf("expected Available == true, errors=%+v", res.Errors)
	}
	if res.Coverage.TotalBusinesses != 4 {
		t.Fatalf("expected 4 businesses scanned, got %d", res.Coverage.TotalBusinesses)
	}
	if hasAnyFindingFor(res.Findings, "stable1") {
		t.Errorf("did not expect any finding for stable1")
	}
	if !hasAnyFindingFor(res.Findings, "declining1") {
		t.Errorf("expected findings for declining1")
	}
	if hasAnyFindingFor(res.Findings, "empty1") {
		t.Errorf("did not expect any finding for empty1")
	}
	if !hasAnyFindingFor(res.Findings, "noprior1") {
		t.Errorf("expected signal-only findings for noprior1")
	}
	if res.Counts.BusinessesWithFindings != 2 {
		t.Fatalf("expected 2 distinct businesses with findings, got %d", res.Counts.BusinessesWithFindings)
	}
}

func hasAnyFindingFor(findings []Finding, id string) bool {
	for _, f := range findings {
		if f.BusinessID == id {
			return true
		}
	}
	return false
}

// TestCalculate_FindingsRankedByPriorityThenCodeThenBusinessID proves the
// documented tie-break chain: PriorityScore descending, then findingOrder
// declaration order, then BusinessID ascending.
func TestCalculate_FindingsRankedByPriorityThenCodeThenBusinessID(t *testing.T) {
	res := Calculate(Input{Portfolio: []BusinessSnapshot{
		decliningBusiness("z-business"),
		decliningBusiness("a-business"),
	}})
	if len(res.Findings) < 2 {
		t.Fatalf("expected multiple findings, got %d", len(res.Findings))
	}
	for i := 1; i < len(res.Findings); i++ {
		prev, cur := res.Findings[i-1], res.Findings[i]
		if prev.PriorityScore < cur.PriorityScore {
			t.Fatalf("findings not sorted by PriorityScore descending at index %d: %+v vs %+v", i, prev, cur)
		}
		if prev.PriorityScore == cur.PriorityScore {
			prevRank, curRank := findingCodeRank[prev.Code], findingCodeRank[cur.Code]
			if prevRank > curRank {
				t.Fatalf("equal-priority findings not ordered by findingOrder at index %d: %+v vs %+v", i, prev, cur)
			}
			if prevRank == curRank && prev.BusinessID > cur.BusinessID {
				t.Fatalf("equal-priority, equal-code findings not ordered by BusinessID at index %d: %+v vs %+v", i, prev, cur)
			}
		}
	}
}

// TestCalculate_EqualSeverityOrderIsStableAndDeterministic proves that two
// businesses with the exact same finding profile (equal severities,
// equal magnitudes) still produce a fully deterministic, tie-broken order
// rather than depending on map/slice iteration order.
func TestCalculate_EqualSeverityOrderIsStableAndDeterministic(t *testing.T) {
	bA := decliningBusiness("bbb")
	bB := decliningBusiness("aaa")

	res1 := Calculate(Input{Portfolio: []BusinessSnapshot{bA, bB}})
	res2 := Calculate(Input{Portfolio: []BusinessSnapshot{bB, bA}})

	if len(res1.Findings) != len(res2.Findings) {
		t.Fatalf("finding count differs by input order: %d vs %d", len(res1.Findings), len(res2.Findings))
	}
	for i := range res1.Findings {
		f1, f2 := res1.Findings[i], res2.Findings[i]
		if f1.BusinessID != f2.BusinessID || f1.Code != f2.Code {
			t.Fatalf("finding %d differs by portfolio input order: %+v vs %+v", i, f1, f2)
		}
	}
}

// TestCalculate_CustomPolicyMaterialChangeThreshold proves Policy is
// respected: a small decline is invisible under the default/loose
// threshold but reported once MaterialChangePercent is tightened below the
// decline's magnitude.
func TestCalculate_CustomPolicyMaterialChangeThreshold(t *testing.T) {
	b := stableBusiness("p1")
	b.Prior.Metrics.Revenue = AvailableValue(1_000_000)
	b.Metrics.Revenue = AvailableValue(980_000) // -2%

	loose := Calculate(Input{Portfolio: []BusinessSnapshot{b}, Policy: Policy{MaterialChangePercent: 0.05}})
	if hasFindingCode(loose.Findings, "p1", FindingRevenueDecline) {
		t.Fatalf("did not expect FindingRevenueDecline below the 5%% material-change threshold, got %+v", loose.Findings)
	}

	strict := Calculate(Input{Portfolio: []BusinessSnapshot{b}, Policy: Policy{MaterialChangePercent: 0.01}})
	if !hasFindingCode(strict.Findings, "p1", FindingRevenueDecline) {
		t.Fatalf("expected FindingRevenueDecline above a 1%% material-change threshold, got %+v", strict.Findings)
	}
}

// TestCalculate_CustomSeverityWeightsChangeRanking proves
// Policy.SeverityWeights is honored by computePriorityScore: inverting the
// weights (critical below warning) inverts two findings' relative order.
func TestCalculate_CustomSeverityWeightsChangeRanking(t *testing.T) {
	b := decliningBusiness("w1")
	inverted := Policy{
		SeverityWeights: map[Severity]float64{
			SeverityCritical: 1,
			SeverityWarning:  10,
			SeverityInfo:     1,
		},
	}
	res := Calculate(Input{Portfolio: []BusinessSnapshot{b}, Policy: inverted})
	if len(res.Findings) == 0 {
		t.Fatalf("expected findings")
	}
	// Under inverted weights, a warning-severity finding must outrank a
	// critical-severity finding somewhere, proving Policy actually drove
	// the ranking rather than a hardcoded severity order.
	var sawWarningBeforeCritical bool
	var sawCritical bool
	for _, f := range res.Findings {
		if f.Severity == SeverityCritical {
			sawCritical = true
		}
		if f.Severity == SeverityWarning && !sawCritical {
			sawWarningBeforeCritical = true
		}
	}
	if !sawWarningBeforeCritical {
		t.Fatalf("expected a warning-severity finding to rank ahead of any critical finding under inverted weights, got %+v", res.Findings)
	}
}

// TestCalculate_ExplicitZeroSeverityWeightIsHonored proves a caller can
// set Policy.SeverityWeights[SeverityInfo] = 0 to exclude info-severity
// findings from ranking entirely, and that the explicit 0 is not silently
// replaced by DefaultPolicy's nonzero value for that severity.
func TestCalculate_ExplicitZeroSeverityWeightIsHonored(t *testing.T) {
	b := noPriorBusiness("z1")
	// FindingSaleReadinessOpportunity fires at SeverityInfo when
	// OverallScore is below threshold with no blockers; b has a blocker,
	// so force the info-only path via a fresh SaleReadiness summary.
	b.SaleReadiness = SaleReadinessSummary{Available: true, OverallScore: AvailableValue(60)}

	zeroed := Policy{SeverityWeights: map[Severity]float64{SeverityInfo: 0}}
	res := Calculate(Input{Portfolio: []BusinessSnapshot{b}, Policy: zeroed})

	found := false
	for _, f := range res.Findings {
		if f.Code == FindingSaleReadinessOpportunity {
			found = true
			if f.Severity != SeverityInfo {
				t.Fatalf("expected FindingSaleReadinessOpportunity at SeverityInfo, got %s", f.Severity)
			}
			if f.PriorityScore != 0 {
				t.Fatalf("expected PriorityScore 0 for an explicitly zero-weighted SeverityInfo finding, got %v", f.PriorityScore)
			}
		}
	}
	if !found {
		t.Fatalf("expected FindingSaleReadinessOpportunity to still be detected (zero weight affects ranking, not detection)")
	}
}

// TestCalculate_DoesNotMutateInput proves Calculate never mutates
// caller-owned Input, including nested Prior pointers.
func TestCalculate_DoesNotMutateInput(t *testing.T) {
	b := decliningBusiness("m1")
	beforeID, beforePeriod := b.ID, b.Period
	beforeRevenue, beforeRatioHealth := b.Metrics.Revenue, b.RatioHealth
	beforePriorID, beforePriorRevenue := b.Prior.ID, b.Prior.Metrics.Revenue

	_ = Calculate(Input{Portfolio: []BusinessSnapshot{b}})

	if b.ID != beforeID || b.Period != beforePeriod {
		t.Fatalf("Calculate mutated the input BusinessSnapshot's ID/Period")
	}
	if b.Metrics.Revenue != beforeRevenue || b.RatioHealth != beforeRatioHealth {
		t.Fatalf("Calculate mutated the input BusinessSnapshot's Metrics/RatioHealth")
	}
	if b.Prior.ID != beforePriorID || b.Prior.Metrics.Revenue != beforePriorRevenue {
		t.Fatalf("Calculate mutated the input's Prior snapshot")
	}
}
