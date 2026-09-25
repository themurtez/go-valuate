package advisory

// StatementCode is a stable, machine-readable identifier for one
// deterministic sentence template this package can generate — task
// section 79. Only codes this package actually generates are defined (see
// statements.go); this taxonomy deliberately does not attempt one code
// per possible source finding (task section 95's "avoid taxonomy
// explosion" instruction) — most Insights instead carry the source
// package's own already-stable code verbatim via SourceCode, and
// StatementCode exists only for this package's own composition-level
// synthesis statements (synthesis.go) and single-fact templated
// statements (statements.go).
type StatementCode string

// Insight is a deterministic fact/interpretation backed by already-computed
// module output — task section 7. Every Insight's Statement is assembled
// from typed fields via a fixed template (statements.go/synthesis.go),
// never generated text — task section 78's "no generative text" rule.
// Insight is used for both a Section's Highlights (a positive/neutral
// noteworthy fact) and its Findings (a condition worth attention); the
// same type serves both roles since the underlying shape (a fact backed by
// evidence, with severity/provenance) is identical — only which slice a
// builder appends to differs.
type Insight struct {
	// Code is this Insight's StatementCode when it was produced by this
	// package's own template (statements.go/synthesis.go), or the source
	// package's own finding code (echoed via SourceCode below) when this
	// Insight is a close-to-verbatim translation of one source fact with no
	// synthesis. Always non-empty.
	Code string `json:"code"`
	// Category is a short, fixed grouping label matching the Section this
	// Insight belongs to (e.g. "liquidity", "working_capital") — task
	// section 7, read by Policy.CategoryWeights when a caller has
	// configured one.
	Category string `json:"category"`
	// Severity is preserved from the source finding whenever this Insight
	// has exactly one source (task section 43's "source severity should
	// usually be preserved" rule); a synthesized multi-source Insight uses
	// an explicit synthesis-rule severity — never an automatic sum/max of
	// its inputs' severities (task section 85).
	Severity Severity `json:"severity"`
	// Priority is this package's own selection/ordering decision, derived
	// from Severity plus Policy via priority.go — see Priority's doc
	// comment for why this is a distinct field from Severity.
	Priority Priority `json:"priority"`

	Title     string `json:"title"`
	Statement string `json:"statement"`

	Current Value  `json:"current"`
	Prior   Value  `json:"prior"`
	Change  Change `json:"change"`

	Period    string `json:"period,omitempty"`
	EntityRef string `json:"entity_ref,omitempty"`

	SourceModule string      `json:"source_module,omitempty"`
	SourceCode   string      `json:"source_code,omitempty"`
	SourceRefs   []SourceRef `json:"source_refs,omitempty"`

	Evidence []Evidence `json:"evidence,omitempty"`
}

// insightSortKey is the shared tie-break tuple every Insight ordering
// operation (executive selection, section Highlights/Findings, synthesis
// output) uses instead of Go map order — task section 60/99's fixed
// "severity, category order, source module, source code, entity ref"
// determinism requirement.
type insightSortKey struct {
	priorityRank int
	categoryRank int
	sourceModule string
	sourceCode   string
	entityRef    string
}

func sortKeyForInsight(in Insight, categoryOrder []SectionCode) insightSortKey {
	return insightSortKey{
		priorityRank: priorityRank(in.Priority),
		categoryRank: sectionRank(SectionCode(in.Category), categoryOrder),
		sourceModule: in.SourceModule,
		sourceCode:   in.SourceCode,
		entityRef:    in.EntityRef,
	}
}

// lessInsightSortKey implements the fixed tie-break order: priority
// (severity-derived) ascending rank (critical first), then category
// declaration order, then source module, then source code, then entity
// ref — all lexical/ordinal, never map order.
func lessInsightSortKey(a, b insightSortKey) bool {
	if a.priorityRank != b.priorityRank {
		return a.priorityRank < b.priorityRank
	}
	if a.categoryRank != b.categoryRank {
		return a.categoryRank < b.categoryRank
	}
	if a.sourceModule != b.sourceModule {
		return a.sourceModule < b.sourceModule
	}
	if a.sourceCode != b.sourceCode {
		return a.sourceCode < b.sourceCode
	}
	return a.entityRef < b.entityRef
}
