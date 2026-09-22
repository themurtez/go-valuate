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

# Prompt 16 — Quality of Earnings (QoE)

Create package:

`analytics/qoe`

## Goal

Produce a deterministic quality-of-earnings analysis from normalized financial history plus confirmed adjustments.

## Input

At minimum:
- `financial.FinancialDataset`
- derived metrics if already available, or calculate through existing metrics package
- confirmed adjustments
- maintainable-earnings strategy/result
- optional policy/options

Do not require customer/user/application objects.

## Output

Return a structured `Result` containing at least:

- reported EBITDA by period
- normalized EBITDA by period
- reported SDE by period
- normalized SDE by period
- total adjustments
- adjustment breakdown by type
- recurring vs non-recurring adjustment summary
- maintainable EBITDA
- maintainable SDE
- revenue growth/stability
- EBITDA margin trend
- earnings volatility
- adjustment-to-EBITDA ratio
- adjustment-to-SDE ratio
- earnings quality flags
- calculation components
- issues/warnings
- formula/schema version

## Deterministic quality flags

Support explainable flags such as:
- large normalization burden
- declining EBITDA despite revenue growth
- volatile earnings
- materially inconsistent margins
- large owner-discretionary component
- repeated “one-time” expenses across multiple years
- non-operating income materially supporting earnings
- negative/near-zero maintainable earnings

Thresholds must be caller-configurable.

Do not score using opaque AI.

## Repeated one-time adjustment detection

If the same adjustment category/source pattern appears repeatedly across periods, flag that the item may not be truly non-recurring.

Do not automatically remove it; just surface the flag.

## Earnings quality score

Optionally produce a deterministic 0–100 score only if:
- exact formula is documented
- component contributions are returned
- it is clearly labeled heuristic, not an accounting standard

Otherwise return flags + raw measures only.

## Tests

Cover:
- clean stable business
- high adjustment burden
- repeated one-time costs
- volatile earnings
- declining margins
- negative EBITDA
- owner-heavy SDE
- no adjustments
- missing periods
- JSON round-trip
- deterministic repeated execution

Update README/docs.

Run full verification and report exported API, formulas, thresholds, flags, fixtures, and test results.
