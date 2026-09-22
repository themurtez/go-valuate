package sde

import (
	"math"
	"testing"

	"github.com/themurtez/go-valuate/valuation"
)

func TestCalculate_NormalCase(t *testing.T) {
	res := Calculate(Input{MaintainableSDE: 300000, Multiple: 2.5})

	if !res.Available {
		t.Fatalf("expected Available, got errors: %+v", res.Errors)
	}
	if res.EquityValue != 750000 {
		t.Errorf("EquityValue = %v, want 750000", res.EquityValue)
	}
	if res.ValueType != valuation.ValueTypeEquity {
		t.Errorf("ValueType = %v, want %v", res.ValueType, valuation.ValueTypeEquity)
	}
	if res.Method != valuation.CodeSDEMultiple {
		t.Errorf("Method = %v, want %v", res.Method, valuation.CodeSDEMultiple)
	}
	if res.MethodVersion != Version {
		t.Errorf("MethodVersion = %v, want %v", res.MethodVersion, Version)
	}
	if len(res.Warnings) != 0 {
		t.Errorf("expected no warnings, got %+v", res.Warnings)
	}
	if len(res.Errors) != 0 {
		t.Errorf("expected no errors, got %+v", res.Errors)
	}
}

func TestCalculate_CalculationTrace(t *testing.T) {
	res := Calculate(Input{MaintainableSDE: 174000, Multiple: 2.0})

	if len(res.Steps) != 1 {
		t.Fatalf("expected exactly 1 step, got %d: %+v", len(res.Steps), res.Steps)
	}
	const want = 348000
	if res.Steps[0].Value != want {
		t.Errorf("step value = %v, want %v", res.Steps[0].Value, want)
	}
	if res.EquityValue != want {
		t.Errorf("EquityValue = %v, want %v", res.EquityValue, want)
	}
	if res.Steps[0].Label != "Equity Value = Maintainable SDE x Multiple" {
		t.Errorf("unexpected step label: %q", res.Steps[0].Label)
	}
}

func TestCalculate_ZeroSDE(t *testing.T) {
	res := Calculate(Input{MaintainableSDE: 0, Multiple: 2.5})

	if !res.Available {
		t.Fatalf("zero SDE should still produce an Available result, got errors: %+v", res.Errors)
	}
	if res.EquityValue != 0 {
		t.Errorf("EquityValue = %v, want 0", res.EquityValue)
	}
	if len(res.Warnings) == 0 {
		t.Fatal("expected a warning for zero SDE")
	}
	found := false
	for _, w := range res.Warnings {
		if w.Code == IssueNonPositiveSDE {
			found = true
		}
	}
	if !found {
		t.Errorf("expected IssueNonPositiveSDE warning, got %+v", res.Warnings)
	}
}

func TestCalculate_NegativeSDE(t *testing.T) {
	res := Calculate(Input{MaintainableSDE: -50000, Multiple: 2.5})

	if !res.Available {
		t.Fatalf("negative SDE should still produce an Available result (not silently clamped), got errors: %+v", res.Errors)
	}
	// Must NOT silently produce a normal positive valuation from negative
	// SDE: the result should be exactly the negative product, not zero or
	// positive.
	want := -50000 * 2.5
	if res.EquityValue != want {
		t.Errorf("EquityValue = %v, want %v (must not clamp negative SDE to a positive value)", res.EquityValue, want)
	}
	if res.EquityValue >= 0 {
		t.Errorf("EquityValue = %v, expected a negative value from negative SDE", res.EquityValue)
	}
	foundWarning := false
	for _, w := range res.Warnings {
		if w.Code == IssueNonPositiveSDE {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Errorf("expected IssueNonPositiveSDE warning, got %+v", res.Warnings)
	}
}

func TestCalculate_InvalidMultiple(t *testing.T) {
	tests := []struct {
		name     string
		multiple float64
	}{
		{"zero", 0},
		{"negative", -1.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := Calculate(Input{MaintainableSDE: 300000, Multiple: tt.multiple})
			if res.Available {
				t.Fatalf("expected Available=false for multiple=%v", tt.multiple)
			}
			if res.EquityValue != 0 {
				t.Errorf("EquityValue = %v, want 0 when unavailable", res.EquityValue)
			}
			if !valuation.HasErrors(res.Errors) {
				t.Fatalf("expected a blocking error, got %+v", res.Errors)
			}
			found := false
			for _, e := range res.Errors {
				if e.Code == IssueNonPositiveMultiple {
					found = true
				}
			}
			if !found {
				t.Errorf("expected IssueNonPositiveMultiple, got %+v", res.Errors)
			}
		})
	}
}

func TestCalculate_NonFiniteInputsRejected(t *testing.T) {
	tests := []struct {
		name  string
		input Input
	}{
		{"NaN SDE", Input{MaintainableSDE: math.NaN(), Multiple: 2.0}},
		{"Inf SDE", Input{MaintainableSDE: math.Inf(1), Multiple: 2.0}},
		{"NaN multiple", Input{MaintainableSDE: 300000, Multiple: math.NaN()}},
		{"-Inf multiple", Input{MaintainableSDE: 300000, Multiple: math.Inf(-1)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := Calculate(tt.input)
			if res.Available {
				t.Fatalf("expected Available=false for %s", tt.name)
			}
			if !valuation.HasErrors(res.Errors) {
				t.Fatalf("expected a blocking error for %s, got %+v", tt.name, res.Errors)
			}
		})
	}
}

func TestCalculate_HighMultiple(t *testing.T) {
	res := Calculate(Input{MaintainableSDE: 200000, Multiple: 6.0})
	if !res.Available {
		t.Fatalf("expected Available, got errors: %+v", res.Errors)
	}
	if res.EquityValue != 1200000 {
		t.Errorf("EquityValue = %v, want 1200000", res.EquityValue)
	}
	// A high multiple is unusual but not itself invalid; no warning is
	// expected purely for multiple magnitude.
	for _, w := range res.Warnings {
		t.Errorf("unexpected warning for a merely high (not invalid) multiple: %+v", w)
	}
}

func TestCalculate_EquityBridgeNotRequestedByDefault(t *testing.T) {
	res := Calculate(Input{MaintainableSDE: 300000, Multiple: 2.5})
	if res.Bridge.Available {
		t.Errorf("expected Bridge.Available=false when EquityBridge.Requested is false, got %+v", res.Bridge)
	}
}

func TestCalculate_EquityBridgeRequested(t *testing.T) {
	res := Calculate(Input{
		MaintainableSDE: 300000, Multiple: 2.5,
		EquityBridge: EquityBridgeInput{
			Requested: true, ExcessCash: 50000, ShortTermDebt: 10000, LongTermDebt: 90000,
		},
	})
	if !res.Available {
		t.Fatalf("expected Available, got errors: %+v", res.Errors)
	}
	if !res.Bridge.Available {
		t.Fatalf("expected Bridge.Available=true, got %+v", res.Bridge)
	}
	baseEquity := 750000.0
	if res.Bridge.EnterpriseValue != baseEquity {
		t.Errorf("Bridge.EnterpriseValue (base) = %v, want %v", res.Bridge.EnterpriseValue, baseEquity)
	}
	wantBridged := baseEquity + 50000 - (10000 + 90000)
	if res.Bridge.EquityValue != wantBridged {
		t.Errorf("Bridge.EquityValue = %v, want %v", res.Bridge.EquityValue, wantBridged)
	}
	if res.Bridge.TotalDebt != 100000 {
		t.Errorf("Bridge.TotalDebt = %v, want 100000", res.Bridge.TotalDebt)
	}
	if len(res.Bridge.DebtComponents) != 2 {
		t.Errorf("expected 2 debt components, got %+v", res.Bridge.DebtComponents)
	}
}

func TestCalculate_InputEchoedInResult(t *testing.T) {
	in := Input{MaintainableSDE: 174000, Multiple: 2.2}
	res := Calculate(in)
	if res.Input != in {
		t.Errorf("Result.Input = %+v, want %+v", res.Input, in)
	}
}
