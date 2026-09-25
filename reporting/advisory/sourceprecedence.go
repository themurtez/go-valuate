package advisory

import "sort"

// candidateValue is one module's candidate figure for a metric code with
// more than one possible authoritative source — the input to
// resolveSourcedMetric.
type candidateValue struct {
	Value      Value
	SourceCode string
}

// Default source-order names for the well-known duplicate metrics this
// package's section builders resolve — task section 58's "default choices
// may exist for well-known duplicates, but must be documented" rule.
// Documented precedence (task section 19's worked example):
//
//	DSO: ar, then ratios
//	DPO: ap, then ratios
//	DIO: inventory, then ratios
//
// A caller overrides any of these via Policy.SourcePreferences using the
// same metric code.
const (
	SourceOrderDSO = metricCodeDSO
	SourceOrderDPO = metricCodeDPO
	SourceOrderDIO = metricCodeDIO
)

var defaultSourceOrder = map[string][]string{
	SourceOrderDSO: {"ar", "ratios"},
	SourceOrderDPO: {"ap", "ratios"},
	SourceOrderDIO: {"inventory", "ratios"},
}

// defaultOrder returns metricCode's fixed default OrderedSources, or nil
// for a metric with no documented default (resolveSourcedMetric then
// requires an explicit Policy.SourcePreferences entry, else any candidate
// disagreement is a conflict). Always returns a fresh slice.
func defaultOrder(metricCode string) []string {
	d, ok := defaultSourceOrder[metricCode]
	if !ok {
		return nil
	}
	return append([]string(nil), d...)
}

// resolveSourcedMetric picks the authoritative candidateValue for
// metricCode among candidates (keyed by module name), per task section 19/
// 58/59:
//
//  1. If policy.SourcePreferences has an entry for metricCode, its
//     OrderedSources wins: the first named module present in candidates is
//     used.
//  2. Else, unless policy.DisableDefaultSourceOrder is set, if
//     fallbackOrder (this package's documented default for metricCode, or
//     nil when none exists) is non-empty, it is used the same way.
//  3. Else, if every present candidate agrees within policy.ConflictTolerance,
//     the first (by deterministic module-name sort) is used — they agree,
//     so any deterministic pick is the same answer.
//  4. Else (no precedence AND candidates disagree beyond tolerance): no
//     Metric is returned; a SOURCE_CONFLICT Issue is returned instead —
//     task section 46/59's "do not pick arbitrarily unless source-
//     precedence policy explicitly resolves it" rule.
//
// Never averages. Returns (nil, nil) if candidates is empty (nothing to
// resolve — not a conflict, just absent).
func resolveSourcedMetric(metricCode, label string, unit Unit, period string, candidates map[string]candidateValue, fallbackOrder []string, policy Policy) (*Metric, *Issue) {
	if len(candidates) == 0 {
		return nil, nil
	}
	if len(candidates) == 1 {
		for module, c := range candidates {
			m := newMetric(metricCode, label, c.Value, unit, period, module, c.SourceCode)
			return &m, nil
		}
	}

	order := sourcePreferenceOrder(policy, metricCode)
	if len(order) == 0 && policy.DisableDefaultSourceOrder {
		fallbackOrder = nil
	}
	if len(order) == 0 {
		order = fallbackOrder
	}
	if len(order) > 0 {
		for _, module := range order {
			if c, ok := candidates[module]; ok {
				m := newMetric(metricCode, label, c.Value, unit, period, module, c.SourceCode)
				return &m, nil
			}
		}
	}

	// No precedence resolved a winner (or none configured): check
	// agreement within tolerance.
	modules := make([]string, 0, len(candidates))
	for module := range candidates {
		modules = append(modules, module)
	}
	sort.Strings(modules)

	agree := true
	var first Value
	haveFirst := false
	for _, module := range modules {
		c := candidates[module]
		if !c.Value.Available {
			continue
		}
		if !haveFirst {
			first = c.Value
			haveFirst = true
			continue
		}
		if absFloat(c.Value.Amount-first.Amount) > policy.ConflictTolerance {
			agree = false
		}
	}
	if agree && haveFirst {
		c := candidates[modules[0]]
		m := newMetric(metricCode, label, c.Value, unit, period, modules[0], c.SourceCode)
		return &m, nil
	}

	return nil, &Issue{
		Code: IssueSourceConflict, Severity: IssueSeverityWarning,
		Message:      "multiple authoritative sources for " + metricCode + " disagree beyond configured tolerance with no source precedence to resolve it",
		SourceModule: modules[0], SourceCode: metricCode, Period: period,
	}
}

// sourcePreferenceOrder returns policy's OrderedSources for metricCode, or
// nil if none configured.
func sourcePreferenceOrder(policy Policy, metricCode string) []string {
	for _, sp := range policy.SourcePreferences {
		if sp.MetricCode == metricCode {
			return sp.OrderedSources
		}
	}
	return nil
}

// sortedSourceRefs returns refs sorted by (Module, Code, Period) — the
// fixed order task section 99 requires for every SourceRefs list, since
// this package builds several such lists from a map (usedModules) whose
// own iteration order is nondeterministic.
func sortedSourceRefs(refs []SourceRef) []SourceRef {
	out := append([]SourceRef(nil), refs...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Module != out[j].Module {
			return out[i].Module < out[j].Module
		}
		if out[i].Code != out[j].Code {
			return out[i].Code < out[j].Code
		}
		return out[i].Period < out[j].Period
	})
	return out
}
