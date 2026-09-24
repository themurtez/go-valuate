# Customer / job / product profitability (`accounting/profitability`)

`accounting/profitability` provides a deterministic profitability engine
that analyzes the same underlying economic facts (revenue, returns,
discounts, direct costs, variable operating costs) by three alternate
analytical dimensions: **CUSTOMER**, **JOB**, and **PRODUCT**. It answers
questions such as which customers/jobs/products generate the most
revenue or gross profit/contribution, which have negative contribution,
where margins are deteriorating, which costs are driving margin
leakage, how much revenue/cost is unattributed, how much shared
overhead was allocated vs left unallocated, whether entity totals
reconcile to business totals, and what profitability looks like before
and after caller-directed overhead allocation.

Like every other package in this repository, it contains **no**
persistence, HTTP/API, auth, UI, background jobs, CRM/ERP/QuickBooks/Xero
integration, invoicing, job management, inventory costing, payroll
processing, pricing recommendations, or AI/LLM — see [What this project
intentionally does not
contain](../README.md#what-this-project-intentionally-does-not-contain)
in the main README. Every exported function is pure (no I/O, no
mutation of caller-owned input, no package-global mutable state, no
wall-clock reads) and deterministic — see
[`determinism_test.go`](../accounting/profitability/determinism_test.go)
and
[`immutability_test.go`](../accounting/profitability/immutability_test.go).

## Purpose and non-goals

**This is not a pricing optimizer, ERP, CRM, or AI recommendation
engine.** It performs deterministic arithmetic and diagnostics only. It
never recommends a price, never recommends dropping a customer or
discontinuing a product, never schedules jobs/projects, never derives
inventory cost, and contains no AI/LLM/ML component. The caller supplies
classification, attribution, and allocation policy; this package never
infers any of the three from names, categories, or amounts. See
[Explicit non-goals](#explicit-non-goals).

This package has **no** compile-time dependency on `accounting/ledger`,
`accounting/statements`, `accounting/labor`, `accounting/inventory`, or
`analytics/*`. Integration with those packages is demonstrated only
through small, portable typed adapters and tests — see
[Integration boundaries](#integration-boundaries).

## Three-dimension architecture: one fact, multiple views

```
one economic fact
        |
explicit dimension attribution
        |
   +----+----+----+
   |         |         |
customer   job      product
  view     view      view
```

A caller never duplicates the same revenue/cost event three times to
analyze it from three angles. Each `Fact` is recorded **once** and
carries explicit per-dimension `Attribution`s: the same $1,000 invoice
line can be 100% attributed to customer `C1`, 100% attributed to job
`J10`, and split 60/40 across products `P1`/`P2` — and `CustomerView`,
`JobView`, and `ProductView` each reconcile independently back to the
same underlying fact population.

**Customer-view totals, job-view totals, and product-view totals are
never summed together.** A single $1,000 fact attributed 100% to one
customer, one job, and one product yields $1,000 in each view — never
$3,000 of business revenue. `Result.BusinessTotals` is computed directly
from the fact/summary population, never by summing the three
`DimensionView`s — see [No-double-count invariant](#no-double-count-invariant).

## Fact and component model

```go
type Fact struct {
    FactID       string
    Period       string
    Date         *time.Time
    Component    Component
    Amount       float64        // non-negative magnitude
    Attributions []Attribution
    SourceType   string
    SourceID     string
    Currency     string
    SourceRef    SourceRef
}
```

`Amount` is always a non-negative magnitude; `Component` determines
whether it adds to revenue, reduces revenue, or adds to cost — this
package never infers economic effect from a sign, and a caller needing
to record a reversal/correction supplies a new Fact with the
appropriate Component rather than an unexplained negative amount.

### Component taxonomy

| Category | Components |
|---|---|
| Revenue | `GROSS_REVENUE`, `RETURN`, `DISCOUNT`, `OTHER_REVENUE_REDUCTION`, `OTHER_REVENUE` |
| Direct cost | `DIRECT_MATERIAL`, `DIRECT_LABOR`, `DIRECT_SUBCONTRACTOR`, `DIRECT_FULFILLMENT`, `DIRECT_OTHER` |
| Variable operating cost | `VARIABLE_COMMISSION`, `VARIABLE_PAYMENT_FEE`, `VARIABLE_OTHER` |

Subcategories beyond this fixed set are caller-defined via
`SourceType`/`SourceID`/provenance, not a new `Component` — this keeps
the profitability bridge formula fixed and auditable.

## Profitability bridge

For each business/entity/period:

```
  Gross Revenue
- Returns
- Discounts
- Other Revenue Reductions
+ Other Revenue
= Net Revenue

- Direct Material
- Direct Labor
- Direct Subcontractor
- Direct Fulfillment
- Direct Other
= Gross Profit

- Variable Commission
- Variable Payment Fees
- Variable Other
= Contribution Profit

- Allocated Shared Costs
= Allocated Profit
```

`GrossProfit` here means Net Revenue minus **caller-classified** direct
costs; it should only be expected to equal a financial-statement gross
profit when the caller's Component classification aligns with that
statement's own COGS definition. Margins (`GrossMargin`,
`ContributionMargin`, `AllocatedMargin`) are `Value`s — unavailable
(never NaN/Inf) when NetRevenue is zero. A negative NetRevenue still
produces a mathematically valid (if unusual) margin ratio; this package
never special-cases it beyond the standard zero-denominator guard.

**All-period margin is always `total profit / total revenue`, never an
arithmetic average of period margins** — see
[`AllPeriodResult`](../accounting/profitability/entityresult.go) and
`TestInvariant_AllPeriodMarginIsNotAverage`.

## Attribution and partial attribution

```go
type Attribution struct {
    Dimension Dimension
    EntityID  string
    Share     float64 // 0-1
}
```

Shares are evaluated **independently per dimension** — a Fact's
CUSTOMER attributions never constrain its JOB or PRODUCT attributions.
For each `(FactID, Dimension)`: every share must be in `[0,1]`, the
entity must exist (when a roster was supplied), there must be no
duplicate entity attribution within that dimension, and the sum of
shares must not exceed 1 (`IssueAttributionExceeds100Percent`) — **this
package never renormalizes silently**; an over-100% dimension's
attributions are dropped entirely rather than partially honored.

If a dimension's shares sum to less than 1, the remainder is explicit
`UNATTRIBUTED` — a $1,000 fact split 60% `P1` + 20% `P2` yields $600 to
P1, $200 to P2, and $200 unattributed for the PRODUCT dimension. `[
DirectAttribution]`/`[DirectAttributions]` are pure helpers for the
common 100%-attribution case.

## Unattributed amounts

Every `DimensionView.Unattributed[period]` reports
`UnattributedRevenue`, `UnattributedDirectCost`, and
`UnattributedVariableCost` with a `ByComponent` breakdown — **never
dropped**. A Fact with no Attribution entries at all for a given
dimension leaves 100% of its Amount unattributed for that dimension.

## Attribution coverage

`DimensionView.AttributionCoverage` reports, per period: fact count
(total vs. attributed), revenue amount, direct cost amount, variable
cost amount, and total economic amount (total vs. attributed), plus
`RevenueCoveragePercent`/`CostCoveragePercent` as `Value`s. **No opaque
completeness score.** `Policy.MinRevenueAttributionCoverage`/
`MinCostAttributionCoverage` are optional thresholds; by default a
coverage gap never blocks computation (`FlagLowRevenueAttributionCoverage`/
`FlagLowCostAttributionCoverage` are simply raised). Setting
`Policy.StrictAttributionCoverage = true` additionally downgrades that
dimension's `ViewStatus` to `AVAILABLE_WITH_GAPS` — computation still
proceeds either way.

## Summary-input path

Callers with already-aggregated entity-period data supply
`EntityPeriodSummaryInput` rows instead of detailed `Fact`s:

```go
type EntityPeriodSummaryInput struct {
    Dimension, EntityID, Period string
    GrossRevenue, Returns, Discounts, OtherRevenue                     Value
    DirectMaterial, DirectLabor, DirectSubcontractor,
        DirectFulfillment, DirectOther                                 Value
    VariableCommission, VariablePaymentFees, VariableOther             Value
}
```

Every field is a `Value` so "not supplied" is distinguishable from
"supplied and exactly zero." Both input paths converge onto the
**identical bridge formula** (`buildEntityPeriodResult`), so a caller
gets the same margins/flags regardless of which path they used — see
`TestSummary_ConvergesWithDetailedPath`. A `(Dimension, EntityID,
Period)` slot present in **both** the detailed-fact and summary-input
populations is a modeling conflict (`IssueInputModeConflict`); the
summary row is excluded and the detailed facts win, since supplying
both would double count.

## Driver observations and per-unit metrics

`DriverObservation{Dimension, EntityID, Period, DriverKey, Value,
Unit}` carries portable activity drivers (`units_sold`, `labor_hours`,
`orders`, `transactions`, `shipments`, or any caller-defined key).
`Policy.PrimaryDriverKey[dimension]` selects, per dimension, which
driver key feeds `PerUnitMetrics` (`NetRevenuePerUnit`,
`DirectCostPerUnit`, `GrossProfitPerUnit`, `ContributionPerUnit`) — a
zero or missing denominator leaves each `Value` unavailable, never
NaN/Inf.

## Shared-cost pools and allocation

`SharedCostPool{PoolID, Period, Amount, Category, Description}` is a
caller-declared pool of overhead — **never inferred from expense data.**

**Allocation is opt-in.** A pool with no matching `AllocationRule` for a
dimension is reported `NOT_ALLOCATED` (fully unallocated, no error —
this is the expected default). A caller supplies an `AllocationRule`
per `(PoolID, Dimension)` to have that pool's amount enter
`AllocatedProfit` for that dimension:

```go
type AllocationRule struct {
    PoolID       string
    Dimension    Dimension
    Basis        AllocationBasis // NET_REVENUE | DIRECT_COST | DIRECT_LABOR_COST | DRIVER | FIXED_WEIGHT | EQUAL
    DriverKey    string          // required for DRIVER
    FixedWeights []AllocationWeight // required for FIXED_WEIGHT
}
```

**There is no default basis.** If the resolved basis denominator is
zero or unavailable, the pool is left unallocated for that
dimension/period with a structured `IssueAllocationDenominatorUnavailable`
— this package **never falls back to equal allocation** in that case
(`PoolStatusDenominatorUnavailable`, distinct from the ordinary
`PoolStatusNotAllocated`).

### Independent allocation per dimension

The same `PoolID` may carry separate `AllocationRule`s for CUSTOMER,
JOB, and PRODUCT — each is an **independent analytical allocation**.
Allocated costs are never aggregated across dimension views (a pool
allocated $10,000 to customers and separately $10,000 to jobs does not
mean $20,000 was spent — it means the same $10,000 was analyzed through
two lenses).

### Allocation trace and reconciliation

Every successful allocation preserves an `AllocationTraceEntry` per
entity: `PoolID`, `Dimension`, `Basis`, `DriverKey`, `EntityID`,
`DriverAmount`, `AllocationShare`, `AllocatedAmount`. For every
pool/dimension/period, `Allocated + Unallocated == PoolAmount` within
`Policy.AllocationTolerance` — locked by
`TestInvariant_AllocationReconciliation`.

Directly-attributed facts (`DIRECT_MATERIAL`, `DIRECT_LABOR`, etc.) are
never themselves run through shared-cost allocation — they flow only
through `DirectCostBridge`/`GrossProfit`; `AllocatedSharedCosts` is a
wholly separate figure fed only by pool allocation.

## Customer / job / product views

`DimensionView` is structurally identical across CUSTOMER, JOB, and
PRODUCT (the three views are alternate lenses over the same fact
population). Each includes: `Status` (see [View status and
applicability](#view-status-and-applicability)), `Periods`,
`EntityPeriods`/`AllPeriod` results, `GroupSummaries`/
`CategorySummaries` (from `Entity.Group`/`Entity.Category` — never
inferred from `Entity.Name`), `Unattributed`, `AttributionCoverage`,
`AllocationResults`, `Rankings`/`MarginRanking`, `Trends`, and `Flags`.

- **Customer view**: revenue, returns/discounts, direct costs,
  contribution, allocated profitability, periods, segment/group,
  coverage, rankings. Does not automatically import AR collection cost
  — a caller who wants that models it as an explicit `DIRECT_OTHER` or
  `VARIABLE_OTHER` Fact.
- **Job view**: direct material/labor/subcontractor/fulfillment/other,
  contribution, allocated overhead, and (via `Policy.PrimaryDriverKey`)
  per-hour economics when an `hours`-keyed `DriverObservation` is
  supplied. Builds no project scheduling or percent-complete accounting.
- **Product view**: revenue, returns/discounts, direct cost, labor,
  fulfillment, variable fees/commissions, and unit economics. Never
  derives product cost from inventory unless an authoritative cost fact
  is supplied — see [Inventory integration](#inventory-integration).

## Business totals and control reconciliation

`Result.BusinessTotals` is computed **directly from Facts/summaries and
SharedCostPools**, never by summing `DimensionView` totals. For every
period/dimension, `DimensionReconciliation` checks `Attributed +
Unattributed == BusinessAmount` within tolerance.

`ControlTotals{Period, NetRevenue, DirectCost, VariableCost,
SharedCost}` is an optional, availability-aware caller-supplied
business/GL control figure. `ControlReconciliation` compares each layer
**separately** (never one blended grand total, so offsetting
differences cannot hide each other) and `FlagControlTotalMismatch`
triggers per-layer once the difference meets
`Policy.ControlMateriality`.

## Rankings, trend, and margin-leakage analytics

`Rankings` (`TopByNetRevenue`, `TopByGrossProfit`,
`TopByContributionProfit`, `NegativeContribution`,
`NegativeAllocatedProfit`) are plain, deterministic sorts by one factual
metric — **no composite score.** `MarginRanking` excludes any entity
whose `NetRevenue` is below `Policy.MinimumRevenueForMarginRanking`, so
a tiny-denominator entity cannot dominate a margin-based ranking.

`EntityTrend` reports adjacent-period changes (`AdjacentChange`, with
`FromPeriod`/`ToPeriod`/`AbsoluteChange`/`PercentChange`) for net
revenue, gross/contribution/allocated profit and margin, and direct
labor/material percent-of-revenue, returns/discount rate — purely
historical comparisons, **no prediction.**

`MarginLeakage` exposes `ReturnRate`, `DiscountRate`,
`DirectMaterialPercentRevenue`, `DirectLaborPercentRevenue`,
`SubcontractorPercentRevenue`, `FulfillmentPercentRevenue`,
`VariableCommissionPercentRevenue`, `PaymentFeePercentRevenue` — every
field a `Value`, unavailable when its denominator is zero.

## Flags and materiality

Every `Flag` is a simple, documented threshold comparison the caller
can see and override via `Policy` — never a hidden composite score. See
[`flags.go`](../accounting/profitability/flags.go) for the full
`FlagCode` taxonomy (negative profit signals, margin compression,
revenue-growth-with-deterioration, high return/discount rate,
cost-share-increasing, attribution/allocation coverage gaps, and
control mismatches).

`NEGATIVE_CONTRIBUTION`/`NEGATIVE_ALLOCATED_PROFIT` use factual codes,
never a broad "unprofitable" label, and are gated by
`Policy.NegativeContributionMateriality` — this package does not label
an entity unprofitable when data/allocation coverage is incomplete.
`SHARED_COST_POOL_UNALLOCATED` (not allocated — expected default, or
allocation-basis-denominator-unavailable) is distinguished from
`PARTIALLY_ALLOCATED_SHARED_COST` (allocation was requested and ran,
but the trace's sum fell short of the pool total, which should not
normally happen and would itself indicate a bug if seen).

`MaterialityPolicy{AbsoluteAmount, PercentOfRevenue}` is used for
unattributed amounts, control mismatches, and negative-contribution/
negative-allocated-profit gating — deliberately never called "audit
materiality," and this package never invents a target margin or
materiality default of its own.

## Integration boundaries

This package has no compile-time dependency on `accounting/ledger`,
`accounting/statements`, `accounting/labor`, `accounting/inventory`, or
`analytics/*`. Each integration below is a small, typed, pure adapter
function plus a demonstrating test — never a hidden import inside
`Calculate`.

### Statements integration

[`statementsadapter.go`](../accounting/profitability/statementsadapter.go)'s
`ControlTotalsFromFinancialDataset` sums `financial.CategoryRevenue`
codes into `ControlTotals.NetRevenue` and `financial.CategoryCogs`
codes into `ControlTotals.DirectCost` for one period. This package
never forces statement COGS to equal a caller's own Fact-level
`DIRECT_*` classification — a mismatch between the two surfaces as a
genuine, informative `ControlTotalMismatch` rather than being silently
forced to match.

### Labor integration

[`laboradapter.go`](../accounting/profitability/laboradapter.go)'s
`DirectLaborFactFromPayrollRecord` converts one
`accounting/labor.PayrollRecord`'s full loaded cost (regular + overtime
+ bonus + commission + other pay, plus employer taxes/benefits/other
cost) into a single `DIRECT_LABOR` `Fact`, given caller-supplied
`Attribution`s. `DirectSubcontractorFactFromContractorRecord` is the
`ContractorLaborRecord` counterpart, mapping to
`DIRECT_SUBCONTRACTOR`. Neither adapter computes or exposes any
employee-performance, productivity, or evaluation metric — `Fact` has
no field for one.

### Inventory integration

[`inventoryadapter.go`](../accounting/profitability/inventoryadapter.go)'s
`ProductCostFactFromInventorySnapshot` only uses
`accounting/inventory.InventorySnapshot.UnitCost` — an **authoritative,
caller-supplied** fact — multiplied by a caller-supplied sale quantity.
It never derives a per-unit cost from turnover, valuation, or any other
`accounting/inventory` analytic. If `UnitCost` is unavailable, the
adapter returns `(Fact{}, false)`: product cost is unavailable for that
item, never fabricated.

### Ledger integration

[`ledgeradapter.go`](../accounting/profitability/ledgeradapter.go)'s
`FactsFromLedgerBalances` requires an explicit
`LedgerAccountMapping{AccountID, Component, Attributions}` per account
— **no name-based dimension or Component inference.** A
`ledger.Balance` whose account has no mapping entry is skipped
entirely.

### Revenue concentration

[`concentrationadapter.go`](../accounting/profitability/concentrationadapter.go)
converts a dimension's `EntityPeriodResult`s into
`analytics/concentration.Observation`s of `NetRevenue`; the caller runs
`concentration.Calculate` itself and converts the result back into this
package's `RevenueConcentration` (`Top1Share`/`Top3Share`/`Top5Share`/
`Top10Share`/`HHI`) via a pure field mapping — the concentration math
itself stays byte-identical to `analytics/concentration`'s own output.
Called **revenue concentration**, never "dependency risk."

## No-double-count invariant

Locked by `TestInvariant_NoDoubleCountAcrossDimensions`,
`TestInvariant_PartialAttribution`,
`TestInvariant_BusinessComponentTotalsMatchSourceFacts`,
`TestInvariant_DimensionReconciliation`,
`TestInvariant_AllocationReconciliation`, and
`TestInvariant_AllPeriodMarginIsNotAverage`:

1. **Business total**: business component totals equal the sum of
   source Fact amounts by component, regardless of attribution.
2. **Dimension reconciliation**: Attributed + Unattributed == Business
   total, for every dimension/period/component.
3. **Allocation**: Allocated + Unallocated == Pool total. No pool
   amount is ever double counted across dimension views.
4. **No cross-dimension double count**: one $1,000 fact attributed 100%
   to one customer, one job, and one product yields $1,000 in each
   view, never $3,000 of business revenue.
5. **Partial attribution**: $1,000 revenue with 60% P1 + 20% P2 yields
   $600/$200/$200 unattributed.
6. **Margin aggregation**: all-period margin is total profit / total
   revenue, never the arithmetic average of period margins.

## Availability, coverage, and view status

`ViewStatus` (`AVAILABLE`, `AVAILABLE_WITH_GAPS`, `NOT_APPLICABLE`,
`UNAVAILABLE`, `INVALID`) is **never inferred from an empty slice** — a
dimension is `NOT_APPLICABLE` only when `Policy.DimensionApplicability`
explicitly says so, and `UNAVAILABLE` only when literally nothing
(entity, fact, or summary row) ever referenced it.

`Value{Available, Amount}` distinguishes a known-zero figure from one
that was never supplied — used throughout `EntityPeriodSummaryInput`,
`ControlTotals`, margins, and per-unit metrics.

`Result.Coverage` reports factual coverage — attribution by dimension,
driver coverage, allocation coverage, and control-total coverage —
**never an opaque score.**

## Issue taxonomy

Only codes this package actually emits are defined — see
[`issues.go`](../accounting/profitability/issues.go): `INVALID_PERIOD`,
`DUPLICATE_PERIOD`, `DUPLICATE_ENTITY`, `UNKNOWN_ENTITY`,
`DUPLICATE_FACT`, `INVALID_COMPONENT`, `NON_FINITE_AMOUNT`,
`NEGATIVE_AMOUNT`, `INVALID_ATTRIBUTION`,
`ATTRIBUTION_EXCEEDS_100_PERCENT`, `INVALID_SUMMARY_ROW`,
`INVALID_DRIVER`, `DUPLICATE_DRIVER`, `INVALID_SHARED_COST_POOL`,
`DUPLICATE_SHARED_COST_POOL`, `INVALID_ALLOCATION_RULE`,
`DUPLICATE_ALLOCATION_RULE`, `ALLOCATION_DENOMINATOR_UNAVAILABLE`,
`INVALID_FIXED_WEIGHTS`, `MIXED_CURRENCY`, `INVALID_CONTROL_TOTAL`,
`INVALID_POLICY`, `INPUT_MODE_CONFLICT`. Validation never silently
repairs input — an offending row/attribution/rule is excluded and a
structured `Issue` recorded.

## Neutral language

Every generated `Flag`/`Issue` message uses neutral, factual language.
This package never says "drop this customer," "discontinue this
product," "fire this customer," "raise/lower price," "best/worst
customer," or issues any pricing, termination, or discontinuation
recommendation — enforced by
[`safety_test.go`](../accounting/profitability/safety_test.go)'s
prohibited-term regression scan. This package informs decisions; it
does not make them.

## Versions

- `SchemaVersion` — the shape of every public contract (`Entity`,
  `Fact`, `Attribution`, `PeriodInfo`, `EntityPeriodSummaryInput`,
  `DriverObservation`, `SharedCostPool`, `AllocationRule`,
  `ControlTotals`, `Policy`, `EntityPeriodResult`, `DimensionView`,
  `BusinessTotals`, `Flag`, `Issue`, `Coverage`, `Result`).
- `FormulaVersion` — the profitability bridge, attribution resolution,
  allocation math, coverage, rankings, trend/margin-leakage, and every
  flag-trigger rule.

Both are bumped whenever a change could make a historical persisted
`Result` fail to reproduce identically under new code.

## Limitations

- This package performs no pricing optimization, customer-termination
  or product-discontinuation recommendation, job scheduling, project
  workflow, CRM/ERP/QuickBooks/Xero integration, commission
  calculation, inventory costing, payroll costing, tax profitability,
  transfer pricing, automatic allocation-basis selection, AI narrative,
  demand forecasting, or churn prediction.
- The caller supplies classification, attribution, and allocation
  policy; this package performs deterministic arithmetic and
  diagnostics on top of them.
- `GrossProfit` will only match a financial-statement gross profit
  figure when the caller's Component classification aligns with that
  statement's own COGS definition — this package does not force the
  two to agree.
- One reporting currency per `Calculate` call — no FX fetching or
  silent mixed-currency aggregation; a non-matching currency is
  excluded with `IssueMixedCurrency`.

## Explicit non-goals

This package does not implement: pricing optimization/recommendations,
customer-termination recommendations, product-discontinuation
recommendations, job scheduling, project workflow, CRM, commission
calculation, inventory costing, payroll costing, tax profitability,
transfer pricing, automatic allocation-basis selection, AI narrative,
demand forecasting, or churn prediction. See also the main README's
[What this project intentionally does not
contain](../README.md#what-this-project-intentionally-does-not-contain).
