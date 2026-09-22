# go-valuate

Reusable, standalone Go domain modules for business valuation: a canonical
financial data model, a deterministic normalizer, a deterministic
rule/alias-based classifier, and a hierarchical valuation-settings resolver.

This is a **library of pure domain logic**, developed independently of any
larger application. The intent is that its packages will later be copied or
moved into a bigger Go application once they've proven out. Until then it
stays self-contained on purpose: no database, no HTTP, no auth, nothing
tying it to a particular product's infrastructure.

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

The design rule is:

```
Go structs / JSON-compatible data in
  → deterministic domain processing
  → Go structs / JSON-compatible data out
```

No global state. No infrastructure dependencies. No side effects. Every
exported function in this repository is a pure function of its inputs.

Deterministic, rule/alias-based classification (deciding *which* canonical
code a raw row maps to) is implemented, in `financial/classification` — see
below. Reconciliation, valuation formulas (SDE/EBITDA multiples, DCF,
adjusted net asset value, etc.), AI/LLM-assisted or statistical
classification, AI-assisted extraction, PDF/XLSX/CSV parsing, and
persistence are all still explicitly **out of scope for this repository** at
this stage. They are expected to be built as later modules, or in the
consuming application, on top of the types and packages defined here — see
[Recommended next module](#recommended-next-module).

## Package boundaries

```
go-valuate/
  financial/                canonical financial model, taxonomy, and normalizer
  financial/classification/ deterministic raw-row -> canonical-code classifier
  settings/                 generic hierarchical settings resolver
  fixtures/                 example JSON matching the Go types, used by tests
                            as living documentation
```

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
long-term. Display labels and category groupings are kept as separate
metadata (`CodeMeta`, looked up via `LookupCode`) specifically so that
copy can be revised without ever changing a code's wire value.

The taxonomy is currently a flat namespace, but it's structured (one
`CodeMeta` entry per code, keyed by a single stable string) so a hierarchical
version — e.g. adding a `Parent Code` to `CodeMeta` — can be introduced later
without changing any existing code's identifier.

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
   account/client/valuation and for `Settings`, snapshotting `Resolution`
   and classification `Result`s), HTTP handlers, auth, multi-tenancy,
   document parsing (PDF/XLSX/CSV/QuickBooks), and actual valuation formulas
   built on top of `FinancialDataset` and a resolved `Resolution`. It may
   also eventually supply a statistical or AI/LLM-based classifier that
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

## Development

```bash
gofmt -l .        # should print nothing
go build ./...
go test ./...
go vet ./...
```

## Recommended next module

With `financial`, `financial/classification`, and `settings` in place, the
next natural addition is a **valuation formulas** package (e.g.
`valuation/`) that consumes a `financial.FinancialDataset` plus a resolved
`settings.Resolution` and computes actual valuation outputs — SDE, EBITDA
multiples, DCF, capitalization of earnings, adjusted net asset value, per
the methods already modeled in `settings.Method`. It should remain just as
pure and infrastructure-free as the existing packages: dataset + resolution
in, valuation results out, no persistence, no UI, no knowledge of where the
dataset or settings came from.

A **reconciliation** package (detecting when reported totals/subtotals in a
source statement don't match the sum of their classified children) is a
reasonable alternative next step if valuation formulas aren't ready to be
designed yet, since `financial/classification`'s structural detection
(`Result.Status` = subtotal/total) now provides the raw material for it.
