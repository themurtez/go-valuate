package vendorspend

import "sort"

// NonPreferredSpend is one supplier's net spend that is not flagged as
// coming from a PreferredSupplier — task section 23. Populated only when
// at least one Supplier in Input.Suppliers has PreferredSupplier == true
// (otherwise "non-preferred" is meaningless: nothing was ever declared
// preferred, so this package never infers preferred status by its
// absence — see Available).
type NonPreferredSpend struct {
	Available         bool                  `json:"available"`
	Suppliers         []SupplierPolicySpend `json:"suppliers,omitempty"`
	TotalSpend        float64               `json:"total_spend"`
	TotalSpendPercent Value                 `json:"total_spend_percent"`
}

// OutsideContractedSpend mirrors NonPreferredSpend for ContractedSupplier
// — task section 23.
type OutsideContractedSpend struct {
	Available         bool                  `json:"available"`
	Suppliers         []SupplierPolicySpend `json:"suppliers,omitempty"`
	TotalSpend        float64               `json:"total_spend"`
	TotalSpendPercent Value                 `json:"total_spend_percent"`
}

// SupplierPolicySpend is one supplier's net spend contributing to
// NonPreferredSpend/OutsideContractedSpend.
type SupplierPolicySpend struct {
	SupplierID string  `json:"supplier_id"`
	Spend      float64 `json:"spend"`
}

// computeSupplierPolicySpend computes NonPreferredSpend and
// OutsideContractedSpend from supplier-level overall net spend.
// Availability requires at least one supplier explicitly declared
// preferred/contracted respectively — a caller who never populated
// either field gets Available == false rather than every supplier being
// silently treated as "non-preferred."
func computeSupplierPolicySpend(supplierNetSpend map[string]float64, suppliersByID map[string]Supplier, netTotal float64) (NonPreferredSpend, OutsideContractedSpend) {
	anyPreferred, anyContracted := false, false
	for _, s := range suppliersByID {
		if s.PreferredSupplier {
			anyPreferred = true
		}
		if s.ContractedSupplier {
			anyContracted = true
		}
	}

	ids := make([]string, 0, len(supplierNetSpend))
	for id := range supplierNetSpend {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	var np NonPreferredSpend
	if anyPreferred {
		np.Available = true
		for _, id := range ids {
			if suppliersByID[id].PreferredSupplier {
				continue
			}
			amt := supplierNetSpend[id]
			np.Suppliers = append(np.Suppliers, SupplierPolicySpend{SupplierID: id, Spend: amt})
			np.TotalSpend += amt
		}
		if netTotal != 0 {
			np.TotalSpendPercent = AvailableValue(np.TotalSpend / netTotal)
		}
	}

	var oc OutsideContractedSpend
	if anyContracted {
		oc.Available = true
		for _, id := range ids {
			if suppliersByID[id].ContractedSupplier {
				continue
			}
			amt := supplierNetSpend[id]
			oc.Suppliers = append(oc.Suppliers, SupplierPolicySpend{SupplierID: id, Spend: amt})
			oc.TotalSpend += amt
		}
		if netTotal != 0 {
			oc.TotalSpendPercent = AvailableValue(oc.TotalSpend / netTotal)
		}
	}

	return np, oc
}
