package profitability

// IssueSeverity distinguishes a problem that excludes a row or blocks
// some portion of the analysis (SeverityError) from one that is advisory
// only (SeverityWarning) — the same two-severity model every sibling
// package in this repository uses.
type IssueSeverity string

const (
	SeverityError   IssueSeverity = "error"
	SeverityWarning IssueSeverity = "warning"
)

// IssueCode is a stable identifier for one kind of input/validation
// problem — never a profitability/business-result signal, which is a
// Flag instead (see flags.go). Only codes this package actually emits
// are defined — task section 62.
type IssueCode string

const (
	IssueInvalidPeriod                    IssueCode = "INVALID_PERIOD"
	IssueDuplicatePeriod                  IssueCode = "DUPLICATE_PERIOD"
	IssueDuplicateEntity                  IssueCode = "DUPLICATE_ENTITY"
	IssueUnknownEntity                    IssueCode = "UNKNOWN_ENTITY"
	IssueDuplicateFact                    IssueCode = "DUPLICATE_FACT"
	IssueInvalidComponent                 IssueCode = "INVALID_COMPONENT"
	IssueNonFiniteAmount                  IssueCode = "NON_FINITE_AMOUNT"
	IssueNegativeAmount                   IssueCode = "NEGATIVE_AMOUNT"
	IssueInvalidAttribution               IssueCode = "INVALID_ATTRIBUTION"
	IssueAttributionExceeds100Percent     IssueCode = "ATTRIBUTION_EXCEEDS_100_PERCENT"
	IssueInvalidSummaryRow                IssueCode = "INVALID_SUMMARY_ROW"
	IssueInvalidDriver                    IssueCode = "INVALID_DRIVER"
	IssueDuplicateDriver                  IssueCode = "DUPLICATE_DRIVER"
	IssueInvalidSharedCostPool            IssueCode = "INVALID_SHARED_COST_POOL"
	IssueDuplicatePool                    IssueCode = "DUPLICATE_SHARED_COST_POOL"
	IssueInvalidAllocationRule            IssueCode = "INVALID_ALLOCATION_RULE"
	IssueDuplicateAllocationRule          IssueCode = "DUPLICATE_ALLOCATION_RULE"
	IssueAllocationDenominatorUnavailable IssueCode = "ALLOCATION_DENOMINATOR_UNAVAILABLE"
	// IssueAllocationEntityExcluded means an AllocationRule's
	// FixedWeights (FIXED_WEIGHT, or EQUAL with a restricted membership
	// list) named an EntityID with no fact/summary activity for that
	// dimension/period. That entity is excluded from the allocation
	// (never allocated dollars that no EntityPeriodResult could ever
	// receive) and the remaining named entities share the pool amount
	// among themselves.
	IssueAllocationEntityExcluded IssueCode = "ALLOCATION_ENTITY_EXCLUDED"
	IssueInvalidFixedWeights      IssueCode = "INVALID_FIXED_WEIGHTS"
	IssueMixedCurrency            IssueCode = "MIXED_CURRENCY"
	IssueInvalidControlTotal      IssueCode = "INVALID_CONTROL_TOTAL"
	IssueInvalidPolicy            IssueCode = "INVALID_POLICY"
	IssueInputModeConflict        IssueCode = "INPUT_MODE_CONFLICT"
)

// issueCodeOrder fixes IssueCode declaration order for deterministic
// Issues sorting — task section 71 "issues code/source ID."
var issueCodeOrder = []IssueCode{
	IssueInvalidPeriod,
	IssueDuplicatePeriod,
	IssueDuplicateEntity,
	IssueUnknownEntity,
	IssueDuplicateFact,
	IssueInvalidComponent,
	IssueNonFiniteAmount,
	IssueNegativeAmount,
	IssueInvalidAttribution,
	IssueAttributionExceeds100Percent,
	IssueInvalidSummaryRow,
	IssueInvalidDriver,
	IssueDuplicateDriver,
	IssueInvalidSharedCostPool,
	IssueDuplicatePool,
	IssueInvalidAllocationRule,
	IssueDuplicateAllocationRule,
	IssueAllocationDenominatorUnavailable,
	IssueAllocationEntityExcluded,
	IssueInvalidFixedWeights,
	IssueMixedCurrency,
	IssueInvalidControlTotal,
	IssueInvalidPolicy,
	IssueInputModeConflict,
}

func issueRank(c IssueCode) int {
	for i, ic := range issueCodeOrder {
		if ic == c {
			return i
		}
	}
	return len(issueCodeOrder)
}

// Issue is a single structured validation/analysis finding, returned
// instead of an error so callers can inspect every problem found in one
// pass.
type Issue struct {
	Code     IssueCode     `json:"code"`
	Severity IssueSeverity `json:"severity"`
	Message  string        `json:"message"`

	// SourceID identifies the FactID/EntityID/PoolID/ObservationID this
	// Issue relates to, if any.
	SourceID  string    `json:"source_id,omitempty"`
	Dimension Dimension `json:"dimension,omitempty"`
	Period    string    `json:"period,omitempty"`
}

// HasErrors reports whether any Issue in issues has SeverityError.
//
// Intentionally duplicated from labor.HasErrors/ar.HasErrors/
// ap.HasErrors and this repository's other sibling HasErrors functions
// rather than shared — see financial/adjustments.HasErrors's doc comment
// for the full rationale (each package's Issue is a distinct Go type with
// no common interface worth introducing for one boolean function).
func HasErrors(issues []Issue) bool {
	for _, i := range issues {
		if i.Severity == SeverityError {
			return true
		}
	}
	return false
}
