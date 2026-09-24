package profitability

import "github.com/themurtez/go-valuate/financial"

// ControlTotalsFromFinancialDataset builds one period's ControlTotals
// from a financial.FinancialDataset — task section 39. This is a typed
// adapter, not a compile-time dependency baked into Calculate: this
// package's core types never import financial, and a caller who has no
// financial.FinancialDataset never needs this function at all.
//
// NetRevenue sums every financial.CategoryRevenue code's amount for
// period. DirectCost sums every financial.CategoryCogs code's amount for
// period — this package never forces statement COGS to equal a caller's
// own Fact-level DIRECT_* classification (task section 39's explicit
// instruction); a caller whose direct-cost Facts are classified
// differently than their statement COGS will see a genuine, informative
// ControlTotalMismatch rather than a silently-forced match.
// VariableCost/SharedCost are left Unavailable: financial.Code has no
// taxonomy split distinguishing variable operating cost or shared
// overhead from the rest of financial.CategoryOpex, so this adapter never
// guesses that split.
func ControlTotalsFromFinancialDataset(dataset financial.FinancialDataset, period string) ControlTotals {
	fp := financial.Period(period)
	var netRevenue, directCost float64
	var haveRevenue, haveCost bool
	for _, item := range dataset.Items {
		if item.Period != fp {
			continue
		}
		meta, ok := financial.LookupCode(item.Code)
		if !ok {
			continue
		}
		switch meta.Category {
		case financial.CategoryRevenue:
			netRevenue += item.Amount
			haveRevenue = true
		case financial.CategoryCogs:
			directCost += item.Amount
			haveCost = true
		}
	}
	c := ControlTotals{Period: period}
	if haveRevenue {
		c.NetRevenue = AvailableValue(netRevenue)
	}
	if haveCost {
		c.DirectCost = AvailableValue(directCost)
	}
	return c
}
