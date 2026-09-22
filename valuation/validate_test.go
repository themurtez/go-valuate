package valuation

import (
	"math"
	"testing"
)

func nan() float64    { return math.NaN() }
func posInf() float64 { return math.Inf(1) }

func TestValidateResultEnvelope_ValidEnvelope(t *testing.T) {
	issues := ValidateResultEnvelope(CodeEBITDAMultiple, "1.0.0", ValueTypeEnterprise)
	if len(issues) != 0 {
		t.Fatalf("ValidateResultEnvelope() = %+v, want no issues", issues)
	}
}

func TestValidateResultEnvelope_EmptyMethodCode(t *testing.T) {
	issues := ValidateResultEnvelope("", "1.0.0", ValueTypeEnterprise)
	if !hasCode(issues, IssueEmptyMethodCode) {
		t.Fatalf("ValidateResultEnvelope() = %+v, want IssueEmptyMethodCode", issues)
	}
}

func TestValidateResultEnvelope_UnknownMethodCode(t *testing.T) {
	issues := ValidateResultEnvelope(Code("NOT_A_METHOD"), "1.0.0", ValueTypeEnterprise)
	if !hasCode(issues, IssueEmptyMethodCode) {
		t.Fatalf("ValidateResultEnvelope() = %+v, want IssueEmptyMethodCode for an unrecognized code", issues)
	}
}

func TestValidateResultEnvelope_EmptyVersion(t *testing.T) {
	issues := ValidateResultEnvelope(CodeSDEMultiple, "", ValueTypeEquity)
	if !hasCode(issues, IssueEmptyVersion) {
		t.Fatalf("ValidateResultEnvelope() = %+v, want IssueEmptyVersion", issues)
	}
}

func TestValidateResultEnvelope_UnknownValueBasis(t *testing.T) {
	issues := ValidateResultEnvelope(CodeSDEMultiple, "1.0.0", ValueType("not_a_basis"))
	if !hasCode(issues, IssueUnknownValueBasis) {
		t.Fatalf("ValidateResultEnvelope() = %+v, want IssueUnknownValueBasis", issues)
	}
}

func TestValidateResultEnvelope_EveryProblemReportedTogether(t *testing.T) {
	issues := ValidateResultEnvelope("", "", "")
	if len(issues) != 3 {
		t.Fatalf("ValidateResultEnvelope() returned %d issues, want 3 (one per empty field)", len(issues))
	}
	for _, iss := range issues {
		if iss.Severity != SeverityError {
			t.Errorf("issue %+v has severity %v, want SeverityError", iss, iss.Severity)
		}
	}
}

func TestValidateFiniteSteps_AllFinite(t *testing.T) {
	steps := []Step{{Label: "a", Value: 100}, {Label: "b", Value: -50.5}, {Label: "c", Value: 0}}
	if issues := ValidateFiniteSteps(steps); len(issues) != 0 {
		t.Fatalf("ValidateFiniteSteps() = %+v, want no issues", issues)
	}
}

func TestValidateFiniteSteps_NaNDetected(t *testing.T) {
	steps := []Step{{Label: "good", Value: 100}, {Label: "bad", Value: nan()}}
	issues := ValidateFiniteSteps(steps)
	if !hasCode(issues, IssueNonFiniteStep) {
		t.Fatalf("ValidateFiniteSteps() = %+v, want IssueNonFiniteStep", issues)
	}
}

func TestValidateFiniteSteps_InfDetected(t *testing.T) {
	steps := []Step{{Label: "bad", Value: posInf()}}
	issues := ValidateFiniteSteps(steps)
	if !hasCode(issues, IssueNonFiniteStep) {
		t.Fatalf("ValidateFiniteSteps() = %+v, want IssueNonFiniteStep", issues)
	}
}

func TestValidateFiniteSteps_Empty(t *testing.T) {
	if issues := ValidateFiniteSteps(nil); len(issues) != 0 {
		t.Fatalf("ValidateFiniteSteps(nil) = %+v, want no issues", issues)
	}
}

func hasCode(issues []Issue, code IssueCode) bool {
	for _, iss := range issues {
		if iss.Code == code {
			return true
		}
	}
	return false
}
