package inventory

// Input bundles every caller-supplied source Calculate needs. Only
// AsOfDate is required for any snapshot-level output at all; Periods is
// required for any period/turnover/DIO/purchases-vs-usage history output.
// Every other slice is optional and Calculate computes whatever subset of
// the analysis the supplied input supports — task section 2's two
// convergent input levels: Items+Snapshots+Movements is the detailed
// path, PeriodFinancials is the summary path, and both converge on
// PeriodSummary wherever their semantics are equivalent.
type Input struct {
	// AsOfDate is the snapshot-level analysis date. Required for
	// PortfolioSummary, AgingSummary, ItemLastMovement, StockPolicy
	// comparison, ExpirySummary, and item/location/category
	// concentration. Format: any valid time.Time.
	AsOfDate string `json:"as_of_date"`

	Items     []Item              `json:"items,omitempty"`
	Snapshots []InventorySnapshot `json:"snapshots,omitempty"`
	Movements []Movement          `json:"movements,omitempty"`

	StockPolicies []StockPolicy `json:"stock_policies,omitempty"`
	MarketValues  []MarketValue `json:"market_values,omitempty"`

	// Periods is the explicit set of periods to analyze — required for
	// TurnoverHistory, PurchaseSummary/UsageSummary-by-period,
	// PurchaseVsUsageTrend, and AdjustmentSummary-by-period.
	Periods []PeriodInfo `json:"periods,omitempty"`
	// Financials is optional per-period Revenue/COGS/beginning-ending
	// inventory/purchases/units — the summary input path (task section
	// 2B), and the source for inventory-build and DIO-history signals
	// regardless of which input path is used.
	Financials []PeriodFinancials `json:"financials,omitempty"`

	// GLControls is optional GL inventory control balance(s) for
	// reconciliation — see GLControl's Component field for multi-account
	// reconciliation.
	GLControls []GLControl `json:"gl_controls,omitempty"`

	// VelocityWindows is an optional, explicit per-item historical
	// window for movement-velocity/supply-duration analysis — task
	// sections 24-25. Keyed by ItemID; a caller wanting the same window
	// for every item supplies one entry per item.
	VelocityWindows map[string]VelocityWindow `json:"velocity_windows,omitempty"`
}

// PeriodSummary is one period's turnover/DIO/purchases/usage/adjustment
// analysis — the unit both input paths converge onto.
type PeriodSummary struct {
	Period PeriodInfo `json:"period"`

	Turnover TurnoverResult `json:"turnover"`
	DIO      DIOResult      `json:"dio"`

	Purchases       PurchaseSummary      `json:"purchases"`
	Usage           UsageSummary         `json:"usage"`
	PurchaseVsUsage PurchaseVsUsageTrend `json:"purchase_vs_usage"`

	Adjustments AdjustmentSummary `json:"adjustments"`

	QuantityRollforward QuantityRollforward `json:"quantity_rollforward"`
	ValueRollforward    ValueRollforward    `json:"value_rollforward"`
}

// Result is Calculate's top-level output.
type Result struct {
	SchemaVersion  string `json:"schema_version"`
	FormulaVersion string `json:"formula_version"`
	// Available is false only if Calculate could not proceed at all (a
	// missing/invalid AsOfDate with no valid Periods either) — every
	// other field is then zero-value.
	Available bool `json:"available"`

	AsOfDate string `json:"as_of_date,omitempty"`

	Portfolio      PortfolioSummary   `json:"portfolio"`
	Aging          AgingSummary       `json:"aging"`
	LastMovement   []ItemLastMovement `json:"last_movement,omitempty"`
	Velocity       []ItemVelocity     `json:"velocity,omitempty"`
	SupplyDuration []SupplyDuration   `json:"supply_duration,omitempty"`

	StockPolicyResults []StockPolicyResult `json:"stock_policy_results,omitempty"`

	Concentration ConcentrationSummary `json:"concentration"`
	Composition   CompositionSummary   `json:"composition"`

	Periods         []PeriodSummary `json:"periods,omitempty"`
	TurnoverHistory TurnoverHistory `json:"turnover_history"`

	Reconciliation ReconciliationSummary `json:"reconciliation"`

	ExpirySummary ExpirySummary `json:"expiry_summary"`

	MarketValueComparisons []MarketValueComparison `json:"market_value_comparisons,omitempty"`

	PossibleDuplicateMovements []PossibleDuplicateMovement `json:"possible_duplicate_movements,omitempty"`

	Coverage Coverage `json:"coverage"`

	Flags  []Flag  `json:"flags,omitempty"`
	Issues []Issue `json:"issues,omitempty"`
}
