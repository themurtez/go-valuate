package review

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/themurtez/go-valuate/financial"
)

// assertRoundTrip marshals v, unmarshals into a fresh value of the same
// type via out, re-marshals, and asserts the two marshaled forms are
// byte-identical — the same pattern adjustments/roundtrip_test.go uses.
func assertRoundTrip[T any](t *testing.T, v T) {
	t.Helper()
	first, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	if !json.Valid(first) {
		t.Fatal("expected valid JSON output")
	}
	var out T
	if err := json.Unmarshal(first, &out); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	second, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("re-marshal failed: %v", err)
	}
	if string(first) != string(second) {
		t.Fatalf("did not round-trip byte-for-byte:\nfirst:  %s\nsecond: %s", first, second)
	}
	assertNoNaNOrInf(t, first)
}

// assertNoNaNOrInf scans a marshaled JSON payload for the literal tokens
// encoding/json would never itself produce for a NaN/Inf float64 (it
// returns a marshal error instead — see json.UnsupportedValueError) but
// which could slip in if this package ever marshaled a float64 field via a
// custom path. Mirrors valuation.ValidateFiniteSteps's
// math.IsNaN/math.IsInf-based philosophy, adapted to a marshaled-JSON
// context: if any of these substrings appear, something bypassed standard
// encoding/json float handling.
func assertNoNaNOrInf(t *testing.T, data []byte) {
	t.Helper()
	s := string(data)
	for _, bad := range []string{"NaN", "Infinity", "-Infinity"} {
		if contains(s, bad) {
			t.Errorf("marshaled JSON contains %q, which should never appear in valid financial JSON output: %s", bad, s)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (func() bool {
		for i := 0; i+len(substr) <= len(s); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	})()
}

// TestJSON_Plan_RoundTrip proves Plan round-trips byte-for-byte.
func TestJSON_Plan_RoundTrip(t *testing.T) {
	plan := Build(buildFullInputFixture(), DefaultPolicy())
	assertRoundTrip(t, plan)
}

// TestJSON_ReviewItem_EachKind proves every ReviewItem Kind's payload
// variant round-trips byte-for-byte.
func TestJSON_ReviewItem_EachKind(t *testing.T) {
	items := map[Kind]ReviewItem{
		KindClassification: {
			ID: "classification:row-1", Kind: KindClassification, Severity: SeverityBlocking, Required: true, Status: StatusPending,
			Classification: &ClassificationPayload{OriginalLabel: "Misc", ProposedCode: financial.CodeOpexOther, Confidence: 0.5, Source: "unknown",
				Alternatives: []ClassificationAlternative{{Code: financial.CodeCogsOther, Confidence: 0.4}}},
		},
		KindOCRText: {
			ID: "ocr-text:row-2:0", Kind: KindOCRText, Severity: SeverityWarning, Status: StatusPending,
			OCRText: &OCRTextPayload{OriginalText: "Adverising", FinalText: "Adverising", Confidence: 55, Page: 1, PixelBounds: &PixelBounds{X: 1, Y: 2, Width: 3, Height: 4}},
		},
		KindOCRNumeric: {
			ID: "ocr-numeric:row-3:2025", Kind: KindOCRNumeric, Severity: SeverityBlocking, Required: true, Period: "2025", Status: StatusPending,
			OCRNumeric: &OCRNumericPayload{OriginalText: "l234O", FinalText: "1234O", Ambiguous: true, Confidence: 80, Page: 0, Period: "2025"},
		},
		KindPeriod: {
			ID: "period:header:2", Kind: KindPeriod, Severity: SeverityBlocking, Required: true, Status: StatusPending,
			PeriodDetail: &PeriodPayload{OriginalLabel: "Current Year", ProposedPeriod: "current_year", ProposedPeriodType: "unknown", ColumnIndex: 2, Confidence: 0},
		},
		KindStructure: {
			ID: "structure:row-4", Kind: KindStructure, Severity: SeverityInfo, Status: StatusPending,
			Structure: &StructurePayload{Label: "Gross Profit", ProposedKind: financial.RowKindSubtotal, StatementType: financial.StatementIncomeStatement},
		},
		KindReconciliation: {
			ID: "reconciliation:BALANCE_SHEET_BALANCES:2025", Kind: KindReconciliation, Severity: SeverityError, Required: true, Period: "2025", Status: StatusPending,
			Reconciliation: &ReconciliationPayload{CheckCode: "BALANCE_SHEET_BALANCES", CheckStatus: "FAIL", Expected: ptrFloat(100), Actual: ptrFloat(90), Difference: ptrFloat(-10), Explanation: "does not balance"},
		},
		KindAdjustment: {
			ID: "adjustment:adj-1", Kind: KindAdjustment, Severity: SeverityInfo, Period: "2025", Status: StatusPending,
			Adjustment: &AdjustmentPayload{AdjustmentID: "adj-1", Type: "personal_vehicle", Amount: 7200, Effect: "increase", Reason: "personal truck", Period: "2025", Included: true},
		},
		KindValuationAssumption: {
			ID: "assumption:sde_multiple", Kind: KindValuationAssumption, Severity: SeverityInfo, Status: StatusPending,
			Assumption: &AssumptionPayload{SettingKey: "sde_multiple", CurrentValue: 2.5, SourceScope: "account"},
		},
	}
	for kind, item := range items {
		t.Run(string(kind), func(t *testing.T) {
			assertRoundTrip(t, item)
		})
	}
}

// TestJSON_Decision_EachItemKindPayload proves every Decision payload
// variant round-trips byte-for-byte.
func TestJSON_Decision_EachItemKindPayload(t *testing.T) {
	decisions := map[string]Decision{
		"classification": {ItemID: "classification:row-1", Action: ActionOverride, Classification: &ClassificationDecision{Code: financial.CodeOpexMarketing}},
		"ocr_numeric":    {ItemID: "ocr-numeric:row-2:2025", Action: ActionOverride, OCRNumeric: &OCRNumericDecision{Amount: 1234.5}},
		"structure":      {ItemID: "structure:row-3", Action: ActionOverride, Structure: &StructureDecision{RowKind: financial.RowKindTotal}},
		"period":         {ItemID: "period:header:0", Action: ActionOverride, PeriodOverride: &PeriodDecision{Period: "2025"}},
		"adjustment":     {ItemID: "adjustment:adj-1", Action: ActionOverride, Adjustment: &AdjustmentDecision{Included: true, Amount: 500, NewAmount: true, Reason: "revised"}},
		"assumption":     {ItemID: "assumption:sde_multiple", Action: ActionOverride, Assumption: &AssumptionDecision{Value: 3.0}},
	}
	for name, d := range decisions {
		t.Run(name, func(t *testing.T) {
			assertRoundTrip(t, d)
		})
	}
}

// TestJSON_ApplyResult_RoundTrip proves ApplyResult round-trips
// byte-for-byte.
func TestJSON_ApplyResult_RoundTrip(t *testing.T) {
	plan, source := classificationPlanFixture()
	decisions := []Decision{{ItemID: "classification:row-1", Action: ActionOverride, Classification: &ClassificationDecision{Code: financial.CodeOpexMarketing}}}
	result := Apply(source, plan, decisions)
	assertRoundTrip(t, result)
}

// TestJSON_Readiness_RoundTrip proves Readiness round-trips byte-for-byte.
func TestJSON_Readiness_RoundTrip(t *testing.T) {
	readiness := EvaluateReadiness([]ReviewItem{
		{ID: "a", Kind: KindClassification, Severity: SeverityBlocking, Status: StatusPending},
	})
	assertRoundTrip(t, readiness)
}

// TestJSON_EnumsAreStableStrings proves every enum type in this package's
// public API marshals as a JSON string (never a bare int), matching
// classification.Source/financial.RowKind/reconciliation.Status's existing
// convention.
func TestJSON_EnumsAreStableStrings(t *testing.T) {
	item := ReviewItem{ID: "x", Kind: KindClassification, Severity: SeverityBlocking, Status: StatusPending}
	b, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	if _, ok := raw["kind"].(string); !ok {
		t.Errorf("expected \"kind\" to marshal as a JSON string, got %T", raw["kind"])
	}
	if _, ok := raw["severity"].(string); !ok {
		t.Errorf("expected \"severity\" to marshal as a JSON string, got %T", raw["severity"])
	}
	if _, ok := raw["status"].(string); !ok {
		t.Errorf("expected \"status\" to marshal as a JSON string, got %T", raw["status"])
	}
	if raw["kind"] != string(KindClassification) {
		t.Errorf("expected kind value %q, got %v", KindClassification, raw["kind"])
	}
}

// TestJSON_NoNaNOrInf_Comprehensive proves a Plan/ApplyResult/Readiness
// built from finite-only input never marshals NaN/Inf. (encoding/json
// itself refuses to marshal a NaN/Inf float64 at all, returning an error —
// this test additionally proves this package's own float64 fields, when
// given ordinary finite domain values, never hit that failure mode.)
func TestJSON_NoNaNOrInf_Comprehensive(t *testing.T) {
	plan := Build(buildFullInputFixture(), DefaultPolicy())
	if _, err := json.Marshal(plan); err != nil {
		t.Fatalf("expected no marshal error for finite Plan, got %v", err)
	}

	// Sanity: encoding/json itself rejects NaN/Inf outright (this is the
	// standard library's own guarantee, not this package's) — confirmed
	// here so a future refactor introducing a custom MarshalJSON couldn't
	// silently reintroduce a NaN/Inf leak without this test catching it.
	badPayload := OCRNumericPayload{ParsedAmount: ptrFloat(math.NaN())}
	if _, err := json.Marshal(badPayload); err == nil {
		t.Fatal("expected json.Marshal to reject a NaN float64, but it succeeded")
	}
}
