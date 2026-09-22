package valuation

import "testing"

func TestErrorsAndWarnings_SplitBySeverity(t *testing.T) {
	issues := []Issue{
		{Code: "A", Severity: SeverityError, Message: "a"},
		{Code: "B", Severity: SeverityWarning, Message: "b"},
		{Code: "C", Severity: SeverityError, Message: "c"},
	}

	errs := Errors(issues)
	if len(errs) != 2 {
		t.Fatalf("Errors() returned %d issues, want 2", len(errs))
	}
	for _, e := range errs {
		if e.Severity != SeverityError {
			t.Errorf("Errors() returned a non-error issue: %+v", e)
		}
	}

	warnings := Warnings(issues)
	if len(warnings) != 1 {
		t.Fatalf("Warnings() returned %d issues, want 1", len(warnings))
	}
	if warnings[0].Code != "B" {
		t.Errorf("Warnings()[0].Code = %v, want B", warnings[0].Code)
	}
}

func TestHasErrors(t *testing.T) {
	if HasErrors(nil) {
		t.Error("HasErrors(nil) = true, want false")
	}
	if HasErrors([]Issue{{Severity: SeverityWarning}}) {
		t.Error("HasErrors with only a warning = true, want false")
	}
	if !HasErrors([]Issue{{Severity: SeverityWarning}, {Severity: SeverityError}}) {
		t.Error("HasErrors with a mixed set containing an error = false, want true")
	}
}
