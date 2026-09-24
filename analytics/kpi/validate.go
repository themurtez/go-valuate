package kpi

import "sort"

// operatorArity describes the accepted Args count for one Operator —
// task section 8's "define exact semantics for every operator" and the
// IssueInvalidArgumentCount check.
type operatorArity struct {
	min int
	max int // -1 means unbounded
	// exact, if true, requires Args count == min (max is ignored).
	exact bool
	// evenNonZero, if true (OpWeightedAverage only), requires a
	// positive, even Args count instead of the min/max/exact shape.
	evenNonZero bool
}

func arityFor(op Operator) (operatorArity, bool) {
	switch op {
	case OpMetric, OpKPI, OpConstant:
		return operatorArity{exact: true, min: 0}, true
	case OpAdd, OpSubtract, OpDivide, OpPercent, OpPercentChange:
		return operatorArity{exact: true, min: 2}, true
	case OpMultiply:
		return operatorArity{exact: true, min: 2}, true
	case OpNegate, OpAbs:
		return operatorArity{exact: true, min: 1}, true
	case OpMin, OpMax:
		return operatorArity{min: 2, max: -1}, true
	case OpSum, OpAverage, OpCoalesce:
		return operatorArity{min: 1, max: -1}, true
	case OpWeightedAverage:
		return operatorArity{evenNonZero: true}, true
	default:
		return operatorArity{}, false
	}
}

func (a operatorArity) accepts(n int) bool {
	if a.evenNonZero {
		return n > 0 && n%2 == 0
	}
	if a.exact {
		return n == a.min
	}
	if n < a.min {
		return false
	}
	if a.max < 0 {
		return true
	}
	return n <= a.max
}

// validateExpression checks e's structural validity: recognized
// operators, correct leaf/Args shape, correct arity, and complexity
// limits. It does not check unit compatibility (checkUnits, run
// separately during evaluation-independent definition validation — see
// validateDefinitionUnits) or KPI-reference resolvability (dependency.go,
// since that requires knowing every other Definition's Code, not just
// this one Expression in isolation).
func validateExpression(kpiCode string, e Expression) []DefinitionIssue {
	shape := measureExpression(e)
	var issues []DefinitionIssue
	if shape.depth > MaxExpressionDepth || shape.nodes > MaxExpressionNodes {
		issues = append(issues, DefinitionIssue{Code: IssueExpressionTooComplex, Severity: SeverityError, KPICode: kpiCode,
			Message: "expression exceeds the maximum depth or node-count limit"})
		return issues // further structural walk is not useful once over the limit
	}
	issues = append(issues, validateExpressionNode(kpiCode, e, 1)...)
	return issues
}

func validateExpressionNode(kpiCode string, e Expression, depth int) []DefinitionIssue {
	if depth > MaxExpressionDepth+1 {
		return nil
	}
	var issues []DefinitionIssue
	arity, known := arityFor(e.Op)
	if !known {
		issues = append(issues, DefinitionIssue{Code: IssueUnknownOperator, Severity: SeverityError, KPICode: kpiCode,
			Message: "unrecognized operator \"" + string(e.Op) + "\""})
		return issues
	}

	switch e.Op {
	case OpMetric:
		if e.MetricRef == nil || e.MetricRef.Code == "" {
			issues = append(issues, DefinitionIssue{Code: IssueInvalidExpression, Severity: SeverityError, KPICode: kpiCode,
				Message: "METRIC node requires a non-empty MetricRef.Code"})
		}
		if e.MetricRef != nil && e.MetricRef.Time == TimeRefTrailingN && e.MetricRef.TrailingN <= 0 {
			issues = append(issues, DefinitionIssue{Code: IssueInvalidExpression, Severity: SeverityError, KPICode: kpiCode,
				Message: "METRIC node with TRAILING_N time reference requires TrailingN > 0"})
		}
		if e.MetricRef != nil && !isRecognizedTimeRef(e.MetricRef.Time) {
			issues = append(issues, DefinitionIssue{Code: IssueInvalidExpression, Severity: SeverityError, KPICode: kpiCode,
				Message: "METRIC node has an unrecognized time reference"})
		}
		if len(e.Args) != 0 {
			issues = append(issues, DefinitionIssue{Code: IssueInvalidExpression, Severity: SeverityError, KPICode: kpiCode,
				Message: "METRIC node must not have Args"})
		}
		return issues
	case OpKPI:
		if e.KPIRef == nil || e.KPIRef.Code == "" {
			issues = append(issues, DefinitionIssue{Code: IssueInvalidExpression, Severity: SeverityError, KPICode: kpiCode,
				Message: "KPI node requires a non-empty KPIRef.Code"})
		}
		if e.KPIRef != nil && e.KPIRef.Time == TimeRefTrailingN {
			issues = append(issues, DefinitionIssue{Code: IssueInvalidExpression, Severity: SeverityError, KPICode: kpiCode,
				Message: "KPI node does not support TRAILING_N time reference"})
		}
		if e.KPIRef != nil && !isRecognizedTimeRef(e.KPIRef.Time) {
			issues = append(issues, DefinitionIssue{Code: IssueInvalidExpression, Severity: SeverityError, KPICode: kpiCode,
				Message: "KPI node has an unrecognized time reference"})
		}
		if len(e.Args) != 0 {
			issues = append(issues, DefinitionIssue{Code: IssueInvalidExpression, Severity: SeverityError, KPICode: kpiCode,
				Message: "KPI node must not have Args"})
		}
		return issues
	case OpConstant:
		if e.Constant == nil {
			issues = append(issues, DefinitionIssue{Code: IssueInvalidExpression, Severity: SeverityError, KPICode: kpiCode,
				Message: "CONSTANT node requires a non-nil Constant"})
		} else if isNonFinite(*e.Constant) {
			issues = append(issues, DefinitionIssue{Code: IssueNonFiniteInput, Severity: SeverityError, KPICode: kpiCode,
				Message: "CONSTANT node value is NaN or Inf"})
		}
		if len(e.Args) != 0 {
			issues = append(issues, DefinitionIssue{Code: IssueInvalidExpression, Severity: SeverityError, KPICode: kpiCode,
				Message: "CONSTANT node must not have Args"})
		}
		return issues
	}

	if !arity.accepts(len(e.Args)) {
		issues = append(issues, DefinitionIssue{Code: IssueInvalidArgumentCount, Severity: SeverityError, KPICode: kpiCode,
			Message: "operator \"" + string(e.Op) + "\" received an invalid number of arguments"})
	}
	for _, arg := range e.Args {
		issues = append(issues, validateExpressionNode(kpiCode, arg, depth+1)...)
	}
	return issues
}

// validateDefinitions checks every top-level Definition-shape concern that
// does not require the full dependency graph: duplicate/invalid Code,
// expression structure, unit/target/band validity, and expression-
// language version. Returns the set of Definitions that passed (keyed by
// Code, first occurrence wins on a duplicate Code — task section 3's
// sibling "never pick first/last arbitrarily" rule is about MetricValues
// specifically; for a genuinely duplicate KPI Code, using the first
// occurrence deterministically, same as every sibling package's duplicate-
// ID convention, is the right behavior here since silently dropping the
// KPI entirely would be worse for a caller debugging their own definition
// list).
func validateDefinitions(defs []Definition, expressionLanguageVersion string) (valid map[string]Definition, issues []DefinitionIssue) {
	valid = make(map[string]Definition, len(defs))
	seen := map[string]bool{}

	if expressionLanguageVersion != "" && expressionLanguageVersion != ExpressionLanguageVersion {
		for _, d := range defs {
			issues = append(issues, DefinitionIssue{Code: IssueUnsupportedExpressionLanguageVersion, Severity: SeverityError, KPICode: d.Code,
				Message: "expression language version \"" + expressionLanguageVersion + "\" is not supported by this engine (supports \"" + ExpressionLanguageVersion + "\")"})
		}
		return valid, issues
	}

	for _, d := range defs {
		if d.Code == "" {
			issues = append(issues, DefinitionIssue{Code: IssueInvalidKPICode, Severity: SeverityError,
				Message: "a KPI Definition has an empty Code"})
			continue
		}
		if seen[d.Code] {
			issues = append(issues, DefinitionIssue{Code: IssueDuplicateKPICode, Severity: SeverityError, KPICode: d.Code,
				Message: "duplicate KPI code \"" + d.Code + "\"; only the first occurrence is used"})
			continue
		}
		seen[d.Code] = true

		var defIssues []DefinitionIssue
		defIssues = append(defIssues, validateExpression(d.Code, d.Formula)...)
		if !d.Unit.Valid() {
			defIssues = append(defIssues, DefinitionIssue{Code: IssueInvalidUnit, Severity: SeverityError, KPICode: d.Code,
				Message: "Definition.Unit is not a structurally valid unit"})
		}
		if d.Target != nil && !d.Target.valid() {
			defIssues = append(defIssues, DefinitionIssue{Code: IssueInvalidTarget, Severity: SeverityError, KPICode: d.Code,
				Message: "Definition.Target is not structurally valid"})
		}
		if len(d.ThresholdBands) > 0 {
			ok, overlapping, degenerate := validateBands(d.ThresholdBands)
			if degenerate {
				defIssues = append(defIssues, DefinitionIssue{Code: IssueInvalidThresholdBand, Severity: SeverityError, KPICode: d.Code,
					Message: "a threshold band has Min >= Max"})
			} else if overlapping {
				defIssues = append(defIssues, DefinitionIssue{Code: IssueOverlappingThresholdBands, Severity: SeverityError, KPICode: d.Code,
					Message: "Definition.ThresholdBands contains overlapping ranges"})
			}
			_ = ok
		}

		if HasDefinitionErrors(defIssues) {
			issues = append(issues, defIssues...)
			continue
		}
		issues = append(issues, defIssues...)
		valid[d.Code] = d
	}
	return valid, issues
}

// sortDefinitionIssues sorts by IssueCode declaration order then KPICode —
// task section 42.
func sortDefinitionIssues(issues []DefinitionIssue) {
	sort.SliceStable(issues, func(i, j int) bool {
		ri, rj := definitionIssueRank(issues[i].Code), definitionIssueRank(issues[j].Code)
		if ri != rj {
			return ri < rj
		}
		return issues[i].KPICode < issues[j].KPICode
	})
}

// sortEvaluationIssues sorts by IssueCode declaration order, then KPICode,
// then Period, then dimension hash — task section 42.
func sortEvaluationIssues(issues []EvaluationIssue) {
	sort.SliceStable(issues, func(i, j int) bool {
		ri, rj := evaluationIssueRank(issues[i].Code), evaluationIssueRank(issues[j].Code)
		if ri != rj {
			return ri < rj
		}
		if issues[i].KPICode != issues[j].KPICode {
			return issues[i].KPICode < issues[j].KPICode
		}
		if issues[i].Period != issues[j].Period {
			return issues[i].Period < issues[j].Period
		}
		return issues[i].Dimensions.hashKey() < issues[j].Dimensions.hashKey()
	})
}
