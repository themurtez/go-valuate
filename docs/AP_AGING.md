# Accounts payable aging (`accounting/ap`)

`accounting/ap` implements deterministic accounts-payable aging and
supplier payment analytics from a portable open-payables model. It is a
sibling of [`accounting/ar`](AR_AGING.md) — both are open-item
aging/analytics engines — but it is **independently usable** — it does not
require `accounting/ledger`, `accounting/statements`, or `accounting/ar`
input at all. A caller can populate it directly from a QuickBooks/Xero
export, a homegrown AP system, or a synthetic fixture (see
[`accounting/ap/fixtures`](../accounting/ap/fixtures/fixtures.go)).

```
accounting/ledger  ──▶  accounting/statements  ──▶  financial.FinancialDataset
                                                            ▲
accounting/ap  (standalone; open payables / vendor bill data) ── reconciles against, never depends on, the above
```

`accounting/ap` mirrors `accounting/ar`'s shape and conventions closely,
but is **not a generic generalization** of it — the two are independent
packages with independent domain models (`Payable` vs. `Receivable`,
supplier vs. customer, DPO vs. DSO, a forward-looking due-date payment
schedule and payment-pressure analysis vs. collections), sharing only
ideas and naming discipline, never code.

Like every other package in this repository, it contains **no**
persistence, HTTP/API, auth, UI, background jobs, QuickBooks/Xero
integration, payment execution, bank integration, AI/LLM, or tax logic —
see [Explicit non-goals](#explicit-non-goals) below and the main README's
[What this project intentionally does not
contain](../README.md#what-this-project-intentionally-does-not-contain).
Every exported function is pure (no I/O, no mutation of caller-owned
input, no package-global mutable state) and deterministic — see
[`safety_test.go`](../accounting/ap/safety_test.go), which covers
no-mutation, determinism, and concurrent-call safety together.

## Input model

`Payable` is the portable open-item / bill-level record:

```go
type Payable struct {
    ID, SupplierID, SupplierName, BillNumber string
    DocumentType                             DocumentType // BILL or VENDOR_CREDIT
    BillDate, DueDate                        time.Time
    OriginalAmount, OpenAmount               float64
    Currency                                 string
    Status                                    PayableStatus
    TermsDays                                 int
    PaidDate                                  *time.Time
    Dimensions                                []Dimension
    SourceRef                                 SourceRef
}
```

`SupplierID` is the identity key; `SupplierName` is optional display-only
metadata — a caller with only opaque IDs (no names at all) gets a fully
functional analysis, mirroring `accounting/ar.Receivable.CustomerID`'s
identical convention.

## Status model

Five stable statuses: `OPEN`, `PARTIALLY_PAID`, `PAID`, `VOIDED`,
`DISPUTED`. By default only `OPEN`, `PARTIALLY_PAID`, and `DISPUTED` are
economically relevant and enter aging (`PAID`/`VOIDED` are excluded). A
caller can override this via `Options.IncludeStatuses`. Status is never
inferred from `OpenAmount` — a `PAID` payable with a nonzero `OpenAmount`
is flagged (`IssuePaidWithOpenBalance`), not silently corrected.

Unlike `accounting/ar`, there is no separate `WRITTEN_OFF` status: a
payable a business no longer intends to pay is simply `VOIDED` (the bill
is canceled/reversed, often via a vendor credit) — an AR-style write-off
is a lender-side credit-loss decision with no AP equivalent this
package's scope covers, so no additional status was added.

## Explicit AsOfDate

Every aging calculation takes an explicit `Options.AsOfDate`. Nothing in
this package calls `time.Now()` — this is essential for reproducibility
and for building historical snapshots. `Calculate` returns
`Result{Available: false}` with an error `Issue` if `AsOfDate` is the zero
`time.Time`.

## Aging basis

`Options.Basis` selects `AGING_BY_DUE_DATE` (default) or
`AGING_BY_BILL_DATE`. Due-date aging measures actual delinquency relative
to the contractual payment obligation; bill-date aging conflates long
payment terms with lateness, so due-date is the default. `DaysPastDue` is
always clamped to `>= 0` — a future-dated or not-yet-due item is
`CURRENT`, never negative (see [Future-dated
items](#future-dated-items)).

## Aging buckets

Default five-bucket schema (`DefaultBuckets()`):

| Code | Range |
|---|---|
| `CURRENT` | 0 days |
| `1_30` | 1-30 days |
| `31_60` | 31-60 days |
| `61_90` | 61-90 days |
| `91_PLUS` | 91+ days |

A caller can supply a custom `[]BucketDefinition` via `Options.Buckets`.
`validateBucketDefinitions` enforces: no overlap, exactly one terminal
open-ended bucket (the last by `MinDaysPastDue`), unique codes, and no
gaps unless `Options.AllowBucketGaps` is set. `Code` is a stable
identifier, not a UI label — `Label` is a separate, purely cosmetic field.

## Open balance semantics / partial payments

Aging amount is always `OpenAmount`, never `OriginalAmount`. For an
ordinary bill, `0 <= OpenAmount <= OriginalAmount` is expected;
`OpenAmount > OriginalAmount` is flagged (`IssueOpenExceedsOriginal`) but
not silently corrected — this package reports problems rather than
repairing financial records. A partially paid bill (`Status ==
PARTIALLY_PAID`) simply carries an `OpenAmount` less than
`OriginalAmount` and ages on that lower figure like any other open bill.

## Vendor credits

`DocumentType` distinguishes `BILL` from `VENDOR_CREDIT`. A vendor
credit's negative `OpenAmount` is never treated as an invalid bill
balance. Default behavior (`CreditNoNetting`) reports vendor-credit
exposure separately via `PortfolioSummary.VendorCredits`/
`SupplierSummary.VendorCreditAmount` and never nets it against unrelated
bills — gross bills and gross vendor credits are always both exposed.
`Options.CreditNetting = CreditNetBySupplier` opts into netting a
supplier's vendor-credit balance against that same supplier's open bills
for aggregate totals; `VendorCreditSummary` always reports the gross
credit balance regardless of netting policy, for audit purposes.

## Validation

`Calculate` validates (and returns structured `Issue`s for, never
silently repairs): duplicate payable IDs (first occurrence wins),
unrecognized status, non-finite amounts, `OpenAmount` exceeding
`OriginalAmount`, `PAID` with nonzero `OpenAmount`, missing/invalid dates,
due-before-bill date, future bill dates, missing supplier ID, missing
currency, invalid bucket configuration, and mixed currency. See [Issue
taxonomy](#issue-taxonomy).

## Portfolio aging summary

`Result.PortfolioSummary` reports: total gross/open payables, current
amount, one `BucketAmount` per bucket (amount + percent-of-total, never
NaN/Inf — see [Availability convention](#availability-convention)),
overdue total/percent, weighted-average days past due, oldest open bill
age, open/overdue bill counts, supplier count, disputed amount, and
`VendorCreditSummary`.

## Supplier-level aging

`Result.SupplierSummaries` is one `SupplierSummary` per supplier: open
amount, per-bucket breakdown, overdue total/percent, weighted-average
days past due, oldest bill age, bill counts, disputed amount, vendor
credit amount, and a per-supplier `MaterialityAssessment` (see
[Materiality](#materiality)). Sorted deterministically: `OpenAmount`
descending, then `SupplierID` ascending.

## DPO

`Result.DPO` computes simple Days Payable Outstanding — `(Ending AP /
Period Denominator) x Days` — only when `Input.PurchasesHistory` is
supplied (never fabricated from bill data). Each `PayablesPeriod` carries
an explicit `Basis` (`PURCHASES`/`COGS`/`OTHER_EXPLICIT`), echoed on the
result so a caller can see whether the figure is the textbook-correct
purchases-basis DPO or the more commonly available COGS-proxy
approximation — this package never implies purchases were known when only
COGS was supplied. `Result.DPOHistory` computes one DPO point per
`PayablesPeriod` that supplies its own `EndingAP` — this package never
derives historical AP from today's open-item list; historical snapshot
data must be caller-supplied.

## Historical aging trend and migration

`Result.AgingTrend` ages each `Input.Snapshots` entry independently (same
bucket schema/basis as the current analysis) into a portfolio-level
overdue/60+/90+ time series. `Result.Migration` compares the most recent
snapshot to the current payables list, matching by `Payable.ID` across
states, and reports from-bucket → to-bucket transition counts/amounts
(including a `ToPaid` transition when a payable is no longer open), plus
`CureAmount`/`DeteriorationAmount` and
`SuppliersImproving`/`SuppliersDeteriorating` counts. Migration is never
inferred from a single snapshot — it requires the same payable ID to
appear in both states.

## Payment terms and history

`Result.TermsAnalysis` reports average and amount-weighted contractual
`TermsDays` across currently outstanding (aging-included) payables, plus
actual `PaymentTiming` (average days to pay, average days beyond terms,
percent paid on-or-before due date, percent paid late) when
`Input.Payments` is supplied — distinguishing long contractual terms from
genuinely chronic lateness. `Result.PaymentMetrics` (available only when
snapshot/payment data exists) reports amount paid, overdue/60+/90+ trend
direction (`improving`/`deteriorating`/`flat`, from `AgingTrend`'s
first-vs-last comparison), average days to pay, and suppliers
improving/deteriorating (from `Migration`).

`TermsAnalysis.AverageTermsDays`/`WeightedAverageTermsDays` are scoped to
currently outstanding (aging-included) payables — a fully `PAID` bill
does not contribute to these averages even though its `TermsDays` is
still a fact about the relationship; `PaymentTiming`, by contrast, is
driven by `Input.Payments` directly and does include payments against
already-fully-paid bills.

## AP concentration

`Result.Concentration.TotalAP` reuses `analytics/concentration.Calculate`
directly (each supplier's total `OpenAmount` adapted into
`concentration.Observation` rows) rather than reimplementing HHI/top-N
share math. `Overdue`/`Overdue60Plus`/`Overdue90Plus` are this package's
own top-N-by-balance rankings — often more operationally useful than
total AP concentration for cash-management prioritization. Supplier IDs,
never names, are the concentration identity key.

**Important semantic rule**: a large AP balance with one supplier does
**not** by itself mean this business operationally depends on that
supplier — it may simply reflect large recent purchase volume, long
negotiated terms, or payment timing. `ConcentrationSummary.Label`
explicitly says so: this is **AP/payment exposure concentration**, not
vendor dependency. True operational supplier dependency (single-sourced
parts, switching cost, lead-time risk) belongs in the separate
[Vendor Spend Analytics module](VENDOR_SPEND_ANALYTICS.md)
(`accounting/vendorspend`), which computes its own supplier spend
concentration from period purchase volume, not from AP balances.

## Due-date schedule

`Result.DueSchedule` groups currently open, non-credit AP into
forward-looking horizon buckets by due date: past due, next 7 days, 8-14
days, 15-30 days, 31-60 days, 61-90 days, 90+ future
(`DefaultDueScheduleHorizons()`), or a caller-supplied
`[]DueScheduleHorizon` via `Options.DueScheduleHorizons`. This is
explicitly **not a forecast** — `DueSchedule.Label` says so — it contains
no projection, no assumption about future purchasing, and no probability
of payment; it is only a grouping of bills that already exist as of
`AsOfDate`.

## Payment pressure

`Result.PaymentPressure` is available only when `Options.PaymentPressure`
supplies at least one of `CashAvailable`/`ExpectedNearTermInflows` — this
package never fabricates a liquidity figure. When supplied, it computes,
for fixed 7/14/30-day windows: `APDue` (cumulative open AP due within the
window), `NearTermCoverage` (liquidity / `APDue`), and `Shortfall`
(`APDue` - liquidity). `Options.ExcludeDisputedFromPaymentPressure` opts
into excluding disputed bills from these `APDue` figures specifically —
disputed bills are **never** excluded from ordinary aging itself, only
(optionally) from this liquidity-pressure view. This is deliberately
separate from any multi-week cash forecast — see [Explicit
non-goals](#explicit-non-goals); the 13-week cash forecast is a later,
separate module (Prompt 43).

## Disputed payables

Disputed bills (`Status == DISPUTED`) age normally by default and are
never automatically excluded from `PortfolioSummary`, bucket totals, or
`SupplierSummaries` — `PortfolioSummary.DisputedAmount` and
`SupplierSummary.DisputedAmount` surface the disputed exposure alongside
ordinary aging. The only place disputed bills can be excluded is
`PaymentPressure`, and only when a caller explicitly opts in via
`Options.ExcludeDisputedFromPaymentPressure`.

## Flags and materiality

`Result.Flags` are deterministic, threshold-based signals (never a hidden
supplier-risk or default-probability model): `HIGH_OVERDUE_PERCENT`,
`LARGE_60_PLUS_BALANCE`, `LARGE_90_PLUS_BALANCE`, `HIGH_AP_CONCENTRATION`,
`HIGH_OVERDUE_CONCENTRATION`, `DPO_DETERIORATION`,
`AGING_DETERIORATION`, `REPEATED_LATE_PAYMENT`, `LARGE_DISPUTED_BALANCE`,
`NEAR_TERM_PAYMENT_PRESSURE`, `CONTROL_ACCOUNT_MISMATCH`,
`VENDOR_CREDIT_REVIEW`. Every threshold is caller-configurable via
`Options.Thresholds`; `DefaultThresholds()` documents the baseline.

Per-supplier flags (`HIGH_OVERDUE_PERCENT`, `LARGE_60_PLUS_BALANCE`,
etc.) are only ever evaluated for suppliers appearing in
`SupplierSummaries` — i.e. suppliers with at least one currently
outstanding (aging-included) balance. A supplier whose entire bill
history is already fully paid off never triggers a per-supplier flag,
even if that history included late payments; there is no longer an
outstanding balance to flag.

`MaterialityAssessment` (on each `SupplierSummary.Materiality`) compares
a supplier's `OverdueTotal` against `Options.MaterialityThreshold`
(absolute dollars) and/or `Options.MaterialityPercentOfAP` (default 5%)
— either rule alone is sufficient to mark an item material. This package
imposes no fixed accounting-materiality standard.

## GL control-account reconciliation

`Result.AgingReconciliation` is an always-computed invariant: sum of
bucket amounts vs. `PortfolioSummary.TotalOpenPayables`
(`IssueUnbalancedAging` if they diverge — should never occur for valid
input). `Result.ControlAccountReconciliation` is an optional
subledger-vs-GL check, available only when `Options.ControlAccountBalance`
is supplied: `SubledgerBalance`, `ControlAccountBalance`, `Difference`,
`Tolerance` (default one cent), `Reconciled`. Neither side is ever
adjusted — this package reports the reconciliation, never forces balance.

## Integration invariants (not dependencies)

`accounting/ap` never imports `accounting/ledger`, `accounting/statements`,
`accounting/ar`, `analytics/workingcapital`, or `analytics/ratios`. Tests
instead prove **invariants** against those packages' own output, using
consistent synthetic fixture data:

- [`integration_test.go`](../accounting/ap/integration_test.go) — a
  statement-builder-derived `CodeBsAccountsPayable` balance equals this
  package's `TotalOpenPayables` for the matching open-item list.
- [`workingcapital_adapter_test.go`](../accounting/ap/workingcapital_adapter_test.go) —
  `TotalOpenPayables`, fed into a `FinancialDataset` as
  `CodeBsAccountsPayable`, reconciles into `analytics/workingcapital`'s
  `OperatingCurrentLiabilities` under `DefaultInclusionPolicy` (which
  includes AP by default).
- [`ratios_adapter_test.go`](../accounting/ap/ratios_adapter_test.go) —
  this package's own DPO matches `analytics/ratios`' `(AP/Total
  COGS)*365` DPO when both are given equivalent AP/COGS figures, the same
  365-day period, **and** this package's own COGS basis is selected
  (`analytics/ratios` has no purchases-basis DPO at all, so that
  combination is a deliberate, documented difference rather than
  something forced into equality — see
  `TestAdapter_DPODiffersFromRatios_OnPurchasesBasis`).

`accounting/ap`'s `TotalOpenPayables` can also feed
`analytics/workingcapital`'s operating current liabilities side directly
(as shown above) when a caller's own `InclusionPolicy` definitions match.

## Availability convention

`AmountValue{Available bool; Value float64}` distinguishes "computed to be
exactly 0" from "cannot be computed because a required input is absent" —
the same convention `analytics/concentration.ConcentrationValue`,
`financial/metrics.MetricValue`, and `accounting/ar.AmountValue` use. A
percentage against a zero denominator is always `Unavailable`, never
`NaN`/`Inf`. `encoding/json` itself refuses to marshal a `NaN`/`Inf`
float64, but this package's own computations never construct one in the
first place.

This package distinguishes five distinct "not a number" states across its
surface, never conflating any of them: a **known zero** (`AmountValue{Available:
true, Value: 0}`, e.g. zero disputed exposure), **not supplied** (a
caller-optional input like `Options.PaymentPressure` was left nil — the
whole sub-result is `{Available: false}`), **not applicable** (e.g.
`DPOHistory.Trend` is empty when fewer than two history points exist —
there is no "first vs. last" to compare), **invalid** (an `Issue` is
raised and the offending row is excluded), and **unavailable** (a
percentage whose denominator is zero). Zero never silently stands in for
missing.

## Issue taxonomy

| Code | Meaning |
|---|---|
| `DUPLICATE_PAYABLE` | Same `Payable.ID` more than once; only the first occurrence is used |
| `INVALID_STATUS` | Unrecognized `PayableStatus` or missing ID |
| `INVALID_DOCUMENT_TYPE` | Reserved; not currently emitted (see doc comment) |
| `INVALID_AMOUNT` | Sign inconsistent with `DocumentType` |
| `NON_FINITE_AMOUNT` | NaN/Inf monetary field; row excluded |
| `OPEN_EXCEEDS_ORIGINAL` | `abs(OpenAmount) > abs(OriginalAmount)` |
| `PAID_WITH_OPEN_BALANCE` | `Status == PAID` but `OpenAmount != 0` |
| `INVALID_DATE` | Missing/zero bill or due date, or due before bill date, or missing `AsOfDate` |
| `FUTURE_BILL` | `BillDate` after `AsOfDate` |
| `INVALID_BUCKET_CONFIGURATION` | Malformed `BucketDefinition` slice |
| `MIXED_CURRENCY` | More than one currency among included payables |
| `MISSING_SUPPLIER` | Empty `SupplierID` |
| `CONTROL_ACCOUNT_MISMATCH` | Subledger total differs from supplied GL balance beyond tolerance |
| `UNBALANCED_AGING` | Bucket sum != total open payables (should not occur for valid input) |
| `MISSING_DENOMINATOR_FOR_DPO` | No `PurchasesHistory` supplied; DPO unavailable |
| `INVALID_PAYMENT` | Non-finite/non-positive `SupplierPayment.Amount` or zero `Date` |
| `UNKNOWN_PAYABLE_PAYMENT` | `SupplierPayment.PayableID` matches no known `Payable.ID` |

`INVALID_DOCUMENT_TYPE` is defined but not currently emitted: an
unrecognized `DocumentType` resolves silently to `DocumentTypeBill` (the
safe default for pre-existing bill data — see `resolvedDocumentType`),
matching `accounting/ar`'s identical `DocumentType`-defaulting
convention. The code is reserved so a future stricter check does not
overload `INVALID_AMOUNT`'s meaning.

## Future-dated items

If `BillDate` is after `AsOfDate`, `FUTURE_BILL` is flagged (warning).
`DaysPastDue` is always clamped to `>= 0` regardless of basis — a
future-dated or not-yet-due item is classified `CURRENT`, never a
negative "days past due."

## Currency

V1 supports one reporting currency per analysis. `Options.ReportingCurrency`
selects it explicitly; if unset, the most common currency among included
payables is used (ties broken alphabetically) and `IssueMixedCurrency` is
flagged (an issue, not a hard rejection — `Result.Available` stays
`true`). This package never fetches FX and never silently sums mixed
currencies together; a caller who has already converted to one reporting
currency upstream can supply pre-converted amounts directly.

## Dimensions

`Options.Dimension` selects one `Dimension.Key` (e.g. `location`,
`department`, `business_unit`, `supplier_category`, `region`) to produce
an optional `Result.Dimension` breakdown (`DimensionSummary`): open
amount, overdue total, and bill count per distinct dimension value,
sorted by open amount descending then value ascending. This is a single
caller-selected grouping, not a generic OLAP/cube engine.

## Versions

- `SchemaVersion` — the shape of `Payable`, `SupplierPayment`,
  `BucketDefinition`, `SupplierSummary`, `Result`, `Issue`, and every
  other serialized contract.
- `FormulaVersion` — aging-basis/bucket assignment, DPO, historical
  trend/migration, payment/terms metrics, concentration, due-date
  schedule, payment pressure, and every flag-trigger rule.

Both are echoed on every `Result` and bumped independently, mirroring
this repository's
[versioning-strategy](../README.md#versioning-strategy) convention and
`accounting/ar`'s identical two-version split.

## Explicit non-goals

This package does **not** implement: vendor payment execution, ACH/check
generation, approval workflow, procurement, purchase orders, vendor
onboarding, fraud detection, supplier credit scoring, bank reconciliation,
QuickBooks/Xero synchronization, future purchase forecasting, or a full
13-week cash forecast (that is `accounting/cashforecast`'s separate
module). True operational vendor-dependency/spend analytics (beyond
simple AP-balance concentration) is
[`accounting/vendorspend`'s](VENDOR_SPEND_ANALYTICS.md) separate module.

## Limitations

- One reporting currency per analysis (no FX conversion).
- Migration compares only the most recent snapshot against the current
  state; a caller wanting every adjacent pair across a longer snapshot
  sequence calls this package repeatedly across their own sequence.
- DPO/DPOHistory require caller-supplied purchases or COGS data; this
  package never fabricates a denominator from bill totals.
- `DueSchedule` and `PaymentPressure` reflect only bills that already
  exist as of `AsOfDate` — neither is a forecast of future purchasing or
  future liquidity.
- Per-supplier flags only fire for suppliers with a currently outstanding
  balance (see [Flags and materiality](#flags-and-materiality)).
