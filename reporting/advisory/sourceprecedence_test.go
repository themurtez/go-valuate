package advisory

import "testing"

func TestResolveSourcedMetric_ExplicitPreference(t *testing.T) {
	candidates := map[string]candidateValue{
		"ar":     {Value: AvailableValue(47.2), SourceCode: "dso"},
		"ratios": {Value: AvailableValue(49.8), SourceCode: "days_sales_outstanding"},
	}
	policy := Policy{SourcePreferences: []SourcePreference{{MetricCode: metricCodeDSO, OrderedSources: []string{"ar", "ratios"}}}}

	m, iss := resolveSourcedMetric(metricCodeDSO, "DSO", UnitDays, "2026-01", candidates, nil, policy)

	if iss != nil {
		t.Fatalf("unexpected Issue: %+v", iss)
	}
	if m == nil || m.Value.Amount != 47.2 || m.SourceModule != "ar" {
		t.Fatalf("expected ar's value (47.2) to win, got %+v", m)
	}
}

func TestResolveSourcedMetric_DefaultOrderNoPolicy(t *testing.T) {
	candidates := map[string]candidateValue{
		"ar":     {Value: AvailableValue(47.2), SourceCode: "dso"},
		"ratios": {Value: AvailableValue(49.8), SourceCode: "days_sales_outstanding"},
	}
	m, iss := resolveSourcedMetric(metricCodeDSO, "DSO", UnitDays, "2026-01", candidates, defaultOrder(SourceOrderDSO), Policy{})

	if iss != nil {
		t.Fatalf("unexpected Issue: %+v", iss)
	}
	if m == nil || m.SourceModule != "ar" {
		t.Fatalf("expected default order (ar first) to win, got %+v", m)
	}
}

func TestResolveSourcedMetric_ConflictWithNoPrecedence(t *testing.T) {
	// task section 59's worked example: ar.dso=47.2, ratios.dso=49.8, no
	// precedence configured, no fallback order supplied -> SOURCE_CONFLICT,
	// never averaged to 48.5.
	candidates := map[string]candidateValue{
		"ar":     {Value: AvailableValue(47.2), SourceCode: "dso"},
		"ratios": {Value: AvailableValue(49.8), SourceCode: "days_sales_outstanding"},
	}
	m, iss := resolveSourcedMetric(metricCodeDSO, "DSO", UnitDays, "2026-01", candidates, nil, Policy{ConflictTolerance: 0.01})

	if m != nil {
		t.Fatalf("expected no Metric on conflict, got %+v (value %v — must never be the average 48.5)", m, m.Value.Amount)
	}
	if iss == nil || iss.Code != IssueSourceConflict {
		t.Fatalf("expected IssueSourceConflict, got %+v", iss)
	}
}

func TestResolveSourcedMetric_AgreeingWithinTolerance(t *testing.T) {
	candidates := map[string]candidateValue{
		"ar":     {Value: AvailableValue(47.20), SourceCode: "dso"},
		"ratios": {Value: AvailableValue(47.21), SourceCode: "days_sales_outstanding"},
	}
	m, iss := resolveSourcedMetric(metricCodeDSO, "DSO", UnitDays, "2026-01", candidates, nil, Policy{ConflictTolerance: 0.1})

	if iss != nil {
		t.Fatalf("expected agreement within tolerance to resolve without conflict, got Issue: %+v", iss)
	}
	if m == nil {
		t.Fatal("expected a resolved Metric")
	}
}

func TestResolveSourcedMetric_SingleCandidateNeverConflicts(t *testing.T) {
	candidates := map[string]candidateValue{"ar": {Value: AvailableValue(47.2), SourceCode: "dso"}}
	m, iss := resolveSourcedMetric(metricCodeDSO, "DSO", UnitDays, "2026-01", candidates, nil, Policy{})

	if iss != nil {
		t.Fatalf("single candidate should never conflict, got %+v", iss)
	}
	if m == nil || m.Value.Amount != 47.2 {
		t.Fatalf("expected the single candidate's value, got %+v", m)
	}
}

func TestResolveSourcedMetric_EmptyCandidatesReturnsNothing(t *testing.T) {
	m, iss := resolveSourcedMetric(metricCodeDSO, "DSO", UnitDays, "2026-01", map[string]candidateValue{}, nil, Policy{})
	if m != nil || iss != nil {
		t.Fatalf("expected (nil, nil) for empty candidates, got (%+v, %+v)", m, iss)
	}
}
