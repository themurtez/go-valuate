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

# Prompt 33 — Accountant Client Portfolio Diagnostics

Create package:

`portfolio/diagnostics`

## Goal

Analyze a collection of already-computed client/business analytics and identify accounts needing attention or advisory follow-up.

No DB and no fetching client data.

## Input

Define a portable `BusinessSnapshot` containing:
- opaque business ID
- optional display label
- period
- selected metrics
- QoE summary
- ratio/health summary
- concentration summary
- cash-flow summary
- valuation summary
- sale-readiness summary
- optional prior snapshot

Not every field required.

## Output

Return:
- ranked/ordered diagnostic findings
- business ID
- finding code
- severity
- metric/current value
- prior value where relevant
- change
- reason
- suggested review category
- portfolio-level counts
- coverage/missing-data summary

Examples:
- margin deterioration
- revenue decline
- cash conversion weakening
- leverage increase
- concentration increase
- valuation movement
- unresolved financial-quality issues
- potential sale-readiness opportunity

Do not generate sales pitches.

## Ranking

If a priority score is used, formula must be deterministic and configurable.

## Tests

Cover multi-client portfolio, missing modules, equal severities/order, trend changes, JSON/determinism.

Update docs and verify.
