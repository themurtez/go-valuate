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

# Prompt 19 — Cash Flow Analytics

Create package:

`analytics/cashflow`

## Goal

Produce a deterministic cash-flow and cash-conversion analysis suitable for SMB advisory and transactions.

## Input

Support:
- normalized financial dataset
- optional cash flow statement data
- EBITDA/metrics
- capex inputs where available
- working-capital changes
- debt service
- owner distributions
- taxes
- optional policy

## Calculations

Where supported:
- EBITDA
- cash from operations
- change in working capital
- capex
- free cash flow
- free cash flow to firm / owner where clearly defined
- EBITDA-to-operating-cash conversion
- EBITDA-to-free-cash-flow conversion
- debt service coverage bridge
- owner distribution coverage
- cash burn/runway for loss-making businesses when enough input exists

Do not infer missing cash flows from EBITDA without clearly labeling the estimate.

## Output

Return:
- period cash-flow bridge
- conversion ratios
- trend
- recurring cash drains
- capex burden
- working-capital burden
- debt-service burden
- flags/warnings
- formula version

## Tests

Include:
- strong conversion
- weak conversion
- working-capital build
- capex-heavy company
- negative cash flow
- missing cash-flow statement
- estimate vs reported distinction
- JSON/determinism

Update docs and verify.
