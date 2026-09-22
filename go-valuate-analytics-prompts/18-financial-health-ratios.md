Continue work in the standalone Go module:

`github.com/themurtez/go-valuate`

Do **not** stage or commit files.

## Architectural rules

This module must remain portable and application-independent.

Do **not** add:
- database/persistence
- HTTP/API handlers
- auth/users/accounts
- Stripe/billing
- frontend/UI
- background jobs
- external SaaS dependencies
- AI/LLM unless this prompt explicitly requests it

Prefer:
- typed Go input
- deterministic calculations
- typed/JSON-safe output
- no mutation of caller-owned input
- stable enum/code values
- explicit availability instead of treating missing data as zero
- structured issues/warnings, not string parsing
- package-level schema/formula version where materially useful
- JSON round-trip tests
- deterministic output ordering
- realistic fixtures
- unit + integration tests
- `gofmt`, `go test`, `go vet`, `go build`, `go test -race`

Reuse existing `financial.FinancialDataset`, metrics, adjustments, valuation, settings, review, and report types where semantically appropriate, but do not force unrelated modules into awkward dependencies.

The package should be usable from any Go app by passing structs in and receiving structs out.

# Prompt 18 — Financial Health / Ratio Engine

Create package:

`analytics/ratios`

## Goal

Provide a reusable deterministic financial-ratio engine with trend and health-signal output.

## Input

- `financial.FinancialDataset`
- existing metrics output where useful
- optional ratio policy/thresholds

## Ratios

Implement where data is available:

Profitability:
- gross margin
- operating margin
- EBITDA margin
- net margin
- return on assets
- return on equity

Liquidity:
- current ratio
- quick ratio
- cash ratio

Leverage:
- debt/equity
- debt/assets
- debt/EBITDA
- net debt/EBITDA
- interest coverage

Efficiency:
- asset turnover
- receivables turnover
- inventory turnover
- days sales outstanding
- days inventory outstanding
- days payable outstanding
- cash conversion cycle

Growth:
- revenue growth
- gross profit growth
- EBITDA growth
- net income growth

## Missing data

Never divide by unavailable/zero denominators silently.

Return availability and issue metadata.

## Signals

Support deterministic caller-configurable flags:
- weakening liquidity
- rising leverage
- margin compression
- slowing collections
- inventory buildup
- weak interest coverage
- improving/deteriorating profitability

No opaque composite score unless formula is explicit.

## Output

Return per-period ratios, trends, comparisons, flags, components, and version.

## Tests

Cover normal, zero denominators, negative equity, negative earnings, missing balance sheet, multi-year trend, deterministic ordering, JSON.

Update docs and verify.
