package capitalization

import (
	"math"
	"testing"

	"github.com/themurtez/go-valuate/valuation"
)

func TestCalculate_NormalCase(t *testing.T) {
	res := Calculate(Input{MaintainableEarnings: 200000, CapitalizationRate: 0.20})

	if !res.Available {
		t.Fatalf("expected Available, got errors: %+v", res.Errors)
	}
	if res.EquityValue != 1000000 {
		t.Errorf("EquityValue = %v, want 1000000", res.EquityValue)
	}
	if res.ValueType != valuation.ValueTypeEquity {
		t.Errorf("ValueType = %v, want %v", res.ValueType, valuation.ValueTypeEquity)
	}
	if res.Method != valuation.CodeCapitalizationOfEarnings {
		t.Errorf("Method = %v, want %v", res.Method, valuation.CodeCapitalizationOfEarnings)
	}
	if res.MethodVersion != Version {
		t.Errorf("MethodVersion = %v, want %v", res.MethodVersion, Version)
	}
	if len(res.Steps) != 1 {
		t.Fatalf("expected 1 step, got %+v", res.Steps)
	}
	if len(res.Errors) != 0 || len(res.Warnings) != 0 {
		t.Errorf("expected no issues, got errors=%+v warnings=%+v", res.Errors, res.Warnings)
	}
}

func TestCalculate_InvalidCapitalizationRate(t *testing.T) {
	for _, rate := range []float64{0, -0.1} {
		res := Calculate(Input{MaintainableEarnings: 200000, CapitalizationRate: rate})
		if res.Available {
			t.Fatalf("rate=%v: expected Available=false", rate)
		}
		if res.EquityValue != 0 {
			t.Errorf("rate=%v: EquityValue = %v, want 0", rate, res.EquityValue)
		}
		found := false
		for _, e := range res.Errors {
			if e.Code == IssueNonPositiveCapRate {
				found = true
			}
		}
		if !found {
			t.Errorf("rate=%v: expected IssueNonPositiveCapRate, got %+v", rate, res.Errors)
		}
	}
}

func TestCalculate_ZeroEarnings(t *testing.T) {
	res := Calculate(Input{MaintainableEarnings: 0, CapitalizationRate: 0.20})
	if !res.Available {
		t.Fatalf("zero earnings should still produce Available=true, got errors: %+v", res.Errors)
	}
	if res.EquityValue != 0 {
		t.Errorf("EquityValue = %v, want 0", res.EquityValue)
	}
	found := false
	for _, w := range res.Warnings {
		if w.Code == IssueNonPositiveEarnings {
			found = true
		}
	}
	if !found {
		t.Errorf("expected IssueNonPositiveEarnings warning, got %+v", res.Warnings)
	}
}

func TestCalculate_NegativeEarnings(t *testing.T) {
	res := Calculate(Input{MaintainableEarnings: -50000, CapitalizationRate: 0.25})
	if !res.Available {
		t.Fatalf("negative earnings should still produce Available=true, got errors: %+v", res.Errors)
	}
	if res.EquityValue != -200000 {
		t.Errorf("EquityValue = %v, want -200000 (must not clamp to zero/positive)", res.EquityValue)
	}
	found := false
	for _, w := range res.Warnings {
		if w.Code == IssueNonPositiveEarnings {
			found = true
		}
	}
	if !found {
		t.Errorf("expected IssueNonPositiveEarnings warning, got %+v", res.Warnings)
	}
}

func TestCalculate_NonFiniteInputsRejected(t *testing.T) {
	tests := []Input{
		{MaintainableEarnings: math.NaN(), CapitalizationRate: 0.2},
		{MaintainableEarnings: math.Inf(-1), CapitalizationRate: 0.2},
		{MaintainableEarnings: 200000, CapitalizationRate: math.NaN()},
		{MaintainableEarnings: 200000, CapitalizationRate: math.Inf(1)},
	}
	for i, in := range tests {
		res := Calculate(in)
		if res.Available {
			t.Errorf("case %d: expected Available=false for %+v", i, in)
		}
		if !valuation.HasErrors(res.Errors) {
			t.Errorf("case %d: expected a blocking error", i)
		}
	}
}

func TestCalculate_DoesNotInventCapRate(t *testing.T) {
	// A zero-value Input (no rate supplied at all) must be rejected, not
	// defaulted to some invented rate.
	res := Calculate(Input{MaintainableEarnings: 200000})
	if res.Available {
		t.Fatal("expected Available=false when no capitalization rate is supplied")
	}
}

func TestCalculate_ValueTypeAlwaysEquity(t *testing.T) {
	res := Calculate(Input{MaintainableEarnings: 500000, CapitalizationRate: 0.18})
	if res.ValueType != valuation.ValueTypeEquity {
		t.Errorf("ValueType = %v, want %v (capitalization of earnings is documented as always producing an equity value)", res.ValueType, valuation.ValueTypeEquity)
	}
}

func TestCalculate_ResultEnvelopeIsValid(t *testing.T) {
	res := Calculate(Input{MaintainableEarnings: 500000, CapitalizationRate: 0.18})
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
}
