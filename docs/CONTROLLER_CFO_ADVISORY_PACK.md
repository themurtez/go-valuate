# Controller / CFO Advisory Pack (`reporting/advisory`)

`reporting/advisory` is the final module in the accounting-operations/
advisory roadmap: a deterministic composer that consolidates every other
package's already-computed results into a coherent, traceable,
presentation-neutral Controller/CFO advisory package. It sits above every
other package in this repository — `financial/*`, every `analytics/*`
package, `transactions/*`, `portfolio/diagnostics`, `reporting/management`,
and every `accounting/*` package — and produces one structured [`Result`]
answering, using only already-computed facts:

```
What changed?
What matters financially?
Where is liquidity tight?
What is happening in working capital?
Where are profitability/margin pressures?
What operating-cost or vendor patterns stand out?
What accounting/close/reconciliation issues remain?
What debt/covenant constraints exist?
What valuation/value-driver movements exist?
What requires management attention?
What factual actions or follow-ups are outstanding?
```

Like every other package in this repository, it contains **no**
persistence, HTTP/API, auth, UI, background jobs, PDF/PowerPoint
rendering, dashboards, notifications, external integrations, AI/LLM
narrative generation, or tax/legal/audit logic — see [What this project
intentionally does not
contain](../README.md#what-this-project-intentionally-does-not-contain)
in the main README, and [Explicit non-goals](#explicit-non-goals) below.
Every exported function is pure (no I/O, no mutation of caller-owned
input, no package-global mutable state, no wall-clock reads, no
`time.Now()`) and deterministic — see
[`determinism_test.go`-equivalent coverage in
`advisory_test.go`](../reporting/advisory/advisory_test.go) and
[`concurrency_test.go`](../reporting/advisory/concurrency_test.go).

## Purpose and the composition-not-recalculation rule

**This package computes no new accounting or valuation formula.** Every
figure in a [`Result`] is read directly from an already-computed sibling
`Result` — `accounting/ar.Result`, `accounting/cashforecast.Result`,
`analytics/debt.Result`, `valuation/consensus.Result`, and so on — via one
of this package's `section_*.go` adapters. The only arithmetic this
package performs itself is generic composition math:

```
count
sum of explicitly additive action exposures
current-vs-prior change (absolute, percent, percentage-point)
ranking/prioritization under an explicit caller policy
```

If a fact is missing, the corresponding `Section`/`Metric` reports
`StatusUnavailable`/`StatusNotSupplied`; it is never reconstructed from
unrelated fields. Concretely:

```
AR aging          -> does not recalculate DSO
AP aging          -> does not recalculate DPO
inventory         -> does not recalculate DIO
labor             -> does not recalculate Revenue/FTE
profitability     -> does not recalculate contribution margins
cashforecast      -> does not rebuild a 13-week forecast
reconciliation    -> does not rematch transactions
closequality      -> does not rerun quality diagnostics
closechecklist    -> does not reevaluate workflow logic
valuation         -> does not rerun methods
```

## No presentation, no narrative

`Build` produces structured data only. It never renders PDF/PowerPoint/
HTML, and it never generates narrative text via AI/LLM — every
`Insight.Statement`/`ActionItem.Description` here is either a caller-
supplied string passed through unchanged (`ActionOriginCallerSupplied`)
or a short, fixed-form template built entirely from already-computed
typed fields (`statements.go`, `actiontemplates.go`, `question.go`) — the
same discipline `reporting/management` already established for this
repository's presentation-neutral composition packages. A caller wanting
a rendered report takes this package's `Result` as its data source and
builds that presentation layer itself.

## Entry point

```go
func Build(in Input, policy Policy) Result
```

`Build` never mutates `in`, `policy`, or any nested sibling `Result`, and
calls no sibling package's own `Calculate`/`Build` — it only reads. Every
`Input` field beyond `Company`/`Periods` is independently optional —
partial packs work; see [Partial input](#partial-input-behavior).

## Package dependency architecture

```
financial/metrics, financial (via nested Result types)
analytics/{ratios,workingcapital,revenuequality,concentration,debt,
           covenants,valuedrivers,consolidation,diagnostics,forecast,kpi}
accounting/{ar,ap,inventory,labor,profitability,vendorspend,cashforecast,
            reconciliation,closequality,closechecklist}
transactions/{salereadiness,acquisition,dealstructure}
portfolio/diagnostics
valuation/consensus, valuation (ValueType)
        |
reporting/advisory
```

`reporting/advisory` depends **downward only** on domain outputs; no
accounting/analytics module imports it back — the dependency graph
remains a DAG. `reporting/advisory` does **not** import
`reporting/management` (and vice versa): both are independent, sibling
composition layers over the same lower packages, avoiding the
"reuse a sibling composition package's types" trap that would either
create a cycle or force one package to depend on the other's own
evolving presentation model. Where the two packages would otherwise
duplicate a shape (e.g. a `Value{Available,Amount}` wrapper), each keeps
its own local copy — the same convention every package in this repository
already follows (see `transactions/salereadiness.Value`'s doc comment for
the canonical rationale, and the many independently-declared `Value`
types the sibling-adapter research for this package confirmed: `ar`/`ap`
use `AmountValue{Available,Value}`, `inventory`/`labor` use
`Value{Available,Amount}`, `cashforecast` uses `AmountValue{Available,
Value}` — every one a distinct Go type).

## Section/result model

```go
type Result struct {
    SchemaVersion, FormulaVersion, AdvisoryContractVersion string
    Status BuildStatus // COMPLETE | PARTIAL | INVALID

    Company Company Context
    Sections []Section
    ExecutiveSummary ExecutiveSummary
    Snapshot Snapshot
    Questions []ManagementQuestion // opt-in
    Coverage Coverage
    SourceVersions SourceVersions
    PriorComparison *PriorComparison // only when Input.Prior supplied

    Warnings, Errors []Issue
}
```

`Section` is presentation-neutral:

```go
type Section struct {
    Code       SectionCode
    Availability AvailabilityStatus // AVAILABLE | UNAVAILABLE | NOT_SUPPLIED | NOT_APPLICABLE | INVALID

    Highlights []Insight
    Metrics    []Metric
    Findings   []Insight
    Actions    []ActionItem
    Sources    []SourceRef
}
```

Fifteen content-bearing sections, always attempted in this fixed order
(task-specified management-flow order — liquidity first, working
capital/profitability/operations in the middle, debt/covenants/close/
forecast/valuation/transaction-readiness after, KPI/ACTION_REGISTER/
APPENDIX last):

```
LIQUIDITY
FINANCIAL_PERFORMANCE
WORKING_CAPITAL
REVENUE
PROFITABILITY
LABOR
INVENTORY
VENDOR_SPEND
DEBT_AND_COVENANTS
ACCOUNTING_AND_CLOSE
FORECAST_AND_OUTLOOK
VALUATION_AND_VALUE_DRIVERS
TRANSACTION_READINESS
KPI
ACTION_REGISTER
APPENDIX
```

`EXECUTIVE` is a defined `SectionCode` (for `Policy.CategoryOrder`
references) but carries no `Section` of its own — the executive summary
is `Result.ExecutiveSummary`, a distinct top-level field built by
*selection* over every other section's own `Highlights`/`Findings`/
`Actions`, not a section with its own `Metrics`/`Findings` — see
[Executive summary](#executive-summary).

A section that has no supporting `Input` reports
`Availability: StatusNotSupplied` rather than being omitted from
`Result.Sections` entirely — every content-bearing `SectionCode` always
has exactly one `Section` entry, so a caller never has to guess whether a
section was "left out" versus "checked and found empty."

## Metric model

```go
type Metric struct {
    Code, Label string
    Value Value
    Unit  Unit // currency | percent | multiple | days | months | count | weeks | ratio
    Period string

    Prior  Value
    Change Change // AbsoluteChange, PercentChange, PercentagePointChange

    SourceModule, SourceCode string
    SourceRefs []SourceRef
}
```

`Value{Available bool; Amount float64}` distinguishes "computed to be
exactly 0" from "unknown" — this package's own copy of the convention
every sibling package in this repository duplicates locally. `Change` is
the only arithmetic this package performs on a source figure:
`AbsoluteChange` (`Current - Prior`), `PercentChange` (relative, never
computed from a zero denominator — task section 10's explicit rule), and
`PercentagePointChange` (for `UnitPercent` metrics specifically, since a
margin's "18% -> 27%" is "+9 points," not "+50%" — both are always
computed when preconditions allow, never conflated). Every arithmetic
result is guarded against `NaN`/`+/-Inf` (`safeValue` in `value.go`) —
found by fuzzing, see [Fuzz results](#fuzz-results).

## Facts vs. interpretations

```
Metric   = raw/computed number, read verbatim from a source Result
Finding  = deterministic condition already supported by a source
           module/policy (an Insight with Severity above INFO)
Insight  = composition-level statement connecting supported facts
           (Highlight or Finding — same Go type, different section slice)
```

Allowed: *"AR over 90 days increased from 18% to 27% and the 13-week
forecast reaches a minimum cash balance of $42,000."* Not allowed unless
caller policy explicitly defines it: *"The company has a severe
collections problem."* Every generated `Insight.Statement` is assembled
from typed values via a fixed Go template (`statements.go`) — see
`StatementCode` for the closed set of templates this package ever emits.

## Source adapters

Each `section_*.go` file is this package's adapter for one or more
sibling packages: it reads a sibling `Result` verbatim, extracts typed
advisory facts, and preserves full provenance
(`SourceModule`/`SourceCode`/`SourceRefs`) — never a domain
recomputation. Representative adapters cover:

```
financial metrics    -> section_financial.go
ratios                -> section_workingcapital.go (source precedence only)
working capital       -> section_workingcapital.go
cash forecast          -> section_liquidity.go
AR / AP                -> section_workingcapital.go, adapter_ar_ap.go
inventory               -> section_inventory.go, section_workingcapital.go
labor                    -> section_labor.go
profitability            -> section_profitability.go
vendor spend              -> section_vendorspend.go
debt / covenants           -> section_debt.go
reconciliation / close     -> section_close.go
closechecklist               -> section_close.go
KPI                            -> section_kpi.go
valuation / value drivers       -> section_valuation.go
revenue quality / concentration  -> section_revenue.go
sale readiness / acquisition      -> section_transaction.go
forecast (financial/operating)     -> section_forecast.go
```

**Adapter correctness invariant:** for every adapter, `advisory metric
value == source result value`, byte-for-byte or tolerance-equivalent —
never recalculation drift. Verified in `fullpack_test.go`'s
`TestIntegration_HealthyReadyCompany`, which spot-checks every
content-bearing section against a fully populated, hand-built fixture.

Each field-selection is the result of first researching the exact
contract of the corresponding sibling package (`Result` shape, `Flag`/
`Finding` severity enums, provenance field names) before writing the
adapter — several sibling packages turned out to have subtly different
shapes than a naive port would assume (e.g. `analytics/debt.Flag` uses
`Scenario`, not `Period`; `analytics/covenants.TestResult` uses
`Explanation`, not `Message`, and has no `Flag` type at all;
`accounting/cashforecast.AmountValue`'s numeric field is named `Value`,
while every other sibling's equivalent wrapper names it `Amount`).

## Source precedence and conflicts

When the same metric exists in more than one supplied module (the
canonical example: AR DSO from `accounting/ar` vs. `analytics/ratios`),
`resolveSourcedMetric` (`sourceprecedence.go`) applies this precedence:

1. **Caller override**: `Policy.SourcePreferences` entry for that metric
   code, if any.
2. **Package default**: a small, documented default order for the three
   well-known duplicates (DSO: `ar` then `ratios`; DPO: `ap` then
   `ratios`; DIO: `inventory` then `ratios`) — unless
   `Policy.DisableDefaultSourceOrder` is set.
3. **Agreement within tolerance**: if every present candidate agrees
   within `Policy.ConflictTolerance`, a deterministic pick (sorted by
   module name) is used — they agree, so any deterministic pick is the
   same answer.
4. **Otherwise**: no `Metric` is returned; a `SOURCE_CONFLICT` `Issue` is
   returned instead, naming both candidate sources. Both values are
   preserved (via the `Issue`'s own fields plus the still-unresolved
   candidates a caller can re-derive from the original `Input`); **this
   package never averages**.

`Policy.DisableDefaultSourceOrder` exists specifically so a caller can
observe case 4 even for a metric with a documented package default (see
`TestIntegration_SourceConflictFixture`) — without it, DSO/DPO/DIO would
always resolve via the package default and never surface a conflict.

## Executive summary

```go
type ExecutiveSummary struct {
    KeyHighlights []Insight
    KeyRisks      []Insight
    KeyActions    []ActionItem
    LiquidityStatus AvailabilityStatus
    CloseStatus     AvailabilityStatus
}
```

Not prose generation: `selectExecutiveSummary` (`executive.go`) collects
every section's own `Highlights` (for `KeyHighlights`), every `Finding`
with `Severity` `BLOCKING`/`HIGH` (for `KeyRisks`), and every `Actions`
entry (for `KeyActions`), sorts each list by the fixed priority/category/
source tie-break (see [Determinism](#determinism-and-ordering)), and caps
each at `Policy.MaxExecutiveHighlights`/`Policy.MaxActions` (default 5
each). `LiquidityStatus`/`CloseStatus` echo the `LIQUIDITY`/
`ACCOUNTING_AND_CLOSE` sections' own `Availability` verbatim.

## Composition by section

**Financial performance** (`section_financial.go`): consumes
`financial/metrics.Snapshot` verbatim (revenue, gross profit/margin,
EBITDA/margin, net income), matching the period named by
`Input.Company.CurrentPeriod`/`PriorPeriod`.

**Liquidity** (`section_liquidity.go`): consumes
`accounting/cashforecast.Result.BaseScenario.Summary`/`OpeningPosition`
— opening cash, ending cash, minimum cash and its week, required funding,
facility capacity — plus the base scenario's own `Flag`s. No new
forecast.

**Working capital** (`section_workingcapital.go`): consumes
`accounting/ar`/`ap`/`inventory` and `analytics/ratios`/`workingcapital`,
resolving DSO/DPO/DIO through [source
precedence](#source-precedence-and-conflicts), plus AR/AP open balances,
overdue percentages, control-account-reconciliation status (which drives
`REVIEW_AR_CONTROL_RECONCILIATION`/`REVIEW_AP_CONTROL_RECONCILIATION`
actions when unreconciled), and NWC from `analytics/workingcapital`.

**Revenue** (`section_revenue.go`): consumes
`analytics/revenuequality` (recurring-revenue %, customer transitions —
lost/retained revenue) and `analytics/concentration` (largest-customer
share, HHI, and its period-over-period change). No churn prediction.

**Profitability** (`section_profitability.go`): consumes
`accounting/profitability.BusinessTotals` (gross/contribution profit,
contribution margin and its change) and every `DimensionView`'s
`Rankings.NegativeContribution` entries. No recommendation to terminate a
customer/product.

**Labor** (`section_labor.go`): consumes
`accounting/labor.PeriodSummary` (total labor cost, labor cost % of
revenue and its period-over-period change, FTE, headcount, revenue/gross-
profit per FTE, overtime %, contractor share) plus every `Flag`. No
employee-ranking or employment recommendation.

**Inventory** (`section_inventory.go`): consumes
`accounting/inventory.Result` (inventory value, turnover, DIO, slow-/
non-moving %, reconciliation status, stock-policy exceptions). No
automatic write-down recommendation.

**Vendor spend** (`section_vendorspend.go`): consumes
`accounting/vendorspend.Result` (net spend, top-1/top-5 supplier share,
tail spend %) plus every `Flag`. No vendor replacement/negotiation
recommendation.

**Debt and covenants** (`section_debt.go`): consumes
`analytics/debt.Result.BaseCase`/`Capacity` (total debt, DSCR, net-debt-
to-EBITDA, capacity headroom) and `analytics/covenants.Result.Tests`,
preserving every `TestResult`'s exact `Explanation`/`Status`/
`WarningBufferStatus` — never a legal conclusion of this package's own.

**Accounting and close** (`section_close.go`): consumes
`accounting/reconciliation`, `accounting/closequality`, and
`accounting/closechecklist`, translating each package's own three-tier-
or-two-tier severity onto this package's own `BLOCKING`/`HIGH`/`MEDIUM`/
`LOW`/`INFO` scale (`severityFromReconciliationFinding`/
`severityFromCloseQualityFinding`) — never downgrading a source's own
blocking condition. Does **not** read `accounting/journaldiagnostics`
directly: `accounting/closequality` already mines it
(`mine_journaldiagnostics.go`), so this section reads that
already-composed output rather than re-mining a second time.

**Forecast and outlook** (`section_forecast.go`): consumes
`analytics/forecast.Result` — the financial/operating forecast, kept
strictly distinct from the 13-week liquidity forecast (`LIQUIDITY`
section) per the task's explicit "do not merge them into one model" rule.

**Valuation and value drivers** (`section_valuation.go`): consumes
`valuation/consensus.Result` (indicated value on its own preserved
`Basis` — enterprise/equity/asset, never combined incorrectly — range,
dispersion score) and `analytics/valuedrivers.Result.Scenarios` (each
scenario's already-computed consensus-value delta). No recomputation.

**Transaction readiness** (`section_transaction.go`): consumes
`transactions/salereadiness.Result` (overall score, blockers) and
`transactions/acquisition.Result` (price multiples, premium to consensus,
cash-on-cash return, red-flag `Flag`s). Never infers that a company
should sell/acquire.

**KPI** (`section_kpi.go`): consumes `Input.KPIValues
([]analytics/kpi.KPIResult)` verbatim — value, unit, target evaluation,
band, change — with no re-evaluation of any KPI formula. Never
reinterprets a caller-supplied band label.

## Action-register model

```go
type ActionItem struct {
    ActionCode, Category string
    Priority Priority
    Title, Description string
    SourceModule, SourceCode string
    SourceRefs []SourceRef
    RelatedMetricCodes, RelatedEntityRefs []string
    DueDate *time.Time
    OwnerRef string
    Status ActionStatus   // OPEN | IN_PROGRESS | RESOLVED | DEFERRED | NOT_APPLICABLE
    Origin ActionOrigin   // GENERATED | CALLER_SUPPLIED
    Blocking bool
    EntityRef, Period string
}
```

Every `GENERATED` `ActionItem` comes from a closed, fixed
`actionTemplates` map (`actiontemplates.go`) — 35 stable `ActionCode`
values, each with a fixed `Title`/`Description` using only review/
resolve/validate/investigate/confirm/complete-framed language. `Build`
never invents action text. `DueDate` is populated only from a source
module's own due date or a caller-supplied one — never `time.Now()`.

## Action deduplication and provenance

The same underlying issue can surface through more than one adapter
(e.g. an unreconciled AR control account appears in `reconciliation`,
`closequality`, and `closechecklist` findings). `dedupeActions`
(`action.go`) merges every `ActionItem` sharing an identical
`ActionIdentity{ActionCode, EntityRef, Period}` into one record: source
refs are unioned (never lost), `Blocking` is OR'd (never downgraded), and
the most urgent contributing `Priority` wins. Identity matching is always
exact-field equality — **never fuzzy-matched titles/descriptions**.
`ACTION_REGISTER` (`section_actionregister.go`) is the single place a
caller sees the fully deduplicated, cross-section action list;
individual sections still carry their own pre-cross-section-dedup
`Actions` for section-local display.

## Management questions

An opt-in (`Policy.IncludeManagementQuestions`) `[]ManagementQuestion`
section, built from a finite, fixed `insightCodeToQuestion` map
(`question.go`) — a deterministic template lookup by `Insight.Code`,
never a generated question. Every question invites context rather than
embedding a conclusion (*"What factors explain the increase in
receivables over 90 days?"*, never *"Why is management failing to
collect receivables?"*) — enforced by the same
`TestNoPrescriptiveLanguage` regression scan that covers action
templates.

## Cross-module synthesis

A small, closed set of deterministic multi-source rules
(`synthesis.go`), each gated by an explicit `Policy.Synthesis` threshold
(never a hard-coded "bad" number):

```
Liquidity + collections pressure:
  13-week minimum cash <= Policy.Synthesis.MinimumCashThreshold
  AND AR overdue % increased by >= Policy.Synthesis.AROverdueIncreasePoints

Margin pressure from labor:
  contribution margin declined by >= Policy.Synthesis.MarginDeclinePoints
  AND labor cost % revenue increased by >= Policy.Synthesis.LaborCostIncreasePoints

Close blocked by reconciliation:
  an ACCOUNTING_AND_CLOSE finding sourced from "reconciliation"
  AND one sourced from "close_checklist" concerning the same EntityRef
```

Every synthesized `Insight` lists every contributing `SourceRef` — never
hiding which modules supported the statement — and uses an explicit
synthesis-priority `Severity`, never an automatic sum of its inputs'
severities.

## Current/prior comparison

`Metric.Change`/`Change.AbsoluteChange`/`PercentChange`/
`PercentagePointChange` cover within-pack current-vs-prior comparison
(driven by `Input.Company.CurrentPeriod`/`PriorPeriod` and a source's own
adjacent-period data). Across two whole packs, `Input.Prior *Result` plus
`comparePrior` (`prior.go`) produces `Result.PriorComparison`:

```
NewInsights / ResolvedInsights / PersistentInsights
NewActions / ResolvedActions / PersistentActions / ReopenedActions / NoLongerGeneratedActions
MetricChanges
SectionAvailabilityChanges
```

Identity is always the explicit stable key (`ActionIdentity` for
actions; `Code + EntityRef` for insights) — never fuzzy-matched text.
Default disappearance reason is `NO_LONGER_GENERATED`
(`ReasonNoLongerGenerated`), **never** silently reported as "resolved" —
`ReasonCallerConfirmedResolved` applies only when a caller-supplied
action with the same identity explicitly carried `ActionStatusResolved`.
`ReopenedActions` is reported only when provably reopened (an action
generated again after its prior-pack record carried a terminal status —
`RESOLVED`/`DEFERRED`/`NOT_APPLICABLE`), never inferred from
disappearance-then-reappearance alone across more than one hop of
history this single-`Prior`-pointer design does not carry.

## Partial input behavior

Every optional `Input` field can be left at its zero value. A section
whose only source(s) are absent reports `Availability: StatusNotSupplied`
— `Build` never errors merely because, say, inventory/debt/valuation are
absent (see `TestIntegration_PartialInput`). `Build` refuses outright
(`Status: BuildInvalid`) only when literally nothing was supplied at all
— no financial, operating, close, transaction, valuation, or KPI input,
and no caller actions.

## Coverage

```go
type Coverage struct {
    RequestedSections, AvailableSections, UnavailableSections []SectionCode
    SourceModulesSupplied, SourceModulesUsed []string
    MetricsAvailable, MetricsUnavailable int
    InsightsGenerated, ActionsGenerated, QuestionsGenerated int
    SourceConflictCount, InvalidSourceCount int
}
```

Purely factual counts — **no overall business health score**, echoing
every prior package in this roadmap's identical rule (the memory of
`portfolio/diagnostics.DefaultPolicy.SeverityWeights`'s historical shared-
mutable-map bug is exactly why `Policy`'s own map/slice fields are always
copied fresh in `resolvePolicy`, never handed back by reference — see
[Determinism, immutability, and
concurrency](#determinism-immutability-and-concurrency)).

## Issues and codes

`Issue{Code IssueCode, Severity, Message, SourceModule, SourceCode,
Period, EntityRef}` — a stable, closed taxonomy (`issues.go`), only codes
this package actually emits: `INVALID_INPUT`, `INVALID_PERIOD`,
`PERIOD_MISMATCH`, `AS_OF_DATE_MISMATCH`, `CURRENCY_MISMATCH`,
`DUPLICATE_SOURCE_METRIC`, `SOURCE_CONFLICT`,
`INVALID_SOURCE_PREFERENCE`, `INVALID_POLICY`, `INVALID_PRIORITY_RULE`,
`INVALID_ACTION_OVERRIDE`, `UNKNOWN_ACTION_STATUS`,
`INVALID_PRIOR_RESULT`, `UNSUPPORTED_SOURCE_VERSION`.

## Neutral/prescriptive-language safeguards

Every generated `ActionItem`/`ManagementQuestion` string uses only
review/resolve/validate/investigate/confirm/complete-framed language.
`TestNoPrescriptiveLanguage` and `TestActionTemplatesUseNeutralVerbs`
(`safety_test.go`) permanently scan every default string this package
generates for a fixed disallowed-term list (`fire`, `terminate employee`,
`drop customer`, `replace supplier`, `fraud`, `theft`, `borrow
immediately`, `sell company`, `write off`, `tax violation`, `audit
opinion`, and related terms) and for a required neutral-verb prefix on
every action `Title`. Caller-supplied text (`ActionOriginCallerSupplied`)
is exempt but always remains marked as caller-supplied — `Build` never
rewrites it (`TestCallerSuppliedActionsNeverAltered`).

## Determinism, immutability, and concurrency

Every collection is sorted by an explicit, documented tie-break —
`insightSortKey`/`actionSortKey` (priority, category-declaration-order,
source module, source code, entity ref) — never Go map iteration order.
`Policy`'s own resolution (`resolvePolicy`) always builds fresh maps/
slices for every field it touches, mirroring
`portfolio/diagnostics.resolvePolicy`'s safe-copy pattern exactly (the
task explicitly called out that package's historical
`DefaultPolicy.SeverityWeights` shared-mutable-map bug as one to avoid
repeating). `TestBuild_ConcurrentSafety` runs 50 concurrent `Build` calls
against the same `Input`/fresh `ExamplePolicy()` values and asserts
byte-identical JSON output; the whole suite passes under `go test -race`.

## JSON and versioning

Every exported type uses explicit `snake_case` JSON tags. Three
independent version constants (mirroring `accounting/closechecklist`'s
three-way `Schema`/`Formula`/`TemplateContract` split, the closest
existing precedent for a package needing more than one version axis):

| Constant | Covers |
|---|---|
| `SchemaVersion` | This package's own `Result`/`Section`/`Insight`/`Metric`/`ActionItem`/`ExecutiveSummary`/`Snapshot`/`Coverage` shape |
| `FormulaVersion` | This package's own composition rules: which sibling field populates each `Metric`/`Insight`, source-precedence/conflict detection, current/prior change formulas, executive selection, priority-rule precedence, synthesis rules, action-deduplication identity |
| `AdvisoryContractVersion` | The semantic meaning of every generated `StatementCode`/action-template/question-template mapping — versioned separately since generated wording may be persisted/exported by a caller independent of whether the underlying composition logic changed |

`SourceVersions` echoes every contributing sibling module's own version
constant, captured once per `Build` call — reproducibility requires
knowing not just this package's three version constants but exactly
which upstream formula/schema version produced each fact.

## Prompt 50 (`accounting/closechecklist`) timing-field compatibility

`closechecklist.PriorCloseComparison.CompletionTimingChanges
(map[string]int)` is declared on that package's own type but its builder
never populates it (confirmed by source inspection during this package's
own adapter research — a currently-dead/future-reserved field, not a
missing one). This package does **not** depend on it: `Build` derives its
own current/prior close-timing comparison generically, from whatever
`closechecklist.Completion`/`Blocker` figures the current and prior
`Input.Close.CloseChecklist` results both populate (via the same generic
`compareMetricSets`/`comparePrior` machinery every other section uses),
never by reading `CompletionTimingChanges` directly. A nil/empty
`CompletionTimingChanges` on either side is handled safely (no panic, no
silently-wrong comparison) — see
`TestCloseChecklistTimingField_CompatibilityDocumented`. Prompt 50's
`accounting/closechecklist` package itself was **not** modified as part
of this task.

## Integration fixtures

`fullpack_test.go`'s `fullyPopulatedInput`/`TestIntegration_
HealthyReadyCompany` builds one hand-constructed `Input` touching every
content-bearing section with unremarkable/positive figures, asserting no
`Blocking` action and every section `StatusAvailable`. `integration_
test.go` and `multiperiod_test.go` cover the task's remaining named
scenarios: liquidity pressure, margin pressure, close-blocked, covenant
condition, source conflict (both with and without precedence), partial
input, action dedup, multi-period comparison, and KPI integration.

## Tests, coverage, fuzz, and benchmark results

- **98 tests** across 14 test files, all passing; **84.3% statement
  coverage** (`go test ./reporting/advisory/... -cover`).
- **`go test -race ./reporting/advisory/... -count=1`**: clean.
- **4 real bugs found and fixed** during this task's own verification
  pass (all confirmed via a failing test before the fix, and a passing
  test after):
  1. `splitIssues`'s `(warnings, errors)` return-value order was swapped
     at its `Build` call site, causing `IssueInvalidInput` to land in
     `Result.Warnings` instead of `Result.Errors` — `Build` never
     returned `BuildInvalid` for a genuinely empty `Input`.
  2. `buildWorkingCapitalSection`'s locally accumulated `SOURCE_CONFLICT`
     `Issue`s were silently discarded — `buildAllSections`'s `issues`
     variable was declared but never actually populated from any section
     builder, so `Result.Warnings`/`Errors` and the `APPENDIX` section
     never surfaced a conflict a caller's own `Policy` should have been
     able to observe.
  3. `contribution_margin` (`PROFITABILITY`) and
     `labor_cost_percent_revenue` (`LABOR`) `Metric`s were built without
     ever computing their `Change`/`Prior` fields, even though each
     section separately computed the same comparison inline for its own
     `Finding` text — this silently broke the margin-pressure synthesis
     rule, which reads `Change.PercentagePointChange` from the `Metric`
     list, not from a `Finding`'s private copy.
  4. `computeChange` could propagate `+Inf`/`NaN` into
     `PercentChange`/`AbsoluteChange` for extreme-magnitude float64
     inputs (found by `FuzzComputeChange` within the first ~150
     executions) — fixed with a `safeValue` guard now applied to every
     arithmetic result the function produces.
- **Fuzz targets** (`fuzz_test.go`): `FuzzComputeChange` (~1.05M
  executions over 20s after the fix, 0 failures — the discovered `+Inf`
  case is now a permanent regression-corpus entry),
  `FuzzResolveSourcedMetric` (~750k executions, 0 failures),
  `FuzzDedupeActions` (~560k executions, 0 failures) — covering exactly
  the task's named targets (source precedence, action deduplication,
  current/prior comparison/metric-change math), each asserting no panic,
  no `NaN`/`Inf`, and determinism across repeated identical calls.
- **Benchmarks** (`bench_test.go`), laptop-safe scaling at 500/2,000/
  5,000 items: `BenchmarkDedupeActions` and `BenchmarkSortInsights` both
  scale roughly linearly / `n log n` respectively with no `O(n²)`
  blowup; `BenchmarkBuild_RepresentativePack` runs the full fixture pack
  in ~100 microseconds/op. `BenchmarkBuild_FullScale` is gated behind
  `ADVISORY_FULL_SCALE_BENCH=1` and was not run as part of this task's
  own verification (laptop-safe default: skipped).

## Limitations

- Reads only what `Input` supplies; it never queries a sibling package's
  `Calculate`/`Build` itself, so a caller integrating this package must
  run every sibling analysis first and pass the results in.
- `ReopenedActions` detection is limited to one hop of prior-pack history
  (see [Current/prior comparison](#currentprior-comparison)) — a caller
  wanting deeper action-history tracking chains its own
  `Result.PriorComparison` across calls.
- `analytics/forecast` (`FORECAST_AND_OUTLOOK`) reads only the base
  (or first) scenario's headline revenue/EBITDA figures; a caller wanting
  every named scenario reads `Input.Financial.Forecast.ScenarioResults`
  directly.
- `Coverage`/`Result.Status` describe pack-data completeness only — never
  a business-health verdict.
- No currency conversion: `Policy`/`Issue` surface a `CURRENCY_MISMATCH`
  condition when monetary source values disagree on currency, but this
  package performs no FX conversion itself.

## Explicit non-goals

This package does not, and will not, implement: accounting calculations
already owned by sibling modules; autonomous CFO recommendations;
freeform AI narrative; budgeting/bookkeeping/close workflow;
notifications; PDF/PPT rendering; dashboards; user assignment; external
benchmarks; ERP/accounting integrations; tax/legal/audit conclusions;
customer/employee termination decisions; supplier replacement decisions;
automatic pricing decisions; financing decisions; M&A decisions. The
package composes factual advisory information for a human decision-maker
— it never decides on their behalf.
