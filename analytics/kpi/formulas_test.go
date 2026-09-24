package kpi

import "testing"

// This file locks the exact worked formula examples task section 33
// requires: every one of these must evaluate correctly end to end.

func TestFormula_RevenuePerFTE(t *testing.T) {
	unit := Unit{Kind: UnitCustom, CustomLabel: "USD_PER_COUNT"}
	def := Definition{Code: "RevenuePerFTE", Unit: unit,
		Formula: Binary(OpDivide, Metric(MetricRef{Code: "Revenue"}), Metric(MetricRef{Code: "FTE"}))}
	res := Calculate(onePeriodInput([]Definition{def}, []MetricValue{
		metricInput("Revenue", "P1", 2000000, true, currencyUnit("USD")),
		metricInput("FTE", "P1", 20, true, Unit{Kind: UnitCount}),
	}), Options{})
	kr := firstResult(t, res, "RevenuePerFTE")
	if !kr.Value.Available || kr.Value.Amount != 100000 {
		t.Fatalf("got %+v", kr.Value)
	}
}

func TestFormula_LaborCostPercentRevenue(t *testing.T) {
	def := Definition{Code: "LaborCostPercentRevenue", Unit: Unit{Kind: UnitPercent},
		Formula: Binary(OpPercent, Metric(MetricRef{Code: "LaborCost"}), Metric(MetricRef{Code: "Revenue"}))}
	res := Calculate(onePeriodInput([]Definition{def}, []MetricValue{
		metricInput("LaborCost", "P1", 300000, true, currencyUnit("USD")),
		metricInput("Revenue", "P1", 1000000, true, currencyUnit("USD")),
	}), Options{})
	kr := firstResult(t, res, "LaborCostPercentRevenue")
	if !kr.Value.Available || kr.Value.Amount != 30 {
		t.Fatalf("got %+v", kr.Value)
	}
}

func TestFormula_AROver60Percent(t *testing.T) {
	def := Definition{Code: "AROver60Percent", Unit: Unit{Kind: UnitPercent},
		Formula: Binary(OpPercent, Metric(MetricRef{Code: "AROver60"}), Metric(MetricRef{Code: "TotalAR"}))}
	res := Calculate(onePeriodInput([]Definition{def}, []MetricValue{
		metricInput("AROver60", "P1", 15000, true, currencyUnit("USD")),
		metricInput("TotalAR", "P1", 100000, true, currencyUnit("USD")),
	}), Options{})
	kr := firstResult(t, res, "AROver60Percent")
	if !kr.Value.Available || kr.Value.Amount != 15 {
		t.Fatalf("got %+v", kr.Value)
	}
}

func TestFormula_MarketingSpendPercentRevenue(t *testing.T) {
	def := Definition{Code: "MarketingSpendPercentRevenue", Unit: Unit{Kind: UnitPercent},
		Formula: Binary(OpPercent, Metric(MetricRef{Code: "MarketingSpend"}), Metric(MetricRef{Code: "Revenue"}))}
	res := Calculate(onePeriodInput([]Definition{def}, []MetricValue{
		metricInput("MarketingSpend", "P1", 80000, true, currencyUnit("USD")),
		metricInput("Revenue", "P1", 1000000, true, currencyUnit("USD")),
	}), Options{})
	kr := firstResult(t, res, "MarketingSpendPercentRevenue")
	if !kr.Value.Available || kr.Value.Amount != 8 {
		t.Fatalf("got %+v", kr.Value)
	}
}

func TestFormula_AverageTicket(t *testing.T) {
	// Revenue (Currency) / TransactionCount (Count) resolves to
	// UnitCustom "USD_PER_COUNT" (combineDivisive), not plain Currency.
	def := Definition{Code: "AverageTicket", Unit: Unit{Kind: UnitCustom, CustomLabel: "USD_PER_COUNT"},
		Formula: Binary(OpDivide, Metric(MetricRef{Code: "Revenue"}), Metric(MetricRef{Code: "TransactionCount"}))}
	res := Calculate(onePeriodInput([]Definition{def}, []MetricValue{
		metricInput("Revenue", "P1", 50000, true, currencyUnit("USD")),
		metricInput("TransactionCount", "P1", 500, true, Unit{Kind: UnitCount}),
	}), Options{})
	kr := firstResult(t, res, "AverageTicket")
	if !kr.Value.Available || kr.Value.Amount != 100 {
		t.Fatalf("got %+v", kr.Value)
	}
}

func TestFormula_GrossMargin(t *testing.T) {
	def := Definition{Code: "GrossMargin", Unit: Unit{Kind: UnitPercent},
		Formula: Binary(OpPercent, Metric(MetricRef{Code: "GrossProfit"}), Metric(MetricRef{Code: "Revenue"}))}
	res := Calculate(onePeriodInput([]Definition{def}, []MetricValue{
		metricInput("GrossProfit", "P1", 400000, true, currencyUnit("USD")),
		metricInput("Revenue", "P1", 1000000, true, currencyUnit("USD")),
	}), Options{})
	kr := firstResult(t, res, "GrossMargin")
	if !kr.Value.Available || kr.Value.Amount != 40 {
		t.Fatalf("got %+v", kr.Value)
	}
}

func TestFormula_RevenuePerSquareFoot(t *testing.T) {
	unit := Unit{Kind: UnitCustom, CustomLabel: "USD_PER_AREA"}
	def := Definition{Code: "RevenuePerSquareFoot", Unit: unit,
		Formula: Binary(OpDivide, Metric(MetricRef{Code: "Revenue"}), Metric(MetricRef{Code: "SquareFeet"}))}
	res := Calculate(onePeriodInput([]Definition{def}, []MetricValue{
		metricInput("Revenue", "P1", 500000, true, currencyUnit("USD")),
		metricInput("SquareFeet", "P1", 2500, true, Unit{Kind: UnitArea}),
	}), Options{})
	kr := firstResult(t, res, "RevenuePerSquareFoot")
	if !kr.Value.Available || kr.Value.Amount != 200 {
		t.Fatalf("got %+v", kr.Value)
	}
}

func TestFormula_CustomComposite(t *testing.T) {
	// CustomComposite = (A + B) / C
	def := Definition{Code: "CustomComposite", Unit: Unit{Kind: UnitRatio},
		Formula: Binary(OpDivide, Binary(OpAdd, Metric(MetricRef{Code: "A"}), Metric(MetricRef{Code: "B"})), Metric(MetricRef{Code: "C"}))}
	res := Calculate(onePeriodInput([]Definition{def}, []MetricValue{
		metricInput("A", "P1", 30, true, currencyUnit("USD")),
		metricInput("B", "P1", 70, true, currencyUnit("USD")),
		metricInput("C", "P1", 20, true, currencyUnit("USD")),
	}), Options{})
	kr := firstResult(t, res, "CustomComposite")
	if !kr.Value.Available || kr.Value.Amount != 5 {
		t.Fatalf("got %+v", kr.Value)
	}
}

// --- Built-in templates ---

func TestTemplates_AllEvaluate(t *testing.T) {
	templates := BuiltInTemplates()
	if len(templates) == 0 {
		t.Fatalf("expected at least one built-in template")
	}
	metrics := []MetricValue{
		metricInput("financial.revenue", "P1", 1000000, true, currencyUnit("USD")),
		metricInput("labor.fte", "P1", 10, true, Unit{Kind: UnitCount}),
		metricInput("labor.total_labor_cost", "P1", 300000, true, currencyUnit("USD")),
		metricInput("ar.over_60_amount", "P1", 5000, true, currencyUnit("USD")),
		metricInput("ar.total_open_ar", "P1", 50000, true, currencyUnit("USD")),
		metricInput("custom.transaction_count", "P1", 1000, true, Unit{Kind: UnitCount}),
		metricInput("financial.gross_profit", "P1", 400000, true, currencyUnit("USD")),
		metricInput("custom.square_feet", "P1", 5000, true, Unit{Kind: UnitArea}),
	}
	res := Calculate(onePeriodInput(templates, metrics), Options{})
	if len(res.DefinitionIssues) != 0 {
		t.Fatalf("built-in templates should have zero DefinitionIssues, got %v", res.DefinitionIssues)
	}
	for _, def := range templates {
		kr := firstResult(t, res, def.Code)
		if !kr.Value.Available {
			t.Errorf("template %q unavailable: reason=%v", def.Code, kr.Value.Reason)
		}
	}
}

func TestTemplates_DoNotSpecialCase(t *testing.T) {
	// Every template must be an ordinary Definition, structurally no
	// different from a caller-authored one -- evaluated via the exact
	// same Calculate path (task section 30). This test proves a
	// caller-authored Definition with the identical shape as
	// GrossMarginTemplate produces the identical KPIResult.
	tmpl := GrossMarginTemplate()
	custom := Definition{
		Code: "my_own_gross_margin", Unit: tmpl.Unit,
		Formula: Binary(OpPercent, Metric(MetricRef{Code: "financial.gross_profit"}), Metric(MetricRef{Code: "financial.revenue"})),
	}
	metrics := []MetricValue{
		metricInput("financial.gross_profit", "P1", 400000, true, currencyUnit("USD")),
		metricInput("financial.revenue", "P1", 1000000, true, currencyUnit("USD")),
	}
	res := Calculate(onePeriodInput([]Definition{tmpl, custom}, metrics), Options{})
	a := firstResult(t, res, tmpl.Code)
	b := firstResult(t, res, custom.Code)
	if a.Value != b.Value {
		t.Fatalf("template and hand-authored equivalent produced different results: %+v vs %+v", a.Value, b.Value)
	}
}
