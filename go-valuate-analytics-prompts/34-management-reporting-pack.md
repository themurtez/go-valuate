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

# Prompt 34 — Management Reporting Data Pack

Create package:

`reporting/management`

## Goal

Assemble presentation-neutral management-reporting data from existing analytics modules.

Do not generate PDF/HTML/charts directly.

## Input

Accept optional:
- FinancialDataset
- metrics
- ratios
- cash-flow analysis
- working capital
- QoE
- variance
- forecast/scenarios
- anomalies
- concentration
- revenue quality
- debt/covenants
- valuation summary

## Output

Produce a JSON-safe `Report` with:
- executive KPI summary
- historical financial series
- profitability series
- liquidity/leverage series
- cash-flow series
- working-capital series
- variance tables
- forecast tables
- top anomalies/issues
- chart-ready series
- data coverage
- source/module versions

No narrative AI in this package.

## Ordering

Deterministic section and series order.

## Tests

Cover full pack, partial modules, missing data, serialization, deterministic output.

Update docs and verify.
