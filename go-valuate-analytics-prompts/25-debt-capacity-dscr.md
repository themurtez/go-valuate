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

# Prompt 25 — Debt Capacity / DSCR

Create package:

`analytics/debt`

## Goal

Analyze debt service coverage, leverage, and caller-defined debt capacity.

## Input

Support:
- normalized/maintainable EBITDA or cash flow
- existing debt balances
- interest expense
- principal payments
- proposed debt terms
- optional lender policy

Loan terms:
- principal
- interest rate
- amortization term
- payment frequency
- interest-only period if supported

## Calculations

- annual debt service
- DSCR
- fixed-charge coverage if inputs supplied
- debt/EBITDA
- net debt/EBITDA
- interest coverage
- maximum debt under caller-supplied minimum DSCR
- maximum debt under caller-supplied leverage cap
- combined limiting capacity
- downside DSCR scenarios

Do not claim lender approval.

## Output

Return calculations, limiting constraint, headroom, scenarios, warnings, version.

## Tests

Cover amortizing debt, zero debt, negative EBITDA, interest-only if supported, DSCR cap, leverage cap, downside, invalid rates/terms, JSON.

Update docs and verify.
