package statements

import "github.com/themurtez/go-valuate/financial"

// Statement is a presentation-neutral, structured financial statement:
// ordered Sections, each holding ordered Rows, across one or more
// Periods. This is task section 17A's "output form A" — the model a
// future application renders (a table, a PDF, an HTML report) without
// re-deriving section/row structure itself. Nothing in this package
// renders a Statement to any output format.
//
// A Statement does not exist anywhere else in this repository: it is a
// new type this package introduces specifically to bridge accounting/
// ledger's account-level detail into the same kind of structural rows
// (financial.RowKind: heading/subtotal/total/normal) financial/
// classification already recognizes at the raw-row level, one layer up.
type Statement struct {
	// StatementType identifies which statement this is.
	StatementType financial.StatementType `json:"statement_type"`
	// Periods is the ordered set of periods this statement covers,
	// exactly Input.Periods' order (never re-sorted — a caller's period
	// order is its own explicit, deliberate choice, per the task's
	// "caller supplies period selection" rule applied to display order as
	// well as inclusion).
	Periods []financial.Period `json:"periods"`
	// Sections is the ordered list of statement sections — see
	// incomeStatementTemplate/balanceSheetTemplate in income.go/
	// balance.go for the fixed section ordering this package uses.
	Sections []Section `json:"sections"`
}

// Section is one named group of Rows within a Statement (e.g. "Revenue",
// "Current Assets").
type Section struct {
	// Name is the section's display label (e.g. "Operating Expenses").
	Name string `json:"name"`
	// Rows is the ordered list of rows within this section — see Row's
	// doc comment for the deterministic ordering rule (task section 31).
	Rows []Row `json:"rows"`
}

// Row is a single line within a Statement Section: either a mapped
// account's aggregated figures (Kind == financial.RowKindNormal) or a
// structural row (HEADING/SUBTOTAL/TOTAL) this package itself computes —
// see the task's explicit "use the actual existing financial.RowKind
// semantics... do not make structural rows normal financial accounts"
// instruction.
type Row struct {
	// Label is the row's display label — the canonical financial.Code's
	// registered CodeMeta.Label for a NORMAL row (never an individual
	// account's own Name, since a many-to-one mapping means one Row can
	// aggregate several accounts — see Contributors), or a fixed
	// structural label ("Gross Profit", "Total Assets", ...) for a
	// structural row.
	Label string `json:"label"`
	// FinancialCode is the canonical code this row represents. Empty for
	// a structural row that has no single underlying code (e.g. "Gross
	// Profit," which is a calculation over other rows, not a taxonomy
	// entry itself).
	FinancialCode financial.Code `json:"financial_code,omitempty"`
	// Kind is this row's structural role — the zero value
	// (financial.RowKindNormal) for an ordinary mapped-account row. This
	// package reuses financial.RowKind directly rather than defining a
	// parallel enum, exactly as review.StructurePayload already does for
	// the same reason (see review/types.go's identical choice).
	Kind financial.RowKind `json:"kind,omitempty"`
	// Values maps period to this row's canonical (sign-normalized)
	// amount for that period. A period absent from this map means "no
	// data for this row in that period" — distinct from a present period
	// mapping to exactly 0.0 (task section 22's zero-balance
	// distinction, carried through to the presentation layer).
	Values map[financial.Period]float64 `json:"values"`
	// Contributors lists the ledger accounts that were summed into this
	// row's Values, for provenance — see provenance.go. Empty for a
	// structural row.
	Contributors []RowContributor `json:"contributors,omitempty"`
}

// RowContributor is one ledger account's contribution to a mapped Row —
// the many-to-one provenance task section 19/20 requires ("output must
// show component accounts and amounts").
type RowContributor struct {
	AccountID     string `json:"account_id"`
	AccountNumber string `json:"account_number,omitempty"`
	AccountName   string `json:"account_name,omitempty"`
	// Amounts maps period to this specific account's own canonical
	// (sign-normalized, and — for an allocated account — already
	// percentage-applied) contribution to the parent Row's Values for
	// that period.
	Amounts map[financial.Period]float64 `json:"amounts,omitempty"`
	// AllocationPercent is set only when this contribution came from an
	// AllocationRule (one-to-many mapping), echoing the percentage
	// applied.
	AllocationPercent *float64 `json:"allocation_percent,omitempty"`
}

// mappedAccountFigures is the internal, per-mapped-target aggregate this
// package computes once (in income.go/balance.go) and reuses to build
// both a Statement's Rows and the FinancialDataset's NormalizedItems —
// see dataset.go. Keeping one aggregation function serving both outputs
// is what prevents the "map a source TB subtotal and then also
// recalculate it" double-counting failure mode the task warns against.
type mappedAccountFigures struct {
	code           financial.Code
	statementType  financial.StatementType
	valuesByPeriod map[financial.Period]float64
	contributors   []RowContributor
}
