package kpi

import "testing"

func TestDimensions_ExactMatch(t *testing.T) {
	def := Definition{Code: "revenue_per_fte", Unit: Unit{Kind: UnitCustom, CustomLabel: "USD_PER_COUNT"},
		Formula: Binary(OpDivide, Metric(MetricRef{Code: "revenue"}), Metric(MetricRef{Code: "fte"}))}
	in := onePeriodInput([]Definition{def}, []MetricValue{
		{Code: "revenue", Period: "P1", Dimensions: DimensionKey{"department": "service"}, Value: 500000, Available: true, Unit: currencyUnit("USD"), Aggregation: AggregationSum},
		{Code: "fte", Period: "P1", Dimensions: DimensionKey{"department": "service"}, Value: 5, Available: true, Unit: Unit{Kind: UnitCount}, Aggregation: AggregationSum},
		{Code: "revenue", Period: "P1", Dimensions: DimensionKey{"department": "sales"}, Value: 1000000, Available: true, Unit: currencyUnit("USD"), Aggregation: AggregationSum},
		{Code: "fte", Period: "P1", Dimensions: DimensionKey{"department": "sales"}, Value: 10, Available: true, Unit: Unit{Kind: UnitCount}, Aggregation: AggregationSum},
	})
	res := Calculate(in, Options{Dimensions: []DimensionKey{{"department": "service"}, {"department": "sales"}}})

	byDim := map[string]KPIResult{}
	for _, kr := range res.KPIResults {
		byDim[kr.Dimensions.hashKey()] = kr
	}
	service := byDim[DimensionKey{"department": "service"}.hashKey()]
	if !service.Value.Available || service.Value.Amount != 100000 {
		t.Fatalf("service got %+v", service.Value)
	}
	sales := byDim[DimensionKey{"department": "sales"}.hashKey()]
	if !sales.Value.Available || sales.Value.Amount != 100000 {
		t.Fatalf("sales got %+v", sales.Value)
	}
}

func TestDimensions_Unavailable_NoBroadcast(t *testing.T) {
	// A business-level-only metric referenced from a dimensioned
	// evaluation group, with no Broadcast set, must be unavailable — task
	// section 5's strict default.
	def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Metric(MetricRef{Code: "revenue"})}
	in := onePeriodInput([]Definition{def}, []MetricValue{
		metricInput("revenue", "P1", 1000000, true, currencyUnit("USD")), // business-level, no Dimensions
	})
	res := Calculate(in, Options{Dimensions: []DimensionKey{{"department": "service"}}})

	var deptResult KPIResult
	for _, kr := range res.KPIResults {
		if !kr.Dimensions.IsBusinessLevel() {
			deptResult = kr
		}
	}
	if deptResult.Value.Available {
		t.Fatalf("business-level metric should not silently broadcast into a dimensioned KPI, got %+v", deptResult.Value)
	}
	if deptResult.Value.Reason != AvailabilityDimensionUnavailable && deptResult.Value.Reason != AvailabilityMissingMetric {
		t.Fatalf("expected DIMENSION_UNAVAILABLE or MISSING_METRIC, got %v", deptResult.Value.Reason)
	}
}

func TestDimensions_ExplicitBroadcast(t *testing.T) {
	// With Broadcast: true, a business-level metric DOES resolve for a
	// dimensioned evaluation group -- task section 5's explicit opt-in.
	def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Metric(MetricRef{Broadcast: true, Code: "company_overhead"})}
	in := onePeriodInput([]Definition{def}, []MetricValue{
		metricInput("company_overhead", "P1", 50000, true, currencyUnit("USD")),
	})
	res := Calculate(in, Options{Dimensions: []DimensionKey{{"department": "service"}}})

	var deptResult KPIResult
	for _, kr := range res.KPIResults {
		if !kr.Dimensions.IsBusinessLevel() {
			deptResult = kr
		}
	}
	if !deptResult.Value.Available || deptResult.Value.Amount != 50000 {
		t.Fatalf("explicit Broadcast should resolve business-level metric, got %+v", deptResult.Value)
	}
}

func TestDimensions_CanonicalEquality(t *testing.T) {
	a := DimensionKey{"department": "service", "location": "toronto"}
	b := DimensionKey{"location": "toronto", "department": "service"} // different insertion order
	if !a.Equal(b) {
		t.Fatalf("dimension keys with same pairs in different order should be equal")
	}
	c := DimensionKey{"department": "service"}
	if a.Equal(c) {
		t.Fatalf("dimension keys with different pairs should not be equal")
	}
}

func TestDimensions_CanonicalEquality_EmptyValueDropped(t *testing.T) {
	a := DimensionKey{"department": "service", "region": ""}
	b := DimensionKey{"department": "service"}
	if !a.Equal(b) {
		t.Fatalf("an empty-string tag value should canonicalize away, making these equal")
	}
}

func TestDimensions_NoImplicitBroadcast_RevenuePerLocation(t *testing.T) {
	// RevenuePerLocation by location -- no accidental cross-dimension
	// mixing (task section 36).
	def := Definition{Code: "revenue_per_location", Unit: currencyUnit("USD"), Formula: Metric(MetricRef{Code: "revenue"})}
	in := onePeriodInput([]Definition{def}, []MetricValue{
		{Code: "revenue", Period: "P1", Dimensions: DimensionKey{"location": "toronto"}, Value: 100, Available: true, Unit: currencyUnit("USD"), Aggregation: AggregationSum},
		{Code: "revenue", Period: "P1", Dimensions: DimensionKey{"location": "vancouver"}, Value: 200, Available: true, Unit: currencyUnit("USD"), Aggregation: AggregationSum},
	})
	res := Calculate(in, Options{Dimensions: []DimensionKey{{"location": "toronto"}, {"location": "vancouver"}}})

	byDim := map[string]float64{}
	for _, kr := range res.KPIResults {
		if kr.Value.Available {
			byDim[kr.Dimensions.hashKey()] = kr.Value.Amount
		}
	}
	if byDim[DimensionKey{"location": "toronto"}.hashKey()] != 100 {
		t.Fatalf("toronto mixed with another location's data")
	}
	if byDim[DimensionKey{"location": "vancouver"}.hashKey()] != 200 {
		t.Fatalf("vancouver mixed with another location's data")
	}
}
