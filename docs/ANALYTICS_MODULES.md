# Analytics Module Catalog

A per-package reference for the analytics/transactions/portfolio/reporting
expansion built on top of the V1 valuation core (see
[`V1_CONTRACTS.md`](V1_CONTRACTS.md) for that core, and
[`ANALYTICS_ARCHITECTURE.md`](ANALYTICS_ARCHITECTURE.md) for how these 20
packages relate to each other and to the core).

Every package listed here exposes a single deterministic entry point
(`Calculate`, or `Execute`/`Build` for the two packages that use a
different verb — noted below), takes a package-local `Input` struct, and
returns a package-local `Result` struct. None of the 20 packages performs
I/O, database access, or network calls; none has package `init()` side
effects or mutable global state (see
[`ANALYTICS_ARCHITECTURE.md` § Dependency graph](ANALYTICS_ARCHITECTURE.md#dependency-graph)
for the verification).

**Column meanings:**
- **FinancialDataset required?** — does `Input` embed a raw
  `financial.FinancialDataset`, or does the package work entirely from
  caller-precomputed figures/upstream `Result`s?
- **Customer-level data?** — does the package accept/require per-customer
  observations (as opposed to only period-level totals)?
- **Calculates vs. aggregates** — does the package perform its own
  financial arithmetic from raw or near-raw inputs ("calculates"), or does
  it primarily read/reclassify/rank already-computed sibling `Result`s
  ("aggregates")? Several packages do both to different degrees; the value
  here describes the dominant mode.

## Financial analytics

| Package | Purpose | Primary Input | Primary Result | Sibling deps | Dataset required? | Customer data? | Mode | Version const |
|---|---|---|---|---|---|---|---|---|
| `analytics/qoe` | Quality-of-earnings analysis: reported vs. normalized EBITDA/SDE history, adjustment breakdown, recurrence detection, deterministic flags, optional score | `Input{Dataset, PeriodMeta, Adjustments, MaintainableEBITDA/SDE}` | `Result{History, Adjustments, Recurrence, Ratios, Flags, Score?}` | none (financial-only) | Yes | No | Calculates | `FormulaVersion` (+ `ScoreVersion`) |
| `analytics/workingcapital` | Historical operating NWC analysis, seasonality, transaction-style peg comparison | `Input{Dataset, PeriodMeta, AsOf, CurrentNWC?}` | `Result{History, Statistics, Trend, SeasonalProfile, PegComparison}` | none (financial-only) | Yes | No | Calculates | `FormulaVersion` |
| `analytics/ratios` | Full financial-ratio suite (liquidity/leverage/profitability/efficiency) with period-over-period signals | `Input{Dataset, PeriodMeta}` | `Result{History, Trend, Signals}` | none (financial-only) | Yes | No | Calculates | `FormulaVersion` (+ `SignalRulesVersion`) |
| `analytics/cashflow` | EBITDA-to-free-cash-flow bridge, conversion ratios, debt-service coverage, cash runway | `Input{Dataset, PeriodMeta, OperatingCashFlow?, Capex?, DebtService?, ...}` | `Result{History, ConversionRatios, ConversionTrend, Flags}` | `analytics/workingcapital` (NWC) | Yes | No | Calculates | `FormulaVersion` |
| `analytics/revenuequality` | Revenue composition/growth/CAGR/volatility, optional customer retention & concentration | `Input{Dataset, PeriodMeta, CustomerRevenue?, Policy}` | `Result{History, RevenueTrend, RevenueCAGR, ConcentrationSummary?, Flags}` | none (financial-only) | Yes | Optional | Calculates | `FormulaVersion` |
| `analytics/concentration` | Dataset-independent entity/customer concentration: shares, HHI, dependency changes, loss scenarios | `Input{Basis, Observations, PeriodMeta, Policy}` | `Result{History, LargestShareTrend, Scenarios, Flags}` | none (financial-only) | No (`Observation` tuple, not `FinancialDataset`) | Yes (its whole purpose) | Calculates | `FormulaVersion` |
| `analytics/anomalies` | 11 deterministic expense-anomaly/margin-leakage detection rules | `Input{Dataset, PeriodMeta, AccountGroups?, DiscretionaryCodes?}` | `Result{Anomalies, Summary}` | none (financial-only) | Yes | No | Calculates | `FormulaVersion` |
| `analytics/variance` | Portable line-series budget/forecast/prior-period vs. actual variance | `Input{Lines, PeriodMeta, Policy}` | `Result{LineVariances, CategorySummaries, PeriodTrends, MaterialExceptions}` | none (financial-only) | No (`LineObservation` tuple) | No | Calculates | `FormulaVersion` |
| `analytics/forecast` | Deterministic multi-scenario revenue/COGS/opex/WC/tax/debt projection engine | `Input{Dataset, PeriodMeta, Horizon, Scenarios}` | `Result{Base, ScenarioResults}` | `analytics/workingcapital` (InclusionPolicy reuse + WC starting point) | Yes (historical base only) | No | Calculates | `FormulaVersion` |
| `analytics/debt` | Dataset-independent DSCR/leverage/capacity solver against caller loan terms | `Input{EBITDA, CashFlow?, ExistingDebt?, ProposedLoans?, Policy}` | `Result{BaseCase, ExistingOnlyCase, Capacity, Flags}` | none (financial-only) | No | No | Calculates | `FormulaVersion` |
| `analytics/covenants` | Pass/fail/unavailable covenant tests against caller-computed metrics | `Input{Tests}` | `Result{Tests, Summary}` | none (financial-only) | No | No | Calculates (thin — mostly comparison) | `FormulaVersion` |
| `analytics/benchmarks` | Percentile/quartile/peer-observation comparison vs. caller benchmark data | `Input{Metrics}` | `Result{Comparisons, Summary}` | none (financial-only) | No | No (peer-level, not customer-level) | Calculates (interpolation) | `FormulaVersion` |
| `analytics/valuedrivers` | Re-runs the full valuation orchestrator/consensus under caller driver/scenario mutations | `Input{BaselineRequest, ConsensusOptions, Weights, Drivers?, Scenarios?}` | `Result{Baseline, OneFactorAtATime, Scenarios}` | full `valuation/*` closure (orchestrator, consensus, applicability, dcf, ebitda, netassets, sde, capitalization, basis, report, sensitivity) | No (valuation `Request`, not `FinancialDataset`) | No | Aggregates + re-derives | `FormulaVersion` |
| `analytics/consolidation` | Multi-entity `FinancialDataset` merge: eliminate → convert → weight | `Input{Entities, Periods, CurrencyRates?, Eliminations?, Policy}` | `Result{Consolidated, EntityContributions, CurrencyConversions}` | none (financial-only) | Yes (per-entity) | No | Calculates — the one package whose primary output is itself a `FinancialDataset` | `FormulaVersion` |

## Transaction analytics

| Package | Purpose | Primary Input | Primary Result | Sibling deps | Dataset required? | Customer data? | Mode | Version const |
|---|---|---|---|---|---|---|---|---|
| `transactions/acquisition` | Acquisition screening: price multiples, consensus premium/discount, DSCR (via `analytics/debt` reuse), returns, caller-only red flags | `Input{Target, Consensus, AskingPrice, Financing, ...}` | `Result{PriceMultiples, DebtCoverage, Returns, Flags}` | `analytics/debt` (DSCR) | No | No | Calculates | `FormulaVersion` |
| `transactions/dealstructure` | Sources-and-uses, financing-percentage, and amortization-schedule builder for a proposed deal structure | `Input{PurchasePrice, BuyerEquity, DebtTranches?, SellerNote?, Earnout?, ...}` | `Result{SourcesAndUses, FinancingPercentages, DebtSchedules, AnnualDebtService}` | none (financial-only) | No | No | Calculates. Entry point is `Build`, not `Calculate`. | `FormulaVersion` |
| `transactions/salereadiness` | 11-dimension sale-readiness Status classification from up to 6 optional sibling `Result`s + profile/data-quality | `Input{Dataset?, Metrics?, QoE?, WorkingCapital?, Concentration?, RevenueQuality?, Consensus?, Profile, DataQuality, Policy}` | `Result{Dimensions, Blockers, Risks, Strengths, MissingInfo, Opportunities, OverallScore?}` | `analytics/qoe`, `analytics/workingcapital`, `analytics/concentration`, `analytics/revenuequality`, `valuation/consensus` | Optional (fallback only) | Indirect (via `Concentration`/`RevenueQuality`) | Aggregates | `FormulaVersion` (+ `ScoreVersion`) |

## Portfolio analytics

| Package | Purpose | Primary Input | Primary Result | Sibling deps | Dataset required? | Customer data? | Mode | Version const |
|---|---|---|---|---|---|---|---|---|
| `portfolio/diagnostics` | Scans a portfolio of many businesses' condensed summaries, surfaces ranked cross-portfolio `Finding`s | `Input{Portfolio []BusinessSnapshot, Policy}` | `Result{Findings, Counts, Coverage}` | none directly — consumes caller-populated condensed `Summary` structs (`QoESummary`, `RatioHealthSummary`, etc.), not sibling packages | No | No (business-level, not customer-level) | Aggregates | `FormulaVersion` (+ `ScoreVersion`) |

## Reporting

| Package | Purpose | Primary Input | Primary Result | Sibling deps | Dataset required? | Customer data? | Mode | Version const |
|---|---|---|---|---|---|---|---|---|
| `reporting/management` | Assembles a presentation-neutral management-reporting data pack from up to 13 optional sibling `Result`s | `Input{Dataset, PeriodMeta, QoE?, WorkingCapital?, CashFlow?, Anomalies?, Variance?, Forecast?, Debt?, Covenants?, Consensus?, ...}` | `Report{ExecutiveSummary, HistoricalSeries, ..., TopIssues, ChartSeries, Versions}` | Consumes 11 sibling analytics packages' `Result` types + `valuation/consensus`/`valuation/basis` (no direct sibling-package function calls — pure data reshaping) | Yes | Indirect | Aggregates. Top-level type is `Report`, not `Result` — the one package in this catalog that departs from the naming convention (still carries `FormulaVersion` + a `Versions ModuleVersions` rollup echoing every contributing sibling's own version). Entry point is `Calculate`. | `FormulaVersion` |

## Diagnostics

| Package | Purpose | Primary Input | Primary Result | Sibling deps | Dataset required? | Customer data? | Mode | Version const |
|---|---|---|---|---|---|---|---|---|
| `analytics/diagnostics` | Mines up to 15 optional sibling `Result`s' own Flags/Signals/Anomalies/Status into 12-category `Finding`s (single business, full depth) | `Input{Dataset?, Ratios?, QoE?, WorkingCapital?, CashFlow?, RevenueQuality?, Concentration?, Anomalies?, Variance?, Forecast?, Debt?, Covenants?, Benchmarks?, ValueDrivers?, SaleReadiness?}` | `Result{Findings, Strengths, Concerns, Opportunities, Coverage, MissingDataAreas, Score?}` | Consumes 13 sibling analytics packages + `transactions/salereadiness`'s `Result` types (type-only imports; no function calls into them beyond field reads) | Optional (display only) | Indirect | Aggregates — no new calculation, no AI | `FormulaVersion` (+ `ScoreVersion`) |

## Notes on packages not in this catalog

`financial/metrics`, `financial/adjustments`, `financial/earnings`,
`settings`, `review`, and every `valuation/*` package predate this
catalog's scope (the V1 core — see
[`V1_CONTRACTS.md`](V1_CONTRACTS.md)) and are not re-documented here,
though several analytics packages depend on them (see
[`ANALYTICS_ARCHITECTURE.md`](ANALYTICS_ARCHITECTURE.md)).
