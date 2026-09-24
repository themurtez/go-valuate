package vendorspend

import "github.com/themurtez/go-valuate/accounting/profitability"

// ProfitabilityClassification is the caller's explicit per-SpendRecord
// classification into accounting/profitability's Component/Attribution
// model — task section 28's "only when caller explicitly classifies the
// spend" rule. A SpendRecord absent from the classifications map supplied
// to FactsFromSpendRecords is simply not converted; this package never
// infers a Component from SpendType or automatically routes vendor spend
// into profitability — task section 28's "do not automatically send all
// vendor spend into profitability" rule.
type ProfitabilityClassification struct {
	Component    profitability.Component
	Attributions []profitability.Attribution
}

// FactsFromSpendRecords converts records into accounting/profitability
// Facts, using ONLY the caller-supplied classifications map (keyed by
// SpendID) — task section 28. This is a typed adapter, not a
// compile-time dependency baked into Calculate: this package's core
// types never import accounting/profitability, and a caller with no
// profitability-package usage never needs this function.
func FactsFromSpendRecords(records []SpendRecord, classifications map[string]ProfitabilityClassification) []profitability.Fact {
	out := make([]profitability.Fact, 0, len(classifications))
	for _, r := range records {
		cls, ok := classifications[r.SpendID]
		if !ok {
			continue
		}
		date := r.Date
		out = append(out, profitability.Fact{
			FactID:       r.SpendID,
			Period:       r.Period,
			Date:         &date,
			Component:    cls.Component,
			Amount:       absFloat(netSpendOf(r)),
			Attributions: cls.Attributions,
			SourceType:   "vendorspend",
			SourceID:     r.SpendID,
			Currency:     r.Currency,
		})
	}
	return out
}
