package kpi

import (
	"math"
	"strconv"
	"testing"
)

// TestDependency_KPIToKPI proves KPI B = KPI A / TargetRevenuePerFTE, where
// KPI A = Revenue / FTE — task section 34's worked example.
func TestDependency_KPIToKPI(t *testing.T) {
	// revenue / fte's actual combined unit is CURRENCY/COUNT ->
	// UnitCustom "USD_PER_COUNT" (see combineDivisive) — kpiA must
	// declare that same unit, not plain CURRENCY, or
	// checkExpectedOutputUnit correctly reports a definition/actual unit
	// mismatch.
	revenuePerFTEUnit := Unit{Kind: UnitCustom, CustomLabel: "USD_PER_COUNT"}
	kpiA := Definition{Code: "revenue_per_fte", Unit: revenuePerFTEUnit,
		Formula: Binary(OpDivide, Metric(MetricRef{Code: "revenue"}), Metric(MetricRef{Code: "fte"}))}
	kpiB := Definition{Code: "revenue_per_fte_vs_target", Unit: Unit{Kind: UnitRatio},
		Formula: Binary(OpDivide, KPIExpr(KPIRef{Code: "revenue_per_fte"}), Metric(MetricRef{Code: "target"}))}

	metrics := []MetricValue{
		metricInput("revenue", "P1", 1000000, true, currencyUnit("USD")),
		metricInput("fte", "P1", 10, true, Unit{Kind: UnitCount}),
		// target must share kpiA's own resolved unit (USD_PER_COUNT) for
		// kpiB's DIVIDE to be unit-compatible.
		metricInput("target", "P1", 50000, true, revenuePerFTEUnit),
	}
	res := Calculate(onePeriodInput([]Definition{kpiA, kpiB}, metrics), Options{})
	krA := firstResult(t, res, "revenue_per_fte")
	if !krA.Value.Available || krA.Value.Amount != 100000 {
		t.Fatalf("kpiA got %+v", krA.Value)
	}
	krB := firstResult(t, res, "revenue_per_fte_vs_target")
	if !krB.Value.Available || math.Abs(krB.Value.Amount-2) > 1e-9 {
		t.Fatalf("kpiB got %+v", krB.Value)
	}
}

// TestDependency_InputOrderIndependence proves the same set of Definitions
// produces identical results regardless of caller order — task section
// 34's "prove input order does not matter."
func TestDependency_InputOrderIndependence(t *testing.T) {
	kpiA := Definition{Code: "a", Unit: currencyUnit("USD"), Formula: Metric(MetricRef{Code: "revenue"})}
	kpiB := Definition{Code: "b", Unit: currencyUnit("USD"), Formula: KPIExpr(KPIRef{Code: "a"})}
	kpiC := Definition{Code: "c", Unit: currencyUnit("USD"), Formula: KPIExpr(KPIRef{Code: "b"})}

	metrics := []MetricValue{metricInput("revenue", "P1", 500, true, currencyUnit("USD"))}

	forward := Calculate(onePeriodInput([]Definition{kpiA, kpiB, kpiC}, metrics), Options{})
	reverse := Calculate(onePeriodInput([]Definition{kpiC, kpiB, kpiA}, metrics), Options{})
	shuffled := Calculate(onePeriodInput([]Definition{kpiB, kpiC, kpiA}, metrics), Options{})

	for _, code := range []string{"a", "b", "c"} {
		f, r, s := firstResult(t, forward, code), firstResult(t, reverse, code), firstResult(t, shuffled, code)
		if f.Value != r.Value || f.Value != s.Value {
			t.Fatalf("code %s: order-dependent result: forward=%+v reverse=%+v shuffled=%+v", code, f.Value, r.Value, s.Value)
		}
	}
}

// TestDependency_Cycle_Regression is the explicit cycle regression test —
// task section 34/15's A -> B -> C -> A example.
func TestDependency_Cycle_Regression(t *testing.T) {
	kpiA := Definition{Code: "a", Unit: currencyUnit("USD"), Formula: KPIExpr(KPIRef{Code: "b"})}
	kpiB := Definition{Code: "b", Unit: currencyUnit("USD"), Formula: KPIExpr(KPIRef{Code: "c"})}
	kpiC := Definition{Code: "c", Unit: currencyUnit("USD"), Formula: KPIExpr(KPIRef{Code: "a"})}
	kpiD := Definition{Code: "d", Unit: currencyUnit("USD"), Formula: Metric(MetricRef{Code: "revenue"})}

	res := Calculate(onePeriodInput([]Definition{kpiA, kpiB, kpiC, kpiD},
		[]MetricValue{metricInput("revenue", "P1", 100, true, currencyUnit("USD"))}), Options{})

	for _, code := range []string{"a", "b", "c"} {
		kr := firstResult(t, res, code)
		if kr.Value.Available {
			t.Fatalf("code %s: expected unavailable (cycle), got %+v", code, kr.Value)
		}
	}
	foundCycleIssue := false
	for _, i := range res.DefinitionIssues {
		if i.Code == IssueDependencyCycle {
			foundCycleIssue = true
		}
	}
	if !foundCycleIssue {
		t.Fatalf("expected IssueDependencyCycle, got %v", res.DefinitionIssues)
	}

	// Independent KPI D still evaluates — task section 15's "independent
	// KPI D should still evaluate in lenient mode."
	krD := firstResult(t, res, "d")
	if !krD.Value.Available || krD.Value.Amount != 100 {
		t.Fatalf("kpi D should still evaluate, got %+v", krD.Value)
	}
}

func TestDependency_SelfReference(t *testing.T) {
	def := Definition{Code: "a", Unit: currencyUnit("USD"), Formula: KPIExpr(KPIRef{Code: "a"})}
	res := Calculate(onePeriodInput([]Definition{def}, nil), Options{})
	kr := firstResult(t, res, "a")
	if kr.Value.Available {
		t.Fatalf("self-reference should be unavailable, got %+v", kr.Value)
	}
}

// TestDependency_SelfReference_MustBeMarkedInCycle is a permanent
// regression test: a self-referencing KPI (A -> KPIRef{A}) was found NOT
// to be added to dependencyGraph.inCycle (only detectCycles' findings
// were folded in; a self-loop was deliberately never added as a g.edges
// entry, so detectCycles itself never saw it) — evaluateKPI stayed safe
// only because of its own independent `visiting` recursion guard, but
// buildProvenance's later memoized rewrite trusted inCycle alone and had
// no such guard, causing a real stack-overflow crash (not just a wrong
// answer) the first time this exact shape was exercised. This test
// exercises buildProvenance directly (the path that actually crashed) to
// make sure this specific gap can never silently reopen.
func TestDependency_SelfReference_MustBeMarkedInCycle(t *testing.T) {
	defs := map[string]Definition{"a": {Code: "a", Unit: currencyUnit("USD"), Formula: KPIExpr(KPIRef{Code: "a"})}}
	graph, _ := buildDependencyGraph(defs, nil)
	if !graph.inCycle["a"] {
		t.Fatalf("a self-referencing KPI must be marked inCycle, not just reported as an issue")
	}

	// End-to-end: Calculate must complete (not hang/crash) and produce a
	// well-formed, empty Provenance for the self-referencing KPI.
	res := Calculate(onePeriodInput([]Definition{defs["a"]}, nil), Options{})
	kr := firstResult(t, res, "a")
	if len(kr.Provenance.SourceMetricCodes) != 0 || len(kr.Provenance.DependencyKPICodes) != 0 {
		t.Fatalf("self-referencing KPI's Provenance should be empty, got %+v", kr.Provenance)
	}
}

func TestDependency_UnknownKPI(t *testing.T) {
	def := Definition{Code: "a", Unit: currencyUnit("USD"), Formula: KPIExpr(KPIRef{Code: "does_not_exist"})}
	res := Calculate(onePeriodInput([]Definition{def}, nil), Options{})
	found := false
	for _, i := range res.DefinitionIssues {
		if i.Code == IssueUnknownKPI {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected IssueUnknownKPI, got %v", res.DefinitionIssues)
	}
}

func TestDependency_LongChain(t *testing.T) {
	const chainLen = 50
	var defs []Definition
	defs = append(defs, Definition{Code: "k0", Unit: currencyUnit("USD"), Formula: Metric(MetricRef{Code: "base"})})
	for i := 1; i < chainLen; i++ {
		code := "k" + strconv.Itoa(i)
		prev := "k" + strconv.Itoa(i-1)
		defs = append(defs, Definition{Code: code, Unit: currencyUnit("USD"), Formula: KPIExpr(KPIRef{Code: prev})})
	}
	res := Calculate(onePeriodInput(defs, []MetricValue{metricInput("base", "P1", 7, true, currencyUnit("USD"))}), Options{})
	last := firstResult(t, res, "k"+strconv.Itoa(chainLen-1))
	if !last.Value.Available || last.Value.Amount != 7 {
		t.Fatalf("long chain got %+v", last.Value)
	}
}

func TestDependency_DuplicateKPICode(t *testing.T) {
	defs := []Definition{
		{Code: "dup", Unit: currencyUnit("USD"), Formula: Metric(MetricRef{Code: "a"})},
		{Code: "dup", Unit: currencyUnit("USD"), Formula: Metric(MetricRef{Code: "b"})},
	}
	res := Calculate(onePeriodInput(defs, []MetricValue{
		metricInput("a", "P1", 1, true, currencyUnit("USD")),
		metricInput("b", "P1", 2, true, currencyUnit("USD")),
	}), Options{})
	found := false
	for _, i := range res.DefinitionIssues {
		if i.Code == IssueDuplicateKPICode {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected IssueDuplicateKPICode, got %v", res.DefinitionIssues)
	}
	// First occurrence wins.
	kr := firstResult(t, res, "dup")
	if kr.Value.Amount != 1 {
		t.Fatalf("expected first occurrence's value (1), got %+v", kr.Value)
	}
}

// TestDependency_TopologicalOrder_CodeTieBreak is a permanent regression
// test for topologicalOrder's O(n^2 log n) bug (found via benchmark
// profiling — see BenchmarkCalculate_ScalingCheck/BenchmarkCalculate_
// 10000Definitions — a full sort.Strings(ready) was called on every
// Kahn's-algorithm iteration instead of once, dominating ~76% of total
// CPU time at 10,000 independent KPI definitions; fixed with a
// container/heap min-heap). This test locks the OUTPUT CONTRACT the fix
// must preserve exactly: many mutually-independent KPIs (all inDegree 0
// at once, the exact shape that triggered the bug) must still evaluate
// in strict lexicographic Code order — task section 42's "dependency
// traversal topological with code tie-break" rule, now enforced by a
// min-heap instead of a repeated full sort, but with an IDENTICAL
// observable ordering contract.
func TestDependency_TopologicalOrder_CodeTieBreak(t *testing.T) {
	// Deliberately out-of-order Definition input, all independent (no
	// KPI-to-KPI edges) -- topologicalOrder must still emit them
	// alphabetically by Code, not in caller/map-iteration order.
	defs := []Definition{
		{Code: "zebra", Unit: currencyUnit("USD"), Formula: Metric(MetricRef{Code: "m_zebra"})},
		{Code: "apple", Unit: currencyUnit("USD"), Formula: Metric(MetricRef{Code: "m_apple"})},
		{Code: "mango", Unit: currencyUnit("USD"), Formula: Metric(MetricRef{Code: "m_mango"})},
	}
	graph, _ := buildDependencyGraph(toDefMap(defs), nil)
	want := []string{"apple", "mango", "zebra"}
	if len(graph.order) != len(want) {
		t.Fatalf("order length = %d, want %d: %v", len(graph.order), len(want), graph.order)
	}
	for i, code := range want {
		if graph.order[i] != code {
			t.Fatalf("order[%d] = %q, want %q (full order: %v)", i, graph.order[i], code, graph.order)
		}
	}
}

func TestDependency_NamespaceCollision(t *testing.T) {
	// A KPI code that collides with a supplied source metric code.
	def := Definition{Code: "revenue", Unit: currencyUnit("USD"), Formula: Const(1)}
	res := Calculate(onePeriodInput([]Definition{def}, []MetricValue{
		metricInput("revenue", "P1", 100, true, currencyUnit("USD")),
	}), Options{})
	found := false
	for _, i := range res.DefinitionIssues {
		if i.Code == IssueNamespaceCollision {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected IssueNamespaceCollision, got %v", res.DefinitionIssues)
	}
}
