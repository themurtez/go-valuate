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

# Prompt 20 — Revenue Quality Analysis

Create package:

`analytics/revenuequality`

## Goal

Evaluate the quality, stability, and composition of revenue without requiring app/database concepts.

## Input

Support:
- normalized revenue history
- optional recurring/non-recurring classifications
- optional customer-level revenue dataset
- optional contract/revenue-type metadata
- caller policy

Define a portable customer-revenue input type such as:
- customer key
- period
- amount
- recurring flag if known
- segment if known

No customer PII is required.

## Output

Return:
- total revenue by period
- recurring revenue and %
- non-recurring revenue and %
- growth
- CAGR
- volatility
- retention where customer history exists
- new-customer revenue
- lost-customer revenue
- expansion/contraction revenue
- concentration summary if customer data supplied
- revenue quality flags
- components/version

## Flags

Deterministic examples:
- declining recurring mix
- growth dependent on new customers
- high lost-customer revenue
- volatile revenue
- one-period spike
- shrinking existing-customer base

Do not claim SaaS-style NRR/GRR unless input semantics support it.

## Tests

Cover:
- recurring service business
- project business
- customer churn
- growth through new customers
- volatile revenue
- missing customer detail
- JSON/determinism

Update docs and verify.
