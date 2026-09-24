package kpi

import "math"

// amountTolerance is the floating-point comparison tolerance used
// throughout this package — mirrors accounting/vendorspend.amountTolerance
// and every sibling package's identical constant.
const amountTolerance = 0.005

func isNonFinite(v float64) bool {
	return math.IsNaN(v) || math.IsInf(v, 0)
}

// metricIndex is a pre-built, read-only lookup over every valid
// MetricValue, keyed by (code, period, canonical dimension hash) — task
// section 46's "index source metrics once" instruction, avoiding an
// O(KPIs x metrics) rescan.
type metricIndex struct {
	byKey map[string]MetricValue
	// byCodeDimension groups every period's MetricValue for one (code,
	// dimension) pair, used by trailing-N/aggregation lookups that need
	// every period, not just one.
	byCodeDimension map[string][]MetricValue
	// byCodeBusinessLevel isolates the business-level (no-dimension)
	// MetricValues for one code, used by Broadcast resolution.
	byCodeBusinessLevel map[string][]MetricValue
}

func buildMetricIndex(metrics []MetricValue) (metricIndex, []EvaluationIssue) {
	idx := metricIndex{
		byKey:               make(map[string]MetricValue, len(metrics)),
		byCodeDimension:     make(map[string][]MetricValue),
		byCodeBusinessLevel: make(map[string][]MetricValue),
	}
	var issues []EvaluationIssue
	for _, m := range metrics {
		key := m.sourceKey()
		if _, exists := idx.byKey[key]; exists {
			issues = append(issues, EvaluationIssue{Code: IssueDuplicateMetricValue, Severity: SeverityError,
				MetricCode: m.Code, Period: m.Period, Dimensions: m.Dimensions,
				Message: "duplicate MetricValue for code \"" + m.Code + "\", period \"" + m.Period + "\", and dimension key"})
			continue // first occurrence wins; never pick first/last "arbitrarily" -- explicit rule: first wins
		}
		// A structurally invalid declared Unit (e.g. UnitCurrency with an
		// empty CurrencyCode, UnitCustom with an empty CustomLabel) can
		// never safely participate in combineAdditive/
		// combineMultiplicative/combineDivisive's compatibility checks —
		// those functions compare Units for equality/combination
		// assuming both sides are already well-formed, and a malformed
		// Unit sailing through silently could otherwise produce a
		// KPIResult whose own Unit is equally malformed (found via
		// fuzzing: FuzzUnitCompatibility). Only checked when Available is
		// true — an unavailable MetricValue's Unit never reaches any
		// arithmetic regardless, so a caller supplying a genuinely
		// zero-value MetricValue{} for "this metric doesn't exist here"
		// is not penalized for also leaving Unit unset.
		if m.Available && !m.Unit.Valid() {
			issues = append(issues, EvaluationIssue{Code: IssueInvalidUnit, Severity: SeverityError,
				MetricCode: m.Code, Period: m.Period, Dimensions: m.Dimensions,
				Message: "MetricValue for code \"" + m.Code + "\" has a structurally invalid Unit"})
			idx.byKey[key] = MetricValue{Code: m.Code, Period: m.Period, Dimensions: m.Dimensions} // unavailable placeholder
			continue
		}
		idx.byKey[key] = m
		cdKey := m.Code + "\x00" + m.Dimensions.hashKey()
		idx.byCodeDimension[cdKey] = append(idx.byCodeDimension[cdKey], m)
		if m.Dimensions.IsBusinessLevel() {
			idx.byCodeBusinessLevel[m.Code] = append(idx.byCodeBusinessLevel[m.Code], m)
		}
	}
	return idx, issues
}

func (idx metricIndex) lookup(code, period string, dim DimensionKey) (MetricValue, bool) {
	key := code + "\x00" + period + "\x00" + dim.hashKey()
	m, ok := idx.byKey[key]
	return m, ok
}

// evalContext carries everything one Evaluate/evaluateExpression pass
// needs, built once per Calculate call and read-only thereafter.
type evalContext struct {
	metrics       metricIndex
	periodsByCode map[string]Period
	periodOrder   []Period // sorted by Sequence
	// periodIndexByCode maps each Period.Code to its position within
	// periodOrder, built once so resolveTimeTarget's
	// TimeRefPriorPeriod/TimeRefPriorYearSamePeriod cases are O(1)/O(k)
	// (k = fiscal years back) index lookups instead of an O(len(periods))
	// linear scan repeated on every call — found via profiling
	// (pprof) to dominate a 100-KPI x 1,000-period benchmark's CPU time,
	// since every KPIResult unconditionally computes Change against
	// PRIOR_PERIOD (see calculate.go), making the old scan run
	// KPIs x periods times over an up-to-periods-long slice each time.
	periodIndexByCode map[string]int
	graph             dependencyGraph
	includeTrace      bool
	// kpiCache memoizes each (kpiCode, period, dimension) result within
	// one Calculate call — task section 46's "avoid rescanning" goal
	// extended to KPI-of-KPI references, so a KPI referenced by several
	// siblings is computed once per (period, dimension) rather than once
	// per reference.
	kpiCache map[string]kpiEvalResult
	// provenanceCache memoizes buildProvenance's result per (kpiCode,
	// period, dimension) — without this, a KPI-of-KPI chain of length N
	// produces O(N^2) total provenance-walk work (each of the N
	// KPIResults independently re-walking its own O(N)-deep dependency
	// chain from scratch), found via BenchmarkDependencyDepth's
	// superlinear scaling. Mirrors kpiCache's identical memoization
	// shape/rationale.
	provenanceCache map[string]Provenance
}

type kpiEvalResult struct {
	value Value
	unit  Unit
	trace *Trace
}

// evalRequest identifies one (KPI or metric-only) evaluation target.
type evalRequest struct {
	period string
	dim    DimensionKey
}

func (ctx *evalContext) kpiCacheKey(code string, req evalRequest) string {
	return code + "\x00" + req.period + "\x00" + req.dim.hashKey()
}

// evaluateKPI resolves Definition code for req, memoized. visiting guards
// against runaway recursion in a way that should never trigger in
// practice (cycles are already excluded from graph.order before
// evaluation begins) but is kept as defense in depth.
func (ctx *evalContext) evaluateKPI(code string, req evalRequest, visiting map[string]bool) kpiEvalResult {
	cacheKey := ctx.kpiCacheKey(code, req)
	if cached, ok := ctx.kpiCache[cacheKey]; ok {
		return cached
	}
	if ctx.graph.inCycle[code] {
		res := kpiEvalResult{value: unavailableValue(AvailabilityDependencyUnavailable)}
		ctx.kpiCache[cacheKey] = res
		return res
	}
	def, ok := ctx.graph.byCode[code]
	if !ok || visiting[code] {
		res := kpiEvalResult{value: unavailableValue(AvailabilityInvalidDefinition)}
		ctx.kpiCache[cacheKey] = res
		return res
	}
	visiting[code] = true
	tv := ctx.evaluateTyped(def.Formula, req, visiting)
	delete(visiting, code)

	res := kpiEvalResult{value: tv.value, unit: tv.unit}
	if ctx.includeTrace {
		res.trace = &Trace{Operator: def.Formula.Op, Result: tv.value}
		switch def.Formula.Op {
		case OpMetric, OpConstant:
			// Leaf node with no operand structure of its own -- Args
			// stays empty, Result alone is meaningful.
		case OpKPI:
			// def.Formula is itself a bare KPI reference: this KPI's own
			// trace is a one-arg wrapper around the referenced KPI's
			// already-built trace (tv.trace), so this KPI's own root
			// Operator (KPI) is never lost by being overwritten with the
			// referenced KPI's operator.
			res.trace.Args = []TraceArg{traceArgFrom(tv)}
		default:
			res.trace.Args = ctx.traceArgsFor(def.Formula, req, visiting)
		}
	}
	ctx.kpiCache[cacheKey] = res
	return res
}

// traceArgsFor rebuilds the TraceArg list for e's immediate Args, reusing
// the memoized kpiCache for any KPIRef operand so this second walk (done
// only when ctx.includeTrace) costs nothing extra for metric-heavy
// formulas and at most a cache hit for KPI-heavy ones.
func (ctx *evalContext) traceArgsFor(e Expression, req evalRequest, visiting map[string]bool) []TraceArg {
	args := make([]TraceArg, 0, len(e.Args))
	for _, argExpr := range e.Args {
		tv := ctx.evaluateTyped(argExpr, req, visiting)
		args = append(args, traceArgFrom(tv))
	}
	return args
}

// resolveTimeTarget returns the actual period Code + ok for ref's Time
// relative to base — task section 20. Uses ctx.periodIndexByCode (base's
// position within the Sequence-sorted ctx.periodOrder) rather than a
// linear scan: TimeRefPriorPeriod is then a direct index-1 lookup, and
// TimeRefPriorYearSamePeriod walks backward from that index (nearest
// candidates checked first, so the FIRST match found while walking
// backward is correctly the NEAREST prior fiscal year with the same
// position — walking forward from index 0, as an earlier version of
// this function did, would incorrectly return the EARLIEST matching
// fiscal year instead when more than one shares the same
// PositionInYear; found via a targeted probe test after profiling
// surfaced this function as a performance hotspot and prompted a closer
// correctness read of its TimeRefPriorYearSamePeriod branch).
func (ctx *evalContext) resolveTimeTarget(base string, timeRef TimeRef) (string, bool) {
	idx, ok := ctx.periodIndexByCode[base]
	if !ok {
		return "", false
	}
	baseP := ctx.periodOrder[idx]
	switch timeRef {
	case TimeRefCurrent:
		return base, true
	case TimeRefPriorPeriod:
		if idx == 0 {
			return "", false
		}
		return ctx.periodOrder[idx-1].Code, true
	case TimeRefPriorYearSamePeriod:
		key, ok := baseP.samePositionKey()
		if !ok {
			return "", false
		}
		for i := idx - 1; i >= 0; i-- {
			p := ctx.periodOrder[i]
			pk, ok := p.samePositionKey()
			if !ok || pk[1] != key[1] {
				continue
			}
			// Walking backward from idx-1: the first match is the
			// nearest prior fiscal year sharing this position.
			return p.Code, true
		}
		return "", false
	default:
		return "", false
	}
}

// trailingWindow returns the N period Codes (chronological, ending at and
// including base) for TimeRefTrailingN, or ok=false if fewer than N
// periods exist at or before base. Uses ctx.periodIndexByCode for a
// direct slice-window computation instead of the O(len(periods))
// candidates scan an earlier version of this function used — same
// rationale as resolveTimeTarget's identical fix.
func (ctx *evalContext) trailingWindow(base string, n int) ([]string, bool) {
	idx, ok := ctx.periodIndexByCode[base]
	if !ok || n <= 0 {
		return nil, false
	}
	if idx+1 < n {
		return nil, false
	}
	start := idx + 1 - n
	codes := make([]string, n)
	for i := 0; i < n; i++ {
		codes[i] = ctx.periodOrder[start+i].Code
	}
	return codes, true
}

// resolveDimension applies MetricRef/KPIRef-style Dimensions/Broadcast
// against the group currently being evaluated (evalGroup) — task section
// 5's strict-exact-match-unless-explicit-broadcast rule.
func resolveDimension(refDim DimensionKey, broadcast bool, evalGroup DimensionKey) DimensionKey {
	if len(refDim.canonicalize()) > 0 {
		return refDim
	}
	if broadcast {
		return DimensionKey{}
	}
	return evalGroup
}

// checkExpectedOutputUnit compares a KPI's actually-produced unit
// (resolvedUnit, from evaluating its formula root — see evaluateTyped)
// against its Definition.Unit. When the KPI's own value is unavailable,
// there is nothing to check (the unit question is moot). When the
// resolved unit could not be determined at all (e.g. a KPI whose root
// operator does not carry a meaningful Unit for some Args shape), no
// mismatch is reported — this check only fires on a POSITIVE, resolved
// mismatch, never on "we don't know."
func checkExpectedOutputUnit(declared, resolved Unit, value Value) (mismatch bool, currencyMismatch bool) {
	if !value.Available {
		return false, false
	}
	if !declared.Valid() || resolved == (Unit{}) {
		return false, false
	}
	if declared.Equal(resolved) {
		return false, false
	}
	if declared.Kind == UnitCurrency && resolved.Kind == UnitCurrency {
		return true, true
	}
	return true, false
}
