package closechecklist

// Template is reusable, versioned, plain domain data describing one close
// checklist's sections and tasks. It carries no period-specific state —
// see [Instance] for pairing a Template with one period's caller-supplied
// state.
//
// Template serializes stably (deterministic field order via Go struct
// tags; Sections/Tasks preserve caller-supplied slice order — see
// sort.go's "presentation order follows template order" convention). A
// persisted close [Instance] must remain attributable to the exact
// TemplateID+Version used to build it — Calculate never silently
// substitutes a different template.
type Template struct {
	TemplateID string `json:"template_id"`
	Name       string `json:"name"`
	// Version is a caller-assigned content-revision identifier for this
	// Template (e.g. "2025.1"), distinct from [TemplateContractVersion]
	// (this package's own Go-shape version). See versions.go.
	Version string `json:"version"`

	Sections []SectionDefinition `json:"sections"`
	Tasks    []TaskDefinition    `json:"tasks"`
}

// SectionDefinition is one caller-defined, opaque checklist section.
// SectionCode is stable identity within a Template version. This package
// attaches no behavior to any particular SectionCode value — see the
// package doc comment's "no hard-coded section names" constraint.
type SectionDefinition struct {
	SectionCode string `json:"section_code"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// TaskDefinition is one caller-defined close task within a Template.
// TaskCode is stable identity within a Template version.
type TaskDefinition struct {
	TaskCode    string `json:"task_code"`
	SectionCode string `json:"section_code"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`

	// Required marks this task as part of the required-completion
	// denominator (see readiness.go) when it resolves APPLICABLE.
	Required bool `json:"required"`
	// AllowSkip permits a SKIPPED TaskState to still count as satisfied
	// for a Required task — see section 17.
	AllowSkip bool `json:"allow_skip"`

	Applicability ApplicabilityRule `json:"applicability"`
	Dependencies  []TaskDependency  `json:"dependencies,omitempty"`

	EvidencePolicy EvidencePolicy `json:"evidence_policy"`
	ReviewPolicy   ReviewPolicy   `json:"review_policy"`
	DueRule        DueRule        `json:"due_rule"`
	GateRules      []GateRule     `json:"gate_rules,omitempty"`

	// ExceptionPolicy controls whether/how a caller-approved Exception
	// may suppress this task's effective blocking — see exceptions.go.
	// The zero value is ExceptionNotAllowed.
	ExceptionPolicy ExceptionPolicy `json:"exception_policy,omitempty"`

	Tags []string `json:"tags,omitempty"`
}
