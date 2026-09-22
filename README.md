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

## Architecture

Every stage below is a pure function of the previous stage's output — no
I/O, no shared mutable state, no hidden global config:

```
CSV / XLSX bytes
        ↓
Tabular Ingestion          (ingestion, ingestion/csv, ingestion/xlsx)
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

A separate, cross-cutting `settings` package supplies the hierarchical
system/account/client/valuation rate/multiple/method-enable resolution
(see [`settings`](#settings) below) that the orchestrator and individual
methods consume; it has no fixed position in the pipeline above since a
resolved `settings.Resolution` snapshot is assembled once, ahead of time,
and handed into a run rather than looked up mid-calculation (see
[Settings snapshot contract](#settings-snapshot-contract)).

**What is deliberately absent from every stage above, and from this
repository entirely:** no database, no HTTP/API layer, no AI/LLM, no PDF
parsing or OCR. Deterministic CSV/XLSX tabular ingestion (`ingestion`,
`ingestion/csv`, `ingestion/xlsx`) is the one exception to the
"no document parsing" rule established in earlier revisions of this
README — see [`ingestion`](#ingestion) below for why a purely
structural, non-AI, non-database tabular reader fits this repository's
constraints while PDF/OCR still does not — see
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
- AI/LLM integrations
- PDF parsing or OCR
- cloud/storage integrations

Deterministic CSV/XLSX tabular ingestion (`ingestion` and its `csv`/`xlsx`
subpackages — see [`ingestion`](#ingestion) below) is a narrow, deliberate
exception: it is pure structural interpretation of already-tabular data
(rows, columns, headers), performs no AI/ML/OCR, touches no database or
network, and hands off to `financial/classification` — which this
repository already contains — rather than duplicating it. PDF parsing and
OCR remain excluded because turning an unstructured document into tabular
data is exactly the document-understanding problem this repository's
"no PDF/OCR" constraint rules out; ingestion only begins once data is
already in rows and columns.

The design rule is:

```
Go structs / JSON-compatible data in
  → deterministic domain processing
  → Go structs / JSON-compatible data out
```

No global state. No infrastructure dependencies. No side effects. Every
exported function in this repository is a pure function of its inputs.

Deterministic CSV/XLSX tabular ingestion (structural interpretation of a
tabular financial statement export into `financial.RawLineItem` values,
with no classification of its own) is implemented in `ingestion` and its
`ingestion/csv`/`ingestion/xlsx` subpackages; deterministic, rule/alias-based
classification (deciding *which* canonical code a raw row maps to) is
implemented in `financial/classification`;
dataset-internal-consistency checking is implemented in
`financial/reconciliation`; derived financial metrics (EBITDA, SDE, working
capital, growth/volatility, etc.) are implemented in `financial/metrics`;
explicit normalization adjustments and the normalized EBITDA/SDE bridges
built from them are implemented in `financial/adjustments`; selecting a
single maintainable-earnings figure across historical periods is
implemented in `financial/earnings`; the individual valuation methods
themselves (SDE multiple, EBITDA multiple, capitalization of earnings, DCF,
adjusted net asset value) are implemented in `valuation` and its
per-method subpackages — see below for all six. Method applicability
rules, a multi-method consensus/weighting engine, sensitivity analysis,
reporting/UI, AI/LLM assistance, a database, and PDF/document parsing are
all still explicitly **out of scope for this repository** at this stage.
They are expected to be built as later modules, or in the consuming
application, on top of the types and packages defined here — see
[Recommended next module](#recommended-next-module).

## Package boundaries

```
go-valuate/
  ingestion/                 CSV/XLSX bytes -> raw tabular rows (no classification)
  ingestion/csv/              CSV parser (encoding/csv, zero external dependencies)
  ingestion/xlsx/              XLSX parser (github.com/xuri/excelize/v2)
  ingestion/internal/tabular/  shared statement-interpretation logic (csv+xlsx)
  ingestion/fixtures/         CSV/XLSX fixture corpus + XLSX generator (fixtures/gen)
  financial/                 canonical financial model, taxonomy, and normalizer
  financial/classification/  deterministic raw-row -> canonical-code classifier
  financial/reconciliation/  dataset internal-consistency checks
  financial/metrics/         centralized derived-metrics engine (EBITDA, SDE, trends, ...)
  financial/adjustments/     explicit normalization adjustments -> normalized EBITDA/SDE bridges
  financial/earnings/        maintainable-earnings selection across historical periods
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
  fixtures/                  example JSON matching the Go types, used by tests
                             as living documentation
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
"no PDF/OCR" boundary described in
[What this project intentionally does not contain](#what-this-project-intentionally-does-not-contain)
— it begins only once data is already tabular (rows and columns), never
attempts document understanding, and performs **no AI/LLM inference**
anywhere in its detection logic. Every parser is a pure function of its
input bytes/reader and `Options`: the same input always produces the same
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
`Row.Kind`/`Row.Status` (subtotal/total) is this package's own
best-effort structural read, used only to populate
`financial.RawLineItem` via `ToRawLineItems()`; `financial/classification`
still runs its own independent structural detection over the resulting
labels (see that package's `structuralTotalTokens`), and the two are not
required to agree in every case — a label like `"Gross Profit"` is
recognized as a subtotal by `ingestion`'s label-shape detection but not by
`financial/classification`'s narrower total/subtotal/net token check,
since `RawLineItem` carries no field for ingestion's structural read to
travel on. This is a known, documented gap — see
[Known deterministic ingestion gaps](#known-deterministic-ingestion-gaps).

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
renumbered around them, so source position stays traceable); heading rows
are retained in `Result.Rows` for context but excluded from
`ToRawLineItems()`, since `financial.RawLineItem` has no field to
represent "this row is a section heading." `Row.ParentLabel` tracks the
innermost currently-open heading section using a simple indent-aware stack
(`ParentTracker`): a heading row opens a section at its indent level, a
subtotal/total row at or below that level closes it, and every other row
inherits the innermost open section's label — see the `IndentLevel` field
for the (deliberately coarse) whitespace-based signal this is built on.

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
every non-heading, non-blank row into a `financial.RawLineItem`, ready for
`classification.ClassifyBatch` exactly as shown in the diagram above; see
[`ingestion/integration_test.go`](ingestion/integration_test.go) for the
full chain exercised against real fixtures, through
`financial.Normalize`, and (for the balance sheet fixture) into
`financial/reconciliation`.

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
no PDF parsing, no OCR, no LLM/AI interpretation or ML classification
(statement-type/period/structural detection are 100% rule-based, same as
`financial/classification`), no database persistence, no file-upload HTTP
endpoints, no frontend, no QuickBooks/Xero API integration, no automatic
add-back/adjustment recommendations, no external financial-data sources.

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

1. **structural detection** — is the label a total/subtotal row (e.g.
   "Total Operating Expenses", "Net Income")? If so, no code is proposed,
   `Status` is set to `subtotal`/`total`, and every other stage is skipped
   entirely — even if an alias exists for that exact label — since
   aggregation correctness for totals matters more than classifying them.
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
| Classification rules | `classification.DefaultRulesVersion` | The built-in rule set `DefaultRules()` returns (`financial/classification`) — a caller's own custom `Config.Rules` versions independently |
| Metrics formulas | `metrics.FormulaVersion`, echoed on `metrics.Result.FormulaVersion` | The fixed metric formula table (`financial/metrics`) |
| Adjustment semantics | `adjustments.SemanticsVersion`, echoed on `adjustments.Result.SemanticsVersion` | The default Targets/Effect table and bridge formulas (`financial/adjustments`) |
| Valuation method versions | `sde.Version` / `ebitda.Version` / `capitalization.Version` / `dcf.Version` / `netassets.Version`, each echoed as `Result.MethodVersion` | Each method's own formula and validation rules |
| Applicability rules | `applicability.RulesVersion`, echoed on `applicability.Results.RulesVersion` | The fixed scoring rules (base score, point deltas, per-method thresholds) — versioned once per `Results` since all five methods score under the same rule set in a single `Calculate` call |
| Consensus formula | `consensus.FormulaVersion`, echoed on `consensus.Result.FormulaVersion` | The fixed statistics/dispersion formula set (`valuation/consensus`) |
| Report schema | `report.SchemaVersion`, echoed on `report.Report.SchemaVersion` | This package's own `Report` shape — distinct from any upstream package's version, which is separately echoed inside each section |
| Settings resolution schema | `settings.ResolutionSchemaVersion`, echoed on `settings.Resolution.SchemaVersion` | The `Values`/`Sources` shape `Resolve` produces |

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

## Error taxonomy

Most packages in this repository do not return a Go `error` at all —
invalidity is communicated through `Result.Available` plus structured
`Result.Errors`/`Result.Warnings` (see each package's own section above).
Where a package does surface structured problems, it uses one of two
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

Two structured-but-not-error-severity vocabularies exist alongside these
and are not folded in, since they already serve the "stable, matchable"
purpose this taxonomy is for: `orchestrator.ExclusionReason` (why a method
never ran) and `applicability.Reason{Kind, Detail, Points}` (a scoring
contribution, not a failure). A genuine `Normalize`-level structural
failure (a malformed row) still returns a real Go `error`
(`financial.ValidationErrors`, inspectable via `errors.As`) — that
boundary is a true parse/validate failure, not a domain outcome a
`Result.Available` flag can represent.

This is deliberately a small, flat set of additions — not a new
framework — sized to what a future consuming application actually needs
to map a domain failure to a UI/API response by code.

## Development

```bash
gofmt -l .        # should print nothing
go build ./...
go test ./...
go vet ./...
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
2. **Document parsing.** CSV/XLSX tabular ingestion is now covered by
   `ingestion` (see [`ingestion`](#ingestion) above). Turning a scanned or
   born-digital **PDF** financial statement into `[]financial.RawLineItem`
   is still exactly the kind of OCR/document-understanding problem this
   repository's "no PDF/OCR" constraint rules out — see
   [Known deterministic ingestion gaps](#known-deterministic-ingestion-gaps)
   and the recommendation immediately below for why PDF is the natural
   next adapter once it's needed, and why it's deliberately not started
   here.
3. **HTTP/API and UI.** `report.Report` is JSON-serializable specifically
   so a future API handler can return it directly and a future Vue UI (or
   any other frontend) can render it — building either is explicitly out
   of scope here.
4. **AI/LLM assistance**, if ever added (e.g. suggesting adjustments, or
   explaining a report in natural language), should consume this
   repository's outputs as context, never replace its deterministic
   calculations — `applicability.Score`/`consensus.Dispersion.Score` must
   remain auditable point totals a reviewer can trace by hand, not
   something an LLM call decides.

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

- **`ingestion`'s own structural read (`Row.Kind`/`Row.Status`) is not
  binding on `financial/classification`.** `financial.RawLineItem` has no
  field for ingestion's subtotal/total/heading determination to travel on,
  so `ToRawLineItems()` only uses it to decide which rows to include/
  exclude; `classification.Classify` re-derives structural status from the
  label text independently once the row reaches it (see the `ingestion`
  section above). The two mostly agree because both use a "total"/
  "subtotal"/"net"-token heuristic, but not always — `"Gross Profit"` is a
  documented example where ingestion's broader label-shape detection
  recognizes a subtotal that classification's narrower token check does
  not. A future revision could add a `RawLineItem.HintStatus` (or similar)
  optional field that classification's structural stage consults before
  falling back to its own detection, closing this gap without either
  package needing to duplicate the other's rules.
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

## Recommended next adapter

With CSV and XLSX both covered, the next natural input adapter — and the
one every remaining realistic financial-statement source funnels through —
is **PDF**, explicitly out of scope for this repository per
[What this project intentionally does not contain](#what-this-project-intentionally-does-not-contain):
a born-digital PDF (text layer already present) is a fundamentally
different, much harder extraction problem than CSV/XLSX (no reliable
row/column grid to begin from at all — layout must be reconstructed from
absolute-positioned text runs), and a scanned PDF requires OCR, which
introduces exactly the non-deterministic, model-based uncertainty this
repository's entire design has been structured to avoid. Should a future
module take this on, it should preserve the same boundary `ingestion`
established here: PDF-specific extraction stays isolated in its own
package (e.g. `ingestion/pdf`), produces the same `ingestion.Result`/
`Row`/`Cell` shapes this package already defines rather than a competing
model, and still performs zero classification of its own — every row it
extracts flows into the exact same `financial/classification` boundary
CSV and XLSX rows do today.
