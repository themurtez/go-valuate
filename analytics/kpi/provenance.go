package kpi

import "sort"

// buildProvenance walks def's formula (transitively through KPI
// dependencies) and collects every distinct source metric code, KPI
// dependency code, and resolved SourceRef actually touched for req — task
// section 25's "even without full trace, preserve SourceMetricCodes/
// DependencyKPICodes/SourceRefs" instruction. This is a separate,
// lightweight walk from evaluateTyped's arithmetic evaluation (rather
// than accumulated as a side effect of it) so Provenance is always
// populated regardless of Options.IncludeTrace.
//
// Memoized via ctx.provenanceCache, keyed like ctx.kpiCache (code,
// period, dimension) — without this, a KPI-of-KPI chain of length N
// costs O(N^2) total (see provenanceCache's doc comment); with it, each
// KPI's own provenance is computed at most once and reused by every
// result/dependent that needs it.
func buildProvenance(ctx *evalContext, code string, req evalRequest) Provenance {
	cacheKey := ctx.kpiCacheKey(code, req)
	if cached, ok := ctx.provenanceCache[cacheKey]; ok {
		return cached
	}
	// A cyclic KPI (ctx.graph.inCycle) never gets a real Provenance —
	// evaluateKPI already reports its Value as
	// AvailabilityDependencyUnavailable without descending into its
	// Formula at all, so Provenance mirrors that same "structurally
	// excluded" treatment. This ALSO breaks what would otherwise be
	// infinite recursion here: A -> B -> C -> A each calling
	// buildProvenance on the next link before any of the three has
	// finished populating its own cache entry.
	if ctx.graph.inCycle[code] {
		ctx.provenanceCache[cacheKey] = Provenance{}
		return Provenance{}
	}

	metricSet := map[string]bool{}
	kpiSet := map[string]bool{}
	refSet := map[string]bool{}
	collectProvenance(ctx, code, req, metricSet, kpiSet, refSet)

	p := Provenance{
		SourceMetricCodes:  sortedSetKeys(metricSet),
		DependencyKPICodes: sortedSetKeys(kpiSet),
		SourceRefs:         sortedSetKeys(refSet),
	}
	ctx.provenanceCache[cacheKey] = p
	return p
}

// collectProvenance walks code's own Formula (one level), folding in the
// ALREADY-MEMOIZED provenance of any KPIRef it touches via buildProvenance
// itself (rather than a raw unmemoized recursive descent) — this is what
// keeps the overall walk linear in the size of the dependency graph
// instead of linear in each individual chain's depth.
func collectProvenance(ctx *evalContext, code string, req evalRequest, metricSet, kpiSet, refSet map[string]bool) {
	def, ok := ctx.graph.byCode[code]
	if !ok {
		return
	}
	collectExpressionProvenance(ctx, def.Formula, req, metricSet, kpiSet, refSet, 1)
}

func collectExpressionProvenance(ctx *evalContext, e Expression, req evalRequest, metricSet, kpiSet, refSet map[string]bool, depth int) {
	if depth > MaxExpressionDepth+1 {
		return
	}
	switch e.Op {
	case OpMetric:
		if e.MetricRef == nil {
			return
		}
		metricSet[e.MetricRef.Code] = true
		dim := resolveDimension(e.MetricRef.Dimensions, e.MetricRef.Broadcast, req.dim)
		period := req.period
		if e.MetricRef.Time != TimeRefCurrent && e.MetricRef.Time != TimeRefTrailingN {
			if p, ok := ctx.resolveTimeTarget(req.period, e.MetricRef.Time); ok {
				period = p
			}
		}
		if m, ok := ctx.metrics.lookup(e.MetricRef.Code, period, dim); ok && m.SourceRef != "" {
			refSet[m.SourceRef] = true
		}
		return
	case OpKPI:
		if e.KPIRef == nil {
			return
		}
		kpiSet[e.KPIRef.Code] = true
		dim := resolveDimension(e.KPIRef.Dimensions, e.KPIRef.Broadcast, req.dim)
		period := req.period
		if e.KPIRef.Time != TimeRefCurrent {
			if p, ok := ctx.resolveTimeTarget(req.period, e.KPIRef.Time); ok {
				period = p
			}
		}
		// Reuse the referenced KPI's OWN memoized provenance rather than
		// recursively re-walking its formula tree — this is the one
		// change that turns an O(chain-length) per-KPIResult cost into
		// O(1) amortized (buildProvenance's own cache) for every KPI
		// beyond the first in a chain.
		childProvenance := buildProvenance(ctx, e.KPIRef.Code, evalRequest{period: period, dim: dim})
		for _, mc := range childProvenance.SourceMetricCodes {
			metricSet[mc] = true
		}
		for _, kc := range childProvenance.DependencyKPICodes {
			kpiSet[kc] = true
		}
		for _, ref := range childProvenance.SourceRefs {
			refSet[ref] = true
		}
		return
	case OpConstant:
		return
	}
	for _, arg := range e.Args {
		collectExpressionProvenance(ctx, arg, req, metricSet, kpiSet, refSet, depth+1)
	}
}

func sortedSetKeys(m map[string]bool) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
