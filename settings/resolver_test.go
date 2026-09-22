package settings

import "testing"

func TestResolve_CompletelyUnsetFieldIsAbsent(t *testing.T) {
	res := Resolve(Settings{}, Settings{}, Settings{}, Settings{})

	if _, ok := res.Values[FieldKeySDEMultiple]; ok {
		t.Errorf("expected sde_multiple to be absent when unset at every scope, got %v", res.Values[FieldKeySDEMultiple])
	}
	if _, ok := res.Sources[FieldKeySDEMultiple]; ok {
		t.Errorf("expected no source recorded for an unset field")
	}
	if len(res.Values) != 0 {
		t.Errorf("expected empty Values map, got %+v", res.Values)
	}
}

func TestResolve_SystemValueUsedWhenNoOverride(t *testing.T) {
	system := Settings{DiscountRate: Float64(0.10)}

	res := Resolve(system, Settings{}, Settings{}, Settings{})

	got, ok := res.Values[FieldKeyDiscountRate]
	if !ok {
		t.Fatal("expected discount_rate to be resolved")
	}
	if got.(float64) != 0.10 {
		t.Errorf("discount_rate = %v, want 0.10", got)
	}
	if res.Sources[FieldKeyDiscountRate] != ScopeSystem {
		t.Errorf("source = %v, want system", res.Sources[FieldKeyDiscountRate])
	}
}

func TestResolve_AccountOverridesSystem(t *testing.T) {
	system := Settings{TaxRate: Float64(0.20)}
	account := Settings{TaxRate: Float64(0.25)}

	res := Resolve(system, account, Settings{}, Settings{})

	got := res.Values[FieldKeyTaxRate].(float64)
	if got != 0.25 {
		t.Errorf("tax_rate = %v, want 0.25 (account should override system)", got)
	}
	if res.Sources[FieldKeyTaxRate] != ScopeAccount {
		t.Errorf("source = %v, want account", res.Sources[FieldKeyTaxRate])
	}
}

func TestResolve_ClientOverridesAccountAndSystem(t *testing.T) {
	system := Settings{DiscountRate: Float64(0.10)}
	account := Settings{DiscountRate: Float64(0.12)}
	client := Settings{DiscountRate: Float64(0.15)}

	res := Resolve(system, account, client, Settings{})

	got := res.Values[FieldKeyDiscountRate].(float64)
	if got != 0.15 {
		t.Errorf("discount_rate = %v, want 0.15 (client should win)", got)
	}
	if res.Sources[FieldKeyDiscountRate] != ScopeClient {
		t.Errorf("source = %v, want client", res.Sources[FieldKeyDiscountRate])
	}
}

func TestResolve_ValuationOverridesEverything(t *testing.T) {
	system := Settings{SDEMultiple: Float64(2.0)}
	account := Settings{SDEMultiple: Float64(2.25)}
	client := Settings{SDEMultiple: Float64(2.5)}
	valuation := Settings{SDEMultiple: Float64(2.75)}

	res := Resolve(system, account, client, valuation)

	got := res.Values[FieldKeySDEMultiple].(float64)
	if got != 2.75 {
		t.Errorf("sde_multiple = %v, want 2.75 (valuation should win over all)", got)
	}
	if res.Sources[FieldKeySDEMultiple] != ScopeValuation {
		t.Errorf("source = %v, want valuation", res.Sources[FieldKeySDEMultiple])
	}
}

func TestResolve_FallsThroughGapsInPrecedenceChain(t *testing.T) {
	// account leaves discount_rate unset; client should fall through to
	// system, not treat account's absence as an override to zero.
	system := Settings{DiscountRate: Float64(0.10)}
	account := Settings{} // unset
	client := Settings{}  // unset
	valuation := Settings{}

	res := Resolve(system, account, client, valuation)

	got := res.Values[FieldKeyDiscountRate].(float64)
	if got != 0.10 {
		t.Errorf("discount_rate = %v, want 0.10 (should fall through unset middle scopes)", got)
	}
	if res.Sources[FieldKeyDiscountRate] != ScopeSystem {
		t.Errorf("source = %v, want system", res.Sources[FieldKeyDiscountRate])
	}
}

func TestResolve_ExplicitZeroOverridesNonZeroLowerScope(t *testing.T) {
	system := Settings{ControlPremium: Float64(0.10)}
	valuation := Settings{ControlPremium: Float64(0)} // explicit zero, not unset

	res := Resolve(system, Settings{}, Settings{}, valuation)

	got, ok := res.Values[FieldKeyControlPremium]
	if !ok {
		t.Fatal("expected control_premium to be present (explicit zero is not unset)")
	}
	if got.(float64) != 0 {
		t.Errorf("control_premium = %v, want 0", got)
	}
	if res.Sources[FieldKeyControlPremium] != ScopeValuation {
		t.Errorf("source = %v, want valuation (explicit zero must still win precedence)", res.Sources[FieldKeyControlPremium])
	}
}

func TestResolve_ExplicitZeroDistinctFromUnset(t *testing.T) {
	// Two resolutions: one where valuation explicitly sets zero, one where
	// valuation leaves it unset. They must behave differently: explicit
	// zero wins over system; unset falls through to system.
	system := Settings{TerminalGrowthRate: Float64(0.03)}

	explicitZero := Resolve(system, Settings{}, Settings{}, Settings{TerminalGrowthRate: Float64(0)})
	unset := Resolve(system, Settings{}, Settings{}, Settings{})

	if v := explicitZero.Values[FieldKeyTerminalGrowthRate].(float64); v != 0 {
		t.Errorf("explicit zero case: terminal_growth_rate = %v, want 0", v)
	}
	if explicitZero.Sources[FieldKeyTerminalGrowthRate] != ScopeValuation {
		t.Errorf("explicit zero case: source = %v, want valuation", explicitZero.Sources[FieldKeyTerminalGrowthRate])
	}

	if v := unset.Values[FieldKeyTerminalGrowthRate].(float64); v != 0.03 {
		t.Errorf("unset case: terminal_growth_rate = %v, want 0.03 (fall through to system)", v)
	}
	if unset.Sources[FieldKeyTerminalGrowthRate] != ScopeSystem {
		t.Errorf("unset case: source = %v, want system", unset.Sources[FieldKeyTerminalGrowthRate])
	}
}

func TestResolve_MethodEnable_ExplicitFalseOverridesTrue(t *testing.T) {
	system := Settings{MethodEnabled: map[Method]*bool{MethodDCF: Bool(true)}}
	client := Settings{MethodEnabled: map[Method]*bool{MethodDCF: Bool(false)}}

	res := Resolve(system, Settings{}, client, Settings{})

	got, ok := res.Values[MethodFieldKey(MethodDCF)]
	if !ok {
		t.Fatal("expected dcf method flag to be present")
	}
	if got.(bool) != false {
		t.Errorf("method_enabled.dcf = %v, want false", got)
	}
	if res.Sources[MethodFieldKey(MethodDCF)] != ScopeClient {
		t.Errorf("source = %v, want client", res.Sources[MethodFieldKey(MethodDCF)])
	}
}

func TestResolve_MethodEnable_UnsetMethodIsAbsent(t *testing.T) {
	system := Settings{MethodEnabled: map[Method]*bool{MethodSDE: Bool(true)}}

	res := Resolve(system, Settings{}, Settings{}, Settings{})

	if _, ok := res.Values[MethodFieldKey(MethodEBITDA)]; ok {
		t.Errorf("expected ebitda method flag to be absent, never set at any scope")
	}
}

func TestResolve_MethodEnable_FallsThroughUnsetScopes(t *testing.T) {
	system := Settings{MethodEnabled: map[Method]*bool{MethodEBITDA: Bool(true)}}
	account := Settings{MethodEnabled: map[Method]*bool{}}                    // present map, but method not set
	client := Settings{}                                                      // no map at all
	valuation := Settings{MethodEnabled: map[Method]*bool{MethodEBITDA: nil}} // explicit nil value = still unset

	res := Resolve(system, account, client, valuation)

	got, ok := res.Values[MethodFieldKey(MethodEBITDA)]
	if !ok {
		t.Fatal("expected ebitda method flag to fall through to system")
	}
	if got.(bool) != true {
		t.Errorf("method_enabled.ebitda = %v, want true", got)
	}
	if res.Sources[MethodFieldKey(MethodEBITDA)] != ScopeSystem {
		t.Errorf("source = %v, want system", res.Sources[MethodFieldKey(MethodEBITDA)])
	}
}

func TestResolve_MultipleFieldsIndependentSources(t *testing.T) {
	// Reproduces the example from the spec: sde_multiple from valuation,
	// discount_rate from client, tax_rate from account.
	system := Settings{
		DiscountRate: Float64(0.12),
	}
	account := Settings{
		DiscountRate: Float64(0.13),
		TaxRate:      Float64(0.21),
	}
	client := Settings{
		DiscountRate: Float64(0.15),
	}
	valuation := Settings{
		SDEMultiple: Float64(2.75),
	}

	res := Resolve(system, account, client, valuation)

	if res.Values[FieldKeySDEMultiple].(float64) != 2.75 {
		t.Errorf("sde_multiple = %v, want 2.75", res.Values[FieldKeySDEMultiple])
	}
	if res.Sources[FieldKeySDEMultiple] != ScopeValuation {
		t.Errorf("sde_multiple source = %v, want valuation", res.Sources[FieldKeySDEMultiple])
	}

	if res.Values[FieldKeyDiscountRate].(float64) != 0.15 {
		t.Errorf("discount_rate = %v, want 0.15", res.Values[FieldKeyDiscountRate])
	}
	if res.Sources[FieldKeyDiscountRate] != ScopeClient {
		t.Errorf("discount_rate source = %v, want client", res.Sources[FieldKeyDiscountRate])
	}

	if res.Values[FieldKeyTaxRate].(float64) != 0.21 {
		t.Errorf("tax_rate = %v, want 0.21", res.Values[FieldKeyTaxRate])
	}
	if res.Sources[FieldKeyTaxRate] != ScopeAccount {
		t.Errorf("tax_rate source = %v, want account", res.Sources[FieldKeyTaxRate])
	}
}

func TestResolve_ScopeFieldOnInputIsIgnoredForPrecedence(t *testing.T) {
	// Mislabeling Scope on an input struct must not change precedence:
	// Resolve trusts argument position, not the Scope field.
	system := Settings{Scope: ScopeValuation, DiscountRate: Float64(0.10)}
	valuation := Settings{Scope: ScopeSystem, DiscountRate: Float64(0.20)}

	res := Resolve(system, Settings{}, Settings{}, valuation)

	got := res.Values[FieldKeyDiscountRate].(float64)
	if got != 0.20 {
		t.Errorf("discount_rate = %v, want 0.20 (argument position determines precedence, not mislabeled Scope field)", got)
	}
	if res.Sources[FieldKeyDiscountRate] != ScopeValuation {
		t.Errorf("source = %v, want valuation", res.Sources[FieldKeyDiscountRate])
	}
}
