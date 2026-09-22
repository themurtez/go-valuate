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

# Prompt 24 — Forecast and Scenario Engine

Create package:

`analytics/forecast`

## Goal

Generate deterministic financial projections and scenario sets from caller-supplied assumptions.

This module does not predict assumptions. It applies them.

## Input

Support:
- historical base financials/metrics
- forecast horizon
- revenue growth assumptions by period
- gross margin or COGS assumptions
- operating expense growth/amount assumptions
- capex
- depreciation/amortization where supplied
- working capital assumptions
- tax assumptions
- debt service assumptions
- named scenarios

## Scenario types

Caller can define:
- base
- downside
- upside
- custom

Do not hard-code business forecasts.

## Output

For each scenario:
- projected P&L
- EBITDA
- SDE where meaningful
- margins
- cash flow if inputs support it
- working capital
- debt-service coverage where inputs support it
- assumption set
- calculation trace
- warnings/version

## Scenario transformations

Allow deterministic helpers:
- revenue +/- %
- margin +/- points
- expense +/- %
- customer-loss shock
- one-time cost shock
- debt-rate shock

## Tests

Cover base/downside/upside, negative growth, margin compression, missing assumptions, multi-year compounding, no hidden mutation, JSON/determinism.

Update docs and verify.
