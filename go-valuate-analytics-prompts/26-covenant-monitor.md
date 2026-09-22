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

# Prompt 26 — Financial Covenant Monitor

Create package:

`analytics/covenants`

## Goal

Evaluate caller-supplied financial covenants against calculated financial metrics.

## Input

Define portable covenant rules:
- covenant ID
- metric
- operator (`>=`, `<=`, etc.)
- threshold
- test period
- optional cure/grace metadata
- optional warning buffer

Supported initial metrics may include:
- DSCR
- debt/EBITDA
- net debt/EBITDA
- current ratio
- quick ratio
- minimum EBITDA
- minimum net worth
- maximum capex
- custom caller-supplied metric

Do not encode loan documents or legal interpretation.

## Output

For each covenant:
- actual
- threshold
- pass/fail/unavailable
- headroom
- warning-buffer status
- explanation
- period

Summary:
- breaches
- near breaches
- unavailable tests

## Tests

Cover pass, breach, near breach, unavailable metric, wrong operator, custom metric, multiple periods, JSON/determinism.

Update docs and verify.
