# Financial statement builder (`accounting/statements`)

`accounting/statements` is the deterministic bridge between
`accounting/ledger` (operational accounting data — chart of accounts,
journal entries, trial balances) and the existing analytics/valuation
ecosystem, which consumes `financial.FinancialDataset`:

```
accounting/ledger
      ↓
account mapping
      ↓
financial statement builder    (this package)
      ↓
financial.FinancialDataset
```

Like every other package in this repository, it contains **no**
persistence, HTTP/API, auth, UI, background jobs, QuickBooks/Xero
integration, AI/LLM, tax logic, external benchmark data, or OCR — see
[What this project intentionally does not contain](../README.md#what-this-project-intentionally-does-not-contain).
Every exported function is pure (no I/O, no mutation of caller-owned
input, no package-global mutable state) and deterministic — see
[`immutability_test.go`](../accounting/statements/immutability_test.go)
and [`determinism_test.go`](../accounting/statements/determinism_test.go).

This package depends on `accounting/ledger`, `financial`,
`financial/classification`, `financial/metrics`, and
`financial/reconciliation`. **`accounting/ledger` does not depend on this
package or on `financial`** — the dependency direction is strictly
`ledger → statements → financial`, exactly as `accounting/ledger`'s own
package doc comment states.

## Architecture: two convergent input paths

`Build` accepts **either** a ledger-derived source **or** an
imported-trial-balance source, selected by `Input.Source`:

- **`SourceLedger`** — `Input.Chart` + `Input.Entries` (optionally
  `Input.OpeningBalances`), replayed per period via
  `ledger.CalculateBalances`.
- **`SourceImportedTrialBalance`** — one `ledger.NormalizedTrialBalance`
  per period in `Input.ImportedTrialBalances`, with no journal detail at
  all.

Both paths normalize into the exact same internal representation
(`resolvedBalance`) in `resolveAccountBalances` (`builder.go`) **before**
mapping or statement construction begins. There is only one
mapping/calculation pipeline in this package — the two input paths never
duplicate logic, they only differ in how they produce the first
intermediate value.

## Mapping model

`AccountMapping{AccountID, FinancialCode, StatementType, SignTreatment,
Allocations, Source, Confirmed, ClassificationReason,
ClassificationConfidence}` is the portable, plain-data instruction for how
one ledger account's balance contributes to the canonical
`financial.FinancialDataset`. It carries no persistence identity of its
own (no mapping ID, no client ID) — a future application layer adds that
around this type, never into it.

`MappingSource` is `EXPLICIT`, `DETERMINISTIC_SUGGESTION`, or `UNMAPPED`.

## Mapping precedence: explicit is always authoritative

A caller-supplied explicit mapping (`Input.Mappings`) **always** wins over
any deterministic suggestion — `resolveOneMapping` (`mapping.go`) never
overrides an explicit mapping using account-name heuristics. The fixed
precedence for a single account is:

```
explicit mapping (if present)
    ↓
deterministic suggestion (only if MappingSuggestDeterministic)
    ↓
unmapped
```

If the same `AccountID` appears in more than one caller-supplied
`AccountMapping`, that is a conflict (`IssueMappingConflict`) — this
package never picks one arbitrarily; the account is excluded from the
resolved mappings entirely until the caller resolves the ambiguity.

## Optional deterministic mapping suggestions

`MappingMode` is `MAPPING_EXPLICIT_ONLY` (the default, safe by design —
no code is ever proposed for an unmapped account) or
`MAPPING_SUGGEST_DETERMINISTIC`. Under the latter, `suggestMapping`
(`mapping.go`) adapts a `ledger.Account` into a synthetic
`financial.RawLineItem{Label: Name, ParentLabel: parent.Name,
StatementType: derived}` and runs it through the existing
`financial/classification.Classify` — **no second classifier was built**;
the existing deterministic engine is reused exactly as-is.

Since `ledger.Account` has no `StatementType` field (ledger is
independent of the financial domain by design), `statementTypeForAccountType`
(`accounttype.go`) derives one from `AccountType`: `REVENUE`/`EXPENSE` →
income statement, `ASSET`/`LIABILITY`/`EQUITY` → balance sheet.

A suggestion is **never** a confirmation: `Confirmed` is always `false` on
a freshly-produced suggestion (`MappingStatusSuggested`, distinct from
`MappingStatusMapped`). `classification.SourceUnknown` is never silently
accepted — it produces `MappingSourceUnmapped`, identical to no
suggestion having been attempted at all.

## Account-type safety

`compatibleCategories` (`accounttype.go`) is a **fixed table**, not a
heuristic:

| `ledger.AccountType` | Allowed `financial.CodeCategory` |
|---|---|
| `REVENUE` | `revenue` |
| `EXPENSE` | `cogs`, `opex`, `other_income_statement` |
| `ASSET` | `balance_sheet` |
| `LIABILITY` | `balance_sheet` |
| `EQUITY` | `balance_sheet` |

A mapping violating this table is **rejected** (`IssueIncompatibleAccountType`),
never silently coerced — and this constraint applies to deterministic
suggestions too, not only caller-supplied explicit mappings: a suggestion
the classifier proposed that turns out account-type-incompatible is
downgraded to `MappingSourceUnmapped` rather than accepted and merely
flagged.

## Sign normalization

Every raw ledger balance is converted into the canonical
`financial.NormalizedItem` amount by one centralized function,
`canonicalAmount` (`signs.go`) — no other code path applies its own ad
hoc sign flip. `SignTreatment` is:

- **`NATURAL`** (default) — `ledger.NormalBalance`-driven: an account in
  its normal position produces a positive canonical amount, exactly like
  `ledger.Balance.DisplayBalance`.
- **`NORMAL`** — the raw balance is used unmodified (bypasses the
  account-type-driven flip; produces the same result as `NATURAL` for
  every non-contra account).
- **`INVERT`** — flips `NATURAL`'s result. The contra-account mechanism
  (see below).

`financial`'s existing convention (documented in
`financial/metrics/income_statement.go` and used by
`financial/reconciliation`'s own contra-asset handling for
`financial.CodeBsAccumDepreciation`) stores every canonical amount as a
**positive "as reported" magnitude** — including a contra account, whose
positive magnitude a downstream formula then explicitly subtracts (see
`financial/reconciliation/balance_sheet.go`'s
`totalAssets -= item.Amount` for that exact code).

### Worked contra-asset example

"Accumulated Depreciation" is a ledger `ASSET`-typed account (so
`ledger.NormalBalance` is `DEBIT`) that naturally carries a `CREDIT`
balance — say raw balance `-4000` (credit-negative, per ledger's own
convention). `ledger`'s display-balance flip **only negates when the
account's normal side is `CREDIT`** (see `ledger`'s `displayBalance`); for
an `ASSET` account (normal = `DEBIT`), `SignNatural` therefore leaves the
value at `-4000`, **unchanged**. But `financial/reconciliation` needs a
**positive** `4000` in the dataset, since it subtracts that positive
figure itself. Mapping this account with `SignTreatment: SignInvert`
flips `SignNatural`'s `-4000` to `+4000` — exactly the convention
`financial/reconciliation`/`financial/metrics` already expect. See
`TestBuild_ContraAccount_SignInvert` for the fully worked, verified test.

If the existing financial taxonomy already represents a contra concept
explicitly (as it does for `CodeBsAccumDepreciation`), this package's
`SignInvert` mechanism reuses that code rather than inventing a parallel
one — there is no silent sign-flip-from-account-name heuristic anywhere
in this package.

## Hierarchy and rollup double-count prevention

`HierarchyPolicy` currently has one implemented mode:
**`HIERARCHY_LEAF_ONLY`**. This package **never calls
`ledger.BuildRollups` itself** — `resolvedBalance` (the internal,
source-agnostic representation both input paths converge into) is always
a **direct** balance (`ledger.CalculateBalances`'/
`ledger.NormalizeTrialBalance`'s own direct-posting figures, never a
rolled-up total). A parent account that itself receives direct postings
(alongside its children) is therefore just another mapped account like
any other; mapping both a parent and its children means both contribute
their own direct balances, with no double counting possible by
construction — see `TestBuild_MultiDepartment_NoDoubleCounting`, which
demonstrates this against `accounting/ledger/fixtures`'
`MultiDepartmentChart`/`MultiDepartmentEntries` (which deliberately posts
directly to a rollup parent alongside its children).

The documented policy: **map leaf/direct accounts, not hierarchy rollup
totals**. A caller wanting a parent's rolled-up figure to be the *only*
number in the statement maps only the parent and leaves its children
unmapped.

## Many-to-one mapping (multiple accounts → one code)

Fully supported and the common case: `aggregateMappedAccounts`
(`aggregate.go`) sums every mapped account's canonical contribution into
one `mappedAccountFigures` per `(FinancialCode, StatementType)` pair,
tracking per-account `RowContributor` provenance (`AccountID`,
`AccountNumber`, `AccountName`, per-period `Amounts`). Both the
`Statement` model's `Row.Contributors` and the `FinancialDataset`'s
`NormalizedItem.Sources` (via `sourceRefsForPeriod`, `provenance.go`)
expose the full list of contributing accounts and their individual
amounts — never just the aggregated total. See
`TestBuild_ManyToOne_MultipleRevenueAccounts`.

## One-to-many mapping (one account → multiple codes)

Not inferred, ever. `AccountMapping.Allocations []AllocationRule` is the
**only** supported mechanism, and it requires every `AllocationRule.Percent`
across the slice to sum to `1.0` within a small float tolerance
(`allocationTolerance`, `validate.go`) — a short or over sum is
`IssueInvalidAllocation` and the account contributes nothing to the
dataset until fixed. Absent `Allocations`, the V1 default rule is
strictly **one account → one canonical code**.

## Mapping validation

`validateMapping`/`validateSingleTarget`/`validateAllocations`
(`validate.go`) check: referenced account exists (including a mapping
whose `AccountID` isn't in the chart at all —
`danglingExplicitMappingIssues`, `mapping.go`, closes a gap where such a
mapping would otherwise never be visited by the main per-chart-account
validation loop), financial code is recognized
(`IssueInvalidFinancialCode`), declared `StatementType` agrees with the
code's own registered statement type, account-type compatibility
(`IssueIncompatibleAccountType`), sign-treatment validity
(`IssueInvalidSignTreatment`, checked against the **raw** caller-supplied
value, not the already-defaulted one), allocation percent validity/sum,
duplicate/conflicting explicit mappings for the same account
(`IssueMappingConflict`), and inactive-account posting policy (a
`SeverityWarning`, not an error — historical balances on an inactive
account remain valid).

## Mapping coverage and materiality

`MappingCoverage` (`coverage.go`) reports **both** count coverage and
amount coverage, since count alone can hide a single large unmapped
balance:

```
CountCoveragePercent  = MappedAccounts / TotalAccounts * 100
AmountCoveragePercent = MappedBalanceAmount / (MappedBalanceAmount + UnmappedBalanceAmount) * 100
```

`MaterialityPolicy{AbsoluteThreshold, PercentOfTotalAssets,
PercentOfRevenue}` mirrors `review.IsMaterial`'s exact two-leg semantics
(absolute threshold OR percent-of-reference, either sufficient) rather
than a wholesale import of `review.Policy` (which carries many unrelated
OCR/reconciliation fields). With every field at its zero value —
`review.IsMaterial`'s own documented default — **materiality gating is
OFF**, so every unmapped account is treated as material.

`UnmappedPolicy` is `ALLOW_UNMAPPED_WITH_WARNING` (default) or
`FAIL_ON_UNMAPPED_MATERIAL`. A material unmapped account is never hidden
from `Result.Mappings` regardless of policy — only the issue severity and
`DatasetAvailability` change.

## Statement model

`Statement{StatementType, Periods, Sections []Section}`,
`Section{Name, Rows []Row}`, `Row{Label, FinancialCode, Kind, Values,
Contributors}` — a presentation-neutral structural model this package
introduces (it did not exist anywhere else in the repository before this
package). `Row.Kind` reuses `financial.RowKind` directly (the same
zero-value-is-NORMAL, HEADING/SUBTOTAL/TOTAL enum
`financial/classification`/`review.StructurePayload` already use), rather
than a parallel enum.

Fixed section ordering:

**Income statement** — Revenue → Cost of Goods Sold → Operating Expenses
→ Other Income/Expense, with calculated structural subtotals (Total
Revenue, Gross Profit, Operating Income) inserted after their respective
sections, and Net Income last.

**Balance sheet** — Current Assets → Non-Current Assets → Current
Liabilities → Non-Current Liabilities → Equity, followed by calculated
Total Assets, Total Liabilities, Total Equity, Total Liabilities & Equity.

Only sections the actual `financial.CodeCategory` taxonomy supports are
generated — this package never invents a row for a category the taxonomy
cannot represent.

Within a section, rows follow `financial.LookupCode`'s own registered
order (via `aggregateMappedAccounts`' code-sorted output) — never Go map
iteration order.

## Canonical calculated subtotals — reused, not reinvented

`buildIncomeStatement` (`income.go`) calls **`financial/metrics.Calculate`**
directly for Total Revenue, Gross Profit, Operating Income (EBIT), and Net
Income — the exact same formula chain every other analytics package in
this repository already relies on. `buildBalanceSheet` (`balance.go`)
computes Total Assets/Liabilities/Equity/Liabilities & Equity as **direct
sums** of the mapped figures, since `financial/metrics.Snapshot` has no
`TotalAssets`/`TotalLiabilities`/`TotalEquity` fields at all to defer to
(it exposes `CurrentAssets`/`CurrentLiabilities`/`WorkingCapital`, not
full balance-sheet totals) — there is no existing formula for those four
figures to reuse, so a direct sum with no independently-invented
arithmetic is the correct choice per this module's "use existing metrics
formulas... if taxonomy structure differs" instruction.

A source TB's own reported subtotal rows are never re-aggregated into the
statement — this package only ever sums mapped `RowStatusNormal`-equivalent
leaf balances; the calculated canonical subtotal is always authoritative
for the generated statement.

## Balance-sheet reconciliation — reused, not reinvented

`reconcileBalanceSheet` (`reconcile.go`) calls
**`financial/reconciliation.Run`** and projects its
`CheckBalanceSheetBalances` check into
`BalanceSheetReconciliation{Period, Assets, Liabilities, Equity, Balanced,
Difference, Tolerance, Status}` — the same contra-asset-aware Assets ≈
Liabilities + Equity check every other consumer of a
`financial.FinancialDataset` already gets, never a second reconciliation
formula. Never forced to balance: `Balanced` reflects exactly what
`reconciliation.Run` computed.

**This package is a reporting transformation, not a posting engine.** It
never automatically posts the current period's net income to retained
earnings. Every fixture's balance sheet in this package's own test suite
is therefore expected to be out of balance by exactly the period's net
income unless the caller's own ledger data (or an explicit
retained-earnings entry) already accounts for it — see
`TestBuild_ServiceBusiness_LedgerDerived`'s explicit note. A caller
wanting current-period net income reflected in equity must arrange that
in the source ledger data itself; this package will not do it silently.

## FinancialDataset construction

`buildDataset` (`dataset.go`) re-runs `aggregateMappedAccounts`
independently for both statement types and converts the combined figures
directly into `financial.FinancialDataset` via `datasetFromFigures`,
using `financial.NormalizedItem` (the existing financial-domain
constructor) rather than bypassing it. This does not route through
`financial.Normalize` itself, since `Normalize` expects
`financial.MappedLineItem` keyed by source **row**; this package has
already performed the equivalent aggregation at the **account** level in
`aggregateMappedAccounts`, so a second pass through `Normalize` would be
strictly redundant work producing an identical result. The output shape
(sorted `Items`, `Code`+`Period` keying, `SourceRef` provenance) is
identical to what `Normalize` itself produces.

## Provenance

Every `NormalizedItem.Sources` entry (`financial.SourceRef`) traces back
to the contributing ledger account: `RowID` carries the `Account.ID`
directly (the closest existing field to an account-provenance pointer),
`Label` carries `"<Number> <Name>"`, `Period` and `Amount` are the
per-account, per-period contribution. When multiple accounts map to one
code, every contributing account appears as its own `SourceRef` — never
collapsed into a single unattributed total.

## Issue taxonomy

One shared `Issue{Code, Severity, Message, AccountID, FinancialCode,
Period}` type, two-severity model (`SeverityError`/`SeverityWarning`),
consistent with every sibling package. `HasErrors(issues)` is
intentionally duplicated per this repository's established convention
(see `financial/adjustments.HasErrors`'s doc comment for the rationale)
rather than shared via an interface.

| Code | Meaning |
|---|---|
| `UNMAPPED_ACCOUNT` | Account has no resolvable mapping (severity/materiality-dependent) |
| `INVALID_MAPPING` | Unknown account reference, empty code with no allocations, statement-type mismatch, or mapped-but-inactive-account warning |
| `MAPPING_CONFLICT` | Duplicate explicit mapping for one account, or an ambiguous same-precedence template rule match |
| `INCOMPATIBLE_ACCOUNT_TYPE` | Mapping's code category is impossible for the account's `AccountType` |
| `INVALID_FINANCIAL_CODE` | Mapping references an unrecognized `financial.Code` |
| `MIXED_CURRENCY` | Contributing accounts use more than one currency and no `ReportingCurrency` was supplied |
| `UNBALANCED_BALANCE_SHEET` | Assets != Liabilities + Equity within reconciliation tolerance |
| `INCOMPLETE_STATEMENT` | A requested statement had no mapped accounts to build from |
| `INVALID_SIGN_TREATMENT` | An unrecognized `SignTreatment` value |
| `INVALID_ALLOCATION` | Allocation percentages invalid or don't sum to 100% |
| `MISSING_PERIOD` | No periods supplied, or a requested period has no matching source data |
| `SOURCE_TRIAL_BALANCE_UNBALANCED` | The upstream ledger TB, or an imported TB, was itself out of balance |

Only codes this package actually emits are defined.

## Availability semantics

`DatasetAvailability` (and the parallel `IncomeStatementAvailability`/
`BalanceSheetAvailability`) distinguish five states — never encoded by an
empty slice alone: `NOT_REQUESTED`, `UNAVAILABLE`, `BUILT`,
`BUILT_WITH_WARNINGS`, `INVALID`.

## Mapping review contract

`AccountMappingResult` exposes everything a future application's review
screen needs for every account: `AccountID`/`Number`/`Name`/`Type`/
`ParentID`/`Active`/`CurrentBalance`/`Currency`, the resolved `Mapping`,
`Alternatives` (runner-up codes from a deterministic suggestion),
`MappingStatus`, and per-account `Issues`. No UI or persistence is built
here — a future caller persists/confirms externally. See
`TestMappingReview_UnmappedThenSuggestThenConfirmThenRebuild` for the full
unmapped → suggested → confirmed → rebuilt → complete cycle, driven
entirely through this package's own types.

## Mapping templates

`MappingTemplate{Version, Mappings []AccountMappingRule}` is reusable,
plain domain data — no persistence, no account IDs baked in.
`ResolveTemplate(template, chart)` matches with a **fixed precedence**:

```
Account ID  →  Account Number  →  Exact Normalized Name  →  (unmatched)
```

"Exact Normalized Name" reuses `financial/classification.NormalizeLabel`'s
existing normalization directly (no second, possibly-divergent
normalization routine) — this is a fixed, deterministic transformation,
not fuzzy matching, consistent with this module's explicit "no fuzzy name
similarity" instruction. More than one same-precedence rule matching one
account is `IssueMappingConflict`; the account is excluded from the
resolved output rather than picking arbitrarily.

## Currency boundary

One `Build` call produces one reporting currency. `resolveReportingCurrency`
(`builder.go`) uses `Input.ReportingCurrency` if supplied, otherwise the
single shared currency across every contributing account, if there is
exactly one. Mixed currencies with no explicit `ReportingCurrency` produce
`IssueMixedCurrency` and **no dataset is built** — this package never
fetches FX and never reuses `analytics/consolidation`'s own conversion
machinery implicitly.

## Multi-entity boundary

One `Build` call is one ledger/entity. Consolidation is explicitly out of
scope here — a caller builds a separate `financial.FinancialDataset` per
entity (via separate `Build` calls) and then uses the existing
`analytics/consolidation` package.

## Zero balances vs. unavailable accounts

The distinction is preserved throughout: an account present with a zero
balance still appears in `Result.Mappings` and (if mapped) contributes a
present `Row.Values`/`NormalizedItem.Amount` of exactly `0.0`, distinct
from a period absent from `Values` entirely (no data at all). Final
`FinancialDataset` output follows `financial.Normalize`'s own
zero-item conventions (a mapped account's `0.0` amount is a real item, not
omitted).

## JSON and versioning

Every exported type has explicit `json` tags. Major types round-trip
through JSON byte-for-byte
(`accounting/statements/json_test.go`), including a full `Result`, a
`MappingTemplate`, an allocated `AccountMapping`, and `MappingCoverage`.

Three independently-versioned constants (`versions.go`), per the main
README's [Versioning strategy](../README.md#versioning-strategy):

- **`SchemaVersion`** — this package's public result/schema shape
  (`AccountMapping`, `Statement`, `Result`, `Issue`, ...).
- **`StatementFormulaVersion`** — sign normalization, structural-row
  assembly, section templates, hierarchy leaf-posting policy.
- **`MappingContractVersion`** — `AccountMapping`'s shape, mapping
  precedence, account-type safety rules, `MappingTemplate` rule
  precedence — versioned independently since a mapping-precedence change
  does not necessarily imply a statement-formula change, and vice versa.

## Limitations / explicit non-goals

Deferred to later prompts, per this task's explicit boundary: AR aging,
AP aging, journal anomaly detection, tax returns, payroll, bank
reconciliation, QuickBooks/Xero ingestion, AI mapping, consolidation
(reuse `analytics/consolidation` after building separate entity
datasets), FX-rate retrieval, financial-statement UI, PDF rendering.

## Fixtures

`accounting/statements/fixtures` builds on `accounting/ledger/fixtures`
(re-exporting its charts/entries directly) and adds the mapping layer:
explicit mappings for the service/retailer/owner-operated/multi-department
businesses, a many-to-one revenue-account scenario, a contra-account
(Accumulated Depreciation) scenario, an invalid-mapping example, and a
reusable `MappingTemplate` plus a deliberately-conflicting one.

## Benchmarks

`accounting/statements/benchmark_test.go` benchmarks a full `Build` call
at 1,000 accounts × 10 periods with many-to-one mapping (~29ms on the
development machine), plus a scaling sweep (100/500/1,000/2,000 accounts)
that showed **linear, not superlinear** scaling — no O(N²) path was found.
