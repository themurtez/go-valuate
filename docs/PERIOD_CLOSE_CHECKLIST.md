# Period close checklist / readiness (`accounting/closechecklist`)

`accounting/closechecklist` provides a deterministic period-close
checklist and close-readiness domain engine: given a reusable, versioned
close [`Template`](#templates-and-versioning) and one period's
caller-supplied task state, it answers *what close tasks are required,
complete, blocked, evidenced, reviewed, signed off, and still outstanding
before the period can be finalized*.

Like every other package in this repository, it contains **no**
persistence, HTTP/API, auth, UI, background jobs/scheduling, actual
period locking, journal posting, reconciliation matching, bank feeds,
QuickBooks/Xero integration, file storage, audit opinion, tax filing, or
AI/LLM — see [Explicit non-goals](#explicit-non-goals) and [What this
project intentionally does not
contain](../README.md#what-this-project-intentionally-does-not-contain)
in the main README. Every exported function is pure (no I/O, no mutation
of caller-owned input, no package-global mutable state, no wall-clock
reads) and deterministic — see
[`determinism_test.go`](../accounting/closechecklist/determinism_test.go),
[`immutability_test.go`](../accounting/closechecklist/immutability_test.go),
and
[`concurrency_test.go`](../accounting/closechecklist/concurrency_test.go).

## Difference from `accounting/closequality`

[`accounting/closequality`](CLOSE_QUALITY.md) asks: **are there
accounting/data/control issues that should block or warn on close?** It
mines ledger, statement, AR/AP, journal-diagnostics, and reconciliation
*status* facts into a books-quality `Result` — dimensions like ledger
integrity, trial-balance integrity, AR/AP control, and journal review.

`accounting/closechecklist` asks a different question: **what close
tasks are required, complete, blocked, evidenced, reviewed, signed off,
and still outstanding before the period can be finalized?** It never
recomputes closequality's diagnostics — it consumes a `closequality.Result`
as a set of typed, opaque [gate facts](#external-gates) through
[`CloseQualityGateFacts`](#closequality-adapter), the same way it
consumes any other sibling module's status. A `closequality.Result` of
`READY` says the books look clean; a `closechecklist` task can still be
unsatisfied because its required workpaper evidence or reviewer sign-off
is missing — see [Task readiness](#task-computed-readiness) and
[integration fixtures](#integration-fixtures).

## Difference from `accounting/reconciliation`

[`accounting/reconciliation`](ACCOUNT_RECONCILIATION.md) performs the
actual book-vs-external matching arithmetic for one account (bank,
credit card, AR/AP control, etc.) and returns a `Result` with a
`Status`. This package never matches transactions itself. A
`reconciliation.Result` is consumed through
[`ReconciliationGateFacts`](#reconciliation-adapter) as a single
pass/warning/fail/unavailable gate fact, attached to whichever checklist
task represents "complete that reconciliation" via a `GateRule`.

## Package composition

```
ledger -> reconciliation -\
                            \
statements -> journaldiagnostics -> closequality -> closechecklist
AR / AP    -/
```

`accounting/closechecklist` computes no ledger/reconciliation/statement/
AR/AP/journal-diagnostics/close-quality logic of its own. It reuses the
already-computed outputs of `closequality` and `reconciliation` (and,
optionally, `journaldiagnostics`/`ar`/`ap`/`statements` directly) as
[external gate facts](#external-gates), and evaluates its own,
independent domain: task definitions, dependencies, evidence, sign-offs,
due dates, exceptions, and readiness.

## Entry point

```go
result := closechecklist.Calculate(instance, policy)
// or, with a prior period's result for comparison:
result := closechecklist.CalculateWithPrior(instance, policy, prior)
```

`Instance` pairs one `Template` (by `TemplateID`+`Version`) with one
`Period`'s caller-supplied task states, evidence, sign-offs, exceptions,
gate facts, and applicability flags/overrides. `Calculate` never mutates
`instance`, `policy`, or (for `CalculateWithPrior`) `prior` — see
`immutability_test.go`.

## Period / state model

```go
type Period struct {
    PeriodID        string
    Label           string
    StartDate       time.Time
    EndDate         time.Time
    TargetCloseDate time.Time
    FiscalYear, FiscalPeriodNumber int  // optional, carried through only
    FiscalPeriodLabel              string
}
```

Every date is caller-supplied; this package never infers a fiscal
period from a label or calendar convention.

`PeriodState` (`OPEN`, `IN_PROGRESS`, `READY_FOR_REVIEW`,
`READY_TO_CLOSE`, `CLOSED`, `LOCKED`, `REOPENED`) is a caller-tracked
lifecycle state. The package **evaluates consistency** between
`PeriodState` and computed readiness (see [Period-state
consistency](#period-state-consistency)) — it never enforces or mutates
`PeriodState` itself.

## Templates and versioning

```go
type Template struct {
    TemplateID string
    Name       string
    Version    string // caller-assigned content-revision id, e.g. "2025.1"
    Sections   []SectionDefinition
    Tasks      []TaskDefinition
}
```

A `Template` is reusable, versioned, plain domain data — no period-
specific state. `TemplateContractVersion` (see [JSON /
versioning](#json--versioning--template-contract-version)) is a
*separate* version from `Template.Version`: the former tracks this
package's own Go shape for `Template`/`SectionDefinition`/
`TaskDefinition`; the latter is the caller's own content-revision id for
one concrete template. A persisted close `Instance` must remain
attributable to the exact `TemplateID`+`Version` used to build it —
`Calculate` never silently substitutes a different template, and
`TemplateVersionComparison` (see [Template-version
comparison](#template-version-comparison)) exists precisely because two
`Instance`s built from different `Template.Version`s are expected to
diverge.

## Sections and tasks

`SectionDefinition.SectionCode` and `TaskDefinition.TaskCode` are
caller-defined opaque codes — this package attaches **no** behavior to
any particular value. `TaskCode` is stable identity within one template
version; a task references its section by `SectionCode`.

```go
type TaskDefinition struct {
    TaskCode, SectionCode, Name, Description string
    Required, AllowSkip bool
    Applicability  ApplicabilityRule
    Dependencies   []TaskDependency
    EvidencePolicy EvidencePolicy
    ReviewPolicy   ReviewPolicy
    DueRule        DueRule
    GateRules      []GateRule
    ExceptionPolicy ExceptionPolicy
    Tags []string
}
```

## Task states

```go
type TaskStatus string // NOT_STARTED, IN_PROGRESS, COMPLETED, BLOCKED, SKIPPED, NOT_APPLICABLE

type TaskState struct {
    TaskCode string
    Status   TaskStatus
    StartedAt, CompletedAt *time.Time
    OwnerRef string // opaque
    Evidence []EvidenceRef
    SignOffs []SignOff
    Note     string
}
```

`OwnerRef` and every sign-off `ActorRef` are opaque strings — no
user-management logic. **Missing `TaskState` for an applicable template
task deterministically means `NOT_STARTED`** (`defaultTaskState`) — a
caller never has to pre-populate state for every task.

## Applicability

Every task resolves to one of three states:

```go
type Applicability string // APPLICABLE, NOT_APPLICABLE, UNDETERMINED
```

Rule types are deliberately few (no generic expression engine — task
spec section 6):

- `ALWAYS` (the zero value) — always `APPLICABLE`.
- `CALLER_FLAG` — looks up `Instance.ApplicabilityFlags[rule.FlagKey]`;
  a missing key resolves `UNDETERMINED`, never a guessed default.
- `EXTERNAL_GATE_PRESENT` — `APPLICABLE` when a named gate fact is
  present in `Instance.Gates` **at all** (any status, including `FAIL` —
  presence, not pass/fail, drives applicability), `NOT_APPLICABLE`
  otherwise.

`ApplicabilityOverride` (`Instance.ApplicabilityOverrides`) is an
explicit per-task override that wins unconditionally over the rule.

A required task that resolves `NOT_APPLICABLE` is excluded from every
completion denominator — see [Core invariants](#core-invariants).

## Dependency graph and cycle handling

```go
type DependencyType string // MUST_BE_COMPLETED, MUST_BE_RESOLVED
```

`MUST_BE_COMPLETED` requires the referenced task to be `Satisfied`.
`MUST_BE_RESOLVED` is looser: the referenced task must be `Satisfied`,
resolve `NOT_APPLICABLE`, or be a permitted `SKIPPED` — i.e. no longer
an open requirement even if not literally completed (useful e.g. for
"inventory adjustment review" depending on "inventory reconciliation,"
which legitimately resolves `NOT_APPLICABLE` for a service business).

The dependency graph is built and validated **once** per `Calculate`
call (`buildDependencyGraph`), independent of `Template.Tasks` slice
order — see [Determinism / order-independence](#determinism--order-independence).
Validation reports (as `Issue`s, never a silent fix):

- `UNKNOWN_DEPENDENCY` — references a task code not in the template.
- `SELF_DEPENDENCY` — a task depends on itself.
- `DUPLICATE_DEPENDENCY` — the same dependency listed twice.
- `DEPENDENCY_CYCLE` — a cycle, detected via an **iterative** (non-
  recursive) three-color DFS (`detectCycles`), so an adversarially deep
  or cyclic graph can never overflow the stack (task spec section 47).

**A cycle is never silently broken.** Every edge inside a cycle is
preserved as-is; every task participating in (or depending only on) a
cycle can never resolve `ReadyToStart`/`Satisfied`, and `Calculate`
still terminates and returns a valid (non-`READY_TO_CLOSE`) `Result` —
see `TestDependencies_Cycle_NeverSilentlyBroken`.

Task **evaluation order** (not just graph validation) also respects the
dependency graph: `evaluationOrder` runs an iterative topological sort
(Kahn's algorithm, using a small min-heap keyed by original template
position for deterministic tie-breaking) so a task's `Satisfied` is
always computed only after every task it depends on has itself been
fully evaluated. Any task left over (inside, or depending only on, a
cycle) is appended in original template order and still evaluated
correctly — its unresolved cyclic dependency is simply reported as
unresolved.

## Evidence requirements

```go
type EvidencePolicy struct {
    Type          EvidencePolicyType // NONE, AT_LEAST_ONE, MIN_COUNT, SPECIFIC_TYPES
    MinCount      int
    RequiredTypes []string
}
```

No file storage, no content inspection — `EvidenceRef` is a generic,
opaque pointer (`Type`, `Reference`, `Description`, `SourceRef`). A task
marked `COMPLETED` whose required evidence is missing is **not**
`Satisfied` — see [Completed-but-invalid state](#completed-but-invalid-state).

## Sign-off / review requirements

```go
type ReviewPolicy struct {
    Type                  ReviewPolicyType // NONE, PREPARER_ONLY, PREPARER_AND_REVIEWER, SPECIFIC_ROLES
    RequiredRoles         []SignOffRole    // PREPARER, REVIEWER, APPROVER, CONTROLLER
    RequireDistinctActors bool
}
```

`RequireDistinctActors` (combined with `PREPARER_AND_REVIEWER` or
`SPECIFIC_ROLES`) requires that no single opaque `ActorRef` fill two of
the required roles. A violation produces a factual
`SIGNOFF_ROLE_CONFLICT` finding — **never** a misconduct inference (see
[Neutral language](#neutral-language)).

## Due-date / evaluation-date behavior

```go
type DueRule struct {
    Type         DueRuleType // PERIOD_END, TARGET_CLOSE_DATE, EXPLICIT_DATE
    OffsetDays   int         // calendar-day arithmetic; may be negative
    ExplicitDate time.Time
}
```

`Instance.EvaluationDate` is **required** for any due-date computation
other than `UNAVAILABLE` — this package never calls `time.Now()`.
Calendar-day behavior is sufficient for V1; business-day support, if
ever added, would require caller-supplied holidays/working days.

```go
type DueStatus string
// NOT_DUE, DUE_TODAY, OVERDUE,
// COMPLETED_ON_TIME, COMPLETED_LATE, UNAVAILABLE
```

## External gate model

```go
type GateStatus string // PASS, WARNING, FAIL, UNAVAILABLE, NOT_APPLICABLE

type GateFact struct {
    GateCode     string
    Status       GateStatus
    SourceModule, SourceCode, SourceRef string
}

type GateRule struct {
    GateCode string
    Require  GateRequirement // PASS, PASS_OR_WARNING, AVAILABLE
    Required bool
}
```

This package never fetches or computes a `GateFact` — every one is
supplied by the caller, typically via one of the adapters below. **A
missing required gate is never implicitly `PASS`** — see [Core
invariants](#core-invariants).

### closequality adapter

`CloseQualityGateFacts(result closequality.Result) []GateFact` emits one
overall gate (`closequality.overall`, mapped from `Result.Status`) plus
one gate per assessed dimension the task spec calls out by example:

| closequality Dimension              | GateCode                          |
|--------------------------------------|------------------------------------|
| `LEDGER_INTEGRITY`                   | `closequality.ledger_integrity`    |
| `FINANCIAL_STATEMENT_INTEGRITY`      | `closequality.statement_integrity` |
| `AR_CONTROL`                         | `closequality.ar_control`          |
| `AP_CONTROL`                         | `closequality.ap_control`          |
| `JOURNAL_REVIEW`                     | `closequality.journal_review`      |

A dimension the caller never asked closequality to assess
(`DimensionUnassessed`) still produces a `GateFact` — with
`GateStatus = UNAVAILABLE` — rather than being silently omitted, so a
task's `GateRule` referencing it always finds a fact to evaluate. This
adapter never recomputes closequality's own dimension logic.

### reconciliation adapter

`ReconciliationGateFacts(gateCode string, result reconciliation.Result) GateFact`
converts one `reconciliation.Result` into a single gate fact.
`gateCode` (e.g. `"reconciliation.cash_main"`) is always caller-supplied
— this package never infers which account a reconciliation represents
or which accounts are "critical."

| reconciliation.Status         | GateStatus    |
|--------------------------------|---------------|
| `RECONCILED`                   | `PASS`        |
| `RECONCILED_WITH_ITEMS`        | `WARNING`     |
| `UNRECONCILED`                 | `FAIL`        |
| `INCOMPLETE` / `INVALID`       | `UNAVAILABLE` |

### Optional sibling gates

`JournalDiagnosticsGateFact`, `ARGateFact`, `APGateFact`, and
`StatementsGateFact` expose factual gates from
`journaldiagnostics`/`ar`/`ap`/`statements` **only** where those
packages' own result semantics already exist (severity/availability),
never a new calculation:

- `JournalDiagnosticsGateFact` — `FAIL` if any `HIGH`-severity
  `Finding`, else `WARNING` if any `WARNING`-severity finding, else
  `PASS`.
- `ARGateFact`/`APGateFact` — `UNAVAILABLE` if `!Available`, else `FAIL`
  if an error-severity `Issue` exists, else `PASS`.
- `StatementsGateFact` — `PASS`/`FAIL` from a caller-supplied dataset
  availability bool.

## Task computed readiness

For every applicable task, `Calculate` returns distinct concepts on
`TaskResult` (never overwriting `ReportedStatus`, the caller's own
`TaskState.Status` or the `NOT_STARTED` default):

```go
type TaskResult struct {
    ReportedStatus TaskStatus // never overwritten

    ReadyToStart, ReadyToComplete, Satisfied bool
    EvidenceStatus TaskEvidenceStatus // NOT_REQUIRED, SATISFIED, MISSING
    ReviewStatus   TaskReviewStatus   // NOT_REQUIRED, SATISFIED, MISSING, CONFLICT
    DueStatus      DueStatus

    Blockers []Blocker
    Findings []Finding
    ExceptionApplied, EffectiveBlocking bool
    DownstreamRequiredTaskCount int
}
```

- **`ReadyToStart`** = every dependency is resolved (per its
  `DependencyType`) and the task is not inside a dependency cycle.
- **`ReadyToComplete`** = `ReadyToStart` **and** every `Required`
  `GateRule` is satisfied.
- **`Satisfied`** = the caller marked the task `COMPLETED` **and**
  dependencies **and** required gates **and** required evidence **and**
  required sign-offs are all satisfied (or a valid, permitted,
  non-expired [exception](#exceptions) suppresses the unmet
  requirement's effective blocking) — see [Core
  invariants](#core-invariants) for the exact rule, including the
  `SKIPPED` case.

### Completed-but-invalid state

If the caller reports a task `COMPLETED` but required evidence, sign-
off, or gate is missing, `Calculate` returns
`Satisfied = false` and emits a
`TASK_MARKED_COMPLETE_WITH_MISSING_REQUIREMENTS` finding. This is a
core, explicitly tested requirement
(`TestCalculate_CompletedWithMissingEvidence_NotSatisfied`,
`TestIntegration_GatePassButEvidenceMissing_NotSatisfied` — the latter
proving *gate success != checklist evidence completion*).

### Skipped tasks

A required, applicable task marked `SKIPPED` does **not** satisfy close
readiness unless `TaskDefinition.AllowSkip = true` (or a valid permitted
exception applies) — otherwise it emits a blocking
`REQUIRED_TASK_SKIPPED` finding.

## Checklist readiness semantics

```go
type ChecklistReadiness string
// INVALID, NOT_STARTED, IN_PROGRESS, READY_FOR_REVIEW,
// READY_TO_CLOSE, CLOSED, CLOSED_WITH_EXCEPTIONS
```

`deriveChecklistReadiness` (`checklistreadiness.go`) applies one fixed,
documented decision table:

- **`INVALID`** — the template/instance is structurally invalid
  (`HasErrors(Issues)`).
- **`NOT_STARTED`** — no applicable task has started, and the caller has
  not marked the period `CLOSED`/`LOCKED`.
- **`IN_PROGRESS`** — required applicable tasks remain unsatisfied.
- **`READY_FOR_REVIEW`** — every required applicable "preparation" task
  (a task whose `ReviewPolicy` is not `PREPARER_AND_REVIEWER`/
  `SPECIFIC_ROLES` — i.e. does not itself represent a final multi-role
  review) is satisfied, but a required final-review/sign-off task is
  not yet satisfied.
- **`READY_TO_CLOSE`** — every required applicable task, gate, evidence
  requirement, and sign-off is satisfied and no effective blocker
  remains.
- **`CLOSED`** / **`CLOSED_WITH_EXCEPTIONS`** — only reachable when the
  caller's own `PeriodState` is `CLOSED`/`LOCKED`: `CLOSED` when
  requirements are satisfied, `CLOSED_WITH_EXCEPTIONS` when the caller
  declared the period closed/locked while an effective unresolved
  requirement remains.

`ChecklistReadiness` is a computed, read-only classification — it never
mutates `Instance.PeriodState`.

## Section and completion summaries

`SectionResult` (one per template section, in template declaration
order) and top-level `Completion` both count `NOT_APPLICABLE` tasks out
of every denominator:

```go
type Completion struct {
    ApplicableTaskCount, RequiredTaskCount int
    SatisfiedRequiredTaskCount, UnsatisfiedRequiredTaskCount int
    OptionalTaskCount, BlockedTaskCount, OverdueTaskCount int
    RequiredCompletionPercent, OverallCompletionPercent float64
}
```

A task counts toward `Satisfied*` fields only when `TaskResult.Satisfied
== true` — never from `ReportedStatus` alone.

## Coverage

Three factual, uncomposited coverage reports — **no composite score**:

```go
type EvidenceCoverage struct{ RequiredEvidenceCount, PresentEvidenceCount, MissingEvidenceCount int; EvidenceCoveragePercent float64 }
type SignOffCoverage  struct{ RequiredSignOffCount, ValidSignOffCount, MissingSignOffCount, ConflictingSignOffCount int; SignOffCoveragePercent float64 }
type GateCoverage      struct{ RequiredGateCount, PassCount, WarningCount, FailCount, UnavailableCount, NotApplicableCount int }
```

## Blocker / warning model

```go
type BlockerReason string
// DEPENDENCY_INCOMPLETE, GATE_FAILED, GATE_UNAVAILABLE, EVIDENCE_MISSING,
// SIGNOFF_MISSING, CALLER_MARKED_BLOCKED, EXCEPTION_EXPIRED

type Blocker struct {
    TaskCode string
    ReasonCode BlockerReason
    Message string
    DependencyTaskCode, GateCode, MissingEvidenceType, MissingSignOffRole string
    ExceptionApplied, EffectiveBlocking bool
}
```

Every `Blocker` is typed and explainable by `ReasonCode` alone — no
prose parsing required.

## Exception behavior

```go
type ExceptionPolicy string // NOT_ALLOWED (default), ALLOWED, ALLOWED_WITH_APPROVAL

type Exception struct {
    ExceptionID, TaskCode, ReasonCode, Description, ApprovedByRef string
    ApprovedAt time.Time
    ExpiresAt  *time.Time
}
```

An exception is **permitted** only when the owning task's
`ExceptionPolicy` allows it (`ALLOWED`, or `ALLOWED_WITH_APPROVAL` with
a non-empty `ApprovedByRef`), and **effective** only when permitted and
not expired (`ExpiresAt` before `Instance.EvaluationDate`).

**A valid exception never deletes the underlying blocker.** Instead:

- The `Blocker` (and its parent `TaskResult`) remain present.
- `ExceptionApplied = true`.
- `EffectiveBlocking = false` — the condition no longer counts toward
  `Satisfied`/readiness.

An expired exception stops suppressing blocking as of
`EvaluationDate` and instead emits an `EXPIRED_EXCEPTION` finding. An
exception recorded against a task whose `ExceptionPolicy` is
`NOT_ALLOWED` (the zero value) never applies — the blocker remains
fully effective. See `exceptions_test.go` and
`TestIntegration_Exceptions`.

## Period-state consistency

`checkPeriodConsistency` reports one factual finding when the caller's
own `PeriodState` is `CLOSED`/`LOCKED` but required applicable tasks
remain unsatisfied:
`PERIOD_MARKED_CLOSED_WITH_INCOMPLETE_REQUIRED_TASKS`. An `OPEN` period
that happens to be fully ready is **not** an error — nothing is
reported. This check never mutates `PeriodState`.

## Prior-close comparison

`CalculateWithPrior(instance, policy, prior)` additionally populates
`Result.PriorCloseComparison` — a pure comparison against a supplied
`PriorClose{Instance, Result}`, never changing the current `Result`'s
own readiness/blockers:

```go
type PriorCloseComparison struct {
    NewBlockerTaskCodes, ResolvedBlockerTaskCodes []string
    NewOverdueTaskCodes, ResolvedOverdueTaskCodes []string
    CompletionTimingChanges map[string]int
    AddedTaskCodes, RemovedTaskCodes []string
}
```

## Template-version comparison

When `prior.Instance.Template.Version != current.Template.Version`,
`Result.TemplateVersionComparison` reports `AddedTaskCodes`,
`RemovedTaskCodes`, and `ChangedTaskCodes` (tasks present in both
versions whose `Required`/`AllowSkip`/`SectionCode` differ — a
straightforward, low-effort definition-change signal). Historical task
state is **never** auto-migrated.

## Example templates

`ServiceBusinessMonthlyClose()` and `InventoryBusinessMonthlyClose()`
are ordinary example constructors — **not accounting standards**, and
hard-code no tax/industry-specific requirements. They cover bank/credit-
card reconciliation, AR/AP control reconciliation, payroll review,
accrual/prepaid review, journal review, financial-statement build/
review, controller review, and final close approval (plus, for
inventory businesses, inventory reconciliation/adjustment/aging
review).

## Issues vs findings

`Issue` (`IssueCode`) reports a **structural** problem with the
`Template`/`Instance`/`Policy` itself (e.g. `INVALID_PERIOD`,
`DEPENDENCY_CYCLE`, `DUPLICATE_TASK_STATE`). `Finding` (`FindingCode`)
reports a **valid close condition needing attention** (e.g.
`REQUIRED_TASK_INCOMPLETE`, `TASK_OVERDUE`,
`SIGNOFF_ROLE_CONFLICT`). Only codes this package actually emits are
defined — see `issues.go` and `findings.go`.

## Neutral language

Every `Blocker`/`Finding`/`Issue`/`PeriodConsistencyFinding` message is
factual — e.g. *"Task bank_reconciliation is blocked because gate
reconciliation.cash_main is FAIL."* — never a performance or misconduct
judgment ("failed to," "neglected," "poor performance," "bad close").
`neutrallanguage_test.go` asserts a banned-term list never appears in
any generated message across a scenario exercising most finding/blocker
codes at once.

## Determinism / order-independence

Recommended output ordering (never Go map iteration order):

- Sections in template order; tasks in template order.
- Dependencies by `TaskCode`.
- Blockers by `TaskCode`, then `ReasonCode`.
- Findings by severity, then `FindingCode` declaration order, then
  `TaskCode`.
- Issues by `IssueCode`, then `TaskCode`.
- Sign-offs by `Role`, then `ActorRef`, then `SignOffID`.

Readiness **semantics** are independent of `Template.Tasks` slice order
— `TestDependencies_OrderIndependence` shuffles task definitions and
asserts identical readiness/satisfaction. `Calculate` is safe for
concurrent, repeated calls against identical input and always returns
byte-for-byte identical JSON — see `determinism_test.go` and
`concurrency_test.go` (the latter run under `-race`).

## Immutability

`Calculate`/`CalculateWithPrior` never mutate `Template`, task
definitions, task states, evidence, sign-offs, exceptions, gates,
`Policy`, or `PriorClose` — asserted by marshaling before/after in
`immutability_test.go`.

## JSON / versioning / template contract version

Every public contract uses explicit `snake_case` JSON tags and
round-trips (`roundtrip_test.go`: `Result`, `Instance`, `Template`,
`Policy`).

```go
const SchemaVersion             = "1.0.0" // Instance/Template/.../Result shape
const FormulaVersion            = "1.0.0" // readiness/dependency/due-date/evidence/sign-off/gate/exception rules
const TemplateContractVersion   = "1.0.0" // Template/SectionDefinition/TaskDefinition shape specifically
```

`TemplateContractVersion` is tracked separately from `SchemaVersion`
because externally persisted `Template`s/`Instance`s may outlive this
package's own release cadence — bump it only when the `Template` Go
shape itself changes in a way that could break a historical persisted
template or instance.

## Core invariants

These are locked by dedicated tests (`fuzz_test.go`'s Fuzz invariants,
plus targeted unit tests):

1. **Required completion**: `SatisfiedRequiredTaskCount <=
   RequiredTaskCount`, always.
2. **Completion denominator**: `NOT_APPLICABLE` tasks are excluded from
   every denominator.
3. **Dependency**: an unsatisfied required dependency implies
   `ReadyToStart = false`.
4. **Evidence**: required evidence missing implies `Satisfied = false`
   unless a valid permitted exception explicitly suppresses effective
   blocking.
5. **Sign-off**: a required reviewer sign-off missing implies
   `Satisfied = false` (subject to the same exception carve-out).
6. **`READY_TO_CLOSE`**: reachable only when every required applicable
   task is satisfied, every final required gate satisfies its policy,
   and no effective blocker remains.
7. **Closed consistency**: `CLOSED`/`LOCKED` with unsatisfied
   requirements always emits an explicit `PeriodConsistencyFinding`.
8. **Missing task state**: an applicable task with no `TaskState`
   behaves as `NOT_STARTED`.
9. **Exception**: an exception never deletes the underlying blocker —
   only `EffectiveBlocking` changes.

## Integration fixtures

`accounting/closechecklist/fixtures` builds a real
`ledger -> reconciliation -> closequality -> closechecklist` chain
(reusing `accounting/closequality/fixtures`' own real
ledger/statements/AR/AP/journal-diagnostics chain, plus a small,
deliberately-clean `reconciliation.Input`) and exercises, via real
upstream package `Calculate` calls (never hand-built fake `Result`s):

- **Clean close** — everything satisfied → `READY_TO_CLOSE`
  (`TestIntegration_CleanClose_IsReadyToClose`).
- **Failed bank reconciliation** — an `UNRECONCILED` gate blocks the
  reconciliation task and every downstream task (journal review,
  statement review, controller review, final approval); never
  `READY_TO_CLOSE` (`TestIntegration_FailedBankReconciliation_BlocksDownstream`).
- **Warning gate** — `closequality` `READY_WITH_WARNINGS` stays ready
  under a `PASS_OR_WARNING` `GateRule`, but blocks under a strict `PASS`
  rule (`TestIntegration_WarningGate_*`).
- **Evidence vs. gate** — a passing reconciliation gate does not
  substitute for missing required workpaper evidence
  (`TestIntegration_GatePassButEvidenceMissing_NotSatisfied`).
- **Review** — all preparation tasks complete but required reviewer
  sign-off missing → `READY_FOR_REVIEW`, not `READY_TO_CLOSE`
  (`TestIntegration_PreparationCompleteReviewMissing_ReadyForReview`).
- **Exceptions** — a valid permitted exception, an expired exception,
  and an exception on a non-exceptable task
  (`TestIntegration_Exceptions`).

## Tests / coverage

93.8% statement coverage in `accounting/closechecklist` (the
`fixtures` subpackage is exercised through the parent package's tests,
per this repository's usual convention for fixture packages). Covers:
core readiness computation, checklist status decision table, dependency
graph/cycle handling and order-independence, evidence/sign-off/gate
policies, due-date computation, exceptions, period-state consistency,
prior-close/template-version comparison, validation/`Issue` reporting,
adapters, neutral-language, immutability, determinism, and concurrency
(`-race`-clean).

## Fuzz results

Two short local fuzz targets, both clean (no panic, no infinite loop, no
impossible `READY_TO_CLOSE`, deterministic invalid state):

- `FuzzCalculate_Template` — randomized task counts, dependency shapes
  (including unknown/self/forward-reference/cycle references), and
  status assignments. ~160,000 executions in a 20s local run, zero
  failures.
- `FuzzCalculate_EvidenceAndSignOff` — randomized evidence/sign-off
  counts and distinct-actor settings. ~310,000 executions in a 15s
  local run, zero failures.

## Benchmark / scaling results

`bottleneck.go`'s `computeDownstreamRequiredCounts` (the optional
`DownstreamRequiredTaskCount` field) was originally implemented as a
breadth-first search from every task over the reverse dependency graph
— correct, but O(V²) on a long dependency chain, since each of V
sources re-walked an O(V)-sized reachable set. A 5,000-task linear-chain
adversarial test (`TestSafety_LongDependencyChain_NoStackOverflow`)
caught this directly: **11.53s** before the fix. The function was
rewritten to compute each task's downstream set as a **bitset**
(`math/bits`), filled via one dynamic-programming pass in reverse
dependency-respecting order — every task's bitset is built by OR-ing its
direct dependents' already-computed bitsets — reducing the same test to
**0.06s** (~190x faster).

The `BenchmarkCalculate_Scaling` sweep (100 → 500 → 1,000 tasks, ~5
dependencies/evidence/sign-off/gate records per task) confirms
near-linear scaling after the fix:

| tasks | ns/op      |
|-------|------------|
| 100   | ~1,090,000 |
| 500   | ~4,656,000 |
| 1,000 | ~9,372,000 |

(100→500 is a 5x task increase for a ~4.3x time increase; 500→1,000 is a
2x task increase for a ~2.0x time increase — no quadratic blowup.)

`BenchmarkCalculate_Representative` (section 46's laptop-safe scale:
1,000 tasks, 5,000 dependencies, 2,000 evidence records, 2,000
sign-offs, 1,000 gate facts) runs in ~10ms per `Calculate` call.

`BenchmarkCalculate_FullScale` (20,000 tasks / 100,000 dependencies) is
gated behind `CLOSECHECKLIST_FULL_SCALE_BENCH=1` and was not run by
default, per the task spec's benchmark-safety guidance.

## Full verification

`gofmt -w .`, `go build ./...`, `go vet ./...`, and the full repository
test suite (`go test ./... -count=1`) all pass with zero regressions
across every existing package. `go test -race
./accounting/closechecklist/... -count=1` is clean. Nothing was staged
or committed.

## Limitations / remaining gaps

- `CompletionTimingChanges` on `PriorCloseComparison` is defined in the
  contract but not yet populated with real per-task day-delta values —
  it is always empty in this version. A future revision can wire this
  up once a concrete "timing change" definition (e.g. days-late delta
  per matched `TaskCode`) is settled.
- `DaysToClose`/`DaysLateVsTarget` in `TimingAnalytics` use only
  applicable tasks' `StartedAt`/`CompletedAt`; a checklist with no
  `StartedAt` data (a caller that only ever sets `CompletedAt`) will
  never populate `DaysToClose`.
- Business-day-aware due-date arithmetic (holidays/working days) is out
  of scope for V1, as specified — calendar-day arithmetic only.
- No generic applicability expression engine, as specified — only the
  three fixed rule types.

This is the deterministic close-checklist domain engine. **Prompt 51
(Controller / CFO Advisory Pack) has not been started.**
