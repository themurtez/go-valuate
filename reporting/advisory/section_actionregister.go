package advisory

// buildActionRegisterSection composes ACTION_REGISTER: every other
// section's own already-built Actions, plus every Input.CallerActions
// entry (stamped ActionOriginCallerSupplied and given ActionStatusOpen
// only when the caller left Status empty — a caller-supplied action's own
// explicit Status, including a resolved one, is never overwritten), all
// deduplicated once more across section boundaries by ActionIdentity —
// task section 31/34/38. This is the single place a caller sees the
// complete, deduplicated action list; individual sections still carry
// their own (pre-cross-section-dedup) Actions slice for section-local
// display, per Section's own doc comment.
func buildActionRegisterSection(sections []Section, callerActions []ActionItem, policy Policy) Section {
	var all []ActionItem
	usedModules := map[string]bool{}
	for _, s := range sections {
		all = append(all, s.Actions...)
		for _, a := range s.Actions {
			if a.SourceModule != "" {
				usedModules[a.SourceModule] = true
			}
		}
	}

	for _, a := range callerActions {
		if a.ActionCode == "" {
			continue // reported via IssueInvalidActionOverride in validate.go; never silently included with a blank code.
		}
		stamped := a
		stamped.Origin = ActionOriginCallerSupplied
		if stamped.Status == "" {
			stamped.Status = ActionStatusOpen
		}
		all = append(all, stamped)
	}

	deduped := dedupeActions(all)
	sortActions(deduped, sectionOrder)

	if len(deduped) == 0 {
		return newUnavailableSection(SectionActionRegister, StatusNotApplicable)
	}

	sources := make([]SourceRef, 0, len(usedModules))
	for m := range usedModules {
		sources = append(sources, SourceRef{Module: m})
	}

	return Section{
		Code: SectionActionRegister, Availability: StatusAvailable,
		Actions: deduped,
		Sources: sortedSourceRefs(sources),
	}
}
