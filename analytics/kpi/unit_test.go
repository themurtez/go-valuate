package kpi

import "testing"

func TestUnits_CompatibleArithmetic(t *testing.T) {
	compat := combineAdditive(currencyUnit("CAD"), currencyUnit("CAD"))
	if !compat.compatible || !compat.result.Equal(currencyUnit("CAD")) {
		t.Fatalf("CAD+CAD should be compatible, got %+v", compat)
	}
}

func TestUnits_InvalidAddition_CrossCurrency(t *testing.T) {
	def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Binary(OpAdd, Metric(MetricRef{Code: "a"}), Metric(MetricRef{Code: "b"}))}
	res := Calculate(onePeriodInput([]Definition{def}, []MetricValue{
		metricInput("a", "P1", 10, true, currencyUnit("CAD")),
		metricInput("b", "P1", 10, true, currencyUnit("USD")),
	}), Options{})
	kr := firstResult(t, res, "k")
	if kr.Value.Available || kr.Value.Reason != AvailabilityCurrencyMismatch {
		t.Fatalf("expected CURRENCY_MISMATCH, got %+v", kr.Value)
	}
}

func TestUnits_InvalidAddition_CurrencyPlusHours(t *testing.T) {
	def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Binary(OpAdd, Metric(MetricRef{Code: "a"}), Metric(MetricRef{Code: "b"}))}
	res := Calculate(onePeriodInput([]Definition{def}, []MetricValue{
		metricInput("a", "P1", 10, true, currencyUnit("USD")),
		metricInput("b", "P1", 10, true, Unit{Kind: UnitHours}),
	}), Options{})
	kr := firstResult(t, res, "k")
	if kr.Value.Available || kr.Value.Reason != AvailabilityUnitMismatch {
		t.Fatalf("expected UNIT_MISMATCH, got %+v", kr.Value)
	}
}

func TestUnits_RatioPercent_Divide(t *testing.T) {
	compat := combineDivisive(currencyUnit("USD"), currencyUnit("USD"))
	if !compat.compatible || compat.result.Kind != UnitRatio {
		t.Fatalf("USD/USD should yield RATIO, got %+v", compat)
	}
}

func TestUnits_CurrencyPerCount(t *testing.T) {
	compat := combineDivisive(currencyUnit("USD"), Unit{Kind: UnitCount})
	if !compat.compatible || compat.result.Kind != UnitCustom {
		t.Fatalf("USD/COUNT should yield a custom currency-per-count unit, got %+v", compat)
	}
}

func TestUnits_CurrencyMismatch_Divide(t *testing.T) {
	compat := combineDivisive(currencyUnit("CAD"), currencyUnit("USD"))
	if compat.compatible || !compat.currencyMismatch {
		t.Fatalf("CAD/USD should be a currency mismatch, got %+v", compat)
	}
}

func TestUnits_ExpectedOutputUnit_Mismatch(t *testing.T) {
	// Declared PERCENT but formula root actually resolves to CURRENCY.
	def := Definition{Code: "k", Unit: Unit{Kind: UnitPercent}, Formula: Metric(MetricRef{Code: "a"})}
	res := Calculate(onePeriodInput([]Definition{def}, []MetricValue{
		metricInput("a", "P1", 100, true, currencyUnit("USD")),
	}), Options{})
	kr := firstResult(t, res, "k")
	if kr.Value.Available || kr.Value.Reason != AvailabilityUnitMismatch {
		t.Fatalf("expected declared-vs-actual unit mismatch, got %+v", kr.Value)
	}
}

func TestUnits_ExpectedOutputUnit_Match(t *testing.T) {
	def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Metric(MetricRef{Code: "a"})}
	res := Calculate(onePeriodInput([]Definition{def}, []MetricValue{
		metricInput("a", "P1", 100, true, currencyUnit("USD")),
	}), Options{})
	kr := firstResult(t, res, "k")
	if !kr.Value.Available || kr.Value.Amount != 100 {
		t.Fatalf("expected available 100, got %+v", kr.Value)
	}
}

func TestUnitValid(t *testing.T) {
	cases := []struct {
		name string
		unit Unit
		want bool
	}{
		{"currency_with_code", currencyUnit("USD"), true},
		{"currency_without_code", Unit{Kind: UnitCurrency}, false},
		{"custom_with_label", Unit{Kind: UnitCustom, CustomLabel: "WIDGETS"}, true},
		{"custom_without_label", Unit{Kind: UnitCustom}, false},
		{"ratio", Unit{Kind: UnitRatio}, true},
		{"unrecognized", Unit{Kind: "BOGUS"}, false},
		{"zero_value", Unit{}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.unit.Valid(); got != c.want {
				t.Fatalf("Valid() = %v, want %v", got, c.want)
			}
		})
	}
}
