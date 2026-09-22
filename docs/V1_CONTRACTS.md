# V1 Contracts

This document is the primary reference for a main application integrating
`github.com/themurtez/go-valuate`. It inventories the data contracts that
cross the boundary between this library and its caller: the shape of every
input, the shape of every output, and the order stages are expected to run
in.

This is not a repeat of the [README](../README.md) — it is a narrower,
integration-focused index into it. Every contract below links back to the
README section that documents its full rationale, formulas, and edge cases.
Read this document to find out **what exists and where it fits**; read the
README section it links to for **why it works the way it does**.

See also [`INTEGRATION.md`](INTEGRATION.md) for the recommended
orchestration flow, [`DEPENDENCIES.md`](DEPENDENCIES.md) for the third-party
dependency inventory, and [`FIXTURES.md`](FIXTURES.md) for the synthetic
test-fixture catalog.

## How to read the pipeline

Every stage below is a pure function of the previous stage's output (see
the README's [Architecture](../README.md#architecture) diagram). No stage
performs I/O, no stage holds hidden state between calls, and — with the
sole exception of the two optional AI capabilities — every stage is
deterministic: the same input always produces the same output.

```
Input adapters (ingestion)
   ↓
Deterministic classification (financial/classification)
   ↓  [optional: AI fallback classification]
Review (review) — human-in-the-loop confirmation
   ↓
Normalize (financial)
   ↓
Reconciliation + Metrics (financial/reconciliation, financial/metrics)
   ↓
Adjustments (financial/adjustments)
   ↓  [optional: AI adjustment suggestions]
Maintainable Earnings (financial/earnings)
   ↓
Resolved Settings Snapshot (settings)
   ↓
Valuation Methods (valuation/sde, ebitda, capitalization, dcf, netassets)
   ↓
Applicability / Orchestration (valuation/applicability, valuation/orchestrator)
   ↓
Value Basis Conversion (valuation/basis)
   ↓
Consensus (valuation/consensus)
   ↓
Sensitivity (valuation/sensitivity)
   ↓
Report Model (valuation/report)
```

## 1. Input adapters

Three format-specific parsers converge on one shared result shape
(`ingestion.Result`) before classification ever runs. See the README's
[`ingestion`](../README.md#ingestion), [`ingestion/pdf`](../README.md#ingestionpdf),
and [Scanned/image PDF support (OCR)](../README.md#scannedimage-pdf-support-ocr)
sections for full detail.

| Function | Signature | Package |
|---|---|---|
| `csv.Parse` | `func Parse(r io.Reader, opts ingestion.Options) (*ingestion.Result, *ingestion.Error)` | `ingestion/csv` |
| `xlsx.Parse` | `func Parse(r io.Reader, opts ingestion.Options) (*ingestion.Result, *ingestion.Error)` | `ingestion/xlsx` |
| `pdf.Parse` | `func Parse(r io.Reader, opts pdf.Options) (pdf.Results, *ingestion.Error)` | `ingestion/pdf` |
| `pdf.ParseWithOCR` | `func ParseWithOCR(ctx context.Context, r io.ReadSeeker, opts pdf.Options, engine ocr.Engine) (pdf.Results, *ingestion.Error)` | `ingestion/pdf` |

**Result types.**

- `csv.Parse`/`xlsx.Parse` return a single `*ingestion.Result` (nil on
  fatal failure, alongside a non-nil `*ingestion.Error`).
- `pdf.Parse`/`pdf.ParseWithOCR` return `pdf.Results{ Statements
  []ingestion.Result, Warnings []ingestion.Warning }` — **one
  `ingestion.Result` per detected statement section**, since a single PDF
  commonly contains more than one statement (see the README's "Multiple
  statements per PDF — Option A"). A single-statement PDF is simply
  `len(Statements) == 1`.
- Every non-fatal issue is an `ingestion.Warning` (stable `WarningCode`,
  attached to a `Result`/`Results`, never aborting the parse). Every fatal
  issue is an `*ingestion.Error` (stable `ErrorCode`, returned alone with a
  nil/zero result).

**Convergence point.** `ingestion.Result.ToRawLineItems() []financial.RawLineItem`
is the one function every format's output funnels through on its way into
classification — CSV, XLSX, and PDF (text or OCR) rows are indistinguishable
to everything downstream of this call.

**Untrusted input.** Every parser treats its input as untrusted bytes:
`ingestion.Options.Limits` bounds file size, row/column/sheet/page counts,
cell text length, and (for OCR) image pixel dimensions and word counts — see
[Security / privacy integration notes](INTEGRATION.md#security--privacy)
in `INTEGRATION.md`.

## 2. Financial pipeline

```
RawLineItem
   ↓  financial/classification.Classify / ClassifyBatch
classification.Result  ──┐
                          │  (optional AI fallback — see §2a)
                          ↓
Result.ToMappedLineItem  →  MappedLineItem
   ↓  review.Build / review.Apply  (human confirmation of anything uncertain)
corrected MappedLineItem[]
   ↓  financial.Normalize
FinancialDataset
   ↓  financial/metrics.Calculate
metrics.Result / metrics.Snapshot
```

| Stage | Entry point | Input | Output |
|---|---|---|---|
| Classification | `classification.Classify` / `classification.ClassifyBatch` | `financial.RawLineItem`, `classification.Config` | `classification.Result` |
| Mapping | `classification.Result.ToMappedLineItem(raw)` | the `Result` + its source `RawLineItem` | `financial.MappedLineItem` |
| Normalization | `financial.Normalize` | `[]financial.MappedLineItem`, `financial.NormalizeOptions` | `financial.FinancialDataset`, `error` |
| Metrics | `metrics.Calculate` | `financial.FinancialDataset`, `metrics.Options` | `metrics.Result` |

**`RawLineItem` → `MappedLineItem` is never skipped.** `Normalize` performs
no classification of its own — see the README's
[`financial/classification`](../README.md#financialclassification) and
[Normalizer](../README.md#normalizer) sections. A `MappedLineItem` with an
empty `Code` and `Status != RowStatusIgnored` fails `Normalize`'s
validation (`financial.ValidationErrors`, a real Go `error`, inspectable via
`errors.As`).

**Structural rows.** `financial.RowKind` (`""`/`HEADING`/`SUBTOTAL`/`TOTAL`)
flows from `ingestion` through `classification` onto `MappedLineItem.Kind`
— see [Structural row contract](../README.md#structural-row-contract-financialrowkind).
`Normalize` still switches on `RowStatus`, not `RowKind` — `RowKind` is
upstream provenance, `RowStatus` is the normalize-time directive derived
from it.

**Signs.** Every amount in a `FinancialDataset` is a positive magnitude in
its "as reported" sense (an expense is a positive number). See
[Sign convention](../README.md#sign-convention).

**Missing vs. zero.** Every `metrics` figure is a `MetricValue{Available
bool, Value float64}` — `Available == false` means "could not be computed
from what's present," never a real zero. See
[Missing vs. zero](../README.md#missing-vs-zero).

### 2a. Optional AI fallback classification

Sits strictly inside the classification step, never replacing it — see
[AI fallback classification](../README.md#ai-fallback-classification).

```
classification.Classify (always runs first, unconditionally)
   ↓
ai.ClassifyWithFallback / ClassifyBatchWithFallback  (opt-in, Policy.Mode)
   ↓
classification.Result  (Source == SourceAI when AI's answer was used)
```

- Entry points: `ai.ClassifyWithFallback(ctx, raw, cfg, classifier, policy)`,
  `ai.ClassifyBatchWithFallback(ctx, raws, cfg, classifier, policy)` in
  `financial/classification/ai`.
- **Default is off** (`ai.AIDisabled`, the zero value of `Policy.Mode`).
- Every AI-sourced `Result` has `ReviewRequired == true` unconditionally —
  it becomes a `review.ReviewItem` like any other classification and can
  never affect a `FinancialDataset` without an explicit `review.Decision`.
- Provider abstraction: `ai.Classifier` interface; the only shipped adapter
  is `financial/classification/ai/openai.Classifier`. Nothing outside that
  adapter subpackage imports any AI SDK.

## 3. Review

The human-in-the-loop layer between classification/ingestion and
normalization/valuation. See the README's [`review`](../README.md#review)
section for full detail — this is a summary of its entry points only.

| Function | Signature |
|---|---|
| `review.Build` | `func Build(in review.BuildInput, policy review.Policy) review.Plan` |
| `review.Apply` | `func Apply(source review.Source, plan review.Plan, decisions []review.Decision) review.ApplyResult` |
| `review.EvaluateReadiness` | `func EvaluateReadiness(items []review.ReviewItem) review.Readiness` |

```go
type BuildInput struct {
    Classifications []classification.Result
    Raws            []financial.RawLineItem
    Rows            []review.RowContext
    Periods         []ingestion.DetectedPeriod
    Reconciliation  *reconciliation.Result
    Adjustments     []adjustments.Adjustment
    RequiredAdjustmentIDs map[string]bool
    Assumptions     review.AssumptionSource
    Revenue         *float64
}
```

Every `BuildInput` field is independently optional — a caller reviewing only
OCR cells supplies only `Rows`, for example.

```go
type Source struct {
    MappedLineItems []financial.MappedLineItem
    Rows            []review.RowContext
    Adjustments     []adjustments.Adjustment
}
```

`Apply` returns corrected `MappedLineItems`, `Adjustments`,
`CorrectedNumerics map[string]float64`, and `CorrectedPeriods
map[string]financial.Period` — never mutating `source`/`plan`/`decisions`.

**Readiness gate.** `EvaluateReadiness` is a pure function of `Severity` +
`Status`: `NOT_READY` iff an unresolved `BLOCKING` item exists;
`READY_WITH_WARNINGS` if only unresolved `WARNING`/`ERROR` items remain;
`READY` otherwise. See [Readiness](../README.md#readiness). **A caller
should not proceed to `financial.Normalize`/valuation while `NOT_READY`.**

**AI-sourced adjustment suggestions get the same Required-review treatment**
via `BuildInput.RequiredAdjustmentIDs` — see §4a below and
[AI adjustment suggestions](../README.md#ai-adjustment-suggestions).

## 4. Adjustments

```
confirmed Adjustment[]  (caller-constructed, or AI-suggested + review-accepted)
   ↓  financial/adjustments.Apply
adjustments.Result{ EBITDABridge, SDEBridge, Skipped, Errors }
```

| Function | Signature |
|---|---|
| `adjustments.Apply` | `func Apply(snapshot metrics.Snapshot, adjs []adjustments.Adjustment) adjustments.Result` |
| `adjustments.Validate` | `func Validate(adjs []adjustments.Adjustment, snapshot metrics.Snapshot) []adjustments.Issue` |

`Apply` never mutates its inputs and performs no I/O. Every `Adjustment`
carries a non-negative `Amount` plus an explicit `Effect`
(`EffectIncrease`/`EffectDecrease`) — never a signed delta. See
[Sign convention: magnitude + explicit direction](../README.md#sign-convention-magnitude--explicit-direction-never-a-signed-delta).

### 4a. Optional AI adjustment suggestions

A second, independent AI capability — not the same package or contract as
classification AI. See [AI adjustment suggestions](../README.md#ai-adjustment-suggestions).

```
already-confirmed FinancialDataset rows
   ↓  ai.SelectCandidates
[]ai.SourceRow  (candidate rows, amounts included — this is source-bound)
   ↓  ai.SuggestAdjustments / SuggestAdjustmentsBatch  (Suggester, opt-in)
ai.Response{ Suggestions []ai.Suggestion }
   ↓  ai.ValidateSuggestions (exact-amount-match against the SourceRow; rejects invented amounts)
[]ai.Suggestion (Valid)
   ↓  ai.ToAdjustments  (always Included == false)
[]adjustments.Adjustment  +  ai.RequiredAdjustmentIDs(adjs)
   ↓  review.BuildInput.Adjustments / .RequiredAdjustmentIDs
review.Plan  (KindAdjustment, Required == true)
   ↓  human ACCEPT/OVERRIDE/IGNORE decision
review.Apply
   ↓
adjustments.Adjustment{ Included: true, ... }
   ↓  financial/adjustments.Apply  (deterministic engine — unchanged)
adjustments.Result
```

- Package: `financial/adjustments/ai` (+ `financial/adjustments/ai/openai`
  adapter) — a **distinct** `Suggester`/`Request`/`Response` contract from
  `financial/classification/ai`'s `Classifier`/`Request`/`Response`, because
  this one must include dollar amounts (source-bound suggestions) while
  classification AI deliberately excludes them (privacy).
  `financial/adjustments/ai.Request`/`Response`/`Issue` are **not** the same
  Go types as `financial/classification/ai.Request`/`Response`/`Issue`,
  despite sharing field/type names in places.
- **AI never invents an amount.** `ai.ValidateSuggestion` rejects any
  suggestion whose `Amount` does not exactly match the referenced
  `SourceRow.Amount`. A suggestion needing a value AI cannot know (a
  market-rate replacement salary, a fair-market rent) sets
  `Suggestion.RequiresUserInput = true` and carries `Amount: 0` into
  `ToAdjustments` — the real figure is supplied later by a human via the
  existing `review.AdjustmentDecision.NewAmount` override, not a new payload
  type.
- **AI never applies an adjustment.** `ToAdjustments` always produces
  `Included: false`; only a `review.Decision` (via `review.Apply`) can flip
  it, and only `financial/adjustments.Apply` changes normalized earnings.

## 5. Maintainable earnings

```
[]earnings.Observation  (e.g. normalized EBITDA or SDE across historical periods)
   ↓  earnings.Calculate
earnings.Result{ Available, Value, Strategy, IncludedPeriods, ExcludedPeriods }
```

`earnings.Calculate(observations []earnings.Observation, opts
earnings.Options) earnings.Result`. See
[`financial/earnings`](../README.md#financialearnings) for the four
strategies (`latest_period`, `simple_average`, `weighted_average`,
`trend_adjusted`) and comparable-period-type filtering.

## 6. Settings resolution

```
system, account, client, valuation Settings  (four caller-assembled layers)
   ↓  settings.Resolve
settings.Resolution{ Values map[string]any, Sources map[string]Scope, SchemaVersion }
```

`settings.Resolve(system, account, client, valuation settings.Settings)
settings.Resolution` — precedence `valuation > client > account > system`.
See [`settings`](../README.md#settings) and
[Settings snapshot contract](../README.md#settings-snapshot-contract).

**Resolve once, use as a snapshot.** `Resolution` has no aliasing back to
the `Settings` values it was built from — a caller resolves once per
valuation run and hands the immutable `Resolution` into
`valuation/orchestrator.Request`, never re-resolving mid-calculation. This
is what makes a persisted historical valuation reproducible even after
system/account defaults change later — see
[§9 Persistence snapshot guidance](#9-persistence-snapshot-guidance-summary)
below.

**Unset vs. explicit zero.** A `nil` field means "inherit from the next
lower-precedence scope"; a non-nil pointer to a zero value means
"explicitly resolved to zero," a real, resolvable value. See
[Unset vs. explicit zero/false](../README.md#unset-vs-explicit-zerofalse).

## 7. Valuation methods

Each method is its own subpackage with a strongly-typed `Input`/`Result` —
deliberately not one generic input type. See
[Method codes and versions](../README.md#method-codes-and-versions).

| Method | Package | `Code` | Native `ValueType` |
|---|---|---|---|
| SDE Multiple | `valuation/sde` | `SDE_MULTIPLE` | `equity_value` |
| EBITDA Multiple | `valuation/ebitda` | `EBITDA_MULTIPLE` | `enterprise_value` |
| Capitalization of Earnings | `valuation/capitalization` | `CAPITALIZATION_OF_EARNINGS` | `equity_value` |
| Discounted Cash Flow | `valuation/dcf` | `DCF` | `enterprise_value` |
| Adjusted Net Asset Value | `valuation/netassets` | `ADJUSTED_NET_ASSET_VALUE` | `asset_value` |

Every `Calculate` function is pure (no I/O, no mutation, no panics on bad
financial input) and returns `Available bool` + `Errors`/`Warnings
[]valuation.Issue` instead of a Go `error` — see
[Validation](../README.md#validation-1).

**Applicability → Orchestration:**

```
profile.Profile
   ↓  applicability.Calculate
applicability.Results  (per-method Score/Level/Reasons, never blocking by itself except DCF's HardBlockReason)
   ↓  (optional input to orchestrator.Request.Applicability + FilterPolicy)
orchestrator.Request{ Resolution, Applicability, FilterPolicy, SDE *sde.Input, EBITDA *ebitda.Input, ... }
   ↓  orchestrator.Execute
orchestrator.Run{ Methods []MethodOutcome }
```

See [`valuation/applicability`](../README.md#valuationapplicability) and
[`valuation/orchestrator`](../README.md#valuationorchestrator) for the
four `FilterPolicy` values and per-method evaluation order.

**Basis conversion → Consensus → Sensitivity → Report:**

```
orchestrator.Run
   ↓  report.BuildConsensusInputs(run, weights)
[]consensus.Input  (each pre-populated with its method's own Bridge)
   ↓  consensus.Calculate(inputs, consensus.Options{TargetBasis: ...})
consensus.Result   ← internally uses valuation/basis.Convert/ConvertAll
   ↓
sensitivity.MultipleSensitivity / EarningsMultipleMatrix / DCFSensitivity  (independent, caller-supplied scenarios)
   ↓
report.Build(report.BuildInput{ Run: &run, Consensus: &consensusResult, ... })
report.Report  (JSON-serializable, presentation-neutral)
```

See [Value basis and conversion](../README.md#value-basis-and-conversion),
[`valuation/consensus`](../README.md#valuationconsensus),
[`valuation/sensitivity`](../README.md#valuationsensitivity), and
[`valuation/report`](../README.md#valuationreport).

**`report.BuildInput` (abridged — every field independently optional):**

```go
type BuildInput struct {
    ValuationDate     string
    Snapshots         []metrics.Snapshot
    NormalizedEBITDA, NormalizedSDE FinancialFigure
    GrowthMetrics     []Assumption
    AdjustmentsEBITDA, AdjustmentsSDE *adjustments.Result
    Run               *orchestrator.Run
    Consensus         *consensus.Result
    MultipleSensitivity    *sensitivity.MultipleSensitivityResult
    EarningsMultipleMatrix *sensitivity.Matrix
    DCFSensitivity         *sensitivity.DCFGrid
}
```

## 8. Version fields a caller can read off every contract

The full inventory (one row per versioned contract, its constant name, and
what it covers) lives in one place — the README's
[Versioning strategy](../README.md#versioning-strategy) table — rather
than being duplicated here. Every persisted result the main application
stores should also persist the version fields the producing package
stamped onto it (`MethodVersion`, `Plan.Version`, `Provenance.*Version`,
etc.) and never re-derive them later, since defaults/rules can change out
from under a historical record.

## 9. Persistence snapshot guidance (summary)

This library performs no persistence. What a consuming application should
generally snapshot for a finalized historical valuation (never re-run
dynamically against changed defaults/rules) is documented in
[INTEGRATION.md § Persistence snapshot guidance](INTEGRATION.md#persistence-snapshot-guidance).

## API freeze markers

**Stable V1** (safe for a main application to depend on directly; will not
be renamed or repurposed without a major version):

- Canonical financial `Code` values (`financial.Code*` constants — see
  [Taxonomy](../README.md#taxonomy): "once released, a canonical `Code` is
  a persistent API identifier, exactly like a stored primary key").
- `financial.RowKind`/`RowStatus` semantics.
- `valuation.Code` method codes (`SDE_MULTIPLE`, `EBITDA_MULTIPLE`, etc.)
  and `valuation.ValueType` (`enterprise_value`/`equity_value`/`asset_value`)
  semantics.
- `review.Kind`/`review.Action`/`review.Severity`/`review.Status` enums and
  the deterministic `ReviewItem.ID` format table (§ Deterministic IDs).
- Major serialized result structures — see
  [§ Serialization coverage](#serialization-coverage) below for the full
  per-type audit table.
- `ingestion.WarningCode`/`ErrorCode`, `review.IssueCode`,
  `adjustments.IssueCode`, `valuation.IssueCode`, `ai.IssueCode` (both AI
  packages) — every stable machine-actionable code constant listed in the
  README's [Error taxonomy](../README.md#error-taxonomy).

**Still evolvable** (may change without a major version bump; do not build
brittle logic that depends on exact values):

- Deterministic classification's heuristic confidence constants
  (`ConfidenceStrongRule = 0.92`, etc.) and rule ordering within
  `DefaultRules()`.
- `applicability`'s point-scoring deltas
  (`pointsStrongPositive`/`pointsPositive`/etc.) — explicitly documented in
  the README as "a first-pass heuristic scale, not derived from any
  empirical study."
- AI prompt text and internal request/response JSON field ordering (the
  Go struct shape is stable; exact prompt wording is not).
- Internal parser heuristics inside `ingestion/internal/tabular` (word-gap
  factors, column-gap factors, indentation-step sizing) — these affect
  ingestion *quality*, not the `ingestion.Result` shape itself, which is
  stable.
- Internal rule/decision-application ordering inside packages where no
  test pins a specific order as a contract (e.g. `review.Apply`'s
  phase-then-original-index order for structure-vs-classification decisions
  **is** pinned/stable per its own dedicated tests — but ordering choices
  not called out as a guarantee anywhere in this document or the README
  should be treated as an implementation detail).

## Known limitations

See [README § Known limitations](../README.md) sections referenced
throughout this document (ingestion gaps, OCR limitations, applicability's
heuristic nature) — consolidated in one place in
[INTEGRATION.md § Known limitations](INTEGRATION.md#known-limitations).

## Public API audit

Every exported symbol across `financial`, `financial/classification`,
`financial/adjustments`, `ingestion`, `review`, `settings`, and `valuation`
(34 packages total) was reviewed and classified as `PUBLIC_V1`,
`INTERNAL_IMPLEMENTATION`, or `EXPERIMENTAL`. The full per-symbol table is
large (several hundred symbols); this section summarizes the classification
rule and the concrete outcomes, rather than reproducing the whole table.

**Classification rule:**

- **`PUBLIC_V1`** — a main application will reference this directly:
  pipeline-boundary types (`RawLineItem`, `MappedLineItem`,
  `FinancialDataset`, `Result`/`Plan`/`Decision` shapes across
  `ingestion`/`classification`/`review`), entry-point functions
  (`Classify`, `Normalize`, `Calculate`, `Build`, `Apply`, `Resolve`,
  `Execute`), and every stable enum/code constant (`financial.Code`,
  `valuation.Code`, every `*IssueCode`/`*ErrorCode`/`*WarningCode`).
- **`INTERNAL_IMPLEMENTATION`** — exported for Go visibility reasons across
  a package boundary that isn't a typical integration surface (e.g.
  `ingestion.BuildResult`/`BuildInput`, shared only by `ingestion/csv`,
  `ingestion/xlsx`, `ingestion/pdf` to keep statement-interpretation logic
  implemented exactly once), or a helper primarily meant for someone
  implementing a `Rule`/`Classifier`/`Suggester`/`Engine` extension point
  rather than a typical caller (e.g. `classification.RuleFunc`,
  `classification.NormalizeLabel`).
- **`EXPERIMENTAL`** — every exported symbol in `financial/classification/ai`,
  `financial/classification/ai/openai`, `financial/adjustments/ai`, and
  `financial/adjustments/ai/openai`, per the task's explicit rule that
  anything AI-related is experimental regardless of how well-tested it is.
  AI capabilities are opt-in, mandatory-human-review, and the two youngest
  subsystems in the module — their wire shapes are the most likely to need
  a breaking change as real-world AI-provider integration experience
  accumulates.

**Every other package audited** (`financial/earnings`, `financial/metrics`,
`financial/reconciliation`, `ingestion/csv`, `ingestion/xlsx`,
`ingestion/pdf`, `ingestion/pdf/pdfimage`, `ingestion/ocr`,
`ingestion/ocr/tesseract`, `review`, `settings`, `valuation` and all five
method subpackages, `valuation/applicability`, `valuation/basis`,
`valuation/orchestrator`, `valuation/profile`, `valuation/consensus`,
`valuation/sensitivity`, `valuation/report`) has an almost entirely
`PUBLIC_V1` exported surface — these are small, focused, intentionally
public packages with very few incidental exports.

### Exports reduced in this pass

- **`ingestion/pdf/pdfimage.Error`** (+ its `ErrCodePageOutOfRange`
  constant) — confirmed genuinely unused anywhere in the repository,
  including this package's own tests (`PageCount`/`PageDims`/
  `ExtractPageImages` all return the plain `error` interface, never a
  concrete `*Error`). Unexported to `extractError`/`errCodeInvalidPDF`,
  matching the package's stated goal of a minimal, intentional exported
  surface. `ErrCodePageOutOfRange` was removed outright — it was never
  constructed anywhere, dead in both directions.
- **`financial/metrics.AvailableValue`**'s doc comment corrected (it began
  "Available reports..." for a function named `AvailableValue` — a
  comment/name drift from an earlier rename, now fixed to read
  "AvailableValue reports...").

### Exports reviewed and deliberately left as-is

Several symbols showed zero in-repository callers under a plain grep, but
were confirmed to be either (a) real cross-package test infrastructure that
would break if unexported, or (b) too minor/low-confidence a call to make
without more evidence — consistent with this task's "reduce unnecessary
exports **where doing so is backward-compatible**" and "do not redesign
working packages merely for aesthetics" constraints:

| Symbol | Why it was left alone |
|---|---|
| `ai.FakeClassifier`/`NewFakeClassifier`/`FakeBatchClassifier` (`financial/classification/ai`) | Used by `review`'s own AI-integration tests across the package boundary — unexporting would break a real, intentional cross-package test dependency, not remove dead code. |
| `ocr.FakeEngine` (`ingestion/ocr`) | Its own doc comment states it is deliberately not `_test.go`-gated specifically so `ingestion/pdf`'s tests can import it — working as designed. |
| `ai.FakeSuggester` (`financial/adjustments/ai`) | Same pattern as `ai.FakeClassifier` — used by `review/ai_adjustment_integration_test.go`. |
| `settings.Bool`/`Float64` | Zero production callers today, but these are the documented, idiomatic way for any future caller to construct a `Settings` value's nil-means-unset pointer fields — likely to gain real callers once a main application starts constructing `Settings` literals. Their naming (no `New`/verb prefix, unlike every other constructor in the module) is a minor, cosmetic outlier, not a functional problem — left as-is rather than renamed, since a rename this late is itself the kind of disruptive, aesthetics-only change this task says to avoid. |
| `financial.CodesByCategory`, `ai.BuildAllowedCodes` (`financial/classification/ai`), `ai.BuildAllowedTypes` (`financial/adjustments/ai`) | Zero current callers, but each is documented as *the* intended way to build a closed-set/category-filtered list for a typical integration — plausible near-term real use, not dead code. |
| `financial.ValidationErrors` (plural) | Zero direct external construction, but it is the concrete type a caller's `errors.As` target should name when inspecting a `financial.Normalize` failure (its singular sibling `*ValidationError` is what appears inside it) — part of the same public error contract, not incidental. |
| `review.PixelBounds` | Zero external readers yet, but it's `review`'s own documented stable copy of `ingestion.PixelBounds` (deliberate duplication, not an oversight — see the near-duplicate note below) for exactly the OCR-overlay-rendering use case a future review UI needs. |

No further exports were reduced. The overall audit conclusion: **this
module's public surface was already close to intentional** — the deep
grep-verification pass found very few genuine accidents, and the ones found
(`pdfimage.Error`, the doc-comment drift) are now fixed.

### Naming and structural notes (documented, not changed)

- **`settings.Bool`/`Float64`** are the one naming-convention outlier in the
  module (no `New`/verb prefix, unlike every other constructor). Noted
  above; not renamed.
- **Deliberate near-duplicate taxonomies** — confirmed intentional via each
  package's own doc comments, not accidental drift:
  - `review.PixelBounds` vs `ingestion.PixelBounds` — same field shape by
    design, so `review` never has to import `ingestion` just for a display
    type.
  - `ingestion.StructuralKind` vs `financial.RowKind` — two layers of the
    same heading/subtotal/total concept: `StructuralKind` is ingestion's
    own pre-`RawLineItem` structural read, `RowKind` is the pipeline-
    boundary field it's translated into (`structuralKindToRowKind`).
    `StructuralKind` additionally has a `Blank` value with no `RowKind`
    equivalent (blank rows never become a `RawLineItem` — see §1 above).
  - `HasErrors` is independently defined three times
    (`valuation.HasErrors`, `financial/adjustments.HasErrors`,
    `review.HasErrors`), identical one-line implementation each, against
    each package's own local `Issue`/`Severity` type — each doc comment
    explicitly cross-references the others as the reason for the pattern.
    Only `valuation.HasErrors` has real cross-package callers today (all
    five method packages); the other two are used within their own
    package. This is a real, if minor, design question (a generic
    `Severity` constraint could have replaced three hand-copies) but not
    a bug — left as-is per "do not redesign working packages for
    aesthetics."
  - The **eight independent structured Issue/Error systems** documented in
    the README's [Error taxonomy](../README.md#error-taxonomy) are all
    confirmed intentional (see [§ Error and issue contracts](#error-and-issue-contracts)
    below) — not merged.

## Data ownership / mutation audit

**Every public processing function/method audited is read-only or
copy-on-write with respect to its caller-owned inputs — no violations of
this library's "public domain processing functions do not mutate
caller-owned inputs unless explicitly documented" rule were found.**

| Function | Behavior | Evidence |
|---|---|---|
| `csv.Parse`, `xlsx.Parse`, `pdf.Parse` | read-only | Each reads its `io.Reader` into a locally-owned byte buffer (`io.ReadAll`/`readAllLimited`) before parsing; there is no caller-owned struct to mutate at this boundary. |
| `ingestion.Result.ToRawLineItems()` | copies | Builds a brand-new `[]financial.RawLineItem` and a brand-new `Values` map per row — an explicit map copy, never an alias. |
| `classification.Classify`/`ClassifyBatch` | read-only | Doc-comment-asserted and code-confirmed: only reads off `raw`; `ClassifyBatch`'s loop variable is a Go range value copy, never written back. |
| `review.Build` | read-only | Every `build*Items` helper reads `BuildInput` fields only; anywhere a stable sort is needed, the slice is explicitly copied first (`rows := make(...); copy(rows, in.Rows)`) rather than sorting the caller's own slice in place. |
| `review.Apply` | read-only (the highest-risk function, most rigorously enforced) | `decisions`, `source.MappedLineItems`, and `source.Adjustments` are all **deep-copied** at the top of `Apply` (`cloneDecisions`, `cloneMappedLineItems`, `cloneAdjustments` — each allocates new slices and new nested maps/slices) before any validation or application logic runs; every mutation inside `applyClassificationDecision`/`applyStructureDecision`/`applyAdjustmentDecision` writes only through the fresh copy inside the returned `ApplyResult`, never through `source`. Backed by a dedicated `apply_immutability_test.go` doing before/after JSON-snapshot comparison, not merely asserted in a doc comment. |
| `financial.Normalize` | read-only | Aggregates into a locally-owned `totals` map and builds a new `Items` slice; never writes through an input `MappedLineItem`. |
| `metrics.Calculate` | read-only | Doc-comment-asserted ("never mutates dataset") and code-confirmed — reads via a local index, builds new `Snapshot` values. |
| `adjustments.Apply`/`Validate` | read-only | `snapshot` is passed and read by value throughout; `adjs` is ranged over read-only, appending only into locally-owned bridge/skip slices. |
| `ai.ClassifyWithFallback`/`ClassifyBatchWithFallback` (classification AI) | read-only | Never writes through an input row; every outcome is built into a freshly allocated `FallbackOutcome`. |
| `ai.SuggestAdjustments`/`SuggestAdjustmentsBatch`/`ToAdjustments` (adjustment AI) | read-only | Same pattern — builds new `Outcome`/`Adjustment` values, never mutates `req.Candidates` or a caller's `[]Suggestion`. |
| `orchestrator.Execute` | read-only | `req` is passed by value; every method's `Calculate` (itself read-only) is called against a dereferenced value copy. |
| `consensus.Calculate` | read-only | `result.Requested = inputs` stores the slice header, but no code path writes through an element of the caller's `inputs` slice — every transformation (basis conversion, weighting) operates on a value-copy of each `Input`. |
| `report.Build` | read-only | Every `build*` helper only reads off `BuildInput` fields, appending into freshly allocated local slices. |

**No violations found.** `review.Apply`, flagged in advance as the
highest-risk function given its role correcting caller-supplied domain
data, turned out to be the most rigorously defended of all of them.

## Nil / zero-value safety audit

**Every config/option struct audited has a safe zero value** — no panics,
no silent nonsense — confirmed both by tracing the code and, for the
highest-uncertainty cases, by actually running the zero-value call.

| Type | Zero-value behavior |
|---|---|
| `ingestion.Options{}` | Safe. `ResolveOptions`/`Limits.withDefaults()` fill every unset field with a documented default before any parser uses it; confirmed by running `csv.Parse`/`pdf.Parse` with `Options{}` against real readers. |
| `pdf.Options{}.OCR` / OCR options | Safe. Zero value is `OCRDisabled`, explicitly documented as identical to pre-OCR-feature behavior; `Parse` (not `ParseWithOCR`) never reads the OCR field at all. |
| `classification.Config{}` | Safe by explicit design — its own doc comment states the zero `Config` classifies every row as `SourceUnknown` except structural detection; nil `Rules` is a no-op range, not a nil-dereference. |
| `ai.Policy{}` (both AI packages) | Safe. Zero value of `Mode`/`FallbackMode` is the disabled state; both orchestration entry points check this first and short-circuit before touching any other field. |
| `settings.Settings{}` (all four layers) | Safe. Every pointer field is nil-checked before dereference in `Resolve`'s field-resolution loop; an all-nil `Settings{}` at every scope simply contributes nothing, the documented "no scope has an opinion" case. |
| `reconciliation.Options{}` | Safe. `tolerance()` falls back to `DefaultTolerance`; `reportedFor()` is nil-map-safe. Confirmed by running `Run(FinancialDataset{}, Options{})`. |
| `review.Policy{}` | Safe. An exact zero-value `Policy{}` (struct-equality-checked) is swapped wholesale for `DefaultPolicy()`; a `Policy` that's non-zero in *any* field uses the caller's values as-is (per-struct substitution, not per-field defaulting — worth knowing, since it differs from `ingestion.Limits`' per-field pattern). |
| Each valuation method's `Input{}` | Safe — and notably these are `Input`, not `Options`, types: every `Calculate` validates every numeric field for finiteness/sign **before** any arithmetic (e.g. `dcf.Calculate` checks `DiscountRate <= TerminalGrowthRate` strictly before the Gordon Growth division; `capitalization.Calculate` checks `CapitalizationRate <= 0` before dividing). Every method's doc comment states "never panics on bad financial input," and the code confirms it. |
| `applicability.Calculate(profile.Profile{})` | Safe by explicit design — an empty `Profile` produces every method's neutral baseline score; DCF is the one exception, starting hard-blocked at `NOT_APPLICABLE` regardless. |
| `consensus.Options{}` | Safe. Empty `TargetBasis` triggers the documented "infer" path; an empty `inputs` slice is checked first and returns a structured error `Result`, never panics. Confirmed by running `Calculate(nil, Options{})`. |

**One adjacent finding, not a zero-value-struct issue but worth a doc
caveat:** `csv.Parse`/`xlsx.Parse`/`pdf.Parse` all accept a plain
`io.Reader`/`io.ReadSeeker` parameter with no doc-comment statement about a
literal `nil` interface value. Passing `nil` panics inside the standard
library (`io.ReadAll` on a nil reader) — standard Go behavior for any
function typed to accept an interface without a nil guard, not specific to
this library, but a one-line "`r` must be non-nil" doc-comment addition
would close the gap. Not fixed in this pass (a documentation nit, not a
zero-value-struct safety violation — the task's zero-value audit scope is
about structs, and every struct here is safe).

## Error and issue contracts

This module deliberately maintains **nine independent structured issue/
error systems** rather than one shared type — confirmed intentional via
each package's own doc comment (four of them explicitly state the same
reasoning: forcing one package's Issue type to also serve an unrelated
problem domain would either leak that domain's codes into an unrelated
package or create an import coupling that shouldn't exist). This section
is the caller-facing index; see the README's
[Error taxonomy](../README.md#error-taxonomy) for the six systems already
documented there.

| System | Package | Emitted by | Fatal/non-fatal | Stable code field |
|---|---|---|---|---|
| `ingestion.Error` / `ingestion.Warning` | `ingestion` | `ingestion`, `ingestion/csv`, `ingestion/xlsx`, `ingestion/pdf` | **Separate types**: `Error` always fatal (no `Result` produced), `Warning` always non-fatal (`Result` still valid) | `Error.Code ErrorCode`, `Warning.Code WarningCode` |
| `financial.ValidationError`/`ValidationErrors` | `financial` | `financial.Normalize` only | Fatal only — no warning concept in this package; a real Go `error` via `errors.As` | `ValidationError.Code ValidationErrorCode` (2 constants: `UNRECOGNIZED_STATUS`, `MISSING_CODE`) |
| `review.Issue`/`IssueCode`/`IssueSeverity` | `review` | `review.Apply` | Same type, `Severity` field (`Error` rejects the decision into `ApplyResult.Invalid`; `Warning` is advisory) | `Issue.Code IssueCode` (12 constants) |
| `adjustments.Issue`/`IssueCode`/`IssueSeverity` | `financial/adjustments` | `adjustments.Validate`/`Apply` | Same type, `Severity` field (`Error` skips the adjustment; `Warning` is advisory) | `Issue.Code IssueCode` (12 constants) |
| `financial/reconciliation`'s `Check`/`Status`/`CheckCode` | `financial/reconciliation` | `reconciliation.Run` | **Four-valued `Status`** (`PASS`/`WARNING`/`FAIL`/`NOT_APPLICABLE`), not fatal/non-fatal at all — `Run` never returns a Go error; whether a `FAIL` blocks anything is entirely a caller policy decision (`review.Policy.ReconciliationFailureBlocks` is the opt-in escalation lever) | `Check.Code CheckCode` (16 constants) |
| `financial/classification/ai.Issue`/`IssueCode`/`IssueSeverity` | `financial/classification/ai` | `ClassifyWithFallback`/`ClassifyBatchWithFallback`/`ValidateResponse` | Same type, `Severity` field | `Issue.Code IssueCode` (11 constants) |
| `financial/adjustments/ai.Issue`/`IssueCode`/`IssueSeverity` | `financial/adjustments/ai` | `SuggestAdjustments`/`ValidateSuggestions` | Same type, `Severity` field | `Issue.Code IssueCode` (13 constants) |
| `valuation.Issue`/`IssueCode`/`IssueSeverity` | `valuation` | Every one of the five method packages + `valuation/consensus` (imports `valuation.Issue` directly, not its own type) | Same type, `Severity` field (`Error` makes `Result.Available == false`; `Warning` is advisory) | `Issue.Code IssueCode` — the largest set: a shared "common" code group plus each method package extending the **same** `valuation.IssueCode` string type with its own narrower codes (the one system here where sub-packages extend a shared parent's code space, rather than defining a fully independent type) |
| `ocr.Error`/`ErrorCode` | `ingestion/ocr` | `ingestion/ocr`, `ingestion/ocr/tesseract`, propagated into `ingestion.Error` by `ingestion/pdf`'s OCR path | Fatal only, mirrors `ingestion.Error`'s Code/Message/Detail shape | `Error.Code ErrorCode` (3 constants) |

**`classification` itself has no dedicated Issue type** — it signals
"needs review" via `Result.ReviewRequired`/`Result.IsUnknown()` directly on
the result, pushing structured issue construction one layer up into
`review.Build`'s `KindClassification` item logic. This is a relevant data
point, not a tenth system.

**One gap remains, real but narrow (a second was closed in Prompt 14A —
see below):**

1. **`valuation/orchestrator.MethodWarning`** flattens a method's
   `valuation.Issue{Code, Severity, Message}` down to `{Method, Message
   string}` when building `Run.Warnings` — the one place in the whole
   pipeline where `Code`/`Severity` are deliberately dropped rather than
   carried through. A caller wanting the original `Code` for a given
   warning needs to read it off the individual method `Result.Warnings`
   directly (still fully populated) rather than off the orchestrator's
   flattened summary.

**Closed in Prompt 14A:** `financial.ValidationError` previously had no
stable `Code` field — the one system in the Prompt 14 survey without one,
requiring a caller to string-match the freeform `Reason` field to
distinguish "missing code" from "unrecognized status." It now carries
`Code ValidationErrorCode` (`ErrCodeUnrecognizedStatus` /
`ErrCodeMissingCode`), populated on every emitted `*ValidationError`, with
JSON tags added to the whole type (`source_id`, `index`, `code`, `reason`)
since it previously had none. Existing callers checking via `errors.As` are
unaffected; only additive fields changed. A caller must branch on `Code`,
never parse `Reason` — see the README's
[Error taxonomy](../README.md#error-taxonomy).

**Which errors are end-user-safe to show directly vs. developer-facing
diagnostics is undocumented at the field level across all nine systems**,
with exactly one exception:
`financial/reconciliation.Check.Explanation` explicitly states "suitable
for display to a reviewer." Every other system's `Message`/`Reason` field
carries no audience statement. In practice: `Code` fields are always safe
to branch on programmatically (never parse `Message`/`Reason` strings — use
the `Code`/`CheckCode`/`ErrorCode`/`WarningCode` constant instead, which is
this whole audit's central recommendation); whether a `Message` string
itself is safe to render directly in an end-user-facing UI without
main-application-side rewording should be treated as **undetermined** per
type until a main application's own UX review decides otherwise — most
read as developer-diagnostic in tone (e.g. `financial.ValidationError`'s
`Error()` output embeds Go-style `%q`/`%d` formatting), but this was not
uniformly asserted by any package's doc comment.

## Version inventory gaps

The README's [Versioning strategy](../README.md#versioning-strategy) table
is the authoritative, complete inventory of every version constant that
exists — 20 constants total, every one confirmed to actually be echoed
onto its corresponding output type (`Result`/`Plan`/`Provenance` field),
not decorative. Cross-verified directly against source (file:line for
every one) — the table is accurate as written.

**Closed in Prompt 14A:** `ingestion` (and its `csv`/`xlsx`/`pdf`
subpackages) previously had no version constant at all, despite
`ingestion.Result` being a JSON-serializable, pipeline-critical output type
whose own structural-detection rules (statement-type detection, period
parsing, numeric-format rules, `ClassifyRowKind` heuristics) are exactly the
kind of thing the README's own versioning criterion describes. It now has
`ingestion.SchemaVersion`, echoed on `ingestion.Result.SchemaVersion` and
populated by every format via the shared `BuildResult` entry point (all
three formats funnel through it; the one bypass path — `xlsx.Parse`'s
early return on ambiguous sheet selection — was patched to set it directly).
One version constant, not one per subpackage, since `csv`/`xlsx`/`pdf` all
produce the exact same `Result` contract shape rather than independent
ones. (`OCRProvenance.EngineVersion` remains a *runtime-reported* string
from the Tesseract binary itself, a categorically different thing that
happens to share the word "Version" — not a contract version, and
unaffected by this change.)

**Packages that still have no version constant, despite having fixed
rules/formulas/taxonomies that could reasonably change (out of scope for
Prompt 14A — see that task's "do not add unnecessary new versions"
instruction):**

- **`financial/reconciliation`** — no version constant despite its own
  fixed `CheckCode` taxonomy and tolerance rules.
- **`valuation`** (the shared envelope package) — no version constant for
  `ValidateResultEnvelope`/`ValidateFiniteSteps`'s own invariant rules, as
  distinct from each method's own `Version` (which *is* present).
- **`valuation/orchestrator`, `valuation/basis`, `valuation/sensitivity`,
  `valuation/profile`** — none have a version constant, despite
  `valuation/basis` implementing a fixed enterprise/equity/asset
  conversion convention and `valuation/sensitivity` implementing fixed
  grid-generation formulas.

None of these gaps are a hidden/unexposed literal (no inline
`"1.0.0"`-shaped string was found sitting in a struct literal that should
have been a named constant) — every gap is a total absence of a version
identifier for that package's output shape. Given how deliberate and
consistent the existing 19-constant scheme already is, this reads as an
intentional scoping decision for these thinner/derivative packages rather
than an oversight — but `ingestion` in particular is flagged as worth a
second look in a future revision, since it's treated as version-worthy in
every other respect (stable `WarningCode`/`ErrorCode` taxonomies,
deterministic Row IDs) but not this one. **No new version constants were
added in this pass** — per this task's explicit "do not add unnecessary
new versions" instruction, this is reported as a finding for a future
decision, not something to fix unilaterally during an API-freeze pass.

## Serialization coverage

Every major result type expected to cross a JSON boundary into a main
application was audited for (a) complete `json:"..."` struct tags and (b)
an existing Marshal/Unmarshal test. **Every type checked has complete JSON
tags on every exported field** — no gaps found there. Test coverage:

| Type(s) | Package | Roundtrip test |
|---|---|---|
| `Result`/`Row`/`Cell`/`Metadata` | `ingestion` | Yes — `types_test.go` |
| `RawLineItem`/`MappedLineItem`/`FinancialDataset` | `financial` | Yes — `roundtrip_test.go`, `types_test.go` |
| `Result` | `financial/classification` | Yes — `roundtrip_test.go` |
| `Provenance` | `financial/classification/ai` | Yes — `json_test.go` |
| `Plan`/`ReviewItem`/`Decision`/`ApplyResult` | `review` | Yes — `roundtrip_test.go`, plus a dedicated NaN-rejection test |
| `Result`/`Bridge` | `financial/adjustments` | Yes — `roundtrip_test.go` |
| `Resolution` | `settings` | Yes — `roundtrip_test.go` |
| Each method's `Result` | `valuation/sde`, `ebitda`, `capitalization`, `dcf`, `netassets` | Yes — each has its own `roundtrip_test.go` |
| `Result`/`Statistics`/`Dispersion` | `valuation/consensus` | Yes — `roundtrip_test.go` |
| `Report`/`Summary`/`MethodComparisonRow` | `valuation/report` | Yes — `report_test.go`, including the zero-mean edge case |
| `Snapshot`/`MetricResult`/`MetricValue` | `financial/metrics` | **Partial** — `determinism_test.go` proves Marshal-succeeds plus deterministic map-key ordering, but has no Marshal→Unmarshal→compare roundtrip test, unlike every sibling package |
| `applicability.Result` | `valuation/applicability` | **None found** |
| `MultiplePoint`/`MultipleSensitivityResult`/`Matrix` | `valuation/sensitivity` | **None found** |

**Three gaps**, listed weakest-first: `financial/metrics` has the weaker
"Marshal succeeds" bar covered but not the stronger roundtrip;
`valuation/applicability` and `valuation/sensitivity` have no serialization
test coverage at all. Regression tests closing these gaps were added in
this pass — see [Verification](#verification) in the completion report;
`financial/metrics/roundtrip_test.go`,
`valuation/applicability/roundtrip_test.go`, and
`valuation/sensitivity/roundtrip_test.go` are new files.

**NaN/Inf risk: none found.** A repo-wide, deliberately-guarded pattern was
confirmed: `valuation.ValidateFiniteSteps`/`ValidateResultEnvelope` are
wired into all five method packages' `finalizeEnvelope` helpers; every
division site checked (`financial/metrics`' margin/growth calculations,
`valuation/consensus`'s `percentOf`/weight normalization) has a paired
zero-denominator guard in the same function, each documented as returning
an explicit `Unavailable`/`0` rather than letting `math.Pow`/division
produce `NaN`/`Inf`. `review/roundtrip_test.go`'s
`TestJSON_NoNaNOrInf_Comprehensive` additionally proves this is load-bearing
(not untested luck) by constructing a deliberately-NaN payload and
confirming `encoding/json` itself rejects it. No float field anywhere in
the audited scope was found unguarded.
