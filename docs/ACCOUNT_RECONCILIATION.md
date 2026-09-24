# Account Reconciliation (`accounting/reconciliation`)

## Purpose

`accounting/reconciliation` implements deterministic account
reconciliation between **book-side** records (a caller's own ledger or
subledger) and **external/control-side** records (a bank statement, a
lender statement, a GL control-account balance, an intercompany
counterparty's reciprocal balance, or any other independent source of
truth for the same account). It supports bank, credit-card, loan, AR/AP
subledger-to-GL, payroll clearing, inventory control, intercompany, and
suspense/clearing reconciliation — plus a generic balance-only case for
anything not covered by a named type.

This package performs **matching and reconciliation arithmetic only**.
It does not fetch bank data, parse OFX/CSV bank files, call a bank or
accounting-system API, post journal entries, generate adjustments,
execute payments, run OCR, manage workflow/approvals, apply AI/LLM or
fuzzy NLP matching, or draw fraud conclusions — see [Explicit
non-goals](#explicit-non-goals).

Unlike most sibling `accounting/*` packages, this one is named the same
as an unrelated existing package, `financial/reconciliation` (a
balance-sheet-equation checker inside the pre-existing
`financial.FinancialDataset` model, built before this package). The two
share only a leaf package name — their import paths
(`github.com/themurtez/go-valuate/accounting/reconciliation` vs
`.../financial/reconciliation`) are distinct, and Go's import system
never confuses them. They solve entirely different problems and neither
imports the other.

## One engine, nine labels

[`ReconciliationType`](../accounting/reconciliation/types.go) (`BANK`,
`CREDIT_CARD`, `LOAN`, `AR_CONTROL`, `AP_CONTROL`, `INVENTORY_CONTROL`,
`PAYROLL_CLEARING`, `INTERCOMPANY`, `SUSPENSE_CLEARING`, `GENERIC`) is
metadata only. It never triggers a parallel special-case code path —
there is exactly one matching/arithmetic engine underneath every type.
What actually differs between a bank reconciliation and a credit-card
reconciliation is [`MatchingPolicy`](#matching-policy) (especially
`Orientation`), never a different function. `Type`'s only computed
effect is on display/labeling; a caller may even pass `""` (treated as
`GENERIC`) and get identical arithmetic to any other type given the same
policy.

## Transaction mode vs balance-only mode

`Calculate` supports three shapes of input, distinguished purely by
what the caller supplies:

- **Balance-only**: `BookBalance`/`ExternalBalance` set, no
  `BookItems`/`ExternalItems` at all. Returns `Difference` and `Status`
  from the [reconciliation equation](#the-reconciliation-equation) —
  never a fake transaction match.
- **Transaction mode**: `BookItems`/`ExternalItems` set (with or without
  balances). Returns `MatchedGroups`, `UnmatchedBookItems`,
  `UnmatchedExternalItems` — no unmatched row is ever silently dropped.
- **Both**: the common real-world case — transaction-level matching
  *and* a balance tie-out, kept as two genuinely separate concerns (see
  [Balance reconciliation vs transaction
  coverage](#balance-reconciliation-vs-transaction-coverage)).

`Result.TransactionModeAvailable` reports whether any item was supplied
at all, independent of whether a balance was also supplied.

## Book vs external item model

[`BookItem`](../accounting/reconciliation/types.go) and
[`ExternalItem`](../accounting/reconciliation/types.go) are structurally
identical (`ItemID`, `Date`, `Amount`, `Direction`, `Reference`,
`Description`, `AccountID`, `Currency`, `SourceType`, `SourceID`,
`SourceRef`) but kept as two distinct Go types so every function
signature states unambiguously which population an argument belongs to.
"Book" is always the caller's own record; "External" is always the
independent control-side record. Which side is which is a caller
choice — this package never infers it.

## Direction and signed-amount convention

Every item carries a non-negative `Amount` plus an explicit `Direction`
(`INFLOW` or `OUTFLOW`), never a single pre-signed amount. Internally,
every function derives one canonical signed figure:

```
INFLOW  -> +Amount
OUTFLOW -> -Amount
```

`BookItem.SignedAmount()` and `ExternalItem.SignedAmount()` compute this.
Every comparison, sum, and equation term in this package uses the signed
figure, never the raw `Amount` alone. The original `Direction`/`Amount`
pair is always preserved on the item itself.

A negative `Amount` is flagged (`IssueInvalidAmount`, warning severity)
but never excludes the item — the item is still matched using its
literal signed value, so a caller's already-signed data is never
silently altered, only warned about.

## Orientation

`MatchingPolicy.Orientation` (`SAME` or `REVERSED`, zero value = `SAME`)
declares whether the external side's signed convention runs the same
way as the book side's, or the opposite way, **before any comparison**.
`ExternalItem.OrientedSignedAmount(o)` applies this; the book side's own
sign is never flipped.

This matters most for liability-type reconciliations:

- **Bank** (an asset account): book and external usually already agree
  on direction — `SAME` is the typical default.
- **Credit card / loan** (liability accounts): a purchase increases what
  is owed; a payment decreases it. From the card issuer's or lender's
  own statement perspective, that framing is naturally the *opposite*
  of the cardholder's/borrower's own cash-movement framing — `REVERSED`
  makes the two sides' figures compare correctly. See
  `fixtures.CreditCardInput`'s doc comment for a fully worked example of
  how `Direction` is recorded on each side under `REVERSED`.
- **Intercompany**: each entity's own books naturally record the
  reciprocal balance in its own normal sign convention, which is
  typically the opposite of the counterparty's — `REVERSED` again.

**Orientation applies to balances too, not just transactions.**
`BalanceInput.OpeningBalance`/`EndingBalance` on the external side are
multiplied by `orientationMultiplier(Orientation)` before any rollforward
or cross-side comparison — this was a real bug caught while building
this package's own fixtures (a balance-only intercompany reconciliation
silently compared unoriented figures until the fix) and is now locked by
`TestBalances_OrientationAppliesToSuppliedBalances` and
`TestInvariant_ReversedOrientationBalanceOnlyReconciles`.

## Balances, derivation, and rollforward

`BalanceInput` carries an optional `OpeningBalance` and `EndingBalance`
per side. `Balance` (the resolved output) adds:

- `EndingProvenance`: `SUPPLIED` or `DERIVED` (see below).
- `ActivityTotal`: the sum of that side's oriented signed item amounts,
  when any were supplied.
- `ExpectedEnding` = `OpeningBalance + ActivityTotal`, when both are
  available.
- `RollforwardDifference`/`RollforwardMismatch`: `EndingBalance -
  ExpectedEnding`, computed **independently per side**, never mixed with
  the cross-side reconciliation difference.

**Derivation** (`MatchingPolicy.DeriveBalances`, off by default) lets
this package compute an ending balance from `Opening + Activity` when
the caller did not supply one. If the caller *did* supply an ending
balance and derivation is enabled, the two are cross-checked: on
disagreement beyond tolerance, `SuppliedDerivedMismatch` is set and
`IssueSuppliedDerivedBalanceMismatch` is reported — the supplied value is
**never silently replaced**.

## Matching precedence

Confirmed matches are applied first (see below). For everything else,
`Calculate` walks book/external items through a fixed precedence order,
never re-considering an item once consumed:

1. **`EXACT_REFERENCE_AMOUNT_DATE`** — normalized reference equality +
   amount within tolerance + date within the (possibly zero) window.
2. **`EXACT_AMOUNT_DATE`** — amount within tolerance + exact same date,
   no reference requirement.
3. **`EXACT_AMOUNT_WITHIN_WINDOW`** — amount within tolerance + date
   within `MatchingPolicy.DateWindowDays` (skipped entirely if that
   field is 0 — no universal clearing-window default is invented).
4. **Composite matching** (`COMPOSITE_SUM_MATCH`) — bounded one-to-many /
   many-to-one, opt-in only (see [Composite matching](#composite-matching)).
5. Anything left is **unmatched**, listed in
   `UnmatchedBookItems`/`UnmatchedExternalItems`.

Each of stages 1–3 only ever proposes a `ONE_TO_ONE` group, and a
book/external item is consumed the instant it is matched — it is never
reconsidered by a later stage.

### Reference normalization

`NormalizeReference` applies only the safe, explicit transformations
`ReferenceNormalization` declares, in order: trim, case-fold, remove
punctuation, remove spaces, strip-leading-zeros (opt-in; leading zeros
are meaningful by default). No fuzzy/NLP transformation exists anywhere
in this package.

### Description boundary

`Description` is never sole matching evidence.
`MatchingPolicy.DescriptionExactMatchEnabled` (off by default) makes it
an *additional narrowing filter* on stages 1–3: when both sides carry a
non-empty description, exact normalized (trimmed, case-folded,
whitespace-collapsed) equality is required in addition to whatever that
stage already requires. An item with no description at all is never
excluded on that basis — a missing description narrows nothing.

## Ambiguity

If more than one equally valid candidate exists under the *current*
precedence rule — from either side's perspective — this package
**never picks one arbitrarily**. Both the forward check (does this book
item have exactly one valid external candidate?) and the reverse check
(is this book item also the *unique* valid book-side candidate for that
external item?) must independently hold, or the pairing is reported as
ambiguous via `Result.AmbiguousCandidates` and a single
`FindingAmbiguousMatch` per distinct anchor item, rather than matched.
Determinism (a fixed algorithm) never justifies an unsupported pairing —
see `TestInvariant_AmbiguityNeverResolvedArbitrarily` and
`TestMatcher_RepeatedAmountsNoArbitraryPairing` (120 identical amounts,
zero arbitrary pairs).

## Composite matching

`MatchingPolicy.EnableCompositeMatching` (off by default) turns on
bounded one-book-to-many-external and many-book-to-one-external
matching: one item's amount equals the sum of several items on the
opposite side. Bounds, all required once enabled:

- `MaxCompositeGroupSize` — the largest subset size searched.
- `MaxCandidatesPerItem` — the candidate pool cap per anchor item; if the
  pool already exceeds this, search for that anchor is skipped and
  `IssueCompositeSearchLimitReached` is reported (the pool is never
  silently truncated to an arbitrary subset).
- `MaxCompositeSearchCombinations` — the total subset-combination budget
  per anchor; if exceeded mid-search, the same issue is reported and
  only whatever was found before the budget ran out is kept.

If exactly one subset sums to the anchor's amount within tolerance, it
is matched (`ONE_BOOK_TO_MANY_EXTERNAL` / `MANY_BOOK_TO_ONE_EXTERNAL`,
confidence `REVIEW`). If two or more equally valid subsets exist, the
anchor is left unresolved — see `TestComposite_AmbiguousSumsNeverAutoMatch`
for the exact worked example (book 300; external 50+250 and 100+200 both
sum to 300; neither is chosen).

Many-to-many is **only** reachable through a `ConfirmedMatch` — this
package never automatically proposes it.

## Confirmed (manual) matches

`ConfirmedMatch` values take absolute precedence and are validated
independently of auto-matching:

- Every referenced `BookItemID`/`ExternalItemID` must resolve to a real
  item.
- No item may appear in more than one `ConfirmedMatch`.
- The group's items must share a compatible currency.
- Amounts must tie within `MatchingPolicy` tolerance, **unless** the
  caller sets `ExplainedDifference` (an accepted, already-explained
  discrepancy).

A `ConfirmedMatch` failing *any* check is rejected **in full** — never
partially applied — and reported via `IssueInvalidConfirmedMatch` /
`IssueUnknownMatchItem` / `IssueItemUsedInMultipleMatches` /
`IssueMixedCurrency`. Its items then fall through to ordinary
auto-matching like any other item. A **valid** confirmed match is never
displaced by a "stronger-looking" automatic candidate — see
`TestInvariant_ManualMatchOverridesStrongerAutomatic` for the exact
scenario the underlying task named ("B1/E4 remains even if E3 looks
stronger").

## Amount and date tolerance

`MatchingPolicy.AmountTolerance` (absolute) and `RelativeTolerance`
(proportional to the larger magnitude) combine via OR:
`amountsMatch(a, b, tol, relTol)` accepts either an absolute or a
relative match. No universal accounting tolerance is invented — the
zero value means exact equality is required.
`MatchingPolicy.DateWindowDays` similarly has no invented default: 0
means only same-date matching is attempted at all (stage 3 is skipped
entirely).

## Reconciling items and the reconciliation equation

`ReconcilingItem` (`OUTSTANDING_CHECK`, `DEPOSIT_IN_TRANSIT`,
`UNRECORDED_FEE`, `UNRECORDED_INTEREST`, `TIMING_DIFFERENCE`,
`BOOK_ADJUSTMENT`, `EXTERNAL_ADJUSTMENT`, `OTHER`) is **always
caller-supplied, never inferred or auto-generated**, and never creates a
journal entry. Each has an explicit `Side` (`BOOK` or `EXTERNAL`) and an
already-signed `Amount` (this package never re-derives or flips its
sign — see `ReconcilingItem.Amount`'s doc comment).

The equation, applied consistently everywhere balances are available:

```
AdjustedBookBalance     = BookEndingBalance     + sum(reconciling items, Side=BOOK)
AdjustedExternalBalance = ExternalEndingBalance + sum(reconciling items, Side=EXTERNAL)
Difference              = AdjustedBookBalance - AdjustedExternalBalance
```

`EquationResult.Available` is true only when **both** sides' ending
balances resolved (`Balance.EndingAvailable`) — never a partial or
assumed difference from one side alone.

**An unmatched item explained by a `ReconcilingItem` (via
`RelatedItemID`) never also produces
`FindingMaterialUnmatchedBookItem`/`ExternalItem`.** It remains listed in
`UnmatchedBookItems`/`UnmatchedExternalItems` (never dropped) and its
dollar effect is still counted through the equation — only the redundant
finding is suppressed, since the caller already explained the gap. See
the classic bank-reconciliation fixture
(`fixtures.BankInput`/`BankReconcilingItems`) for a fully worked example:
outstanding-check and deposit-in-transit items adjust the *external*
side (the bank hasn't processed them yet); an unrecorded fee adjusts the
*book* side (the books haven't recorded it yet).

## Status semantics

```go
type Status string
const (
    StatusReconciled          Status = "RECONCILED"
    StatusReconciledWithItems Status = "RECONCILED_WITH_ITEMS"
    StatusUnreconciled        Status = "UNRECONCILED"
    StatusIncomplete          Status = "INCOMPLETE"
    StatusInvalid             Status = "INVALID"
)
```

Exact decision rule (`status.go`'s `deriveStatus`):

- **`INVALID`**: `AccountID` or `AsOfDate` itself is invalid. Note this
  is deliberately **narrower** than "any error-severity `Issue` exists
  anywhere" — a malformed individual item, a duplicate ID, or a rejected
  confirmed match each produce their own error-severity `Issue` but are
  recoverable per-item problems this package's own validation already
  excluded/fell back from; they do not make the *whole* reconciliation
  `INVALID` on their own. Only the reconciliation's own identity is
  load-bearing enough for that.
- **`INCOMPLETE`**: the key/date are valid, but neither a resolvable
  balance nor any transaction-level item data was supplied for either
  side — not enough data to assess anything.
- **`RECONCILED`**: adjusted balances tie within tolerance (or no
  balance data was supplied at all, only transactions) **and** no
  material unmatched items, no unresolved reconciling items, and no
  ambiguous matches remain.
- **`RECONCILED_WITH_ITEMS`**: the same tie condition holds, but
  explicit `ReconcilingItem`s remain (the caller has explained the gap,
  but it is still visibly present).
- **`UNRECONCILED`**: a balance difference beyond tolerance exists, OR
  material unresolved items (unmatched or ambiguous) remain — regardless
  of whether the balance happens to tie.

Invariant locked by test: `RECONCILED` implies the balance difference
was within tolerance (never the reverse claim — an out-of-tolerance
difference can never be labeled `RECONCILED`).

## Material unmatched policy

`MatchingPolicy.UnmatchedPolicy`:

- **`ALLOW_IMMATERIAL_UNMATCHED`** (default): only unmatched items
  material per `Materiality` (`AbsoluteAmount` / `PercentOfBalance`, OR
  semantics — everything is material when both are zero) produce
  `FindingMaterialUnmatchedBookItem`/`ExternalItem` and affect `Status`.
  Immaterial unmatched items still appear in
  `UnmatchedBookItems`/`ExternalItems` — **never silently discarded** —
  they simply produce no finding and don't block a reconciled status.
- **`REQUIRE_ALL_ITEMS_MATCHED`**: any unmatched item at all (material or
  not) produces the finding and blocks `RECONCILED`/`RECONCILED_WITH_ITEMS`.

## Stale items

`MatchingPolicy.StaleDaysThreshold` (0 = never assessed — no universal
default) gates `FindingStaleUnmatchedItem`/`FindingStaleReconcilingItem`,
computed from each item's own date relative to `Input.AsOfDate`. This is
**generic reconciliation aging**, distinct from `accounting/ar`'s or
`accounting/ap`'s own domain-specific receivable/payable aging — no
formula is shared or duplicated between them.
`AgingBucket`/`AgedItem` provide optional caller-configured bucket
labels alongside the raw `DaysOutstanding` figure.

## Coverage and match-quality summary

`Coverage` reports **factual** figures only — item counts, matched
amount/item percentages per side, reference/description coverage
percentages, balance availability — **no composite quality score**.
`MatchQualitySummary` breaks every item into exactly one mutually
exclusive bucket by *how* it was resolved (`Confirmed`,
`ReferenceMatch`, `SameDateAmount`, `DateWindowAmount`, `Composite`,
`Ambiguous`, `Unmatched`), each with count/gross/net amounts — again, no
opaque score.

## Balance reconciliation vs transaction coverage

These are kept **strictly separate**, per design: a balance-only
reconciliation can be `RECONCILED` while `Coverage.BalanceAvailable` is
true and `TransactionModeAvailable` is false — there is no transaction
population to assess at all, and that is not itself a defect. Symmetric-
ally, a caller doing transaction-level-only matching (no balances
supplied) can still reach `RECONCILED` purely on unmatched/ambiguous
item counts. Neither mode implies or requires the other.

## Adapters

Six typed, portable adapters bridge sibling packages' already-computed
results into this package's `BookItem`/`BalanceInput` shapes — none
recompute a sibling's own domain logic:

| Adapter | File | Reuses |
|---|---|---|
| Ledger | `ledgeradapter.go` | `accounting/ledger.JournalEntry`/`ChartOfAccounts` — converts one account's posted-entry lines into `BookItem`s, correctly orienting debit/credit by the account's own `NormalBalance` (never hard-coded cash-asset semantics) |
| AR | `aradapter.go` | `accounting/ar.Result` — subledger ending balance, GL control balance, and open-receivable-to-`BookItem` conversion; never recomputes aging |
| AP | `apadapter.go` | `accounting/ap.Result` — mirrors AR |
| Inventory | `inventoryadapter.go` | `accounting/inventory.Result.Reconciliation` — per-component subledger/GL balances; never recomputes turnover/aging |
| Labor | `laboradapter.go` | `accounting/labor.GLPayrollControl` (itself usually built via `labor.GLControlFromLedgerBalances`) — payroll-clearing balances; never calculates payroll |
| Close quality | `closequalityadapter.go` | Converts a `Result` into `accounting/closequality.ReconciliationStatus`, preserving status/as-of-date/difference/unresolved-count/source-ref |

### Close-quality integration

`accounting/closequality` already defined `ReconciliationStatus` as a
lightweight status carrier (documented from that package's own
inception as deferred to this later package). `CloseQualityReconciliationStatus`
is the promised adapter. Two integration tests
(`fixtures/closequalityadapter_test.go`) prove the exact requirement:
feeding a real `RECONCILED` result for a critical, required account into
a real `closequality.Calculate` call produces no blocking finding, while
feeding a real `UNRECONCILED` result for the same account produces
`Status: NOT_READY` with `FindingUnreconciledCriticalAccount` among the
blockers.

## Fixtures

`accounting/reconciliation/fixtures` provides four worked scenarios,
each proven against real behavior (not hand-guessed arithmetic — several
fixture-design mistakes were caught and fixed exactly by running them
and comparing computed output against intent, see the completion report):

- **Bank** (`BankInput`) — a real `accounting/ledger` ServiceBusiness
  Cash-account ledger converted via `BookItemsFromLedger`, reconciled
  against normalized bank-statement items: two exact matches, an
  outstanding check, a deposit in transit, an unrecorded fee, a repeated
  ambiguous $25 pair, and a stale $12 ATM fee — resolves to
  `UNRECONCILED` with a small, genuinely-unexplained $62 residual (the
  two ambiguous/stale items), which is realistic, not artificially
  forced to zero.
- **Credit card** (`CreditCardInput`) — `REVERSED` orientation,
  purchases, a payment, a statement annual fee, a repeated $60 pair left
  ambiguous, and a caller-confirmed manual match that ties exactly once
  orientation is applied.
- **Loan** (`LoanInput`) — opening balance, a principal payment, an
  unrecorded interest/fee charge, `DeriveBalances` enabled: both sides'
  rollforwards check out cleanly, but the cross-side difference is a
  clean, genuine $150 mismatch — no amortization schedule is assumed or
  computed anywhere.
- **Generic balance-only** — inventory control (exact tie), payroll
  clearing (small genuine difference), intercompany (`REVERSED`
  orientation, exercising the balance-orientation fix above).

Plus six adapter-integration test files using **real sibling package
fixtures/results** (`ar/fixtures.GLSubledgerMismatch`,
`ap/fixtures.GLSubledgerMismatch`,
`inventory/fixtures.GLReconciliationExact`/`Mismatch`, a real
`labor.GLControlFromLedgerBalances` call, and the closequality
integration above) — never a recalculated stand-in for a sibling's own
formula.

## Determinism, immutability, concurrency

Every exported function is pure: no I/O, no mutation of caller-owned
input, no package-global mutable state, and **no `time.Now()` call
anywhere** — every date this package reasons about is supplied
explicitly. `Calculate` is safe to call concurrently and repeatedly
against identical input and always returns byte-for-byte identical JSON
(`determinism_test.go`, `concurrency_test.go`, `immutability_test.go`,
race-clean under `go test -race`).

## Performance and scaling

`matchStage` indexes **both** sides once, before any matching stage
runs (`itemIndex` in `matcher.go`, keyed by normalized reference and by
amount bucket), rather than scanning the full opposite population for
every item. **This was not the original design** — the first
implementation re-scanned (and re-normalized every external reference
from scratch) for every book item, which profiling caught as a genuine
O(book × external) hotspot: at 10,000/10,000 scale, runtime measured
~12.1 seconds before the fix and ~54 milliseconds after — roughly a
225× improvement, now clearly sub-linear-to-linear rather than
quadratic. `TestScaling_MatcherIsSubQuadratic` locks this permanently as
a fast (~30ms) CI-safe regression test; `benchmark_test.go`'s
`BenchmarkCalculate_ScalingLinearity` reproduces the full 1k→5k→10k sweep
for manual inspection. Composite matching stays independently bounded by
its own explicit combination budget (`MaxCompositeSearchCombinations`),
confirmed via `TestComposite_SearchStaysBoundedUnderCombinationExplosion`.

A heavier 100,000/100,000 stress benchmark exists but is gated behind
`RECONCILIATION_FULL_SCALE_BENCH=1` (never run by default) — this
repository's established laptop-safety convention after a prior prompt's
stacked-concurrent-benchmark incident. Run any heavy benchmark alone,
after confirming (via `ps`) that no previous benchmark process is still
running.

## Issue and finding taxonomy

`IssueCode` (input/config/engine problems) and `FindingCode`
(reconciliation conditions needing review) are each a **closed,
fixed set** — only the codes actually emitted are defined; see `issues.go`
and `findings.go` for the complete lists.

## Neutral-language safeguard

No message, `Finding`, or `Issue` text anywhere in this package uses
fraud/theft/suspicion/intent language — locked permanently by
`neutrallanguage_test.go`, which scans every message produced by a
deliberately messy `Calculate` call exercising most code paths at once.

## JSON, versioning, matching version

Every public type uses explicit snake_case JSON tags; `Input` and
`Result` both round-trip through JSON byte-for-byte
(`roundtrip_test.go`). Three independent version constants are echoed on
every `Result.Versions`:

- **`SchemaVersion`** — the shape of every public type.
- **`FormulaVersion`** — the reconciliation-equation and rollforward
  arithmetic, and the status decision table.
- **`MatchingVersion`** — the matching-precedence order and
  composite-search behavior specifically, kept separate from
  `FormulaVersion` because a persisted auto-match decision may need to
  evolve (a new precedence rule, a different composite bound)
  independently of the equation/status arithmetic itself.

## Explicit non-goals

This package does not implement, and will not grow to implement:

- Bank APIs, feed ingestion, OFX/CSV parsing.
- OCR.
- Journal entry creation/posting, adjustment generation, bank-fee
  journal generation.
- Payment execution.
- Workflow/approvals, user assignments.
- Fuzzy/NLP matching, AI/LLM matching.
- Fraud detection, audit opinions.
- Unconstrained subset-sum search (composite matching is always bounded).
- Automatic many-to-many matching (only reachable via a `ConfirmedMatch`).
- FX conversion (V1 assumes one reporting currency, or values already
  converted by the caller before this package sees them).

## Known limitations

- **Ambiguous candidates are per-stage-local**, not reconstructed as a
  single global "best ambiguity graph" across all stages — an item that
  is ambiguous at stage 1 does not currently get re-examined for a
  *different* ambiguity shape at stage 2 if it happened to also be a
  candidate there; in practice this rarely matters since stage
  precedence already narrows aggressively, but it is a real V1
  simplification, not an oversight to silently paper over.
- **No FX conversion** (by design — see non-goals above); a mixed-
  currency population is flagged via `IssueMixedCurrency` and those
  pairs excluded from matching, never coerced.
- **Description matching is exact-normalized only** — no similarity
  scoring, by design (see non-goals above).
