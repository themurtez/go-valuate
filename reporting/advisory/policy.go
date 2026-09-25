package advisory

// Priority is a small, ordinal urgency scale for [Insight]s and
// [ActionItem]s — task section 13's "prefer ordinal priority rules over
// opaque composite scores" and "no proprietary CFO Health Score"
// instruction. Every Priority a caller sees traces back to an explicit
// [PriorityRule] this package's own priority.go applied, never a hidden
// weighted sum.
type Priority string

const (
	PriorityCritical      Priority = "CRITICAL"
	PriorityHigh          Priority = "HIGH"
	PriorityMedium        Priority = "MEDIUM"
	PriorityLow           Priority = "LOW"
	PriorityInformational Priority = "INFORMATIONAL"
)

// priorityRank orders Priority for sorting: critical first, informational
// last. Unrecognized values sort after every known Priority.
func priorityRank(p Priority) int {
	switch p {
	case PriorityCritical:
		return 0
	case PriorityHigh:
		return 1
	case PriorityMedium:
		return 2
	case PriorityLow:
		return 3
	case PriorityInformational:
		return 4
	default:
		return 5
	}
}

// Severity is this package's own small, factual severity model for
// [Insight]s — task section 43. Distinct from Priority: Severity describes
// the underlying condition (usually preserved verbatim from the source
// finding's own severity), while Priority is this pack's selection/
// ordering decision derived from Severity plus [Policy] — task section
// 43's "source severity should usually be preserved; do not silently
// downgrade source blocking conditions" instruction is what keeps these
// two axes from collapsing into one.
type Severity string

const (
	SeverityBlocking Severity = "BLOCKING"
	SeverityHigh     Severity = "HIGH"
	SeverityMedium   Severity = "MEDIUM"
	SeverityLow      Severity = "LOW"
	SeverityInfo     Severity = "INFO"
)

// PriorityRuleKind identifies which fixed precedence tier (task section 14)
// a matched [PriorityRule] falls into. Declaration order below is
// precedence order: a lower ordinal always outranks a higher one,
// regardless of Policy.PriorityRules slice order — task section 14's fixed
// "1. caller override, 2. blocking condition, ... 7. informational"
// sequence is not caller-reorderable, only individual categories are
// enable/disable-able via Policy (task section 14's "caller policy should
// enable/disable categories" — see Policy.DisabledPriorityKinds).
type PriorityRuleKind string

const (
	// PriorityRuleCallerOverride means a caller-supplied override
	// (Policy.PriorityRules entry with an explicit Priority, or an
	// ActionItem/Insight the caller marked CALLER_SUPPLIED with its own
	// Priority) applied — always wins over every computed rule.
	PriorityRuleCallerOverride PriorityRuleKind = "CALLER_OVERRIDE"
	// PriorityRuleBlockingCondition means a source fact was itself blocking
	// (e.g. closequality.StatusNotReady, a covenants.StatusFail test, an
	// unreconciled-and-blocking reconciliation finding).
	PriorityRuleBlockingCondition PriorityRuleKind = "BLOCKING_CONDITION"
	// PriorityRuleLiquidityThreshold means a liquidity figure breached a
	// caller-supplied threshold in Policy.Materiality/Policy.Synthesis
	// (e.g. minimum 13-week cash below Policy's configured floor).
	PriorityRuleLiquidityThreshold PriorityRuleKind = "LIQUIDITY_THRESHOLD"
	// PriorityRuleMaterialDeterioration means a financial metric worsened
	// beyond Policy.Materiality's configured bar.
	PriorityRuleMaterialDeterioration PriorityRuleKind = "MATERIAL_DETERIORATION"
	// PriorityRuleSignificantChange means a working-capital/profitability
	// (or other non-headline) metric changed materially, a notch below
	// PriorityRuleMaterialDeterioration in the fixed precedence.
	PriorityRuleSignificantChange PriorityRuleKind = "SIGNIFICANT_CHANGE"
	// PriorityRuleWarning means the source fact carried Severity ==
	// SeverityMedium/SeverityLow with no material-change/blocking
	// qualification.
	PriorityRuleWarning PriorityRuleKind = "WARNING"
	// PriorityRuleInformational is the fallback tier for a source fact with
	// no other qualification.
	PriorityRuleInformational PriorityRuleKind = "INFORMATIONAL"
)

// priorityRuleKindOrder is PriorityRuleKind's fixed precedence order — task
// section 14. Never caller-reorderable; see PriorityRuleKind's doc
// comment.
var priorityRuleKindOrder = []PriorityRuleKind{
	PriorityRuleCallerOverride,
	PriorityRuleBlockingCondition,
	PriorityRuleLiquidityThreshold,
	PriorityRuleMaterialDeterioration,
	PriorityRuleSignificantChange,
	PriorityRuleWarning,
	PriorityRuleInformational,
}

// priorityRuleKindRank returns k's position in the fixed precedence order,
// len(priorityRuleKindOrder) for an unrecognized kind.
func priorityRuleKindRank(k PriorityRuleKind) int {
	for i, o := range priorityRuleKindOrder {
		if o == k {
			return i
		}
	}
	return len(priorityRuleKindOrder)
}

// PriorityRule is one caller-supplied override: when Match's fields (every
// non-empty field must match) identify a specific source fact, ForcePriority
// applies instead of this package's own computed priority — task section
// 12/14's "1. Caller override" tier. Matching is exact-field equality
// only, never fuzzy/prose matching — task section 35's "do not fuzzy-match
// titles/descriptions" rule extended to rule matching.
type PriorityRule struct {
	// MatchSourceModule/MatchSourceCode/MatchEntityRef identify which
	// fact(s) this rule applies to. An empty field matches any value for
	// that field (e.g. MatchSourceModule alone matches every fact from
	// that module, regardless of code/entity).
	MatchSourceModule string `json:"match_source_module,omitempty"`
	MatchSourceCode   string `json:"match_source_code,omitempty"`
	MatchEntityRef    string `json:"match_entity_ref,omitempty"`

	ForcePriority Priority `json:"force_priority"`
}

// matches reports whether r applies to a fact with the given
// module/code/entityRef — see PriorityRule's doc comment for the
// empty-field-matches-any rule.
func (r PriorityRule) matches(module, code, entityRef string) bool {
	if r.MatchSourceModule != "" && r.MatchSourceModule != module {
		return false
	}
	if r.MatchSourceCode != "" && r.MatchSourceCode != code {
		return false
	}
	if r.MatchEntityRef != "" && r.MatchEntityRef != entityRef {
		return false
	}
	return true
}

// MaterialityPolicy configures which current/prior metric changes qualify
// as "material" for composition-level selection (executive-summary
// inclusion, PriorityRuleMaterialDeterioration/
// PriorityRuleSignificantChange classification) — task section 44/104.
// This never overrides a source-domain package's own materiality decision
// (e.g. accounting/journaldiagnostics's own finding-trigger thresholds);
// it governs only how THIS package selects/prioritizes among facts it was
// already given — task section 44's "never erase source facts" rule.
type MaterialityPolicy struct {
	// AbsoluteThreshold: a currency-unit Metric's |AbsoluteChange| at or
	// above this is material. Zero means this test is disabled (never
	// silently interpreted as "any change is material" — see
	// resolvePolicy).
	AbsoluteThreshold float64 `json:"absolute_threshold,omitempty"`
	// PercentThreshold: a Metric's |PercentChange| at or above this
	// (decimal, e.g. 0.10 = 10%) is material.
	PercentThreshold float64 `json:"percent_threshold,omitempty"`
	// PercentagePointThreshold: a percent-unit Metric's
	// |PercentagePointChange| at or above this (decimal points, e.g. 0.05 =
	// 5 percentage points) is material.
	PercentagePointThreshold float64 `json:"percentage_point_threshold,omitempty"`
}

// isMaterial reports whether c qualifies as material under p — true if ANY
// configured (nonzero) threshold is met; false if every configured
// threshold is unmet OR no threshold is configured at all AND no change
// figure is available (an unavailable Change is never treated as
// material). Mirrors review.IsMaterial's identical
// "any-threshold-met-is-material" rule, this package's own copy per every
// sibling package's own-taxonomy convention.
func (p MaterialityPolicy) isMaterial(c Change) bool {
	if p.AbsoluteThreshold > 0 && c.AbsoluteChange.Available && absFloat(c.AbsoluteChange.Amount) >= p.AbsoluteThreshold {
		return true
	}
	if p.PercentThreshold > 0 && c.PercentChange.Available && absFloat(c.PercentChange.Amount) >= p.PercentThreshold {
		return true
	}
	if p.PercentagePointThreshold > 0 && c.PercentagePointChange.Available && absFloat(c.PercentagePointChange.Amount) >= p.PercentagePointThreshold {
		return true
	}
	return false
}

// SynthesisThresholds configures the small, fixed set of cross-module
// synthesis rules in synthesis.go — task section 84's explicit "caller
// supplies thresholds, no hard-coded bad thresholds" instruction. Zero
// value disables every threshold-gated synthesis rule (never silently
// treated as "always trigger").
type SynthesisThresholds struct {
	// MinimumCashThreshold gates the liquidity-pressure synthesis rule
	// (synthesis.go) — a 13-week minimum cash at or below this, combined
	// with worsening AR overdue, produces a synthesized liquidity insight.
	MinimumCashThreshold float64 `json:"minimum_cash_threshold,omitempty"`
	// AROverdueIncreasePoints gates the same rule's AR-side condition
	// (percentage-point increase in AR-over-90-days share).
	AROverdueIncreasePoints float64 `json:"ar_overdue_increase_points,omitempty"`
	// MarginDeclinePoints gates the margin-pressure synthesis rule
	// (percentage-point decline in contribution margin).
	MarginDeclinePoints float64 `json:"margin_decline_points,omitempty"`
	// LaborCostIncreasePoints gates the same rule's labor-side condition
	// (percentage-point increase in labor cost % of revenue).
	LaborCostIncreasePoints float64 `json:"labor_cost_increase_points,omitempty"`
}

// SourcePreference declares which sibling module is authoritative for one
// metric code when more than one supplied module could answer it — task
// section 19/58. OrderedSources is a priority list of Module names (see
// SourceVersions' field naming for the canonical module-name strings);
// the first entry present in Input wins for MetricCode. When MetricCode
// has no matching SourcePreference entry and more than one candidate
// module supplied a value beyond Policy's ConflictTolerance, the composed
// Metric is reported via a SOURCE_CONFLICT Issue instead of an arbitrary
// pick — task section 19/59's "do not average conflicting versions"
// instruction.
type SourcePreference struct {
	MetricCode     string   `json:"metric_code"`
	OrderedSources []string `json:"ordered_sources"`
}

// Policy is the caller-supplied selection/priority/materiality
// configuration driving [Build] — task section 12. Every threshold's zero
// value means "this specific check is disabled," never a silently
// substituted nonzero default — see resolvePolicy's doc comment for why
// this package's zero-value convention differs deliberately from
// portfolio/diagnostics.Policy's "zero falls back to DefaultPolicy"
// convention: an advisory pack's selection/materiality bars are
// inherently business/caller-specific (task section 12's "caller-supplied
// thresholds/targets," never a repo-wide accounting default), so Build
// applies exactly what Policy says, substituting only the small set of
// structural defaults documented per field below (max counts, ordering).
type Policy struct {
	// MaxExecutiveHighlights/MaxSectionHighlights/MaxActions cap
	// ExecutiveSummary.KeyHighlights/KeyRisks, each Section's Highlights,
	// and ExecutiveSummary.KeyActions respectively. Zero falls back to this
	// package's fixed default of 5 each — task section 11's "default e.g.
	// 5 highlights, 5 attention items, 5 actions" (the one Policy field
	// group with a genuine structural default, since "no cap at all" is
	// rarely what a caller wants from an executive summary and task
	// section 11 explicitly proposes 5 as the example default).
	MaxExecutiveHighlights int `json:"max_executive_highlights,omitempty"`
	MaxSectionHighlights   int `json:"max_section_highlights,omitempty"`
	MaxActions             int `json:"max_actions,omitempty"`

	// SeverityWeights/CategoryWeights are reserved for a caller wanting to
	// annotate its own downstream presentation layer with a numeric
	// weight; Build itself never sums or otherwise combines these into a
	// hidden score (task section 13's "no hidden scoring" rule) — Priority
	// selection always goes through PriorityRules/priorityRuleKindOrder,
	// never through these maps. Nil-safe: a nil map here is read as empty,
	// never causing a nil-map-write panic (Build never writes to it).
	SeverityWeights map[Severity]int `json:"severity_weights,omitempty"`
	CategoryWeights map[string]int   `json:"category_weights,omitempty"`

	Materiality MaterialityPolicy   `json:"materiality"`
	Synthesis   SynthesisThresholds `json:"synthesis"`

	IncludeWarnings      bool `json:"include_warnings"`
	IncludeInformational bool `json:"include_informational"`

	PriorityRules []PriorityRule `json:"priority_rules,omitempty"`

	// SourcePreferences declares authoritative-source precedence per
	// metric code — task section 19/58. A metric code with no entry here
	// falls back to this package's own documented default order for a
	// handful of well-known duplicates (DSO/DPO/DIO — see
	// defaultSourceOrder in sourceprecedence.go), unless
	// DisableDefaultSourceOrder is set; a metric code with neither an
	// explicit entry here nor a package default, whose candidates
	// disagree beyond ConflictTolerance, produces a SOURCE_CONFLICT Issue
	// rather than a silent pick.
	SourcePreferences []SourcePreference `json:"source_preferences,omitempty"`
	// DisableDefaultSourceOrder turns off this package's own built-in
	// default source order (documented in defaultSourceOrder,
	// sourceprecedence.go) for every metric code without an explicit
	// SourcePreferences entry — a caller wanting SOURCE_CONFLICT surfaced
	// for every undeclared-precedence disagreement, including the
	// well-known DSO/DPO/DIO pairs this package would otherwise resolve
	// silently, sets this true.
	DisableDefaultSourceOrder bool `json:"disable_default_source_order,omitempty"`
	// ConflictTolerance is the maximum absolute difference between two
	// same-metric-code candidate values from different modules that is
	// still treated as agreement (not a conflict) — task section 46/59.
	// Zero means any nonzero difference is a conflict.
	ConflictTolerance float64 `json:"conflict_tolerance,omitempty"`

	// CategoryOrder overrides DefaultCategoryOrder's fixed section
	// sequence when non-empty — task section 61. An entry naming a
	// SectionCode not in DefaultCategoryOrder is ignored; a
	// DefaultCategoryOrder entry missing from CategoryOrder is appended
	// after every named entry, in its own default relative order, so an
	// incomplete override never silently drops a section from output.
	CategoryOrder []SectionCode `json:"category_order,omitempty"`

	// IncludeManagementQuestions turns on the QuestionsForManagement
	// section — task section 41's "caller can disable this section"
	// requirement. Defaults to false (opt-in), since template questions
	// are a presentation choice some callers may not want.
	IncludeManagementQuestions bool `json:"include_management_questions"`

	// AsOfDateTolerance / RequireAlignedAsOfDates configure task section
	// 89's as-of-date consistency check for point-in-time inputs (cash,
	// AR, AP, inventory, reconciliation, close). A caller leaving both at
	// zero value never triggers AS_OF_DATE_MISMATCH — the check is opt-in
	// via RequireAlignedAsOfDates since many legitimate packs mix
	// point-in-time inputs captured on different days.
	RequireAlignedAsOfDates bool `json:"require_aligned_as_of_dates"`
}

const defaultMaxCount = 5

// resolvePolicy returns p with structural zero-value fields substituted —
// see Policy's doc comment for exactly which fields get a default (only
// the three max-count fields; every threshold/materiality/weight field is
// used exactly as supplied, zero meaning "disabled"). Never mutates p;
// always builds fresh slices/maps for anything it returns, per this
// package's own copy of the portfolio/diagnostics.resolvePolicy safe-copy
// pattern (see the repository memory's "remember the prior
// portfolio/diagnostics.DefaultPolicy.SeverityWeights bug" note this
// package was explicitly told to avoid repeating — task section 12).
func resolvePolicy(p Policy) Policy {
	resolved := p
	if resolved.MaxExecutiveHighlights == 0 {
		resolved.MaxExecutiveHighlights = defaultMaxCount
	}
	if resolved.MaxSectionHighlights == 0 {
		resolved.MaxSectionHighlights = defaultMaxCount
	}
	if resolved.MaxActions == 0 {
		resolved.MaxActions = defaultMaxCount
	}

	if len(p.SeverityWeights) > 0 {
		merged := make(map[Severity]int, len(p.SeverityWeights))
		for k, v := range p.SeverityWeights {
			merged[k] = v
		}
		resolved.SeverityWeights = merged
	}
	if len(p.CategoryWeights) > 0 {
		merged := make(map[string]int, len(p.CategoryWeights))
		for k, v := range p.CategoryWeights {
			merged[k] = v
		}
		resolved.CategoryWeights = merged
	}
	if len(p.PriorityRules) > 0 {
		resolved.PriorityRules = append([]PriorityRule(nil), p.PriorityRules...)
	}
	if len(p.SourcePreferences) > 0 {
		out := make([]SourcePreference, len(p.SourcePreferences))
		for i, sp := range p.SourcePreferences {
			out[i] = SourcePreference{MetricCode: sp.MetricCode, OrderedSources: append([]string(nil), sp.OrderedSources...)}
		}
		resolved.SourcePreferences = out
	}
	if len(p.CategoryOrder) > 0 {
		resolved.CategoryOrder = append([]SectionCode(nil), p.CategoryOrder...)
	}
	return resolved
}

// resolveCategoryOrder returns Policy.CategoryOrder extended with any
// DefaultCategoryOrder entry it omitted (appended in default relative
// order) — see Policy.CategoryOrder's doc comment. Returns
// DefaultCategoryOrder() unchanged when Policy.CategoryOrder is empty.
// Always returns a fresh slice.
func resolveCategoryOrder(p Policy) []SectionCode {
	if len(p.CategoryOrder) == 0 {
		return DefaultCategoryOrder()
	}
	seen := make(map[SectionCode]bool, len(p.CategoryOrder))
	out := make([]SectionCode, 0, len(sectionOrder))
	for _, c := range p.CategoryOrder {
		if seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, c)
	}
	for _, c := range sectionOrder {
		if !seen[c] {
			out = append(out, c)
		}
	}
	return out
}

// ExamplePolicy returns a conservative, documented example [Policy] for
// tests and documentation — task section 105. This is a labeled example,
// not accounting advice or a recommended production configuration; a real
// caller's thresholds depend on its own business context. It prioritizes
// blocking close/reconciliation conditions, failed covenants, liquidity
// threshold breaches, material adverse movement, warnings, then
// informational facts — the fixed precedence task section 14 already
// requires; this example only supplies concrete threshold numbers. Returns
// a fresh Policy on every call; never a shared mutable value.
func ExamplePolicy() Policy {
	return Policy{
		MaxExecutiveHighlights: 5,
		MaxSectionHighlights:   5,
		MaxActions:             5,
		Materiality: MaterialityPolicy{
			AbsoluteThreshold:        10000,
			PercentThreshold:         0.10,
			PercentagePointThreshold: 0.05,
		},
		Synthesis: SynthesisThresholds{
			MinimumCashThreshold:    25000,
			AROverdueIncreasePoints: 0.05,
			MarginDeclinePoints:     0.03,
			LaborCostIncreasePoints: 0.03,
		},
		IncludeWarnings:            true,
		IncludeInformational:       false,
		IncludeManagementQuestions: true,
		ConflictTolerance:          0.01,
	}
}
