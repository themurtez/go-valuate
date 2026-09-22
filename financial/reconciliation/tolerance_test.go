package reconciliation

import "testing"

func TestTolerance_ExactMatch(t *testing.T) {
	tol := Tolerance{Absolute: 1.0}
	status, diff := tol.Evaluate(523421, 523421)
	if status != StatusPass {
		t.Errorf("status = %v, want PASS", status)
	}
	if diff != 0 {
		t.Errorf("difference = %v, want 0", diff)
	}
}

func TestTolerance_WithinAbsoluteTolerance(t *testing.T) {
	// Matches the example from the design brief: reported 523421,
	// reconstructed 523420, difference 1, should PASS.
	tol := Tolerance{Absolute: 1.0}
	status, diff := tol.Evaluate(523421, 523420)
	if status != StatusPass {
		t.Errorf("status = %v, want PASS", status)
	}
	if diff != -1 {
		t.Errorf("difference = %v, want -1", diff)
	}
}

func TestTolerance_OutsideAbsoluteTolerance(t *testing.T) {
	tol := Tolerance{Absolute: 1.0}
	status, diff := tol.Evaluate(523421, 523425)
	if status != StatusFail {
		t.Errorf("status = %v, want FAIL", status)
	}
	if diff != 4 {
		t.Errorf("difference = %v, want 4", diff)
	}
}

func TestTolerance_RelativePercentAllowsLargerAbsoluteDifference(t *testing.T) {
	// 1% of 1,000,000 = 10,000; a difference of 8,000 should pass on
	// relative tolerance even though it exceeds a small absolute tolerance.
	tol := Tolerance{Absolute: 1.0, RelativePercent: 0.01}
	status, diff := tol.Evaluate(1000000, 1008000)
	if status != StatusPass {
		t.Errorf("status = %v, want PASS (within 1%% relative tolerance)", status)
	}
	if diff != 8000 {
		t.Errorf("difference = %v, want 8000", diff)
	}
}

func TestTolerance_RelativePercentExceeded(t *testing.T) {
	tol := Tolerance{Absolute: 1.0, RelativePercent: 0.01}
	status, _ := tol.Evaluate(1000000, 1020000)
	if status != StatusFail {
		t.Errorf("status = %v, want FAIL (exceeds 1%% relative tolerance)", status)
	}
}

func TestTolerance_RelativePercentIgnoredWhenExpectedIsZero(t *testing.T) {
	// A relative tolerance against a zero base is undefined; only the
	// absolute tolerance should apply.
	tol := Tolerance{Absolute: 5.0, RelativePercent: 0.5}
	status, _ := tol.Evaluate(0, 100)
	if status != StatusFail {
		t.Errorf("status = %v, want FAIL (relative tolerance must not apply against a zero expected value)", status)
	}

	status, _ = tol.Evaluate(0, 3)
	if status != StatusPass {
		t.Errorf("status = %v, want PASS (within absolute tolerance)", status)
	}
}

func TestTolerance_NegativeDifferenceUsesAbsoluteValue(t *testing.T) {
	tol := Tolerance{Absolute: 10.0}
	status, diff := tol.Evaluate(100, 85)
	if status != StatusFail {
		t.Errorf("status = %v, want FAIL", status)
	}
	if diff != -15 {
		t.Errorf("difference = %v, want -15", diff)
	}
}

func TestOptions_ZeroToleranceUsesDefault(t *testing.T) {
	opts := Options{}
	tol := opts.tolerance()
	if tol != DefaultTolerance {
		t.Errorf("tolerance = %+v, want DefaultTolerance %+v", tol, DefaultTolerance)
	}
}

func TestOptions_ExplicitToleranceIsUsedVerbatim(t *testing.T) {
	opts := Options{Tolerance: Tolerance{Absolute: 500}}
	tol := opts.tolerance()
	if tol.Absolute != 500 {
		t.Errorf("Absolute = %v, want 500", tol.Absolute)
	}
}
