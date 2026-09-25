package advisory

// BuildStatus is the pack-data-completeness verdict [Build] returns — task
// section 62. This is data completeness only; it is never a business
// health verdict (task section 62/127's explicit "no health score"
// instruction, echoed from every prior prompt in this roadmap's own
// "no proprietary health score" rule).
type BuildStatus string

const (
	// BuildComplete means every section Policy/Input requested is
	// StatusAvailable (or a source-driven StatusNotApplicable, never
	// StatusUnavailable/StatusInvalid) — see Coverage.UnavailableSections.
	BuildComplete BuildStatus = "COMPLETE"
	// BuildPartial means at least one requested section is unavailable but
	// Build otherwise proceeded normally — the ordinary case for a caller
	// supplying a subset of Input.
	BuildPartial BuildStatus = "PARTIAL"
	// BuildInvalid means Input itself was invalid enough that Build could
	// not proceed at all (no usable section, no CompanyContext, or Policy
	// itself was structurally invalid) — see Result.Errors.
	BuildInvalid BuildStatus = "INVALID"
)

// Coverage summarizes factual pack completeness — task section 63/127.
// Never a health score: purely counts and code lists.
type Coverage struct {
	RequestedSections   []SectionCode `json:"requested_sections,omitempty"`
	AvailableSections   []SectionCode `json:"available_sections,omitempty"`
	UnavailableSections []SectionCode `json:"unavailable_sections,omitempty"`

	// SourceModulesSupplied is every module name (matching SourceVersions'
	// field naming) whose corresponding Input field was non-zero-value.
	SourceModulesSupplied []string `json:"source_modules_supplied,omitempty"`
	// SourceModulesUsed is the subset of SourceModulesSupplied that
	// actually contributed at least one Metric/Insight/ActionItem to
	// Result — a supplied-but-entirely-unavailable source (e.g.
	// ar.Result.Available == false) is supplied but not used.
	SourceModulesUsed []string `json:"source_modules_used,omitempty"`

	MetricsAvailable   int `json:"metrics_available"`
	MetricsUnavailable int `json:"metrics_unavailable"`

	InsightsGenerated  int `json:"insights_generated"`
	ActionsGenerated   int `json:"actions_generated"`
	QuestionsGenerated int `json:"questions_generated"`

	SourceConflictCount int `json:"source_conflict_count"`
	InvalidSourceCount  int `json:"invalid_source_count"`
}

// buildCoverage derives Coverage from the built sections, the resolved
// Input's supplied-module list, and issues — a single post-pass over
// already-built state (never a second scan of every source Result), per
// task section 123's "index facts once" performance discipline.
func buildCoverage(sections []Section, order []SectionCode, suppliedModules []string, usedModules map[string]bool, issues []Issue) Coverage {
	c := Coverage{RequestedSections: append([]SectionCode(nil), order...)}

	for _, s := range sections {
		switch s.Availability {
		case StatusAvailable, StatusNotApplicable:
			c.AvailableSections = append(c.AvailableSections, s.Code)
		default:
			c.UnavailableSections = append(c.UnavailableSections, s.Code)
		}
		for _, m := range s.Metrics {
			if m.Value.Available {
				c.MetricsAvailable++
			} else {
				c.MetricsUnavailable++
			}
		}
		c.InsightsGenerated += len(s.Highlights) + len(s.Findings)
		c.ActionsGenerated += len(s.Actions)
	}

	c.SourceModulesSupplied = append([]string(nil), suppliedModules...)
	for _, m := range suppliedModules {
		if usedModules[m] {
			c.SourceModulesUsed = append(c.SourceModulesUsed, m)
		}
	}

	for _, iss := range issues {
		switch iss.Code {
		case IssueSourceConflict:
			c.SourceConflictCount++
		case IssueInvalidPriorResult, IssueUnsupportedSourceVersion:
			c.InvalidSourceCount++
		}
	}

	return c
}

// resolveBuildStatus derives BuildStatus from Coverage and issues — task
// section 62/64. INVALID only when Build could not proceed at all (no
// section available and a blocking IssueSeverityError present); COMPLETE
// only when every requested section is available; PARTIAL otherwise.
func resolveBuildStatus(c Coverage, issues []Issue) BuildStatus {
	if len(c.AvailableSections) == 0 && HasErrors(issues) {
		return BuildInvalid
	}
	if len(c.UnavailableSections) == 0 {
		return BuildComplete
	}
	return BuildPartial
}
