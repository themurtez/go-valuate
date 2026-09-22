package financial

import (
	"errors"
	"fmt"
	"sort"
)

// ErrMissingCurrency is returned by Normalize when no currency is supplied.
// A FinancialDataset without a currency is not valid output, since every
// downstream valuation calculation depends on knowing what unit its amounts
// are in.
var ErrMissingCurrency = errors.New("financial: currency is required")

// ValidationError describes a problem with a single input row that prevented
// normalization from proceeding. Normalize collects every problem it finds
// and returns them together, rather than failing on the first one, so
// callers can surface a complete list of issues to fix.
type ValidationError struct {
	// SourceID identifies the offending MappedLineItem (its SourceID
	// field), if available.
	SourceID string
	// Index is the position of the offending item in the input slice,
	// included even when SourceID is empty.
	Index int
	// Reason describes what was wrong.
	Reason string
}

func (e *ValidationError) Error() string {
	if e.SourceID != "" {
		return fmt.Sprintf("financial: row %d (source_id=%q): %s", e.Index, e.SourceID, e.Reason)
	}
	return fmt.Sprintf("financial: row %d: %s", e.Index, e.Reason)
}

// ValidationErrors is a collection of ValidationError values returned when
// one or more input rows are malformed. It implements error so it can be
// returned and checked like any other error, while still allowing callers
// to inspect individual problems via errors.As.
type ValidationErrors []*ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 1 {
		return e[0].Error()
	}
	return fmt.Sprintf("financial: %d validation errors (first: %s)", len(e), e[0].Error())
}

// NormalizeOptions controls optional behavior of Normalize.
type NormalizeOptions struct {
	// Currency is the ISO 4217 currency code all input values are
	// denominated in (e.g. "USD"). Required; Normalize returns
	// ErrMissingCurrency if empty.
	Currency string
	// IncludeProvenance, when true, causes each resulting NormalizedItem to
	// carry SourceRef entries pointing back to every contributing row.
	// Defaults to false (no provenance) to keep output minimal unless the
	// caller asks for it.
	IncludeProvenance bool
}

// Normalize aggregates already-classified line items into a FinancialDataset.
//
// For each row in items, Normalize:
//   - skips rows with Status == RowStatusIgnored, RowStatusSubtotal, or
//     RowStatusTotal (subtotals/totals are excluded to avoid double
//     counting the rows they summarize),
//   - otherwise sums the row's per-period Values into the running total for
//     (row.Code, period), independently for each period,
//   - optionally records a SourceRef for every contributing (row, period)
//     pair when opts.IncludeProvenance is true.
//
// Normalize performs no classification: every row must already carry a
// canonical Code (unless ignored/subtotal/total). It is purely additive
// aggregation plus validation — it never inspects labels or infers a code.
//
// Normalize validates its input and returns ValidationErrors (satisfying
// error, inspectable via errors.As) if any row is malformed, e.g. a normal
// row missing its Code, or an unrecognized Status. All rows are checked
// before returning, so the caller sees every problem at once rather than
// one at a time. If opts.Currency is empty, Normalize returns
// ErrMissingCurrency immediately without inspecting rows.
//
// The returned FinancialDataset.Items order is deterministic: sorted by
// Code, then by Period.
func Normalize(items []MappedLineItem, opts NormalizeOptions) (FinancialDataset, error) {
	if opts.Currency == "" {
		return FinancialDataset{}, ErrMissingCurrency
	}

	var problems ValidationErrors
	type key struct {
		code   Code
		period Period
	}
	totals := make(map[key]float64)
	var sources map[key][]SourceRef
	if opts.IncludeProvenance {
		sources = make(map[key][]SourceRef)
	}

	for i, item := range items {
		status := item.Status
		if status == "" {
			status = RowStatusNormal
		}

		switch status {
		case RowStatusIgnored, RowStatusSubtotal, RowStatusTotal:
			continue
		case RowStatusNormal:
			// fall through to aggregation below
		default:
			problems = append(problems, &ValidationError{
				SourceID: item.SourceID,
				Index:    i,
				Reason:   fmt.Sprintf("unrecognized status %q", status),
			})
			continue
		}

		if item.Code == "" {
			problems = append(problems, &ValidationError{
				SourceID: item.SourceID,
				Index:    i,
				Reason:   "missing canonical code for non-ignored, non-subtotal, non-total row",
			})
			continue
		}

		for period, amount := range item.Values {
			k := key{code: item.Code, period: period}
			totals[k] += amount
			if opts.IncludeProvenance {
				sources[k] = append(sources[k], SourceRef{
					RowID:  item.SourceID,
					Label:  item.Label,
					Period: period,
					Amount: amount,
				})
			}
		}
	}

	if len(problems) > 0 {
		return FinancialDataset{}, problems
	}

	result := FinancialDataset{
		Currency: opts.Currency,
		Items:    make([]NormalizedItem, 0, len(totals)),
	}
	for k, amount := range totals {
		normalized := NormalizedItem{
			Code:   k.code,
			Period: k.period,
			Amount: amount,
		}
		if opts.IncludeProvenance {
			refs := sources[k]
			sort.Slice(refs, func(i, j int) bool {
				if refs[i].RowID != refs[j].RowID {
					return refs[i].RowID < refs[j].RowID
				}
				return refs[i].Period < refs[j].Period
			})
			normalized.Sources = refs
		}
		result.Items = append(result.Items, normalized)
	}

	sort.Slice(result.Items, func(i, j int) bool {
		if result.Items[i].Code != result.Items[j].Code {
			return result.Items[i].Code < result.Items[j].Code
		}
		return result.Items[i].Period < result.Items[j].Period
	})

	return result, nil
}
