package advisory

// SectionCode is a stable identifier for one fixed advisory-pack section.
// Declaration order below is [Result.Sections]' fixed output order and
// [DefaultCategoryOrder]'s default — task section 61's "provide a stable
// default section/category order... avoid alphabetical if a management-
// flow order is better" instruction. The order follows task section 61's
// recommended management-flow sequence (liquidity, financial performance,
// working capital, profitability, operations, debt/covenants, accounting/
// close, forecast, valuation, transaction readiness) with KPI/
// ACTION_REGISTER/APPENDIX last, matching task section 5's listed section
// set. SectionExecutive is defined (task section 5 lists EXECUTIVE) for a
// caller wanting to reference it in Policy.CategoryOrder, but carries no
// content of its own: the executive summary is [Result.ExecutiveSummary],
// a distinct top-level field (task section 11's own ExecutiveSummary
// type) built by selection over every other Section's own Highlights/
// Findings/Actions, not a Section with its own Metrics/Findings — see
// executive.go. SectionExecutive is therefore excluded from sectionOrder
// (Build never constructs a Section with that Code) and from every
// section-count-based Coverage figure.
type SectionCode string

const (
	SectionExecutive          SectionCode = "EXECUTIVE"
	SectionLiquidity          SectionCode = "LIQUIDITY"
	SectionFinancialPerf      SectionCode = "FINANCIAL_PERFORMANCE"
	SectionWorkingCapital     SectionCode = "WORKING_CAPITAL"
	SectionRevenue            SectionCode = "REVENUE"
	SectionProfitability      SectionCode = "PROFITABILITY"
	SectionLabor              SectionCode = "LABOR"
	SectionInventory          SectionCode = "INVENTORY"
	SectionVendorSpend        SectionCode = "VENDOR_SPEND"
	SectionDebtAndCovenants   SectionCode = "DEBT_AND_COVENANTS"
	SectionAccountingAndClose SectionCode = "ACCOUNTING_AND_CLOSE"
	SectionForecastAndOutlook SectionCode = "FORECAST_AND_OUTLOOK"
	SectionValuation          SectionCode = "VALUATION_AND_VALUE_DRIVERS"
	SectionTransactionReady   SectionCode = "TRANSACTION_READINESS"
	SectionKPI                SectionCode = "KPI"
	SectionActionRegister     SectionCode = "ACTION_REGISTER"
	SectionAppendix           SectionCode = "APPENDIX"
)

// DefaultCategoryOrder is the fixed declaration order above, exported so a
// caller can see (or reuse as a starting point for
// [Policy.CategoryOrder]) exactly what "default" means — task section 61's
// "caller may override" allowance. Returns a fresh slice on every call;
// never a shared backing array a caller could mutate to affect future
// calls.
func DefaultCategoryOrder() []SectionCode {
	out := make([]SectionCode, len(sectionOrder))
	copy(out, sectionOrder)
	return out
}

// sectionOrder is SectionCode's fixed declaration/output order, the
// package-internal source DefaultCategoryOrder copies from and
// resolveCategoryOrder falls back to when Policy.CategoryOrder is empty.
// Excludes SectionExecutive — see SectionCode's doc comment.
var sectionOrder = []SectionCode{
	SectionLiquidity,
	SectionFinancialPerf,
	SectionWorkingCapital,
	SectionRevenue,
	SectionProfitability,
	SectionLabor,
	SectionInventory,
	SectionVendorSpend,
	SectionDebtAndCovenants,
	SectionAccountingAndClose,
	SectionForecastAndOutlook,
	SectionValuation,
	SectionTransactionReady,
	SectionKPI,
	SectionActionRegister,
	SectionAppendix,
}

// sectionRank returns c's position in the resolved order, len(order) for
// an unrecognized code (sorts last, never panics) — the shared tie-break
// primitive every ordering function in this package (executive selection,
// action/insight sort, coverage listing) uses instead of Go map order.
func sectionRank(c SectionCode, order []SectionCode) int {
	for i, s := range order {
		if s == c {
			return i
		}
	}
	return len(order)
}

// AvailabilityStatus distinguishes why a [Section]/[Metric] does or does
// not carry a figure — task section 45's five-state model. Do not equate
// StatusNotSupplied with StatusUnavailable: the former means the caller
// never gave this pack the relevant Input section at all, the latter means
// the caller did supply it but the underlying sibling Result itself
// reported Available == false (or, for a point value, its own
// Value.Available == false).
type AvailabilityStatus string

const (
	// StatusAvailable means the figure/section was supplied and usable.
	StatusAvailable AvailabilityStatus = "AVAILABLE"
	// StatusUnavailable means the relevant Input section was supplied but
	// the sibling Result (or the specific field read from it) itself
	// reported unavailable.
	StatusUnavailable AvailabilityStatus = "UNAVAILABLE"
	// StatusNotSupplied means the caller left the relevant Input field at
	// its zero value entirely.
	StatusNotSupplied AvailabilityStatus = "NOT_SUPPLIED"
	// StatusNotApplicable means this figure/section does not apply given
	// the pack's scope/context (e.g. a debt section when CompanyContext
	// declares no debt facilities exist is still StatusNotSupplied, not
	// StatusNotApplicable, since this package never infers applicability —
	// StatusNotApplicable is reserved for a source sibling's own explicit
	// NOT_APPLICABLE-shaped classification, e.g.
	// accounting/profitability.ViewStatusNotApplicable, echoed through
	// unchanged).
	StatusNotApplicable AvailabilityStatus = "NOT_APPLICABLE"
	// StatusInvalid means the relevant Input section was supplied but was
	// structurally invalid (e.g. a Prior result whose own Available is
	// true but whose section shape could not be reconciled) — see the
	// section/metric's own accompanying Issue for detail.
	StatusInvalid AvailabilityStatus = "INVALID"
)

// Section is one fixed, presentation-neutral advisory-pack section — task
// section 6. Every Section independently reports its own Availability;
// Highlights/Metrics/Findings/Actions are non-empty only when Availability
// is StatusAvailable.
type Section struct {
	Code         SectionCode        `json:"code"`
	Availability AvailabilityStatus `json:"availability"`

	Highlights []Insight    `json:"highlights,omitempty"`
	Metrics    []Metric     `json:"metrics,omitempty"`
	Findings   []Insight    `json:"findings,omitempty"`
	Actions    []ActionItem `json:"actions,omitempty"`

	// Sources lists every distinct SourceModule this Section drew from, in
	// sibling-declaration order — a quick "what fed this section" index
	// without scanning every Metric/Insight/Action's own SourceRefs.
	Sources []SourceRef `json:"sources,omitempty"`
}

// newUnavailableSection returns the canonical empty Section for status,
// with no Highlights/Metrics/Findings/Actions/Sources — the shared
// constructor every section_*.go builder uses for its early-return path
// when its own required Input is absent/invalid, so every unavailable
// Section is byte-identical in shape regardless of which builder produced
// it.
func newUnavailableSection(code SectionCode, status AvailabilityStatus) Section {
	return Section{Code: code, Availability: status}
}
