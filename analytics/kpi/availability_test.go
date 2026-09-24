package kpi

import "testing"

// TestAvailability_KnownZero proves a MetricValue with Available=true and
// Value=0 is distinguished from a MetricValue that was never supplied —
// task section 3/35's central invariant.
func TestAvailability_KnownZero(t *testing.T) {
	def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Metric(MetricRef{Code: "spend"})}
	res := Calculate(onePeriodInput([]Definition{def}, []MetricValue{
		metricInput("spend", "P1", 0, true, currencyUnit("USD")),
	}), Options{})
	kr := firstResult(t, res, "k")
	if !kr.Value.Available || kr.Value.Amount != 0 {
		t.Fatalf("known zero should be Available=true, Amount=0, got %+v", kr.Value)
	}
}

func TestAvailability_Missing(t *testing.T) {
	def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Metric(MetricRef{Code: "spend"})}
	res := Calculate(onePeriodInput([]Definition{def}, nil), Options{})
	kr := firstResult(t, res, "k")
	if kr.Value.Available || kr.Value.Reason != AvailabilityMissingMetric {
		t.Fatalf("missing metric should be unavailable MISSING_METRIC, got %+v", kr.Value)
	}
}

func TestAvailability_SourceUnavailable(t *testing.T) {
	def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Metric(MetricRef{Code: "spend"})}
	res := Calculate(onePeriodInput([]Definition{def}, []MetricValue{
		metricInput("spend", "P1", 0, false, currencyUnit("USD")),
	}), Options{})
	kr := firstResult(t, res, "k")
	if kr.Value.Available || kr.Value.Reason != AvailabilitySourceUnavailable {
		t.Fatalf("supplied-but-unavailable metric should be SOURCE_UNAVAILABLE, got %+v", kr.Value)
	}
}

// TestAvailability_DependencyUnavailable: Revenue available + FTE
// unavailable -> RevenuePerFTE unavailable — task section 35's exact
// example.
func TestAvailability_DependencyUnavailable(t *testing.T) {
	kpiA := Definition{Code: "revenue_per_fte", Unit: Unit{Kind: UnitCustom, CustomLabel: "USD_PER_COUNT"},
		Formula: Binary(OpDivide, Metric(MetricRef{Code: "revenue"}), Metric(MetricRef{Code: "fte"}))}
	res := Calculate(onePeriodInput([]Definition{kpiA}, []MetricValue{
		metricInput("revenue", "P1", 100, true, currencyUnit("USD")),
		// fte deliberately absent
	}), Options{})
	kr := firstResult(t, res, "revenue_per_fte")
	if kr.Value.Available {
		t.Fatalf("revenue available + fte unavailable should yield unavailable RevenuePerFTE, got %+v", kr.Value)
	}
}

// TestAvailability_KPIDependencyUnavailable: a KPI referencing another
// KPI whose own value is unavailable propagates
// AvailabilityDependencyUnavailable.
func TestAvailability_KPIDependencyUnavailable(t *testing.T) {
	inner := Definition{Code: "inner", Unit: currencyUnit("USD"), Formula: Metric(MetricRef{Code: "missing"})}
	outer := Definition{Code: "outer", Unit: currencyUnit("USD"), Formula: KPIExpr(KPIRef{Code: "inner"})}
	res := Calculate(onePeriodInput([]Definition{inner, outer}, nil), Options{})
	kr := firstResult(t, res, "outer")
	if kr.Value.Available {
		t.Fatalf("expected unavailable, got %+v", kr.Value)
	}
}

// TestAvailability_DivideByZero_100_0: Revenue=100 + FTE=0 known ->
// unavailable/DIVIDE_BY_ZERO — task section 35's exact example.
func TestAvailability_DivideByZero_100_0(t *testing.T) {
	def := Definition{Code: "k", Unit: Unit{Kind: UnitCustom, CustomLabel: "USD_PER_COUNT"},
		Formula: Binary(OpDivide, Metric(MetricRef{Code: "revenue"}), Metric(MetricRef{Code: "fte"}))}
	res := Calculate(onePeriodInput([]Definition{def}, []MetricValue{
		metricInput("revenue", "P1", 100, true, currencyUnit("USD")),
		metricInput("fte", "P1", 0, true, Unit{Kind: UnitCount}),
	}), Options{})
	kr := firstResult(t, res, "k")
	if kr.Value.Available || kr.Value.Reason != AvailabilityDivideByZero {
		t.Fatalf("expected DIVIDE_BY_ZERO, got %+v", kr.Value)
	}
}

// TestAvailability_ZeroSourceValuesRemainKnown: Revenue=0 known +
// Marketing=0 known -> source values remain known zero — task section
// 35's exact example.
func TestAvailability_ZeroSourceValuesRemainKnown(t *testing.T) {
	def := Definition{Code: "k", Unit: Unit{Kind: UnitPercent},
		Formula: Binary(OpPercent, Metric(MetricRef{Code: "marketing"}), Metric(MetricRef{Code: "revenue"}))}
	res := Calculate(onePeriodInput([]Definition{def}, []MetricValue{
		metricInput("revenue", "P1", 0, true, currencyUnit("USD")),
		metricInput("marketing", "P1", 0, true, currencyUnit("USD")),
	}), Options{})
	kr := firstResult(t, res, "k")
	// 0/0 -> DIVIDE_BY_ZERO is the correct result (not silently 0), but
	// both source facts were themselves known-zero, not missing.
	if kr.Value.Available || kr.Value.Reason != AvailabilityDivideByZero {
		t.Fatalf("expected DIVIDE_BY_ZERO, got %+v", kr.Value)
	}
}

// TestAvailability_RevenuePlusHours_UnitMismatch — task section 35.
func TestAvailability_RevenuePlusHours_UnitMismatch(t *testing.T) {
	def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Binary(OpAdd, Metric(MetricRef{Code: "revenue"}), Metric(MetricRef{Code: "hours"}))}
	res := Calculate(onePeriodInput([]Definition{def}, []MetricValue{
		metricInput("revenue", "P1", 100, true, currencyUnit("USD")),
		metricInput("hours", "P1", 10, true, Unit{Kind: UnitHours}),
	}), Options{})
	kr := firstResult(t, res, "k")
	if kr.Value.Available || kr.Value.Reason != AvailabilityUnitMismatch {
		t.Fatalf("expected UNIT_MISMATCH, got %+v", kr.Value)
	}
}

// TestAvailability_CADPlusUSD_CurrencyMismatch — task section 35.
func TestAvailability_CADPlusUSD_CurrencyMismatch(t *testing.T) {
	def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Binary(OpAdd, Metric(MetricRef{Code: "a"}), Metric(MetricRef{Code: "b"}))}
	res := Calculate(onePeriodInput([]Definition{def}, []MetricValue{
		metricInput("a", "P1", 100, true, currencyUnit("CAD")),
		metricInput("b", "P1", 50, true, currencyUnit("USD")),
	}), Options{})
	kr := firstResult(t, res, "k")
	if kr.Value.Available || kr.Value.Reason != AvailabilityCurrencyMismatch {
		t.Fatalf("expected CURRENCY_MISMATCH, got %+v", kr.Value)
	}
}

// TestAvailability_Coalesce_SuppressesRuntimeMissingness proves COALESCE
// falls through to its second Arg when the first is missing, and that
// the KPI's own declared Unit must match whichever Arg actually resolves
// (checkExpectedOutputUnit applies to COALESCE's resolved unit exactly
// like any other operator's).
func TestAvailability_Coalesce_SuppressesRuntimeMissingness(t *testing.T) {
	def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Variadic(OpCoalesce, Metric(MetricRef{Code: "missing"}), Metric(MetricRef{Code: "fallback"}))}
	res := Calculate(onePeriodInput([]Definition{def}, []MetricValue{
		metricInput("fallback", "P1", 250, true, currencyUnit("USD")),
	}), Options{})
	kr := firstResult(t, res, "k")
	if !kr.Value.Available || kr.Value.Amount != 250 {
		t.Fatalf("got %+v", kr.Value)
	}
}

// TestAvailability_Coalesce_ResolvedUnitStillChecked proves a COALESCE
// falling through to a unit-mismatched Arg still triggers
// checkExpectedOutputUnit at the KPI level — COALESCE resolves
// AVAILABILITY, never UNIT correctness (a caller wanting unit-consistent
// fallback values supplies unit-consistent Args).
func TestAvailability_Coalesce_ResolvedUnitStillChecked(t *testing.T) {
	def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Variadic(OpCoalesce, Metric(MetricRef{Code: "missing"}), Const(0))}
	res := Calculate(onePeriodInput([]Definition{def}, nil), Options{})
	kr := firstResult(t, res, "k")
	if kr.Value.Available || kr.Value.Reason != AvailabilityUnitMismatch {
		t.Fatalf("expected UNIT_MISMATCH once COALESCE resolves to a UNITLESS constant against a declared USD KPI, got %+v", kr.Value)
	}
}

// TestAvailability_Coalesce_NeverSuppressesDefinitionError proves a
// structurally invalid Expression under a COALESCE Arg is still a
// definition error — task section 10's explicit rule.
func TestAvailability_Coalesce_NeverSuppressesDefinitionError(t *testing.T) {
	badArg := Unary(OpAdd, Const(1)) // ADD requires 2 args, this supplies 1
	def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Variadic(OpCoalesce, badArg, Const(0))}
	res := Calculate(onePeriodInput([]Definition{def}, nil), Options{})
	found := false
	for _, i := range res.DefinitionIssues {
		if i.Code == IssueInvalidArgumentCount {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected IssueInvalidArgumentCount even under COALESCE, got %v", res.DefinitionIssues)
	}
}
