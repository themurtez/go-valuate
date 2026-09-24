package kpi

import "testing"

// This file closes coverage gaps in small exported/reachable helper
// functions the main scenario-driven test files don't happen to exercise
// directly (e.g. a helper reachable only via a code path no scenario
// test's inputs trigger, or an exported convenience alias).

func TestUnit_String(t *testing.T) {
	cases := []struct {
		u    Unit
		want string
	}{
		{currencyUnit("USD"), "CURRENCY:USD"},
		{Unit{Kind: UnitCurrency}, "CURRENCY"},
		{Unit{Kind: UnitCustom, CustomLabel: "WIDGETS"}, "CUSTOM:WIDGETS"},
		{Unit{Kind: UnitCustom}, "CUSTOM"},
		{Unit{Kind: UnitRatio}, "RATIO"},
	}
	for _, c := range cases {
		if got := c.u.String(); got != c.want {
			t.Errorf("Unit{%+v}.String() = %q, want %q", c.u, got, c.want)
		}
	}
}

func TestIsRecognizedOperator(t *testing.T) {
	if !isRecognizedOperator(OpAdd) {
		t.Errorf("OpAdd should be recognized")
	}
	if isRecognizedOperator("BOGUS") {
		t.Errorf("BOGUS should not be recognized")
	}
}

func TestHasEvaluationErrors(t *testing.T) {
	if HasEvaluationErrors(nil) {
		t.Errorf("nil should have no errors")
	}
	warningOnly := []EvaluationIssue{{Code: IssueInvalidPeriod, Severity: SeverityWarning}}
	if HasEvaluationErrors(warningOnly) {
		t.Errorf("warning-only should have no errors")
	}
	withError := []EvaluationIssue{{Code: IssueInvalidPeriod, Severity: SeverityError}}
	if !HasEvaluationErrors(withError) {
		t.Errorf("expected HasEvaluationErrors to find the error")
	}
}

func TestEvaluationIssueRank_UnknownCodeFallsToEnd(t *testing.T) {
	if got := evaluationIssueRank("NOT_A_REAL_CODE"); got != len(evaluationIssueCodeOrder) {
		t.Errorf("unranked code should sort to the end, got rank %d want %d", got, len(evaluationIssueCodeOrder))
	}
	if got := evaluationIssueRank(IssueUnknownMetric); got != 0 {
		t.Errorf("IssueUnknownMetric should be rank 0, got %d", got)
	}
}

func TestFirstScalar(t *testing.T) {
	ratio := Unit{Kind: UnitRatio}
	percent := Unit{Kind: UnitPercent}
	unitless := Unit{Kind: UnitUnitless}
	if got := firstScalar(ratio, percent); got.Kind != UnitRatio {
		t.Errorf("ratio should win over percent, got %v", got)
	}
	if got := firstScalar(percent, ratio); got.Kind != UnitRatio {
		t.Errorf("ratio should win regardless of position, got %v", got)
	}
	if got := firstScalar(percent, unitless); got.Kind != UnitPercent {
		t.Errorf("percent should win over unitless, got %v", got)
	}
	if got := firstScalar(unitless, unitless); got.Kind != UnitUnitless {
		t.Errorf("unitless+unitless should stay unitless, got %v", got)
	}
}

func TestDimensionKey_Canonical(t *testing.T) {
	d := DimensionKey{"b": "2", "a": "1", "empty": ""}
	got := d.Canonical()
	if len(got) != 2 {
		t.Fatalf("expected empty-value pair dropped, got %+v", got)
	}
	if got["a"] != "1" || got["b"] != "2" {
		t.Fatalf("got %+v", got)
	}
}

func TestKPIUnit_UnknownCode(t *testing.T) {
	ctx := &evalContext{graph: dependencyGraph{byCode: map[string]Definition{}}}
	if got := ctx.kpiUnit("does_not_exist"); got != (Unit{}) {
		t.Errorf("unknown KPI code should yield zero Unit, got %+v", got)
	}
}

func TestKPIUnit_KnownCode(t *testing.T) {
	ctx := &evalContext{graph: dependencyGraph{byCode: map[string]Definition{
		"k": {Code: "k", Unit: currencyUnit("USD")},
	}}}
	if got := ctx.kpiUnit("k"); !got.Equal(currencyUnit("USD")) {
		t.Errorf("got %+v", got)
	}
}
