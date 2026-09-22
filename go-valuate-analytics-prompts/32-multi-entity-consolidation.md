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

# Prompt 32 — Multi-Entity Financial Consolidation

Create package:

`analytics/consolidation`

## Goal

Combine multiple canonical financial datasets into a consolidated dataset using caller-supplied ownership/currency/elimination information.

## Input

- multiple `financial.FinancialDataset`s
- entity IDs
- ownership percentage if needed
- common reporting periods
- optional currency conversion rates supplied by caller
- explicit intercompany elimination entries

This module does not fetch FX rates.

## Behavior

Support:
- full consolidation for caller-selected entities
- simple ownership-weighted mode if explicitly selected
- period alignment validation
- currency consistency
- explicit FX conversion only when rates supplied
- intercompany eliminations
- provenance linking consolidated values to source entities/items

## Output

- consolidated FinancialDataset
- entity contribution summary
- eliminations
- currency conversions
- reconciliation issues
- provenance/version

Do not infer eliminations.

## Tests

Cover two entities, eliminations, different periods, different currencies with/without rate, ownership weighting, JSON/determinism.

Update docs and verify.
