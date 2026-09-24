package kpi

import "sort"

// Calculate derives a full Result from in under opts. It never mutates
// in.Definitions/in.Metrics/in.Periods or any Expression/MetricValue/
// Period/DimensionKey/TargetPolicy/ThresholdBand reachable from them —
// see immutability_test.go. It performs no I/O and never calls
// time.Now().
func Calculate(in Input, opts Options) Result {
	result := Result{
		SchemaVersion:             SchemaVersion,
		FormulaVersion:            FormulaVersion,
		ExpressionLanguageVersion: ExpressionLanguageVersion,
	}
	result.Coverage.DefinitionsSupplied = len(in.Definitions)

	metricCodes := make(map[string]bool)
	for _, m := range in.Metrics {
		metricCodes[m.Code] = true
	}

	validDefs, defIssues := validateDefinitions(in.Definitions, opts.ExpressionLanguageVersion)
	graph, graphIssues := buildDependencyGraph(validDefs, metricCodes)
	defIssues = append(defIssues, graphIssues...)

	// Any KPI named in a graph issue that isn't a structural
	// validate-time rejection (e.g. IssueUnknownKPI, IssueDependencyCycle,
	// IssueNamespaceCollision, IssueDependencyTooDeep, IssueDependencyCycle
	// for self-reference) still remains "valid" for Coverage purposes if it
	// passed validateDefinitions — dependency-graph-level exclusion from
	// evaluation is tracked separately via graph.inCycle, not by removing
	// it from validDefs, so a caller inspecting Coverage sees the true
	// definition-validity count while evaluation-exclusion is visible via
	// AvailabilityInvalidDefinition/AvailabilityDependencyUnavailable on
	// the affected KPIResults.
	result.Coverage.DefinitionsValid = len(validDefs)
	result.Coverage.DefinitionsInvalid = len(in.Definitions) - len(validDefs)

	sortDefinitionIssues(defIssues)
	result.DefinitionIssues = defIssues

	if opts.FailAllOnDefinitionError && HasDefinitionErrors(defIssues) {
		return result
	}

	metricIdx, metricIssues := buildMetricIndex(in.Metrics)

	periodsByCode := make(map[string]Period, len(in.Periods))
	var periodIssues []EvaluationIssue
	seenPeriod := map[string]bool{}
	seenSequence := map[int]bool{}
	var periodOrder []Period
	for _, p := range in.Periods {
		if p.Code == "" {
			periodIssues = append(periodIssues, EvaluationIssue{Code: IssueInvalidPeriod, Severity: SeverityError,
				Message: "a Period has an empty Code"})
			continue
		}
		if seenPeriod[p.Code] {
			periodIssues = append(periodIssues, EvaluationIssue{Code: IssueInvalidPeriod, Severity: SeverityError, Period: p.Code,
				Message: "duplicate period code \"" + p.Code + "\"; only the first occurrence is used"})
			continue
		}
		if seenSequence[p.Sequence] {
			periodIssues = append(periodIssues, EvaluationIssue{Code: IssueInvalidPeriod, Severity: SeverityError, Period: p.Code,
				Message: "period \"" + p.Code + "\" has a Sequence that ties with another period; Sequence must be strictly increasing"})
			continue
		}
		seenPeriod[p.Code] = true
		seenSequence[p.Sequence] = true
		periodsByCode[p.Code] = p
		periodOrder = append(periodOrder, p)
	}
	sort.Slice(periodOrder, func(i, j int) bool { return periodOrder[i].Sequence < periodOrder[j].Sequence })
	periodIndexByCode := make(map[string]int, len(periodOrder))
	for i, p := range periodOrder {
		periodIndexByCode[p.Code] = i
	}

	evalPeriods := resolveEvalPeriods(opts.Periods, periodOrder, &periodIssues)
	evalDimensions := resolveEvalDimensions(opts.Dimensions)

	result.Coverage.PeriodsEvaluated = len(evalPeriods)
	result.Coverage.DimensionGroupsEvaluated = len(evalDimensions)

	ctx := &evalContext{
		metrics:           metricIdx,
		periodsByCode:     periodsByCode,
		periodOrder:       periodOrder,
		periodIndexByCode: periodIndexByCode,
		graph:             graph,
		includeTrace:      opts.IncludeTrace,
		kpiCache:          make(map[string]kpiEvalResult),
		provenanceCache:   make(map[string]Provenance),
	}

	var kpiResults []KPIResult
	missingMetricSet := map[string]bool{}
	var evalIssues []EvaluationIssue

	for _, code := range graph.evaluationOrder {
		def := graph.byCode[code]
		for _, period := range evalPeriods {
			for _, dim := range evalDimensions {
				req := evalRequest{period: period, dim: dim}
				res := ctx.evaluateKPI(code, req, map[string]bool{})

				kr := KPIResult{
					Code:       code,
					Period:     period,
					Dimensions: dim.canonicalize(),
					Value:      res.value,
					Unit:       def.Unit,
				}

				if mismatch, currencyMismatch := checkExpectedOutputUnit(def.Unit, res.unit, res.value); mismatch {
					reason := AvailabilityUnitMismatch
					if currencyMismatch {
						reason = AvailabilityCurrencyMismatch
					}
					kr.Value = unavailableValue(reason)
				}

				if def.Target != nil {
					kr.TargetEvaluation = evaluateTarget(*def.Target, true, kr.Value)
				}
				if len(def.ThresholdBands) > 0 && kr.Value.Available {
					if b, ok := bandFor(def.ThresholdBands, kr.Value.Amount); ok {
						kr.Band = b
						kr.BandAvailable = true
					}
				}

				kr.Provenance = buildProvenance(ctx, code, req)
				for _, mc := range kr.Provenance.SourceMetricCodes {
					if _, ok := ctx.metrics.byCodeDimension[mc+"\x00"+dim.hashKey()]; !ok {
						missingMetricSet[mc+"\x00"+period+"\x00"+dim.hashKey()] = true
					}
				}

				if opts.IncludeTrend {
					kr.Trend = buildTrend(ctx, code, dim, period, opts.TrendStabilityTolerance)
				} else {
					// Change is always computed against PRIOR_PERIOD
					// regardless of IncludeTrend (task section 21's
					// "support KPI outputs such as... prior value" is a
					// base output, distinct from the opt-in full Trend
					// series) — see resolveTimeTarget.
					if prior, ok := ctx.resolveTimeTarget(period, TimeRefPriorPeriod); ok {
						priorRes := ctx.evaluateKPI(code, evalRequest{period: prior, dim: dim}, map[string]bool{})
						kr.Change = computeChange(kr.Value, priorRes.value, def.Unit)
					} else {
						kr.Change = Change{Current: kr.Value, Prior: unavailableValue(AvailabilityPeriodUnavailable)}
					}
				}

				if res.trace != nil {
					kr.Trace = res.trace
				}

				kpiResults = append(kpiResults, kr)
			}
		}
	}

	// kpiResults is already in the required deterministic order by
	// construction: the outer loop walks graph.evaluationOrder
	// (topological order first, code tie-break — task section 14 — then
	// every cyclic/too-deep-excluded code appended sorted), then
	// evalPeriods (chronological — task section 42), then evalDimensions
	// (canonical dimension-key order — task section 42/5). No further
	// sort is needed or applied here; a future change to the
	// construction loop must preserve this nesting order rather than
	// re-adding a synthetic final sort.
	result.KPIResults = kpiResults

	result.Coverage.KPIsEvaluated = len(graph.evaluationOrder)
	for _, kr := range kpiResults {
		if kr.Value.Available {
			result.Coverage.AvailableValues++
		} else {
			result.Coverage.UnavailableValues++
		}
	}
	result.Coverage.MissingSourceMetricCount = len(missingMetricSet)

	evalIssues = append(evalIssues, metricIssues...)
	evalIssues = append(evalIssues, periodIssues...)
	sortEvaluationIssues(evalIssues)
	result.EvaluationIssues = evalIssues

	return result
}

// resolveEvalPeriods returns the Period.Code list to actually evaluate:
// requested (in chronological order, unknowns flagged) or, if empty,
// every valid Input.Periods entry.
func resolveEvalPeriods(requested []string, periodOrder []Period, issues *[]EvaluationIssue) []string {
	if len(requested) == 0 {
		codes := make([]string, len(periodOrder))
		for i, p := range periodOrder {
			codes[i] = p.Code
		}
		return codes
	}
	known := make(map[string]int, len(periodOrder))
	for _, p := range periodOrder {
		known[p.Code] = p.Sequence
	}
	var valid []string
	for _, code := range requested {
		if _, ok := known[code]; !ok {
			*issues = append(*issues, EvaluationIssue{Code: IssueInvalidPeriod, Severity: SeverityError, Period: code,
				Message: "requested period \"" + code + "\" is not present in Input.Periods"})
			continue
		}
		valid = append(valid, code)
	}
	sort.Slice(valid, func(i, j int) bool { return known[valid[i]] < known[valid[j]] })
	return dedupeStrings(valid)
}

// resolveEvalDimensions always includes the business-level group first,
// then any caller-requested dimensioned groups, canonicalized and
// deduplicated, in canonical sorted order — task section 5's "default
// dimension behavior must be strict" rule extended to which groups
// Calculate itself produces output for.
func resolveEvalDimensions(requested []DimensionKey) []DimensionKey {
	out := []DimensionKey{{}}
	seen := map[string]bool{"": true}
	canon := make([]DimensionKey, 0, len(requested))
	for _, d := range requested {
		c := d.canonicalize()
		if c.IsBusinessLevel() {
			continue
		}
		canon = append(canon, c)
	}
	sorted := sortedDimensionKeys(canon)
	for _, d := range sorted {
		key := d.hashKey()
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, d)
	}
	return out
}

func dedupeStrings(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
