package kpi

// AvailabilityReason is a stable, typed reason a Value's Amount could not
// be computed — task section 9's "do not encode the reason only in prose"
// instruction. Only reasons this package actually emits are defined.
type AvailabilityReason string

const (
	// AvailabilityReasonNone is the zero value, used only when Available
	// is true (a computed Value carries no reason).
	AvailabilityReasonNone AvailabilityReason = ""
	// AvailabilityMissingMetric means a METRIC expression referenced a
	// MetricValue (code, period, dimension) combination that was not
	// supplied at all.
	AvailabilityMissingMetric AvailabilityReason = "MISSING_METRIC"
	// AvailabilitySourceUnavailable means the referenced MetricValue was
	// supplied but had Available == false.
	AvailabilitySourceUnavailable AvailabilityReason = "SOURCE_UNAVAILABLE"
	// AvailabilityDependencyUnavailable means a KPI expression referenced
	// another KPI (directly or transitively) whose own Value was
	// unavailable.
	AvailabilityDependencyUnavailable AvailabilityReason = "DEPENDENCY_UNAVAILABLE"
	// AvailabilityDivideByZero means a DIVIDE/PERCENT/PERCENT_CHANGE
	// operator's denominator resolved to exactly 0.
	AvailabilityDivideByZero AvailabilityReason = "DIVIDE_BY_ZERO"
	// AvailabilityUnitMismatch means an operator's operands had
	// incompatible Units (e.g. Currency + Hours) — see unit.go.
	AvailabilityUnitMismatch AvailabilityReason = "UNIT_MISMATCH"
	// AvailabilityCurrencyMismatch means an operator's operands were both
	// UnitCurrency but with different CurrencyCode values.
	AvailabilityCurrencyMismatch AvailabilityReason = "CURRENCY_MISMATCH"
	// AvailabilityPeriodUnavailable means the requested Period was not
	// present among the periods this evaluation was asked to cover, or
	// (for PRIOR_PERIOD/PRIOR_YEAR_SAME_PERIOD/TRAILING_N references) no
	// qualifying comparison period exists.
	AvailabilityPeriodUnavailable AvailabilityReason = "PERIOD_UNAVAILABLE"
	// AvailabilityDimensionUnavailable means a MetricRef/KPIRef requested
	// a specific DimensionKey that was not present among the supplied
	// MetricValues, and no applicable broadcast/aggregation rule resolved
	// it (see dimension.go).
	AvailabilityDimensionUnavailable AvailabilityReason = "DIMENSION_UNAVAILABLE"
	// AvailabilityNotApplicable means this KPI/period/dimension
	// combination is structurally not something this engine attempts to
	// evaluate (e.g. a KPIRef naming a KPI whose own Definition was
	// invalid — see AvailabilityInvalidDefinition, which is used instead
	// for that specific case; AvailabilityNotApplicable is reserved for
	// future structural exclusions and is not currently emitted, kept
	// defined per task section 9's fixed reason enum).
	AvailabilityNotApplicable AvailabilityReason = "NOT_APPLICABLE"
	// AvailabilityInvalidDefinition means the KPI's own Definition failed
	// validation (see definition.go/dependency.go) — evaluated instances
	// of it are unavailable for that reason rather than attempting partial
	// evaluation of a structurally broken formula.
	AvailabilityInvalidDefinition AvailabilityReason = "INVALID_DEFINITION"
	// AvailabilityNonFiniteResult means an operator produced NaN or +/-Inf
	// (e.g. extreme-magnitude overflow) — task section 8's "guard every
	// result against NaN/Inf/overflow" instruction. Never surfaced as a
	// silently-returned NaN/Inf float.
	AvailabilityNonFiniteResult AvailabilityReason = "NON_FINITE_RESULT"
)

// Value is a single evaluated figure with typed availability — task
// section 9. Amount is meaningful only when Available is true; an
// unavailable Value always has Amount == 0 and a non-empty Reason (unless
// Available, in which case Reason is AvailabilityReasonNone).
type Value struct {
	Amount    float64            `json:"amount"`
	Available bool               `json:"available"`
	Reason    AvailabilityReason `json:"reason,omitempty"`
}

// availableValue returns a Value carrying a successfully computed amount.
func availableValue(amount float64) Value {
	return Value{Amount: amount, Available: true}
}

// unavailableValue returns a Value carrying no amount, with reason
// explaining why.
func unavailableValue(reason AvailabilityReason) Value {
	return Value{Reason: reason}
}
