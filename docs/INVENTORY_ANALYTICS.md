# Inventory analytics (`accounting/inventory`)

`accounting/inventory` provides deterministic inventory analytics for
accountants, controllers, CFOs, and operating/business-advisory
workflows: on-hand quantity and value, turnover and Days Inventory
Outstanding (DIO), value aging with slow/non-moving detection, last-
movement and velocity metrics, caller-defined stock-level (min/target/
max/reorder) comparison, inventory value concentration, purchases-vs-
usage and inventory-build signals, adjustment/write-off review, expiry
review, GL/subledger reconciliation, and quantity/value rollforwards —
from a portable, caller-supplied inventory-facts model.

Like every other package in this repository, it contains **no**
persistence, HTTP/API, auth, UI, background jobs, QuickBooks/Xero/
Shopify/ERP/WMS integration, AI/LLM, or ML demand forecasting — see
[What this project intentionally does not
contain](../README.md#what-this-project-intentionally-does-not-contain)
in the main README, and this doc's own [Explicit
non-goals](#explicit-non-goals) section below. Every exported function is
pure (no I/O, no mutation of caller-owned input, no package-global
mutable state, no wall-clock reads) and deterministic — see
[`determinism_test.go`-equivalent coverage in
`safety_test.go`](../accounting/inventory/safety_test.go) and
[`immutability`-equivalent coverage, also in
`safety_test.go`](../accounting/inventory/safety_test.go).

## Purpose and ERP/WMS/costing boundary

**This is not an ERP, warehouse-management system, costing engine, or tax
inventory system.** It never posts inventory transactions, manages
warehouse operations, performs barcode/scanning, creates purchase orders,
executes replenishment, computes an economic order quantity, forecasts
demand, runs a FIFO/LIFO/specific-identification costing engine, renders
a GAAP/IFRS write-down opinion, posts an automatic journal entry, or
calculates product profitability. Every quantity, cost, and value figure
is a fact the caller supplies, or a simple, documented arithmetic
combination of caller-supplied facts (quantity × unit cost); this package
never invents a cost, a demand estimate, or a shelf life.

This package is not `accounting/ar` or `accounting/ap` — an inventory
item is not a receivable/payable — though it mirrors their shape in
spirit (an aging/concentration/GL-reconciliation analytics engine with a
neutral-language flag/issue split). It is independent of
`accounting/ledger` and `accounting/statements`: this package never
imports either, and integration (subledger-to-GL-control reconciliation,
statement-builder inventory balance, working-capital component, ratios
DIO) is demonstrated only via portable typed input and adapter tests —
see [Statement / working-capital / ratios
integrations](#statement--working-capital--ratios-integrations).

## Package composition

| File(s) | Responsibility |
|---|---|
| `inventory.go` | Package doc, `Value`/`Qty` availability wrappers, `SourceRef`/`Dimension` |
| `item.go`, `snapshot.go`, `movement.go` | Core domain models: `Item`, `InventorySnapshot`, `Movement` |
| `period.go`, `financials.go` | `PeriodInfo`, `PeriodFinancials` (summary input path), `GLControl` |
| `stockpolicy.go` | `StockPolicy`, `StockPolicyResult`, `BoolResult` |
| `uom.go` | `UOMConversion` and the explicit-only conversion table |
| `nrv.go` | `MarketValue`/`MarketValueComparison` (optional NRV comparison) |
| `buckets.go` | Value-aging `BucketDefinition`, `DefaultBuckets`, validation |
| `policy.go` | The single `Policy` struct, defaults, validation |
| `issues.go`, `flagtypes.go` | `Issue`/`IssueCode` and `Flag`/`FlagCode` taxonomies |
| `value.go` | Snapshot/movement value resolution (quantity × cost vs. explicit) |
| `itemstate.go` | Internal per-item as-of-date state (latest snapshot per location/lot, last-movement dates, aggregated quantity/value) |
| `validate.go` | Structural validation for every input type |
| `aging.go` | `AgingSummary`, `ItemLastMovement`, slow/non-moving classification |
| `turnover.go` | `TurnoverResult`, `DIOResult`, `TurnoverHistory` |
| `usage.go` | `PurchaseSummary`, `UsageSummary`, `PurchaseVsUsageTrend` |
| `velocity.go` | `ItemVelocity`, `SupplyDuration` |
| `concentration.go` | `ConcentrationSummary` (reuses `analytics/concentration`) |
| `composition.go` | `CompositionSummary` (value/share by `InventoryClass`) |
| `adjustments.go` | `AdjustmentSummary` |
| `duplicates.go` | `PossibleDuplicateMovement` detection |
| `expiry.go` | `ExpirySummary`, `LotExpiry` |
| `reconcile.go` | `ReconciliationSummary`, `QuantityRollforward`, `ValueRollforward` |
| `portfolio.go` | `PortfolioSummary` |
| `materiality.go` | `MaterialityAssessment` |
| `coverage.go` | `Coverage` (factual data-coverage counts) |
| `flags.go` | `computeFlags` — every deterministic flag-trigger rule |
| `result.go`, `calculate.go` | `Input`, `Result`, `PeriodSummary`, and the `Calculate` orchestrator |
| `versions.go` | `SchemaVersion`, `FormulaVersion` |

## Entry point

```go
result := inventory.Calculate(inventory.Input{
    AsOfDate: "2025-06-30",
    Items:     items,
    Snapshots: snapshots,
    Movements: movements,
}, inventory.Policy{
    SlowMovingDays: 90,
    NonMovingDays:  180,
})
```

`Calculate` never returns an `error`; every input/configuration problem
becomes a structured `Issue`, and `Result.Available` is `false` only when
`Calculate` could not proceed at all (no valid `AsOfDate` and no valid
`Periods`).

## Two convergent input levels

A caller may supply either or both of:

- **Detailed path**: `Input.Items` + `Input.Snapshots` + `Input.Movements`
  — SKU/movement-level facts.
- **Summary path**: `Input.Periods` + `Input.Financials`
  (`PeriodFinancials`: beginning/ending inventory, COGS, purchases,
  units) — already-aggregated period data.

Both paths converge on the same `PeriodSummary` shape wherever the
underlying semantics are equivalent: `Turnover`, `DIO`,
`PurchaseVsUsage`. `PeriodFinancials`' own `BeginningInventoryValue`/
`EndingInventoryValue`/`COGS` take precedence over a snapshot-derived
reconstruction whenever both are supplied for the same period, since a
caller who supplies period-summary financials is the authority on that
period's own reported figures.

Item/SKU-level detail — aging, slow/non-moving, stock policy, item value
concentration, expiry, quantity/value rollforward — is naturally only
available from the detailed path; there is no period-summary equivalent
for "which specific items are slow-moving."

## Costing boundary

This package never derives an authoritative FIFO/LIFO/specific-
identification cost basis from raw purchase/shipment history. Every
`InventorySnapshot.UnitCost`, `Movement.UnitCost`/`Amount`, and
`PeriodFinancials.COGS` is a fact the caller supplies. **Accounting
valuation method remains the caller/source-system's responsibility.**

The only arithmetic this package performs on cost data is the value/
quantity resolution described below (quantity × unit cost, when an
explicit extended value is not separately supplied) — never a cost-flow
assumption about which unit was sold first.

## Value / quantity semantics

For each `InventorySnapshot` (and, identically, each `Movement`), value
is resolved by `resolveSnapshotValue`/`resolveMovementValue`:

1. If an explicit extended value (`InventoryValue`/`Amount`) is supplied,
   it is used as-is — the more direct fact.
2. Otherwise, if both `QuantityOnHand`/`Quantity` and `UnitCost` are
   available, an analytical value is computed as their product.
3. If **both** are supplied and disagree beyond a small floating-point
   tolerance, the explicit value still wins (never silently overwritten
   in either direction), but `IssueValueQuantityCostMismatch` is
   reported so the discrepancy is never silent.

A known-zero quantity/value is a real fact (`Available: true, Amount:
0`), never conflated with "unavailable" (no supplied data at all).

### Negative inventory

Negative on-hand quantity/value can legitimately result from timing/
system issues (e.g. a shipment recorded before its matching receipt).
`Policy.NegativeInventoryHandling` controls the response:

- `ALLOW_WITH_WARNING` (default): the row is included in every
  calculation, and `FlagNegativeInventory` is reported.
- `REJECT`: the row is excluded, with an error-severity `Issue`.

Neither mode auto-corrects a negative figure to zero.

## Turnover and Days Inventory Outstanding (DIO)

```
Inventory Turnover = COGS / Average Inventory
DIO = (Average Inventory / COGS) × Days
```

`AverageInventory` is resolved with an explicit, labeled basis
(`AverageBasis`):

- `BEGINNING_ENDING` (default): `(Beginning + Ending) / 2`.
- `CALLER_SUPPLIED`: an explicit higher-frequency average, when one is
  given directly.
- `ENDING_ONLY`: used only when no beginning balance is available at
  all — a single point-in-time balance, clearly labeled as such.

**Turnover** is available whenever COGS and a nonzero average inventory
are both available — including the legitimate case of `Turnover = 0`
when COGS is a known zero (a real ratio, not "unavailable"). It is
unavailable only when average inventory is zero/unavailable (a genuinely
undefined division).

**DIO**, by contrast, divides *by* COGS: it is `Unavailable` (never
`Inf`) whenever COGS is zero or unavailable, or `Days <= 0`. `Days`
comes from `PeriodInfo.Days` if supplied, otherwise the period's own
inclusive calendar-day count — so DIO can legitimately differ slightly
across periods of different lengths (a 92-day quarter vs. a 90-day
quarter) even with identical average inventory and COGS; this is correct
days-outstanding math, not a defect (see
`TestTurnoverHistory_DaysVaryByCalendarPeriodLength`).

`TurnoverHistory` reports the chronological series plus adjacent-period
change, first-vs-last change, and a `DIOTrend` label
(`improving`/`deteriorating`/`flat`) — direction only, never a value
judgment (a rising DIO from a deliberate stock-up is not automatically
"bad").

Item/category-level turnover (`ItemTurnover`) is only ever computed from
caller-supplied item/category-level COGS/usage cost — **this package
never allocates company-level COGS to items using guessed percentages**.
V1 has no dedicated per-item COGS input field; see [Limitations](#limitations).

## Aging

Value aging is computed per item/lot, based only on supplied aging
evidence, in this preference order (task-mandated, never a further
guess):

1. The lot's own `InventorySnapshot.ReceivedDate`.
2. The item's `LastReceiptDate` (from a `PURCHASE_RECEIPT` movement).
3. The item's `LastAnyMovementDate`.
4. `AgeEvidenceUnknown` — the age is genuinely unknown.

This package never reconstructs exact stock age from a current snapshot
plus aggregate movement history when inventory-layer attribution is
ambiguous (e.g. multiple lots of the same item).

### Buckets

Default value-aging buckets: `0-30`, `31-60`, `61-90`, `91-180`,
`181-365`, `365+`. A caller supplies `Policy.Buckets` for a custom
schema. Unlike AR/AP due-date aging, **a gap in the bucket schema is
legal** — "current" has no accounting meaning here, and an item falling
into an uncovered gap simply resolves to the reserved `UNKNOWN` bucket
code, alongside items lacking age evidence entirely.

`AgingSummary.UnknownAgeValue`/`UnknownAgePercent` report this
explicitly — important for data-quality visibility, never silently
folded into the youngest or oldest bucket.

### Slow-moving / non-moving

Configured via `Policy.SlowMovingDays`/`NonMovingDays`
(`NonMovingDays >= SlowMovingDays`). Classification is based on days
since the item's `LastOutboundDate` (falling back to
`LastAnyMovementDate` if no outbound evidence exists at all) — **this
package never invents a default obsolescence period**: leaving
`SlowMovingDays` at zero (the default) means `SlowMovingValue`/
`NonMovingValue` are simply `Unavailable`, not zero.

`SLOW_MOVING_INVENTORY`/`NON_MOVING_INVENTORY`/`AGED_INVENTORY_REVIEW`/
`WRITE_DOWN_REVIEW_CANDIDATE`-style language throughout is a review
candidate, never an accounting write-down conclusion.

## Last movement and velocity

For each item, `ItemLastMovement` reports `LastReceiptDate`,
`LastOutboundDate`, `LastAnyMovementDate`, and the corresponding
`DaysSinceOutbound`/`DaysSinceAnyMovement` — computed against the
explicit `AsOfDate`, never `time.Now()`.

For a caller-supplied `VelocityWindow` per item (`Input.VelocityWindows`),
`ItemVelocity` reports `OutboundQuantityPerDay`/`OutboundQuantityPerWeek`/
`OutboundValuePerDay`. `SupplyDuration` (`QuantityOnHand / Recent
AverageOutboundQuantity`) is `Unavailable` — never `Inf` — when there was
no recent outbound movement in the window. This is explicitly not demand
forecasting: no extrapolation into the future is performed.

## Stock-policy analytics

`StockPolicy` carries explicit, optional `MinimumQuantity`/
`TargetQuantity`/`MaximumQuantity`/`ReorderPoint`, each paired with a
`*Set` bool distinguishing "explicitly zero" from "not supplied at all."
`StockPolicyResult` reports `BelowMinimum`/`AboveMaximum`/
`BelowReorderPoint`/`ShortfallVsMinimum`/`ExcessQuantityVsMaximum` — each
individually `Unavailable` unless its corresponding policy field was set.

**Without an explicit `StockPolicy` for an item, this package makes no
overstock/understock claim about it at all** — no `StockPolicyResult`
row is even produced. Old/slow-moving findings remain independently
available regardless. This package never infers a reorder point,
minimum, or maximum from historical usage, and never creates a purchase
order.

## Inventory value concentration

`ConcentrationSummary` reports value concentration by item, category, and
location — reusing `analytics/concentration.Calculate` directly for
share/HHI/top-N math (the same reuse pattern `accounting/ar` and
`accounting/ap` already use for their own concentration needs), since
"concentration among entities by dollar amount" is identical arithmetic
regardless of what the entity represents.

Location totals are built purely from each contributing
`InventorySnapshot`'s own `Location` (an item can be split across
multiple locations, one snapshot per (location, lot)) — never from
`Item.Location` (a display/default field), which would double-count a
multi-location item.

**This is inventory VALUE concentration — never supplier or customer
dependency.** `ConcentrationSummary.Label` says so explicitly on every
result.

## Purchases vs. usage

`PurchaseSummary`/`UsageSummary` are built from `PURCHASE_RECEIPT` and
outbound-type movements respectively when movement data exists for a
period, falling back to `PeriodFinancials.PurchaseValue`/`PurchaseUnits`/
`UnitsSold` on the summary path. `UsageSummary` distinguishes customer
shipment, production issue, transfer-out, return-out, and other-out
value — **write-offs are never folded into ordinary usage** (they are
adjustment activity; see [Adjustment / write-off
analytics](#adjustment--write-off-analytics)), and this package never
treats every outbound movement as COGS.

`PurchaseVsUsageTrend.NetInventoryBuild` is `PurchasesValue -
OutboundUsageValue`. `FlagPurchasesOutpaceUsage` triggers only past a
caller-configured threshold (`Policy.PurchaseVsUsageThreshold`) — never
an automatic "excessive inventory" conclusion.

`FlagInventoryBuildWithoutMatchingCOGSGrowth` compares first-vs-last
inventory growth against COGS growth across supplied `PeriodFinancials`
— it fires only when compatible COGS data exists, and asserts no causal
conclusion (it is not automatically "bad" for inventory to build ahead
of sales, e.g. a deliberate seasonal stock-up).

## Adjustment / write-off analytics

`AdjustmentSummary` summarizes `ADJUSTMENT_INCREASE`/
`ADJUSTMENT_DECREASE`/`WRITE_OFF` movement activity per period:
`AdjustmentIncrease`, `AdjustmentDecrease` (already negative-signed),
`WriteOffValue`, `NetAdjustment`, `AdjustmentCount`,
`AbsoluteAdjustmentValue`, and `AdjustmentRate` (=
`AbsoluteAdjustmentValue / AverageInventory`, when a denominator is
available).

Deterministic review flags:

- `HIGH_INVENTORY_ADJUSTMENT_RATE` — `AdjustmentRate` beyond
  `Policy.AdjustmentRateThreshold`.
- `LARGE_WRITE_OFF` — a period write-off total beyond
  `Policy.LargeWriteOffThreshold` (defaults from resolved materiality).
- `REPEATED_ITEM_ADJUSTMENTS` — an item with at least
  `Policy.RepeatedItemAdjustmentCount` adjustment/write-off movements.
- `PERIOD_END_INVENTORY_ADJUSTMENT` — at least one adjustment/write-off
  movement dated within `Policy.PeriodEndAdjustmentWindowDays` (inclusive)
  of the period's own `EndDate` (`AdjustmentSummary.PeriodEndAdjustmentCount`)
  — a genuine date-proximity check against period end, not merely "any
  adjustment activity occurred somewhere in the period." A neutral timing
  observation, not a fraud inference.

**Neutral language only.** This package never uses "shrinkage," "fraud,"
or "theft" in any generated message — `safety_test.go` enforces this as
a permanent regression test. A caller-supplied `Movement.ReasonCode`
(e.g. `"SHRINKAGE"`) is preserved verbatim as source data on the
`Movement` itself; it is never surfaced as this package's own inference,
label, or conclusion.

## Expiry analytics

For lots carrying `ExpiryDate`, `ExpirySummary` classifies each into
`NONE`/`OK`/`EXPIRING` (within `Policy.ExpiryWarningDays` of `AsOfDate`)/
`EXPIRED` (on or before `AsOfDate`). **Inventory already expired as of
`AsOfDate` is a finding, not invalid input** — `EXPIRED_INVENTORY_REVIEW`
and `EXPIRING_INVENTORY` are the corresponding flags. This package never
infers a shelf life, and never automatically writes off expired
inventory — its `InventoryValue` is preserved unchanged in every
portfolio total.

`ExpiryDate` before `ReceivedDate` **is** invalid input
(`IssueInvalidExpiryDate`).

### Optional NRV / market-value comparison

If a caller supplies `MarketValue` (`NetRealizableValue`, or
`ExpectedSellingPrice` − `DisposalCosts`), `MarketValueComparison`
reports `InventoryCost` vs. the resolved NRV and their `Difference`,
labeled as an analytical comparison. This package never sources a
selling price, never infers NRV on its own, and never automatically
posts a write-down.

## GL / subledger reconciliation

`ReconciliationSummary` compares the subledger inventory total against
one or more caller-supplied `GLControl` balances. `GLControl.Component`
(`""` for a single combined account, or a label such as
`"RAW_MATERIAL"`/`"FINISHED_GOOD"` matching a known `InventoryClass`)
supports multi-component reconciliation — **using caller-defined
component/category mapping, never a hard-coded manufacturing
requirement**. Reconciliation never adjusts either side; a mismatch is
reported (`FlagInventoryGLMismatch`), never masked.

## Quantity / value rollforwards

`QuantityRollforward`/`ValueRollforward` test:

```
Beginning + Inbound - Outbound +/- Adjustments = Ending
```

separately for quantity (UOM-safe: computed only when beginning, ending,
and every contributing movement share one unit of measure) and value
(computed only when beginning, ending, and every contributing movement's
value are all available). **This package never forces a rollforward when
source data is incomplete** — it is simply `Unavailable` rather than a
fabricated tie. A genuine mismatch reports `Difference` with explicit
evidence, never silently accepted.

## UOM and currency safety

Quantities from incompatible units of measure are **never summed
silently**. An item's on-hand quantity aggregates across snapshots only
when every contributing snapshot's UOM matches (or an explicit
`Policy.UOMConversions` entry resolves the conversion); otherwise the
aggregate quantity is `Unavailable` for that item (value still
aggregates normally, since money is unit-agnostic). `UOMConversion` is
never inferred — a caller must supply an explicit `{From, To, Factor}`
row, and the inverse direction is not auto-derived from a forward one.

Mixed currencies across `Item.Currency` are resolved to a single
reporting currency (`Policy.ReportingCurrency`, or the most common
currency among supplied items) and reported via `IssueMixedCurrency`;
items outside that currency are excluded from every downstream
aggregate — this package never fetches FX or silently sums currencies.

## Coverage

`Coverage` reports factual data-coverage counts/percentages —
`SnapshotCoverage`, `UnitCostCoverage`, `InventoryValueCoverage`,
`MovementCoverage`, `AgeCoverage`, `LocationCoverage`,
`CategoryCoverage`, `COGSCoverage`, `GLReconciliationAvailable`,
`StockPolicyCoverage` — **never an opaque quality score**.

## Issues vs. Flags

- **Issue**: an input/configuration/integrity problem (duplicate item,
  unknown item reference, non-finite value, mixed currency, conflicting
  snapshot, invalid policy). `IssueSeverity` is `error` (excludes the row
  or blocks a portion of the analysis) or `warning` (advisory).
- **Flag**: a business/accounting condition this package's own
  deterministic rules detected (slow-moving, negative inventory, GL
  mismatch, large adjustment, expired inventory). Never a hidden
  inventory-health score — every flag is a simple, documented threshold
  comparison a caller can fully see and override via `Policy`.

These are kept strictly separate; see `issues.go`/`flagtypes.go` for the
full stable-code taxonomies.

## Statement / working-capital / ratios integrations

`accounting/inventory` has no compile-time dependency on
`accounting/statements`, `analytics/workingcapital`, or `analytics/ratios`
— every integration below is a portable typed-adapter test, not a
package dependency:

- **`statementsadapter_test.go`**: a synthetic ledger built through
  `accounting/statements` produces a `CodeBsInventory` balance that
  exactly matches this package's `PortfolioSummary.TotalInventoryValue`
  for the same underlying facts.
- **`workingcapitaladapter_test.go`**: this package's
  `TotalInventoryValue`, fed into a `financial.FinancialDataset` as
  `CodeBsInventory`, contributes exactly that amount to
  `analytics/workingcapital`'s `OperatingCurrentAssets` under
  `DefaultInclusionPolicy`.
- **`ratiosadapter_test.go`**: this package's own DIO exactly matches
  `analytics/ratios`' `DaysInventoryOutstanding` under the one case where
  their average-inventory semantics are genuinely equivalent —
  `analytics/ratios` uses a single ending-balance figure
  (`financial.CodeBsInventory`), never a beginning/ending average, so
  equality is demonstrated specifically under this package's
  `AverageBasisEndingOnly` mode (no beginning balance supplied), not
  claimed universally. Where the two packages' average-basis semantics
  genuinely differ (this package's own default is
  `(Beginning + Ending) / 2`), that difference is documented here rather
  than forced to agree.

`PeriodFinancials` (this package's own portable Revenue/COGS input) is
never `financial.FinancialDataset` — a caller populates it from
`financial/metrics` or `analytics/ratios` output via their own adapter,
exactly as these integration tests demonstrate.

## JSON and determinism

Every exported type uses explicit `snake_case` JSON tags and round-trips
through `encoding/json` without loss (see `json_test.go`). No field is
ever serialized as `NaN`/`Inf` — every "genuinely undefined" ratio
(zero-COGS DIO, zero-velocity supply duration, zero-average turnover)
reports `Available: false` with `Amount`/`Value` left at its zero value
instead.

Output ordering is fully deterministic and independent of Go map
iteration order: periods chronologically, items by value descending then
`ItemID` ascending, categories/locations by normalized name, aging
buckets in their defined order (`UNKNOWN` last), flags by declaration
order then `ItemID` then `Period`, issues in validation-pass order. Every
map-backed aggregation sorts its keys before floating-point accumulation
— see `itemstate.go`'s `sortLocationLotKeys` and its accompanying
comment.

`Calculate` never mutates caller-owned input (see `safety_test.go`'s
`TestSafety_NoMutationOfInput`), is safe for concurrent use (see
`TestSafety_ConcurrentCalls`, run with `-race`), and produces
byte-for-byte identical JSON across repeated calls against identical
input (see `TestSafety_DeterministicRepeatedExecution`).

## Versioning

- `SchemaVersion` — this package's public result/schema contract: the
  shape of `Item`, `InventorySnapshot`, `Movement`, `StockPolicy`,
  `PeriodInfo`, `PeriodFinancials`, `GLControl`, `Policy`,
  `PeriodSummary`, `ItemAging`, `AgingSummary`, `ReconciliationSummary`,
  `Flag`, `Issue`, `Coverage`, `Result`.
- `FormulaVersion` — this package's fixed calculation semantics: value/
  quantity resolution, turnover/DIO, aging buckets and slow/non-moving
  thresholds, last-movement/velocity, stock-policy comparison,
  concentration, purchases-vs-usage and inventory-build signals,
  adjustment/write-off review, expiry review, reconciliation and
  rollforwards, and every flag-trigger rule.

Both are echoed on every `Result` (`Result.SchemaVersion`,
`Result.FormulaVersion`) and bumped whenever the corresponding contract
changes in a way that could make a historical persisted `Result` not
reproduce identically under new code — see the main README's
[Versioning strategy](../README.md#versioning-strategy).

## Limitations

- Item/category-level (`ItemTurnover`) turnover has no dedicated
  per-item COGS input field in V1 — it is defined at the type level but
  not yet wired through `Calculate`. A caller with item-level COGS/usage
  cost can compute the same `COGS / AverageInventory` ratio directly
  using this package's exported `Value`/`Qty` types.
- `AdjustmentSummary.PeriodEndAdjustmentCount` performs a genuine
  date-proximity check per movement, but reports only a count, not which
  specific `Movement.ID`s fell within the window — a caller wanting that
  detail compares `Movement.Date` against `PeriodInfo.EndDate` directly.
- GL component reconciliation (`GLControl.Component`) resolves against
  `CompositionSummary.ByClass` only when `Component` matches a known
  `InventoryClass` string; there is no separate caller-defined component-
  mapping input beyond `Item.Class` in V1.
- `findPossibleDuplicateMovements` bounds pairwise comparison within one
  matching-signature group to `maxDuplicateGroupSize` (50) to guarantee
  linear-time behavior even against an adversarial bulk-import shape
  (see `TestDuplicates_LargeGroupNeverBlowsUp`); a group larger than that
  reports pairs only among its first 50 members, not every combination.
- No serialized/lot-level asset tracking: `LotID` is preserved as
  provenance only, never turned into per-unit tracking.

## Explicit non-goals

This package does not implement: inventory transaction posting,
warehouse operations, barcode scanning, purchase orders, replenishment
execution, demand forecasting, automatic reorder quantities, economic
order quantity optimization, a FIFO/LIFO tax-accounting engine, GAAP/
IFRS write-down determination, automatic journal entries, product
profitability, supplier procurement analytics, warehouse-route
optimization, or AI demand planning. Customer/job/product profitability
(combining revenue, direct costs, labor, and inventory/product costs) is
a later module's responsibility; vendor spend analytics is a separate,
later module.
