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

# Prompt 35 — Business Diagnostic Engine

Create package:

`analytics/diagnostics`

## Goal

Combine outputs from multiple deterministic analytics modules into one structured business diagnostic.

This is not an AI narrative layer.

## Input

Support optional:
- financial metrics
- ratios
- QoE
- working capital
- cash flow
- revenue quality
- concentration
- anomalies
- variance
- forecast
- debt
- covenants
- benchmark results
- valuation/value drivers
- sale readiness

## Diagnostic categories

Examples:
- profitability
- growth
- liquidity
- leverage
- cash conversion
- working capital
- revenue quality
- concentration
- earnings quality
- operational cost control
- valuation
- transaction readiness

## Output

Each diagnostic finding:
- stable code
- category
- severity
- title
- evidence
- source module
- metric/value
- prior/comparison value
- explanation
- recommended investigation/action phrased neutrally

Return:
- strengths
- concerns
- opportunities
- missing-data areas
- module coverage
- optional deterministic overall health score only with explicit formula
- schema version

Do not make legal/tax/investment conclusions.

## Tests

Cover healthy business, stressed business, partial inputs, contradictory signals, deterministic ordering, JSON.

Update docs and verify.
