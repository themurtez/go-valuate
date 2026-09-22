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

# Prompt 21 — Customer / Revenue Concentration

Create package:

`analytics/concentration`

## Goal

Analyze concentration risk from caller-supplied customer/vendor revenue or spend observations.

## Generic input

Use portable observations:
- entity ID/key
- period
- amount
- optional segment/category

Allow caller to label the dataset as:
- customer revenue
- supplier spend
- other concentration basis

## Metrics

Calculate:
- top 1 / top 3 / top 5 / top 10 shares
- largest entity share
- Herfindahl-Hirschman Index (HHI)
- entity ranking
- concentration trend
- dependency changes over time
- lost-largest-entity scenario
- top-N loss scenarios

If earnings/margin assumptions supplied, allow deterministic impact estimates.

## Output

Return concentration metrics, ranked entities, scenarios, flags, issues, version.

## Privacy

Do not require actual customer names; opaque keys are sufficient.

## Tests

Cover:
- highly concentrated
- diversified
- one customer
- changing concentration
- zero/negative malformed inputs
- top-N scenario
- JSON/determinism

Update docs and verify.
