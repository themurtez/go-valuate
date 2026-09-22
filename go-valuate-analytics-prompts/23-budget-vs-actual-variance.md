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

# Prompt 23 — Budget vs Actual / Variance Analysis

Create package:

`analytics/variance`

## Goal

Compare actual financials against budget, forecast, prior period, or other caller-supplied baseline.

## Input

Define portable series for:
- canonical account code
- period
- actual
- baseline
- baseline type

Baseline types:
- budget
- forecast
- prior year
- custom

## Calculations

Return:
- absolute variance
- percentage variance when meaningful
- favorable/unfavorable classification based on account semantics
- materiality
- contribution to total variance
- aggregated variance by category
- period trends

Revenue increase may be favorable while expense increase may be unfavorable; encode this deterministically by taxonomy/category with caller override.

## Output

- line variances
- category summaries
- top favorable
- top unfavorable
- material exceptions
- total bridge
- issues/version

## Tests

Cover revenue, expense, COGS, zero baseline, negative values, missing baseline, multiple periods, category rollup, JSON/determinism.

Update docs and verify.
