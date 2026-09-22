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

# Prompt 22 — Expense Anomaly / Margin Leakage Analysis

Create package:

`analytics/anomalies`

## Goal

Detect deterministic financial anomalies and unusual changes across normalized accounts and periods.

No AI/ML in this module.

## Input

- normalized financial dataset
- optional metrics
- caller thresholds
- optional account groups/categories

## Detection

Support explainable rules such as:
- absolute amount spike
- percentage change spike
- expense growing much faster than revenue
- margin deterioration
- new material expense category
- account disappearing/reappearing
- repeated unusual value
- sign flip
- duplicate-like equal amounts across suspicious accounts/periods
- unusually high owner/discretionary expense share
- unexpected negative expense/revenue

Do not call something fraud.

Use neutral terms such as anomaly, variance, unusual pattern, review recommended.

## Output

Each anomaly:
- code
- severity
- account
- period
- baseline
- observed value
- delta
- threshold
- explanation
- provenance where available

Return deterministic summary/version.

## Tests

Cover:
- stable dataset
- spikes
- revenue-linked expense growth
- margin leak
- sign flips
- small immaterial changes
- missing periods
- JSON/determinism

Update docs and verify.
