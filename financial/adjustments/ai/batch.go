package ai

import (
	"encoding/json"

	"github.com/themurtez/go-valuate/financial/adjustments"
)

// DefaultMaxRequestCharacters is BatchPolicy.MaxRequestCharacters' default
// when zero — a deterministic, conservative estimate/limit on one Request's
// marshaled-JSON size (not an exact token count), mirroring
// financial/classification/ai/openai.DefaultMaxBatchCharacters' identical
// role for that package's batch requests.
const DefaultMaxRequestCharacters = 24000

// BatchPolicy configures how BuildRequests splits a caller-selected
// candidate set into one or more bounded Request values (section 15):
// MaxCandidateRows is a row-count cap, MaxRequestCharacters is a
// size-estimate cap, and the two compose (whichever limit is hit first ends
// the current chunk) exactly like
// financial/classification/ai/openai's row-count-then-character-budget
// layering.
type BatchPolicy struct {
	// MaxCandidateRows caps how many SourceRow entries one Request may
	// carry. Zero means DefaultMaxCandidateRows (see candidates.go).
	MaxCandidateRows int
	// MaxRequestCharacters caps the estimated marshaled-JSON size of one
	// Request. Zero means DefaultMaxRequestCharacters.
	MaxRequestCharacters int
}

func (p BatchPolicy) maxCandidateRows() int {
	if p.MaxCandidateRows <= 0 {
		return DefaultMaxCandidateRows
	}
	return p.MaxCandidateRows
}

func (p BatchPolicy) maxRequestCharacters() int {
	if p.MaxRequestCharacters <= 0 {
		return DefaultMaxRequestCharacters
	}
	return p.MaxRequestCharacters
}

// BuildRequests deterministically splits candidates into one or more Request
// values, each carrying allowedTypes plus whatever ContextRows/
// MultiYearValues entries in contextByRowID/multiYearByRowID apply to its
// own rows — never reordering candidates, never depending on map iteration
// for chunk boundaries (only for the per-row context/multi-year lookups,
// which are keyed by RowID and therefore order-independent). Source
// identity (RowID+Period) is preserved verbatim across every chunk, so a
// caller can always reassemble which candidate a returned Suggestion
// belongs to regardless of which chunk carried it (section 15's "preserve
// source identity across batches" requirement).
//
// A single candidate row whose own estimated size already exceeds
// policy.MaxRequestCharacters is never dropped — it forms its own one-row
// chunk (mirroring
// financial/classification/ai/openai.splitBatchByCharacterBudget's
// identical no-drop guarantee) rather than being silently sent unbounded or
// discarded.
func BuildRequests(candidates []SourceRow, allowedTypes []adjustments.TypeMeta, industryContext string, contextByRowID map[string][]ContextRow, multiYearByRowID map[string][]MultiYearValue, policy BatchPolicy) []Request {
	if len(candidates) == 0 {
		return nil
	}

	types := BuildAllowedTypes(allowedTypes)
	maxRows := policy.maxCandidateRows()
	maxChars := policy.maxRequestCharacters()

	var chunks [][]SourceRow
	start := 0
	chunkChars := 0
	for i, row := range candidates {
		rowChars := estimateSourceRowChars(row)
		rowCount := i - start
		if rowCount > 0 && (rowCount >= maxRows || chunkChars+rowChars > maxChars) {
			chunks = append(chunks, candidates[start:i])
			start = i
			chunkChars = 0
		}
		chunkChars += rowChars
	}
	chunks = append(chunks, candidates[start:])

	reqs := make([]Request, len(chunks))
	for i, chunk := range chunks {
		reqs[i] = Request{
			Candidates:      chunk,
			AllowedTypes:    types,
			IndustryContext: industryContext,
			ContextRows:     subsetContext(chunk, contextByRowID),
			MultiYearValues: subsetMultiYear(chunk, multiYearByRowID),
		}
	}
	return reqs
}

func estimateSourceRowChars(row SourceRow) int {
	data, _ := json.Marshal(row)
	return len(data)
}

func subsetContext(chunk []SourceRow, contextByRowID map[string][]ContextRow) map[string][]ContextRow {
	if len(contextByRowID) == 0 {
		return nil
	}
	out := make(map[string][]ContextRow)
	for _, row := range chunk {
		if ctx, ok := contextByRowID[row.RowID]; ok {
			out[row.RowID] = ctx
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func subsetMultiYear(chunk []SourceRow, multiYearByRowID map[string][]MultiYearValue) map[string][]MultiYearValue {
	if len(multiYearByRowID) == 0 {
		return nil
	}
	out := make(map[string][]MultiYearValue)
	for _, row := range chunk {
		if vals, ok := multiYearByRowID[row.RowID]; ok {
			out[row.RowID] = vals
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
