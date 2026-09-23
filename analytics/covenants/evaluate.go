package covenants

import (
	"fmt"
	"math"
)

// evaluateTest evaluates a single CovenantTest, returning its TestResult
// plus any Issue found. ref identifies this test for Issue.CovenantID
// when t.CovenantID itself is empty (the problem being reported).
func evaluateTest(t CovenantTest, ref string) (TestResult, []Issue) {
	res := TestResult{
		CovenantID:        t.CovenantID,
		Label:             t.Label,
		Metric:            t.Metric,
		CustomMetricLabel: t.CustomMetricLabel,
		Operator:          t.Operator,
		Threshold:         t.Threshold,
		Period:            t.Period,
		CureGrace:         t.CureGrace,
		Actual:            t.Actual,
	}

	var issues []Issue
	covenantRef := t.CovenantID
	if covenantRef == "" {
		covenantRef = ref
	}

	if t.CovenantID == "" {
		issues = append(issues, Issue{
			Code:       IssueMissingCovenantID,
			Severity:   SeverityWarning,
			Message:    fmt.Sprintf("%s: covenant_id is empty; test reported as unavailable", ref),
			CovenantID: covenantRef,
		})
		res.Status = StatusUnavailable
		res.WarningBufferStatus = WarningBufferNotApplicable
		res.Explanation = "covenant ID is missing; this test cannot be identified or evaluated"
		return res, issues
	}

	above, ok := isDirectionAbove(t.Operator)
	if !ok {
		issues = append(issues, Issue{
			Code:       IssueInvalidOperator,
			Severity:   SeverityWarning,
			Message:    fmt.Sprintf("%s: operator %q is not a recognized Operator value; test reported as unavailable", covenantRef, t.Operator),
			CovenantID: covenantRef,
		})
		res.Status = StatusUnavailable
		res.WarningBufferStatus = WarningBufferNotApplicable
		res.Explanation = fmt.Sprintf("operator %q is not recognized; this test cannot be evaluated", t.Operator)
		return res, issues
	}

	if !t.Actual.Available {
		issues = append(issues, Issue{
			Code:       IssueActualUnavailable,
			Severity:   SeverityWarning,
			Message:    fmt.Sprintf("%s: actual value is unavailable; test reported as unavailable", covenantRef),
			CovenantID: covenantRef,
		})
		res.Status = StatusUnavailable
		res.WarningBufferStatus = WarningBufferNotApplicable
		res.Explanation = "actual value is unavailable; this test cannot be evaluated"
		return res, issues
	}

	if isInvalidFloat(t.Actual.Amount) || isInvalidFloat(t.Threshold) {
		issues = append(issues, Issue{
			Code:       IssueInvalidActualOrThreshold,
			Severity:   SeverityWarning,
			Message:    fmt.Sprintf("%s: actual or threshold is NaN or infinite; test reported as unavailable", covenantRef),
			CovenantID: covenantRef,
		})
		res.Status = StatusUnavailable
		res.WarningBufferStatus = WarningBufferNotApplicable
		res.Explanation = "actual or threshold is NaN or infinite; this test cannot be evaluated"
		return res, issues
	}

	passed := evaluateOperator(t.Operator, t.Actual.Amount, t.Threshold)
	if passed {
		res.Status = StatusPass
	} else {
		res.Status = StatusFail
	}

	res.Headroom = computeHeadroom(t.Operator, above, t.Actual.Amount, t.Threshold)
	res.WarningBufferStatus = classifyWarningBuffer(t, res.Status, res.Headroom)
	res.Explanation = buildExplanation(t, res)

	return res, issues
}

// computeHeadroom returns the signed distance between actual and
// threshold in the direction of safety for op: positive means actual has
// room before breaching, negative means actual has already breached (by
// that amount). For OperatorGTE/OperatorGT (above == true), headroom is
// actual - threshold (more is safer). For OperatorLTE/OperatorLT
// (above == false), headroom is threshold - actual (less is safer). For
// OperatorEQ there is no single safe direction (any deviation either way
// is a breach), so headroom is left unavailable — a signed "distance from
// exact equality" would not usefully answer "how much room before
// breach" the way it does for an inequality.
func computeHeadroom(op Operator, above bool, actual, threshold float64) Value {
	if op == OperatorEQ {
		return Unavailable()
	}
	if above {
		return AvailableValue(actual - threshold)
	}
	return AvailableValue(threshold - actual)
}

// classifyWarningBuffer applies t's WarningBufferPercent/WarningBufferAmount
// to headroom, following the same OR-of-two-legs, off-by-default pattern
// review.IsMaterial and variance.Policy already use: a test triggers
// WarningBufferWithinBuffer if headroom falls at or below either
// configured leg. Both legs default to 0 (disabled); a test with neither
// leg configured always reports WarningBufferNotConfigured.
func classifyWarningBuffer(t CovenantTest, status Status, headroom Value) WarningBufferStatus {
	if t.WarningBufferPercent == 0 && t.WarningBufferAmount == 0 {
		return WarningBufferNotConfigured
	}
	if status != StatusPass {
		return WarningBufferNotApplicable
	}
	if !headroom.Available {
		return WarningBufferHeadroomUnavailable
	}

	within := false
	if t.WarningBufferAmount > 0 && headroom.Amount <= t.WarningBufferAmount {
		within = true
	}
	if t.WarningBufferPercent > 0 {
		bufferFloor := t.WarningBufferPercent * absFloat(t.Threshold)
		if headroom.Amount <= bufferFloor {
			within = true
		}
	}
	if within {
		return WarningBufferWithinBuffer
	}
	return WarningBufferOutsideBuffer
}

// buildExplanation composes a deterministic, plain-language sentence for
// res's outcome. Formula-derived only, never AI-generated — mirroring
// debt.Flag.Message/variance.Issue.Message's identical inline
// fmt.Sprintf convention (see the package doc comment).
func buildExplanation(t CovenantTest, res TestResult) string {
	name := metricDisplayName(t)

	switch res.Status {
	case StatusPass:
		s := fmt.Sprintf("%s of %s %s the required %s of %s", name, formatNumber(t.Actual.Amount), operatorSatisfiedPhrase(t.Operator), operatorRequirementPhrase(t.Operator), formatNumber(t.Threshold))
		if res.Headroom.Available {
			s += fmt.Sprintf("; headroom of %s.", formatNumber(res.Headroom.Amount))
		} else {
			s += "."
		}
		if res.WarningBufferStatus == WarningBufferWithinBuffer {
			s += " Within the configured warning buffer (near breach)."
		}
		return s
	case StatusFail:
		s := fmt.Sprintf("%s of %s does not %s the required %s of %s (operator %s)", name, formatNumber(t.Actual.Amount), operatorSatisfiedPhrase(t.Operator), operatorRequirementPhrase(t.Operator), formatNumber(t.Threshold), t.Operator)
		if res.Headroom.Available {
			s += fmt.Sprintf("; breach of %s.", formatNumber(-res.Headroom.Amount))
		} else {
			s += "."
		}
		return s
	default:
		return res.Explanation
	}
}

// metricDisplayName returns t's metric label for use in an Explanation
// sentence: t.Label if supplied, else t.CustomMetricLabel for
// MetricCustom, else the Metric code itself.
func metricDisplayName(t CovenantTest) string {
	if t.Label != "" {
		return t.Label
	}
	if t.Metric == MetricCustom && t.CustomMetricLabel != "" {
		return t.CustomMetricLabel
	}
	return string(t.Metric)
}

// operatorSatisfiedPhrase returns the verb phrase describing a value that
// satisfies op (e.g. "meets or exceeds" for OperatorGTE).
func operatorSatisfiedPhrase(op Operator) string {
	switch op {
	case OperatorGTE:
		return "meets or exceeds"
	case OperatorGT:
		return "exceeds"
	case OperatorLTE:
		return "meets or is below"
	case OperatorLT:
		return "is below"
	case OperatorEQ:
		return "equals"
	default:
		return "compares against"
	}
}

// operatorRequirementPhrase returns the noun phrase describing what op
// requires (e.g. "minimum" for OperatorGTE/OperatorGT, "maximum" for
// OperatorLTE/OperatorLT).
func operatorRequirementPhrase(op Operator) string {
	switch op {
	case OperatorGTE, OperatorGT:
		return "minimum"
	case OperatorLTE, OperatorLT:
		return "maximum"
	case OperatorEQ:
		return "exact value"
	default:
		return "threshold"
	}
}

// formatNumber renders v with up to 4 decimal places, trimming trailing
// zeros, so an Explanation reads naturally for both a DSCR-style multiple
// (1.25) and a dollar figure (500000) without a fixed, potentially
// misleading decimal count. Explanation is a display string only —
// TestResult.Actual/Threshold/Headroom carry the exact float64 values, so
// no precision is lost from this package's actual output.
//
// A magnitude too small to show at 4 decimal places (e.g. -0.00003) is
// normalized to "0" rather than the trimmed-negative-sign artifact "-0"
// fmt.Sprintf("%.4f", -0.00003) would otherwise produce, which would read
// as a fabricated negative distinct from zero in a lender-facing sentence.
func formatNumber(v float64) string {
	s := fmt.Sprintf("%.4f", v)
	for len(s) > 0 && s[len(s)-1] == '0' {
		s = s[:len(s)-1]
	}
	if len(s) > 0 && s[len(s)-1] == '.' {
		s = s[:len(s)-1]
	}
	if s == "-0" {
		s = "0"
	}
	return s
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// isInvalidFloat reports whether v is NaN or +/-Inf — used to reject a
// genuinely invalid Actual.Amount or Threshold before either reaches
// evaluateOperator/computeHeadroom's arithmetic.
func isInvalidFloat(v float64) bool {
	return math.IsNaN(v) || math.IsInf(v, 0)
}
