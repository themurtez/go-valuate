package kpi

// evaluateOperator applies op to already-evaluated args and returns the
// combined typedValue — task section 8's "define exact semantics for
// every operator" instruction. Arity is assumed already valid
// (validateExpression checks this at definition time); this function
// still defends against a wrong count defensively by treating it as
// AvailabilityInvalidDefinition rather than panicking or indexing out of
// range.
func evaluateOperator(op Operator, args []typedValue) typedValue {
	switch op {
	case OpAdd:
		return combineTwo(args, combineAdditive, func(a, b float64) float64 { return a + b })
	case OpSubtract:
		return combineTwo(args, combineAdditive, func(a, b float64) float64 { return a - b })
	case OpMultiply:
		return combineTwo(args, combineMultiplicative, func(a, b float64) float64 { return a * b })
	case OpDivide:
		return divideTwo(args, false)
	case OpPercent:
		return percentOp(args)
	case OpPercentChange:
		return percentChangeOp(args)
	case OpNegate:
		return unaryOp(args, func(a float64) float64 { return -a })
	case OpAbs:
		return unaryOp(args, absFloat)
	case OpMin:
		return extremum(args, true)
	case OpMax:
		return extremum(args, false)
	case OpSum:
		return sumOp(args)
	case OpAverage:
		return averageOp(args)
	case OpWeightedAverage:
		return weightedAverageOp(args)
	case OpCoalesce:
		return coalesceOp(args)
	default:
		return typedValue{value: unavailableValue(AvailabilityInvalidDefinition)}
	}
}

func guardResult(v float64) Value {
	if isNonFinite(v) {
		return unavailableValue(AvailabilityNonFiniteResult)
	}
	return availableValue(v)
}

// requireArgsAvailable reports the first unavailable arg's Value (with its
// own Reason propagated, since a missing METRIC/KPI's reason is more
// specific/useful than a generic one), or ok=true if every arg is
// available and finite.
func requireArgsAvailable(args []typedValue) (Value, bool) {
	for _, a := range args {
		if !a.value.Available {
			return a.value, false
		}
		if isNonFinite(a.value.Amount) {
			return unavailableValue(AvailabilityNonFiniteResult), false
		}
	}
	return Value{}, true
}

func combineTwo(args []typedValue, unitFn func(a, b Unit) unitCompatibility, mathFn func(a, b float64) float64) typedValue {
	if len(args) != 2 {
		return typedValue{value: unavailableValue(AvailabilityInvalidDefinition)}
	}
	if v, ok := requireArgsAvailable(args); !ok {
		return typedValue{value: v}
	}
	compat := unitFn(args[0].unit, args[1].unit)
	if !compat.compatible {
		reason := AvailabilityUnitMismatch
		if compat.currencyMismatch {
			reason = AvailabilityCurrencyMismatch
		}
		return typedValue{value: unavailableValue(reason)}
	}
	return typedValue{value: guardResult(mathFn(args[0].value.Amount, args[1].value.Amount)), unit: compat.result}
}

func divideTwo(args []typedValue, forcePercent bool) typedValue {
	if len(args) != 2 {
		return typedValue{value: unavailableValue(AvailabilityInvalidDefinition)}
	}
	if v, ok := requireArgsAvailable(args); !ok {
		return typedValue{value: v}
	}
	if args[1].value.Amount == 0 {
		return typedValue{value: unavailableValue(AvailabilityDivideByZero)}
	}
	compat := combineDivisive(args[0].unit, args[1].unit)
	if !compat.compatible {
		reason := AvailabilityUnitMismatch
		if compat.currencyMismatch {
			reason = AvailabilityCurrencyMismatch
		}
		return typedValue{value: unavailableValue(reason)}
	}
	return typedValue{value: guardResult(args[0].value.Amount / args[1].value.Amount), unit: compat.result}
}

// percentOp implements OpPercent — Args[0]/Args[1]*100, requiring the
// divisive result Unit to specifically be UnitRatio (see OpPercent's doc
// comment in expression.go).
func percentOp(args []typedValue) typedValue {
	if len(args) != 2 {
		return typedValue{value: unavailableValue(AvailabilityInvalidDefinition)}
	}
	if v, ok := requireArgsAvailable(args); !ok {
		return typedValue{value: v}
	}
	if args[1].value.Amount == 0 {
		return typedValue{value: unavailableValue(AvailabilityDivideByZero)}
	}
	compat := combineDivisive(args[0].unit, args[1].unit)
	if !compat.compatible || compat.result.Kind != UnitRatio {
		reason := AvailabilityUnitMismatch
		if compat.currencyMismatch {
			reason = AvailabilityCurrencyMismatch
		}
		return typedValue{value: unavailableValue(reason)}
	}
	return typedValue{value: guardResult(args[0].value.Amount / args[1].value.Amount * 100), unit: Unit{Kind: UnitPercent}}
}

// percentChangeOp implements OpPercentChange — (current-prior)/prior*100,
// a relative percent change (task section 13).
func percentChangeOp(args []typedValue) typedValue {
	if len(args) != 2 {
		return typedValue{value: unavailableValue(AvailabilityInvalidDefinition)}
	}
	if v, ok := requireArgsAvailable(args); !ok {
		return typedValue{value: v}
	}
	if !args[0].unit.Equal(args[1].unit) {
		reason := AvailabilityUnitMismatch
		if args[0].unit.Kind == UnitCurrency && args[1].unit.Kind == UnitCurrency {
			reason = AvailabilityCurrencyMismatch
		}
		return typedValue{value: unavailableValue(reason)}
	}
	prior := args[1].value.Amount
	if prior == 0 {
		return typedValue{value: unavailableValue(AvailabilityDivideByZero)}
	}
	current := args[0].value.Amount
	return typedValue{value: guardResult((current - prior) / prior * 100), unit: Unit{Kind: UnitPercent}}
}

func unaryOp(args []typedValue, fn func(float64) float64) typedValue {
	if len(args) != 1 {
		return typedValue{value: unavailableValue(AvailabilityInvalidDefinition)}
	}
	if v, ok := requireArgsAvailable(args); !ok {
		return typedValue{value: v}
	}
	return typedValue{value: guardResult(fn(args[0].value.Amount)), unit: args[0].unit}
}

// extremum implements MIN/MAX. Every arg must share the same Unit (exact
// match, since these compare magnitudes directly).
func extremum(args []typedValue, wantMin bool) typedValue {
	if len(args) < 2 {
		return typedValue{value: unavailableValue(AvailabilityInvalidDefinition)}
	}
	if v, ok := requireArgsAvailable(args); !ok {
		return typedValue{value: v}
	}
	unit := args[0].unit
	best := args[0].value.Amount
	for _, a := range args[1:] {
		if !a.unit.Equal(unit) {
			reason := AvailabilityUnitMismatch
			if a.unit.Kind == UnitCurrency && unit.Kind == UnitCurrency {
				reason = AvailabilityCurrencyMismatch
			}
			return typedValue{value: unavailableValue(reason)}
		}
		if (wantMin && a.value.Amount < best) || (!wantMin && a.value.Amount > best) {
			best = a.value.Amount
		}
	}
	return typedValue{value: guardResult(best), unit: unit}
}

// sumOp implements SUM: every arg must share the same Unit.
func sumOp(args []typedValue) typedValue {
	if len(args) < 1 {
		return typedValue{value: unavailableValue(AvailabilityInvalidDefinition)}
	}
	if v, ok := requireArgsAvailable(args); !ok {
		return typedValue{value: v}
	}
	unit := args[0].unit
	total := 0.0
	for _, a := range args {
		if !a.unit.Equal(unit) {
			reason := AvailabilityUnitMismatch
			if a.unit.Kind == UnitCurrency && unit.Kind == UnitCurrency {
				reason = AvailabilityCurrencyMismatch
			}
			return typedValue{value: unavailableValue(reason)}
		}
		total += a.value.Amount
	}
	return typedValue{value: guardResult(total), unit: unit}
}

// averageOp implements AVERAGE: the arithmetic mean, never a silent
// SUM-then-divide-by-len that could hide a units mismatch.
func averageOp(args []typedValue) typedValue {
	sum := sumOp(args)
	if !sum.value.Available {
		return sum
	}
	return typedValue{value: guardResult(sum.value.Amount / float64(len(args))), unit: sum.unit}
}

// weightedAverageOp implements WEIGHTED_AVERAGE, reading args as
// (value, weight) pairs — task section 37.
func weightedAverageOp(args []typedValue) typedValue {
	if len(args) < 2 || len(args)%2 != 0 {
		return typedValue{value: unavailableValue(AvailabilityInvalidDefinition)}
	}
	if v, ok := requireArgsAvailable(args); !ok {
		return typedValue{value: v}
	}
	valueUnit := args[0].unit
	numerator := 0.0
	totalWeight := 0.0
	for i := 0; i < len(args); i += 2 {
		value, weight := args[i], args[i+1]
		if !value.unit.Equal(valueUnit) {
			reason := AvailabilityUnitMismatch
			if value.unit.Kind == UnitCurrency && valueUnit.Kind == UnitCurrency {
				reason = AvailabilityCurrencyMismatch
			}
			return typedValue{value: unavailableValue(reason)}
		}
		if !isScalarKind(weight.unit.Kind) && weight.unit.Kind != UnitCount && weight.unit.Kind != UnitQuantity {
			return typedValue{value: unavailableValue(AvailabilityUnitMismatch)}
		}
		numerator += value.value.Amount * weight.value.Amount
		totalWeight += weight.value.Amount
	}
	if totalWeight == 0 {
		return typedValue{value: unavailableValue(AvailabilityDivideByZero)}
	}
	return typedValue{value: guardResult(numerator / totalWeight), unit: valueUnit}
}

// coalesceOp implements COALESCE: the first Available arg, unit taken
// from that same arg (task section 10 — must be explicit in the
// Definition; this package never applies COALESCE semantics implicitly
// anywhere else).
func coalesceOp(args []typedValue) typedValue {
	if len(args) < 1 {
		return typedValue{value: unavailableValue(AvailabilityInvalidDefinition)}
	}
	for _, a := range args {
		if a.value.Available && !isNonFinite(a.value.Amount) {
			return a
		}
	}
	return typedValue{value: unavailableValue(AvailabilityMissingMetric)}
}
