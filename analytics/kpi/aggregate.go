package kpi

// aggregateAcrossPeriods rolls up metric code's values across periodCodes
// (already exact dim) using its own declared MetricValue.Aggregation —
// task sections 17/19/20's "if caller rolls monthly metrics into
// quarter/year, require explicit source aggregation semantics" rule. Used
// by TimeRefTrailingN (evaluate.go). A metric whose values across the
// window have inconsistent Aggregation declarations (should not happen if
// a caller is consistent, but is not assumed) uses the first found
// window-point's declared rule.
func (ctx *evalContext) aggregateAcrossPeriods(code string, dim DimensionKey, periodCodes []string) Value {
	var points []MetricValue
	for _, p := range periodCodes {
		if m, ok := ctx.metrics.lookup(code, p, dim); ok {
			points = append(points, m)
		}
	}
	if len(points) == 0 {
		return unavailableValue(AvailabilityMissingMetric)
	}
	rule := resolvedAggregation(points[0].Aggregation)
	return aggregateMetricValues(points, rule)
}

// aggregateMetricValues applies rule to vals — the one shared
// aggregation-math implementation used both by TRAILING_N period rollup
// (aggregateAcrossPeriods above) and by any future dimension-rollup
// caller (kept as one function so the two never drift — task section 18's
// "if aggregation across dimensions is supported, require explicit
// aggregation semantics" reuses the exact same rule set).
//
// AggregationNotAggregatable always returns unavailable — task section
// 17's explicit "Margin % -> NOT_AGGREGATABLE" example; a caller wanting
// a rolled-up ratio must compose it from SUM-aggregatable numerator/
// denominator metrics via a DIVIDE expression instead (see Definition
// examples in fixtures and docs/KPI_ENGINE.md).
func aggregateMetricValues(vals []MetricValue, rule AggregationRule) Value {
	var available []MetricValue
	for _, v := range vals {
		if v.Available && !isNonFinite(v.Value) {
			available = append(available, v)
		}
	}
	if len(available) == 0 {
		return unavailableValue(AvailabilityMissingMetric)
	}
	switch rule {
	case AggregationSum:
		total := 0.0
		for _, v := range available {
			total += v.Value
		}
		return guardResult(total)
	case AggregationAverage:
		total := 0.0
		for _, v := range available {
			total += v.Value
		}
		return guardResult(total / float64(len(available)))
	case AggregationLast:
		return guardResult(available[len(available)-1].Value)
	case AggregationMin:
		best := available[0].Value
		for _, v := range available[1:] {
			if v.Value < best {
				best = v.Value
			}
		}
		return guardResult(best)
	case AggregationMax:
		best := available[0].Value
		for _, v := range available[1:] {
			if v.Value > best {
				best = v.Value
			}
		}
		return guardResult(best)
	case AggregationWeightedAverage:
		// Weighted-average rollup of a bare metric series (as opposed to
		// the explicit-Expression WEIGHTED_AVERAGE operator) has no
		// caller-supplied weight source at this level — an
		// AggregationWeightedAverage-tagged MetricValue rolled up via
		// TRAILING_N without an explicit weight metric is unavailable
		// rather than silently falling back to a plain average, matching
		// task section 37's "weighted average requires explicit values +
		// weights" instruction. A caller wanting this uses the explicit
		// WEIGHTED_AVERAGE Expression operator instead, supplying its own
		// weight metric references.
		return unavailableValue(AvailabilityNotApplicable)
	default: // AggregationNotAggregatable
		return unavailableValue(AvailabilityNotApplicable)
	}
}
