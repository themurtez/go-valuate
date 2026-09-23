# 13-week cash forecast (`accounting/cashforecast`)

`accounting/cashforecast` provides a deterministic rolling cash forecast
for short-term liquidity management — 13 weeks by default, configurable
from 1 to 52. It answers seven questions: what cash is expected in each
week, what cash is expected out, what the weekly ending cash balance is,
when cash falls below a caller's minimum, what funding gap exists, which
scheduled receipts/payments drive the result, and how explicit downside/
upside timing assumptions change the picture.

Like every other package in this repository, it contains **no**
persistence, HTTP/API, auth, UI, background jobs, bank API integration,
QuickBooks/Xero integration, payment execution, invoice generation,
AI/LLM, or tax logic — see [What this project intentionally does not
contain](../README.md#what-this-project-intentionally-does-not-contain)
in the main README, and [Explicit non-goals](#explicit-non-goals) below
for the fuller list specific to this package. Every exported function is
pure (no I/O, no mutation of caller-owned input, no package-global
mutable state, no wall-clock reads) and deterministic — see
[`determinism_test.go`](../accounting/cashforecast/determinism_test.go)
and
[`immutability_test.go`](../accounting/cashforecast/immutability_test.go).

## Distinct from `analytics/forecast`

This package is not a smaller version of, and does not depend on or
import, `analytics/forecast`. `analytics/forecast` projects financial
statements (revenue, COGS, opex, EBITDA, ...) from caller growth/margin
assumptions over months or years — a P&L/valuation-input forecasting
engine. `cashforecast` instead buckets caller-supplied cash events (known
bills, scheduled receipts, payroll, taxes, debt service, ...) into
calendar weeks and rolls forward an explicit opening cash balance. It
answers a treasury question ("do we have enough cash the next 13 weeks"),
not a P&L or valuation question. The two packages share no types; a
caller may use `analytics/forecast`'s output to *inform* assumptions fed
into `cashforecast`, but neither package requires the other.

It is also distinct from `accounting/ap`'s `DueSchedule` (a grouping of
already-existing open bills by due-date horizon, explicitly labeled "not
a forecast") and `PaymentPressure` (a single-snapshot liquidity check).
Both of those AP-package types exist precisely because AP intentionally
stops short of projecting a multi-week cash position — this package is
that projection.

## The package never invents future cash flows

Every inflow and outflow in a `Result` traces back to a caller-supplied
`CashFlowEvent` — either directly, or generated deterministically from a
caller-supplied recurring rule, AR/AP scheduling assumption, or scenario
transformation. This package performs no statistical prediction, no
AI/LLM forecasting, and applies no default collection or payment timing
the caller did not explicitly request. An AR receivable's due date is
never silently assumed to be its cash receipt date — see [AR
scheduling](#ar-scheduling). An AP bill's due date only becomes a
scheduled outflow when the caller explicitly opts in — see [AP
scheduling](#ap-scheduling).

## Entry point

```go
func Calculate(in Input, opts Options) Result
```

`Calculate` never mutates any field of `in` or `opts`, performs no I/O,
and is safe for concurrent use — repeated calls with identical input
produce byte-for-byte identical JSON output.

## 13-week semantics and explicit start date

`Input.ForecastStartDate` is required and is never defaulted from
`time.Now()` — a zero-value date produces `Result{Available: false}` with
`IssueInvalidForecastStart`. There is no hidden current-date dependency
anywhere in this package.

`Options.HorizonWeeks` defaults to 13 (`DefaultHorizonWeeks`) when zero,
and accepts any value from 1 to `MaxHorizonWeeks` (52). Extra
configurability was deliberately kept out of the core contract: nothing
in the calculation logic branches on the horizon length beyond "how many
weeks to generate."

Week boundaries are deterministic and calendar-based, controlled by
`Options.WeekAlignment`:

- `CALLER_START_DATE` (the default): Week 1 starts exactly on
  `ForecastStartDate` and spans 7 calendar days (start inclusive, end 6
  days later, inclusive). Week 2 starts immediately after, and so on.
- `MONDAY`: Week 1 starts on the Monday on or before `ForecastStartDate`.
- `SUNDAY`: Week 1 starts on the Sunday on or before `ForecastStartDate`.

This package never assumes a Monday/Sunday week without the caller
explicitly choosing one of the latter two alignments.

Every date field on `Result`/`WeeklyForecast` is a formatted
`"2006-01-02"` string (matching `ar.Result.AsOfDate`/`ap.Result.AsOfDate`'s
identical convention), even though every corresponding `Input` field is a
`time.Time`.

## Opening and restricted cash

`Input.OpeningCash` (amount, currency, as-of date, source ref) is
required and is authoritative — this package never derives an opening
balance from `CashFlowEvent` history.

Optionally, `Input.CashAccounts` supplies a per-account breakdown
(`AccountID`, `Balance`, `Currency`, `Restricted`, `MinimumReserve`).
When supplied, `OpeningPosition.UnrestrictedCash` (the figure every
weekly rollforward and minimum-cash/funding-gap comparison is measured
against) excludes every `Restricted` account's balance;
`OpeningPosition.RestrictedCash` and `OpeningPosition.TotalCash` remain
separately visible. Restricted cash is never silently treated as
available liquidity. Without `CashAccounts`, `UnrestrictedCash` equals
`TotalCash` — this reflects "no restriction information supplied," not
"confirmed unrestricted."

If both `OpeningCash.Amount` and `CashAccounts` are supplied and their
totals disagree beyond floating-point tolerance, `IssueOpeningCashMismatch`
is raised (a warning); the `CashAccounts`-derived total is what the
forecast actually uses.

## Minimum-cash policy

`Options.MinimumCash` (`MinimumCashPolicy`) configures the threshold used
for headroom/funding-gap analysis. This package never invents a default
minimum: if neither `MinimumCashBalance` nor a nonzero
`CashAccount.MinimumReserve` sum is supplied, every threshold-dependent
field (`Headroom`, `FundingGap`, `FirstWeekBelowMinimum`,
`MaximumFundingGap`, `RequiredFunding`) is left unavailable while weekly
cash balances remain fully available. A caller wanting an explicit $0
minimum (distinct from "no threshold supplied at all") sets
`MinimumCashBalanceExplicitZero`.

## Cash event model

`CashFlowEvent` is the portable, dated unit every weekly bucket is built
from:

```go
type CashFlowEvent struct {
    ID, Date, Amount, Direction, Category, Subcategory, Description,
    CounterpartyID, SourceType, SourceID, Basis, Certainty, Commitment,
    Priority, ScenarioTags, Dimensions
}
```

- **Direction** (`INFLOW`/`OUTFLOW`) — never encoded via the sign of
  `Amount`. `Amount` must always be `>= 0`; `Direction` determines sign.
  A refund/credit uses `Direction`/`Category` explicitly, never a
  negative `Amount`.
- **Category** — a small, fixed, closed taxonomy (see below), chosen
  deliberately small per the "do not create an unnecessarily huge
  taxonomy" design goal. Finer-grained classification uses
  `Subcategory`, an open string this package never validates.
- **Basis** — `KNOWN`/`SCHEDULED`/`ASSUMED`/`SCENARIO`, distinguishing
  how firmly grounded the event is. This package never calls an assumed
  amount "known," and never infers or upgrades a `Basis` value.
- **Certainty** — an optional `COMMITTED`/`HIGH`/`MEDIUM`/`LOW`/`UNKNOWN`
  label, descriptive metadata only. Never converted into a probability;
  no probabilistic forecasting exists in this package.
- **Commitment** — an optional `REQUIRED`/`DEFERRABLE`/`DISCRETIONARY`
  label on outflows, never inferred from `Category`.
- **Priority** — an optional `CRITICAL`/`HIGH`/`NORMAL`/`LOW` label,
  metadata only in the base calculation; only used by the opt-in
  `DeferByPriority` scenario helper.

## Cash categories

Inflows: `AR_COLLECTION`, `CASH_SALE`, `OTHER_OPERATING_INFLOW`,
`LOAN_PROCEEDS`, `OWNER_CONTRIBUTION`, `ASSET_SALE`,
`OTHER_FINANCING_INFLOW`, `OTHER_INFLOW`.

Outflows: `AP_PAYMENT`, `PAYROLL`, `PAYROLL_TAX`, `SALES_TAX`,
`INCOME_TAX`, `RENT`, `DEBT_SERVICE`, `CAPEX`, `OWNER_DISTRIBUTION`,
`OTHER_OPERATING_OUTFLOW`, `OTHER_FINANCING_OUTFLOW`, `OTHER_OUTFLOW`.

Each category has a fixed expected `Direction` (validated against the
event's actual `Direction` — a mismatch is rejected, not silently
corrected) and a fixed `CashFlowClass` (`OPERATING`/`INVESTING`/
`FINANCING`), used for `WeeklyForecast`'s operating/investing/financing
split.

## AR scheduling

**An AR receivable's due date is never automatically its cash receipt
date.** Supplying `Input.ARSources` alone (with no
`Input.ARCollections`) produces zero AR inflow events — the entire open
amount is reported via `UnscheduledAR` instead. This is a permanent,
regression-tested invariant (see
[`arapintegration_test.go`](../accounting/cashforecast/arapintegration_test.go)'s
`TestARIntegration_DueDateNeverAutoBecomesReceiptDate`).

To schedule an actual AR inflow, a caller supplies explicit
`ARCollectionAssumption{ID, ReceivableID, ExpectedReceiptDate,
ExpectedAmount, Certainty, Basis}` entries. Multiple partial assumptions
against the same `ReceivableID` are supported as long as their total does
not exceed that receivable's `OpenAmount` (from the matching
`ARReceivableSource`, when supplied) — an over-scheduled receivable
raises `IssueARScheduleExceedsOpen` and the offending assumption is
excluded.

`ARReceivableSource{ReceivableID, CustomerID, OpenAmount}` is a portable
identity+amount type a caller derives from its own `accounting/ar`
`Input.Receivables` (or `Result.CustomerSummaries`, if only customer-level
totals are available) — this package never imports `accounting/ar`
directly, per the architectural instruction to prefer portable typed
inputs over a runtime dependency when the semantics fit cleanly.

## AP scheduling

AP is closer to cash scheduling than AR because bills already carry due
dates, but the same caller-explicit-control principle applies. The
default, safest behavior is **no automatic AP payment schedule** —
businesses often pay early, late, or selectively.

A caller supplies explicit `APPaymentPlan{ID, PayableID, PaymentDate,
Amount, Certainty, Basis}` entries the same way as AR. Over-scheduling
(the sum of plans against one `PayableID` exceeding that payable's open
amount) raises `IssueAPScheduleExceedsOpen`.

Setting `Options.PayAPOnDueDate = true` opts into a due-date adapter:
every `APPayableSource` payable **not already fully covered by an
explicit `APPaymentPlan`** generates one outflow event on its own due
date, for the remaining (unscheduled) amount only — never double-counting
the explicitly planned portion. In this mode `UnscheduledAP` is always
empty by construction, since every open payable is covered.

## Payroll, tax, debt service, and capex

None of these are calculated by this package — the caller supplies cash
figures directly:

- `PayrollEvent{..., EmployeeNetCash, EmployerTaxes,
  EmployeeWithholdingsRemitted, Benefits, OtherCash}` — no employment/tax
  rules; the total cash-out is the sum of every supplied component.
- `TaxEvent{..., Category: SALES/PAYROLL_REMITTANCE/INCOME/OTHER}` — no
  tax-liability computation.
- `DebtServiceEvent{..., Date, Amount, LoanLabel}` — a caller-supplied
  explicit date and amount. See [No `analytics/debt`
  adapter](#why-there-is-no-analyticsdebt-adapter) for why this package
  never derives debt-service dates from an amortization schedule.
- `CapexEvent{..., Date, Amount, Description, Commitment}` — kept
  separate from ordinary operating outflows via `CategoryCapex`; no
  depreciation calculation.

## Financing events

`FinancingEvent{..., Type, Amount, Date}` covers loan proceeds,
line-of-credit draws, loan repayments, equity contributions, owner
contributions, and owner distributions — each mapped to a fixed
`CashCategory`/`Direction` pair. This package never automatically creates
a financing event to cover a funding gap; every `FinancingEvent` is
caller-initiated.

## Recurring events

`RecurringRule{ID, Amount, Direction, Category, StartDate, EndDate,
Frequency, SemimonthlyDays}` deterministically expands into one
`CashFlowEvent` per occurrence within the forecast horizon. Supported
frequencies: `WEEKLY`, `BIWEEKLY`, `SEMIMONTHLY`, `MONTHLY`, `QUARTERLY`.

**Month-end day-roll behavior**: a `StartDate` anchored on the 29th,
30th, or 31st clamps to the target month's *last day* when that month has
fewer days, rather than rolling forward into the next month the way
`time.Time.AddDate(0, 1, 0)` would (e.g. `AddDate` on Jan 31 produces Mar
3, since Go normalizes "Feb 31"). This package instead produces Jan 31 →
Feb 28/29 → Mar 31 → Apr 30 → ... A leap-year Feb 29 anchor clamps to Feb
28 in a non-leap year and reappears on Feb 29 the following leap year —
the anchor day is never permanently downgraded. See
[`recurring_test.go`](../accounting/cashforecast/recurring_test.go)'s
`TestRecurring_Monthly31st` and `TestRecurring_MonthlyLeapYearFeb29` for
the exact locked date series.

Semimonthly rules fire on two fixed days-of-month (clamped to `[1, 28]`
so every month, including February, always produces exactly two
occurrences).

This package never infers recurring costs from historical ledger
activity — every rule is fully caller-specified.

## Why there is no `analytics/debt` adapter

`analytics/debt`'s public API (`AmortizationSchedule`) exposes only
`FirstYearAnnualDebtService` and `SteadyStateAnnualDebtService` — annual
aggregate figures with no calendar anchor and no per-payment date list at
all. There is no way to derive which week within a 13-week horizon any
specific debt payment falls in from that package's output alone, and
dividing an annual figure by 52 would fabricate weekly timing this
package has no basis for — exactly what this package's design
deliberately avoids. See
[`debtadapter.go`](../accounting/cashforecast/debtadapter.go) for the
full explanation. A caller with real debt-service dates (from a loan
servicer or its own date-aware amortization wrapper) supplies them
directly as `DebtServiceEvent`s — no adapter is needed for that path.

## Scenarios

`Scenario{Label, Transforms}` is a named what-if case: a set of pure
`EventTransform`s applied, in order, to a fresh copy of the base event
set. This package never invents scenario assumptions — every
transformation is a caller-specified parameter (a delay in days, a scale
factor, an explicit added/removed event, a priority-based deferral).

Available transform kinds: `DELAY_INFLOWS`, `SCALE_INFLOWS`,
`DELAY_OUTFLOWS`, `ACCELERATE_OUTFLOWS`, `SCALE_CATEGORY`,
`REMOVE_EVENT`, `ADD_EVENT`, `DEFER_BY_PRIORITY`. Each corresponding
exported helper function (`DelayEvents`, `ScaleEvents`,
`AccelerateEvents`, `RemoveEventByID`, `AddEvent`, `DeferByPriority`)
returns a new slice and never mutates its input.

Every named scenario is computed independently from the same immutable
base event set — scenarios are never chained. `Result.BaseScenario` is
always present; `Result.Scenarios` holds one `ScenarioResult` per
`Input.Scenarios` entry, each with `DeltaVsBase` (per-week and
cumulative-horizon variance against the base case). Recomputing the same
scenarios in a different input order produces byte-identical
per-scenario results — see
[`determinism_test.go`](../accounting/cashforecast/determinism_test.go)'s
`TestDeterminism_ScenarioOrderIndependence`.

## Liquidity and funding gap

For each scenario, `LiquiditySummary` reports `StartingCash`,
`TotalInflows`, `TotalOutflows`, `NetChange`, `EndingCash`,
`LowestCashBalance`/`LowestCashWeek`, and — only when a minimum-cash
threshold is resolved — `FirstWeekBelowMinimum`, `WeeksBelowMinimum`,
`MaximumFundingGap`, `EndingFundingGap`, and `FirstNegativeCashWeek`/
`NegativeCashReached`.

**`RequiredFunding.RequiredAtStart`** is the minimum single upfront cash
injection at Week 1's start that keeps every week's ending cash at or
above the minimum threshold across the entire horizon. Because the
weekly rollforward is a simple additive chain (each week's opening cash
equals the prior week's ending cash), adding a constant `X` to Week 1's
opening cash adds exactly `X` to every subsequent week's ending cash
too — so a single upfront injection equal to the horizon's single worst
(most negative) `FundingGap` week is sufficient to clear every week's
gap simultaneously. `RequiredAtStart` is therefore exactly
`MaximumFundingGap`. Locked example: minimum cash $50,000, lowest
projected cash −$20,000 → `RequiredAtStart` = $70,000 (see
[`fundinggap_test.go`](../accounting/cashforecast/fundinggap_test.go)'s
`TestFundingGap_LockedFormula`).

`Options.RequiredInputs`/`MinimumCash`/`FlagThresholds` never imply
financing is actually obtainable — this is a deterministic arithmetic
requirement, not a statement that a lender would approve it.

## Credit-facility boundary

`CreditFacility{FacilityID, AvailableToDraw, MinimumDraw, MaximumDraw,
ExpiryDate}` is reported purely as available capacity alongside the
funding gap. This package **never automatically creates a draw** against
a facility. `LiquiditySummary.GapAfterFacility` reports the arithmetic
remainder (`MaximumFundingGap - FacilityCapacity`, floored at 0) without
asserting a draw occurred — a caller wanting to model an actual draw adds
an explicit `FinancingEvent` (`LOAN_PROCEEDS`/`LINE_OF_CREDIT_DRAW`) or
uses a scenario transform instead.

## Coverage, completeness, and staleness

`Coverage` reports, per category (AR, AP, Payroll, DebtService, Tax,
RecurringOpex, Capex), whether the caller supplied anything at all
(`Supplied`), and — for AR/AP specifically — what fraction of the known
open amount was actually scheduled (`ScheduledPercent`). This package
never invents a universal completeness score; every figure is a factual
count or percentage.

`Options.RequiredInputs` lets a caller mark a category as required;
a missing required category raises `IssueMissingRequiredInput` and
appears in `Coverage.RequiredInputsMissing` — no category is required by
default.

`Coverage.ARStaleness`/`APStaleness`/`CashStaleness` report source-as-of-
date age when both the as-of date and a caller-supplied
`StalenessPolicy.Max*AgeDays` threshold are available; without a
threshold, age is still reported but `Stale` is always `false` — this
package never infers a staleness threshold on its own.

## Unscheduled AR/AP

`Result.UnscheduledAR`/`UnscheduledAP` report, for every source with a
known open amount, the portion not covered by any scheduled event — count,
total amount, and IDs. This is deliberately surfaced at the top level so a
13-week forecast never looks complete when known receivables/payables
were silently omitted from scheduling.

## Provenance

Every `WeeklyForecast.Inflows`/`Outflows` category and basis breakdown
carries the exact `EventID`s that produced it
(`CategoryAmount.EventIDs`, sorted ascending). `Result`'s per-scenario
`DetailedSchedule` is a flat, presentation-neutral list of every event
with its resolved week, sorted by date, then direction, then category,
then ID. A future consumer can answer "why is Week 7 cash down $84,000"
by inspecting these fields directly, without parsing message text.

## Issues vs. flags

**Issue** — an input/configuration/integrity problem (e.g.
`INVALID_EVENT`, `DUPLICATE_EVENT`, `MIXED_CURRENCY`,
`AR_SCHEDULE_EXCEEDS_OPEN`, `MISSING_REQUIRED_INPUT`).

**Flag** — a liquidity/business-result signal (e.g.
`CASH_BELOW_MINIMUM`, `NEGATIVE_CASH`, `MATERIAL_FUNDING_GAP`,
`UNSCHEDULED_AR`, `STALE_AR_SOURCE`, `FACILITY_INSUFFICIENT_FOR_GAP`).

Both taxonomies are package-local, matching every sibling package's "each
package defines its own taxonomy" convention. Flag trigger points are
caller-configurable via `Options.FlagThresholds`; there is no hidden risk
score.

## Validation

`Calculate` validates: forecast start date, horizon, opening cash
(structure and currency consistency), event IDs (non-empty, no
duplicates), event dates (non-zero), amount finiteness (`NaN`/`Inf`
rejected), amount sign (`Amount` must be `>= 0`), direction/category
validity and their pairing, recurring-rule structure, AR/AP plan totals
against open amounts, mixed currency, scenario label uniqueness, facility
structure, and minimum-cash threshold resolution. An invalid `Event` is
excluded, never silently repaired.

## Availability semantics

This package distinguishes known-zero from not-supplied from
not-applicable throughout: `AmountValue{Available, Value}` marks a
figure unavailable rather than defaulting it to 0; `UnscheduledSummary`,
`Coverage.*`, and `ThresholdPosition` each have their own `Available`
gate. "No debt service supplied" (`Coverage.DebtService.Supplied ==
false`) is never conflated with "debt service is exactly $0" (a
`DebtServiceEvent` with `Amount: 0`, which is a valid, meaningful input).

## JSON and determinism

Every type uses explicit snake_case JSON tags. `CashFlowEvent`,
recurring rules, AR/AP plans, `WeeklyForecast`, `ScenarioResult`,
`Coverage`, and the full `Result` all round-trip through
`encoding/json` — see
[`roundtrip_test.go`](../accounting/cashforecast/roundtrip_test.go). No
`NaN`/`Inf` ever appears in JSON output (non-finite amounts are excluded
at validation, before serialization).

All output ordering is deterministic: weeks chronological; events by
date, then direction, then category, then ID; scenarios in input order;
flags by fixed declaration-order rank, then scenario, then week;
categories in fixed enumeration order. No output ever depends on Go map
iteration order — see
[`determinism_test.go`](../accounting/cashforecast/determinism_test.go).

## Versioning

`SchemaVersion` covers the shape of `CashFlowEvent`, `OpeningCash`,
`CashAccount`, every recurring/AR/AP/schedule type, `CreditFacility`,
`Scenario`, `WeeklyForecast`, `ScenarioResult`, `Coverage`, and `Result`.
`FormulaVersion` covers week-boundary assignment, the weekly rollforward
and category-breakdown aggregation, AR/AP scheduling adapters, recurring-
event generation, scenario transformation and delta-vs-base calculation,
liquidity/funding-gap/runway/required-funding formulas, and
coverage/completeness. Both are echoed on every `Result`.

## Explicit non-goals

This package does not implement: AI cash prediction, bank-balance
syncing, bank reconciliation, invoice-collection prediction, payment
execution, AP payment optimization, borrowing decisions, lender approval,
tax computation, payroll computation, daily/intraday treasury, or any
automatic AR/AP behavior assumption beyond what a caller explicitly opts
into (`PayAPOnDueDate`). It does not implement long-term P&L or
valuation forecasting — `analytics/forecast` remains the long-range
financial-projection engine.

## Limitations

- Cash balance is weekly, not intraday — multiple same-day events sum
  deterministically with no same-day payment ordering. A caller needing
  daily liquidity needs a separate extension.
- V1 supports one reporting currency; all events must already be in that
  currency (or the caller pre-converts). This package never fetches FX
  and never silently sums mixed currencies.
- No `analytics/debt`/`transactions/dealstructure` adapter exists — see
  [Why there is no `analytics/debt`
  adapter](#why-there-is-no-analyticsdebt-adapter).
- `RequiredFunding` is a deterministic arithmetic figure, not a statement
  that financing is obtainable on any terms.
