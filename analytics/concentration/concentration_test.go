package concentration

import (
	"testing"

	"github.com/themurtez/go-valuate/financial"
)

func TestCalculate_NoObservations(t *testing.T) {
	res := Calculate(Input{}, Options{})
	if res.Available {
		t.Fatalf("expected Available == false for empty Observations")
	}
	if !HasErrors(res.Errors) {
		t.Fatalf("expected an error Issue for empty Observations")
	}
	if len(res.Errors) != 1 || res.Errors[0].Code != IssueNoObservations {
		t.Fatalf("expected IssueNoObservations, got %+v", res.Errors)
	}
}

func TestCalculate_HighlyConcentrated(t *testing.T) {
	res := Calculate(Input{
		Basis:        BasisCustomerRevenue,
		Observations: highlyConcentratedObservations(),
		PeriodMeta:   threeYearMeta(),
	}, Options{})

	if !res.Available {
		t.Fatalf("expected Available == true, errors: %+v", res.Errors)
	}
	if len(res.History) != 3 {
		t.Fatalf("expected 3 periods in History, got %d", len(res.History))
	}

	last := res.History[2]
	if last.Period != "2025" {
		t.Fatalf("expected last period 2025, got %s", last.Period)
	}
	if !last.LargestEntityShare.Available {
		t.Fatalf("expected LargestEntityShare available")
	}
	wantShare := 850_000.0 / 1_000_000.0
	if diff := last.LargestEntityShare.Value - wantShare; diff > 1e-9 || diff < -1e-9 {
		t.Fatalf("expected largest entity share %.4f, got %.4f", wantShare, last.LargestEntityShare.Value)
	}
	if last.RankedEntities[0].EntityKey != "whale" {
		t.Fatalf("expected 'whale' ranked first, got %s", last.RankedEntities[0].EntityKey)
	}
	if !last.HHI.Available || last.HHI.Value < 2500 {
		t.Fatalf("expected highly concentrated HHI >= 2500, got %+v", last.HHI)
	}

	// Concentration worsened from 2023 (70%) to 2025 (85%): trend should be
	// increasing.
	if res.LargestShareTrend.Direction != TrendIncreasing {
		t.Fatalf("expected increasing largest-share trend, got %s", res.LargestShareTrend.Direction)
	}

	foundHighShare := false
	foundHighHHI := false
	for _, f := range res.Flags {
		if f.Code == FlagHighLargestEntityConcentration {
			foundHighShare = true
		}
		if f.Code == FlagHighHHI {
			foundHighHHI = true
		}
	}
	if !foundHighShare {
		t.Errorf("expected FlagHighLargestEntityConcentration to trigger")
	}
	if !foundHighHHI {
		t.Errorf("expected FlagHighHHI to trigger")
	}
}

func TestCalculate_Diversified(t *testing.T) {
	res := Calculate(Input{
		Observations: diversifiedObservations(),
		PeriodMeta: map[financial.Period]PeriodInfo{
			"2025": {Type: PeriodTypeFiscalYear, FiscalYear: 2025},
		},
	}, Options{})

	if !res.Available {
		t.Fatalf("expected Available == true, errors: %+v", res.Errors)
	}
	pc := res.History[0]
	if pc.EntityCount != 10 {
		t.Fatalf("expected 10 entities, got %d", pc.EntityCount)
	}
	if !pc.LargestEntityShare.Available || pc.LargestEntityShare.Value != 0.1 {
		t.Fatalf("expected largest entity share 0.1 (equal split), got %+v", pc.LargestEntityShare)
	}
	// HHI for 10 equal entities: 10 * (0.1^2) * 10000 = 1000.
	if !pc.HHI.Available || diffAbs(pc.HHI.Value, 1000) > 1e-6 {
		t.Fatalf("expected HHI ~1000 for 10 equal entities, got %+v", pc.HHI)
	}

	for _, f := range res.Flags {
		if f.Code == FlagHighLargestEntityConcentration || f.Code == FlagHighHHI {
			t.Errorf("did not expect concentration flag %s for diversified base", f.Code)
		}
	}
}

func TestCalculate_OneCustomer(t *testing.T) {
	res := Calculate(Input{
		Observations: []Observation{obs("only-customer", "2025", 500_000)},
		PeriodMeta: map[financial.Period]PeriodInfo{
			"2025": {Type: PeriodTypeFiscalYear, FiscalYear: 2025},
		},
	}, Options{})

	if !res.Available {
		t.Fatalf("expected Available == true, errors: %+v", res.Errors)
	}
	pc := res.History[0]
	if pc.EntityCount != 1 {
		t.Fatalf("expected 1 entity, got %d", pc.EntityCount)
	}
	if !pc.LargestEntityShare.Available || pc.LargestEntityShare.Value != 1.0 {
		t.Fatalf("expected largest entity share 1.0, got %+v", pc.LargestEntityShare)
	}
	if !pc.HHI.Available || pc.HHI.Value != 10000 {
		t.Fatalf("expected HHI 10000 for a single entity, got %+v", pc.HHI)
	}

	// TopNShares for N > 1 should still be 1.0 (only one entity available).
	for _, s := range pc.TopNShares {
		if s.N >= 1 && (!s.Share.Available || s.Share.Value != 1.0) {
			t.Errorf("expected top-%d share 1.0 with a single entity, got %+v", s.N, s.Share)
		}
	}
}

func TestCalculate_ChangingConcentration_DependencyChanges(t *testing.T) {
	res := Calculate(Input{
		Observations: []Observation{
			obs("cust-a", "2023", 500_000),
			obs("cust-b", "2023", 500_000),

			// cust-a grows, cust-b is lost, cust-c is new.
			obs("cust-a", "2024", 900_000),
			obs("cust-c", "2024", 100_000),
		},
		PeriodMeta: map[financial.Period]PeriodInfo{
			"2023": {Type: PeriodTypeFiscalYear, FiscalYear: 2023},
			"2024": {Type: PeriodTypeFiscalYear, FiscalYear: 2024},
		},
	}, Options{})

	if !res.Available {
		t.Fatalf("expected Available == true, errors: %+v", res.Errors)
	}
	if len(res.DependencyChanges) != 3 {
		t.Fatalf("expected 3 dependency changes (a, b, c), got %d: %+v", len(res.DependencyChanges), res.DependencyChanges)
	}

	byEntity := make(map[string]DependencyChange, 3)
	for _, dc := range res.DependencyChanges {
		byEntity[dc.EntityKey] = dc
	}

	a := byEntity["cust-a"]
	if !a.FromShare.Available || a.FromShare.Value != 0.5 {
		t.Errorf("expected cust-a FromShare 0.5, got %+v", a.FromShare)
	}
	if !a.ToShare.Available || a.ToShare.Value != 0.9 {
		t.Errorf("expected cust-a ToShare 0.9, got %+v", a.ToShare)
	}
	if !a.ShareChange.Available || a.ShareChange.Value <= 0 {
		t.Errorf("expected cust-a ShareChange positive, got %+v", a.ShareChange)
	}

	b := byEntity["cust-b"]
	if !b.FromAmount.Available || b.FromAmount.Value != 500_000 {
		t.Errorf("expected cust-b FromAmount 500000, got %+v", b.FromAmount)
	}
	if b.ToAmount.Available {
		t.Errorf("expected cust-b ToAmount unavailable (lost customer), got %+v", b.ToAmount)
	}

	c := byEntity["cust-c"]
	if c.FromAmount.Available {
		t.Errorf("expected cust-c FromAmount unavailable (new customer), got %+v", c.FromAmount)
	}
	if !c.ToAmount.Available || c.ToAmount.Value != 100_000 {
		t.Errorf("expected cust-c ToAmount 100000, got %+v", c.ToAmount)
	}
}

func TestCalculate_ZeroNegativeMalformedInputs(t *testing.T) {
	res := Calculate(Input{
		Observations: []Observation{
			obs("cust-a", "2025", 100_000),
			obs("cust-b", "2025", -50_000), // invalid: negative
			obs("cust-c", "2025", 0),       // valid: zero is allowed
		},
		PeriodMeta: map[financial.Period]PeriodInfo{
			"2025": {Type: PeriodTypeFiscalYear, FiscalYear: 2025},
		},
	}, Options{})

	if !res.Available {
		t.Fatalf("expected Available == true, errors: %+v", res.Errors)
	}

	foundInvalid := false
	for _, w := range res.Warnings {
		if w.Code == IssueInvalidObservation {
			foundInvalid = true
		}
	}
	if !foundInvalid {
		t.Fatalf("expected IssueInvalidObservation warning for the negative amount")
	}

	pc := res.History[0]
	if pc.EntityCount != 2 {
		t.Fatalf("expected 2 valid entities (a, c), got %d: %+v", pc.EntityCount, pc.RankedEntities)
	}
	for _, re := range pc.RankedEntities {
		if re.EntityKey == "cust-b" {
			t.Fatalf("expected cust-b excluded (negative amount), but found it ranked")
		}
	}
}

func TestCalculate_AllZeroAmounts(t *testing.T) {
	res := Calculate(Input{
		Observations: []Observation{
			obs("cust-a", "2025", 0),
			obs("cust-b", "2025", 0),
		},
		PeriodMeta: map[financial.Period]PeriodInfo{
			"2025": {Type: PeriodTypeFiscalYear, FiscalYear: 2025},
		},
	}, Options{})

	if !res.Available {
		t.Fatalf("expected Available == true, errors: %+v", res.Errors)
	}
	pc := res.History[0]
	if pc.EntityCount != 2 {
		t.Fatalf("expected 2 entities, got %d", pc.EntityCount)
	}
	if pc.HHI.Available {
		t.Fatalf("expected HHI unavailable when total amount is zero, got %+v", pc.HHI)
	}
	if pc.LargestEntityShare.Available {
		t.Fatalf("expected LargestEntityShare unavailable when total amount is zero, got %+v", pc.LargestEntityShare)
	}
}

func TestCalculate_TopNLossScenario(t *testing.T) {
	rate := 0.4
	res := Calculate(Input{
		Observations: highlyConcentratedObservations(),
		PeriodMeta:   threeYearMeta(),
		Policy: Policy{
			ScenarioTopN:            []int{1, 2},
			DefaultImpactMarginRate: &rate,
		},
	}, Options{})

	if !res.Available {
		t.Fatalf("expected Available == true, errors: %+v", res.Errors)
	}
	if len(res.Scenarios) != 3 { // lost-largest + top-1 + top-2
		t.Fatalf("expected 3 scenarios, got %d: %+v", len(res.Scenarios), res.Scenarios)
	}

	var lostLargest, top1, top2 *Scenario
	for i := range res.Scenarios {
		s := &res.Scenarios[i]
		switch {
		case s.Kind == ScenarioLostLargestEntity:
			lostLargest = s
		case s.Kind == ScenarioTopNLoss && s.N == 1:
			top1 = s
		case s.Kind == ScenarioTopNLoss && s.N == 2:
			top2 = s
		}
	}
	if lostLargest == nil || top1 == nil || top2 == nil {
		t.Fatalf("missing expected scenario(s): lostLargest=%v top1=%v top2=%v", lostLargest, top1, top2)
	}

	if lostLargest.Period != "2025" {
		t.Fatalf("expected scenario computed against most recent period 2025, got %s", lostLargest.Period)
	}
	if !lostLargest.TotalRevenueImpact.Available || lostLargest.TotalRevenueImpact.Value != 850_000 {
		t.Fatalf("expected lost-largest revenue impact 850000, got %+v", lostLargest.TotalRevenueImpact)
	}
	if !lostLargest.TotalEarningsImpact.Available || lostLargest.TotalEarningsImpact.Value != 850_000*0.4 {
		t.Fatalf("expected lost-largest earnings impact 340000, got %+v", lostLargest.TotalEarningsImpact)
	}
	if !lostLargest.RemainingRevenue.Available || lostLargest.RemainingRevenue.Value != 150_000 {
		t.Fatalf("expected remaining revenue 150000, got %+v", lostLargest.RemainingRevenue)
	}

	// top1 should equal lostLargest in substance.
	if top1.TotalRevenueImpact.Value != lostLargest.TotalRevenueImpact.Value {
		t.Errorf("expected top1 scenario to match lost-largest-entity scenario")
	}

	// top2 removes whale (850k) + small-1 (100k) = 950k.
	if !top2.TotalRevenueImpact.Available || top2.TotalRevenueImpact.Value != 950_000 {
		t.Fatalf("expected top2 revenue impact 950000, got %+v", top2.TotalRevenueImpact)
	}
	if len(top2.EntityImpacts) != 2 {
		t.Fatalf("expected 2 entity impacts in top2 scenario, got %d", len(top2.EntityImpacts))
	}

	foundScenarioFlag := false
	for _, f := range res.Flags {
		if f.Code == FlagHighScenarioImpact {
			foundScenarioFlag = true
		}
	}
	if !foundScenarioFlag {
		t.Errorf("expected FlagHighScenarioImpact to trigger for an 85%% revenue-loss scenario")
	}
}

func TestCalculate_ScenarioWithoutMarginAssumption(t *testing.T) {
	res := Calculate(Input{
		Observations: highlyConcentratedObservations(),
		PeriodMeta:   threeYearMeta(),
	}, Options{})

	if !res.Available {
		t.Fatalf("expected Available == true, errors: %+v", res.Errors)
	}
	if len(res.Scenarios) == 0 {
		t.Fatalf("expected scenarios to be computed even without a margin assumption")
	}
	for _, s := range res.Scenarios {
		if s.TotalEarningsImpact.Available {
			t.Errorf("expected TotalEarningsImpact unavailable with no margin assumption, got %+v for scenario %s/%d", s.TotalEarningsImpact, s.Kind, s.N)
		}
		if !s.TotalRevenueImpact.Available {
			t.Errorf("expected TotalRevenueImpact available regardless of margin assumption")
		}
	}

	foundNoImpactWarning := false
	for _, w := range res.Warnings {
		if w.Code == IssueNoImpactAssumption {
			foundNoImpactWarning = true
		}
	}
	if !foundNoImpactWarning {
		t.Errorf("expected IssueNoImpactAssumption warning")
	}
}

func TestCalculate_EntityImpactAssumptionOverridesDefault(t *testing.T) {
	defaultRate := 0.2
	res := Calculate(Input{
		Observations: []Observation{
			obs("whale", "2025", 800_000),
			obs("small", "2025", 200_000),
		},
		PeriodMeta: map[financial.Period]PeriodInfo{
			"2025": {Type: PeriodTypeFiscalYear, FiscalYear: 2025},
		},
		Policy: Policy{
			ScenarioTopN:            []int{2},
			DefaultImpactMarginRate: &defaultRate,
			EntityImpactAssumptions: []EntityImpactAssumption{
				{EntityKey: "whale", MarginRate: 0.5},
			},
		},
	}, Options{})

	if !res.Available {
		t.Fatalf("expected Available == true, errors: %+v", res.Errors)
	}

	var top2 *Scenario
	for i := range res.Scenarios {
		if res.Scenarios[i].Kind == ScenarioTopNLoss && res.Scenarios[i].N == 2 {
			top2 = &res.Scenarios[i]
		}
	}
	if top2 == nil {
		t.Fatalf("expected a top-2 scenario")
	}

	for _, ei := range top2.EntityImpacts {
		switch ei.EntityKey {
		case "whale":
			if ei.MarginRateUsed != 0.5 {
				t.Errorf("expected whale to use its EntityImpactAssumption rate 0.5, got %v", ei.MarginRateUsed)
			}
		case "small":
			if ei.MarginRateUsed != 0.2 {
				t.Errorf("expected small to use the default margin rate 0.2, got %v", ei.MarginRateUsed)
			}
		}
	}

	wantEarnings := 800_000*0.5 + 200_000*0.2
	if !top2.TotalEarningsImpact.Available || top2.TotalEarningsImpact.Value != wantEarnings {
		t.Fatalf("expected total earnings impact %v, got %+v", wantEarnings, top2.TotalEarningsImpact)
	}
}

// TestCalculate_ImpactAssumptionCoversOnlyRemovedEntities proves
// IssueNoImpactAssumption is scoped to entities Scenarios actually remove:
// a caller who supplies margin data only for the top-2 entities, with
// ScenarioTopN limited to 2, should not see the warning even though a
// third, uncovered entity exists in the period.
func TestCalculate_ImpactAssumptionCoversOnlyRemovedEntities(t *testing.T) {
	res := Calculate(Input{
		Observations: []Observation{
			obs("whale", "2025", 600_000),
			obs("mid", "2025", 300_000),
			obs("tail", "2025", 100_000), // no margin assumption, never removed
		},
		PeriodMeta: map[financial.Period]PeriodInfo{
			"2025": {Type: PeriodTypeFiscalYear, FiscalYear: 2025},
		},
		Policy: Policy{
			ScenarioTopN: []int{2},
			EntityImpactAssumptions: []EntityImpactAssumption{
				{EntityKey: "whale", MarginRate: 0.4},
				{EntityKey: "mid", MarginRate: 0.3},
			},
		},
	}, Options{})

	if !res.Available {
		t.Fatalf("expected Available == true, errors: %+v", res.Errors)
	}
	for _, w := range res.Warnings {
		if w.Code == IssueNoImpactAssumption {
			t.Fatalf("did not expect IssueNoImpactAssumption when every removed entity has a margin assumption, got %+v", res.Warnings)
		}
	}

	for _, s := range res.Scenarios {
		if !s.TotalEarningsImpact.Available {
			t.Errorf("expected TotalEarningsImpact available for scenario %s/%d, got %+v", s.Kind, s.N, s.TotalEarningsImpact)
		}
	}
}

func TestCalculate_CategoryBreakdown(t *testing.T) {
	res := Calculate(Input{
		Observations: []Observation{
			obsCat("cust-a", "2025", 300_000, "enterprise"),
			obsCat("cust-b", "2025", 100_000, "enterprise"),
			obsCat("cust-c", "2025", 200_000, "smb"),
			obs("cust-d", "2025", 400_000), // no category
		},
		PeriodMeta: map[financial.Period]PeriodInfo{
			"2025": {Type: PeriodTypeFiscalYear, FiscalYear: 2025},
		},
	}, Options{})

	if !res.Available {
		t.Fatalf("expected Available == true, errors: %+v", res.Errors)
	}
	pc := res.History[0]
	if len(pc.Categories) != 2 {
		t.Fatalf("expected 2 categories, got %d: %+v", len(pc.Categories), pc.Categories)
	}
	// Sorted ascending: "enterprise" before "smb".
	if pc.Categories[0].Category != "enterprise" || pc.Categories[1].Category != "smb" {
		t.Fatalf("expected categories sorted ascending, got %+v", pc.Categories)
	}
	if pc.Categories[0].Amount.Value != 400_000 {
		t.Fatalf("expected enterprise category amount 400000, got %+v", pc.Categories[0].Amount)
	}
}

func TestCalculate_NoPeriodMeta(t *testing.T) {
	res := Calculate(Input{
		Observations: []Observation{
			obs("cust-a", "2024", 100_000),
			obs("cust-a", "2025", 200_000),
		},
	}, Options{})

	if !res.Available {
		t.Fatalf("expected Available == true, errors: %+v", res.Errors)
	}
	if len(res.History) != 2 {
		t.Fatalf("expected 2 periods still computed without PeriodMeta, got %d", len(res.History))
	}
	if res.LargestShareTrend.Direction != TrendUnavailable {
		t.Fatalf("expected trend unavailable without PeriodMeta")
	}
	if len(res.DependencyChanges) != 0 {
		t.Fatalf("expected no dependency changes without PeriodMeta")
	}
	if len(res.Scenarios) != 0 {
		t.Fatalf("expected no scenarios without PeriodMeta")
	}

	found := false
	for _, w := range res.Warnings {
		if w.Code == IssueNoPeriodMeta {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected IssueNoPeriodMeta warning")
	}
}

func TestCalculate_PeriodMissingFromMeta(t *testing.T) {
	res := Calculate(Input{
		Observations: []Observation{
			obs("cust-a", "2024", 100_000),
			obs("cust-a", "2025", 200_000),
		},
		PeriodMeta: map[financial.Period]PeriodInfo{
			"2024": {Type: PeriodTypeFiscalYear, FiscalYear: 2024},
			// 2025 intentionally missing.
		},
	}, Options{})

	if !res.Available {
		t.Fatalf("expected Available == true, errors: %+v", res.Errors)
	}
	found := false
	for _, w := range res.Warnings {
		if w.Code == IssuePeriodMissingFromMeta {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected IssuePeriodMissingFromMeta warning")
	}
	if res.LargestShareTrend.Direction != TrendUnavailable {
		t.Fatalf("expected trend unavailable when a period is missing from PeriodMeta")
	}
}

func TestDefaultPolicy(t *testing.T) {
	p := DefaultPolicy()
	if len(p.TopN) == 0 || len(p.ScenarioTopN) == 0 {
		t.Fatalf("expected non-empty defaults, got %+v", p)
	}
}

func TestHasErrors(t *testing.T) {
	if HasErrors(nil) {
		t.Fatalf("expected no errors for nil issues")
	}
	if !HasErrors([]Issue{{Severity: SeverityError}}) {
		t.Fatalf("expected HasErrors true for an error-severity issue")
	}
	if HasErrors([]Issue{{Severity: SeverityWarning}}) {
		t.Fatalf("expected HasErrors false for warning-only issues")
	}
}
