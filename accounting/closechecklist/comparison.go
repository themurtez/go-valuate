package closechecklist

// PriorCloseComparison is section 25's factual delta between this
// Result and an optionally-supplied prior period's Result. Comparison
// only — it never changes the current Result's own readiness/blockers.
type PriorCloseComparison struct {
	PriorPeriodID string `json:"prior_period_id,omitempty"`

	NewBlockerTaskCodes      []string `json:"new_blocker_task_codes,omitempty"`
	ResolvedBlockerTaskCodes []string `json:"resolved_blocker_task_codes,omitempty"`

	NewOverdueTaskCodes      []string `json:"new_overdue_task_codes,omitempty"`
	ResolvedOverdueTaskCodes []string `json:"resolved_overdue_task_codes,omitempty"`

	// CompletionTimingChanges maps a TaskCode to a signed day delta
	// between this period's and the prior period's DaysLateVsTarget-style
	// completion timing, only for tasks completed in both periods.
	CompletionTimingChanges map[string]int `json:"completion_timing_changes,omitempty"`

	AddedTaskCodes   []string `json:"added_task_codes,omitempty"`
	RemovedTaskCodes []string `json:"removed_task_codes,omitempty"`
}

// TemplateVersionComparison is section 26's factual delta between two
// template versions. Historical task state is never auto-migrated.
type TemplateVersionComparison struct {
	PriorVersion   string `json:"prior_version,omitempty"`
	CurrentVersion string `json:"current_version,omitempty"`

	AddedTaskCodes   []string `json:"added_task_codes,omitempty"`
	RemovedTaskCodes []string `json:"removed_task_codes,omitempty"`

	// ChangedTaskCodes lists tasks present in both versions whose
	// Required/AllowSkip/SectionCode differ — a straightforward,
	// low-effort definition-change signal (section 26: "changed
	// definitions if straightforward").
	ChangedTaskCodes []string `json:"changed_task_codes,omitempty"`
}

// buildPriorCloseComparison computes section 25's comparison from the
// current task evaluations and blockers against a supplied PriorClose.
// It never mutates prior.
func buildPriorCloseComparison(prior PriorClose, currentTaskCodes map[string]bool, currentBlockerTasks map[string]bool, currentOverdueTasks map[string]bool) *PriorCloseComparison {
	priorTaskCodes := make(map[string]bool)
	for _, t := range prior.Instance.Template.Tasks {
		priorTaskCodes[t.TaskCode] = true
	}
	priorBlockerTasks := make(map[string]bool)
	for _, b := range prior.Result.Blockers {
		priorBlockerTasks[b.TaskCode] = true
	}
	priorOverdueTasks := make(map[string]bool)
	for _, t := range prior.Result.Tasks {
		if t.DueStatus == DueStatusOverdue {
			priorOverdueTasks[t.TaskCode] = true
		}
	}

	c := &PriorCloseComparison{PriorPeriodID: prior.Instance.Period.PeriodID}
	c.NewBlockerTaskCodes = setDiffSorted(currentBlockerTasks, priorBlockerTasks)
	c.ResolvedBlockerTaskCodes = setDiffSorted(priorBlockerTasks, currentBlockerTasks)
	c.NewOverdueTaskCodes = setDiffSorted(currentOverdueTasks, priorOverdueTasks)
	c.ResolvedOverdueTaskCodes = setDiffSorted(priorOverdueTasks, currentOverdueTasks)
	c.AddedTaskCodes = setDiffSorted(currentTaskCodes, priorTaskCodes)
	c.RemovedTaskCodes = setDiffSorted(priorTaskCodes, currentTaskCodes)
	return c
}

// buildTemplateVersionComparison computes section 26's comparison
// between prior.Instance.Template and current.
func buildTemplateVersionComparison(prior Template, current Template) *TemplateVersionComparison {
	if prior.Version == current.Version {
		return nil
	}
	priorByCode := make(map[string]TaskDefinition, len(prior.Tasks))
	priorSet := make(map[string]bool, len(prior.Tasks))
	for _, t := range prior.Tasks {
		priorByCode[t.TaskCode] = t
		priorSet[t.TaskCode] = true
	}
	currentByCode := make(map[string]TaskDefinition, len(current.Tasks))
	currentSet := make(map[string]bool, len(current.Tasks))
	for _, t := range current.Tasks {
		currentByCode[t.TaskCode] = t
		currentSet[t.TaskCode] = true
	}

	c := &TemplateVersionComparison{PriorVersion: prior.Version, CurrentVersion: current.Version}
	c.AddedTaskCodes = setDiffSorted(currentSet, priorSet)
	c.RemovedTaskCodes = setDiffSorted(priorSet, currentSet)

	var changed []string
	for code, cur := range currentByCode {
		p, ok := priorByCode[code]
		if !ok {
			continue
		}
		if p.Required != cur.Required || p.AllowSkip != cur.AllowSkip || p.SectionCode != cur.SectionCode {
			changed = append(changed, code)
		}
	}
	c.ChangedTaskCodes = sortStrings(changed)
	return c
}

func setDiffSorted(a, b map[string]bool) []string {
	var out []string
	for k := range a {
		if !b[k] {
			out = append(out, k)
		}
	}
	return sortStrings(out)
}
