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

# Prompt 27 — Financial Benchmark Engine

Create package:

`analytics/benchmarks`

## Goal

Compare company metrics against caller-supplied benchmark datasets.

This module does **not** source benchmark data.

## Input

- company metrics
- benchmark set metadata
- benchmark observations or percentile thresholds
- industry/size/geography labels as caller metadata
- optional period

Support benchmark forms:
- median
- percentile bands
- quartiles
- explicit peer observations

## Output

For each metric:
- company value
- benchmark median
- percentile/range if calculable
- difference
- relative difference
- band/quartile
- favorable/unfavorable only where semantics are caller-defined
- source metadata
- issues/version

## Provenance

Benchmark output must preserve:
- benchmark source name
- effective date
- population/segment
- caller-supplied license/source ID if desired

Do not fetch or redistribute proprietary benchmark data.

## Tests

Cover percentile math, sparse benchmarks, higher-is-better/lower-is-better semantics, unavailable values, JSON/determinism.

Update docs and verify.
