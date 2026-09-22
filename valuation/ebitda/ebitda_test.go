package ebitda

import (
	"math"
	"testing"

	"github.com/themurtez/go-valuate/valuation"
)

func TestCalculate_NormalEnterpriseValue(t *testing.T) {
	res := Calculate(Input{MaintainableEBITDA: 500000, Multiple: 4.0})

	if !res.Available {
		t.Fatalf("expected Available, got errors: %+v", res.Errors)
	}
	if res.EnterpriseValue != 2000000 {
		t.Errorf("EnterpriseValue = %v, want 2000000", res.EnterpriseValue)
	}
	if res.ValueType != valuation.ValueTypeEnterprise {
		t.Errorf("ValueType = %v, want %v", res.ValueType, valuation.ValueTypeEnterprise)
	}
	if res.Method != valuation.CodeEBITDAMultiple {
		t.Errorf("Method = %v, want %v", res.Method, valuation.CodeEBITDAMultiple)
	}
	if res.MethodVersion != Version {
		t.Errorf("MethodVersion = %v, want %v", res.MethodVersion, Version)
	}
	if res.Bridge.Available {
		t.Errorf("expected no bridge when not requested, got %+v", res.Bridge)
	}
	if len(res.Errors) != 0 || len(res.Warnings) != 0 {
		t.Errorf("expected no issues, got errors=%+v warnings=%+v", res.Errors, res.Warnings)
	}
}

func TestCalculate_EquityBridge(t *testing.T) {
	res := Calculate(Input{
		MaintainableEBITDA: 500000, Multiple: 4.0,
		EquityBridge: EquityBridgeInput{
			Requested: true, ExcessCash: 150000, ShortTermDebt: 50000, LongTermDebt: 400000,
		},
	})

	if !res.Available {
		t.Fatalf("expected Available, got errors: %+v", res.Errors)
	}
	if !res.Bridge.Available {
		t.Fatalf("expected Bridge.Available=true")
	}
	if res.Bridge.EnterpriseValue != 2000000 {
		t.Errorf("Bridge.EnterpriseValue = %v, want 2000000", res.Bridge.EnterpriseValue)
	}
	if res.Bridge.TotalDebt != 450000 {
		t.Errorf("Bridge.TotalDebt = %v, want 450000", res.Bridge.TotalDebt)
	}
	wantEquity := 2000000.0 + 150000 - 450000
	if res.Bridge.EquityValue != wantEquity {
		t.Errorf("Bridge.EquityValue = %v, want %v", res.Bridge.EquityValue, wantEquity)
	}
	// Bridge must be explicit, not buried: debt/cash never silently folded
	// into EnterpriseValue itself.
	if res.EnterpriseValue != 2000000 {
		t.Errorf("EnterpriseValue must remain the pre-bridge value; got %v", res.EnterpriseValue)
	}
}

func TestCalculate_ExcessCashOnly(t *testing.T) {
	res := Calculate(Input{
		MaintainableEBITDA: 500000, Multiple: 4.0,
		EquityBridge: EquityBridgeInput{Requested: true, ExcessCash: 200000},
	})
	if !res.Bridge.Available {
		t.Fatalf("expected Bridge.Available=true")
	}
	if res.Bridge.TotalDebt != 0 {
		t.Errorf("TotalDebt = %v, want 0", res.Bridge.TotalDebt)
	}
	if len(res.Bridge.DebtComponents) != 0 {
		t.Errorf("expected no debt components, got %+v", res.Bridge.DebtComponents)
	}
	wantEquity := 2000000.0 + 200000
	if res.Bridge.EquityValue != wantEquity {
		t.Errorf("Bridge.EquityValue = %v, want %v", res.Bridge.EquityValue, wantEquity)
	}
}

func TestCalculate_DebtOnly(t *testing.T) {
	res := Calculate(Input{
		MaintainableEBITDA: 500000, Multiple: 4.0,
		EquityBridge: EquityBridgeInput{Requested: true, ShortTermDebt: 30000, LongTermDebt: 470000},
	})
	if !res.Bridge.Available {
		t.Fatalf("expected Bridge.Available=true")
	}
	if res.Bridge.ExcessCash != 0 {
		t.Errorf("ExcessCash = %v, want 0", res.Bridge.ExcessCash)
	}
	wantEquity := 2000000.0 - 500000
	if res.Bridge.EquityValue != wantEquity {
		t.Errorf("Bridge.EquityValue = %v, want %v", res.Bridge.EquityValue, wantEquity)
	}
	if len(res.Bridge.DebtComponents) != 2 {
		t.Errorf("expected 2 debt components (short+long term), got %+v", res.Bridge.DebtComponents)
	}
}

func TestCalculate_OtherDebtComponent(t *testing.T) {
	res := Calculate(Input{
		MaintainableEBITDA: 500000, Multiple: 4.0,
		EquityBridge: EquityBridgeInput{Requested: true, OtherDebt: 75000},
	})
	if res.Bridge.TotalDebt != 75000 {
		t.Errorf("TotalDebt = %v, want 75000", res.Bridge.TotalDebt)
	}
	if len(res.Bridge.DebtComponents) != 1 || res.Bridge.DebtComponents[0].Label != "Other Debt" {
		t.Errorf("expected a single Other Debt component, got %+v", res.Bridge.DebtComponents)
	}
}

func TestCalculate_NegativeEBITDA(t *testing.T) {
	res := Calculate(Input{MaintainableEBITDA: -100000, Multiple: 4.0})

	if !res.Available {
		t.Fatalf("negative EBITDA should still produce Available=true, got errors: %+v", res.Errors)
	}
	if res.EnterpriseValue != -400000 {
		t.Errorf("EnterpriseValue = %v, want -400000 (must not clamp to zero/positive)", res.EnterpriseValue)
	}
	found := false
	for _, w := range res.Warnings {
		if w.Code == IssueNonPositiveEBITDA {
			found = true
		}
	}
	if !found {
		t.Errorf("expected IssueNonPositiveEBITDA warning, got %+v", res.Warnings)
	}
}

func TestCalculate_ZeroEBITDA(t *testing.T) {
	res := Calculate(Input{MaintainableEBITDA: 0, Multiple: 4.0})
	if !res.Available {
		t.Fatalf("expected Available, got errors: %+v", res.Errors)
	}
	if res.EnterpriseValue != 0 {
		t.Errorf("EnterpriseValue = %v, want 0", res.EnterpriseValue)
	}
	if len(res.Warnings) == 0 {
		t.Error("expected a warning for zero EBITDA")
	}
}

func TestCalculate_InvalidMultiple(t *testing.T) {
	for _, multiple := range []float64{0, -2.0} {
		res := Calculate(Input{MaintainableEBITDA: 500000, Multiple: multiple})
		if res.Available {
			t.Fatalf("multiple=%v: expected Available=false", multiple)
		}
		if res.EnterpriseValue != 0 {
			t.Errorf("multiple=%v: EnterpriseValue = %v, want 0", multiple, res.EnterpriseValue)
		}
		if !valuation.HasErrors(res.Errors) {
			t.Fatalf("multiple=%v: expected a blocking error", multiple)
		}
	}
}

func TestCalculate_NonFiniteInputsRejected(t *testing.T) {
	tests := []Input{
		{MaintainableEBITDA: math.NaN(), Multiple: 4.0},
		{MaintainableEBITDA: math.Inf(1), Multiple: 4.0},
		{MaintainableEBITDA: 500000, Multiple: math.NaN()},
		{MaintainableEBITDA: 500000, Multiple: 4.0, EquityBridge: EquityBridgeInput{Requested: true, ExcessCash: math.Inf(1)}},
	}
	for i, in := range tests {
		res := Calculate(in)
		if res.Available {
			t.Errorf("case %d: expected Available=false for %+v", i, in)
		}
	}
}

func TestCalculate_BridgeNotComputedWhenBaseInputInvalid(t *testing.T) {
	res := Calculate(Input{
		MaintainableEBITDA: 500000, Multiple: 0,
		EquityBridge: EquityBridgeInput{Requested: true, ExcessCash: 100000},
	})
	if res.Available {
		t.Fatal("expected Available=false")
	}
	if res.Bridge.Available {
		t.Errorf("bridge must not be computed when the base calculation is invalid, got %+v", res.Bridge)
	}
}

func TestCalculate_ResultEnvelopeIsValid(t *testing.T) {
	res := Calculate(Input{MaintainableEBITDA: 500000, Multiple: 3.5})
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
	if res.ValueType != valuation.ValueTypeEnterprise {
		t.Errorf("ValueType = %v, want %v", res.ValueType, valuation.ValueTypeEnterprise)
	}
}
