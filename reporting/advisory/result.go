package advisory

// Result is the output of [Build]: every fixed [Section] in category
// order, an [ExecutiveSummary], a [Snapshot], [Coverage], version
// metadata, and every prior-pack/current-vs-prior comparison the supplied
// [Input] supports.
type Result struct {
	SchemaVersion           string `json:"schema_version"`
	FormulaVersion          string `json:"formula_version"`
	AdvisoryContractVersion string `json:"advisory_contract_version"`

	Status BuildStatus `json:"status"`

	Company CompanyContext `json:"company"`

	Sections         []Section        `json:"sections"`
	ExecutiveSummary ExecutiveSummary `json:"executive_summary"`
	Snapshot         Snapshot         `json:"snapshot"`

	// Questions is the optional QuestionsForManagement section — task
	// section 41. Populated only when Policy.IncludeManagementQuestions is
	// true.
	Questions []ManagementQuestion `json:"questions,omitempty"`

	Coverage Coverage `json:"coverage"`

	SourceVersions SourceVersions `json:"source_versions"`

	// PriorComparison is populated only when Input.Prior was supplied —
	// task section 40/102.
	PriorComparison *PriorComparison `json:"prior_comparison,omitempty"`

	Warnings []Issue `json:"warnings,omitempty"`
	Errors   []Issue `json:"errors,omitempty"`
}

// sectionByCode returns the Section in sections with the given code, and
// true, or the zero Section and false if not present — the shared lookup
// helper cross-section builders (executive.go, snapshot.go, prior.go) use
// instead of repeating a linear scan inline.
func sectionByCode(sections []Section, code SectionCode) (Section, bool) {
	for _, s := range sections {
		if s.Code == code {
			return s, true
		}
	}
	return Section{}, false
}
