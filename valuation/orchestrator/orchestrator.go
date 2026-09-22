// Package orchestrator runs a selected set of individual valuation methods
// (valuation/sde, ebitda, capitalization, dcf, netassets) against
// caller-supplied, method-specific inputs, and reports one outcome per
// method — success, excluded, or unavailable — without ever aborting the
// whole run because a single method could not produce a result.
//
// This package computes nothing itself: every Result it returns comes
// directly from the corresponding method package's own Calculate. It only
// decides, for each method, whether to run it at all (based on
// settings.Resolution method-enable flags and, optionally, an
// applicability.Results recommendation), and collects every outcome into
// one Run so a caller never has to hand-wire five independent calls and
// their own excluded/failed bookkeeping. Every function here is pure: no
// I/O, no mutation of its inputs, no panics on missing/invalid input.
package orchestrator

import (
	"github.com/themurtez/go-valuate/settings"
	"github.com/themurtez/go-valuate/valuation"
	"github.com/themurtez/go-valuate/valuation/applicability"
	"github.com/themurtez/go-valuate/valuation/capitalization"
	"github.com/themurtez/go-valuate/valuation/dcf"
	"github.com/themurtez/go-valuate/valuation/ebitda"
	"github.com/themurtez/go-valuate/valuation/netassets"
	"github.com/themurtez/go-valuate/valuation/sde"
)

// Outcome classifies what happened to a single method during a Run.
type Outcome string

const (
	// OutcomeSuccess means the method ran and produced an Available result.
	OutcomeSuccess Outcome = "success"
	// OutcomeUnavailable means the method ran but its own validation
	// blocked it (Result.Available == false in the underlying method
	// package, e.g. dcf.Result.Available == false because no forecast
	// periods were supplied) — the method was attempted, not skipped.
	OutcomeUnavailable Outcome = "unavailable"
	// OutcomeExcluded means the method was never run at all: it was
	// disabled by settings.Resolution, had no Input supplied, or (when an
	// applicability.Results was supplied and MinLevel configured) scored
	// below the configured minimum applicability level.
	OutcomeExcluded Outcome = "excluded"
)

// ExclusionReason explains why a method was excluded without being run.
type ExclusionReason string

const (
	// ExclusionDisabledBySettings means settings.Resolution explicitly
	// disabled this method (method_enabled.<method> resolved to false).
	ExclusionDisabledBySettings ExclusionReason = "disabled_by_settings"
	// ExclusionNoInput means the caller did not supply an Input for this
	// method in Request (e.g. Request.DCF is nil) — the orchestrator never
	// invents a default input.
	ExclusionNoInput ExclusionReason = "no_input"
	// ExclusionLowApplicability means an applicability.Results was
	// supplied, Request.MinApplicabilityLevel was set above
	// applicability.LevelNotApplicable, and this method's scored Level fell
	// below it.
	ExclusionLowApplicability ExclusionReason = "low_applicability"
)

// MethodOutcome is one method's result from a single Run: exactly one of
// Result/Excluded is meaningful, selected by Outcome.
type MethodOutcome struct {
	// Method is the method's stable valuation.Code.
	Method valuation.Code `json:"method"`
	// Outcome classifies what happened — see the Outcome constants.
	Outcome Outcome `json:"outcome"`
	// Available echoes the underlying method Result's Available field when
	// Outcome is OutcomeSuccess or OutcomeUnavailable; always false when
	// Outcome is OutcomeExcluded (the method never ran).
	Available bool `json:"available"`
	// SDE/EBITDA/Capitalization/DCF/NetAssets carries the method's own
	// Result when this MethodOutcome.Method matches and Outcome is
	// OutcomeSuccess or OutcomeUnavailable (i.e. the method actually ran).
	// Exactly one of these five is non-nil in that case; all are nil when
	// Outcome is OutcomeExcluded. Kept as separate strongly-typed pointers
	// (rather than an `any`) so a caller never needs a type switch to get a
	// usable, fully-typed Result back out — see the package doc comment on
	// favoring type safety.
	SDE            *sde.Result            `json:"sde,omitempty"`
	EBITDA         *ebitda.Result         `json:"ebitda,omitempty"`
	Capitalization *capitalization.Result `json:"capitalization,omitempty"`
	DCF            *dcf.Result            `json:"dcf,omitempty"`
	NetAssets      *netassets.Result      `json:"net_assets,omitempty"`
	// ExclusionReason is populated only when Outcome is OutcomeExcluded.
	ExclusionReason ExclusionReason `json:"exclusion_reason,omitempty"`
	// Detail is a short human-readable elaboration on ExclusionReason (e.g.
	// naming the applicability Level that fell short).
	Detail string `json:"detail,omitempty"`
	// Applicability echoes this method's applicability.Result, when
	// Request.Applicability was supplied, regardless of Outcome — so a
	// caller/report can always show why a method was included or excluded
	// alongside its applicability score.
	Applicability *applicability.Result `json:"applicability,omitempty"`
}

// Request is everything the orchestrator needs to run a single valuation
// pass. Every method's Input field is optional (nil = not supplied, so
// that method is excluded with ExclusionNoInput unless settings disable it
// first) — this package never generates a method's input on the caller's
// behalf, matching every individual method package's own "caller-supplied
// input, no invented figures" rule.
type Request struct {
	// Resolution is the resolved hierarchical settings (see
	// settings.Resolve) used to determine which methods are enabled. A
	// zero-value Resolution (no method_enabled entries at all) means every
	// method is enabled by default — see Run's doc comment.
	Resolution settings.Resolution
	// Applicability, if non-nil, is consulted for MinApplicabilityLevel
	// filtering and is echoed on every MethodOutcome.Applicability
	// regardless of whether filtering is active.
	Applicability *applicability.Results
	// MinApplicabilityLevel, if non-empty, excludes any method whose
	// applicability.Result.Level is below this threshold (ordered
	// NOT_APPLICABLE < LOW < MEDIUM < HIGH) — see levelRank. Ignored if
	// Applicability is nil. Left empty (the zero value), no applicability
	// filtering happens: a method runs whenever it is enabled and has an
	// Input, regardless of its applicability score — applicability is
	// informational only unless the caller explicitly opts into filtering.
	MinApplicabilityLevel applicability.Level

	// SDE is the caller-constructed input for the SDE multiple method. nil
	// means "do not run this method."
	SDE *sde.Input
	// EBITDA is the caller-constructed input for the EBITDA multiple
	// method. nil means "do not run this method."
	EBITDA *ebitda.Input
	// Capitalization is the caller-constructed input for the
	// capitalization-of-earnings method. nil means "do not run this
	// method."
	Capitalization *capitalization.Input
	// DCF is the caller-constructed input for the DCF method. nil means "do
	// not run this method" — including the common case where no forecast
	// was ever assembled; the orchestrator does not distinguish "no
	// forecast" from "no DCF input at all" (both are ExclusionNoInput). A
	// caller that wants DCF to be attempted-and-reported-unavailable rather
	// than excluded should construct a dcf.Input with an empty
	// ForecastPeriods and let dcf.Calculate's own validation report
	// IssueNoForecastPeriods as OutcomeUnavailable instead.
	DCF *dcf.Input
	// NetAssets is the caller-constructed input for the adjusted net asset
	// value method. nil means "do not run this method."
	NetAssets *netassets.Input
}

// Run is the outcome of executing a single Request: every method's
// MethodOutcome, plus convenience views.
type Run struct {
	// Methods lists every method's MethodOutcome, in a fixed order (SDE,
	// EBITDA, Capitalization, DCF, NetAssets) regardless of which were
	// supplied/enabled, so a caller can always render a complete method
	// comparison table without checking for missing entries.
	Methods []MethodOutcome `json:"methods"`
	// Warnings collects every included method's own Warnings (mapped
	// through with the method identified), so a caller can surface every
	// validation warning from a single Run without iterating Methods
	// itself.
	Warnings []MethodWarning `json:"warnings,omitempty"`
}

// MethodWarning attributes one underlying method Warning/Issue to the
// method that produced it, flattened for a caller that wants a single
// list across every method without per-package Issue types.
type MethodWarning struct {
	Method  valuation.Code `json:"method"`
	Message string         `json:"message"`
}

// Successful returns every MethodOutcome with Outcome == OutcomeSuccess,
// in Methods' fixed order.
func (r Run) Successful() []MethodOutcome {
	return r.filter(OutcomeSuccess)
}

// Excluded returns every MethodOutcome with Outcome == OutcomeExcluded.
func (r Run) Excluded() []MethodOutcome {
	return r.filter(OutcomeExcluded)
}

// Unavailable returns every MethodOutcome with Outcome == OutcomeUnavailable.
func (r Run) Unavailable() []MethodOutcome {
	return r.filter(OutcomeUnavailable)
}

func (r Run) filter(o Outcome) []MethodOutcome {
	var out []MethodOutcome
	for _, m := range r.Methods {
		if m.Outcome == o {
			out = append(out, m)
		}
	}
	return out
}

// levelRank orders applicability.Level for MinApplicabilityLevel
// filtering: higher is more applicable. A Level not recognized here (which
// should not occur given applicability.Level's closed set of constants)
// ranks as if NOT_APPLICABLE, the most conservative treatment.
func levelRank(l applicability.Level) int {
	switch l {
	case applicability.LevelNotApplicable:
		return 0
	case applicability.LevelLow:
		return 1
	case applicability.LevelMedium:
		return 2
	case applicability.LevelHigh:
		return 3
	default:
		return 0
	}
}

// settingsEnabled reports whether method is enabled per req.Resolution: a
// method with no explicit method_enabled.<method> entry defaults to
// enabled (settings.Resolution's own convention is "unset means inherit
// from a lower scope," and this package treats a Resolution with no entry
// at all, at any scope, as "no opinion, default enabled" — a resolved
// Settings hierarchy that never once addresses whether DCF is enabled has
// not implicitly disabled DCF).
func settingsEnabled(res settings.Resolution, method settings.Method) bool {
	key := settings.MethodFieldKey(method)
	v, ok := res.Values[key]
	if !ok {
		return true
	}
	enabled, ok := v.(bool)
	if !ok {
		return true
	}
	return enabled
}

// applicabilityFor looks up code's Result within results, if supplied.
func applicabilityFor(results *applicability.Results, code valuation.Code) *applicability.Result {
	if results == nil {
		return nil
	}
	res, ok := results.ForMethod(string(code))
	if !ok {
		return nil
	}
	return &res
}
