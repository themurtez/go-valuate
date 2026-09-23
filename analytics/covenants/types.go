// Package covenants evaluates caller-supplied financial covenant rules
// (a minimum DSCR, a maximum leverage multiple, a minimum EBITDA or net
// worth, a maximum capex, or any custom metric a caller names) against
// already-calculated financial metrics, and reports pass/fail/unavailable,
// headroom, and warning-buffer status for each — the analysis a lender,
// borrower, or advisor performs to answer "does this business currently
// satisfy its covenant tests," not "what does the loan agreement say" or
// "what happens legally on a breach." Nothing in this package's output
// is, or should be read as, a legal interpretation of a credit agreement,
// a notice of default, or a substitute for counsel's own reading of the
// actual loan documents.
//
// This package is deliberately independent of financial.FinancialDataset,
// the same design analytics/debt and analytics/variance already
// established for a domain whose inputs frequently do not come from a
// normalized financial statement traversal at all: a covenant's Actual
// figure is typically a DSCR or leverage multiple already computed
// upstream (e.g. via analytics/debt, financial/metrics, or
// analytics/ratios), a balance-sheet figure pulled from a compliance
// certificate, or a wholly custom metric a lender's credit agreement
// defines in its own idiosyncratic way. This package never recomputes
// DSCR, leverage, EBITDA, net worth, or any other metric itself — a
// caller supplies each covenant's already-calculated Actual value, and
// this package's only job is comparing it against Threshold via Operator
// and classifying the result.
//
// Every function here is pure: no I/O, no mutation of caller-owned input,
// no package-global mutable state. Calculate can be called concurrently
// and repeatedly against identical input and always returns byte-for-byte
// identical JSON — see determinism_test.go.
package covenants

import "github.com/themurtez/go-valuate/financial"

// FormulaVersion identifies this package's fixed formula set: the
// operator-evaluation rule, the headroom formula (direction-aware,
// per Operator), the pass/fail/unavailable classification, and the
// warning-buffer near-breach classification. Bump this whenever any of
// that changes in a way that could make a historical Result not
// reproduce identically under new code — see the repository README's
// versioning-strategy section, which this constant follows exactly
// (financial.TaxonomyVersion, metrics.FormulaVersion, debt.FormulaVersion,
// variance.FormulaVersion, etc.).
const FormulaVersion = "1.0.0"

// Value represents a single figure that may or may not be available,
// distinguishing "computed/reported to be exactly 0" from "unknown
// because a required input was absent" — the same availability
// convention debt.Value/variance.VarianceValue/metrics.MetricValue all
// use, duplicated here as its own type per this repository's established
// convention (see cashflow.CashFlowValue's doc comment) rather than
// importing another analytics package into a package that otherwise has
// no dependency on it.
type Value struct {
	// Available is true if Amount is meaningful.
	Available bool `json:"available"`
	// Amount is the figure itself. Meaningful only when Available is
	// true; always 0 when Available is false.
	Amount float64 `json:"amount"`
}

// Unavailable is the canonical zero-information Value.
func Unavailable() Value { return Value{} }

// AvailableValue reports a Value for a known figure (which may
// legitimately be zero or negative).
func AvailableValue(v float64) Value { return Value{Available: true, Amount: v} }

// Operator is a comparison a covenant test applies between Actual and
// Threshold. This package defines its own taxonomy since no generic
// comparator type exists elsewhere in this repository (analytics/variance
// solved its structurally similar favorable/unfavorable problem with a
// domain-specific bool rather than a reusable operator enum).
type Operator string

const (
	// OperatorGTE requires Actual >= Threshold (e.g. a minimum DSCR or
	// minimum EBITDA covenant).
	OperatorGTE Operator = ">="
	// OperatorLTE requires Actual <= Threshold (e.g. a maximum
	// debt/EBITDA leverage or maximum capex covenant).
	OperatorLTE Operator = "<="
	// OperatorGT requires Actual > Threshold.
	OperatorGT Operator = ">"
	// OperatorLT requires Actual < Threshold.
	OperatorLT Operator = "<"
	// OperatorEQ requires Actual == Threshold. Rare in practice but
	// supported for a covenant defined as an exact figure.
	OperatorEQ Operator = "="
)

// isDirectionAbove reports whether op requires Actual to be at or above
// Threshold to pass (true for OperatorGTE/OperatorGT, false for
// OperatorLTE/OperatorLT), and whether op is recognized at all. OperatorEQ
// has no single "safe direction" (headroom is undefined either way — see
// computeHeadroom), so it reports ok == true with dir left false/unused by
// its caller.
func isDirectionAbove(op Operator) (above bool, ok bool) {
	switch op {
	case OperatorGTE, OperatorGT:
		return true, true
	case OperatorLTE, OperatorLT:
		return false, true
	case OperatorEQ:
		return false, true
	default:
		return false, false
	}
}

// evaluateOperator reports whether actual satisfies op against threshold.
// The caller must have already confirmed op is recognized via
// isDirectionAbove/a switch of its own; an unrecognized op reports false.
func evaluateOperator(op Operator, actual, threshold float64) bool {
	switch op {
	case OperatorGTE:
		return actual >= threshold
	case OperatorLTE:
		return actual <= threshold
	case OperatorGT:
		return actual > threshold
	case OperatorLT:
		return actual < threshold
	case OperatorEQ:
		return actual == threshold
	default:
		return false
	}
}

// Metric identifies what kind of figure a covenant test's Actual value
// represents, for display/grouping purposes only — this package never
// computes any metric itself (see the package doc comment) and applies
// the identical Operator/Threshold/headroom logic regardless of Metric.
type Metric string

const (
	MetricDSCR            Metric = "DSCR"
	MetricDebtToEBITDA    Metric = "DEBT_TO_EBITDA"
	MetricNetDebtToEBITDA Metric = "NET_DEBT_TO_EBITDA"
	MetricCurrentRatio    Metric = "CURRENT_RATIO"
	MetricQuickRatio      Metric = "QUICK_RATIO"
	MetricMinimumEBITDA   Metric = "MINIMUM_EBITDA"
	MetricMinimumNetWorth Metric = "MINIMUM_NET_WORTH"
	MetricMaximumCapex    Metric = "MAXIMUM_CAPEX"
	// MetricCustom is any covenant metric this package's initial set does
	// not name. CovenantTest.CustomMetricLabel supplies its display name.
	MetricCustom Metric = "CUSTOM"
)

// CureGrace is optional, purely descriptive cure/grace-period metadata
// echoed alongside a TestResult. This package records it verbatim and
// never uses it to alter Status, Headroom, or WarningBufferStatus — per
// the package doc comment, this package does not encode loan documents or
// legal interpretation, and a cure period's legal effect on an actual
// default is a determination for the credit agreement and counsel, not
// this package.
type CureGrace struct {
	// Description is free-text describing the cure/grace provision (e.g.
	// "10 business days to cure following notice"). Optional.
	Description string `json:"description,omitempty"`
	// GraceDays is the number of days after the test date before a
	// failed test becomes a reportable breach, per the credit agreement.
	// Zero means not specified. Purely descriptive.
	GraceDays int `json:"grace_days,omitempty"`
	// CureDays is the number of days the borrower has to cure a breach
	// after notice, per the credit agreement. Zero means not specified.
	// Purely descriptive.
	CureDays int `json:"cure_days,omitempty"`
}

// CovenantTest bundles one covenant rule's definition together with the
// already-calculated Actual figure to test it against — the portable
// input tuple this package requires for every computation (see the
// package doc comment), mirroring variance.LineObservation's identical
// "rule plus observation in one caller-assembled row" shape.
type CovenantTest struct {
	// CovenantID is a caller-assigned stable identifier for this covenant
	// (e.g. "MIN_DSCR", "MAX_LEVERAGE", or a credit-agreement section
	// reference). Required; Calculate records IssueMissingCovenantID and
	// leaves the resulting TestResult unavailable if empty.
	CovenantID string `json:"covenant_id"`
	// Label is a short human-readable name for this covenant (e.g.
	// "Minimum Fixed Charge Coverage Ratio"), for display only. Optional.
	Label string `json:"label,omitempty"`
	// Metric identifies what kind of figure Actual represents, for
	// display/grouping only (see Metric).
	Metric Metric `json:"metric"`
	// CustomMetricLabel names Metric when Metric == MetricCustom (e.g.
	// "Minimum Liquidity", "Maximum Owner Distributions"). Ignored for
	// every other Metric value.
	CustomMetricLabel string `json:"custom_metric_label,omitempty"`

	// Operator is the comparison Actual must satisfy against Threshold to
	// pass. Required; an empty or unrecognized Operator produces
	// IssueInvalidOperator and an unavailable TestResult.
	Operator Operator `json:"operator"`
	// Threshold is the covenant's required/limiting value.
	Threshold float64 `json:"threshold"`

	// Actual is this covenant's already-calculated figure for Period,
	// computed upstream by the caller (e.g. via analytics/debt for DSCR/
	// leverage, or analytics/ratios for current/quick ratio). Unavailable
	// means the figure could not be computed for this period (e.g. a
	// required input was missing upstream) — this package never treats an
	// unavailable Actual as zero.
	Actual Value `json:"actual"`

	// Period is the reporting period this test applies to. financial.Period
	// is a plain string with no guaranteed sort order; this package draws
	// no chronological inference from it and only uses it to label
	// TestResult and group Summary.ByPeriod.
	Period financial.Period `json:"period"`

	// WarningBufferPercent, when nonzero, is the fraction of Threshold's
	// magnitude (e.g. 0.10 for a 10% buffer) within which a passing test
	// is classified WarningBufferWithinBuffer (a "near breach") rather
	// than WarningBufferOutsideBuffer — mirroring
	// review.Policy.MaterialPercentOfRevenue/variance.Policy's identical
	// off-by-default design. Zero (the default) disables the percent-of-
	// threshold buffer test.
	WarningBufferPercent float64 `json:"warning_buffer_percent,omitempty"`
	// WarningBufferAmount, when nonzero, is an absolute headroom floor
	// (in Actual's units, e.g. 0.15 of DSCR-multiple headroom or $50,000
	// of net-worth headroom) below which a passing test is classified
	// WarningBufferWithinBuffer. Independent of WarningBufferPercent — a
	// caller may set either, both (a test triggers the buffer if it fails
	// either leg, the same OR-of-two-legs pattern review.IsMaterial and
	// variance.Policy already use), or neither. Zero (the default)
	// disables the absolute-headroom buffer test.
	WarningBufferAmount float64 `json:"warning_buffer_amount,omitempty"`

	// CureGrace is optional, purely descriptive cure/grace-period
	// metadata, echoed on TestResult but never used to alter Status,
	// Headroom, or WarningBufferStatus (see CureGrace's doc comment).
	CureGrace *CureGrace `json:"cure_grace,omitempty"`
}

// Input bundles everything Calculate needs.
type Input struct {
	// Tests is the full set of covenant tests to evaluate, in the order
	// Calculate reports them in Result.Tests. Required; Calculate returns
	// a Result with Available == false and an Errors entry if empty.
	Tests []CovenantTest `json:"tests"`
}

// IssueSeverity distinguishes an input problem Calculate could not
// proceed past for a given test (SeverityError) from one that is advisory
// only (SeverityWarning) — the same two-severity model every analytics
// sibling package uses.
type IssueSeverity string

const (
	SeverityError   IssueSeverity = "error"
	SeverityWarning IssueSeverity = "warning"
)

// IssueCode is a stable identifier for one kind of Calculate-time input
// problem. This package defines its own separate taxonomy, consistent
// with every other package in this repository, rather than reusing one of
// theirs.
type IssueCode string

const (
	// IssueNoTests means Input.Tests was empty; Calculate returns
	// Available == false.
	IssueNoTests IssueCode = "NO_TESTS"
	// IssueMissingCovenantID means a CovenantTest had an empty
	// CovenantID. That test's TestResult is still produced (so a caller
	// sees every input row reflected in output) but is left
	// StatusUnavailable.
	IssueMissingCovenantID IssueCode = "MISSING_COVENANT_ID"
	// IssueInvalidOperator means a CovenantTest's Operator was empty or
	// not one of the recognized Operator constants. That test's
	// TestResult is StatusUnavailable — this package never guesses a
	// direction for an unrecognized operator.
	IssueInvalidOperator IssueCode = "INVALID_OPERATOR"
	// IssueActualUnavailable means a CovenantTest's Actual.Available was
	// false. That test's TestResult is StatusUnavailable. Advisory only
	// (this is expected whenever an upstream figure could not be
	// computed for a period).
	IssueActualUnavailable IssueCode = "ACTUAL_UNAVAILABLE"
	// IssueInvalidActualOrThreshold means Actual was Available but
	// Actual.Amount was NaN or +/-Inf, or Threshold itself was NaN or
	// +/-Inf — a genuinely invalid figure, distinct from
	// IssueActualUnavailable's "no figure was supplied" (Threshold has no
	// Available wrapper, so this is the only signal a caller gets that a
	// non-finite Threshold was rejected). That test's TestResult is
	// StatusUnavailable rather than letting a non-finite value propagate
	// into Headroom/WarningBufferStatus arithmetic, mirroring
	// analytics/benchmarks.IssueInvalidCompanyValue's identical guard.
	IssueInvalidActualOrThreshold IssueCode = "INVALID_ACTUAL_OR_THRESHOLD"
	// IssueDuplicateCovenantID means two or more tests share the same
	// non-empty CovenantID and Period. Every such test is still evaluated
	// independently and included in Result.Tests; this is advisory only
	// since a caller may legitimately re-test the same covenant ID under
	// different assumptions, but is surfaced since it usually indicates
	// duplicated input.
	IssueDuplicateCovenantID IssueCode = "DUPLICATE_COVENANT_ID"
)

// Issue is a single Calculate-time input finding.
type Issue struct {
	Code     IssueCode     `json:"code"`
	Severity IssueSeverity `json:"severity"`
	Message  string        `json:"message"`
	// CovenantID identifies which CovenantTest this Issue relates to, by
	// echoing CovenantTest.CovenantID (or, when CovenantID itself is
	// empty/the problem, an index reference like "tests[2]"). Empty when
	// the Issue is not test-specific.
	CovenantID string `json:"covenant_id,omitempty"`
}

// HasErrors reports whether any Issue in issues has SeverityError.
//
// Intentionally duplicated from debt.HasErrors/variance.HasErrors/this
// repository's other sibling HasErrors functions rather than shared — see
// adjustments.HasErrors's doc comment for the full rationale (each
// package's Issue is a distinct Go type with no common interface worth
// introducing for one boolean function).
func HasErrors(issues []Issue) bool {
	for _, i := range issues {
		if i.Severity == SeverityError {
			return true
		}
	}
	return false
}

// Status is a covenant test's pass/fail/unavailable outcome.
type Status string

const (
	// StatusPass means Actual satisfied Operator against Threshold.
	StatusPass Status = "pass"
	// StatusFail means Actual did not satisfy Operator against
	// Threshold — a breach.
	StatusFail Status = "fail"
	// StatusUnavailable means this test could not be evaluated at all
	// (missing CovenantID, invalid Operator, or an unavailable Actual —
	// see the Issue recorded for the specific reason).
	StatusUnavailable Status = "unavailable"
)

// WarningBufferStatus classifies a passing test's proximity to its
// Threshold against CovenantTest.WarningBufferPercent/WarningBufferAmount
// — a "near breach" is a test that currently passes but would fail on a
// small adverse move, which a caller typically wants surfaced separately
// from an outright breach.
type WarningBufferStatus string

const (
	// WarningBufferNotConfigured means neither WarningBufferPercent nor
	// WarningBufferAmount was set on this CovenantTest, so no buffer
	// classification applies.
	WarningBufferNotConfigured WarningBufferStatus = "not_configured"
	// WarningBufferWithinBuffer means Status == StatusPass and Headroom
	// is available and positive but at or below at least one configured
	// buffer threshold — a near breach.
	WarningBufferWithinBuffer WarningBufferStatus = "within_buffer"
	// WarningBufferOutsideBuffer means Status == StatusPass and Headroom
	// cleared every configured buffer threshold — safely clear.
	WarningBufferOutsideBuffer WarningBufferStatus = "outside_buffer"
	// WarningBufferNotApplicable means a buffer was configured but Status
	// is not StatusPass — a buffer only classifies how close a passing
	// test is to breaching, and a test that has already failed or is
	// unavailable has no "how close to breach" question left to answer.
	WarningBufferNotApplicable WarningBufferStatus = "not_applicable"
	// WarningBufferHeadroomUnavailable means Status == StatusPass (so a
	// "how close to breach" question genuinely applies) but Headroom
	// itself could not be computed — in practice this means
	// CovenantTest.Operator is OperatorEQ, the one Operator computeHeadroom
	// has no distance-to-threshold concept for (see computeHeadroom's doc
	// comment). Kept distinct from WarningBufferNotApplicable so a caller
	// scanning for OperatorEQ-covenant buffer gaps does not have to also
	// filter out every already-failed test to find them.
	WarningBufferHeadroomUnavailable WarningBufferStatus = "headroom_unavailable"
)

// TestResult is one CovenantTest's full evaluation.
type TestResult struct {
	// CovenantID/Label/Metric/CustomMetricLabel/Operator/Threshold/Period/
	// CureGrace echo the source CovenantTest.
	CovenantID        string           `json:"covenant_id"`
	Label             string           `json:"label,omitempty"`
	Metric            Metric           `json:"metric"`
	CustomMetricLabel string           `json:"custom_metric_label,omitempty"`
	Operator          Operator         `json:"operator"`
	Threshold         float64          `json:"threshold"`
	Period            financial.Period `json:"period"`
	CureGrace         *CureGrace       `json:"cure_grace,omitempty"`

	// Actual echoes the source CovenantTest.Actual.
	Actual Value `json:"actual"`

	// Status is this test's pass/fail/unavailable outcome.
	Status Status `json:"status"`

	// Headroom is the signed distance between Actual and Threshold in the
	// direction of safety: positive means Actual has room before
	// breaching, negative means Actual has already breached (by that
	// amount), computed direction-aware per Operator — see
	// computeHeadroom for the exact per-operator formula. Available only
	// when Actual is available and Operator is recognized (i.e. whenever
	// Status is StatusPass or StatusFail; always unavailable when Status
	// is StatusUnavailable).
	Headroom Value `json:"headroom"`

	// WarningBufferStatus classifies a passing test's proximity to
	// breach against CovenantTest.WarningBufferPercent/
	// WarningBufferAmount (see WarningBufferStatus).
	WarningBufferStatus WarningBufferStatus `json:"warning_buffer_status"`

	// Explanation is a plain-language, deterministically generated
	// sentence describing this test's outcome (e.g. "DSCR of 1.15x is
	// below the required minimum of 1.25x (operator >=); breach of
	// 0.10x."). Formula-derived only, never AI-generated — mirroring
	// debt.Flag.Message/variance.Issue.Message's identical inline
	// fmt.Sprintf convention.
	Explanation string `json:"explanation"`
}

// PeriodSummary aggregates TestResult.Status/WarningBufferStatus for one
// Period, so a caller does not have to re-scan Result.Tests itself to
// answer "how many covenants breached in Q3."
type PeriodSummary struct {
	// Period is the period this aggregate applies to.
	Period financial.Period `json:"period"`
	// TestCount is the number of TestResult entries for Period.
	TestCount int `json:"test_count"`
	// Breaches is the number of TestResult entries for Period with
	// Status == StatusFail.
	Breaches int `json:"breaches"`
	// NearBreaches is the number of TestResult entries for Period with
	// WarningBufferStatus == WarningBufferWithinBuffer.
	NearBreaches int `json:"near_breaches"`
	// Unavailable is the number of TestResult entries for Period with
	// Status == StatusUnavailable.
	Unavailable int `json:"unavailable"`
}

// Summary aggregates every TestResult in Result.Tests.
type Summary struct {
	// TestCount is len(Result.Tests).
	TestCount int `json:"test_count"`
	// Breaches is the number of TestResult entries with
	// Status == StatusFail.
	Breaches int `json:"breaches"`
	// NearBreaches is the number of TestResult entries with
	// WarningBufferStatus == WarningBufferWithinBuffer. A test counted
	// here is never also counted in Breaches (WarningBufferStatus is only
	// evaluated for a passing test — see WarningBufferStatus's doc
	// comment), so Breaches and NearBreaches never double-count the same
	// TestResult.
	NearBreaches int `json:"near_breaches"`
	// Unavailable is the number of TestResult entries with
	// Status == StatusUnavailable.
	Unavailable int `json:"unavailable"`

	// BreachedCovenantIDs is CovenantID, in Result.Tests order, for every
	// TestResult with Status == StatusFail — for direct display without
	// requiring a caller to filter Result.Tests itself.
	BreachedCovenantIDs []string `json:"breached_covenant_ids,omitempty"`
	// NearBreachCovenantIDs mirrors BreachedCovenantIDs for
	// WarningBufferWithinBuffer.
	NearBreachCovenantIDs []string `json:"near_breach_covenant_ids,omitempty"`
	// UnavailableCovenantIDs mirrors BreachedCovenantIDs for
	// StatusUnavailable.
	UnavailableCovenantIDs []string `json:"unavailable_covenant_ids,omitempty"`

	// ByPeriod is one PeriodSummary per distinct CovenantTest.Period
	// present in Input.Tests, sorted lexically by Period — see
	// financial.Period's doc comment on why this package draws no
	// chronological inference beyond lexical order.
	ByPeriod []PeriodSummary `json:"by_period,omitempty"`
}

// Result is the output of Calculate: every test's evaluation plus a
// summary and formula version.
type Result struct {
	// FormulaVersion identifies which version of this package's fixed
	// formula set produced this Result.
	FormulaVersion string `json:"formula_version"`
	// Available is false only if Calculate could not proceed at all (see
	// IssueNoTests) — every other field is then zero-value.
	Available bool `json:"available"`

	// Tests is one TestResult per Input.Tests entry, in the same order.
	Tests []TestResult `json:"tests,omitempty"`

	// Summary aggregates Tests.
	Summary Summary `json:"summary"`

	// Warnings carries every Issue with SeverityWarning.
	Warnings []Issue `json:"warnings,omitempty"`
	// Errors carries every Issue with SeverityError.
	Errors []Issue `json:"errors,omitempty"`
}
