# Vendor Spend Analytics (`accounting/vendorspend`)

`accounting/vendorspend` implements deterministic economic supplier spend
analytics from a portable purchase-record model, for accountants,
controllers, CFOs, and business-advisory workflows. It answers questions
like "who are our largest suppliers by spend," "how concentrated is
supplier spend," "which suppliers are growing/new/lost," "where is unit
price increasing," and "how much spend is uncategorized/one-time/
committed" — see the package doc comment for the full list.

Like every analytics-style package in this repository, it is
application-independent: it does not require `accounting/ap`,
`accounting/inventory`, `accounting/ledger`, `accounting/statements`, or
`analytics/*` input at all. A caller populates it directly from an AP/ERP
purchase export, a homegrown procurement system, or a synthetic fixture
— see `accounting/vendorspend/fixtures`.

## AP vs. spend

`accounting/ap` answers **what we currently owe suppliers** — an
open-item, balance-as-of-a-date question (`OpenAmount`, aged against an
explicit `AsOfDate`). This package answers **what we buy from suppliers
over time** — an economic period-spend question, driven by purchase/
receipt/accrual/cash events, not by outstanding-balance snapshots.

A supplier can have $0 ending AP and still be this business's largest
vendor by annual spend (every bill paid promptly), or vice versa (a
single large unpaid bill against otherwise modest annual purchasing).
This package never treats an `accounting/ap` balance as a substitute for
spend, and never asserts period spend equals ending AP —
`ap_boundary_test.go`'s `TestAPBoundary_SpendNeverBecomesEndingAP` is a
permanent regression test locking exactly this: a supplier with
$100,000 of annual spend and only $10,000 of ending AP must never report
`NetSpend == 10,000`.

The two packages can be bridged for a caller who wants both views from
one source — see [AP integration](#ap-integration) below — but this
package's core types never import `accounting/ap`.

## Supplier model

```go
type Supplier struct {
    SupplierID string
    Name       string
    Category   string
    Country    string
    Active     bool
    ParentID   string
    Dependency DependencyClass
    PreferredSupplier  bool
    ContractedSupplier bool
    SourceRef  SourceRef
}
```

`SupplierID` is identity throughout this package; `Name` is display-only
and never used as a matching key. `ParentID` optionally references
another `Supplier.SupplierID` for a caller-declared parent-company
relationship — this package never infers parent/child relationships from
`Name`, and performs no automatic parent-child spend rollup; `ParentID`
is validated only for existence and cycles (`IssueInvalidSupplierParent`,
guarded against infinite loops — see `validate_test.go`'s
`TestValidate_SupplierParentCycle`).

## Period model

`Period{Period, StartDate, EndDate, SequenceInYear}` requires
caller-supplied chronological periods with explicit start/end dates —
this package never infers fiscal periods or calls `time.Now()`.
`SequenceInYear` breaks ties for periods sharing a `StartDate`.

## Spend record

```go
type SpendRecord struct {
    SpendID, SupplierID, Period string
    Date       time.Time
    Amount     float64
    Currency   string
    Category, Subcategory string
    Quantity, UnitPrice Value
    UnitOfMeasure string
    Description, ReferenceID string
    SpendType  SpendType
    Effect     SpendEffect
    Recurrence RecurrenceType
    Commitment CommitmentType
    Basis      SpendBasis
    ProductID  string
    Location, Department, CostCenter string
    SourceRef  SourceRef
}
```

`Amount` is expected non-negative regardless of `Effect` — `Effect`,
never `Amount`'s sign, determines whether a record adds to or reduces
spend (mirrors `accounting/ap.DocumentType`'s identical "type, not sign,
determines semantics" convention, extended here to three reducing kinds).

## Spend types

A controlled, high-level taxonomy — deliberately not a chart-of-accounts
replacement:

```
GOODS  INVENTORY  RAW_MATERIAL  SUBCONTRACTOR  PROFESSIONAL_SERVICE
SOFTWARE  RENT  UTILITIES  MARKETING  LOGISTICS  INSURANCE
OTHER_OPERATING  CAPEX  OTHER
```

An unrecognized `SpendType` is advisory-only (`IssueInvalidSpendType`,
`SeverityWarning`): the record is still included, resolved to `OTHER`.

## Gross/credit/net bridge

```go
type SpendBridge struct {
    GrossSpend     float64
    CreditsRefunds float64
    NetSpend       float64
}
```

`Effect` distinguishes an ordinary purchase (`NORMAL`) from a reducing
event (`CREDIT`/`REFUND`/`REVERSAL`). `GrossSpend` sums only `NORMAL`
records; `CreditsRefunds` sums the magnitude of every reducing record
(reported as a positive "amount of credits," never a signed offset) so
gross purchasing is never hidden through netting. Locked invariant (see
`invariants_test.go`):

```
GrossSpend - CreditsRefunds = NetSpend
```

`SpendBridge` appears at every level this package reports: the overall
analysis, each `PeriodSummary`, each `SupplierPeriodSummary`, each
`CategorySummary`, and every other grouped-spend figure.

## Spend basis

`ACCRUAL | PURCHASE | RECEIPT | CASH | UNKNOWN` is required per-record
(`SpendRecord.Basis`). This package never silently mixes bases: mixed
bases among included records are always reported via
`Result.SpendByBasis` (a per-basis net-spend breakdown, computed
unconditionally), and flagged (`IssueMixedSpendBasis`, `SeverityWarning`
by default). `Policy.RequireSingleBasis` escalates that to a hard error
that aborts `Calculate` — see [Policy](#policy).

## Supplier/period/category summaries

`PeriodSummary` (overall per-period `SpendBridge`, supplier/transaction
counts, average/median transaction size, and `SpendByType/Category/
Department/Location/CostCenter` grouped breakdowns) and
`SupplierPeriodSummary` (the same per supplier/period, plus `SpendShare`,
`CategoryMix`, first/last spend date, and — when a single compatible
`UnitOfMeasure` exists — `TotalQuantity`/`WeightedAverageUnitPrice`) are
the two core building blocks every other analysis composes from.
`CategorySummary`/`ProductSpend`/`GroupedSpend` mirror the same shape for
the overall (all-period) view by category/product/department/location/
cost center.

## Concentration

`Result.Concentration` (`SpendConcentration`: `Top1/Top3/Top5/Top10`
shares and `HHI`, for the chronologically most recent period) is called
"**supplier spend concentration**," explicitly never "vendor dependency"
or "supplier failure risk" — a high concentration figure may simply
reflect a large recent purchase volume or negotiated terms, not
operational dependency. It reuses `analytics/concentration.Calculate`
directly (each supplier's `NetSpend` per period adapted into
`concentration.Observation` rows) rather than reimplementing HHI/top-N
share math — `invariants_test.go`'s
`TestInvariant_ConcentrationMatchesAnalyticsConcentration` locks this.
`analytics/concentration.Calculate` runs exactly **once** per `Calculate`
call; both the most-recent-period view and the full chronological history
(used only by the increasing-concentration flag check) are derived from
that single result, not two separate concentration passes.

`Policy.TopN` (default `[1, 3, 5, 10]`) selects which cutoffs are
reported — a cutoff absent from `TopN` leaves its `Top3/Top5/Top10`
field `Unavailable`; `Top1` is the one exception, always available from
the underlying `LargestEntityShare` figure regardless of `TopN`.

## Dependency metadata

`Supplier.Dependency` is a **caller-supplied**, factual label
(`CRITICAL | STRATEGIC | SINGLE_SOURCE | REPLACEABLE | UNKNOWN`) — never
inferred from spend share or any computed figure.
`Result.DependencySummary` only totals spend by that caller-supplied
label (`ByClass`, plus `CriticalSupplierSpend`/`CriticalSupplierSpend
Percent` called out by name since `CRITICAL` is the label most often
needed at a glance). No hidden risk score is derived from it anywhere.

## New/lost suppliers

`Result.NewLostSuppliers` detects new-supplier-activity and
supplier-spend-discontinued events across every chronologically adjacent
period pair, each optionally flagged `Material` against
`Policy.NewSupplierMaterialAmount`/`LostSupplierMaterialAmount`.
Detection requires **at least two chronological periods**; with only one
period, `NewLostSuppliers.Available` is `false` — this package never
calls every one-period supplier "new" (`scenarios_test.go`'s
`TestScenario_OnePeriod_NewLostUnavailable`).

## Growth and share trends

`Result.Trends` (`SpendTrends`) computes adjacent-period `GrowthChange`
for total spend, each supplier, and each category, plus per-supplier
`ShareChange` in raw percentage points (0.05 means 5 points — never a
percent-of-percent). Zero-base behavior is explicit: `PercentChange` is
`Unavailable` (never `+Inf`/`NaN`) when the prior period's amount was
zero, while `Direction` is still reported from the raw sign of the
change alone.

## Category analytics

`Result.CategorySummaries` (overall bridge/share/supplier-count per
category); category growth itself is reported via
`Result.Trends.CategorySpendGrowth` (keyed identically by `Category`)
rather than duplicated — growth is inherently an adjacent-period-pair
figure, and `SpendTrends` already owns that machinery.

## Quantity/UOM safety and unit-price analytics

`UnitPricePoint` is computed per (`SupplierID`, `ProductID`,
`UnitOfMeasure`) group — records are **only ever compared or aggregated
within an identical group**; there is no inferred UOM conversion
anywhere in this package. `WeightedAverageUnitPrice`/`MinUnitPrice`/
`MaxUnitPrice`/`MedianUnitPrice` and first-vs-last `UnitPriceChange`/
`UnitPriceChangePercent` are all computed per group.
`FlagUnitPriceIncrease` triggers against `Policy.UnitPriceIncreasePercent`
— worded as a factual observation, never "overcharging."

`scenarios_test.go`'s `TestScenario_MixedUOM_NeverAggregatedAcrossUnits`
locks that a supplier quoting the same `ProductID` in two different UOMs
(e.g. `EA` vs. `CASE`) always produces two separate `UnitPricePoint`
entries, never one blended figure.

## Cross-supplier price comparison

`Result.ProductPriceComparisons` compares `SupplierWeightedAveragePrice`
against `ProductMedianPrice`/`LowestObservedPrice` for the same explicit
`ProductID` + compatible `UnitOfMeasure`, populated only when 2+ distinct
suppliers have eligible data for that pair. Product identity is never
inferred from `Description`. This package computes no supplier-switching
recommendation of any kind — it reports the comparison figures only.

## Product spend and observed single source

`Result.ProductSummaries` (`ProductSpend`) reports per-product spend,
quantity, supplier count, largest-supplier share, and
`ObservedSingleSource` — which means **only** "exactly one supplier had
any spend for this ProductID in the supplied data." It is never evidence
that no alternate supplier could exist; `FlagObservedSingleSourceProduct`
is phrased accordingly.

## Price-volume decomposition

For a single, **homogeneous** (`SupplierID`, `ProductID`,
`UnitOfMeasure`) group with eligible quantity+price data in two adjacent
periods, `Result.PriceVolume` reports:

```
PriceEffect  = (P1 - P0) * Q0
VolumeEffect = (Q1 - Q0) * P0
Interaction  = (P1 - P0) * (Q1 - Q0)
```

Locked identity (`invariants_test.go`'s `TestInvariant_PriceVolume
Identity`, plus `TestInvariant_PriceVolume_PureIsolation` verifying a
pure-price-change fixture produces zero `VolumeEffect`/`Interaction` and
vice versa):

```
PriceEffect + VolumeEffect + Interaction = TotalSpendChange
```

Never applied across a heterogeneous product/supplier mix — each
decomposition is scoped to exactly one group.

## Recurrence

`Result.RecurringSpend.Mix` is the caller-declared `RecurrenceType`
(`RECURRING | ONE_TIME | IRREGULAR | UNKNOWN`) breakdown, built strictly
from `SpendRecord.Recurrence`. Separately, `Result.RecurringSpend.
ObservedRecurring` detects supplier/category groups this package's own
pattern-matching found to show comparable spend (within
`Policy.ObservedRecurringAmountTolerance`) across at least
`Policy.ObservedRecurringMinPeriods` distinct periods —
`FlagObservedRepeatedSpend`. The two lists are entirely independent:
pattern detection never overwrites a caller's own declaration, and a
caller-declared `RECURRING` record contributes to `Mix` regardless of
whether pattern detection also happens to find it comparable.

## Committed/discretionary mix

`Result.CommitmentMix` is the caller-declared `CommitmentType`
(`COMMITTED | DEFERRABLE | DISCRETIONARY | UNKNOWN`) breakdown, built
strictly from `SpendRecord.Commitment` — never inferred from `SpendType`.

## Tail spend

`Result.TailSpend` is available **only** when `Policy.TailSpend` defines
exactly one of `BelowAmount` (every supplier below an absolute dollar
threshold) or `OutsideTopN` (every supplier outside the top N by
`NetSpend`) — this package never reports tail spend without the
definition used, and `TailSpend.Definition`/`DefinitionAmount`/
`DefinitionTopN` always echo which was applied. If a caller sets both,
`BelowAmount` takes precedence and an advisory `IssueInvalidPolicy` is
raised naming the discarded field, rather than silently dropping
`OutsideTopN` with no signal.

## Duplicate-like spend

A duplicate `SpendID` is a structural input problem
(`IssueDuplicateSpend`) — a different concern from possible **economic**
duplicates, which `Result.DuplicateLikeGroups` detects via a normalized
`DuplicateSignature` (`SupplierID`, `Date`, `Amount`, `ProductID`,
`ReferenceID`, `Category`) within `Policy.DuplicateWindowDays` (default
3). Detection uses a bounded, bucketed/indexed approach — records are
grouped by `(SupplierID, rounded Amount, date-window bucket)`, and only
records already sharing a bucket (or an adjacent one, to catch pairs
straddling a bucket boundary) are ever compared pairwise — never a full
O(N²) pairwise scan; see [Benchmark safety](#benchmark-safety). No fraud
language: `FlagPossibleDuplicateSpend`'s message only ever says two
records "share a normalized signature," never that anything is wrong,
erroneous, or fraudulent (`neutral_language_test.go`).

Amount bucketing rounds to the nearest cent via `math.Round` (half away
from zero), correct for both positive amounts and the negative amounts a
`CREDIT`/`REFUND`/`REVERSAL` record's `Amount` can legitimately carry.

## Preferred/contracted supplier policy

`Supplier.PreferredSupplier`/`ContractedSupplier` are caller-supplied
policy flags, never inferred. `Result.NonPreferredSpend`/
`OutsideContractedSpend` are `Available` only when at least one supplier
in the analysis was explicitly marked preferred/contracted respectively
— otherwise "non-preferred" is meaningless (nothing was ever declared
preferred). `FlagSpendWithNonPreferredSupplier`/
`FlagSpendOutsideContractedSuppliers` trigger on **presence** of
non-preferred/non-contracted activity (at least one such supplier had
any records), not on a positive netted dollar total — a non-preferred
supplier whose activity nets to zero or negative (e.g. largely offset by
a credit) is still real activity that occurred and must not be silently
hidden by a sign-dependent threshold.

## AP integration

`SpendRecordsFromPayables` converts `accounting/ap.Payable`s into
`SpendRecord`s under `BasisAccrual`, using each `Payable`'s
`OriginalAmount` — **never** `OpenAmount`, which reflects today's
remaining balance, not the period's original economic spend. A
`DocumentTypeVendorCredit` `Payable` maps to `EffectCredit` with a
non-negative `Amount`, converting `accounting/ap`'s own "credit is a
negative `OriginalAmount`" convention into this package's "`Effect`
determines sign" convention.

Timing/accrual differences: a `Payable`'s `BillDate` is normally the same
economic event as a `SpendRecord`'s `Date` under `BasisAccrual`, but the
two packages diverge the moment a bill is partially paid, disputed, or
carries a vendor credit — `accounting/ap` tracks `OpenAmount` (what
remains owed today), this package tracks `Amount` (what was economically
purchased), and this package never derives one from the other. This
package never asserts `period spend == ending AP` — see
[AP vs. spend](#ap-vs-spend) and `ap_boundary_test.go`.

## Inventory integration

`SpendRecordsFromInventoryReceipts` converts
`accounting/inventory.Movement`s into `SpendRecord`s under
`BasisReceipt`, but **only** using an explicit caller-supplied
`map[MovementID]InventoryPurchaseReceipt` bridge (`SupplierID` +
authoritative purchase `Amount`). `accounting/inventory.Movement` carries
**no `SupplierID` field at all** — this adapter can never attribute a
Movement to a supplier on its own. A `Movement` absent from the supplied
bridge is skipped (never guessed); the function's second return value
(`ok`) is `false` whenever no receipts were supplied at all, so a caller
can distinguish "adapter ran and found nothing" from "adapter is
unavailable because no attribution was ever supplied" — see
`inventoryadapter_test.go`.

## Labor integration

`SpendRecordsFromContractorLabor` converts
`accounting/labor.ContractorLaborRecord`s into `SpendRecord`s under
`SpendTypeSubcontractor`, using an explicit caller-supplied
`map[ContractorID]SupplierID` mapping — a `ContractorID` absent from the
mapping is skipped, never guessed to equal its own `ContractorID`.
Double-counting prevention is the caller's responsibility: if the same
contractor spend already exists directly in `Input.SpendRecords` (e.g.
entered from AP data), the caller supplies exactly one of the two
sources, or assigns each a disjoint `SpendID` namespace and only calls
`Calculate` with the union it intends.

## Profitability integration

`FactsFromSpendRecords` converts `SpendRecord`s into
`accounting/profitability.Fact`s, but **only** for records present in an
explicit caller-supplied `map[SpendID]ProfitabilityClassification`
(`Component` + `Attributions`) — a `SpendRecord` absent from that map is
never converted, and `Component`/attribution is never inferred from
`SpendType`. This package never automatically routes all vendor spend
into profitability.

## Control reconciliation

`Result.ControlReconciliation` compares this package's own `NetSpend`
against up to five caller-supplied `ControlTotals` (`Purchases`,
`ExpenseSpend`, `CapexSpend`, `InventoryPurchases`, `ContractorSpend`),
each independently — `ComponentReconciliation` reports `VendorSpend`,
`ControlAmount`, `Difference` (`VendorSpend - ControlAmount`),
`Tolerance`, and `Reconciled`. Every `ControlTotals` field is a
`*float64` so "not supplied" (`nil`) is distinct from "supplied as
zero." A non-finite (NaN/Inf) control total is never silently
reconciled against — that component's `ComponentReconciliation.
Available` stays `false` and `IssueInvalidControlTotal` is reported.
Offsetting differences remain visible: each component is reconciled
independently against the same overall `NetSpend`, never
summed/netted against another component before comparison — a control
over-stated by $1,000 and another under-stated by $1,000 both surface
their own `Difference` and their own `FlagControlTotalMismatch`, never
canceling each other out.

`Tolerance` is the greater of `Policy.ControlTolerance` and the resolved
`Policy.Materiality` threshold — a mismatch smaller than what the
analysis already considers immaterial should not by itself flag
`FlagControlTotalMismatch`.

## Coverage

`Result.CategoryProductCoverage` reports `CategorizedSpend`/
`UncategorizedSpend` and `ProductAttributedSpend`/`UnattributedProduct
Spend` on the same signed net-spend basis as every other dollar figure
in this package — `CategorizedSpend + UncategorizedSpend` always equals
the analysis's total `NetSpend` (locked invariant,
`invariants_test.go`'s `TestInvariant_CategorizedPlusUncategorizedEquals
Total`).

`CategoryCoveragePercent`/`ProductCoveragePercent` are **deliberately**
computed from a separate, unsigned-magnitude denominator (sum of
`|Amount|` grouped by category/product presence), not from
`CategorizedSpend`/`UncategorizedSpend` directly: a ratio built on signed
net dollars degenerates when an uncategorized credit largely offsets
categorized purchases — e.g. $100,000 categorized net spend against a
$99,999 uncategorized net credit gives a signed total of $1 and an
arithmetically correct but meaningless ~10,000,000% "coverage" figure,
even though the underlying transaction volume is almost entirely
categorized. The magnitude-basis percentage reports coverage as "share
of transactional dollar volume that is categorized," which stays
sensible regardless of credits/refunds mixed into either side
(`codereview_regression_test.go`'s
`TestRegression_CategoryCoverage_NotDistortedByOffsettingCredit`).

`Result.Coverage` (`MetadataCoverage`) separately reports factual,
row-count-based field-population ratios (`CategoryCoverage`,
`ProductCoverage`, `QuantityCoverage`, `UnitPriceCoverage`,
`DepartmentCoverage`, `LocationCoverage`, `CostCenterCoverage`,
`RecurrenceCoverage`, `CommitmentCoverage`, `ControlCoverage`) — no
opaque composite score anywhere.

## Materiality

`Policy.Materiality` (`AbsoluteAmount` + `PercentOfTotalSpend`) is
explicitly never called "audit materiality" — it is a caller-adjustable
analysis threshold, not an attestation concept. `resolvedThreshold`
returns the greater of the two tests against a given total.

## Issues and flags

`Issue` and `Flag` are two independent, fully-defined taxonomies (only
codes this package actually emits are defined — no aspirational unused
codes). Both sort deterministically: `Issue`s by declared `IssueCode`
order then source ID (SpendID/SupplierID/Period, in that preference
order); `Flag`s by declared `FlagCode` order, then `SupplierID`, then
`ProductID`.

Every `Flag.Message` is a factual, rule-based observation — never
"overcharging," "bad vendor," "replace vendor," "negotiate harder,"
"risky," or "fraud" anywhere in this package
(`neutral_language_test.go`'s permanent regression coverage runs this
check across every named fixture scenario, not just one).

## Determinism and immutability

Every exported function is pure: no I/O, no mutation of caller-owned
input, no package-global mutable state. `Calculate` can be called
concurrently and repeatedly against identical input and always returns
byte-for-byte identical JSON — see `determinism_test.go`. Nothing in this
package calls `time.Now()`. `immutability_test.go` verifies `Calculate`
never mutates `Input`/`Options`, and that mutating a returned `Result`
never corrupts a subsequent call.

## Versions

- `SchemaVersion` — the `Supplier`/`Period`/`SpendRecord`/`Policy`/
  every `Result` sub-shape's JSON contract.
- `FormulaVersion` — the gross/credit/net bridge, every summary/
  concentration/growth/unit-price/price-volume/recurrence/tail-spend/
  duplicate-detection/reconciliation/coverage formula, and every
  flag-trigger rule.

Both are echoed on every `Result` and bumped independently, mirroring
this repository's [versioning-strategy](../README.md#versioning-strategy)
convention.

## Fixtures

`accounting/vendorspend/fixtures` provides ~25 named synthetic
scenarios (diversified/concentrated/increasing-concentration suppliers,
new/lost supplier, one-period insufficient history, price/volume/
combined price-volume changes, multi-supplier same product, observed
single source, declared/observed recurring spend, one-time spend,
committed/discretionary mix, category trend, uncategorized spend,
non-preferred supplier, tail spend, duplicate-like/near-duplicate spend,
vendor credit, mixed currency/UOM, zero-spend period) plus a
`LargePopulation` generator for benchmarking — see
[Benchmark safety](#benchmark-safety).

## Benchmark safety

Default `go test -bench=.` scale (20,000 records / 2,000 suppliers /
2,000 products / 24 periods) runs in well under a second on a
development laptop. The task's own recommended default sizing (100,000
records / 10,000 suppliers / 10,000 products / 60 periods) is exercised
by `BenchmarkCalculate_LargePopulation_FullScale`, which is **skipped by
default** and only runs when `VENDORSPEND_FULL_SCALE_BENCH=1` is set —
this repository's Prompt 46 development session drove a laptop to 34GB
RAM via stacked, unconfirmed-exited concurrent large benchmark runs, so
every large-scale benchmark here follows the same gate-by-default
convention `accounting/profitability`/`accounting/inventory` already
established. Before rerunning any large benchmark, confirm the previous
benchmark process has actually exited (e.g. via `ps`) — never launch
multiple heavy background runs concurrently.

Scaling linearity is verified at the default scale by two benchmarks:
`BenchmarkCalculate_ScalingLinearity` (the whole `Calculate` pipeline)
and `BenchmarkDuplicateDetection_ScalingLinearity` (isolating
possible-duplicate detection specifically, since a naive pairwise
implementation would be the most obvious O(N²) risk in this package). At
2,000 → 20,000 records (10x), observed scaling was ~7.3x-10.6x — linear,
not quadratic.

## Explicit non-goals

This package does **not** implement: procurement workflow, PO creation/
approval, vendor onboarding, payment execution, contract workflow,
RFQ/RFP, negotiation recommendations, vendor-replacement recommendations,
vendor credit scoring, sanctions/compliance screening, fraud
determination, market-price scraping, supplier-failure prediction,
purchasing optimization, automatic category classification, or AI
narrative. No database/persistence, HTTP/API, auth/users, frontend/UI, or
background jobs.

## Limitations

- One reporting currency per analysis (no FX conversion); a caller with
  mixed-currency data either pre-converts or accepts that only the most
  common (or explicitly chosen) currency's records are included in
  aggregate totals.
- New/lost-supplier detection, growth trends, price-volume decomposition,
  and observed-recurring pattern detection all require at least two
  chronological periods; with only one, those outputs are `Available ==
  false` rather than guessed.
- Cross-supplier price comparison and observed-single-source detection
  reflect only the supplier(s) present in the supplied data — neither is
  evidence about whether alternate sourcing exists in the real world.
- Tail spend requires an explicit `Policy.TailSpend` definition; there is
  no invented default cutoff.
- Duplicate-like detection is a normalized-signature match within a
  caller-configurable date window, not a guarantee of true duplication —
  it is a review signal, phrased neutrally, never a fraud or error
  determination.
