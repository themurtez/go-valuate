package kpi

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
)

// roundTrip marshals v, unmarshals into a fresh T, and returns it.
func roundTrip[T any](t *testing.T, v T) T {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out T
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return out
}

func TestJSON_Definition_RoundTrip(t *testing.T) {
	def := Definition{
		Code: "gross_margin", Name: "Gross Margin %", Description: "GP / Revenue",
		Formula:           Binary(OpPercent, Metric(MetricRef{Code: "financial.gross_profit"}), Metric(MetricRef{Code: "financial.revenue"})),
		Unit:              Unit{Kind: UnitPercent},
		Category:          "Profitability",
		Tags:              []string{"core", "profitability"},
		Target:            &TargetPolicy{Kind: TargetMinimum, Min: 35},
		ThresholdBands:    []ThresholdBand{{Label: "LOW", Min: 0, Max: 20}, {Label: "HIGH", Min: 20, Max: 100}},
		DefinitionVersion: "1",
	}
	got := roundTrip(t, def)
	if got.Code != def.Code || got.Formula.Op != def.Formula.Op {
		t.Fatalf("got %+v", got)
	}
	if len(got.Formula.Args) != 2 {
		t.Fatalf("expression argument ordering not preserved: %+v", got.Formula.Args)
	}
	if got.Formula.Args[0].MetricRef.Code != "financial.gross_profit" || got.Formula.Args[1].MetricRef.Code != "financial.revenue" {
		t.Fatalf("argument order changed across round-trip: %+v", got.Formula.Args)
	}
	if got.Target == nil || got.Target.Min != 35 {
		t.Fatalf("target not round-tripped: %+v", got.Target)
	}
	if len(got.ThresholdBands) != 2 {
		t.Fatalf("bands not round-tripped: %+v", got.ThresholdBands)
	}
}

func TestJSON_Expression_ArgOrderStable(t *testing.T) {
	// (A - B) / C -- order matters semantically for SUBTRACT/DIVIDE.
	expr := Binary(OpDivide, Binary(OpSubtract, Metric(MetricRef{Code: "A"}), Metric(MetricRef{Code: "B"})), Metric(MetricRef{Code: "C"}))
	got := roundTrip(t, expr)
	if got.Args[0].Args[0].MetricRef.Code != "A" || got.Args[0].Args[1].MetricRef.Code != "B" || got.Args[1].MetricRef.Code != "C" {
		t.Fatalf("argument order not preserved: %+v", got)
	}
}

func TestJSON_MetricValue_RoundTrip(t *testing.T) {
	mv := MetricValue{
		Code: "ar.dso", Period: "2025", Dimensions: DimensionKey{"region": "west"},
		Value: 42.5, Available: true, Unit: Unit{Kind: UnitDays}, Aggregation: AggregationNotAggregatable,
		Source: "accounting/ar", SourceRef: "snapshot-1",
	}
	got := roundTrip(t, mv)
	if !reflect.DeepEqual(got, mv) {
		t.Fatalf("MetricValue did not round-trip exactly: got %+v want %+v", got, mv)
	}
}

func TestJSON_DimensionKey_RoundTrip(t *testing.T) {
	dim := DimensionKey{"department": "service", "location": "toronto"}
	got := roundTrip(t, dim)
	if !got.Equal(dim) {
		t.Fatalf("got %+v want %+v", got, dim)
	}
}

func TestJSON_Unit_RoundTrip(t *testing.T) {
	units := []Unit{
		{Kind: UnitCurrency, CurrencyCode: "CAD"},
		{Kind: UnitCustom, CustomLabel: "SQUARE_FEET"},
		{Kind: UnitRatio},
	}
	for _, u := range units {
		got := roundTrip(t, u)
		if !got.Equal(u) {
			t.Fatalf("got %+v want %+v", got, u)
		}
	}
}

func TestJSON_TargetPolicy_RoundTrip(t *testing.T) {
	tp := TargetPolicy{Kind: TargetRange, Min: 70, Max: 85, Tolerance: 1}
	got := roundTrip(t, tp)
	if got != tp {
		t.Fatalf("got %+v want %+v", got, tp)
	}
}

func TestJSON_ThresholdBand_RoundTrip(t *testing.T) {
	b := ThresholdBand{Label: "MID", Min: 50, Max: 80}
	got := roundTrip(t, b)
	if got != b {
		t.Fatalf("got %+v want %+v", got, b)
	}
}

func TestJSON_KPIResult_RoundTrip(t *testing.T) {
	def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Metric(MetricRef{Code: "revenue"}),
		Target: &TargetPolicy{Kind: TargetMinimum, Min: 100}}
	res := Calculate(onePeriodInput([]Definition{def}, []MetricValue{
		metricInput("revenue", "P1", 150, true, currencyUnit("USD")),
	}), Options{IncludeTrace: true})
	kr := firstResult(t, res, "k")
	got := roundTrip(t, kr)
	if got.Code != kr.Code || got.Value.Amount != kr.Value.Amount || got.Value.Available != kr.Value.Available {
		t.Fatalf("got %+v want %+v", got, kr)
	}
}

func TestJSON_Result_RoundTrip(t *testing.T) {
	in := buildDeterminismFixture()
	res := Calculate(in, Options{IncludeTrace: true, IncludeTrend: true, Dimensions: []DimensionKey{{"department": "service"}}})
	got := roundTrip(t, res)
	if got.SchemaVersion != res.SchemaVersion || len(got.KPIResults) != len(res.KPIResults) {
		t.Fatalf("Result did not round-trip: got %d KPIResults, want %d", len(got.KPIResults), len(res.KPIResults))
	}
}

func TestJSON_Issues_RoundTrip(t *testing.T) {
	def := Definition{Code: "a", Unit: currencyUnit("USD"), Formula: KPIExpr(KPIRef{Code: "a"})} // self-cycle
	res := Calculate(onePeriodInput([]Definition{def}, nil), Options{})
	if len(res.DefinitionIssues) == 0 {
		t.Fatalf("expected at least one DefinitionIssue to test round-trip against")
	}
	got := roundTrip(t, res.DefinitionIssues)
	if len(got) != len(res.DefinitionIssues) {
		t.Fatalf("issue count changed across round-trip")
	}
}

// TestJSON_NoNaNOrInf proves the engine never serializes NaN/Inf into
// JSON output — task section 44's explicit "No NaN/Inf" requirement.
// encoding/json itself refuses to marshal NaN/Inf float64 values
// (returns an error), so this test's real assertion is that Calculate's
// OWN guardResult/isNonFinite logic keeps every float in Value.Amount
// finite, meaning json.Marshal never even hits that error path.
func TestJSON_NoNaNOrInf(t *testing.T) {
	def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Binary(OpDivide, Metric(MetricRef{Code: "a"}), Metric(MetricRef{Code: "b"}))}
	res := Calculate(onePeriodInput([]Definition{def}, []MetricValue{
		metricInput("a", "P1", 1, true, currencyUnit("USD")),
		metricInput("b", "P1", 0, true, currencyUnit("USD")), // divide by zero
	}), Options{})
	b, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal should never fail due to NaN/Inf, got: %v", err)
	}
	kr := firstResult(t, res, "k")
	if math.IsNaN(kr.Value.Amount) || math.IsInf(kr.Value.Amount, 0) {
		t.Fatalf("Value.Amount leaked NaN/Inf: %+v", kr.Value)
	}
	_ = b
}
