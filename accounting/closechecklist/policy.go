package closechecklist

// Policy is every caller-configurable rule this package uses beyond what
// is already encoded per-task in the Template itself. Nothing that
// materially changes a Finding or Result.Readiness is a hidden code
// constant.
type Policy struct {
	// TimingWarningsAreFindingsOnly: when true (the default), timing
	// inconsistencies (section 24: TASK_COMPLETED_BEFORE_DEPENDENCY,
	// SIGNOFF_BEFORE_TASK_COMPLETION) are always factual warnings and
	// never affect Satisfied/readiness. When false, callers may instead
	// list specific codes in BlockingTimingFindingCodes to make them
	// effective blockers.
	TimingWarningsAreFindingsOnly bool `json:"timing_warnings_are_findings_only"`
	// BlockingTimingFindingCodes: FindingCode values (by string) that
	// should be treated as effective blockers rather than warnings, only
	// consulted when TimingWarningsAreFindingsOnly is false.
	BlockingTimingFindingCodes []string `json:"blocking_timing_finding_codes,omitempty"`

	// StaleSignOffRule, when non-nil, defines when a REOPENED period's
	// prior sign-off should be treated as stale (section 29's "only if
	// caller policy explicitly defines when review must be repeated").
	// Nil means this package never flags a stale sign-off.
	StaleSignOffRule *StaleSignOffRule `json:"stale_sign_off_rule,omitempty"`
}

// StaleSignOffRule declares that a sign-off recorded before a
// caller-supplied cutoff date should be flagged as stale once a period
// is REOPENED and the owning task was subsequently reopened/modified.
// Purely caller-declared; never inferred.
type StaleSignOffRule struct {
	// StaleIfSignedBeforeReopen: when true, any SignOff on a task whose
	// TaskState.StartedAt (interpreted as "reopened" activity) is after
	// the SignOff's SignedAt is flagged STALE_SIGNOFF-equivalent via
	// FindingSignOffBeforeTaskCompletion's sibling — see timing.go.
	StaleIfSignedBeforeReopen bool `json:"stale_if_signed_before_reopen"`
}

// DefaultPolicy returns a safe default: timing inconsistencies are
// always non-blocking findings, and no stale-sign-off rule (so REOPENED
// periods never get an unrequested stale-review finding — section 29).
func DefaultPolicy() Policy {
	return Policy{TimingWarningsAreFindingsOnly: true}
}

func (p Policy) timingIsBlocking(code FindingCode) bool {
	if p.TimingWarningsAreFindingsOnly {
		return false
	}
	for _, c := range p.BlockingTimingFindingCodes {
		if FindingCode(c) == code {
			return true
		}
	}
	return false
}
