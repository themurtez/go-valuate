package kpi

// TraceArg is one resolved operand within a Trace — task section 25.
type TraceArg struct {
	// Label identifies what this operand was: "metric:<code>",
	// "kpi:<code>", or "constant".
	Label  string `json:"label"`
	Result Value  `json:"result"`
	Unit   Unit   `json:"unit"`
	// Trace is set only for a nested KPI-reference operand (recursion),
	// left nil for a metric/constant leaf.
	Trace *Trace `json:"trace,omitempty"`
}

// Trace is an optional, typed record of how one KPI's Value was computed
// — task section 25. Only populated when Options.IncludeTrace is set.
type Trace struct {
	Operator Operator   `json:"operator"`
	Args     []TraceArg `json:"args,omitempty"`
	Result   Value      `json:"result"`
}

// typedValue pairs a Value with the Unit it was computed under — every
// operator needs both to check compatibility and compute a result Unit —
// plus, when tracing is enabled, a display Label and (for a KPIRef
// operand only) the already-built child Trace.
type typedValue struct {
	value Value
	unit  Unit
	label string
	trace *Trace
}

func traceArgFrom(tv typedValue) TraceArg {
	return TraceArg{Label: tv.label, Result: tv.value, Unit: tv.unit, Trace: tv.trace}
}

// evaluateTyped is the single recursive evaluator for every Operator. req
// identifies the (period, dimension) group currently being evaluated;
// visiting guards KPI-of-KPI recursion (see evalContext.evaluateKPI).
func (ctx *evalContext) evaluateTyped(e Expression, req evalRequest, visiting map[string]bool) typedValue {
	switch e.Op {
	case OpMetric:
		v := ctx.evaluateMetricRef(*e.MetricRef, req)
		unit := ctx.metricUnit(*e.MetricRef, req)
		return typedValue{value: v, unit: unit, label: "metric:" + e.MetricRef.Code}
	case OpKPI:
		return ctx.evaluateKPIRefTyped(*e.KPIRef, req, visiting)
	case OpConstant:
		if e.Constant == nil || isNonFinite(*e.Constant) {
			return typedValue{value: unavailableValue(AvailabilityInvalidDefinition), unit: Unit{Kind: UnitUnitless}, label: "constant"}
		}
		return typedValue{value: availableValue(*e.Constant), unit: Unit{Kind: UnitUnitless}, label: "constant"}
	default:
		args := make([]typedValue, len(e.Args))
		for i, argExpr := range e.Args {
			args[i] = ctx.evaluateTyped(argExpr, req, visiting)
		}
		return evaluateOperator(e.Op, args)
	}
}

// metricUnit/kpiUnit look up the declared Unit for a reference, used for
// trace display and (for arithmetic operators combining two typedValues)
// actual compatibility checking. A missing metric has no declared unit
// (the zero Unit{}), which is simply incompatible with everything except
// another missing metric — the resulting AvailabilityUnitMismatch never
// actually fires in that case only because the operator's own
// arg.value.Available guard (see requireArgsAvailable in arithmetic.go)
// already reports AvailabilityMissingMetric/AvailabilitySourceUnavailable
// first, before unit compatibility is ever checked.
func (ctx *evalContext) metricUnit(ref MetricRef, req evalRequest) Unit {
	dim := resolveDimension(ref.Dimensions, ref.Broadcast, req.dim)
	period := req.period
	if ref.Time != TimeRefCurrent && ref.Time != TimeRefTrailingN {
		if p, ok := ctx.resolveTimeTarget(req.period, ref.Time); ok {
			period = p
		}
	}
	if m, ok := ctx.metrics.lookup(ref.Code, period, dim); ok {
		return m.Unit
	}
	return Unit{}
}

func (ctx *evalContext) kpiUnit(code string) Unit {
	if def, ok := ctx.graph.byCode[code]; ok {
		return def.Unit
	}
	return Unit{}
}

// evaluateMetricRef resolves one METRIC leaf against the metric index,
// handling Dimensions/Broadcast (dimension.go) and Time/TrailingN
// (period.go, expression.go).
func (ctx *evalContext) evaluateMetricRef(ref MetricRef, req evalRequest) Value {
	dim := resolveDimension(ref.Dimensions, ref.Broadcast, req.dim)

	if ref.Time == TimeRefTrailingN {
		codes, ok := ctx.trailingWindow(req.period, ref.TrailingN)
		if !ok {
			return unavailableValue(AvailabilityPeriodUnavailable)
		}
		return ctx.aggregateAcrossPeriods(ref.Code, dim, codes)
	}

	period := req.period
	if ref.Time != TimeRefCurrent {
		p, ok := ctx.resolveTimeTarget(req.period, ref.Time)
		if !ok {
			return unavailableValue(AvailabilityPeriodUnavailable)
		}
		period = p
	}

	m, ok := ctx.metrics.lookup(ref.Code, period, dim)
	if !ok {
		if !dim.IsBusinessLevel() {
			return unavailableValue(AvailabilityDimensionUnavailable)
		}
		return unavailableValue(AvailabilityMissingMetric)
	}
	if !m.Available {
		return unavailableValue(AvailabilitySourceUnavailable)
	}
	if isNonFinite(m.Value) {
		return unavailableValue(AvailabilityNonFiniteResult)
	}
	return availableValue(m.Value)
}

// evaluateKPIRefTyped resolves one KPI leaf, recursing through
// evaluateKPI and carrying through that call's own (possibly cached)
// Trace when tracing is enabled — so a KPIRef operand's Trace is never
// rebuilt from scratch, only reused.
func (ctx *evalContext) evaluateKPIRefTyped(ref KPIRef, req evalRequest, visiting map[string]bool) typedValue {
	dim := resolveDimension(ref.Dimensions, ref.Broadcast, req.dim)
	period := req.period
	if ref.Time != TimeRefCurrent {
		p, ok := ctx.resolveTimeTarget(req.period, ref.Time)
		if !ok {
			return typedValue{value: unavailableValue(AvailabilityPeriodUnavailable), unit: ctx.kpiUnit(ref.Code), label: "kpi:" + ref.Code}
		}
		period = p
	}
	res := ctx.evaluateKPI(ref.Code, evalRequest{period: period, dim: dim}, visiting)
	return typedValue{value: res.value, unit: res.unit, label: "kpi:" + ref.Code, trace: res.trace}
}
