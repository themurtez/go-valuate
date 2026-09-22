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

# Prompt 29 — Acquisition Screening

Create package:

`transactions/acquisition`

## Goal

Evaluate a prospective business acquisition using caller-supplied target financials, asking price, and financing assumptions.

This is decision-support, not investment advice.

## Input

Support:
- target financial dataset/metrics
- normalized EBITDA/SDE
- consensus valuation
- asking price
- transaction fees if supplied
- financing/deal-structure input
- working-capital requirement
- capex assumptions
- buyer compensation assumption if relevant
- scenario inputs

## Output

Return:
- asking price / revenue
- asking price / EBITDA
- asking price / SDE
- asking price vs consensus value
- premium/discount as neutral arithmetic
- required equity contribution
- annual debt service
- DSCR
- post-debt cash flow
- cash-on-cash return
- simple payback period
- leverage
- downside/upside scenario results
- red-flag metrics based on caller thresholds
- issues/version

Avoid evaluative buy/don't-buy verdicts.

## Tests

Cover all-cash, leveraged, seller note, price above/below valuation, weak DSCR, negative cash flow, downside shocks, JSON/determinism.

Update docs and verify.
