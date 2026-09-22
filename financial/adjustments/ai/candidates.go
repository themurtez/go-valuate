package ai

import (
	"github.com/themurtez/go-valuate/financial"
)

// CandidateCodes is the default deterministic set of financial.Code values
// SelectCandidates treats as plausible normalization-adjustment candidates
// (section 6): owner compensation, vehicle, travel, professional fees,
// repairs, other operating expense, and the non-operating/unusual income
// statement codes. A caller wanting a narrower or broader rule set builds
// its own []financial.Code and passes it via CandidatePolicy.Codes instead
// of using this default.
var CandidateCodes = []financial.Code{
	financial.CodeOpexOwnerComp,
	financial.CodeOpexVehicle,
	financial.CodeOpexTravel,
	financial.CodeOpexProfessionalFees,
	financial.CodeOpexRepairs,
	financial.CodeOpexOther,
	financial.CodeOtherIncome,
	financial.CodeOtherExpense,
}

// CandidatePolicy configures SelectCandidates' deterministic candidate-row
// rules (section 6). Every field is caller-controlled and transparent — no
// hidden heuristic beyond what these fields document.
type CandidatePolicy struct {
	// EnableCodeRules, when true, includes every SourceRow whose Code is a
	// member of Codes (or CandidateCodes if Codes is empty). Defaults to
	// false: candidate selection is opt-in, mirroring
	// financial/classification/ai.Policy's "AI is disabled by default"
	// convention.
	EnableCodeRules bool
	// Codes overrides CandidateCodes when EnableCodeRules is true and this
	// is non-empty.
	Codes []financial.Code
	// ExplicitRowIDs is a caller-supplied set of SourceRow.RowID values to
	// always include as candidates, regardless of EnableCodeRules/Codes
	// (section 6: "the caller may pass explicit candidate row IDs"). A row
	// ID here that does not match any supplied SourceRow is simply ignored
	// (not an error) — SelectCandidates only selects FROM the rows it is
	// given.
	ExplicitRowIDs []string
	// MaxRows caps the number of candidates SelectCandidates returns. Zero
	// means DefaultMaxCandidateRows. Rows are truncated only after
	// deterministic ordering (see SelectCandidates), never by map iteration.
	MaxRows int
}

// DefaultMaxCandidateRows is CandidatePolicy.MaxRows' default when zero.
const DefaultMaxCandidateRows = 40

func (p CandidatePolicy) maxRows() int {
	if p.MaxRows <= 0 {
		return DefaultMaxCandidateRows
	}
	return p.MaxRows
}

func (p CandidatePolicy) codes() []financial.Code {
	if len(p.Codes) > 0 {
		return p.Codes
	}
	return CandidateCodes
}

// isStructuralSourceRow reports whether row is a heading/subtotal/total row
// — mirrors financial/classification/ai's identical isStructuralKind guard
// (see that package's orchestrate.go), applied here so a structural row
// never even becomes a request candidate.
func isStructuralSourceRow(row SourceRow) bool {
	switch row.RowKind {
	case financial.RowKindHeading, financial.RowKindSubtotal, financial.RowKindTotal:
		return true
	default:
		return false
	}
}

// SelectCandidates deterministically narrows rows down to the bounded
// candidate set a Request will carry, per CandidatePolicy (section 6). Never
// mutates rows. A structural row or a row flagged AmbiguousOCR is never
// selected, regardless of policy — those are safety rules, not a candidate
// heuristic (see ValidateSuggestion for the identical rule enforced again at
// validation time, in case a caller bypasses SelectCandidates and builds a
// Request by hand).
//
// Selection order (for MaxRows truncation and for a stable, reviewable
// candidate list): explicit-ID matches first (in ExplicitRowIDs' own order,
// deduplicated), then code-rule matches (in rows' own input order) — so a
// caller-forced row is never silently dropped by a low MaxRows before a
// merely-heuristic match is.
func SelectCandidates(rows []SourceRow, policy CandidatePolicy) []SourceRow {
	byID := make(map[string]SourceRow, len(rows))
	for _, r := range rows {
		byID[r.RowID] = r
	}

	var out []SourceRow
	seen := make(map[string]bool, len(rows))

	for _, id := range policy.ExplicitRowIDs {
		row, ok := byID[id]
		if !ok || seen[id] {
			continue
		}
		if isStructuralSourceRow(row) || row.AmbiguousOCR {
			continue
		}
		seen[id] = true
		out = append(out, row)
	}

	if policy.EnableCodeRules {
		allowed := make(map[financial.Code]bool)
		for _, c := range policy.codes() {
			allowed[c] = true
		}
		for _, row := range rows {
			if seen[row.RowID] {
				continue
			}
			if row.Code == "" || !allowed[row.Code] {
				continue
			}
			if isStructuralSourceRow(row) || row.AmbiguousOCR {
				continue
			}
			seen[row.RowID] = true
			out = append(out, row)
		}
	}

	max := policy.maxRows()
	if max > 0 && len(out) > max {
		out = out[:max]
	}
	return out
}
