package profitability

// ViewStatus reports one DimensionView's availability — task section 66.
// Never inferred from an empty slice; explicitly set by Calculate based
// on Policy.DimensionApplicability and the input actually supplied.
type ViewStatus string

const (
	// ViewStatusAvailable means this dimension was computed and every
	// configured coverage threshold was met (or none was configured).
	ViewStatusAvailable ViewStatus = "AVAILABLE"
	// ViewStatusAvailableWithGaps means this dimension was computed but
	// Policy.StrictAttributionCoverage is set and at least one period
	// fell short of MinRevenueAttributionCoverage/
	// MinCostAttributionCoverage.
	ViewStatusAvailableWithGaps ViewStatus = "AVAILABLE_WITH_GAPS"
	// ViewStatusNotApplicable means Policy.DimensionApplicability marked
	// this dimension ApplicabilityNotApplicable.
	ViewStatusNotApplicable ViewStatus = "NOT_APPLICABLE"
	// ViewStatusUnavailable means this dimension is ApplicabilityEnabled
	// but no Entity/Fact/EntityPeriodSummaryInput referenced it at all.
	ViewStatusUnavailable ViewStatus = "UNAVAILABLE"
	// ViewStatusInvalid means this dimension's input was structurally
	// invalid in a way that blocked computation (reserved; V1 never
	// returns this — validation failures instead exclude the offending
	// row and computation proceeds with what remains).
	ViewStatusInvalid ViewStatus = "INVALID"
)

// DimensionView is one dimension's (CUSTOMER, JOB, or PRODUCT) full
// analytical output — task sections 31-33, 65-66. Structurally identical
// across all three dimensions (the three views are alternate lenses over
// the same fact population, task section 1); dimension-specific
// documentation lives in customerview.go/jobview.go/productview.go's
// constructor doc comments, not in separate types.
type DimensionView struct {
	Dimension Dimension  `json:"dimension"`
	Status    ViewStatus `json:"status"`

	Periods []PeriodInfo `json:"periods,omitempty"`

	// EntityPeriods is every entity's per-period result, sorted by
	// Period then ContributionProfit descending then EntityID.
	EntityPeriods []EntityPeriodResult `json:"entity_periods,omitempty"`
	// AllPeriod is every entity's all-period total, sorted by
	// ContributionProfit descending then EntityID.
	AllPeriod []AllPeriodResult `json:"all_period,omitempty"`

	GroupSummaries    []GroupSummary `json:"group_summaries,omitempty"`
	CategorySummaries []GroupSummary `json:"category_summaries,omitempty"`

	Unattributed map[string]UnattributedAmounts `json:"unattributed,omitempty"`

	AttributionCoverage []AttributionCoverage `json:"attribution_coverage,omitempty"`

	AllocationResults []PoolAllocationResult `json:"allocation_results,omitempty"`

	Rankings      Rankings             `json:"rankings"`
	MarginRanking []MarginRankingEntry `json:"margin_ranking,omitempty"`

	Trends []EntityTrend `json:"trends,omitempty"`

	Flags []Flag `json:"flags,omitempty"`
}

// BusinessTotals is the business-level (all-dimension-independent)
// totals computed directly from Facts/EntityPeriodSummaries and
// SharedCostPools — task section 35. Never derived by summing
// DimensionView totals (which would double count, since the same fact
// contributes to every dimension it is attributed to).
type BusinessTotals struct {
	Periods   []BusinessPeriodTotals `json:"periods,omitempty"`
	AllPeriod BusinessPeriodTotals   `json:"all_period"`
}

// BusinessPeriodTotals is one period's (or the all-period aggregate's)
// business-level bridge, computed directly from the fact/summary
// population regardless of any dimension's attribution.
type BusinessPeriodTotals struct {
	Period string `json:"period,omitempty"`

	RevenueBridge      RevenueBridge      `json:"revenue_bridge"`
	DirectCostBridge   DirectCostBridge   `json:"direct_cost_bridge"`
	VariableCostBridge VariableCostBridge `json:"variable_cost_bridge"`

	GrossProfit        float64 `json:"gross_profit"`
	ContributionProfit float64 `json:"contribution_profit"`

	SharedCostTotal float64 `json:"shared_cost_total"`

	Margins Margins `json:"margins"`
}

func buildBusinessPeriodTotals(period string, amounts map[Component]float64, sharedCostTotal float64) BusinessPeriodTotals {
	b := BusinessPeriodTotals{Period: period}
	b.RevenueBridge = buildRevenueBridge(amounts)
	b.DirectCostBridge = buildDirectCostBridge(amounts)
	b.VariableCostBridge = buildVariableCostBridge(amounts)
	b.GrossProfit = b.RevenueBridge.NetRevenue - b.DirectCostBridge.Total
	b.ContributionProfit = b.GrossProfit - b.VariableCostBridge.Total
	b.SharedCostTotal = sharedCostTotal
	b.Margins = Margins{
		GrossMargin:        marginValue(b.GrossProfit, b.RevenueBridge.NetRevenue),
		ContributionMargin: marginValue(b.ContributionProfit, b.RevenueBridge.NetRevenue),
	}
	return b
}

// DimensionReconciliation is one period/component/dimension's
// Attributed + Unattributed == Business total check — task section 36.
type DimensionReconciliation struct {
	Dimension      Dimension `json:"dimension"`
	Period         string    `json:"period"`
	Component      Component `json:"component,omitempty"`
	Attributed     float64   `json:"attributed"`
	Unattributed   float64   `json:"unattributed"`
	BusinessAmount float64   `json:"business_amount"`
	Reconciled     bool      `json:"reconciled"`
}

// RevenueConcentration optionally reports Top-N/HHI revenue
// concentration for a dimension — task section 43. Left entirely
// Unavailable (nil Result) unless a caller-supplied adapter populates it
// via analytics/concentration — this package has no compile-time
// dependency on analytics/concentration itself (see
// concentrationadapter.go).
type RevenueConcentration struct {
	Dimension  Dimension `json:"dimension"`
	Top1Share  Value     `json:"top_1_share"`
	Top3Share  Value     `json:"top_3_share"`
	Top5Share  Value     `json:"top_5_share"`
	Top10Share Value     `json:"top_10_share"`
	HHI        Value     `json:"hhi"`
}

// Result is this package's top-level Calculate output — task section 65.
type Result struct {
	SchemaVersion  string `json:"schema_version"`
	FormulaVersion string `json:"formula_version"`

	BusinessTotals BusinessTotals `json:"business_totals"`

	CustomerView DimensionView `json:"customer_view"`
	JobView      DimensionView `json:"job_view"`
	ProductView  DimensionView `json:"product_view"`

	SharedCostPools []SharedCostPool `json:"shared_cost_pools,omitempty"`

	DimensionReconciliation []DimensionReconciliation `json:"dimension_reconciliation,omitempty"`

	ControlReconciliation []ControlReconciliation `json:"control_reconciliation,omitempty"`

	Coverage Coverage `json:"coverage"`

	Flags  []Flag  `json:"flags,omitempty"`
	Issues []Issue `json:"issues,omitempty"`
}

// ViewFor returns the DimensionView matching dim, or the zero value for
// an unrecognized Dimension.
func (r Result) ViewFor(dim Dimension) DimensionView {
	switch dim {
	case DimensionCustomer:
		return r.CustomerView
	case DimensionJob:
		return r.JobView
	case DimensionProduct:
		return r.ProductView
	default:
		return DimensionView{}
	}
}
