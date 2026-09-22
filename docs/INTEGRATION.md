# Integration Guide

This document shows the recommended high-level orchestration a main
application should follow to wire `github.com/themurtez/go-valuate` into a
real product, plus the security/privacy boundaries the library assumes its
caller will enforce, and what a finalized valuation run should persist.

See [`V1_CONTRACTS.md`](V1_CONTRACTS.md) for the exact data shapes at each
step, [`DEPENDENCIES.md`](DEPENDENCIES.md) for the third-party dependency
inventory, and [`FIXTURES.md`](FIXTURES.md) for the synthetic test-fixture
catalog. This document contains no persistence code, no HTTP handlers, and
no UI — it describes the shape of the integration, not an implementation
of it.

## Recommended orchestration flow

```
 1. Main app receives file bytes
 2. Select parser from content type
 3. ingestion parser (csv.Parse / xlsx.Parse / pdf.Parse[WithOCR]) -> Result(s)
 4. Result.ToRawLineItems()
 5. classification.ClassifyBatch (deterministic)
 6. optional: ai.ClassifyBatchWithFallback (AI fallback, opt-in)
 7. review.Build
 8. UI/main app collects Decisions from a human
 9. review.Apply
10. financial.Normalize
11. reconciliation.Run + metrics.Calculate
12. adjustments.Apply / optional AI adjustment suggestion review (financial/adjustments/ai + review.Build/Apply again)
13. review.EvaluateReadiness gate (must not be NOT_READY before continuing)
14. settings.Resolve -> resolved valuation settings snapshot
15. individual valuation methods (sde/ebitda/capitalization/dcf/netassets) via valuation/orchestrator.Execute
16. valuation/consensus.Calculate (basis-converted via valuation/basis internally)
17. report.Build
18. main app persists result
```

| # | Step | Nature | Persisted by main app? | Requires user interaction? |
|---|---|---|---|---|
| 1 | Receive file bytes | main-app I/O | source bytes/metadata, yes | no |
| 2 | Select parser from content type | main-app logic | no | no |
| 3 | `ingestion` parser → `Result`(s) | **stateless/pure** | raw parse result + warnings, recommended | no |
| 4 | `Result.ToRawLineItems()` | **stateless/pure** | via the dataset it feeds, optional standalone | no |
| 5 | `classification.ClassifyBatch` | **stateless/pure** | recommended (proposed mappings + confidence) | no |
| 6 | AI fallback classification | **optionally AI-backed**, opt-in (`Policy.Mode`, default off) | recommended (provenance — see §AI provenance below) | no (but its output always requires human confirmation downstream, at step 8) |
| 7 | `review.Build` | **stateless/pure** | the `Plan` itself, recommended | no |
| 8 | Collect `Decision`s | **expected to require user interaction** | yes — this is the audit trail | **yes** |
| 9 | `review.Apply` | **stateless/pure** | `ApplyResult`, recommended | no |
| 10 | `financial.Normalize` | **stateless/pure** | `FinancialDataset`, recommended | no |
| 11 | `reconciliation.Run` + `metrics.Calculate` | **stateless/pure** | both results, recommended | no |
| 12 | `adjustments.Apply` (+ optional AI suggestion review, itself steps 5-9 repeated over adjustments) | **stateless/pure**, optionally AI-backed | confirmed adjustments + bridges, recommended | **yes**, for any AI-suggested or caller-proposed adjustment |
| 13 | `review.EvaluateReadiness` | **stateless/pure** | the `Readiness` value, recommended | no — but its result gates whether a human must go back to step 8 |
| 14 | `settings.Resolve` | **stateless/pure** | `Resolution`, **required** (see [Persistence snapshot guidance](#persistence-snapshot-guidance)) | no (the four `Settings` layers themselves may be user-configured upstream of this call, but resolution is mechanical) |
| 15 | `valuation/orchestrator.Execute` | **stateless/pure** | `Run` (every method's `Result`), recommended | no |
| 16 | `valuation/consensus.Calculate` | **stateless/pure** | `Result`, recommended | no |
| 17 | `report.Build` | **stateless/pure** | the `Report`, **required** (this is the artifact a user reviews/exports) | no |
| 18 | Persist result | main-app I/O | — | no |

**Steps 3–17 are entirely this library's responsibility and contain no I/O,
no database calls, and no HTTP.** Steps 1, 2, and 18 are the main
application's responsibility and are not implemented here. Step 8 (and the
equivalent human decision inside step 12, for adjustment suggestions) is
the only place this flow requires a human in the loop — everything else is
either fully deterministic or, where AI-backed, produces a proposal that
still routes through the same human-decision step before it can affect a
result.

### Where sensitivity analysis and applicability fit

`valuation/sensitivity` (multiple/earnings/DCF-rate grids) and
`valuation/applicability` (method-fit scoring) are not numbered in the
18-step flow above because they are optional side calculations a caller
invokes as needed rather than a strict linear dependency of the next step:

- `applicability.Calculate(profile.Profile) Results` can run any time after
  the caller has enough business-description context (independent of
  ingestion/classification), and its output is an optional input to step
  15's `orchestrator.Request.Applicability` (see `FilterPolicy`).
- `sensitivity.*` functions run on caller-supplied scenario grids (a list
  of multiples, discount rates, etc.) independent of steps 15-16's actual
  computed results, and feed into step 17's `report.BuildInput` alongside
  them.

## AI provenance

Both optional AI capabilities (classification fallback, adjustment
suggestions) produce a `Provenance` value stamped with
`RequestSchemaVersion`/`OrchestrationVersion`/`AdapterVersion` (see the
README's [Versioning](../README.md#versioning) subsections for each). If a
main application uses either AI capability, **persist this provenance
alongside the resulting `ReviewItem`/`Decision`** — a finalized valuation
that used an AI suggestion should be traceable back to exactly which
provider/model/prompt-contract version produced it, for the same reason
every valuation method result is version-stamped (see
[Persistence snapshot guidance](#persistence-snapshot-guidance)).

## Security / privacy

- **Uploaded documents are untrusted input.** Every `ingestion` parser
  treats its input as untrusted bytes (see the README's
  [Security and limits](../README.md#security-and-limits) and
  [PDF-specific security/limits](../README.md#pdf-specific-securitylimits)
  sections): no macro/VBA execution (none of `encoding/csv`, excelize, or
  `ledongthuc/pdf` implement one), no formula evaluation (only a workbook's
  own cached value is read), no external workbook link following, and PDF
  parsing wraps the underlying library in `recover()` so a malformed/
  malicious PDF produces a clean `INVALID_FILE` error rather than crashing
  the process.
- **Parser limits must be set by the main app.** `ingestion.Options.Limits`
  (`DefaultLimits()`) bounds file size, sheet/row/column/page counts, cell
  text length, and OCR image dimensions/word counts — the library ships
  conservative defaults, but a main application accepting arbitrary
  user-uploaded files should review `DefaultLimits()` against its own
  threat model (e.g. lowering `MaxFileSizeBytes` for a public-facing
  upload endpoint) rather than assuming the library's defaults are
  appropriate for every deployment.
- **OCR executable invocation safety.** `ingestion/ocr/tesseract` invokes a
  local `tesseract` executable via `os/exec`, **never through a shell** —
  every argument is a separate `exec.CommandContext` argument, no
  string-built command line, no `sh -c`. `Engine.ExecutablePath` is
  configurable; a main application that lets any part of a file path or
  filename influence this value should still never pass caller-controlled
  data into it as a shell fragment (it isn't one, but treat it as a trust
  boundary regardless).
- **Temporary-file behavior.** Tesseract's CLI is file-based; one temporary
  input PNG and one temporary TSV output are created per `Recognize` call
  via `os.CreateTemp` (OS temp dir by default, `Engine.TempDir`
  overridable), with unpredictable names and `0600` permissions where the
  platform supports it, unconditionally removed via `defer`-guarded cleanup
  regardless of success/failure. A main application running in a
  multi-tenant environment sharing a temp directory across processes should
  still consider setting `Engine.TempDir` to a per-request-isolated path if
  its threat model calls for it.
- **AI data-minimization rules.** Classification AI's `Request` never
  includes per-period amounts, full documents, or user/account identity —
  only label/context text and a closed set of allowed codes (see
  [Privacy / minimal-context policy](../README.md#privacy--minimal-context-policy)).
  Adjustment-suggestion AI's `Request` **does** include amounts (the whole
  point is source-bound dollar-figure suggestions — see
  [Privacy: amounts are necessary here](../README.md#privacy-amounts-are-necessary-here))
  but still never includes unrelated rows, full documents, or user/account
  identity. Both AI packages pin `Store: false` on every OpenAI request. A
  main application choosing to enable either AI capability inherits this
  minimization but is still the one responsible for its own upstream
  decision about whether a given document/client is eligible for
  third-party AI processing at all (e.g. contractual or regulatory
  restrictions) — this library has no concept of client consent or data
  classification.
- **API keys belong to main app/config, not domain packages.** Neither AI
  package reads any AI-provider environment variable except the one
  explicitly-named opt-in convenience constructor,
  `openai.NewFromEnv()` (`OPENAI_API_KEY`/`OPENAI_MODEL`/`OPENAI_BASE_URL`).
  Every other construction path takes an explicit `openai.Config`. A main
  application should supply its own key management (its own secrets
  store/config layer) via `openai.Config`, not rely on process environment
  variables in production.
- **Financial document retention belongs to the main app.** This library
  never writes an uploaded document, an intermediate parse result, or a
  `FinancialDataset` to any persistent store — see
  [Persistence snapshot guidance](#persistence-snapshot-guidance) for what
  a main application should retain and for how long that decision is
  entirely the main application's own policy.
- **Do not log raw statements/prompts by default.** Neither AI adapter logs
  API keys, full request/response contents, or prompts/responses to any
  persistent store (see each AI section's own privacy paragraph in the
  README). A main application adding its own logging around calls into
  this library should apply the same discipline — a `financial.RawLineItem`
  label or an AI `Request`/`Response` can contain client-identifying
  financial detail even though this library's own logging never captures
  it.
- **Review decisions should be auditable in the main app.** `review`
  deliberately carries no user ID or timestamp on `AppliedDecision` (a
  scope decision, not an oversight — see
  [Audit/explainability](../README.md#auditexplainability)): "what did the
  system propose, what did the user change, why was review requested, what
  value was ultimately used" is all present, but *who* and *when* are the
  main application's responsibility to attach.

No security middleware is implemented here, and none should be — this
section documents boundaries this library assumes its caller enforces, not
an in-library enforcement mechanism.

## Error handling

**A main application must branch on a stable `Code` field, never on a
`Message`/`Reason`/`Detail` string.** Every structured issue/error type this
library exposes carries one (see the README's
[Error taxonomy](../README.md#error-taxonomy) for the full nine-system
list and [`V1_CONTRACTS.md`](V1_CONTRACTS.md#error-and-issue-contracts) for
the caller-facing index): `ingestion.Error.Code`/`ingestion.Warning.Code`,
`financial.ValidationError.Code`, `valuation.Issue.Code` (and every method
package's own extensions), `adjustments.Issue.Code`, `review.Issue.Code`,
`reconciliation.Check.Code`, and both AI packages' `ai.Issue.Code`. Message
strings are for humans and are not guaranteed stable across versions — a
wording change is not treated as a breaking change by this library's own
versioning strategy, since only `Code` is the contract. A UI wanting a
specific localized/branded message per error should maintain its own
`Code -> display string` mapping rather than rendering `Message`/`Reason`
directly, or should treat the library string as a developer-diagnostic
fallback only.

## Persistence snapshot guidance

This library performs no persistence — everything below is guidance for
what a consuming application should generally capture for a **finalized**
valuation run, not a schema.

**Why a finalized valuation must not dynamically re-run against changed
defaults/rules.** Every stage in this library is a pure function of its
inputs and the *current* version of its own rules/formulas/defaults. If a
main application re-ran, say, `valuation/applicability` or
`financial/metrics` against a historical valuation using today's rule set
instead of the rule set active when that valuation was finalized, the
result could silently differ from what a client was actually shown or
relied on — an SDE multiple applicability score, a metrics formula, or an
AI provenance version could all have changed since. This is precisely why
every stage here stamps a version constant onto its output (see the
README's [Versioning strategy](../README.md#versioning-strategy) and
[Method versioning](../README.md#method-codes-and-versions):
"a `Result` computed today must remain self-describing about exactly which
calculation logic produced it, even after this package's formulas or
validation rules evolve later"). **A finalized valuation is a historical
record, not a live view** — persist the snapshot, don't recompute it on
read.

**Suggested candidates to snapshot per finalized valuation:**

| Candidate | Source | Why |
|---|---|---|
| Source document metadata | main app (filename, upload date, content type) | provenance — this library never retains the source document itself |
| Parsed/raw line items | `ingestion.Result`/`Results` (already `SchemaVersion`-stamped), `financial.RawLineItem[]` | reproducibility of what classification saw |
| Confirmed mappings | `financial.MappedLineItem[]` (post-`review.Apply`) | the actual classification decisions used, including any human override |
| Review decisions | `review.Plan`, `[]review.Decision`, `review.ApplyResult` | the full audit trail behind every correction — who/when belongs to the main app, but the domain content belongs here |
| Normalized financial dataset | `financial.FinancialDataset` | the dataset every downstream figure was computed from |
| Adjustments | `[]adjustments.Adjustment`, `adjustments.Result` (bridges) | the confirmed normalization add-backs and their effect |
| Resolved settings snapshot | `settings.Resolution` | **required** — the exact rate/multiple/method-enable values used, immune to later default changes (see [Settings snapshot contract](../README.md#settings-snapshot-contract)) |
| Metrics | `metrics.Result` | the derived EBITDA/SDE/working-capital figures as computed at that time |
| Maintainable earnings | `earnings.Result` | which strategy and periods were used to arrive at the maintainable figure |
| Individual valuation method results | each method's `Result` (already `MethodVersion`-stamped) | the full calculation trace per method |
| Basis conversions | `valuation/basis.Conversion[]` (or the `consensus.Result.Conversions` that already carries them) | how enterprise/equity/asset values were reconciled onto one comparable basis |
| Consensus result | `consensus.Result` | the simple/weighted mean, range, and dispersion actually shown |
| Report model/schema version | `report.Report` (already `SchemaVersion`-stamped) | the exact presentation-neutral structure a UI rendered |
| AI provenance, if used | `ai.Provenance` (either package) | which provider/model/prompt-contract version produced a suggestion the user acted on |
| OCR provenance, if used | `ingestion.OCRProvenance`/`ingestion.Metadata.OCR` | engine/version/confidence for any OCR-derived figures the user accepted |

No SQL schema is prescribed here — this is a list of *what*, not *how*.

## Known limitations

Consolidated from the README's per-package limitation sections:

- **English-only OCR.** `ingestion/ocr/tesseract`'s language-pack scope is
  `eng` only for this MVP; multi-language support would require the
  caller's own Tesseract installation to have additional language-data
  packages, which this library does not manage.
- **JBIG2-encoded scans are not supported.** `ingestion/pdf/pdfimage`
  returns `UNSUPPORTED_SCANNED_PDF_LAYOUT` for a JBIG2-only scan page —
  `pdfcpu` provides no built-in JBIG2 decoder, and this library does not
  add one. CCITT Group 4 fax and JPEG (the two most common real-world scan
  encodings) are both fully supported.
- **North American numeric formatting only.** Locale is fixed at
  `LocaleEnUS` across CSV/XLSX/PDF/OCR — a European-formatted document
  (`.` thousands separator, `,` decimal) is out of scope for V1.
- **Multi-row (wrapped) headers are not specifically handled**, across
  every ingestion format.
- **Applicability scoring is a heuristic, not an empirical model.**
  `valuation/applicability`'s point deltas are explicitly documented as "a
  first-pass heuristic scale, not derived from any empirical study of
  which business characteristics actually predict a method's real-world
  reliability."
- **No deskew/rotation correction for scanned PDFs** — only mild skew
  (absorbed by row-grouping Y-tolerance) is tolerated.
- **OCR confidence is engine-specific and uncalibrated** — never a
  statistical guarantee, only a relative, per-engine signal.
- **Optional provider/runtime dependencies.** Real OpenAI calls require a
  caller-supplied API key (this library never ships one); real OCR
  requires a local Tesseract installation. Neither is required to build,
  test, or use the deterministic core — see
  [`DEPENDENCIES.md`](DEPENDENCIES.md).
- **No external comparable-transaction dataset.** This library computes
  valuations from caller-supplied assumptions (multiples, rates); it does
  not source or maintain any external market/comparable-transaction data.
- **No persistence, UI, or API in this module** — by design; see
  [What this project intentionally does not contain](../README.md#what-this-project-intentionally-does-not-contain).

See [`INTEGRATION.md`](#) → this section, and the README's individual
package sections, for the full detail behind each item above.
