# Journal entry diagnostics (`accounting/journaldiagnostics`)

`accounting/journaldiagnostics` implements deterministic journal-entry
diagnostics for accountant/controller review: unusual patterns,
control-review indicators, duplicate-like activity, and period-end/
post-close/timing behavior, mined from an [`accounting/ledger`](LEDGER.md)
`Ledger`.

Like every other package in this repository, it contains **no**
persistence, HTTP/API, auth, UI, background jobs, QuickBooks/Xero
integration, AI/LLM, or tax logic — see [What this project intentionally
does not contain](../README.md#what-this-project-intentionally-does-not-contain)
in the main README. Every exported function is pure (no I/O, no mutation of
caller-owned input, no package-global mutable state) and deterministic —
see [`safety_test.go`](../accounting/journaldiagnostics/safety_test.go).

## Purpose and non-fraud boundary

**This is not a fraud-detection engine.** It identifies deterministic,
explainable patterns worth an accountant or controller's attention — it
never states or implies fraud, theft, embezzlement, intentional
manipulation, or misconduct.

Every generated message uses neutral review language only: *anomaly*,
*unusual pattern*, *unusual activity*, *review recommended*,
*control-review indicator*. `safety_test.go`'s
`TestSafety_NoFraudLanguageInFindingMessages` and
`TestSafety_NoFraudLanguageInIssueMessages` scan every message this package
can generate (fixed templates and dynamically-built ones) for prohibited
terms (`fraud`, `fraudulent`, `theft`, `embezzlement`, `stolen`,
`manipulation`, `misconduct`) as a permanent regression guard — this is a
domain/product requirement, not merely a style preference.

`Evidence.ExternalLabel` is the one field reserved for a caller's own
external classification (e.g. from the caller's own fraud/risk system),
carried through as opaque external data if the caller chooses to populate
it in its own downstream presentation layer. This package's own `Calculate`
never sets it and never reads or interprets it.

`Severity` (`INFO`/`WARNING`/`HIGH`) is **review priority, not probability
of wrongdoing** — a `HIGH` finding means "look at this soon," never "this
is likely fraudulent."

This package also produces **no composite score**: no fraud probability, no
fraud score, no misconduct score, and no single deterministic
review-priority number either. It returns `Finding`s with `Severity`, typed
`Evidence`, and summary counts; the calling application decides
presentation and prioritization.

## Ledger integration

This package works directly from `ledger.Ledger` (`ledger.Account` +
`ledger.JournalEntry`) — it does not define a second journal-entry
accounting model. It reuses:

- `ledger.ValidateEntries` for structural validation (see [Input
  validation](#input-validation)),
- `ledger.NormalBalance` for opposite-normal-balance detection,
- `ledger.Reversal`/`ledger.EntryStatus` for reversal-aware analysis — a
  reversal is **never** inferred from equal-and-opposite amounts, only from
  an explicit `Reversal` relationship.

Nothing in this package mutates `Ledger`, any `Account`, or any
`JournalEntry` it is given.

## Optional metadata

`ledger.JournalEntry` intentionally does not carry every audit/control
field. Rather than pollute `accounting/ledger` to satisfy this package,
diagnostics accept optional `EntryMetadata`, supplied separately and joined
by `EntryID`:

```go
type EntryMetadata struct {
    EntryID     string
    Source      EntrySource // MANUAL, IMPORT, SYSTEM, RECURRING, ADJUSTING, CLOSING, UNKNOWN
    CreatedAt   *time.Time
    PostedAt    *time.Time
    PreparerID  string // opaque, never a real name
    ApproverID  string // opaque
    BatchID     string
    ExternalRef string
}
```

Every field is independently optional. When a diagnostic depends on a field
no entry supplied, that diagnostic reports itself `UNAVAILABLE` (see [Rule
availability](#rule-availability)) — **never false**, and never silently
omitted without being visible in `Result.Coverage` and
`Result.RuleAvailability`.

`PostedAt` is preferred over `CreatedAt` wherever the choice matters
(post-close, business-hours), since *posting* — not drafting — is the
control-relevant event.

## Explicit analysis period

Callers must supply an explicit `PeriodWindow`:

```go
type PeriodWindow struct {
    Period    string
    StartDate string // "YYYY-MM-DD", required
    EndDate   string // "YYYY-MM-DD", required, >= StartDate
    CloseDate *string // optional; only set when the caller actually knows one
}
```

This package **never** guesses a fiscal year, month boundary, close date,
or reporting period, and **never calls `time.Now()`** — every diagnostic
that needs "today" or "now" instead reads it from `PeriodWindow` or
`EntryMetadata`. An empty/backwards window produces `IssueInvalidPeriod`
and most time-based diagnostics become unavailable; a missing `CloseDate`
leaves the post-close diagnostic unavailable rather than inferring one.

## Input validation

Before diagnostics run, `Calculate`:

1. Filters `Ledger.Entries` to the posted-status scope (see [Posted-entry
   scope](#posted-entry-scope)).
2. Runs `ledger.ValidateEntries` against that in-scope set.
3. **Excludes** any entry `ledger.ValidateEntries` flagged with a
   `SeverityError` issue from statistical diagnostics — an unbalanced
   entry, a reference to an unknown account, a non-finite amount, an
   invalid debit/credit shape, a duplicate entry ID, etc. never contaminates
   a baseline or a finding.
4. Surfaces the exclusion via `Result.PopulationSummary.ExcludedEntries`, a
   summary `IssueLedgerValidationFailed` `Issue`, and the full raw
   `ledger.Issue` detail in `Result.LedgerIssues`.

This is the repository-recommended policy stated in the task itself:
*exclude materially invalid entries from statistical diagnostics, but
surface their validation issues.* This package never re-implements
`ledger.ValidateEntries`' own formulas.

## Posted-entry scope

By default, diagnostics analyze `ledger.PostedStatuses()` — `POSTED` and
`REVERSED` — mirroring `ledger.BalanceOptions.IncludeStatuses`' identical
default. `DRAFT` and `VOIDED` entries are excluded unless a caller
explicitly opts in via `Policy.IncludeStatuses`. `VOIDED` is **never**
included, even when explicitly listed, matching
`ledger.BalanceOptions.includeStatuses`'s identical safety rule.

A `StatusReversed` entry is **not** double-counted against its own
reversal by any rule that is reversal-aware (opposite-normal-balance
movement, rapid/cross-period reversal) — see [Reversal
diagnostics](#reversal-diagnostics).

## Entry magnitude convention

One documented convention, used consistently by every amount-based rule:

```go
func EntryMagnitude(e ledger.JournalEntry) float64 { return e.TotalDebits() }
```

**Total debits**, not `debits + credits` — for a balanced entry the two are
equal, so this is simply "the entry's size." Using the doubled sum would
silently overstate every threshold comparison in this package.

## Coverage reporting

`Result.Coverage` reports, over the *analyzed* population (post-exclusion),
what fraction of entries actually had each optional field:
`PercentWithSource`, `PercentWithCreatedAt`, `PercentWithPostedAt`,
`PercentWithPreparerID`, `PercentWithApproverID`. A caller sees *why* a
rule came back empty (0% coverage) rather than mistaking a data gap for a
clean population.

## Rule availability

Every diagnostic rule reports a `RuleState`:

| State | Meaning |
|---|---|
| `AVAILABLE` | Ran with sufficient data/configuration. |
| `UNAVAILABLE` | Required data or configuration was absent (no timestamps, no `CloseDate`, insufficient baseline for every account, no `ApprovalThreshold`, …). |
| `DISABLED` | Deliberately not opted into (e.g. `Policy.RequiredReferenceFields` left empty) — a policy choice, not a data gap. |

`Result.RuleAvailability` carries one field per rule family. An empty
`Findings` list alone is never reassurance — a caller must check
`RuleAvailability` first.

## Diagnostic rules

Each rule below states: what it detects, what it needs, its `FindingCode`,
and its default availability/disabled condition.

### Manual entries

`FindingMaterialManualEntry` — a manual entry (`EntryMetadata.Source ==
MANUAL`) whose `EntryMagnitude` meets `Policy.MaterialAmount`. Not every
manual entry is flagged by default; this is the materiality-gated
combination. `UNAVAILABLE` if no entry has `Source` metadata at all;
`DISABLED` if `MaterialAmount` is unset.

### Period-end activity

`Result.PeriodEndSummary` reports period-end entry count, amount, and
percent of total activity within `Policy.PeriodEndDays` (default 3) of
`PeriodWindow.EndDate`. `FindingMaterialPeriodEndEntry` fires for a
period-end entry meeting `Policy.MaterialAmount`. Never claims a period-end
entry is improper — pure timing observation.

### Post-close activity

`FindingPostCloseEntry` fires when an entry's posted timestamp
(`EntryMetadata.PostedAt`, or `CreatedAt` if `PostedAt` is absent) falls
after `PeriodWindow.CloseDate`, but the entry's own effective `Date` falls
within the closed period (`<= PeriodWindow.EndDate`). `HIGH` severity.
`UNAVAILABLE` without a `CloseDate` or without any posted timestamp — this
package never guesses a close date.

### Weekend activity

`FindingWeekendEntry` fires for an entry whose effective timestamp
(`PostedAt` preferred, else `CreatedAt`) falls on a day not in
`Policy.WorkingDays` (default Monday–Friday). `INFO` severity alone — a
weekend entry by itself is a weak signal; combine with materiality or
account-sensitivity policy for a stronger one. A date-only
`JournalEntry.Date` (no `EntryMetadata` timestamp) is **never** treated as
a midnight posting. `UNAVAILABLE` with no metadata timestamps at all.

### Outside-business-hours activity

`FindingOutsideBusinessHours` fires only when `Policy.BusinessHours` is
fully configured (`StartHour`, `EndHour`, and an IANA `TimeZone` loadable
via `time.LoadLocation`) — this package never infers a timezone.
`UNAVAILABLE` without full configuration, an unloadable `TimeZone`, or no
usable timestamps. Like weekend detection, a date-only entry is never
assumed to post at midnight.

### Round-dollar entries

`FindingRoundDollarEntry` fires when `EntryMagnitude >=
Policy.RoundDollarMinAmount` and the amount is evenly divisible (within a
small floating-point tolerance) by the largest of `Policy.RoundDollarBases`
(default `{100, 1000, 10000}`) it matches. `DISABLED` (not
"flag-everything") when `RoundDollarMinAmount` is 0.

### Large-entry detection

Two independent rules, both under `FindingLargeEntry` /
`FindingAccountRelativeLargeEntry`:

- **Absolute**: `EntryMagnitude > Policy.LargeEntryAbsoluteThreshold`.
  `DISABLED` when the threshold is 0.
- **Account-relative (median + K·MAD)**: for each account with at least
  `Policy.MinBaselineObservations` (default 5) historical entries in the
  analyzed population, computes that account's median magnitude and median
  absolute deviation (MAD), scaled by the standard consistency constant
  `1.4826` (so `Policy.MADMultiplier`, default `3.5`, reads on roughly the
  same scale a stddev-based multiplier would). An entry exceeding `median +
  MADMultiplier * MAD * 1.4826` for a touched account is flagged **against
  that account specifically** — a single entry touching several accounts
  can be flagged against one, several, or none of them independently. An
  account below the baseline minimum is reported via
  `IssueInsufficientBaseline`, never silently skipped without a trace.

**Why median+MAD**: deterministic, robust to outliers (unlike mean/stddev,
which the outliers themselves distort), and requires no machine learning.

The baseline is computed once over the whole analyzed population (a batch
statistic), not recomputed per-entry excluding that entry — the same
tradeoff `analytics/benchmarks` and `analytics/anomalies` already make for
their own peer-comparison baselines, and the only practical option working
from a supplied journal-entry slice rather than a live running ledger feed.
An account's first-ever activity (baseline `n = 0`) is **never** flagged as
an outlier by this rule — see [Rare and new
accounts](#rare-and-new-accounts) instead.

### Rare and new accounts

Two separate codes, kept independent as the task requires:

- `FindingNewAccountActivity` — a material entry (`Policy.MaterialAmount`)
  touching an account with **zero** other historical entries in the
  analyzed population.
- `FindingRareAccountActivity` — a material entry touching an account with
  1 to `Policy.RareAccountMaxHistoricalEntries` (default 2) other
  historical entries.

Both `DISABLED` without `Policy.MaterialAmount`.

### Opposite-normal-balance movements

`FindingOppositeNormalBalanceMovement` — a material line moving an account
against `ledger.NormalBalance(account.Type)` (e.g. a revenue account
debited, an expense account credited). This can be entirely legitimate
(returns, corrections, reclasses) — **review indicator only**. An entry
that is itself an explicit `ledger.Reversal` is excluded, since a reversal
moving against normal balance is expected by construction.

### Manual revenue/equity entries

`FindingManualRevenueEntry` / `FindingManualEquityEntry` — a manual entry
touching a `REVENUE` or `EQUITY` account, as two independent codes so a
caller can adopt one policy without the other. Never hard-coded as
inherently wrong. `UNAVAILABLE` without any `Source` metadata.

### Sensitive-account policy

`FindingSensitiveAccountEntry` — an entry touching any account in a
caller-supplied `Policy.SensitiveAccounts` (`AccountReviewPolicy{Label,
AccountIDs}`) set (e.g. cash, clearing, suspense, related-party,
management-override accounts). **Never inferred from an account's `Name`
or `Number`** — only explicit caller policy triggers it. `DISABLED` when no
policy is supplied, regardless of how an account happens to be named.

### Exact and possible duplicates

`entrySignature` builds a deterministic economic-content signature per
entry: effective date + normalized, sorted `(account, debit, credit)`
lines. **`EntryID` is never part of the signature.**

- `FindingExactDuplicateEntry` — two or more entries share the full
  signature (same date, same content). Grouped by a map keyed on
  signature, not pairwise comparison, so this stays linear in entry count
  (see [Benchmarks](#benchmarks)).
- `FindingPossibleDuplicateEntry` — two or more entries share only the
  content portion of the signature (date excluded) and fall within
  `Policy.DuplicateWindowDays` (default 3) of each other. An entry already
  claimed by an exact-duplicate group is never also reported as a possible
  duplicate — the same pair never matches both codes.

`Result.DuplicateGroups` lists every group (exact and possible) with its
signature, member `EntryIDs`, date range, and total amount, sorted by
(earliest date, first entry ID).

### Repeated identical amounts

`FindingRepeatedIdenticalAmount` — `Policy.MinRepeatedAmountCount` (default
3) or more **distinct** entries sharing the same `EntryMagnitude` (>=
`Policy.RepeatedAmountMinAmount`). An entry whose `Source == RECURRING` is
excluded, so a legitimate recurring journal (rent, a subscription) isn't
conflated with suspicious duplication — a caller wanting a full recurring
inventory instead reads `Result.SourceSummary`.

### Reversal diagnostics

`Result.ReversalSummary` always reports `Available: true` (reversal
detection needs no optional metadata, only `ledger.Reversal`, which is
always structurally present on `JournalEntry`) and lists every explicit
original/reversal `ReversalPair` found in the analyzed population, with
`DaysBetween`, `CrossPeriod`, and `Rapid` flags.

- `FindingRapidReversal` — the reversal occurs within
  `Policy.RapidReversalDays` (default 3) of the original.
- `FindingCrossPeriodReversal` — the original and its reversal fall in
  different `Period` values.
- `FindingPeriodEndEntryWithEarlyReversal` — a material original entry
  posted near period end, explicitly reversed within
  `Policy.RapidReversalDays` of that same period-end proximity. `HIGH`
  severity; **no motive inferred**.

A reversal relationship is **never inferred from equal-and-opposite
amounts** — only from an explicit `ledger.Reversal.ReversalOfEntryID` /
`ReversedByEntryID` pair that agrees on both ends (a one-sided or
inconsistent claim is already flagged separately by
`ledger.ValidateEntries`' `IssueInvalidReversal`).

### Threshold and split-entry clustering

Both require `Policy.ApprovalThreshold` — `UNAVAILABLE` without it; this
package never invents a business-specific approval limit.

- `FindingThresholdCluster` — `Policy.ThresholdClusterMinCount` (default 2)
  or more entries, on the same date, whose magnitude falls in
  `[LowerPercent, 1.0) * ApprovalThreshold` (default lower bound 90%).
- `FindingSplitEntryCluster` — multiple same-date, same-account-pattern
  entries (further narrowed to a single preparer when
  `EntryMetadata.PreparerID` is available for every candidate), each
  individually below `ApprovalThreshold`, whose **combined** magnitude
  meets or exceeds it. `HIGH` severity. Deliberately conservative — no
  fuzzy account/description matching.

Neither code implies circumvention; both are neutral, review-oriented
labels.

### Description and reference controls

- `FindingBlankDescription` — always available (no metadata needed): an
  entry with an empty/whitespace-only `Description`.
- `FindingGenericDescription` — an entry's normalized (lowercased,
  trimmed) `Description` exactly matches one of caller-supplied
  `Policy.GenericDescriptions`. **Exact matching only** — no broad
  built-in text heuristic. `DISABLED` without a caller list.
- `FindingMissingReference` — a material entry missing every one of
  caller-opted-into `Policy.RequiredReferenceFields` (`"reference"`,
  `"external_reference"`, `"external_ref"`, `"batch_id"`). `DISABLED`
  without opt-in.

### Preparer/approver diagnostics

Opaque `PreparerID`/`ApproverID` values, exactly as caller-supplied — this
package never identifies, resolves, or profiles a real person.

- `FindingSamePreparerApprover` — `PreparerID == ApproverID` on one entry.
- `FindingMissingApprover` — `PreparerID` set, `ApproverID` empty.
- `FindingHighVolumeByPreparer` — a preparer's entry count in the analysis
  period meets `Policy.HighVolumePreparerCount` (0 disables this specific
  check).

`UNAVAILABLE` without any `PreparerID` metadata at all.

### Source-system mix

`Result.SourceSummary` reports count/amount/percentage per `EntrySource`
(`MANUAL`, `IMPORT`, `SYSTEM`, `RECURRING`, `ADJUSTING`, `CLOSING`,
`UNKNOWN`), plus a separate `UnknownSourceCount`/`Amount` bucket for
entries with **no metadata at all** (distinct from the explicit
`SourceUnknown` value a caller can assert). Useful even with zero findings.

### Account-combination frequency

`FindingRareAccountCombination` — a material entry whose account
combination (sorted debit account IDs + sorted credit account IDs) occurs
at most `Policy.RareAccountMaxHistoricalEntries` times across the analyzed
population, which itself must have at least `Policy.MinBaselineObservations`
entries before this rule considers itself available (a small population
makes every combination look rare by construction). `DISABLED` without
`MaterialAmount`.

### Cross-period comparison

`CalculateWithPrior(ledger, metadata, window, policy, priorResult) Result`
additionally populates `Result.CrossPeriodComparison`: raw count/amount
deltas (manual entries, period-end activity, reversals, duplicate groups,
round-dollar/after-hours/unusual-account finding counts) against a
caller-supplied prior-period `Result`. **Never fabricates a trend
conclusion** — only the raw deltas; interpretation is left to the caller.
Plain `Calculate` (no prior) leaves this unavailable.

## Finding model and evidence

```go
type Finding struct {
    Code       FindingCode
    Severity   Severity // INFO, WARNING, HIGH — review priority, not probability
    Period     string
    EntryIDs   []string // sorted; 1 for a single-entry finding, 2+ for a group
    AccountIDs []string // sorted; may be empty
    Amount     Value
    Evidence   Evidence
    Message    string   // fixed template — never parse this
}
```

`Message` is a fixed template per `Code` (see `messages.go`) — **machine
consumers rely on `Code` and `Evidence`, never on parsing `Message`**.
`Evidence` is one flat struct (mirroring `accounting/ap.Flag`'s
convention) whose sub-fields are populated only for the codes documented
against each field (e.g. `HistoricalMedian`/`MAD`/`ObservationCount` only
for `FindingAccountRelativeLargeEntry`).

## Issue taxonomy

`Issue` covers *analysis/input problems*, distinct from `Finding` (a
journal-entry anomaly):

| Code | Meaning |
|---|---|
| `INVALID_PERIOD` | `PeriodWindow` missing `StartDate`/`EndDate` or backwards. |
| `LEDGER_VALIDATION_FAILED` | `ledger.ValidateEntries` reported at least one error; see `Result.LedgerIssues`. |
| `DUPLICATE_METADATA` | Two `EntryMetadata` values share an `EntryID`; first occurrence used. |
| `UNKNOWN_METADATA_ENTRY` | `EntryMetadata.EntryID` does not match any ledger entry. |
| `INVALID_TIMESTAMP` | (Reserved for future timezone-specific timestamp issues.) |
| `INVALID_POLICY` | A `Policy` field is structurally invalid (negative, out-of-range percent). |
| `NON_FINITE_THRESHOLD` | A `Policy` numeric field was NaN/Inf; the dependent rule is disabled. |
| `INSUFFICIENT_BASELINE` | An account had too few historical entries for account-relative testing this run. |

## Neutral-language safeguards

See [Purpose and non-fraud boundary](#purpose-and-non-fraud-boundary)
above — enforced by a permanent regression test
(`TestSafety_NoFraudLanguageInFindingMessages`/
`TestSafety_NoFraudLanguageInIssueMessages`), not just a style convention.

## JSON and versioning

Every public type uses explicit `snake_case` JSON tags and round-trips
byte-for-byte (`TestSafety_JSONRoundTrip_*`). No `NaN`/`Inf` value is ever
serialized — non-finite `Policy` fields are sanitized to 0 (disabling the
dependent rule) before any comparison or serialization, and non-finite
entry amounts are excluded from the analyzed population entirely (via
`ledger.ValidateEntries`' own `IssueNonFiniteAmount`).

- `SchemaVersion` — the public serialized contract: the shape of
  `EntryMetadata`, `PeriodWindow`, `AccountReviewPolicy`, `Policy`,
  `Finding`, `Evidence`, `DuplicateGroup`, `Result`, and `Issue`.
- `FormulaVersion` — this package's fixed diagnostic-rule semantics and
  defaults: every finding-trigger rule, the median+MAD baseline method,
  duplicate-signature normalization, and `DefaultPolicy`'s threshold
  values.

Both are echoed on `Result.SchemaVersion`/`Result.FormulaVersion` — see the
repository README's [versioning
strategy](../README.md#versioning-strategy).

## Output ordering

Deterministic throughout, never exposing Go map iteration order:

- **Findings**: `Severity` (`HIGH`, `WARNING`, `INFO`) → `FindingCode`
  declaration order → effective date (`Evidence.EntryDate`, `""` sorts
  first) → first `EntryID`.
- **Duplicate groups**: earliest date → first `EntryID`.
- **Account summaries**: `AccountID`.
- The analyzed population itself is sorted to a fixed `(date, EntryID)` key
  before any accumulation, so float64 summation order — and therefore
  every downstream total — is independent of caller-supplied
  `Ledger.Entries` order.

## Known limitations

- Account-relative baselines (median+MAD) and rare/new-account detection
  are **batch statistics over the supplied population**, not a
  point-in-time "before this entry" comparison against a live running
  ledger — see the [large-entry](#large-entry-detection) section.
- Weekend/business-hours detection requires an actual `EntryMetadata`
  timestamp; a date-only `JournalEntry.Date` never participates.
- Split-entry clustering intentionally does not fuzzy-match descriptions or
  near-identical (but not identical) account sets — a conservative,
  explainable rule set over a broad one.
- No cross-currency normalization: this package does not convert or
  reconcile amounts across `Account.Currency` values (mirrors
  `accounting/ledger`'s own explicit non-goal there).

## Future relationship to close-quality/account-reconciliation modules

```
accounting/ledger
      ↓
accounting/journaldiagnostics
      ↓
future bookkeeping/close-quality module
```

A future close-quality/bookkeeping-quality module is expected to aggregate
this package's `Result` alongside other close checks (account
reconciliation, statement completeness, etc.). This package does **not**
orchestrate that future module and knows nothing about it — the dependency
runs one direction only.

## Explicit non-goals

Fraud determination, audit opinion, transaction approval, journal
posting/editing, segregation-of-duties workflow, user identity management,
access-control testing, bank reconciliation, invoice/vendor/customer
matching, ML anomaly detection, AI narrative, tax diagnostics.
