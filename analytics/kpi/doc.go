// Package kpi implements a safe, deterministic, reusable KPI (key
// performance indicator) calculation engine.
//
// # Why this package exists
//
// Every other accounting/analytics package in this repository computes a
// fixed, hard-coded set of formulas (DSO, DPO, inventory turnover, revenue
// per FTE, contribution margin, ...). Those formulas are the authoritative
// source for the metrics they own — this package never recomputes them
// (see "Authoritative-domain-metric boundary" below). But a real
// application also needs client- or industry-specific KPIs that no
// pre-built package will ever cover: "Revenue per Square Foot" for a
// retail client, "AR 60+ % of AR" for a collections dashboard, "(A + B) /
// C" for a custom composite a controller defines once and reuses. Adding a
// new Go package for every such formula does not scale. This package is a
// small, safe evaluation engine so a caller can define those formulas as
// data (a Definition value) instead of code.
//
// # Not a scripting engine
//
// This package does not execute arbitrary code. There is no eval, no
// reflection-based invocation, no Go/JavaScript/Python/SQL/template
// interpreter, and no user-defined function mechanism. A KPI formula is a
// typed Expression tree (see expression.go) built from a small, fixed set
// of arithmetic/aggregation Operators (ADD, DIVIDE, PERCENT, ...) applied
// to references to other KPIs or source metrics. Every operator's
// semantics is fixed Go code in this package; a Definition can only choose
// *which* fixed operators to combine and in what shape, never introduce a
// new one. See "No string DSL in V1" below for why there is not even a
// string-formula parser yet.
//
// # Core data flow
//
//	existing module results / app metrics / custom facts
//	                    |
//	             MetricValue inputs  (metric.go)
//	                    +
//	             KPI Definitions     (definition.go, expression.go)
//	                    |
//	              analytics/kpi      (evaluate.go, Calculate in calculate.go)
//	                    |
//	          calculated KPI values  (KPIResult in result.go)
//	          targets / bands / trends
//	          dependency trace
//	          coverage / issues
//
// This package does not own source accounting data. A caller assembles
// MetricValue facts from whatever authoritative source computed them
// (accounting/ar.Result.DSO, financial/metrics.Snapshot, a homegrown app
// metric, ...) — see the *adapter.go files for portable, tested bridges
// from eight sibling packages' representative outputs, and "Adapter
// philosophy" below.
//
// # No string DSL in V1
//
// A typed Expression tree, not a string formula parser, is this package's
// V1 persistence contract (see expression.go's doc comment). If a string
// DSL is added in a later version, it will compile down into the exact
// same Expression AST this package already evaluates, and will carry its
// own, separately versioned grammar — it will never bypass the AST or
// introduce a second evaluation path. See ExpressionLanguageVersion in
// versions.go.
//
// # Authoritative-domain-metric boundary
//
// Where a domain package already authoritatively computes a metric (AR
// DSO, AP DPO, inventory DIO, labor RevenuePerFTE, profitability
// ContributionMargin, ...), this package's role is composition, custom
// ratios, client/industry formulas, targets, and trend/threshold
// evaluation on top of that figure — never recomputing it. The
// *adapter.go files expose those existing computed values as MetricValue
// facts; they contain no formula logic of their own (see each adapter's
// doc comment and docs/KPI_ENGINE.md's "Adapter philosophy" section).
//
// # Determinism and safety
//
// Calculate is a pure function: no I/O, no time.Now(), no package-level
// mutable state, and it never mutates any caller-supplied Definition,
// Expression, MetricValue, Period, DimensionKey, TargetPolicy, or
// ThresholdBand (see immutability_test.go). Every ordering-sensitive
// output (definitions, periods, dimension groups, issues, dependency
// traversal) is explicitly sorted rather than relying on Go map order
// (see determinism_test.go). Repeated Calculate calls against identical,
// unmodified input — including concurrently, from multiple goroutines —
// always return byte-for-byte identical JSON (see
// concurrency_test.go/roundtrip_test.go).
//
// See docs/KPI_ENGINE.md for the full design write-up.
package kpi
