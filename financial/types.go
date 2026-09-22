// Package financial defines the canonical, presentation-agnostic financial
// model used by valuation domain logic: statement types, reporting periods,
// raw and classified line items, normalized financial data, and provenance
// references back to source rows.
//
// Nothing in this package knows about PDF, XLSX, CSV, QuickBooks, databases,
// or UI concerns. It consumes and produces plain Go structs that are
// JSON-compatible, so callers can serialize/deserialize freely at their own
// boundaries.
package financial

// StatementType identifies which financial statement a line item belongs to.
type StatementType string

const (
	StatementIncomeStatement StatementType = "income_statement"
	StatementBalanceSheet    StatementType = "balance_sheet"
	StatementCashFlow        StatementType = "cash_flow"
)

// Period identifies a single reporting period, such as a fiscal year,
// quarter, or month. It is intentionally a plain string so callers can use
// whatever granularity and formatting convention their data has ("2025",
// "2025-Q1", "2025-03"), as long as it is used consistently.
type Period string

// RowStatus classifies how a raw or mapped line item should be treated
// during normalization and aggregation.
type RowStatus string

const (
	// RowStatusNormal is an ordinary line item that should be included in
	// aggregation.
	RowStatusNormal RowStatus = "normal"
	// RowStatusSubtotal is a subtotal row (e.g. "Total Operating Expenses")
	// that must be excluded from aggregation to avoid double counting.
	RowStatusSubtotal RowStatus = "subtotal"
	// RowStatusTotal is a grand total row (e.g. "Total Expenses") that must
	// be excluded from aggregation to avoid double counting.
	RowStatusTotal RowStatus = "total"
	// RowStatusIgnored marks a row that should be skipped entirely, e.g. a
	// blank separator row or an explicitly excluded line.
	RowStatusIgnored RowStatus = "ignored"
)

// RowKind classifies the structural role a source row plays in its
// document, as read by the ingestion/extraction layer, independent of
// RowStatus. Where RowStatus is the normalize-time directive ("should this
// row's Values be aggregated?"), RowKind is the richer upstream structural
// signal an adapter (ingestion/csv, ingestion/xlsx, ingestion/pdf, or any
// other caller) can supply about what the row actually is in the source
// document — including RowKindHeading, a concept RowStatus cannot express
// on its own (RowStatusIgnored also covers blank separators and other
// caller-excluded rows, not just headings).
//
// The zero value is RowKindNormal, so an existing caller that never sets
// this field (including a bare RawLineItem{} built by hand, as many
// existing tests do) gets exactly today's behavior: classification treats
// the row as carrying no upstream structural hint and falls back to its own
// label-based heuristic. This is what makes the field backward compatible.
//
// financial/classification.Classify reads RawLineItem.Kind before running
// its own structural-detection heuristic: a non-zero Kind (HEADING,
// SUBTOTAL, or TOTAL) short-circuits classification's narrower label-token
// check, so an adapter that has already done a better structural read (e.g.
// ingestion's ClassifyRowKind, which recognizes "Gross Profit" as a
// subtotal from label shape alone) does not have its answer silently
// overridden by a weaker one downstream. See financial/classification's
// package doc comment for the full precedence pipeline.
type RowKind string

const (
	// RowKindNormal is an ordinary line item, or "no upstream structural
	// signal supplied" (the zero value — see the RowKind doc comment).
	RowKindNormal RowKind = ""
	// RowKindHeading is a section heading/title row: it introduces a group
	// of following rows (e.g. "Operating Expenses") and carries no
	// financial amount of its own. RowStatus has no equivalent concept;
	// classification maps RowKindHeading to RowStatusIgnored, which
	// financial.Normalize already excludes from aggregation.
	RowKindHeading RowKind = "heading"
	// RowKindSubtotal is a subtotal row (e.g. "Total Operating Expenses",
	// "Gross Profit") that summarizes a group of preceding rows and must be
	// excluded from aggregation to avoid double counting.
	RowKindSubtotal RowKind = "subtotal"
	// RowKindTotal is a grand-total row (e.g. "Net Income", "Total Assets")
	// that summarizes the whole statement and must be excluded from
	// aggregation to avoid double counting.
	RowKindTotal RowKind = "total"
)

// SourceRef is a provenance reference back to a single source row. Normalized
// financial values may carry zero or more of these so downstream consumers
// can trace an aggregated figure back to the original data it was built
// from.
type SourceRef struct {
	// RowID is the identifier of the source row (RawLineItem.ID or
	// MappedLineItem.ID) this reference points to.
	RowID string `json:"row_id"`
	// Label is the human-readable label of the source row at the time it
	// was captured, preserved for readability even if the source row is
	// later unavailable.
	Label string `json:"label,omitempty"`
	// Period is the specific period within the source row that contributed
	// to the normalized value, since a single row can carry values for
	// multiple periods.
	Period Period `json:"period"`
	// Amount is the value contributed by this source row for Period.
	Amount float64 `json:"amount"`
}

// RawLineItem represents a single row of financial data as it appears in a
// source document, before any classification or mapping to a canonical
// taxonomy code. Values are keyed by period so a single row can carry
// multiple years/columns, matching how financial statements are commonly
// laid out.
type RawLineItem struct {
	// ID is a caller-assigned identifier unique within the source document
	// (e.g. "row-42"). Used for provenance tracking.
	ID string `json:"id"`
	// StatementType indicates which statement this row was extracted from.
	StatementType StatementType `json:"statement_type"`
	// Label is the line item's label as it appeared in the source
	// (e.g. "Advertising & Promotion").
	Label string `json:"label"`
	// ParentLabel is the label of the enclosing group/section, if any
	// (e.g. "Operating Expenses"). Optional.
	ParentLabel string `json:"parent_label,omitempty"`
	// Kind is the upstream structural read of this row (heading/subtotal/
	// total), when the caller/adapter that produced this RawLineItem
	// already determined one. Zero value (RowKindNormal) means no upstream
	// signal was supplied, in which case financial/classification falls
	// back to its own label-based structural heuristic exactly as it did
	// before this field existed. See RowKind's doc comment.
	Kind RowKind `json:"kind,omitempty"`
	// Values maps period to the reported amount for that period.
	Values map[Period]float64 `json:"values"`
}

// MappedLineItem represents a raw line item that has been classified against
// the canonical taxonomy. It is the input to the normalizer. Classification
// (i.e. producing MappedLineItem values from RawLineItem values) is
// explicitly out of scope for this package; callers supply already-mapped
// items.
type MappedLineItem struct {
	// SourceID identifies the originating raw row, for provenance.
	SourceID string `json:"source_id"`
	// Label is the source row's label, preserved for provenance/readability.
	Label string `json:"label,omitempty"`
	// StatementType indicates which statement this item belongs to.
	StatementType StatementType `json:"statement_type"`
	// Code is the canonical taxonomy code this row has been classified as
	// (e.g. taxonomy.CodeOpexMarketing). May be empty only if Status is
	// RowStatusIgnored, RowStatusSubtotal, or RowStatusTotal.
	Code Code `json:"code"`
	// Status indicates how this row should be treated during aggregation.
	// The zero value is treated as RowStatusNormal.
	Status RowStatus `json:"status,omitempty"`
	// Kind carries forward the upstream structural read from the
	// originating RawLineItem (see RawLineItem.Kind), preserved through
	// classification so a downstream consumer (e.g. a future review UI)
	// can distinguish a heading from an ordinary ignored row without
	// re-deriving it. Classification does not use this field itself —
	// Status is what drives normalization; Kind is carried for provenance/
	// display only.
	Kind RowKind `json:"kind,omitempty"`
	// Values maps period to the reported amount for that period.
	Values map[Period]float64 `json:"values"`
}

// NormalizedItem is a single aggregated financial figure: one canonical
// code, for one period, with an amount and optional provenance back to the
// source rows that contributed to it.
type NormalizedItem struct {
	// Code is the canonical taxonomy code.
	Code Code `json:"code"`
	// Period is the reporting period this amount applies to.
	Period Period `json:"period"`
	// Amount is the aggregated value for Code in Period.
	Amount float64 `json:"amount"`
	// Sources optionally traces this amount back to the one or more source
	// rows that were summed to produce it. Omitted when provenance was not
	// requested or is unavailable.
	Sources []SourceRef `json:"sources,omitempty"`
}

// FinancialDataset is a normalized collection of financial data: a flat list
// of NormalizedItem values plus the currency they are denominated in. It is
// the output of the normalizer and the primary input to downstream valuation
// modules.
type FinancialDataset struct {
	// Currency is the ISO 4217 currency code (e.g. "USD") all amounts in
	// Items are denominated in. Required and never empty.
	Currency string `json:"currency"`
	// Items is the flat set of normalized (code, period) aggregates.
	// Deterministically sorted by Code, then by Period, whenever produced
	// by Normalize — never relies on Go map iteration order. A caller
	// constructing a FinancialDataset directly (e.g. deserializing one
	// from storage) is responsible for preserving this order if it wants
	// downstream comparisons/tests to remain stable.
	Items []NormalizedItem `json:"items"`
}

// ByCodeAndPeriod returns the NormalizedItem for the given code and period,
// and whether it was found. This is a convenience lookup; FinancialDataset
// itself stores Items as a flat slice to keep the type JSON-friendly and
// order-stable.
func (d FinancialDataset) ByCodeAndPeriod(code Code, period Period) (NormalizedItem, bool) {
	for _, item := range d.Items {
		if item.Code == code && item.Period == period {
			return item, true
		}
	}
	return NormalizedItem{}, false
}

// Periods returns the distinct set of periods present in the dataset, sorted
// lexically for determinism. Callers needing chronological order should
// ensure their Period values sort lexically as such (e.g. "2024" < "2025",
// or "2025-Q1" < "2025-Q2").
func (d FinancialDataset) Periods() []Period {
	seen := make(map[Period]struct{})
	var periods []Period
	for _, item := range d.Items {
		if _, ok := seen[item.Period]; !ok {
			seen[item.Period] = struct{}{}
			periods = append(periods, item.Period)
		}
	}
	sortPeriods(periods)
	return periods
}
