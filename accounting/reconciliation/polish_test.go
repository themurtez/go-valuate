package reconciliation

import "testing"

func TestHasErrors(t *testing.T) {
	if HasErrors(nil) {
		t.Fatalf("expected false for nil issues")
	}
	if HasErrors([]Issue{{Severity: IssueSeverityWarning}}) {
		t.Fatalf("expected false when only warnings present")
	}
	if !HasErrors([]Issue{{Severity: IssueSeverityWarning}, {Severity: IssueSeverityError}}) {
		t.Fatalf("expected true when an error is present")
	}
}

func TestIsRecognizedType(t *testing.T) {
	for _, typ := range []ReconciliationType{
		"", TypeBank, TypeCreditCard, TypeLoan, TypeARControl, TypeAPControl,
		TypeInventoryControl, TypePayrollClearing, TypeIntercompany,
		TypeSuspenseClearing, TypeGeneric,
	} {
		if !isRecognizedType(typ) {
			t.Errorf("expected %q to be recognized", typ)
		}
	}
	if isRecognizedType("BOGUS") {
		t.Fatalf("expected unrecognized type to return false")
	}
}
