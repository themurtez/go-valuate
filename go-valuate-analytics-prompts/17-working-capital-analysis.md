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

# Prompt 17 — Working Capital Analysis

Create package:

`analytics/workingcapital`

## Goal

Analyze operating working capital historically and support transaction-style working-capital peg analysis.

## Input

Support:
- normalized balance-sheet data
- revenue history
- optional monthly/quarterly detail
- caller-supplied inclusion/exclusion policy
- optional current transaction date/current working capital
- optional peg method

## Core calculations

At minimum:

```text
Operating Current Assets
- Operating Current Liabilities
= Net Working Capital
```

Allow caller policy to define inclusion/exclusion of:
- cash
- debt
- taxes payable
- shareholder/related-party balances
- other current assets/liabilities

Do not hard-code one universal deal definition.

## Output

Return:
- NWC by period
- NWC as % revenue
- historical average
- median
- min/max
- volatility
- trend
- seasonal profile if monthly/quarterly data exists
- suggested peg under caller-selected method
- current excess/deficit versus peg
- component bridge
- excluded accounts
- warnings/issues
- formula version

## Peg methods

Support deterministic strategies:
- latest
- simple average
- median
- trailing-N-period average
- caller-specified fixed peg

Do not invent a “market” peg.

## Tests

Cover:
- seasonal business
- stable NWC
- declining NWC
- cash/debt exclusions
- negative working capital
- missing revenue
- quarterly/monthly vs annual comparability
- peg excess/deficit
- JSON/determinism

Update docs and run full verification.
