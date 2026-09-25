package advisory

// SchemaVersion identifies this package's own Result/Section/Insight/
// Metric/ActionItem/ExecutiveSummary/Snapshot/Coverage shape — the field
// layout, not what populates it. Bump whenever any exported type's shape
// changes in a way that could make a historical [Result] fail to
// unmarshal, or unmarshal into the wrong field, under new code. Distinct
// from [FormulaVersion] and [AdvisoryContractVersion] — see the repository
// README's versioning-strategy section, whose one-constant-per-
// versionable-concept rule this package follows exactly (mirroring
// accounting/closechecklist's three-way Schema/Formula/TemplateContract
// split, the closest existing precedent for a package needing more than
// one version axis).
const SchemaVersion = "1.0.0"

// FormulaVersion identifies this package's own composition rules: which
// sibling Result field populates each [Metric]/[Insight] (the adapter_*.go
// files), the source-precedence/conflict-detection rule (sourceprecedence.go),
// the current/prior change formulas (value.go), the executive-selection
// and priority-rule precedence (priority.go), the cross-module synthesis
// rules (synthesis.go), and the action-deduplication identity rule
// (action.go). This package computes no new domain formula of its own —
// see the package doc comment — so FormulaVersion covers composition
// logic only, never a recalculated accounting/analytics figure.
const FormulaVersion = "1.0.0"

// AdvisoryContractVersion identifies the semantic meaning of every
// generated statement/insight/action mapping this package produces: which
// [StatementCode]/action-template/question-template exists and what
// deterministic rule produces it (statements.go, action templates in
// action.go, question templates in question.go). Because generated
// wording may be persisted or exported by a caller, this is versioned
// separately from FormulaVersion (a caller may need to know "did
// AR_OVER_90_INCREASED mean the same thing in this historical pack as it
// does today" independent of whether the underlying arithmetic changed) —
// see docs/CONTROLLER_CFO_ADVISORY_PACK.md's versioning section.
const AdvisoryContractVersion = "1.0.0"

// SourceVersions echoes every contributing sibling package's own version
// constant(s), captured once per [Build] call from whatever was actually
// Available in [Input] — reproducibility requires knowing not just this
// package's own three version constants but exactly which upstream
// formula/schema version produced each fact a [Result] is built from (task
// section 96). Every field is the empty string when its corresponding
// Input section was unavailable; never omitted, so a caller can always
// range over a complete, fixed-shape list without a nil check.
type SourceVersions struct {
	Metrics        string `json:"metrics,omitempty"`
	Ratios         string `json:"ratios,omitempty"`
	WorkingCapital string `json:"working_capital,omitempty"`
	CashFlow       string `json:"cash_flow,omitempty"`
	RevenueQuality string `json:"revenue_quality,omitempty"`
	Concentration  string `json:"concentration,omitempty"`
	QoE            string `json:"qoe,omitempty"`
	Variance       string `json:"variance,omitempty"`
	Forecast       string `json:"forecast,omitempty"`
	Debt           string `json:"debt,omitempty"`
	Covenants      string `json:"covenants,omitempty"`
	ValueDrivers   string `json:"value_drivers,omitempty"`
	Consolidation  string `json:"consolidation,omitempty"`
	Diagnostics    string `json:"diagnostics,omitempty"`

	Acquisition   string `json:"acquisition,omitempty"`
	DealStructure string `json:"deal_structure,omitempty"`
	SaleReadiness string `json:"sale_readiness,omitempty"`

	PortfolioDiagnostics string `json:"portfolio_diagnostics,omitempty"`
	Management           string `json:"management,omitempty"`

	Ledger             string `json:"ledger,omitempty"`
	Statements         string `json:"statements,omitempty"`
	AR                 string `json:"ar,omitempty"`
	AP                 string `json:"ap,omitempty"`
	JournalDiagnostics string `json:"journal_diagnostics,omitempty"`
	CloseQuality       string `json:"close_quality,omitempty"`
	CashForecast       string `json:"cash_forecast,omitempty"`
	Labor              string `json:"labor,omitempty"`
	Inventory          string `json:"inventory,omitempty"`
	Profitability      string `json:"profitability,omitempty"`
	VendorSpend        string `json:"vendor_spend,omitempty"`
	Reconciliation     string `json:"reconciliation,omitempty"`
	CloseChecklist     string `json:"close_checklist,omitempty"`

	KPI string `json:"kpi,omitempty"`

	Consensus string `json:"consensus,omitempty"`
}
