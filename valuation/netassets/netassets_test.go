package netassets

import (
	"math"
	"testing"

	"github.com/themurtez/go-valuate/valuation"
)

func TestCalculate_NormalPositiveNAV(t *testing.T) {
	res := Calculate(Input{
		Assets: []AssetItem{
			{Label: "Cash", Amount: 63000},
			{Label: "Accounts Receivable", Amount: 44500},
			{Label: "Inventory", Amount: 17800},
			{Label: "Fixed Assets (net book value)", Amount: 87000},
		},
		Liabilities: []LiabilityItem{
			{Label: "Accounts Payable", Amount: 24500},
			{Label: "Other Current Liabilities", Amount: 4400},
			{Label: "Short-Term Debt", Amount: 8000},
			{Label: "Long-Term Debt", Amount: 47000},
		},
	})

	if !res.Available {
		t.Fatalf("expected Available, got errors: %+v", res.Errors)
	}
	if res.ValueType != valuation.ValueTypeAsset {
		t.Errorf("ValueType = %v, want %v", res.ValueType, valuation.ValueTypeAsset)
	}
	if res.Method != valuation.CodeAdjustedNetAssetValue {
		t.Errorf("Method = %v, want %v", res.Method, valuation.CodeAdjustedNetAssetValue)
	}
	wantAssets := 63000.0 + 44500 + 17800 + 87000
	if res.TotalAdjustedAssets != wantAssets {
		t.Errorf("TotalAdjustedAssets = %v, want %v", res.TotalAdjustedAssets, wantAssets)
	}
	wantLiabilities := 24500.0 + 4400 + 8000 + 47000
	if res.TotalAdjustedLiabilities != wantLiabilities {
		t.Errorf("TotalAdjustedLiabilities = %v, want %v", res.TotalAdjustedLiabilities, wantLiabilities)
	}
	wantNAV := wantAssets - wantLiabilities
	if res.AdjustedNetAssetValue != wantNAV {
		t.Errorf("AdjustedNetAssetValue = %v, want %v", res.AdjustedNetAssetValue, wantNAV)
	}
	if len(res.AssetComponents) != 4 {
		t.Errorf("expected 4 asset components, got %d", len(res.AssetComponents))
	}
	if len(res.LiabilityComponents) != 4 {
		t.Errorf("expected 4 liability components, got %d", len(res.LiabilityComponents))
	}
	if len(res.Errors) != 0 || len(res.Warnings) != 0 {
		t.Errorf("expected no issues, got errors=%+v warnings=%+v", res.Errors, res.Warnings)
	}
}

func TestCalculate_LiabilitiesExceedAssets(t *testing.T) {
	res := Calculate(Input{
		Assets:      []AssetItem{{Label: "Cash", Amount: 10000}},
		Liabilities: []LiabilityItem{{Label: "Long-Term Debt", Amount: 500000}},
	})

	if !res.Available {
		t.Fatalf("expected Available=true for an insolvent NAV, got errors: %+v", res.Errors)
	}
	if res.AdjustedNetAssetValue != -490000 {
		t.Errorf("AdjustedNetAssetValue = %v, want -490000 (must not clamp to zero)", res.AdjustedNetAssetValue)
	}
	found := false
	for _, w := range res.Warnings {
		if w.Code == IssueLiabilitiesExceedAssets {
			found = true
		}
	}
	if !found {
		t.Errorf("expected IssueLiabilitiesExceedAssets warning, got %+v", res.Warnings)
	}
}

func TestCalculate_AdjustedAssetOverrides(t *testing.T) {
	res := Calculate(Input{
		Assets: []AssetItem{
			{Label: "Fixed Assets (book value)", Amount: 145000, SourceCode: "BS_FIXED_ASSETS"},
			{Label: "Fixed Assets (appraisal fair-value adjustment)", Amount: 55000, IsOverride: true, Notes: "per 2025 appraisal, replacing book value uplift"},
		},
	})
	if !res.Available {
		t.Fatalf("expected Available, got errors: %+v", res.Errors)
	}
	if res.TotalAdjustedAssets != 200000 {
		t.Errorf("TotalAdjustedAssets = %v, want 200000", res.TotalAdjustedAssets)
	}
	// The override item must be traceable in the itemized components, not
	// silently merged away.
	foundOverride := false
	for i, item := range res.Input.Assets {
		if item.IsOverride {
			foundOverride = true
			if res.AssetComponents[i].Amount != 55000 {
				t.Errorf("override component amount = %v, want 55000", res.AssetComponents[i].Amount)
			}
		}
	}
	if !foundOverride {
		t.Error("expected an IsOverride asset item to be echoed in Result.Input.Assets")
	}
}

func TestCalculate_ZeroAssets(t *testing.T) {
	res := Calculate(Input{Assets: nil, Liabilities: []LiabilityItem{{Label: "Accounts Payable", Amount: 5000}}})
	if res.Available {
		t.Fatal("expected Available=false with no assets")
	}
	if res.AdjustedNetAssetValue != 0 {
		t.Errorf("AdjustedNetAssetValue = %v, want 0", res.AdjustedNetAssetValue)
	}
	found := false
	for _, e := range res.Errors {
		if e.Code == IssueNoAssets {
			found = true
		}
	}
	if !found {
		t.Errorf("expected IssueNoAssets, got %+v", res.Errors)
	}
}

func TestCalculate_AssetsSummingToZeroIsNotAnError(t *testing.T) {
	// Distinct from TestCalculate_ZeroAssets: here Assets is non-empty but
	// its total happens to be zero (e.g. a fully written-down asset base).
	res := Calculate(Input{
		Assets: []AssetItem{
			{Label: "Fixed Assets (fully written down)", Amount: 0},
		},
	})
	if !res.Available {
		t.Fatalf("expected Available=true when Assets is non-empty but sums to zero, got errors: %+v", res.Errors)
	}
	if res.TotalAdjustedAssets != 0 {
		t.Errorf("TotalAdjustedAssets = %v, want 0", res.TotalAdjustedAssets)
	}
}

func TestCalculate_NoLiabilitiesIsValid(t *testing.T) {
	res := Calculate(Input{Assets: []AssetItem{{Label: "Cash", Amount: 50000}}})
	if !res.Available {
		t.Fatalf("expected Available=true with no liabilities (debt-free business), got errors: %+v", res.Errors)
	}
	if res.AdjustedNetAssetValue != 50000 {
		t.Errorf("AdjustedNetAssetValue = %v, want 50000", res.AdjustedNetAssetValue)
	}
}

func TestCalculate_NegativeItemAmountWarns(t *testing.T) {
	res := Calculate(Input{
		Assets: []AssetItem{
			{Label: "Cash", Amount: 50000},
			{Label: "Impaired Goodwill Adjustment", Amount: -20000},
		},
	})
	if !res.Available {
		t.Fatalf("expected Available, got errors: %+v", res.Errors)
	}
	if res.TotalAdjustedAssets != 30000 {
		t.Errorf("TotalAdjustedAssets = %v, want 30000", res.TotalAdjustedAssets)
	}
	found := false
	for _, w := range res.Warnings {
		if w.Code == IssueNegativeItemAmount {
			found = true
		}
	}
	if !found {
		t.Errorf("expected IssueNegativeItemAmount warning, got %+v", res.Warnings)
	}
}

func TestCalculate_NonFiniteInputsRejected(t *testing.T) {
	tests := []Input{
		{Assets: []AssetItem{{Label: "Cash", Amount: math.NaN()}}},
		{Assets: []AssetItem{{Label: "Cash", Amount: 1000}}, Liabilities: []LiabilityItem{{Label: "Debt", Amount: math.Inf(1)}}},
	}
	for i, in := range tests {
		res := Calculate(in)
		if res.Available {
			t.Errorf("case %d: expected Available=false", i)
		}
		if !valuation.HasErrors(res.Errors) {
			t.Errorf("case %d: expected a blocking error", i)
		}
	}
}

func TestCalculate_DoesNotAssumeBookEqualsFairValue(t *testing.T) {
	// Two items with identical labels/amounts but different IsOverride
	// flags must both be carried through as supplied — this package
	// applies no implicit fair-value adjustment of its own.
	bookOnly := Calculate(Input{Assets: []AssetItem{{Label: "Fixed Assets", Amount: 100000, IsOverride: false}}})
	overridden := Calculate(Input{Assets: []AssetItem{{Label: "Fixed Assets", Amount: 100000, IsOverride: true}}})
	if bookOnly.TotalAdjustedAssets != overridden.TotalAdjustedAssets {
		t.Errorf("book-value and overridden items with the same Amount should produce the same total; this package must not silently adjust either")
	}
	if bookOnly.Input.Assets[0].IsOverride == overridden.Input.Assets[0].IsOverride {
		t.Error("expected the IsOverride flag to be preserved distinctly")
	}
}

func TestCalculate_ResultEnvelopeIsValid(t *testing.T) {
	res := Calculate(Input{Assets: []AssetItem{{Label: "Cash", Amount: 200000}}})
	if issues := valuation.ValidateResultEnvelope(res.Method, res.MethodVersion, res.ValueType); len(issues) != 0 {
		t.Errorf("ValidateResultEnvelope() = %+v, want no issues", issues)
	}
	if issues := valuation.ValidateFiniteSteps(res.Steps); len(issues) != 0 {
		t.Errorf("ValidateFiniteSteps() = %+v, want no issues", issues)
	}
	if res.Method != Code {
		t.Errorf("Method = %v, want %v", res.Method, Code)
	}
	if res.MethodVersion != Version {
		t.Errorf("MethodVersion = %v, want %v", res.MethodVersion, Version)
	}
	if res.ValueType != valuation.ValueTypeAsset {
		t.Errorf("ValueType = %v, want %v", res.ValueType, valuation.ValueTypeAsset)
	}
}
