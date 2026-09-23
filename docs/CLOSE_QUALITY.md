# Bookkeeping / close quality (`accounting/closequality`)

`accounting/closequality` provides deterministic bookkeeping-quality and
period-close-readiness diagnostics for a single accounting period. It sits
above [`accounting/ledger`](LEDGER.md), [`accounting/statements`](STATEMENT_BUILDER.md),
`accounting/ar`, `accounting/ap`, and
[`accounting/journaldiagnostics`](JOURNAL_DIAGNOSTICS.md), composing their
already-computed outputs into one structured answer to five questions: are
the books structurally sound, are key balances plausible, are there
unresolved bookkeeping issues, is the period ready for controller/accountant
review, and what specifically still needs attention before close.

Like every other package in this repository, it contains **no** persistence,
HTTP/API, auth, UI, background jobs, QuickBooks/Xero integration, AI/LLM, or
tax logic — see [What this project intentionally does not
contain](../README.md#what-this-project-intentionally-does-not-contain) in
the main README. Every exported function is pure (no I/O, no mutation of
caller-owned input, no package-global mutable state, no wall-clock reads)
and deterministic — see
[`determinism_test.go`](../accounting/closequality/determinism_test.go) and
[`concurrency_test.go`](../accounting/closequality/concurrency_test.go).

## Purpose and non-audit boundary

**This is not an audit opinion and not a full account-reconciliation
engine.** It never renders an audit opinion, performs no tax calculation,
implements no fraud detection, and never enforces period locking, journal
posting, or close workflow. A dedicated account-reconciliation *matching*
engine (bank feeds, credit-card, loan reconciliation) is a separate, later
package (Prompt 49) — this package only consumes already-computed
[`ReconciliationStatus`](#reconciliation-status-boundary) values a caller
supplies. A richer Period Close Checklist / workflow model is likewise a
separate, later package (Prompt 50) — this package only consumes
already-tracked [`CloseTaskStatus`](#close-checklist) values.

Every generated `Finding`/`Issue` message uses neutral, factual language —
see [Neutral language](#neutral-language).

## Package composition

```
ledger
statements
AR
AP
journal diagnostics
        |
accounting/closequality
```

This package computes **no** AR/AP aging math, no journal-entry anomaly
detection, no financial-statement mapping, and no ledger validation of its
own. It reuses the typed outputs of its five upstream packages, translating
their existing issues/findings/reconciliations into close-quality
`Finding`s that preserve full provenance (`Finding.SourceModule`/
`Finding.SourceCode`) back to the originating package and code. See
[Provenance and evidence](#provenance-and-evidence).

## Entry point

```go
func Calculate(in Input, policy Policy) Result
func CalculateWithPrior(in Input, policy Policy, prior Result) Result
```

`CalculateWithPrior` is `Calculate` plus a pure comparison against a prior
period's `Result` (`Result.Comparison`) — the prior `Result` is never used to
change current-period calculations, only to report deltas. See
[Cross-period comparison](#cross-period-comparison).

## Required vs optional inputs

`Input` carries every upstream result this package may compose, following
`analytics/diagnostics`'s precedent (the closest existing multi-sibling
aggregator in this repository): every sibling `Result` is a plain value, not
a pointer, and each sibling's own availability signal (`ar.Result.Available`,
`ap.Result.Available`, `statements.Result`'s per-statement
`DatasetAvailability`) is what this package checks — a caller with nothing
for a module simply leaves that field at its zero value.

`ledger.Ledger` has no such `Available` concept (it is raw domain input, not
a computed `Result`), so its presence is instead signaled explicitly via
`Input.LedgerProvided`.

**All upstream modules are optional unless the caller's `Policy.Applicability`
marks them required.** A missing module that is not required never produces
a finding and never lowers `Coverage`'s denominator (see [Coverage
model](#coverage-model)); a missing module that *is* required produces
`FindingMissingRequiredInput` under `DimensionDataCompleteness`, severity
controlled by `Policy.TreatMissingRequiredInputAsBlocker`.

`Applicability` has no `LedgerRequired` toggle — a caller who wants
ledger/trial-balance checks simply supplies a ledger; an absent ledger is
`NotApplicable`, never `MISSING_REQUIRED_INPUT`.

## Period identity

`PeriodInfo` requires explicit `Period`/`StartDate`/`EndDate` strings
(`"YYYY-MM-DD"`), an optional `CloseDate`, and an optional `Lock` state
(`OPEN`/`SOFT_CLOSED`/`CLOSED`/`LOCKED`). This package never calls
`time.Now()` and never infers a fiscal period from data — an invalid or
inconsistent period (missing fields, `EndDate` before `StartDate`, an
unparseable date) produces `IssueInvalidPeriod` and the ledger/trial-balance/
account-balance-plausibility dimensions that need `Period.EndDate` are
simply evaluated against whatever was supplied (garbage in, garbage
evidenced — this package does not additionally guess a corrected period).

## Result model

```go
type Result struct {
    Period            PeriodInfo
    Status            Status
    Coverage          Coverage
    DimensionCoverage DimensionCoverage
    Dimensions        []DimensionResult
    Blockers          []Finding
    Warnings          []Finding
    Information       []Finding
    MissingInputs     []string
    UnresolvedItems   []Finding // Blockers + Warnings, concatenated
    CloseTaskSummary  CloseTaskSummary
    Comparison        *ComparisonResult // only with CalculateWithPrior
    Versions          Versions
    Issues            []Issue
}
```

`Status` is one of:

| Status | Meaning |
|---|---|
| `READY` | At least one substantive dimension was assessed, no blockers, no warnings. |
| `READY_WITH_WARNINGS` | No blockers, but at least one warning. |
| `NOT_READY` | At least one blocking finding (including a known-missing required input). |
| `UNASSESSED` | No substantive dimension could be assessed and no required input is known to be missing — there is genuinely insufficient basis for a determination. |

See [Readiness logic](#readiness-logic) for the exact decision rule.

## Dimensions

Close quality is assessed across twelve fixed dimensions. Not every
dimension is assessable for every `Input` — an unassessable dimension
reports `DimensionUnassessed` with an explicit `UnassessedReason`, never
silently omitted:

| Dimension | Source | Depends on |
|---|---|---|
| `LEDGER_INTEGRITY` | `ledger.Ledger.Validate` issues (excluding TB) | `Input.Ledger`/`LedgerValidation` |
| `TRIAL_BALANCE_INTEGRITY` | `ledger.IssueUnbalancedTrialBalance` | `Input.Ledger`/`LedgerValidation` |
| `FINANCIAL_STATEMENT_INTEGRITY` | `statements.Result` build status, mapping coverage, reconciliation | `Input.Statements` |
| `AR_CONTROL` | `ar.Result.AgingReconciliation`/`ControlAccountReconciliation` | `Input.AR` |
| `AP_CONTROL` | `ap.Result.AgingReconciliation`/`ControlAccountReconciliation` | `Input.AP` |
| `JOURNAL_REVIEW` | `journaldiagnostics.Result.Findings` (policy-dispositioned) | `Input.JournalDiagnostics` |
| `BALANCE_SHEET_QUALITY` | equity-rollforward review, prior-period rollforward mismatch | `Statements.Reconciliation` and/or `Policy.PriorPeriodBalances` |
| `ACCOUNT_BALANCE_PLAUSIBILITY` | `Policy.AccountExpectations` sign/range/clearing/stale checks | `Policy.AccountExpectations` + `Input.Ledger` |
| `RECONCILIATION_COVERAGE` | `Input.Reconciliations` vs `Policy.RequiredReconciliationAccounts` | `Policy.RequiredReconciliationAccounts` |
| `CLOSE_TASK_COMPLETION` | `Input.CloseTasks` | `Input.CloseTasks` |
| `DATA_COMPLETENESS` | `Coverage` vs `Policy.Applicability`, expected-activity/expected-entry rules | always assessable |
| `PERIOD_LOCK_POST_CLOSE_ACTIVITY` | `journaldiagnostics.FindingPostCloseEntry` | `Input.JournalDiagnostics` |

`DimensionResult` reports `Status` (`PASS`/`WARNING`/`BLOCKING`/
`UNASSESSED`), `Assessed`, the `FindingCode`s attributed to it, free-form
factual `Evidence` strings, and `UnassessedReason` when applicable.

`BALANCE_SHEET_QUALITY` and `ACCOUNT_BALANCE_PLAUSIBILITY` are
deliberately split: the former covers statement/rollforward-level checks
(equity rollforward review, opening+movement=closing rollforward
mismatches) that need no per-account caller declaration; the latter covers
only the per-account rules a caller explicitly declares via
`AccountExpectation` (sign, range, clearing, staleness) — see [Account
expectations](#account-expectations).

This package deliberately does **not** compute a composite score. Coverage
and dimension status are the only aggregate signals.

## Ledger integrity and trial-balance integrity

Both dimensions reuse `ledger.Ledger.Validate` (or a caller-supplied
`Input.LedgerValidation`, used as-is and never re-run — "reuse, don't
recalculate") rather than rebuilding ledger validation logic.
`ledger.IssueUnbalancedTrialBalance` issues are routed to
`TRIAL_BALANCE_INTEGRITY`; every other ledger `Issue` is routed to
`LEDGER_INTEGRITY`. Ledger `error`-severity issues become `BLOCKING`
findings; `warning`-severity issues become `WARNING` findings. Every
`Finding` preserves the originating `ledger.IssueCode` as `SourceCode`.

This package never fabricates a balancing adjustment.

## Financial-statement integrity

Consumes `statements.Result` directly: build/dataset availability
(`STATEMENT_BUILD_INVALID` for an `INVALID` availability or an
`UNBALANCED_BALANCE_SHEET`/`SOURCE_TRIAL_BALANCE_UNBALANCED` issue),
`Coverage.UnmappedAccounts`/`UnmappedBalanceAmount` (materiality-gated
`MATERIAL_UNMAPPED_ACCOUNTS`), and `Reconciliation` entries
(`BALANCE_SHEET_OUT_OF_BALANCE` when `!Balanced`). It never recomputes
mapping coverage or balance-sheet reconciliation — both are read directly
off `statements.Result`.

## AR / AP control quality

Both dimensions read only `AgingReconciliation` and
`ControlAccountReconciliation` off the respective `ar.Result`/`ap.Result` —
`Issues` are not re-surfaced (they are already visible in the sibling
package's own output and are input/data-quality concerns, not distinct
close-readiness conditions). An unbalanced aging reconciliation
(`AR_AGING_NOT_RECONCILED`/`AP_AGING_NOT_RECONCILED`) is always blocking. An
unreconciled control account (`AR_CONTROL_MISMATCH`/`AP_CONTROL_MISMATCH`)
is blocking when the difference is material under `Policy.Materiality`,
warning otherwise.

This package never judges overdue customers or vendors, collection
performance, or payment pressure — that is `ar`/`ap`'s own domain. This
package asks only "do the subledger and control account agree," not "should
we be worried about this balance."

## Journal review

Every `journaldiagnostics.Finding` (except `POST_CLOSE_ENTRY`, handled
separately — see below) is translated according to
`Policy.JournalFindingRules[code]`, a caller-configurable map from
`journaldiagnostics.FindingCode` string to `BLOCKING`/`WARNING`/`INFO`/
`IGNORE`. `DefaultJournalFindingRules()` supplies a recommended default for
every code (see the table in [`policy.go`](../accounting/closequality/policy.go));
a code with no explicit override falls back to the default, and a wholly
unrecognized code defaults to `INFO`. This package never hard-codes which
journal findings are blocking, and it never infers fraud or wrongdoing —
`journaldiagnostics` doesn't either.

## Post-close activity

`journaldiagnostics.FindingPostCloseEntry` findings are translated into
`PERIOD_LOCK_POST_CLOSE_ACTIVITY` findings (`POST_CLOSE_ACTIVITY`).
Severity:

- `BLOCKING` if `PeriodInfo.Lock` is `CLOSED`/`LOCKED` **and**
  `Policy.TreatLockedPostCloseAsBlocking` is true (the default).
- `WARNING` if the entry's amount is material under `Policy.Materiality`
  (or amount is unavailable).
- `INFO` otherwise.

This package never guesses whether posting was authorized — it only
reflects the caller's own declared period-lock state and materiality
policy back as a severity.

## Balance-sheet quality

Two independent, opt-in checks:

- **Equity rollforward review**: when `statements.Result.Reconciliation`
  reports an unbalanced balance sheet, this package adds a neutral
  `EQUITY_ROLLFORWARD_REVIEW` finding suggesting review of whether
  current-period earnings have been rolled into equity — a common cause of
  exactly this symptom. It never auto-posts net income or "fixes" retained
  earnings itself.
- **Period-over-period rollforward**: for each `Policy.PriorPeriodBalances`
  entry, checks `OpeningBalance + PeriodMovement == ClosingBalance` within
  `Tolerance`, producing `BALANCE_ROLLFORWARD_MISMATCH` on a tie failure.
  This reuses simple addition, not a reconciliation engine — no fuzzy
  matching, no adjustment fabrication.

## Account expectations

`Policy.AccountExpectations` is the *only* way an account's balance sign or
range is checked — **this package never infers expected sign from account
type, `ledger.NormalBalance`, or account name.** Opposite-sign balances
(an overdraft, a credit memo, a contra balance, a reclass) are frequently
legitimate; a check only fires when the caller has explicitly declared an
`AccountExpectation` for that account.

```go
type AccountExpectation struct {
    AccountID     string
    ExpectedSign  ExpectedSign // "", POSITIVE, NEGATIVE, NORMAL_ONLY
    AllowZero     bool
    AllowNegative bool
    MinBalance    *float64
    MaxBalance    *float64
    ShouldClear   bool
    MaxAgeDays    *int
    Materiality   *float64
    Label         string
}
```

Ending balances are computed via `ledger.CalculateBalances` against
`Input.Ledger`, scoped through `Period.EndDate` — unavailable without a
supplied ledger. A duplicate `AccountExpectation` for the same `AccountID`
produces `IssueInvalidAccountExpectation`; only the first is used. An
expectation referencing an account with no ledger activity produces
`IssueUnknownAccount`.

## Suspense / clearing accounts

Suspense/clearing accounts are **never** auto-detected from name. A caller
marks an account `ShouldClear: true` in its `AccountExpectation`; a nonzero
(and, if `Materiality` is set, material) ending balance produces
`CLEARING_ACCOUNT_NOT_CLEARED`.

## Stale balance

If the caller also supplies a matching `Policy.BalanceAges` entry and the
expectation declares `MaxAgeDays`, an age exceeding that threshold produces
`STALE_ACCOUNT_BALANCE`. This package never reconstructs an account's age
from its net ledger balance — staleness is unavailable without explicit
`BalanceAge` metadata.

## Reconciliation status boundary

`ReconciliationStatus` is a **status carrier**, not a matching engine —
Prompt 49 will implement actual reconciliation matching (bank feeds,
credit-card, loan). This package only reads `Status`/`Difference`/
`UnresolvedCount` etc. off caller-supplied entries.

```go
type ReconciliationState string
const (
    ReconciliationReconciled          = "RECONCILED"
    ReconciliationReconciledWithItems = "RECONCILED_WITH_ITEMS"
    ReconciliationUnreconciled        = "UNRECONCILED"
    ReconciliationNotRequired         = "NOT_REQUIRED"
    ReconciliationUnavailable         = "UNAVAILABLE"
)
```

### Coverage and critical accounts

`Policy.RequiredReconciliationAccounts` declares which accounts must have a
reconciliation status — an account not listed is never counted against
coverage (this package never assumes every account requires reconciliation).
`Policy.CriticalAccounts` (never inferred from name) upgrades an
unreconciled/uncleared required account from `WARNING` to `BLOCKING`
(`UNRECONCILED_CRITICAL_ACCOUNT`). `RECONCILED_WITH_ITEMS` with
`UnresolvedCount` exceeding `Policy.MaxAllowedUnreconciledItems` produces
`RECONCILIATION_ITEMS_OUTSTANDING`.

## Close checklist

`CloseTaskStatus` tracks status only — this package executes no tasks and
manages no assignment/workflow (`OwnerRef`/`EvidenceRef` are opaque
passthroughs; Prompt 50 will build a richer workflow model). A required task
in `BLOCKED` status always produces `REQUIRED_CLOSE_TASK_BLOCKED`
(`BLOCKING`). A required task in `NOT_STARTED`/`IN_PROGRESS` produces
`REQUIRED_CLOSE_TASK_INCOMPLETE`, `WARNING` by default or `BLOCKING` when
`Policy.CloseTaskRules.RequiredIncompleteBlocks` is true.
`Result.CloseTaskSummary` reports counts by status and `CompletionPercent`
over required tasks only.

## Expected recurring entries and expected activity

Two opt-in, entirely caller-declared checks (never inferred from account
names or history):

- `Policy.ExpectedActivityRules` — "this set of accounts should show at
  least N entries / M absolute movement this period."
- `Policy.ExpectedPeriodEntries` — "this specific recurring entry (e.g.
  monthly depreciation) should appear," matched by account membership,
  `Source`, exact normalized (trimmed, case-folded) description, and/or
  amount range. **No fuzzy/NLP matching.** A rule with no usable criteria,
  or with no ledger supplied, is unavailable and produces no finding either
  way — never treated as unmet.

Both fold into `DATA_COMPLETENESS` and produce
`EXPECTED_PERIOD_ENTRY_MISSING` on a miss.

## Applicability and missing-module behavior

```go
type Applicability struct {
    ARRequired                 bool
    APRequired                 bool
    StatementsRequired         bool
    JournalDiagnosticsRequired bool
    ReconciliationsRequired    bool
    CloseTasksRequired         bool
}
```

A missing module that is **not** required never produces a finding and is
reported under `Coverage.NotApplicableModules`, not
`Coverage.MissingModules`. A missing module that **is** required produces
`MISSING_REQUIRED_INPUT`, severity per
`Policy.TreatMissingRequiredInputAsBlocker` (default: blocking).

## Coverage model

`Coverage` (module-level) uses a 0–1 decimal `CoveragePercent`, matching
`analytics/diagnostics.Coverage`'s convention rather than
`statements.MappingCoverage`'s 0–100 scale — there is no single repo-wide
convention for this, so this package picks and documents one explicitly.
Only modules that are either required or actually supplied count toward the
denominator; a module that is both not required and not supplied is
`NotApplicable` and excluded entirely.

`DimensionCoverage` (dimension-level) mirrors the same idea one level up:
`RequiredDimensions`/`AssessedDimensions`/`PassedDimensions`/
`WarningDimensions`/`BlockingDimensions`/`UnassessedDimensions` plus a 0–1
`CoveragePercent` over assessed-vs-total dimensions.

## Readiness logic

```
1. If no *substantive* dimension was assessed (every dimension other than
   DATA_COMPLETENESS is UNASSESSED) AND no MISSING_REQUIRED_INPUT finding
   exists -> UNASSESSED.
2. Else if any BLOCKING finding exists -> NOT_READY.
3. Else if any WARNING finding exists -> READY_WITH_WARNINGS.
4. Else -> READY.
```

`DATA_COMPLETENESS` is excluded from the "substantive dimension" check in
rule 1 because it is a meta-dimension about input coverage, always
assessable — its being assessed alone (with everything else absent and
nothing required) is not a basis for calling a period `READY`. But a
`MISSING_REQUIRED_INPUT` finding it produces *is* itself a substantive,
actionable fact, so it independently satisfies having a basis for
`NOT_READY` even when nothing else could be assessed — matching this
package's documented preference: "missing required input -> NOT_READY; no
required dimensions configured / insufficient basis -> UNASSESSED."

There is no opaque score anywhere in this readiness path.

## Deduplication

The same underlying condition can, in principle, be surfaced through more
than one upstream module. `Finding`s are deduplicated by a stable key —
`(Code, Dimension, first AccountID)` — computed from the mine-function run
order (itself deterministic), so the first-seen `Finding`'s body survives
and every later duplicate's `(SourceModule, SourceCode)` reference is
folded into the survivor's `AdditionalSources` rather than discarded. If a
duplicate carries a worse `Severity` than the survivor, the survivor's
`Severity` is promoted to the worse of the two so a duplicate is never
silently hidden behind a lower-severity survivor.

This is deliberately conservative: only an *exact* key match is merged.
Findings that merely have similar-looking evidence or messages are never
merged — see
[`dedup_test.go`](../accounting/closequality/dedup_test.go) for both the
positive (same account, same code, same dimension) and negative (different
accounts) cases.

## Finding vs Issue

- **`Finding`** — a close/bookkeeping condition needing review (e.g.
  unbalanced trial balance, AR control mismatch, uncleared suspense
  account, missing required close task, post-close entry).
- **`Issue`** — a problem with the `Input`/`Policy` this package was given
  (e.g. invalid period, duplicate reconciliation status, unknown account in
  an expectation, non-finite threshold).

Every emitted code is a stable, documented constant in
[`findings.go`](../accounting/closequality/findings.go) and
[`issues.go`](../accounting/closequality/issues.go) — no ad hoc strings.

## Provenance and evidence

Every translated `Finding` preserves `SourceModule` (the exact upstream
import path, e.g. `"accounting/ar"`) and `SourceCode` (the exact upstream
`Code`, e.g. `"CONTROL_ACCOUNT_MISMATCH"`) verbatim — never flattened away.
Findings originating entirely within this package (e.g. reconciliation
coverage, close-task completion) use `SourceModule ==
"accounting/closequality"`.

Every `Finding` carries typed `Evidence` (sub-fields such as
`SubledgerBalance`/`ControlBalance`/`Difference`/`Tolerance`/
`EndingBalance`/`TaskCode`/`CloseDate`) — evidence is never hidden only in
`Message` text.

## Neutral language

Generated messages are factual: *"AR subledger differs from the supplied GL
control balance by 12,400"*, *"required close task 'bank-reconciliation' is
not completed."* This package never says "books are bad," "books are
unreliable," "fraud," "misstatement detected," or "accountant error" unless
an external, caller-supplied classification says so explicitly (which this
package never does on its own) — enforced by a permanent regression test,
[`safety_test.go`](../accounting/closequality/safety_test.go).

## JSON and determinism

Every exported type uses explicit snake_case JSON tags. `Calculate`/
`CalculateWithPrior` never produce NaN/Inf (non-finite policy inputs are
rejected as `IssueNonFiniteValue`, never propagated — see
`isNonFinite`/`math.IsNaN`/`math.IsInf`, this package's own local copy of
the repo-wide convention). Ordering is fully deterministic: `Blockers` ->
`Warnings` -> `Information`, each internally ordered by
`Dimension` -> `Code` -> first `AccountID` -> `SourceCode` — never Go map
order. See
[`determinism_test.go`](../accounting/closequality/determinism_test.go),
[`roundtrip_test.go`](../accounting/closequality/roundtrip_test.go), and
[`immutability_test.go`](../accounting/closequality/immutability_test.go).

## Versioning

- `SchemaVersion` — the shape of `Input`, `Policy`,
  `Dimension`/`DimensionResult`, `Finding`, `Evidence`, `Coverage`,
  `CloseTaskSummary`, `ComparisonResult`, `Result`, and `Issue`.
- `FormulaVersion` — every mine-function translation rule, the readiness
  decision table, deduplication keying, and `DefaultPolicy`/
  `DefaultJournalFindingRules`'s values.

Both are echoed on `Result.Versions`.

## Cross-period comparison

`CalculateWithPrior` never changes current-period calculations — it is a
pure, separate comparison against a supplied prior `Result`:
`NewBlockers`/`ResolvedBlockers`/`NewWarnings`/`ResolvedWarnings` (keyed by
the same dedup key as within-period deduplication),
`DimensionStatusChanges`, `CoverageChange`, and a `Trend`
(`IMPROVING`/`STABLE`/`DETERIORATING`/`UNAVAILABLE`) derived strictly from
concrete blocker/warning deltas — never a numeric quality score. When the
trend would be ambiguous, it reports `STABLE` rather than guessing, and a
caller can always inspect the explicit delta lists directly.

## Limitations

- No account reconciliation matching (bank, credit-card, loan) — Prompt 49.
- No close-task workflow, assignment, or execution — Prompt 50.
- No tax calculation, audit opinion, or fraud detection.
- No AI/LLM narrative layer.
- No composite close-quality score.
- Balance-sheet/account-expectation checks require a supplied
  `ledger.Ledger` — without one, `ACCOUNT_BALANCE_PLAUSIBILITY` and the
  expected-activity/expected-entry checks are unavailable, not silently
  skipped-as-passing (`DimensionUnassessed`, explicit reason).

## Relationship to Prompt 49 (account reconciliation)

This package's [`ReconciliationStatus`](#reconciliation-status-boundary)
type is deliberately shaped as a lightweight, externally-computed status
carrier so that a future `accounting/reconciliation`-style package (Prompt
49) can either populate it directly from real matching output, or this
package's `Input.Reconciliations` field can be swapped for a call into that
package's own richer result type without changing this package's coverage/
critical-account/readiness semantics.
