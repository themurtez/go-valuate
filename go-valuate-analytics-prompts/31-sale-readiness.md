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

# Prompt 31 — Sale Readiness Analysis

Create package:

`transactions/salereadiness`

## Goal

Produce a deterministic sale-readiness assessment from financial and business-profile inputs.

## Input

Support:
- financial dataset/metrics
- QoE result
- working-capital result
- concentration result
- revenue-quality result
- valuation result
- business profile
- document/data-quality indicators
- caller policy

All sub-results optional; missing modules reduce coverage, not silently score zero.

## Dimensions

Examples:
- financial record quality
- earnings stability
- normalization burden
- customer concentration
- recurring revenue
- owner dependence
- margin trend
- working-capital stability
- debt/leverage
- data completeness
- valuation-method consensus

## Output

- dimension scores or statuses
- overall readiness score only if formula is explicit
- blockers
- risks
- strengths
- missing information
- improvement opportunities framed as factual actions
- calculation trace/version

Do not claim a business is guaranteed to sell.

## Tests

Cover ready/stable, owner-dependent, concentrated, poor records, missing modules, negative earnings, JSON/determinism.

Update docs and verify.
