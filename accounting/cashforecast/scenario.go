package cashforecast

// Scenario is one named what-if case: a Label plus zero or more
// caller-supplied event transformations applied to the base event set.
// This package never invents scenario assumptions — every transformation
// here is a pure function of caller-specified parameters (a delay in
// days, a scale factor, an explicit added/removed event) — see the task's
// section 26.
type Scenario struct {
	// Label uniquely identifies this scenario (e.g. "DOWNSIDE",
	// "UPSIDE", or a caller-chosen custom name). Required; must be unique
	// across Input.Scenarios and distinct from the reserved base-scenario
	// label (see BaseScenarioLabel).
	Label string `json:"label"`
	// Transforms is every transformation applied, in order, to a working
	// copy of the base event set. Transformations never mutate the base
	// events — see scenario_test.go's isolation tests.
	Transforms []EventTransform `json:"transforms,omitempty"`
}

// BaseScenarioLabel is the reserved label used internally for the base
// case in DeltaVsBase/Flag.Scenario reporting; a caller-supplied
// Scenario.Label equal to this is rejected (IssueInvalidScenario).
const BaseScenarioLabel = "BASE"

// EventTransform is one pure transformation applied to a working slice of
// CashFlowEvents to build a scenario, in the order supplied. Every
// transform returns a new slice (see the helper functions below); the
// input slice and its events are never mutated.
type EventTransform struct {
	Kind EventTransformKind `json:"kind"`

	// TargetIDs, when non-empty, restricts a delay/scale transform to
	// events whose ID is in this set; when empty, the transform applies
	// to every event matching Category (or every event, if Category is
	// also empty).
	TargetIDs []string `json:"target_ids,omitempty"`
	// Category, when non-empty, restricts a delay/scale/removal transform
	// to events of this category.
	Category CashCategory `json:"category,omitempty"`
	// Direction, when non-empty, restricts to events of this direction —
	// used mainly so a category-agnostic delay/scale can still be
	// narrowed to only inflows or only outflows.
	Direction CashDirection `json:"direction,omitempty"`

	// DelayDays is the number of days to shift matching events' Date
	// later (TransformDelay) or earlier (TransformAccelerate, where a
	// positive DelayDays moves the date earlier).
	DelayDays int `json:"delay_days,omitempty"`
	// ScaleFactor multiplies matching events' Amount (TransformScale);
	// 0.9 means 90% of the original amount.
	ScaleFactor float64 `json:"scale_factor,omitempty"`

	// EventToAdd is the event TransformAddEvent inserts. Its
	// ScenarioTags is set to [Scenario.Label] automatically (a caller
	// need not set it) unless already non-empty.
	EventToAdd CashFlowEvent `json:"event_to_add,omitempty"`
	// RemoveEventID is the CashFlowEvent.ID TransformRemoveEvent excludes.
	RemoveEventID string `json:"remove_event_id,omitempty"`

	// Priorities, for TransformDeferByPriority, is the set of Priority
	// values whose matching events are deferred by DelayDays — see
	// DeferByPriority.
	Priorities []Priority `json:"priorities,omitempty"`
}

// EventTransformKind identifies which pure transformation an
// EventTransform applies.
type EventTransformKind string

const (
	TransformDelayInflows       EventTransformKind = "DELAY_INFLOWS"
	TransformScaleInflows       EventTransformKind = "SCALE_INFLOWS"
	TransformDelayOutflows      EventTransformKind = "DELAY_OUTFLOWS"
	TransformAccelerateOutflows EventTransformKind = "ACCELERATE_OUTFLOWS"
	TransformScaleCategory      EventTransformKind = "SCALE_CATEGORY"
	TransformRemoveEvent        EventTransformKind = "REMOVE_EVENT"
	TransformAddEvent           EventTransformKind = "ADD_EVENT"
	TransformDeferByPriority    EventTransformKind = "DEFER_BY_PRIORITY"
)

func isRecognizedTransformKind(k EventTransformKind) bool {
	switch k {
	case TransformDelayInflows, TransformScaleInflows, TransformDelayOutflows,
		TransformAccelerateOutflows, TransformScaleCategory, TransformRemoveEvent,
		TransformAddEvent, TransformDeferByPriority:
		return true
	default:
		return false
	}
}

func matchesTarget(e CashFlowEvent, targetIDs []string, category CashCategory, direction CashDirection) bool {
	if category != "" && e.Category != category {
		return false
	}
	if direction != "" && e.Direction != direction {
		return false
	}
	if len(targetIDs) == 0 {
		return true
	}
	for _, id := range targetIDs {
		if e.ID == id {
			return true
		}
	}
	return false
}

// DelayEvents returns a copy of events with every matching event's Date
// shifted forward by days (never mutating events or any element in it).
func DelayEvents(events []CashFlowEvent, days int, targetIDs []string, category CashCategory, direction CashDirection) []CashFlowEvent {
	out := cloneEvents(events)
	for i := range out {
		if matchesTarget(out[i], targetIDs, category, direction) {
			out[i].Date = out[i].Date.AddDate(0, 0, days)
		}
	}
	return out
}

// AccelerateEvents returns a copy of events with every matching event's
// Date shifted earlier by days.
func AccelerateEvents(events []CashFlowEvent, days int, targetIDs []string, category CashCategory, direction CashDirection) []CashFlowEvent {
	return DelayEvents(events, -days, targetIDs, category, direction)
}

// ScaleEvents returns a copy of events with every matching event's Amount
// multiplied by factor.
func ScaleEvents(events []CashFlowEvent, factor float64, targetIDs []string, category CashCategory, direction CashDirection) []CashFlowEvent {
	out := cloneEvents(events)
	for i := range out {
		if matchesTarget(out[i], targetIDs, category, direction) {
			out[i].Amount *= factor
		}
	}
	return out
}

// RemoveEventByID returns a copy of events with the event whose ID equals
// id excluded (a no-op copy if no event matches).
func RemoveEventByID(events []CashFlowEvent, id string) []CashFlowEvent {
	out := make([]CashFlowEvent, 0, len(events))
	for _, e := range events {
		if e.ID == id {
			continue
		}
		out = append(out, cloneEvent(e))
	}
	return out
}

// AddEvent returns a copy of events with e appended, tagged for
// scenarioLabel (so it applies only under that scenario, per
// appliesToScenario) unless e.ScenarioTags is already non-empty.
func AddEvent(events []CashFlowEvent, e CashFlowEvent, scenarioLabel string) []CashFlowEvent {
	out := cloneEvents(events)
	added := cloneEvent(e)
	if len(added.ScenarioTags) == 0 && scenarioLabel != "" {
		added.ScenarioTags = []string{scenarioLabel}
	}
	return append(out, added)
}

// DeferByPriority returns a copy of events with every outflow whose
// Priority is in priorities shifted forward by days — the only
// Priority-aware transform, applied only when a caller explicitly
// requests this policy (see Priority's doc comment's "never
// automatically choose which bills not to pay" rule: this function is
// opt-in, never invoked implicitly by Calculate).
func DeferByPriority(events []CashFlowEvent, days int, priorities []Priority) []CashFlowEvent {
	set := make(map[Priority]bool, len(priorities))
	for _, p := range priorities {
		set[p] = true
	}
	out := cloneEvents(events)
	for i := range out {
		if out[i].Direction == DirectionOutflow && set[out[i].Priority] {
			out[i].Date = out[i].Date.AddDate(0, 0, days)
		}
	}
	return out
}

// applyTransform dispatches one EventTransform against events under
// scenarioLabel (used for TransformAddEvent's scenario tagging).
func applyTransform(events []CashFlowEvent, tr EventTransform, scenarioLabel string) []CashFlowEvent {
	switch tr.Kind {
	case TransformDelayInflows:
		return DelayEvents(events, tr.DelayDays, tr.TargetIDs, tr.Category, DirectionInflow)
	case TransformScaleInflows:
		return ScaleEvents(events, tr.ScaleFactor, tr.TargetIDs, tr.Category, DirectionInflow)
	case TransformDelayOutflows:
		return DelayEvents(events, tr.DelayDays, tr.TargetIDs, tr.Category, DirectionOutflow)
	case TransformAccelerateOutflows:
		return AccelerateEvents(events, tr.DelayDays, tr.TargetIDs, tr.Category, DirectionOutflow)
	case TransformScaleCategory:
		return ScaleEvents(events, tr.ScaleFactor, tr.TargetIDs, tr.Category, tr.Direction)
	case TransformRemoveEvent:
		return RemoveEventByID(events, tr.RemoveEventID)
	case TransformAddEvent:
		return AddEvent(events, tr.EventToAdd, scenarioLabel)
	case TransformDeferByPriority:
		return DeferByPriority(events, tr.DelayDays, tr.Priorities)
	default:
		return cloneEvents(events)
	}
}

// applyScenario applies every transform in scenario.Transforms, in order,
// to a fresh copy of baseEvents — see the task's section 28's "calculate
// each scenario from the same immutable base input, never chain
// scenarios unintentionally" instruction: this function is always called
// with the original baseEvents, never a previously transformed scenario's
// output.
//
// Before transforming, events already carrying a non-empty ScenarioTags
// (e.g. from a prior AddEvent call the caller pre-built into
// Input.Events, or reused across calls) are filtered to only those that
// apply to this scenario.Label — see appliesToScenario and
// CashFlowEvent.ScenarioTags's doc comment ("empty means the event
// applies to every scenario ... including base"). Without this filter, a
// caller-supplied event pre-tagged for one scenario would silently leak
// into every other scenario and the base case.
func applyScenario(baseEvents []CashFlowEvent, scenario Scenario) []CashFlowEvent {
	filtered := make([]CashFlowEvent, 0, len(baseEvents))
	for _, e := range baseEvents {
		if appliesToScenario(e, scenario.Label) {
			filtered = append(filtered, e)
		}
	}
	working := cloneEvents(filtered)
	for _, tr := range scenario.Transforms {
		working = applyTransform(working, tr, scenario.Label)
	}
	return working
}
