package ai

import (
	"testing"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/adjustments"
)

func baseRequest() Request {
	return Request{
		Candidates: []SourceRow{
			{RowID: "row-1", Period: "2025", Label: "Legal Settlement", Code: financial.CodeOpexOther, StatementType: financial.StatementIncomeStatement, Amount: 42000},
			{RowID: "row-2", Period: "2025", Label: "Owner Auto Lease", Code: financial.CodeOpexVehicle, StatementType: financial.StatementIncomeStatement, Amount: 9600},
			{RowID: "row-3", Period: "2025", Label: "Gain on Asset Sale", Code: financial.CodeOtherIncome, StatementType: financial.StatementIncomeStatement, Amount: 30000},
			{RowID: "row-4", Period: "2025", Label: "Fire Damage Loss", Code: financial.CodeOtherExpense, StatementType: financial.StatementIncomeStatement, Amount: 18000},
			{RowID: "row-5", Period: "2025", Label: "Officer Compensation", Code: financial.CodeOpexOwnerComp, StatementType: financial.StatementIncomeStatement, Amount: 220000},
			{RowID: "row-6", Period: "2025", Label: "Rent - Related Party", Code: financial.CodeOpexRent, StatementType: financial.StatementIncomeStatement, Amount: 60000},
			{RowID: "row-heading", Period: "2025", Label: "Operating Expenses", StatementType: financial.StatementIncomeStatement, RowKind: financial.RowKindHeading, Amount: 0},
			{RowID: "row-ambiguous", Period: "2025", Label: "Misc Fee", Code: financial.CodeOpexOther, StatementType: financial.StatementIncomeStatement, Amount: 500, AmbiguousOCR: true},
		},
		AllowedTypes: BuildAllowedTypes(adjustments.AllTypes()),
	}
}

func mustValid(t *testing.T, req Request, s Suggestion) ValidatedSuggestion {
	t.Helper()
	out := ValidateSuggestions(req, []Suggestion{s})
	if len(out) != 1 {
		t.Fatalf("expected 1 result, got %d", len(out))
	}
	if !out[0].Valid {
		t.Fatalf("expected valid suggestion, got rejected: %+v", out[0].Issue)
	}
	return out[0]
}

func mustRejected(t *testing.T, req Request, s Suggestion, wantCode IssueCode) ValidatedSuggestion {
	t.Helper()
	out := ValidateSuggestions(req, []Suggestion{s})
	if len(out) != 1 {
		t.Fatalf("expected 1 result, got %d", len(out))
	}
	if out[0].Valid {
		t.Fatalf("expected suggestion to be rejected, got valid: %+v", out[0].Suggestion)
	}
	if out[0].Issue.Code != wantCode {
		t.Fatalf("expected issue code %s, got %s (%s)", wantCode, out[0].Issue.Code, out[0].Issue.Message)
	}
	return out[0]
}

// --- Valid suggestions -------------------------------------------------

func TestValidate_OneTimeExpense_Valid(t *testing.T) {
	req := baseRequest()
	s := Suggestion{
		SourceRowID: "row-1", Period: "2025", AdjustmentType: adjustments.TypeOneTimeExpense,
		Amount: 42000, Direction: DirectionIncreaseEarnings, Reason: "one-time legal settlement",
	}
	mustValid(t, req, s)
}

func TestValidate_NonOperatingIncomeRemoval_Valid(t *testing.T) {
	req := baseRequest()
	s := Suggestion{
		SourceRowID: "row-3", Period: "2025", AdjustmentType: adjustments.TypeNonOperatingIncome,
		Amount: 30000, Direction: DirectionDecreaseEarnings, Reason: "one-time asset sale gain",
	}
	mustValid(t, req, s)
}

func TestValidate_OwnerDiscretionaryExpense_Valid(t *testing.T) {
	req := baseRequest()
	s := Suggestion{
		SourceRowID: "row-2", Period: "2025", AdjustmentType: adjustments.TypePersonalVehicle,
		Amount: 9600, Direction: DirectionIncreaseEarnings, Reason: "personal vehicle lease run through the business",
	}
	mustValid(t, req, s)
}

func TestValidate_UnusualLoss_Valid(t *testing.T) {
	req := baseRequest()
	s := Suggestion{
		SourceRowID: "row-4", Period: "2025", AdjustmentType: adjustments.TypeUnusualLoss,
		Amount: 18000, Direction: DirectionIncreaseEarnings, Reason: "one-time fire damage loss",
	}
	mustValid(t, req, s)
}

func TestValidate_CustomType_EitherDirectionAllowed(t *testing.T) {
	req := baseRequest()
	inc := Suggestion{SourceRowID: "row-1", Period: "2025", AdjustmentType: adjustments.TypeCustom, Amount: 42000, Direction: DirectionIncreaseEarnings, Reason: "custom"}
	mustValid(t, req, inc)
	dec := Suggestion{SourceRowID: "row-3", Period: "2025", AdjustmentType: adjustments.TypeCustom, Amount: 30000, Direction: DirectionDecreaseEarnings, Reason: "custom"}
	mustValid(t, req, dec)
}

// --- Invalid provider suggestions --------------------------------------

func TestValidate_InventedAmount_Rejected(t *testing.T) {
	req := baseRequest()
	s := Suggestion{
		SourceRowID: "row-1", Period: "2025", AdjustmentType: adjustments.TypeOneTimeExpense,
		Amount: 99999, Direction: DirectionIncreaseEarnings, Reason: "invented",
	}
	mustRejected(t, req, s, IssueInventedAmount)
}

func TestValidate_UnknownRow_Rejected(t *testing.T) {
	req := baseRequest()
	s := Suggestion{
		SourceRowID: "row-does-not-exist", Period: "2025", AdjustmentType: adjustments.TypeOneTimeExpense,
		Amount: 42000, Direction: DirectionIncreaseEarnings, Reason: "?",
	}
	mustRejected(t, req, s, IssueUnknownSourceRow)
}

func TestValidate_WrongPeriod_Rejected(t *testing.T) {
	req := baseRequest()
	s := Suggestion{
		SourceRowID: "row-1", Period: "2024", AdjustmentType: adjustments.TypeOneTimeExpense,
		Amount: 42000, Direction: DirectionIncreaseEarnings, Reason: "?",
	}
	mustRejected(t, req, s, IssueWrongPeriod)
}

func TestValidate_InvalidAdjustmentType_Rejected(t *testing.T) {
	req := baseRequest()
	s := Suggestion{
		SourceRowID: "row-1", Period: "2025", AdjustmentType: adjustments.Type("NOT_A_REAL_TYPE"),
		Amount: 42000, Direction: DirectionIncreaseEarnings, Reason: "?",
	}
	mustRejected(t, req, s, IssueInvalidAdjustmentType)
}

func TestValidate_IncompatibleDirection_Rejected(t *testing.T) {
	req := baseRequest()
	// TypeOneTimeExpense's only valid effect is Increase; DECREASE_EARNINGS
	// must be rejected.
	s := Suggestion{
		SourceRowID: "row-1", Period: "2025", AdjustmentType: adjustments.TypeOneTimeExpense,
		Amount: 42000, Direction: DirectionDecreaseEarnings, Reason: "?",
	}
	mustRejected(t, req, s, IssueIncompatibleDirection)
}

func TestValidate_StructuralRow_Rejected(t *testing.T) {
	req := baseRequest()
	s := Suggestion{
		SourceRowID: "row-heading", Period: "2025", AdjustmentType: adjustments.TypeOneTimeExpense,
		Amount: 0, Direction: DirectionIncreaseEarnings, Reason: "?",
	}
	mustRejected(t, req, s, IssueStructuralSourceRow)
}

func TestValidate_AmbiguousOCRAmount_Rejected(t *testing.T) {
	req := baseRequest()
	s := Suggestion{
		SourceRowID: "row-ambiguous", Period: "2025", AdjustmentType: adjustments.TypeOneTimeExpense,
		Amount: 500, Direction: DirectionIncreaseEarnings, Reason: "?",
	}
	mustRejected(t, req, s, IssueAmbiguousSourceAmount)
}

func TestValidate_DuplicateSuggestion_SecondRejected(t *testing.T) {
	req := baseRequest()
	s := Suggestion{
		SourceRowID: "row-1", Period: "2025", AdjustmentType: adjustments.TypeOneTimeExpense,
		Amount: 42000, Direction: DirectionIncreaseEarnings, Reason: "first",
	}
	dup := s
	dup.Reason = "duplicate"

	out := ValidateSuggestions(req, []Suggestion{s, dup})
	if len(out) != 2 {
		t.Fatalf("expected 2 results, got %d", len(out))
	}
	if !out[0].Valid {
		t.Fatalf("expected first suggestion valid, got rejected: %+v", out[0].Issue)
	}
	if out[1].Valid || out[1].Issue.Code != IssueDuplicateSuggestion {
		t.Fatalf("expected second suggestion rejected as duplicate, got %+v", out[1])
	}
}

func TestValidate_NonFiniteAmount_Rejected(t *testing.T) {
	req := baseRequest()
	s := Suggestion{
		SourceRowID: "row-1", Period: "2025", AdjustmentType: adjustments.TypeOneTimeExpense,
		Amount: nan(), Direction: DirectionIncreaseEarnings, Reason: "?",
	}
	mustRejected(t, req, s, IssueNonFiniteAmount)
}

func nan() float64 {
	var zero float64
	return zero / zero
}

// --- User-input-required -------------------------------------------------

func TestValidate_OwnerCompensationNormalization_RequiresUserInput(t *testing.T) {
	req := baseRequest()
	s := Suggestion{
		SourceRowID: "row-5", Period: "2025", AdjustmentType: adjustments.TypeOwnerCompensationNormalization,
		Amount: 220000, Direction: DirectionDecreaseEarnings, Reason: "officer compensation appears potentially above market rate; requires normalization review",
		RequiresUserInput: true,
	}
	out := mustValid(t, req, s)
	if !out.Suggestion.RequiresUserInput {
		t.Fatal("expected RequiresUserInput true to survive validation")
	}
	if out.Issue == nil || out.Issue.Code != IssueMissingUserInput || out.Issue.Severity != SeverityWarning {
		t.Fatalf("expected a SeverityWarning IssueMissingUserInput note, got %+v", out.Issue)
	}
}

func TestValidate_RelatedPartyRent_RequiresUserInput(t *testing.T) {
	req := baseRequest()
	s := Suggestion{
		SourceRowID: "row-6", Period: "2025", AdjustmentType: adjustments.TypeRelatedPartyRentAdjustment,
		Amount: 60000, Direction: DirectionDecreaseEarnings, Reason: "rent paid to a related party; requires fair-market rent comparison",
		RequiresUserInput: true,
	}
	out := mustValid(t, req, s)
	if !out.Suggestion.RequiresUserInput {
		t.Fatal("expected RequiresUserInput true to survive validation")
	}
	if out.Issue == nil || out.Issue.Code != IssueMissingUserInput {
		t.Fatalf("expected IssueMissingUserInput note, got %+v", out.Issue)
	}
}

func TestToAdjustments_RequiresUserInput_NoInventedAmount(t *testing.T) {
	s := Suggestion{
		SourceRowID: "row-5", Period: "2025", AdjustmentType: adjustments.TypeOwnerCompensationNormalization,
		Amount: 220000, Direction: DirectionDecreaseEarnings, Reason: "needs review", RequiresUserInput: true,
	}
	adjs := ToAdjustments([]Suggestion{s}, "")
	if len(adjs) != 1 {
		t.Fatalf("expected 1 adjustment, got %d", len(adjs))
	}
	if adjs[0].Amount != 0 {
		t.Errorf("expected amount 0 (no invented replacement value) for a requires-user-input adjustment, got %v", adjs[0].Amount)
	}
	if adjs[0].Included {
		t.Error("expected Included false for any AI-suggested adjustment")
	}
	if adjs[0].Effect != "" {
		t.Errorf("expected no Effect set for a requires-user-input adjustment, got %q", adjs[0].Effect)
	}
}

func TestToAdjustments_NeverIncluded(t *testing.T) {
	s := Suggestion{
		SourceRowID: "row-1", Period: "2025", AdjustmentType: adjustments.TypeOneTimeExpense,
		Amount: 42000, Direction: DirectionIncreaseEarnings, Reason: "one-time",
	}
	adjs := ToAdjustments([]Suggestion{s}, "")
	if len(adjs) != 1 {
		t.Fatalf("expected 1 adjustment, got %d", len(adjs))
	}
	if adjs[0].Included {
		t.Fatal("expected Included false: AI must never apply an adjustment itself")
	}
	if adjs[0].Amount != 42000 {
		t.Errorf("expected amount 42000 copied verbatim, got %v", adjs[0].Amount)
	}
	if adjs[0].Effect != adjustments.EffectIncrease {
		t.Errorf("expected EffectIncrease, got %q", adjs[0].Effect)
	}
}
