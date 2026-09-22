package ratios

import "github.com/themurtez/go-valuate/financial"

// codeIndex is a lookup from (code, period) to amount, built once per
// Calculate call so per-period computation doesn't repeatedly scan
// Dataset.Items — mirroring workingcapital.codeIndex/metrics.codeIndex's
// identical role.
type codeIndex map[financial.Code]map[financial.Period]float64

func buildIndex(ds financial.FinancialDataset) codeIndex {
	idx := make(codeIndex)
	for _, item := range ds.Items {
		byPeriod, ok := idx[item.Code]
		if !ok {
			byPeriod = make(map[financial.Period]float64)
			idx[item.Code] = byPeriod
		}
		byPeriod[item.Period] = item.Amount
	}
	return idx
}

func (idx codeIndex) lookup(code financial.Code, period financial.Period) (float64, bool) {
	byPeriod, ok := idx[code]
	if !ok {
		return 0, false
	}
	v, ok := byPeriod[period]
	return v, ok
}
