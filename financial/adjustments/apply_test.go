package adjustments

import (
	"math"
	"testing"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/metrics"
)

// snapshotWithEBITDASDE builds a minimal metrics.Snapshot with EBITDA, SDE,
// and OwnerCompensation set as if metrics.Calculate had produced them,
// without needing a full financial.FinancialDataset for every test.
func snapshotWithEBITDASDE(period string, ebitda, ownerComp float64, ebitdaAvailable, sdeAvailable bool) metrics.Snapshot {
	s := metrics.Snapshot{Period: financial.Period(period)}
	if ebitdaAvailable {
		s.EBITDA = metrics.AvailableValue(ebitda)
	}
	s.OwnerCompensation = metrics.AvailableValue(ownerComp)
	if sdeAvailable {
		s.SDE = metrics.AvailableValue(ebitda + ownerComp)
	}
	return s
}

func TestApply_PositiveAddBack(t *testing.T) {
	snap := snapshotWithEBITDASDE("2025", 400000, 0, true, true)
	adjs := []Adjustment{
		{ID: "a1", Period: "2025", Type: TypeOneTimeExpense, Amount: 15000, Reason: "one-time equipment repair", Included: true},
	}
	res := Apply(snap, adjs)

	if !res.EBITDABridge.BaseAvailable || res.EBITDABridge.NormalizedValue != 415000 {
		t.Fatalf("EBITDA bridge = %+v, want normalized 415000", res.EBITDABridge)
	}
	if len(res.EBITDABridge.Applied) != 1 || res.EBITDABridge.Applied[0].SignedAmount != 15000 {
		t.Errorf("expected one applied line of +15000, got %+v", res.EBITDABridge.Applied)
	}
	if res.SDEBridge.NormalizedValue != 415000 {
		t.Errorf("SDE bridge normalized = %v, want 415000", res.SDEBridge.NormalizedValue)
	}
}

func TestApply_NegativeNormalizationAdjustment(t *testing.T) {
	snap := snapshotWithEBITDASDE("2025", 400000, 0, true, true)
	adjs := []Adjustment{
		{ID: "a1", Period: "2025", Type: TypeNonOperatingIncome, Amount: 25000, Reason: "one-time gain on asset sale", Included: true},
	}
	res := Apply(snap, adjs)

	if res.EBITDABridge.NormalizedValue != 375000 {
		t.Fatalf("EBITDA normalized = %v, want 375000 (400000 - 25000)", res.EBITDABridge.NormalizedValue)
	}
	if res.EBITDABridge.Applied[0].SignedAmount != -25000 {
		t.Errorf("signed amount = %v, want -25000", res.EBITDABridge.Applied[0].SignedAmount)
	}
}

func TestApply_OwnerCompensationNormalization_TargetsSDEOnlyByDefault(t *testing.T) {
	// Owner drew $180k; a market-rate GM replacement would cost $90k.
	// Caller supplies the *difference* ($90k) as the adjustment amount.
	snap := snapshotWithEBITDASDE("2025", 300000, 180000, true, true) // SDE = 480000
	adjs := []Adjustment{
		{ID: "a1", Period: "2025", Type: TypeOwnerCompensationNormalization, Amount: 90000, Effect: EffectDecrease, Reason: "normalize to market-rate GM salary", Included: true},
	}
	res := Apply(snap, adjs)

	if len(res.EBITDABridge.Applied) != 0 {
		t.Errorf("expected owner comp normalization to NOT apply to EBITDA bridge by default, got %+v", res.EBITDABridge.Applied)
	}
	if res.EBITDABridge.NormalizedValue != 300000 {
		t.Errorf("EBITDA normalized = %v, want unchanged 300000", res.EBITDABridge.NormalizedValue)
	}
	foundSkip := false
	for _, sk := range res.Skipped {
		if sk.Adjustment.ID == "a1" && sk.Reason == SkipNotTargeted {
			foundSkip = true
		}
	}
	if !foundSkip {
		t.Error("expected owner comp normalization to appear in Skipped for EBITDA bridge with SkipNotTargeted")
	}

	if res.SDEBridge.NormalizedValue != 390000 {
		t.Fatalf("SDE normalized = %v, want 390000 (480000 - 90000)", res.SDEBridge.NormalizedValue)
	}
}

func TestApply_DoubleCountPrevention_OwnerCompNotAddedTwice(t *testing.T) {
	// Base SDE already includes the full owner compensation (per
	// metrics.sde's EBITDA+OwnerComp formula). Applying a normalization
	// adjustment must change SDE by exactly the adjustment's own Amount,
	// never by OwnerCompensation's raw value on top of that.
	const ownerComp = 150000.0
	const ebitda = 250000.0
	snap := snapshotWithEBITDASDE("2025", ebitda, ownerComp, true, true)
	baselineSDE := snap.SDE.Value
	if baselineSDE != ebitda+ownerComp {
		t.Fatalf("test setup: baseline SDE = %v, want %v", baselineSDE, ebitda+ownerComp)
	}

	const normalizationDelta = 60000.0
	adjs := []Adjustment{
		{ID: "a1", Period: "2025", Type: TypeOwnerCompensationNormalization, Amount: normalizationDelta, Effect: EffectDecrease, Reason: "normalize owner comp to market rate", Included: true},
	}
	res := Apply(snap, adjs)

	wantSDE := baselineSDE - normalizationDelta
	if res.SDEBridge.NormalizedValue != wantSDE {
		t.Fatalf("normalized SDE = %v, want %v (baseline %v - delta %v, NOT baseline - full owner comp %v)",
			res.SDEBridge.NormalizedValue, wantSDE, baselineSDE, normalizationDelta, ownerComp)
	}
	// Explicitly assert the bug this test guards against: subtracting full
	// owner compensation again would produce baselineSDE - ownerComp, a
	// different (wrong) number given normalizationDelta != ownerComp.
	wrongDoubleCounted := baselineSDE - ownerComp
	if res.SDEBridge.NormalizedValue == wrongDoubleCounted {
		t.Fatalf("normalized SDE equals the double-counted (wrong) value %v", wrongDoubleCounted)
	}
}

func TestApply_EBITDAAdjustment(t *testing.T) {
	snap := snapshotWithEBITDASDE("2025", 200000, 50000, true, true)
	adjs := []Adjustment{
		{ID: "a1", Period: "2025", Type: TypePersonalVehicle, Amount: 8000, Reason: "personal use vehicle", Included: true},
	}
	res := Apply(snap, adjs)
	if res.EBITDABridge.NormalizedValue != 208000 {
		t.Errorf("EBITDA normalized = %v, want 208000", res.EBITDABridge.NormalizedValue)
	}
}

func TestApply_SDEAdjustment(t *testing.T) {
	snap := snapshotWithEBITDASDE("2025", 200000, 50000, true, true) // SDE = 250000
	adjs := []Adjustment{
		{ID: "a1", Period: "2025", Type: TypePersonalTravel, Amount: 5000, Targets: []Target{TargetSDE}, Reason: "personal travel", Included: true},
	}
	res := Apply(snap, adjs)
	if len(res.EBITDABridge.Applied) != 0 {
		t.Errorf("expected no EBITDA-targeted adjustments, got %+v", res.EBITDABridge.Applied)
	}
	if res.SDEBridge.NormalizedValue != 255000 {
		t.Errorf("SDE normalized = %v, want 255000", res.SDEBridge.NormalizedValue)
	}
}

func TestApply_NoAdjustments(t *testing.T) {
	snap := snapshotWithEBITDASDE("2025", 200000, 0, true, true)
	res := Apply(snap, nil)
	if res.EBITDABridge.NormalizedValue != 200000 || res.SDEBridge.NormalizedValue != 200000 {
		t.Errorf("expected normalized values to equal base values with no adjustments, got %+v / %+v", res.EBITDABridge, res.SDEBridge)
	}
	if len(res.EBITDABridge.Applied) != 0 || len(res.Skipped) != 0 {
		t.Errorf("expected no applied or skipped lines, got %+v / %+v", res.EBITDABridge.Applied, res.Skipped)
	}
}

func TestApply_MultipleAdjustments(t *testing.T) {
	snap := snapshotWithEBITDASDE("2025", 300000, 0, true, true)
	adjs := []Adjustment{
		{ID: "a1", Period: "2025", Type: TypeOneTimeExpense, Amount: 10000, Reason: "one-time repair", Included: true},
		{ID: "a2", Period: "2025", Type: TypeNonOperatingIncome, Amount: 4000, Reason: "remove interest income", Included: true},
		{ID: "a3", Period: "2025", Type: TypePersonalVehicle, Amount: 6000, Reason: "personal vehicle", Included: true},
	}
	res := Apply(snap, adjs)
	want := 300000.0 + 10000 - 4000 + 6000
	if res.EBITDABridge.NormalizedValue != want {
		t.Errorf("normalized EBITDA = %v, want %v", res.EBITDABridge.NormalizedValue, want)
	}
	if len(res.EBITDABridge.Applied) != 3 {
		t.Errorf("expected 3 applied lines, got %d", len(res.EBITDABridge.Applied))
	}
}

func TestApply_DuplicateAdjustmentID(t *testing.T) {
	snap := snapshotWithEBITDASDE("2025", 300000, 0, true, true)
	adjs := []Adjustment{
		{ID: "dup", Period: "2025", Type: TypeOneTimeExpense, Amount: 1000, Reason: "a", Included: true},
		{ID: "dup", Period: "2025", Type: TypeOneTimeExpense, Amount: 2000, Reason: "b", Included: true},
	}
	res := Apply(snap, adjs)
	if !HasErrors(res.Errors) {
		t.Fatal("expected a duplicate-ID error")
	}
	foundDup := false
	for _, e := range res.Errors {
		if e.Code == IssueDuplicateID {
			foundDup = true
		}
	}
	if !foundDup {
		t.Errorf("expected IssueDuplicateID among errors, got %+v", res.Errors)
	}
	// Both duplicate-ID adjustments should be skipped as invalid, so
	// neither corrupts the bridge.
	if len(res.EBITDABridge.Applied) != 0 {
		t.Errorf("expected duplicate-ID adjustments to be skipped, got applied %+v", res.EBITDABridge.Applied)
	}
}

func TestApply_UnavailableBaseMetric(t *testing.T) {
	snap := snapshotWithEBITDASDE("2025", 0, 0, false, false)
	adjs := []Adjustment{
		{ID: "a1", Period: "2025", Type: TypeOneTimeExpense, Amount: 1000, Reason: "x", Included: true},
	}
	res := Apply(snap, adjs)
	if res.EBITDABridge.BaseAvailable {
		t.Fatal("expected EBITDA base to be unavailable")
	}
	found := false
	for _, sk := range res.Skipped {
		if sk.Adjustment.ID == "a1" && sk.Reason == SkipBaseMetricUnavailable {
			found = true
		}
	}
	if !found {
		t.Errorf("expected SkipBaseMetricUnavailable in Skipped, got %+v", res.Skipped)
	}
}

func TestApply_NotIncludedAdjustmentIsSkipped(t *testing.T) {
	snap := snapshotWithEBITDASDE("2025", 300000, 0, true, true)
	adjs := []Adjustment{
		{ID: "a1", Period: "2025", Type: TypeOneTimeExpense, Amount: 5000, Reason: "proposed but not confirmed", Included: false},
	}
	res := Apply(snap, adjs)
	if res.EBITDABridge.NormalizedValue != 300000 {
		t.Errorf("normalized = %v, want unchanged 300000", res.EBITDABridge.NormalizedValue)
	}
	if len(res.Skipped) != 2 { // skipped from both EBITDA and SDE bridges
		t.Errorf("expected 2 skip entries (one per bridge), got %d: %+v", len(res.Skipped), res.Skipped)
	}
}

func TestApply_RelatedPartyRentRequiresExplicitEffect(t *testing.T) {
	snap := snapshotWithEBITDASDE("2025", 300000, 0, true, true)
	adjs := []Adjustment{
		{ID: "a1", Period: "2025", Type: TypeRelatedPartyRentAdjustment, Amount: 12000, Reason: "adjust to fair market rent", Included: true},
	}
	res := Apply(snap, adjs)
	if !HasErrors(res.Errors) {
		t.Fatal("expected an error for missing explicit Effect/Targets on related-party rent")
	}
}

func TestApply_RelatedPartyRentIncreaseAndDecrease(t *testing.T) {
	snap := snapshotWithEBITDASDE("2025", 300000, 0, true, true)

	// Below-market rent being replaced with fair-market rent: decreases
	// earnings.
	decRes := Apply(snap, []Adjustment{
		{ID: "a1", Period: "2025", Type: TypeRelatedPartyRentAdjustment, Amount: 12000, Effect: EffectDecrease, Targets: []Target{TargetEBITDA, TargetSDE}, Reason: "below-market rent to fair market", Included: true},
	})
	if decRes.EBITDABridge.NormalizedValue != 288000 {
		t.Errorf("decrease case: normalized = %v, want 288000", decRes.EBITDABridge.NormalizedValue)
	}

	// Above-market rent being replaced with fair-market rent: increases
	// earnings.
	incRes := Apply(snap, []Adjustment{
		{ID: "a1", Period: "2025", Type: TypeRelatedPartyRentAdjustment, Amount: 12000, Effect: EffectIncrease, Targets: []Target{TargetEBITDA, TargetSDE}, Reason: "above-market rent to fair market", Included: true},
	})
	if incRes.EBITDABridge.NormalizedValue != 312000 {
		t.Errorf("increase case: normalized = %v, want 312000", incRes.EBITDABridge.NormalizedValue)
	}
}

func TestValidate_UnknownPeriod(t *testing.T) {
	snap := snapshotWithEBITDASDE("2025", 300000, 0, true, true)
	adjs := []Adjustment{{ID: "a1", Period: "2024", Type: TypeOneTimeExpense, Amount: 1000, Reason: "x", Included: true}}
	issues := Validate(adjs, snap)
	found := false
	for _, i := range issues {
		if i.Code == IssueUnknownPeriod {
			found = true
		}
	}
	if !found {
		t.Errorf("expected IssueUnknownPeriod, got %+v", issues)
	}
}

func TestValidate_NonFiniteAmount(t *testing.T) {
	snap := snapshotWithEBITDASDE("2025", 300000, 0, true, true)
	adjs := []Adjustment{{ID: "a1", Period: "2025", Type: TypeOneTimeExpense, Amount: math.Inf(1), Reason: "x", Included: true}}
	issues := Validate(adjs, snap)
	found := false
	for _, i := range issues {
		if i.Code == IssueNonFiniteAmount {
			found = true
		}
	}
	if !found {
		t.Errorf("expected IssueNonFiniteAmount, got %+v", issues)
	}
}

func TestValidate_NegativeAmountNotRejected(t *testing.T) {
	snap := snapshotWithEBITDASDE("2025", 300000, 0, true, true)
	adjs := []Adjustment{{ID: "a1", Period: "2025", Type: TypeOneTimeExpense, Amount: -5000, Reason: "legit negative-sign source data", Included: true}}
	issues := Validate(adjs, snap)
	for _, i := range issues {
		if i.Severity == SeverityError {
			t.Errorf("did not expect a negative Amount alone to be an error, got %+v", i)
		}
	}
}

func TestValidate_SuspiciousDuplicate(t *testing.T) {
	snap := snapshotWithEBITDASDE("2025", 300000, 0, true, true)
	adjs := []Adjustment{
		{ID: "a1", Period: "2025", Type: TypeOneTimeExpense, Amount: 5000, Reason: "repair", Included: true},
		{ID: "a2", Period: "2025", Type: TypeOneTimeExpense, Amount: 5000, Reason: "repair (dup entry?)", Included: true},
	}
	issues := Validate(adjs, snap)
	count := 0
	for _, i := range issues {
		if i.Code == IssueSuspiciousDuplicate {
			count++
		}
	}
	if count != 2 {
		t.Errorf("expected 2 suspicious-duplicate warnings (one per adjustment), got %d: %+v", count, issues)
	}
}

func TestValidate_IncompatibleTarget(t *testing.T) {
	snap := snapshotWithEBITDASDE("2025", 300000, 0, true, true)
	adjs := []Adjustment{
		{ID: "a1", Period: "2025", Type: TypeCustom, Amount: 1000, Effect: EffectIncrease, Targets: []Target{"revenue"}, Reason: "x", Included: true},
	}
	issues := Validate(adjs, snap)
	found := false
	for _, i := range issues {
		if i.Code == IssueIncompatibleTarget {
			found = true
		}
	}
	if !found {
		t.Errorf("expected IssueIncompatibleTarget, got %+v", issues)
	}
}
