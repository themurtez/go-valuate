package advisory

import "sort"

// DisappearanceReason distinguishes a caller-confirmed resolution from a
// fact simply no longer being generated — task section 40/102's "do not
// call disappearance 'resolved' by default" rule.
type DisappearanceReason string

const (
	// ReasonNoLongerGenerated means a prior-pack Insight/ActionItem's
	// identity did not appear in the current pack. This is the default —
	// see NoLongerGeneratedActions/ResolvedActions below.
	ReasonNoLongerGenerated DisappearanceReason = "NO_LONGER_GENERATED"
	// ReasonCallerConfirmedResolved means the corresponding current-pack
	// ActionItem (matched by identity in Input.CallerActions, since a
	// no-longer-generated GENERATED action has no current-pack entry to
	// carry a Status on) was explicitly supplied with ActionStatusResolved
	// — the one case task section 40 allows disappearance to be reported
	// as resolved.
	ReasonCallerConfirmedResolved DisappearanceReason = "CALLER_CONFIRMED_RESOLVED"
)

// ActionComparison reports one action's cross-period identity outcome —
// task section 40's NewActions/ResolvedActions/PersistentActions/
// ReopenedActions.
type ActionComparison struct {
	Identity ActionIdentity `json:"identity"`
	// Current is the current-pack ActionItem, when present (nil for a
	// no-longer-generated action).
	Current *ActionItem `json:"current,omitempty"`
	// Prior is the prior-pack ActionItem this identity matched, when
	// present (nil for a NewActions entry).
	Prior  *ActionItem         `json:"prior,omitempty"`
	Reason DisappearanceReason `json:"reason,omitempty"`
}

// InsightComparison mirrors ActionComparison for Insight identity (Code +
// EntityRef — task section 103).
type InsightComparison struct {
	Code      string   `json:"code"`
	EntityRef string   `json:"entity_ref,omitempty"`
	Current   *Insight `json:"current,omitempty"`
	Prior     *Insight `json:"prior,omitempty"`
}

// MetricChange is one metric's current-vs-prior-pack value comparison —
// task section 102's MetricChanges.
type MetricChange struct {
	MetricCode string `json:"metric_code"`
	Current    Value  `json:"current"`
	Prior      Value  `json:"prior"`
	Change     Change `json:"change"`
}

// SectionAvailabilityChange reports one section's availability moving
// between the prior and current pack — task section 102.
type SectionAvailabilityChange struct {
	Code                SectionCode        `json:"code"`
	PriorAvailability   AvailabilityStatus `json:"prior_availability"`
	CurrentAvailability AvailabilityStatus `json:"current_availability"`
}

// PriorComparison is the full current-vs-prior-pack comparison — task
// section 40/102. Populated only when Input.Prior was supplied.
type PriorComparison struct {
	NewInsights        []InsightComparison `json:"new_insights,omitempty"`
	ResolvedInsights   []InsightComparison `json:"resolved_insights,omitempty"`
	PersistentInsights []InsightComparison `json:"persistent_insights,omitempty"`

	NewActions               []ActionComparison `json:"new_actions,omitempty"`
	ResolvedActions          []ActionComparison `json:"resolved_actions,omitempty"`
	PersistentActions        []ActionComparison `json:"persistent_actions,omitempty"`
	ReopenedActions          []ActionComparison `json:"reopened_actions,omitempty"`
	NoLongerGeneratedActions []ActionComparison `json:"no_longer_generated_actions,omitempty"`

	MetricChanges              []MetricChange              `json:"metric_changes,omitempty"`
	SectionAvailabilityChanges []SectionAvailabilityChange `json:"section_availability_changes,omitempty"`
}

// comparePrior builds the full PriorComparison between prior and the
// just-built current result — task section 40/102. Identity is always
// the explicit stable key (ActionIdentity for actions; Code+EntityRef for
// insights — task section 103), never fuzzy-matched text. Default
// disappearance reason is ReasonNoLongerGenerated (task section 40's "do
// not infer resolution solely from disappearance" rule); an action is
// reported ReasonCallerConfirmedResolved only when a caller-supplied
// current-pack entry with the same identity carries
// ActionStatusResolved.
func comparePrior(prior Result, current Result, in Input) *PriorComparison {
	pc := &PriorComparison{}

	pc.NewActions, pc.ResolvedActions, pc.PersistentActions, pc.ReopenedActions, pc.NoLongerGeneratedActions =
		compareActionSets(collectAllActions(prior), collectAllActions(current), in.CallerActions)

	pc.NewInsights, pc.ResolvedInsights, pc.PersistentInsights = compareInsightSets(collectAllInsights(prior), collectAllInsights(current))

	pc.MetricChanges = compareMetricSets(collectAllMetrics(prior), collectAllMetrics(current))

	pc.SectionAvailabilityChanges = compareSectionAvailability(prior.Sections, current.Sections)

	return pc
}

func collectAllActions(r Result) []ActionItem {
	var out []ActionItem
	for _, s := range r.Sections {
		out = append(out, s.Actions...)
	}
	return out
}

func collectAllInsights(r Result) []Insight {
	var out []Insight
	for _, s := range r.Sections {
		out = append(out, s.Highlights...)
		out = append(out, s.Findings...)
	}
	return out
}

func collectAllMetrics(r Result) []Metric {
	var out []Metric
	for _, s := range r.Sections {
		out = append(out, s.Metrics...)
	}
	return out
}

// compareActionSets classifies every action identity present in either
// priorActions or currentActions — task section 40's four buckets plus
// NoLongerGeneratedActions. A GENERATED action's ActionOrigin is not part
// of its identity (an action can be caller-overridden across periods and
// still be "the same" issue), so identity match alone (ActionIdentity)
// drives classification.
func compareActionSets(priorActions, currentActions, callerActions []ActionItem) (newA, resolved, persistent, reopened, noLonger []ActionComparison) {
	priorByID := indexActionsByIdentity(priorActions)
	currentByID := indexActionsByIdentity(currentActions)
	callerByID := indexActionsByIdentity(callerActions)

	for id, cur := range currentByID {
		curCopy := cur
		prior, wasGenerated := priorByID[id]
		if !wasGenerated {
			newA = append(newA, ActionComparison{Identity: id, Current: &curCopy})
			continue
		}
		priorCopy := prior
		// Reopened: this identity was generated last time too, but a
		// caller-supplied record for it (from the PRIOR pack's own
		// caller actions, i.e. what that pack's ACTION_REGISTER actually
		// carried as Status) had been marked resolved/deferred/not-
		// required, and it is being generated again now — a proven
		// one-hop signal, never inferred from disappearance alone (task
		// section 40).
		if isTerminalStatus(prior.Status) {
			reopened = append(reopened, ActionComparison{Identity: id, Current: &curCopy, Prior: &priorCopy})
			continue
		}
		persistent = append(persistent, ActionComparison{Identity: id, Current: &curCopy, Prior: &priorCopy})
	}

	for id, prior := range priorByID {
		if _, stillGenerated := currentByID[id]; stillGenerated {
			continue
		}
		priorCopy := prior
		reason := ReasonNoLongerGenerated
		if caller, ok := callerByID[id]; ok && caller.Status == ActionStatusResolved {
			reason = ReasonCallerConfirmedResolved
		}
		if reason == ReasonCallerConfirmedResolved {
			resolved = append(resolved, ActionComparison{Identity: id, Prior: &priorCopy, Reason: reason})
		} else {
			noLonger = append(noLonger, ActionComparison{Identity: id, Prior: &priorCopy, Reason: reason})
		}
	}

	sortActionComparisons(newA)
	sortActionComparisons(resolved)
	sortActionComparisons(persistent)
	sortActionComparisons(reopened)
	sortActionComparisons(noLonger)

	return newA, resolved, persistent, reopened, noLonger
}

// isTerminalStatus reports whether s represents a status a caller would
// reasonably consider "done" for this action — the condition an action
// generated again after carrying one of these statuses in the prior pack
// is classified ReopenedActions rather than plain PersistentActions.
func isTerminalStatus(s ActionStatus) bool {
	switch s {
	case ActionStatusResolved, ActionStatusDeferred, ActionStatusNotRequired:
		return true
	default:
		return false
	}
}

func indexActionsByIdentity(actions []ActionItem) map[ActionIdentity]ActionItem {
	out := make(map[ActionIdentity]ActionItem, len(actions))
	for _, a := range actions {
		out[a.identity()] = a
	}
	return out
}

type insightIdentity struct {
	code      string
	entityRef string
}

func compareInsightSets(priorInsights, currentInsights []Insight) (newI, resolved, persistent []InsightComparison) {
	priorByID := make(map[insightIdentity]Insight, len(priorInsights))
	for _, in := range priorInsights {
		priorByID[insightIdentity{in.Code, in.EntityRef}] = in
	}
	currentByID := make(map[insightIdentity]Insight, len(currentInsights))
	for _, in := range currentInsights {
		currentByID[insightIdentity{in.Code, in.EntityRef}] = in
	}

	for id, cur := range currentByID {
		curCopy := cur
		if prior, ok := priorByID[id]; ok {
			priorCopy := prior
			persistent = append(persistent, InsightComparison{Code: id.code, EntityRef: id.entityRef, Current: &curCopy, Prior: &priorCopy})
		} else {
			newI = append(newI, InsightComparison{Code: id.code, EntityRef: id.entityRef, Current: &curCopy})
		}
	}
	for id, prior := range priorByID {
		if _, ok := currentByID[id]; ok {
			continue
		}
		priorCopy := prior
		resolved = append(resolved, InsightComparison{Code: id.code, EntityRef: id.entityRef, Prior: &priorCopy})
	}

	sortInsightComparisons(newI)
	sortInsightComparisons(resolved)
	sortInsightComparisons(persistent)

	return newI, resolved, persistent
}

func compareMetricSets(priorMetrics, currentMetrics []Metric) []MetricChange {
	priorByCode := make(map[string]Metric, len(priorMetrics))
	for _, m := range priorMetrics {
		if _, exists := priorByCode[m.Code]; !exists {
			priorByCode[m.Code] = m
		}
	}

	var out []MetricChange
	seen := make(map[string]bool)
	for _, m := range currentMetrics {
		if seen[m.Code] {
			continue
		}
		seen[m.Code] = true
		prior, ok := priorByCode[m.Code]
		if !ok || !prior.Value.Available || !m.Value.Available {
			continue
		}
		out = append(out, MetricChange{
			MetricCode: m.Code, Current: m.Value, Prior: prior.Value,
			Change: computeChange(m.Value, prior.Value, isPercentUnit(m.Unit)),
		})
	}

	sortMetricChanges(out)
	return out
}

func compareSectionAvailability(prior, current []Section) []SectionAvailabilityChange {
	priorByCode := make(map[SectionCode]AvailabilityStatus, len(prior))
	for _, s := range prior {
		priorByCode[s.Code] = s.Availability
	}

	var out []SectionAvailabilityChange
	for _, s := range current {
		p, ok := priorByCode[s.Code]
		if !ok || p == s.Availability {
			continue
		}
		out = append(out, SectionAvailabilityChange{Code: s.Code, PriorAvailability: p, CurrentAvailability: s.Availability})
	}

	sortSectionAvailabilityChanges(out)
	return out
}

// Deterministic ordering for every PriorComparison list — task section
// 99/103. Every comparator sorts by ActionIdentity/insight identity
// fields (never Go map order, since every list above was built by
// ranging a map).

func sortActionComparisons(in []ActionComparison) {
	sort.SliceStable(in, func(i, j int) bool {
		a, b := in[i].Identity, in[j].Identity
		if a.ActionCode != b.ActionCode {
			return a.ActionCode < b.ActionCode
		}
		if a.EntityRef != b.EntityRef {
			return a.EntityRef < b.EntityRef
		}
		return a.Period < b.Period
	})
}

func sortInsightComparisons(in []InsightComparison) {
	sort.SliceStable(in, func(i, j int) bool {
		if in[i].Code != in[j].Code {
			return in[i].Code < in[j].Code
		}
		return in[i].EntityRef < in[j].EntityRef
	})
}

func sortMetricChanges(in []MetricChange) {
	sort.SliceStable(in, func(i, j int) bool { return in[i].MetricCode < in[j].MetricCode })
}

func sortSectionAvailabilityChanges(in []SectionAvailabilityChange) {
	sort.SliceStable(in, func(i, j int) bool { return in[i].Code < in[j].Code })
}
