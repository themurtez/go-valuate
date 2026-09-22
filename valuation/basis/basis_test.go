package basis

import (
	"testing"

	"github.com/themurtez/go-valuate/valuation"
)

func TestConvert_SameBasisIsDirect(t *testing.T) {
	in := ConversionInput{Method: valuation.CodeSDEMultiple, ValueType: valuation.ValueTypeEquity, Value: 547000}
	c := Convert(in, valuation.ValueTypeEquity)
	if c.Outcome != OutcomeDirect {
		t.Fatalf("Outcome = %v, want OutcomeDirect", c.Outcome)
	}
	if c.ConvertedValue != 547000 {
		t.Errorf("ConvertedValue = %v, want 547000", c.ConvertedValue)
	}
	if c.OriginalValue != 547000 || c.OriginalBasis != valuation.ValueTypeEquity {
		t.Errorf("original value/basis not echoed correctly: %+v", c)
	}
	if c.Bridge.Available {
		t.Errorf("expected no bridge for a direct match, got %+v", c.Bridge)
	}
}

func TestConvert_EnterpriseToEquity_UsesMethodsOwnBridge(t *testing.T) {
	// The task's own worked example:
	//   Enterprise Value = 1,800,000
	//   + Excess Cash      100,000
	//   - Debt             300,000
	//   Equity Value     = 1,600,000
	bridge := valuation.Bridge{
		Available:       true,
		EnterpriseValue: 1_800_000,
		ExcessCash:      100_000,
		TotalDebt:       300_000,
		EquityValue:     1_600_000,
	}
	in := ConversionInput{
		Method: valuation.CodeEBITDAMultiple, ValueType: valuation.ValueTypeEnterprise,
		Value: 1_800_000, Bridge: bridge,
	}
	c := Convert(in, valuation.ValueTypeEquity)
	if c.Outcome != OutcomeConverted {
		t.Fatalf("Outcome = %v, want OutcomeConverted", c.Outcome)
	}
	if c.ConvertedValue != 1_600_000 {
		t.Errorf("ConvertedValue = %v, want 1,600,000 (never the raw 1,800,000 enterprise value)", c.ConvertedValue)
	}
	if c.OriginalValue != 1_800_000 {
		t.Errorf("OriginalValue = %v, want the original enterprise value 1,800,000 preserved", c.OriginalValue)
	}
	if !c.Bridge.Available || c.Bridge.EquityValue != bridge.EquityValue || c.Bridge.EnterpriseValue != bridge.EnterpriseValue {
		t.Errorf("Bridge = %+v, want the method's own bridge echoed exactly (%+v)", c.Bridge, bridge)
	}
}

func TestConvert_EnterpriseToEquity_NoBridgeAvailable_Excluded(t *testing.T) {
	in := ConversionInput{
		Method: valuation.CodeDCF, ValueType: valuation.ValueTypeEnterprise, Value: 900000,
		// Bridge left zero-value: Available == false.
	}
	c := Convert(in, valuation.ValueTypeEquity)
	if c.Outcome != OutcomeExcluded {
		t.Fatalf("Outcome = %v, want OutcomeExcluded", c.Outcome)
	}
	if c.ExclusionReason == "" {
		t.Error("expected a non-empty ExclusionReason")
	}
	if c.ConvertedValue != 0 {
		t.Errorf("ConvertedValue = %v, want 0 for an excluded conversion (never a fabricated number)", c.ConvertedValue)
	}
}

func TestConvert_AssetToEquity_IsIdentityNotBridge(t *testing.T) {
	in := ConversionInput{Method: valuation.CodeAdjustedNetAssetValue, ValueType: valuation.ValueTypeAsset, Value: 1_413_000}
	c := Convert(in, valuation.ValueTypeEquity)
	if c.Outcome != OutcomeConverted {
		t.Fatalf("Outcome = %v, want OutcomeConverted", c.Outcome)
	}
	if c.ConvertedValue != 1_413_000 {
		t.Errorf("ConvertedValue = %v, want the identical 1,413,000 (identity conversion)", c.ConvertedValue)
	}
	if c.Bridge.Available {
		t.Errorf("asset->equity is a relabel, not a cash/debt bridge; expected Bridge.Available=false, got %+v", c.Bridge)
	}
}

func TestConvert_EquityToEnterprise_Excluded(t *testing.T) {
	in := ConversionInput{Method: valuation.CodeSDEMultiple, ValueType: valuation.ValueTypeEquity, Value: 547000}
	c := Convert(in, valuation.ValueTypeEnterprise)
	if c.Outcome != OutcomeExcluded {
		t.Fatalf("Outcome = %v, want OutcomeExcluded (no equity->enterprise conversion is defined anywhere in this repository)", c.Outcome)
	}
	if c.ExclusionReason == "" {
		t.Error("expected a non-empty ExclusionReason")
	}
}

func TestConvert_AssetToEnterprise_Excluded(t *testing.T) {
	in := ConversionInput{Method: valuation.CodeAdjustedNetAssetValue, ValueType: valuation.ValueTypeAsset, Value: 1_413_000}
	c := Convert(in, valuation.ValueTypeEnterprise)
	if c.Outcome != OutcomeExcluded {
		t.Fatalf("Outcome = %v, want OutcomeExcluded", c.Outcome)
	}
}

func TestConvertAll_PreservesOrderAndCount(t *testing.T) {
	inputs := []ConversionInput{
		{Method: valuation.CodeSDEMultiple, ValueType: valuation.ValueTypeEquity, Value: 1},
		{Method: valuation.CodeEBITDAMultiple, ValueType: valuation.ValueTypeEnterprise, Value: 2},
		{Method: valuation.CodeAdjustedNetAssetValue, ValueType: valuation.ValueTypeAsset, Value: 3},
	}
	got := ConvertAll(inputs, valuation.ValueTypeEquity)
	if len(got) != 3 {
		t.Fatalf("ConvertAll returned %d conversions, want 3 (one per input, regardless of outcome)", len(got))
	}
	for i, c := range got {
		if c.Method != inputs[i].Method {
			t.Errorf("conversion %d Method = %v, want %v (order must be preserved)", i, c.Method, inputs[i].Method)
		}
	}
}

func TestConvertAll_Empty(t *testing.T) {
	got := ConvertAll(nil, valuation.ValueTypeEquity)
	if len(got) != 0 {
		t.Errorf("ConvertAll(nil) = %+v, want empty", got)
	}
}

func TestConvert_ExcludedNeverReportsAFabricatedValue(t *testing.T) {
	// Regression guard for the core "never silently average incompatible
	// raw values" requirement: an excluded conversion's ConvertedValue must
	// never equal the (wrong-basis) OriginalValue by coincidence of a
	// missed early return.
	in := ConversionInput{Method: valuation.CodeDCF, ValueType: valuation.ValueTypeEnterprise, Value: 123456}
	c := Convert(in, valuation.ValueTypeEquity)
	if c.Outcome != OutcomeExcluded {
		t.Fatalf("expected OutcomeExcluded for an enterprise value with no bridge")
	}
	if c.ConvertedValue == in.Value {
		t.Errorf("ConvertedValue must not silently equal the original enterprise value when excluded")
	}
}
