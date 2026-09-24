# KPI Engine (`analytics/kpi`)

`analytics/kpi` implements a safe, deterministic, reusable KPI (key
performance indicator) calculation engine, for the client- and
industry-specific formulas no pre-built accounting/analytics package will
ever cover — "Revenue per Square Foot" for a retail client, "AR 60+ % of
AR" for a collections dashboard, "(A + B) / C" for a custom composite a
controller defines once and reuses.

## Purpose

Every other accounting/analytics package in this repository computes a
fixed, hard-coded set of formulas (DSO, DPO, inventory turnover, revenue
per FTE, contribution margin, ...). Those formulas are the authoritative
source for the metrics they own; this package never recomputes them (see
[Authoritative-domain-metric boundary](#authoritative-domain-metric-boundary)).
Adding a new Go package for every client-specific formula does not scale.
This package is a small, safe evaluation engine so a caller can define a
KPI formula as data (a `Definition` value) instead of code, and evaluate
it against whatever facts the rest of the application already computed.

## Not a scripting engine

There is no `eval`, no reflection-based invocation, no Go/JavaScript/
Python/SQL/template interpreter, and no user-defined function mechanism.
A KPI formula is a typed `Expression` tree built from a small, fixed set
of arithmetic/aggregation `Operator`s (`ADD`, `DIVIDE`, `PERCENT`, ...)
applied to references to other KPIs or source metrics. Every operator's
semantics is fixed Go code in this package; a `Definition` can only
choose *which* fixed operators to combine and in what shape, never
introduce a new one.

## Typed AST — the V1 persistence contract

A typed `Expression` tree, not a string formula parser, is this
package's V1 formula representation:

```go
type Expression struct {
    Op        Operator
    MetricRef *MetricRef
    KPIRef    *KPIRef
    Constant  *float64
    Args      []Expression
}
```

Exactly one of `MetricRef`/`KPIRef`/`Constant` is set when `Op` is
`METRIC`/`KPI`/`CONSTANT` (leaf nodes, `Args` empty); for every other
`Op`, `Args` holds the operator's operands. This shape (rather than a
single `interface{}` payload) keeps `Expression` JSON-round-trippable
with a fixed, predictable shape — `json_test.go`'s
`TestJSON_Expression_ArgOrderStable` locks that argument order survives a
round-trip exactly, which matters for non-commutative operators like
`SUBTRACT`/`DIVIDE`.

If a string DSL is added in a later version, it will compile down into
this exact same `Expression` AST — it will never bypass the AST or
introduce a second evaluation path. See
[Expression-language versioning](#expression-language-versioning).

## Core data flow

```
existing module results / app metrics / custom facts
                    |
             MetricValue inputs
                    +
             KPI Definitions
                    |
              analytics/kpi
                    |
          calculated KPI values
          targets / bands / trends
          dependency trace
          coverage / issues
```

This package does not own source accounting data. A caller assembles
`MetricValue` facts from whatever authoritative source computed them —
see the `*adapter.go` files for portable, tested bridges from eight
sibling packages' representative outputs.

## Metric input model

```go
type MetricValue struct {
    Code        string
    Period      string
    Dimensions  DimensionKey
    Value       float64
    Available   bool
    Unit        Unit
    Aggregation AggregationRule
    Source      string
    SourceRef   string
}
```

`Available`/`Value` distinguish "computed to be exactly 0" from "cannot
be computed because a required input is absent" — the same convention
`financial/metrics.MetricValue`, `accounting/vendorspend.Value`, and
every sibling package's own availability type already use.
`availability_test.go`'s `TestAvailability_KnownZero`/`_Missing` lock
this at the KPI-result level too. Duplicate `(code, period, canonical
dimension key)` combinations are rejected (`IssueDuplicateMetricValue`);
the first occurrence wins — this package never picks first/last
arbitrarily, it always picks first, consistent with every duplicate-ID
convention elsewhere in this repository.

## KPI definitions

```go
type Definition struct {
    Code              string
    Name              string
    Description       string
    Formula           Expression
    Unit              Unit
    Category          string
    Tags              []string
    Target            *TargetPolicy
    ThresholdBands    []ThresholdBand
    DefinitionVersion string
}
```

`Code` is stable machine identity, used everywhere a dependency graph or
`Result` needs to name this KPI; `Name`/`Description` are display text
only. `Unit` is this KPI's *declared* output unit — checked against what
`Formula`'s root *actually* produces once real `MetricValue`s are
supplied (see [Unit compatibility](#unit-compatibility)); a mismatch is
a runtime `AvailabilityUnitMismatch`/`AvailabilityCurrencyMismatch`, not
a definition-time error, since a `METRIC` leaf's unit is a runtime fact.

## Operators

| Operator | Arity | Semantics |
|---|---|---|
| `METRIC` | leaf | Resolves `MetricRef` |
| `KPI` | leaf | Resolves `KPIRef` |
| `CONSTANT` | leaf | Resolves `Constant` |
| `ADD` | 2 | `Args[0] + Args[1]` |
| `SUBTRACT` | 2 | `Args[0] - Args[1]` |
| `MULTIPLY` | 2 | `Args[0] * Args[1]` |
| `DIVIDE` | 2 | `Args[0] / Args[1]`, unavailable if `Args[1] == 0` |
| `NEGATE` | 1 | `-Args[0]` |
| `ABS` | 1 | `\|Args[0]\|` |
| `MIN` | 2+ | Minimum of all `Args` (same `Unit` required) |
| `MAX` | 2+ | Maximum of all `Args` (same `Unit` required) |
| `SUM` | 1+ | Sum of all `Args` (same `Unit` required) |
| `AVERAGE` | 1+ | Arithmetic mean of all `Args` (same `Unit` required) |
| `WEIGHTED_AVERAGE` | even, 2+ | `(value,weight)` pairs: `sum(v*w)/sum(w)`, unavailable if total weight is 0 |
| `PERCENT` | 2 | `Args[0]/Args[1]*100`, result `Unit` `PERCENT` |
| `PERCENT_CHANGE` | 2 | `(current-prior)/prior*100`, a *relative* percent change |
| `COALESCE` | 1+ | First `Arg` that resolves `Available`, else unavailable |

Every operator guards its result against `NaN`/`Inf`/overflow —
`AvailabilityNonFiniteResult` — rather than silently returning a
non-finite float (`fuzz_test.go`'s targets specifically fuzz for NaN/Inf
leaks). `COALESCE` must be used explicitly in a `Definition` (never
implicit anywhere else in this package) and never suppresses a
structural definition error under one of its own `Args` —
`availability_test.go`'s `TestAvailability_Coalesce_
NeverSuppressesDefinitionError` locks this.

## Availability semantics

```go
type Value struct {
    Amount    float64
    Available bool
    Reason    AvailabilityReason
}
```

`AvailabilityReason` is a closed, typed enum
(`MISSING_METRIC`/`SOURCE_UNAVAILABLE`/`DEPENDENCY_UNAVAILABLE`/
`DIVIDE_BY_ZERO`/`UNIT_MISMATCH`/`CURRENCY_MISMATCH`/
`PERIOD_UNAVAILABLE`/`DIMENSION_UNAVAILABLE`/`NOT_APPLICABLE`/
`INVALID_DEFINITION`/`NON_FINITE_RESULT`) — the reason is never encoded
only in prose. Missing metric defaults to unavailable, never zero;
`COALESCE` is the only opt-in exception, and it is always explicit in
the `Definition`.

## Unit model

```go
type Unit struct {
    Kind         UnitKind // CURRENCY, COUNT, HOURS, DAYS, PERCENT,
                          // RATIO, QUANTITY, AREA, UNITLESS, CUSTOM
    CurrencyCode string   // required iff Kind == CURRENCY
    CustomLabel  string   // required iff Kind == CUSTOM
}
```

There is no default currency — a `Unit{Kind: CURRENCY}` with an empty
`CurrencyCode` is itself structurally invalid (`Unit.Valid()` false) and
is rejected wherever it would otherwise participate in arithmetic
(`buildMetricIndex` rejects an `Available` `MetricValue` with an invalid
`Unit`, `IssueInvalidUnit`; the `combineAdditive`/`combineMultiplicative`/
`combineDivisive` primitives also independently refuse any invalid input
`Unit`, found via `FuzzUnitCompatibility` — see
[Fuzzing](#fuzzing-results)).

### Unit compatibility

This is deliberately a small explicit system, not a full symbolic units
engine (a full system was judged unnecessary complexity for the
practical combinations this package actually needs):

```
CAD + CAD           -> valid (CAD)
CAD + USD           -> invalid (currency mismatch)
Currency + Hours     -> invalid (unit mismatch)
Currency * Ratio/Percent/Unitless -> Currency (scaling)
Currency / Count     -> currency-per-count (a CUSTOM unit, "<code>_PER_COUNT")
Currency / Currency  -> RATIO (same currency only)
Currency / <any other non-scalar, non-currency kind> -> currency-per-<kind>
                        (generalizes the Count case — "Revenue per Square
                        Foot" needs Currency/Area, "Average Ticket" needs
                        Currency/Count; both resolve the same way)
<any> / <same Unit>  -> RATIO
<any> / scalar        -> unchanged (dividing by a plain count/ratio/
                        percent/unitless scales, does not change kind)
```

Two `UnitCustom` values are compatible for `ADD`/`SUBTRACT` only when
`CustomLabel` matches exactly.

## Percent convention

`RATIO` values are a raw ratio (e.g. `0.25`); `PERCENT` values are
0–100 (e.g. `25`). The two are never mixed — `PERCENT` and `RATIO` are
distinct `UnitKind` values, and `PERCENT`/`PERCENT_CHANGE`'s output is
always explicitly `UnitPercent`.

For a percent-typed KPI's period-over-period `Change`, two distinct
fields answer two distinct questions and are both always computed:

```go
type Change struct {
    Current, Prior         Value
    AbsoluteChange          Value // Current - Prior, in the KPI's own Unit
    RelativeChangePercent   Value // (Current-Prior)/Prior * 100 -- a
                                  // RELATIVE percent change
    PercentagePointChange   Value // Current - Prior, ONLY when the KPI's
                                  // own Unit is UnitPercent
}
```

30% → 35% is `PercentagePointChange = +5`, `RelativeChangePercent =
+16.7%` — both computed, a caller reads whichever it means. `period_test.
go`'s `TestPeriods_PercentagePointChange` locks the worked example
exactly.

## Periods

```go
type Period struct {
    Code           string
    Sequence       int    // required strictly increasing; chronological
                          // order, never inferred from Code's text
    StartDate      string
    EndDate        string
    Days           int
    FiscalYear     string // required for PRIOR_YEAR_SAME_PERIOD
    PositionInYear string // required for PRIOR_YEAR_SAME_PERIOD
}
```

No fiscal-calendar inference, no `time.Now()`. Each requested period is
evaluated independently — no implicit forward-fill/backfill across a
period a caller did not supply
(`period_test.go`'s
`TestPeriods_MultiPeriod_IndependentEvaluation`).

Four safe, explicit time references (`TimeRef`) are supported on a
`MetricRef`/`KPIRef`:

- `CURRENT` (the zero value) — the period being evaluated.
- `PRIOR_PERIOD` — the immediately preceding `Sequence`.
- `PRIOR_YEAR_SAME_PERIOD` — resolved purely from
  `(FiscalYear, PositionInYear)`, never by parsing `Period.Code`'s text.
  When more than one prior fiscal year shares the same
  `PositionInYear`, the *nearest* one is returned, not the earliest —
  `period_test.go`'s `TestPeriods_PriorYearSamePeriod_
  NearestNotEarliest` is a permanent regression test for a real bug
  found here (an earlier implementation scanned forward from the start
  and returned the first, i.e. earliest, match).
- `TRAILING_N` — the trailing N periods ending at (and including) the
  evaluated one, rolled up via the referenced metric's own
  `AggregationRule`. Not supported on a `KPIRef` (deferred — see
  [Limitations](#limitations)).

## Dimensions

```go
type DimensionKey map[string]string // opaque, exact-match tags,
                                     // e.g. {"department": "service"}
```

Canonicalized (sorted keys, empty-string values dropped) for
determinism; two `DimensionKey`s are equal only when their canonical
forms match exactly — no inferred hierarchies, no fuzzy matches. **The
default is strict**: a business-level (no-dimension) `MetricValue` does
not silently resolve for a dimensioned KPI evaluation group unless a
`MetricRef`/`KPIRef` explicitly sets `Broadcast: true`
(`dimension_test.go`'s `TestDimensions_Unavailable_NoBroadcast` /
`_ExplicitBroadcast`).

## Aggregation semantics

```go
type AggregationRule string // SUM, AVERAGE, WEIGHTED_AVERAGE, LAST,
                            // MIN, MAX, NOT_AGGREGATABLE
```

An empty/unrecognized `Aggregation` resolves to `NOT_AGGREGATABLE`,
never a silent `SUM` — `aggregate_test.go`'s
`TestAggregation_DefaultIsNotAggregatable`. A margin percentage tagged
`NOT_AGGREGATABLE` is never rolled up directly; the recommended pattern
is recomputing a ratio from underlying `SUM`-aggregatable totals rather
than averaging percentages —
`TestAggregation_RecomputeRatioFromTotals` proves this end to end
(SUM(profit)/SUM(revenue), not AVERAGE(margin%)).
`WEIGHTED_AVERAGE`-tagged metrics rolled up via a bare `TRAILING_N`
reference (as opposed to the explicit `WEIGHTED_AVERAGE` `Expression`
operator, which always has its own explicit weight `Args`) have no
implicit weight source and are reported unavailable rather than
silently falling back to a plain average.

## KPI dependency graph

A KPI's `Formula` may reference another KPI's `Code` via a `KPIRef`. The
dependency graph is built once per `Calculate` call and validated for:

- unknown KPI (`IssueUnknownKPI`)
- self-reference / cycles (`IssueDependencyCycle`)
- duplicate KPI code (`IssueDuplicateKPICode`, first occurrence wins)
- KPI/metric namespace collision (`IssueNamespaceCollision`)

Evaluation order is topological, independent of `Definition` input
order, with `Code` as the deterministic tie-break —
`dependency_test.go`'s `TestDependency_InputOrderIndependence` proves
the identical result set regardless of caller order.

### Cycle handling

`A -> B -> C -> A` makes `A`/`B`/`C` unavailable
(`AvailabilityDependencyUnavailable`) with a stable
`IssueDependencyCycle` finding; an independent `D` still evaluates in
the default lenient mode. `Options.FailAllOnDefinitionError` (default
`false`) switches to a strict mode where any definition error zeroes out
every `KPIResult`. A self-referencing KPI is folded into the same
cycle-exclusion set as a multi-node cycle — this specific case had a
real bug (see [Determinism/immutability/concurrency](#determinismimmutabilityconcurrency)).

## Targets

```go
type TargetPolicy struct {
    Kind      TargetKind // MINIMUM, MAXIMUM, RANGE, EXACT
    Min, Max  float64
    Exact     float64
    Tolerance float64
}
```

Never invented — always exactly what the caller supplied, or absent.
`DSO <= 45` is `MAXIMUM{Max: 45}`; `Gross Margin >= 35%` is
`MINIMUM{Min: 35}`; `Utilization between 70% and 85%` is
`RANGE{Min: 70, Max: 85}` (both bounds inclusive). Returns
`TargetAvailable`/`TargetMet`/`Difference`/`DifferencePercent` (the last
unavailable when the relevant boundary is zero).

## Threshold bands

```go
type ThresholdBand struct {
    Label    string // caller-supplied verbatim, never invented
    Min, Max float64
}
```

`Min` inclusive, `Max` exclusive — `Min <= v < Max` — **except** the
single band whose `Max` is the overall maximum across all supplied
bands, which is closed on both ends (`Min <= v <= Max`) so that band's
own upper boundary value is actually reachable: the worked example "0–50
LOW, 50–80 MID, 80+ HIGH" requires `80` itself, and any value at the top
band's stated ceiling, to land in `HIGH`. Overlapping bands are rejected
(`IssueOverlappingThresholdBands`); a degenerate band (`Min >= Max`) is
rejected too (`IssueInvalidThresholdBand`).

## Trends

```go
type Trend struct {
    Points      []TrendPoint
    Adjacent    []Change // one per adjacent pair
    FirstVsLast Change   // Points[0] vs Points[len-1], skipping the middle
    Direction   TrendDirection // INCREASING, DECREASING, STABLE, UNAVAILABLE
}
```

Only populated when `Options.IncludeTrend` is set. `Direction` compares
`FirstVsLast.AbsoluteChange` against a caller-configurable
`Options.TrendStabilityTolerance`; this package never labels
improving/deteriorating — only numeric direction — since whether a
rising or falling KPI is "good" depends on caller-supplied semantics
this package does not have.

## Trace and provenance

`Provenance` (always populated, cheap) preserves
`SourceMetricCodes`/`DependencyKPICodes`/`SourceRefs` for every
`KPIResult`, even without full tracing. The optional, opt-in `Trace`
(`Options.IncludeTrace`) additionally records the operator, resolved
`Args` (with nested `Trace` for a KPI-reference operand), and the final
result — useful for "explain this number" UI without paying the payload
cost by default.

Both `buildProvenance` and `evaluateKPI` are memoized per `(code,
period, dimension)` within one `Calculate` call — see
[Complexity/safety limits](#complexitysafety-limits) for why this
matters, not just for efficiency.

## Definition validation vs runtime availability

Kept as two distinct Go types, never one flat list a caller has to
filter by convention:

- **`DefinitionIssue`** — a structural problem with a `Definition`
  itself, independent of any specific period/dimension (invalid
  expression, cycle, bad operator arity, invalid unit, invalid
  target/bands, unsupported expression-language version, ...).
- **`EvaluationIssue`** — a runtime problem tied to specific input data
  (unknown metric, duplicate metric value, unit/currency mismatch,
  invalid period, invalid dimension, non-finite input/result, invalid
  policy). Deliberately sparse — most runtime unavailability is
  communicated via a `KPIResult`'s own `Value.Reason` instead of a
  separate top-level issue.

## Expression-language versioning

Three independent version axes, each bumped for a different reason:

- **`SchemaVersion`** — every public type's field shape.
- **`FormulaVersion`** — every operator's/aggregation rule's/target-
  band's/trend's fixed semantics; bump only when an *existing*
  operator's meaning changes (adding a new operator a `Definition` must
  opt into does not require a bump).
- **`ExpressionLanguageVersion`** — whether a given `Expression` tree,
  as a *value*, is one this engine's validator/evaluator can accept at
  all. A `Definition` does not carry its own
  `ExpressionLanguageVersion` field in V1 (only one version exists to
  declare); a persistence layer is expected to record which version
  each stored `Definition` was authored against.
  `Options.ExpressionLanguageVersion`, if set and mismatched, rejects
  every supplied `Definition` with
  `IssueUnsupportedExpressionLanguageVersion`.

## Complexity/safety limits

```go
const (
    MaxExpressionDepth  = 64
    MaxExpressionNodes  = 2000
    MaxDependencyDepth  = 200
)
```

Generous, not tiny/arbitrary — a realistic hand-authored or even
generator-composed formula is nowhere near these ceilings; they exist
purely as a circuit breaker against pathological/adversarial input
(`fuzz_test.go`'s `FuzzExpressionEvaluation`/`FuzzDependencyGraph`
specifically fuzz for unbounded depth/complexity). `validateExpression`
measures depth/node-count once and reports
`IssueExpressionTooComplex`/`IssueDependencyTooDeep` rather than
recursing arbitrarily deep.

## Built-in templates

A small, optional library (`RevenuePerFTETemplate`,
`LaborCostPercentRevenueTemplate`, `AROver60PercentTemplate`,
`AverageTicketTemplate`, `GrossMarginTemplate`,
`RevenuePerSquareFootTemplate`, plus `BuiltInTemplates()` returning all
of them). Every template is an ordinary `Definition` value evaluated by
the exact same `Calculate` code path as any caller-authored one —
nothing here special-cases a formula internally
(`formulas_test.go`'s `TestTemplates_DoNotSpecialCase` proves a
hand-authored `Definition` with the identical shape as
`GrossMarginTemplate` produces the byte-identical `KPIResult`).

## Adapter philosophy

Where a domain package already authoritatively computes a metric (AR
DSO, AP DPO, inventory DIO, labor `RevenuePerFTE`, profitability
`ContributionMargin`, ...), this package's role is composition, custom
ratios, client/industry formulas, targets, and trend/threshold
evaluation on top of that figure — **never recomputing it**. Every
`*adapter.go` file exposes an existing module's already-computed value
as a `MetricValue`; none contains formula logic of its own. The one
partial exception, `aradapter.go`'s AR-60+ figure, sums
already-computed bucket amounts against the bucket schema's own
`MinDaysPastDue` metadata — still "expose existing computed values," not
a new formula, since every number summed was already computed by
`accounting/ar` itself; it never hard-codes a bucket-code assumption,
since a caller may configure custom aging buckets.

### Authoritative-domain-metric boundary

| Adapter | Namespace | Representative codes |
|---|---|---|
| `financialadapter.go` | `financial.*` | `financial.revenue`, `financial.gross_profit`, `financial.ebitda`, `financial.gross_margin`, `financial.ebitda_margin`, `financial.net_income`, `financial.working_capital` |
| `laboradapter.go` | `labor.*` | `labor.total_labor_cost`, `labor.fte` |
| `aradapter.go` | `ar.*` | `ar.total_open_ar`, `ar.over_60_amount`, `ar.dso` |
| `apadapter.go` | `ap.*` | `ap.total_open_ap`, `ap.dpo` |
| `inventoryadapter.go` | `inventory.*` | `inventory.total_value`, `inventory.turnover`, `inventory.dio` |
| `profitabilityadapter.go` | `profitability.*` | `profitability.contribution_margin` |
| `vendorspendadapter.go` | `vendorspend.*` | `vendorspend.total_spend` |
| `cashforecastadapter.go` | `cashforecast.*` | `cashforecast.ending_cash`, `cashforecast.maximum_funding_gap` |

This package does not enforce any particular namespace convention — a
`MetricValue.Code` is an opaque, exact-match string. A caller's own
custom app metrics never need to use these namespaces; the table above
is a convention the shipped adapters/templates happen to follow so they
compose with each other predictably (`adapters_test.go`'s
`TestAdapter_MultiModuleComposite` combines the `financial.*` and
`labor.*` adapters in one KPI formula).

## Determinism/immutability/concurrency

`Calculate` is a pure function: no I/O, no `time.Now()`, no
package-level mutable state, and it never mutates any caller-supplied
`Definition`/`Expression`/`MetricValue`/`Period`/`DimensionKey`/
`TargetPolicy`/`ThresholdBand` (`safety_test.go`'s
`TestImmutability_Input`). Every ordering-sensitive output (periods,
dimension groups, issues, dependency traversal) is explicitly sorted
rather than relying on Go map order. Repeated `Calculate` calls against
identical, unmodified input — including concurrently, from multiple
goroutines — always return byte-for-byte identical JSON
(`TestDeterminism_RepeatedCalls`, `TestConcurrency_ParallelCalculate`
under `-race`).

Three real bugs surfaced building this package, each found by a
different verification technique and each with a permanent regression
test:

1. **A self-referencing KPI (`A -> KPIRef{A}`) was never actually
   marked `inCycle`** — only reported as a `DefinitionIssue`. Evaluation
   stayed accidentally safe only because `evaluateKPI` has its own
   independent recursion guard local to its own call tree; a later,
   separately-motivated performance rewrite of `buildProvenance`
   (memoization — see below) trusted `inCycle` as the single source of
   truth and had no such guard, causing a real stack-overflow crash the
   first time this exact shape was exercised. Fixed by tracking
   self-references explicitly and folding them into `inCycle` alongside
   `detectCycles`' own findings.
   `dependency_test.go`'s `TestDependency_SelfReference_
   MustBeMarkedInCycle` is a permanent regression test, verified
   red→green by temporarily disabling the fix.
2. **`buildProvenance` was unmemoized**, making a KPI-of-KPI chain of
   length N cost O(N²) total (each of the N `KPIResult`s independently
   re-walking its own O(N)-deep chain from scratch) — found via
   `BenchmarkDependencyDepth`'s superlinear scaling. Fixed with a
   `provenanceCache` on `evalContext`, mirroring `evaluateKPI`'s
   existing `kpiCache` memoization pattern exactly.
3. **`topologicalOrder` called `sort.Strings(ready)` on every
   Kahn's-algorithm iteration** instead of once, making a large set of
   mutually-independent KPI definitions (all `inDegree == 0` at once —
   the realistic "many unrelated KPIs" shape) cost O(n² log n) instead
   of O(n log n) — found via CPU profiling
   (`BenchmarkCalculate_ScalingCheck`/`BenchmarkCalculate_
   10000Definitions`, ~76% of total CPU time in one function). Fixed
   with a `container/heap` min-heap keyed by `Code`, preserving the
   identical Code-tie-break output contract
   (`TestDependency_TopologicalOrder_CodeTieBreak`).
4. A related correctness bug was found while investigating (1)/(2)'s
   performance work: **`resolveTimeTarget`'s `PRIOR_YEAR_SAME_PERIOD`
   case scanned forward from the start of the period list and returned
   the *first* match**, which is the *earliest* prior fiscal year
   sharing a position, not the *nearest* one, whenever more than one
   qualified. Fixed by indexing periods once
   (`periodIndexByCode`) and scanning backward from the evaluated
   period's own index, so the first match found walking backward is
   correctly the nearest — this also made the lookup O(1)/O(k) instead
   of O(n), closing a real hotspot (`resolveTimeTarget` was ~34% of
   total CPU time on `BenchmarkCalculate_100000MetricValues` before the
   fix, ~10x faster after). See
   `TestPeriods_PriorYearSamePeriod_NearestNotEarliest`.

## JSON

Every public contract round-trips through JSON with semantic equality
(`json_test.go`) and explicit `snake_case` tags. `Expression` argument
ordering is stable across a round-trip. No `NaN`/`Inf` is ever
serialized — every operator guards its own result first.

## Fuzzing results

Four short, laptop-safe fuzz targets (`fuzz_test.go`), each run for a
15–30s live session plus the recorded regression corpus on every normal
`go test` run:

| Target | What it exercises | Executions (live run) |
|---|---|---|
| `FuzzExpressionEvaluation` | Arithmetic operators over arbitrary values/availability | ~1.9M |
| `FuzzDependencyGraph` | KPI-to-KPI edge combinations, cycle detection | ~0.9M |
| `FuzzThresholdBands` | Band boundary combinations | ~1.5M |
| `FuzzUnitCompatibility` | `combine*` over arbitrary `UnitKind` pairs | ~1.7M |

One real bug found: `combineMultiplicative`/`combineAdditive`/
`combineDivisive` did not themselves reject a structurally invalid input
`Unit` (e.g. `UnitCurrency` with an empty `CurrencyCode`), only
propagating whatever shape was handed to them. Fixed at two layers —
`buildMetricIndex` now rejects an `Available` `MetricValue` with an
invalid `Unit` (`IssueInvalidUnit`) at the point a caller actually
supplies one, and the `combine*` primitives themselves now also
independently refuse any invalid input, so they are safe standalone and
not merely safe by caller discipline.

## Benchmark/scaling results

Representative default scales (laptop-safe, foreground only):

| Benchmark | Scale | Time |
|---|---|---|
| `BenchmarkCalculate_1000Definitions` | 1,000 independent KPIs | ~0.5ms |
| `BenchmarkCalculate_10000Definitions` | 10,000 independent KPIs | ~84ms |
| `BenchmarkCalculate_100000MetricValues` | 100 KPIs × 1,000 periods | ~341ms |
| `BenchmarkCalculate_100PeriodDimensionGroups` | 10 periods × 10 dims | ~1.1ms |
| `BenchmarkDependencyDepth_1000` | 1,000-deep KPI-of-KPI chain | ~8ms |

`BenchmarkCalculate_ScalingCheck` runs the independent-KPI workload at
1,000/2,000/4,000/8,000 in one benchmark specifically to make
superlinear scaling visible in `go test -bench` output directly; after
the three fixes above, doubling N roughly doubles time (2.1–2.2x per
doubling), confirming near-linear scaling. No larger stress benchmark is
gated behind an env var in V1 — the scales above were sufficient to
surface (and, after fixing, confirm the absence of) every real
superlinear-scaling defect found; a future prompt can add a
`KPI_FULL_SCALE_BENCH=1`-gated benchmark if a specific larger scale
becomes a concern.

## Limitations

- **No string formula DSL** — see
  [Typed AST](#typed-ast--the-v1-persistence-contract).
- **`TRAILING_N` is not supported on a `KPIRef`** (only `MetricRef`) —
  trailing-N KPI-of-KPI aggregation was judged to materially complicate
  V1 (task instruction: "if trailing-N materially complicates V1, defer
  it and document") and is deferred; a `KPIRef` with `Time:
  TimeRefTrailingN` is a definition error.
  `COALESCE` intentionally has no implicit fallback — a caller wanting
  a default value composes it explicitly.
- **No cross-dimension rollup logic beyond exact match and explicit
  `Broadcast`** — no inferred hierarchy (e.g. a "region" containing
  multiple "locations").
- **Unit compatibility is a small explicit system, not a symbolic units
  engine** — a combination this package does not recognize (e.g.
  `Hours / Count`) is reported incompatible even if a caller could
  reasonably define a meaning for it; the caller sets `Definition.Unit`
  explicitly and accepts that `checkExpectedOutputUnit`'s
  declared-vs-actual check will not itself infer that unit through
  `DIVIDE`.

## Possible future string DSL

If added, it will parse into the identical `Expression` AST this
package already validates/evaluates, will carry its own, separately
versioned grammar (a new `ExpressionLanguageVersion`, never silently
reusing `"1.0.0"`), and will never introduce a second evaluation path
alongside the typed-AST one described here.
