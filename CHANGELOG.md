# Changelog

All notable changes to this project, in commit order (oldest to newest).

## c11a6ca9 — analytics/qoe

Clean, complete, well-organized public API surface — exactly as designed. The package is done. Let me write up the final completion report.

**Package:** `analytics/qoe` — deterministic quality-of-earnings analysis. 14 source files (6 implementation + 8 test), 2,736 lines total.

- **`Calculate(Input, Options) Result`** — the single entry point. Recomputes `metrics.Snapshot` per period itself (never guesses chronology — falls back to dataset order when `PeriodMeta` is nil/partial), walks each period through `adjustments.Apply`, and derives everything else from that history.
- **`Input`**: `Dataset`, `PeriodMeta`, `Adjustments`, `MaintainableEBITDA`/`MaintainableSDE` (both `earnings.Result`, independently optional).
- **`Options`**: `Thresholds`, `ComputeScore` (opt-in).
- **`Result`**: `History []PeriodFigures` (reported vs. normalized EBITDA/SDE per period + full snapshot/bridge), `Adjustments AdjustmentBreakdown` (EBITDA/SDE kept separate, per-type breakdown), `Recurrence []RecurrencePattern` (repeated-one-time detection), `RecurringAdjustments RecurringSummary` (inherent recurring/non-recurring/unclassified split — added after re-reading the spec caught a gap), maintainable EBITDA/SDE, `RevenueGrowth`/`RevenueVolatility`/`EBITDAMarginTrend`/`EBITDAVolatility` (copied verbatim from `metrics.Trend`, never re-derived), `SDEVolatility` (this package's own, using metrics' identical method), `Ratios`, `Flags`, optional `*Score`, `Thresholds` (echoed), `Warnings`/`Errors`.

| Flag | Default threshold |
|---|---|
| Large normalization burden | 30% of reported EBITDA/SDE |
| Volatile earnings | 35% YoY-growth stddev |
| Inconsistent margins | 15-point swing |
| Large owner-discretionary component | 25% of normalized SDE |
| Repeated one-time adjustments | ≥2 distinct periods |
| Non-operating income supporting earnings | 20% of reported EBITDA |
| Negative/near-zero maintainable earnings | ≤$0 absolute **or** ≤2% of revenue (OR test, mirrors `review.IsMaterial`) |

Plus declining-EBITDA-despite-revenue-growth (no threshold — structural). Score: `100 − 15×critical − 8×warning − 3×info`, clamped `[0,100]`, versioned separately (`ScoreVersion`) from `FormulaVersion`, `Heuristic: true` always in the JSON itself.

`buildRecurrence`'s cross-bridge dedup was keyed on `(period, Type)` — two *different* adjustments of the same type in the same period (one EBITDA-only, one SDE-only) would silently drop the second one's amount. Fixed to dedupe on `Adjustment.ID`; added a regression test (`TestBuildRecurrence_DistinctSameTypeAdjustmentsInSamePeriodBothCounted`) that fails against the old logic. Also fixed a README table cell that would have rendered literal backslashes instead of pipe characters.

Every required archetype covered: clean stable business, high adjustment burden, repeated one-time costs, volatile earnings, declining margins, negative EBITDA, owner-heavy SDE, no adjustments, missing/partial periods, JSON round-trip, deterministic repeated execution (10× + 20× map-order stress). Coverage: **93.7%**.

`gofmt -l .` clean · `go build ./...` clean · `go vet ./...` clean · `go test ./... -race`: **965 passing, 0 failing, 9 skipped** (credential-gated, pre-existing) across all 34 packages — no regressions.

README updated: new `### analytics/qoe` section (formulas, flag table, score formula, exported-surface-by-file), package-boundaries tree entry, versioning-strategy table (+2 rows), deterministic-ordering-guarantees table (+4 rows), error-taxonomy (`qoe.Issue` as an 8th independent system + stale count fixes from before this session).

Nothing staged or committed, per instructions.

## 70d7a702 — analytics/workingcapital

Added `analytics/workingcapital`, a new package for historical operating working-capital analysis and transaction-style peg comparison, following the same conventions as the existing `analytics/qoe` sibling package.

**Design:** No hard-coded deal definition of working capital — a caller-supplied `InclusionPolicy` controls which `financial.Code` values count as operating current assets/liabilities. `DefaultInclusionPolicy()` excludes cash and short-term debt by default (financing items typically settled outside a WC peg); anything excluded is reported in `Result.ExcludedCodes`, never silently dropped.

**Core capability:**
- Per-period `PeriodNWC` (operating current assets/liabilities, NWC, revenue, NWC-as-%-revenue, full component bridge)
- `Statistics` (average, median, min/max, volatility) over both NWC and NWC-%-revenue series
- `Trend` (increasing/declining/stable, ±5% flat band)
- `SeasonalProfile` when quarterly/monthly `PeriodMeta` is present
- Five deterministic `PegMethod` strategies (latest, simple average, median, trailing-N average, fixed) plus `PegComparison` (excess/deficit vs. current NWC)

**Files:** `types.go`, `policy.go`, `workingcapital.go`, `stats.go`, `seasonal.go`, `peg.go`, plus 6 test files — 50 new tests covering seasonal business, stable/declining/increasing NWC, cash/debt inclusion/exclusion, negative NWC, missing revenue, quarterly-vs-annual comparability, every peg method, and JSON/determinism.

**Verification:** full repo — `go build`, `go vet`, `gofmt`, `go test`, and `go test -race` all clean across all 35 packages (985 tests total, no regressions). README updated with the package-boundaries diagram, a new `### analytics/workingcapital` section, and entries in the versioning-strategy, deterministic-ordering, and error-taxonomy tables. Nothing staged or committed, per instructions.

## b8d2777d — analytics/ratios

24 top-level test functions (some are table-driven with subtests, accounting for the discrepancy with the 27 seen earlier via `RUN` lines). Everything checks out. Here's a summary of the completed work.

Built `analytics/ratios`, a new deterministic financial-ratio engine, following the exact conventions of its sibling `analytics/qoe`/`analytics/workingcapital` packages.

**Package structure** (17 files, ~1,850 lines):
- `types.go` — `Input`/`Options`/`Result`, `Ratio`/`PeriodRatios`, `GrowthPoint`/`Growth`, `RatioTrend`/`Comparison`, `Signal`/`SignalCode`, `Issue`/`IssueCode`
- `policy.go` — `Thresholds`/`DefaultThresholds` (zero-value-means-defaults, matching `qoe.Thresholds`)
- `components.go`/`index.go` — Total Assets/Total Equity sum-of-codes (per your first answer), quick-assets, codeIndex
- `profitability.go`/`liquidity.go`/`leverage.go`/`efficiency.go` — all 21 ratios from the prompt (gross/operating/EBITDA/net margin, ROA/ROE, current/quick/cash ratio, debt/equity/assets/EBITDA, net-debt/EBITDA, interest coverage, asset/receivables/inventory turnover, DSO/DIO/DPO/CCC)
- `growth.go`/`trend.go` — period-over-period growth series (self-contained, not FY-restricted, per your second answer), `RatioTrend`, adjacent-period `Comparison`
- `signals.go` — 8 deterministic, threshold-configurable signals (weakening liquidity, rising leverage, margin compression, slowing collections, inventory buildup, weak interest coverage, improving/deteriorating profitability) — no composite score, per the prompt's explicit constraint

**Verification**: `gofmt`, `go build ./...`, `go vet ./...`, `go test ./...`, and `go test -race ./...` all pass clean across the whole repository (1039 tests total, up from 985). The new package's 24 test functions cover every case the prompt asked for: normal case, zero denominators, negative equity, negative earnings, missing balance sheet, multi-year trend, every signal's positive/suppressed case, custom thresholds, determinism, and JSON round-trips — all hand-verified against computed expected values.

**Docs**: Added a full `### analytics/ratios` README section (ratio table, signal table, Total Assets/Equity rationale, exported surface) plus entries in the versioning-strategy, deterministic-ordering, and error-taxonomy tables.

Nothing staged or committed, no new dependencies, no architectural-rule violations (no persistence/HTTP/AI/etc.).

## 874500b7 — analytics/cashflow

Built `analytics/cashflow`, a new deterministic package producing an EBITDA-to-free-cash-flow bridge, cash-conversion ratios, debt-service and owner-distribution coverage, recurring cash drains, and a burn/runway estimate for loss-making businesses.

**Key design decisions** (confirmed with you first):
- Since the taxonomy has no codes for a cash-flow statement, capex, debt service, or owner distributions, `Input` takes these as caller-supplied typed per-period maps (`CashFlowValue`, `DebtServiceFigure`) rather than deriving them from `financial.Code`.
- EBITDA-based cash-flow estimation is strictly opt-in via `Options.AllowEBITDAEstimate`, and every estimated figure is labeled with `IsEstimate: true` + an `EstimateBasis` string — never silently blended with reported data.

**What's in the package:**
- `types.go` — full type system (`Input`, `Options`, `Bridge`, `ConversionRatios`, `Trend`, `RecurringDrain`, `CashRunway`, `Thresholds`, `Flag`, `Issue`)
- `cashflow.go` — `Calculate` orchestration; internally recomputes EBITDA via `financial/metrics` and net-working-capital change via `analytics/workingcapital` (first cross-import between sibling `analytics/*` packages)
- `trend.go`, `drains.go`, `runway.go`, `flags.go` — supporting calculations
- 17 tests across `cashflow_test.go`, `determinism_test.go`, `roundtrip_test.go` covering every scenario the prompt asked for (strong/weak conversion, working-capital build, capex-heavy, negative cash flow/runway, missing cash-flow statement, estimate-vs-reported, debt-service/distribution coverage, JSON/determinism)

**Verification:** `go build`, `go vet`, `gofmt`, and `go test -race` all pass clean across the entire module (not just the new package) — 1046 total tests now passing.

**Docs:** README updated with a full `### analytics/cashflow` section (bridge diagram, estimate-opt-in explanation, flags table, exported surface by file), plus entries in the package-boundaries list, versioning table, determinism/ordering table, and error-taxonomy section.

Nothing was staged or committed, per your instructions.

## 089b0848 — analytics/revenuequality

Built `analytics/revenuequality` — a new deterministic package following the exact conventions of its `analytics/{qoe,workingcapital,ratios,cashflow}` siblings.

**What it produces:**
- Period-level revenue composition (recurring `CodeRevRecurring` vs. non-recurring `CodeRevProduct/Service/Other` — the taxonomy has no finer split, so the package doesn't guess one), with growth points, CAGR, and volatility
- Customer-level analysis, entirely opt-in via `Input.CustomerRevenue`: retention/new/lost/expansion/contraction between chronologically adjacent periods, plus concentration (top-N shares, HHI, segment breakdown)
- Deterministic flags (declining recurring mix, growth dependent on new customers, high lost-customer revenue, volatile revenue, one-period spike, shrinking existing base)
- Deliberately never claims SaaS-style NRR/GRR — reports raw dollar movements instead, per the prompt's instruction

**Quality bar:** 20 test functions covering every scenario the prompt named (recurring service, project business, churn, new-customer-driven growth, volatility, missing customer detail, JSON/determinism). Whole-module `gofmt`/`go vet`/`go build`/`go test -race` all clean.

A `/code-review high` pass on my own diff caught two real bugs before finishing: a map-order float64-summation nondeterminism bug in the HHI calculation, and — more significantly — an arithmetic bug where two flags reconstructed "from-period total customer revenue" with a formula that undercounted by 2×Expansion whenever a retained customer grew. Both are fixed (the second by storing the correct total directly on `CustomerTransition` instead of letting each flag re-derive it) and locked in with regression tests.

README updated: package tree, a new `analytics/revenuequality` section, versioning-strategy and deterministic-ordering table rows, and an error-taxonomy entry. Nothing staged or committed, per your instructions.

## 50f4fe70 — analytics/concentration

Built `analytics/concentration`, a new deterministic package for customer/vendor/other concentration-risk analysis, per Prompt 21.

**Key design choice**: unlike its dataset-bound siblings (`qoe`, `workingcapital`, `cashflow`, `revenuequality`), this package is fully independent of `financial.FinancialDataset` — it takes a portable `Observation{EntityKey, Period, Amount, Category}` tuple, matching the prompt's explicit ask for generic customer-revenue/supplier-spend/other input.

**What it computes**: per-period ranked entities, top-N shares (default 1/3/5/10), HHI, largest-entity share, category breakdown; first-vs-last trends for largest-share and HHI; period-over-period `DependencyChange` per entity; `Scenario`s for losing the largest entity and top-N entities (default N=3/5), with optional earnings-impact via a caller-supplied global or per-entity margin rate; five deterministic flags; structured issues; `FormulaVersion`.

**Verification**: `gofmt`, `go build ./...`, `go vet ./...`, and `go test ./... -race` all pass clean across the whole module — 21 tests in the new package covering highly-concentrated/diversified/single-customer/changing-concentration/malformed-input/top-N-scenario/JSON-determinism cases, plus a self-caught bug (the `NO_IMPACT_ASSUMPTION` warning was scoped too broadly — fixed to only consider entities a scenario actually removes).

README updated: package-boundary listing, versioning-strategy table, and a full `### analytics/concentration` section. Nothing staged or committed, per your instructions.

## 33ac1fb9 — analytics/anomalies

Built `analytics/anomalies`, a new deterministic package implementing all 11 detection rules from the prompt: absolute/percentage amount spikes, expense growth outpacing revenue, gross/operating margin deterioration, new material expense categories, accounts disappearing/reappearing, repeated suspiciously-identical values, sign flips, duplicate-like amounts across distinct accounts, an outsized owner/discretionary expense share, and unexpected negative revenue/expense amounts. Every finding uses neutral language only ("anomaly," "unusual pattern," "review recommended") — nothing is ever called fraud, per the prompt's instruction.

Two design decisions were confirmed with you up front (both recommended options): prior-period-only baselines for spike detection, and `CodeOpexOwnerComp` + optional caller-supplied codes for the discretionary-expense pool.

A `/code-review high` pass on the diff caught two real correctness bugs before calling this done, both now fixed with dedicated regression tests:
- An explicitly-reported `$0.00` amount was being treated as "account absent," causing spurious disappeared/reappeared findings on accounts that were actually reported every period.
- When the most recent period had `$0` Total Revenue, the owner-discretionary-share rule silently gave up instead of falling back to an earlier period with real revenue — meaning a genuinely high share in an earlier year could go undetected.

The review also flagged (and I fixed) a redundant per-code revenue recomputation and a duplicated sorting helper. 28 tests pass under `-race`, covering every scenario the prompt lists. The README got a full new `### analytics/anomalies` section plus the standard package-boundary/versioning/ordering/error-taxonomy updates. Whole-module suite: 1124 passing, 9 skipped, 0 failing, clean `gofmt`/`go vet`/`go build`.

## 0a0d10f0 — analytics/variance

Built `analytics/variance`, a new deterministic package implementing Prompt 23 (budget vs. actual / variance analysis):

- **Input**: a portable `LineObservation{AccountCode, Period, Actual, Baseline, BaselineType}` series (not tied to `financial.FinancialDataset`, mirroring `analytics/concentration`'s design), with `BaselineType` covering budget/forecast/prior_period/custom.
- **Direction semantics**: favorable/unfavorable is derived from `financial.CodesByCategory` (revenue up = favorable, COGS/OPEX up = unfavorable), with the mixed `CategoryOtherIncomeStatement` resolved per-code (interest income/other income vs. depreciation/interest expense/tax/other expense), plus a caller-overridable `Policy.DirectionOverrides`.
- **Calculations**: absolute/percent variance (zero-baseline handled without a fabricated percentage), materiality (`review.IsMaterial`'s OR-of-two-legs, off by default), contribution-to-total-variance, taxonomy-category and custom-category rollups, top-N favorable/unfavorable, period trends, and a total bridge (`FavorableVariance + UnfavorableVariance == TotalVariance`).
- **Tests**: 22 tests covering revenue, expense, COGS, zero baseline, negative values, missing baseline/baseline-type, unknown account codes (with/without override), multiple periods with/without `PeriodMeta`, category and custom-category rollup, materiality by dollar and by percent, the bridge identity, and JSON/determinism (including map-order stress tests) — all passing under `go build`/`go vet`/`go test -race` for the whole repo.
- **Docs**: added the package to the README's package overview, versioning table, error taxonomy, and deterministic-ordering-guarantees table, plus a full `### analytics/variance` section mirroring the `anomalies` section's structure.

## 2bbdb3b3 — analytics/forecast

Built `analytics/forecast`, implementing Prompt 24 in full: a deterministic projection/scenario engine that compounds caller-supplied assumptions (revenue growth, gross margin/COGS, opex, D&A, capex, working capital, tax, debt service) forward from a historical `financial.FinancialDataset` base period, across as many named base/downside/upside/custom scenarios as the caller wants — never predicting an assumption itself, mirroring `valuation/dcf`'s "caller forecasts, package computes" boundary.

**What it produces per scenario:** projected P&L (revenue/COGS/opex lines, gross profit/margin, EBIT, EBITDA, SDE and their margins, pretax/net income), working capital, cash flow (operating → free cash flow → free cash flow to owner), debt-service coverage, a full calculation trace, and structured warnings — plus six deterministic scenario-transformation helpers (`ApplyRevenueShock`, `ApplyMarginShock`, `ApplyExpenseShock`, `ApplyCustomerLossShock`, `ApplyOneTimeCostShock`, `ApplyDebtRateShock`), each returning a modified copy without mutating caller input.

**Two real bugs found and fixed during my own testing, before handing this off:**
1. `ApplyOneTimeCostShock` let a one-time expense silently compound into every future period's growth base — fixed by adding `OpexMethodExcludeAmount` and auto-pinning it onto the following period (you confirmed this approach over two alternatives).
2. The determinism test caught genuine floating-point nondeterminism in `projectCOGS` from summing a Go map directly instead of a sorted slice — same bug class as Prompt 20's HHI issue, now fixed and regression-tested.

Full repository: 1192 tests passing, 9 pre-existing skips, 0 failures, clean under `gofmt`/`go vet`/`go build`/`go test -race`. README updated with a new `analytics/forecast` section, directory listing, versioning table, error taxonomy, and ordering guarantees. Nothing staged or committed, per your instruction.

## 9ed5fb83 — analytics/debt

Built `analytics/debt`, implementing Prompt 25 in full — a deterministic debt service coverage, leverage, and debt capacity analysis package:

- **Amortization** (`amortization.go`): level-payment formula with interest-only period support, including a straddled-first-year blend when the I/O period isn't a whole number of years.
- **Coverage** (`coverage.go`): DSCR (cash-flow-preferred over EBITDA), fixed-charge coverage, interest coverage, debt/EBITDA and net-debt/EBITDA leverage.
- **Capacity** (`capacity.go`): maximum debt under a caller minimum DSCR (linear-scaling solve against the proposed loan's pricing terms), maximum debt under a leverage cap (gross/net, picks the tighter), combined capacity + headroom, with no invented default cap when no policy is supplied.
- **Scenarios** (`scenarios.go`): caller-defined downside stress cases with debt service held fixed.
- **Flags/Issues** (`flags.go`, `types.go`): package-local `IssueCode`/`FlagCode` taxonomies, following repo convention.

Followed `analytics/concentration`'s precedent rather than `analytics/cashflow`'s: this package is dataset-independent (no `financial.FinancialDataset` dependency), since debt capacity work is usually run against a caller's already-normalized EBITDA figure.

23 new tests (amortization math, zero/negative EBITDA, interest-only, DSCR/leverage caps, downside scenarios, invalid terms, JSON round-trip, determinism) — all pass, including `-race`. A test caught a real bug: `DebtToEBITDA`/`NetDebtToEBITDA` were gated on `!= 0` instead of `> 0` as documented; fixed. `gofmt`, `go vet`, `go build`, and the full repo test suite (`go test ./...`) are all clean with no regressions. README updated with a full `analytics/debt` section, a versioning-strategy table entry, and an error-taxonomy entry.

## e506eff7 — analytics/covenants

Built `analytics/covenants`, implementing Prompt 26 in full: a deterministic financial covenant monitor.

**Package** (`types.go`, `evaluate.go`, `covenants.go`) — `Calculate(Input) Result` evaluates a batch of caller-supplied `CovenantTest` rows (ID, metric, operator `>=`/`<=`/`>`/`<`/`=`, threshold, already-computed actual value, period, optional cure/grace metadata, optional warning buffer) and returns per-test pass/fail/unavailable status, direction-aware headroom, warning-buffer (near-breach) classification, a deterministic explanation sentence, and period, plus an aggregate `Summary` of breaches/near-breaches/unavailable tests with a per-period breakdown. Like `analytics/debt`/`analytics/variance`, it's independent of `financial.FinancialDataset` and never computes any metric itself — callers pass in already-calculated DSCR, leverage, ratio, or custom figures.

This was the first prompt needing a generic comparison operator, so `covenants.Operator` is a new taxonomy (nothing to reuse — `analytics/variance` had solved its similar problem with a domain-specific bool instead).

**Tests**: 19 top-level tests (21 with subtests) covering pass, breach in both directions (min-style and max-style), near breach (both warning-buffer legs), unavailable metric, wrong/empty operator, missing/duplicate covenant ID, custom metric, cure/grace passthrough, multiple periods, the equality operator's headroom-unavailable rule, no-input-mutation, and JSON/determinism. A medium-effort code review caught one real bug — `formatNumber` rendered tiny negative magnitudes as the misleading `"-0"` in explanation strings — which I fixed and covered with a regression test. All pass with `-race`; full repo suite (1555 tests) is green with no regressions.

**Docs**: README updated with a package-boundaries tree line, a full `### analytics/covenants` section (mirroring `analytics/debt`'s structure), and a new versioning-strategy table row for `covenants.FormulaVersion`.

Nothing staged or committed, per the prompt's instruction — nothing else in the working tree was touched.

## b295f85c — analytics/benchmarks

Built `analytics/benchmarks` implementing Prompt 27 in full: a deterministic peer/industry benchmark comparison engine.

**What it does:** compares caller-supplied company metrics against caller-supplied benchmark data in any of four forms — median only, percentile bands, quartiles (Q1/Median/Q3), or explicit peer observations. For each metric it reports the benchmark median and known range, an estimated percentile/quartile band for the company's value (via bidirectional piecewise-linear interpolation, or rank-based percentile for peer observations), difference and relative difference, favorable/unfavorable classification (only when the caller supplies a direction), and full provenance (source name, effective date, population/segment, sample size, license ID). It never sources or redistributes benchmark data itself — everything comes from the caller.

**Design:** dataset-independent like `analytics/debt`/`analytics/covenants` — no `financial.FinancialDataset` dependency. Follows repo conventions throughout: local `Value` availability type, `IssueCode`/`IssueSeverity` structured issues, `FormulaVersion`, deterministic ordering, no mutation of caller input.

**Bug caught by code review:** the initial implementation assumed benchmark points were always value-ascending with percentile. A caller supplying a "lower is better" table in descending order (e.g., Q1=80, Median=50, Q3=20) would silently produce an inverted range and wrong band classification with no warning. Fixed by validating monotonicity explicitly and added `IssueNonMonotonicBenchmarkPoints`, which leaves percentile/band/range unavailable rather than reporting self-contradictory output — with two regression tests confirming the fix.

**Verification:** gofmt clean, `go build`/`go vet`/`go test -race` all pass across the entire repo (962 tests total, 27 new in this package). README updated with the package-boundary entry, a full documentation section, and a versioning-strategy table row. Nothing staged or committed, per your instructions.

## 8ce24c4f — analytics/valuedrivers

Built `analytics/valuedrivers`, implementing Prompt 28 in full: a deterministic business-value-driver/scenario engine.

**Architecture:** the package takes a baseline `valuation/orchestrator.Request` + `consensus.Options`/weights + caller-defined `Driver`s/`Scenario`s, deep-copies the request, mutates only the same typed `Input` fields a caller could change by hand (`sde.Input.MaintainableSDE`, `ebitda.Input.Multiple`, `dcf.Input.DiscountRate`, bridge debt/cash fields, etc.), and re-runs `orchestrator.Execute`/`consensus.Calculate` verbatim — no new valuation formula anywhere, per the prompt's core requirement.

**11 driver types** cover every example in the prompt (revenue growth, margin change, SDE change, method multiple, cap rate, DCF rates, debt change, working-capital change, owner-comp addback, customer-loss impact, and an explicit method-multiple rule). Every driver carries a `Linkage` (`Applied`/`NotApplicable`/`MethodExcluded`) per method so "no effect" is never ambiguous — this is what stops the package from ever claiming causation a valuation method doesn't actually have (the recurring-revenue-% example requires the caller to supply the resulting multiple explicitly).

**One-factor-at-a-time and combined scenarios** are both supported and reported separately (`Result.OneFactorAtATime` vs. `Result.Scenarios`), sharing one `ScenarioResult` shape.

A high-effort 8-angle code review caught one real confirmed bug (`MethodMultipleRuleParams.Method` had no validation despite being documented "Required") plus three plausible design smells (an unprotected `Applicability` field in the deep-copy, a fragile synthetic-`Driver` re-dispatch pattern, and a `LinkageApplied`-with-no-evidence edge case) — all four fixed with regression tests.

**Verification:** 27 new tests, 1288 passing repo-wide, full `gofmt`/`go build`/`go vet`/`go test -race` clean across the whole module. README updated (package tree, full `### analytics/valuedrivers` section, versioning-strategy table row).

## f2d9a0c5 — transactions/acquisition

Built `transactions/acquisition`, implementing Prompt 29 in full — a deterministic acquisition-screening package with no persistence, HTTP, auth, billing, UI, background jobs, or AI/LLM dependencies, exactly as the architectural rules require.

**What it does:** given a target's normalized EBITDA/SDE, an already-computed consensus valuation, an asking price, financing (debt tranches via `analytics/debt.LoanTerms`, buyer cash), fees, working capital, capex, and buyer-compensation assumptions, `Calculate` derives:
- price/revenue, price/EBITDA, price/SDE multiples
- premium/discount vs. consensus (neutral, signed arithmetic — never "over/underpriced")
- sources-and-uses and required equity contribution
- annual debt service, DSCR, post-debt cash flow, leverage (via `analytics/debt.Amortize` — reused, not reimplemented)
- cash-on-cash return and simple payback period
- downside/upside scenarios (revenue/EBITDA/SDE haircuts, financing held fixed)
- red flags gated entirely on caller-supplied `RedFlagThresholds` — no invented defaults, no composite score, no buy/don't-buy verdict

**Files:** `types.go`, `acquisition.go`, `pricing.go`, `scenarios.go`, `flags.go`, plus `acquisition_test.go` (14 unit tests covering all-cash/leveraged/seller-note deals, price above/below valuation, weak DSCR, negative cash flow, downside/upside shocks, SDE fallback, capex/comp deductions, invalid loan terms, empty input, no-thresholds), `determinism_test.go`, and `roundtrip_test.go`.

**Verification:** `gofmt`, `go build ./...`, `go vet ./...`, `go test ./...` (1305 passing, up from 1288), and `go test -race ./...` all clean across the whole module. README updated with the package-boundary diagram entry, a full documentation section, and a versioning-strategy table row. Nothing staged or committed, per instructions.

Two test-assertion bugs were caught and fixed while writing tests (not package bugs): an all-cash deal correctly reports `Leverage` as a known `0x`, not unavailable (zero debt is a known figure, same convention `analytics/debt` already uses); and an initial 50% EBITDA downside haircut wasn't steep enough to flip cash flow negative given the test's loan terms, so it was bumped to 65%.

## 52375698 — transactions/dealstructure

Confirmed — nothing staged or committed, exactly as instructed. Note: "ahead of origin by 1 commit" refers to a pre-existing local commit from before this session (the `f2d9a0c` acquisition commit noted in the git status snapshot), not anything from this work.

Built `transactions/dealstructure` implementing Prompt 30 in full — a deterministic acquisition-financing-structure package with no persistence, HTTP, auth, billing, UI, background jobs, or AI/LLM dependencies.

**Package** (6 files, ~1,000 lines): `types.go`, `amortization.go`, `sourcesanduses.go`, `earnout.go`, `annualdebtservice.go`, `dealstructure.go` — entry point `Build(Input) Result`.

**Key design decision**: `analytics/debt.Amortize` only computes first-year/steady-state debt service with no balloon support, so this package built its own amortization engine (same level-payment formula, extended with a full per-period schedule and balloon sizing) rather than force an awkward dependency — documented explicitly in the package doc comment.

**Two real bugs caught by the test suite before shipping**, both fixed with regression tests:
1. A `BalloonAmount` explicitly set on a tranche was sized against the wrong horizon (`AmortizationYears` instead of `TermYears`), producing a materially wrong balloon figure.
2. A tranche whose entire `TermYears` fell inside its own `InterestOnlyYears` (e.g., a 2-year interest-only bridge loan due at year 2) was left with a nonzero `EndingBalance` and no `Balloon` recorded — the loan was never resolved at maturity.

**Testing**: 27 tests covering single loan, multiple tranches (differing rates/frequencies/terms), seller note, explicit and implicit balloons, interest-only periods (including the full-term edge case), funding gap/surplus, every invalid-input variant, suspicious interest rates, earnout scheduling/sorting, zero-rate loans, fees/working-capital/closing adjustments, no-mutation, and full JSON round-trip/determinism for both `Input` and `Result`. 97.6% coverage; `gofmt`/`go vet`/`go build`/`go test -race` all clean repo-wide.

**Docs**: added a `### transactions/dealstructure` README section (package listing, design rationale, file map, test coverage) and a `FormulaVersion` row in the versioning-strategy table, mirroring `transactions/acquisition`'s exact documentation structure.

## 445d8ce0 — transactions/salereadiness

Built `transactions/salereadiness`, implementing Prompt 31 in full: a deterministic sale-readiness assessment package that aggregates whichever of six optional upstream results a caller has (`analytics/qoe`, `analytics/workingcapital`, `analytics/concentration`, `analytics/revenuequality`, `valuation/consensus`, `financial/metrics`) plus a `valuation/profile.Profile` and its own `DataQuality`/`Policy` inputs.

**What it produces:** 11 fixed dimensions (financial record quality, earnings stability, normalization burden, customer concentration, recurring revenue, owner dependence, margin trend, working-capital stability, debt/leverage, data completeness, valuation-method consensus), each classified `Strong`/`Acceptable`/`Weak`/`Concerning`/`Unassessed`; Blockers/Risks/Strengths/MissingInformation/Opportunities derived from those classifications; a `Coverage` summary; and an optional explicit-formula overall score (never computed with zero assessed dimensions).

**Key design choices:**
- Missing inputs classify `StatusUnassessed` and are excluded from coverage/scoring — never silently scored as a concern.
- A hand-picked `blockingDimensions` subset (not every `StatusConcerning`) produces actual Blockers; a fixed structural check catches negative earnings independent of any threshold.
- The overall score is a simple, documented formula over assessed-dimension Statuses only, mirroring `qoe.Score`'s "weight, don't recompute" precedent.

**Verification:** 13 tests (ready/stable, owner-dependent, concentrated, poor records, missing modules, negative earnings, no-Policy fallback, empty input, no-mutation, determinism, dimension-order, full JSON round-trip) all pass with `-race`; `gofmt`/`go vet`/`go build`/`go test` are clean across the whole module. README updated with a full narrative section, versioning-table rows, deterministic-ordering rows, and an error-taxonomy entry.

## eb399cc4 — analytics/consolidation

Built `analytics/consolidation`, implementing Prompt 32 in full: a deterministic multi-entity financial consolidation engine that combines multiple `financial.FinancialDataset`s into one consolidated dataset using caller-supplied ownership/currency/elimination information — the first analytics package whose primary output is itself a dataset rather than a bespoke analysis result.

**What it does:** eliminates intercompany entries (native currency) → converts currency (only with caller-supplied rates) → applies ownership weighting (full or explicit ownership-weighted mode), producing a consolidated `FinancialDataset` with per-item provenance back to source entities, plus entity contribution summaries, applied eliminations, currency conversions, and structured reconciliation issues. It never fetches FX rates, never infers eliminations, and never guesses which periods are common — all three are explicit caller inputs per the prompt.

**Quality process:** a code-review pass (high effort) caught two real bugs before this was reported done — an out-of-scope-period elimination silently marked "applied" with no actual effect, and a silent-empty-currency path that would have produced an invalid `FinancialDataset.Currency == ""`. Both are fixed and regression-tested. Combined with a gap I caught myself while writing eliminations logic, that's 3 real correctness issues closed pre-delivery.

**Verification:** 26 tests covering every scenario the prompt requires (two entities, eliminations on both sides, different period coverage, different currencies with/without rates, ownership weighting, JSON round-trip/determinism including a 15-entity map-order stress test) — all passing, clean under `-race` ×3, and the whole repo (1300+ tests) still builds/vets/tests clean. README updated with a full narrative section, versioning row, 8 deterministic-ordering rows, and an error-taxonomy entry.

## 8215a43d — portfolio/diagnostics

Built `portfolio/diagnostics`, implementing Prompt 33 in full: a deterministic package that scans a portfolio of `BusinessSnapshot`s (one per client/business/period) and returns ranked `Finding`s for margin deterioration, revenue decline, cash-conversion weakening, leverage increase, concentration increase, valuation movement, unresolved financial quality, and sale-readiness opportunity.

**Architecture** (confirmed with you first): `BusinessSnapshot` embeds condensed `*Summary` structs (`QoESummary`, `RatioHealthSummary`, `ConcentrationSummary`, `CashFlowSummary`, `ValuationSummary`, `SaleReadinessSummary`) rather than full sibling `Result` types — keeps a large portfolio's payload proportional to what's needed, matching Prompt 33's "selected metrics, QoE summary, ..." wording. It sits one level above `transactions/salereadiness`: many businesses from small summaries, vs. one business from full results.

Files: `types.go`, `findings.go` (8 `detect*` rules), `score.go` (configurable `PriorityScore` formula + deterministic sort), `diagnostics.go` (`Calculate` orchestration), plus `fixtures_test.go`, `diagnostics_test.go`, `determinism_test.go`, `roundtrip_test.go` — 21 tests covering multi-client portfolios, missing modules, equal-severity/deterministic ordering, trend changes, no-`Prior` signal-only paths, custom policy thresholds/weights, no-mutation, JSON round-trip, determinism, and concurrency.

A code-review pass (medium level) caught and I fixed three real issues: a scoring bug where an explicit `SeverityWeights[X] = 0` was silently overridden by the default weight (added a regression test), duplicated change/severity math in `detectValuationMovement` refactored to reuse shared helpers, and a misleading doc comment on `SelectedMetrics.Extra`.

`go build`, `go vet`, `go test -race` all clean across the whole repo. README updated with a new package-tree line and a full `### portfolio/diagnostics` section matching the `salereadiness` section's style.

## 4a890b05 — reporting/management

Summary
Built reporting/management, implementing Prompt 34 in full — a deterministic management-reporting data pack that assembles presentation-neutral output from up to 13 optional sibling analytics/valuation results:

Calculate(Input) Report produces 10 fixed sections in a stable order: executive KPI summary, historical/profitability/liquidity-leverage/cash-flow/working-capital series, variance/forecast tables, a merged top-issues rollup, and chart-ready series with historical+forecast points.
No PDF/HTML/chart rendering, no narrative AI — every string is either passed through or built from a short fixed template.
Computes no figure of its own, with one documented exception: it recomputes metrics.Calculate internally when only a raw FinancialDataset is supplied (no pre-built Metrics.Result).
Follows this repo's established conventions exactly: own Value/Issue/HasErrors copy, FormulaVersion const, fixed SectionCode ordering mirroring orchestrator.Run.Methods.
Two rounds of code review (medium effort) caught and fixed 5 real bugs: a silent chronological-ordering loss with no warning, an ExecutiveSummary.Period that ignored 6 of 7 KPI sources, a doc comment promising a nonexistent Debt fallback (fixed by wiring Debt.Flags into TopIssues instead, since analytics/debt.Result turns out to have no Period field at all), a fragile string-switch in chart-series forecast dispatch, and a mislabeled chart Source field on forecast-only input.

Final state: 34 tests passing (94.1% coverage), gofmt/go vet/go build/go test -race all clean across the entire repository. README updated with a package-index entry, a versioning-strategy row, 6 new deterministic-ordering rows, an error-taxonomy entry, and a full narrative section matching the portfolio/diagnostics template.

## d9f128a3 — analytics/diagnostics

Summary
Built analytics/diagnostics, implementing Prompt 35 in full: a deterministic single-business diagnostic engine that mines up to 15 optional already-computed sibling Results (financial/metrics through transactions/salereadiness) for their own Flags/Signals/Anomalies/Status classifications and republishes them as Findings across 12 fixed categories, split into Strengths/Concerns/Opportunities, plus module Coverage, MissingDataAreas, and an optional explicit-formula OverallHealthScore. No new financial calculation, no AI narrative layer.

A code-review pass caught and fixed 6 real bugs before completion — most notably a FindingCode collision between debt-free businesses and genuine DSCR breaches, and between sale-readiness Strengths and Opportunities — each with a regression test. Final state: 14 source/test files, 27 tests, 96.1% coverage, gofmt/go vet/go build/go test/go test -race all clean across the entire 1452-test repository. README updated with a package-tree entry, full prose section, versioning-strategy rows, deterministic-ordering rows, and an error-taxonomy entry.
</content>
</invoke>
