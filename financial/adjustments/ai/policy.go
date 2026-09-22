package ai

import "time"

// Mode is the caller-controlled switch deciding whether SuggestAdjustments
// consults a Suggester at all. Unlike
// financial/classification/ai.FallbackMode (which has several
// confidence-driven trigger rules), this package has exactly one on/off
// switch: candidate SELECTION is already the caller's deterministic,
// transparent decision (see CandidatePolicy/SelectCandidates) — there is no
// analogous "try AI only below some confidence" concept for a capability
// that never runs unless explicitly asked to.
type Mode string

const (
	// ModeDisabled never calls the Suggester. This is the zero value and the
	// Policy default (see DefaultPolicy) — AI is opt-in, never on by
	// default, per section 1 of the task brief.
	ModeDisabled Mode = "DISABLED"
	// ModeEnabled calls the Suggester for the caller-supplied candidate set.
	ModeEnabled Mode = "ENABLED"
)

// DefaultTimeout bounds a single Suggest call when the caller's ctx does not
// already carry a tighter deadline.
const DefaultTimeout = 30 * time.Second

// Policy bundles every caller-controlled AI-adjustment-suggestion decision:
// whether to call AI at all (Mode) and cost/latency controls (Timeout). Plain
// data, no I/O, mirroring financial/classification/ai.Policy's role.
type Policy struct {
	// Mode selects whether SuggestAdjustments consults a Suggester. Zero
	// value is ModeDisabled.
	Mode Mode
	// Timeout bounds a single Suggest call when the caller's ctx does not
	// already carry a tighter deadline. Zero means DefaultTimeout.
	Timeout time.Duration
}

// DefaultPolicy returns the zero-risk default: AI adjustment suggestions
// disabled entirely.
func DefaultPolicy() Policy {
	return Policy{Mode: ModeDisabled}
}

func (p Policy) timeout() time.Duration {
	if p.Timeout == 0 {
		return DefaultTimeout
	}
	return p.Timeout
}
