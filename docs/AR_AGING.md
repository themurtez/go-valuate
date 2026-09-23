# Accounts receivable aging (`accounting/ar`)

`accounting/ar` implements deterministic accounts-receivable aging and
collections analytics from a portable open-receivables model. It is the
next operational-accounting module after
[`accounting/ledger`](LEDGER.md) and [`accounting/statements`](STATEMENT_BUILDER.md),
but it is **independently usable** — it does not require `accounting/ledger`
input at all. A caller can populate it directly from a QuickBooks/Xero
export, a homegrown billing system, or a synthetic fixture (see
[`accounting/ar/fixtures`](../accounting/ar/fixtures/fixtures.go)).

```
accounting/ledger  ──▶  accounting/statements  ──▶  financial.FinancialDataset
                                                            ▲
accounting/ar  (standalone; open receivables / invoice data) ── reconciles against, never depends on, the above
```

Like every other package in this repository, it contains **no**
persistence, HTTP/API, auth, UI, background jobs, QuickBooks/Xero
integration, email/collections automation, AI/LLM, tax logic, or
credit-bureau logic — see [Explicit non-goals](#explicit-non-goals) below
and the main README's [What this project intentionally does not
contain](../README.md#what-this-project-intentionally-does-not-contain).
Every exported function is pure (no I/O, no mutation of caller-owned
input, no package-global mutable state) and deterministic — see
[`safety_test.go`](../accounting/ar/safety_test.go), which covers
no-mutation, determinism, and concurrent-call safety together.

## Input model

`Receivable` is the portable open-item / invoice-level record:

```go
type Receivable struct {
    ID, CustomerID, CustomerName, InvoiceNumber string
    DocumentType                                DocumentType // INVOICE or CREDIT_MEMO
    InvoiceDate, DueDate                        time.Time
    OriginalAmount, OpenAmount                  float64
    Currency                                    string
    Status                                       ReceivableStatus
    TermsDays                                    int
    PaidDate                                     *time.Time
    Dimensions                                   []Dimension
    SourceRef                                    SourceRef
}
```

`CustomerID` is the identity key; `CustomerName` is optional display-only
metadata — a caller with only opaque IDs (no names at all) gets a fully
functional analysis, mirroring `analytics/concentration.Observation`'s
identical convention.

## Status model

Six stable statuses: `OPEN`, `PARTIALLY_PAID`, `PAID`, `VOIDED`,
`WRITTEN_OFF`, `DISPUTED`. By default only `OPEN`, `PARTIALLY_PAID`, and
`DISPUTED` are economically relevant and enter aging (`WRITTEN_OFF` is
tracked separately via `PortfolioSummary.WrittenOffAmount` and
`WriteOffSummary`; `PAID`/`VOIDED` are excluded). A caller can override
this via `Options.IncludeStatuses`. Status is never inferred from
`OpenAmount` — a `PAID` receivable with a nonzero `OpenAmount` is flagged
(`IssuePaidWithOpenBalance`), not silently corrected.

## Explicit AsOfDate

Every aging calculation takes an explicit `Options.AsOfDate`. Nothing in
this package calls `time.Now()` — this is essential for reproducibility
and for building historical snapshots. `Calculate` returns
`Result{Available: false}` with an error `Issue` if `AsOfDate` is the zero
`time.Time`.

## Aging basis

`Options.Basis` selects `AGING_BY_DUE_DATE` (default) or
`AGING_BY_INVOICE_DATE`. Due-date aging measures actual delinquency
relative to the contractual payment obligation; invoice-date aging
conflates long payment terms with lateness, so due-date is the default.
`DaysPastDue` is always clamped to `>= 0` — a future-dated or not-yet-due
item is `CURRENT`, never negative (see
[Future-dated items](#future-dated-items)).

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
open-ended bucket (the last by `MinDaysPastDue`), unique codes, and no gaps
unless `Options.AllowBucketGaps` is set. `Code` is a stable identifier, not
a UI label — `Label` is a separate, purely cosmetic field.

## Open balance semantics

Aging amount is always `OpenAmount`, never `OriginalAmount`. `OpenAmount >
OriginalAmount` (in magnitude, for an ordinary invoice) is flagged
(`IssueOpenExceedsOriginal`) but not silently corrected — this package
reports problems rather than repairing financial records.

## Credit memos

`DocumentType` distinguishes `INVOICE` from `CREDIT_MEMO`. A credit memo's
negative `OpenAmount` is never treated as an invalid invoice balance.
Default behavior (`CreditNoNetting`) reports credit exposure separately
via `PortfolioSummary.Credits`/`CustomerSummary.CreditAmount` and never
nets it against unrelated invoices. `Options.CreditNetting =
CreditNetByCustomer` opts into netting a customer's credit balance against
that same customer's open invoices for aggregate totals — `CreditSummary`
always reports the gross credit balance regardless of netting policy, for
audit purposes.

## Validation

`Calculate` validates (and returns structured `Issue`s for, never silently
repairs): duplicate receivable IDs (first occurrence wins), unrecognized
status, non-finite amounts, `OpenAmount` exceeding `OriginalAmount`, `PAID`
with nonzero `OpenAmount`, missing/invalid dates, due-before-invoice date,
future invoice dates, missing customer ID, missing currency, invalid
bucket configuration, and mixed currency. See [Issue
taxonomy](#issue-taxonomy).

## Portfolio aging summary

`Result.PortfolioSummary` reports: total gross/open receivables, current
amount, one `BucketAmount` per bucket (amount + percent-of-total, never
NaN/Inf — see [Availability convention](#availability-convention)),
overdue total/percent, weighted-average days past due, oldest open
receivable age, open/overdue invoice counts, customer count, disputed
amount, written-off amount, and `CreditSummary`.

## Customer-level aging

`Result.CustomerSummaries` is one `CustomerSummary` per customer: open
amount, per-bucket breakdown, overdue total/percent, weighted-average days
past due, oldest invoice age, invoice counts, disputed amount, credit
amount, and a per-customer `MaterialityAssessment` (see
[Materiality](#materiality)). Sorted deterministically: `OpenAmount`
descending, then `CustomerID` ascending.

## DSO

`Result.DSO` computes simple DSO — `(Ending AR / Period Sales) x Days` —
only when `Input.SalesHistory` is supplied (never fabricated from invoice
data). Each `SalesPeriod` carries an explicit `Basis`
(`CREDIT_SALES`/`TOTAL_SALES`), echoed on the result so a caller can see
whether the figure is the textbook-correct credit-sales DSO or a weaker
total-sales approximation. `Result.DSOHistory` computes one DSO point per
`SalesPeriod` that supplies its own `EndingAR` — this package never
derives historical AR from today's open-item list; historical snapshot
data must be caller-supplied.

## Historical aging trend and migration

`Result.AgingTrend` ages each `Input.Snapshots` entry independently (same
bucket schema/basis as the current analysis) into a portfolio-level
overdue/60+/90+ time series. `Result.Migration` compares the most recent
snapshot to the current receivables list, matching by `Receivable.ID`
across states, and reports from-bucket → to-bucket transition counts/
amounts (including a `ToPaid` transition when a receivable is no longer
open), plus `CureAmount`/`DeteriorationAmount` and
`CustomersImproving`/`CustomersDeteriorating` counts. Migration is never
inferred from a single snapshot — it requires the same receivable ID to
appear in both states.

## Collection metrics

`Result.CollectionMetrics` (available only when snapshot/payment data
exists) reports amount collected, overdue/60+/90+ trend direction
(`improving`/`deteriorating`/`flat`, from `AgingTrend`'s first-vs-last
comparison), average days to pay, and customers improving/deteriorating
(from `Migration`). `Result.TermsAnalysis` reports average and
amount-weighted contractual `TermsDays`, plus actual `PaymentTiming`
(average days to pay, average days beyond terms) when `Input.Payments` is
supplied — distinguishing long contractual terms from genuinely late
collections.

## Concentration

`Result.Concentration.TotalAR` reuses `analytics/concentration.Calculate`
directly (each customer's total `OpenAmount` adapted into
`concentration.Observation` rows) rather than reimplementing HHI/top-N
share math. `Overdue`/`Overdue60Plus`/`Overdue90Plus` are this package's
own top-N-by-balance rankings — often more operationally useful than total
AR concentration for collections prioritization. Customer IDs, never
names, are the concentration identity key.

## Flags and materiality

`Result.Flags` are deterministic, threshold-based signals (never a hidden
credit-risk model): `HIGH_OVERDUE_PERCENT`, `LARGE_60_PLUS_BALANCE`,
`LARGE_90_PLUS_BALANCE`, `HIGH_AR_CONCENTRATION`,
`CUSTOMER_OVERDUE_CONCENTRATION`, `DSO_DETERIORATION`,
`AGING_DETERIORATION`, `REPEATED_LATE_PAYMENT`, `LARGE_DISPUTED_BALANCE`,
`CREDIT_BALANCE_REVIEW`. Every threshold is caller-configurable via
`Options.Thresholds`; `DefaultThresholds()` documents the baseline.

`MaterialityAssessment` (on each `CustomerSummary.Materiality`) compares a
customer's `OverdueTotal` against `Options.MaterialityThreshold` (absolute
dollars) and/or `Options.MaterialityPercentOfAR` (default 5%) — either
rule alone is sufficient to mark an item material. This package imposes no
fixed accounting-materiality standard.

## Collection priority

`Result.CollectionPriority` is an explicit, formula-based **heuristic**
ranking — always labeled as such (`CollectionPriority.Label`), never a
"probability of default" or other statistical claim. Score components
(`PriorityFactor`: balance size, days past due, bucket severity, AR
concentration, dispute penalty) are always returned so a caller can see
exactly how the score was built. Weights are fully overridable via
`Options.PriorityWeights`.

## GL control-account reconciliation

`Result.AgingReconciliation` is an always-computed invariant: sum of
bucket amounts vs. `PortfolioSummary.TotalOpenReceivables`
(`IssueUnbalancedAging` if they diverge — should never occur for valid
input). `Result.ControlAccountReconciliation` is an optional
subledger-vs-GL check, available only when `Options.ControlAccountBalance`
is supplied: `SubledgerBalance`, `ControlAccountBalance`, `Difference`,
`Tolerance` (default one cent), `Reconciled`. Neither side is ever
adjusted — this package reports the reconciliation, never forces balance.

## Integration invariants (not dependencies)

`accounting/ar` never imports `accounting/ledger`, `accounting/statements`,
`analytics/workingcapital`, or `analytics/ratios`. Tests instead prove
**invariants** against those packages' own output, using consistent
synthetic fixture data:

- [`integration_test.go`](../accounting/ar/integration_test.go) — a
  statement-builder-derived `CodeBsAccountsReceivable` balance equals this
  package's `TotalOpenReceivables` for the matching open-item list.
- [`workingcapital_adapter_test.go`](../accounting/ar/workingcapital_adapter_test.go) —
  `TotalOpenReceivables`, fed into a `FinancialDataset` as
  `CodeBsAccountsReceivable`, reconciles into
  `analytics/workingcapital`'s `OperatingCurrentAssets` under
  `DefaultInclusionPolicy` (which includes AR by default).
- [`ratios_adapter_test.go`](../accounting/ar/ratios_adapter_test.go) —
  this package's own DSO matches `analytics/ratios`' `(AR/Revenue)*365`
  DSO when both are given equivalent AR/revenue figures and the same
  365-day period.

## Availability convention

`AmountValue{Available bool; Value float64}` distinguishes "computed to be
exactly 0" from "cannot be computed because a required input is absent" —
the same convention `analytics/concentration.ConcentrationValue` and
`financial/metrics.MetricValue` use. A percentage against a zero
denominator is always `Unavailable`, never `NaN`/`Inf`. `encoding/json`
itself refuses to marshal a `NaN`/`Inf` float64, but this package's own
computations never construct one in the first place.

## Issue taxonomy

| Code | Meaning |
|---|---|
| `DUPLICATE_RECEIVABLE` | Same `Receivable.ID` more than once; only the first occurrence is used |
| `INVALID_STATUS` | Unrecognized `ReceivableStatus` or missing ID |
| `INVALID_AMOUNT` | Sign inconsistent with `DocumentType` |
| `NON_FINITE_AMOUNT` | NaN/Inf monetary field; row excluded |
| `OPEN_EXCEEDS_ORIGINAL` | `abs(OpenAmount) > abs(OriginalAmount)` |
| `PAID_WITH_OPEN_BALANCE` | `Status == PAID` but `OpenAmount != 0` |
| `INVALID_DATE` | Missing/zero invoice or due date, or due before invoice |
| `FUTURE_INVOICE` | `InvoiceDate` after `AsOfDate` |
| `INVALID_BUCKET_CONFIGURATION` | Malformed `BucketDefinition` slice |
| `MIXED_CURRENCY` | More than one currency among included receivables |
| `MISSING_CUSTOMER` | Empty `CustomerID` |
| `CONTROL_ACCOUNT_MISMATCH` | Subledger total differs from supplied GL balance beyond tolerance |
| `UNBALANCED_AGING` | Bucket sum != total open receivables (should not occur for valid input) |
| `MISSING_SALES_FOR_DSO` | No `SalesHistory` supplied; DSO unavailable |
| `INVALID_PAYMENT` | Non-finite/non-positive `Payment.Amount` or zero `Date` |
| `UNKNOWN_RECEIVABLE_PAYMENT` | `Payment.ReceivableID` matches no known `Receivable.ID` |

## Future-dated items

If `InvoiceDate` is after `AsOfDate`, `FUTURE_INVOICE` is flagged
(warning). `DaysPastDue` is always clamped to `>= 0` regardless of basis —
a future-dated or not-yet-due item is classified `CURRENT`, never a
negative "days past due."

## Currency

V1 supports one reporting currency per analysis. `Options.ReportingCurrency`
selects it explicitly; if unset, the most common currency among included
receivables is used (ties broken alphabetically) and `IssueMixedCurrency`
is flagged. This package never fetches FX and never silently sums mixed
currencies together.

## Versions

- `SchemaVersion` — the shape of `Receivable`, `Payment`,
  `BucketDefinition`, `CustomerSummary`, `Result`, `Issue`, and every other
  serialized contract.
- `FormulaVersion` — aging-basis/bucket assignment, DSO, historical
  trend/migration, collection metrics, concentration, and every flag-
  trigger rule.

Both are echoed on every `Result` and bumped independently, mirroring this
repository's [versioning-strategy](../README.md#versioning-strategy)
convention.

## Explicit non-goals

This package does **not** implement: collections workflow, reminder
emails, payment processing, credit scoring, credit-bureau integrations,
bad-debt prediction ML, AR invoice creation, a payment-application engine,
QuickBooks/Xero synchronization, bank reconciliation, or cash forecasting
(cash forecast is a later, separate module).

## Limitations

- One reporting currency per analysis (no FX conversion).
- `CollectionPriority` is an explicit heuristic, not a statistical model —
  it makes no probability-of-default claim.
- Migration compares only the most recent snapshot against the current
  state; a caller wanting every adjacent pair across a longer snapshot
  sequence calls this package repeatedly across their own sequence.
- DSO/DSOHistory require caller-supplied sales data; this package never
  fabricates sales from invoice totals.
