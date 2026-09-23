# General ledger / trial balance (`accounting/ledger`)

`accounting/ledger` is the accounting-operations layer beneath the
existing analytics/valuation suite: a reusable, application-independent
general ledger and trial-balance domain engine. It is the first package in
this repository's `accounting/` namespace — a new top-level area,
alongside `financial/`, `analytics/`, `valuation/`, `transactions/`,
`portfolio/`, and `reporting/`.

Like every other package in this repository, it contains **no**
persistence, HTTP/API, auth, UI, background jobs, QuickBooks/Xero
integration, AI/LLM, or tax logic — see [What this project intentionally
does not contain](../README.md#what-this-project-intentionally-does-not-contain)
in the main README, which this package follows exactly. Every exported
function is pure (no I/O, no mutation of caller-owned input, no
package-global mutable state) and deterministic — see
[`determinism_test.go`](../accounting/ledger/determinism_test.go) and
[`mutation_test.go`](../accounting/ledger/mutation_test.go).

## Chart of accounts

`Account` is a portable chart-of-accounts entry: `ID`, `Number`, `Name`,
`Type`, `Subtype`, `ParentID`, `Currency`, `Active`. It is deliberately
**independent of `financial.Code`** — this package never maps an account
to the canonical valuation taxonomy. That mapping is a separate, later
concern (see [Boundary to the existing financial model](#boundary-to-the-existing-financial-model)
below).

`AccountType` is a fixed, five-value enum: `ASSET`, `LIABILITY`, `EQUITY`,
`REVENUE`, `EXPENSE`. `ChartOfAccounts` (built via `BuildChartOfAccounts`)
indexes a slice of `Account` by `ID` for O(1) lookup and exposes
deterministic iteration (`IDs()`) and hierarchy traversal (`Children`,
`Descendants`).

`ValidateAccounts` checks: empty ID, empty Name (warning), unrecognized
`Type`, duplicate IDs, missing parent accounts, and hierarchy cycles (see
[Hierarchy](#account-hierarchy) below).

## Normal balance

`NormalBalance(AccountType) (DebitCredit, bool)` is a **fixed table**, not
a heuristic — it never inspects an account's `Name` or any other field:

| Type | Normal balance |
|---|---|
| `ASSET` | DEBIT |
| `EXPENSE` | DEBIT |
| `LIABILITY` | CREDIT |
| `EQUITY` | CREDIT |
| `REVENUE` | CREDIT |

An unrecognized `AccountType` returns `("", false)`.

## Journal model

`JournalEntry` bundles an `ID`, `Date` (a plain `"YYYY-MM-DD"` string, not
parsed into `time.Time` — same plain-string philosophy as
`financial.Period`), `Period`, `Description`, `Source`, `Reference`,
`ExternalReference`, `Status`, an explicit `Reversal` relationship, and its
`Lines`.

`JournalLine` carries `AccountID`, explicit separate `Debit`/`Credit`
fields (never a single signed amount — this lets validation catch a line
that mistakenly sets both), `Memo`, optional `Dimensions`, and
`SourceRef`.

### Debit/credit and sign convention

This package uses one canonical **raw sign convention**: **debit-positive,
credit-negative**. A debit movement is positive; a credit movement is
negative. `Balance.RawBalance = OpeningNet + Movement`, where
`OpeningNet = OpeningDebit - OpeningCredit` and
`Movement = PeriodDebits - PeriodCredits`.

Separately, every balance-shaped type also exposes a **`DisplayBalance`** —
`RawBalance`, sign-flipped for natural-credit account types (`LIABILITY`,
`EQUITY`, `REVENUE`) via `NormalBalance`, so a healthy balance in an
account's normal position always displays as a positive number. Equal to
`RawBalance` for `ASSET`/`EXPENSE` accounts.

## Entry validation

`ValidateEntries(entries, chart, ValidateOptions)` checks, per entry:
entry ID presence/uniqueness, required `Date` or `Period`, minimum 2 lines
for a posted/reversed entry (a draft with fewer lines gets a warning, not
an error — it may still be under construction), known accounts, invalid
debit/credit shape (both set, negative, non-finite), debit == credit
within `ValidateOptions.Tolerance`, inactive-account posting policy
(`InactivePostingWarning`, the default, or `InactivePostingError`),
duplicate line IDs within an entry, and reversal-reference consistency
(see [Reversals](#reversals)). `Ledger.Validate` composes
`ValidateAccounts` + `ValidateEntries`; `CheckLedgerIntegrity` wraps that
in a single `IntegrityReport{Issues, HasErrors}`.

## Status and posting policy

`EntryStatus` is `DRAFT` (zero value), `POSTED`, `VOIDED`, or `REVERSED`.
`AffectsBalances(status)`/`PostedStatuses()` define the default rule:
`POSTED` and `REVERSED` entries affect balances; `DRAFT` and `VOIDED` never
do, regardless of `BalanceOptions.IncludeStatuses` (a caller cannot opt a
voided entry back in).

## Reversals

`Reversal{ReversalOfEntryID, ReversedByEntryID}` is an **explicit**
relationship a caller states on both the original and the reversing entry.
**Never inferred from equal-and-opposite amounts** — two entries that
happen to be numerically opposite but declare no `Reversal` field are
treated as two independent postings (see
`TestReversal_NeverInferredFromEqualAndOpposite`).
`ValidateEntries` cross-checks that both ends of a declared relationship
agree (`IssueInvalidReversal` if one side is missing, unknown, or does not
point back).

## Dimensions

`Dimension{Key, Value}` is a lightweight, optional, open-string tag on a
`JournalLine` or an imported TB line. Conventional keys
(`DimensionDepartment`, `DimensionLocation`, `DimensionClass`,
`DimensionProject`, `DimensionCustomer`, `DimensionVendor`) are provided as
suggestions; nothing in this package rejects an unrecognized key.

## Account balances

`CalculateBalances(chart, entries, BalanceOptions)` returns one `Balance`
per account (sorted by `AccountID`): opening debit/credit/net, period
debits/credits/movement, raw and display closing balance, and provenance
counts (`SourceEntryCount`, `SourceLineCount`).

`BalanceOptions.Range` (a `PeriodRange`) selects in-range entries;
`BalanceOptions.Openings` supplies explicit starting balances per account
(never fabricated as a journal entry). When an account has no matching
`OpeningBalance` and `Range.StartDate` is set, its opening is instead
derived by summing entries whose `Date` is strictly before `StartDate`; if
`Range.StartDate` is empty, an account with no explicit opening starts at
zero. `BalanceOptions.IncludeStatuses` defaults to `PostedStatuses()`.

## Trial balance

`BuildTrialBalance(chart, entries, opts, mode, tolerance)` combines
`CalculateBalances` with the conventional two-column (`ClosingDebit`/
`ClosingCredit`) presentation. Two modes:

- `ModePeriodActivity` — populates `PeriodDebit`/`PeriodCredit` alongside
  opening/closing.
- `ModeEndingBalances` — zeroes `PeriodDebit`/`PeriodCredit`; only opening
  and closing columns are meaningful.

`TrialBalance.Balanced` is **never forced to true**. It reflects
`|TotalDebits - TotalCredits| <= tolerance` exactly, and an out-of-balance
result is additionally flagged via `IssueUnbalancedTrialBalance` in
`TrialBalance.Issues`.

## Imported trial balance (no journal detail)

For a caller who only has a trial balance — no journal-entry detail —
`TrialBalanceInput{Period, Lines []TrialBalanceInputLine}` is a portable
input shape (`AccountID`, `Debit`/`Credit`, optional `*OpeningBalance`,
`Currency`, `Dimensions`, `SourceRef`).

`NormalizeTrialBalance(in, chart, tolerance) NormalizedTrialBalance`
validates and normalizes it: known account identity, valid debit/credit
shape, duplicate account rows, non-finite values, and currency
consistency. **This package never fabricates `JournalEntry` values** to
represent an imported TB — `NormalizedTrialBalance` has no
`[]JournalEntry` field at all. Like `BuildTrialBalance`, an imbalance is
never auto-corrected.

## Account hierarchy

`Account.ParentID` establishes parent-child relationships.
`validateHierarchy` (invoked by `ValidateAccounts`) detects:

- **`IssueMissingParentAccount`** — `ParentID` references an account not
  present in the chart.
- **`IssueAccountHierarchyCycle`** — following `ParentID` links from some
  account eventually returns to itself, including the degenerate
  self-parent case (`a.ParentID == a.ID`). Each account participating in a
  cycle is individually flagged (not just one global issue), so every
  affected account is visible to a caller scanning `Issues`.

Accounts without a `ParentID` are always valid top-level accounts.

## Rollups

`BuildRollups(chart, balances) []RollupBalance` computes, per account:

- **`DirectRawBalance`/`DirectDisplayBalance`** — the account's own
  `Balance`, from postings made straight to it.
- **`ChildRawBalance`/`ChildDisplayBalance`** — the sum of every
  descendant's *own* Direct figures (recursively, via
  `ChartOfAccounts.Descendants`), so a multi-level hierarchy is never
  double-counted regardless of depth.
- **`TotalRawBalance`/`TotalDisplayBalance`** — Direct + Child.

A parent account can itself receive postings (not just serve as a grouping
node) — `Direct`/`Child`/`Total` stay distinct specifically so that case
is representable without double-counting. See
`TestBuildRollups_DirectParentPosting` and the `MultiDepartmentChart`/
`MultiDepartmentEntries` fixture, which posts directly to a
"Total Operating Expenses" parent alongside its two department children.

## Opening balances

`OpeningBalance{AccountID, Debit, Credit, Currency, SourceRef}` is an
optional, caller-supplied starting position — used when a caller does not
have (or does not want to replay) prior journal history.
`validateOpeningBalances` checks: known account, finite values, sign
(`Debit`/`Credit` both `>= 0`, not both nonzero). An invalid
`OpeningBalance` is excluded from calculation (contributes zero), never
silently included. **Never fabricated as a journal entry.**

## Period handling

`PeriodRange{Periods, StartDate, EndDate}` selects entries by `Period`
membership and/or lexical `Date` range (works correctly for `"YYYY-MM-DD"`
dates). `PeriodRange.Valid()`/`ValidateRange` catch a `StartDate` after
`EndDate` (`IssueInvalidPeriod`) rather than silently returning an empty
result. This package reuses `financial.Period`'s plain-string philosophy
but does **not** import `financial` — it never guesses fiscal-period
boundaries; a caller wanting fiscal-year-aware filtering resolves that
upstream (e.g. via `financial/metrics.PeriodInfo`) and supplies explicit
`Periods`/dates.

## Multi-currency boundary

`Account.Currency` and `OpeningBalance.Currency`/`TrialBalanceInputLine.Currency`
are optional ISO 4217 codes. **This package never fetches or infers an FX
rate** — there is no FX-rate field or HTTP client anywhere in it.
`NormalizeTrialBalance` resolves a `BaseCurrency` (the first non-empty
currency encountered in `AccountID`/input order) and **excludes** any line
whose resolved currency disagrees from the aggregate totals, flagging it
with `IssueMixedCurrency` — never silently summing across currencies. A
caller wanting cross-currency aggregation must supply already-converted
values (or explicit rates) itself, upstream of this package — the same
boundary `analytics/consolidation` already established for its own
currency handling.

## Issue taxonomy

All structured findings use one shared `Issue{Code, Severity, Message,
Entry, Line, Account}` type and two-severity model (`SeverityError`,
`SeverityWarning`), consistent with every analytics package in this
repository. `HasErrors(issues)` is intentionally duplicated per-package
convention (see `financial/adjustments.HasErrors`'s doc comment for the
rationale) rather than shared via an interface.

| Code | Meaning |
|---|---|
| `UNBALANCED_ENTRY` | Entry's total debits != total credits within tolerance |
| `UNBALANCED_TRIAL_BALANCE` | TB (built or imported) total debits != total credits within tolerance |
| `UNKNOWN_ACCOUNT` | A line/opening balance/account reference does not resolve |
| `DUPLICATE_ACCOUNT` | Duplicate `Account.ID`, or duplicate row in an imported TB |
| `DUPLICATE_ENTRY` | Duplicate `JournalEntry.ID` |
| `DUPLICATE_LINE` | Duplicate non-empty `JournalLine.ID` within one entry |
| `INVALID_DEBIT_CREDIT` | Both Debit and Credit set (or neither, on a posted line) |
| `NEGATIVE_AMOUNT` | A Debit/Credit/opening amount is negative |
| `NON_FINITE_AMOUNT` | NaN or +/-Inf in a money-bearing field |
| `INACTIVE_ACCOUNT_POSTING` | A line posts to an `Active == false` account |
| `ACCOUNT_HIERARCHY_CYCLE` | A `ParentID` chain cycles back to its starting account |
| `MISSING_PARENT_ACCOUNT` | `ParentID` references an account not in the chart |
| `MIXED_CURRENCY` | An aggregation combined more than one currency without conversion |
| `INVALID_PERIOD` | Missing `Date`/`Period`, or `StartDate` after `EndDate` |
| `INVALID_REVERSAL` | A `Reversal` reference doesn't resolve or doesn't agree both ways |
| `TOO_FEW_LINES` | Fewer than 2 lines (error if posted/reversed, warning if draft) |
| `MISSING_ENTRY_ID` | Empty `JournalEntry.ID` |
| `INVALID_ACCOUNT` | Unrecognized `Type`, empty `Name`, or empty `ID` |
| `INVALID_OPENING_BALANCE` | Opening balance has both Debit and Credit set, or fails validation |

Only codes this package actually emits are defined — no speculative codes.

## Provenance

Every `Issue` carries opaque `Entry`/`Line`/`Account` identifiers back to
their source. `Balance.SourceEntryCount`/`SourceLineCount` trace how many
distinct entries/lines contributed. `JournalLine.SourceRef`,
`OpeningBalance.SourceRef`, and `TrialBalanceInputLine.SourceRef` are
caller-defined opaque pointers this package never interprets — the same
"opaque caller ID, never parsed" convention `financial.SourceRef` and
sibling analytics packages use.

## JSON and versioning

Every exported type has explicit `json` tags (snake_case), and every major
type round-trips through JSON byte-for-byte
(`accounting/ledger/roundtrip_test.go`), with a dedicated test
(`TestJSONRoundTrip_NoNaNInfInValidOutput`) confirming valid output never
contains `NaN`/`Infinity`/`-Infinity`.

`ledger.SchemaVersion` (currently `"1.0.0"`) identifies this package's
public contract, per the main README's [Versioning
strategy](../README.md#versioning-strategy). A separate `FormulaVersion`
is deliberately **not** defined: every calculation here (balance movement,
trial-balance totals, hierarchy rollups) is a direct, non-optional
consequence of the double-entry rules `SchemaVersion` already covers —
there is no independently-versionable formula choice the way e.g.
`analytics/debt.FormulaVersion` (amortization method) or
`analytics/qoe.ScoreVersion` (heuristic weighting) have. A future change
introducing a genuinely separate calculation semantic (an alternate rollup
strategy, an FX conversion mode) should get its own versioned constant at
that time.

## Boundary to the existing financial model

```
accounting/ledger
      ↓
Prompt 38 statement mapper/builder
      ↓
financial.FinancialDataset
```

This package models the general ledger/trial balance a bookkeeping system
produces, **before** that data is mapped to the canonical `financial.Code`
taxonomy or aggregated into `financial.NormalizedItem` values. A future
statement mapper/builder (not implemented here) is expected to consume a
`ledger.TrialBalance` (or a `[]ledger.Balance`) and produce
`financial.RawLineItem`/`financial.MappedLineItem` values by mapping each
`Account` to a `financial.Code` — the same role `ingestion/csv`,
`ingestion/xlsx`, and `ingestion/pdf` already play for their respective
source formats. **Nothing in `accounting/ledger` imports `financial`**,
and nothing in it guesses at that mapping.

## Fixtures

`accounting/ledger/fixtures` provides synthetic, hand-built scenarios
(mirroring this repository's `fixtures/synthetic` convention for the
analytics suite, but scoped to this package alone since no other package
yet depends on ledger data):

- `ServiceBusinessChart`/`ServiceBusinessEntries` — a professional-services
  business, no inventory/COGS.
- `RetailerChart`/`RetailerEntries` — a small retailer with inventory and a
  matching COGS entry.
- `OwnerOperatedChart`/`OwnerOperatedEntries` — a sole-proprietor-style
  business with an owner's-draw equity account.
- `MultiDepartmentChart`/`MultiDepartmentEntries` — a two-department
  hierarchy, including a deliberate direct posting to a rollup parent.
- `UnbalancedJournalEntries` — a deliberately unbalanced entry.
- `UnbalancedTrialBalance` — a deliberately unbalanced imported TB.
- `OpeningBalancesFixture` — a mid-year cutover opening-balance set.
- `ReversalPairEntries` — an explicit original/reversal pair.
- `MixedCurrencyChart`/`MixedCurrencyTrialBalance` — USD/EUR mixed without
  conversion.

## Known gaps before Prompt 38

- No `financial.Code` mapping — by design; deferred to the statement
  mapper/builder.
- No fiscal-period inference — a caller must supply `PeriodRange`/period
  metadata explicitly; this package will not guess a fiscal year from a
  date string.
- No FX-rate fetching or conversion — mixed-currency aggregation is
  detected and excluded, never computed.
- `BuildRollups`' `ChartOfAccounts.Descendants` is O(depth x fan-out) per
  account (not memoized across the whole rollup pass) — fine at the
  benchmarked scale (1,000 accounts, ~37ms) and left unoptimized
  intentionally per this task's anti-premature-optimization instruction;
  worth revisiting only if a real caller's chart is orders of magnitude
  larger.
