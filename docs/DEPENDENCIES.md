# Dependencies

Direct runtime dependencies and external runtime tools used by
`github.com/themurtez/go-valuate`, verified against `go.mod`/`go.sum` and
each dependency's own package source, not merely assumed.

See [`V1_CONTRACTS.md`](V1_CONTRACTS.md) for the data contracts these
dependencies sit behind, and
[`INTEGRATION.md § Security / privacy`](INTEGRATION.md#security--privacy)
for the runtime-safety posture each isolation boundary maintains.

## Direct dependencies

| Module/tool | Version | Purpose | License | Linked/imported vs. external | Required or optional | Isolation boundary |
|---|---|---|---|---|---|---|
| [`github.com/ledongthuc/pdf`](https://github.com/ledongthuc/pdf) | pinned in `go.mod`/`go.sum` (a maintained fork of the archived `rsc.io/pdf`) | Pure-Go, dependency-free born-digital (text-layer) PDF reader | BSD-3-Clause | Linked/imported | Optional — only needed by callers using `ingestion/pdf` (text-PDF ingestion); CSV/XLSX-only callers never need it | `ingestion/pdf` — no `ledongthuc/pdf` type appears in any exported `ingestion/pdf` API |
| [`github.com/openai/openai-go/v2`](https://github.com/openai/openai-go) | v2.7.1 | Official OpenAI SDK, used by both AI-fallback provider adapters | Apache-2.0 | Linked/imported | **Fully optional/opt-in** — nothing outside the two `.../ai/openai` adapter subpackages imports it; every other package (including their own tests) builds and runs with zero network access and zero credentials | Two separate boundaries: `financial/classification/ai/openai` and `financial/adjustments/ai/openai` — neither provider-neutral parent package (`financial/classification/ai`, `financial/adjustments/ai`) imports the SDK directly |
| [`github.com/pdfcpu/pdfcpu`](https://github.com/pdfcpu/pdfcpu) | v0.15.0 | PDF page-image extraction for scanned/image-only pages (the OCR pipeline) | Apache-2.0 | Linked/imported | Optional — only needed by callers using `ingestion/pdf/pdfimage` / the OCR path (`pdf.ParseWithOCR`); the default `pdf.Parse` (text-layer-only) never touches it | `ingestion/pdf/pdfimage` — `extract.go` is the only file in this package that imports it directly |
| [`github.com/xuri/excelize/v2`](https://github.com/qax-os/excelize) | v2.11.0 | XLSX (Office Open XML) worksheet reader | BSD-3-Clause | Linked/imported | Optional — only needed by callers using `ingestion/xlsx`; CSV/PDF-only callers never need it | `ingestion/xlsx` — `excelize.File` and every other excelize type stay entirely internal to that package |
| [`golang.org/x/image`](https://pkg.go.dev/golang.org/x/image) | v0.44.0 | `x/image/tiff` decoder, for a `pdfcpu`-extracted TIFF-format embedded scanned image | BSD-3-Clause | Linked/imported | Optional — same OCR/scanned-PDF path as pdfcpu, exercised only for TIFF-format embedded images specifically | `ingestion/pdf/pdfimage` (same package as pdfcpu's boundary — a sub-dependency of that package's own image-decoding needs) |
| Tesseract OCR | 4.0+ (LSTM engine, stable `tsv` output) required; not tested against legacy 3.x | Local OCR text recognition for scanned/image-only PDF pages | Apache License 2.0 | **External runtime executable — not a Go dependency at all** | **Fully optional, and not even a build dependency** — the whole module compiles with `go build ./...` on a machine with no Tesseract installed; availability is discovered only at runtime (`Engine.Recognize`/`Available()`), producing a structured `*ocr.Error{Code: OCR_ENGINE_UNAVAILABLE}` rather than a build failure or panic if missing | `ingestion/ocr/tesseract` — invoked via `os/exec`, never CGO, never a shell (`exec.CommandContext` with separate arguments, no string-built command line) |

**`ledongthuc/pdf` isolation note.** `ingestion/pdf/extract.go` is where
every call into the library happens; `ingestion/types.go` also carries an
import of the same package, but only for a type reference used in internal
plumbing — no `ledongthuc/pdf` type is exposed in `ingestion/pdf`'s own
exported API surface, so the substantive isolation guarantee (a caller of
`ingestion/pdf` never needs to import `ledongthuc/pdf` directly) still
holds.

## Commercial-use notes

Every direct dependency above is permissively licensed
(BSD-3-Clause or Apache-2.0) — both license families permit commercial use,
modification, and redistribution with attribution, and neither imposes a
copyleft/share-alike obligation on this module or its consumers. Tesseract
(Apache-2.0) is invoked as an external process, never linked into this
module's binary, so its license terms apply to the separately-installed
Tesseract executable itself, not to `go-valuate` or any application
importing it. No dependency in this list is AGPL, GPL, or otherwise
copyleft-encumbered. This is a summary for integration planning, not legal
advice — a main application with its own compliance requirements should
still independently confirm license terms before shipping.

## Indirect dependencies

Every entry in `go.mod`'s second (`// indirect`) require block, and which
direct dependency pulls each one in:

| Indirect dependency | Pulled in by |
|---|---|
| `github.com/clipperhouse/uax29/v2` | pdfcpu |
| `github.com/hhrutter/tiff` | pdfcpu |
| `github.com/mattn/go-runewidth` | pdfcpu |
| `github.com/richardlehane/mscfb` | excelize/v2 |
| `github.com/richardlehane/msoleps` | excelize/v2 |
| `github.com/tidwall/gjson` | openai-go/v2 |
| `github.com/tidwall/match` | openai-go/v2 (via gjson) |
| `github.com/tidwall/pretty` | openai-go/v2 (via gjson/sjson) |
| `github.com/tidwall/sjson` | openai-go/v2 |
| `github.com/tiendc/go-deepcopy` | excelize/v2 |
| `github.com/xuri/efp` | excelize/v2 |
| `github.com/xuri/nfp` | excelize/v2 |
| `go.yaml.in/yaml/v3` | pdfcpu |
| `golang.org/x/crypto` | pdfcpu (excelize also wants a lower version; Go's minimal version selection keeps the higher one) |
| `golang.org/x/net` | excelize/v2 |
| `golang.org/x/text` | pdfcpu, excelize, and `golang.org/x/image` all want it; the highest requested version wins |

This 16-entry list is `go.mod`'s own authoritative "what's actually
compiled into a go-valuate build" set. A fuller `go list -m all`/`go mod
graph` traversal additionally surfaces packages from openai-go's and
pdfcpu's own `go.mod` files (Azure SDK packages, `golang-jwt/jwt`,
`google/uuid`, `spf13/cobra`, `stretchr/testify`, etc.) that Go's module
graph pruning (available since Go 1.17) has determined are **not** needed
to build `go-valuate`'s own package set — typically test-only dependencies
of those libraries, or optional code paths (e.g. Azure AD auth for Azure
OpenAI) this module's code never reaches. Those are not go-valuate's
dependencies in any meaningful sense and are omitted from the table above.

## Replacement boundary summary

Every direct dependency is confined to a single, documented package (its
"isolation boundary" column above) whose exported API never leaks the
third-party type — this is a deliberate, verified design property, not
incidental. A future decision to replace any one of these libraries (e.g.
swapping `excelize` for a different XLSX reader) is scoped to rewriting
that one package's internals; no other package in this module would need
to change.
