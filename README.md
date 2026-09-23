# go-valuate

Reusable, standalone Go domain modules for business valuation: a canonical
financial data model, a deterministic normalizer and classifier, derived
financial metrics and normalization adjustments, five individual valuation
methods (SDE multiple, EBITDA multiple, capitalization of earnings, DCF,
adjusted net asset value), method applicability scoring, an orchestrator,
a consensus/dispersion engine, sensitivity analysis, a presentation-neutral
report model, and a hierarchical valuation-settings resolver — together a
complete deterministic valuation core, front to back
(see [`valuation/e2e`](valuation/e2e)).

This is a **library of pure domain logic**, developed independently of any
larger application. The intent is that its packages will later be copied or
moved into a bigger Go application once they've proven out. Until then it
stays self-contained on purpose: no database, no HTTP, no auth, nothing
tying it to a particular product's infrastructure.

**The standalone V1 domain library is integration-ready.** Every pipeline
stage described below — ingestion, classification, review, normalization,
reconciliation, metrics, adjustments, maintainable earnings, valuation
methods, applicability, orchestration, consensus, sensitivity, report
model, settings resolution, and both optional AI capabilities — is
contract-frozen for V1: public/internal/experimental API boundaries are
audited, data-ownership and zero-value behavior are documented and tested,
every major result type round-trips through JSON, and a canonical
end-to-end test
([`review/e2e_v1_contract_test.go`](review/e2e_v1_contract_test.go))
exercises the complete chain from raw document bytes to a serialized
report using only deterministic/fake dependencies. See:

- [`docs/V1_CONTRACTS.md`](docs/V1_CONTRACTS.md) — the primary data
  contracts a main application integrates with, and which parts of the
  public API are frozen vs. still evolvable.
- [`docs/INTEGRATION.md`](docs/INTEGRATION.md) — the recommended
  orchestration flow, security/privacy boundaries, and persistence
  snapshot guidance.
- [`docs/DEPENDENCIES.md`](docs/DEPENDENCIES.md) — every direct
  dependency and external runtime tool, with license/purpose/isolation
  boundary.
- [`docs/FIXTURES.md`](docs/FIXTURES.md) — the synthetic test-fixture
  catalog, so future test-writing reuses existing fixtures instead of
  inventing new ones.
- [`examples/full_flow`](examples/full_flow) — a runnable, narrated
  program demonstrating the complete flow end to end.

This is **not** a certified appraisal or compliance product — it is a
deterministic calculation core an application can build a real valuation
workflow on top of; see
[`docs/INTEGRATION.md` § Known limitations](docs/INTEGRATION.md#known-limitations)
for the consolidated list of what remains out of scope.

## Architecture

Every stage below is a pure function of the previous stage's output — no
I/O, no shared mutable state, no hidden global config:

```
CSV / XLSX / Text PDF bytes
        ↓
Tabular Ingestion          (ingestion, ingestion/csv, ingestion/xlsx, ingestion/pdf)
        ↓
Raw financial rows
        ↓
Classification            (financial/classification)
        ↓
Mapped rows
        ↓
Normalization              (financial — Normalize)
        ↓
FinancialDataset
        ↓
Reconciliation + Metrics   (financial/reconciliation, financial/metrics)
        ↓
Adjustments                (financial/adjustments)
        ↓
Maintainable Earnings      (financial/earnings)
        ↓
Valuation Methods           (valuation/sde, ebitda, capitalization, dcf, netassets)
        ↓
Applicability / Orchestration (valuation/applicability, valuation/orchestrator)
        ↓
Common Value Basis          (valuation/basis)
        ↓
Consensus                   (valuation/consensus)
        ↓
Sensitivity                 (valuation/sensitivity)
        ↓
Report Model                 (valuation/report)
```

The ingestion stage in detail — the shape every input adapter converges on
before classification ever runs:

```
CSV / XLSX / Text PDF
          ↓
      Ingestion
          ↓
  Raw Financial Rows
          ↓
    Classification
          ↓
 User Confirmation
   (future app)
          ↓
     Normalize
```

**Image/scanned PDFs return `OCR_REQUIRED` by default.** `pdf.Parse`
never attempts OCR — see [`ingestion/pdf`](#ingestionpdf) below for the
exact detection thresholds. A caller that explicitly opts in
(`pdf.ParseWithOCR` with `Options.OCR` set to `OCRAuto`/`OCRForce` and a
supplied `ingestion/ocr.Engine`) can OCR a scanned PDF via a local
Tesseract installation — see
[Scanned/image PDF support (OCR)](#scannedimage-pdf-support-ocr) below.

A separate, cross-cutting `settings` package supplies the hierarchical
system/account/client/valuation rate/multiple/method-enable resolution
(see [`settings`](#settings) below) that the orchestrator and individual
methods consume; it has no fixed position in the pipeline above since a
resolved `settings.Resolution` snapshot is assembled once, ahead of time,
and handed into a run rather than looked up mid-calculation (see
[Settings snapshot contract](#settings-snapshot-contract)).

**What is deliberately absent from every stage above, and from this
repository entirely:** no database, no HTTP/API layer, and no AI/LLM
beyond one narrow, opt-in, strictly-supervised exception — see [AI
fallback classification](#ai-fallback-classification) below. Deterministic
tabular ingestion (`ingestion`, `ingestion/csv`, `ingestion/xlsx`, text-based
PDF support in [`ingestion/pdf`](#ingestionpdf), and opt-in local-Tesseract
OCR for scanned PDFs — see
[Scanned/image PDF support (OCR)](#scannedimage-pdf-support-ocr)) is the
one exception to the "no document parsing" rule established in earlier
revisions of this README — see [`ingestion`](#ingestion) below for why a
purely structural, non-AI, non-database reader (plus a narrowly-scoped,
provenance-tracked, local-only OCR step) fits this repository's
constraints. `ingestion/pdf`'s default (`pdf.Parse`) still reads only a
PDF's own embedded text layer deterministically and returns an explicit
`OCR_REQUIRED` condition, never fabricated content, when no usable text
layer exists and OCR was not explicitly requested — see
[What this project intentionally does not contain](#what-this-project-intentionally-does-not-contain)
below for the full list and the reasoning behind it. Every stage's output
is plain, JSON-serializable Go structs; see
[Value basis and conversion](#value-basis-and-conversion),
[Versioning strategy](#versioning-strategy), and
[Deterministic ordering guarantees](#deterministic-ordering-guarantees)
below for the specific contracts a future integration layer can rely on
without re-deriving them from source.

## What this project intentionally does not contain

This is not an accident of scope — it's a design constraint. This repository
contains **no**:

- database code, PostgreSQL, repositories, or migrations
- HTTP handlers, REST APIs, or authentication
- users, organizations, or tenants
- Stripe, subscriptions, or billing
- Vue or any frontend code
- AI/LLM document understanding, OCR correction, add-back recommendations,
  valuation method/multiple selection, DCF forecasting, or report narrative
  generation — the one narrow exception is `financial/classification/ai`'s
  optional, closed-set, mandatory-human-review classification fallback (see
  [AI fallback classification](#ai-fallback-classification) below), which
  never bypasses the deterministic classification pipeline or the `review`
  domain layer
- cloud/storage integrations
- QuickBooks/Xero or other accounting-software API integrations
- external valuation data sources

Deterministic tabular ingestion (`ingestion` and its `csv`/`xlsx`/`pdf`
subpackages — see [`ingestion`](#ingestion) below) is a narrow, deliberate
exception: it is pure structural interpretation of data that is either
already tabular (CSV/XLSX: rows, columns, headers), has a deterministic
embedded text layer a born-digital PDF's own producer wrote (text-based
PDF — see [`ingestion/pdf`](#ingestionpdf)), or — as of the OCR support
described in [`ingestion/ocr`](#ingestionocr) — a scanned/image-only PDF
page whose raster content has been converted to positioned text via a
local OCR engine. It performs no AI/ML document-understanding, touches no
database or network, and hands off to `financial/classification` — which
this repository already contains — rather than duplicating it. OCR is
narrowly scoped and clearly separated from the rest of this repository's
deterministic guarantees: it is the one genuinely non-deterministic
component in the ingestion layer (see `ingestion/ocr`'s package doc
comment), is entirely opt-in (`ingestion/pdf`'s default behavior is
unchanged — a PDF with no usable embedded text layer still returns
`OCR_REQUIRED` unless a caller explicitly requests OCR and supplies an
engine), runs only via a local executable (Tesseract, invoked as an
external process — never a cloud API, never bundled/linked into this
module), and every OCR-derived value carries confidence/provenance data
rather than being presented with the same "deterministic and explainable"
guarantee the rest of `ingestion` offers. This remains categorically
distinct from AI/LLM document understanding: OCR here does character
recognition only, with zero interpretation, correction, or classification
delegated to any model — see `ingestion/ocr`'s and `ingestion/pdf`'s
numeric-safety sections for exactly how narrow the one permitted
correction step is.

The design rule is:

```
Go structs / JSON-compatible data in
  → deterministic domain processing
  → Go structs / JSON-compatible data out
```

No global state. No infrastructure dependencies. No side effects. Every
exported function in this repository is a pure function of its inputs.

Deterministic tabular/text-PDF/scanned-PDF ingestion (structural
interpretation of a tabular financial statement export, a text-based
PDF's positioned text, or a scanned PDF's OCR-recognized text, into
`financial.RawLineItem` values, with no classification of its own) is
implemented in `ingestion` and its `ingestion/csv`/`ingestion/xlsx`/
`ingestion/pdf` subpackages, with the OCR engine abstraction and local
Tesseract adapter in `ingestion/ocr`/`ingestion/ocr/tesseract` and PDF
page-image extraction in `ingestion/pdf/pdfimage`; deterministic,
rule/alias-based classification (deciding *which* canonical code a raw row
maps to) is implemented in `financial/classification`;
dataset-internal-consistency checking is implemented in
`financial/reconciliation`; derived financial metrics (EBITDA, SDE, working
capital, growth/volatility, etc.) are implemented in `financial/metrics`;
explicit normalization adjustments and the normalized EBITDA/SDE bridges
built from them are implemented in `financial/adjustments`; selecting a
single maintainable-earnings figure across historical periods is
implemented in `financial/earnings`; the individual valuation methods
themselves (SDE multiple, EBITDA multiple, capitalization of earnings, DCF,
adjusted net asset value) are implemented in `valuation` and its
per-method subpackages — see below for all six; optional, closed-set,
mandatory-human-review AI classification fallback is implemented in
`financial/classification/ai` and `financial/classification/ai/openai` —
see [AI fallback classification](#ai-fallback-classification). Method
applicability rules, a multi-method consensus/weighting engine, sensitivity
analysis, reporting/UI, every other form of AI/LLM assistance, a database,
and QuickBooks/Xero-style accounting-software integrations are all still
explicitly **out of scope
for this repository** at this stage. They are expected to be built as
later modules, or in the consuming application, on top of the types and
packages defined here — see
[Recommended next module](#recommended-next-module).

## Package boundaries

```
go-valuate/
  accounting/ledger/         general ledger / trial balance domain engine: chart of accounts, journal entries, balances, trial balance (built or imported), hierarchy rollups — independent of financial.Code (see docs/LEDGER.md)
  accounting/statements/     deterministic ledger-to-FinancialDataset bridge: account mapping (explicit/deterministic-suggestion/unmapped, account-type safety, sign normalization/contra accounts), income statement/balance sheet construction, mapping coverage/materiality, balance-sheet reconciliation — the ledger -> statements -> financial dependency boundary (see docs/STATEMENT_BUILDER.md)
  accounting/ar/             deterministic AR aging/collections analytics: portable open-receivables model (independent of accounting/ledger), due-date/invoice-date bucket aging, customer-level summaries, DSO, historical migration/trend, collection metrics, concentration (reuses analytics/concentration), deterministic collection-priority heuristic, GL control-account reconciliation (see docs/AR_AGING.md)
  accounting/ap/             deterministic AP aging/supplier payment analytics: portable open-payables model (independent of accounting/ledger and accounting/ar), due-date/bill-date bucket aging, supplier-level summaries, DPO (purchases or COGS-proxy basis), historical migration/trend, payment terms/timing, concentration (reuses analytics/concentration; AP-balance exposure, not vendor dependency), forward-looking due-date schedule, optional near-term payment-pressure analysis, GL control-account reconciliation (see docs/AP_AGING.md)
  accounting/journaldiagnostics/  deterministic journal-entry diagnostics for accountant/controller review (depends on accounting/ledger): manual/period-end/post-close/weekend/business-hours timing, round-dollar and median+MAD account-relative large-entry detection, rare/new-account and opposite-normal-balance movements, exact/possible duplicate and repeated-amount detection, explicit reversal-aware timing, threshold/split-entry clustering, description/reference/preparer-approver controls, source-mix and account-combination summaries — neutral review language only, no fraud/composite scoring (see docs/JOURNAL_DIAGNOSTICS.md)
  accounting/closequality/   deterministic bookkeeping-quality and period-close-readiness diagnostics composing accounting/ledger, accounting/statements, accounting/ar, accounting/ap, and accounting/journaldiagnostics (all optional): ledger/trial-balance integrity, financial-statement integrity, AR/AP control reconciliation, policy-dispositioned journal review, post-close activity, caller-declared account-expectation/suspense-clearing/stale-balance/rollforward checks, reconciliation-status and close-checklist status consumption (no matching/workflow engine of its own), expected-recurring-entry checks, coverage/applicability, deterministic dedup, cross-period comparison — no composite score, no audit opinion (see docs/CLOSE_QUALITY.md)
  accounting/cashforecast/  deterministic rolling 13-week (1-52 configurable) cash forecast for short-term liquidity: explicit weekly buckets from a caller-supplied ForecastStartDate, opening/restricted cash, portable dated CashFlowEvent model (known/scheduled/assumed/scenario basis), AR/AP scheduling adapters (AR due date is never auto-assumed as receipt date; AP due-date scheduling is opt-in only), recurring-event generation, payroll/tax/debt-service/capex/financing inputs, named what-if scenarios via pure event transforms, minimum-cash headroom/funding-gap/runway/single-upfront-funding-requirement, credit-facility capacity reporting (never auto-draws), unscheduled AR/AP and source-staleness reporting — distinct from analytics/forecast (long-range P&L/valuation projection), never invents a future cash flow (see docs/CASH_FORECAST_13_WEEK.md)
  ingestion/                 CSV/XLSX/PDF bytes -> raw tabular rows (no classification)
  ingestion/csv/              CSV parser (encoding/csv, zero external dependencies)
  ingestion/xlsx/              XLSX parser (github.com/xuri/excelize/v2)
  ingestion/pdf/               text-PDF parser (github.com/ledongthuc/pdf)
  ingestion/internal/tabular/  shared statement-interpretation logic (csv+xlsx+pdf)
  ingestion/fixtures/         CSV/XLSX/PDF fixture corpus + generators (fixtures/gen)
  financial/                 canonical financial model, taxonomy, and normalizer
  financial/classification/  deterministic raw-row -> canonical-code classifier
  financial/reconciliation/  dataset internal-consistency checks
  financial/metrics/         centralized derived-metrics engine (EBITDA, SDE, trends, ...)
  financial/adjustments/     explicit normalization adjustments -> normalized EBITDA/SDE bridges
  financial/earnings/        maintainable-earnings selection across historical periods
  analytics/qoe/             quality-of-earnings analysis: adjustment burden, recurrence, flags, score
  analytics/workingcapital/  operating working-capital history, statistics, seasonality, and peg analysis
  analytics/ratios/          financial-ratio suite: profitability/liquidity/leverage/efficiency/growth, trends, signals
  analytics/cashflow/        EBITDA-to-free-cash-flow bridge, conversion ratios, coverage, burn/runway
  analytics/revenuequality/  revenue composition, growth/volatility, customer retention, concentration
  analytics/concentration/   customer/vendor concentration risk: shares, HHI, dependency changes, loss scenarios
  analytics/anomalies/       expense anomaly / margin-leakage detection: deterministic spike/variance/pattern rules
  analytics/variance/        budget/forecast/prior-period vs. actual variance: line/category/bridge analysis
  analytics/forecast/        deterministic financial projections and scenario sets from caller-supplied assumptions
  analytics/debt/            debt service coverage, leverage, and caller-defined debt capacity (DSCR, amortization, scenarios)
  analytics/covenants/       caller-supplied covenant rules vs. already-calculated metrics: pass/fail/unavailable, headroom, warning buffer
  analytics/benchmarks/      caller-supplied company metrics vs. caller-supplied benchmark datasets: percentile/band placement, difference, favorable/unfavorable
  analytics/valuedrivers/    deterministic driver/scenario sensitivity: re-runs orchestrator+consensus under caller-defined metric/assumption changes, one-factor-at-a-time and combined
  analytics/consolidation/  multi-entity consolidation: caller-driven ownership weighting, explicit FX conversion and intercompany eliminations, period-alignment/reconciliation issues, produces a consolidated financial.FinancialDataset
  analytics/diagnostics/    single-business diagnostic engine: mines up to 15 optional sibling analytics/valuation results for their own already-computed flags/signals/anomalies/status classifications, republishes as categorized findings (strengths/concerns/opportunities), module coverage, optional overall health score — no new calculation, no narrative AI
  transactions/acquisition/ acquisition screening: price multiples, consensus premium/discount, financing/DSCR (via analytics/debt), returns, scenarios, caller-defined red flags
  transactions/dealstructure/ acquisition financing structure: sources and uses, debt tranches/seller note (own amortization engine incl. balloons), earnout schedule, funding gap/surplus
  transactions/salereadiness/ deterministic sale-readiness assessment: 11 dimension statuses from optional QoE/working-capital/concentration/revenue-quality/consensus/metrics results plus a business profile, blockers/risks/strengths/missing-information/opportunities, optional overall score
  portfolio/diagnostics/     multi-business portfolio scan: ranked findings (margin/revenue/cash/leverage/concentration/valuation/quality/sale-readiness) from condensed per-business summaries, configurable priority score, coverage/missing-data summary
  reporting/management/     presentation-neutral management-reporting data pack: KPI summary, historical/profitability/liquidity-leverage/cash-flow/working-capital series, variance/forecast tables, top-issues rollup, chart-ready series, from up to 13 optional sibling results — no PDF/HTML/chart rendering, no narrative AI
  valuation/                 common valuation result envelope (value types, bridge, issues)
  valuation/sde/              SDE multiple method
  valuation/ebitda/           EBITDA multiple method
  valuation/capitalization/   capitalization of earnings method
  valuation/dcf/               discounted cash flow method
  valuation/netassets/         adjusted net asset value method
  valuation/basis/            explicit value-basis conversion (enterprise/equity/asset)
  valuation/profile/          minimal business-description input to applicability
  valuation/applicability/    deterministic method-fit scoring, with a full explanation trail
  valuation/orchestrator/     runs a selected set of methods under a caller-chosen filter policy
  valuation/consensus/        multi-method consensus/dispersion, computed on one value basis
  valuation/sensitivity/      multiple/earnings/DCF-rate sensitivity grids
  valuation/report/           presentation-neutral, JSON-serializable report model
  valuation/e2e/               end-to-end fixture tests exercising the full pipeline
  settings/                  generic hierarchical settings resolver
  review/                    review Plan/Decisions/Apply/Readiness domain layer (see `review` below)
  fixtures/                  example JSON matching the Go types, used by tests
                             as living documentation
  docs/                      V1 contracts/integration/dependency/fixture-catalog docs (see below)
  examples/full_flow/         runnable, narrated demonstration of the complete V1 flow
```

### `ingestion`

Deterministic input adapters that convert tabular financial statement
exports (CSV, XLSX) into `financial.RawLineItem` values, sitting entirely
ahead of `financial/classification` in the pipeline:

```
CSV/XLSX bytes
   ↓
Tabular Ingestion   (ingestion, ingestion/csv, ingestion/xlsx)
   ↓
RawLineItem[]
   ↓
Classification      (financial/classification)
   ↓
User Confirmation   (future application — resolves UNKNOWN classifications)
   ↓
Normalize           (financial)
```

`ingestion` is developed independently of any future application in the
same way every other package here is: it is the input-adapter half of the
"no OCR" boundary described in
[What this project intentionally does not contain](#what-this-project-intentionally-does-not-contain)
— this base package and its `csv`/`xlsx` subpackages begin only once data
is already tabular (rows and columns; see [`ingestion/pdf`](#ingestionpdf)
for the text-based-PDF adapter, which begins once data is already text),
never attempt document understanding beyond that, and perform **no AI/LLM
inference** anywhere in their detection logic. Every parser is a pure
function of its input bytes/reader and `Options`: the same input always
produces the same
`Result`.

**Package layout.**

- **`ingestion`** — the shared, format-agnostic public API: `Options`,
  `Result`, `Row`, `Cell`, `Metadata`, `Warning`/`Error` taxonomies, and
  `Result.ToRawLineItems()`. Also hosts `BuildResult`, the shared
  orchestration entry point both `ingestion/csv` and `ingestion/xlsx` call
  so statement/period/structural interpretation is implemented exactly
  once and can never drift between formats.
- **`ingestion/csv`** — `Parse(io.Reader, Options) (*Result, *Error)` using
  only the Go standard library's `encoding/csv`. Zero external
  dependencies, matching the deterministic core's own dependency
  discipline.
- **`ingestion/xlsx`** — `Parse(io.Reader, Options) (*Result, *Error)`
  using `github.com/xuri/excelize/v2` (BSD-3-Clause; see XLSX dependency
  below). excelize's own types never appear in this package's exported
  API, so a caller depending on `ingestion/xlsx` never needs to import
  excelize directly, and the dependency could be swapped later without an
  API break.
- **`ingestion/internal/tabular`** — unexported, shared statement-
  interpretation engine: numeric parsing, period parsing, header/label-
  column detection, statement-type detection, structural row
  classification, and parent/section tracking, all operating on a plain
  `Grid [][]string` with no knowledge of `encoding/csv` or excelize. Both
  format packages translate their native rows into a `Grid` and call this
  package, so the CSV and XLSX adapters can never disagree about what
  counts as a subtotal, a period header, or a statement type.
- **`ingestion/fixtures`** — the fixture corpus (below) plus
  `ingestion/fixtures/gen`, a small `go run`-able generator for the binary
  XLSX fixtures (XLSX is a zip-based binary format unsuitable for hand
  authoring or text diffing).

**What ingestion decides vs. what it doesn't.** Ingestion determines
purely *structural* facts about a tabular document: which row holds
line-item labels, which columns are reporting periods, what a cell's
numeric value is, whether a row is blank/heading/subtotal/total, and
(best-effort) which section a row belongs to. It never decides that
`"Advertising"` maps to `OPEX_MARKETING` — that is exactly
`financial/classification`'s job, and duplicating any part of it here
would create two places that could disagree about the same question. A
`Row.Kind`/`Row.Status` is this package's own best-effort structural read;
`ToRawLineItems()` carries `Row.Kind` forward onto
`financial.RawLineItem.Kind` (a `financial.RowKind` — see
[`financial`](#financial) below) for every row, including headings, which
now survive `ToRawLineItems()` instead of being dropped.
`financial/classification.Classify` reads `RawLineItem.Kind` **first**: a
non-zero `Kind` (heading/subtotal/total) wins outright, and only when it is
the zero value does `Classify` fall back to its own narrower label-token
heuristic (`structuralTotalTokens`). This means `ingestion` and
`financial/classification` can no longer silently disagree about a row's
structural role — a label like `"Gross Profit"`, recognized as a subtotal
by `ingestion`'s label-shape detection (`ClassifyRowKind`) but not by
`financial/classification`'s own narrower total/subtotal/net token check,
now resolves consistently because ingestion's read reaches classification
directly via `Kind`. This was a known, documented gap in earlier revisions
of this README (see the "Gross Profit" case history removed from
[Known deterministic ingestion gaps](#known-deterministic-ingestion-gaps)
below) and is now closed.

**CSV support.** Comma, semicolon, and tab delimiters (auto-detected by
counting occurrences on the first line, or forced via `Options.Delimiter`);
UTF-8 byte-order marks (stripped); CRLF and LF line endings; quoted cells
with embedded delimiters/commas; blank rows; malformed rows (reported as
`MALFORMED_ROW_SKIPPED` warnings rather than aborting the whole parse,
since `encoding/csv`'s default behavior is to fail the entire read on one
bad row).

**XLSX support.** Deterministic worksheet enumeration; explicit sheet
selection via `Options.SheetName`; auto-selection of the single most
plausible non-empty sheet by a combination of sheet-name signal (`"P&L"`,
`"Balance Sheet"` score positively; `"Notes"`, `"Cover"` score negatively)
and row density; when more than one sheet ties for most-plausible,
`Parse` returns an **empty result with a `MULTIPLE_PLAUSIBLE_SHEETS`
warning** rather than silently guessing — sheet selection is always
deterministic and explainable, never arbitrary. Cell values are read via
excelize's `GetRows`, which surfaces a formula cell's cached/calculated
result (exactly what every real authoring application — Excel, Google
Sheets, QuickBooks — persists alongside the formula) rather than
re-evaluating the formula; a formula cell with no usable cached value
produces a `FORMULA_WITHOUT_CACHED_VALUE` warning instead of a fabricated
number. Formula text is preserved on `Cell.Formula` when present.

**XLSX dependency.** [`github.com/xuri/excelize/v2`](https://github.com/qax-os/excelize)
(pinned at v2.11.0), **BSD-3-Clause** license — mature, actively
maintained, one of the most widely used Go Excel libraries, chosen over a
hand-rolled Office Open XML reader because correctness and maintainability
of a binary zip/XML-based format matter more here than preserving the
deterministic core's zero-dependency property, which was never a goal for
the ingestion layer specifically (see the task brief this package was
built against). The dependency is isolated behind `ingestion/xlsx`;
`excelize.File` and every other excelize type stay entirely internal to
that package.

**Statement-type detection** (`INCOME_STATEMENT`, `BALANCE_SHEET`,
`CASH_FLOW_STATEMENT`, or undetected/`UNKNOWN`) runs a fixed
signal-priority pipeline and always records its evidence in
`Metadata.StatementTypeEvidence`:

1. **sheet name** — e.g. a sheet literally named `"Balance Sheet"`.
2. **title rows** — non-blank rows above the detected period-header row,
   matched against phrases like `"profit and loss"`, `"statement of
   financial position"`, `"statement of cash flows"`.
3. **line-item labels** — distinctive structural terms anywhere in the
   sheet (`"gross profit"`, `"total assets"`, `"cash flows from
   operating"`, etc.), counted per statement type; the type with strictly
   the most matches wins.

A tie at the line-item tier, or no signal at any tier, leaves
`Metadata.StatementTypeUnknown = true` rather than forcing a guess — see
`WarnStatementTypeUnknown`. `Options.StatementTypeOverride` bypasses all of
this when the caller already knows the statement type.

**Period detection** (`Metadata.Periods`, each a `DetectedPeriod`) scans
up to the first 15 non-blank rows for the row with the most cells that
parse as a period, selecting it as the header row (ties go to the row with
more period-like columns; multiple plausible header rows are reported via
`MULTIPLE_POSSIBLE_HEADER_ROWS`). Recognized label shapes, each producing
an explicit `PeriodType` and canonical ID:

| Example label | `PeriodType` | Canonical ID |
|---|---|---|
| `2025` | `calendar_year` | `2025` |
| `FY2025`, `FY 2025`, `FY'25` | `fiscal_year` | `FY2025` |
| `Q1 2025`, `2025 Q1`, `Q1'25` | `quarter` | `2025Q1` |
| `12/31/2024`, `Dec 31 2024`, `December 31, 2024`, `2024-12-31` | `calendar_year` | `2024` |
| `March 2025`, `Mar 2025`, `03/2025` | `month` | `2025-03` |
| `Jan-Dec 2024` | `calendar_year` | `2024` |
| `Year Ended December 31, 2024`, `For the Year Ended Dec 31 2024` | `calendar_year` | `2024` |
| `YTD`, `YTD 2024` | `ytd` | `ytd`, `2024-YTD` |
| `Current Year`, `Prior Year` (no absolute date) | `unknown` | normalized label |

When a header label matches none of these deterministic rules, ingestion
**never fabricates a date**: `PeriodType` is `unknown`, the canonical ID
falls back to a normalized (lowercased, underscored) form of the original
label, `Confidence` is `0`, and a `PERIOD_LABEL_AMBIGUOUS` warning is
emitted — `DetectedPeriod.OriginalLabel` always preserves the exact source
text regardless. `Options.PeriodColumnOverrides` lets a caller pin specific
columns to specific canonical periods, bypassing detection entirely for
those columns.

**Numeric parsing** supports common North American financial formatting:
thousands separators (`"1,234.56"`), a `$` prefix or suffix, parentheses as
negative (`"(1,234.56)"` → `-1234.56`), a leading `-`/`+` sign in any order
relative to a currency symbol (`"-$1,234.56"` and `"$-1,234.56"` both
work), and blank cells. A bare dash/em-dash (`"-"`, `"—"`, `"--"`) is
`Options.DashTreatment`-controlled: `DashAsBlank` (default, no value) or
`DashAsZero`. A cell ending in `%` is never coerced into a monetary value
— it is reported as an `UNPARSEABLE_NUMERIC_CELL` warning, since a
percentage silently becoming `12` or `0.12` would corrupt aggregation
without any signal that something went wrong. Any other text that doesn't
cleanly reduce to a signed decimal (`"N/A"`, `"TBD"`, `"1,234.56 USD"`,
double-negative forms like `"(-1,234.56)"`) is likewise reported as a
warning and **never silently becomes zero**.

**Structural/parent detection.** Each row is classified as `blank`
(no text anywhere), `heading` (has label text but no numeric/dash values —
a section header like `"Operating Expenses"`), `subtotal`/`total` (label
starts with `"total"`/`"subtotal"`, or is a recognized whole-statement
total phrase like `"Net Income"`/`"Total Assets"`), or `normal`. Blank rows
are always excluded from `Result.Rows` (their `RowIndex` is never
renumbered around them, so source position stays traceable). Heading rows
are retained in `Result.Rows` **and** included in `ToRawLineItems()` output
(as of `financial.RawLineItem` gaining a `Kind` field — see
[`financial`](#financial) below): a heading becomes a `RawLineItem` with
`Kind = financial.RowKindHeading` and empty `Values`, so a caller can
display section headings from the same `[]financial.RawLineItem`/
`[]financial.MappedLineItem` slices it already has, without separately
keeping `Result.Rows` around and re-correlating by `RowID`.
`Row.ParentLabel` tracks the innermost currently-open heading section using
a simple indent-aware stack (`ParentTracker`): a heading row opens a
section at its indent level, a subtotal/total row at or below that level
closes it, and every other row inherits the innermost open section's label
— see the `IndentLevel` field for the (deliberately coarse) whitespace-based
signal this is built on.

**Security and limits.** Every parser treats its input as untrusted:
macros are never executed (`encoding/csv` and excelize implement no
macro/VBA runtime at all); formulas are never evaluated — only a
workbook's own cached value is read; external workbook links are never
followed or refreshed (`excelize.File.UpdateLinkedValue` is never called).
`Options.Limits` (defaults via `DefaultLimits()`) bounds `MaxFileSizeBytes`
(checked before the archive is even opened, and used to bound excelize's
own `UnzipSizeLimit`/`UnzipXMLSizeLimit` against a zip-bomb-style
pathological input), `MaxSheets`, `MaxRows`, `MaxColumns`, and
`MaxCellTextLength` (excess cell text is truncated with a
`CELL_TEXT_TRUNCATED` warning rather than silently accepted). Every limit
violation is a fatal `LIMIT_EXCEEDED` error, not a warning, so a caller
never processes a partially-truncated result without knowing it happened.

**Warning and error taxonomy.** Non-fatal issues (`Warning`, always
returned alongside a valid `Result`) use stable `WarningCode` values:
`PERIOD_LABEL_AMBIGUOUS`, `STATEMENT_TYPE_UNKNOWN`,
`MULTIPLE_POSSIBLE_HEADER_ROWS`, `UNPARSEABLE_NUMERIC_CELL`,
`FORMULA_WITHOUT_CACHED_VALUE`, `MULTIPLE_PLAUSIBLE_SHEETS`,
`MALFORMED_ROW_SKIPPED`, `EMPTY_SHEET_SKIPPED`, `CELL_TEXT_TRUNCATED`,
`STRUCTURAL_INTERPRETATION_UNCERTAIN`. Fatal issues (`*Error`, returned
alone with a nil `*Result`) use stable `ErrorCode` values:
`INVALID_FILE`, `NO_TABULAR_DATA`, `LIMIT_EXCEEDED`,
`UNSUPPORTED_FORMAT`. Both mirror the rest of this repository's
stable-code convention (`financial.Code`, `reconciliation.CheckCode`,
`adjustments.Type`) rather than requiring callers to parse human-readable
strings.

**Deterministic row IDs.** Every `Row.ID` is shaped `sheet-<index>-row-<n>`
(0-based sheet index, 0-based row index as it appeared in the source) —
stable within a single parse, never a UUID, and never renumbered around
skipped blank rows. A future application attaching persistent database IDs
does so on top of this, not instead of it.

**Integration with classification.** `Result.ToRawLineItems()` converts
every non-blank row (including headings — see above) into a
`financial.RawLineItem`, ready for `classification.ClassifyBatch` exactly
as shown in the diagram above; see
[`ingestion/integration_test.go`](ingestion/integration_test.go) for the
full chain exercised against real fixtures, through
`financial.Normalize`, and (for the balance sheet fixture) into
`financial/reconciliation` — including
`TestStructuralContract_LabelsSurviveAsStructuralNotOrdinaryAccounts`,
which proves "Gross Profit", "Total Operating Expenses", "Net Income", and
"Total Assets" all resolve to `classification.SourceStructural` (never an
ordinary account code) and never appear in the normalized
`FinancialDataset`, across both the CSV and XLSX adapters.

**Fixture corpus** (`ingestion/fixtures`): a QuickBooks-style P&L
(`quickbooks_pl.csv`, nested Income/COGS/Expense sections with a Gross
Profit subtotal), an accountant-custom multi-period P&L
(`accountant_custom_pl.csv`, FY2024/FY2025 columns, parenthetical negative
interest expense), a balance sheet (`balance_sheet.csv`, nested
current/fixed asset and liability sections), a four-year SaaS P&L
(`multi_year_saas_pl.csv`), a fixture exercising every malformed-numeric
case at once (`malformed_numeric_values.csv`), a fixture with sparse/blank
separator rows throughout (`sparse_blank_rows.csv`), a three-sheet XLSX
workbook exercising auto-selection (`multi_sheet_workbook.xlsx`: a
low-plausibility "Notes" sheet plus Income Statement and Balance Sheet
sheets), a deliberately ambiguous two-sheet XLSX workbook
(`ambiguous_workbook.xlsx`), and an XLSX workbook exercising nested
totals/subtotals with indentation (`totals_subtotals.xlsx`). No proprietary
or customer data is used anywhere in the corpus.

**Explicit non-goals**, matching this repository's constraints exactly:
no OCR, no LLM/AI interpretation or ML classification (statement-type/
period/structural detection are 100% rule-based, same as
`financial/classification`), no database persistence, no file-upload HTTP
endpoints, no frontend, no QuickBooks/Xero API integration, no automatic
add-back/adjustment recommendations, no external financial-data sources.
Text-based PDF ingestion is covered separately below — see
[`ingestion/pdf`](#ingestionpdf).

### `ingestion/pdf`

Deterministic ingestion of **text-based (born-digital) PDF** financial
statements — PDFs with an actual embedded text layer, as virtually every
PDF exported directly from accounting software, Excel, or a word processor
has. This is explicitly **not OCR**: a PDF with no usable embedded text
(an image-only or scanned statement) returns an `OCR_REQUIRED` error
rather than attempting any image-understanding step — see
[OCR-required detection](#ocr-required-detection) below.

```
PDF bytes/io.Reader
   ↓
pdf.Parse
   ↓
pdf.Results{ Statements []ingestion.Result, ... }
   ↓ (one Result per detected statement — see Multiple statements below)
Result.ToRawLineItems()
   ↓
RawLineItem[]
   ↓
Classification      (financial/classification)
   ↓
User Confirmation   (future application)
   ↓
Normalize           (financial)
```

**PDF dependency.** [`github.com/ledongthuc/pdf`](https://github.com/ledongthuc/pdf)
(pinned at the version recorded in `go.mod`/`go.sum`), **BSD-3-Clause**
license — a pure-Go, dependency-free PDF reader (a maintained fork of the
archived `rsc.io/pdf`, itself originally written by The Go Authors). It
was verified directly (not merely assumed from its documentation) to
expose usable per-glyph X/Y-positioned text before being adopted: a small
hand-constructed PDF with a proper font `/Widths` array was parsed and its
`Page.Content().Text` confirmed to return correct, monotonically advancing
per-character X positions and stable per-line Y positions, which this
package's row/column reconstruction (below) depends on. The dependency is
isolated entirely behind `ingestion/pdf`: no `ledongthuc/pdf` type appears
in any exported `ingestion/pdf` API (see `extract.go`, the only file in
this package that imports it). It implements no JavaScript engine, no
hyperlink/attachment/launch-action execution, and no macro/VBA runtime —
it is a structural/content-stream reader only, so unlike a full-featured
PDF viewer there is no such execution surface to disable in the first
place. `ledongthuc/pdf` uses panic-based internal error handling for
malformed object graphs (some inherited from its `rsc.io/pdf` ancestry);
`ingestion/pdf` wraps every call into it with its own `recover()` (see
`extract.go`), converting any panic into an `INVALID_FILE` error, so a
malicious or corrupt PDF can never crash the calling process.

**Positioned-text extraction.** `ledongthuc/pdf`'s `Page.Content()`
returns one `Text` primitive per glyph/short run — page, X, Y, font,
font size, and advance width — not one primitive per word or line (a PDF
content stream has no inherent concept of either; see `extract.go`).
Extraction order is never assumed to equal visual reading order: every
fragment carries its own absolute page-relative X/Y, and reconstruction
(below) sorts and groups from there.

**Row reconstruction** (`layout.go`). Two stages: (1) **word grouping** —
adjacent same-line fragments are merged into words when their horizontal
gap is under 30% of the font size (`defaultWordGapFactor`), splitting on
any larger gap; (2) **row grouping** — words are clustered into lines by
Y-proximity within `Options.RowYTolerance` (default 2.0 points —
`defaultRowYTolerance` — chosen to absorb the sub-point Y jitter real PDF
producers introduce between glyphs on one nominal baseline, while staying
well under any realistic line-to-line spacing). Left-to-right order within
a line is preserved via an X sort, independent of extraction order.

**Column reconstruction** (`columns.go`). Column boundaries are never
assumed at fixed X coordinates: when a recognizable period-header line
exists (reusing `ingestion/internal/tabular.ParsePeriodLabel` per
candidate word — never a separate reimplementation), its own word X
positions anchor the columns directly; otherwise, boundaries are inferred
by greedily clustering every word's start-X across the whole section, with
a gap threshold of 300% of the dominant font size
(`defaultColumnGapFactor`) separating genuinely different columns from the
natural X variance of right-aligned numbers with differing digit counts. A
multi-word label (e.g. "Total Operating Expenses" split into three words
by row reconstruction) reassembles into one grid cell because every word
composing it falls within the label column's X region.

**Multi-page statements.** A statement's rows continue across pages
seamlessly — `Row.ParentLabel` section context (via
`ingestion/internal/tabular.ParentTracker`, unmodified) survives a page
break exactly as it survives any other row transition, and every row
carries its exact source `PageIndex`. Repeated page headers/column
headings (a title or period-header block reprinted at the top of every
page) are detected by exact text match against an earlier line in the
same section and suppressed (`REPEATED_HEADER_REMOVED`), never becoming
duplicate rows. A page-footer/page-number line (`"Page 3"`, `"Page 3 of
12"`, a bare number) positioned in a page's bottom margin is dropped
silently (`repeated.go`'s `looksLikePageFooter`) — never flagged as a
warning, since it was never financial content to begin with.

**Multiple statements per PDF — Option A.** A single PDF commonly
contains more than one statement (e.g. an income statement followed by a
balance sheet in one filing). `pdf.Parse` never silently combines them:
it returns `pdf.Results{ Statements []ingestion.Result }`, one `Result`
**per detected statement section**, each independently statement-typed —
a caller never needs a second call or an option to get every statement
out, and a single-statement PDF is simply the same shape with
`len(Statements) == 1`. Boundaries are detected from deterministic
signals only (`detect.go`'s `splitSections`): a short, title-shaped line
matching a *different* statement type than the currently open section
starts a new one (a REPEATED occurrence of the *same* type, e.g. a title
reprinted per page, does not); an unusually large vertical gap on the same
page with no confirming keyword signal on either side also splits, but is
flagged `STATEMENT_BOUNDARY_AMBIGUOUS` since no evidence confirms it is
genuinely a different statement rather than a visual section break within
one. `MULTIPLE_STATEMENTS_DETECTED` is emitted at the document level
whenever more than one section results. Leading preamble text (e.g. a
company-name line before the actual "Income Statement" title line) is
folded into the section that follows it, never treated as its own
statement.

**Statement/period/numeric reuse — no reimplementation.** Every bit of
header-row detection, period parsing, statement-type detection, and
structural row classification is driven through the exact same
`ingestion.BuildResult` entry point `ingestion/csv` and `ingestion/xlsx`
already call (`build.go`): once a section's lines are reshaped into a
`tabular.Grid` (the identical `[][]string` shape both other formats
produce), this package has zero further statement-interpretation logic of
its own. The only genuinely PDF-specific work is (1) getting from
positioned glyphs to a `Grid` in the first place (extraction/layout/column
reconstruction, above) and (2) a small set of documented PDF
extraction-quirk accommodations layered strictly ahead of the existing
parsers, never replacing them:

- **Numeric parsing** (`numeric.go`): `"$ 1,234.00"` (spaced currency
  prefix), `"( 1,234 )"` (spaced parentheses), `"1,234 -"` (a trailing
  minus sign separated from the digits by a gap — rewritten to the
  leading-minus form `tabular.ParseNumeric` already understands), and
  `"1 234"` (space-thousands separator, anchored so it never matches
  running text) are all normalized to the exact form
  `ingestion/internal/tabular.ParseNumeric` already parses correctly,
  before handing off to that unmodified function. A value not matching
  one of these four documented patterns is passed through completely
  untouched — this package can never mask a genuinely malformed value;
  it fails exactly as CSV/XLSX would, reported as `UNPARSEABLE_PDF_VALUE`
  (distinct from `UNPARSEABLE_NUMERIC_CELL` specifically for a cell that
  needed PDF normalization and still failed afterward, so a caller can
  tell "malformed in the source" apart from "a PDF-extraction-layout
  ambiguity").
- **Indentation** (`build.go`): rather than a parallel X-offset-based
  indent algorithm, a label word's X-offset from the section's baseline
  label X is converted into synthetic leading spaces (one indent step —
  150% of the dominant font size — per 2 spaces), so
  `ingestion/internal/tabular.DetectIndentLevel` (which reads leading
  whitespace — the same signal XLSX cell text already carries) runs
  completely unmodified for PDF's X-offset indentation too.

**Structural detection.** Heading/subtotal/total rows are detected via
`ClassifyRowKind` exactly as for CSV/XLSX (same label-shape rules,
running on the same reconstructed `Grid`) — see
[Structural row contract](#structural-row-contract-financialrowkind)
below and `financial/classification`'s `RawLineItem.Kind`-first
precedence, which this package benefits from identically to CSV/XLSX with
no PDF-specific code.

**OCR-required detection.** A PDF is treated as having no usable text
layer — returning a fatal `OCR_REQUIRED` `*ingestion.Error`, never an
empty-but-successful result — when EITHER: zero text fragments are
extracted across the whole document; OR the total extracted fragment
count is below `minTextFragmentsForUsableDocument` (20 — a real
single-page statement, even a very short one, produces comfortably more
than this from its title and a handful of line items alone); OR the
average fragment count per page is below `minFragmentsPerPageRatio` (3.0).
Both thresholds are deliberately conservative in the direction of never
false-triggering on a real, if sparse, text document. A page that
produces zero text within an otherwise-usable document (e.g. one scanned
exhibit page inserted into a normal text statement) does not fail the
whole parse — it is reported per-page as `PDF_TEXT_LAYER_MISSING` instead.
`ingestion/pdf` never attempts OCR under any circumstance; this is a
hard, permanent boundary, not a "not implemented yet."

**PDF-specific security/limits.** `ingestion.Limits` gained three
additive PDF-only fields (zero value = the `DefaultLimits()` default,
identical pattern to every existing limit): `MaxPages` (default 500 —
exceeding it is a fatal `PDF_PAGE_LIMIT_EXCEEDED`, never a silent partial
read that could drop financial rows), `MaxTextFragments` (default
2,000,000, bounding parser work against a pathologically dense or
maliciously crafted PDF), and `MaxTextLengthPerPage` (default 200,000
runes). Both text limits fail as `PDF_TEXT_LIMIT_EXCEEDED`. As documented
above under PDF dependency, `ledongthuc/pdf` implements no JavaScript,
hyperlink/launch-action, embedded-file, or macro execution of any kind —
confirmed directly from its source, not assumed — so there is no such
surface for this package to additionally disable. External workbook-style
links have no PDF equivalent to follow. Every parse wraps
`ledongthuc/pdf` calls in `recover()` (see PDF dependency above) so a
malformed/malicious PDF can only ever produce a clean `INVALID_FILE`
error, never a crash.

**Options.** `pdf.Options` embeds `ingestion.Options` for every field
shared with CSV/XLSX (`Locale`, `DashTreatment`, `Limits`,
`PeriodColumnOverrides`, `LabelColumnOverride`, `HeaderRowOverride`,
`StatementTypeOverride`, etc. — reused directly, not duplicated) and adds
three PDF-only fields: `PageStart`/`PageEnd` (1-based, inclusive,
restricts extraction to a page range) and `RowYTolerance` (see Row
reconstruction above). Auto-detection remains the default for everything;
these are all opt-in overrides, matching the CSV/XLSX `Options`
philosophy exactly. A `StatementSection`-style option was deliberately
NOT added: Option A already returns every detected statement from a
single call, so there is nothing such an option would select.

**Warning and error taxonomy — extended, not duplicated.** New stable
`WarningCode` values (added to the *same* `ingestion.WarningCode`
taxonomy CSV/XLSX already use, per this repository's one-taxonomy
discipline): `PDF_TEXT_LAYER_MISSING`, `PDF_LAYOUT_AMBIGUOUS` (row
reconstruction found no clean line grouping — most lines in a section
reconstructed as a single word), `MULTIPLE_STATEMENTS_DETECTED`,
`STATEMENT_BOUNDARY_AMBIGUOUS`, `COLUMN_ALIGNMENT_AMBIGUOUS` (column
reconstruction produced an unusually sparse grid), `REPEATED_HEADER_REMOVED`,
`UNPARSEABLE_PDF_VALUE`. New `ErrorCode` values (same `ingestion.ErrorCode`
taxonomy): `OCR_REQUIRED`, `PDF_PAGE_LIMIT_EXCEEDED`,
`PDF_TEXT_LIMIT_EXCEEDED`. `ingestion.Row` gained `PageIndex int` (always 0
for CSV/XLSX) and `ingestion.Cell` gained an optional `Bounds *CellBounds`
(nil for CSV/XLSX, and nil for any PDF cell this package could not
confidently attribute a single bounding box to).

**Fixture corpus** (`ingestion/fixtures`, generated by
`ingestion/fixtures/gen` alongside the existing XLSX generator — see that
package's `pdf_writer.go` for why a small hand-rolled PDF writer was used
instead of a second PDF-writing dependency: this fixture corpus needs
nothing beyond positioned Helvetica text across one or more pages, which
a few hundred lines of deterministic code produce exactly as reliably as
a full library, at zero added dependency cost):
`simple_pl.pdf` (one-page income statement), `multi_year_pl.pdf`
(three period columns), `multi_page_pl.pdf` (two pages, repeated
title/header block on page 2, parent context spanning the page break),
`balance_sheet.pdf` (nested asset/liability/equity sections with
subtotals and a grand total), `pl_and_balance_sheet.pdf` (an income
statement and a balance sheet in one two-page PDF — the Option A
multi-statement fixture), `negative_parentheses.pdf` (including the
"( 1,234 )" spaced-parentheses extraction quirk),
`indented_sections.pdf` (X-offset indentation under a heading),
`unusual_spacing.pdf` (every documented numeric extraction quirk at
once), `image_only.pdf` (zero text objects — only a drawn rectangle — for
OCR-required-detection testing; a real scanned-looking raster image is
unnecessary since the trigger is "no usable extractable text," which a
text-free content stream demonstrates just as validly and far more
deterministically), and `ambiguous_layout.pdf` (scattered text with no
discernible row/column alignment, for `PDF_LAYOUT_AMBIGUOUS`/
`COLUMN_ALIGNMENT_AMBIGUOUS`). No proprietary or customer data anywhere in
the corpus, matching the CSV/XLSX corpus's identical constraint.

**Known PDF-specific limitations**, in the same spirit as
[Known deterministic ingestion gaps](#known-deterministic-ingestion-gaps)
below:

- **Fixed-width/monospace or unusual embedded fonts with no `/Widths`
  array can degrade per-character X positioning.** `ledongthuc/pdf`'s
  `Font.Width()` reads only an explicit `/Widths` array and does not fall
  back to built-in AFM metrics for an unembedded base-14 font; this was
  confirmed directly during this package's own dependency evaluation (see
  PDF dependency above). Virtually every real-world PDF producer
  (accounting software, Excel, word processors, "Print to PDF") embeds
  `/Widths` explicitly, so this is a narrow edge case in practice, but a
  PDF from an unusual producer that omits it could see word-grouping
  degrade.
- **Merged/rotated/vertical text, and multi-row (wrapped) headers, are
  not specifically handled** — same limitation CSV/XLSX already document
  for wrapped headers, inherited here since header handling reuses the
  identical `ingestion.BuildResult` logic.
- **A statement laid out as true side-by-side columns spanning the full
  page width with no shared period header** (rare in practice; most
  multi-column financial statements share one header row) relies entirely
  on `clusterColumnXs`'s gap-based inference, which is less certain than
  header-anchored column detection — `COLUMN_ALIGNMENT_AMBIGUOUS` is the
  signal to watch for here.
- **Tables with visible ruling lines (drawn rectangles/lines as cell
  borders) are not used as a column-boundary signal** — only text
  positioning drives reconstruction; a ruled table with unusual text
  spacing that doesn't align with its own ruling lines could reconstruct
  less accurately than the ruling would suggest.
- **Locale is inherited from CSV/XLSX's single supported value
  (`LocaleEnUS`)** — a European-formatted PDF (`.` thousands, `,`
  decimal) is out of scope for the same reason it already is for
  CSV/XLSX (see Known deterministic ingestion gaps below).

## Scanned/image PDF support (OCR)

With `ingestion/pdf` covering born-digital text PDFs, the remaining gap —
a **scanned or image-only PDF**, with no embedded text layer at all — is
closed by three additional packages: `ingestion/ocr` (an engine-agnostic
OCR abstraction), `ingestion/ocr/tesseract` (a local Tesseract adapter),
and `ingestion/pdf/pdfimage` (PDF page-image extraction), plus OCR
integration inside `ingestion/pdf` itself (`ocr_*.go`). OCR is entirely
**opt-in**: every existing `pdf.Parse` caller sees no behavior change
whatsoever — a scanned PDF still returns `OCR_REQUIRED` unless a caller
explicitly calls `pdf.ParseWithOCR` with `Options.OCR` set to `OCRAuto` or
`OCRForce` and supplies an `ingestion/ocr.Engine`.

```
                       ┌─ embedded text ─────────┐
PDF ─→ page inspection                           ├→ positioned layout
                       └─ scanned image → OCR ──┘
                                                     ↓
                                            statement parser
                                                     ↓
                                                RawLineItem
                                                     ↓
                                             classification
                                                     ↓
                                               normalization
```

**No second interpretation pipeline.** This is the load-bearing design
constraint of the entire OCR addition: OCR-recognized words are converted
(`ocr_layout.go`) into the *exact same* `word`/`line` types
`ingestion/pdf`'s embedded-text extraction already produces (`layout.go`),
then flow through the *identical, unmodified* row/column reconstruction
(`groupRows`, `detectColumnBoundaries`/`buildSectionGrid` —
`columns.go`/`build.go`), statement-boundary detection (`splitSections` —
`detect.go`), and `ingestion.BuildResult` entry point every other format
(CSV, XLSX, text PDF) already drives. There is exactly one
statement-interpretation pipeline in this repository; OCR only supplies a
second SOURCE of positioned text into it, never a competing interpreter.

### `ingestion/ocr`

Defines `Engine`, the one interface every OCR backend implements:

```go
type Engine interface {
    Recognize(ctx context.Context, image ImageInput, opts Options) (Result, error)
}
```

`ImageInput` wraps a standard `image.Image` (aliased as `DecodedImage`)
plus a page index and optional DPI hint. `Result` carries `[]Word` —
positioned recognition output with `Text`, `Confidence`, `X`/`Y`/`Width`/
`Height` (source-image pixel coordinates), `PageIndex`, and the engine's
own block/paragraph/line grouping IDs when available — plus `EngineName`/
`EngineVersion`. **Confidence is never treated as a probability** (see the
package doc comment): it is carried through unmodified from whatever scale
the underlying engine reports (Tesseract: 0–100, a heuristic score with no
statistical grounding), useful only as a relative, engine-internal signal.

A `FakeEngine` (not test-file-gated, so `ingestion/pdf`'s own tests can
import it directly) provides a fully deterministic, in-memory `Engine`
implementation for testing every layer downstream of OCR recognition
(positioned-word handling, confidence propagation, numeric ambiguity,
fallback mode selection, mixed text/OCR pages, timeout/error propagation)
without requiring Tesseract — or any real OCR engine — installed
anywhere. Every OCR test in this repository except the explicitly-gated
`Tesseract*Integration*` tests uses `FakeEngine`.

Errors are a small stable `ErrorCode` set (`OCR_ENGINE_UNAVAILABLE`,
`OCR_ENGINE_FAILED`, `OCR_TIMEOUT`) wrapped in `*ocr.Error`, mirroring
`ingestion.Error`'s Code/Message/Detail shape.

### `ingestion/ocr/tesseract`

A local-Tesseract `Engine` implementation, invoking a locally installed
`tesseract` executable via `os/exec` — never CGO, never a native link
dependency, **never through a shell** (every argument is a separate
`exec.CommandContext` argument; no string-built command line, no `sh -c`).

**Runtime dependency, not a build dependency.** This is the load-bearing
requirement: the entire `go-valuate` module, including this package,
compiles with `go build ./...` on a machine with **no Tesseract
installed at all** — confirmed directly in this repository's own CI-style
verification, which never has Tesseract present. Tesseract availability is
discovered only when `Engine.Recognize` (or `Engine.Available()`) is
actually called: a missing/unresolvable executable produces a structured
`*ocr.Error{Code: OCR_ENGINE_UNAVAILABLE}`, never a build failure, never a
panic. `Engine.ExecutablePath` is configurable (default: bare `"tesseract"`,
resolved via `PATH`).

**Dependency/license/version, verified rather than assumed:**

| | |
|---|---|
| Tesseract OCR | External executable, **Apache License 2.0**. Never vendored, compiled, or linked into this Go module or its binary in any form — a caller who never calls `Engine.Recognize` never touches Tesseract at all. |
| Minimum version | 4.0+ (LSTM engine, stable `tsv` output format). Not tested against the legacy Tesseract 3.x engine. |
| Language packs | English only (`eng`) for this repository's MVP scope — the caller's system must have `tesseract-ocr-eng` (or the distribution equivalent) installed alongside the executable; this package does not download, bundle, or manage language data. |

**Output format.** Tesseract is invoked as `tesseract <in> <out> -l eng
tsv`, producing Tesseract's own TSV format (`level page_num block_num
par_num line_num word_num left top width height conf text`) — the one
Tesseract output format that supplies text, confidence, bounding boxes,
and line/block grouping all in one deterministic, line-oriented file
(`tsv.go`'s `parseTSV`). Only level-5 (word) rows are kept; the
line/paragraph/block aggregate rows Tesseract also emits are discarded,
since this repository re-derives row/line grouping itself from word
positions (see "no second interpretation pipeline" above) rather than
trusting any engine's own grouping as authoritative.

**Temporary files.** Tesseract's CLI is file-based (no reliable
positioned-output stdin/stdout mode across versions), so one temporary
input PNG and one temporary TSV output are created per `Recognize` call,
via `os.CreateTemp` (OS temp dir by default, `Engine.TempDir` overridable;
unpredictable names; `0600` permissions where the platform supports it)
and unconditionally removed via `defer`-guarded cleanup regardless of
success/failure — no name reuse between calls, no persisted upload data.
No global mutable state anywhere in this package: an `Engine` value holds
only its own configuration and is safe for concurrent use.

**Stdout/stderr** are captured into bounded in-memory buffers (a
`boundedWriter` caps stderr at 64 KiB) — never connected to the parent
process's own streams, never unbounded.

### `ingestion/pdf/pdfimage`

Extracts a scanned PDF page's dominant embedded raster image, isolating
`github.com/pdfcpu/pdfcpu` behind this package's own types — no `pdfcpu`
type appears in its exported API (`extract.go` is the only file that
imports it directly).

**Dependency, verified rather than assumed:** `github.com/pdfcpu/pdfcpu`
(pinned in `go.mod`/`go.sum`), **Apache-2.0**. Confirmed directly (not
assumed from documentation) before adoption: actively maintained (roughly
monthly releases), a dependency tree free of GPL/AGPL (every transitive
dependency — `hhrutter/tiff`, `mattn/go-runewidth`, `clipperhouse/uax29/v2`,
`go.yaml.in/yaml/v3`, the `golang.org/x/*` packages — is MIT, BSD-3, or
Apache/MIT dual-licensed), and its `pkg/api` image-extraction functions
(`ExtractImagesRaw`, `PageDims`, `PageCount`) operate on `io.ReadSeeker`,
never requiring a filesystem-only API. Pure Go: no CGO, no native link
dependency, so it adds no build requirement beyond what `go build` already
needs. **Known gap:** JBIG2-encoded monochrome scans come back from
`pdfcpu` as a raw undecoded stream (no built-in JBIG2 decoder) — this
package does not attempt to decode that raw stream (silently
misinterpreting it as a different format would be worse than skipping it),
so a JBIG2-only scan page reports `UNSUPPORTED_SCANNED_PDF_LAYOUT`. JPEG
(`DCTDecode`) and CCITT Group 3/4 fax (`CCITTFaxDecode`, the standard
monochrome-scan encoding), plus raw raster (`FlateDecode`/`LZWDecode`
across every common PDF color space), are fully decoded to a standard
`image.Image`.

**Scanned-PDF support boundary — the common case only.** The first
implementation supports exactly one shape: **one dominant, full-page
raster image per scanned page.** A page whose real content is genuine
vector-drawn material (not a raster scan at all) — which would require
true PDF page rendering, which this package does not implement — reports
`UNSUPPORTED_SCANNED_PDF_LAYOUT` (surfaced as the fatal
`PDF_PAGE_RENDER_REQUIRED` error from `ingestion/pdf`). **Correctly
rejecting an unsupported scan is treated as strictly better than producing
bad financial data** — this package never silently OCRs an arbitrary
embedded logo or partial image as if it were the page.

**Dominant-image selection (`dominant.go`) — never "pick the largest byte
stream."** When a page contains multiple embedded images (a scan plus a
letterhead logo is common), `SelectDominantImage` uses **aspect-ratio
shape matching** against the PDF page's own `MediaBox` proportions as the
deciding signal, not an assumed absolute DPI or raw pixel/byte volume: a
genuine full-page scan has the same width:height ratio as its page
regardless of scan resolution, while a logo/icon/stamp almost always has a
very different one. (An earlier assumed-DPI area-projection design was
tried and rejected during this implementation specifically because it
conflated "a small logo" with "a genuinely low-resolution full-page scan"
— both look small under a fixed-DPI area projection, but only aspect
ratio correctly tells them apart independent of resolution.) A secondary
absolute-pixel-size floor still rejects a tiny thumbnail that happens to
share the page's proportions. Exactly one page-shaped candidate → that
image is used; zero → `NO_DOMINANT_PAGE_IMAGE`; two or more equally
plausible → `MULTIPLE_PAGE_IMAGES` (this package refuses to guess).

### OCR modes (`ingestion/pdf`'s `ParseWithOCR`)

```go
func ParseWithOCR(ctx context.Context, r io.ReadSeeker, opts Options, engine ocr.Engine) (Results, *ingestion.Error)
```

`Options.OCR` selects one of three modes:

- **`OCRDisabled`** (the zero value/default) — identical to `Parse`: an
  image-only page/document returns `OCR_REQUIRED`. `ParseWithOCR` called
  with this mode simply calls `Parse` internally; no behavior differs at
  all from before this feature existed.
- **`OCRAuto`** — tries embedded-text extraction first, for the *whole
  document* (the same `hasUsableTextLayer` check `Parse` already uses).
  Only pages that genuinely lack usable embedded text are OCR'd — a text
  PDF **never** invokes the OCR engine at all under this mode (verified by
  a dedicated test asserting zero engine calls). This is the mode that
  supports **mixed PDFs**: some pages read via embedded text, some via
  OCR, combined into one deterministic result with page provenance
  preserved throughout, and `MIXED_TEXT_AND_OCR_PAGES` emitted when both
  kinds of pages contributed to one document.
- **`OCRForce`** — ignores any embedded text layer entirely and OCRs every
  page's dominant image, regardless of whether usable embedded text
  exists. A page with no extractable dominant image under this mode is a
  fatal error for the whole parse (`OCRAuto`, by contrast, treats an
  unreadable individual page as non-fatal — see below).

**Per-page failure handling differs by mode.** Under `OCRAuto`, one
page that cannot be OCR'd (ambiguous/missing dominant image) contributes
no rows and is recorded via `NO_DOMINANT_PAGE_IMAGE`/
`MULTIPLE_PAGE_IMAGES`, without failing the rest of the document. Under
`OCRForce`, the same condition fails the entire parse — the caller
explicitly asked for every page to be OCR'd.

**Deterministic image preprocessing (`preprocess.go`).** `Options.Preprocess`
(`PreprocessOptions`) controls three conservative, fully deterministic,
individually testable steps, in a fixed order — grayscale conversion,
linear min/max contrast stretch, and fixed-threshold binarization — every
one **off by default** ("do not over-process by default"). No ML-based
image enhancement anywhere. Deskew/rotation-correction was deliberately
**not** implemented: a robust, safe deskew algorithm is neither trivial
nor conservative to hand-roll, and this repository's row-grouping Y
-tolerance already absorbs the mild skew a real scan typically exhibits
(see `scanned_skewed.pdf`'s fixture test) without needing an explicit
correction step — see Known OCR limitations below.

**Resolution/DPI.** Once a dominant image is selected, its effective
resolution is computed directly and exactly (`estimateDPI` — the page's
known point dimensions against the image's actual pixel dimensions,
distinct from `pdfimage`'s resolution-*independent* aspect-ratio
selection heuristic, which deliberately avoids assuming any DPI at
selection time). A page estimated below 150 DPI (Tesseract's own
documented reliability threshold) still gets OCR'd, but is flagged
`LOW_OCR_RESOLUTION` — resizing/upscaling is not performed by this
package, since it would not create missing source detail and could
misleadingly suggest higher confidence than the source data supports.

### OCR numeric safety

Financial-number OCR errors are treated as high-risk by design (per the
task's explicit warning about `0`↔`O`, `1`↔`I`↔`l`, `5`↔`S`, `8`↔`B`,
`,`↔`.`, and missed parentheses/minus signs). `ocr_numeric.go` implements
**exactly one** narrow, deterministic, context-sensitive correction rule,
never aggressive global character substitution:

1. `tabular.ParseNumeric` (the *same, unmodified* numeric parser CSV/
   XLSX/text-PDF already use) is tried on the raw OCR text first. A cell
   that already parses cleanly is **never** touched, even if it contains
   a letter elsewhere in the row.
2. Only on failure, and only for text that is whole-cell numeric-*shaped*
   with confusable letters (`reNumericShapeWithLetters` — optional
   `$`/`(` prefix, a run of digits/confusable-letters/commas/periods,
   optional trailing `)`/`%`/minus, nothing else — a label like "Cost of
   Goods Sold" never matches this and is never touched), a substitution
   table (`O`/`o`→`0`, `I`/`l`/`i`→`1`, `S`/`s`→`5`, `B`→`8`) is applied
   and the **same unmodified** `tabular.ParseNumeric` is tried again on
   the corrected text.
3. If the correction succeeds, the corrected value is used and
   `OCR_NUMERIC_CORRECTED` is emitted. If it still fails, the cell is
   left completely unparsed (`Cell.Parsed == false`, `Cell.Numeric ==
   nil`) and `OCR_NUMERIC_AMBIGUOUS` is emitted instead of the generic
   `UNPARSEABLE_NUMERIC_CELL` a plain parse failure would produce —
   **this package never invents a value it cannot deterministically
   justify.**

`Cell.Raw`/`OCRProvenance.OriginalText` always preserve the **uncorrected**
OCR text regardless of outcome, so a future review UI can show exactly
what the engine reported alongside whatever value (if any) was derived
from it.

### OCR provenance and review metadata

`ingestion.Cell` gained an optional `OCR *OCRProvenance` field (nil for
CSV/XLSX and for any PDF cell read via the embedded-text path), carrying
`OriginalText`, `Confidence` (the *minimum* across every OCR word
contributing to that cell — a cell is only as trustworthy as its
least-confident constituent word), `NumericCorrected`, `ReviewRecommended`
(a single boolean summary of confidence/correction/ambiguity signals, so a
consuming review UI doesn't need to reimplement this package's own
threshold logic), and `PixelBounds` (the cell's bounding box on the
*source scanned image*, in image pixel coordinates — distinct from the
existing `CellBounds`, which is still populated alongside it in PDF
points). `ingestion.Row.PageIndex`/`Cell.Bounds` (already existing PDF
provenance) are populated identically for OCR-derived rows.

`ingestion.Metadata` gained an optional `OCR *OCRMetadata` (nil unless OCR
was used), summarizing: whether OCR was used, which pages were OCR'd vs.
read via embedded text, engine name/version, average confidence
(**informational only — never a guarantee of accounting correctness**; a
high average can coexist with one badly misread critical number, which is
exactly why per-cell provenance and the `LowConfidence*`/
`OCR_NUMERIC_AMBIGUOUS` warnings exist rather than relying on this single
aggregate), low-confidence-numeric-cell count, and unsupported-scan-page
count.

### OCR-specific limits/security

`ingestion.Limits` gained seven additive OCR-only fields (zero value =
`DefaultLimits()`, identical pattern to every existing limit):
`MaxOCRPages` (default 50), `MaxImagePixels` (default 50,000,000 — a
defensive bound against a decompression-bomb-style oversized raster
embedded in an untrusted PDF), `MaxImageDimension` (default 10,000px, an
independent per-axis bound), `MaxOCRWords` (default 200,000),
`MaxOCRTextBytes` (default 10 MiB), `OCRPageTimeoutSeconds` (default 60),
and `OCRTotalTimeoutSeconds` (default 600). Exceeding a page/image bound
is a fatal `OCR_PAGE_LIMIT_EXCEEDED`/`OCR_IMAGE_LIMIT_EXCEEDED`; exceeding
a timeout is `OCR_TIMEOUT`. `ParseWithOCR`'s `ctx context.Context`
parameter bounds the whole call independently of these limits.

### OCR warning/error taxonomy — extended, not duplicated

New `WarningCode` values (same `ingestion.WarningCode` taxonomy every
other format already uses): `OCR_USED`, `LOW_OCR_RESOLUTION`,
`LOW_CONFIDENCE_LABEL`, `LOW_CONFIDENCE_NUMERIC_VALUE`,
`OCR_NUMERIC_CORRECTED`, `OCR_NUMERIC_AMBIGUOUS`, `MULTIPLE_PAGE_IMAGES`,
`NO_DOMINANT_PAGE_IMAGE`, `MIXED_TEXT_AND_OCR_PAGES`,
`UNSUPPORTED_EMBEDDED_IMAGE`. New `ErrorCode` values:
`OCR_ENGINE_UNAVAILABLE`, `OCR_ENGINE_FAILED`, `OCR_TIMEOUT`,
`OCR_PAGE_LIMIT_EXCEEDED`, `OCR_IMAGE_LIMIT_EXCEEDED`,
`SCANNED_PAGE_IMAGE_UNAVAILABLE`, `PDF_PAGE_RENDER_REQUIRED`. The
pre-existing `OCR_REQUIRED` error's meaning is unchanged for `Parse`/
`OCRDisabled`; it is simply now escapable via `OCRAuto`/`OCRForce`.

### OCR fixture corpus and testability

Every OCR code path is fully unit-testable without Tesseract installed,
via `ocr.FakeEngine` (see `ingestion/ocr` above). `ingestion/fixtures/gen`
gained `scan_image.go` (renders synthetic financial-statement-shaped text
onto a raster canvas using only the standard library plus
`golang.org/x/image/font`'s `basicfont`, then JPEG-encodes it — zero
external asset files) and `generate_ocr_pdf.go`, which embeds these
synthetic scans as JPEG XObjects into hand-rolled PDFs (extending
`pdf_writer.go` with image-XObject support), producing: `scanned_pl.pdf`,
`scanned_balance_sheet.pdf`, `scanned_low_resolution.pdf` (deliberately
half-resolution), `scanned_skewed.pdf` (each line shifted slightly right),
`scanned_negative_parentheses.pdf`, `scanned_multi_year.pdf`,
`scanned_multi_page.pdf`, `mixed_text_and_scanned.pdf` (real embedded
text on page 1, a genuine scan on page 2), `scanned_with_logo.pdf` (two
embedded images: a small logo plus the dominant page scan), and
`scanned_ambiguous_numeric.pdf` (a deliberate letter-for-digit OCR
ambiguity in the fixture's own rendered text, for real-Tesseract testing).
No proprietary/customer data, matching the existing CSV/XLSX/text-PDF
corpus's identical constraint.

Real-Tesseract integration tests (`ingestion/ocr/tesseract/
integration_test.go` and `ingestion/pdf/ocr_tesseract_integration_test.go`)
run conditionally: `Skip("SKIPPED — tesseract executable not installed")`
when no executable resolves, a real end-to-end OCR pass against the
synthetic fixtures when one does. Every other OCR test always runs.

### Known OCR limitations

- **No deskew/rotation correction** — only mild skew (absorbed by the
  existing row-grouping Y-tolerance) is tolerated; a significantly rotated
  scan is not corrected.
- **JBIG2-encoded scans are not supported** (`pdfimage`'s `extract.go` —
  `pdfcpu` returns a raw undecoded stream for this filter, and this
  package does not add a JBIG2 decoder). CCITT Group 4 fax and JPEG,
  the two most common real-world scan encodings, are both fully supported.
- **True vector-rendered scanned-looking pages require full PDF page
  rendering**, which this package does not implement — reported as
  `PDF_PAGE_RENDER_REQUIRED` rather than guessed at.
- **English only** for this MVP (`ingestion/ocr/tesseract`'s language-pack
  scope) — multi-language support would need the caller's own Tesseract
  installation to have the relevant language-data packages available,
  which this package does not manage.
- **OCR confidence is engine-specific and uncalibrated** — never usable as
  a statistical guarantee, only as a relative, per-engine signal (see
  `ingestion/ocr`'s package doc comment).
- **Letter-digit numeric correction (`ocr_numeric.go`) does not extend to
  period/header text** — a year header misread with a letter confusion
  (e.g. "2O25" for "2025") is not corrected, only numeric VALUE cells are
  (deliberately: correcting a period label risks silently reassigning a
  value to the wrong reporting period, a materially different and riskier
  kind of mistake than a numeric value simply failing to parse — see
  `WarnPeriodLabelAmbiguous`, which still fires for an uncorrected
  unparseable header exactly as it already does for CSV/XLSX/text-PDF).
- **`golang.org/x/image/tiff`** (BSD-3-Clause, already part of this
  repository's dependency tree via `golang.org/x/image`) is used to decode
  a `pdfcpu`-extracted TIFF-format image when that's the format `pdfcpu`
  chose to render a particular embedded filter/color-space combination to
  — see `pdfimage/extract.go`'s `decodeImage`.

### `financial`

Defines the presentation-agnostic financial model shared by every valuation
module:

- **`StatementType`** — `income_statement`, `balance_sheet`, `cash_flow`.
- **`Period`** — a reporting period, e.g. `"2025"`. Deliberately just a
  string so callers can choose their own granularity (`"2025"`, `"2025-Q1"`,
  `"2025-03"`) as long as it's used consistently and sorts lexically in
  chronological order.
- **`RawLineItem`** — one row of financial data as it appeared in a source
  document, before any classification.
- **`Code`** — a stable canonical taxonomy identifier (e.g.
  `OPEX_MARKETING`). See [Taxonomy](#taxonomy) below.
- **`MappedLineItem`** — a raw row that has already been classified against
  the canonical taxonomy (`Code` + `Status`). This is the *input* to
  normalization. Classification itself — the logic that decides which code a
  given label maps to — is not implemented here; callers supply
  already-mapped items.
- **`RowStatus`** — `normal`, `subtotal`, `total`, or `ignored`. Subtotals
  and totals are excluded during normalization to prevent double counting;
  ignored rows are skipped entirely.
- **`NormalizedItem`** — one aggregated `(code, period) → amount` figure,
  with optional provenance back to its source rows.
- **`FinancialDataset`** — the *output* of normalization: a currency plus a
  flat, deterministically ordered list of `NormalizedItem`.
- **`SourceRef`** — a provenance pointer from a normalized amount back to
  one contributing source row.

None of these types know about PDF, XLSX, CSV, QuickBooks, a database, or a
UI. They exist purely to move financial data between processing stages as
plain, JSON-serializable Go values.

#### Taxonomy

`financial` defines a fixed set of canonical `Code` values across five
categories: revenue, COGS, operating expenses, other income-statement items,
and balance sheet accounts (see `taxonomy.go` for the full list — 43 codes in
total).

Codes are stable string identifiers meant to be persisted and relied upon
long-term — **once released, a canonical `Code` is a persistent API
identifier, exactly like a stored primary key: never renamed, never
repurposed.** Display labels and category groupings are kept as separate
metadata (`CodeMeta`, looked up via `LookupCode`) specifically so that
copy can be revised without ever changing a code's wire value.
`financial.TaxonomyVersion` (see [Versioning strategy](#versioning-strategy)
below) identifies the exact set of codes currently defined, bumped only
when a code is added.
[`financial/taxonomy_test.go`](financial/taxonomy_test.go) asserts the
registry has no duplicate codes, every declared `Code` constant is
registered, and every registered entry has valid metadata.

The taxonomy is currently a flat namespace, but it's structured (one
`CodeMeta` entry per code, keyed by a single stable string) so a hierarchical
version — e.g. adding a `Parent Code` to `CodeMeta` — can be introduced later
without changing any existing code's identifier.

**Known coarse-grained areas, evaluated and left as-is.** `financial/classification`'s
rule set previously flagged three areas where the taxonomy is coarser than a
real chart of accounts might distinguish: R&D/engineering-heavy payroll (has
no dedicated code — falls under `OPEX_PAYROLL` or `OPEX_SOFTWARE`), owner
draw vs. owner compensation (both collapse into the single
`OPEX_OWNER_COMP`), and freight-in vs. freight-out (both collapse into the
single `COGS_FREIGHT`, or, for freight-out specifically, may land in
`COGS_OTHER`/`OPEX_OTHER` depending on classification). None of these
currently block any metric required by `financial/metrics` (§8 of this
module's design brief): gross profit only needs a `TotalCOGS` sum, not a
freight direction breakdown; `SDE` only needs total owner compensation, not
a draw/comp split. Per this module's "don't expand the taxonomy casually"
rule, no new codes were added — a future module should only add one of
these distinctions if a specific downstream calculation genuinely needs the
finer granularity, and even then should preserve every existing stable code
rather than renaming or repurposing one.

#### Normalizer

`Normalize(items []MappedLineItem, opts NormalizeOptions) (FinancialDataset, error)`
is the one exported entry point for aggregation. It:

- sums rows mapped to the same canonical `Code`, independently per `Period`,
- skips rows with `Status` of `ignored`, `subtotal`, or `total` (subtotal and
  total rows are excluded specifically to avoid double-counting the rows
  they summarize),
- optionally attaches `SourceRef` provenance for every contributing row when
  `NormalizeOptions.IncludeProvenance` is `true`,
- requires `NormalizeOptions.Currency` to be non-empty, returning
  `ErrMissingCurrency` otherwise,
- validates every row before returning: a `normal` row without a `Code`, or
  a row with an unrecognized `Status`, produces a `ValidationErrors` value
  (inspectable via `errors.As`) listing *every* problem found, not just the
  first.

`Normalize` performs no classification — it never looks at a row's `Label`
to infer a code. It is intentionally "dumb" aggregation plus validation, so
that classification logic (rules-based, ML-based, or otherwise) can evolve
independently as a separate module that simply needs to produce
`[]MappedLineItem`.

#### Structural row contract (`financial.RowKind`)

`RowKind` (a new string type: `""`/`RowKindNormal` (zero value),
`"heading"`/`RowKindHeading`, `"subtotal"`/`RowKindSubtotal`,
`"total"`/`RowKindTotal` — no `BLANK` value; a genuinely blank row never
becomes a `RawLineItem` at all, see [`ingestion`](#ingestion)) is a field
on both `RawLineItem` and `MappedLineItem` (`Kind RowKind`, `json:"kind,
omitempty"`) carrying an **upstream structural read** from whichever
adapter produced the row — distinct from `RowStatus`, which remains
exactly what it always was: the *normalize-time* directive
(`Normalize` already knows nothing new was needed here; it still switches
on `Status`, never on `Kind`).

**Zero-value semantics.** `RowKindNormal` is the empty string, so it is
both the Go zero value and omitted from JSON entirely
(`omitempty`) — a `RawLineItem` built by an existing caller that never
sets `Kind` (including a bare `RawLineItem{}`, as most of this
repository's own tests construct) behaves in classification exactly as it
did before this field existed: `financial/classification.Classify` falls
back to its own pre-existing label-based heuristic. This is the field's
entire backward-compatibility guarantee, and it is what closes the
"Gross Profit" gap without changing behavior for anyone who never
populates the field.

**Precedence.** `Classify`'s structural-detection stage (still stage 1 of
its pipeline — unchanged in position, only smarter) now checks
`RawLineItem.Kind` **first**. A non-zero `Kind` — supplied by an adapter
that has already done a better structural read than a bare label-token
check can (e.g. `ingestion`'s `ClassifyRowKind`, which recognizes "Gross
Profit" as a subtotal from label shape alone) — wins outright and skips
the label heuristic entirely. Only when `Kind` is the zero value does
`Classify` fall back to `structuralTotalTokens`
(`"total"`/`"subtotal"`/`"net"`), exactly as before. A `RowKindHeading`
row resolves to `Status = RowStatusIgnored` (a heading is not a subtotal
or total — it simply carries no financial amount — and `Normalize`
already excludes `ignored` rows from aggregation, so no code change was
needed there); `RowKindSubtotal`/`RowKindTotal` resolve to the
corresponding `RowStatus` directly.

**Heading rows now survive ingestion.** `ingestion.Result.ToRawLineItems()`
used to drop `StructuralHeading` rows entirely, since `RawLineItem` had no
field to represent "this is a section heading." It no longer does: a
heading becomes a `RawLineItem` with `Kind = RowKindHeading` and empty
`Values`, flows through classification (`SourceStructural`,
`RowStatusIgnored`), and lands on the resulting `MappedLineItem` — so a
caller (e.g. a future review UI) can now display section headings
straight from the same `[]RawLineItem`/`[]MappedLineItem` slices it
already has, instead of separately keeping `ingestion.Result.Rows` around
and re-correlating by `RowID`. Only genuinely blank rows (no text
anywhere) are still excluded from `RawLineItem` — `RowKind` has no
`BLANK` value, per the original task brief's explicit guidance that blank
rows do not need to become `RawLineItem`s.

**Regression coverage.** JSON round-trip tests exist for `Kind` on both
`RawLineItem` and `MappedLineItem` (including the zero-value-omitted
case), in [`financial/types_test.go`](financial/types_test.go). A
dedicated end-to-end regression,
`TestStructuralContract_LabelsSurviveAsStructuralNotOrdinaryAccounts` in
[`ingestion/integration_test.go`](ingestion/integration_test.go), proves
"Gross Profit", "Total Operating Expenses", "Net Income", and "Total
Assets" all resolve to `SourceStructural` (never an ordinary account
code) and never appear in a normalized `FinancialDataset`, across CSV,
XLSX, and (see [`ingestion/pdf`](#ingestionpdf)) PDF. A separate
backward-compatibility regression,
`TestRegression_OnlyKindChangedForPreExistingRows`, re-runs an existing
CSV fixture and asserts every row that was already present before this
field existed is unchanged in every OTHER field, with the only new rows
being the previously-dropped headings.

### `financial/classification`

The missing piece between raw source data and `financial.Normalize`:
`Classify`/`ClassifyBatch` turn `financial.RawLineItem` values into
classification `Result`s that carry a proposed `financial.Code`, a
`Result.ToMappedLineItem` conversion feeds directly into `Normalize`.

Like every other package here, `classification` has no idea where its
configuration comes from — no database, no accounts, no clients, no HTTP, no
AI/LLM calls. Callers assemble a `Config` (explicit overrides, alias layers,
rules) however they like and pass it in alongside the raw rows.

**Deterministic-only design.** This package is 100% rule-based: same
`(row, config)` in, same `Result` out, every time. It is explicitly built so
a future statistical or AI/LLM-based classifier can sit behind the exact
same boundary — `[]financial.RawLineItem` + config in, `[]Result` (and from
there `[]financial.MappedLineItem`) out — without `financial.Normalize` or
any downstream valuation code ever needing to change. Confidence scores
(see below) are a heuristic authored by this package's rule set, not a
statistical estimate; nothing here is trained on data.

**Classification precedence.** `Classify` runs a fixed pipeline and takes
the first stage that produces a match:

1. **structural detection** — is this row a heading/subtotal/total rather
   than an ordinary account? This checks `RawLineItem.Kind` first — a
   non-zero `Kind` supplied by the row's originating adapter (e.g.
   `ingestion`'s label-shape-based structural detection) wins outright;
   only when `Kind` is the zero value does this stage fall back to its own
   label heuristic (e.g. "Total Operating Expenses", "Net Income"). If the
   row is structural, no code is proposed, `Status` is set to
   `subtotal`/`total` (or `ignored` for a heading — see below), and every
   other stage is skipped entirely — even if an alias exists for that exact
   label — since aggregation correctness for totals matters more than
   classifying them.
2. **explicit mapping** — an exact `RawLineItem.ID` override in
   `Config.Explicits`. This is the human-review escape hatch: once someone
   has decided a specific row's code, nothing overrides it.
3. **alias** — an exact match of the row's normalized label against
   `Config.AliasLayers` (see alias precedence below).
4. **rules** — `Config.Rules`, evaluated in order; `DefaultRules()` supplies
   a built-in set covering common context-aware cases (e.g. labor under
   Cost of Sales vs. Operating Expenses) ahead of weaker phrase/token rules.
5. **UNKNOWN** — nothing above matched. `Classify` never falls back to a
   generic "other" code just because it couldn't find a real match (see
   UNKNOWN behavior below).

**Alias precedence.** Aliases are supplied as an ordered list of
`AliasLayer` values — this package has no built-in concept of "account" or
"client," each layer is just a name plus a list of `Alias{Label, Code}`
pairs. Precedence is purely positional, later layers win, mirroring
`settings.Resolve`'s "later argument wins" convention:

```
Config.AliasLayers = []AliasLayer{global, account, client, valuation}
// precedence: valuation > client > account > global
```

A higher-precedence layer's alias for a given normalized label completely
overrides a lower-precedence layer's alias for that same label; labels the
higher layer doesn't mention still fall through to lower layers untouched.

**Label normalization.** `NormalizeLabel` produces a `Comparable` form used
for alias/rule matching while preserving `Original` untouched: trims
whitespace, strips a leading account-number prefix (`"6100 Advertising"` /
`"6100 - Advertising"` -> `"advertising"`), lowercases, expands a
standalone `&` to `and` (never touching a `&` glued to letters, e.g.
`"AT&T"`), and turns punctuation into either nothing (apostrophes/quotes,
so `"Owner's"` -> `"owners"`) or a space (commas, dashes, parentheses,
etc.), then collapses whitespace. Every transformation is boundary-aware,
never a raw substring replacement — this is deliberate, since naive
substring handling of short tokens (e.g. an "ad" abbreviation) can silently
corrupt unrelated words like "advertising" or "adjustment". See
`normalize_label_test.go` for the regression tests protecting this.

**Confidence semantics.** `Confidence` is a `float64` in `[0, 1]`,
heuristic and deterministic, **not a statistical probability**:

| Stage                        | Constant                 | Value |
|-------------------------------|-------------------------|-------|
| explicit mapping               | `ConfidenceExplicit`    | 1.00  |
| structural (total/subtotal)    | `ConfidenceStructural`  | 0.97  |
| exact alias                    | `ConfidenceAlias`       | 0.98  |
| strong context-aware rule      | `ConfidenceStrongRule`  | 0.92  |
| weaker phrase/token rule       | `ConfidenceWeakRule`    | 0.75  |
| UNKNOWN                        | `ConfidenceUnknown`     | 0     |

`Config.ReviewThreshold` (default `DefaultReviewThreshold` = 0.90) is the
cutoff `Result.ReviewRequired` is computed against: at or above it, a
result is considered trustworthy enough to skip human review; below it,
review is recommended. `Classify` only *proposes* — it never auto-confirms
a mapping on the caller's behalf.

**UNKNOWN behavior.** `Result.IsUnknown()` (equivalently
`Source == SourceUnknown`) is a valid, important, and expected outcome:
`Code` is empty, `Confidence` is 0, and `ReviewRequired` is always true.
Ambiguous labels like "Misc", "General", "Adjustment", or "Other" are
expected to land here rather than being guessed at — `Classify` never
falls back to `CodeOpexOther` just because nothing else matched; `OTHER`
codes are only ever proposed when a rule genuinely identifies a line as
belonging to an "other" category.

**Alternatives.** `Result.Alternatives` carries up to
`Config.MaxAlternatives` (default 3) runner-up `Candidate` values — other
codes considered but not chosen — sorted by descending confidence, for a
future review UI to offer as quick alternatives to the primary proposal.

**Example usage:**

```go
cfg := classification.Config{
    AliasLayers: []classification.AliasLayer{
        {Name: "global", Aliases: []classification.Alias{
            {Label: "Advertising & Promotion", Code: financial.CodeOpexMarketing},
        }},
    },
    Rules: classification.DefaultRules(),
}

results := classification.ClassifyBatch(rawRows, cfg)

mapped := make([]financial.MappedLineItem, 0, len(results))
for i, result := range results {
    if result.IsUnknown() {
        continue // or route to a human-review queue
    }
    mapped = append(mapped, result.ToMappedLineItem(rawRows[i]))
}

dataset, err := financial.Normalize(mapped, financial.NormalizeOptions{Currency: "USD"})
```

### `financial/metrics`

A centralized derived-metrics engine. Every metric a valuation method needs
(EBITDA, SDE, working capital, growth rates, ...) is computed here, once,
from a `financial.FinancialDataset` — later valuation methods are expected
to consume `metrics.Calculate`'s output rather than each recomputing
EBITDA/SDE independently.

```go
result := metrics.Calculate(dataset, metrics.Options{PeriodMeta: periodMeta})
snapshot, _ := result.SnapshotFor("2025")
if snapshot.EBITDA.Available {
    fmt.Println(snapshot.EBITDA.Value)
}
```

`Calculate` never mutates `dataset` and performs no I/O.

#### Missing vs. zero

The central design rule of this package: **missing data is not zero**.
Every computed figure is a `MetricValue{Available bool, Value float64}`:

```go
type MetricValue struct {
    Available bool
    Value     float64
}
```

`Available == false` means one or more required inputs were absent from the
dataset — `Value` is always `0` in that case and must not be read as a real
calculated result. `Available == true, Value == 0` means the metric was
genuinely computed to be zero (e.g. a business with exactly break-even
EBIT). Callers must check `Available` before trusting `Value`; this is why
`Snapshot`'s fields are all `MetricValue`, never a bare `float64`.

Per-code absence inside a sum is tolerant (a business with no inventory
line simply contributes `0` to current assets without invalidating the
subtotal), but a subtotal itself is `Unavailable` if **none** of its
component codes are present at all (e.g. `TotalCOGS` is unavailable for a
dataset with zero COGS codes, but available-and-`0` for a dataset that
explicitly reports `COGS_MATERIAL: 0`).

#### Sign convention

Every amount in a `financial.FinancialDataset` is a **positive magnitude**
in its natural "as reported" sense — an expense code (COGS, OPEX,
depreciation, interest expense, taxes) holds a positive number representing
how much was spent, never a pre-negated contribution to income.
`financial.Normalize` performs no sign manipulation, so this flows through
unchanged from however the caller's source data represented it. Every
formula below explicitly subtracts expense-side codes; none of them assume
a pre-negated sign.

#### Exact formulas

| Metric | Formula |
|---|---|
| `TotalRevenue` | `REV_PRODUCT + REV_SERVICE + REV_RECURRING + REV_OTHER` |
| `TotalCOGS` | `COGS_MATERIAL + COGS_DIRECT_LABOR + COGS_FREIGHT + COGS_OTHER` |
| `GrossProfit` | `TotalRevenue - TotalCOGS` |
| `GrossMargin` | `GrossProfit / TotalRevenue` (unavailable if `TotalRevenue == 0`) |
| `TotalOpex` | sum of every `OPEX_*` code, including owner compensation |
| `EBIT` | `GrossProfit - TotalOpex` |
| **`EBITDA`** | **`EBIT + Depreciation + Amortization`** — missing `DEPRECIATION`/`AMORTIZATION` codes contribute `0` (many statements bury D&A inside COGS/OPEX without a separate line), so `EBITDA` stays available whenever `EBIT` is, even with no D&A broken out. |
| `EBITDAMargin` | `EBITDA / TotalRevenue` (unavailable if `TotalRevenue == 0`) |
| **`SDE`** | **`EBITDA + OPEX_OWNER_COMP`** — Seller's Discretionary Earnings, the total financial benefit available to a single working owner-operator. Owner compensation is added back because `EBITDA` already deducted it as an operating expense, but SDE by definition includes it. **This module implements no further discretionary add-backs** (personal vehicle expenses run through the business, one-time legal settlements, above-market related-party rent, etc.) beyond `OPEX_OWNER_COMP` — `financial/adjustments` owns that layer, applied on top of this baseline `SDE`/`EBITDA`. Treat this `SDE` as a baseline. |
| `NetIncome` | `EBIT + OTHER_INCOME + INTEREST_INCOME - INTEREST_EXPENSE - OTHER_EXPENSE - INCOME_TAX` — every below-the-line code absent from the dataset contributes `0` rather than making the whole figure unavailable, since most small-business statements below the operating-income line are sparse. |
| `CurrentAssets` | `BS_CASH + BS_ACCOUNTS_RECEIVABLE + BS_INVENTORY + BS_PREPAID + BS_CURRENT_ASSET_OTHER` |
| `CurrentLiabilities` | `BS_ACCOUNTS_PAYABLE + BS_CURRENT_LIABILITY_OTHER + BS_SHORT_TERM_DEBT` |
| `WorkingCapital` | `CurrentAssets - CurrentLiabilities` |
| `TotalDebt` | `BS_SHORT_TERM_DEBT + BS_LONG_TERM_DEBT` |
| `NetDebt` | `TotalDebt - BS_CASH` (unavailable unless cash is present in the dataset — netting against unknown cash would overstate precision) |
| `TangibleAssetValue` | `BS_FIXED_ASSETS - BS_ACCUM_DEPRECIATION` (excludes `BS_INTANGIBLE_ASSETS`/`BS_GOODWILL` by design) |

See each formula's doc comment in `financial/metrics/income_statement.go` and
`balance_sheet.go` for the exact availability rules.

#### Historical/trend metrics

`Calculate` also computes a `Trend` (growth, CAGR, volatility, margin
trends) whenever the dataset has 2+ periods, but **only across
`PeriodTypeFiscalYear` periods** — quarters, months, and YTD periods are
excluded from these calculations even when present, since naively mixing
granularities (e.g. treating consecutive quarters as consecutive years)
would produce misleading numbers. A future version may add dedicated
quarter-over-quarter or month-over-month trend calculations.

**Period ordering is never guessed.** `financial.Period` is just a string
with no guaranteed sort order (`"2025-Q1"` does not reliably precede
`"2025-Q2"` for every caller's convention). Trend calculations require
`Options.PeriodMeta map[financial.Period]PeriodInfo`, and return an explicit
`Trend.Error` (a `*PeriodOrderError`) — never a silently-wrong guess — if
metadata is missing for any period in the dataset, or if fewer than two
comparable fiscal-year periods remain after filtering:

```go
type PeriodInfo struct {
    Type           PeriodType // fiscal_year, ytd, quarter, month
    FiscalYear     int
    SequenceInYear int        // 1-4 for quarters, 1-12 for months
}
```

**Year-over-year growth**: `(to - from) / |from|`. If `from == 0`,
`Growth` is `Unavailable` and `GrowthFromZeroBase` is set to `true` — a
growth percentage from a zero base is undefined, not "infinite" or "0%".

**CAGR**: `(End / Start)^(1 / Years) - 1`, where `Years` comes from
`PeriodInfo.FiscalYear`, never re-derived from a period's string label.
Requires `Start > 0`: a zero or negative starting value makes compound
growth mathematically undefined or misleading, so `CAGRResult.Invalid`
explains why rather than returning a fabricated number (e.g. from a
negative-base fractional power, which `math.Pow` would otherwise silently
return as `NaN`).

**Volatility**: the **sample standard deviation of the year-over-year
percentage-change series** (not of the raw dollar levels), expressed as a
decimal. Growth rates are used instead of raw levels because raw-level
standard deviation scales with the size of the business rather than
measuring how stable its trajectory is. Sample (not population) standard
deviation is used (divide by `n-1`), treating the observed years as a
sample of the business's underlying behavior. Requires at least 2 valid
growth-rate observations (3 fiscal years); fewer than that leaves
`VolatilityResult.Value` unavailable with `SampleSize` reported for
context.

#### Explainability

Every computed figure is available both as a typed `Snapshot` field and as
a `MetricResult` (via `Snapshot.Results[metrics.MetricEBITDA]`, etc.) that
carries a fixed `Formula` string and a `Components` breakdown:

```json
{
  "metric": "EBITDA",
  "period": "2025",
  "value": { "available": true, "value": 420000 },
  "formula": "EBIT + Depreciation + Amortization",
  "components": [
    { "code": "EBIT", "label": "EBIT", "amount": 370000 },
    { "code": "DEPRECIATION", "label": "Depreciation", "amount": 35000 },
    { "code": "AMORTIZATION", "label": "Amortization", "amount": 15000 }
  ]
}
```

This is deliberately *not* a generic expression engine — each metric's
formula is fixed Go code — but every result still carries enough structure
for a future report to explain exactly how a number was produced.

### `financial/reconciliation`

Evaluates the internal consistency of a normalized `financial.FinancialDataset`:
whether reported subtotals match their reconstructed components, whether the
balance sheet balances, and whether the dataset is structurally well-formed.
`Run` never mutates the dataset and performs no I/O.

```go
result := reconciliation.Run(dataset, reconciliation.Options{
    Tolerance: reconciliation.Tolerance{Absolute: 1.0},
    Reported: reconciliation.ReportedTotals{
        "2025": {GrossProfit: ptr(523421.0)},
    },
})
for _, check := range result.Checks {
    fmt.Println(check.Code, check.Status, check.Explanation)
}
```

#### Why reconciliation needs a separate `Reported` argument

`financial.FinancialDataset` has **no representation for a reported
subtotal** (gross profit, operating income, EBITDA, net income):
`financial.Normalize` deliberately excludes subtotal/total rows to avoid
double counting (`financial.RowStatusSubtotal`/`RowStatusTotal`), and the
canonical taxonomy has no codes for them either, since they are computed
aggregates rather than classifiable accounts. A caller that has access to
the original statement's reported subtotals (e.g. from the
`financial.RawLineItem` rows tagged as subtotal/total before
`classification`/`Normalize` dropped them) supplies them via
`Options.Reported`, keyed by period:

```go
type ReportedPeriodTotals struct {
    GrossProfit     *float64
    OperatingIncome *float64
    EBITDA          *float64
    NetIncome       *float64
}
```

Every field is a pointer, mirroring `settings.Settings`' nil-means-unset
convention: `nil` means "not supplied," never "reported as zero." A caller
with no access to reported subtotals simply omits `Options.Reported`
entirely — every reported-vs-reconstructed check then returns
`StatusNotApplicable`, not a fabricated comparison or a `FAIL`.

#### Check statuses

Never a bare boolean — four explicit statuses:

| Status | Meaning |
|---|---|
| `PASS` | The check ran and its values matched within tolerance (or the integrity condition held). |
| `WARNING` | Something worth a human's attention, not necessarily wrong — e.g. a figure that couldn't be calculated from partial data, or an unusual-but-valid structural shape (e.g. an income-statement-only dataset with no balance sheet at all is fine; a balance sheet with assets but no liabilities/equity codes is a warning). |
| `FAIL` | A real problem: a difference outside tolerance, a balance sheet that doesn't balance, or malformed data (non-finite values, duplicate entries, unrecognized codes). |
| `NOT_APPLICABLE` | The check couldn't run because a required input was absent — e.g. no reported gross profit was supplied, or a period has no balance sheet data at all. Distinct from `PASS`: the check makes no correctness claim when it has nothing to check. |

#### Tolerance behavior

```go
type Tolerance struct {
    Absolute        float64
    RelativePercent float64 // e.g. 0.01 = 1%
}
```

A difference passes if it is within `Absolute` **OR** (when
`RelativePercent` is set and the expected value is nonzero) within
`RelativePercent` of `|expected|` — whichever is more permissive. This `OR`
combination lets a tiny absolute tolerance still absorb rounding noise on a
very large statement, and lets a relative tolerance still mean something
against a near-zero expected value. `RelativePercent` never applies against
a zero expected value (a relative tolerance against zero is undefined).

A zero-value `Tolerance` passed via `Options` is replaced with
`DefaultTolerance` (`{Absolute: 1.0}`) rather than silently becoming an
unreachable exact-match requirement — this package never hard-codes a
single *global* tolerance deep inside a check; every numeric comparison
reads the tolerance from `Options`.

```
reported gross profit:      523,421
reconstructed gross profit: 523,420
difference: 1
tolerance: {Absolute: 1.0}
status: PASS
```

#### Checks implemented

Income statement (all require `Options.Reported` for that period, else
`NOT_APPLICABLE`):

- `GROSS_PROFIT_RECONCILES` — `Revenue - COGS` vs. reported gross profit.
- `OPERATING_INCOME_RECONCILES` — `Gross Profit - Opex` (EBIT) vs. reported operating income.
- `EBITDA_BRIDGE_RECONCILES` — `EBIT + D&A` vs. reported EBITDA (expected to be `NOT_APPLICABLE` far more often than not, since most statements never report EBITDA directly — this is not itself a problem).
- `NET_INCOME_RECONCILES` — the full net income bridge vs. reported net income.

Balance sheet:

- `BALANCE_SHEET_BALANCES` — `Assets ≈ Liabilities + Equity`. `BS_ACCUM_DEPRECIATION` is treated as a contra-asset (subtracted, not added) since it's stored as a positive magnitude like every other code. `NOT_APPLICABLE` (not `FAIL`) when a period has no balance sheet codes at all.
- `CURRENT_ASSETS_SUBTOTAL`, `CURRENT_LIABILITIES_SUBTOTAL`, `WORKING_CAPITAL_CALCULATED`, `DEBT_TOTALS_CALCULATED` — informational availability checks (via `financial/metrics`) with no reported figure to compare against: `PASS` if calculable (of any value, including a real `0`), `WARNING` if the relevant codes are entirely absent.

Dataset-level integrity (structural only — no business judgement):

- `DATASET_NOT_EMPTY`, `VALID_CURRENCY`, `FINITE_NUMERIC_VALUES` (rejects `NaN`/`±Inf`), `NO_DUPLICATE_ENTRIES` (duplicate `(code, period)` pairs that should have been aggregated away by `Normalize`), `NO_UNKNOWN_PERIOD_REFERENCES` (empty period or unrecognized `financial.Code`), `NO_SUSPICIOUS_DUPLICATE_SOURCES` (the same source row referenced twice within one aggregate's provenance), `INCOME_STATEMENT_HAS_REVENUE` and `BALANCE_SHEET_HAS_LIABILITIES_OR_EQUITY` (warn on lopsided/incomplete statements without treating an income-statement-only or balance-sheet-only dataset as inherently invalid).

### `financial/adjustments`

Models explicit, human-supplied normalization adjustments (owner
compensation normalization, personal expenses run through the business,
one-time items, related-party rent, non-operating income/gains/losses) and
applies them to a `metrics.Snapshot` to produce transparent, auditable
**normalized EBITDA** and **normalized SDE** bridges.

This package does not decide which adjustments exist for a given business,
does not store them, and does not know who created them or where they came
from — `Adjustment.SourceRef` is an opaque caller-supplied string, exactly
like `reconciliation`'s treatment of reported totals. `Apply` never
mutates its inputs and performs no I/O.

```go
result := adjustments.Apply(snapshot, []adjustments.Adjustment{
    {
        ID: "adj-1", Period: "2025", Type: adjustments.TypePersonalVehicle,
        Amount: 7200, Reason: "owner's personal truck lease run through the business",
        Included: true,
    },
    {
        ID: "adj-2", Period: "2025", Type: adjustments.TypeOwnerCompensationNormalization,
        Amount: 32000, Effect: adjustments.EffectDecrease, Targets: []adjustments.Target{adjustments.TargetSDE},
        Reason: "normalize $92k actual draw to a $60k market-rate replacement salary",
        Included: true,
    },
})
fmt.Println(result.EBITDABridge.NormalizedValue)
fmt.Println(result.SDEBridge.NormalizedValue)
```

#### Sign convention: magnitude + explicit direction, never a signed delta

Every `Adjustment` carries a **non-negative `Amount`** (a magnitude) plus an
explicit **`Effect`** (`EffectIncrease` or `EffectDecrease`) stating whether
applying it makes the target metric larger or smaller. There is no signed
"delta" anywhere in this package's public API — a bare `-50000` forces every
caller and reviewer to separately remember whether negative means "this
expense is being added back" or "this reduces earnings," and that
convention is exactly the kind of thing that gets flipped by accident.
"Remove non-operating income" is `Amount: 75000, Effect: EffectDecrease`,
never `Amount: -75000`.

`Validate` deliberately does **not** reject a negative `Amount` outright, or
reject an adjustment because its net effect reduces earnings — only
non-finite amounts are rejected on the amount itself. Legitimate
adjustments (removing non-operating income, normalizing above-market
related-party rent down to fair value) are expected to *decrease* normalized
earnings; treating "decreases the number" as inherently suspect would be
wrong.

#### Adjustment types and their defaults

Each `Type` carries default `Targets` (which bridge(s) it applies to) and a
default `Effect`, both overridable per-`Adjustment` (see `LookupType`):

| Type | Default Targets | Default Effect |
|---|---|---|
| `owner_compensation_normalization` | SDE only | *(none — see below)* |
| `owner_discretionary_expense` | EBITDA + SDE | increase |
| `personal_vehicle` | EBITDA + SDE | increase |
| `personal_travel` | EBITDA + SDE | increase |
| `one_time_expense` | EBITDA + SDE | increase |
| `non_recurring_professional_fees` | EBITDA + SDE | increase |
| `related_party_rent_adjustment` | *(none — caller must specify)* | *(none — caller must specify)* |
| `non_operating_income` | EBITDA + SDE | decrease |
| `unusual_gain` | EBITDA + SDE | decrease |
| `unusual_loss` | EBITDA + SDE | increase |
| `custom` | *(none — caller must specify)* | *(none — caller must specify)* |

`related_party_rent_adjustment` has no default effect because the direction
genuinely depends on the fact pattern: replacing *below*-market rent with
fair-market rent **decreases** earnings (the fair-market rent is higher),
while replacing *above*-market rent with fair-market rent **increases**
earnings (the fair-market rent is lower). `custom` likewise has no default —
both require the caller to set `Effect`/`Targets` explicitly, and `Validate`
reports `IssueAmbiguousEffect`/`IssueAmbiguousTargets` (errors) if they
don't. `Type` is a plain string, like `financial.Code`, so new types can be
added later without breaking existing callers.

#### Owner compensation: no double counting

`financial/metrics`' baseline `SDE` is `EBITDA + OwnerCompensation` — EBITDA
already deducted owner compensation as an operating expense, and SDE adds
it back in full because SDE is defined as the total benefit available to a
single working owner. This means owner compensation is **already fully
reflected** in the SDE baseline `Apply` starts from.

An `owner_compensation_normalization` adjustment therefore represents a
*replacement* of that baseline figure with a market-adjusted one, not a
second independent add-back of the full compensation figure. The caller
supplies `Amount` as the **difference** between actual and market-rate
compensation (e.g. "the owner drew $180k; a hired GM would cost $90k; the
adjustment amount is the $90k difference"), not the raw compensation
figure — this package has no visibility into what a market-rate replacement
would actually cost, so it cannot derive that difference itself. By
default, `owner_compensation_normalization` targets `TargetSDE` only:
EBITDA's baseline never added owner compensation back in the first place,
so there is nothing to normalize in an EBITDA bridge, and an adjustment of
this type is reported in `Result.Skipped` with `SkipNotTargeted` for the
EBITDA bridge rather than silently ignored.
`TestApply_DoubleCountPrevention_OwnerCompNotAddedTwice` in
`apply_test.go` asserts this directly: applying the adjustment changes SDE
by exactly its own `Amount`, never by the full `OwnerCompensation` value on
top of that.

#### The bridges

`Apply` returns both bridges unconditionally — a `Bridge` is a transparent
walk from a base metric to a normalized one:

```
Reported/Calculated EBITDA
+ confirmed positive adjustments targeting EBITDA
- confirmed negative adjustments targeting EBITDA
= Normalized EBITDA
```

```
Base SDE (EBITDA + Owner Compensation, per financial/metrics)
+/- confirmed adjustments targeting SDE
= Normalized SDE
```

Each `Bridge` carries `BaseAvailable`/`BaseValue` (from the underlying
`metrics.Snapshot`), every `AppliedLine` (with its resolved
`SignedAmount`), `TotalAdjustment`, and `NormalizedValue = BaseValue +
TotalAdjustment`. `Result` additionally carries every adjustment that
didn't contribute to a given bridge, in `Skipped`, with one of:

| `SkipReason` | Meaning |
|---|---|
| `not_included` | `Adjustment.Included` was `false`. |
| `wrong_period` | The adjustment's `Period` doesn't match the snapshot. |
| `not_targeted` | The adjustment's resolved `Targets` doesn't include this bridge. |
| `invalid` | The adjustment failed `Validate` (see `Result.Errors`). |
| `base_metric_unavailable` | The bridge's starting metric (EBITDA or SDE) is itself unavailable for this period. |

#### Validation

`Validate(adjs, snapshot)` checks a set of adjustments for internal
consistency, returning `Issue`s with `SeverityError` (block application —
`Apply` skips the adjustment with `SkipInvalid`) or `SeverityWarning`
(surfaced but non-blocking): missing ID, duplicate ID, missing/mismatched
period, missing type, non-finite amount, ambiguous effect/targets (a type
with no default that didn't set one explicitly), an unrecognized target,
and a suspicious duplicate (same period + type + amount + effect appearing
more than once — likely a double-entered adjustment, as opposed to two
distinct legitimate adjustments that happen to share a type). A missing
`Reason` is a warning, not an error — auditable but not blocking.
`HasErrors(issues)` is a convenience check for "is this set safe to apply."

See
[`fixtures/adjustments_by_business_type.json`](fixtures/adjustments_by_business_type.json)
for realistic confirmed adjustments across the four business archetypes
(owner compensation normalization, personal vehicle, and a one-time repair
for the HVAC business; owner salary normalization and a one-time
rebranding cost for the agency; an unusual repair and a related-party rent
adjustment for the manufacturer; non-recurring professional fees and
non-operating income removal for the SaaS company), exercised end-to-end
against the corresponding `normalized_*_multi_year.json` datasets in
[`financial/adjustments/fixtures_test.go`](financial/adjustments/fixtures_test.go).

### `financial/earnings`

Selects or derives a single **maintainable earnings** figure from a
caller-supplied, chronologically-ordered series of period/value
`Observation`s (e.g. normalized EBITDA or normalized SDE values produced by
`financial/adjustments` across several historical periods), for use as a
future valuation formula's input (an SDE/EBITDA multiple, a DCF terminal
value, etc.).

This package has no idea where its observations came from and does not
compute EBITDA/SDE itself — it only knows how to combine an ordered series
of `(period, value)` pairs into one defensible number, under a
caller-selected `Strategy`, with a fully explainable trail of what was
included, excluded, and why.

```go
result := earnings.Calculate(observations, earnings.Options{
    Strategy: earnings.StrategyWeightedAverage,
    Weights:  map[string]float64{"2023": 20, "2024": 30, "2025": 50}, // percentages
})
fmt.Println(result.Value, result.Available)
```

#### Strategies

| `Strategy` | Behavior |
|---|---|
| `latest_period` | Uses the single most recent comparable, available observation. Every other comparable observation is excluded with `ExclusionNotLatest`. |
| `simple_average` | Unweighted arithmetic mean across every comparable, available observation. |
| `weighted_average` | Caller-supplied explicit weights per period (`Options.Weights`, keyed by `Observation.Period`) — see below. |
| `trend_adjusted` | Deterministic ordinary-least-squares linear regression (value against period index) across comparable observations; maintainable earnings is the fitted line's value at the final period. Requires **at least 3** comparable available observations — 2 points make any "trend" identical to a straight line between them, no more informative than `latest_period` or a 2-point average. |

`trend_adjusted`'s documented limitation: it assumes the trend is
well-approximated by a straight line across the full comparable window. It
does not detect or special-case a level shift (e.g. a one-time
acquisition), a regime change, or seasonality — those require subjective
judgment this package deliberately leaves to the caller (e.g. by choosing
which observations to include, or not using this strategy at all when a
straight-line trend isn't a defensible model of the business). No more
sophisticated method (exponential smoothing, seasonal decomposition,
outlier-robust regression) is implemented, since those all require a
subjective parameter or domain assumption this package's design brief asks
to leave as documented future work rather than build prematurely.

#### Weighted average validation

`Options.Weights` must satisfy, in order — any failure makes the whole
result `Unavailable` rather than silently coercing invalid weights into
something that "works":

1. Every comparable, available observation's period must have a weight
   entry (`ExclusionNoWeight` otherwise — a forgotten weight is excluded,
   never silently treated as `0`).
2. Every supplied weight must be finite.
3. Every supplied weight must be non-negative (unlike `adjustments.Amount`,
   a negative weight is never mathematically usable for an average).
4. The weights actually used must sum to **approximately 1.0** (fractional
   form) **or approximately 100** (percentage form, e.g. the `20/30/50`
   example above) — within a small tolerance for float/rounding noise. Any
   other sum is rejected outright with an explanatory error, **not**
   silently rescaled to sum to 1: this package never guesses that a caller
   meant to normalize a wildly invalid set of weights (e.g. `{1, 2, 3}`)
   into valid ones.

#### Comparable periods

`Calculate` never blindly averages observations of different
`PeriodType` together (e.g. three full fiscal years and one trailing YTD
stub). It first narrows to a comparable subset: if
`Options.ComparablePeriodType` is set, only observations of that exact type
are eligible; otherwise the comparable type is **inferred as whichever
`PeriodType` the largest number of supplied observations share** — so the
common shape of three full fiscal years plus one trailing YTD period
correctly resolves to "fiscal year" as comparable, excluding the YTD stub,
rather than the reverse (which a naive "use the last observation's type"
rule would produce). Every excluded observation is reported in
`Result.ExcludedPeriods` with `ExclusionIncomparablePeriodType`. A caller
that genuinely wants to mix granularities (e.g. annualizing a YTD figure)
must do that conversion itself before building the `Observation` slice.

#### Explainability

`Result` is designed for direct display in a future valuation report:
`Strategy`, `IncludedPeriods`, `ExcludedPeriods` (each with a `Reason`),
`RawValues`, `Weights` (for `weighted_average`), `Available`/`Value`, and
`Warnings`/`Errors`.

See
[`financial/earnings/fixtures_test.go`](financial/earnings/fixtures_test.go)
for all four business archetypes' EBITDA series run through
`latest_period`, `simple_average`, `weighted_average`, and
`trend_adjusted`, including a case that appends a synthetic YTD period to
confirm it's excluded rather than blended into the fiscal-year average.

### `analytics/qoe`

Produces a deterministic **quality-of-earnings (QoE)** analysis from a
normalized `financial.FinancialDataset` plus a caller's confirmed
`financial/adjustments.Adjustment` set and chosen
`financial/earnings.Result` maintainable-earnings figures — the package a
caller reaches for once EBITDA/SDE have been normalized and a maintainable
figure selected, to answer "how good is this earnings number, and why,"
rather than just "what is it."

This package computes nothing upstream of that: it does not classify raw
rows, does not decide which adjustments are legitimate, and does not pick a
maintainable-earnings strategy on the caller's behalf. It recomputes
`financial/metrics.Snapshot` values across every period in the dataset
itself (rather than accepting a pre-built `metrics.Result`), walks each
period's normalized-EBITDA/SDE bridge via `financial/adjustments.Apply`,
and turns that per-period history into one explainable `Result`. Every
function is pure — no I/O, no mutation of caller-owned input — and
`Calculate` is proven to return byte-for-byte identical JSON across
repeated runs against identical input.

```go
res := qoe.Calculate(qoe.Input{
    Dataset:             dataset,             // financial.FinancialDataset
    PeriodMeta:          periodMeta,          // map[financial.Period]metrics.PeriodInfo
    Adjustments:         confirmedAdjustments, // []adjustments.Adjustment (Included == true ones apply)
    MaintainableEBITDA:  maintainableEBITDA,  // earnings.Result, basis = normalized EBITDA
    MaintainableSDE:     maintainableSDE,     // earnings.Result, basis = normalized SDE
}, qoe.Options{ComputeScore: true})

fmt.Println(res.Ratios.AdjustmentToEBITDA.Value, res.Flags, res.Score.Value)
```

A caller typically sequences this two-pass, since `financial/earnings.Calculate`
itself needs `qoe`'s normalized-EBITDA/SDE series as its `Observation`s: run
`Calculate` once with `MaintainableEBITDA`/`MaintainableSDE` left zero to get
`Result.History`, build `earnings.Observation`s from
`History[i].NormalizedEBITDA`/`NormalizedSDE`, run `earnings.Calculate`, then
run `qoe.Calculate` again with those results supplied — see
[`analytics/qoe/fixtures_test.go`](analytics/qoe/fixtures_test.go)'s
`runQoE` helper for the exact sequencing every test in the package uses.

#### What `Result` contains

| Field | What it is |
|---|---|
| `History` | One `PeriodFigures` per period (chronological when `PeriodMeta` is supplied and complete, else dataset order): reported vs. normalized EBITDA/SDE, the full `metrics.Snapshot`, and the full `adjustments.Result` bridge for that period. |
| `Adjustments` | `AdjustmentBreakdown` — total confirmed adjustment contribution across every period in `History`, **kept separate for EBITDA and SDE** (never summed together — a single `Adjustment` can target one bridge, the other, or both, and its EBITDA-bridge and SDE-bridge `SignedAmount` are not always equal, most visibly for `TypeOwnerCompensationNormalization`), plus a per-`adjustments.Type` breakdown sorted by `Type` string. |
| `Recurrence` | `[]RecurrencePattern` — every nominally non-recurring `adjustments.Type` (`one_time_expense`, `non_recurring_professional_fees`, `unusual_gain`, `unusual_loss`) that appeared in confirmed, applied lines, with the distinct periods it appeared in and whether that count meets `Thresholds.RepeatedOneTimeMinPeriods`. See [Repeated one-time detection](#repeated-one-time-detection) below. |
| `RecurringAdjustments` | `RecurringSummary` — every confirmed, applied adjustment line split into three buckets by its `Type`'s *inherent* nature (recurring: owner compensation/personal vehicle/personal travel/owner-discretionary/related-party rent; non-recurring: the same four types `Recurrence` tracks; unclassified: `non_operating_income` and `custom`, whose nature this package cannot infer — see below), each with an EBITDA-bridge total, an SDE-bridge total, and a count. Distinct from `Recurrence`: this asks "is this kind of item expected to recur at all," not "did this specific `Type` actually repeat in this dataset." |
| `MaintainableEBITDA` / `MaintainableSDE` | Echo `Input.MaintainableEBITDA`/`MaintainableSDE` when `Available`; otherwise the zero `earnings.Result`, with an advisory `Issue` explaining why every dependent ratio/flag was skipped. |
| `RevenueGrowth`, `RevenueVolatility`, `EBITDAMarginTrend`, `EBITDAVolatility` | Copied verbatim from `metrics.Trend` — never re-derived by hand, so they always match exactly what `financial/metrics` itself would report for this dataset. |
| `SDEVolatility` | The SDE-basis counterpart to `EBITDAVolatility`, computed by this package using the identical year-over-year-growth sample-standard-deviation method `financial/metrics` uses internally (SDE volatility is out of `metrics.Trend`'s own field set). |
| `Ratios` | `AdjustmentToEBITDA`/`AdjustmentToSDE` — the absolute adjustment total divided by the absolute reported base, for the most recent period, each a `metrics.MetricValue`. |
| `Flags` | `[]Flag` — every deterministic quality signal that triggered, in `FlagCode` declaration order. See [Deterministic quality flags](#deterministic-quality-flags) below. |
| `Score` | `*Score`, populated only when `Options.ComputeScore` is true. See [Earnings quality score](#earnings-quality-score) below. |
| `Thresholds` | The resolved `Thresholds` (after `DefaultThresholds` substitution) this `Result` was computed under, so a persisted `Result` remains self-describing. |
| `Warnings` / `Errors` | Structured `Issue`s (own `IssueCode` system — see [Error taxonomy](#error-taxonomy)) for input-level problems (`NO_PERIODS`, `NO_PERIOD_META`, `MAINTAINABLE_EBITDA_UNAVAILABLE`, `MAINTAINABLE_SDE_UNAVAILABLE`). |

#### Deterministic quality flags

Every flag is rule-based against caller-configurable `Thresholds`
(`DefaultThresholds()` for the conservative defaults; the zero `Thresholds`
passed to `Calculate` resolves to these, mirroring
`review.Policy`/`DefaultPolicy`) — **no AI, no opaque scoring**:

| `FlagCode` | Triggers when |
|---|---|
| `LARGE_NORMALIZATION_BURDEN` | `Ratios.AdjustmentToEBITDA` or `AdjustmentToSDE` ≥ `LargeNormalizationBurdenRatio` (default 30%). |
| `DECLINING_EBITDA_DESPITE_REVENUE_GROWTH` | A fiscal-year transition in `RevenueGrowth` shows revenue grew while reported EBITDA (from `History`) declined. |
| `VOLATILE_EARNINGS` | `EBITDAVolatility` or `SDEVolatility` ≥ `VolatileEarningsRatio` (default 35%). |
| `INCONSISTENT_MARGINS` | `EBITDAMarginTrend`'s margin values swing (max − min) ≥ `InconsistentMarginSwing` (default 15 points). |
| `LARGE_OWNER_DISCRETIONARY_COMPONENT` | The most recent period's owner-related SDE-bridge adjustments (owner compensation normalization, personal vehicle/travel, owner-discretionary expense) total ≥ `OwnerDiscretionaryShareOfSDE` (default 25%) of normalized SDE. |
| `REPEATED_ONE_TIME_ADJUSTMENTS` | At least one `RecurrencePattern.LikelyNotNonRecurring` is true. See below. |
| `NON_OPERATING_INCOME_SUPPORTING_EARNINGS` | The most recent period's non-operating-income-removal adjustments (`non_operating_income`, `unusual_gain`) total ≥ `NonOperatingIncomeShareOfEBITDA` (default 20%) of reported EBITDA. |
| `NEGATIVE_OR_NEAR_ZERO_MAINTAINABLE_EARNINGS` | `MaintainableEBITDA`/`MaintainableSDE` ≤ the absolute floor `NearZeroMaintainableEarnings` (default 0) **or** ≤ `NearZeroMaintainableEarningsPercentOfRevenue` (default 2%) of the most recent period's reported revenue — a two-leg OR test mirroring `review.IsMaterial`'s materiality logic, since "near zero" is inherently scale-dependent. |

Every `Flag` carries a stable `Code`, a `Severity` (`info`/`warning`/
`critical`), the `Period` it concerns (when applicable), a pre-filled
`Message`, and the exact `Value`/`Threshold` compared — `Message` is display
only and never parsed by this package's own logic; `Code` is the stable,
matchable signal.

#### Repeated one-time detection

If the same nominally non-recurring `adjustments.Type` (`one_time_expense`,
`non_recurring_professional_fees`, `unusual_gain`, `unusual_loss` — **not**
owner-related types like `personal_vehicle`, which legitimately recur every
year without being suspicious) appears in confirmed, applied adjustment
lines across `RepeatedOneTimeMinPeriods` or more distinct periods (default
**2**), `RecurrencePattern.LikelyNotNonRecurring` is set and
`REPEATED_ONE_TIME_ADJUSTMENTS` fires. **This package never automatically
removes or reclassifies the adjustment** — it only surfaces the flag; the
confirmed adjustment set a caller supplied is never second-guessed or
mutated.

Deduplication when summing `RecurrencePattern.TotalAmount` is keyed on each
`Adjustment`'s own `ID` (unique within a single `Apply` call), not on
`Type` alone — two *different* adjustments of the same `Type` in the same
period (e.g. one targeting the EBITDA bridge only, another targeting SDE
only) are two distinct real-world amounts and both must be counted; a naive
`(period, Type)` dedup would silently drop the second one. See
[`analytics/qoe/repeated.go`](analytics/qoe/repeated.go)'s
`buildRecurrence` and the regression test
`TestBuildRecurrence_DistinctSameTypeAdjustmentsInSamePeriodBothCounted`.

This is a different question from `Result.RecurringAdjustments`
(`RecurringSummary`): `Recurrence`/`REPEATED_ONE_TIME_ADJUSTMENTS` ask
"did this specific `Type` actually repeat in this dataset," which only
applies to the four nominally-non-recurring types. `RecurringSummary`
instead classifies **every** confirmed, applied line by whether its
`Type` is *inherently* the kind of thing expected to recur — owner
compensation normalization, personal vehicle/travel, owner-discretionary
expense, and related-party rent are always "recurring" in nature by this
classification, regardless of whether they happened to appear once or
every year in a given dataset. `non_operating_income` and `custom` land in
`RecurringSummary`'s `Unclassified` bucket rather than being guessed into
either side, since `non_operating_income`'s own definition covers both a
recurring investment-income stream and a one-off asset-sale gain without
distinguishing which, and `custom` is caller-defined with no inherent
nature this package can infer.

#### Earnings quality score

Optional, deterministic, and explicitly labeled a **heuristic composite,
not an accounting standard** (`Score.Heuristic` is always `true` in the
JSON output itself, not just in documentation). Populated only when
`Options.ComputeScore` is true — a caller that wants only flags and raw
measures gets exactly that, with no implied endorsement that one composite
number is meaningful for their use case.

Exact, fixed formula (`ScoreVersion`, versioned independently of
`FormulaVersion` — see [Versioning strategy](#versioning-strategy)):

```
100 points, baseline
- 15 points  per "critical"-severity Flag
-  8 points  per "warning"-severity Flag
-  3 points  per "info"-severity Flag
clamped to [0, 100]
```

Every deduction reads only `Result.Flags` (already-computed, already-
explainable rule-based signals) — `Score` never introduces a new threshold
or computation of its own, so it can never disagree with `Flags` about
whether something is a problem; it only weights already-identified problems
into one number. `Score.Components` lists each deduction in the same order
as `Result.Flags`, so a caller can reconstruct `Score.Value` from
`Result.Flags` alone. `Score.Label` is a fixed characterization band (`high
quality` ≥ 85, `moderate quality` ≥ 65, `elevated concern` ≥ 40, `low
quality` otherwise) — display only, never parsed.

#### Exported surface, by file

- **`types.go`** — `Input`, `Options`, `Result`, `PeriodFigures`,
  `AdjustmentBreakdown`, `TypeBreakdown`, `RecurrencePattern`,
  `RecurringSummary`, `Ratios`, `FlagCode`/`FlagSeverity`/`Flag`,
  `IssueCode`/`IssueSeverity`/`Issue`, `HasErrors`, `FormulaVersion`.
- **`thresholds.go`** — `Thresholds`, `DefaultThresholds()`.
- **`qoe.go`** — `Calculate(Input, Options) Result`, the per-period
  metrics/adjustments wiring, `AdjustmentBreakdown`/`Ratios` aggregation,
  and this package's own `SDEVolatility` computation.
- **`repeated.go`** — `buildRecurrence` (repeated one-time detection) and
  `buildRecurringSummary` (inherent recurring/non-recurring/unclassified
  classification — see [What `Result` contains](#what-result-contains)).
- **`flags.go`** — every deterministic flag-trigger rule.
- **`score.go`** — `Score`, `ScoreComponent`, `ScoreVersion`,
  `computeScore`.

See [`analytics/qoe/flags_test.go`](analytics/qoe/flags_test.go) and
[`analytics/qoe/qoe_test.go`](analytics/qoe/qoe_test.go) for every flag and
edge case (clean stable business, high adjustment burden, repeated one-time
costs, volatile earnings, declining margins despite revenue growth,
negative EBITDA, owner-heavy SDE, zero adjustments, missing/partial
`PeriodMeta`) exercised against both the repository's realistic multi-year
fixtures and hand-built minimal datasets sized to cross specific
thresholds.

### `analytics/workingcapital`

Analyzes historical **operating working capital** from a normalized
`financial.FinancialDataset` and supports transaction-style
working-capital **peg** analysis: comparing a current/target NWC figure
against a caller-selected historical benchmark and reporting the signed
excess or deficit.

There is no universal "deal definition" of operating working capital —
whether cash, debt, taxes payable, or shareholder/related-party balances
belong in the number is a negotiated, transaction-specific question. This
package never hard-codes one: every `financial.Code` counted as an
operating current asset or liability is controlled entirely by a
caller-supplied `InclusionPolicy`, and every code present in the dataset
but assigned to neither side is reported in `Result.ExcludedCodes` for
auditability rather than silently dropped. Every function is pure — no
I/O, no mutation of caller-owned input — and `Calculate` returns
byte-for-byte identical JSON across repeated runs against identical input.

```go
res := workingcapital.Calculate(workingcapital.Input{
    Dataset:    dataset,    // financial.FinancialDataset
    PeriodMeta: periodMeta, // map[financial.Period]workingcapital.PeriodInfo
    AsOf:       "2025-Q3",  // current transaction date's period (optional)
}, workingcapital.Options{
    PegMethod:       workingcapital.PegMethodTrailingAverage,
    TrailingPeriods: 3,
})

fmt.Println(res.NWCStatistics.Average.Value, res.SuggestedPeg.Value, res.PegComparison.ExcessDeficit)
```

#### The `InclusionPolicy`

```go
type InclusionPolicy struct {
    AssetCodes     []financial.Code // counted as operating current assets
    LiabilityCodes []financial.Code // counted as operating current liabilities
}
```

The zero value resolves to `DefaultInclusionPolicy()` (mirroring
`review.Policy`/`qoe.Thresholds`' identical zero-value-means-defaults
convention): every standard current-asset code **except cash**
(`BS_ACCOUNTS_RECEIVABLE`, `BS_INVENTORY`, `BS_PREPAID`,
`BS_CURRENT_ASSET_OTHER`) and every standard current-liability code
**except short-term debt** (`BS_ACCOUNTS_PAYABLE`,
`BS_CURRENT_LIABILITY_OTHER`) — the common convention that financing
balances (cash, interest-bearing debt) are settled separately at close
rather than trued up through a working-capital peg. A caller with a
deal-specific convention (e.g. including cash, excluding part of "other")
supplies its own policy; this repository's taxonomy has no dedicated codes
for "taxes payable" or "shareholder/related-party balances" distinct from
`BS_CURRENT_LIABILITY_OTHER`/`BS_CURRENT_ASSET_OTHER`, so isolating just
one of those requires classifying it onto a distinct code upstream first.

```
Operating Current Assets (per InclusionPolicy.AssetCodes)
- Operating Current Liabilities (per InclusionPolicy.LiabilityCodes)
= Net Working Capital (PeriodNWC.NWC)
```

#### What `Result` contains

| Field | What it is |
|---|---|
| `History` | One `PeriodNWC` per period (chronological when `PeriodMeta` is supplied and complete, else dataset lexical order): `OperatingCurrentAssets`/`OperatingCurrentLiabilities`/`NWC`, `Revenue` and `NWCPercentOfRevenue`, plus the full asset/liability `Component` bridge for that period. |
| `NWCStatistics` / `NWCPercentOfRevenueStatistics` | `Statistics` — average, median, min, max, and volatility (sample standard deviation of the period-over-period percentage-change series, the same method `financial/metrics.Trend` uses for revenue/EBITDA volatility) across `History`'s available observations. |
| `Trend` | A three-way `TrendDirection` (`increasing`/`declining`/`stable`) from comparing the first vs. last available `NWC` observation, using a fixed ±5% flat band (`TrendFlatBandPercent`) — never inferred from a display string. |
| `SeasonalProfile` | Present only when `PeriodMeta` marks quarter or month periods: `NWCPercentOfRevenue` averaged by calendar position (`SequenceInYear`) across every fiscal year sharing it — e.g. every Q4 averaged together — revealing a recurring seasonal pattern. Empty (not an error) for an annual-only dataset. |
| `SuggestedPeg` | The peg figure derived under `Options.PegMethod` (see below), plus which periods contributed to it. Zero value (`Method == ""`) if no method was requested — this package never picks a default peg method. |
| `PegComparison` | `CurrentNWC` (from `Input.CurrentNWC` if supplied, else derived from `Dataset` at `Input.AsOf`) vs. `SuggestedPeg.Value`, and the signed `ExcessDeficit = CurrentNWC - Peg`. This package reports only the arithmetic difference — it does not decide deal-specific true-up/settlement mechanics. |
| `ExcludedCodes` | Every balance-sheet `financial.Code` present in `Dataset` that `InclusionPolicy` assigned to neither side, sorted by `Code`. |
| `InclusionPolicy` | The resolved policy (after `DefaultInclusionPolicy` substitution) this `Result` was computed under, so a persisted `Result` remains self-describing. |
| `Warnings` / `Errors` | Structured `Issue`s (own `IssueCode` system — see [Error taxonomy](#error-taxonomy)) for input-level problems (`NO_PERIODS`, `NO_PERIOD_META`, `PERIOD_MISSING_FROM_META`, `NO_REVENUE_DATA`, `EMPTY_INCLUSION_POLICY`, `PEG_METHOD_UNAVAILABLE`, `CURRENT_NWC_UNAVAILABLE`). |

#### Peg methods

`Options.PegMethod` selects a deterministic strategy over `History`'s
available `NWC` observations — mirroring `financial/earnings.Strategy`'s
identical "pick a method, get an auditable trail" design. **This package
never invents a "market" peg** — every method reduces to arithmetic over
the caller's own historical series or a caller-supplied fixed figure:

| `PegMethod` | Derivation |
|---|---|
| `latest` | The single most recent available `NWC` observation. |
| `simple_average` | Unweighted arithmetic mean of every available observation. |
| `median` | Median of every available observation. |
| `trailing_average` | Unweighted arithmetic mean of the most recent `Options.TrailingPeriods` available observations. Unavailable (with `PEG_METHOD_UNAVAILABLE`) if `TrailingPeriods` is unset or exceeds the number of available observations. |
| `fixed` | `Options.FixedPeg` verbatim — for a peg dollar figure already negotiated outside this package (e.g. from a letter of intent). Uses no historical periods. |

#### Exported surface, by file

- **`types.go`** — `Input`, `Options`, `Result`, `PeriodInfo`/`PeriodType`,
  `NWCValue`/`Unavailable`/`AvailableValue`, `Component`, `PeriodNWC`,
  `Statistics`, `Trend`/`TrendDirection`, `SeasonalProfile`/
  `SeasonalPeriod`, `PegMethod`, `SuggestedPeg`, `PegComparison`,
  `IssueCode`/`IssueSeverity`/`Issue`, `HasErrors`, `FormulaVersion`.
- **`policy.go`** — `InclusionPolicy`, `DefaultInclusionPolicy()`.
- **`workingcapital.go`** — `Calculate(Input, Options) Result`, per-period
  NWC/revenue computation, chronological ordering, and current-NWC
  resolution.
- **`stats.go`** — `Statistics`/`Trend` computation (average, median, min,
  max, volatility, direction).
- **`seasonal.go`** — `SeasonalProfile` computation.
- **`peg.go`** — every `PegMethod` strategy and `PegComparison`.

See [`analytics/workingcapital/workingcapital_test.go`](analytics/workingcapital/workingcapital_test.go),
[`analytics/workingcapital/peg_test.go`](analytics/workingcapital/peg_test.go),
[`analytics/workingcapital/stats_test.go`](analytics/workingcapital/stats_test.go), and
[`analytics/workingcapital/seasonal_test.go`](analytics/workingcapital/seasonal_test.go)
for every scenario (seasonal retail business, stable/declining/increasing
NWC, cash/debt inclusion and exclusion, negative working capital, missing
revenue, quarterly-vs-annual comparability, every peg method's excess/
deficit comparison, and JSON/determinism) exercised against both the
repository's realistic multi-year fixtures and hand-built minimal datasets.

### `analytics/ratios`

A deterministic **financial-ratio suite** — profitability, liquidity,
leverage, efficiency, and growth ratios — computed from a normalized
`financial.FinancialDataset`, plus period-over-period trend/comparison
output and caller-configurable, rule-based health signals.

Like `analytics/qoe` and `analytics/workingcapital`, this package
recomputes whatever `financial/metrics.Snapshot` figures it needs directly
from `Input.Dataset` rather than requiring a caller to separately run
`financial/metrics` first. Every ratio is an **ending-balance** calculation
for its period — this package never averages a balance-sheet figure across
two periods, consistent with every other formula in this repository
(`financial/metrics.debtMetrics`/`workingCapital`,
`workingcapital.PeriodNWC`). Every function is pure — no I/O, no mutation
of caller-owned input — and `Calculate` returns byte-for-byte identical
JSON across repeated runs against identical input.

```go
res := ratios.Calculate(ratios.Input{
    Dataset:    dataset,    // financial.FinancialDataset
    PeriodMeta: periodMeta, // map[financial.Period]ratios.PeriodInfo
}, ratios.Options{})

latest := res.History[len(res.History)-1]
fmt.Println(latest.CurrentRatio.Value.Value, latest.DebtToEBITDA.Value.Value, res.Signals)
```

#### Total Assets and Total Equity

This repository's taxonomy has no single "Total Assets" or "Total Equity"
`financial.Code` (see `financial/taxonomy.go`), and no package anywhere in
this repository assumes the balance-sheet identity Assets = Liabilities +
Equity holds for a given dataset. `analytics/ratios` derives both figures
directly as a **sum of existing taxonomy codes** (`components.go`), the
same approach `analytics/workingcapital` already uses for its own
operating-current-asset/liability sums, rather than deriving one side as a
plug from the other (which would silently mask a source statement that
does not actually balance):

```
Total Assets = Cash + AR + Inventory + Prepaid + Other Current Assets
             + (Fixed Assets - Accumulated Depreciation) + Intangible Assets + Goodwill
Total Equity = Retained Earnings + Owner Equity
```

Both are `metrics.MetricValue`s, `Available` only if at least one
contributing code is present in the dataset for that period — a dataset
with no balance-sheet data at all leaves every Total-Assets-or-Total-Equity-
dependent ratio (`ReturnOnAssets`, `ReturnOnEquity`, `DebtToEquity`,
`DebtToAssets`, `AssetTurnover`) `Unavailable`, not silently zero.

#### What `Result` contains

| Field | What it is |
|---|---|
| `History` | One `PeriodRatios` per period (chronological when `PeriodMeta` is supplied and complete, else dataset lexical order): every ratio below, the full `metrics.Snapshot`, and this package's own `TotalAssets`/`TotalEquity`. |
| `Trends` | One `RatioTrend` per ratio metric with at least one available observation: a three-way `RatioTrendDirection` (`increasing`/`declining`/`stable`) from the first vs. last available observation, using the same fixed ±5% flat band (`TrendFlatBandPercent`) `analytics/workingcapital.Trend` uses. Empty unless `PeriodMeta` covers every period. |
| `Comparisons` | Every adjacent-period `Comparison` (`FromValue`/`ToValue`/`Change`) for every ratio metric with at least one available observation — the finer-grained, every-adjacent-pair counterpart to `Trends`' first-vs-last view, and what `Signals` is derived from. Empty under the same condition as `Trends`. |
| `Growth` | `RevenueGrowth`/`GrossProfitGrowth`/`EBITDAGrowth`/`NetIncomeGrowth` — period-over-period `GrowthPoint` series using the same `(to-from)/\|from\|` formula `financial/metrics.Trend` uses for its own YoY growth, but **not restricted to fiscal-year-to-fiscal-year comparisons** the way `metrics.Trend`'s MVP scope is (see `metrics.Trend`'s doc comment) — computed across whatever chronological sequence `Input.PeriodMeta` establishes. Empty under the same condition as `Trends`. |
| `Signals` | Every deterministic health signal that triggered, in `SignalCode` declaration order. See [Deterministic health signals](#deterministic-health-signals-ratios) below. |
| `Thresholds` | The resolved `Thresholds` (after `DefaultThresholds` substitution) this `Result` was computed under, so a persisted `Result` remains self-describing. |
| `Warnings` / `Errors` | Structured `Issue`s (own `IssueCode` system) for input-level problems (`NO_PERIODS`, `NO_PERIOD_META`, `PERIOD_MISSING_FROM_META`). |

#### Ratios computed

Every ratio is a `Ratio` (`Metric`/`Period`/`Value`/`Formula`/`Components`,
mirroring `metrics.MetricResult`'s shape), `Available` only when every
required input is present **and** any denominator is present and nonzero
— never a silent 0 or a divide-by-zero result:

| Category | Ratio | Formula |
|---|---|---|
| Profitability | `GrossMargin` | Gross Profit / Total Revenue |
| | `OperatingMargin` | EBIT / Total Revenue |
| | `EBITDAMargin` | EBITDA / Total Revenue |
| | `NetMargin` | Net Income / Total Revenue |
| | `ReturnOnAssets` | Net Income / Total Assets |
| | `ReturnOnEquity` | Net Income / Total Equity |
| Liquidity | `CurrentRatio` | Current Assets / Current Liabilities |
| | `QuickRatio` | (Cash + AR + Other Current Assets) / Current Liabilities |
| | `CashRatio` | Cash / Current Liabilities |
| Leverage | `DebtToEquity` | Total Debt / Total Equity |
| | `DebtToAssets` | Total Debt / Total Assets |
| | `DebtToEBITDA` | Total Debt / EBITDA |
| | `NetDebtToEBITDA` | Net Debt / EBITDA |
| | `InterestCoverage` | EBIT / Interest Expense |
| Efficiency | `AssetTurnover` | Total Revenue / Total Assets |
| | `ReceivablesTurnover` | Total Revenue / Accounts Receivable |
| | `InventoryTurnover` | Total COGS / Inventory |
| | `DaysSalesOutstanding` | (Accounts Receivable / Total Revenue) × 365 |
| | `DaysInventoryOutstanding` | (Inventory / Total COGS) × 365 |
| | `DaysPayableOutstanding` | (Accounts Payable / Total COGS) × 365 |
| | `CashConversionCycle` | DSO + DIO − DPO |

A negative result (e.g. `ReturnOnEquity` against an accumulated deficit, or
`DebtToEBITDA` against negative EBITDA) is reported as computed, not
suppressed — this package reports the arithmetic result of available data
and leaves interpretation of an unusual figure to the caller.

Every days-outstanding/`CashConversionCycle` ratio uses a **fixed 365-day
year** (`daysInPeriod`), regardless of a period's actual granularity. A
quarterly or monthly period's revenue/COGS is that period's own
(non-annualized) figure, so this package deliberately does not attempt to
annualize a sub-annual figure by multiplying by 4 or 12 — doing so would
assume no seasonality, an assumption `analytics/workingcapital`'s
`SeasonalProfile` exists specifically because it often does *not* hold. A
caller comparing a quarterly DSO against an annual one must annualize the
underlying revenue/COGS themselves first.

#### Deterministic health signals {#deterministic-health-signals-ratios}

Every signal is rule-based against caller-configurable `Thresholds`
(`DefaultThresholds()` for the conservative defaults; the zero `Thresholds`
passed to `Calculate` resolves to these) — **no AI, no opaque scoring, and
no composite "health score"**: every signal reads only an already-computed
`Comparison` or a single most-recent-period ratio level, so a caller can
always trace a `Signal` back to the exact `Comparison`/`Ratio` it came
from.

| `SignalCode` | Triggers when |
|---|---|
| `WEAKENING_LIQUIDITY` | `CurrentRatio`'s most recent period-over-period `Comparison` declined by ≥ `LiquidityDeclineThreshold` (default 0.20). |
| `RISING_LEVERAGE` | `DebtToEBITDA`'s most recent `Comparison` increased by ≥ `LeverageIncreaseThreshold` (default 0.50x). |
| `MARGIN_COMPRESSION` | `EBITDAMargin`'s most recent `Comparison` declined by ≥ `MarginCompressionThreshold` (default 3 points). |
| `SLOWING_COLLECTIONS` | `DaysSalesOutstanding`'s most recent `Comparison` increased by ≥ `CollectionsSlowdownDays` (default 10 days). |
| `INVENTORY_BUILDUP` | `DaysInventoryOutstanding`'s most recent `Comparison` increased by ≥ `InventoryBuildupDays` (default 10 days). |
| `WEAK_INTEREST_COVERAGE` | The most recent period's `InterestCoverage` is available and ≤ `WeakInterestCoverageRatio` (default 1.5x). |
| `IMPROVING_PROFITABILITY` | `EBITDAMargin`'s most recent `Comparison` improved by ≥ `ProfitabilityChangeThreshold` (default 2 points). |
| `DETERIORATING_PROFITABILITY` | `EBITDAMargin`'s most recent `Comparison` worsened by ≥ `ProfitabilityChangeThreshold` (default 2 points). |

`MARGIN_COMPRESSION` and `DETERIORATING_PROFITABILITY` both read the same
`EBITDAMargin` `Comparison` and commonly fire together on a meaningful
decline (compression uses a less sensitive default threshold and names the
specific concern; deteriorating/improving profitability is the more general
directional signal the task's own signal list calls for as a separate
bullet) — every signal carries its own `Code`, so a caller filters on
whichever specific signal it cares about. Every `Signal` carries a stable
`Code`, a `Severity` (`info`/`warning`/`critical`), the `Period` it
concerns, a pre-filled `Message` (display only, never parsed), and the
exact `Value`/`Threshold` compared.

#### Exported surface, by file

- **`types.go`** — `Input`, `Options`, `Result`, `PeriodInfo`/`PeriodType`,
  `Component`, `Ratio`, the `RatioXxx` metric-name constants,
  `PeriodRatios`, `GrowthPoint`, `Growth`, `RatioTrendDirection`/
  `RatioTrend`, `Comparison`, `SignalCode`/`SignalSeverity`/`Signal`,
  `IssueCode`/`IssueSeverity`/`Issue`, `HasErrors`, `FormulaVersion`,
  `SignalRulesVersion`.
- **`policy.go`** — `Thresholds`, `DefaultThresholds()`.
- **`components.go`** — Total Assets/Total Equity/quick-assets/total-debt
  sum-of-codes definitions and `sumCodes`.
- **`index.go`** — the internal `codeIndex` lookup.
- **`ratios.go`** — `Calculate(Input, Options) Result` and per-period ratio
  wiring.
- **`profitability.go`**, **`liquidity.go`**, **`leverage.go`**,
  **`efficiency.go`** — every ratio formula in each category.
- **`growth.go`** — the `Growth` period-over-period series.
- **`trend.go`** — chronological ordering, `Trends`, `Comparisons`.
- **`signals.go`** — every deterministic signal-trigger rule.

See [`analytics/ratios/ratios_test.go`](analytics/ratios/ratios_test.go),
[`analytics/ratios/signals_test.go`](analytics/ratios/signals_test.go),
[`analytics/ratios/determinism_test.go`](analytics/ratios/determinism_test.go),
and [`analytics/ratios/roundtrip_test.go`](analytics/ratios/roundtrip_test.go)
for every scenario (normal case, zero denominators, negative equity,
negative earnings, missing balance sheet, multi-year trend, every signal's
positive and suppressed case, custom thresholds, and JSON/determinism)
exercised against both the repository's realistic multi-year fixtures and
hand-built minimal datasets.

### `analytics/cashflow`

A deterministic **cash-flow and cash-conversion analysis** for SMB advisory
and transaction (M&A) use: a period-by-period bridge from EBITDA to free
cash flow, EBITDA-to-cash conversion ratios, debt-service and
owner-distribution coverage, recurring cash drains, and (for loss-making
businesses) a cash burn/runway estimate.

This repository's canonical `financial.Code` taxonomy has no dedicated
codes for a cash-flow statement, capital expenditures, debt-service
principal/interest, or owner distributions — those figures vary too much in
how source systems report them to force into one flat code list. So this
package recomputes EBITDA (`financial/metrics`) and the period-over-period
change in net working capital (`analytics/workingcapital`) itself directly
from `Input.Dataset`, exactly as `analytics/qoe` and `analytics/ratios`
already do for `financial/metrics`, and takes operating cash flow, capex,
debt service, owner distributions, and cash taxes paid as **caller-supplied,
per-period `CashFlowValue` figures** — this package never invents a
canonical source for them.

**Nothing is inferred from EBITDA unless a caller explicitly opts in** via
`Options.AllowEBITDAEstimate`. When it does, the affected `CashFlowValue`
carries `IsEstimate == true` and a populated `EstimateBasis` explaining
exactly how it was derived, and `Result.Warnings` notes that estimation
occurred — this package never blends a reported figure and an estimated one
into one undifferentiated number. Every function is pure — no I/O, no
mutation of caller-owned input — and `Calculate` returns byte-for-byte
identical JSON across repeated runs against identical input.

```go
res := cashflow.Calculate(cashflow.Input{
    Dataset:    dataset,    // financial.FinancialDataset
    PeriodMeta: periodMeta, // map[financial.Period]metrics.PeriodInfo
    OperatingCashFlow: map[financial.Period]cashflow.CashFlowValue{
        "2025": cashflow.Reported(320_000),
    },
    Capex: map[financial.Period]cashflow.CashFlowValue{
        "2025": cashflow.Reported(40_000),
    },
    DebtService: map[financial.Period]cashflow.DebtServiceFigure{
        "2025": {Principal: cashflow.Reported(50_000), Interest: cashflow.Reported(10_000)},
    },
}, cashflow.Options{AllowEBITDAEstimate: true}) // opt-in EBITDA-based estimate for periods with no reported OCF

last := res.History[len(res.History)-1]
fmt.Println(last.FreeCashFlow.Value, last.FreeCashFlowToOwner.Value)
```

#### The EBITDA-to-free-cash-flow bridge

```
EBITDA (financial/metrics.Snapshot.EBITDA, recomputed from Dataset)
- Change in Net Working Capital (analytics/workingcapital, under Input.Policy)
= Operating Cash Flow  (reported, or an EBITDA-based estimate — see below)
- Capex
= Free Cash Flow
+ Interest (Bridge.DebtService.Interest)
= Free Cash Flow to Firm     (all-capital-providers figure; pre-tax-shield approximation — this
                               package has no tax-rate input to tax-affect interest with)
- Debt Service (Principal + Interest)
= Free Cash Flow to Owner    (cash an owner can draw without impairing the business or
                               defaulting on debt)
```

Every step is an independently `Available` `CashFlowValue`/`MetricValue` —
a missing `Capex` figure does not block `FreeCashFlow` (it is treated as
zero, with `IsEstimate`/`EstimateBasis` noting exactly that assumption, so a
caller can never mistake "not supplied" for "confirmed to be zero" — see
`Bridge.FreeCashFlow`'s doc comment), and a missing `DebtService` figure
leaves only `FreeCashFlowToFirm`/`FreeCashFlowToOwner` unavailable rather
than the whole bridge.

#### The EBITDA-based estimate (opt-in only)

With `Options.AllowEBITDAEstimate: true`, a period with no reported
`Input.OperatingCashFlow` gets `EBITDA - ChangeInNWC` instead, **only** when
both are available (in particular, never for the earliest period in a
series, which has no preceding period to diff `ChangeInNWC` against). The
resulting `CashFlowValue` carries `IsEstimate: true` and
`EstimateBasis: "EBITDA - change in net working capital; no reported
operating cash flow supplied for this period"` — grep-able and
programmatically filterable, never buried in a display string. Without the
opt-in, an unreported period's `OperatingCashFlow` (and everything
downstream of it) is simply `Unavailable()`, and `Result.Warnings` carries
`NO_CASH_FLOW_STATEMENT` instead.

#### What `Result` contains

| Field | What it is |
|---|---|
| `History` | One `Bridge` per period (chronological when `PeriodMeta` is supplied and complete, else dataset lexical order): the full EBITDA-to-free-cash-flow walk above, plus `OwnerDistributions` and `CashTaxesPaid` echoed verbatim from `Input`. |
| `Conversion` | One `ConversionRatios` per period: `EBITDAToOperatingCashFlow` and `EBITDAToFreeCashFlow` — how much of EBITDA actually became cash. |
| `OperatingCashFlowTrend` / `FreeCashFlowTrend` / `ConversionTrend` | A three-way `TrendDirection` (`increasing`/`declining`/`stable`) from the first vs. last available observation, using the same fixed ±5% flat band (`TrendFlatBandPercent`) `analytics/workingcapital.Trend` and `analytics/ratios.RatioTrend` use. |
| `RecurringDrains` | Four fixed `RecurringDrain` entries (`capex`, `working_capital_build`, `debt_service`, `owner_distributions`), each with `TotalAmount` across `History` and `AverageOfEBITDA` — recurring cash outflows that reduce owner cash even when EBITDA looks healthy. |
| `CashRunway` | `Available` only when average monthly operating cash flow is negative (i.e. the business is actually burning cash): `MonthlyBurnRate`, `CurrentCashBalance` (most recent `Input.CashBalance`), and `MonthsOfRunway`. A profitable business gets `Available == false`, never an infinite or nonsensical runway figure. |
| `Flags` | Deterministic, `Thresholds`-driven signals (see below), ordered by `FlagCode`'s declaration order, then `Period`. |
| `Warnings` / `Errors` | Structured `Issue`s (own `IssueCode` system — see [Error taxonomy](#error-taxonomy)) for input-level problems (`NO_PERIODS`, `NO_PERIOD_META`, `PERIOD_MISSING_FROM_META`, `NO_CASH_FLOW_STATEMENT`, `ESTIMATED_FROM_EBITDA`). |

#### Flags

`Options.Thresholds` (zero value resolves to `DefaultThresholds()`, the
same zero-value-means-defaults convention every other package's policy type
uses) configures seven rule-based, deterministic triggers evaluated against
the most recent period in `History`: `WEAK_CASH_CONVERSION`,
`HIGH_CAPEX_BURDEN`, `HIGH_WORKING_CAPITAL_BURDEN`,
`LOW_DEBT_SERVICE_COVERAGE`, `DISTRIBUTIONS_EXCEED_FREE_CASH_FLOW`,
`LOW_CASH_RUNWAY`, and `DECLINING_CONVERSION_TREND`. Every `Flag` carries a
stable `Code`, a `Severity` (`info`/`warning`/`critical`), the specific
`Value`/`Threshold` compared, and a pre-filled `Message` (display only,
never parsed) — no AI/LLM, no opaque scoring, mirroring `analytics/qoe.Flag`
and `analytics/ratios.Signal`'s identical design.

#### Exported surface, by file

- **`types.go`** — `Input`, `Options`, `Result`, `PeriodInfo`/`PeriodType`,
  `CashFlowValue`/`Unavailable`/`Reported`/`Estimated`, `DebtServiceFigure`,
  `Bridge`, `ConversionRatios`, `TrendDirection`/`Trend`, `RecurringDrain`/
  `RecurringDrainCategory`, `CashRunway`, `Thresholds`/`DefaultThresholds()`,
  `FlagCode`/`FlagSeverity`/`Flag`, `IssueCode`/`IssueSeverity`/`Issue`,
  `HasErrors`, `FormulaVersion`.
- **`cashflow.go`** — `Calculate(Input, Options) Result`: recomputes
  `financial/metrics` and `analytics/workingcapital` internally, builds each
  period's `Bridge`, and wires every other section together.
- **`trend.go`** — `Trend` computation for the OCF/FCF/conversion series.
- **`drains.go`** — `RecurringDrains` aggregation.
- **`runway.go`** — `CashRunway` burn-rate/runway computation.
- **`flags.go`** — every deterministic flag-trigger rule.

See [`analytics/cashflow/cashflow_test.go`](analytics/cashflow/cashflow_test.go),
[`analytics/cashflow/determinism_test.go`](analytics/cashflow/determinism_test.go),
and [`analytics/cashflow/roundtrip_test.go`](analytics/cashflow/roundtrip_test.go)
for every scenario (strong/weak conversion, working-capital build,
capex-heavy, negative cash flow/runway, missing cash-flow statement, the
estimate-vs-reported distinction, debt-service and distribution coverage,
and JSON/determinism) exercised against both the repository's realistic
multi-year fixtures and hand-built minimal datasets.

### `analytics/revenuequality`

A deterministic **revenue-quality analysis**: period-level revenue
composition (recurring vs. non-recurring), growth/CAGR/volatility, and —
when a caller supplies customer-level detail — customer retention,
new/lost/expansion/contraction revenue, and concentration
(top-N shares, HHI).

This repository's canonical `financial.Code` taxonomy carries revenue at
only four flat codes (`CodeRevProduct`, `CodeRevService`,
`CodeRevRecurring`, `CodeRevOther`), each a mutually-exclusive
classification with no finer recurring/non-recurring split within a single
line item. So this package's dataset-level split is exactly as fine-grained
as the taxonomy allows: `CodeRevRecurring` is recurring revenue, and the
other three summed together are non-recurring — a caller whose source data
needs finer granularity reclassifies upstream, onto `CodeRevRecurring`,
before this package can see it as recurring.

Customer-level analysis is **entirely optional** and activates only when a
caller supplies `Input.CustomerRevenue` — a portable
`CustomerPeriodRevenue{CustomerKey, Period, Amount, RecurringFlag, Segment}`
input type with no PII requirement and no application customer/account
object. Customer-over-customer comparisons are computed only between
**chronologically adjacent periods** (never arbitrary caller-specified
pairs), mirroring `analytics/cashflow.Bridge.ChangeInNWC`'s identical
adjacent-period convention.

**This package never claims SaaS-style NRR/GRR.** Those ratios carry
contractual assumptions (a defined subscription book, cohort tracking, a
consistent renewal cadence) this package's caller-agnostic input cannot
guarantee holds for every business it might analyze. Instead it reports the
underlying dollar movements directly — `RetainedRevenue`,
`NewCustomerRevenue`, `LostCustomerRevenue`, `ExpansionRevenue`,
`ContractionRevenue` — and leaves any SaaS-specific ratio as arithmetic a
caller performs once it has independently confirmed its business matches
that model's assumptions.

Every function is pure — no I/O, no mutation of caller-owned input — and
`Calculate` returns byte-for-byte identical JSON across repeated runs
against identical input, regardless of Go's randomized map iteration order
(every customer/segment aggregation this package performs internally is
accumulated in sorted-key order before any float64 summation, since
floating-point addition is not associative).

```go
res := revenuequality.Calculate(revenuequality.Input{
    Dataset:    dataset,    // financial.FinancialDataset
    PeriodMeta: periodMeta, // map[financial.Period]revenuequality.PeriodInfo
    CustomerRevenue: []revenuequality.CustomerPeriodRevenue{
        {CustomerKey: "cust-1", Period: "2024", Amount: 120_000, Segment: "enterprise"},
        {CustomerKey: "cust-1", Period: "2025", Amount: 150_000, Segment: "enterprise"},
        {CustomerKey: "cust-2", Period: "2025", Amount: 40_000, Segment: "smb"}, // new in 2025
    },
}, revenuequality.Options{})

last := res.TotalRevenueHistory[len(res.TotalRevenueHistory)-1]
fmt.Println(last.RecurringPercent.Value, res.ConcentrationSummary.HHI.Value)
```

#### Revenue composition and trend

Each `PeriodRevenue` in `TotalRevenueHistory` carries `TotalRevenue`,
`RecurringRevenue`, `NonRecurringRevenue`, and their percentages —
`RecurringPercent`/`NonRecurringPercent` are `Available` only when all three
underlying figures are (never computed from a partial revenue picture, e.g.
a dataset with `CodeRevRecurring` but a missing `CodeRevProduct` for a
period that actually earned product revenue). `RevenueTrend` (first-vs-last,
the same fixed ±5% flat band every sibling package's `Trend` uses),
`RevenueGrowth` (one `GrowthPoint` per chronologically adjacent pair),
`RevenueCAGR`, and `RevenueVolatility` (sample standard deviation of the
period-over-period growth-rate series) all require `Input.PeriodMeta` to
establish chronological order — without it, they are left `Unavailable`
with an advisory `NO_PERIOD_META` warning, while per-period
`TotalRevenueHistory` still computes fully in dataset lexical order.

#### Customer transitions

Each `CustomerTransition` compares one chronologically adjacent period pair
and classifies every customer into new, lost, retained, expanded, or
contracted, summing each category's dollar impact —
`RetainedRevenue + ExpansionRevenue - ContractionRevenue` reconstructs each
retained customer's full to-period revenue. `FromPeriodTotalRevenue` /
`ToPeriodTotalRevenue` / `TotalRevenueGrowth` are each period's true summed
customer revenue, computed once and stored directly on the transition —
every ratio this package derives against "the from-period total" (the
lost-revenue and shrinking-base flags below) reads these fields rather than
reconstructing the total from the categorized fields, which are **not**
algebraically sufficient to recover it (`RetainedRevenue` is already
`min(from, to)` per customer, so `ExpansionRevenue` sits on top of it, not
inside it — subtracting it back out silently undercounts the true total).
`ExistingCustomerBaseChange` isolates the net change in revenue from
customers who were already present in the from-period, excluding
`NewCustomerRevenue` entirely — negative means the existing base shrank even
before counting new-customer growth.

#### Concentration

`ConcentrationSummary` is computed once, for the most recent period present
in `Input.CustomerRevenue` — concentration risk is a point-in-time
diligence question, not a trend. `TopNShares` reports the share of total
customer revenue held by each `Policy.ConcentrationTopN` cutoff (default
`[1, 5, 10]`); `HHI` is the Herfindahl-Hirschman Index on the conventional
0–10,000 scale; `Segments` breaks down revenue by `CustomerPeriodRevenue.Segment`
when supplied. `UnallocatedRevenue` (`Dataset`'s total revenue minus the sum
of customer-level rows for that period) surfaces a reconciliation mismatch
as data, not an error — a caller's customer export commonly covers only a
subset of total revenue streams.

#### What `Result` contains

| Field | What it is |
|---|---|
| `TotalRevenueHistory` | One `PeriodRevenue` per period (chronological when `PeriodMeta` is supplied and complete, else dataset lexical order): total/recurring/non-recurring revenue and their percentages. |
| `RevenueStatistics` / `RevenueTrend` / `RevenueGrowth` / `RevenueCAGR` / `RevenueVolatility` | Central tendency/dispersion, first-vs-last direction, period-over-period growth points, compound annual growth rate, and growth-rate volatility across `TotalRevenueHistory`. |
| `CustomerHistory` | One `CustomerPeriodTotal` per period with customer data: customer count, total customer revenue, and (when any row supplied `RecurringFlag`) the recurring share. |
| `CustomerTransitions` | One `CustomerTransition` per chronologically adjacent period pair with customer data on both sides — see above. |
| `ConcentrationSummary` | Top-N shares, HHI, and segment breakdown for the most recent period with customer data. |
| `Flags` | Deterministic, `Thresholds`-driven signals (see below), ordered by `FlagCode`'s declaration order, then `Period`. |
| `Warnings` / `Errors` | Structured `Issue`s (own `IssueCode` system — see [Error taxonomy](#error-taxonomy)) for input-level problems (`NO_PERIODS`, `NO_PERIOD_META`, `PERIOD_MISSING_FROM_META`, `NO_REVENUE_DATA`, `NO_CUSTOMER_DATA`, `CUSTOMER_PERIOD_NOT_IN_DATASET`, `CUSTOMER_REVENUE_UNRECONCILED`). |

#### Flags

`Options.Thresholds` (zero value resolves to `DefaultThresholds()`)
configures six rule-based, deterministic triggers:
`DECLINING_RECURRING_MIX`, `GROWTH_DEPENDENT_ON_NEW_CUSTOMERS`,
`HIGH_LOST_CUSTOMER_REVENUE`, `VOLATILE_REVENUE`, `ONE_PERIOD_SPIKE`, and
`SHRINKING_EXISTING_CUSTOMER_BASE`. Every `Flag` carries a stable `Code`, a
`Severity` (`info`/`warning`/`critical`), the specific `Value`/`Threshold`
compared, and a pre-filled `Message` (display only, never parsed) — no
AI/LLM, no opaque scoring, mirroring `analytics/qoe.Flag` and
`analytics/cashflow.Flag`'s identical design.

#### Exported surface, by file

- **`types.go`** — `Input`, `Options`, `Result`, `PeriodInfo`/`PeriodType`,
  `RevenueValue`/`Unavailable`/`AvailableValue`, `CustomerPeriodRevenue`,
  `Policy`/`DefaultPolicy()`, `PeriodRevenue`, `Component`, `Statistics`,
  `TrendDirection`/`Trend`, `GrowthPoint`, `CAGRResult`, `VolatilityResult`,
  `CustomerPeriodTotal`, `CustomerTransition`, `TopNShare`, `SegmentShare`,
  `ConcentrationSummary`, `Thresholds`/`DefaultThresholds()`,
  `FlagCode`/`FlagSeverity`/`Flag`, `IssueCode`/`IssueSeverity`/`Issue`,
  `HasErrors`, `FormulaVersion`.
- **`revenuequality.go`** — `Calculate(Input, Options) Result`: builds
  `TotalRevenueHistory` from `Input.Dataset` and wires every other section
  together.
- **`stats.go`** — `Statistics`/`Trend`/`GrowthPoint`/`CAGRResult`/
  `VolatilityResult` computation.
- **`customers.go`** — customer-row validation, `CustomerHistory`,
  `CustomerTransitions`, and `ConcentrationSummary` computation.
- **`flags.go`** — every deterministic flag-trigger rule.

See [`analytics/revenuequality/revenuequality_test.go`](analytics/revenuequality/revenuequality_test.go),
[`analytics/revenuequality/determinism_test.go`](analytics/revenuequality/determinism_test.go),
and [`analytics/revenuequality/roundtrip_test.go`](analytics/revenuequality/roundtrip_test.go)
for every scenario (recurring service business, project business, customer
churn, growth through new customers, volatile revenue, one-period spike,
declining recurring mix, missing customer detail, concentration/HHI,
unreconciled customer revenue, and JSON/determinism) exercised against both
the repository's realistic multi-year fixtures and hand-built minimal
datasets.

### `analytics/concentration`

A deterministic **concentration-risk analysis**: how dependent a business is
on its largest customers, suppliers, or any other counterparty relationship,
from caller-supplied revenue/spend observations.

Unlike `analytics/revenuequality` (which reads optional customer detail as
an enrichment of a normalized `financial.FinancialDataset`), this package is
**entirely independent of `financial.FinancialDataset` and the
`financial.Code` taxonomy**. Concentration analysis is frequently performed
on data a financial dataset never carries: a per-customer invoicing export,
per-vendor accounts-payable detail, or per-referral-source revenue. So this
package defines its own minimal, fully portable
`Observation{EntityKey, Period, Amount, Category}` tuple and never requires
a dataset-shaped input. `Input.Basis` (`customer_revenue`/`supplier_spend`/
`other`) labels what `Amount` represents purely for display — every metric
(shares, HHI, ranking, scenarios) is computed identically regardless of
`Basis`.

`EntityKey` is an opaque, caller-assigned string — this package never
requires, stores, or infers any real name, email, address, or other PII,
the same privacy convention `analytics/revenuequality.CustomerPeriodRevenue`
established for customer-level detail.

Every function is pure — no I/O, no mutation of caller-owned input — and
`Calculate` returns byte-for-byte identical JSON across repeated runs
against identical input, regardless of Go's randomized map iteration order.

```go
rate := 0.35 // caller's blended contribution-margin assumption
res := concentration.Calculate(concentration.Input{
    Basis: concentration.BasisCustomerRevenue,
    Observations: []concentration.Observation{
        {EntityKey: "cust-1", Period: "2024", Amount: 700_000, Category: "enterprise"},
        {EntityKey: "cust-2", Period: "2024", Amount: 150_000, Category: "smb"},
        {EntityKey: "cust-1", Period: "2025", Amount: 850_000, Category: "enterprise"},
        {EntityKey: "cust-2", Period: "2025", Amount: 100_000, Category: "smb"},
    },
    PeriodMeta: map[financial.Period]concentration.PeriodInfo{
        "2024": {Type: concentration.PeriodTypeFiscalYear, FiscalYear: 2024},
        "2025": {Type: concentration.PeriodTypeFiscalYear, FiscalYear: 2025},
    },
    Policy: concentration.Policy{DefaultImpactMarginRate: &rate},
}, concentration.Options{})

latest := res.History[len(res.History)-1]
fmt.Println(latest.LargestEntityShare.Value, latest.HHI.Value)
fmt.Println(res.Scenarios[0].TotalEarningsImpact.Value) // lost-largest-entity earnings impact
```

#### Per-period concentration

Each `PeriodConcentration` in `History` carries `RankedEntities` (sorted
descending by amount, ties broken by `EntityKey` for determinism),
`LargestEntityShare`, `TopNShares` (one per `Policy.TopN` cutoff, default
`[1, 3, 5, 10]`), `HHI` (the conventional 0–10,000-scale
Herfindahl-Hirschman Index), and an optional `Categories` breakdown.
`LargestShareTrend`/`HHITrend` (first-vs-last, the same fixed ±5% flat band
every sibling package's `Trend` uses) require `Input.PeriodMeta` to
establish chronological order — without it, per-period `History` still
computes fully (in order of first appearance), but trends, dependency
changes, and scenarios are left `Unavailable` with an advisory
`NO_PERIOD_META` warning.

#### Dependency changes

Each `DependencyChange` compares one entity's amount and share of total
between two chronologically adjacent periods — mirroring
`revenuequality.CustomerTransition`'s identical adjacent-period-only
convention, but reported as one flat record per (entity, period-pair)
rather than pre-aggregated new/lost/retained categories, since a caller
most often wants dependency changes sorted or filtered per entity.
`FromAmount`/`ToAmount` being `Unavailable` (rather than `0`) distinguishes
"this entity had no observation in this period" from "this entity had a
zero-dollar observation."

#### Scenarios

`Scenarios` always includes `ScenarioLostLargestEntity` plus one
`ScenarioTopNLoss` per `Policy.ScenarioTopN` cutoff (default `[3, 5]`), all
computed against the chronologically most recent period in `History` —
concentration risk is a point-in-time diligence question, not a trend, the
same convention `revenuequality.ConcentrationSummary` uses. Each
`Scenario.TotalRevenueImpact`/`RemainingRevenue`/`RevenueImpactPercent` is
always available; `TotalEarningsImpact` is available only if **every**
entity the scenario removes has an applicable margin rate (a partial sum
would understate the true earnings impact without saying so). A margin
rate applies per entity from `Policy.EntityImpactAssumptions` (a per-entity
override) or `Policy.DefaultImpactMarginRate` (a blended default); if
neither is supplied for an entity a scenario removes, that scenario's
`TotalEarningsImpact` (and that entity's own `EntityImpact.EarningsImpact`)
is left `Unavailable` and `NO_IMPACT_ASSUMPTION` is recorded as an advisory
warning — this package never invents a margin assumption a caller did not
supply, and the warning is scoped only to entities a configured `Scenario`
actually removes.

#### What `Result` contains

| Field | What it is |
|---|---|
| `History` | One `PeriodConcentration` per period (chronological when `PeriodMeta` is supplied and complete, else order of first appearance): entity count, total amount, ranked entities, top-N shares, HHI, category breakdown. |
| `LargestShareTrend` / `HHITrend` | First-vs-last direction characterization of `LargestEntityShare`/`HHI` across `History`. |
| `DependencyChanges` | One `DependencyChange` per entity active in either side of every chronologically adjacent period pair — see above. |
| `Scenarios` | `ScenarioLostLargestEntity` plus one `ScenarioTopNLoss` per `Policy.ScenarioTopN`, computed against the most recent period — see above. |
| `Flags` | Deterministic, `Thresholds`-driven signals (see below), ordered by `FlagCode`'s declaration order, then `Period`. |
| `Warnings` / `Errors` | Structured `Issue`s (own `IssueCode` system) for input-level problems (`NO_OBSERVATIONS`, `INVALID_OBSERVATION`, `NO_PERIOD_META`, `PERIOD_MISSING_FROM_META`, `NO_IMPACT_ASSUMPTION`). |

#### Flags

`Options.Thresholds` (zero value resolves to `DefaultThresholds()`)
configures five rule-based, deterministic triggers:
`HIGH_LARGEST_ENTITY_CONCENTRATION`, `HIGH_TOP_5_CONCENTRATION` (only
evaluated if `Policy.TopN` includes `5`), `HIGH_HHI` (default 2500, the
U.S. DOJ/FTC "highly concentrated" merger-guidelines figure, used only as a
familiar reference point), `INCREASING_CONCENTRATION`, and
`HIGH_SCENARIO_IMPACT`. Every `Flag` carries a stable `Code`, a `Severity`
(`info`/`warning`/`critical`), the specific `Value`/`Threshold` compared,
and a pre-filled `Message` (display only, never parsed) — no AI/LLM, no
opaque scoring, mirroring every analytics sibling package's identical
`Flag` design.

#### Exported surface, by file

- **`types.go`** — `Input`, `Options`, `Result`, `PeriodInfo`/`PeriodType`,
  `ConcentrationValue`/`Unavailable`/`AvailableValue`, `Basis`,
  `Observation`, `EntityImpactAssumption`,
  `Policy`/`DefaultPolicy()`, `RankedEntity`, `TopNShare`, `CategoryShare`,
  `PeriodConcentration`, `TrendDirection`/`Trend`, `DependencyChange`,
  `EntityImpact`, `ScenarioKind`, `Scenario`,
  `Thresholds`/`DefaultThresholds()`, `FlagCode`/`FlagSeverity`/`Flag`,
  `IssueCode`/`IssueSeverity`/`Issue`, `HasErrors`, `FormulaVersion`.
- **`concentration.go`** — `Calculate(Input, Options) Result`: wires every
  section together.
- **`periods.go`** — observation validation, period ordering, and
  `PeriodConcentration` (ranking, top-N shares, HHI, categories)
  computation.
- **`stats.go`** — `Trend` computation for `LargestShareTrend`/`HHITrend`.
- **`dependency.go`** — `DependencyChanges` computation.
- **`scenarios.go`** — `Scenarios` (lost-largest-entity, top-N-loss, and
  the optional earnings-impact conversion) computation.
- **`flags.go`** — every deterministic flag-trigger rule.

See [`analytics/concentration/concentration_test.go`](analytics/concentration/concentration_test.go),
[`analytics/concentration/determinism_test.go`](analytics/concentration/determinism_test.go),
and [`analytics/concentration/roundtrip_test.go`](analytics/concentration/roundtrip_test.go)
for every scenario (highly concentrated single-customer dependency,
diversified base, one customer, changing concentration with dependency
changes, zero/negative/malformed observations, top-N-loss scenarios with
default and per-entity margin assumptions, category breakdown, missing
`PeriodMeta`, and JSON/determinism) exercised against hand-built fixtures.

### `analytics/anomalies`

A deterministic **expense anomaly / margin-leakage analysis**: explainable
spikes, unusual variances, and structural pattern changes across a
normalized `financial.FinancialDataset`'s accounts and periods. No AI/ML,
and no rule here ever calls a finding fraud — every `Anomaly` uses neutral
language ("anomaly," "variance," "unusual pattern," "review recommended")
and is the output of one fixed, documented, `Thresholds`-driven rule, never
a model score or an accusation.

Unlike `analytics/concentration` (which is dataset-independent), this
package reads `Input.Dataset` directly — its task explicitly calls for
"normalized financial dataset" input, and every rule (expense-outpacing-
revenue, margin deterioration, owner/discretionary share) needs the
taxonomy's revenue/COGS/OPEX category structure, matching
`analytics/qoe`/`workingcapital`/`revenuequality`'s dataset-bound
convention instead.

Every function is pure — no I/O, no mutation of caller-owned input — and
`Calculate` returns byte-for-byte identical JSON across repeated runs
against identical input, regardless of Go's randomized map iteration order.

```go
res := anomalies.Calculate(anomalies.Input{
    Dataset: dataset, // financial.FinancialDataset
    PeriodMeta: map[financial.Period]anomalies.PeriodInfo{
        "2024": {Type: anomalies.PeriodTypeFiscalYear, FiscalYear: 2024},
        "2025": {Type: anomalies.PeriodTypeFiscalYear, FiscalYear: 2025},
    },
    AccountGroups: []anomalies.AccountGroup{
        {Name: "Marketing & Advertising", Codes: []financial.Code{financial.CodeOpexMarketing}},
    },
}, anomalies.Options{})

for _, a := range res.Anomalies {
    fmt.Println(a.Code, a.Account, a.Period, a.Explanation)
}
fmt.Println(res.Summary.Total, res.Summary.ReviewRecommended)
```

#### Detection rules

Eleven deterministic `RuleCode`s, each documented with its exact comparison
method on the constant itself:

| `RuleCode` | What it detects |
|---|---|
| `ABSOLUTE_AMOUNT_SPIKE` | One account's unsigned period-over-prior-period dollar change at or above `Thresholds.AbsoluteAmountSpike` |
| `PERCENTAGE_CHANGE_SPIKE` | The same comparison as a `\|change\| / \|prior\|` ratio at or above `Thresholds.PercentageChangeSpike` (requires a nonzero baseline) |
| `EXPENSE_OUTPACING_REVENUE` | A COGS/OPEX account's growth rate exceeding Total Revenue's growth rate, over the same adjacent period pair, by at least `Thresholds.ExpenseOutpacingRevenueGap` raw points |
| `MARGIN_DETERIORATION` | Gross margin or operating margin declining by at least `Thresholds.MarginDeteriorationPoints` raw points, period over prior period (checked independently; `Account` is empty and `MetricLabel` is set, since this is a synthetic figure) |
| `NEW_MATERIAL_EXPENSE_CATEGORY` | An expense account absent in the prior period reporting a materially large amount in the current one — the two-leg `NewCategoryMaterialAmount`/`NewCategoryMaterialPercentOfRevenue` test, same OR logic as `review.IsMaterial` |
| `ACCOUNT_DISAPPEARED_REAPPEARED` | Any account (income-statement or balance-sheet) transitioning from "has a `NormalizedItem`" to "no `NormalizedItem` at all," or reappearing after a gap. An explicitly reported `$0` is a real, present value here — never treated as absent |
| `REPEATED_UNUSUAL_VALUE` | The same account reporting the identical nonzero amount in at least `Thresholds.RepeatedValueMinOccurrences` distinct periods — a copy-paste/stale-value signal |
| `SIGN_FLIP` | An account's sign reversing between adjacent periods, with both magnitudes at least `Thresholds.SignFlipMinMagnitude` |
| `DUPLICATE_LIKE_AMOUNTS` | The identical amount recurring across two or more **distinct** COGS/OPEX accounts (a value repeating on a single account is `REPEATED_UNUSUAL_VALUE`'s concern, not this rule's) |
| `HIGH_OWNER_DISCRETIONARY_SHARE` | `financial.CodeOpexOwnerComp` plus `Input.DiscretionaryCodes`, as a fraction of the most recent period's Total Revenue, at or above `Thresholds.OwnerDiscretionaryShareOfRevenue` |
| `UNEXPECTED_NEGATIVE_AMOUNT` | A revenue or expense account reporting a negative amount at least `Thresholds.UnexpectedNegativeMinMagnitude` in magnitude — this repository's sign convention treats both as conventionally non-negative |

`REPEATED_UNUSUAL_VALUE`, `DUPLICATE_LIKE_AMOUNTS`, and
`UNEXPECTED_NEGATIVE_AMOUNT` need no `Input.PeriodMeta` (order-independent);
every other rule requires it and is skipped — with an advisory
`NO_PERIOD_META`/`PERIOD_MISSING_FROM_META` warning — when chronological
order is unavailable, mirroring every `analytics/` sibling's identical
all-or-nothing period-ordering rule.

#### Account groups and the discretionary-expense pool

`Input.AccountGroups` optionally labels `Anomaly.Group` for display/
filtering — it never changes which anomalies are detected, only how they
can be labeled afterward, and a code listed in more than one group is
labeled by whichever group appears first (deterministic, caller-controlled
precedence, never Go map order).

`HIGH_OWNER_DISCRETIONARY_SHARE` always includes
`financial.CodeOpexOwnerComp` (the one taxonomy code unambiguously
owner-related) and lets the caller extend the pool via
`Input.DiscretionaryCodes` — e.g. a caller whose classification pipeline
routes personal vehicle/travel expenses onto `CodeOpexVehicle`/
`CodeOpexTravel` supplies those here. This mirrors
`workingcapital.InclusionPolicy`'s "one unambiguous default plus explicit
caller extension" pattern, since the taxonomy has no broader "discretionary"
grouping the way `financial/adjustments.Type` does for confirmed
adjustments (not applicable here — this package runs on the raw dataset,
before any adjustment has been proposed or confirmed).

#### Float equality and the duplicate/repeated-value rules

`REPEATED_UNUSUAL_VALUE` and `DUPLICATE_LIKE_AMOUNTS` both need to decide
"are these two amounts the same value" — a structural equality check, not a
materiality judgment, so it is **not** one of the caller-adjustable
`Thresholds` fields. Amounts are considered equal when they round to the
same value at cents precision (`FloatEqualityTolerance = 0.01`), which
absorbs floating-point noise from upstream ingestion/normalization
arithmetic without ever conflating two amounts a caller would consider
genuinely distinct dollar figures.

#### What `Result` contains

| Field | What it is |
|---|---|
| `Anomalies` | Every anomaly found, ordered by `Period`, then `RuleCode` declaration order, then `Account` — see [Deterministic ordering guarantees](#deterministic-ordering-guarantees). |
| `Summary` | `Total`, `ByRule`/`BySeverity` rollups, and `ReviewRecommended` (`true` when `Total > 0`) — a deterministic rollup so a caller doesn't have to re-scan `Anomalies` itself. |
| `Thresholds` | The resolved `Options.Thresholds` (after `DefaultThresholds` substitution) this `Result` was computed under, so a persisted `Result` remains self-describing. |
| `Warnings` / `Errors` | Structured `Issue`s (own `IssueCode` system — see [Error taxonomy](#error-taxonomy)) for input-level problems (`NO_PERIODS`, `NO_PERIOD_META`, `PERIOD_MISSING_FROM_META`). |

Every `Anomaly` carries `Code`, `Severity`, `Account` (or `MetricLabel` for
a synthetic-metric finding), `Group`, `Period`/`BaselinePeriod`,
`Baseline`/`Observed` (each an `AnomalyValue` distinguishing "computed/
reported to be exactly `$0`" from "cannot be computed"), `Delta`,
`Threshold`, a pre-filled `Explanation` (display only, never parsed), and
`Provenance` (verbatim `financial.SourceRef`s from `Input.Dataset`, when
present) — every field the task's output contract requires.

#### Exported surface, by file

- **`types.go`** — `Input`, `Options`, `Result`, `PeriodInfo`/`PeriodType`,
  `AccountGroup`, `AnomalyValue`/`Unavailable`/`AvailableValue`, `RuleCode`,
  `AnomalySeverity`, `Anomaly`, `Provenance`, `Thresholds`/`DefaultThresholds()`,
  `IssueCode`/`IssueSeverity`/`Issue`, `HasErrors`, `Summary`, `RuleCount`,
  `SeverityCount`, `FloatEqualityTolerance`, `FormulaVersion`.
- **`anomalies.go`** — `Calculate(Input, Options) Result`: wires every rule
  together, plus the shared `codeIndex`/period-ordering/sorting/summary
  helpers every rule file uses.
- **`accounts.go`** — revenue/COGS/OPEX code sets (derived from
  `financial.CodesByCategory`) and the gross-margin/operating-margin
  helpers.
- **`spikes.go`** — `ABSOLUTE_AMOUNT_SPIKE`, `PERCENTAGE_CHANGE_SPIKE`,
  `SIGN_FLIP`, `UNEXPECTED_NEGATIVE_AMOUNT`.
- **`growth.go`** — `EXPENSE_OUTPACING_REVENUE`, `MARGIN_DETERIORATION`.
- **`categories.go`** — `NEW_MATERIAL_EXPENSE_CATEGORY`,
  `ACCOUNT_DISAPPEARED_REAPPEARED`.
- **`discretionary.go`** — `HIGH_OWNER_DISCRETIONARY_SHARE`.
- **`duplicates.go`** — `REPEATED_UNUSUAL_VALUE`, `DUPLICATE_LIKE_AMOUNTS`.

See [`analytics/anomalies/anomalies_test.go`](analytics/anomalies/anomalies_test.go),
[`analytics/anomalies/determinism_test.go`](analytics/anomalies/determinism_test.go),
and [`analytics/anomalies/roundtrip_test.go`](analytics/anomalies/roundtrip_test.go)
for every scenario the task requires (stable dataset, spikes, revenue-linked
expense growth, margin leak, sign flips, small immaterial changes, missing
periods including both the disappeared and reappeared-after-a-gap branches,
repeated values, duplicate-like cross-account amounts, owner/discretionary
share, unexpected negative amounts, account grouping, custom thresholds,
and JSON/determinism) exercised against hand-built fixtures.

### `analytics/variance`

A deterministic **budget/forecast/prior-period vs. actual variance
analysis**: line-level, category-level, and total reconciliation between
actual financial results and any caller-supplied baseline.

Like `analytics/concentration` (and unlike the dataset-bound
`analytics/qoe`/`workingcapital`/`cashflow`/`revenuequality`/`anomalies`),
this package is independent of `financial.FinancialDataset` — a budget or
forecast is frequently produced and stored entirely outside any financial
dataset this repository would see (a spreadsheet budget, an FP&A tool's
forecast export). It defines its own portable
`LineObservation{AccountCode, Period, Actual, Baseline, BaselineType}`
tuple. `AccountCode` is still a `financial.Code` (not a bare string)
because favorable/unfavorable classification is genuinely
taxonomy-dependent: a revenue account beating its baseline is favorable, an
expense account exceeding its baseline is unfavorable — the same dollar
sign means opposite things depending on the account. `financial.
CodesByCategory` supplies this default; a caller can pin a specific code's
direction via `Policy.DirectionOverrides`, taking precedence over the
category default.

Every function is pure — no I/O, no mutation of caller-owned input — and
`Calculate` returns byte-for-byte identical JSON across repeated runs
against identical input, regardless of Go's randomized map iteration order.

```go
res := variance.Calculate(variance.Input{
    Lines: []variance.LineObservation{
        {AccountCode: financial.CodeRevProduct, Period: "2025", Actual: 610_000,
            BaselineAvailable: true, Baseline: 560_000, BaselineType: variance.BaselineTypeBudget},
        {AccountCode: financial.CodeOpexMarketing, Period: "2025", Actual: 35_000,
            BaselineAvailable: true, Baseline: 30_000, BaselineType: variance.BaselineTypeBudget},
    },
    PeriodMeta: map[financial.Period]variance.PeriodInfo{
        "2025": {Type: variance.PeriodTypeFiscalYear, FiscalYear: 2025},
    },
    Policy: variance.Policy{MaterialAmountThreshold: 10_000},
})

for _, lv := range res.LineVariances {
    fmt.Println(lv.AccountCode, lv.Favorability, lv.AbsoluteVariance.Value)
}
fmt.Println(res.Bridge.TotalVariance)
```

#### Favorable/unfavorable direction

Every `financial.CategoryRevenue` code defaults to "an increase is
favorable"; every `financial.CategoryCogs`/`CategoryOpex` code defaults to
"an increase is unfavorable." `financial.CategoryOtherIncomeStatement`
mixes true income lines with expense lines, so this package resolves each
of its codes individually rather than treating the category uniformly:
`CodeInterestIncome`/`CodeOtherIncome` default favorable-on-increase;
`CodeDepreciation`/`CodeAmortization`/`CodeInterestExpense`/
`CodeIncomeTax`/`CodeOtherExpense` default unfavorable-on-increase.
Balance-sheet codes (and any code the taxonomy does not recognize) have no
default direction and are reported `FavorabilityUnknown` — recorded as an
advisory `UNKNOWN_ACCOUNT_CODE` warning — unless a caller supplies a
`Policy.DirectionOverrides` entry for that specific code. This package's
direction rule is a plain sign comparison on `Actual - Baseline`; it does
not special-case a negative amount on an otherwise-conventional revenue or
expense code (see `financial/metrics`' non-negative sign convention and
`anomalies.RuleUnexpectedNegativeAmount`) — a caller feeding out-of-
convention negative amounts still gets a correct arithmetic
`AbsoluteVariance`, but `Favorability` follows the same plain sign rule
regardless.

#### Materiality

`Policy.MaterialAmountThreshold`/`MaterialPercentOfBaseline` follow
`review.IsMaterial`'s exact OR-of-two-legs, off-by-default design: a
variance is material if its unsigned amount is at or above
`MaterialAmountThreshold`, or (`MaterialPercentOfBaseline > 0` and the
baseline is available and nonzero) the unsigned amount is at or above that
fraction of `|Baseline|`. Both default to 0, so materiality gating is off
by default — a caller must opt in explicitly, exactly like
`review.DefaultPolicy`.

#### What `Result` contains

| Field | What it is |
|---|---|
| `LineVariances` | One entry per input line: `AbsoluteVariance`, `PercentVariance` (unavailable on a zero baseline — see `VarianceFromZeroBase`), `Favorability`, `Materiality`. Ordered by `Period` (chronological when `PeriodMeta` covers it), then `AccountCode`. |
| `CategorySummaries` | Roll-up by `financial.CodeCategory`, sorted by category. |
| `CustomCategorySummaries` | Roll-up by the caller's own `LineObservation.Category` label (e.g. a department or cost center), independent of taxonomy category. |
| `TopFavorable` / `TopUnfavorable` | The `Policy.TopN` largest-dollar-impact lines in each direction. |
| `MaterialExceptions` | Every `MaterialityMaterial` line, each with `ContributionToTotalVariance` (share of `Bridge.TotalAbsoluteVariance`). |
| `PeriodTrends` / `TrendSummary` | Per-period aggregate variance and a first-vs-last direction characterization — unavailable without `Input.PeriodMeta`. |
| `Bridge` | The total actual-vs-baseline reconciliation: `TotalActual`, `TotalBaseline`, `TotalVariance`, `FavorableVariance` + `UnfavorableVariance` (which sum back to `TotalVariance` whenever every contributing line has a known `Favorability`). |
| `Warnings` / `Errors` | Structured `Issue`s (own `IssueCode` system — see [Error taxonomy](#error-taxonomy)) for input-level problems (`NO_LINES`, `MISSING_BASELINE`, `MISSING_BASELINE_TYPE`, `UNKNOWN_ACCOUNT_CODE`, `NO_PERIOD_META`, `PERIOD_MISSING_FROM_META`). |

A line with `BaselineAvailable == false` (no budget/forecast/prior-period
figure exists for that account/period) still appears in `LineVariances`
with its `Actual` intact, but every variance figure is left `Unavailable`
and it is excluded from `Bridge` — a missing baseline is never silently
treated as a $0 baseline.

#### Exported surface, by file

- **`types.go`** — `Input`, `Policy`/`DefaultPolicy()`, `Result`,
  `PeriodInfo`/`PeriodType`, `LineObservation`, `BaselineType`,
  `DirectionOverride`, `VarianceValue`/`Unavailable`/`AvailableValue`,
  `Favorability`, `Materiality`, `LineVariance`, `CategorySummary`,
  `TrendDirection`/`TrendFlatBandPercent`, `PeriodTrend`, `TrendSummary`,
  `MaterialException`, `Bridge`, `IssueCode`/`IssueSeverity`/`Issue`,
  `HasErrors`, `FormulaVersion`.
- **`variance.go`** — `Calculate(Input) Result`: per-line variance and
  favorability/materiality classification, plus `directionIndex` (taxonomy-
  category defaults, the mixed other-income-statement rule, and
  `DirectionOverrides` precedence) and `isMaterial`.
- **`periods.go`** — chronological-ordering helpers (mirroring
  `concentration.chronologicalPeriods`), `sortLineVariances`,
  `buildCategorySummaries`, `topFavorable`/`topUnfavorable`, `buildBridge`,
  `buildMaterialExceptions`, `buildPeriodTrends`, `buildTrendSummary`.

See [`analytics/variance/variance_test.go`](analytics/variance/variance_test.go),
[`analytics/variance/determinism_test.go`](analytics/variance/determinism_test.go),
and [`analytics/variance/roundtrip_test.go`](analytics/variance/roundtrip_test.go)
for every scenario the task requires (revenue, expense, COGS, zero
baseline, negative values, missing baseline, missing baseline type,
unknown account codes with and without a `DirectionOverrides` entry,
multiple periods with and without `PeriodMeta`, category rollup, custom
category rollup, top favorable/unfavorable, materiality by amount and by
percent, the bridge reconciliation identity, and JSON/determinism)
exercised against hand-built fixtures.

### `analytics/forecast`

A deterministic **financial projection and scenario engine**: given a
historical base period and a caller-supplied set of period-by-period
assumptions (revenue growth, gross margin/COGS, opex, D&A, capex, working
capital, tax, debt service), it compounds those assumptions forward across
a forecast horizon and reports the resulting projected P&L, EBITDA, SDE,
margins, working capital, cash flow, and debt-service coverage — for as
many independently-named scenarios (base/downside/upside/custom) as the
caller wants to run side by side.

This package does not predict assumptions. It applies them. Every growth
rate, margin target, and dollar figure is caller-supplied; nothing here
guesses at what a "downside" scenario should look like on its own. This
mirrors `valuation/dcf`'s identical "the caller forecasts, this package
only computes" boundary — `dcf.Input.ForecastPeriods` takes free cash flow
as an opaque caller-supplied series and only discounts it; `forecast` is
the natural upstream complement that can *produce* that series (via
`ScenarioResult.CashFlow[i].FreeCashFlow`) from assumptions. The two
packages remain fully independent — `forecast` never calls into
`valuation/dcf` — a caller wanting a DCF valuation off a `forecast` run
feeds the projected free cash flow into `dcf.Input` itself.

Unlike `analytics/variance`/`concentration` (which define their own
portable tuple, independent of `financial.FinancialDataset`), this package
reads its historical base directly from a normalized
`financial.FinancialDataset`, exactly like `analytics/qoe`/
`workingcapital`/`cashflow`/`revenuequality`/`anomalies` — a forecast
starts from real historical actuals, so there is no case for a
dataset-independent input shape here. Every forecast period's P&L line is
keyed by `financial.Code`, so `EBITDA`/`SDE`/margins reuse this
repository's one canonical set of code buckets (`financial.
CodesByCategory`) rather than a second, forecast-specific taxonomy.

Every function is pure — no I/O, no mutation of caller-owned input — and
`Calculate` returns byte-for-byte identical JSON across repeated runs
against identical input, regardless of Go's randomized map iteration order.

```go
res := forecast.Calculate(forecast.Input{
    Dataset: historicalDataset, // financial.FinancialDataset
    PeriodMeta: map[financial.Period]forecast.PeriodInfo{
        "2025": {Type: forecast.PeriodTypeFiscalYear, FiscalYear: 2025},
    },
    Horizon:              3,
    ForecastPeriodLabels: []string{"2026", "2027", "2028"},
    Scenarios: []forecast.Scenario{
        {
            Name: "Base", Type: forecast.ScenarioTypeBase,
            Assumptions: forecast.Assumptions{
                Revenue: []forecast.RevenuePeriodAssumption{
                    {Method: forecast.RevenueMethodGrowthRate, GrowthRate: 0.08},
                    {Method: forecast.RevenueMethodGrowthRate, GrowthRate: 0.08},
                    {Method: forecast.RevenueMethodGrowthRate, GrowthRate: 0.08},
                },
                COGS: []forecast.COGSPeriodAssumption{
                    {Method: forecast.COGSMethodGrossMarginPercent, GrossMarginPercent: 0.55},
                    {Method: forecast.COGSMethodGrossMarginPercent, GrossMarginPercent: 0.55},
                    {Method: forecast.COGSMethodGrossMarginPercent, GrossMarginPercent: 0.55},
                },
                Opex: []forecast.OpexPeriodAssumption{
                    {Method: forecast.OpexMethodGrowthRate, GrowthRate: 0.04},
                    {Method: forecast.OpexMethodGrowthRate, GrowthRate: 0.04},
                    {Method: forecast.OpexMethodGrowthRate, GrowthRate: 0.04},
                },
            },
        },
    },
})

for _, sr := range res.ScenarioResults {
    for _, p := range sr.ProjectedPeriods {
        fmt.Println(sr.Name, p.Period, p.TotalRevenue.Value, p.EBITDA.Value)
    }
}
```

#### The historical base

`Calculate` identifies the base period as the chronologically **last**
period in `Input.Dataset`, per `Input.PeriodMeta` — never by lexical or
dataset-item order (the same no-guessing rule `financial/metrics` and
every dataset-bound `analytics/` sibling already follows). Unlike those
siblings, a missing or incomplete `PeriodMeta` here is a **blocking**
`SeverityError` (`Result.Available == false`), not merely an advisory
warning: this package has no fallback base period to project forward from
at all without chronological order, whereas a sibling merely loses a
trend/seasonality output it can otherwise do without. `Result.Base` reports
the identified period's actual P&L and working capital, computed with the
exact same EBIT/EBITDA/SDE/margin formulas `financial/metrics` uses (see
`base.go`'s `buildBasePL`, deliberately duplicated from `financial/metrics`
rather than depending on its full `Snapshot` machinery — the same choice
`analytics/workingcapital` already made for its own formulas). Every
`Scenario` in one `Calculate` call projects forward from this identical
base — scenarios differ only in `Assumptions`, never in starting point.

#### Assumption shape: aggregate default plus per-code override

`RevenuePeriodAssumption`/`OpexPeriodAssumption` each carry one aggregate
growth rate (or fixed amount) applied to every code in that section without
its own entry in `CodeOverrides`, plus a list of specific
`financial.Code` overrides that take precedence for that code alone — the
same "aggregate default, explicit override wins" precedence
`variance.Policy.DirectionOverrides` and `workingcapital.InclusionPolicy`
already use elsewhere in this repository. A caller who only wants "grow
revenue 10%, hold opex flat" supplies one aggregate rate per period and
never touches `CodeOverrides`; a caller modeling, say, a specific
product line's revenue growing faster than the rest supplies one
`RevenueCodeAssumption` entry for that code, leaving every other revenue
code at the aggregate rate. `COGSPeriodAssumption` has no per-code override
(gross margin is inherently an aggregate ratio against total revenue);
its result is spread across COGS codes proportionally to the base period's
mix, exactly like `RevenueMethodFixedAmount`'s proportional split for
revenue.

A period with the zero-value assumption (an empty slice entry, or a
`Scenario` whose assumption slice is shorter than `Input.Horizon`) is not
treated as "no assumption at all" for revenue/COGS/opex — it defaults to
flat, 0%-growth carry-forward from the prior period (`RevenueMethodGrowthRate`/
`OpexMethodGrowthRate` with a 0 rate, or `COGSMethodGrossMarginPercent`
with a 0% target), since a zero-value revenue/opex assumption is
unambiguous. `Calculate` still reports `MISSING_REVENUE_ASSUMPTION`/
`MISSING_COGS_ASSUMPTION`/`MISSING_OPEX_ASSUMPTION` so the caller knows a
default was silently applied. This is the opposite convention from
`WorkingCapitalPeriodAssumption` and `TaxPeriodAssumption`/
`DebtServiceAssumption`, where a missing assumption leaves that period's
figure genuinely `Unavailable` rather than defaulting to something that
could be mistaken for a deliberate choice (0% of revenue, or $0 tax, are
not unambiguous defaults the way flat revenue growth is).

#### One-time items don't compound: `OpexMethodExcludeAmount`

Because every non-overridden opex code's amount grows from "whatever the
prior period actually reported for that code," a one-time expense added
via a `FixedAmount` override in period *N* would otherwise silently become
part of period *N+1*'s growth base too, permanently inflating every
subsequent period's trajectory by a cost that was only ever meant to hit
once. `OpexMethodExcludeAmount` solves this generally: it grows a code from
`(prior period's reported amount - ExcludeFromBase) x (1 + GrowthRate)`
rather than from the prior amount alone. `ApplyOneTimeCostShock` (see
[Scenario-transformation helpers](#scenario-transformation-helpers)
below) sets this automatically on the period immediately following the
one-time item, so a caller using that helper gets a truly one-time cost
with zero extra effort; a caller building `Assumptions` by hand can use
`OpexMethodExcludeAmount` directly for the same effect on any code.

#### Cash flow, working capital, and debt-service coverage

`WorkingCapitalPeriodAssumption` projects one period's operating NWC as a
percent of that period's projected revenue (`WorkingCapitalMethodPercentOfRevenue`,
the common "NWC scales with revenue" convention), a fixed dollar figure, or
held flat at the prior period's level. `CashFlowPeriod.OperatingCashFlow` is
`EBITDA - ChangeInNWC - IncomeTax` (mirroring `analytics/cashflow`'s
EBITDA-to-cash-flow bridge, adapted here to build forward from a
projection rather than from a caller-reported cash-flow statement, since a
forecast has no reported statement to read from by definition);
`FreeCashFlow` subtracts `Capex`; `FreeCashFlowToOwner` further subtracts
total debt service. `DebtServiceCoverage` divides
`Input.DebtServiceCoverageSource` (`operating_cash_flow`, the default, or
`ebitda`) by total debt service — left `Unavailable` (never "infinite")
when total debt service is exactly zero, mirroring `metrics.grossMargin`'s
identical zero-denominator guard. `DebtServiceAssumption.Interest` is
recomputed as `BeginningBalance x InterestRate` whenever a period supplies
both, rather than using a flat caller-supplied `Interest` figure directly —
this is the mechanism `ApplyDebtRateShock` adjusts (see below).
`TaxPeriodAssumption`'s `percent_of_pretax_income` method floors at $0: a
projected pretax loss produces $0 projected tax, never a fabricated
negative "tax benefit," since modeling a realizable tax benefit requires
assumptions (carryback availability, valuation allowances) this package
has no basis for.

#### Scenario-transformation helpers

`transform.go` exports small, composable pure functions that each return a
*modified copy* of an `Assumptions` value (never mutating the caller's
original) — a caller builds a downside/upside/custom `Scenario` by starting
from a base `Assumptions` and applying one or more of these in sequence,
rather than this package guessing what "downside" means on its own:

| Helper | What it shifts |
|---|---|
| `ApplyRevenueShock(a, deltaGrowthRate, fromPeriod, horizon)` | Every in-window period's aggregate revenue growth rate, skipping any period already using `RevenueMethodFixedAmount` |
| `ApplyMarginShock(a, deltaMarginPoints, fromPeriod, horizon)` | Every in-window period's gross-margin target, skipping `COGSMethodGrowthRate`/`COGSMethodFixedAmount` periods |
| `ApplyExpenseShock(a, deltaGrowthRate, codes, fromPeriod, horizon)` | The aggregate opex growth rate when `codes` is empty, or specific `financial.Code` overrides (shifting an existing growth-rate override, or appending a new one) when `codes` is non-empty |
| `ApplyCustomerLossShock(a, lossFraction, baseRevenue, fromPeriod, horizon)` | A permanent step-down in revenue at `fromPeriod` (computed by replaying the unshocked assumptions' own trajectory up to that point, then applying `1 - lossFraction`), with every later period continuing to compound from the newly-lowered base at its own already-specified rate |
| `ApplyOneTimeCostShock(a, amount, code, period, horizon)` | Adds a single-period expense via a `FixedAmount` override, auto-pinning `period+1`'s override for the same code to `OpexMethodExcludeAmount` so the spike does not compound forward (see above) |
| `ApplyDebtRateShock(a, deltaRate, fromPeriod, horizon)` | Every in-window period's `DebtServiceAssumption.InterestRate`, only where `InterestRate`/`BeginningBalance` are already both supplied |

Every helper treats a `fromPeriod` beyond `horizon` as a valid no-op (an
empty application window) rather than erroring, since a caller composing
several shocks across different windows may legitimately end up with one
that's empty for a given horizon.

#### What `Result` contains

| Field | What it is |
|---|---|
| `Base` | The historical base period's actual P&L (`PeriodPL`) and working capital — identical across every `ScenarioResult` in the same `Calculate` call. |
| `ScenarioResults` | One `ScenarioResult` per valid `Input.Scenarios` entry (skipping empty/duplicate names — see `EMPTY_SCENARIO_NAME`/`DUPLICATE_SCENARIO_NAME`), each with `ProjectedPeriods`, `WorkingCapital`, `CashFlow`, a full `Trace`, and scenario-scoped `Warnings`. |
| `PeriodPL` (on `Base.PL` and every `ProjectedPeriods` entry) | `RevenueLines`/`COGSLines`/`OpexLines` (per-`financial.Code`, sorted), `TotalRevenue`, `GrossProfit`/`GrossMargin`, `TotalOpex`, `EBIT`, `EBITDA`/`EBITDAMargin`, `SDE`/`SDEMargin`, `PretaxIncome`, `IncomeTax`, `NetIncome` — every derived figure using the exact same formula and availability rule as `financial/metrics`' identically-named metric. |
| `WorkingCapital` | Per-period `NWC` and `ChangeInNWC` (positive = cash use, mirroring `cashflow.Bridge.ChangeInNWC`'s sign convention). |
| `CashFlow` | Per-period `OperatingCashFlow`, `Capex`, `FreeCashFlow`, `DebtService`, `FreeCashFlowToOwner`, `DebtServiceCoverage`. |
| `ForecastPeriods` | The resolved display label for each forecast period (`Input.ForecastPeriodLabels`, falling back to a generated `"Period N"`). |
| `Warnings` / `Errors` | Structured `Issue`s (own `IssueCode` system — see [Error taxonomy](#error-taxonomy)), each optionally scoped to one `Scenario`/`Period`. |

#### Exported surface, by file

- **`types.go`** — `Input`, `PeriodInfo`/`PeriodType`, `ForecastValue`/
  `Unavailable`/`AvailableValue`, `RevenueMethod`/`RevenuePeriodAssumption`/
  `RevenueCodeAssumption`, `COGSMethod`/`COGSPeriodAssumption`,
  `OpexMethod`/`OpexPeriodAssumption`/`OpexCodeAssumption`,
  `DepreciationAmortizationAssumption`, `CapexAssumption`,
  `WorkingCapitalMethod`/`WorkingCapitalPeriodAssumption`,
  `TaxMethod`/`TaxPeriodAssumption`, `DebtServiceAssumption`,
  `Assumptions`, `ScenarioType`, `Scenario`,
  `DebtServiceCoverageSource`, `IssueCode`/`IssueSeverity`/`Issue`,
  `HasErrors`, `LineItem`, `PeriodPL`, `WorkingCapitalPeriod`,
  `CashFlowPeriod`, `ScenarioResult`, `TraceStep`, `BaseFinancials`,
  `Result`, `FormulaVersion`.
- **`base.go`** — historical-base derivation: `buildBasePL` (the
  `financial/metrics`-formula-mirroring P&L build), `buildBaseWorkingCapital`,
  `resolveBasePeriod` (chronological-order validation and last-period
  selection), and the package's own `codeIndex`.
- **`project.go`** — `projectScenario` (the per-period projection loop) and
  every line-level projection function: `projectRevenue`, `projectCOGS`,
  `projectOpex`, `projectTax`, `projectWorkingCapital`,
  `resolveDebtServiceInterest`, `buildCashFlowPeriod`.
- **`forecast.go`** — `Calculate(Input) Result`: top-level validation,
  base-period resolution, and per-`Scenario` orchestration.
- **`transform.go`** — every `Apply*` scenario-transformation helper (see
  above) and `cloneAssumptions`, the deep-enough copy every helper builds
  on so none of them ever mutates a caller's `Assumptions`.

See [`analytics/forecast/forecast_test.go`](analytics/forecast/forecast_test.go),
[`analytics/forecast/transform_test.go`](analytics/forecast/transform_test.go),
[`analytics/forecast/determinism_test.go`](analytics/forecast/determinism_test.go),
and [`analytics/forecast/roundtrip_test.go`](analytics/forecast/roundtrip_test.go)
for every scenario the task requires (base/downside/upside scenarios
projected from an identical base, negative growth, margin compression,
missing revenue/COGS/opex assumptions falling back to a flat default,
multi-year compounding, no hidden mutation of `Input.Dataset`/`Scenarios`/
`Assumptions`, every scenario-transformation helper individually and
end-to-end through `Calculate`, the working-capital/cash-flow/debt-service-
coverage bridge, and JSON/determinism including a map-order stress test)
exercised against both hand-built fixtures and the repository's realistic
`normalized_hvac_multi_year.json` fixture.

### `analytics/debt`

A deterministic **debt service coverage / leverage / debt capacity**
analysis: given a business's normalized/maintainable EBITDA (or an
optional cash-flow figure), its existing and proposed loans, and an
optional lender policy, this package computes annual debt service (via a
built-in amortization solver), DSCR, fixed-charge coverage, debt/EBITDA
and net-debt/EBITDA leverage, interest coverage, the maximum debt the
business could support under a caller-supplied minimum DSCR and/or
leverage cap, the more restrictive combined capacity limit and headroom
against it, and coverage under caller-defined downside stress scenarios.

Unlike `analytics/cashflow`/`analytics/ratios` (which recompute EBITDA and
other figures directly from a `financial.FinancialDataset`), this package
is deliberately independent of `financial.FinancialDataset` and the
`financial.Code` taxonomy — the same design `analytics/concentration`
already established. Debt capacity analysis is frequently performed
against a caller's own already-normalized EBITDA figure (already run
through `financial/metrics` and `financial/adjustments` upstream), a
lender-supplied term sheet, or a standalone "what could this business
support" what-if calculation with no dataset in the loop at all. A caller
with a `financial.FinancialDataset` computes EBITDA itself and passes the
resulting figure into `Input.EBITDA`; this package never requires or
recomputes it.

**This package never claims lender approval.** Every doc comment and the
package doc comment itself are explicit that this is a mathematical
capacity/coverage analysis under the assumptions supplied, never a
statement that a lender would approve any amount — `MaximumCapacity`'s doc
comment repeats this explicitly.

Availability follows this repository's standard convention: `Value{
Available bool; Amount float64 }` distinguishes "computed/reported to be
exactly 0" from "unknown because a required input was absent," duplicated
locally (mirroring `cashflow.CashFlowValue`) rather than importing
`financial/metrics` into a package that otherwise has no dependency on it.

**Amortization.** `LoanTerms{Principal, AnnualInterestRate,
AmortizationYears, Frequency, InterestOnlyYears}` describes one loan (an
existing balance or a proposed structure) without requiring the caller to
pre-compute a payment schedule. `Amortize` derives an `AmortizationSchedule`
using the standard level-payment formula (`payment = principal x rate / (1
- (1 + rate)^-n)`, or straight-line when the rate is exactly zero), and
reports both `SteadyStateAnnualDebtService` (a typical year once
amortization is underway) and `FirstYearAnnualDebtService` (which may be
lower, reflecting an `InterestOnlyYears` period, a straddled
transition-year blend, or identical to the steady state when there is no
interest-only period). An interest-only period defers amortization without
shortening it: a 10-year loan with a 2-year I/O period still amortizes the
full principal over 10 years once amortization begins, per standard
commercial lending practice.

**Coverage.** `CoverageResult.DSCR` divides a numerator (`Input.CashFlow`
if supplied, otherwise `Input.EBITDA` — see `CoverageNumeratorSource`) by
total annual debt service, left unavailable (not zero or infinite) when
debt service is exactly zero. `FixedChargeCoverage` follows the
conventional formula `(numerator - CashTaxes - [UnfinancedCapex ?
CapitalExpenditures : 0] + LeasePayments) / (AnnualDebtService +
CurrentPortionLongTermDebt + LeasePayments)`, computed only when at least
one `FixedChargeInputs` field is supplied. `DebtToEBITDA`/`NetDebtToEBITDA`
are available only when EBITDA is strictly positive — a leverage multiple
against a zero or negative EBITDA is not a meaningful ratio, mirroring
`IssueNegativeEBITDA`'s rationale for why DSCR itself is still computed
(and can legitimately read negative) but leverage is not.

**Capacity.** `MaxDebtUnderDSCR` solves for the principal that, amortized
at the same rate/term/frequency/interest-only structure as the first
supplied `ProposedLoans`/`ExistingDebt` entry ("pricing terms"), produces
annual debt service exactly equal to the coverage numerator divided by
`Policy.MinimumDSCR` — exploiting that `FirstYearAnnualDebtService` is
linear in principal, so the solve is a direct ratio rather than an
iterative search. `MaxDebtUnderLeverage` is `EBITDA x
Policy.MaximumDebtToEBITDA`, or, when `Policy.MaximumNetDebtToEBITDA` is
also set, whichever gross-debt-equivalent figure (after adding back
`CashAndEquivalents` to the net cap) is smaller. `CombinedMaximumDebt`
picks the more restrictive of the two available figures and
`LimitingConstraint` reports which one; `Headroom` is that figure minus
the resolved current total debt balance. Every one of these calculations
is skipped (left unavailable, with `IssueNoLenderPolicy` recorded) when
`Input.Policy` has no non-zero threshold — this package never invents a
default cap.

**Scenarios.** `Input.DownsideScenarios` applies an
`EBITDAHaircutPercent`/`CashFlowHaircutPercent` to the base-case earnings
figures (a negative percentage models an upside case) while holding debt
service, leverage, and cash fixed, then recomputes `CoverageResult` under
the stressed figures. `ScenarioResult.BreachesMinimumDSCR` is `true` only
when `Policy.MinimumDSCR` is set and the scenario's DSCR falls below it —
`false` (not merely unavailable) when no policy threshold exists to
breach.

Files:

- **`types.go`** — `Value`, `LoanTerms`/`AmortizationSchedule`,
  `LenderPolicy`, `FixedChargeInputs`, `DownsideScenario`, `Input`,
  `CoverageResult`, `MaximumCapacity`, `ScenarioResult`,
  `IssueCode`/`IssueSeverity`/`Issue`, `HasErrors`, `FlagCode`/`Flag`,
  `Result`, `FormulaVersion`.
- **`amortization.go`** — `validateLoanTerms` (structural validation) and
  `Amortize` (the level-payment formula, interest-only handling, and
  first-year vs. steady-state debt service).
- **`coverage.go`** — `computeCoverage`: DSCR, fixed-charge coverage,
  interest coverage, and leverage ratio computation shared by the base
  case, the existing-only case, and every scenario.
- **`capacity.go`** — `computeCapacity`: the maximum-debt-under-DSCR solve,
  the maximum-debt-under-leverage-cap calculation, and the
  combined-capacity/headroom rule.
- **`scenarios.go`** — `computeScenarios`: applies each
  `DownsideScenario`'s haircuts and recomputes coverage.
- **`debt.go`** — `Calculate(Input) Result`: top-level validation,
  schedule construction, and orchestration of coverage/capacity/scenarios.
- **`flags.go`** — every deterministic flag-trigger rule.

See [`analytics/debt/debt_test.go`](analytics/debt/debt_test.go),
[`analytics/debt/amortization_test.go`](analytics/debt/amortization_test.go),
[`analytics/debt/determinism_test.go`](analytics/debt/determinism_test.go),
and [`analytics/debt/roundtrip_test.go`](analytics/debt/roundtrip_test.go)
for every scenario the task requires (a standard amortizing loan validated
against a reference payment figure, zero interest, zero principal, an
interest-only period both spanning and straddling the first year, zero
debt, negative EBITDA, cash-flow-preferred-over-EBITDA numerator
selection, fixed-charge coverage, the maximum-debt-under-DSCR solve
verified by re-amortizing the solved principal, the maximum-debt-under-
leverage-cap calculation for gross/net/combined caps, the combined-
capacity limiting-constraint and headroom rule, downside scenarios with
debt service held fixed and a policy-breach flag, invalid loan terms
excluded but reported by index, and JSON/determinism) exercised against
hand-built fixtures.

### `analytics/covenants`

A deterministic **financial covenant monitor**: given a batch of
caller-defined covenant tests — each an ID, a metric label, an operator
(`>=`, `<=`, `>`, `<`, `=`), a threshold, an already-calculated actual
figure, a test period, and optional cure/grace metadata and warning-buffer
settings — this package evaluates every test independently and reports
pass/fail/unavailable, direction-aware headroom, warning-buffer (near-
breach) status, a plain-language explanation, and an aggregate summary of
breaches, near breaches, and unavailable tests.

Like `analytics/debt`/`analytics/variance`, this package is deliberately
independent of `financial.FinancialDataset` — a covenant's actual figure
is typically a DSCR or leverage multiple already computed upstream (via
`analytics/debt`), a liquidity ratio from `analytics/ratios`, or a balance-
sheet figure pulled from a compliance certificate. **This package never
computes DSCR, leverage, EBITDA, net worth, or any other metric itself**;
a caller supplies each covenant's already-calculated `Actual` value, and
this package's only job is comparing it against `Threshold` via `Operator`
and classifying the result. It also **never encodes loan-document or
legal interpretation**: `CureGrace` metadata is recorded verbatim and
never changes a test's `Status`, `Headroom`, or `WarningBufferStatus` — a
cure period's legal effect on an actual default is a determination for the
credit agreement and counsel, not this package.

Availability follows this repository's standard convention: `Value{
Available bool; Amount float64 }`, duplicated locally per this
repository's established convention rather than importing another
analytics package.

**Operators and headroom.** `Operator` (`>=`/`<=`/`>`/`<`/`=`) is a new
taxonomy this package introduces — no generic comparator type existed
elsewhere in this repository (`analytics/variance` solved its structurally
similar favorable/unfavorable problem with a domain-specific bool instead).
`Headroom` is the signed distance between `Actual` and `Threshold` in the
direction of safety: for a minimum-style covenant (`>=`/`>`) it is
`Actual - Threshold`; for a maximum-style covenant (`<=`/`<`) it is
`Threshold - Actual`; positive means room before breaching, negative means
already breached by that amount. `OperatorEQ` has no single safe
direction, so `Headroom` is left unavailable for an exact-equality
covenant rather than reporting a signed distance that doesn't answer "how
much room before breach." An empty or unrecognized `Operator` produces
`IssueInvalidOperator` and `StatusUnavailable` — this package never
guesses a direction for an operator it doesn't recognize.

**Metrics.** `Metric` labels what kind of figure `Actual` represents
(`DSCR`, `DEBT_TO_EBITDA`, `NET_DEBT_TO_EBITDA`, `CURRENT_RATIO`,
`QUICK_RATIO`, `MINIMUM_EBITDA`, `MINIMUM_NET_WORTH`, `MAXIMUM_CAPEX`, or
`CUSTOM` with a caller-supplied `CustomMetricLabel`) for display and
`Explanation` purposes only — this package applies the identical
`Operator`/`Threshold`/`Headroom` logic regardless of which `Metric` a
test names, so a fully custom, lender-specific covenant metric is
evaluated exactly as any named one.

**Warning buffer.** `WarningBufferPercent` (a fraction of `|Threshold|`)
and `WarningBufferAmount` (an absolute headroom floor) are independent,
off-by-default, OR-of-two-legs settings per covenant test — the same
pattern `review.IsMaterial`/`analytics/variance.Policy` already use for
materiality — since a meaningful DSCR buffer and a meaningful net-worth
buffer are inherently different scales and belong on the rule, not a
single package-wide policy. `WarningBufferStatus` only classifies a
*passing* test (`WarningBufferWithinBuffer` means it currently passes but
would fail on a small adverse move — a near breach); a failed or
unavailable test reports `WarningBufferNotApplicable` rather than being
double-counted as both a breach and a near breach in `Summary`.

**Availability, not silence.** A missing `CovenantID`, an unrecognized
`Operator`, or an unavailable `Actual` each produce a `StatusUnavailable`
`TestResult` (still included in `Result.Tests`, so every input row is
reflected in output) plus a structured `Issue` — never a dropped test,
a silent pass, or a fabricated zero.

**Summary.** `Summary` tallies `Breaches`/`NearBreaches`/`Unavailable`
plus their `CovenantID` lists for direct display, and `ByPeriod` breaks
the same three counts down per distinct `financial.Period` present in the
input batch, sorted lexically (this package draws no chronological
inference from `financial.Period` — the same no-guessing rule every
analytics sibling package's `PeriodInfo` already documents, though this
package needs no `PeriodInfo` of its own since it computes no trend across
periods, only independent per-period aggregates).

Files:

- **`types.go`** — `Value`, `Operator`, `Metric`, `CureGrace`,
  `CovenantTest`, `Input`, `IssueCode`/`IssueSeverity`/`Issue`,
  `HasErrors`, `Status`, `WarningBufferStatus`, `TestResult`,
  `PeriodSummary`, `Summary`, `Result`, `FormulaVersion`.
- **`evaluate.go`** — `evaluateTest`: per-test validation, operator
  evaluation, headroom, warning-buffer classification, and the
  deterministic `Explanation` sentence builder.
- **`covenants.go`** — `Calculate(Input) Result`: top-level orchestration,
  duplicate-covenant-ID detection, and `Summary`/`PeriodSummary`
  aggregation.

See [`analytics/covenants/covenants_test.go`](analytics/covenants/covenants_test.go),
[`analytics/covenants/determinism_test.go`](analytics/covenants/determinism_test.go),
and [`analytics/covenants/roundtrip_test.go`](analytics/covenants/roundtrip_test.go)
for every scenario the task requires (a comfortably passing test, a
minimum-style and a maximum-style breach with the headroom direction
verified for each, a near breach via both the percent-of-threshold and
absolute-amount warning-buffer legs plus confirmation a failed test is
never also classified within-buffer, an unavailable-actual test, an
unrecognized-operator test, a custom caller-supplied metric, cure/grace
metadata passing through unchanged, multiple covenants across multiple
periods with a per-period summary breakdown, the equality operator's
headroom-unavailable rule, no-mutation of caller-owned input, and
JSON/determinism) exercised against hand-built fixtures.

### `analytics/benchmarks`

A deterministic **peer/industry benchmark comparison engine**: given a
batch of caller-defined metric requests — each an ID, an already-
calculated company value, an optional period, an optional favorable-
direction hint, and a `BenchmarkSet` in whichever form the caller has
available — this package compares every request independently and
reports the benchmark's median and known range, an estimated percentile
and quartile band for the company's value, the signed difference and
relative difference against the median, a favorable/unfavorable
classification (only where the caller supplies `Direction`), and every
piece of provenance metadata the caller attached to the benchmark, plus
an aggregate summary.

**This package does not source, fetch, or redistribute benchmark data.**
Every `BenchmarkSet` is supplied entirely by the caller, who is
responsible for having the appropriate license to use whatever survey,
association report, or peer dataset it derives from — this package only
performs the comparison arithmetic and echoes back whatever provenance
the caller attaches.

Like `analytics/debt`/`analytics/variance`/`analytics/covenants`, this
package is deliberately independent of `financial.FinancialDataset` — a
company metric to benchmark is typically already computed upstream (via
`financial/metrics`, `analytics/ratios`, `analytics/debt`, or any other
caller-side calculation), and a benchmark dataset routinely comes from a
source with no relationship to this repository's canonical taxonomy at
all. `MetricRequest{CompanyValue, BenchmarkSet}` is the only required
input shape.

**Benchmark forms.** `BenchmarkForm` supports four shapes a caller might
have on hand, from least to most granular: `MEDIAN` (only a single
summary figure — no percentile or band is calculable, only difference/
relative difference), `PERCENTILE_BANDS` (an arbitrary set of
`(percentile, value)` points, linearly interpolated), `QUARTILES` (the
common Q1/Median/Q3 special case, broken out as its own struct since most
published industry reports present data this way), and
`PEER_OBSERVATIONS` (explicit individual peer values — the richest form,
since Calculate derives every other statistic, including an empirical
rank-based percentile estimate, directly from the raw observations
rather than trusting a pre-aggregated summary). Only the field(s)
belonging to the declared `Form` are read; a caller populating an
unrelated field has it silently ignored, never merged in as additional
evidence.

**Percentile/band math.** For `PERCENTILE_BANDS`/`QUARTILES`, Calculate
sorts the supplied points by percentile and applies piecewise-linear
interpolation both ways: `interpolate` (percentile -> value, used for the
median and `BenchmarkRange`) and its inverse `percentileOfValue` (value ->
percentile, used for the company's own `Percentile` and `Band`). A value
below the lowest known point or above the highest is clamped to that
point's own percentile (extrapolation is never attempted) but is still
classified `BAND_BELOW_MIN`/`BAND_ABOVE_MAX` rather than merely the
nearest quartile, so a caller can distinguish "at the edge of the known
range" from "beyond it." For `PEER_OBSERVATIONS`, Calculate sorts the raw
values and assigns each the percentile `100*i/(n-1)` (the same rank
convention as Excel's `PERCENTILE.INC`/the "R-7" method), then applies
the identical interpolation logic — so every `BenchmarkForm` shares one
comparison code path.

**Monotonicity precondition.** Every percentile-band/quartile table is
required to have `Value` non-decreasing as `Percentile` increases (true
by construction for peer observations, since Calculate itself sorts by
value to build them — but not guaranteed for a caller-supplied
`PercentileBands`/`Quartiles` table, e.g. one entered in descending order
for a "lower is better" metric like DSO). A caller expresses "lower is
better" via `Direction`, never via point order. A table that violates
this produces `IssueNonMonotonicBenchmarkPoints` and leaves
`Percentile`/`Band`/`BenchmarkRange` unavailable for that metric — this
package never reports an inverted range or a band computed from a
self-contradictory table. `BenchmarkMedian` and the difference/relative-
difference/favorable fields are unaffected when they rest on an
explicitly supplied `Median`/`Quartiles.Median`, since those don't depend
on the points table's ordering.

**Favorable/unfavorable, caller-defined only.** `Direction`
(`HIGHER_IS_BETTER`/`LOWER_IS_BETTER`/`NEUTRAL`/unspecified) is entirely
at the caller's discretion — this package has no built-in opinion on
whether any given metric is better higher or lower, and `Favorable` is
always `NOT_APPLICABLE` when `Direction` is left unspecified or set to
`NEUTRAL`.

**Availability, not silence.** A missing `MetricID` produces an entirely
unavailable `Comparison`. An unavailable `CompanyValue`, an invalid or
unrecognized `Form`, missing benchmark data, an insufficient (fewer than
two distinct) percentile-point count, or a non-monotonic points table
each leave only the affected fields unavailable — plus a structured
`Issue` — rather than discarding the whole comparison; every input row is
always reflected in `Result.Comparisons`.

**Provenance.** `BenchmarkSource` (name, effective date, population/
segment, sample size, and an optional license/source ID) plus
`IndustryLabel`/`SizeLabel`/`GeographyLabel` are recorded verbatim on
every `Comparison` — this package never fetches, validates, or infers any
of these fields.

**Summary.** `Summary` tallies `FavorableCount`/`UnfavorableCount`/
`UnavailableCount` plus `UnfavorableMetricIDs` for direct display.

Files:

- **`types.go`** — `Value`, `Direction`, `BenchmarkForm`,
  `PercentilePoint`, `Quartiles`, `PeerObservation`, `BenchmarkSource`,
  `BenchmarkSet`, `MetricRequest`, `Input`,
  `IssueCode`/`IssueSeverity`/`Issue`, `HasErrors`, `Band`, `Favorable`,
  `Range`, `Comparison`, `Summary`, `Result`, `FormulaVersion`.
- **`percentile.go`** — `deriveBenchmarkStats`: per-`BenchmarkForm`
  normalization into a common `benchmarkStats` shape, the
  `dedupSortPoints`/`isNonDecreasing` point-table helpers, the
  peer-observation rank-based `peerPercentilePoints` deriver, and the
  bidirectional `interpolate`/`percentileOfValue` piecewise-linear
  functions.
- **`evaluate.go`** — `evaluateMetric`: per-metric validation, benchmark
  median/range/percentile/band derivation, difference/relative-difference,
  and favorable/unfavorable classification.
- **`benchmarks.go`** — `Calculate(Input) Result`: top-level
  orchestration, duplicate-metric-ID detection, and `Summary` aggregation.

See [`analytics/benchmarks/benchmarks_test.go`](analytics/benchmarks/benchmarks_test.go),
[`analytics/benchmarks/determinism_test.go`](analytics/benchmarks/determinism_test.go),
and [`analytics/benchmarks/roundtrip_test.go`](analytics/benchmarks/roundtrip_test.go)
for every scenario the task requires (all four benchmark forms, linear
interpolation and its inverse, an out-of-range company value clamped but
distinguished as below-min/above-max, sparse quartile data with only one
or two of the three points known, unsorted peer observations, a single
peer observation, an explicit median overriding a derived one, a
non-monotonic points table for both `PERCENTILE_BANDS` and `QUARTILES`,
an unavailable company value, a missing metric ID, an invalid/unrecognized
form, missing benchmark data, a zero benchmark median's division-by-zero
guard, both favorable directions plus neutral/unspecified, duplicate
metric IDs, summary aggregation, no-mutation of caller-owned input,
provenance round-tripping, and JSON/determinism) exercised against
hand-built fixtures.

### `valuation`

Implements the individual valuation methods themselves: SDE multiple,
EBITDA multiple, capitalization of earnings, discounted cash flow, and
adjusted net asset value — each as its own independently testable,
strongly-typed subpackage (`valuation/sde`, `valuation/ebitda`,
`valuation/capitalization`, `valuation/dcf`, `valuation/netassets`) sharing
one common result envelope defined in `valuation` itself.

None of these packages compute EBITDA/SDE, select a multiple or discount
rate, or forecast cash flows — they consume already-determined figures
(typically `financial/metrics.Snapshot` values, a `financial/adjustments`
normalized bridge, or a `financial/earnings.Result`) and a caller-selected
assumption (a multiple, a capitalization rate, a discount rate, explicit
forecast cash flows, explicit fair-value asset/liability figures) and
combine them into a single defensible value with a full calculation trace.
Every `Calculate` function is pure: no I/O, no mutation of its inputs, and
— consistent with every other package in this repository — no Go `error`
return; invalidity is always communicated through `Result.Available` plus
structured `Result.Errors`/`Result.Warnings`, mirroring
`financial/metrics.MetricValue` and `financial/earnings.Result`'s
established convention.

#### Method codes and versions

| Method | `Code` | Package |
|---|---|---|
| SDE Multiple | `SDE_MULTIPLE` | `valuation/sde` |
| EBITDA Multiple | `EBITDA_MULTIPLE` | `valuation/ebitda` |
| Capitalization of Earnings | `CAPITALIZATION_OF_EARNINGS` | `valuation/capitalization` |
| Discounted Cash Flow | `DCF` | `valuation/dcf` |
| Adjusted Net Asset Value | `ADJUSTED_NET_ASSET_VALUE` | `valuation/netassets` |

Every method also exports a `Version` constant (currently `"1.0.0"` for
all five), echoed on every `Result` as `MethodVersion`. **Method versioning
is mandatory** because the main application is expected to persist
historical valuations: a `Result` computed today must remain
self-describing about exactly which calculation logic produced it, even
after this package's formulas or validation rules evolve later. Bump a
method's `Version` whenever its formula, validation rules, or output shape
change in a way that could make an old `Result` not reproduce identically
under the new code.

#### Common result envelope

Every method's `Result` (see each subpackage's `Input`/`Result` types)
independently defines its own strongly-typed `Input` — deliberately not a
single generic input shared across methods, since an SDE multiple's inputs
share almost nothing in shape with a DCF's forecast-period list, and
forcing them into one generic type would mean either a bag of
mostly-unused fields or a loss of compile-time type safety for no benefit.
What every `Result` *does* share, via the `valuation` package's types, is:

- **`Method` / `MethodVersion`** — the method's stable `valuation.Code` and
  the `Version` it ran under.
- **`ValueType`** — `enterprise_value`, `equity_value`, or `asset_value`
  (`valuation.ValueTypeEnterprise`/`Equity`/`Asset`) — see
  [Enterprise value vs. equity value](#enterprise-value-vs-equity-value)
  below. Never mixed silently: a method's headline numeric field name
  always matches its `ValueType` (`EnterpriseValue`, `EquityValue`, or
  `AdjustedNetAssetValue`).
- **`Input`** — the exact input `Calculate` was given, echoed back so a
  `Result` is self-contained and reproducible without the caller
  separately retaining its own copy.
- **`Available` + the headline value field** — `false` only when the
  method could not produce a defensible number at all (see
  [Validation](#validation) below); the headline field is always `0` when
  `Available` is `false`.
- **`Steps []valuation.Step`** — the full calculation trace, in the order
  computed: every labeled intermediate and final figure a reviewer would
  want to see, never collapsed to just the headline number.
- **`Warnings []valuation.Issue` / `Errors []valuation.Issue`** — see
  [Validation](#validation).

```go
type Step struct {
    Label  string  // e.g. "Enterprise Value = Maintainable EBITDA x Multiple"
    Value  float64
    Detail string  // e.g. "500000 x 4"
}
```

#### Enterprise value vs. equity value

Methods never mix value types silently. Each method's doc comment states
exactly which `ValueType` it produces and why:

| Method | Native `ValueType` | Reasoning |
|---|---|---|
| SDE Multiple | `equity_value` | SDE already includes the return to a single working owner, which small-business valuation practice treats as pricing the owner's equity directly, not a capital-structure-neutral enterprise. |
| EBITDA Multiple | `enterprise_value` | EBITDA is capital-structure-neutral; an EBITDA multiple conventionally prices the whole cash-free/debt-free operating business. |
| Capitalization of Earnings | `equity_value` | Conventionally applied directly to an earnings stream already understood to belong to equity holders (e.g. SDE or owner net income). |
| DCF | `enterprise_value` | Discounts unlevered free cash flow to the firm at a WACC-style rate — the standard FCFF-to-Enterprise-Value convention. Levered FCFE-to-equity discounting is not supported (see `valuation/dcf`'s package doc comment on why mixing the two conventions in one `Input` is deliberately avoided). |
| Adjusted Net Asset Value | `asset_value` | Distinct from both: an itemized adjusted-assets-minus-adjusted-liabilities calculation, not necessarily interchangeable with a going-concern equity value from an earnings-based method. |

**The bridge.** Where an enterprise-value method (`valuation/ebitda`,
`valuation/dcf`) needs conversion to an equity value — or, unusually, where
a caller wants one applied on top of an already-equity-value method
(`valuation/sde` also exposes this, in case a caller's convention wants
it) — every method uses the exact same explicit formula, returned as its
own inspectable `valuation.Bridge`, never folded invisibly into the
headline number:

```
Equity Value = Enterprise Value + Excess Cash - Total Debt
```

```go
type Bridge struct {
    Available       bool
    EnterpriseValue float64
    ExcessCash      float64
    TotalDebt       float64
    DebtComponents  []Component // e.g. Short-Term Debt, Long-Term Debt, Other Debt
    EquityValue     float64
}
```

The bridge is opt-in per call (`Input.EquityBridge.Requested`) — a method
whose native result is already an enterprise or equity value remains fully
valid on its own; a caller only pays for the bridge's extra inputs
(`ExcessCash`, `ShortTermDebt`, `LongTermDebt`, `OtherDebt`) when it
actually wants one. `Result.Bridge.Available` is `false` whenever no
bridge was requested, distinct from a bridge that was requested and
computed with every field at its legitimate zero value (e.g. a debt-free,
cash-free business).

#### Value basis and conversion

A consensus across several valuation methods must never silently average
an Enterprise Value against an Equity Value against an adjusted net asset
value — three different quantities that happen to be denominated in the
same currency. `valuation/basis` is a small, focused package built
specifically to make that mixing impossible without an explicit,
inspectable conversion step; `valuation/consensus.Calculate` uses it
internally whenever a caller asks for a specific basis.

```go
type ConversionInput struct {
    Method    valuation.Code
    ValueType valuation.ValueType
    Value     float64
    Bridge    valuation.Bridge // the method's own already-computed bridge, if any
}

func Convert(in ConversionInput, target valuation.ValueType) Conversion
func ConvertAll(inputs []ConversionInput, target valuation.ValueType) []Conversion
```

**Conversion rules — exactly three, no others invented:**

1. **Same basis already** (`ValueType == target`): `OutcomeDirect`, the
   value passes through unchanged.
2. **Enterprise → Equity, with a bridge available**
   (`in.Bridge.Available == true`): `OutcomeConverted`, using the exact
   `Bridge.EquityValue` the source method itself already computed — never
   recomputed here. Worked example, from an EBITDA-multiple result that
   requested an equity bridge:

   ```
   EBITDA method:
   Enterprise Value = 1,800,000
   + Excess Cash      100,000
   - Debt             300,000
   ---------------------------
   Equity Value     = 1,600,000
   ```

   A consensus requested on an equity basis uses `1,600,000`, never the
   raw `1,800,000`.
3. **Asset value → Equity value**: `OutcomeConverted` via an explicit
   *identity* (`ConvertedValue == OriginalValue`), not a cash/debt bridge
   — `Conversion.Bridge.Available` stays `false` for this case, so it's
   never confused with rule 2's real bridge arithmetic. An adjusted net
   asset value (`valuation/netassets`'s `Adjusted Assets - Adjusted
   Liabilities`) and a balance-sheet-basis equity value are the same
   subtraction; this rule only says that once a caller has *explicitly*
   chosen an equity-basis comparison, this is the one defensible number to
   use for a net-asset-value result — it does **not** mean adjusted net
   asset value and a going-concern earnings-based equity value are
   interchangeable in what they represent (see
   [`valuation/netassets`](#valuationnetassets--adjusted-net-asset-value)'s
   own section below for why its `ValueType` stays `ValueTypeAsset` rather
   than being reclassified as `ValueTypeEquity`).

**Everything else is `OutcomeExcluded`**, with a structured
`ExclusionReason` — equity → enterprise, enterprise → equity with no
bridge available, and asset → enterprise. No conversion is invented for
these cases; a result excluded for the "no bridge available" reason can
often be included on retry simply by supplying
`EquityBridge.Requested = true` on that method's own `Input` upstream, so
its `Result.Bridge` is populated the next time consensus is computed.

**How `valuation/consensus` uses this:**

```go
c := consensus.Calculate(inputs, consensus.Options{
    TargetBasis: valuation.ValueTypeEquity,
})
```

- `Options.TargetBasis` left empty means "infer": if every `Input` already
  shares one `ValueType`, that becomes `Result.Basis`; if they don't,
  `Calculate` refuses to guess — `Result.Available` is `false` and
  `Result.Errors` carries an `IssueIncompatibleValueBasis` entry, rather
  than falling back to averaging raw incompatible values (the previous,
  now-removed behavior).
- With `TargetBasis` set, every `Input` goes through `basis.Convert`.
  `Result.Included` holds only the successfully converted subset (on
  `Result.Basis`) that `Statistics`/`Range`/`Dispersion` are actually
  computed over. `Result.Conversions` lists every input's outcome, in
  order, always populated; `Result.BasisExclusions` is the subset that
  couldn't convert — never silently vanished, always paired with a reason.
- `report.BuildConsensusInputs` populates each `consensus.Input.Bridge`
  from the corresponding method's own `Result.Bridge` automatically (for
  EBITDA/DCF), so a caller doesn't need to re-supply cash/debt figures a
  second time just to enable basis conversion.

**Not part of this mechanism:** `valuation/sde.EquityBridgeInput`/
`Result.Bridge` is a different, narrower thing — an optional cash/debt
adjustment applied *on top of* SDE's already-equity-value result "in case
a caller's convention wants it" (see that package's own section below),
not an Enterprise-Value-to-Equity-Value conversion. It is never consulted
by `valuation/basis`.

#### `valuation/sde` — SDE Multiple

```
Equity Value = Maintainable SDE x Multiple
```

| | |
|---|---|
| Code / Version | `SDE_MULTIPLE` / `1.0.0` |
| Required inputs | `MaintainableSDE`, `Multiple` |
| Output value type | `equity_value` |
| Optional | `EquityBridge` (see above) |

Validation: `Multiple` must be finite and `> 0` (`IssueNonPositiveMultiple`,
blocking) — a multiple of zero or less has no defensible interpretation.
`MaintainableSDE` must be finite, but a **zero or negative** value is
**not** a blocking error: it is a real, calculable outcome (a business with
no discretionary earnings is validly priced at zero-or-below by a pure
multiple method), surfaced as a non-blocking `IssueNonPositiveSDE`
warning. `Calculate` never clamps a negative result up to zero — see
`sde_test.go`'s `TestCalculate_NegativeSDE`, which asserts the exact
negative product is returned.

```go
res := sde.Calculate(sde.Input{MaintainableSDE: 218800, Multiple: 2.5})
// res.EquityValue == 547000, res.Available == true
```

#### `valuation/ebitda` — EBITDA Multiple

```
Enterprise Value = Maintainable EBITDA x Multiple
Equity Value      = Enterprise Value + Excess Cash - Total Debt   (if EquityBridge requested)
```

| | |
|---|---|
| Code / Version | `EBITDA_MULTIPLE` / `1.0.0` |
| Required inputs | `MaintainableEBITDA`, `Multiple` |
| Output value type | `enterprise_value` (native); `equity_value` via `Bridge.EquityValue` when requested |
| Optional | `EquityBridge`: `ExcessCash`, `ShortTermDebt`, `LongTermDebt`, `OtherDebt` |

Validation mirrors `valuation/sde`: `Multiple` must be finite and `> 0`
(blocking `IssueNonPositiveMultiple`); a zero or negative
`MaintainableEBITDA` is a non-blocking `IssueNonPositiveEBITDA` warning,
never clamped. Every `EquityBridge` field is independently checked for
finiteness when a bridge is requested, and the bridge is never computed at
all if the base calculation itself is invalid.

```go
res := ebitda.Calculate(ebitda.Input{
    MaintainableEBITDA: 126800, Multiple: 3.5,
    EquityBridge: ebitda.EquityBridgeInput{
        Requested: true, ExcessCash: 63000, ShortTermDebt: 8000, LongTermDebt: 47000,
    },
})
// res.EnterpriseValue == 443800
// res.Bridge.EquityValue == 443800 + 63000 - 55000 == 451800
```

#### `valuation/capitalization` — Capitalization of Earnings

```
Equity Value = Maintainable Earnings / Capitalization Rate
```

| | |
|---|---|
| Code / Version | `CAPITALIZATION_OF_EARNINGS` / `1.0.0` |
| Required inputs | `MaintainableEarnings`, `CapitalizationRate` (decimal, e.g. `0.20` = 20%) |
| Output value type | `equity_value` (always — see the package doc comment on why this holds regardless of whether the caller's earnings base is SDE-like or EBIT-like) |

This package **never invents a capitalization rate** — no build-up-method
helper, no default. Validation: `CapitalizationRate` must be finite and
`> 0` (blocking `IssueNonPositiveCapRate`) — a zero rate is an undefined
division, and a negative rate would invert the formula's direction under
an unstated alternate convention this package refuses to guess at. A zero
or negative `MaintainableEarnings` is a non-blocking `IssueNonPositiveEarnings`
warning, never clamped.

```go
res := capitalization.Calculate(capitalization.Input{
    MaintainableEarnings: 218800, CapitalizationRate: 0.30,
})
// res.EquityValue == 729333.33...
```

#### `valuation/dcf` — Discounted Cash Flow

```
PV(period i)    = FCF(i) / (1 + discount_rate)^i
FCF(n+1)        = FCF(n) x (1 + terminal_growth_rate)
Terminal Value  = FCF(n+1) / (discount_rate - terminal_growth_rate)
Enterprise Value = sum(PV(period i)) + Terminal Value / (1 + discount_rate)^n
Equity Value     = Enterprise Value + Excess Cash - Total Debt   (if EquityBridge requested)
```

| | |
|---|---|
| Code / Version | `DCF` / `1.0.0` |
| Required inputs | `ForecastPeriods []ForecastPeriod` (>= 1, caller-supplied, chronological), `DiscountRate`, `TerminalGrowthRate` |
| Output value type | `enterprise_value` (native); `equity_value` via `Bridge.EquityValue` when requested |
| Optional | `MidYearConvention` (discount exponent `i-0.5` instead of `i`), `EquityBridge` |

**This package generates no forecasts.** Every `ForecastPeriod.FreeCashFlow`
is caller-supplied; `Calculate` only discounts, sums, and computes the
Gordon Growth terminal value from the figures it is given —
`IssueNoForecastPeriods` blocks an empty forecast outright rather than
inventing one.

Validation: `DiscountRate` must be finite and `> 0`
(`IssueNonPositiveDiscountRate`, blocking); **`DiscountRate` must be
strictly greater than `TerminalGrowthRate`**
(`IssueDiscountRateNotAboveTerminalGrowth`, blocking) — at or below zero,
Gordon Growth's denominator implies a business growing as fast as or
faster than it is discounted, which is not a large-but-real number, it is
economically undefined. A zero/negative forecast cash flow —
**including a negative terminal-year cash flow**, which propagates
straight through to a negative terminal value — is **not** blocking: both
are reported as non-blocking warnings (`IssueNegativeForecastCashFlow`,
`IssueNegativeTerminalCashFlow`) and never clamped.

`Result` never hides a calculation step: every `ProjectedPeriod`
(cash flow, discount factor, present value), `SumOfPresentValues`,
`TerminalYearCashFlow`, `TerminalCashFlow` (`FCF(n+1)`), `TerminalValue`,
`TerminalValueDiscountFactor`, and `TerminalValuePresentValue` are all
independently exposed fields, not folded into `EnterpriseValue` alone.

```go
res := dcf.Calculate(dcf.Input{
    ForecastPeriods: []dcf.ForecastPeriod{
        {Period: "2026", FreeCashFlow: 130000},
        {Period: "2027", FreeCashFlow: 140000},
        {Period: "2028", FreeCashFlow: 150000},
    },
    DiscountRate: 0.22, TerminalGrowthRate: 0.03,
})
// res.TerminalValue == 154500 / (0.22 - 0.03) == 813157.89...
// res.EnterpriseValue == res.SumOfPresentValues + res.TerminalValuePresentValue
```

#### `valuation/netassets` — Adjusted Net Asset Value

```
Adjusted Net Asset Value = Total Adjusted Assets - Total Adjusted Liabilities
```

| | |
|---|---|
| Code / Version | `ADJUSTED_NET_ASSET_VALUE` / `1.0.0` |
| Required inputs | `Assets []AssetItem` (>= 1, non-empty) |
| Optional inputs | `Liabilities []LiabilityItem` (may be empty — a debt-free, liability-free business is possible) |
| Output value type | `asset_value` (never `equity_value` — see below) |

This package **never assumes book value equals fair market value.** Every
`AssetItem`/`LiabilityItem.Amount` is a caller-supplied fair/adjusted
value; this package has no idea whether a given `Amount` is a raw book
figure carried through unchanged or a genuine fair-value override — that
distinction is made explicit per item via `IsOverride` (`true` marks an
item as an explicit fair-value override that differs from reported book
value, e.g. a fixed asset revalued via appraisal), purely for
traceability. A caller that wants to start from book values (e.g.
`metrics.Snapshot.TangibleAssetValue`) and apply no further adjustment
does so explicitly — that is a legitimate, but distinct, choice this field
makes visible rather than leaving a reader to guess whether an adjustment
happened.

Validation: `Assets` must be non-empty (`IssueNoAssets`, blocking) — a
net asset value with *no* assets at all is invalid input, as opposed to
assets that are present but sum to zero (a fully written-down asset base),
which is a valid, calculable `0`. Every item's `Amount` must be finite. A
negative item `Amount` is a non-blocking `IssueNegativeItemAmount` warning
(unusual, e.g. a write-down override, but not rejected). **Liabilities
exceeding assets is not a blocking error** — a negative net asset value
for an insolvent or heavily-levered business is a real, calculable outcome
(`IssueLiabilitiesExceedAssets`, warning, never clamped to zero).

`Result.AssetComponents`/`LiabilityComponents` itemize every contributing
line, in input order, so the total is never an opaque number.

```go
res := netassets.Calculate(netassets.Input{
    Assets: []netassets.AssetItem{
        {Label: "Cash", Amount: 238000},
        {Label: "Fixed Assets (net book value)", Amount: 1595000},
        {Label: "Fixed Assets (appraisal fair-value adjustment)", Amount: 405000, IsOverride: true,
            Notes: "independent appraisal values the facility 405,000 above depreciated book value"},
    },
    Liabilities: []netassets.LiabilityItem{
        {Label: "Long-Term Debt", Amount: 825000},
    },
})
// res.ValueType == valuation.ValueTypeAsset
// res.AdjustedNetAssetValue == (238000+1595000+405000) - 825000 == 1413000
```

Net asset value is deliberately **not** reconciled or compared against an
earnings-based method's equity value here — see
[Recommended next module](#recommended-next-module) for why that
comparison is left to a future consensus module.

#### Validation

Every method follows the same two-tier convention, using the shared
`valuation.IssueSeverity`/`valuation.Issue` types (`valuation.SeverityError`
blocks the result; `valuation.SeverityWarning` does not):

- **Blocking (`Result.Available == false`, headline value `0`):** a
  non-finite numeric input anywhere in `Input` (including bridge/DCF
  sub-fields), a non-positive multiple/capitalization-rate/discount-rate,
  `DiscountRate <= TerminalGrowthRate`, no DCF forecast periods, or no NAV
  assets. **No method ever panics on bad financial input** — every one of
  these paths returns a normal `Result` with `Errors` populated.
- **Non-blocking (`Result.Available == true`, `Warnings` populated):** a
  zero/negative maintainable SDE/EBITDA/earnings, a zero/negative forecast
  or terminal-year DCF cash flow, a negative NAV item amount, or
  liabilities exceeding assets. These are real, calculable — if
  concerning — outcomes. **No method silently clamps a negative or
  zero-derived result up to a "normal-looking" positive number** — see
  each package's `_test.go` for a test asserting the exact (negative)
  value is returned rather than zero.

`valuation.HasErrors(issues)`, `valuation.Errors(issues)`, and
`valuation.Warnings(issues)` are shared helpers for filtering a mixed
`[]valuation.Issue` slice, mirroring
`financial/adjustments.HasErrors`'s role for that package's `Issue` type.

#### Golden fixtures

[`fixtures/valuation_by_business_type.json`](fixtures/valuation_by_business_type.json)
holds each of the same four business archetypes' (HVAC, agency,
manufacturer, SaaS growth company) method assumptions — SDE/EBITDA
multiples, a capitalization rate, a DCF forecast with discount/terminal
growth rates, and itemized adjusted net asset value inputs (including the
manufacturer's appraisal-override fixed-asset item) — used by every method
package's own `fixtures_test.go`. Each of those tests derives the
archetype's maintainable SDE/EBITDA **live** from
`financial/metrics.Calculate` over the corresponding
`fixtures/normalized_<archetype>_multi_year.json` 2025 snapshot (the same
pattern `financial/adjustments/fixtures_test.go` uses) rather than
duplicating that figure in the valuation fixture, so expected outputs are
computed independently in the tests themselves and can never silently
drift from either fixture file.

### `valuation/profile`

A minimal, domain-level description of the business being valued, used
**only** to drive `valuation/applicability`'s scoring rules. It is
deliberately not a CRM/business-entity model: no name, address, contact,
ownership, or account/client/tenant concept — those belong to a consuming
application.

`Profile` fields are optional (pointers, or a zero value that is itself a
legitimate "unknown" state): `Industry`, `OwnerOperated *bool`,
`AnnualRevenue *float64`, `EmployeeCount *int`, `AssetIntensity *float64`
(tangible operating assets / revenue), `RecurringRevenuePercent *float64`,
`HistoricalGrowthRate *float64`, `EarningsStability`, `Profitability`,
`YearsInOperation *int`, and `DataAvailability` (which underlying inputs —
multi-year financials, a balance sheet, an explicit forecast, appraised
asset values — the caller actually has on hand). A `Profile` with every
field left unset is valid input; applicability rules degrade to a neutral
default rather than failing.

### `valuation/applicability`

Deterministic, transparent rules that score how well-suited each
individual valuation method is to a particular business, from a
`profile.Profile`. **Every rule is a fixed, documented point-scoring
heuristic — never a statistical model, never trained on data, and never a
probability.**

```go
type Level string

const (
    LevelHigh          Level = "HIGH"
    LevelMedium        Level = "MEDIUM"
    LevelLow           Level = "LOW"
    LevelNotApplicable Level = "NOT_APPLICABLE"
)

func Calculate(p profile.Profile) Results
```

`Results.Methods` holds one `Result` per method (SDE, EBITDA,
Capitalization, DCF, NetAssets), each with `Score int` (0-100), `Level`,
`Recommended bool` (`Level` is HIGH or MEDIUM), `Reasons []Reason` (every
point contribution, positive or negative, with a fixed human-readable
`Detail` — nothing is scored silently), and `Warnings []string`.

**Scoring, and the full explanation trail.** Every method starts at
`BaseScore = 50` (a neutral MEDIUM, so a wholly-empty `Profile` doesn't
default to HIGH with zero evidence or LOW as if missing data were itself
disqualifying). Rules then add or subtract fixed point deltas
(`+25`/`+15`/`+8`/`-8`/`-15`/`-25`) based on `Profile` fields relevant to
that method's conventional fit — e.g. SDE gains heavily for
`OwnerOperated == true` and small revenue/headcount; EBITDA gains for
`OwnerOperated == false` and larger revenue; Capitalization needs stable
`EarningsStability`; NetAssets needs high `AssetIntensity` and an available
balance sheet. `Result` exposes every intermediate a reviewer needs to
reconstruct `Score` without reading source — `BaseScore`, `RawScore` (the
pre-clamp total: `BaseScore` + every `Reason.Points`), `Score` (`RawScore`
clamped to `[0,100]`), and `Clamped bool` (whether clamping actually
changed the number) — matching this worked example exactly:

```
base score:                         50
owner-operated service business:  +25
low asset intensity:              +15
stable positive earnings:         +15
--------------------------------------
raw score:                        105
clamped score:                     100
```

`Score` is mapped to a `Level` via fixed thresholds (80-100 HIGH, 50-79
MEDIUM, 1-49 LOW, 0 NOT_APPLICABLE).

**DCF is the one hard-blocked method.** `valuation/dcf` never generates a
forecast (see its own section below), so `scoreDCF` returns `Score: 0,
Level: NOT_APPLICABLE` outright — with `HardBlockReason` set — whenever
`Profile.DataAvailability.HasForecast` is false — a data-availability
block, not a matter of degree — regardless of how well the business's
growth profile would otherwise suit a DCF. **Revenue-multiple valuation is
out of scope for this repository and is never invented here as a DCF
substitute** for a growth company with weak current earnings; DCF is
scored applicable only when the caller supplies (or intends to supply) an
explicit forecast. `HardBlockReason` is what distinguishes this genuine
structural block from an ordinary low score reached by accumulating
`Reason`s elsewhere: "this method cannot run at all without more data"
(hard block, `HardBlockReason` non-empty) is a materially different
message to show a reviewer than "this method could run, but scores poorly
for this business" (a low but non-blocked `RawScore`, `HardBlockReason`
empty).

```go
r := applicability.Calculate(profile.Profile{
    OwnerOperated:  boolPtr(true),
    AnnualRevenue:  floatPtr(900_000),
    AssetIntensity: floatPtr(0.2),
})
sdeResult, _ := r.ForMethod(string(valuation.CodeSDEMultiple))
// sdeResult.Level == applicability.LevelHigh
```

### `valuation/orchestrator`

A pure orchestrator that runs a selected set of individual valuation
methods against caller-supplied, method-specific inputs and reports one
outcome per method — **it never aborts the whole run because one method
is unavailable or excluded.**

```go
type Outcome string

const (
    OutcomeSuccess     Outcome = "success"     // ran, Result.Available == true
    OutcomeUnavailable Outcome = "unavailable" // ran, but the method's own validation blocked it
    OutcomeExcluded    Outcome = "excluded"    // never ran at all
)

func Execute(req Request) Run
```

`Request` carries a `settings.Resolution`, an optional
`*applicability.Results` (+ `FilterPolicy` to choose how applicability
affects which methods run — see below), and one optional `*Input` per
method (`SDE *sde.Input`, `EBITDA *ebitda.Input`, etc. — `nil` means
"don't run this method"). `Run.Methods` lists every method's
`MethodOutcome`, in a fixed order (SDE, EBITDA, Capitalization, DCF,
NetAssets), each carrying the method's own strongly-typed `*Result`
pointer (not an `any`) when it ran, plus
`Successful()`/`Excluded()`/`Unavailable()` convenience filters and a
flattened `Warnings []MethodWarning` across every method.

**Applicability filtering policy.** The library never hard-codes a single
product behavior for how a low applicability score affects whether a
method runs — the caller chooses via `applicability.FilterPolicy`:

```go
const (
    PolicyIncludeAllEnabled    FilterPolicy = "INCLUDE_ALL_ENABLED"    // the zero value: applicability is informational only
    PolicyExcludeNotApplicable FilterPolicy = "EXCLUDE_NOT_APPLICABLE" // excludes only a hard block (Level == NOT_APPLICABLE)
    PolicyMinimumLevel         FilterPolicy = "MINIMUM_LEVEL"          // excludes anything below Request.MinApplicabilityLevel
    PolicyExplicitSelection    FilterPolicy = "EXPLICIT_METHOD_SELECTION" // only Request.SelectedMethods run, regardless of score
)
```

`PolicyIncludeAllEnabled` (the zero value — a caller that never opts in
gets this) means every settings-enabled method with a supplied `Input`
runs regardless of score, `Applicability` remaining purely informational
on every `MethodOutcome`. `MinApplicabilityLevel`/`SelectedMethods` are
only consulted under their respective policy; ignored otherwise.

**Evaluation order per method:** (1) explicitly disabled by
`settings.Resolution` → `ExclusionDisabledBySettings`; (2) no `Input`
supplied → `ExclusionNoInput`; (3) `Request.FilterPolicy` applied, if
`Applicability` was supplied →
`ExclusionNotApplicable`/`ExclusionLowApplicability`/`ExclusionNotSelected`
depending on policy; (4) otherwise the method's own `Calculate` runs, and
`Outcome` is `OutcomeSuccess`/`OutcomeUnavailable` from that `Result`'s own
`Available` field — the orchestrator never re-derives or overrides it. A
method with no explicit `method_enabled.<method>` entry anywhere in the
`Resolution` defaults to **enabled**.

```
SDE:        success
EBITDA:     success
DCF:        unavailable — no forecast cash flow supplied
NetAssets:  excluded — no input supplied
```

### `valuation/consensus`

Combines multiple included method results into descriptive statistics —
never a claim about the business's "true" value — over a single,
explicit value basis (see [Value basis and conversion](#value-basis-and-conversion)
above for the full mechanism this package builds on).

```go
type Input struct {
    Method    valuation.Code
    ValueType valuation.ValueType
    Value     float64
    Weight    float64
    Bridge    valuation.Bridge // the method's own bridge, for enterprise->equity conversion
}

type Options struct {
    TargetBasis valuation.ValueType // empty = infer only if every Input already shares one basis
}

func Calculate(inputs []Input, opts Options) Result
```

**Value basis is resolved before any statistic is computed.** With
`Options.TargetBasis` set, every `Input` is run through
`valuation/basis.Convert`; `Result.Included` holds only the subset that
converted successfully (on `Result.Basis`), and `Statistics`/`Range`/
`Dispersion` are computed over exactly that subset — never over a raw mix
of incompatible bases. With `TargetBasis` left empty, a basis is inferred
only if every `Input` already shares one `ValueType`; if they don't,
`Calculate` returns `Available: false` with an `IssueIncompatibleValueBasis`
error rather than guessing. `Result.Conversions` lists every input's
outcome (direct/converted/excluded), always populated;
`Result.BasisExclusions` is the excluded subset, each with a structured
reason — a method a caller expected to see in the consensus but that
couldn't convert is never silently absent without explanation.

`Result.Statistics` (`Statistics` struct) holds: `SimpleMean` ("Simple
Consensus"), `WeightedMean` ("Weighted Consensus", meaningful only when
`Result.WeightsValid`), `Median`, `Min`/`Max`/`Spread`, `StdDev`
(**population** standard deviation — every included result is the complete
set being summarized, not a sample), `CoefficientOfVariation` (=
`StdDev / |SimpleMean|`), and `DeviationsFromMean`/
`DeviationsFromWeightedMean` (each method's signed and percent deviation
from the respective central figure). `Result.Range` is the explicit
`{Min, Max}` method range — **this package invents no narrower "likely
range."** `Result.MixedValueTypes` echoes whether `Requested` carried more
than one `ValueType` *before* conversion — purely informational once
`Basis`/`Conversions` exist; it no longer determines blocking/exclusion
behavior by itself.

**Formulas:**

```
Simple Consensus (SimpleMean)     = sum(values) / count
Weighted Consensus (WeightedMean) = sum(value_i * normalized_weight_i)
Median                             = middle value(s) of sorted values
Spread                             = max - min
StdDev (population)                = sqrt(sum((v_i - mean)^2) / count)
CoefficientOfVariation             = StdDev / |SimpleMean|
```

A zero-mean edge case (e.g. included values straddling zero) never
produces `NaN`/`Inf`: any ratio dividing by a zero denominator is defined
as `0` (`percentOf`), since `encoding/json` rejects `NaN`/`Inf` outright and
a corrupted downstream figure would be worse than a defined zero.

**Weight handling** (`ValidateWeights`): every `Weight` must be finite and
non-negative, and the included set's weights must sum to a strictly
positive number — any failure invalidates the *entire* weighted-mean
calculation (`Result.WeightsValid = false`, with the reason(s) in
`Result.Errors`); this package never silently drops the offending
`Input` or substitutes equal weights. Every other statistic (`SimpleMean`,
`Median`, etc.) is still computed and returned regardless, since they don't
depend on weights. **Weights are always normalized by dividing each by the
sum of every supplied weight**, so `{3, 1}`, `{30, 10}`, and `{0.75, 0.25}`
all produce an identical weighted mean — a deliberately simpler rule than
`financial/earnings`' fraction-or-percentage-near-1-or-100 acceptance
window, since dividing by the actual sum is always mathematically correct
here and removes an entire class of "did my weights sum right?" caller
error.

#### Consensus/dispersion score

`CalculateDispersion(stats Statistics) Dispersion` derives a deterministic
agreement indicator from `Statistics.CoefficientOfVariation` alone:

```go
const dispersionScoreCVDivisor = 0.5

Score = round(100 * (1 - CV/0.5)), clamped to [0, 100]
```

A CV of `0` (every included method agreed exactly) scores `100`; a CV at
or beyond `0.5` floors at `0`. `Dispersion` returns both the raw measures
(`CoefficientOfVariation`, `RelativeSpread = Spread / |SimpleMean|`) and
the derived `Score`/`Level`:

```go
const (
    LevelHighConsensus     Level = "HIGH_CONSENSUS"     // Score 70-100
    LevelModerateConsensus Level = "MODERATE_CONSENSUS" // Score 40-69
    LevelLowConsensus      Level = "LOW_CONSENSUS"       // Score 0-39
)
```

**This is a fixed, documented heuristic transformation for human-facing
display — not an industry-standard statistic**, and not a probability that
the methods "agree" in any statistical sense.

**Single-method consensus.** A `Result` with exactly one `Included` value
is still `Available` — dispersion reads as `Score: 100` (`CV == 0`,
mathematically, since a single value has nothing to differ from), meaning
**dispersion is zero by definition**, not "unavailable" or "not
meaningful." `Result.Warnings` still flags the single-method case
separately, since a Score of 100 from one method carries far less
evidentiary weight than the same Score from several independently
agreeing methods — but the Score/Level themselves follow the same formula
as any other count.

**Error taxonomy.** `Result.Errors`/`Result.Warnings` are
`[]valuation.Issue` (the same stable-code type every individual method
package uses — see [Error taxonomy](#error-taxonomy) below), not freeform
strings: `IssueIncompatibleValueBasis`, `IssueInvalidWeight`,
`IssueMissingRequiredData`, and `IssueDuplicateMethodResult` (a warning —
the same `valuation.Code` appearing more than once in `Included`, which
would otherwise silently overweight that method in every mean/dispersion
figure) are the codes this package produces.

### `valuation/sensitivity`

Reusable, deterministic sensitivity analysis over already-defined
valuation calculations. **This package generates no scenarios itself** —
every multiple, earnings adjustment, discount rate, and terminal growth
rate analyzed here is caller-supplied.

- **`MultipleSensitivity(earnings float64, multiples []float64) MultipleSensitivityResult`**
  — one `MultiplePoint{Multiple, Value, Valid, Reason}` per multiple.
  `Value = earnings * multiple`. A non-finite or non-positive multiple is
  marked `Valid: false` with a `Reason` — **never silently dropped, never
  computed under a substituted value.**
- **`EarningsMultipleMatrix(scenarios []EarningsScenario, multiples []float64) Matrix`**
  — a full 2D grid, `Rows[i][j] = EarningsScenarios[i].Earnings *
  Multiples[j]`, with the same per-cell invalid-multiple handling (an
  invalid column doesn't affect other columns or rows).
- **`DCFSensitivity(forecastPeriods []dcf.ForecastPeriod, discountRates, terminalGrowthRates []float64, equityBridge dcf.EquityBridgeInput) DCFGrid`**
  — for every `(discountRate, terminalGrowthRate)` combination, re-runs
  `dcf.Calculate` and reports the full `dcf.Result`. **A combination where
  `discountRate <= terminalGrowthRate` (or any other `dcf.Calculate`
  validation failure) is marked `Valid: false` rather than computed** —
  this package duplicates none of `dcf.Calculate`'s validation logic itself,
  it only reports what that call already decided.

### `valuation/report`

A presentation-neutral, JSON-serializable report data model assembled from
every other package's output. **This package computes nothing new** —
`Build` only reshapes upstream `Result`/`Run`/`Result` values into one
structure suitable for a future Vue UI, a JSON API response, PDF
generation, or a CSV/export pipeline, none of which this package
implements. No chart library types, no HTML, no PDF bytes — only plain
structs, strings, and numbers.

```go
func Build(in BuildInput) Report
func BuildConsensusInputs(run orchestrator.Run, weights map[valuation.Code]float64) []consensus.Input
```

Every `BuildInput` field is optional; a caller can build a minimal
`Report` (e.g. just `Methods`) as easily as a complete end-to-end one —
each `Report` section simply stays at its zero value when the
corresponding input wasn't supplied.

`Report` sections:

- **`Summary`** — `ValuationDate` (caller-supplied, unparsed),
  `SimpleConsensus`, `WeightedConsensus` (+ `WeightsValid`), `Median`,
  `MethodRange`, `ConsensusLevel`/`ConsensusScore` (from
  `consensus.Dispersion`), `ConsensusAvailable`. Field names deliberately
  echo the package brief's caller-facing vocabulary ("Simple Consensus",
  "Weighted Consensus") — **never "true value."**
- **`Financial`** — one `FinancialPeriod` (`Revenue`, `EBITDA`,
  `EBITDAMargin`, `SDE`, `GrossMargin`, each an available/value pair) per
  supplied `metrics.Snapshot`, plus `NormalizedEBITDA`/`NormalizedSDE`
  and caller-reshaped `GrowthMetrics`.
- **`Methods`** — one `MethodComparisonRow` per method, in the same fixed
  order as `orchestrator.Execute`: `Value`, `ValueType`, `Included`,
  `Outcome`, `ExclusionReason`, `Weight` (the *normalized* weight actually
  used in `WeightedConsensus`, re-derived via `consensus.ValidateWeights`
  — not the caller's raw supplied weight), `Applicability`, `Assumptions`
  (plain label/value pairs, e.g. `{"Multiple", "3.50x"}` — so a consumer
  never needs to know five different `Input` schemas), `Steps` (the
  method's own calculation trace), `Warnings`.
- **`Adjustments`** — `Applied` adjustments and `EBITDABridge`/`SDEBridge`
  as a flat, ordered `[]BridgeLine` (starting reported figure, each signed
  adjustment delta, final normalized figure — `IsTotal` marks the two
  running-total lines).
- **`Sensitivity`** — `MultipleSensitivity`, `EarningsMultipleMatrix`, and
  `DCFSensitivity`, each flattened to a single-level `[]Row` slice (never
  nested), so a CSV export can emit any of them directly.
- **`Series`** (`ChartSeries`) — `ValuationByMethod`, `RevenueHistory`,
  `EBITDAHistory`, `SDEHistory`, `MarginHistory`, and a
  `ValuationHistory` placeholder (empty until a future persistence layer
  exists), each a plain `[]SeriesPoint{Label, Value}` with **no dependency
  on any charting library.**

Every field serializes cleanly via `encoding/json` (snake_case tags, no
`NaN`/`Inf` — `valuation/consensus`'s zero-denominator convention makes
this guaranteed rather than incidental); see
[`valuation/report/report_test.go`](valuation/report/report_test.go)'s
round-trip and NaN/Inf-safety tests.

### `analytics/valuedrivers`

A deterministic **business value driver/scenario engine**: given a
baseline `valuation/orchestrator.Request` (the same request shape a
caller already builds to run SDE/EBITDA/capitalization/DCF/net-asset-value
together), a `valuation/consensus.Options`/weights pair, and a set of
caller-defined named **drivers** (revenue growth, margin change, SDE
change, a method's multiple, a capitalization rate, DCF's discount/
terminal-growth rate, a debt/cash bridge change, a working-capital
change, an owner-compensation addback, a customer-loss impact, or an
explicit method-multiple replacement), this package shows how each
driver — alone (one-factor-at-a-time) or combined into a named scenario
— changes the resulting method results and consensus, against a baseline
it computes once from the same `Request`.

**This package invents no new valuation formula.** Every recalculated
figure in a `ScenarioResult` comes from re-running
`valuation/orchestrator.Execute` and `valuation/consensus.Calculate`
exactly as a caller invoking them directly would, against a deep-copied
and driver-mutated `orchestrator.Request` — see `cloneRequest` in
[`apply.go`](analytics/valuedrivers/apply.go). A `Driver` only ever
changes the same strongly-typed `Input` fields
(`sde.Input.MaintainableSDE`, `ebitda.Input.Multiple`,
`dcf.Input.DiscountRate`, an `EquityBridgeInput`'s debt/cash fields, etc.)
a caller could change by hand.

**Explicit linkage only, never inferred causation.** Every `DriverType`
constant's doc comment names exactly which method(s) and `Input` field(s)
it can ever touch. A driver that names a method it has no documented
linkage to, or whose baseline `Request` had no `Input` for that method at
all, produces a `Linkage` of `LinkageNotApplicable` or
`LinkageMethodExcluded` for it — not silence, and not an error — while
still running to completion for every method it does apply to. This is
the package brief's central guardrail: a "recurring revenue percentage"
input can only change a method's multiple through
`DriverMethodMultipleRule`, which requires the caller to supply the
resulting multiple explicitly (`NewMultiple`) and only echoes
`TriggerLabel`/`TriggerValue` as descriptive metadata — this package never
derives a multiple from a percentage (or any other figure) itself.

**One-factor-at-a-time vs. combined.** `Input.Drivers` is analyzed
independently — each entry becomes its own single-driver `Scenario`
against a fresh clone of the baseline `Request` — and reported in
`Result.OneFactorAtATime`. `Input.Scenarios` is analyzed separately: each
named `Scenario`'s full `Drivers` list is applied together, in order,
against one fresh clone, so later drivers see earlier drivers' already-
mutated fields (deltas compound, exactly as a caller manually building
the same combined `Request` by hand would expect) — reported in
`Result.Scenarios`. Both share one `ScenarioResult` shape.

**Availability, not silence.** A structurally invalid `Driver` (empty
`ID`, unrecognized `Type`, a nil params field for the selected `Type`, a
missing required sub-field like `MarginChangeParams.RevenueBase` or
`MethodMultipleRuleParams.Method`) is recorded as a blocking
`DriverIssue` and contributes no mutation at all — never silently
ignored. A driver that runs without error but ends up with
`LinkageApplied` for zero methods (every targeted method was excluded or
not applicable) is flagged `IssueNoLinkedMethod` as an advisory warning.
A `Scenario` with an empty `ID` or no `Drivers` is `Available: false`.

**Deltas.** Every `MethodDelta` and the scenario-level consensus delta
are gated on both the baseline and scenario figure being `Available` —
`DeltaAvailable`/`ConsensusDeltaAvailable` — so a method that flips
availability in either direction under a scenario (e.g. a multiple driver
pushing `Multiple` non-positive) never reports a misleading numeric
delta.

Files:

- **`types.go`** — `DriverType` and its eleven params structs
  (`RevenueGrowthParams`, `MarginChangeParams`, `SDEChangeParams`,
  `MultipleChangeParams`, `CapRateChangeParams`,
  `DiscountRateChangeParams`, `DebtChangeParams`,
  `WorkingCapitalChangeParams`, `OwnerCompensationAdjustmentParams`,
  `CustomerLossImpactParams`, `MethodMultipleRuleParams`), `Driver`,
  `Scenario`, `Linkage`/`LinkageStatus`, `DriverIssueCode`/
  `DriverIssueSeverity`/`DriverIssue`, `HasDriverErrors`, `MethodValue`,
  `MethodDelta`, `ChangedInput`, `Assumption`, `ScenarioResult`,
  `Baseline`, `Input`, `Result`, `FormulaVersion`.
- **`apply.go`** — `cloneRequest` (the deep-copy every scenario mutates a
  fresh instance of, including `Applicability`), `applyDriver`'s
  per-`DriverType` dispatch, and one `apply<DriverType>` function per
  driver stating its own per-method linkage precondition.
- **`valuedrivers.go`** — `Calculate(Input) Result`: baseline
  orchestrator/consensus run, the shared `runScenario` engine behind both
  `OneFactorAtATime` and `Scenarios`, and every delta/assumption-rendering
  helper.

See [`analytics/valuedrivers/valuedrivers_test.go`](analytics/valuedrivers/valuedrivers_test.go),
[`analytics/valuedrivers/determinism_test.go`](analytics/valuedrivers/determinism_test.go),
and [`analytics/valuedrivers/roundtrip_test.go`](analytics/valuedrivers/roundtrip_test.go)
for every scenario the task requires (revenue growth, margin change, SDE
change, every rate/multiple driver, debt and working-capital changes,
owner-compensation and customer-loss impacts, the caller-supplied
method-multiple rule, a structurally invalid driver, a driver with no
linkage to any method in a given baseline, a combined scenario that
compounds differently from the naive sum of its one-factor-at-a-time
parts, no-mutation-of-caller-input including `Applicability`, and full
JSON round-trip/determinism coverage).

### `analytics/consolidation`

A deterministic **multi-entity financial consolidation** engine: given
multiple entities' own `financial.FinancialDataset`s plus caller-supplied
ownership/currency/elimination information, `Calculate` combines them into
a single consolidated `financial.FinancialDataset` — one `NormalizedItem`
per `(Code, Period)`, with `Sources` linking every consolidated figure back
to the entities that contributed to it — plus a full audit trail of what
was combined, converted, and eliminated along the way.

**The one analytics package whose primary output is itself a
`financial.FinancialDataset`.** Every sibling `analytics/` package reads a
dataset; this one produces one, using `financial.FinancialDataset`'s own
existing shape and provenance mechanism (`NormalizedItem.Sources`) rather
than inventing a parallel one — so `Result.Consolidated` can be fed
straight into any other package in this repository (`financial/metrics`,
`analytics/qoe`, `valuation/orchestrator`, ...) exactly as it would a
single entity's own dataset.

**Three things this package deliberately never does**, each directly from
its own prompt brief:

- **It never fetches FX rates.** `Input.CurrencyRates` is the complete,
  explicit set of `(FromCurrency, ToCurrency, Period, Rate)` conversions
  `Calculate` is allowed to use. A currency mismatch with no matching rate
  produces `IssueMissingCurrencyRate`, and that entity's items for the
  affected period are excluded from the consolidated total — never
  assumed to convert 1:1.
- **It never infers intercompany eliminations.** `Input.Eliminations` is
  the complete, explicit list of what to remove, keyed by
  `(EntityID, Code, Period, Amount)`. `Calculate` performs no analysis to
  detect an intercompany relationship on its own — it never assumes two
  entities' matching revenue/expense codes in the same period are
  intercompany trade, since that judgment requires visibility into the
  caller's own corporate structure this package cannot infer from amounts
  alone.
- **It never guesses which periods are "common."** `Input.Periods` is the
  caller-selected, explicit set of reporting periods to consolidate. Two
  entities' same-looking `financial.Period` string (e.g. `"2025"`) is a
  caller convention this package cannot verify actually denotes the same
  fiscal period without being told so explicitly — mirroring
  `analytics/workingcapital.Input.PeriodMeta`'s and every other analytics
  sibling's identical "no guessing chronology/period identity" rule,
  applied here to period *selection* rather than *ordering*.

#### Full vs. ownership-weighted consolidation

```go
type Mode string

const (
    ModeFullConsolidation Mode = "full_consolidation"
    ModeOwnershipWeighted Mode = "ownership_weighted"
)
```

The zero `Mode` resolves to `ModeFullConsolidation` — the standard
majority/controlling-interest accounting convention: every selected
entity's full amount is summed at every `(Code, Period)`, independent of
`EntityDataset.OwnershipPercent`. A caller cannot accidentally
under-consolidate a majority-owned subsidiary by forgetting to set
`OwnershipPercent`; supplying it is always harmless under this mode. Under
`ModeOwnershipWeighted` — which must be explicitly selected, per the task
brief ("simple ownership-weighted mode if explicitly selected") — every
converted amount is multiplied by `OwnershipPercent` before summing.
`OwnershipPercent` is required (non-nil, finite, in `[0, 1]`) for every
selected entity under this mode; an entity missing or failing that check
is excluded from consolidation entirely
(`IssueMissingOwnershipPercent`/`IssueInvalidOwnershipPercent`) rather than
treated as 0% or 100% owned.

#### Order of operations: eliminate, then convert, then weight

For each selected entity's item at `(Code, Period)`:

```
1. RawAmount       = item.Amount - sum(matching Eliminations), in the entity's native currency
2. ConvertedAmount = RawAmount * matching CurrencyRate (or RawAmount unchanged, if no conversion needed)
3. WeightedAmount  = ConvertedAmount * ownership weight (1.0 under ModeFullConsolidation)
```

`WeightedAmount` is what is actually summed into `Result.Consolidated`.
Eliminations apply first and in the entity's native currency —
`Elimination.Amount` is "removed before any currency conversion," mirroring
how a real consolidation nets out intercompany balances before translating
the net figure. An elimination can drive a contribution negative; this is
preserved rather than clamped to zero, since clamping would silently
understate the elimination. `EntityCodeContribution` (on each
`EntityContribution.Items` entry) exposes all three figures so a caller can
audit exactly what happened to a given line.

#### Target currency resolution

`Policy.TargetCurrency`, if set, is used as-is. If left empty, every
selected entity must share exactly one non-empty currency, which is then
used with no conversion attempted. Resolution fails
(`Result.Available == false`, `IssueMissingTargetCurrency`) — never
silently returning an empty string, since
`financial.FinancialDataset.Currency` is "required and never empty" — if
selected entities report more than one currency, or if no selected entity
has a non-empty currency at all, and `Policy.TargetCurrency` was not
supplied to resolve the ambiguity.

#### Reconciliation issues

`Calculate` never returns a Go `error`; every input or consistency problem
is a structured `Issue{Code, Severity, EntityID, Period, Message}` in
`Result.ReconciliationIssues` (split into `Warnings`/`Errors` by
`Severity`, for a caller that wants only one or the other) — the same
two-severity model every analytics sibling package uses, applied here to
consolidation-specific problems: an unknown/unselected/duplicate entity, a
missing or out-of-range ownership percentage, a missing or invalid
currency rate, an elimination targeting an unknown entity, an unselected
entity, an out-of-scope period, or a `(Code, Period)` with no matching
item, and a period in `Input.Periods` an entity has no data for at all
(`IssuePeriodMissingForEntity` — advisory only: that entity simply
contributes nothing for that period, mirroring every analytics sibling
package's "missing is not zero" availability discipline applied here at
the entity/period level).

#### What `Result` contains

| Field | What it is |
|---|---|
| `Consolidated` | The combined `financial.FinancialDataset`: one `NormalizedItem` per `(Code, Period)` across every selected entity's `WeightedAmount` contributions, with `Sources` carrying one `financial.SourceRef` per contributing entity (`RowID` is the `EntityID`, `Label` is the `EntityLabel`, `Amount` is that entity's `WeightedAmount`) — the provenance link back to source entities the task brief requires, expressed through `financial.FinancialDataset`'s own existing mechanism. |
| `EntityContributions` | One `EntityContribution` per selected entity that contributed at least one item: its currency, resolved `Mode`, `OwnershipPercent`, every `EntityCodeContribution` it contributed, and its `TotalWeightedContribution`. |
| `EliminationsApplied` | Every `Input.Eliminations` entry naming a known, selected entity (even one that turned out to target an out-of-scope period or a nonexistent line item — see the reconciliation-issues list above for those advisory flags). |
| `CurrencyConversions` | One entry per distinct `(EntityID, FromCurrency, ToCurrency, Period)` conversion actually applied, with the effective `Rate` used. |
| `ReconciliationIssues` / `Warnings` / `Errors` | Every input/consistency problem found — see above. |

Files:

- **`types.go`** — `Mode`, `EntityDataset`, `CurrencyRate`, `Elimination`,
  `Policy`/`resolvePolicy`/`resolveMode`, `Input`,
  `IssueSeverity`/`IssueCode`/`Issue`/`HasErrors`,
  `EntityCodeContribution`, `EntityContribution`, `CurrencyConversion`,
  `Result`, `FormulaVersion`.
- **`eliminations.go`** — `resolveEliminations`/`hasMatchingItem`: matches
  `Input.Eliminations` against known/selected entities and their actual
  line items, and builds the `elimKey -> amount` lookup
  `buildEntityContributions` subtracts.
- **`currency.go`** — `buildRateIndex`: validates `Input.CurrencyRates`
  and builds the `(FromCurrency, ToCurrency, Period) -> Rate` lookup.
- **`periods.go`** — `validatePeriodCoverage`: the period-alignment
  validation the task brief requires.
- **`contributions.go`** — `buildEntityContributions` (the eliminate ->
  convert -> weight pipeline per entity), `resolveOwnershipWeight`,
  `buildConsolidatedDataset` (the final per-`(Code, Period)` summation
  into a `financial.FinancialDataset`).
- **`consolidation.go`** — `Calculate(Input) Result`: input-availability
  checks (no entities, no periods, duplicate `EntityID`), entity
  selection, target-currency resolution, and orchestration of the helpers
  above.

See [`analytics/consolidation/consolidation_test.go`](analytics/consolidation/consolidation_test.go),
[`analytics/consolidation/determinism_test.go`](analytics/consolidation/determinism_test.go),
and [`analytics/consolidation/roundtrip_test.go`](analytics/consolidation/roundtrip_test.go)
for every case the task requires (two entities, eliminations on both sides
of an intercompany fee, an elimination targeting an unknown entity/an
out-of-scope period/a nonexistent line item, entities reporting different
period coverage, entities in different currencies with and without a
matching rate, ambiguous multi-currency input with no `TargetCurrency`,
explicit ownership weighting alongside a missing/invalid ownership
percentage, full consolidation deliberately ignoring `OwnershipPercent`,
every unavailable-input case (no entities, no periods, duplicate
`EntityID`, no resolvable currency), `Policy.SelectedEntityIDs` narrowing
and an unknown ID within it, no-mutation-of-caller-input, and full JSON
round-trip/determinism coverage including a map-order stress test across
15 entities/3 currencies/8 codes/3 periods).

### `transactions/acquisition`

A deterministic **acquisition screening** calculator: given a target
business's headline financials (revenue, normalized EBITDA/SDE), an
already-computed consensus valuation, an asking price, financing/deal-
structure assumptions, and optional fees/working-capital/capex/buyer-
compensation inputs, this package derives the price-multiple, financing,
coverage, and buyer-return arithmetic a buyer or advisor runs to screen a
prospective deal — **decision-support, not investment advice**. It
computes no score, no composite rating, and no buy/don't-buy verdict;
every figure is neutral arithmetic or a caller-defined threshold
comparison (see `RedFlagThresholds`).

**Dataset-independent, like `analytics/debt`/`analytics/covenants`.**
This package has no dependency on `financial.FinancialDataset` — a
target's `NormalizedEBITDA`/`NormalizedSDE` is expected to already be
computed upstream via `financial/metrics` and `financial/adjustments` (or
supplied from a broker/CIM), and `Consensus.Value` is expected to be
`valuation/consensus.Result.Statistics.WeightedMean` or `.SimpleMean` on
whatever basis the caller's `AskingPrice` is also expressed on. This
package performs no basis conversion itself (see `valuation/basis` for
that) and assumes the two are already comparable.

**Financing reuses `analytics/debt` directly**, rather than
reimplementing amortization math: `Input.Financing.DebtTranches` is a
`[]debt.LoanTerms`, one entry per funding tranche (a bank acquisition term
loan, a seller note, or both — an all-cash deal simply supplies none), and
`Calculate` derives each tranche's annual debt service via
`debt.Amortize` exactly as a caller building the same analysis directly
would. `Result.DebtSchedules` echoes each valid tranche's
`debt.AmortizationSchedule`.

**Earnings-base fallback.** Every earnings-based figure (price/EBITDA,
DSCR, leverage) prefers `Target.NormalizedEBITDA` and falls back to
`Target.NormalizedSDE` when EBITDA is unavailable — mirroring
`debt.CoverageResult.NumeratorSource`'s cash-flow/EBITDA fallback
pattern — and reports which source was used via
`CoverageResult.EarningsBaseSource`.

**Neutral premium/discount arithmetic.** `Result.Consensus.Premium`/
`PremiumPercent` is `AskingPrice - ConsensusValue` (signed): positive
means above consensus, negative means below. Neither direction is labeled
favorable or unfavorable, and
`RedFlagThresholds.MaximumPremiumToConsensusPercent` never triggers on a
discount regardless of magnitude — only a premium above the threshold
does.

**Availability, not silence.** A zero-debt (all-cash) deal reports a
known `AnnualDebtService`/`Leverage` of exactly 0 (not unavailable) — "no
debt" is a known figure of zero, not a missing one — while `DSCR` is left
unavailable (coverage of zero debt service is not a meaningful ratio),
mirroring `debt.CoverageResult.DSCR`'s identical convention. An invalid
debt tranche (negative principal/rate, non-positive amortization years,
an unrecognized frequency, or an interest-only period at least as long as
the amortization period) is excluded from every downstream calculation
and reported as a `SeverityError` `Issue`, never silently ignored or
amortized anyway.

**Scenarios.** `Input.Scenarios` is zero or more caller-defined
`ScenarioAdjustment` haircuts (revenue/EBITDA/SDE, independently, negative
values modeling an upside case), evaluated with `AskingPrice`, financing
terms, and fees held fixed — a scenario stresses the target's financial
performance, not the deal structure — mirroring
`debt.DownsideScenario`'s identical haircut convention exactly.

**Red flags are caller-driven only.** `RedFlagThresholds` defines no
default thresholds; each of its seven fields independently gates its own
flag check (DSCR, price/EBITDA, price/SDE, premium-to-consensus,
cash-on-cash return, payback period, leverage), and a zero-value
`RedFlagThresholds` skips every threshold check (recorded as advisory
`IssueNoRedFlagThresholds`) — mirroring `analytics/debt.LenderPolicy` and
`analytics/covenants`' fully caller-driven threshold model.
`FlagNegativePostDebtCashFlow` is the one flag that requires no threshold,
since a deal not cash-flowing at the base case is worth surfacing
unconditionally.

Files:

- **`types.go`** — `Value`/`Unavailable`/`AvailableValue`,
  `TargetFinancials`, `ConsensusValuation`, `TransactionFees`,
  `Financing`, `WorkingCapitalRequirement`, `CapexAssumption`,
  `BuyerCompensationAssumption`, `ScenarioAdjustment`,
  `RedFlagThresholds`, `Input`, `IssueCode`/`IssueSeverity`/`Issue`/
  `HasErrors`, `EarningsBaseSource`, `PriceMultiples`,
  `ConsensusComparison`, `SourcesAndUses`, `CoverageResult`,
  `ReturnMetrics`, `ScenarioResult`, `FlagCode`/`FlagSeverity`/`Flag`,
  `Result`, `FormulaVersion`.
- **`acquisition.go`** — `Calculate(Input) Result`: debt-tranche
  validation/amortization (via `analytics/debt`), base-case orchestration,
  and every `resolve*`/`sum*` helper.
- **`pricing.go`** — `computeMultiples`, `computeConsensusComparison`,
  `computeSourcesAndUses`, `resolveEarningsBase`, `computeCoverage`,
  `computeReturns`: the price-multiple, premium/discount, sources-and-
  uses, DSCR/leverage/post-debt-cash-flow, and cash-on-cash-return/
  payback-period formulas.
- **`scenarios.go`** — `computeScenarios`/`applyHaircut`: the downside/
  upside scenario engine, re-running `pricing.go`'s formulas against
  haircut earnings with financing held fixed.
- **`flags.go`** — `buildFlags` and one `*Flag` function per
  `RedFlagThresholds` field, plus the two unconditional/scenario-driven
  flags.

See [`transactions/acquisition/acquisition_test.go`](transactions/acquisition/acquisition_test.go),
[`transactions/acquisition/determinism_test.go`](transactions/acquisition/determinism_test.go),
and [`transactions/acquisition/roundtrip_test.go`](transactions/acquisition/roundtrip_test.go)
for every case the task requires (an all-cash deal, a leveraged deal, a
seller-note-plus-bank-debt deal, an asking price above and below
consensus value, a weak-DSCR deal, a deal with negative post-debt cash
flow, downside/upside scenario shocks, SDE fallback, capex/buyer-
compensation deductions, an invalid debt tranche, the degenerate empty-
input case, no-mutation-of-caller-input, and full JSON round-trip/
determinism coverage).

### `transactions/dealstructure`

A deterministic **acquisition financing structure** calculator: given a
purchase price, a buyer equity contribution, zero or more debt tranches,
an optional seller note, an optional deterministic earnout schedule, and
optional fees/working-capital/closing-adjustment inputs, this package
derives the deal's sources and uses, financing-percentage breakdown, and
every financing instrument's full period-by-period payment schedule —
**how the deal is financed**, never what the business is worth. It
computes no valuation, no DSCR, no leverage ratio, and no buyer return of
its own; a caller feeding this package's output onward into
`transactions/acquisition` supplies that package's own
`Financing.DebtTranches`/coverage inputs separately.

**Dataset-independent, like `transactions/acquisition`/`analytics/debt`/
`analytics/covenants`.** This package has no dependency on
`financial.FinancialDataset` — every input is a deal fact (a purchase
price, a lender's term sheet, a negotiated seller-note rate, an
already-agreed earnout schedule) with no financial-statement-traversal
representation.

**Its own amortization engine, not `analytics/debt.Amortize`.** That
function derives only a first-year and steady-state annual debt service
figure, never a full period-by-period schedule, and `debt.LoanTerms` has
no balloon field — both of which this package's brief explicitly
requires ("payment schedule," "interest/principal by period," "balloon
if supplied"). Rather than force a balloon-capable, full-schedule
amortizer into an awkward shared dependency with a package whose
`LoanTerms`/`Value` types were not designed for it, `amortize` (in
`amortization.go`) implements the same standard level-payment formula
`analytics/debt.Amortize` already uses, extended with a full
`[]PeriodDebtService` schedule and balloon-payment sizing. `DebtTranche`
and `SellerNote.Terms` share this identical engine, since a seller note
is amortized exactly like any other debt tranche — it is only tracked
separately in `Result` because a caller's sources-and-uses and
financing-percentage breakdown always distinguish seller financing from
third-party debt.

**`TermYears` vs. `AmortizationYears`, and how a balloon arises.** A
tranche's payment is always sized against `AmortizationYears` (the
schedule shape), but the tranche's actual life is `TermYears` — which
may be shorter. Three distinct situations all resolve to the same
underlying event in the schedule's final period ("the tranche comes due
with some balance still remaining, so that balance is paid as a
balloon," per `PeriodDebtService.Balloon`): an explicit
`BalloonAmount` sized directly into the payment formula (the payment is
solved so the balance reaches exactly `BalloonAmount` at `TermYears`);
`TermYears < AmortizationYears` with `BalloonAmount` unset (the loan
simply comes due with whatever balance its normal `AmortizationYears`-
sized payment naturally leaves at that point — an implicit, unsized
balloon); and a tranche whose entire `TermYears` falls inside its own
`InterestOnlyYears` (e.g. a 2-year interest-only bridge loan due in full
at year 2, where no amortization ever occurs and the full original
principal comes due as a balloon at maturity). `validateTrancheTerms`
rejects a `BalloonAmount` that exceeds what the loan's own amortization
schedule would leave outstanding at `TermYears` (`balloonCeiling`) —
a balloon larger than the loan's own remaining balance is not a coherent
input.

**Earnout is deterministic-schedule-only.** `Earnout.Payments` is a set
of already-agreed `EarnoutPayment` entries (period + amount), never a
formula this package evaluates against future performance — per the task
brief ("earnout if deterministic schedule supplied"). A contingent,
performance-based earnout is out of scope, since evaluating one would
require forecasting the target's future performance, a concern this
package does not take on. `Result.EarnoutSchedule` sorts by
`PeriodNumber` (ties broken by input order) regardless of the order
supplied.

**Availability, not silence.** `TotalDebtFinancing`/`SellerNoteAmount`/
`TotalEarnoutAmount` are known-zero (not unavailable) once `Build` runs
at all — "no third-party debt"/"no seller note"/"no earnout" is a known
figure of zero, not a missing one — mirroring
`acquisition.SourcesAndUses.TotalDebtFinancing`'s identical convention.
An invalid debt tranche (negative amount/rate, non-positive
amortization/term years, `TermYears > AmortizationYears`, an
unrecognized frequency, an interest-only period at least as long as the
amortization period, or an incoherent balloon) is excluded from every
downstream calculation and reported as a `SeverityError` `Issue`, never
silently amortized anyway — the same treatment `IssueInvalidEarnoutPayment`
gives one bad earnout entry (excluded, not blocking the rest of the
schedule or `Result.Available`).

**`FundingGapOrSurplus` is signed, neutral arithmetic**, mirroring
`acquisition.ConsensusComparison.Premium`'s signed-value convention:
positive means the deal's sources exceed its uses (a surplus), negative
means a funding gap. A negative value is recorded as advisory
`IssueFundingGap` but never blocks `Result.Available` — the caller sees
the exact gap size and decides how to close it (more equity, more debt,
a lower price).

Files:

- **`types.go`** — `Value`/`Unavailable`/`AvailableValue`,
  `PaymentFrequency`, `DebtTranche`, `SellerNote`, `EarnoutPayment`,
  `Earnout`, `TransactionFees`, `WorkingCapitalContribution`,
  `ClosingAdjustments`, `Input`, `IssueCode`/`IssueSeverity`/`Issue`/
  `HasErrors`, `PeriodDebtService`, `AmortizationSchedule`,
  `AnnualDebtServicePeriod`, `SourcesAndUses`, `FinancingPercentages`,
  `Result`, `FormulaVersion`.
- **`amortization.go`** — `validateTrancheTerms`, `balloonCeiling`,
  `amortize` (the level-payment/balloon-sizing formula and the
  period-by-period declining-balance simulation), `levelPayment`,
  `buildPeriods`, `firstYearDebtService`.
- **`sourcesanduses.go`** — `computeSourcesAndUses`,
  `computeFinancingPercentages`.
- **`earnout.go`** — `buildEarnoutSchedule`: validation, exclusion of
  invalid entries, and the `PeriodNumber` sort.
- **`annualdebtservice.go`** — `aggregateAnnualDebtService`: sums
  principal/interest/balloon across every tranche (of possibly differing
  frequencies and term lengths) year by year.
- **`dealstructure.go`** — `Build(Input) Result`: tranche/seller-note
  validation and amortization, sources-and-uses/financing-percentage
  orchestration, and annual-debt-service aggregation.

See [`transactions/dealstructure/dealstructure_test.go`](transactions/dealstructure/dealstructure_test.go),
[`transactions/dealstructure/determinism_test.go`](transactions/dealstructure/determinism_test.go),
and [`transactions/dealstructure/roundtrip_test.go`](transactions/dealstructure/roundtrip_test.go)
for every case the task requires (a single loan, multiple tranches of
differing rates/frequencies/terms, a seller note, an explicit balloon, a
`TermYears`-driven implicit balloon, an interest-only period, an
interest-only period spanning the tranche's entire term, a funding gap, a
funding surplus, every kind of invalid tranche input, a suspicious
(likely-whole-number-percent) interest rate, an earnout schedule with an
invalid entry, a zero-interest-rate tranche, closing adjustments, fees,
working capital, financing supplied with no purchase price, the
degenerate empty-input case, no-mutation-of-caller-input (including the
earnout-sort not mutating the caller's slice in place), and full JSON
round-trip/determinism coverage for both `Input` and `Result`).

### `transactions/salereadiness`

A deterministic **sale-readiness assessment**: given whichever of a
business's already-computed financial/analytics results a caller has on
hand — `analytics/qoe`, `analytics/workingcapital`,
`analytics/concentration`, `analytics/revenuequality`,
`valuation/consensus`, `financial/metrics` — plus a caller-supplied
`valuation/profile.Profile` and this package's own `DataQuality` read,
`Calculate` classifies 11 fixed dimensions (financial record quality,
earnings stability, normalization burden, customer concentration,
recurring revenue, owner dependence, margin trend, working-capital
stability, debt/leverage, data completeness, valuation-method consensus),
derives blockers/risks/strengths/missing-information/opportunities, and
computes an optional overall heuristic score. It answers "what would a
buyer's diligence likely flag, and what is still missing to know," never
"will this business sell" — **it makes no guarantee of a sale outcome**.

**Every input is optional; a missing module narrows coverage, never
scores zero.** This is the package's central design rule (directly from
its own prompt brief). Each of the 11 dimensions is classified
independently from whichever inputs are actually present; an absent
input classifies that dimension `StatusUnassessed` — listed in
`Result.MissingInformation` and excluded from both `Result.Coverage` and
`computeOverallScore`'s divisor — rather than being scored as though it
were a `StatusConcerning` finding. `Result.Coverage.AssessedDimensions`
lets a caller see at a glance how much of the full 11-dimension surface
was actually reached.

**No new figures computed from a dataset.** Unlike most `analytics/`
packages, this one performs no traversal of `financial.FinancialDataset`,
`financial/adjustments`, or a customer ledger itself — it is a pure
aggregation layer that reads already-computed `Result`s from five sibling
packages plus a business profile, mirroring `analytics/valuedrivers`' and
`transactions/acquisition`'s identical "compose, don't recompute" design.
`Input.Dataset`/`Input.Metrics` are read only for period counts and the
most recent `metrics.Snapshot` (via `mostRecentSnapshot`, itself modeled
on `analytics/qoe.chronologicalPeriods`' exact
`FiscalYear`/`Type`/`SequenceInYear` tie-break rule).

**Status, not a per-dimension score.** Each dimension classifies to one
of `StatusStrong`/`StatusAcceptable`/`StatusWeak`/`StatusConcerning`/
`StatusUnassessed` — a structured, matchable classification (mirroring
`workingcapital.TrendDirection`/`review.ReadinessState`'s identical
"never inferred from free text" discipline), not an invented per-
dimension numeric formula. Every threshold-graded dimension reads its
threshold from caller-supplied `Policy` (e.g.
`MaxAcceptableLargestCustomerShare`); a zero-value threshold means "not
specified," and the dimension still classifies (as `StatusAcceptable`)
from the raw figure alone rather than silently applying an invented
default — the same fully caller-driven threshold model
`analytics/debt.LenderPolicy`/`transactions/acquisition.RedFlagThresholds`
already established.

**The one explicit overall score.** `Result.OverallScore` is computed
only when at least one dimension was assessed (Prompt 31: "overall
readiness score only if formula is explicit"). The formula
(`computeOverallScore`, `score.go`) is fixed and simple: each assessed
dimension contributes a fixed point value by its `Status` (`Strong`=10,
`Acceptable`=7, `Weak`=3, `Concerning`=0), averaged over the number of
dimensions actually assessed and scaled to `[0, 100]` — never diluted by
`StatusUnassessed` entries, which contribute to neither the numerator nor
the divisor. This mirrors `qoe.Score`'s identical "never introduces a new
threshold of its own, only weights already-computed classifications"
design.

**Blockers vs. risks.** A `StatusConcerning` classification on one of a
fixed, documented subset of dimensions (`blockingDimensions` in
`findings.go`: financial record quality, earnings stability, customer
concentration, owner dependence, debt/leverage — the dimensions that
reflect a fact about the business rather than analysis coverage) produces
a `Blocker`; every other `StatusConcerning`/`StatusWeak` classification
produces a `Risk`. Negative maintainable/reported EBITDA additionally
triggers a fixed structural `Blocker`
(`negativeEarningsBlocker`) independent of any single dimension's
threshold — a business that is not currently profitable is a blocking
fact regardless of `Policy`.

**Opportunities are actions, never promises.** Every `Opportunity` is
phrased as "do X" (e.g. "Commission a review or audit of the financial
statements...") mechanically derived from a `StatusWeak`/
`StatusConcerning`/`StatusUnassessed` dimension — never a subjective
judgment about how the finding would affect a sale, consistent with the
package's no-guaranteed-sale disclaimer.

Files:

- **`types.go`** — `Value`/`Unavailable`/`AvailableValue`, `DataQuality`,
  `Policy`, `Input`, `DimensionCode`/`dimensionOrder`, `Status`,
  `Dimension`, `Severity`, `Blocker`, `Risk`, `Strength`,
  `MissingInformation`, `OpportunityCode`/`Opportunity`, `Coverage`,
  `IssueSeverity`/`IssueCode`/`Issue`/`HasErrors`, `Score`/
  `ScoreComponent`, `Result`, `FormulaVersion`, `ScoreVersion`.
- **`helpers.go`** — `chronologicalSnapshots`/`mostRecentSnapshot`: the
  `metrics.Snapshot` chronological-ordering helper, mirroring
  `analytics/qoe.chronologicalPeriods`.
- **`dimensions.go`** — one `classify*` function per `DimensionCode`,
  plus `buildDimensions`.
- **`findings.go`** — `buildFindings` (Blockers/Risks/Strengths/
  MissingInformation/Opportunities derivation) and
  `negativeEarningsBlocker`.
- **`score.go`** — `computeOverallScore`, `labelForScore`.
- **`salereadiness.go`** — `Calculate(Input) Result`: input-availability
  issue detection, dimension/coverage/findings/score orchestration.

See [`transactions/salereadiness/salereadiness_test.go`](transactions/salereadiness/salereadiness_test.go),
[`transactions/salereadiness/determinism_test.go`](transactions/salereadiness/determinism_test.go),
and [`transactions/salereadiness/roundtrip_test.go`](transactions/salereadiness/roundtrip_test.go)
for every case the task requires (a ready/stable business, an
owner-dependent business, a concentrated customer base, poor financial
records, missing analytical modules, negative earnings, no-Policy
threshold fallback, the degenerate empty-input case,
no-mutation-of-caller-input, and full JSON round-trip/determinism
coverage).

### `portfolio/diagnostics`

A deterministic **accountant client-portfolio scan**: given a slice of
`BusinessSnapshot` (one per business/period, each a condensed summary of
whichever sibling analytics packages a caller has already run —
`analytics/qoe`, `analytics/ratios`, `analytics/concentration`,
`analytics/cashflow`, `valuation/consensus`, `transactions/salereadiness`
— plus a small caller-chosen `SelectedMetrics` block and an optional
`Prior` snapshot), `Calculate` scans the whole book and returns a ranked
list of `Finding`s pointing at accounts needing attention or advisory
follow-up. It answers "which clients in this book need a look, and why,"
never "should we pitch this client anything" — **findings state facts,
never sales pitches**.

**One level above `transactions/salereadiness`, not a duplicate of it.**
Where salereadiness assesses *one* business across 11 fixed dimensions
from several full sibling `Result`s, diagnostics assesses *many*
businesses from small condensed summaries of those same sibling packages,
ranking findings across the whole portfolio rather than dimensions within
one business. `BusinessSnapshot` deliberately embeds `QoESummary`/
`RatioHealthSummary`/`ConcentrationSummary`/`CashFlowSummary`/
`ValuationSummary`/`SaleReadinessSummary` — condensed, caller-populated
structs with a handful of fields each — rather than the full sibling
`Result` types salereadiness embeds, so a multi-hundred-business portfolio
payload stays proportional to what diagnostics actually needs.

**Trend findings require a `Prior` snapshot; single-period findings do
not.** `BusinessSnapshot.Prior` is an optional pointer to that same
business's previous-period snapshot. Six of the eight `FindingCode`s
(`MARGIN_DETERIORATION`, `REVENUE_DECLINE`, `CASH_CONVERSION_WEAKENING`,
`LEVERAGE_INCREASE`, `CONCENTRATION_INCREASE`, `VALUATION_MOVEMENT`) are
primarily change-based and compare `Prior` against the current snapshot
under `Policy.MaterialChangePercent`; four of those six also fall back to
firing at `SeverityWarning` off an already-computed sibling signal alone
(e.g. `RatioHealthSummary.HasRisingLeverageSignal`) when no `Prior` is
available, so a first-period business is not silently invisible to
diagnostics. The remaining two codes (`UNRESOLVED_FINANCIAL_QUALITY`,
`SALE_READINESS_OPPORTUNITY`) are single-period reads and fire with no
`Prior` at all. `VALUATION_MOVEMENT` is the one finding that fires on a
rise as well as a decline, since a large swing either direction is itself
the signal worth a conversation.

**Missing modules narrow coverage, never invent a finding.** Every
`BusinessSnapshot` summary field is independently optional; an absent
summary simply means the `detect*` rules that depend on it cannot fire for
that business — never scored as though it were a concerning finding. A
business with every summary at its zero value produces zero findings, not
a wall of "missing data" warnings (`Result.Coverage` reports the gap
instead — see `CoverageCounts`).

**Priority score is one explicit, configurable formula.** `Finding.
PriorityScore = Policy.SeverityWeights[Severity] * (1 + |Change|)` when
`Change` is available, else just the severity weight — `score.go`'s
`computePriorityScore`, never an opaque or learned ranking.
`Policy.SeverityWeights` is fully caller-configurable (including setting a
severity's weight to exactly `0` to exclude it from ranking entirely —
honored, not silently replaced by `DefaultPolicy`); any weight left
unset in a caller's map falls back to `DefaultPolicy.SeverityWeights` via
`resolvePolicy`. `Result.Findings` is always sorted by `PriorityScore`
descending, then `FindingCode` declaration order, then `BusinessID`
ascending — a fully specified, tested tie-break chain, never left to
map/slice iteration order.

**No sales pitches.** Every `Finding.Reason` is built entirely from
already-computed fields, and `SuggestedReviewCategory` names a
conversation topic (`PROFITABILITY`, `CASH_FLOW`, `RISK`, `VALUATION`,
`FINANCIAL_QUALITY`, `TRANSACTION_READINESS`), never a specific
engagement, product, or dollar-value pitch — mirroring
`transactions/salereadiness`'s identical no-promise discipline.

Files:

- **`types.go`** — `Value`/`Unavailable`/`AvailableValue`, `Direction`,
  `SelectedMetrics`, `QoESummary`, `RatioHealthSummary`,
  `ConcentrationSummary`, `CashFlowSummary`, `ValuationSummary`,
  `SaleReadinessSummary`, `BusinessSnapshot`, `Policy`/`DefaultPolicy`/
  `resolvePolicy`, `Input`, `Severity`, `FindingCode`/`findingOrder`,
  `ReviewCategory`, `Finding`, `CoverageCounts`, `PortfolioCounts`,
  `IssueSeverity`/`IssueCode`/`Issue`/`HasErrors`, `Result`,
  `FormulaVersion`, `ScoreVersion`.
- **`findings.go`** — `percentChange`/`changeDirection`/
  `severityForDirection` (shared change-magnitude/materiality/severity
  helpers), one `detect*` function per `FindingCode`, `detectFindings`.
- **`score.go`** — `computePriorityScore`, `sortFindings`.
- **`diagnostics.go`** — `Calculate(Input) Result`: business-ID
  validation/dedup, per-business finding detection, ranking, portfolio-
  level counts, coverage summary.

See [`portfolio/diagnostics/diagnostics_test.go`](portfolio/diagnostics/diagnostics_test.go),
[`portfolio/diagnostics/determinism_test.go`](portfolio/diagnostics/determinism_test.go),
and [`portfolio/diagnostics/roundtrip_test.go`](portfolio/diagnostics/roundtrip_test.go)
for every case the task requires (a multi-client portfolio, missing
modules/entirely empty snapshots, equal-severity/deterministic tie-break
ordering, trend changes on every finding code, a signal-only no-`Prior`
path, custom `Policy` thresholds/weights including an explicit zero
weight, no-mutation-of-caller-input, and full JSON round-trip/determinism/
concurrency coverage).

### `reporting/management`

A deterministic **management-reporting data pack**: given whichever of up
to 13 optional already-computed sibling `Result`s a caller has on hand
(`financial/metrics`, `analytics/ratios`, `analytics/cashflow`,
`analytics/workingcapital`, `analytics/qoe`, `analytics/variance`,
`analytics/forecast`, `analytics/anomalies`, `analytics/concentration`,
`analytics/revenuequality`, `analytics/debt`, `analytics/covenants`,
`valuation/consensus`), `Calculate` reshapes them into a fixed, JSON-safe
`Report` — an executive KPI summary, historical/profitability/
liquidity-leverage/cash-flow/working-capital series, variance and forecast
tables, a merged top-issues rollup, and chart-ready series — for a caller
to render however it chooses. **It renders nothing itself**: no PDF, no
HTML, no charts, and no narrative text of any kind, AI-generated or
otherwise — every string field is either a caller-supplied label passed
through unchanged or a short, fixed-form template built entirely from
already-computed fields.

**A pure reshaping layer, computing no figure of its own.** Every number
in `Report` is read directly from an already-computed sibling `Result`;
`management` never calls `metrics.Calculate`/`ratios.Calculate`/etc. on a
caller's behalf except one narrow, explicitly documented case:
`buildHistoricalSeries` recomputes `metrics.Calculate` itself when
`Input.Metrics.Snapshots` is empty but `Input.Dataset` carries items — the
same "cheap, pure, no I/O" self-sufficiency choice `analytics/qoe`/
`analytics/cashflow`/`analytics/ratios` already make for their own
per-period figures, so a caller supplying only a raw
`financial.FinancialDataset` still gets a historical series rather than
silence.

**Fixed sections, not scored dimensions or ranked findings.** Unlike
`transactions/salereadiness` (fixed dimension statuses) or
`portfolio/diagnostics` (priority-ranked findings), `Report`'s ten
sections are identified by `SectionCode` and always populated in
`sectionOrder`, one typed field of content per section — mirroring
`valuation/orchestrator.Run.Methods`'s "fixed order regardless of
availability" rule. Each section independently reports its own
`Available`, so a caller can range over `Report`'s fields without special-
casing which sibling inputs happened to be supplied.

**Profitability prefers ratios, falls back to metrics.** `ProfitabilitySeries`
sources every margin from `Input.Ratios.History`
(`ReturnOnAssets`/`ReturnOnEquity` included) when `Ratios` is available,
and falls back to the `GrossMargin`/`EBITDAMargin` figures
`financial/metrics.Snapshot` already computes — reading them from this
package's own already-built `HistoricalSeries` rather than re-deriving a
margin by division, so the fallback never drifts from
`financial/metrics`'s own formula. `LiquidityLeverageSeries` has no such
fallback: `analytics/debt`'s coverage figures are single point-in-time
results with no `Period` of their own, so there is no per-period row to
join a `Debt`-sourced leverage figure into (`Debt` still contributes to
`TopIssues` via `Debt.Flags`).

**Top issues are merged, never re-detected.** `TopIssues` translates
already-detected findings from `Input.Anomalies`, `Input.QoE.Flags`,
`Input.Debt.Flags`, and breached/near-breached `Input.Covenants.Tests`
entries onto this package's own three-level `TopIssueSeverity` scale
(a lossless mapping in every case — all four source packages already use
an identical info/warning/critical scale), sorted by severity then by
source declaration order (anomalies, qoe, debt, covenants) then by each
source's own order — mirroring `analytics/anomalies.Summary`'s
by-rule/by-severity rollup shape for `TopIssuesSummary`.

**Chart series span historical and forecast under one label.** `ChartSeries`
for Total Revenue and EBITDA append a forecast tail from
`Input.Forecast`'s first `ScenarioResults` entry when available, each
point flagged `IsForecast` so a caller can render the projected portion
distinctly (e.g. a dashed line) without re-deriving which points are
projected from the X-axis label alone. `Source` reflects exactly which
inputs actually contributed points (`"metrics"`, `"forecast"`, or
`"metrics+forecast"`) — never a fixed label independent of what was
actually available.

**Coverage counts modules, not dimensions.** `Coverage` has one `WithX`
field per of the 12 section-backing sibling packages (mirroring
`portfolio/diagnostics.CoverageCounts`'s per-sibling-package shape more
than `transactions/salereadiness.Coverage`'s abstract-dimension-count
shape, since `management`'s sections map 1:1 to named sibling packages).
`Consensus` is tracked (`WithConsensus`) but excluded from
`TotalModules`/`CoveragePercent`, since it contributes only a handful of
`ExecutiveSummary` KPIs rather than backing an entire section the way the
other twelve do. `ModuleVersions` echoes every contributing sibling's own
`FormulaVersion` (or `SemanticsVersion`) alongside `management`'s own, so
a persisted `Report` remains self-describing about exactly which upstream
formula versions produced it.

Files:

- **`types.go`** — `FormulaVersion`, `Value`/`Unavailable`/`AvailableValue`,
  `Input`, `IssueSeverity`/`IssueCode`/`Issue`/`HasErrors`, `SectionCode`/
  `sectionOrder`, `KPI`, `Unit`, `ExecutiveSummary`.
- **`series_types.go`** — `HistoricalPeriod`/`HistoricalSeries`,
  `ProfitabilityPeriod`/`ProfitabilitySeries`,
  `LiquidityLeveragePeriod`/`LiquidityLeverageSeries`,
  `CashFlowPeriod`/`CashFlowSeries`,
  `WorkingCapitalPeriod`/`WorkingCapitalSeries`.
- **`table_types.go`** — `VarianceLine`/`VarianceTable`/`VarianceTables`,
  `ForecastPeriod`/`ForecastScenarioTable`/`ForecastTables`.
- **`issue_types.go`** — `TopIssueSeverity`, `TopIssueSource`, `TopIssue`,
  `TopIssuesSummary`, `TopIssues`.
- **`chart_types.go`** — `ChartPoint`, `ChartSeries`, `ChartSeriesSection`.
- **`coverage_types.go`** — `Coverage`, `ModuleVersion`, `ModuleVersions`,
  `Report`.
- **`management.go`** — `Calculate(Input) Report`: input-availability
  checks, section assembly in `sectionOrder`, coverage/version computation.
- **`series.go`** — `buildHistoricalSeries` (with the `Dataset` fallback),
  `buildProfitabilitySeries`, `buildLiquidityLeverageSeries`,
  `buildCashFlowSeries`, `buildWorkingCapitalSeries`.
- **`tables.go`** — `buildVarianceTables`, `buildForecastTables`.
- **`issues.go`** — `buildTopIssues`, one `*SeverityToTopIssue` translation
  function per contributing sibling package.
- **`kpi.go`** — `buildExecutiveSummary`, `kpiFromValue`, `periodIsAfter`
  (a best-effort chronological comparison across several independently-
  sourced KPI series, exact when `Input.PeriodMeta` covers every candidate
  period).
- **`chart.go`** — `buildChartSeries`, `chartFromHistorical` (with an
  explicit `forecastExtract` parameter per series, never a string switch
  on the display label), `chartFromHistoricalOnly`.
- **`coverage.go`** — `buildCoverage`, `buildModuleVersions`.
- **`helpers.go`** — `orderedSnapshots` (this package's own copy of the
  `analytics/ratios.chronologicalPeriods`-style all-or-nothing
  fallback-plus-warning rule, since `financial/metrics.Result.Snapshots`
  is not guaranteed chronologically ordered regardless of whether
  `PeriodMeta` was supplied to that call).

See [`reporting/management/management_test.go`](reporting/management/management_test.go),
[`reporting/management/determinism_test.go`](reporting/management/determinism_test.go),
and [`reporting/management/roundtrip_test.go`](reporting/management/roundtrip_test.go)
for every case the task requires (the full 13-module fixture, partial
modules, entirely missing data, the `Dataset`-only historical-series
fallback, the profitability ratios-vs-metrics fallback, chart-series
forecast tails and their source-label attribution, top-issues merging/
sorting across all four contributing sources, coverage/module-version
counts, no-mutation-of-caller-input, and full JSON round-trip/determinism/
concurrency coverage).

### `analytics/diagnostics`

A deterministic **single-business diagnostic engine**: given up to fifteen
optional already-computed sibling results (`financial/metrics`,
`analytics/ratios`, `analytics/qoe`, `analytics/workingcapital`,
`analytics/cashflow`, `analytics/revenuequality`,
`analytics/concentration`, `analytics/anomalies`, `analytics/variance`,
`analytics/forecast`, `analytics/debt`, `analytics/covenants`,
`analytics/benchmarks`, `analytics/valuedrivers`,
`transactions/salereadiness`), `Calculate` mines whichever are available
for their own already-computed Flags/Signals/Anomalies/Status
classifications and republishes them as categorized `Finding`s. **This is
not an AI narrative layer and computes no financial figure of its own** —
every `Finding` traces back to a specific field on a specific sibling
`Result` (a Flag/Signal/Anomaly `Code`, a covenant breach, an unfavorable
benchmark comparison), never a generated summary.

**A different altitude from both `transactions/salereadiness` and
`portfolio/diagnostics`.** Where salereadiness classifies eleven fixed
sale-readiness dimensions specifically and `portfolio/diagnostics` scans
*many* businesses from condensed per-business summaries and ranks
change-based findings across a whole book, `analytics/diagnostics` is
single-business and mines the full depth of whichever sibling `Result`s a
caller has on hand across twelve fixed diagnostic categories
(profitability, growth, liquidity, leverage, cash conversion, working
capital, revenue quality, concentration, earnings quality, operational
cost control, valuation, transaction readiness) — the broadest-coverage,
most granular of the three.

**Nine of fifteen sibling packages have a genuine Flag/Signal/Anomaly list
to mine directly** (`analytics/qoe`, `analytics/ratios`,
`analytics/cashflow`, `analytics/revenuequality`,
`analytics/concentration`, `analytics/anomalies`, `analytics/debt`, plus
`analytics/covenants`' breach/near-breach `Status` and
`analytics/benchmarks`' `Favorable` classification counted here since both
serve the identical role) — `mine_flags.go`'s `genericFlag`/`mapFlag`
translates the five packages sharing the exact `{Code, Severity, Period,
Message, Value, Threshold}` Flag shape (`qoe`, `cashflow`,
`revenuequality`, `concentration`, `debt`) through one shared function
rather than five near-identical copies, since that shape is a genuine
repository-wide convention, not a coincidence this package invented. The
remaining six packages (`financial/metrics`, `analytics/workingcapital`,
`analytics/variance`, `analytics/forecast`, `analytics/valuedrivers`,
`transactions/salereadiness`) have no Flag/Signal of their own, so
`mine_metrics.go`/`mine_status.go`/`mine_salereadiness.go` mine their
`Trend.Direction`/`MaterialExceptions`/`Status`/`Opportunity` fields
instead, each behind its own fixed reference threshold where no
Policy-configurable one exists upstream (documented as part of
`FormulaVersion`).

**`Finding.Code` is a normalized taxonomy, `Finding.SourceCode` is full
traceability.** Several sibling codes from different modules can map to
the same `FindingCode` when they describe the same business-level
observation from different angles (e.g. `ratios.SignalMarginCompression`
and `qoe.FlagInconsistentMargins` both map to `FindingMarginPressure`), so
a caller filtering by `FindingCode` sees one coherent signal regardless of
which module(s) corroborate it — while `SourceCode` always echoes the
exact origin Flag/Signal/RuleCode/Status/OpportunityCode string for full
traceability back to the specific field that produced it. This package
never suppresses or reconciles a contradiction between two sibling
sources (e.g. `analytics/ratios` reporting improving profitability the
same period `analytics/qoe` reports a critical earnings-quality flag) —
both `Finding`s survive independently; this is a mining layer, not a
narrative one that resolves disagreement.

**Strengths/Concerns/Opportunities is a `Severity`-driven split, not a
separate mining pass.** Every mined `Finding` is classified into exactly
one of `Result.Strengths` (`SeverityInfo`, from an explicitly positive
sibling classification like `ratios.SignalImprovingProfitability` or a
`salereadiness.Strength` — absence of a Concern is never itself treated as
a Strength), `Result.Concerns` (`SeverityWarning`/`SeverityCritical`), or
`Result.Opportunities` (currently `salereadiness.Opportunity` only, always
an improvement-area framing regardless of severity, echoing
salereadiness's own no-guaranteed-outcome discipline).

**`Coverage` mirrors `reporting/management`'s per-sibling-module shape**
(`TotalModules`/`AvailableModules`/`CoveragePercent` plus one `WithX` bool
per module and a `MissingModules` list in `Input`'s own field order) rather
than `transactions/salereadiness.Coverage`'s abstract-dimension-count
shape, since this package's Findings map to named sibling modules, not
classified dimensions. `Result.MissingDataAreas` is the structured
counterpart naming which diagnostic `Categories` each missing module would
have contributed to.

**`OverallHealthScore` is one explicit, optional formula** — `score.go`'s
`computeHealthScore` starts every category with at least one available
module at a neutral baseline, pulls it toward a Strength ceiling or a
Concern floor (a critical Concern outweighs a warning one; multiple
Concerns of the same severity in one category do not compound the
penalty), and averages only across categories actually backed by an
available module. `nil` whenever zero modules are available, or when the
caller-configurable `Policy.MinCoveragePercentForScore` is set and not
met — a Result with too little coverage has nothing meaningful for a
formula to weight, never silently substituted.

Files:

- **`types.go`** — `Value`/`Unavailable`/`AvailableValue`, `Severity`,
  `Category`/`categoryOrder`, `SourceModule`, `FindingCode`/
  `findingSortOrder`, `Finding`, `Policy`/`DefaultPolicy`, `Input`,
  `IssueSeverity`/`IssueCode`/`Issue`/`HasErrors`, `Coverage`,
  `MissingDataArea`, `Score`/`ScoreComponent`, `Result`, `FormulaVersion`,
  `ScoreVersion`.
- **`mine_flags.go`** — `genericFlag`/`flagMapping`/`mapFlag` (the shared
  Flag-to-Finding translator), `thresholdValue`/`comparisonLabelForValue`,
  `mineQoE`/`mineCashFlow`/`mineRevenueQuality`/`mineConcentration`/
  `mineDebt` and their five `*FlagTable`s.
- **`mine_ratios.go`** — `mineRatios`/`ratioSignalTable` (the one Flag-
  shaped source with an explicitly positive signal code).
- **`mine_anomalies.go`** — `mineAnomalies`/`anomalyRuleTable`.
- **`mine_status.go`** — `mineWorkingCapital`/`mineVariance`/
  `mineCovenants`/`mineBenchmarks`/`mineValueDrivers` (the five
  no-Flag/no-Signal sources, each mined from its own Status/Trend/
  MaterialException/Favorable classification instead).
- **`mine_metrics.go`** — `mineMetrics` (fallback-only when `QoE` is
  unavailable, so the same EBITDA-volatility observation is never
  double-reported from both sources).
- **`mine_salereadiness.go`** — `mineSaleReadiness`/
  `categoryForDimension` (Blockers/Risks/Strengths/Opportunities, the sole
  source of `CategoryTransactionReadiness` findings).
- **`coverage.go`** — `moduleCategories`/`moduleOrder`/`moduleAvailable`,
  `buildCoverage`, `buildMissingDataAreas`.
- **`score.go`** — `computeHealthScore`/`categoriesWithAvailableModule`,
  `labelForHealthScore`.
- **`diagnostics.go`** — `Calculate(Input) Result`: orchestrates every
  `mine*` function, splits into Strengths/Concerns/Opportunities,
  `sortFindings`.

See [`analytics/diagnostics/diagnostics_test.go`](analytics/diagnostics/diagnostics_test.go),
[`analytics/diagnostics/determinism_test.go`](analytics/diagnostics/determinism_test.go),
and [`analytics/diagnostics/roundtrip_test.go`](analytics/diagnostics/roundtrip_test.go)
for every case the task requires (a healthy business, a stressed business
across most of the twelve categories, partial single-module input,
contradictory signals from two sibling sources surviving independently,
an unrecognized future Flag/RuleCode ignored rather than crashing, the
`MinCoveragePercentForScore` gate, deterministic Category/Severity/
FindingCode-ordered output regardless of `mine*` append order, no-map-
order-dependence across twenty runs, no-mutation-of-caller-input, and full
JSON round-trip/determinism/race coverage).

### `valuation/e2e`

Not a reusable package — a single end-to-end deterministic fixture test
([`valuation/e2e/e2e_test.go`](valuation/e2e/e2e_test.go)) exercising the
**entire pipeline**, front to back, for one realistic small owner-operated
HVAC service business:

```
normalized financial data
  → financial/metrics
  → financial/adjustments (normalized EBITDA/SDE bridges)
  → financial/earnings (maintainable earnings)
  → individual valuation methods (sde, ebitda, capitalization, dcf, netassets)
  → valuation/applicability
  → valuation/orchestrator
  → valuation/consensus
  → valuation/sensitivity
  → valuation/report
```

using the same `fixtures/normalized_hvac_multi_year.json`,
`fixtures/adjustments_by_business_type.json`, and
`fixtures/valuation_by_business_type.json` fixtures every other package's
own tests already use — no new fixture data was invented for this test.
No database, no AI, no PDF, no network I/O anywhere in the chain; every
stage is a pure function over the previous stage's output, and the test
asserts on intermediate results at every stage, not only the final
`report.Report`.

This package exists specifically because it is the one place allowed to
import nearly every package in the module at once; no other package
should need to.

### `settings`

A generic four-scope settings resolver:

```
system → account → client → valuation
```

with precedence `valuation > client > account > system`. This package has
**no concept of what an account or a client is** — those are the consuming
application's domain objects. It only knows about four ordered layers of
plain `Settings` values and combines them.

Exported surface:

- **`Settings`** — one layer's worth of values. Every numeric field
  (`SDEMultiple`, `EBITDAMultiple`, `DiscountRate`, `TerminalGrowthRate`,
  `TaxRate`, `MarketabilityDiscount`, `ControlPremium`) is a `*float64`, and
  method-enable flags (`MethodEnabled map[Method]*bool`) use `*bool`.
  Methods: `sde`, `ebitda`, `dcf`, `capitalization_of_earnings`,
  `adjusted_net_asset_value`.
- **`Resolve(system, account, client, valuation Settings) Resolution`** —
  the one exported entry point. Combines the four layers into a single
  result.
- **`Resolution`** — `Values map[string]any` plus
  `Sources map[string]settings.Scope`, recording which scope won for each
  field that was set anywhere.
- **`Float64(v float64) *float64`** and **`Bool(v bool) *bool`** —
  convenience constructors for building `Settings` values, including
  explicit zero/false overrides.

`Resolve` does not persist anything and holds no state between calls. The
`Resolution` it returns is meant to be used immediately, and is a plain,
JSON-compatible value the caller can snapshot however it likes.

#### Settings snapshot contract

`Resolution` is an immutable-style snapshot, suitable for passing directly
into a valuation run instead of re-resolving settings during individual
method calculations: `Resolve` returns a freshly allocated `Values`/
`Sources` map pair with no aliasing back to the `Settings` values (or the
`*float64`/`*bool` pointers inside them) it was built from — dereferencing
happens once, at resolution time. Changing a system/account/client/
valuation default afterward — including mutating the exact pointer a
`Settings` field held — can never retroactively alter a `Resolution`
already captured earlier; see
`TestResolve_LaterMutationOfSourceSettingsDoesNotAffectSnapshot` in
[`settings/resolver_test.go`](settings/resolver_test.go) for this under
direct (in-memory, no database) test. `Resolution.SchemaVersion` (see
[Versioning strategy](#versioning-strategy) below) identifies which
version of this package's `Values`/`Sources` shape produced a given
snapshot. `Resolution` carries no database ID, user ID, account ID, or any
other identifier belonging to a consuming application — it is pure
resolved domain data, exactly like every other snapshot type in this
repository.

#### Unset vs. explicit zero/false

This is the resolver's central guarantee: a field left `nil` means
*unset/inherit from the next lower-precedence scope that has it set*, while
a non-nil pointer to the zero value means *explicitly set to zero (or
false)* — a real, resolvable value that still participates in precedence.

```go
system := settings.Settings{
    ControlPremium: settings.Float64(0.10), // 10%
}
valuation := settings.Settings{
    ControlPremium: settings.Float64(0), // explicitly zero, NOT unset
}

res := settings.Resolve(system, settings.Settings{}, settings.Settings{}, valuation)
// res.Values["control_premium"] == 0.0
// res.Sources["control_premium"] == settings.ScopeValuation
```

Had `valuation.ControlPremium` been left `nil` instead, resolution would
have fallen through to `system`'s `0.10` instead of returning `0`. A field
left `nil` at *every* scope is simply absent from both `Values` and
`Sources` — it is never reported as a resolved zero.

The same distinction applies to `MethodEnabled`: a method key that is absent
from the map, or present with a `nil` `*bool`, is unset/inherit; a present
non-nil pointer — including one pointing to `false` — is an explicit,
resolvable value.

## Example: input → output

**Raw source row** (`financial.RawLineItem`, before classification):

```json
{
  "id": "row-42",
  "statement_type": "income_statement",
  "label": "Advertising & Promotion",
  "parent_label": "Operating Expenses",
  "values": { "2024": 35000, "2025": 42000 }
}
```

**After classification** (`financial.MappedLineItem` — produced by
`classification.Classify`/`ClassifyBatch` plus `Result.ToMappedLineItem`,
from `financial/classification`; see that package's section above):

```json
{
  "source_id": "row-42",
  "label": "Advertising & Promotion",
  "statement_type": "income_statement",
  "code": "OPEX_MARKETING",
  "status": "normal",
  "values": { "2024": 35000, "2025": 42000 }
}
```

**After `Normalize`** (`financial.FinancialDataset`) — see
[`fixtures/mapped_income_statement.json`](fixtures/mapped_income_statement.json)
and [`fixtures/normalized_dataset.json`](fixtures/normalized_dataset.json)
for a fuller worked example with two source rows aggregating into one code:

```json
{
  "code": "OPEX_MARKETING",
  "period": "2025",
  "amount": 42000,
  "sources": [
    { "row_id": "row-42", "label": "Advertising & Promotion", "period": "2025", "amount": 42000 }
  ]
}
```

The `fixtures/` directory holds the canonical versions of these examples;
[`financial/fixtures_test.go`](financial/fixtures_test.go) asserts that
running the mapped fixture through `Normalize` produces byte-for-byte the
normalized fixture, so the README and the fixtures can't silently drift from
actual behavior.

[`fixtures/raw_line_items_by_business_type.json`](fixtures/raw_line_items_by_business_type.json)
holds realistic (and deliberately messy) raw rows from four business
archetypes — an owner-operated HVAC/service business, an agency/professional
services firm, a manufacturer, and a growth-stage software company — used by
[`financial/classification/fixtures_test.go`](financial/classification/fixtures_test.go)
to assert both correct classification and correct UNKNOWN behavior on
genuinely ambiguous labels.

The same four archetypes reappear as **already-normalized, three-fiscal-year
datasets** for `financial/metrics` and `financial/reconciliation` to exercise
multi-period trend calculations and realistic reconciliation checks against:
[`fixtures/normalized_hvac_multi_year.json`](fixtures/normalized_hvac_multi_year.json),
[`fixtures/normalized_agency_multi_year.json`](fixtures/normalized_agency_multi_year.json),
[`fixtures/normalized_manufacturer_multi_year.json`](fixtures/normalized_manufacturer_multi_year.json), and
[`fixtures/normalized_saas_multi_year.json`](fixtures/normalized_saas_multi_year.json).
Every balance sheet in these four balances exactly (`Assets == Liabilities +
Equity`) by construction — owner/shareholder equity is derived as the
plug that makes the identity hold, since a hand-authored fixture is not the
place to also make readers cross-check the arithmetic themselves.

[`fixtures/normalized_inconsistent_balance_sheet.json`](fixtures/normalized_inconsistent_balance_sheet.json)
is the intentional counterexample: a single-period dataset with a
**deliberate $50,000 balance sheet imbalance**, used specifically to exercise
`CheckBalanceSheetBalances`'s `FAIL` path in
[`financial/reconciliation/fixtures_test.go`](financial/reconciliation/fixtures_test.go).

## Settings inheritance example

```go
system := settings.Settings{
    DiscountRate: settings.Float64(0.12),
}
account := settings.Settings{
    DiscountRate: settings.Float64(0.13),
    TaxRate:      settings.Float64(0.21),
}
client := settings.Settings{
    DiscountRate: settings.Float64(0.15),
}
valuation := settings.Settings{
    SDEMultiple: settings.Float64(2.75),
}

res := settings.Resolve(system, account, client, valuation)
```

produces:

```json
{
  "values": {
    "sde_multiple": 2.75,
    "discount_rate": 0.15,
    "tax_rate": 0.21
  },
  "sources": {
    "sde_multiple": "valuation",
    "discount_rate": "client",
    "tax_rate": "account"
  }
}
```

`sde_multiple` comes from `valuation` (highest precedence, and the only
scope that set it). `discount_rate` is set at every scope but `client` wins
since it's the highest-precedence scope with an explicit value.  `tax_rate`
is set only at `account`, so it wins by being the only scope that set it —
notice there is no `control_premium` or `marketability_discount` key at all,
since no scope set either of those.

See [`fixtures/settings_resolution.json`](fixtures/settings_resolution.json)
for this same scenario laid out as data, and
[`settings/resolver_test.go`](settings/resolver_test.go) for the full set of
inheritance behaviors under test.

## How this is intended to be integrated later

This repository is a staging ground, not a permanent standalone service.
The expected path is:

1. Packages here (`financial`, `financial/classification`, `settings`) get
   copied or moved wholesale into a larger Go application's module, as
   internal packages (e.g. `internal/financial`, `internal/settings`) or as
   an internal shared module.
2. The larger application supplies everything this repository deliberately
   omits: persistence (database-backed storage for aliases/rules per
   account/client/valuation and for `Settings`, snapshotting `Resolution`,
   classification `Result`s, `valuation.*` method `Result`s — see
   [Method versioning](#method-codes-and-versions) for why every method
   result is version-stamped specifically for this — plus
   `applicability.Results`, `orchestrator.Run`, `consensus.Result`, and
   `report.Report`), HTTP handlers, auth, multi-tenancy, document parsing
   (PDF/XLSX/CSV/QuickBooks), actual chart/PDF/UI rendering of a
   `report.Report`, and AI/LLM assistance. The deterministic valuation
   logic itself — which methods apply to a given business
   (`valuation/applicability`), running a selected set of methods
   (`valuation/orchestrator`), a multi-method consensus value
   (`valuation/consensus`), sensitivity analysis
   (`valuation/sensitivity`), and a presentation-neutral report shape
   (`valuation/report`) — already lives in this repository; the larger
   application consumes it rather than reimplementing it. It may also
   eventually supply a statistical or AI/LLM-based classifier that
   implements the same `[]RawLineItem` + config → `[]Result` boundary
   `classification.Classify` implements today.
3. Because every exported function here is a pure function over
   JSON-compatible Go structs, integration is expected to be closer to
   wiring than to rewriting: the consuming application constructs
   `[]RawLineItem` and a `classification.Config` (however it likes, e.g.
   loading aliases/rules from its own database) and calls
   `classification.ClassifyBatch`, converts the results to
   `[]MappedLineItem` and calls `Normalize`; it constructs four
   `settings.Settings` values (however it likes) and calls `Resolve`.

Until that integration happens, this repository should keep gaining
domain modules (classification, valuation formulas, etc.) as pure,
dependency-free Go packages following the same input/output discipline —
see [Recommended next module](#recommended-next-module) for the suggested
next step.

## Versioning strategy

Every package whose output could later be persisted by a consuming
application, and whose formulas/rules/semantics could reasonably change in
a future edit, carries an explicit version constant — so that given an old
persisted result, a future integration layer can always know which
deterministic rules produced it, without guessing from a timestamp or a
git commit. This directly extends the same guarantee the five valuation
methods' `Version`/`MethodVersion` already established (see
[Method codes and versions](#method-codes-and-versions) above: "Method
versioning is mandatory because the main application is expected to
persist historical valuations").

| Version | Constant | Scope |
|---|---|---|
| Canonical taxonomy | `financial.TaxonomyVersion` | The fixed set of `financial.Code` values and their `CodeMeta` (`financial/taxonomy.go`) |
| Ingestion schema | `ingestion.SchemaVersion`, echoed on `ingestion.Result.SchemaVersion` | The fixed `Result` shape (`Row`, `Cell`, `Metadata`, `DetectedPeriod`, `Warning`, and OCR-specific `OCRProvenance`/`OCRMetadata`) produced by every format — `csv.Parse`, `xlsx.Parse`, `ingestion/pdf` — via the shared `BuildResult` entry point (`ingestion`) |
| Classification rules | `classification.DefaultRulesVersion` | The built-in rule set `DefaultRules()` returns (`financial/classification`) — a caller's own custom `Config.Rules` versions independently |
| Metrics formulas | `metrics.FormulaVersion`, echoed on `metrics.Result.FormulaVersion` | The fixed metric formula table (`financial/metrics`) |
| Adjustment semantics | `adjustments.SemanticsVersion`, echoed on `adjustments.Result.SemanticsVersion` | The default Targets/Effect table and bridge formulas (`financial/adjustments`) |
| Valuation method versions | `sde.Version` / `ebitda.Version` / `capitalization.Version` / `dcf.Version` / `netassets.Version`, each echoed as `Result.MethodVersion` | Each method's own formula and validation rules |
| Applicability rules | `applicability.RulesVersion`, echoed on `applicability.Results.RulesVersion` | The fixed scoring rules (base score, point deltas, per-method thresholds) — versioned once per `Results` since all five methods score under the same rule set in a single `Calculate` call |
| Consensus formula | `consensus.FormulaVersion`, echoed on `consensus.Result.FormulaVersion` | The fixed statistics/dispersion formula set (`valuation/consensus`) |
| Report schema | `report.SchemaVersion`, echoed on `report.Report.SchemaVersion` | This package's own `Report` shape — distinct from any upstream package's version, which is separately echoed inside each section |
| Settings resolution schema | `settings.ResolutionSchemaVersion`, echoed on `settings.Resolution.SchemaVersion` | The `Values`/`Sources` shape `Resolve` produces |
| Review schema | `review.SchemaVersion`, echoed on `review.Plan.Version` | `ReviewItem`/`Plan`/`Decision`/`ApplyResult` shapes and the deterministic ID/severity/readiness rules that produce them (`review`) |
| QoE analysis formulas | `qoe.FormulaVersion`, echoed on `qoe.Result.FormulaVersion` | The fixed ratio formulas, `DefaultThresholds`, and flag-trigger rules (`analytics/qoe`) |
| QoE heuristic score | `qoe.ScoreVersion`, echoed on `qoe.Score.Version` | The fixed baseline/deduction/clamp/label-band formula (`analytics/qoe`) — versioned separately from `qoe.FormulaVersion` since a caller may change how flags/ratios are computed independently of how they are weighted into one composite number |
| Working-capital analysis formulas | `workingcapital.FormulaVersion`, echoed on `workingcapital.Result.FormulaVersion` | The NWC definition given an `InclusionPolicy`, the `Statistics`/`Trend`/`SeasonalProfile` formulas, and the `PegMethod` strategies (`analytics/workingcapital`) |
| Ratio analysis formulas | `ratios.FormulaVersion`, echoed on `ratios.Result.FormulaVersion` | Every profitability/liquidity/leverage/efficiency/growth ratio formula, the Total Assets/Total Equity sum-of-codes definitions, and the `RatioTrend`/`Comparison` methodology (`analytics/ratios`) |
| Ratio signal rules | `ratios.SignalRulesVersion`, echoed on `ratios.Result.SignalRulesVersion` | The fixed `DefaultThresholds` and every signal-trigger rule in `signals.go` (`analytics/ratios`) — versioned separately from `ratios.FormulaVersion` since a caller may change how ratios are computed independently of which signals are derived from them |
| Cash-flow analysis formulas | `cashflow.FormulaVersion`, echoed on `cashflow.Result.FormulaVersion` | The EBITDA-to-free-cash-flow bridge, every conversion ratio, the `RecurringDrains`/`CashRunway` formulas, the `DefaultThresholds` flag-trigger rules, and the EBITDA-based estimate method used under `Options.AllowEBITDAEstimate` (`analytics/cashflow`) |
| Revenue-quality analysis formulas | `revenuequality.FormulaVersion`, echoed on `revenuequality.Result.FormulaVersion` | The recurring/non-recurring revenue split, the `Statistics`/`Trend`/`CAGRResult`/`VolatilityResult` formulas, the customer-transition (new/lost/retained/expansion/contraction) formulas, the `ConcentrationSummary`/HHI formula, and the `DefaultThresholds` flag-trigger rules (`analytics/revenuequality`) |
| Concentration analysis formulas | `concentration.FormulaVersion`, echoed on `concentration.Result.FormulaVersion` | The top-N-share/HHI formulas, the entity-ranking and `DependencyChange` methodology, the lost-entity/top-N-loss `Scenario` formulas (including the optional earnings-impact conversion), and the `DefaultThresholds` flag-trigger rules (`analytics/concentration`) |
| Anomaly detection rules | `anomalies.FormulaVersion`, echoed on `anomalies.Result.FormulaVersion` | Every `RuleCode`'s exact comparison method (spike/variance/growth-gap/margin/materiality/gap/duplicate/sign/negative-amount detection) and the `DefaultThresholds` trigger points (`analytics/anomalies`) |
| Variance analysis formulas | `variance.FormulaVersion`, echoed on `variance.Result.FormulaVersion` | The absolute/percentage variance formulas, the favorable/unfavorable direction rules (taxonomy-category defaults, the mixed other-income-statement per-code rule, and `DirectionOverrides` precedence), the materiality test, the contribution-to-total-variance formula, the category rollup, and the period-trend formula (`analytics/variance`) |
| Forecast/scenario formulas | `forecast.FormulaVersion`, echoed on `forecast.Result.FormulaVersion` | The historical-base derivation, the per-period compounding rule for revenue/COGS/opex (aggregate-plus-override precedence, the `FixedAmount` proportional-split rule, `OpexMethodExcludeAmount`'s one-time-item exclusion), the EBIT/EBITDA/SDE/margin/tax/net-income formulas, the working-capital and cash-flow bridge, the debt-service-coverage formula, and every `Apply*` scenario-transformation helper's exact arithmetic (`analytics/forecast`) |
| Debt capacity/DSCR formulas | `debt.FormulaVersion`, echoed on `debt.Result.FormulaVersion` | The amortization/payment formula (including interest-only handling), the annual-debt-service aggregation, the DSCR/fixed-charge-coverage/leverage/interest-coverage formulas, the maximum-debt-under-DSCR and maximum-debt-under-leverage solvers, the combined-capacity (most-restrictive-constraint) rule, and the downside-scenario methodology (`analytics/debt`) |
| Covenant evaluation formulas | `covenants.FormulaVersion`, echoed on `covenants.Result.FormulaVersion` | The operator-evaluation rule, the direction-aware headroom formula, the pass/fail/unavailable classification, and the warning-buffer (near-breach) classification (`analytics/covenants`) |
| Benchmark comparison formulas | `benchmarks.FormulaVersion`, echoed on `benchmarks.Result.FormulaVersion` | The percentile-band/quartile linear-interpolation rule, the peer-observation rank-based percentile estimate, the non-decreasing-value precondition and its `IssueNonMonotonicBenchmarkPoints` guard, the band-placement rule, the difference/relative-difference formulas, and the favorable/unfavorable classification rule (`analytics/benchmarks`) |
| Value driver/scenario formulas | `valuedrivers.FormulaVersion`, echoed on `valuedrivers.Result.FormulaVersion` | Every `DriverType`'s exact per-method mutation rule (which `Input` field(s) it changes and how), the `LinkageApplied`/`LinkageNotApplicable`/`LinkageMethodExcluded` classification, the one-factor-at-a-time vs. combined-scenario compounding order, and the value/percent-delta formulas (`analytics/valuedrivers`) |
| Multi-entity consolidation formulas | `consolidation.FormulaVersion`, echoed on `consolidation.Result.FormulaVersion` | The full/ownership-weighted consolidation formulas, the eliminate-then-convert-then-weight order of operations, the target-currency resolution rule, and every reconciliation-`Issue` trigger (`analytics/consolidation`) |
| Acquisition screening formulas | `acquisition.FormulaVersion`, echoed on `acquisition.Result.FormulaVersion` | The price-to-revenue/EBITDA/SDE multiple formulas, the premium/discount-to-consensus formula, the sources-and-uses/required-equity arithmetic, the annual-debt-service/DSCR/post-debt-cash-flow formulas (via `analytics/debt.Amortize`), the cash-on-cash-return/simple-payback-period formulas, the leverage formula, the downside/upside scenario methodology, and the red-flag threshold rules (`transactions/acquisition`) |
| Deal-structure/financing formulas | `dealstructure.FormulaVersion`, echoed on `dealstructure.Result.FormulaVersion` | The sources-and-uses/required-equity/funding-gap arithmetic, this package's own per-tranche amortization formula (including interest-only handling and balloon-payment sizing — distinct from `analytics/debt.Amortize`'s formula), the seller-note and earnout schedule derivations, the financing-percentage formula, and the annual-debt-service aggregation across tranches (`transactions/dealstructure`) |
| Sale-readiness assessment formulas | `salereadiness.FormulaVersion`, echoed on `salereadiness.Result.FormulaVersion` | Every `DimensionCode`'s classification rule and Policy-threshold comparison (`dimensions.go`), the Blocker/Risk/Strength/MissingInformation/Opportunity derivation rules including `blockingDimensions` and `negativeEarningsBlocker` (`findings.go`) (`transactions/salereadiness`) |
| Sale-readiness heuristic score | `salereadiness.ScoreVersion`, echoed on `salereadiness.Score.Version` | The fixed per-`Status` point value, averaging-over-assessed-dimensions, and `[0, 100]` scaling formula (`score.go`) — versioned separately from `salereadiness.FormulaVersion` since a caller may change how dimensions are classified independently of how already-classified dimensions are weighted into one composite number (`transactions/salereadiness`) |
| AI request/response schema | `ai.RequestSchemaVersion`, echoed on `ai.Provenance.RequestSchemaVersion` | The `Request`/`Response` wire shape `financial/classification/ai` sends to/expects from a `Classifier` |
| AI fallback orchestration | `ai.OrchestrationVersion`, echoed on `ai.Provenance.OrchestrationVersion` | The trigger/fallback/safety decision logic in `ClassifyWithFallback`/`ClassifyBatchWithFallback` (which `FallbackMode` runs AI when, structural-row skipping, disagreement handling) |
| OpenAI adapter (classification) | `openai.AdapterVersion`, echoed on `ai.Provenance.AdapterVersion` | This specific provider adapter's prompt-construction/response-parsing logic (`financial/classification/ai/openai`) |
| AI adjustment-suggestion request/response schema | `ai.RequestSchemaVersion`, echoed on `ai.Provenance.RequestSchemaVersion` | The `Request`/`Response` wire shape `financial/adjustments/ai` sends to/expects from a `Suggester` — a distinct constant/package from classification's identically-named one |
| AI adjustment-suggestion orchestration | `ai.OrchestrationVersion`, echoed on `ai.Provenance.OrchestrationVersion` | The batching/validation/provenance decision logic in `SuggestAdjustments`/`SuggestAdjustmentsBatch` (`financial/adjustments/ai`) — a distinct constant/package from classification's identically-named one |
| OpenAI adapter (adjustment suggestions) | `openai.AdapterVersion`, echoed on `ai.Provenance.AdapterVersion` | This specific provider adapter's prompt-construction/response-parsing logic (`financial/adjustments/ai/openai`) |
| Management-reporting pack assembly | `management.FormulaVersion`, echoed on `management.Report.FormulaVersion` | Which sibling fields populate each `Section` (`series.go`, `tables.go`, `issues.go`, `chart.go`), the KPI-selection rule (`kpi.go`), and the coverage/version-echo computation (`coverage.go`) (`reporting/management`) |
| Business diagnostic mining rules | `diagnostics.FormulaVersion`, echoed on `diagnostics.Result.FormulaVersion` | Every `mine*` function's field-to-`Finding` mapping (`mine_flags.go`, `mine_ratios.go`, `mine_anomalies.go`, `mine_status.go`, `mine_metrics.go`, `mine_salereadiness.go`), the `Category` assignment per source, and the `Strengths`/`Concerns`/`Opportunities` split rule (`analytics/diagnostics`) |
| Business diagnostic health score | `diagnostics.ScoreVersion`, echoed on `diagnostics.Score.Version` | The per-category neutral-baseline/Strength-ceiling/Concern-floor formula and the averaging-over-available-categories rule (`score.go`) — versioned separately from `diagnostics.FormulaVersion` since a caller may change which findings are mined independently of how already-mined findings are weighted into one composite score (`analytics/diagnostics`) |
| General ledger / trial balance schema | `ledger.SchemaVersion`, echoed on `TrialBalance.SchemaVersion`/`NormalizedTrialBalance.SchemaVersion`/`IntegrityReport.SchemaVersion` | The `Account`/`JournalEntry`/`JournalLine`/`Ledger`/`Balance`/`TrialBalance`/`TrialBalanceInput`/`NormalizedTrialBalance`/`Issue` shapes and the validation/balance/trial-balance/rollup rules that produce them (`accounting/ledger`) — no separate `FormulaVersion`, since every calculation is a direct consequence of the double-entry rules `SchemaVersion` already covers; see `docs/LEDGER.md` |
| Statement builder result/schema | `statements.SchemaVersion`, echoed on `Result.SchemaVersion` | The `AccountMapping`/`Statement`/`Section`/`Row`/`Result`/`Issue` shapes (`accounting/statements`); see `docs/STATEMENT_BUILDER.md` |
| Statement builder formulas | `statements.StatementFormulaVersion`, echoed on `Result.StatementFormulaVersion` | Sign normalization (`signs.go`), structural-row/section-template assembly (`income.go`/`balance.go`), hierarchy leaf-posting policy — versioned separately from `SchemaVersion` since the calculation semantics can change independently of the result shape (`accounting/statements`) |
| Statement builder mapping contract | `statements.MappingContractVersion`, echoed on `Result.MappingContractVersion` | `AccountMapping`'s shape, mapping precedence (explicit > deterministic suggestion > unmapped), account-type safety rules (`accounttype.go`), `MappingTemplate` rule precedence (`templates.go`) — versioned separately since a mapping-precedence change does not necessarily imply a statement-formula change (`accounting/statements`) |
| AR aging schema | `ar.SchemaVersion`, echoed on `Result.SchemaVersion` | The `Receivable`/`Payment`/`BucketDefinition`/`CustomerSummary`/`Result`/`Issue` shapes (`accounting/ar`); see `docs/AR_AGING.md` |
| AR aging formulas | `ar.FormulaVersion`, echoed on `Result.FormulaVersion` | Aging-basis/bucket assignment (`buckets.go`), DSO (`dso.go`), historical trend/migration (`trends.go`), collection metrics (`collections.go`), concentration (`concentration.go`), every flag-trigger rule (`flags.go`) (`accounting/ar`) |
| AP aging schema | `ap.SchemaVersion`, echoed on `Result.SchemaVersion` | The `Payable`/`SupplierPayment`/`BucketDefinition`/`SupplierSummary`/`Result`/`Issue` shapes (`accounting/ap`); see `docs/AP_AGING.md` |
| AP aging formulas | `ap.FormulaVersion`, echoed on `Result.FormulaVersion` | Aging-basis/bucket assignment (`buckets.go`), DPO (`dpo.go`), historical trend/migration (`trends.go`), payment/terms metrics (`payments.go`), concentration (`concentration.go`), due-date schedule and payment pressure (`schedule.go`/`pressure.go`), every flag-trigger rule (`flags.go`) (`accounting/ap`) |
| Journal entry diagnostics schema | `journaldiagnostics.SchemaVersion`, echoed on `Result.SchemaVersion` | The `EntryMetadata`/`PeriodWindow`/`AccountReviewPolicy`/`Policy`/`Finding`/`Evidence`/`DuplicateGroup`/`Result`/`Issue` shapes (`accounting/journaldiagnostics`); see `docs/JOURNAL_DIAGNOSTICS.md` |
| Journal entry diagnostics formulas | `journaldiagnostics.FormulaVersion`, echoed on `Result.FormulaVersion` | Every finding-trigger rule (`manual.go`/`periodend.go`/`timing.go`/`amounts.go`/`accounts.go`/`duplicates.go`/`reversals.go`/`clustering.go`/`controls.go`), the median+MAD baseline method and duplicate-signature normalization, `DefaultPolicy`'s threshold values (`accounting/journaldiagnostics`) |
| Close quality schema | `closequality.SchemaVersion`, echoed on `Result.Versions.SchemaVersion` | The `Input`/`Policy`/`Dimension`/`DimensionResult`/`Finding`/`Evidence`/`Coverage`/`CloseTaskSummary`/`ComparisonResult`/`Result`/`Issue` shapes (`accounting/closequality`); see `docs/CLOSE_QUALITY.md` |
| Close quality formulas | `closequality.FormulaVersion`, echoed on `Result.Versions.FormulaVersion` | Every `mine_*.go` translation rule, the readiness decision table (`readiness.go`), deduplication keying (`findings.go`/`sort.go`), `DefaultPolicy`/`DefaultJournalFindingRules`'s values (`accounting/closequality`) |
| 13-week cash forecast schema | `cashforecast.SchemaVersion`, echoed on `Result.SchemaVersion` | The `CashFlowEvent`/`OpeningCash`/`CashAccount`/`RecurringRule`/`ARCollectionAssumption`/`APPaymentPlan`/`Scenario`/`CreditFacility`/`WeeklyForecast`/`ScenarioResult`/`Coverage`/`Result`/`Issue` shapes (`accounting/cashforecast`); see `docs/CASH_FORECAST_13_WEEK.md` |
| 13-week cash forecast formulas | `cashforecast.FormulaVersion`, echoed on `Result.FormulaVersion` | Week-boundary assignment (`weeks.go`), the weekly rollforward and category-breakdown aggregation (`weeklyengine.go`/`calculate.go`), AR/AP scheduling adapters (`aradapter.go`/`apadapter.go`), recurring-event generation (`recurring.go`), scenario transformation and delta-vs-base calculation (`scenario.go`/`liquidity.go`), the single-upfront funding-requirement formula, and every flag-trigger rule (`flags.go`) (`accounting/cashforecast`) |

**The rule for bumping a version:** whenever a formula, an availability/
validation rule, a default, a sign convention, or an output shape changes
in a way that could make a historical result not reproduce identically
under the new code. A cosmetic change (a display label, a doc comment, a
variable rename) never requires a bump. This is a small, coherent, flat
scheme — one constant per package with a genuinely versionable concept,
never sprinkled ad hoc, and never added merely for decoration.

## Deterministic ordering guarantees

Every collection this repository exposes to a caller has a documented,
deterministic order — none rely on Go map iteration order (the two raw
map fields that do exist, `settings.Resolution.Values`/`Sources` and
`financial/metrics.Snapshot.Results`, are both string-keyed, so
`encoding/json` sorts their keys alphabetically on marshal; both are
covered by dedicated JSON-key-order tests rather than left as an
unverified incidental property of the standard library).

| Collection | Order |
|---|---|
| `financial.FinancialDataset.Items` | Sorted by `Code`, then by `Period` (produced by `Normalize`) |
| `financial.FinancialDataset.Periods()` | Sorted lexically |
| `financial/taxonomy.AllCodes()` / `CodesByCategory()` | Sorted by `Code` string |
| `financial/metrics.Trend`'s `[]GrowthPoint`/`[]MarginPoint` series | Chronological, matching `Trend.FiscalYearsUsed` |
| `financial/classification.Result.Alternatives` | Descending confidence (strongest candidate first) |
| `financial/adjustments.Bridge.Applied` | The order adjustments were supplied to `Apply` |
| `valuation/applicability.Results.Methods` | Fixed: SDE, EBITDA, Capitalization, DCF, NetAssets |
| `valuation/orchestrator.Run.Methods` | Same fixed method order |
| `valuation/consensus.Result.Requested` / `Included` / `Conversions` | `Requested` preserves caller order exactly; `Included` and `Conversions` preserve that same order (with excluded entries removed from `Included` only) |
| `valuation/report.Report.Methods` | Same fixed method order (`methodOrder`, mirroring the orchestrator's) |
| `review.Plan.Items` | Primarily by `Severity` (`BLOCKING`, `ERROR`, `WARNING`, `INFO`), then by `Kind` (string order), then by `SourceRowID`, then by `ID` — a real `sort.SliceStable` over exactly these keys (`review`'s `sortItems`), never left to `Build`'s internal append order |
| `qoe.Result.History` | Chronological when `Input.PeriodMeta` covers every period in the dataset (by `FiscalYear`, then granularity, then `SequenceInYear`); falls back to `financial.FinancialDataset.Periods()`'s lexical order when `PeriodMeta` is nil/partial, rather than guessing a partial sort (`analytics/qoe`) |
| `qoe.AdjustmentBreakdown.ByType` / `qoe.Result.Recurrence` | Sorted by `adjustments.Type` string (`analytics/qoe`'s `buildAdjustmentBreakdown`/`buildRecurrence`) |
| `qoe.RecurrencePattern.Periods` | Chronological, matching the period order `History` was built in (never re-sorted lexically, since `financial.Period`'s string value has no guaranteed chronological order) |
| `qoe.Result.Flags` | `FlagCode` declaration order (`LARGE_NORMALIZATION_BURDEN` through `NEGATIVE_OR_NEAR_ZERO_MAINTAINABLE_EARNINGS`), then by `Period` within a code — never a severity-ranked priority queue, since a QoE flag list is read as a checklist |
| `ai.BatchOutcome.Outcomes` (`financial/classification/ai`) | Input order preserved exactly: `Outcomes[i]` always corresponds to the `i`-th row passed to `ClassifyBatchWithFallback` |
| `ai.BatchOutcome.Outcomes` (`financial/adjustments/ai`) | Input order preserved exactly: `Outcomes[i]` always corresponds to the `i`-th `Request` passed to `SuggestAdjustmentsBatch` |
| `workingcapital.Result.History` | Chronological when `Input.PeriodMeta` covers every period in the dataset (by `FiscalYear`, then granularity, then `SequenceInYear`); falls back to `financial.FinancialDataset.Periods()`'s lexical order when `PeriodMeta` is nil/partial (`analytics/workingcapital`) |
| `workingcapital.Result.ExcludedCodes` | Sorted by `Code` string |
| `workingcapital.SeasonalProfile.Periods` | Sorted by `SequenceInYear` ascending |
| `workingcapital.SuggestedPeg.PeriodsUsed` | Chronological, matching the period order `History` was built in |
| `ratios.Result.History` | Chronological when `Input.PeriodMeta` covers every period in the dataset (by `FiscalYear`, then granularity, then `SequenceInYear`); falls back to `financial.FinancialDataset.Periods()`'s lexical order when `PeriodMeta` is nil/partial (`analytics/ratios`) |
| `ratios.Result.Trends` / `Comparisons` | By `RatioXxx` metric-name declaration order; `Comparisons` then chronologically within each metric |
| `ratios.Result.Signals` | By `SignalCode` declaration order, then by `Period` |
| `cashflow.Result.History` / `Conversion` | Chronological when `Input.PeriodMeta` covers every period in the dataset (by `FiscalYear`, then granularity, then `SequenceInYear`); falls back to `financial.FinancialDataset.Periods()`'s lexical order when `PeriodMeta` is nil/partial (`analytics/cashflow`) |
| `cashflow.Result.RecurringDrains` | Fixed `RecurringDrainCategory` declaration order (`capex`, `working_capital_build`, `debt_service`, `owner_distributions`) — always all four entries, never Go map order |
| `cashflow.Result.Flags` | `FlagCode` declaration order (`WEAK_CASH_CONVERSION` through `DECLINING_CONVERSION_TREND`), then by `Period` within a code |
| `revenuequality.Result.TotalRevenueHistory` / `CustomerHistory` | Chronological when `Input.PeriodMeta` covers every period in the dataset (by `FiscalYear`, then granularity, then `SequenceInYear`); falls back to `financial.FinancialDataset.Periods()`'s lexical order when `PeriodMeta` is nil/partial (`analytics/revenuequality`) |
| `revenuequality.Result.CustomerTransitions` | Chronological, one entry per adjacent pair in the same order as `CustomerHistory` |
| `revenuequality.ConcentrationSummary.TopNShares` | Ascending by `N`, deduplicated (`Policy.ConcentrationTopN`) |
| `revenuequality.ConcentrationSummary.Segments` | Sorted by `Segment` string ascending |
| `revenuequality.Result.Flags` | `FlagCode` declaration order (`DECLINING_RECURRING_MIX` through `SHRINKING_EXISTING_CUSTOMER_BASE`), then by `Period` within a code |
| `anomalies.Result.Anomalies` | By `Period` (chronologically when `Input.PeriodMeta` covers every period, else `financial.FinancialDataset.Periods()`'s lexical order), then by `RuleCode` declaration order, then by `Account` string — a real `sort.SliceStable` (`anomalies`'s `sortAnomalies`), never left to `Calculate`'s internal per-rule append order |
| `anomalies.Summary.ByRule` / `BySeverity` | `RuleCode` declaration order / fixed `info`, `warning`, `critical` order — never Go map order (`analytics/anomalies`'s `buildSummary`) |
| `anomalies.Anomaly.RelatedPeriods` (on `REPEATED_UNUSUAL_VALUE`/`DUPLICATE_LIKE_AMOUNTS` anomalies) | Sorted by `Period` string ascending |
| `variance.Result.LineVariances` | By `Period` (chronologically when `Input.PeriodMeta` covers every period present, else lexical `Period` order), then by `AccountCode` ascending, stable on original input order for ties (`analytics/variance`'s `sortLineVariances`) |
| `variance.Result.CategorySummaries` / `CustomCategorySummaries` | Sorted by `Category` string ascending |
| `variance.Result.TopFavorable` | Descending by `AbsoluteVariance.Value` (largest favorable dollar impact first), ties broken by `AccountCode` then `Period` |
| `variance.Result.TopUnfavorable` | Ascending by `AbsoluteVariance.Value` (most negative-impact-magnitude first), ties broken by `AccountCode` then `Period` |
| `variance.Result.MaterialExceptions` | Same order as `LineVariances`, filtered to `MaterialityMaterial` |
| `variance.Result.PeriodTrends` | Chronological when `Input.PeriodMeta` covers every period present; falls back to lexical `Period` order when `PeriodMeta` is nil/partial (`analytics/variance`) |
| `forecast.Result.ScenarioResults` | Same order as `Input.Scenarios`, skipping any entry with an empty or duplicate `Name` (`analytics/forecast`) |
| `forecast.ScenarioResult.ProjectedPeriods` / `WorkingCapital` / `CashFlow` | Forecast-period-number order, `1..Input.Horizon`, always — there is no chronological-vs-lexical fallback here (unlike every dataset-bound sibling above) since a forecast period has no `financial.Period` string to order by in the first place |
| `forecast.PeriodPL.RevenueLines` / `COGSLines` / `OpexLines` | Sorted by `financial.Code` ascending (`analytics/forecast`'s `sortedCodesByAmount`), never Go map order |
| `forecast.ScenarioResult.Trace` | Calculation order: for each forecast period in sequence, one `TraceStep` per line as it was computed (Total Revenue, Total COGS, Gross Profit, Total Opex, EBIT, EBITDA, SDE, Pretax Income, Net Income) |
| `salereadiness.Result.Dimensions` | Fixed `dimensionOrder` (financial record quality, earnings stability, normalization burden, customer concentration, recurring revenue, owner dependence, margin trend, working-capital stability, debt/leverage, data completeness, valuation-method consensus) — always all 11 entries in this order whenever `Available` is true, one per `DimensionCode`, regardless of how many were actually assessed (`transactions/salereadiness`) |
| `salereadiness.Result.Blockers` / `Risks` / `Strengths` / `MissingInformation` / `Opportunities` | Each in `dimensionOrder` (the same fixed order `Dimensions` is in), since `buildFindings` ranges over `dims` once; `negativeEarningsBlocker`'s structural Blocker (when triggered) is prepended ahead of every dimension-derived Blocker (`transactions/salereadiness`) |
| `salereadiness.Score.Components` | Same `dimensionOrder` as `Result.Dimensions`, one entry per assessed (non-`StatusUnassessed`) dimension, `StatusUnassessed` entries omitted entirely (`transactions/salereadiness`) |
| `consolidation.Result.Consolidated.Items` | Sorted by `Code`, then by `Period` — the same order `financial.Normalize` produces, since `buildConsolidatedDataset` uses the identical sort (`analytics/consolidation`) |
| `consolidation.Result.Consolidated.Items[i].Sources` | Sorted by `RowID` (the contributing `EntityID`) ascending |
| `consolidation.Result.EntityContributions` | Sorted by `EntityID` ascending |
| `consolidation.EntityContribution.Items` | Sorted by `Code`, then by `Period` |
| `consolidation.Result.EliminationsApplied` | Sorted by `EntityID`, then `Code`, then `Period` |
| `consolidation.Result.CurrencyConversions` | Sorted by `EntityID`, then `Period` |
| `consolidation.Result.ReconciliationIssues` / `Warnings` / `Errors` | Sorted by `EntityID`, then `Period` — a real `sort.SliceStable`, never left to `Calculate`'s internal per-check append order (`analytics/consolidation`) |
| `management.Report`'s section fields | Fixed `sectionOrder` (executive summary, historical series, profitability series, liquidity/leverage series, cash-flow series, working-capital series, variance tables, forecast tables, top issues, chart series) — always all 10 sections populated in this order whenever `Available` is true, one typed field per `SectionCode`, each independently reporting its own availability (`reporting/management`) |
| `management.TopIssues.Issues` | Sorted by `Severity` (critical, then warning, then info), then by `Source` declaration order (anomalies, qoe, debt, covenants), then by each source's own slice order — never re-ranked by any score of this package's own (`reporting/management`) |
| `management.ExecutiveSummary.KPIs` | Fixed declaration order (revenue/gross-profit/EBITDA/net-income from metrics, EBITDA margin from profitability, current-ratio/net-debt-to-EBITDA from liquidity-leverage, free-cash-flow/conversion from cash flow, NWC from working capital, largest-customer share from concentration, recurring-revenue percent from revenue quality, indicated value from consensus); a KPI whose underlying figure is entirely unavailable is omitted, never included as an unavailable entry (`reporting/management`) |
| `management.Coverage.MissingModules` | `Input`'s own field declaration order (metrics, ratios, cash flow, working capital, QoE, variance, forecast, anomalies, concentration, revenue quality, debt, covenants) restricted to the modules with `WithX == false` (`reporting/management`) |
| `management.ModuleVersions.Modules` | Fixed order matching `Coverage`'s `WithX` field order plus `consensus` last — always all 13 entries regardless of availability, an unavailable module's `Version` is empty rather than the entry being omitted (`reporting/management`) |
| `diagnostics.Result.Findings` | By `categoryOrder` (profitability through transaction readiness), then by `Severity` descending (critical, warning, info), then by `FindingCode` declaration order, then by `Period` ascending, then by `SourceCode` ascending — a real `sort.SliceStable` (`diagnostics`'s `sortFindings`), never left to `Calculate`'s internal per-`mine*`-function append order (`analytics/diagnostics`) |
| `diagnostics.Result.Strengths` / `Concerns` / `Opportunities` | Each filtered from the already-sorted `Findings` (or, for `salereadiness`-sourced Strengths/Opportunities, separately built and appended then independently re-sorted with the same `sortFindings` rule), so every one of the three preserves `Findings`' own `categoryOrder`/`Severity`/`FindingCode` ordering (`analytics/diagnostics`) |
| `diagnostics.Coverage.MissingModules` / `Result.MissingDataAreas` | `Input`'s own field declaration order (metrics, ratios, QoE, working capital, cash flow, revenue quality, concentration, anomalies, variance, forecast, debt, covenants, benchmarks, value drivers, sale readiness) restricted to the modules with `WithX == false` (`analytics/diagnostics`) |
| `diagnostics.Score.Components` | `categoryOrder`, restricted to categories backed by at least one available module — a category with zero available backing modules is excluded entirely, never included at the neutral baseline (`analytics/diagnostics`) |

## Error taxonomy

Most packages in this repository do not return a Go `error` at all —
invalidity is communicated through `Result.Available` plus structured
`Result.Errors`/`Result.Warnings` (see each package's own section above).
Where a package does surface structured problems, it uses one of several
established, stable-code systems rather than a caller having to parse
message strings:

- **`valuation.Issue{Code valuation.IssueCode, Severity, Message}`** —
  shared by every one of the five valuation methods (each defining its
  own method-specific `IssueCode` constants, e.g. `sde.IssueNonPositiveMultiple`)
  and by `valuation/consensus`. The common codes every package can draw
  from: `IssueInvalidInput`, `IssueMissingRequiredData`,
  `IssueIncompatibleValueBasis`, `IssueInvalidRate`, `IssueInvalidWeight`,
  `IssueUnavailableMetric` — plus a handful backing
  `valuation.ValidateResultEnvelope`/`ValidateFiniteSteps`'s own invariant
  checks (`IssueEmptyMethodCode`, `IssueEmptyVersion`,
  `IssueUnknownValueBasis`, `IssueNonFiniteStep`,
  `IssueDuplicateMethodResult`).
- **`adjustments.Issue{Code adjustments.IssueCode, Severity, Message}`** —
  `financial/adjustments`' own separate, independently well-formed system
  (`IssueMissingID`, `IssueDuplicateID`, `IssueAmbiguousEffect`, etc.),
  intentionally not merged into `valuation.Issue`: the two packages'
  problem domains don't overlap, and forcing one type to serve both would
  either leak valuation-specific codes into `financial` or vice versa.
- **`review.Issue{ItemID, Code review.IssueCode, Severity, Message}`** —
  `review`'s own separate, independently well-formed system
  (`IssueUnknownItemID`, `IssueWrongPayloadKind`, `IssueInvalidAction`,
  `IssueMissingPayload`, `IssueInvalidCode`, `IssueInvalidRowKind`,
  `IssueNonFiniteAmount`, `IssueMalformedPeriod`, `IssueDuplicateItemID`,
  `IssueConflictingDecision`, `IssueIncompatibleDecision`,
  `IssueStructuralRowOverride`), a third system
  rather than reusing either of the two above: `review`'s problem domain
  (unknown review-item IDs, wrong decision-payload-for-item-kind,
  conflicting duplicate decisions) doesn't overlap with `valuation`'s
  (rate/weight/value-basis validity) or `adjustments`' (adjustment-set
  internal consistency) domain, and forcing one of those types to also
  serve `review` would either leak review-specific codes into
  `financial`/`valuation` or vice versa — the same reasoning above applied
  a third time.
- **`qoe.Issue{Code qoe.IssueCode, Severity, Message}`** — `analytics/qoe`'s
  own separate system (`NO_PERIODS`, `NO_PERIOD_META`,
  `MAINTAINABLE_EBITDA_UNAVAILABLE`, `MAINTAINABLE_SDE_UNAVAILABLE`), a
  fourth system: an input-level QoE-analysis problem ("no periods in
  dataset," "no maintainable-earnings figure supplied") is a different
  problem domain from `adjustments`' adjustment-set consistency problem,
  `review`'s decision-validation problem, or `valuation`'s rate/weight/
  value-basis validity problem, even though `qoe` reads `adjustments.Result`
  directly. `qoe` also defines its own separate `FlagCode` vocabulary
  (`LARGE_NORMALIZATION_BURDEN`, `VOLATILE_EARNINGS`, etc.) for a
  structurally different purpose — a quality *signal*, not an input
  *problem* — mirroring how `orchestrator.ExclusionReason` and
  `applicability.Reason` sit alongside this taxonomy without being folded
  into it (see below).
- **`workingcapital.Issue{Code workingcapital.IssueCode, Severity, Message}`**
  — `analytics/workingcapital`'s own separate system (`NO_PERIODS`,
  `NO_PERIOD_META`, `PERIOD_MISSING_FROM_META`, `NO_REVENUE_DATA`,
  `EMPTY_INCLUSION_POLICY`, `PEG_METHOD_UNAVAILABLE`,
  `CURRENT_NWC_UNAVAILABLE`), a fifth system: an input-level
  working-capital-analysis problem is a different problem domain from
  `qoe`'s earnings-quality-analysis problem even though the two packages
  sit side by side under `analytics/` and follow the identical
  `Available`/`Warnings`/`Errors` shape.
- **`ratios.Issue{Code ratios.IssueCode, Severity, Message}`** —
  `analytics/ratios`' own separate system (`NO_PERIODS`, `NO_PERIOD_META`,
  `PERIOD_MISSING_FROM_META`), a sixth system for the same reason as the
  fourth and fifth above: an input-level ratio-analysis problem is its own
  problem domain, distinct from `qoe`'s and `workingcapital`'s despite all
  three packages sitting side by side under `analytics/` and sharing the
  identical `Available`/`Warnings`/`Errors` shape. `ratios` also defines its
  own separate `SignalCode` vocabulary (`WEAKENING_LIQUIDITY`,
  `RISING_LEVERAGE`, etc.), mirroring `qoe.FlagCode`'s identical
  input-problem/quality-signal split.
- **`cashflow.Issue{Code cashflow.IssueCode, Severity, Message}`** —
  `analytics/cashflow`'s own separate system (`NO_PERIODS`,
  `NO_PERIOD_META`, `PERIOD_MISSING_FROM_META`, `NO_CASH_FLOW_STATEMENT`,
  `ESTIMATED_FROM_EBITDA`), a seventh system for the same reason as the
  fourth through sixth above: an input-level cash-flow-analysis problem is
  its own problem domain, distinct from its `analytics/` siblings despite
  the identical `Available`/`Warnings`/`Errors` shape.
  `NO_CASH_FLOW_STATEMENT`/`ESTIMATED_FROM_EBITDA` specifically flag the
  reported-vs-estimated distinction this package's `CashFlowValue.IsEstimate`
  carries at the per-figure level (see the package's own README section).
  `cashflow` also defines its own separate `FlagCode` vocabulary
  (`WEAK_CASH_CONVERSION`, `LOW_DEBT_SERVICE_COVERAGE`, etc.), mirroring
  `qoe.FlagCode`/`ratios.SignalCode`'s identical input-problem/quality-signal
  split.
- **`revenuequality.Issue{Code revenuequality.IssueCode, Severity, Message}`**
  — `analytics/revenuequality`'s own separate system (`NO_PERIODS`,
  `NO_PERIOD_META`, `PERIOD_MISSING_FROM_META`, `NO_REVENUE_DATA`,
  `NO_CUSTOMER_DATA`, `CUSTOMER_PERIOD_NOT_IN_DATASET`,
  `CUSTOMER_REVENUE_UNRECONCILED`), an eighth system for the same reason as
  the fourth through seventh above: an input-level revenue-quality-analysis
  problem is its own problem domain, distinct from its `analytics/` siblings
  despite the identical `Available`/`Warnings`/`Errors` shape.
  `CUSTOMER_PERIOD_NOT_IN_DATASET`/`CUSTOMER_REVENUE_UNRECONCILED`
  specifically flag mismatches between `Input.CustomerRevenue` and
  `Input.Dataset` — both advisory only, since a caller's customer-level
  export commonly covers only a subset of total revenue streams or periods.
  `revenuequality` also defines its own separate `FlagCode` vocabulary
  (`DECLINING_RECURRING_MIX`, `GROWTH_DEPENDENT_ON_NEW_CUSTOMERS`, etc.),
  mirroring `qoe.FlagCode`/`ratios.SignalCode`/`cashflow.FlagCode`'s
  identical input-problem/quality-signal split.
- **`anomalies.Issue{Code anomalies.IssueCode, Severity, Message}`** —
  `analytics/anomalies`'s own separate system (`NO_PERIODS`,
  `NO_PERIOD_META`, `PERIOD_MISSING_FROM_META`), for the same reason as
  every other `analytics/` sibling above: an input-level
  anomaly-detection problem is its own problem domain, distinct from its
  siblings despite the identical `Available`/`Warnings`/`Errors` shape.
  `anomalies` also defines its own separate `RuleCode` vocabulary
  (`ABSOLUTE_AMOUNT_SPIKE`, `MARGIN_DETERIORATION`, etc.) and
  `AnomalySeverity` (kept distinct from `IssueSeverity` — see
  `AnomalySeverity`'s own doc comment), mirroring
  `qoe.FlagCode`/`ratios.SignalCode`/`cashflow.FlagCode`/
  `revenuequality.FlagCode`'s identical input-problem/quality-signal split.
- **`variance.Issue{Code variance.IssueCode, Severity, Message}`** —
  `analytics/variance`'s own separate system (`NO_LINES`,
  `MISSING_BASELINE`, `MISSING_BASELINE_TYPE`, `UNKNOWN_ACCOUNT_CODE`,
  `NO_PERIOD_META`, `PERIOD_MISSING_FROM_META`), for the same reason as
  every other `analytics/` sibling above: an input-level variance-analysis
  problem is its own problem domain, distinct from its siblings despite the
  identical `Available`/`Warnings`/`Errors` shape.
  `MISSING_BASELINE`/`MISSING_BASELINE_TYPE`/`UNKNOWN_ACCOUNT_CODE` are all
  advisory only — a line missing a baseline, missing a `BaselineType`
  label, or carrying an unrecognized `financial.Code` still contributes
  every other output it can (see `analytics/variance`'s own README section
  for exactly what stays available in each case). Unlike its `analytics/`
  siblings, `variance` does not define a separate `FlagCode`/rule
  vocabulary: `Favorability` and `Materiality` are per-line classification
  fields computed unconditionally for every line (not conditional signals
  that may or may not fire), so there is no analogous "quality signal"
  layer to split out.
- **`forecast.Issue{Code forecast.IssueCode, Severity, Scenario, Period, Message}`** —
  `analytics/forecast`'s own separate system (`NO_PERIODS`,
  `NO_PERIOD_META`, `INVALID_HORIZON`, `NO_SCENARIOS`,
  `EMPTY_SCENARIO_NAME`, `DUPLICATE_SCENARIO_NAME`, `NO_BASE_REVENUE`,
  `MISSING_REVENUE_ASSUMPTION`, `MISSING_COGS_ASSUMPTION`,
  `MISSING_OPEX_ASSUMPTION`, `IMPLIED_ZERO_GROSS_MARGIN`,
  `NEGATIVE_PROJECTED_VALUE`), for the same reason as every other
  `analytics/` sibling above: a forecast-input problem is its own problem
  domain. Unlike its siblings, `Issue` here carries two extra optional
  scope fields — `Scenario` (the `Scenario.Name` an issue applies to) and
  `Period` (the forecast-period label) — since a single `Calculate` call
  can produce issues scoped to one specific scenario/period among several
  running side by side, not just to the whole `Result`; a `Scenario`-scoped
  issue is duplicated onto that `ScenarioResult.Warnings` in addition to
  `Result.Warnings` so a caller working with one `ScenarioResult` in
  isolation still sees it. Unlike every sibling above, a missing
  `PeriodMeta` here is a blocking `SeverityError`
  (`Result.Available == false`), not an advisory warning: this package
  cannot identify the historical base period to project forward from at
  all without chronological order, whereas siblings merely lose a
  trend/seasonality output.
- **`debt.Issue{Code debt.IssueCode, Severity, Message, Loan}`** —
  `analytics/debt`'s own separate system (`NO_EBITDA`, `NEGATIVE_EBITDA`,
  `INVALID_LOAN_TERMS`, `SUSPICIOUS_INTEREST_RATE`, `NO_DEBT`,
  `NO_LENDER_POLICY`), for the same reason as every other `analytics/`
  sibling above: a debt-capacity-input problem is its own problem domain.
  Unlike its siblings, `Issue` here carries one extra optional scope field
  — `Loan` (e.g. `"proposed_loans[1]"`) — identifying which `LoanTerms`
  slice entry an issue applies to, since a single `Calculate` call
  validates a list of loans independently and an invalid entry is excluded
  from every downstream calculation rather than failing the whole `Result`.
  `INVALID_LOAN_TERMS` is the only blocking `SeverityError`, and it is
  scoped to just that one loan (which is dropped from
  `ExistingSchedules`/`ProposedSchedules`), not to the whole `Result`.
- **`ai.Issue{RowID, Code ai.IssueCode, Severity, Message}`** —
  `financial/classification/ai`'s own separate system (`AI_DISABLED`,
  `AI_PROVIDER_UNAVAILABLE`, `AI_TIMEOUT`, `AI_PROVIDER_ERROR`,
  `AI_INVALID_RESPONSE`, `AI_INVALID_CODE`, `AI_EMPTY_RESPONSE`,
  `AI_RATE_LIMITED`, `AI_CONTEXT_TOO_LARGE`, `AI_BUDGET_EXCEEDED`,
  `AI_STRUCTURAL_ROW_SKIPPED`), a fourth system for the same reason as the
  three above: a provider-call failure or an invalid AI response is a
  fundamentally different problem domain from a review-decision validation
  problem, an adjustment-set consistency problem, or a valuation-input
  validity problem, and this package must remain importable/testable
  without ever pulling in `review`/`valuation`/`adjustments`' own error
  vocabularies (or vice versa).
- **`ai.Issue{SourceRowID, Code ai.IssueCode, Severity, Message}`** —
  `financial/adjustments/ai`'s own separate system (a distinct Go type from
  classification's identically-named `ai.Issue`, in a different package):
  `AI_PROVIDER_UNAVAILABLE`, `AI_TIMEOUT`, `AI_PROVIDER_ERROR`,
  `AI_RATE_LIMITED`, `AI_MALFORMED_RESPONSE`, `UNKNOWN_SOURCE_ROW`,
  `WRONG_PERIOD`, `INVENTED_AMOUNT`, `INVALID_ADJUSTMENT_TYPE`,
  `INCOMPATIBLE_DIRECTION`, `STRUCTURAL_SOURCE_ROW`,
  `AMBIGUOUS_SOURCE_AMOUNT`, `DUPLICATE_SUGGESTION`, `NON_FINITE_AMOUNT`,
  `REQUIRES_USER_INPUT` — a fifth system: source-bound adjustment-suggestion
  validation (is this suggestion tied to a real row at a real amount?) is a
  different problem domain from classification-fallback validation (is
  this code in the closed set?), even though both packages sit under the
  same "optional AI capability" umbrella.
- **`financial.ValidationError{SourceID, Index, Code
  financial.ValidationErrorCode, Reason}`** — the one system in this list
  that is a genuine Go `error` (returned as `financial.ValidationErrors`,
  a `[]*ValidationError`, inspectable via `errors.As`) rather than a
  `Result.Errors` entry, since a malformed row is a true parse/validate
  failure `Normalize` cannot proceed past, not a domain outcome a
  `Result.Available` flag can represent. `Code` is stable and matchable
  exactly like the systems above (`UNRECOGNIZED_STATUS`,
  `MISSING_CODE`); a caller must branch on `Code`, never parse `Reason`,
  which is free text.
- **`salereadiness.Issue{Code salereadiness.IssueCode, Severity,
  Message}`** — `transactions/salereadiness`'s own separate system
  (`NO_INPUT_SUPPLIED`, `QOE_UNAVAILABLE`, `WORKING_CAPITAL_UNAVAILABLE`,
  `CONCENTRATION_UNAVAILABLE`, `REVENUE_QUALITY_UNAVAILABLE`,
  `CONSENSUS_UNAVAILABLE`, `METRICS_UNAVAILABLE`,
  `NO_POLICY_THRESHOLDS`), for the same reason as every `analytics/`
  sibling above: an input-level sale-readiness-assessment problem (which
  optional module was left unsupplied) is its own problem domain, distinct
  from every sibling package's despite reading several of their `Result`
  types directly. `salereadiness` also defines its own separate
  `DimensionCode`-scoped `Status` vocabulary
  (`StatusStrong`/`StatusAcceptable`/`StatusWeak`/`StatusConcerning`/
  `StatusUnassessed`) and `OpportunityCode` vocabulary, mirroring
  `qoe.FlagCode`/`ratios.SignalCode`/etc.'s identical input-problem/
  quality-signal split — except `Status` classifies a dimension
  (analogous to `workingcapital.TrendDirection`) rather than firing a
  discrete flag.
- **`consolidation.Issue{Code consolidation.IssueCode, Severity, EntityID,
  Period, Message}`** — `analytics/consolidation`'s own separate system
  (`NO_ENTITIES`, `NO_PERIODS`, `DUPLICATE_ENTITY_ID`,
  `UNKNOWN_SELECTED_ENTITY`, `MISSING_OWNERSHIP_PERCENT`,
  `INVALID_OWNERSHIP_PERCENT`, `MISSING_TARGET_CURRENCY`,
  `MISSING_CURRENCY_RATE`, `INVALID_CURRENCY_RATE`,
  `UNKNOWN_ELIMINATION_ENTITY`, `ELIMINATION_ENTITY_NOT_SELECTED`,
  `ELIMINATION_PERIOD_OUT_OF_SCOPE`, `ELIMINATION_TARGET_NOT_FOUND`,
  `PERIOD_MISSING_FOR_ENTITY`, `ENTITY_CURRENCY_EMPTY`), its own problem
  domain for the same reason as every sibling above: a consolidation
  input/reconciliation problem (an unresolvable target currency, a stale
  elimination, a gap in period coverage) doesn't overlap with any existing
  system, even though this package reads several entities'
  `financial.FinancialDataset`s directly. The one addition beyond every
  prior `Issue` shape: `EntityID` and `Period` fields alongside `Code`,
  since a consolidation problem is almost always scoped to one specific
  entity and/or period rather than the calculation as a whole.
- **`management.Issue{Code management.IssueCode, Severity, Message}`** —
  `reporting/management`'s own separate system
  (`NO_INPUT_SUPPLIED`, `METRICS_UNAVAILABLE`, `RATIOS_UNAVAILABLE`,
  `CASH_FLOW_UNAVAILABLE`, `WORKING_CAPITAL_UNAVAILABLE`,
  `QOE_UNAVAILABLE`, `VARIANCE_UNAVAILABLE`, `FORECAST_UNAVAILABLE`,
  `ANOMALIES_UNAVAILABLE`, `CONCENTRATION_UNAVAILABLE`,
  `REVENUE_QUALITY_UNAVAILABLE`, `DEBT_UNAVAILABLE`,
  `COVENANTS_UNAVAILABLE`, `CONSENSUS_UNAVAILABLE`,
  `NO_PERIOD_META_FOR_HISTORICAL_ORDER`), its own problem domain for the
  same reason as every sibling above: an input-availability problem for a
  pure aggregation/reshaping layer (which of up to 13 optional sibling
  results was supplied) is a different problem domain from any analysis
  package's own input-validity problem, even though `management` reads
  most of those packages' `Result` types directly.
- **`diagnostics.Issue{Code diagnostics.IssueCode, Severity, Message,
  Module}`** — `analytics/diagnostics`' own separate system, but a
  narrower one than `management.IssueCode`'s per-module-code approach: a
  single `MODULE_UNAVAILABLE` code (plus `NO_INPUT_SUPPLIED` for the
  degenerate zero-Input case) carries a `Module` field naming which of the
  fifteen optional sibling modules the Issue concerns, rather than fifteen
  separate `IssueXxxUnavailable` constants — a deliberate simplification
  since every one of those fifteen Issues means exactly the same thing
  ("this module was not supplied or was unavailable, narrowing which
  Findings could be mined for it"), so a `Module` field discriminates as
  precisely as a per-module code would without multiplying the taxonomy.

Three structured-but-not-error-severity vocabularies exist alongside these
and are not folded in, since they already serve the "stable, matchable"
purpose this taxonomy is for: `orchestrator.ExclusionReason` (why a method
never ran), `applicability.Reason{Kind, Detail, Points}` (a scoring
contribution, not a failure), and `qoe.FlagCode` (a quality signal, not a
failure — see above).

This is deliberately a small, flat set of additions — not a new
framework — sized to what a future consuming application actually needs
to map a domain failure to a UI/API response by code.

## Development

```bash
gofmt -w .        # should produce no diff on a clean tree
go test ./...
go vet ./...
go build ./...
go test -race ./...
```

Regenerate the binary CSV/XLSX/PDF/scanned-PDF fixtures (only needed when
a fixture's shape changes — see [`ingestion/pdf`](#ingestionpdf)'s Fixture
corpus for why PDF fixtures are hand-generated rather than
checked-in-only, and [Scanned/image PDF support (OCR)](#scannedimage-pdf-support-ocr)'s
fixture corpus section for the scanned-PDF fixtures specifically):

```bash
go run ./ingestion/fixtures/gen
```

## Recommended next module

With `financial`, `financial/classification`, `financial/reconciliation`,
`financial/metrics`, `financial/adjustments`, `financial/earnings`,
`valuation` (and its five method subpackages), `valuation/basis`,
`valuation/profile`, `valuation/applicability`, `valuation/orchestrator`,
`valuation/consensus`, `valuation/sensitivity`, `valuation/report`,
`settings`, and now `ingestion` (`ingestion/csv`, `ingestion/xlsx`) all in
place, **the deterministic valuation core described in
this README is now complete and contract-frozen**: every method produces a
version-stamped `Result`; applicability scores which methods suit a given
business with a full explanation trail; the orchestrator runs a selected
set under a caller-chosen filtering policy without one method's failure
affecting another; consensus combines results into simple/weighted means, a
range, and a dispersion indicator only after converting every included
result onto one explicit, auditable value basis (see
[Value basis and conversion](#value-basis-and-conversion)); sensitivity
analysis explores multiples/earnings/DCF-rate grids; and the report package
reshapes all of it into one presentation-neutral, JSON-serializable,
version-stamped structure — see
[`valuation/e2e/e2e_test.go`](valuation/e2e/e2e_test.go) and
[`valuation/e2e/e2e_asset_heavy_test.go`](valuation/e2e/e2e_asset_heavy_test.go)
for the full chain exercised end to end against two realistic fixtures (an
owner-operated service business and an asset-heavy manufacturer).

Nothing further can be added to this repository *as a deterministic
module* without crossing into scope this repository has deliberately
excluded from the start (see
[What this project intentionally does not contain](#what-this-project-intentionally-does-not-contain)).
The next steps are integration, not new packages:

1. **Persistence.** A consuming application needs to store
   `settings.Settings`/`Resolution`, classification `Result`s, individual
   method `Result`s (version-stamped specifically for this — see
   [Method versioning](#method-codes-and-versions)), `applicability.Results`,
   `orchestrator.Run`, `consensus.Result`, and `report.Report` as
   historical records tied to its own account/client/valuation entities.
   This repository defines the shapes; it does not decide how they're
   stored.
2. **Document parsing.** CSV/XLSX tabular ingestion, text-based
   (born-digital) PDF ingestion, AND scanned/image-only PDF ingestion (via
   local Tesseract OCR, entirely opt-in) are now all covered by
   `ingestion`, `ingestion/pdf`, `ingestion/ocr`, `ingestion/ocr/tesseract`,
   and `ingestion/pdf/pdfimage` — see [`ingestion`](#ingestion),
   [`ingestion/pdf`](#ingestionpdf), and
   [Scanned/image PDF support (OCR)](#scannedimage-pdf-support-ocr) above.
   What remains out of scope is a review UI for low-confidence OCR rows
   (the domain metadata to support one — `OCRProvenance`,
   `ReviewRecommended`, per-cell confidence — already exists; only the UI
   itself is not built here) and any AI/LLM-based OCR correction or
   document understanding, which would reintroduce exactly the kind of
   unaudited non-determinism this repository's design has consistently
   avoided.
3. **HTTP/API and UI.** `report.Report` is JSON-serializable specifically
   so a future API handler can return it directly and a future Vue UI (or
   any other frontend) can render it — building either is explicitly out
   of scope here.
4. **AI/LLM assistance**, if ever added further (e.g. suggesting
   adjustments, or explaining a report in natural language), should consume
   this repository's outputs as context, never replace its deterministic
   calculations — `applicability.Score`/`consensus.Dispersion.Score` must
   remain auditable point totals a reviewer can trace by hand, not
   something an LLM call decides. One such capability already exists,
   scoped narrowly to classification: see
   [AI fallback classification](#ai-fallback-classification).

Until that integration happens, this repository should keep gaining
domain modules as pure, dependency-free Go packages following the same
input/output discipline — see
[How this is intended to be integrated later](#how-this-is-intended-to-be-integrated-later).

**Unresolved domain assumptions left for a later revision of an earlier
package (or the integration layer above) to address:**

- **`valuation/consensus` does not itself enforce a single value type
  across included methods.** `Result.MixedValueTypes` warns when an
  `enterprise_value`, `equity_value`, and `asset_value` are averaged
  together (exactly what happens if a caller equal-weights all five
  methods without bridging EBITDA/DCF to equity first — see the `e2e`
  fixture, which deliberately does this and asserts on the resulting
  warning rather than hiding it), but nothing stops a caller from doing it
  anyway. A future integration layer should decide whether to bridge
  every enterprise-value method to equity before consensus by default, or
  surface `MixedValueTypes` prominently enough that a reviewer catches it.
- **`valuation/applicability`'s point deltas
  (`pointsStrongPositive`/`pointsPositive`/etc. = 25/15/8) are a first-pass
  heuristic scale**, not derived from any empirical study of which
  business characteristics actually predict a method's real-world
  reliability. A future revision with real user feedback on applicability
  accuracy should feel free to retune these constants — they're
  centralized in `valuation/applicability/rules.go` specifically so that's
  a small, contained change.
- **`valuation/orchestrator.Request.MinApplicabilityLevel` filtering is
  opt-in and all-or-nothing** (a method either runs or is fully excluded);
  there's no "run it anyway but flag it as low-applicability" middle
  ground beyond what `MethodOutcome.Applicability` already exposes for a
  caller to act on itself.
- `financial/adjustments` does not itself compute a market-rate
  replacement-owner salary for `TypeOwnerCompensationNormalization` — the
  caller supplies the already-computed *difference* as `Amount`. A future
  module could add a market-compensation-lookup helper, but that would
  introduce an external data dependency (salary survey data) this
  repository's "no external services" constraint currently rules out.
- `financial/earnings`'s `StrategyTrendAdjusted` is a plain OLS linear fit
  with no outlier handling, level-shift detection, or seasonality —
  deliberately left simple per this module's brief ("if domain assumptions
  become subjective, do not implement it yet"). A future module needing
  more sophistication (e.g. excluding a one-time COVID-affected year from
  a trend fit) should treat that as a caller-side observation-selection
  decision, not something this package infers automatically.
- `financial/adjustments` does not cap or sanity-check adjustment magnitude
  against the base metric (e.g. an adjustment larger than EBITDA itself
  producing a sign-flipped "normalized EBITDA" is allowed through
  uncaught). This mirrors `financial/metrics`' general stance of computing
  exactly what the formula says and leaving business-judgment plausibility
  review to a human or a future review-workflow layer, rather than this
  package guessing at what counts as "too large."
- **`valuation/capitalization` does not verify its earnings base matches
  its own equity-value labeling.** The package always reports
  `ValueTypeEquity`, which is correct for an SDE-like or owner-net-income
  earnings base but would be a mislabeled *enterprise* value if a caller
  supplied a debt-free/EBIT-like earnings figure instead — this package
  has no way to detect which kind of figure it was handed. A future
  module (or a documentation-only convention enforced at the call site)
  should make explicit which earnings base each caller is expected to
  supply, rather than this package guessing.
- **No method validates its multiple/rate against a plausible range.** A
  multiple of 50x or a capitalization rate of 200% both pass validation
  today (only sign/finiteness/ordering are checked) — implausible-but-
  technically-valid assumptions are allowed through uncaught, mirroring
  `financial/adjustments`' identical stance on unchecked adjustment
  magnitude above. A future review-workflow layer, not this package, is
  expected to catch "technically valid but implausible" input.
- **The DCF's mid-year convention is a single global on/off flag**
  (`Input.MidYearConvention`), applied identically to every forecast
  period and the terminal value. A more granular model (e.g. mid-year for
  operating cash flow but end-of-year for a known one-time terminal
  transaction) is not supported and would need a per-period override this
  package's `Input` does not currently expose.

- `financial/adjustments` does not itself compute a market-rate
  replacement-owner salary for `TypeOwnerCompensationNormalization` — the
  caller supplies the already-computed *difference* as `Amount`. A future
  module could add a market-compensation-lookup helper, but that would
  introduce an external data dependency (salary survey data) this
  repository's "no external services" constraint currently rules out.
- `financial/earnings`'s `StrategyTrendAdjusted` is a plain OLS linear fit
  with no outlier handling, level-shift detection, or seasonality —
  deliberately left simple per this module's brief ("if domain assumptions
  become subjective, do not implement it yet"). A future module needing
  more sophistication (e.g. excluding a one-time COVID-affected year from
  a trend fit) should treat that as a caller-side observation-selection
  decision, not something this package infers automatically.
- `financial/adjustments` does not cap or sanity-check adjustment magnitude
  against the base metric (e.g. an adjustment larger than EBITDA itself
  producing a sign-flipped "normalized EBITDA" is allowed through
  uncaught). This mirrors `financial/metrics`' general stance of computing
  exactly what the formula says and leaving business-judgment plausibility
  review to a human or a future review-workflow layer, rather than this
  package guessing at what counts as "too large."
- **`valuation/capitalization` does not verify its earnings base matches
  its own equity-value labeling.** The package always reports
  `ValueTypeEquity`, which is correct for an SDE-like or owner-net-income
  earnings base but would be a mislabeled *enterprise* value if a caller
  supplied a debt-free/EBIT-like earnings figure instead — this package
  has no way to detect which kind of figure it was handed. A future
  module (or a documentation-only convention enforced at the call site)
  should make explicit which earnings base each caller is expected to
  supply, rather than this package guessing.
- **No method validates its multiple/rate against a plausible range.** A
  multiple of 50x or a capitalization rate of 200% both pass validation
  today (only sign/finiteness/ordering are checked) — implausible-but-
  technically-valid assumptions are allowed through uncaught, mirroring
  `financial/adjustments`' identical stance on unchecked adjustment
  magnitude above. A future review-workflow layer, not this package, is
  expected to catch "technically valid but implausible" input.
- **No cross-method reconciliation exists yet.** Each method here is
  independently correct and independently explainable, but nothing
  compares, say, an EBITDA-multiple equity value against a DCF equity
  value for the same business and flags a large divergence — that is
  exactly the applicability/consensus module's job, deliberately not
  built prematurely into any individual method package.
- **The DCF's mid-year convention is a single global on/off flag**
  (`Input.MidYearConvention`), applied identically to every forecast
  period and the terminal value. A more granular model (e.g. mid-year for
  operating cash flow but end-of-year for a known one-time terminal
  transaction) is not supported and would need a per-period override this
  package's `Input` does not currently expose.

### Known deterministic ingestion gaps

Real-world tabular financial exports are wildly inconsistent, so
`ingestion`'s deterministic rules — like every other detection heuristic in
this repository — trade some recall for the guarantee that a match is
always explainable and never fabricated. Known gaps, left for a future
revision or for a caller-side override (`Options.LabelColumnOverride`,
`HeaderRowOverride`, `PeriodColumnOverrides`, `StatementTypeOverride`) to
handle in the meantime:

- ~~`ingestion`'s own structural read (`Row.Kind`/`Row.Status`) is not
  binding on `financial/classification`~~ — **resolved.** `financial.
  RawLineItem` and `financial.MappedLineItem` now carry a `Kind
  financial.RowKind` field (`""`/normal, `"heading"`, `"subtotal"`,
  `"total"` — zero value `RowKindNormal`, `omitempty` in JSON).
  `ToRawLineItems()` populates it from `Row.Kind`, and
  `classification.Classify`'s structural-detection stage checks
  `RawLineItem.Kind` **before** falling back to its own label-token
  heuristic (`structuralTotalTokens`) — see
  [`financial/classification`](#financialclassification) below. A non-zero
  `Kind` from ingestion now wins outright, so `"Gross Profit"` (recognized
  as a subtotal by ingestion's broader label-shape detection but not by
  classification's narrower total/subtotal/net token check) resolves
  consistently end to end. `Kind`'s zero value means "no upstream signal
  supplied," so a hand-built `RawLineItem` (as most existing tests
  construct) is completely unaffected and classification falls back to
  exactly its pre-existing label heuristic — see
  `TestClassify_GrossProfitWithoutKindFallsThroughToOrdinaryClassification`
  in `financial/classification/classify_test.go`, which pins this fallback
  behavior in place. As a consequence, heading rows (`Kind ==
  RowKindHeading`) also now survive `ToRawLineItems()` instead of being
  dropped, and `classification.Classify` maps a heading to
  `RowStatusIgnored` (which `financial.Normalize` already excludes from
  aggregation) rather than requiring the caller to have kept `Result.Rows`
  around separately.
- **Indentation-based parent/section detection depends on the source
  preserving leading whitespace**, which plain CSV frequently does not
  (many spreadsheet-to-CSV exporters strip it) while XLSX cell text
  usually does. A CSV export with no indentation still gets a `ParentLabel`
  from heading rows, but nested sub-sections within a single indent level
  cannot be distinguished from each other by indentation alone in that
  case.
- **Header/period detection scans only the first 15 non-blank rows**
  (`headerScanLimit`) before giving up. A statement with an unusually long
  cover-page/title block ahead of its real header row would need
  `Options.HeaderRowOverride`.
- **Numeric parsing assumes North American formatting** (`,` thousands
  separator, `.` decimal point) per `Options.Locale`'s current single
  supported value, `LocaleEnUS`. A European-formatted export (`.`
  thousands, `,` decimal) would need a new `Locale` value and its own
  numeric-parsing rules — deliberately not built until there's a concrete
  need, per this repository's general "don't build for hypothetical
  requirements" discipline.
- **Multi-row (wrapped) headers are not supported** — a header spanning
  two physical rows (e.g. a merged-cell "2024" above a second row reading
  "Actual"/"Budget") is read as two independent candidate header rows, not
  combined into one composite period label.
- **XLSX cell styling (bold, fill color, cell borders) is not read as a
  structural signal**, even though the ingestion contract's structural-
  detection section lists it as a possible signal alongside label text and
  blank-value patterns. Label-shape detection alone (starts with "Total",
  has no numeric values, etc.) has proven sufficient for every fixture in
  the corpus so far; adding style-based detection would mean threading
  excelize style lookups through `ingestion/internal/tabular`, which
  currently has zero XLSX-specific knowledge by design.

**PDF-specific known limitations** are documented separately in
[`ingestion/pdf`](#ingestionpdf)'s own "Known PDF-specific limitations"
subsection, to keep PDF-only detail out of this CSV/XLSX-focused list.

## Recommended next phase

With CSV, XLSX, text-based PDF, scanned/image-only PDF (via local
Tesseract OCR — see [Scanned/image PDF support (OCR)](#scannedimage-pdf-support-ocr)),
and now the deterministic `review` domain layer (see [`review`](#review)
below) all in place, this repository's deterministic domain scope is now
essentially complete: ingestion, classification, normalization,
reconciliation, metrics, adjustments, maintainable earnings, the five
valuation methods, applicability, orchestration, consensus, sensitivity,
reporting, settings resolution, and — closing the loop between
machine-proposed values and a human confirming them — a structured review/
decision/readiness layer sitting immediately ahead of "proceed to
valuation." The remaining gaps are the ones this repository has
consistently scoped out at every phase (see
[What this project intentionally does not contain](#what-this-project-intentionally-does-not-contain)):
a persistence layer, HTTP/API handlers and auth, and an actual rendered
review UI (the domain metadata AND the full review-domain model needed to
build one — `review.Plan`, `review.ReviewItem`, `review.Decision`,
`review.ApplyResult`, `review.Readiness` — are already in place; only the
UI itself, Vue or otherwise, is out of scope here). Each remains a natural
candidate for a future module or application layer built on top of the
types and packages already defined here, but none should be started as
part of this repository's own scope.

Two exceptions have since been added, both strictly behind provider-neutral
interfaces, closed-set, mandatory-review, and never a replacement for their
respective deterministic pipeline: **optional, human-reviewed AI
classification fallback** (see [AI fallback classification](#ai-fallback-classification))
and **optional, human-reviewed AI adjustment/add-back suggestions** (see
[AI adjustment suggestions](#ai-adjustment-suggestions)). Every OTHER form
of AI/LLM assistance (document understanding, OCR correction, AI-invented
replacement salaries/market rents, valuation method/multiple selection, DCF
forecasting, report narrative generation) remains explicitly out of scope —
see each section's own "Explicit non-goals" for the full list.

## V1 integration readiness

With every domain module above in place, this repository went through a
dedicated integration-readiness pass rather than adding further domain
scope: a full public-API audit (which exported symbols are
`PUBLIC_V1`/`INTERNAL_IMPLEMENTATION`/`EXPERIMENTAL`), a data-ownership
audit (confirming no public processing function mutates a caller-owned
input), a nil/zero-value safety audit (every config/options struct's zero
value is safe to use), a JSON serialization audit (every major result type
round-trips, closing three gaps found in `financial/metrics`,
`valuation/applicability`, and `valuation/sensitivity`), a dependency/
licensing inventory, lightweight benchmarks on the hot paths, and targeted
fuzz tests on the highest-risk deterministic parsers (numeric parsing,
period parsing, CSV input, label normalization, review decision
validation). See [`docs/V1_CONTRACTS.md`](docs/V1_CONTRACTS.md) for the
full findings and [`docs/INTEGRATION.md`](docs/INTEGRATION.md) for the
recommended orchestration flow this pass validated end to end via
[`review/e2e_v1_contract_test.go`](review/e2e_v1_contract_test.go) and
[`examples/full_flow`](examples/full_flow).

## `review`

The last major deterministic domain layer ahead of persistence/HTTP/UI/AI
work: it sits immediately after ingestion/classification/reconciliation and
immediately before a caller commits to running valuation, turning
"something upstream could not resolve automatically" into a structured,
stable, JSON-serializable model a future application can render, collect
decisions against, and apply — rather than each caller inventing its own ad
hoc review/confirmation flow on top of this repository's other packages.

```
Ingestion
   ↓
Classification
   ↓
Review Plan
   ↓
User Decisions   ← future UI/main app
   ↓
Apply Decisions
   ↓
Normalize / Reconcile
   ↓
Readiness Gate
   ↓
Valuation
```

**No UI, no persistence, no AI.** `review` contains the exact same
exclusions the rest of this repository does (see
[What this project intentionally does not contain](#what-this-project-intentionally-does-not-contain)):
no Vue components, no forms, no HTTP endpoints, no database tables, no user
identity, no audit database, no AI/LLM suggestions, no automatic add-back
suggestions, no workflow queues, no notifications/email, no cloud OCR. It
has no idea whether the "caller" resolving a `Decision` is a script, a
future Vue app, or a human clicking a button in a browser — it only defines
the domain shape a review workflow needs (`Build` → `Plan` → `Decisions` →
`Apply` → corrected data) and applies it deterministically. **The future UI
is expected to render this model, not duplicate its rules in Vue** — every
threshold, every ID scheme, every severity/readiness rule lives here in Go,
once, so a review screen is a rendering problem, not a second
implementation of "when does this need review."

Exported surface, by file:

- **`types.go`** — `Kind` (the eight review kinds, below), `Severity`
  (`INFO`/`WARNING`/`ERROR`/`BLOCKING`), `Status`
  (`PENDING`/`RESOLVED`/`REJECTED`/`INVALID_DECISION`), `ReviewItem` (one
  stable review unit, with one typed `*...Payload` field populated per
  `Kind` — mirroring `orchestrator.MethodOutcome`'s five separate typed
  pointers rather than a generic `any`/`map[string]any`), the eight
  `*Payload` types, `Summary`, `Plan`, `SchemaVersion`.
- **`errors.go`** — `IssueSeverity`, `IssueCode`, `Issue`, `HasErrors` — a
  third stable-code system alongside `valuation.Issue` and
  `adjustments.Issue` (see [Error taxonomy](#error-taxonomy) below for why).
- **`decisions.go`** — `Action`
  (`ACCEPT`/`OVERRIDE`/`IGNORE`/`REJECT`/`CONFIRM`), `Decision` (an
  `ItemID` + `Action` + one typed `*...Decision` payload field per item
  kind), and the six typed decision-payload structs.
- **`build.go`** — `Policy`, `DefaultPolicy()`, `RowContext`, `BuildInput`,
  `AssumptionSource`, `Build(BuildInput, Policy) Plan`, `IsMaterial`.
- **`apply.go`** — `Source`, `AppliedDecision`, `InvalidDecision`,
  `ApplyResult`, `Apply(Source, Plan, []Decision) ApplyResult`.
- **`readiness.go`** — `ReadinessState`
  (`READY`/`READY_WITH_WARNINGS`/`NOT_READY`), `ReasonCode`, `Reason`,
  `Readiness`, `EvaluateReadiness([]ReviewItem) Readiness`.

### Review kinds

Eight kinds, each with its own typed payload (`ReviewItem.Classification`,
`.OCRText`, `.OCRNumeric`, `.PeriodDetail`, `.Structure`,
`.Reconciliation`, `.Adjustment`, `.Assumption` — exactly one populated per
item, matching `Kind`):

| Kind | Triggered by |
|---|---|
| `CLASSIFICATION` | `classification.Result.IsUnknown()`; `ReviewRequired == true`; confidence below `Policy.ClassificationConfidenceThreshold`; the strongest alternative candidate is within `Policy.AlternativeConfidenceGap` of the primary result ("materially close"); or `Policy.ReviewAllClassifications` requests review for every row. |
| `OCR_TEXT` | A label cell (`ingestion.Cell.OCR != nil`, not a recognized value column) below `Policy.OCRConfidenceThreshold`, or `Policy.RequireReviewForAllOCR`. |
| `OCR_NUMERIC` | A value-column cell with `Cell.OCR != nil` and either a parsed value below `Policy.NumericOCRConfidenceThreshold`, `OCRProvenance.NumericCorrected == true`, or the OCR-ambiguous case (`Cell.OCR != nil && !Cell.Parsed`, distinct from an ordinary non-OCR parse failure) — or `Policy.RequireReviewForAllOCR`. |
| `PERIOD` | `ingestion.DetectedPeriod.PeriodType == PeriodTypeUnknown` (unparsed — always `BLOCKING`) or `Confidence < 1.0` (ambiguous). |
| `STRUCTURE` | Any row carrying a non-zero upstream `financial.RowKind` (heading/subtotal/total) — always `INFO`, advisory confirmation only, never blocking. |
| `RECONCILIATION` | A `reconciliation.Check` with `Status == StatusWarning` or `StatusFail`; a `FAIL` is `ERROR` by default, or `BLOCKING` when `Policy.ReconciliationFailureBlocks` is true. |
| `ADJUSTMENT` | Every `adjustments.Adjustment` a caller supplies gets a confirmation item — `Build` never invents one, only confirms. |
| `VALUATION_ASSUMPTION` | Every resolved key in a supplied `settings.Resolution`-shaped `AssumptionSource`, only when `Policy.EnableAssumptionReview` is true (opt-in). |

### Severity semantics

`INFO` (context only — e.g. a structural row confirmation), `WARNING`
(review recommended, does not block), `ERROR` (suspicious/invalid input
that should normally be corrected, still does not block by itself), and
`BLOCKING` (valuation must not proceed until resolved). Severity is
assigned entirely at `Build` time from structured `Policy` thresholds and
upstream enum/boolean fields — **never** by parsing a `Reason`/`Title`
string; [Readiness](#readiness) below is a pure function of `Severity` +
`Status` for exactly this reason.

### Deterministic IDs

Every `ReviewItem.ID` follows a fixed, colon-delimited, kind-specific
format, documented on each ID-building function in `build.go`:

| Kind | ID format | What legitimately changes the ID |
|---|---|---|
| `CLASSIFICATION` | `classification:<row-id>` | Only the row ID. A re-run with the same row but a different proposed code/confidence keeps the SAME ID. |
| `OCR_TEXT` | `ocr-text:<row-id>:<column-index>` | Row ID or column index. |
| `OCR_NUMERIC` | `ocr-numeric:<row-id>:<period>` (falls back to `ocr-numeric:<row-id>:col<n>` when no period is known for that column) | Row ID, period, or column (when period is unknown). Not the parsed amount/confidence. |
| `PERIOD` | `period:header:<column-index>` | Only the column index. |
| `STRUCTURE` | `structure:<row-id>` | Only the row ID. |
| `RECONCILIATION` | `reconciliation:<check-code>:<period>` (`dataset` in place of period for dataset-wide checks) | Check code or period. Not Status/Expected/Actual. |
| `ADJUSTMENT` | `adjustment:<adjustment-id>` | Only the caller-supplied adjustment ID. |
| `VALUATION_ASSUMPTION` | `assumption:<setting-key>` | Only the setting key. |

No random UUIDs anywhere — every ID is built from slice iteration or a
fixed string template, never from Go map iteration order (`Build`'s
internal sorts, e.g. over `AssumptionSource.AssumptionValues()`'s keys, are
always `sort.Strings`/`sort.SliceStable` before an ID is assigned). This
package needed **zero new `go.mod` entries** to achieve this.

### Decision model

`Decision{ItemID, Action, <one typed payload>}` mirrors `ReviewItem`'s own
typed-payload design rather than a generic value bag. `Action` is one of
`ACCEPT`, `OVERRIDE`, `IGNORE`, `REJECT`, `CONFIRM`; which actions a given
`Kind` accepts, and which payload `OVERRIDE` requires, is a fixed table in
`apply.go`'s `actionAllowedForKind`:

| Kind | Allowed actions | `OVERRIDE` payload |
|---|---|---|
| `CLASSIFICATION` | accept, override, ignore | `ClassificationDecision{Code}` — validated via `financial.IsValidCode`; `override` is additionally rejected (`IssueStructuralRowOverride`) when the row's current `financial.RowKind` is structural (see "Classification decisions never change row structure" below) |
| `OCR_TEXT` | accept, override, reject | *(text override carried as a display value only — no domain structure to correct beyond the label)* |
| `OCR_NUMERIC` | accept, override, reject | `OCRNumericDecision{Amount}` — must be finite |
| `PERIOD` | accept, override | `PeriodDecision{Period}` — must be non-empty |
| `STRUCTURE` | accept, override | `StructureDecision{RowKind}` — must be one of the four known `financial.RowKind` values |
| `RECONCILIATION` | accept, confirm, ignore | *(acknowledgment only)* |
| `ADJUSTMENT` | accept, override, ignore | `AdjustmentDecision{Included, Amount, NewAmount, Reason}` |
| `VALUATION_ASSUMPTION` | accept, override, confirm | `AssumptionDecision{Value}` — must be a `float64` (finite) or `bool` |

`Apply` validates every decision (unknown item ID; wrong action for the
item's kind; missing payload; invalid code/RowKind; non-finite amount;
empty period; conflicting or merely-repeated duplicate decisions for the
same `ItemID`) and reports every rejection as a `review.Issue` in
`ApplyResult.Invalid` — never a bare Go `error`. Two byte-identical
repeated decisions for the same item are a harmless warning (only the
first is applied); two *different* decisions for the same item in one call
are `CONFLICTING_DECISION` errors and neither is applied.

### Apply semantics

`Apply(source Source, plan Plan, decisions []Decision) ApplyResult` never
mutates `source`, `plan`, or `decisions` (proven directly in
`apply_immutability_test.go` via before/after JSON-snapshot comparison, not
merely asserted in a comment) and never returns a Go `error` — an empty
`decisions` slice is a valid, empty case (nothing applied, every item stays
unresolved), and every structural problem is a decision-level `Issue`
instead. "Corrected domain structures" come back as: `MappedLineItems`
(`[]financial.MappedLineItem`, with classification `Code`/`Status` and
structure `Kind`/`Status` corrected — an overridden `RowKind` is translated
to the matching `financial.RowStatus` exactly as
`classification.Classify` already does, so headings/totals are never
accidentally normalized after a review override); `Adjustments`
(`[]adjustments.Adjustment`, with `Included`/`Amount`/`Reason` corrected);
`CorrectedNumerics map[string]float64` (keyed by `ReviewItem.ID`, a
rejected numeric item is simply absent — treated exactly like a cell that
never parsed); and `CorrectedPeriods map[string]financial.Period`. Every
returned collection is a fresh copy — mutating `ApplyResult`'s returned
slices/maps never leaks back into the caller's original `Source`.

**Classification decisions never change row structure.** A `CLASSIFICATION`
item can exist even for a structural row (`buildClassificationItems` does
not skip heading/subtotal/total rows, so `Policy.ReviewAllClassifications`
can surface one), but an `ACTION_OVERRIDE` against it is rejected with
`IssueStructuralRowOverride` whenever the row's current `financial.RowKind`
is `HEADING`, `SUBTOTAL`, or `TOTAL` — assigning a financial code to such a
row would silently flip it to `RowStatusNormal` and double-count it during
`financial.Normalize`. `ACCEPT`/`IGNORE` on that same item remain valid
(they never assign a code or change structural semantics). Structural
semantics can only be changed through a `STRUCTURE` review decision
(`applyStructureDecision`, via `RowKind` ↔ `RowStatus` translation above);
once an explicit `STRUCTURE` override changes a row to `RowKindNormal`, a
subsequent `CLASSIFICATION` override on that same row applies normally.

**`Apply` resolves dependent decisions in deterministic domain order.**
Structural decisions are applied before classification decisions, so
equivalent decision sets produce the same result regardless of caller array
order — `ApplyResult.Applied`/`.Invalid` reflect this same domain order
rather than raw input order (see each field's own doc comment). Duplicate/
conflicting-decision detection is unaffected: it still keys off each
decision's original slice position.

### Readiness

`EvaluateReadiness([]ReviewItem) Readiness` is a **pure function of
`Severity` + `Status`**, never of any free-text field: `NOT_READY` if and
only if at least one unresolved (`Status.IsUnresolved()`) item has
`Severity == BLOCKING`; `READY_WITH_WARNINGS` if no such item exists but at
least one unresolved `WARNING`/`ERROR` item does; `READY` otherwise. An
unresolved `WARNING`/`ERROR` item — no matter how many — can never by
itself force `NOT_READY`; only `Policy`-driven `BLOCKING` severity gates
readiness (e.g. a reconciliation failure only blocks when
`Policy.ReconciliationFailureBlocks` is set). `Readiness.Reasons` carries a
stable `ReasonCode` + message + `ItemID`/`Kind` per contributing item,
never a bare string.

### Materiality policy

`Policy` (constructed via `DefaultPolicy()`, matching `ingestion.DefaultLimits`'
pattern):

| Field | Default | Effect |
|---|---|---|
| `ClassificationConfidenceThreshold` | `classification.DefaultReviewThreshold` (0.90) | Below this, a classification gets a review item. |
| `OCRConfidenceThreshold` | `70.0` | **Tesseract's native 0–100 scale**, NOT `classification.Confidence`'s `[0, 1]` scale — see the scale-mismatch warning below. |
| `NumericOCRConfidenceThreshold` | `70.0` | Same 0–100 scale, for numeric OCR cells. |
| `RequireReviewForAllOCR` | `false` | "Strict mode" — review every OCR-derived cell regardless of confidence. |
| `ReconciliationFailureBlocks` | `false` | Conservative opt-in: a reconciliation `FAIL` is `ERROR`, not `BLOCKING`, unless set. |
| `MaterialAmountThreshold` | `0` | `0` means "not applied" — materiality gating is **off** by default (everything material), never "nothing is material." |
| `MaterialPercentOfRevenue` | `0` | Same "0 = not applied" convention, as a fraction of revenue (e.g. `0.01` = 1%). |
| `ReviewAllClassifications` | `false` | Caller-opt-in: review every classification, not just low-confidence/UNKNOWN ones. |
| `EnableAssumptionReview` | `false` | Caller-opt-in: expose `VALUATION_ASSUMPTION` items at all. |
| `AlternativeConfidenceGap` | `0.05` | How close an alternative's confidence must be to count as "materially close." |

**Scale-mismatch footgun, called out prominently because it is a real
one:** `OCRConfidenceThreshold`/`NumericOCRConfidenceThreshold` are on
Tesseract's native 0–100 scale (`ingestion.OCRProvenance.Confidence`,
matching `lowConfidenceThreshold` in `ingestion/pdf/ocr_provenance.go`) —
**not** the `[0, 1]` scale `classification.Confidence` uses. Setting
`OCRConfidenceThreshold: 0.9` (a natural-looking number if you're used to
classification's scale) would flag virtually every OCR cell as
low-confidence, since Tesseract routinely reports well above `0.9` on its
own 0–100 scale.

`IsMaterial(amount float64, revenue *float64, policy Policy) bool`
(`build.go`) is `review`'s one small materiality helper per the task
brief's explicit "not its own subsystem" scope: material if
`abs(amount) >= MaterialAmountThreshold` OR (`revenue` known,
`MaterialPercentOfRevenue > 0`, and `abs(amount) >=
MaterialPercentOfRevenue * revenue`) — with both thresholds at their `0`
default, every amount is material.

### Audit/explainability

Every `AppliedDecision` (part of `ApplyResult.Applied`) carries exactly:
`ItemID`, `Kind`, `Action`, `OriginalProposal` (what `Build` originally
proposed), `FinalValue` (what was ultimately used), and `Reason` (the
structured reason the item needed review in the first place) — enough to
answer "what did the system propose, what did the user change, why was
review requested, and what value was ultimately used" without a database.
**No user ID or timestamp** — a deliberate scope decision for this
standalone library, not an oversight; a consuming application layer owns
identity/time.

### Integration with a future Vue/main app

A future UI's job is to **render** `Plan`/`ReviewItem`/`Decision`/
`ApplyResult`/`Readiness` — every field needed to build a review screen
(title, reason, severity, current/proposed value, alternatives, provenance,
bounding boxes for an OCR overlay) is already here — and to collect
`Decision` values back from a human, never to re-derive severity
thresholds, ID schemes, or readiness rules in TypeScript/Vue. Those rules
living in exactly one place (this package) is the entire point of building
`review` as a deterministic domain layer instead of embedding review logic
in a frontend.

See [`review/e2e_test.go`](review/e2e_test.go) for a complete worked
example: raw financial rows → classification → `review.Build` → an
attempted `review.Apply` while a `BLOCKING` classification item is still
unresolved (asserted `NOT_READY`) → decisions resolve it → `review.Apply`
again → `financial.Normalize` → `financial/metrics.Calculate` →
`financial/adjustments.Apply` → `financial/earnings.Calculate` →
`valuation/sde.Calculate`, producing a real priced equity value only once
every blocking review item is resolved.

## AI fallback classification

The first (and, for now, only) AI capability in this repository: an
**entirely optional** classification fallback that runs only when the
deterministic pipeline (`financial/classification`) cannot produce an
acceptable result, and whose output is never trusted automatically — it
becomes a `review.ReviewItem` like anything else and requires an explicit
human decision before it can affect a normalized dataset.

```
Raw line
   ↓
Deterministic classifier
   ↓
Acceptable result?
 ├─ yes → classification result
 └─ no
      ↓
  optional AI fallback
      ↓
 AI suggestion
      ↓
 review.Build
      ↓
 human confirmation
      ↓
 normalize
```

**AI suggestions are never authoritative financial mappings until confirmed
through the review domain.** Nothing in `financial/classification/ai`
mutates a `RawLineItem`, a `RowKind`, source provenance, period values, a
normalized `FinancialDataset`, or a valuation assumption — it proposes
classification metadata only, and `financial.Normalize` never runs against
anything an AI suggestion touched until a `review.Decision` has accepted or
overridden it.

**Package layout:**

- **`financial/classification/ai`** — the provider-neutral boundary and all
  orchestration logic. No dependency on any AI SDK; imports only
  `financial` and `financial/classification`. Exported surface:
  `Classifier` (the interface every provider adapter implements),
  `Request`/`Response`/`AllowedCode`/`ContextRow`/`DeterministicSummary`
  (the wire contract), `Policy`/`FallbackMode` (trigger/cost-control
  policy), `ClassifyWithFallback`/`ClassifyBatchWithFallback` (the
  orchestration entry points), `Provenance`/`Disagreement`/`FallbackOutcome`/
  `BatchOutcome` (the audit/result shapes), `ValidateResponse` (closed-set
  enforcement), `Issue`/`IssueCode` (this package's own error taxonomy),
  `FakeClassifier` (a credential-free test double), `BatchClassifier` (an
  optional batching capability a provider may additionally implement).
- **`financial/classification/ai/openai`** — one concrete provider adapter,
  using the official `github.com/openai/openai-go/v2` SDK (Apache-2.0
  licensed). Every OpenAI-specific type is confined to this subpackage —
  nothing outside it ever imports `openai-go`, so the rest of the
  repository (including every other package's tests) builds and runs with
  zero network access and zero credentials.

### Core principle: AI is a fallback, never a replacement

The classification precedence order established in
[`financial/classification`](#financialclassification)
is unchanged and always runs in full first:

```
explicit mapping → alias → context rule → phrase rule → AI fallback (optional) → UNKNOWN
```

`ai.ClassifyWithFallback`/`ClassifyBatchWithFallback` always call
`classification.Classify`/`ClassifyBatch` first, unconditionally. AI is
consulted only per `Policy.Mode`, and even then only ever REPLACES the row's
usable result when the deterministic side had nothing better (`UNKNOWN`, or
— under `AIForce`/`AIBelowConfidence` — no competing non-UNKNOWN code); see
[Disagreement](#ai-vs-deterministic-disagreement) below for what happens
when both sides propose something.

### Fallback modes (`Policy.Mode`)

| Mode | When AI runs |
|---|---|
| `AIDisabled` | Never. **This is the default** (`DefaultPolicy()`) — AI is opt-in only, never on by default. |
| `AIUnknownOnly` | Only for rows where the deterministic result is `SourceUnknown`. An already-classified row, at any confidence, is left alone. |
| `AIBelowConfidence` | `SourceUnknown` rows, plus any row whose deterministic `Confidence` is below `Policy.ConfidenceThreshold` (default `0.90`, matching `classification.DefaultReviewThreshold`). |
| `AIForce` | Every non-structural row, regardless of the deterministic result — for comparing AI against the deterministic pipeline. Structural-row safety (below) still applies. |

### Closed-set classification only

The model is never free to invent an account code. Every request carries an
explicit closed set (`Request.AllowedCodes`, built from
`financial.AllCodes()` — narrowed to the row's own `StatementType` when
known, via `ai.BuildAllowedCodes`) plus the literal option to answer
`"UNKNOWN"` (`ai.CodeUnknown`) if uncertain. `ai.ValidateResponse` checks
every response — `Response.Code` AND every entry in
`Response.Alternatives` — against that exact set before anything is
trusted: a code outside it is rejected outright
(`IssueInvalidCode`), the row's `UNKNOWN`/deterministic result is preserved
untouched, and no partial credit is given for "close enough." This is
enforced identically regardless of which provider adapter is used, since
`ValidateResponse` runs in the provider-neutral orchestration layer, not
inside any adapter.

### Structural-row safety

A row whose upstream `financial.RowKind` is `HEADING`, `SUBTOTAL`, or
`TOTAL` is skipped by AI fallback entirely by default
(`IssueStructuralRowSkipped`), even under `AIForce` — `Policy.AllowStructuralRows`
must be explicitly set to true to send a diagnostic request for one. Even
then, the returned code can **never** cause that row to normalize as an
ordinary account: `RowKind`/`RowStatus` are always preserved untouched, and
the AI's answer is recorded on `Provenance` purely for diagnostics, never
applied to the row's usable `Result`. This mirrors — and is independently
enforced alongside — `review`'s own [structural-override safety
rule](#apply-semantics) for classification decisions on structural rows:
two separate layers, same guarantee, deliberately not merged into one to
keep AI-fallback safety self-contained.

### Mandatory human review

Every AI-produced classification (`classification.Result` with `Source ==
classification.SourceAI`) has `ReviewRequired == true` — a hard rule, never
conditional on model-reported confidence. `review.Build` reads this exactly
like it reads any deterministic `ReviewRequired`/`Source`, with one small,
explicit addition: an AI-sourced classification item is always `Required ==
true` regardless of its `Severity` (which stays `WARNING` for a
successfully-validated non-`UNKNOWN` AI result, not `BLOCKING` — AI is not
presumed wrong, only unconfirmed). No second AI-specific review system
exists; `review` consumes an AI-sourced result through the exact same
`Build → Plan → Decision → Apply` pipeline as every other classification
source. See `review.TestBuild_AISourcedClassification_AlwaysRequired` and
`review.TestIntegration_AIFallback_AcceptedThroughReviewToNormalize` for
this pinned end to end.

### AI vs. deterministic disagreement

When the deterministic pipeline already proposed a real (non-`UNKNOWN`)
code and AI proposes a **different** one (only reachable under `AIForce` or
`AIBelowConfidence` — `AIUnknownOnly` only ever calls AI when the
deterministic side has nothing to disagree with), neither is silently
preferred. Both are preserved on `Provenance.Disagreement`
(`{DeterministicCode, AICode}`), and the row's usable `Result` stays the
**deterministic** one — AI is never auto-promoted over an existing
classification. `ai.DescribeDisagreement` renders the pair as the two-line
display a future review UI can show as-is:

```
Rule classifier: OPEX_PAYROLL
AI fallback: COGS_DIRECT_LABOR
```

### Confidence semantics

`classification.Confidence` (the deterministic pipeline's own heuristic
scale) and a provider's `Response.RawConfidence` are never mathematically
combined into a fake unified probability. They remain two separately
identifiable fields end to end:
`Provenance.DeterministicResult.Confidence` (the deterministic side, exactly
as `classification.Result.Confidence` reported it) and
`Provenance.ModelConfidence` (the provider's own raw, uncalibrated number,
carried forward under a name that makes that explicit at every call site).
The one place a `RawConfidence` is reinterpreted onto
`classification.Confidence`'s scale is `buildAIResult`'s
`Result.Confidence` field, purely so existing `Confidence`-consuming code
(`review.Build`'s threshold checks, display sorting) keeps working
unchanged for an AI-sourced `Result` — the original, uninterpreted number
always remains separately available on `Provenance.ModelConfidence`.

### Privacy / minimal-context policy

`Request` is the single source of truth for exactly what is sent to a
provider for one row:

- raw label, normalized label, parent label
- statement type, row kind (structural rows are normally never sent at
  all — see above)
- the closed set of allowed codes (with labels/categories, not raw enum
  strings)
- an optional, explicitly caller-supplied `IndustryContext` string
- an optional, caller-bounded window of nearby rows (`ContextRows`,
  capped at `Policy.MaxContextRows`, default 4) — label/parent-label only,
  never amounts
- a summary of what the deterministic pipeline already concluded

**Never sent:** per-period amounts/values, full uploaded documents, user
identity, account/customer identity, tax IDs, bank details, or any row
unrelated to the one being classified. `financial/classification/ai/openai`'s
own tests
(`TestClassify_RequestNeverIncludesUnrelatedState`) assert on the literal
wire payload to keep this guarantee from silently drifting. The OpenAI
adapter also pins `Store: false` on every request (the provider is not
asked to retain it for model-distillation/eval products) and never logs
API keys, full request/response contents, or prompts/responses to any
persistent store — see the adapter package's own doc comment for the exact
boundary.

### Provider abstraction and the initial adapter

`ai.Classifier` is a one-request-in/one-response-out interface; nothing in
`financial/classification/ai` or `review` imports any provider SDK. The
initial adapter, `financial/classification/ai/openai`, uses the official
`github.com/openai/openai-go/v2` SDK (Apache-2.0) with Structured Outputs
(a strict JSON Schema response format) so every response is guaranteed to
parse into `ai.Response`'s shape rather than requiring free-text scraping.
Configuration (API key, model, base URL, HTTP client, SDK options) is
always supplied explicitly via `openai.Config`/`openai.New` — the one
exception is `openai.NewFromEnv()`, a single named, opt-in convenience
constructor that reads `OPENAI_API_KEY`/`OPENAI_MODEL`/`OPENAI_BASE_URL`;
no other code in this repository reads AI-provider environment variables.
The adapter respects `ctx` cancellation/deadlines directly and performs no
hidden retries of its own (the SDK's own configurable retry behavior is
available via `option.WithMaxRetries` in `Config.ClientOptions` if a caller
wants it — never hardcoded here).

### Batching (`openai.Classifier` as an `ai.BatchClassifier`)

`financial/classification/ai/openai.Classifier` additionally implements
`ai.BatchClassifier`, so `ai.ClassifyBatchWithFallback` automatically issues
one structured chat-completion request per batch instead of one per row
whenever the caller's `Policy` makes more than one row eligible for AI at
once. **This is an optimization only** — it changes how many provider calls
are made, never what AI fallback means: the same closed-set contract, the
same mandatory-review guarantee, and the same
per-row provenance apply identically whether a row was classified via the
single-row path or as part of a batch. A caller that never sees
`Provenance`/`FallbackOutcome` cannot tell which path produced a given
result.

**Wire shape:** a batch request sends one JSON object with a `rows` array
(each row carrying the exact same fields `Request` sends for the single-row
path, plus a request-scoped positional id such as `"row-0"` this adapter
assigns purely to map a result back to its row — the id never leaves this
adapter and is not part of `ai.Request`/`ai.Response`) and a single
`allowed_codes` array (the union of every row's own closed set in this
batch, deduplicated). The model must return exactly one `results` entry per
row, each echoing that row's id, inside a strict JSON Schema Structured
Outputs envelope exactly like the single-row path's schema, just wrapped in
an array.

**Privacy boundary is unchanged:** each row in a batch request carries only
the same fields the single-row `Request` sends (label, normalized label,
parent label, statement type, row kind, allowed codes, bounded context
rows, deterministic-result summary) — never amounts, full documents, or
user/account/customer identity, and never any row from outside the batch
being classified. `openai.TestClassifyBatch_PrivacyBoundary` asserts this at
the wire level, the same way `TestClassify_RequestNeverIncludesUnrelatedState`
does for the single-row path.

**Splitting/limits compose, they don't replace each other:**
`Policy.MaxBatchSize` (row-count cap, enforced in
`ai.ClassifyBatchWithFallback`/`classifyMany` before this adapter is ever
called) and `openai.Config.MaxBatchCharacters` (an estimated-size cap —
marshaled-JSON byte length, not an exact token count — enforced inside
`ClassifyBatch` itself, defaulting to `openai.DefaultMaxBatchCharacters`)
both apply: a caller with a generous `MaxBatchSize` but unusually long
labels/context rows is still protected from an unbounded prompt, because
this adapter further splits any chunk it receives into deterministic,
input-order-preserving sub-batches whenever the running character estimate
would exceed the configured budget. Neither limit ever reorders rows.

**Partial-failure isolation:** if the provider returns a valid batch
envelope but one row's result is malformed — a duplicate id, an id that
matches no requested row, a missing id, or (one layer up, via the same
`ai.ValidateResponse` every path uses) a code outside that row's own closed
set — only that row is affected: it gets a per-row error (and, once it
reaches `ai.ClassifyBatchWithFallback`, the appropriate `Issue` and a
deterministic/`UNKNOWN` result), while every other row in the same batch
still returns its valid result normally. A whole-envelope failure (transport
error, non-2xx status, empty/unparseable JSON) is reported for every row in
that one character-bounded sub-batch only — other sub-batches in the same
`ClassifyBatch` call are still attempted independently. `Policy.Strict`
governs this identically to the single-row path: non-`Strict` (the default)
isolates and continues, `Strict` abandons the remaining batch at the first
failure — see Failure behavior below, which this adapter changes nothing
about.

**Single-row compatibility:** `Classify` (the pre-existing one-row method)
is unchanged — same request/response JSON shape as before, same tests
(`TestClassify_StructuredResponseParsed` etc.) still pass unmodified. It is
kept as its own thin path rather than routed through the batch envelope, so
the already-pinned single-row wire contract never changes shape; the two
paths share error-wrapping, provider/model/adapter metadata stamping, and
(one layer up) `ai.ValidateResponse` — the only code that is
necessarily different is the two schema-building functions themselves,
since a single object and an array-of-objects envelope are genuinely
different shapes.

### Failure behavior

AI failure can never make a classification result **worse** than the
deterministic pipeline already produced. A provider error, timeout, rate
limit, or invalid/malformed response always leaves the row on its
deterministic/`UNKNOWN` result plus a structured `Issue` explaining why —
never a Go `error` return from the orchestration functions themselves,
matching this repository's `review.Apply`-style "no bare error, everything
via issues" convention. One row's AI failure never corrupts or blocks any
other row's result (`ClassifyBatchWithFallback` isolates every row
independently) unless the caller explicitly opts into `Policy.Strict`,
which abandons the rest of the batch — with every not-yet-attempted row
left on its deterministic result plus an explanatory `Issue` — at the FIRST
AI failure, still without ever returning a Go `error`.

**Error taxonomy** (`ai.IssueCode`): `AI_DISABLED`,
`AI_PROVIDER_UNAVAILABLE`, `AI_TIMEOUT`, `AI_PROVIDER_ERROR`,
`AI_INVALID_RESPONSE`, `AI_INVALID_CODE`, `AI_EMPTY_RESPONSE`,
`AI_RATE_LIMITED`, `AI_CONTEXT_TOO_LARGE`, `AI_BUDGET_EXCEEDED`,
`AI_STRUCTURAL_ROW_SKIPPED` — a caller never needs to parse a provider error
string to know what happened; `ai.classifyProviderError` inspects a
returned error structurally (`errors.Is(err, context.DeadlineExceeded)`,
and the optional `ai.TimeoutError`/`ai.RateLimitError` interfaces an
adapter's error may satisfy) to pick the most specific code, and the
original wrapped error is always preserved on `FallbackOutcome.Err` for
diagnostics.

### Cost-control options (`Policy`)

`MaxAIRows` (total rows sent to AI across one batch call — once reached,
every remaining eligible row is left on its deterministic result with
`IssueBudgetExceeded`, never silently exceeding the configured budget),
`MaxBatchSize` (chunk size when a `Classifier` also implements the optional
`BatchClassifier` capability), `MaxContextRows` (per-row context-window
cap, oversized input is trimmed rather than rejecting the row), and
`Timeout` (bounds a single provider call when the caller's own `ctx` has no
tighter deadline already). No pricing/currency estimation is included —
these are row/request/time budgets only.

The OpenAI adapter adds one adapter-specific guard on top of these:
`openai.Config.MaxBatchCharacters` bounds the estimated request size (see
Batching above) of a single underlying provider call, independent of and
composed with `Policy.MaxBatchSize` — a row-count cap and a size-estimate
cap that both apply, never one substituting for the other.

### Versioning

Three independently-versioned concerns, per this repository's
[versioning strategy](#versioning-strategy):
`ai.RequestSchemaVersion` (the `Request`/`Response` wire shape),
`ai.OrchestrationVersion` (the trigger/fallback/safety decision logic in
`ClassifyWithFallback`/`ClassifyBatchWithFallback`), and
`openai.AdapterVersion` (this specific adapter's prompt-construction/
response-parsing logic) — each echoed on `Provenance` so a persisted AI
suggestion can always be traced back to the exact contract that produced
it, important because a future application layer is expected to persist AI
suggestions (see [Recommended next phase](#recommended-next-phase)).

### Testing without credentials

Every test in this repository except two runs with zero network access and
zero credentials: `financial/classification/ai`'s own orchestration tests
use `ai.FakeClassifier` (a fully in-memory `Classifier`/`BatchClassifier`
implementation), and `financial/classification/ai/openai`'s tests — including
its batching-specific suite in `batch_test.go` (multiple valid rows, mixed
UNKNOWN/valid, an invalid code isolated to one row, duplicate/missing/
unknown result ids, a whole-envelope provider failure,
`MaxBatchCharacters`/`MaxBatchSize`/`MaxAIRows` all respected, deterministic
splitting/ordering) — inject a fake `http.RoundTripper` so even the
OpenAI-specific request-building/response-parsing logic is exercised
without touching the network. The two exceptions,
`openai.TestIntegration_RealProvider` (single-row) and
`openai.TestIntegration_RealProvider_Batch` (a two-row batch), are opt-in
only: both `SKIPPED — provider credentials not configured` unless
`OPENAI_API_KEY` is set in the environment, are kept to the smallest size
that still exercises their respective path (one row/one call; two rows/one
batch call) to avoid meaningful API cost, and verify the response(s) parse
correctly, every returned code is a member of the closed set (or
`UNKNOWN`), and review-required semantics still hold — neither ever runs as
part of a normal `go test ./...`, including CI.

### Explicit non-goals for this phase

Not implemented, and not planned as part of this repository's own scope
(see [What this project intentionally does not contain](#what-this-project-intentionally-does-not-contain)):
AI OCR correction, AI financial-statement extraction, AI valuation method
selection, AI multiple selection, AI DCF forecasting, AI report narrative
generation, database persistence, HTTP endpoints, UI, or external
comparable-sales search. Any of these would be a separate, later phase —
this one is scoped strictly to optional, human-reviewed classification
fallback. (AI add-back/adjustment suggestions are a separate, narrower
capability — see [AI adjustment suggestions](#ai-adjustment-suggestions)
below — not a general recommendation engine.)

## AI adjustment suggestions

The second (and, for now, last) AI capability in this repository: an
**entirely optional** assistant that proposes normalization/add-back
**suggestions** over already-confirmed financial rows. Like [AI fallback
classification](#ai-fallback-classification), it never acts on its own —
every suggestion becomes a `review.ReviewItem` (`KindAdjustment`, always
`Required == true`) and requires an explicit human decision before the
deterministic `financial/adjustments` engine may use it.

```
Confirmed financial rows
        ↓
deterministic candidate selection
        ↓
optional AI suggestion
        ↓
validation against source rows
        ↓
review.KindAdjustment
        ↓
human confirmation
        ↓
deterministic adjustment engine
        ↓
normalized earnings
```

**Core safety rules** (enforced by `financial/adjustments/ai.ValidateSuggestions`,
never by trusting the provider):

- **AI never invents a financial amount.** Every suggestion must reference
  an existing `SourceRow` (row id + period) the caller explicitly supplied,
  and the suggested `Amount` must EXACTLY equal that row's own `Amount`. A
  mismatch of any size is rejected outright — never rounded, scaled, or
  "repaired."
- **AI never applies an adjustment.** `ToAdjustments` always builds
  `adjustments.Adjustment` values with `Included == false`; only a
  `review.Decision` (via the existing `review` package — see [Review
  integration](#review-integration-1) below) can flip that to `true`, and
  only `financial/adjustments.Apply` ever changes normalized EBITDA/SDE.
- **AI never changes source data or canonical classification** — this
  package has no write access to `financial.MappedLineItem`/
  `financial.RawLineItem`/`financial.Code` at all; it only reads
  caller-supplied `SourceRow` values.
- **AI never invents a replacement salary or market rent.** For owner
  compensation normalization or related-party rent, the model may flag
  that a row "appears potentially discretionary / requires normalization
  review" and set `Suggestion.RequiresUserInput = true`, but it must not
  propose what the replacement benchmark figure should be — see
  [`REQUIRES_USER_INPUT`](#requires_user_input-benchmark-values) below.
- **A closed-set adjustment taxonomy.** `Suggestion.AdjustmentType` must be
  a member of the `Request.AllowedTypes` closed set (built from the
  repository's actual `adjustments.AllTypes()` — never a duplicated or
  renamed enum); anything else is rejected as `IssueInvalidAdjustmentType`.

**Package layout:**

- **`financial/adjustments/ai`** — the provider-neutral boundary,
  deterministic candidate selection, and all validation/orchestration
  logic. Imports only `financial` and `financial/adjustments` — no AI SDK.
  Exported surface: `Suggester` (the interface every provider adapter
  implements), `Request`/`Response`/`SourceRow`/`Suggestion`/`AllowedType`/
  `ContextRow`/`MultiYearValue` (the wire contract), `Policy`/`Mode`
  (on/off trigger), `CandidatePolicy`/`SelectCandidates` (deterministic
  candidate rules), `BatchPolicy`/`BuildRequests` (cost-bounded batching),
  `SuggestAdjustments`/`SuggestAdjustmentsBatch` (orchestration entry
  points), `ValidateSuggestions`/`ValidatedSuggestion` (source-bound
  validation), `Provenance`/`Outcome`/`BatchOutcome`/`RejectedSuggestion`
  (the audit/result shapes), `Issue`/`IssueCode` (this package's own error
  taxonomy), `FakeSuggester` (a credential-free test double),
  `ToAdjustments`/`RequiredAdjustmentIDs` (conversion into the existing
  `review`/`adjustments` domain).
- **`financial/adjustments/ai/openai`** — one concrete provider adapter,
  using the same `github.com/openai/openai-go/v2` SDK as
  `financial/classification/ai/openai`, kept as an entirely independent
  package (different `Config`/`Suggester` types, different Structured
  Outputs schema) since the two AI capabilities solve different problems
  with different wire contracts — see that package's own doc comment.

### Why this is a second, independent capability (not reused classification AI)

`financial/classification/ai` answers "what canonical code is this row?"
for one row at a time, with amounts deliberately excluded from its request
(see that section's privacy policy). `financial/adjustments/ai` answers a
different question — "does this already-classified row look like a
normalization candidate, and if so, which adjustment type?" — for a
**bounded batch** of rows, and amounts are unavoidably part of the
question, since a suggestion is only meaningful tied to a specific dollar
figure (see [Privacy: amounts are necessary
here](#privacy-amounts-are-necessary-here)). Forcing both capabilities
through one shared `Classifier`/`Request` shape would either strip
amounts from a use case that needs them or leak them into a use case that
must never carry them — so this package defines its own
`Suggester`/`Request`/`Response`, reusing established *patterns*
(explicit `Config`, `Store: false`, Structured Outputs, character-budgeted
batching, `FakeSuggester`/`FakeClassifier`-style test doubles) rather than
a shared abstraction.

### Deterministic candidate selection

`SelectCandidates(rows []SourceRow, policy CandidatePolicy) []SourceRow`
never sends "every row in the dataset" to a provider. A row becomes a
candidate only if:

- its `RowID` is explicitly listed in `CandidatePolicy.ExplicitRowIDs`
  (the caller hand-picks it), or
- `CandidatePolicy.EnableCodeRules` is true (opt-in, like every AI trigger
  in this repository) and its `Code` is a member of `CandidatePolicy.Codes`
  (or the default `CandidateCodes`: owner compensation, vehicle, travel,
  professional fees, repairs, other operating expense, other income, other
  expense).

Regardless of policy, a **structural row** (`RowKind` heading/subtotal/
total) or a row flagged **`AmbiguousOCR`** (unconfirmed OCR-derived amount
— see `review.OCRNumericPayload.Ambiguous`) is never selected, and is
rejected again defensively at validation time even if a caller bypasses
`SelectCandidates` and builds a `Request` by hand. `CandidatePolicy.MaxRows`
(default `DefaultMaxCandidateRows` = 40) bounds the result; explicit-ID
matches are preferred over code-rule matches when truncating, so a
caller-forced row is never silently dropped ahead of a merely heuristic
one.

### Privacy: amounts are necessary here

Unlike `financial/classification/ai.Request`, `financial/adjustments/ai.Request`
**does include amounts** — this is a deliberate, documented difference,
not an oversight. A `SourceRow` carries:

- row id, period, label, parent label
- canonical `Code` and `StatementType`, when known
- the row's own `Amount` (required — an adjustment suggestion is
  inescapably about a specific dollar figure)
- `RowKind`/`AmbiguousOCR` (safety flags, never sent onward as "context")

Plus, at the `Request` level: the closed `AllowedTypes` set, an optional
caller-supplied `IndustryContext` string, an optional bounded
`ContextRows` window (label/parent-label only — **`ContextRow` has no
amount field at all**, pinned by
`openai.TestPrivacy_ContextRowsNeverCarryAmounts`-style tests), and
optional `MultiYearValues` (other periods' amounts for the *same*
row/account, only when the caller judges the trend useful).

**Never sent:** customer identity, user identity, bank details, tax IDs,
full uploaded statements, or any row not explicitly selected as a
candidate. `financial/adjustments/ai`'s own `TestPrivacy_*` tests assert on
the literal marshaled request payload to keep this guarantee from
silently drifting.

### `REQUIRES_USER_INPUT`: benchmark values

For `adjustments.TypeOwnerCompensationNormalization` and
`adjustments.TypeRelatedPartyRentAdjustment` — the two adjustment types
that inherently require an externally-benchmarked replacement figure (a
market-rate salary, a fair-market rent) — the model may set
`Suggestion.RequiresUserInput = true` while still pointing at the row
being flagged (e.g. "Officer Compensation appears potentially
discretionary / requires normalization review"). It must never propose
what that benchmark should be.

`ToAdjustments` converts such a suggestion into an `adjustments.Adjustment`
with `Amount: 0` (never the flagged current amount reinterpreted as a
normalization delta) and a `Notes` string stating a benchmark is still
needed. The adjustment stays `Included: false` and, with no `Effect` set
and no default available for these two types, cannot pass
`adjustments.Validate` until a human supplies the real figure — via a
`review.Decision` with `ActionOverride`/`AdjustmentDecision.NewAmount` (the
existing, unmodified review-domain lever for correcting an adjustment
amount).

### Structured response schema

Conceptually:

```json
{
  "suggestions": [
    {
      "source_row_id": "row-27",
      "period": "2025",
      "adjustment_type": "one_time_expense",
      "amount": 42000,
      "direction": "INCREASE_EARNINGS",
      "reason": "The label indicates a one-time relocation expense.",
      "confidence": 0.78,
      "requires_user_input": false
    }
  ]
}
```

The OpenAI adapter enforces this with Structured Outputs (`strict: true`),
so every response is guaranteed to parse rather than requiring free-text
scraping — see `financial/adjustments/ai/openai/prompt.go`.

### Validation (`ValidateSuggestions`)

Every suggestion is checked, independently, against the exact `Request`
that (supposedly) produced it. Rejected, with a specific `IssueCode`, when:

- the referenced source row does not exist (`IssueUnknownSourceRow`)
- the period does not match that row's own period (`IssueWrongPeriod`)
- the amount does not EXACTLY match the source row's amount
  (`IssueInventedAmount`) — no tolerance under MVP rules
- the adjustment type is not in the closed set
  (`IssueInvalidAdjustmentType`)
- the direction is incompatible with that type's fixed
  `adjustments.TypeMeta.DefaultEffect` (`IssueIncompatibleDirection`) — a
  type with no fixed default (`TypeCustom`,
  `TypeRelatedPartyRentAdjustment`) accepts either direction, since the
  real-world sign genuinely varies
- the source row is structural (`IssueStructuralSourceRow`) or
  `AmbiguousOCR` (`IssueAmbiguousSourceAmount`)
- another suggestion in the same response already targets the same (row,
  period, type) (`IssueDuplicateSuggestion`)
- the amount or confidence is non-finite (`IssueNonFiniteAmount`)

A rejected suggestion is never silently repaired — it is discarded, with
the reason preserved on `Outcome.Rejected` for diagnostics, and never
reaches `ToAdjustments`.

### Review integration

`financial/adjustments/ai` does not create a second review system. A valid
suggestion becomes an ordinary `adjustments.Adjustment`
(`ai.ToAdjustments`, always `Included: false`), and
`ai.RequiredAdjustmentIDs(adjs)` gives the caller the set to pass as
`review.BuildInput.RequiredAdjustmentIDs` — the one small, additive
extension `review.Build` gained for this capability
(`buildAdjustmentItems` now checks `RequiredAdjustmentIDs` and, when set,
raises that item to `Required: true`/`SeverityWarning`, versus the existing
default `Required: false`/`SeverityInfo` for an ordinary caller-constructed
adjustment). From there it is the exact same `KindAdjustment` review item,
`AdjustmentDecision`, and `applyAdjustmentDecision` code path any other
adjustment confirmation uses:

- **ACCEPT** (`AdjustmentDecision{Included: true}`) → the adjustment's
  `Included` becomes `true`, ready for `adjustments.Apply`.
- **REJECT/EXCLUDE** (`ActionIgnore`, or `ActionOverride` with
  `Included: false`) → `Included` stays/becomes `false`; the suggestion
  never reaches a bridge.
- **Modify amount** — only via `AdjustmentDecision.NewAmount`, the same
  lever used to supply a `REQUIRES_USER_INPUT` benchmark value or correct
  any other adjustment's amount; `review.Apply`'s existing
  `IssueNonFiniteAmount` validation applies identically.

### Readiness

An unresolved AI adjustment suggestion's review item is `Required: true`
but `SeverityWarning`, never `SeverityBlocking` — `EvaluateReadiness` (a
pure function of `Severity`+`Status`, unchanged by this capability) reports
`READY_WITH_WARNINGS`, not `NOT_READY`, while it sits unresolved. AI
finding a possible add-back must never by itself block a valuation a
caller is otherwise ready to run. A caller wanting stricter behavior
enforces its own policy on top — e.g. refusing to proceed while
`ApplyResult.UnresolvedRequired` is non-empty — independent of
`Readiness.State`.

### Failure behavior

Provider failure never blocks the deterministic pipeline. `SuggestAdjustments`
never returns a Go error: a disabled `Policy`, a nil `Suggester`, or a
`Suggester.Suggest` error (timeout, rate limit, transport failure) all
produce a normal `Outcome` with `Valid == nil` and a structured `Issue`
(`IssueProviderUnavailable`/`IssueTimeout`/`IssueRateLimited`/
`IssueProviderError`) explaining why — the caller decides whether/how to
surface that, and the rest of the deterministic ingestion → classification
→ review → adjustments → valuation pipeline is entirely unaffected.
`SuggestAdjustmentsBatch` isolates one request chunk's failure from every
other chunk in the same batch.

### Batching and cost controls

`BuildRequests(candidates, allowedTypes, industryContext, contextByRowID,
multiYearByRowID, policy BatchPolicy)` deterministically splits a candidate
set into one or more bounded `Request` values: `BatchPolicy.MaxCandidateRows`
(default `DefaultMaxCandidateRows` = 40) caps rows per request,
`BatchPolicy.MaxRequestCharacters` (default `DefaultMaxRequestCharacters` =
24000) caps the estimated marshaled-JSON size, and the two compose —
whichever limit is hit first ends the current chunk. A single row whose own
estimated size already exceeds the character budget is never dropped; it
forms its own one-row chunk. Source identity (row id + period) is preserved
verbatim across chunks, so a caller can always reassemble which candidate a
returned suggestion belongs to. `Policy.Timeout` (default `DefaultTimeout`
= 30s) bounds each underlying provider call.

### Provider adapter (`financial/adjustments/ai/openai`)

Mirrors `financial/classification/ai/openai`'s conventions: explicit
`Config`/`New` (no global client), `NewFromEnv()` as the one named opt-in
convenience constructor reading `OPENAI_API_KEY`/`OPENAI_MODEL`/
`OPENAI_BASE_URL`, `Store: false` on every request, Structured Outputs for
guaranteed-parseable responses, `ctx` cancellation respected directly with
no hidden retries, and no logging of raw request/response contents by
default. Independent `Config`/`Suggester` types from the classification
adapter (see [Why this is a second, independent
capability](#why-this-is-a-second-independent-capability-not-reused-classification-ai)).

### Testing without credentials

Every test in `financial/adjustments/ai` and
`financial/adjustments/ai/openai` except one runs with zero network access
and zero credentials, using `ai.FakeSuggester` (a fully in-memory
`Suggester`). The one exception,
`openai.TestIntegration_RealProvider`, is opt-in only: `SKIPPED — provider
credentials not configured` unless `OPENAI_API_KEY` is set, kept to a tiny
two-candidate-row request to avoid meaningful API cost, and verifies the
response parses correctly and every returned suggestion is source-bound
(passes `ai.ValidateSuggestions` against the exact request sent). It never
runs as part of a normal `go test ./...`, including CI.

### Explicit non-goals

Not implemented, and not planned as part of this capability's scope: AI
creating financial amounts out of thin air, AI replacement-salary
estimates, AI market-rent estimates, AI valuation multiples, AI
comparables search, AI forecasts, AI valuation calculations, AI report
narrative generation, database persistence, HTTP endpoints, or UI. This
capability is scoped strictly to optional, source-bound, human-reviewed
adjustment suggestions — the deterministic `financial/adjustments` engine
remains the only code that ever changes normalized EBITDA/SDE.
