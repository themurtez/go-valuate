package vendorspend

import "sort"

// TailSpend reports caller-defined tail-spend measures — task section 21.
// Never computed without an explicit Policy.TailSpend definition (see
// Available and Definition) — task section 21's "never report tail spend
// without the definition used" rule.
type TailSpend struct {
	Available bool `json:"available"`
	// Definition echoes which TailSpendDefinitionKind (and its
	// parameter) was actually applied.
	Definition TailSpendDefinitionKind `json:"definition,omitempty"`
	// DefinitionAmount/DefinitionTopN echo the specific threshold/cutoff
	// used, for whichever Definition applies.
	DefinitionAmount float64 `json:"definition_amount,omitempty"`
	DefinitionTopN   int     `json:"definition_top_n,omitempty"`

	TailSupplierCount int     `json:"tail_supplier_count"`
	TailSpendAmount   float64 `json:"tail_spend_amount"`
	TailSpendPercent  Value   `json:"tail_spend_percent"`
}

// computeTailSpend applies policy's tail-spend definition against
// supplierNetSpend (SupplierID -> total NetSpend for the scope being
// analyzed, normally the overall/all-period totals).
func computeTailSpend(supplierNetSpend map[string]float64, netTotal float64, policy TailSpendPolicy) TailSpend {
	kind := policy.kind()
	if kind == TailSpendUndefined {
		return TailSpend{}
	}

	ids := make([]string, 0, len(supplierNetSpend))
	for id := range supplierNetSpend {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		if supplierNetSpend[ids[i]] != supplierNetSpend[ids[j]] {
			return supplierNetSpend[ids[i]] > supplierNetSpend[ids[j]]
		}
		return ids[i] < ids[j]
	})

	ts := TailSpend{Available: true, Definition: kind}

	var tailIDs []string
	switch kind {
	case TailSpendBelowAmount:
		ts.DefinitionAmount = policy.BelowAmount
		for _, id := range ids {
			if supplierNetSpend[id] < policy.BelowAmount {
				tailIDs = append(tailIDs, id)
			}
		}
	case TailSpendOutsideTopN:
		ts.DefinitionTopN = policy.OutsideTopN
		if policy.OutsideTopN < len(ids) {
			tailIDs = ids[policy.OutsideTopN:]
		}
	}

	ts.TailSupplierCount = len(tailIDs)
	for _, id := range tailIDs {
		ts.TailSpendAmount += supplierNetSpend[id]
	}
	if netTotal != 0 {
		ts.TailSpendPercent = AvailableValue(ts.TailSpendAmount / netTotal)
	}
	return ts
}
