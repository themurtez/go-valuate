# Analytics Suite Architecture

A hardening/contract-freeze reference for the analytics/transactions/
portfolio/reporting expansion (20 packages) built on top of the V1
valuation core. See [`V1_CONTRACTS.md`](V1_CONTRACTS.md) for that core's
own freeze pass, [`ANALYTICS_MODULES.md`](ANALYTICS_MODULES.md) for a
per-package catalog, and [`ANALYTICS_VERSIONS.md`](ANALYTICS_VERSIONS.md)
for the version inventory. Findings below are verified directly against
source (`go build`/`go vet`/`go list`/direct file reads), not assumed.

## Dependency graph

### Internal dependency table

| Package | Depends on (this module only) | Classification |
|---|---|---|
| `analytics/qoe` | `financial`, `financial/adjustments`, `financial/earnings`, `financial/metrics` | (a) financial-only |
| `analytics/workingcapital` | `financial` | (a) financial-only |
| `analytics/ratios` | `financial`, `financial/metrics` | (a) financial-only |
| `analytics/cashflow` | `analytics/workingcapital`, `financial`, `financial/metrics` | (b) sibling dep: `workingcapital` |
| `analytics/revenuequality` | `financial` | (a) financial-only |
| `analytics/concentration` | `financial` | (a) financial-only |
| `analytics/anomalies` | `financial` | (a) financial-only |
| `analytics/variance` | `financial` | (a) financial-only |
| `analytics/forecast` | `analytics/workingcapital`, `financial` | (b) sibling dep: `workingcapital` |
| `analytics/debt` | *(none)* | (a) financial-only (zero internal imports) |
| `analytics/covenants` | `financial` | (a) financial-only |
| `analytics/benchmarks` | `financial` | (a) financial-only |
| `analytics/valuedrivers` | `financial`, `financial/adjustments`, `financial/metrics`, `settings`, full `valuation/*` closure | (c) valuation dependency |
| `transactions/acquisition` | `analytics/debt` | (b) sibling dep: `debt` |
| `transactions/dealstructure` | *(none)* | (a) financial-only (zero internal imports) |
| `transactions/salereadiness` | `analytics/concentration`, `analytics/qoe`, `analytics/revenuequality`, `analytics/workingcapital`, `financial`, `financial/adjustments`, `financial/earnings`, `financial/metrics`, `valuation`, `valuation/basis`, `valuation/consensus`, `valuation/profile` | (b)+(c) sibling + valuation |
| `analytics/consolidation` | `financial` | (a) financial-only |
| `portfolio/diagnostics` | `financial` | (a) financial-only — consumes caller-populated condensed summaries, not sibling packages directly |
| `reporting/management` | 11 sibling analytics packages, `financial`, `financial/adjustments`, `financial/earnings`, `financial/metrics`, `valuation`, `valuation/basis`, `valuation/consensus` | (d) aggregator |
| `analytics/diagnostics` | 13 sibling analytics packages, `transactions/salereadiness`, `financial`-family, `settings`, full `valuation/*` closure (via `valuedrivers`) | (d) aggregator — largest fan-out in the module |

### Verified properties

- **No import cycles.** Confirmed via `go build ./...` (which would fail on any cycle) and a reverse-import check: nothing imports `portfolio/diagnostics`, `analytics/diagnostics`, or `reporting/management` — all three are pure consumers, never consumed. `transactions/salereadiness` is imported only by `analytics/diagnostics`, one-directionally.
- **No third-party imports** in any of the 20 packages — every import is either Go standard library or `github.com/themurtez/go-valuate/...`. Confirmed via `go list -f '{{join .Imports "\n"}}'` per package.
- **No `func init()` anywhere** in the 20 packages.
- **No hidden application-layer coupling.** No package imports anything related to a database, HTTP handler, auth/session, or billing concept — the 20 packages' only external-facing dependency is the Go standard library.
- **No hidden global provider/registry pattern.** Only two exported package-level `var`s exist anywhere in the 20-package scope: `portfolio/diagnostics.DefaultPolicy` and `analytics/diagnostics.DefaultPolicy`, both plain default-value structs resolved through a `resolvePolicy`-style function, never a swappable-implementation registry. (`portfolio/diagnostics.DefaultPolicy` contained a real map-aliasing hazard, since fixed — see § Nil/zero-value safety below.) Every other package-level `var` found (~25 across the 20 packages) is an unexported, read-only lookup table (code classification lists, fixed declaration-order slices, rank tables) built once from literals and never mutated at runtime.
- **Architectural direction is consistent and one-way**: `financial/*` → leaf analytics/transaction packages → mid-tier packages that depend on one sibling (`cashflow`→`workingcapital`, `forecast`→`workingcapital`, `acquisition`→`debt`) → aggregators (`salereadiness`, `reporting/management`, `analytics/diagnostics`). `portfolio/diagnostics` sits conceptually alongside the aggregators but is architecturally a leaf (it never imports another analytics package — callers populate its condensed `Summary` structs from their own already-computed sibling `Result`s).
- **`analytics/valuedrivers` and `analytics/diagnostics`** are the only two packages in this scope that pull in the full `valuation/*` closure (`valuedrivers` directly, by re-running the orchestrator; `diagnostics` transitively, via consuming `valuedrivers.Result`).

Two sibling dependencies are worth calling out by name as intentional,
even though they're the kind of "one leaf package quietly depending on
another" coupling a dependency audit typically flags:

- `analytics/cashflow` imports `analytics/workingcapital` directly (not just the type shape) to derive `ChangeInNWC` for its EBITDA-to-FCF bridge — a genuine shared computation, not a duplicated one. **Not removed** — it's the one case in this audit of true reuse rather than parallel reimplementation.
- `transactions/acquisition` imports `analytics/debt` for its DSCR figure, reusing `debt`'s loan-amortization/coverage math rather than reimplementing it. **Not removed** — same reasoning.

## Common primitive audit

Repeated concepts found across 3+ packages, classified per this task's
rule: **A** = correctly domain-local (shapes/meanings differ enough that
sharing would be wrong), **B** = semantically identical and genuinely
shareable, **C** = superficially similar but intentionally distinct.
Extraction only happens where classification is B *and* all of: identical
semantics, 3+ packages benefit, no cycle risk, JSON compatibility
preserved, and the shared code is smaller than the duplication it
replaces. **No extraction met that bar in this pass** — every B-classified
concept below is documented as a considered-but-declined extraction, not
silently duplicated.

### Value / MetricValue / AvailableValue

Two field-naming variants exist, split cleanly by role:

- **Variant A — domain-named type, field named `Value`** (packages that derive their own domain-specific figure): `workingcapital.NWCValue`, `revenuequality.RevenueValue`, `concentration.ConcentrationValue`, `anomalies.AnomalyValue`, `variance.VarianceValue`, `forecast.ForecastValue` — all `{Available bool, Value float64}`. `cashflow.CashFlowValue` adds `IsEstimate bool, EstimateBasis string` (provenance tracking a plain 2-field type can't express). `valuedrivers.MethodValue` adds `ValueType valuation.ValueType` (a value-basis tag meaningless outside a valuation-method context).
- **Variant B — generic `Value` type, field named `Amount`** (packages that pass through/aggregate already-computed figures rather than deriving their own): `debt.Value`, `covenants.Value`, `benchmarks.Value`, `acquisition.Value`, `dealstructure.Value`, `salereadiness.Value`, `portfolio/diagnostics.Value`, `reporting/management.Value`, `analytics/diagnostics.Value` — all `{Available bool, Amount float64}`.
- `analytics/qoe` and `analytics/ratios` reuse `financial/metrics.MetricValue` directly rather than defining a local copy.

**Classification: B for the 15 plain 2-field instances** (identical shape and meaning: distinguish "computed and equal to 0" from "unavailable"). **Not extracted** — the `Amount`-vs-`Value` naming split exists purely to avoid a `Value.Value` stutter, and a shared type would need to live in a new leaf package every one of these 15 already-independent packages would then depend on, for a saving of ~3 lines each. **Classification: C for `cashflow.CashFlowValue` and `valuedrivers.MethodValue`** — each carries a genuinely load-bearing extra field the shared shape can't express.

### Issue / IssueCode / Severity

Three struct shapes, all sharing a `{Code, Severity, Message}` core:

- **Shape 1 — bare 3-field**: 11 packages (`qoe`, `workingcapital`, `ratios`, `cashflow`, `revenuequality`, `concentration`, `anomalies`, `variance`, `salereadiness`, `portfolio/diagnostics`, `reporting/management`).
- **Shape 2 — 3-field core + one scoping field, each package-specific**: `debt.Issue{..., Loan string}`, `covenants.Issue{..., CovenantID string}`, `benchmarks.Issue{..., MetricID string}`, `acquisition.Issue{..., Tranche string}`, `dealstructure.Issue{..., Tranche string}` (same field name as `acquisition`'s but a different index space — not interchangeable), `analytics/diagnostics.Issue{..., Module SourceModule}` (a typed field, not a raw string index).
- **Shape 3 — 5-field, two scoping fields**: `forecast.Issue{Code, Severity, Scenario, Period, Message}`, `consolidation.Issue{Code, Severity, EntityID, Period financial.Period, Message}` (note: `Period` here is the typed `financial.Period`, not a bare string).
- A parallel, differently-named family: `valuedrivers.DriverIssue{Code DriverIssueCode, Severity DriverIssueSeverity, Message, DriverID}` — structurally Shape 2, but under its own type names rather than reusing `IssueCode`/`IssueSeverity`.
- Severity constant naming has a cosmetic two-tier split: 14 "leaf" packages use bare `SeverityError`/`SeverityWarning`; the 4 aggregator-tier packages (`salereadiness`, `portfolio/diagnostics`, `reporting/management`, `analytics/diagnostics`) use `IssueSeverityError`/`IssueSeverityWarning`. Both resolve to the identical 2-value `IssueSeverity string` enum everywhere.

**Classification: A for the whole family.** The shared 3-field core is real, but every package's actual `IssueCode` *values* are irreducibly package-specific (dozens per package, zero semantic overlap), and the scoping-field packages each need a genuinely different field of a genuinely different type. Extracting the struct shell alone would save a few lines per package at the cost of a new cross-package type dependency, for a family whose entire value is in its per-package enum content.

### TrendDirection

**6-package identical 4-value model** (`Increasing`/`Declining`/`Stable`/`Unavailable`, string values `"increasing"/"declining"/"stable"/"unavailable"`): `workingcapital.TrendDirection`, `ratios.RatioTrendDirection`, `cashflow.TrendDirection`, `revenuequality.TrendDirection`, `concentration.TrendDirection`, `variance.TrendDirection`.

**Classification: B — the strongest extraction candidate in this audit, and still not extracted.** No field-shape ambiguity at all (a bare string type), identical values, identical meaning (first-vs-last-observation direction with a flat-band threshold for "stable") across all 6, and every package's own doc comment already treats it as one shared concept in prose ("mirrors X's identical model") while maintaining 6 separate Go type identities in code. Not extracted in this pass because doing so would change 6 already-frozen `Result`-reachable field types — a real, if narrow, breaking change this task's "avoid unnecessary breaking changes" instruction weighs against for marginal benefit. **Recorded here as the one item most worth extracting in a future major-version pass**, not silently duplicated without comment.

Two *different* concepts share the word "Direction" but are correctly **classification C**, not part of the above group: `benchmarks.Direction` (a caller-*stated preference*, e.g. "higher is better" — unrelated to any time series) and `portfolio/diagnostics.Direction` (a *normalized aggregation output* that resolves the "good direction" sign-flip across heterogeneous source trends — e.g. rising customer concentration is bad while rising cash conversion is good, so this package needs a source-agnostic replacement enum, not a reused `TrendDirection`).

### PeriodMeta / period-metadata handling

The canonical type is `financial/metrics.PeriodInfo{Type PeriodType, FiscalYear int, SequenceInYear int}`.

- **Group 1 — reuses `metrics.PeriodInfo` directly**: `analytics/qoe`, `transactions/salereadiness`, `reporting/management`, `analytics/diagnostics`.
- **Group 2 — defines its own field-for-field-identical local `PeriodInfo`/`PeriodType` duplicate**: `analytics/workingcapital`, `analytics/ratios`, `analytics/revenuequality`, `analytics/concentration`, `analytics/anomalies`, `analytics/variance`, `analytics/forecast`.
- **Group 2 outlier**: `analytics/cashflow` defines the same local duplicate type as Group 2, but its actual `Input.PeriodMeta` field is typed `map[financial.Period]metrics.PeriodInfo` (Group 1 style) — the local type is used only as an internal conversion target when calling into `analytics/workingcapital` (which is itself Group 2). This is a real, if minor, inconsistency: a reader would reasonably expect `cashflow.Input.PeriodMeta` to be `cashflow.PeriodInfo`, and it is not.
- **Group 3 — no `PeriodMeta` field at all**, architecturally correctly: `debt`, `covenants`, `benchmarks`, `valuedrivers`, `acquisition`, `dealstructure`, `consolidation`, `portfolio/diagnostics` all take pre-computed figures rather than a raw dataset, so there is no period-ordering question for them to answer.

**Classification: B for the Group-2 7-package duplicate — the second-strongest extraction candidate, also not extracted for the same reason as `TrendDirection`.** Unlike the `Value`/`Issue` families, the doc comments here don't even attempt a semantic-difference justification for the duplication — they state it as a stylistic choice. The fact that Group 1 already demonstrates the "just reuse `metrics.PeriodInfo`" alternative works in production, in 4 packages, makes Group 2's duplication read as drift rather than a considered per-package decision. **`analytics/cashflow.PeriodInfo` specifically is flagged as the one item worth cleaning up first** in any future pass (either by having it adopt `metrics.PeriodInfo` directly, removing the conversion function, or documenting why the extra hop is load-bearing) — not fixed here, since it's a live, working module and this task avoids refactoring for aesthetics alone.

### Coverage (module-availability tracking)

Four distinct shapes, correctly **classification C** as a family:

- `salereadiness.Coverage{TotalDimensions, AssessedDimensions, CoveragePercent}` — 2 ints + 1 float, no per-item breakdown (its 11 dimensions are already enumerable via `Result.Dimensions`).
- `analytics/diagnostics.Coverage` and `reporting/management.Coverage` share a "3 summary fields + N `WithX bool` flags" shape, but enumerate entirely different, non-overlapping module sets (15 vs. 12 modules) — collapsing them would need either a lossy shared subset or a `map[string]bool` that discards every field's own doc comment and JSON-key stability.
- `portfolio/diagnostics.CoverageCounts` counts **businesses in a portfolio** that had each summary available (integer counts only, no percent field) — a genuinely different unit of counting ("how many businesses had a QoE summary" vs. "was the QoE module available"), correctly given a different type name.

### Scenario

Four genuinely distinct concepts sharing only the name — **classification A**, no extraction considered: `concentration.Scenario` is a **computed output** (an entity-loss simulation result); `forecast.Scenario` is a **caller-supplied full assumption bundle** for a from-scratch projection; `debt.DownsideScenario` is a **caller-supplied stress haircut** applied to already-known figures; `valuedrivers.Scenario` is a **caller-supplied ordered driver-mutation list**. No two share more than the word "scenario."

### Summary

Every `*Summary`-suffixed type aggregates a different set of source fields for a different purpose — **classification A** across the board, confirmed by direct inspection (`qoe.RecurringSummary`, `revenuequality.ConcentrationSummary`, `anomalies.Summary`, `variance.CategorySummary`/`TrendSummary`, `covenants.Summary`/`PeriodSummary`, `benchmarks.Summary`, `portfolio/diagnostics.QoESummary`/`RatioHealthSummary`/`ConcentrationSummary`/`CashFlowSummary`/`ValuationSummary`/`SaleReadinessSummary`, `reporting/management.ExecutiveSummary`/`TopIssuesSummary`). **One naming collision worth documenting**: `analytics/revenuequality.ConcentrationSummary` and `portfolio/diagnostics.ConcentrationSummary` share a name but have completely different shapes (a period-level customer-revenue detail record vs. a portfolio-digest struct) — not a bug, since they're in different packages, but a reader searching the module for "ConcentrationSummary" should know both exist.

## Chronology / period-ordering audit

Audited: `qoe`, `workingcapital`, `ratios`, `cashflow`, `revenuequality`,
`anomalies`, `variance`, `forecast`, `concentration`, `portfolio/diagnostics`,
`reporting/management`.

**Repo-wide convention, held consistently in 8 of 9 dataset-driven
packages** (`qoe`, `workingcapital`, `ratios`, `cashflow`,
`revenuequality`, `anomalies`, `variance`, `concentration`): missing or
partial `PeriodMeta` is never fatal. Each package falls back to dataset
lexical/encounter order for the specific outputs that need chronology,
emits an advisory `IssueNoPeriodMeta`/`IssuePeriodMissingFromMeta`
warning, and leaves only the ordering-dependent fields
unavailable/degraded — every non-ordering-dependent output still
computes. This is implemented as a `chronologicalPeriods(...)` helper,
**independently reimplemented in each of the 8 packages** (different
signatures: some return `(ordered, *Issue)`, `qoe` returns `(ordered,
bool)`, `anomalies` returns a 3-tuple with its own `orderedPeriod` wrapper
type) rather than shared — this is the same "duplicated-per-package
helper, not extracted" pattern documented in § Common primitive audit
above, for the same reasons (each package's own `Issue` type differs, so
sharing the helper would require sharing that type too).

**One deliberate, documented exception: `analytics/forecast`.**
`forecast.Input.PeriodMeta`'s own doc comment states plainly: unlike every
sibling, this package cannot identify a base period to project *from* at
all without chronological order, so missing/partial `PeriodMeta` makes
the *entire* `Result` unavailable (`IssueNoPeriodMeta` at
`SeverityError`) rather than degrading gracefully. Verified correct and
necessary, not an oversight: `forecast.Input.PeriodMeta`'s own doc comment
explains the reasoning, and `resolveBasePeriod`'s implementation matches
it exactly (returns an error `Issue` immediately, never falls back to a
lexically-last period that might not be chronologically last).

`portfolio/diagnostics` and `reporting/management` use different,
correctly domain-appropriate ordering rules rather than
`chronologicalPeriods`: `portfolio/diagnostics.Findings` orders by
`PriorityScore` descending then a fixed `FindingCode` declaration order
then `BusinessID` (not by period at all — it operates across businesses,
not across one business's periods); `reporting/management`'s chart/table
sections each state their own fixed ordering (chronological historical
periods, then `Input.Forecast.ForecastPeriods`' own order, for series
that span both).

**No package forces FY-only behavior onto an arbitrary-period module.**
Every package accepting `PeriodType` supports `fiscal_year`/`ytd`/
`quarter`/`month` identically; `workingcapital.SeasonalProfile` is the one
output that specifically requires quarter-or-month granularity (a
seasonal position within a year is meaningless for an annual figure), and
this is documented as a real domain constraint, not an arbitrary
restriction.

## Floating-point determinism audit

**One real bug found and fixed**: `analytics/workingcapital`'s
`calculateSeasonalProfile` accumulated `SeasonalPeriod.AverageNWCPercentOfRevenue`'s
bucket sum by ranging over a map built from its own `history` parameter,
instead of ranging `history` itself — genuine Go map-iteration-order
float64 sum-order nondeterminism. Fixed to range the slice directly; a
regression test using a Go-verified (not merely reasoned-about)
order-sensitive float64 triple was added — see `analytics/workingcapital/seasonal_test.go`.

**Everything else checked came back clean.** A repository-wide sweep for
`range`-over-map combined with float64 accumulation, across all 20
packages, found no other genuine case — every other candidate traced to
one of: (a) the map is ranged only to collect keys before a `sort.Slice`/
`sort.Strings` call (the repo-wide "map for lookup, sorted slice for
accumulation" discipline, held everywhere else it was checked, including
HHI in both `concentration` and `revenuequality`, category/segment
rollups, consolidation elimination/contribution sums, portfolio/diagnostics
and analytics/diagnostics scoring, benchmark peer aggregation, and
forecast's COGS/revenue projection), or (b) the accumulation is on an
int counter (excluded by this audit's own scope), or (c) the map-range
only performs idempotent boolean-set writes.

## Availability / missing-data semantics audit

**Design confirmed sound overall.** Every one of the 20 packages uses an
`Available`+numeric-field wrapper type for every optional result field
(see § Common primitive audit) — never a bare `float64` with a sentinel
value standing in for "missing." Every wrapper type's zero value is
`{Available: false}`, so a bare map lookup on a wrapper-typed value
(`v := m[key]`) is safe by construction even without an explicit `, ok`
check, since a missing key and an explicit "supplied but unavailable"
entry are indistinguishable in exactly the way that's meant to be
indistinguishable (both mean "no figure here").

**Four real gaps found and fixed, all the identical class of bug**: an
`Available: true` wrapper whose numeric payload was NaN or +/-Inf flowed
unguarded into arithmetic, because every package's validation checked
`!Available` but never checked *validity* of an available figure.
Discovered via targeted fuzzing (see § Fuzz testing below), fixed in
`analytics/benchmarks` (3 related sub-cases: invalid `CompanyValue`,
invalid `BenchmarkMedian`, and an overflow from two individually-valid
extreme values), `analytics/covenants` (invalid `Actual` or `Threshold`),
`transactions/dealstructure` (9 caller `Value` fields, fixed with one
comprehensive `sanitizeInput` pass rather than per-site patches), and
`analytics/consolidation` (an individually-valid currency rate whose
*product* with a large raw amount overflows). Each fix adds a new,
precisely-scoped `IssueCode` and a regression test — see
§ Issue/error taxonomy below for the new codes, and each package's own
`fuzz_test.go`/regression test for the exact scenario.

**One real "degenerate zero threshold" gap found and fixed**:
`analytics/concentration`'s 5 flag functions had no `> 0` guard on their
`Thresholds` fields (unlike `analytics/debt`'s identical sibling pattern),
so a caller setting one `Thresholds` field while leaving a sibling field
at Go's natural zero (a real risk, since `resolveThresholds` only
substitutes defaults for a *wholly* zero-value struct — a repo-wide
convention, not unique to this package) got a threshold of exactly 0,
which fired on almost any period. Fixed with defensive `<= 0` gates
mirroring `debt`'s existing pattern; regression test confirmed 4 of 5
flags fired spuriously pre-fix.

**One real map-aliasing hazard found and fixed** (not a NaN/availability
bug, but the same "available-looking value that's actually unsafe" class
of issue): `portfolio/diagnostics.resolvePolicy`'s zero-value-`Policy`
branch assigned `DefaultPolicy.SeverityWeights` directly into the
returned `Result.Policy.SeverityWeights` — a map-header copy, not a value
copy. A caller (or a concurrent caller) mutating that returned map could
corrupt the shared package-level default for every future call. Fixed to
always build a fresh map; verified with both a sequential and a real
concurrent-goroutine regression test under `-race`.

**"Not applicable" vs. "unavailable" is correctly distinguished in most
places it matters**, with one real gap found and fixed:
`analytics/covenants.WarningBufferStatus`'s `WarningBufferNotApplicable`
conflated two different causes (a test that already failed, vs. a passing
`OperatorEQ` test where `Headroom` is structurally uncomputable) into one
enum value — fixed by splitting off a new `WarningBufferHeadroomUnavailable`
value. `analytics/benchmarks.Favorable`, by contrast, already got this
right before this pass: `FavorableNotApplicable` is distinct from the
comparison itself being unavailable (`Comparison.Available`), with a
clear doc comment on when each applies.

## Issue/error taxonomy inventory

Every `IssueCode`/`Severity`/`Status`-shaped enum in all 20 packages uses
an explicit `type X string` with literal string constants — **zero
`iota`-based typed ints found anywhere in this scope**, confirmed via a
repository-wide grep. This means no enum here carries a silent-renumbering
risk from a future field reordering; every code is already the stable,
machine-readable string a JSON consumer needs.

### New codes added by this hardening pass

| Package | New code | Meaning |
|---|---|---|
| `analytics/covenants` | `WarningBufferHeadroomUnavailable` | A passing `OperatorEQ` test where `Headroom` is structurally uncomputable — distinct from an already-failed test |
| `analytics/covenants` | `IssueInvalidActualOrThreshold` | `Actual`/`Threshold` was available but NaN/Inf |
| `analytics/benchmarks` | `IssueInvalidCompanyValue` | `CompanyValue` was available but NaN/Inf |
| `analytics/benchmarks` | `IssueInvalidBenchmarkMedian` | The resolved benchmark median was NaN/Inf |
| `analytics/benchmarks` | `IssueRelativeDifferenceOverflow` | Two individually-finite values produced a NaN/Inf `Difference`/`RelativeDifference` |
| `transactions/dealstructure` | `IssueInvalidValue` | One or more of 9 caller `Value` fields was available but NaN/Inf |
| `analytics/consolidation` | `IssueCurrencyConversionOverflow` | An individually-valid currency rate's product with a raw amount overflowed |

### Cross-package code-string collisions (confirmed consistent, not conflicting)

Every UPPER_SNAKE_CASE `IssueCode` literal that recurs across 2+ packages
was checked for meaning consistency. All are consistent:
`NO_PERIODS`/`NO_PERIOD_META`/`PERIOD_MISSING_FROM_META` (identical
"chronology unavailable, degrade gracefully" meaning in 7-9 packages
each — see § Chronology audit), `NO_REVENUE_DATA` (`workingcapital`,
`revenuequality`), `INVALID_LOAN_TERMS` (`debt`, `acquisition` —
`acquisition`'s own doc comment states it applies `debt`'s validation
rules identically, by design), `METRICS_UNAVAILABLE`/
`WORKING_CAPITAL_UNAVAILABLE`/`CONCENTRATION_UNAVAILABLE`/
`CONSENSUS_UNAVAILABLE`/`QOE_UNAVAILABLE`/`REVENUE_QUALITY_UNAVAILABLE`
(`salereadiness`, `reporting/management` — both mean the named upstream
module was unavailable), `NO_INPUT_SUPPLIED` (`salereadiness`,
`reporting/management`, `analytics/diagnostics` — all mean every optional
`Input` field was left zero-value), `SCENARIO_BREACHES_DSCR` (`debt`,
`acquisition` — same downside-scenario-DSCR-breach meaning, both feeding
the same `analytics/diagnostics` mining table).

**One same-package (not cross-package) collision worth a footnote**:
`analytics/diagnostics` uses the literal string `"MODULE_UNAVAILABLE"` as
both a `FindingCode` value and, separately, an `IssueCode` value — related
but structurally distinct concepts (an output finding vs. an input
diagnostic). Go's type system keeps the two apart at the API level; a
flattened/JSON-only consumer filtering generically on the bare string
could not distinguish them without also checking which JSON object
they're nested under. Not changed — this is exactly the situation
`FindingCode`'s own doc comment already documents (several sibling codes
mapping onto shared `analytics/diagnostics` identities is the intended
design), and no live JSON contract in this repository relies on the two
strings being globally unique.

**One deliberate three-way naming divergence worth documenting**: the
string `"MARGIN_DETERIORATION"` is a real constant in both
`analytics/anomalies.RuleCode` and `portfolio/diagnostics.FindingCode`
(same meaning, margin decline), while `analytics/diagnostics` — a third,
closely related package — deliberately renames the same underlying
concept to `FindingMarginPressure = "MARGIN_PRESSURE"` instead, with its
own mapping table explicitly translating `anomalies`' rule literal onto
that different string. Each package's choice is independently justified
in its own source; documented here so the divergence reads as intentional
rather than an inconsistency to fix.

## Cross-module semantic consistency

Definitions verified directly against source (not inferred from names)
for every concept this task named:

### EBITDA / SDE

- **`analytics/qoe`** recomputes `financial/metrics.Snapshot` itself from `Dataset`/`PeriodMeta` (never accepts a pre-computed snapshot) — the one package that always derives EBITDA/SDE from the ground up.
- **`analytics/cashflow`** also recomputes `metrics.Snapshot` for EBITDA, reusing the identical `metrics.Calculate` call `qoe` makes (not a second implementation).
- **`analytics/ratios`, `analytics/debt`, `analytics/covenants`, `analytics/benchmarks`, `transactions/acquisition`** all take an already-computed maintainable EBITDA/SDE as a plain caller-supplied figure — none of them re-derives it.
- **`analytics/valuedrivers`** is the only package that runs the full valuation `orchestrator`/`consensus` pipeline (which itself consumes a caller-supplied maintainable EBITDA/SDE, one layer further upstream).

No definitional drift found: every package that consumes a maintainable-earnings figure treats it as an opaque caller-supplied number, never silently re-deriving or reinterpreting it.

### Debt / net debt

- **`analytics/ratios`** uses `financial/metrics.Snapshot.NetDebt` directly — the canonical financial/metrics definition (gross debt minus cash, computed from `Dataset` line items).
- **`analytics/debt`** independently resolves its own debt balance via `resolveTotalDebtBalance` (caller-supplied `ExistingDebtBalance`, or summed `AmortizationSchedule`s) minus `CashAndEquivalents`, arriving at the same "gross debt minus cash" definition through a **different resolution mechanism** — necessarily so, since `debt` has no `FinancialDataset` dependency and must support hypothetical/proposed debt structures that don't exist in any historical dataset.
- **`transactions/acquisition`** reuses `analytics/debt`'s coverage machinery directly for DSCR (see § Dependency graph) rather than a third implementation.
- **`analytics/valuedrivers`** never computes net debt itself; a `DriverDebtChange` mutation targets a caller-named debt-bridge field on the underlying valuation `Request`.

**Both `ratios` and `debt` land on the identical definition of net debt (gross debt − cash) through independently-implemented resolution chains** — this is architecturally correct (each needs a different data source), and is exactly the kind of pair this task's § Cross-module invariants calls out as a good candidate for an equivalence test under matching inputs (see § Invariants below).

### Working capital

- **`analytics/workingcapital`** is the sole source of *historical* NWC analysis, under a caller-supplied `InclusionPolicy`.
- **`analytics/cashflow`** imports `workingcapital.PeriodNWC` directly for `ChangeInNWC` — true reuse, not reimplementation (see § Dependency graph).
- **`analytics/forecast`**'s "NWC" (`WorkingCapitalPeriodAssumption`/`projectWorkingCapital`) is a **forward-looking projection assumption**, not a recomputation of historical NWC — a genuinely different concept (a caller-supplied target, not a derived-from-history figure), correctly not reusing `workingcapital`'s analysis machinery, though it does reuse `workingcapital.InclusionPolicy` (via `DefaultInclusionPolicy()`) to derive its own *starting* NWC from history before projecting forward.
- **`transactions/acquisition`** takes `WorkingCapitalRequirement` as a plain caller-supplied `Value`, explicitly documented as "may come from a caller's own `analytics/workingcapital` analysis" — a deliberate composition boundary, not a missing dependency.
- **`analytics/valuedrivers`** never computes NWC; `DriverWorkingCapitalChange` names a caller-supplied bridge-field mutation, same pattern as its debt driver.

No drift found — the "history vs. projection vs. caller-external figure" split is clean and consistently applied.

### Revenue growth

- **`analytics/qoe`** copies `RevenueGrowth` verbatim from `financial/metrics.Trend` — never re-derives it.
- **`analytics/ratios`** computes its own `Growth` block from `PeriodRatios` history, independently of `metrics.Trend` (a different, ratios-package-local growth-rate calculation over the same underlying revenue figures).
- **`analytics/revenuequality`** computes `RevenueTrend`/`RevenueCAGR` from `Dataset` directly (its own `calculateCAGR`), plus customer-level growth decomposition (new/lost/expansion/contraction) that neither `qoe` nor `ratios` attempts.
- **`analytics/forecast`** projects revenue forward via caller-supplied `RevenueMethod`/`GrowthRate` assumptions — an input, not a historical measurement.
- **`portfolio/diagnostics`** detects revenue decline via `SelectedMetrics.Revenue` percent-change against `Prior`, a portfolio-level comparison distinct from any of the above.

Three independently-implemented "revenue growth" calculations exist
(`qoe`'s copy of `metrics.Trend`, `ratios`'s own, `revenuequality`'s own)
— all should agree on a simple period-over-period growth rate given
identical revenue figures and periods, since none applies a different
smoothing/weighting method. **Not unified** (each serves a different
package's own trend/signal machinery and unifying them would be exactly
the kind of premature abstraction this task's constraints warn against),
but flagged as a reasonable candidate for a future cross-module invariant
test beyond the ones this pass already added (see § Invariants below).

### Concentration

- **`analytics/concentration`** and **`analytics/revenuequality`**'s `ConcentrationSummary` independently implement the identical HHI formula (`Σ(share²) × 10,000`, the standard DOJ/FTC 0-10,000 scale), each with its own `sortedKeys`-before-summing determinism guard. Verified via direct source comparison to be byte-for-byte the same algorithm.
- **`transactions/salereadiness`** consumes `analytics/concentration.PeriodConcentration.LargestEntityShare` as an input (correctly composing, not recomputing).
- **`portfolio/diagnostics.ConcentrationSummary`** and **`analytics/diagnostics`**'s concentration findings both mine already-computed `analytics/concentration`/`analytics/revenuequality` output rather than computing anything themselves.

**This is the cleanest cross-module pair in the whole audit** — genuinely identical formula, independently implemented, verified to agree given equivalent input (see § Invariants below, where this is one of the two invariants actually written as a test).

### Valuation

- **`analytics/valuedrivers`** is the only package that re-runs the full `orchestrator`/`consensus` pipeline.
- **`transactions/salereadiness`** and **`reporting/management`** both consume an already-computed `valuation/consensus.Result` as a plain input field — neither recomputes valuation.
- **`transactions/acquisition`** never touches the `valuation/*` packages at all — its price multiples are computed directly against caller-supplied `TargetFinancials`/`ConsensusValuation` (a plain figure, not a `consensus.Result`).
- **`portfolio/diagnostics.ValuationSummary`** and **`analytics/diagnostics`**'s valuation findings both mine an already-computed `consensus.Result`/`valuedrivers.Result`, never recomputing.

One package computes, the rest consume — a clean, consistent boundary
with no definitional drift.

## Threshold / unit semantics

Field units were spot-checked across the packages most likely to have an
ambiguous unit (percentage vs. fraction vs. points vs. basis points):
`Thresholds`/`Policy` fields throughout consistently use a **0-1 decimal
fraction** for anything described as a percentage in its doc comment
(e.g. `0.30` = 30%), never basis points and never a 0-100 scale, with the
single documented exception of `analytics/concentration.HHI`/
`revenuequality.ConcentrationSummary.HHI`, which are explicitly on the
**0-10,000 DOJ/FTC scale** (both doc comments state this plainly). Every
"points"-named field (`MarginDeteriorationPoints`,
`IncreasingLargestShareTrendPoints`, etc.) is a **raw decimal difference**
between two already-fractional figures (e.g. `0.10` = "10 percentage
points", not "10%"), consistently named and documented. No silent
reinterpretation of an existing field's unit was made or found necessary
in this pass.

## Nil / zero-value safety

Every package's `Options`/`Policy`/`Thresholds` zero value was audited for
panic-safety and sane defaulting. **Confirmed sound, with the one real
issue already covered under § Availability semantics above**
(`portfolio/diagnostics`'s `DefaultPolicy.SeverityWeights` map-aliasing,
fixed).

The repo-wide convention — a zero-value `Thresholds`/`Policy` resolves
entirely to `DefaultThresholds()`/`DefaultPolicy()`, held in every package
that has one (`qoe`, `ratios`, `cashflow`, `revenuequality`,
`concentration`, `anomalies`) — was found to have exactly one exploitable
gap (`concentration`'s missing `> 0` flag guards, already fixed; see
§ Availability semantics). The all-or-nothing nature of that
zero-value-means-defaults check itself (a caller setting one field
doesn't get defaults for sibling fields left at Go's natural zero) is a
**repo-wide idiom, not unique to any one package** — flagged here as
worth being aware of when adding any *new* threshold field to any of
these packages in the future, but not changed, since redesigning the
defaulting rule itself would be a behavior change to 6 already-frozen
packages for a hypothetical future risk, not a currently-exploitable one
beyond the single case already fixed.

Packages with genuinely **no** `Thresholds`/`Policy`/`Options` type at
all (`forecast`, `covenants`, `benchmarks`, `valuedrivers`, `acquisition`,
`dealstructure`) were confirmed to have no internal judgment-threshold
concept to default in the first place — every one of their numeric inputs
is caller-supplied domain data (loan terms, covenant tests, benchmark
peer values, driver deltas, deal terms), correctly left with no invented
default. `analytics/debt.LenderPolicy` is the one policy-shaped type with
no `Default*()` constructor that *does* have a real reason: every field's
doc comment states "zero means not specified" (a lender's own terms are
genuinely arbitrary caller data, not something this package could sanely
default), and `Calculate` already emits an advisory `IssueNoLenderPolicy`
and skips capacity calculations gracefully rather than inventing a value.

## Concurrency

All 20 packages passed `go test -race ./...` clean, including every new
test added by this hardening pass. No package holds mutable state across
calls; every `Calculate`/`Build`/`Execute` entry point builds its working
state fresh on each call. Concurrent-call regression tests exist for
`analytics/diagnostics` (pre-existing) and `portfolio/diagnostics` (one
pre-existing test plus a new one added by this pass specifically
targeting the zero-value-`Policy` map-aliasing fix under real concurrent
goroutines).

## Cross-module invariants

Two genuinely-equivalent-definition pairs identified in
§ Cross-module semantic consistency above are asserted as executable
tests in `analytics/smoketest/invariants_test.go`, both passing:

- **`analytics/concentration`'s HHI == `analytics/revenuequality.ConcentrationSummary`'s HHI** for the same underlying customer-revenue data — both are independent implementations of the identical `Σ(share²) × 10,000` formula, and the test asserts exact float64 equality (`891.4896502052551` in both, for the canonical fixture's 2025 period), not merely "close."
- **`analytics/debt`'s `TotalDebtBalance - CashAndEquivalents` == `financial/metrics.Snapshot.NetDebt`** when `analytics/debt.Input` is fed the same debt/cash figures the dataset actually carries. This one required a real fixture fix during this task: `fixtures/synthetic.BuildDebtInput`'s `CashAndEquivalents` had been set to an inconsistent, independently-chosen figure ($2,940,000) rather than the dataset's actual 2025 `BS_CASH` ($7,223,656) — the invariant test caught this immediately (exact-equality assertion failed with the two figures off by over $4M) and the fixture was corrected, not the test loosened.

One additional equivalent-definition candidate was identified but **not**
turned into an invariant test in this pass: three independently-implemented
"revenue growth" calculations exist (`qoe`'s copy of `metrics.Trend`,
`ratios`'s own, `revenuequality`'s own — see § Cross-module semantic
consistency § Revenue growth) that should agree on a simple
period-over-period rate given identical inputs. Documented as a
reasonable future addition rather than added here, to keep this pass's
scope to the invariants this task explicitly named as examples
(`workingcapital` NWC == `cashflow` NWC — verified as true reuse via a
direct import, not independent computation, so no invariant test is
needed for that pair at all; `debt` annual debt service ==
`acquisition` debt service for the same terms — both call the identical
`debt.Amortize`/coverage machinery, verified via § Dependency graph, so
likewise no separate invariant test is needed; `consensus` baseline ==
`valuedrivers` baseline before mutation — `valuedrivers.Calculate`'s own
`Baseline` field is produced by calling `orchestrator.Execute`/
`consensus.Calculate` directly against `Input.BaselineRequest` with no
intervening transformation, confirmed by direct source read of
`analytics/valuedrivers/valuedrivers.go`; `TestCalculate_Baseline`
already asserts the resulting baseline figures match hand-computed
expected values for a known input, which is the correctness check that
matters here — a separate invariant test re-asserting "calling the same
function the same way gives the same result" would not exercise anything
`TestCalculate_Baseline` doesn't already cover).

## Fuzz testing

New targeted fuzz tests were added for `analytics/concentration`,
`analytics/variance`, `analytics/benchmarks`, `analytics/covenants`,
`transactions/dealstructure`, and `analytics/consolidation` (see each
package's own `fuzz_test.go`). **Four of the six immediately found real
bugs** in the seed corpus alone — see § Availability/missing-data
semantics above for what was found and fixed. All six now run clean for
1M+ executions each (`go test -fuzz=... -fuzztime=15s`), with zero panics,
zero NaN/Inf leaks, and zero unbounded loops.

## Composition patterns

None of the 20 packages requires every other package to run — each is
independently useful given its own `Input`. The patterns below are the
common orderings a caller composes them in; see
`analytics/smoketest/smoketest_test.go` for a working, tested example of
the full chain (§ 26 in the completion report covers what it verifies).

**Monthly/quarterly advisory** — the recurring-engagement pattern for an
already-onboarded client:

```
financial.FinancialDataset
  → analytics/ratios      (health signals)
  → analytics/cashflow    (conversion/runway)
  → analytics/workingcapital (NWC trend/seasonality)
  → analytics/anomalies   (expense-anomaly sweep)
  → analytics/variance    (budget-vs-actual, if a budget exists)
  → reporting/management  (assembles the above into one presentation-neutral pack)
```

**Business valuation** — the core V1-plus-analytics valuation flow:

```
financial.FinancialDataset
  → analytics/qoe                (normalized maintainable earnings)
  → valuation/orchestrator+consensus  (the V1 core — see V1_CONTRACTS.md)
  → analytics/valuedrivers       (sensitivity to driver/scenario changes)
```

**Transaction advisory** — a sell-side or buy-side engagement:

```
financial.FinancialDataset
  → analytics/qoe
  → analytics/workingcapital
  → analytics/concentration        (customer-level, if available)
  → transactions/salereadiness     (readiness assessment)
  → transactions/acquisition       (buy-side screening, if evaluating a target)
  → transactions/dealstructure     (once terms are being negotiated)
```

**Accounting-firm portfolio monitoring** — many clients, one dashboard:

```
per-client: financial.FinancialDataset → whichever analytics packages the firm runs
  → caller condenses each client's Results into the small Summary structs
    portfolio/diagnostics.BusinessSnapshot documents (QoESummary,
    RatioHealthSummary, ConcentrationSummary, CashFlowSummary,
    ValuationSummary, SaleReadinessSummary)
  → portfolio/diagnostics (ranks findings across the whole book)
```

`analytics/diagnostics` (single-business, mines up to 15 sibling
`Result`s) and `analytics/consolidation` (multi-entity `FinancialDataset`
merge, run *before* any of the above when a caller has multiple legal
entities to combine first) are not tied to one specific pattern above —
each composes with whichever of the packages above a caller has already
run.

No pattern above is mandatory; a caller with only `financial/metrics` and
`analytics/ratios` gets a complete, valid (if narrower) `Result` from each
— every package's optional-input degradation (see
§ Availability/missing-data semantics) is designed exactly for this kind
of partial adoption.

## Package maturity classification

- **STABLE_V1** (API frozen, safe to depend on): all 20 packages covered
  by this document. Every package's `Calculate`/`Build`/`Execute` entry
  point, `Input`/`Result` shape, and enum value set are considered stable
  as of this pass — see § Public API freeze below for exactly what
  "stable" covers and excludes.
- **EXPERIMENTAL**: none of the 20 packages in this scope. (The AI-adapter
  packages — `financial/classification/ai`, `financial/adjustments/ai`,
  and their `.../openai` subpackages — predate this scope and remain
  experimental per their own documentation; this pass did not touch
  them.)
- **INTERNAL**: no unexported package in this scope is re-exposed through
  a stable public path that would need separate classification.

## Public API freeze

**Frozen as of this pass** (a breaking change to any of these requires a
`FormulaVersion` bump and a deliberate decision, not an incidental edit):

- Every package's exported `Input`/`Result` (or `Report`, for
  `reporting/management`) struct shape, including field names, JSON tags,
  and nesting.
- Every exported enum type and its declared constant values (`IssueCode`,
  `Severity`/`IssueSeverity`, `FlagCode`, `Status`, `Direction`,
  `Operator`, etc.) — including the 7 new codes this pass added (see
  § Issue taxonomy above), which are now themselves frozen going forward.
- Every package's `Calculate`/`Build`/`Execute` function signature.
- Every `FormulaVersion`/`ScoreVersion`/`SignalRulesVersion` constant name
  (not its *value*, which is expected to change on a genuine formula
  change — see [`ANALYTICS_VERSIONS.md`](ANALYTICS_VERSIONS.md)).

**Explicitly NOT frozen** (may change without a major version signal):

- Internal, unexported helper functions and types.
- Default threshold *values* inside each package's `Default*()`
  constructor (the constructor's existence and name are frozen; the
  numbers it returns are tunable — though changing one does require a
  `FormulaVersion` bump per each package's own documented rule).
- Heuristic scoring weights (e.g. `portfolio/diagnostics.DefaultPolicy.SeverityWeights`'s
  actual numbers).
- Internal map/slice ordering implementation details, as long as the
  documented *output* ordering contract (declaration order, priority
  order, etc.) is preserved.
- AI prompt text (not present in any of these 20 packages).

## Known limitations

Real, permanent limitations of the analytics suite as designed — not bugs,
not gaps this pass could or should have closed:

- **No external benchmark sourcing.** `analytics/benchmarks` compares
  against whatever `BenchmarkSet` a caller supplies; it never fetches,
  licenses, or infers industry benchmark data itself.
- **No tax/legal conclusions.** Nothing in this suite (including
  `analytics/covenants`, which evaluates covenant *tests*, not legal
  covenant *language*) offers a tax or legal opinion.
- **No lender approval prediction.** `analytics/debt`'s `LenderPolicy` and
  `analytics/covenants`' pass/fail results are descriptive comparisons
  against caller-supplied thresholds, never a prediction of what an actual
  lender would approve.
- **No automatic FX sourcing.** `analytics/consolidation.CurrencyRate`
  must be supplied by the caller for every (from-currency, to-currency,
  period) combination it uses; this package never fetches or interpolates
  an exchange rate itself.
- **No inferred intercompany eliminations.** `analytics/consolidation.Eliminations`
  must be the caller's complete, explicit list; this package never
  detects or infers an intercompany relationship from the data itself.
- **Forecast assumptions are entirely caller-supplied.**
  `analytics/forecast` projects forward from whatever `Assumptions` a
  caller provides; it never predicts a growth rate, margin, or any other
  assumption from historical data.
- **Customer concentration requires customer-level observations.**
  `analytics/concentration` and `analytics/revenuequality`'s customer
  features produce nothing without caller-supplied per-customer revenue
  rows — neither package infers a customer breakdown from period-level
  totals.
- **Covenant semantics are caller-provided, not legal interpretation.**
  `analytics/covenants.CovenantTest` is exactly what the caller defines it
  to be; this package has no concept of what a real credit agreement's
  covenant language actually requires beyond the `Operator`/`Threshold`
  the caller states.
- **Scores are heuristic where present.** Every `*Score`-shaped output in
  this suite (`qoe.Score`, `salereadiness.OverallScore`,
  `portfolio/diagnostics`'s `PriorityScore`, `analytics/diagnostics.Score`)
  is an explicit, documented formula over already-computed figures — none
  claims to be a market-validated or empirically-calibrated model.

## Verification

`gofmt -w .`, `go build ./...`, `go vet ./...`, `go test ./... -count=1`,
and `go test -race ./... -count=1` all pass clean across the entire
module as of this pass. See the completion report for exact test counts
and benchmark/fuzz results.
