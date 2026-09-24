package vendorspend

// BasisSpend is one SpendBasis's net-spend total — task section 6's
// "if mixed basis is intentionally allowed, summarize separately" rule:
// this is always computed (regardless of Policy.RequireSingleBasis), so
// a caller allowing mixed bases can still see exactly how much spend
// falls under each one.
type BasisSpend struct {
	Basis SpendBasis `json:"basis"`
	Spend float64    `json:"spend"`
}

// Result is the output of Calculate — task section 39.
type Result struct {
	// SchemaVersion/FormulaVersion identify the fixed shapes/formulas
	// that produced this Result — see versions.go.
	SchemaVersion  string `json:"schema_version"`
	FormulaVersion string `json:"formula_version"`

	// Available is false only if Calculate could not proceed at all (no
	// valid periods, suppliers, or spend records survived validation) —
	// every other field is then zero-value except Issues.
	Available bool `json:"available"`

	// ReportingCurrency is the single currency this Result's aggregate
	// dollar figures are computed under — see resolveReportingCurrency.
	ReportingCurrency string `json:"reporting_currency,omitempty"`

	// Periods is every period label included in this analysis, in
	// chronological order.
	Periods []string `json:"periods,omitempty"`

	// Bridge is the overall (every included period, every included
	// supplier) gross/credit/net spend bridge.
	Bridge SpendBridge `json:"bridge"`
	// SpendByBasis is the net-spend total for each distinct SpendBasis
	// present among included records — see BasisSpend.
	SpendByBasis []BasisSpend `json:"spend_by_basis,omitempty"`

	// PeriodSummaries is one PeriodSummary per included period, in
	// chronological order.
	PeriodSummaries []PeriodSummary `json:"period_summaries,omitempty"`
	// SupplierSummaries is one SupplierPeriodSummary per (supplier,
	// period) combination with activity, grouped by period in
	// chronological order and, within each period, sorted by NetSpend
	// descending then SupplierID ascending.
	SupplierSummaries []SupplierPeriodSummary `json:"supplier_summaries,omitempty"`
	// CategorySummaries is the overall (all-period) category breakdown.
	CategorySummaries []CategorySummary `json:"category_summaries,omitempty"`
	// ProductSummaries is the overall (all-period) product breakdown —
	// see ProductSpend.
	ProductSummaries []ProductSpend `json:"product_summaries,omitempty"`
	// DepartmentSummaries/LocationSummaries mirror CategorySummaries'
	// overall (all-period) breakdown for Department/Location.
	DepartmentSummaries []GroupedSpend `json:"department_summaries,omitempty"`
	LocationSummaries   []GroupedSpend `json:"location_summaries,omitempty"`

	Concentration     SpendConcentration `json:"concentration"`
	DependencySummary DependencySummary  `json:"dependency_summary"`
	NewLostSuppliers  NewLostSuppliers   `json:"new_lost_suppliers"`
	Trends            SpendTrends        `json:"trends"`

	UnitPricePoints         []UnitPricePoint           `json:"unit_price_points,omitempty"`
	ProductPriceComparisons []ProductPriceComparison   `json:"product_price_comparisons,omitempty"`
	PriceVolume             []PriceVolumeDecomposition `json:"price_volume,omitempty"`

	RecurringSpend RecurringSpend  `json:"recurring_spend"`
	CommitmentMix  []CommitmentMix `json:"commitment_mix,omitempty"`

	TailSpend           TailSpend            `json:"tail_spend"`
	DuplicateLikeGroups []DuplicateLikeGroup `json:"duplicate_like_groups,omitempty"`

	NonPreferredSpend      NonPreferredSpend      `json:"non_preferred_spend"`
	OutsideContractedSpend OutsideContractedSpend `json:"outside_contracted_spend"`

	ControlReconciliation ControlReconciliation `json:"control_reconciliation"`

	CategoryProductCoverage CategoryProductCoverage `json:"category_product_coverage"`
	Coverage                MetadataCoverage        `json:"coverage"`

	Flags  []Flag  `json:"flags,omitempty"`
	Issues []Issue `json:"issues,omitempty"`

	// Policy/Thresholds echo the resolved configuration this Result was
	// computed under.
	Policy     Policy     `json:"policy"`
	Thresholds Thresholds `json:"thresholds"`
}
