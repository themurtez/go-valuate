package closechecklist

// SchemaVersion identifies this package's public result/schema contract:
// the shape of Instance, Template, SectionDefinition, TaskDefinition,
// TaskState, EvidenceRef, SignOff, GateFact, Exception, Policy,
// TaskResult, SectionResult, Blocker, Finding, Issue, and Result. Bump
// whenever any of that shape changes in a way that could make a
// historical persisted Result not reproduce identically under new code —
// see the repository README's versioning-strategy section, which every
// sibling package's SchemaVersion already follows.
const SchemaVersion = "1.0.0"

// FormulaVersion identifies this package's fixed readiness semantics:
// applicability.go's resolution rules, dependencies.go's graph/cycle
// handling, duedates.go's due-state computation, evidence.go's coverage
// rules, signoff.go's review rules, gates.go's gate-satisfaction rules,
// exceptions.go's exception-application rules, and readiness.go's
// task/section/checklist status decision tables. Bump this whenever any
// of that changes in a way that could make a historical Result not
// reproduce identically under new code.
const FormulaVersion = "1.0.0"

// TemplateContractVersion identifies the shape of Template,
// SectionDefinition, and TaskDefinition specifically — separated from
// SchemaVersion because externally persisted templates and close
// instances may outlive a given code release: an application stores
// Template values (and Instances referencing a TemplateID+Template.Version
// pair) durably, independent of this package's own release cadence.
// TemplateContractVersion bumps only when the template *shape* itself
// changes (a new required field, a renamed rule type); it is orthogonal
// to Template.Version, which is a caller-assigned identifier for one
// concrete template's content revision (e.g. "2025.1" of "Service
// Business Monthly Close"). Bump TemplateContractVersion whenever a
// change to Template/SectionDefinition/TaskDefinition's Go shape could
// make a historical persisted Template or Instance fail to round-trip or
// evaluate identically under new code.
const TemplateContractVersion = "1.0.0"

// Versions is the triple of version strings echoed on Result, grouped
// into one struct for JSON convenience.
type Versions struct {
	SchemaVersion           string `json:"schema_version"`
	FormulaVersion          string `json:"formula_version"`
	TemplateContractVersion string `json:"template_contract_version"`
}

func currentVersions() Versions {
	return Versions{
		SchemaVersion:           SchemaVersion,
		FormulaVersion:          FormulaVersion,
		TemplateContractVersion: TemplateContractVersion,
	}
}
