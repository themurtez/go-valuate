package metrics

import "github.com/themurtez/go-valuate/financial"

// codeIndex is a (code, period) -> amount index built once per Calculate
// call so repeated lookups across many metrics don't each re-scan
// dataset.Items. It also tracks which (code, period) pairs are present, so
// availability can be judged by presence in the index rather than by
// reading a zero value that could equally be a real zero.
type codeIndex struct {
	amounts map[financial.Code]map[financial.Period]float64
}

func buildCodeIndex(dataset financial.FinancialDataset) codeIndex {
	idx := codeIndex{amounts: make(map[financial.Code]map[financial.Period]float64)}
	for _, item := range dataset.Items {
		periods, ok := idx.amounts[item.Code]
		if !ok {
			periods = make(map[financial.Period]float64)
			idx.amounts[item.Code] = periods
		}
		periods[item.Period] += item.Amount
	}
	return idx
}

// lookup returns the amount for one code in one period and whether it was
// present in the dataset at all.
func (idx codeIndex) lookup(code financial.Code, period financial.Period) (float64, bool) {
	periods, ok := idx.amounts[code]
	if !ok {
		return 0, false
	}
	amount, ok := periods[period]
	return amount, ok
}

// sumResult is the outcome of summing a set of codes for one period: the
// total, whether at least one of the codes was present, and the individual
// components that were found (in the order codes was given).
type sumResult struct {
	total      float64
	anyPresent bool
	components []Component
}

// sumCodes sums the amounts for the given codes in period, treating any code
// absent from the dataset as simply not contributing (not as zero-and-thus-
// disqualifying). anyPresent is true if at least one of the requested codes
// had an entry for period; this is the caller's signal for whether the sum
// is a genuine (if partial) figure or whether none of the requested data
// exists at all for this period.
//
// Per-code absence is intentionally tolerant (a business with no inventory
// line simply contributes nothing to current assets) while the caller
// decides, using anyPresent, whether a wholesale absence should instead be
// reported as Unavailable. This mirrors real statements: not every business
// has every account, but a statement with none of the requested accounts
// present likely means the section wasn't supplied at all.
func sumCodes(idx codeIndex, period financial.Period, codes ...financial.Code) sumResult {
	var res sumResult
	for _, code := range codes {
		amount, ok := idx.lookup(code, period)
		if !ok {
			continue
		}
		res.anyPresent = true
		res.total += amount
		meta, _ := financial.LookupCode(code)
		res.components = append(res.components, Component{
			Code:   string(code),
			Label:  meta.Label,
			Amount: amount,
		})
	}
	return res
}
