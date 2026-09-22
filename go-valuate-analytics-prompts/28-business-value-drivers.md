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

# Prompt 28 — Business Value Driver Engine

Create package:

`analytics/valuedrivers`

## Goal

Show how deterministic changes in business metrics/assumptions affect valuation.

This module must use existing valuation methods/sensitivity logic rather than inventing a new valuation formula.

## Input

- baseline financial/valuation inputs
- selected valuation methods
- baseline consensus result
- caller-defined driver scenarios

Driver examples:
- revenue growth
- EBITDA margin
- SDE
- recurring revenue percentage where it affects caller-supplied method assumptions
- debt
- working capital
- owner compensation adjustment
- customer-loss impact
- method multiple change

Only model drivers with explicit deterministic linkage.

## Output

For each driver/scenario:
- baseline value
- changed inputs
- recalculated method results
- recalculated consensus
- value delta
- percentage delta
- assumptions
- warnings

Support combined scenarios separately from one-factor-at-a-time analysis.

## Important

Do not claim causation where the valuation method does not actually use the driver.

Example:
Recurring revenue should not magically alter a multiple unless the caller explicitly supplies a rule mapping recurring revenue to a changed multiple.

## Tests

Cover revenue/margin changes, debt change, multiple change, no-effect driver, combined scenario, JSON/determinism.

Update docs and verify.
