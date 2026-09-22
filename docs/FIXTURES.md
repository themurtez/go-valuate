# Test Fixture Catalog

This module's test suite is built almost entirely on synthetic, non-
proprietary fixture data, reused deliberately across many packages'
tests rather than each package inventing its own one-off data. This
document catalogs the major fixture families so a future engineer adding a
test knows which existing fixture to reach for before creating a new one.

See [`V1_CONTRACTS.md`](V1_CONTRACTS.md) for the pipeline these fixtures
exercise, and [`INTEGRATION.md`](INTEGRATION.md) for how the pipeline is
meant to be orchestrated end to end.

## The four business archetypes

The backbone of the test suite: **HVAC** (small owner-operated service
business), **agency** (professional services), **manufacturer**
(asset-heavy), and **SaaS** (growth-stage software) recur as the *same*
underlying synthetic entities at every pipeline stage, each getting
progressively richer data moving downstream. No proprietary or customer
data is used anywhere in any fixture in this repository.

### Stage 1 — raw/unclassified

`fixtures/raw_line_items_by_business_type.json` — consumed by
`financial/classification/fixtures_test.go`. Proves correct classification
*and* correct `UNKNOWN` behavior on genuinely ambiguous labels, across all
four archetypes' deliberately messy raw rows.

### Stage 2 — normalized, three-fiscal-year

One JSON file per archetype:

- `fixtures/normalized_hvac_multi_year.json`
- `fixtures/normalized_agency_multi_year.json`
- `fixtures/normalized_manufacturer_multi_year.json`
- `fixtures/normalized_saas_multi_year.json`

Consumed by `financial/metrics/fixtures_test.go`,
`financial/reconciliation/fixtures_test.go`,
`financial/adjustments/fixtures_test.go`,
`financial/earnings/fixtures_test.go`, every `valuation/*/fixtures_test.go`
(ebitda, sde, capitalization, dcf, netassets), and both
`valuation/e2e/*_test.go` files. Proves multi-period trend/growth
calculations and realistic reconciliation checks — each balance sheet
balances exactly by construction (assets == liabilities + equity, with
owner/shareholder equity as the derived plug).

### Stage 3 — confirmed adjustments

`fixtures/adjustments_by_business_type.json` — consumed by
`financial/adjustments/fixtures_test.go` and both `valuation/e2e` tests.
Per archetype: HVAC = owner comp normalization + personal vehicle +
one-time repair; agency = owner salary normalization + one-time rebranding
cost; manufacturer = unusual repair + related-party rent; SaaS =
non-recurring professional fees + non-operating income removal.

### Stage 4 — valuation assumptions

`fixtures/valuation_by_business_type.json` — consumed by every
`valuation/*/fixtures_test.go` package and both `valuation/e2e` tests.
SDE/EBITDA multiples, a capitalization rate, a DCF forecast (discount +
terminal growth), and itemized adjusted-net-asset-value inputs including
the manufacturer's appraisal-override fixed-asset item. **Design property
worth preserving**: each method package's own test derives maintainable
SDE/EBITDA *live* via `financial/metrics.Calculate` over the matching
`normalized_<archetype>_multi_year.json` file rather than hardcoding the
expected figure, so expected outputs can never silently drift from the
upstream fixture.

### Full pipeline runs

- `valuation/e2e/e2e_test.go` (`TestEndToEnd_HVAC`) — owner-operated,
  SDE-dominant applicability profile.
- `valuation/e2e/e2e_asset_heavy_test.go` (`TestEndToEnd_Manufacturer`) —
  proves high asset-intensity pushes Adjusted Net Asset Value to HIGH/MEDIUM
  applicability (opposite of HVAC), exercises the asset→equity value-basis
  conversion path, and shows single-period-only adjustments legitimately
  selecting `earnings.StrategyLatestPeriod` over `StrategySimpleAverage`.

Both explicitly reuse existing fixtures rather than inventing new data — a
deliberate repo convention worth preserving in future tests.

### Archetype-driven scoring assertions (no JSON fixture)

`valuation/applicability/applicability_test.go` builds hand-written
`profile.Profile{}` literals per archetype
(`TestCalculate_OwnerOperatedServiceBusiness`,
`TestCalculate_AssetHeavyManufacturer`,
`TestCalculate_HighGrowthCompanyWithForecast`/
`...WithoutForecast_DCFNotApplicable`) — the applicability-package-local
equivalent of the shared JSON archetypes, proving the scoring rules respond
correctly to profile shape.

## Other JSON "living documentation" fixtures (`fixtures/`)

- `mapped_income_statement.json` + `normalized_dataset.json` — a minimal
  worked example (2 source rows aggregating into 1 code), run through
  `Normalize` and byte-compared in `financial/fixtures_test.go` to keep the
  README's own "Example: input → output" section honest against actual
  code behavior.
- `normalized_inconsistent_balance_sheet.json` — a single-period dataset
  with a deliberate $50,000 balance-sheet imbalance, the one fixture in
  this family that's supposed to be wrong. Exercises the `FAIL` path of
  `CheckBalanceSheetBalances` in `financial/reconciliation/fixtures_test.go`.
- `raw_income_statement.json` — a smaller, earlier raw example (pre-dates
  the four-archetype family), still referenced for basic mapping examples.
- `settings_resolution.json` — lays out the four-scope settings precedence
  example (system/account/client/valuation) as data, mirroring the worked
  example in the README. Consumed by `settings/resolver_test.go`.

## Ingestion CSV/XLSX corpus (`ingestion/fixtures/`)

Already fully documented in the README's [`ingestion`](../README.md#ingestion)
section; summarized here for the catalog:

- `quickbooks_pl.csv` — QuickBooks-style P&L, nested Income/COGS/Expense
  sections + Gross Profit subtotal. Proves structural-subtotal survival
  (`SourceStructural`, never an ordinary account) via
  `TestStructuralContract_LabelsSurviveAsStructuralNotOrdinaryAccounts` in
  `ingestion/integration_test.go` — the fixture behind the "Gross Profit
  doesn't become an ordinary account" guarantee.
- `accountant_custom_pl.csv` — FY2024/FY2025 columns, parenthetical
  negative interest expense.
- `balance_sheet.csv` — nested current/fixed asset and liability sections.
- `multi_year_saas_pl.csv` — four-year SaaS P&L (the CSV-ingestion
  counterpart to the JSON SaaS archetype).
- `malformed_numeric_values.csv` — every malformed-numeric case at once
  (percent signs, "N/A", double-negative parens, etc.).
- `sparse_blank_rows.csv` — blank/separator rows throughout, proves
  blank-row exclusion without ID-renumbering drift.
- `multi_sheet_workbook.xlsx` — three-sheet workbook (low-plausibility
  "Notes" + Income Statement + Balance Sheet) exercising XLSX sheet
  auto-selection scoring.
- `ambiguous_workbook.xlsx` — two sheets deliberately tied in plausibility,
  proves `MULTIPLE_PLAUSIBLE_SHEETS` (never a silent guess).
- `totals_subtotals.xlsx` — nested totals/subtotals with indentation,
  exercises `ParentTracker`/indent-aware structural detection.

## Ingestion text-PDF corpus (`ingestion/fixtures/`)

Generated by `ingestion/fixtures/gen` (a hand-rolled Helvetica PDF writer,
chosen over a second PDF-writing dependency — see
[`DEPENDENCIES.md`](DEPENDENCIES.md)):

`simple_pl.pdf`, `multi_year_pl.pdf` (three period columns),
`multi_page_pl.pdf` (header repeats on page 2, parent context spans the
page break), `balance_sheet.pdf` (nested sections + grand total),
`pl_and_balance_sheet.pdf` (two statements in one PDF — the "Option A"
multi-statement fixture), `negative_parentheses.pdf` (the `"( 1,234 )"`
spaced-parens extraction quirk), `indented_sections.pdf` (X-offset
indentation), `unusual_spacing.pdf` (every numeric-extraction quirk at
once), `image_only.pdf` (zero text objects — proves `OCR_REQUIRED`
detection without needing a real raster scan), and `ambiguous_layout.pdf`
(scattered text, no row/column alignment — proves `PDF_LAYOUT_AMBIGUOUS`/
`COLUMN_ALIGNMENT_AMBIGUOUS`).

## OCR ambiguity / low-confidence corpus (`ingestion/fixtures/`)

Generated by `ingestion/fixtures/gen`'s `generate_ocr_pdf.go` +
`scan_image.go`:

- `scanned_pl.pdf`, `scanned_balance_sheet.pdf` — baseline scanned
  statements.
- `scanned_low_resolution.pdf` — deliberately half-resolution; proves OCR
  still succeeds with a warning rather than failing outright.
- `scanned_skewed.pdf` — each line shifted slightly right; proves mild-skew
  tolerance via row-grouping Y-tolerance.
- `scanned_negative_parentheses.pdf`, `scanned_multi_year.pdf`,
  `scanned_multi_page.pdf`.
- `mixed_text_and_scanned.pdf` — real embedded text on page 1, a genuine
  scan on page 2 (proves per-page OCR-required decisioning, not a
  whole-document flag).
- `scanned_with_logo.pdf` — two embedded images (a logo plus the dominant
  page scan); proves the dominant-image selector isn't confused by a
  non-content image.
- `scanned_ambiguous_numeric.pdf` — a deliberate letter-for-digit OCR
  confusion baked into the fixture's own rendered text, for real-Tesseract
  testing specifically. The equivalent unit-level ambiguity tests use
  `ocr.FakeEngine` rather than this fixture directly, proving ambiguous OCR
  numerics are rejected outright rather than silently guessed.

Every OCR path is unit-testable without a real Tesseract install via
`ocr.FakeEngine` (not test-file-gated — see its own package doc comment);
real-Tesseract integration tests self-skip cleanly when no executable
resolves.

## AI classification fixtures (`financial/classification/ai/`)

No JSON fixture file — fixtures are hand-built `financial.RawLineItem{}`/
`classification.Config{}` literals per test in
`financial/classification/ai/orchestrate_test.go`. Scenario coverage:
disabled-by-default policy, unknown-only fallback skip logic, closed-set
request construction, invalid/out-of-range-confidence rejection,
structural-row skip (and forced-non-skip), deterministic-vs-AI
disagreement, provider error/timeout/rate-limit isolation, batch order
preservation + per-row failure isolation, cost-control budgets, strict-mode
abandonment.

## AI adjustment fixtures (`financial/adjustments/ai/`)

Central shared fixture: `baseRequest()` in
`financial/adjustments/ai/validate_test.go` — a hand-built `Request` with
eight candidate rows covering a legal settlement, an owner auto lease, a
gain on asset sale, a fire-damage loss, officer compensation, related-party
rent, a heading row, and an ambiguous-OCR row. Every scenario in that
file's ~25 test functions draws from this one fixture, proving: invented
amounts rejected, wrong-period rejected, structural rows rejected,
ambiguous-OCR-amount rows rejected, duplicate suggestions' second
occurrence rejected, non-finite amounts rejected, and
owner-comp-normalization/related-party-rent specifically flagged
`RequiresUserInput` rather than auto-applied. A separate fixture family in
`candidates_test.go` proves structural/ambiguous-row exclusion, explicit
row-ID overrides, `MaxRows` truncation ordering, and input-never-mutated.
`batch_test.go` proves request-chunking by row count and character budget,
and that context rows sent to a provider never carry dollar amounts.

`review/ai_adjustment_integration_test.go` wires this package into
`review/` end to end (candidate selection → fake suggester → validation →
`review.Build`/`Apply` → `financial/adjustments.Apply`) — see
[`V1_CONTRACTS.md § 4a`](V1_CONTRACTS.md#4a-optional-ai-adjustment-suggestions).

## Canonical V1 end-to-end fixtures

`examples/full_flow` and the canonical end-to-end test
(`review/e2e_v1_contract_test.go`) build their own small, self-contained
synthetic income statement inline (two fiscal years, HVAC-style labels)
rather than reusing the four-archetype JSON family, specifically so the
full contract — including ingestion, both optional AI capabilities via
fakes, and JSON serialization of the final report — is demonstrated from
literal source bytes forward, matching what a real integrating application
would actually receive. See [`V1_CONTRACTS.md`](V1_CONTRACTS.md) and
[`INTEGRATION.md`](INTEGRATION.md) for the full flow this exercises.
