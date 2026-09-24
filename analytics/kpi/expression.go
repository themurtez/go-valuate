package kpi

// Operator is a fixed, closed set of arithmetic/aggregation operations an
// Expression node may perform — task section 6. This is not an extensible
// registry: adding a new kind of computation to this package means adding
// a new Operator constant and its evaluateXxx case in evaluate.go, never a
// caller-supplied function. See doc.go's "Not a scripting engine" section.
type Operator string

const (
	// OpMetric resolves Expression.MetricRef. Leaf node; Args must be
	// empty.
	OpMetric Operator = "METRIC"
	// OpKPI resolves Expression.KPIRef. Leaf node; Args must be empty.
	OpKPI Operator = "KPI"
	// OpConstant resolves Expression.Constant. Leaf node; Args must be
	// empty.
	OpConstant Operator = "CONSTANT"

	// OpAdd requires exactly 2 Args: Args[0] + Args[1]. Unit rule:
	// combineAdditive.
	OpAdd Operator = "ADD"
	// OpSubtract requires exactly 2 Args: Args[0] - Args[1]. Unit rule:
	// combineAdditive.
	OpSubtract Operator = "SUBTRACT"
	// OpMultiply requires exactly 2 Args: Args[0] * Args[1]. Unit rule:
	// combineMultiplicative.
	OpMultiply Operator = "MULTIPLY"
	// OpDivide requires exactly 2 Args: Args[0] / Args[1], unavailable
	// (AvailabilityDivideByZero) when Args[1] resolves to exactly 0. Unit
	// rule: combineDivisive.
	OpDivide Operator = "DIVIDE"
	// OpNegate requires exactly 1 Arg: -Args[0]. Unit unchanged.
	OpNegate Operator = "NEGATE"
	// OpAbs requires exactly 1 Arg: |Args[0]|. Unit unchanged.
	OpAbs Operator = "ABS"
	// OpMin requires 2+ Args: the minimum of all Args' values. Every Arg
	// must share the same Unit (combineAdditive-equivalent — exact Unit
	// match required, since MIN/MAX compare magnitudes directly).
	OpMin Operator = "MIN"
	// OpMax requires 2+ Args: the maximum of all Args' values. Same Unit
	// rule as OpMin.
	OpMax Operator = "MAX"
	// OpSum requires 1+ Args: the sum of all Args' values. Every Arg must
	// share the same Unit (combineAdditive-equivalent).
	OpSum Operator = "SUM"
	// OpAverage requires 1+ Args: the arithmetic mean of all Args' values,
	// unavailable if zero Args resolved to Available (never divides by a
	// count of zero). Every Arg must share the same Unit.
	OpAverage Operator = "AVERAGE"
	// OpWeightedAverage requires an even, non-zero number of Args, read as
	// (value, weight) pairs: Args[0]=value1, Args[1]=weight1,
	// Args[2]=value2, Args[3]=weight2, ... — task section 37's "weighted
	// average requires explicit values + weights" instruction. Result is
	// sum(value_i * weight_i) / sum(weight_i), unavailable
	// (AvailabilityDivideByZero) if the total weight resolves to exactly
	// 0 (task section 37's "zero total weight -> unavailable" rule).
	// Value Args must share the same Unit; weight Args must be
	// UnitRatio/UnitPercent/UnitUnitless/UnitCount/UnitQuantity (a scalar
	// or count-like weight, never Currency/Hours/Days, since a weight is
	// a multiplier, not itself a dollar/time figure).
	OpWeightedAverage Operator = "WEIGHTED_AVERAGE"
	// OpPercent requires exactly 2 Args: Args[0] / Args[1] * 100, the
	// PERCENT-convention (0-100) sibling of OpDivide's raw-ratio result —
	// see doc.go / docs/KPI_ENGINE.md's percent-convention section.
	// Unavailable (AvailabilityDivideByZero) when Args[1] is exactly 0.
	// Unit rule: combineDivisive's result, then forced to UnitPercent
	// (the divisive Unit rule already requires Args[0]/Args[1] to be
	// ratio-compatible; OpPercent additionally requires that resolved
	// Unit to be UnitRatio specifically, distinguishing a true percentage
	// from e.g. Currency/Count's currency-per-count result, which
	// OpPercent rejects as a unit mismatch).
	OpPercent Operator = "PERCENT"
	// OpPercentChange requires exactly 2 Args, read as (current, prior):
	// (Args[0] - Args[1]) / Args[1] * 100 — a *relative* percent change,
	// in percentage points of the *relative* change (task section 13:
	// "relative percent change"). Unavailable
	// (AvailabilityDivideByZero) when Args[1] (prior) is exactly 0. Both
	// Args must share the same Unit (combineAdditive-equivalent); result
	// Unit is always UnitPercent. Distinct from the percentage-point
	// change Trend/Change computes for a KPI whose own Unit is already
	// UnitPercent — see trend.go's doc comment and task section 13's
	// worked example (30% -> 35% is +5 percentage points via Change, not
	// automatically +16.7% via OpPercentChange, since those answer
	// different questions).
	OpPercentChange Operator = "PERCENT_CHANGE"
	// OpCoalesce requires 1+ Args: the first Arg that resolves Available,
	// or unavailable if none do. Must be explicitly used in a Definition
	// (task section 10 — never implicit) and never suppresses a
	// structural DEFINITION issue (an invalid Expression under a
	// OpCoalesce Arg is still a definition error, not silently skipped —
	// see validateExpression). Every Arg must share the same Unit as the
	// first Arg that is itself validly-typed (unit compatibility is
	// checked structurally regardless of which Arg ultimately resolves at
	// runtime).
	OpCoalesce Operator = "COALESCE"
)

func isRecognizedOperator(op Operator) bool {
	switch op {
	case OpMetric, OpKPI, OpConstant, OpAdd, OpSubtract, OpMultiply, OpDivide,
		OpNegate, OpAbs, OpMin, OpMax, OpSum, OpAverage, OpWeightedAverage,
		OpPercent, OpPercentChange, OpCoalesce:
		return true
	default:
		return false
	}
}

// TimeRef is a safe, closed set of explicit time-series references a
// MetricRef/KPIRef may use in place of (or alongside) an explicit target
// period — task section 20. This package never parses semantics from an
// arbitrary Period.Code string; every TimeRef resolves purely from
// Period.Sequence/FiscalYear/PositionInYear (see period.go).
type TimeRef string

const (
	// TimeRefCurrent (the zero value) means "the period being evaluated."
	TimeRefCurrent TimeRef = ""
	// TimeRefPriorPeriod means the Period with the next-lower Sequence
	// among Input.Periods, regardless of fiscal-year boundaries.
	TimeRefPriorPeriod TimeRef = "PRIOR_PERIOD"
	// TimeRefPriorYearSamePeriod means the Period sharing
	// (PositionInYear) with the evaluated period, in the fiscal year
	// immediately before it — resolved via Period.samePositionKey(), task
	// section 20's "prior-year requires caller period metadata sufficient
	// to identify comparable periods" rule.
	TimeRefPriorYearSamePeriod TimeRef = "PRIOR_YEAR_SAME_PERIOD"
	// TimeRefTrailingN means the trailing N periods ending at (and
	// including) the evaluated period, aggregated via the referenced
	// metric/KPI's own Aggregation rule (metrics) or, for a KPIRef,
	// unavailable — trailing-N KPI-of-KPI aggregation is deferred, see
	// TrailingN's doc comment.
	TimeRefTrailingN TimeRef = "TRAILING_N"
)

func isRecognizedTimeRef(t TimeRef) bool {
	switch t {
	case TimeRefCurrent, TimeRefPriorPeriod, TimeRefPriorYearSamePeriod, TimeRefTrailingN:
		return true
	default:
		return false
	}
}

// MetricRef points an OpMetric leaf at one source metric — task section
// 16's namespace-safety concern is handled by keeping MetricRef and
// KPIRef as distinct typed fields (never a single ambiguous "ref" string)
// rather than by any naming convention on Code itself.
type MetricRef struct {
	// Code matches MetricValue.Code exactly.
	Code string `json:"code"`
	// Dimensions, if non-empty, restricts this reference to exactly that
	// DimensionKey — task section 5's strict-exact-match default. Left
	// empty, this reference resolves against the business-level
	// (no-dimension) scope only, UNLESS Broadcast is set — see
	// dimension.go's Broadcast rules.
	Dimensions DimensionKey `json:"dimensions,omitempty"`
	// Broadcast, if true, explicitly allows this reference to resolve a
	// business-level MetricValue even when the KPI is being evaluated for
	// a specific (non-business-level) dimension group — task section 5's
	// "must not silently broadcast... unless the definition explicitly
	// allows broadcast" rule. Ignored (has no effect) when Dimensions is
	// itself non-empty.
	Broadcast bool `json:"broadcast,omitempty"`
	// Time selects which period, relative to the one being evaluated,
	// this reference resolves against — see TimeRef. Zero value
	// (TimeRefCurrent) means the evaluated period itself.
	Time TimeRef `json:"time,omitempty"`
	// TrailingN is the window size when Time == TimeRefTrailingN.
	// Required (> 0) in that case.
	TrailingN int `json:"trailing_n,omitempty"`
}

// KPIRef points an OpKPI node at another KPI Definition.Code — task
// section 14. Same Dimensions/Broadcast/Time/TrailingN shape as
// MetricRef, with TrailingN not currently supported for KPIRef (see
// TimeRefTrailingN's doc comment) — a KPIRef with Time ==
// TimeRefTrailingN is a definition error (IssueInvalidExpression).
type KPIRef struct {
	Code       string       `json:"code"`
	Dimensions DimensionKey `json:"dimensions,omitempty"`
	Broadcast  bool         `json:"broadcast,omitempty"`
	Time       TimeRef      `json:"time,omitempty"`
	TrailingN  int          `json:"trailing_n,omitempty"`
}

// Expression is one node of a KPI's formula — task section 6. Exactly one
// of MetricRef/KPIRef/Constant is set when Op is OpMetric/OpKPI/
// OpConstant respectively (and Args is empty); for every other Op, Args
// holds the operator's operands and MetricRef/KPIRef/Constant are all nil.
// This shape (rather than a single interface{} payload) keeps Expression
// JSON-round-trippable with a fixed, predictable shape — see
// docs/KPI_ENGINE.md's "typed AST as V1 persistence contract" section.
type Expression struct {
	Op        Operator     `json:"op"`
	MetricRef *MetricRef   `json:"metric_ref,omitempty"`
	KPIRef    *KPIRef      `json:"kpi_ref,omitempty"`
	Constant  *float64     `json:"constant,omitempty"`
	Args      []Expression `json:"args,omitempty"`
}

// Metric builds a leaf Expression referencing a source metric — a small
// constructor convenience, not a second API surface (the returned value
// is an ordinary Expression, fully described by the struct fields above).
func Metric(ref MetricRef) Expression { return Expression{Op: OpMetric, MetricRef: &ref} }

// KPI builds a leaf Expression referencing another KPI.
func KPIExpr(ref KPIRef) Expression { return Expression{Op: OpKPI, KPIRef: &ref} }

// Const builds a leaf Expression for a fixed constant.
func Const(v float64) Expression { c := v; return Expression{Op: OpConstant, Constant: &c} }

// Binary builds a 2-arg Expression node.
func Binary(op Operator, a, b Expression) Expression {
	return Expression{Op: op, Args: []Expression{a, b}}
}

// Unary builds a 1-arg Expression node.
func Unary(op Operator, a Expression) Expression { return Expression{Op: op, Args: []Expression{a}} }

// Variadic builds an N-arg Expression node.
func Variadic(op Operator, args ...Expression) Expression { return Expression{Op: op, Args: args} }
