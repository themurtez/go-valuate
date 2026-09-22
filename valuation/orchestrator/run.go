package orchestrator

import (
	"github.com/themurtez/go-valuate/settings"
	"github.com/themurtez/go-valuate/valuation"
	"github.com/themurtez/go-valuate/valuation/capitalization"
	"github.com/themurtez/go-valuate/valuation/dcf"
	"github.com/themurtez/go-valuate/valuation/ebitda"
	"github.com/themurtez/go-valuate/valuation/netassets"
	"github.com/themurtez/go-valuate/valuation/sde"
)

// methodSpec drives Execute's fixed per-method evaluation order and
// exclusion checks, decoupled from each method's concrete Input/Result
// type via the run/hasInput closures below — see Execute.
type methodSpec struct {
	code        valuation.Code
	settingsKey settings.Method
	hasInput    func(Request) bool
	run         func(Request) MethodOutcome
}

// Execute runs req against every individual valuation method, in a fixed
// order (SDE, EBITDA, Capitalization, DCF, NetAssets), and returns a
// complete Run. Execute never aborts because one method is excluded or
// unavailable — every method gets its own independent MethodOutcome; a
// failure or exclusion in one method has no effect on any other.
//
// For each method, in order:
//
//  1. If settings.Resolution explicitly disables the method (see
//     settingsEnabled), it is OutcomeExcluded with
//     ExclusionDisabledBySettings — checked first, and before even looking
//     at whether an Input was supplied, since a caller may reasonably
//     construct every Input unconditionally and let settings decide.
//  2. Else if no Input was supplied for the method, it is OutcomeExcluded
//     with ExclusionNoInput.
//  3. Else if req.Applicability and req.MinApplicabilityLevel are both
//     set, and the method's applicability.Result.Level ranks below
//     MinApplicabilityLevel, it is OutcomeExcluded with
//     ExclusionLowApplicability.
//  4. Else the method's own Calculate runs, and the MethodOutcome is
//     OutcomeSuccess (Result.Available == true) or OutcomeUnavailable
//     (Result.Available == false) accordingly — Execute reports the
//     method's own Result either way; it never re-derives or overrides
//     Available itself.
//
// Every MethodOutcome carries the method's applicability.Result (if
// req.Applicability was supplied) regardless of which of the above
// branches it took, so a report can always show applicability alongside
// outcome even for an excluded or unavailable method.
func Execute(req Request) Run {
	specs := []methodSpec{
		{
			code:        valuation.CodeSDEMultiple,
			settingsKey: settings.MethodSDE,
			hasInput:    func(r Request) bool { return r.SDE != nil },
			run: func(r Request) MethodOutcome {
				res := sde.Calculate(*r.SDE)
				return MethodOutcome{
					Method:    valuation.CodeSDEMultiple,
					Outcome:   outcomeFromAvailable(res.Available),
					Available: res.Available,
					SDE:       &res,
				}
			},
		},
		{
			code:        valuation.CodeEBITDAMultiple,
			settingsKey: settings.MethodEBITDA,
			hasInput:    func(r Request) bool { return r.EBITDA != nil },
			run: func(r Request) MethodOutcome {
				res := ebitda.Calculate(*r.EBITDA)
				return MethodOutcome{
					Method:    valuation.CodeEBITDAMultiple,
					Outcome:   outcomeFromAvailable(res.Available),
					Available: res.Available,
					EBITDA:    &res,
				}
			},
		},
		{
			code:        valuation.CodeCapitalizationOfEarnings,
			settingsKey: settings.MethodCapitalizationEarnings,
			hasInput:    func(r Request) bool { return r.Capitalization != nil },
			run: func(r Request) MethodOutcome {
				res := capitalization.Calculate(*r.Capitalization)
				return MethodOutcome{
					Method:         valuation.CodeCapitalizationOfEarnings,
					Outcome:        outcomeFromAvailable(res.Available),
					Available:      res.Available,
					Capitalization: &res,
				}
			},
		},
		{
			code:        valuation.CodeDCF,
			settingsKey: settings.MethodDCF,
			hasInput:    func(r Request) bool { return r.DCF != nil },
			run: func(r Request) MethodOutcome {
				res := dcf.Calculate(*r.DCF)
				return MethodOutcome{
					Method:    valuation.CodeDCF,
					Outcome:   outcomeFromAvailable(res.Available),
					Available: res.Available,
					DCF:       &res,
				}
			},
		},
		{
			code:        valuation.CodeAdjustedNetAssetValue,
			settingsKey: settings.MethodAdjustedNetAssetValue,
			hasInput:    func(r Request) bool { return r.NetAssets != nil },
			run: func(r Request) MethodOutcome {
				res := netassets.Calculate(*r.NetAssets)
				return MethodOutcome{
					Method:    valuation.CodeAdjustedNetAssetValue,
					Outcome:   outcomeFromAvailable(res.Available),
					Available: res.Available,
					NetAssets: &res,
				}
			},
		},
	}

	run := Run{Methods: make([]MethodOutcome, 0, len(specs))}

	for _, spec := range specs {
		outcome := evaluate(req, spec)
		outcome.Applicability = applicabilityFor(req.Applicability, spec.code)
		run.Methods = append(run.Methods, outcome)
		run.Warnings = append(run.Warnings, warningsFor(outcome)...)
	}

	return run
}

// evaluate applies the exclusion checks in Execute's documented order and
// either returns an OutcomeExcluded MethodOutcome directly or delegates to
// spec.run for an actually-attempted method.
func evaluate(req Request, spec methodSpec) MethodOutcome {
	if !settingsEnabled(req.Resolution, spec.settingsKey) {
		return MethodOutcome{
			Method:          spec.code,
			Outcome:         OutcomeExcluded,
			ExclusionReason: ExclusionDisabledBySettings,
			Detail:          "method disabled via resolved settings",
		}
	}
	if !spec.hasInput(req) {
		return MethodOutcome{
			Method:          spec.code,
			Outcome:         OutcomeExcluded,
			ExclusionReason: ExclusionNoInput,
			Detail:          "no input supplied for this method",
		}
	}
	if req.Applicability != nil && req.MinApplicabilityLevel != "" {
		if res, ok := req.Applicability.ForMethod(string(spec.code)); ok {
			if levelRank(res.Level) < levelRank(req.MinApplicabilityLevel) {
				return MethodOutcome{
					Method:          spec.code,
					Outcome:         OutcomeExcluded,
					ExclusionReason: ExclusionLowApplicability,
					Detail:          "applicability level " + string(res.Level) + " is below the configured minimum " + string(req.MinApplicabilityLevel),
				}
			}
		}
	}
	return spec.run(req)
}

// outcomeFromAvailable maps a method Result's Available field to an Outcome
// for an attempted (not excluded) method.
func outcomeFromAvailable(available bool) Outcome {
	if available {
		return OutcomeSuccess
	}
	return OutcomeUnavailable
}

// warningsFor flattens one MethodOutcome's underlying method Warnings (if
// it ran) into MethodWarning entries.
func warningsFor(m MethodOutcome) []MethodWarning {
	var msgs []string
	switch {
	case m.SDE != nil:
		msgs = issueMessages(m.SDE.Warnings)
	case m.EBITDA != nil:
		msgs = issueMessages(m.EBITDA.Warnings)
	case m.Capitalization != nil:
		msgs = issueMessages(m.Capitalization.Warnings)
	case m.DCF != nil:
		msgs = issueMessages(m.DCF.Warnings)
	case m.NetAssets != nil:
		msgs = issueMessages(m.NetAssets.Warnings)
	}
	out := make([]MethodWarning, 0, len(msgs))
	for _, msg := range msgs {
		out = append(out, MethodWarning{Method: m.Method, Message: msg})
	}
	return out
}

func issueMessages(issues []valuation.Issue) []string {
	out := make([]string, 0, len(issues))
	for _, iss := range issues {
		out = append(out, iss.Message)
	}
	return out
}
