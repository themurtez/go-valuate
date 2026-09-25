package advisory

import "time"

// ActionStatus is the caller-supplied lifecycle state of one [ActionItem]
// — task section 37. This package never persists or updates status; every
// ActionItem's Status is either left at ActionStatusOpen (the default for
// a freshly generated action) or copied through unchanged from a
// caller-supplied action / a [Result.Prior] lookup by stable identity (see
// [ActionIdentity]).
type ActionStatus string

const (
	ActionStatusOpen        ActionStatus = "OPEN"
	ActionStatusInProgress  ActionStatus = "IN_PROGRESS"
	ActionStatusResolved    ActionStatus = "RESOLVED"
	ActionStatusDeferred    ActionStatus = "DEFERRED"
	ActionStatusNotRequired ActionStatus = "NOT_APPLICABLE"
)

// ActionOrigin distinguishes an action this package generated from a rule
// template (action.go's actionTemplates) from one the caller supplied
// directly on [Input] — task section 38. A CALLER_SUPPLIED action's
// Title/Description are never altered.
type ActionOrigin string

const (
	ActionOriginGenerated      ActionOrigin = "GENERATED"
	ActionOriginCallerSupplied ActionOrigin = "CALLER_SUPPLIED"
)

// ActionIdentity is the explicit, deterministic deduplication/cross-period
// identity key for one [ActionItem] — task section 35/103's "ActionCode +
// EntityRef + Period, or a source-provided related ID; never fuzzy-match
// titles/descriptions" rule. Two ActionItems with an equal ActionIdentity
// are the same underlying issue and are merged into one ActionItem with
// combined SourceRefs (see dedupeActions) — never merged merely because
// their rendered text looks similar.
type ActionIdentity struct {
	ActionCode string `json:"action_code"`
	EntityRef  string `json:"entity_ref,omitempty"`
	Period     string `json:"period,omitempty"`
}

// ActionItem is one typed, template-generated (or caller-supplied) action
// — task section 31. Every GENERATED ActionItem traces back to a specific
// source finding/gate via SourceModule/SourceCode/SourceRefs; this package
// never invents freeform advice (task section 33's "do not generate
// arbitrary action text" rule) — see actionTemplates in action.go for the
// closed set of Title/Description templates actually used.
type ActionItem struct {
	ActionCode string   `json:"action_code"`
	Category   string   `json:"category"`
	Priority   Priority `json:"priority"`

	Title       string `json:"title"`
	Description string `json:"description"`

	SourceModule string      `json:"source_module,omitempty"`
	SourceCode   string      `json:"source_code,omitempty"`
	SourceRefs   []SourceRef `json:"source_refs,omitempty"`

	RelatedMetricCodes []string `json:"related_metric_codes,omitempty"`
	RelatedEntityRefs  []string `json:"related_entity_refs,omitempty"`

	// DueDate is populated only from a source module's own due date or a
	// caller-supplied due date — never invented, never time.Now() — task
	// section 39.
	DueDate  *time.Time `json:"due_date,omitempty"`
	OwnerRef string     `json:"owner_ref,omitempty"`

	Status ActionStatus `json:"status"`
	Origin ActionOrigin `json:"origin"`

	// Blocking mirrors the source finding's own blocking-ness verbatim
	// (e.g. accounting/closechecklist.Blocker.EffectiveBlocking,
	// accounting/closequality's SeverityBlocking findings) — never
	// downgraded or upgraded by this package (task section 43's "do not
	// silently downgrade source blocking conditions" rule, applied
	// identically to actions).
	Blocking bool `json:"blocking"`

	// EntityRef/Period echo ActionIdentity's own fields, duplicated here
	// (rather than only inside a nested Identity field) so a caller reading
	// one ActionItem in isolation never has to reconstruct them — kept
	// consistent with Identity by construction (see newGeneratedAction/
	// dedupeActions).
	EntityRef string `json:"entity_ref,omitempty"`
	Period    string `json:"period,omitempty"`
}

// identity returns a's deduplication/cross-period identity key.
func (a ActionItem) identity() ActionIdentity {
	return ActionIdentity{ActionCode: a.ActionCode, EntityRef: a.EntityRef, Period: a.Period}
}

// actionSortKey mirrors insightSortKey for [ActionItem] ordering — task
// section 99's "actions priority desc / category / code / entity" rule.
type actionSortKey struct {
	priorityRank int
	categoryRank int
	actionCode   string
	entityRef    string
}

func sortKeyForAction(a ActionItem, categoryOrder []SectionCode) actionSortKey {
	return actionSortKey{
		priorityRank: priorityRank(a.Priority),
		categoryRank: sectionRank(SectionCode(a.Category), categoryOrder),
		actionCode:   a.ActionCode,
		entityRef:    a.EntityRef,
	}
}

func lessActionSortKey(a, b actionSortKey) bool {
	if a.priorityRank != b.priorityRank {
		return a.priorityRank < b.priorityRank
	}
	if a.categoryRank != b.categoryRank {
		return a.categoryRank < b.categoryRank
	}
	if a.actionCode != b.actionCode {
		return a.actionCode < b.actionCode
	}
	return a.entityRef < b.entityRef
}

// actionTemplate is one closed, fixed rule-to-action mapping — task
// section 33's "deterministic rule-to-action templates... do not generate
// arbitrary action text" requirement. Every actionTemplates entry uses
// only review/resolve/validate/investigate/confirm/complete-framed
// language — task section 107's suggested-advice boundary, enforced
// permanently by TestNoPrescriptiveLanguage (safety_test.go).
type actionTemplate struct {
	Code        string
	Category    string
	Title       string
	Description string
	Blocking    bool
}

// newGeneratedAction builds an ActionItem from tmpl, stamping
// ActionOriginGenerated, ActionStatusOpen (the only sensible default for a
// freshly generated action — this package never infers RESOLVED/DEFERRED
// on its own, task section 37), and the identity/provenance fields a
// caller needs to trace and later deduplicate/compare this action — task
// section 36. priority is supplied by the caller (priority.go has already
// resolved it via the fixed precedence rule) rather than fixed per
// template, since the same template can arise at different priorities
// depending on the source finding's own severity.
func newGeneratedAction(tmpl actionTemplate, priority Priority, sourceModule, sourceCode string, refs []SourceRef, entityRef, period string) ActionItem {
	return ActionItem{
		ActionCode:   tmpl.Code,
		Category:     tmpl.Category,
		Priority:     priority,
		Title:        tmpl.Title,
		Description:  tmpl.Description,
		SourceModule: sourceModule,
		SourceCode:   sourceCode,
		SourceRefs:   refs,
		Status:       ActionStatusOpen,
		Origin:       ActionOriginGenerated,
		Blocking:     tmpl.Blocking,
		EntityRef:    entityRef,
		Period:       period,
	}
}

// dedupeActions merges every ActionItem in items sharing an identical
// ActionIdentity into one ActionItem — task section 34/35's "deduplicate
// into one primary action when identity can be proven... never merely
// because prose looks similar" rule. The first-encountered ActionItem for
// each identity (in items' own input order) is kept as the primary
// record; every later duplicate's SourceRefs (plus its own
// SourceModule/SourceCode, folded into a SourceRef so no provenance is
// lost — task section 36) are appended to the primary's SourceRefs, in
// input order. RelatedMetricCodes/RelatedEntityRefs are unioned
// (deduplicated, first-seen order preserved). Blocking is OR'd across
// every duplicate (a primary action is blocking if ANY contributing
// source says so — never silently downgraded, task section 43). Never
// mutates items; always returns a fresh slice, in first-occurrence
// identity order (stable, not sorted — priority.go/executive.go sort the
// final action list separately).
func dedupeActions(items []ActionItem) []ActionItem {
	if len(items) == 0 {
		return nil
	}
	order := make([]ActionIdentity, 0, len(items))
	byIdentity := make(map[ActionIdentity]ActionItem, len(items))
	for _, item := range items {
		id := item.identity()
		existing, ok := byIdentity[id]
		if !ok {
			// Copy defensively: the merged record's slices must never
			// alias the caller's original item's slices, since we may
			// append to them below.
			merged := item
			merged.SourceRefs = append([]SourceRef(nil), item.SourceRefs...)
			merged.RelatedMetricCodes = append([]string(nil), item.RelatedMetricCodes...)
			merged.RelatedEntityRefs = append([]string(nil), item.RelatedEntityRefs...)
			byIdentity[id] = merged
			order = append(order, id)
			continue
		}
		existing.SourceRefs = append(existing.SourceRefs, item.SourceRefs...)
		if item.SourceModule != "" {
			existing.SourceRefs = append(existing.SourceRefs, SourceRef{Module: item.SourceModule, Code: item.SourceCode})
		}
		existing.RelatedMetricCodes = unionStrings(existing.RelatedMetricCodes, item.RelatedMetricCodes)
		existing.RelatedEntityRefs = unionStrings(existing.RelatedEntityRefs, item.RelatedEntityRefs)
		existing.Blocking = existing.Blocking || item.Blocking
		if priorityRank(item.Priority) < priorityRank(existing.Priority) {
			existing.Priority = item.Priority
		}
		byIdentity[id] = existing
	}
	out := make([]ActionItem, 0, len(order))
	for _, id := range order {
		out = append(out, byIdentity[id])
	}
	return out
}

// unionStrings returns a ∪ b, deduplicated, preserving a's order then b's
// first-seen order. Never mutates a or b.
func unionStrings(a, b []string) []string {
	if len(a) == 0 && len(b) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(a)+len(b))
	out := make([]string, 0, len(a)+len(b))
	for _, s := range a {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	for _, s := range b {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
