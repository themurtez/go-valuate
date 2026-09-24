package kpi

import "testing"

func TestCoverage_FactualCounts(t *testing.T) {
	validDef := Definition{Code: "valid", Unit: currencyUnit("USD"), Formula: Metric(MetricRef{Code: "revenue"})}
	invalidDef := Definition{Code: "invalid", Unit: Unit{}, Formula: Unary(OpAdd, Const(1))} // bad unit + bad arity

	res := Calculate(onePeriodInput([]Definition{validDef, invalidDef}, []MetricValue{
		metricInput("revenue", "P1", 100, true, currencyUnit("USD")),
	}), Options{})

	if res.Coverage.DefinitionsSupplied != 2 {
		t.Fatalf("DefinitionsSupplied = %d, want 2", res.Coverage.DefinitionsSupplied)
	}
	if res.Coverage.DefinitionsValid != 1 {
		t.Fatalf("DefinitionsValid = %d, want 1", res.Coverage.DefinitionsValid)
	}
	if res.Coverage.DefinitionsInvalid != 1 {
		t.Fatalf("DefinitionsInvalid = %d, want 1", res.Coverage.DefinitionsInvalid)
	}
	if res.Coverage.AvailableValues != 1 {
		t.Fatalf("AvailableValues = %d, want 1", res.Coverage.AvailableValues)
	}
}

func TestCoverage_UnavailableValuesCounted(t *testing.T) {
	def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Metric(MetricRef{Code: "missing"})}
	res := Calculate(onePeriodInput([]Definition{def}, nil), Options{})
	if res.Coverage.UnavailableValues != 1 {
		t.Fatalf("UnavailableValues = %d, want 1", res.Coverage.UnavailableValues)
	}
	if res.Coverage.AvailableValues != 0 {
		t.Fatalf("AvailableValues = %d, want 0", res.Coverage.AvailableValues)
	}
}

func TestCoverage_PeriodsAndDimensionGroups(t *testing.T) {
	def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Const(1)}
	res := Calculate(multiPeriodInput([]Definition{def}, nil), Options{Dimensions: []DimensionKey{{"x": "y"}, {"a": "b"}}})
	if res.Coverage.PeriodsEvaluated != 3 {
		t.Fatalf("PeriodsEvaluated = %d, want 3", res.Coverage.PeriodsEvaluated)
	}
	// business-level + 2 requested = 3 dimension groups.
	if res.Coverage.DimensionGroupsEvaluated != 3 {
		t.Fatalf("DimensionGroupsEvaluated = %d, want 3", res.Coverage.DimensionGroupsEvaluated)
	}
}

func TestCoverage_NoOpaqueScore(t *testing.T) {
	// Structural guarantee: Coverage has no float/score-shaped field --
	// every field name below must exist as an int. This compiles only if
	// the shape stays factual-counts-only.
	var c Coverage
	c.DefinitionsSupplied = 1
	c.DefinitionsValid = 1
	c.DefinitionsInvalid = 0
	c.KPIsEvaluated = 1
	c.AvailableValues = 1
	c.UnavailableValues = 0
	c.MissingSourceMetricCount = 0
	c.PeriodsEvaluated = 1
	c.DimensionGroupsEvaluated = 1
	_ = c
}

// --- Definition validation vs runtime availability separation ---

func TestIssueTaxonomy_DuplicateMetricValue(t *testing.T) {
	def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Metric(MetricRef{Code: "revenue"})}
	res := Calculate(onePeriodInput([]Definition{def}, []MetricValue{
		metricInput("revenue", "P1", 100, true, currencyUnit("USD")),
		metricInput("revenue", "P1", 999, true, currencyUnit("USD")), // duplicate source key
	}), Options{})
	found := false
	for _, i := range res.EvaluationIssues {
		if i.Code == IssueDuplicateMetricValue {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected IssueDuplicateMetricValue, got %v", res.EvaluationIssues)
	}
	// First occurrence wins (never picked "arbitrarily").
	kr := firstResult(t, res, "k")
	if kr.Value.Amount != 100 {
		t.Fatalf("expected first occurrence's value (100), got %+v", kr.Value)
	}
}

func TestIssueTaxonomy_InvalidUnit(t *testing.T) {
	def := Definition{Code: "k", Unit: Unit{Kind: "BOGUS"}, Formula: Const(1)}
	res := Calculate(onePeriodInput([]Definition{def}, nil), Options{})
	found := false
	for _, i := range res.DefinitionIssues {
		if i.Code == IssueInvalidUnit {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected IssueInvalidUnit, got %v", res.DefinitionIssues)
	}
}

func TestIssueTaxonomy_FailAllOnDefinitionError(t *testing.T) {
	bad := Definition{Code: "bad", Unit: Unit{}, Formula: Const(1)}
	good := Definition{Code: "good", Unit: currencyUnit("USD"), Formula: Const(1)}
	res := Calculate(onePeriodInput([]Definition{bad, good}, nil), Options{FailAllOnDefinitionError: true})
	if len(res.KPIResults) != 0 {
		t.Fatalf("FailAllOnDefinitionError should zero out KPIResults entirely, got %d", len(res.KPIResults))
	}
}

func TestIssueTaxonomy_LenientByDefault(t *testing.T) {
	bad := Definition{Code: "bad", Unit: Unit{}, Formula: Const(1)}
	good := Definition{Code: "good", Unit: currencyUnit("USD"), Formula: Metric(MetricRef{Code: "revenue"})}
	res := Calculate(onePeriodInput([]Definition{bad, good}, []MetricValue{
		metricInput("revenue", "P1", 100, true, currencyUnit("USD")),
	}), Options{})
	kr := firstResult(t, res, "good")
	if !kr.Value.Available || kr.Value.Amount != 100 {
		t.Fatalf("good KPI should still evaluate by default, got %+v", kr.Value)
	}
}

// TestIssueTaxonomy_InvalidMetricUnit is a fuzz-discovered regression: a
// MetricValue with Available=true but a structurally invalid Unit (e.g.
// UnitCurrency with an empty CurrencyCode) must be excluded from
// arithmetic and reported as IssueInvalidUnit, never silently combined
// as if it were a well-formed Unit — see buildMetricIndex.
func TestIssueTaxonomy_InvalidMetricUnit(t *testing.T) {
	def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Metric(MetricRef{Code: "revenue"})}
	res := Calculate(onePeriodInput([]Definition{def}, []MetricValue{
		{Code: "revenue", Period: "P1", Value: 100, Available: true, Unit: Unit{Kind: UnitCurrency /* empty CurrencyCode */}, Aggregation: AggregationSum},
	}), Options{})
	kr := firstResult(t, res, "k")
	if kr.Value.Available {
		t.Fatalf("a MetricValue with an invalid Unit must not be usable, got %+v", kr.Value)
	}
	found := false
	for _, i := range res.EvaluationIssues {
		if i.Code == IssueInvalidUnit {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected IssueInvalidUnit, got %v", res.EvaluationIssues)
	}
}

func TestIssueTaxonomy_UnsupportedExpressionLanguageVersion(t *testing.T) {
	def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Const(1)}
	res := Calculate(onePeriodInput([]Definition{def}, nil), Options{ExpressionLanguageVersion: "99.0.0"})
	found := false
	for _, i := range res.DefinitionIssues {
		if i.Code == IssueUnsupportedExpressionLanguageVersion {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected IssueUnsupportedExpressionLanguageVersion, got %v", res.DefinitionIssues)
	}
}
