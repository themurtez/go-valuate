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

# Prompt 30 — Deal Structure / Financing Model

Create package:

`transactions/dealstructure`

## Goal

Model acquisition financing structures independently from valuation.

## Input

Support:
- purchase price
- buyer equity/down payment
- one or more debt tranches
- seller note
- earnout if deterministic schedule supplied
- transaction fees
- working-capital contribution
- optional closing cash/debt adjustments

Debt tranche:
- amount
- rate
- amortization
- term
- payment frequency
- interest-only period
- balloon if supplied

## Output

- sources and uses
- total debt
- total equity
- financing percentages
- payment schedule
- annual debt service
- interest/principal by period
- balloon
- seller-note schedule
- earnout schedule
- funding gap/surplus
- issues/version

Do not calculate valuation here.

## Tests

Cover single loan, multiple tranches, seller note, balloon, interest-only, funding gap, invalid terms, JSON/determinism.

Update docs and verify.
