package covenants

import (
	"strings"
	"testing"
)

// TestCalculate_Pass covers a straightforward passing covenant test with
// comfortable headroom and no configured warning buffer.
func TestCalculate_Pass(t *testing.T) {
	res := Calculate(Input{Tests: []CovenantTest{
		{
			CovenantID: "MIN_DSCR",
			Label:      "Minimum DSCR",
			Metric:     MetricDSCR,
			Operator:   OperatorGTE,
			Threshold:  1.25,
			Actual:     AvailableValue(1.60),
			Period:     "2025-Q3",
		},
	}})

	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	if len(res.Tests) != 1 {
		t.Fatalf("expected 1 test result, got %d", len(res.Tests))
	}
	tr := res.Tests[0]
	if tr.Status != StatusPass {
		t.Errorf("Status = %q, want %q", tr.Status, StatusPass)
	}
	if !tr.Headroom.Available || !approxEqual(tr.Headroom.Amount, 0.35, 1e-9) {
		t.Errorf("Headroom = %+v, want available ~0.35", tr.Headroom)
	}
	if tr.WarningBufferStatus != WarningBufferNotConfigured {
		t.Errorf("WarningBufferStatus = %q, want %q", tr.WarningBufferStatus, WarningBufferNotConfigured)
	}
	if tr.Explanation == "" {
		t.Error("expected a non-empty Explanation")
	}

	if res.Summary.TestCount != 1 || res.Summary.Breaches != 0 || res.Summary.NearBreaches != 0 || res.Summary.Unavailable != 0 {
		t.Errorf("unexpected Summary: %+v", res.Summary)
	}
	if len(res.Errors) != 0 {
		t.Errorf("expected no Errors, got %+v", res.Errors)
	}
}

// TestCalculate_Breach covers a covenant test that fails its threshold,
// verifying Headroom reports the negative breach amount and Summary
// counts it.
func TestCalculate_Breach(t *testing.T) {
	res := Calculate(Input{Tests: []CovenantTest{
		{
			CovenantID: "MIN_DSCR",
			Metric:     MetricDSCR,
			Operator:   OperatorGTE,
			Threshold:  1.25,
			Actual:     AvailableValue(1.10),
			Period:     "2025-Q3",
		},
	}})

	tr := res.Tests[0]
	if tr.Status != StatusFail {
		t.Fatalf("Status = %q, want %q", tr.Status, StatusFail)
	}
	if !tr.Headroom.Available {
		t.Fatal("expected Headroom to be available on a breach")
	}
	if got, want := tr.Headroom.Amount, -0.15; !approxEqual(got, want, 1e-9) {
		t.Errorf("Headroom.Amount = %v, want %v", got, want)
	}
	if tr.WarningBufferStatus != WarningBufferNotConfigured {
		t.Errorf("WarningBufferStatus = %q, want %q (buffer only classifies a passing test)", tr.WarningBufferStatus, WarningBufferNotConfigured)
	}

	if res.Summary.Breaches != 1 {
		t.Errorf("Summary.Breaches = %d, want 1", res.Summary.Breaches)
	}
	if len(res.Summary.BreachedCovenantIDs) != 1 || res.Summary.BreachedCovenantIDs[0] != "MIN_DSCR" {
		t.Errorf("BreachedCovenantIDs = %v, want [MIN_DSCR]", res.Summary.BreachedCovenantIDs)
	}
}

// TestCalculate_BreachLTE covers a maximum-style covenant (OperatorLTE),
// verifying the headroom direction flips correctly (threshold - actual)
// relative to the minimum-style OperatorGTE case above.
func TestCalculate_BreachLTE(t *testing.T) {
	res := Calculate(Input{Tests: []CovenantTest{
		{
			CovenantID: "MAX_LEVERAGE",
			Metric:     MetricDebtToEBITDA,
			Operator:   OperatorLTE,
			Threshold:  4.0,
			Actual:     AvailableValue(4.5),
			Period:     "2025-Q3",
		},
	}})

	tr := res.Tests[0]
	if tr.Status != StatusFail {
		t.Fatalf("Status = %q, want %q", tr.Status, StatusFail)
	}
	if got, want := tr.Headroom.Amount, -0.5; !approxEqual(got, want, 1e-9) {
		t.Errorf("Headroom.Amount = %v, want %v", got, want)
	}
}

// TestCalculate_NearBreach covers a passing test that falls within the
// configured warning buffer (both the percent-of-threshold and the
// absolute-amount legs are exercised in separate subtests, mirroring
// review.IsMaterial/variance.Policy's OR-of-two-legs pattern).
func TestCalculate_NearBreach(t *testing.T) {
	t.Run("percent leg", func(t *testing.T) {
		res := Calculate(Input{Tests: []CovenantTest{
			{
				CovenantID:           "MIN_DSCR",
				Metric:               MetricDSCR,
				Operator:             OperatorGTE,
				Threshold:            1.25,
				Actual:               AvailableValue(1.30), // headroom 0.05
				WarningBufferPercent: 0.10,                 // 10% of 1.25 = 0.125 floor
				Period:               "2025-Q3",
			},
		}})
		tr := res.Tests[0]
		if tr.Status != StatusPass {
			t.Fatalf("Status = %q, want %q", tr.Status, StatusPass)
		}
		if tr.WarningBufferStatus != WarningBufferWithinBuffer {
			t.Errorf("WarningBufferStatus = %q, want %q", tr.WarningBufferStatus, WarningBufferWithinBuffer)
		}
		if res.Summary.NearBreaches != 1 {
			t.Errorf("Summary.NearBreaches = %d, want 1", res.Summary.NearBreaches)
		}
		if res.Summary.Breaches != 0 {
			t.Errorf("Summary.Breaches = %d, want 0 (a near breach is not also counted as a breach)", res.Summary.Breaches)
		}
	})

	t.Run("amount leg", func(t *testing.T) {
		res := Calculate(Input{Tests: []CovenantTest{
			{
				CovenantID:          "MIN_DSCR",
				Metric:              MetricDSCR,
				Operator:            OperatorGTE,
				Threshold:           1.25,
				Actual:              AvailableValue(1.30), // headroom 0.05
				WarningBufferAmount: 0.10,                 // flat 0.10 floor
				Period:              "2025-Q3",
			},
		}})
		tr := res.Tests[0]
		if tr.WarningBufferStatus != WarningBufferWithinBuffer {
			t.Errorf("WarningBufferStatus = %q, want %q", tr.WarningBufferStatus, WarningBufferWithinBuffer)
		}
	})

	t.Run("outside buffer", func(t *testing.T) {
		res := Calculate(Input{Tests: []CovenantTest{
			{
				CovenantID:           "MIN_DSCR",
				Metric:               MetricDSCR,
				Operator:             OperatorGTE,
				Threshold:            1.25,
				Actual:               AvailableValue(1.60), // headroom 0.35, well clear
				WarningBufferPercent: 0.10,
				Period:               "2025-Q3",
			},
		}})
		tr := res.Tests[0]
		if tr.WarningBufferStatus != WarningBufferOutsideBuffer {
			t.Errorf("WarningBufferStatus = %q, want %q", tr.WarningBufferStatus, WarningBufferOutsideBuffer)
		}
		if res.Summary.NearBreaches != 0 {
			t.Errorf("Summary.NearBreaches = %d, want 0", res.Summary.NearBreaches)
		}
	})

	t.Run("failed test never classified within buffer", func(t *testing.T) {
		res := Calculate(Input{Tests: []CovenantTest{
			{
				CovenantID:           "MIN_DSCR",
				Metric:               MetricDSCR,
				Operator:             OperatorGTE,
				Threshold:            1.25,
				Actual:               AvailableValue(1.10), // fails
				WarningBufferPercent: 0.50,                 // would trivially "trigger" if not gated
				Period:               "2025-Q3",
			},
		}})
		tr := res.Tests[0]
		if tr.Status != StatusFail {
			t.Fatalf("Status = %q, want %q", tr.Status, StatusFail)
		}
		if tr.WarningBufferStatus != WarningBufferNotApplicable {
			t.Errorf("WarningBufferStatus = %q, want %q", tr.WarningBufferStatus, WarningBufferNotApplicable)
		}
	})
}

// TestCalculate_UnavailableMetric covers a covenant test whose Actual
// value is unavailable (the upstream metric could not be computed for
// this period), verifying it is reported as StatusUnavailable rather than
// silently treated as zero or dropped from output.
func TestCalculate_UnavailableMetric(t *testing.T) {
	res := Calculate(Input{Tests: []CovenantTest{
		{
			CovenantID: "MIN_NET_WORTH",
			Metric:     MetricMinimumNetWorth,
			Operator:   OperatorGTE,
			Threshold:  1_000_000,
			Actual:     Unavailable(),
			Period:     "2025-Q3",
		},
	}})

	if len(res.Tests) != 1 {
		t.Fatalf("expected 1 test result even though Actual is unavailable, got %d", len(res.Tests))
	}
	tr := res.Tests[0]
	if tr.Status != StatusUnavailable {
		t.Errorf("Status = %q, want %q", tr.Status, StatusUnavailable)
	}
	if tr.Headroom.Available {
		t.Errorf("Headroom = %+v, want unavailable", tr.Headroom)
	}
	if tr.WarningBufferStatus != WarningBufferNotApplicable {
		t.Errorf("WarningBufferStatus = %q, want %q", tr.WarningBufferStatus, WarningBufferNotApplicable)
	}
	if tr.Explanation == "" {
		t.Error("expected a non-empty Explanation even when unavailable")
	}

	if res.Summary.Unavailable != 1 {
		t.Errorf("Summary.Unavailable = %d, want 1", res.Summary.Unavailable)
	}
	if len(res.Summary.UnavailableCovenantIDs) != 1 || res.Summary.UnavailableCovenantIDs[0] != "MIN_NET_WORTH" {
		t.Errorf("UnavailableCovenantIDs = %v, want [MIN_NET_WORTH]", res.Summary.UnavailableCovenantIDs)
	}

	if HasErrors(res.Warnings) {
		t.Error("an unavailable Actual should be a Warning, not an Error")
	}
	found := false
	for _, w := range res.Warnings {
		if w.Code == IssueActualUnavailable {
			found = true
		}
	}
	if !found {
		t.Errorf("expected IssueActualUnavailable in Warnings, got %+v", res.Warnings)
	}
}

// TestCalculate_WrongOperator covers a covenant test with an unrecognized
// Operator value, verifying it is reported as StatusUnavailable with a
// structured Issue rather than panicking, silently passing, or silently
// failing.
func TestCalculate_WrongOperator(t *testing.T) {
	res := Calculate(Input{Tests: []CovenantTest{
		{
			CovenantID: "MIN_DSCR",
			Metric:     MetricDSCR,
			Operator:   Operator("~="), // not a recognized Operator constant
			Threshold:  1.25,
			Actual:     AvailableValue(1.60),
			Period:     "2025-Q3",
		},
	}})

	tr := res.Tests[0]
	if tr.Status != StatusUnavailable {
		t.Errorf("Status = %q, want %q", tr.Status, StatusUnavailable)
	}
	if tr.Headroom.Available {
		t.Errorf("Headroom = %+v, want unavailable for an unrecognized operator", tr.Headroom)
	}

	found := false
	for _, w := range res.Warnings {
		if w.Code == IssueInvalidOperator {
			found = true
			if w.CovenantID != "MIN_DSCR" {
				t.Errorf("Issue.CovenantID = %q, want MIN_DSCR", w.CovenantID)
			}
		}
	}
	if !found {
		t.Errorf("expected IssueInvalidOperator in Warnings, got %+v", res.Warnings)
	}
}

// TestCalculate_EmptyOperator covers the zero-value Operator ("") as a
// distinct case from a garbled-but-nonempty operator string.
func TestCalculate_EmptyOperator(t *testing.T) {
	res := Calculate(Input{Tests: []CovenantTest{
		{CovenantID: "X", Actual: AvailableValue(1.0), Threshold: 1.0, Period: "2025"},
	}})
	if res.Tests[0].Status != StatusUnavailable {
		t.Errorf("Status = %q, want %q", res.Tests[0].Status, StatusUnavailable)
	}
}

// TestCalculate_MissingCovenantID covers a test with an empty CovenantID,
// verifying it is still included in output (so every input row is
// reflected) but marked unavailable.
func TestCalculate_MissingCovenantID(t *testing.T) {
	res := Calculate(Input{Tests: []CovenantTest{
		{Operator: OperatorGTE, Threshold: 1.25, Actual: AvailableValue(1.6), Period: "2025-Q3"},
	}})

	if len(res.Tests) != 1 {
		t.Fatalf("expected 1 test result, got %d", len(res.Tests))
	}
	if res.Tests[0].Status != StatusUnavailable {
		t.Errorf("Status = %q, want %q", res.Tests[0].Status, StatusUnavailable)
	}

	found := false
	for _, w := range res.Warnings {
		if w.Code == IssueMissingCovenantID {
			found = true
		}
	}
	if !found {
		t.Errorf("expected IssueMissingCovenantID in Warnings, got %+v", res.Warnings)
	}
}

// TestCalculate_DuplicateCovenantID covers two tests sharing the same
// CovenantID and Period, verifying both are still evaluated independently
// and a DuplicateCovenantID Issue is recorded.
func TestCalculate_DuplicateCovenantID(t *testing.T) {
	res := Calculate(Input{Tests: []CovenantTest{
		{CovenantID: "MIN_DSCR", Operator: OperatorGTE, Threshold: 1.25, Actual: AvailableValue(1.6), Period: "2025-Q3"},
		{CovenantID: "MIN_DSCR", Operator: OperatorGTE, Threshold: 1.25, Actual: AvailableValue(1.1), Period: "2025-Q3"},
	}})

	if len(res.Tests) != 2 {
		t.Fatalf("expected both duplicate tests evaluated, got %d results", len(res.Tests))
	}
	if res.Tests[0].Status != StatusPass || res.Tests[1].Status != StatusFail {
		t.Errorf("expected the two duplicates to be evaluated independently, got %q and %q", res.Tests[0].Status, res.Tests[1].Status)
	}

	found := false
	for _, w := range res.Warnings {
		if w.Code == IssueDuplicateCovenantID {
			found = true
		}
	}
	if !found {
		t.Errorf("expected IssueDuplicateCovenantID in Warnings, got %+v", res.Warnings)
	}
}

// TestCalculate_CustomMetric covers MetricCustom with a caller-supplied
// CustomMetricLabel, verifying this package evaluates it identically to
// any named metric and reflects the label in Explanation.
func TestCalculate_CustomMetric(t *testing.T) {
	res := Calculate(Input{Tests: []CovenantTest{
		{
			CovenantID:        "MAX_OWNER_DIST",
			Metric:            MetricCustom,
			CustomMetricLabel: "Maximum Owner Distributions",
			Operator:          OperatorLTE,
			Threshold:         250_000,
			Actual:            AvailableValue(300_000),
			Period:            "2025-Q3",
		},
	}})

	tr := res.Tests[0]
	if tr.Metric != MetricCustom {
		t.Errorf("Metric = %q, want %q", tr.Metric, MetricCustom)
	}
	if tr.Status != StatusFail {
		t.Errorf("Status = %q, want %q", tr.Status, StatusFail)
	}
	if got, want := tr.Headroom.Amount, -50_000.0; !approxEqual(got, want, 1e-6) {
		t.Errorf("Headroom.Amount = %v, want %v", got, want)
	}
	if tr.CustomMetricLabel != "Maximum Owner Distributions" {
		t.Errorf("CustomMetricLabel = %q, want echoed value", tr.CustomMetricLabel)
	}
}

// TestCalculate_CureGrace covers CureGrace metadata passing through
// unchanged to TestResult, without affecting Status/Headroom/
// WarningBufferStatus — this package never encodes legal interpretation
// of a cure/grace provision (see the package doc comment).
func TestCalculate_CureGrace(t *testing.T) {
	cg := &CureGrace{Description: "10 business days to cure following notice", GraceDays: 5, CureDays: 10}
	res := Calculate(Input{Tests: []CovenantTest{
		{
			CovenantID: "MIN_DSCR",
			Operator:   OperatorGTE,
			Threshold:  1.25,
			Actual:     AvailableValue(1.10),
			Period:     "2025-Q3",
			CureGrace:  cg,
		},
	}})

	tr := res.Tests[0]
	if tr.Status != StatusFail {
		t.Fatalf("Status = %q, want %q (CureGrace must not change the evaluation)", tr.Status, StatusFail)
	}
	if tr.CureGrace == nil || *tr.CureGrace != *cg {
		t.Errorf("CureGrace = %+v, want echoed %+v", tr.CureGrace, cg)
	}
}

// TestCalculate_MultiplePeriods covers a batch of tests spanning more
// than one Period, verifying Summary.ByPeriod aggregates each period
// independently and in lexical order (this package draws no
// chronological inference from financial.Period — see its doc comment).
func TestCalculate_MultiplePeriods(t *testing.T) {
	res := Calculate(Input{Tests: []CovenantTest{
		{CovenantID: "MIN_DSCR", Operator: OperatorGTE, Threshold: 1.25, Actual: AvailableValue(1.60), Period: "2025-Q2"},
		{CovenantID: "MAX_LEVERAGE", Operator: OperatorLTE, Threshold: 4.0, Actual: AvailableValue(4.5), Period: "2025-Q2"},
		{CovenantID: "MIN_DSCR", Operator: OperatorGTE, Threshold: 1.25, Actual: AvailableValue(1.10), Period: "2025-Q3"},
		{CovenantID: "MAX_LEVERAGE", Operator: OperatorLTE, Threshold: 4.0, Actual: AvailableValue(3.0), Period: "2025-Q3"},
		{CovenantID: "MIN_NET_WORTH", Operator: OperatorGTE, Threshold: 1_000_000, Actual: Unavailable(), Period: "2025-Q1"},
	}})

	if res.Summary.TestCount != 5 {
		t.Fatalf("Summary.TestCount = %d, want 5", res.Summary.TestCount)
	}
	if res.Summary.Breaches != 2 { // Q2 leverage, Q3 DSCR
		t.Errorf("Summary.Breaches = %d, want 2", res.Summary.Breaches)
	}
	if res.Summary.Unavailable != 1 {
		t.Errorf("Summary.Unavailable = %d, want 1", res.Summary.Unavailable)
	}

	if len(res.Summary.ByPeriod) != 3 {
		t.Fatalf("Summary.ByPeriod length = %d, want 3", len(res.Summary.ByPeriod))
	}
	// Lexical order: 2025-Q1, 2025-Q2, 2025-Q3.
	wantOrder := []string{"2025-Q1", "2025-Q2", "2025-Q3"}
	for i, want := range wantOrder {
		if string(res.Summary.ByPeriod[i].Period) != want {
			t.Errorf("ByPeriod[%d].Period = %q, want %q", i, res.Summary.ByPeriod[i].Period, want)
		}
	}
	q2 := res.Summary.ByPeriod[1]
	if q2.TestCount != 2 || q2.Breaches != 1 {
		t.Errorf("2025-Q2 summary = %+v, want TestCount=2 Breaches=1", q2)
	}
	q3 := res.Summary.ByPeriod[2]
	if q3.TestCount != 2 || q3.Breaches != 1 {
		t.Errorf("2025-Q3 summary = %+v, want TestCount=2 Breaches=1", q3)
	}
	q1 := res.Summary.ByPeriod[0]
	if q1.TestCount != 1 || q1.Unavailable != 1 {
		t.Errorf("2025-Q1 summary = %+v, want TestCount=1 Unavailable=1", q1)
	}
}

// TestCalculate_NoTests covers the degenerate empty-input case.
func TestCalculate_NoTests(t *testing.T) {
	res := Calculate(Input{})
	if res.Available {
		t.Error("expected Available == false for empty Input.Tests")
	}
	if len(res.Errors) != 1 || res.Errors[0].Code != IssueNoTests {
		t.Errorf("Errors = %+v, want a single IssueNoTests", res.Errors)
	}
	if !HasErrors(res.Errors) {
		t.Error("HasErrors should report true for an Error-severity Issue")
	}
}

// TestCalculate_OperatorEQ covers the equality operator, verifying it
// evaluates pass/fail correctly while leaving Headroom unavailable (no
// single safe direction exists for an exact-equality covenant — see
// computeHeadroom's doc comment).
func TestCalculate_OperatorEQ(t *testing.T) {
	t.Run("pass", func(t *testing.T) {
		res := Calculate(Input{Tests: []CovenantTest{
			{CovenantID: "EXACT", Operator: OperatorEQ, Threshold: 100, Actual: AvailableValue(100), Period: "2025"},
		}})
		if res.Tests[0].Status != StatusPass {
			t.Errorf("Status = %q, want %q", res.Tests[0].Status, StatusPass)
		}
		if res.Tests[0].Headroom.Available {
			t.Errorf("Headroom = %+v, want unavailable for OperatorEQ", res.Tests[0].Headroom)
		}
	})
	t.Run("fail", func(t *testing.T) {
		res := Calculate(Input{Tests: []CovenantTest{
			{CovenantID: "EXACT", Operator: OperatorEQ, Threshold: 100, Actual: AvailableValue(99), Period: "2025"},
		}})
		if res.Tests[0].Status != StatusFail {
			t.Errorf("Status = %q, want %q", res.Tests[0].Status, StatusFail)
		}
	})
}

// TestCalculate_NoMutation proves Calculate never mutates caller-owned
// Input, per this package's architectural guarantee.
func TestCalculate_NoMutation(t *testing.T) {
	tests := []CovenantTest{
		{CovenantID: "MIN_DSCR", Operator: OperatorGTE, Threshold: 1.25, Actual: AvailableValue(1.6), Period: "2025-Q3"},
	}
	in := Input{Tests: tests}
	snapshot := in.Tests[0]

	_ = Calculate(in)

	if in.Tests[0] != snapshot {
		t.Errorf("Calculate mutated Input.Tests[0]: got %+v, want %+v", in.Tests[0], snapshot)
	}
}

// TestCalculate_ExplanationNegativeZero covers a covenant whose Actual/
// Threshold produce a headroom or figure with a magnitude too small to
// show at 4 decimal places (e.g. -0.00003), verifying Explanation renders
// it as "0" rather than the misleading "-0" fmt.Sprintf's default
// trailing-zero trim would otherwise produce.
func TestCalculate_ExplanationNegativeZero(t *testing.T) {
	res := Calculate(Input{Tests: []CovenantTest{
		{
			CovenantID: "MIN_NET_INCOME",
			Operator:   OperatorGTE,
			Threshold:  -0.00003,
			Actual:     AvailableValue(-0.00003),
			Period:     "2025-Q3",
		},
	}})

	tr := res.Tests[0]
	if tr.Status != StatusPass {
		t.Fatalf("Status = %q, want %q", tr.Status, StatusPass)
	}
	if strings.Contains(tr.Explanation, "-0") {
		t.Errorf("Explanation = %q, want no \"-0\" artifact", tr.Explanation)
	}
}

func approxEqual(got, want, tolerance float64) bool {
	d := got - want
	if d < 0 {
		d = -d
	}
	return d <= tolerance
}
