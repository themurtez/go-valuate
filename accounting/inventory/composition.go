package inventory

// ClassShare is one InventoryClass's value and share of total inventory
// value — task section 46.
type ClassShare struct {
	Class InventoryClass `json:"class"`
	Value float64        `json:"value"`
	Share Value          `json:"share"`
}

// CompositionSummary is the value/share breakdown by caller-defined
// InventoryClass — task section 46. Unclassified items (Item.Class == "")
// are reported separately rather than silently omitted or guessed.
type CompositionSummary struct {
	Available         bool         `json:"available"`
	ByClass           []ClassShare `json:"by_class,omitempty"`
	UnclassifiedValue float64      `json:"unclassified_value"`
	UnclassifiedShare Value        `json:"unclassified_share"`
}

// classOrder fixes InventoryClass declaration order for deterministic
// ByClass output.
var classOrder = []InventoryClass{ClassRawMaterial, ClassWIP, ClassFinishedGood, ClassMerchandise, ClassSupplies, ClassOther}

func buildCompositionSummary(order []string, states map[string]*itemState) CompositionSummary {
	byClass := map[InventoryClass]float64{}
	var unclassified float64
	var total float64
	var any bool

	for _, id := range order {
		st := states[id]
		if !st.value.Available {
			continue
		}
		any = true
		total += st.value.Amount
		if st.item.Class == "" {
			unclassified += st.value.Amount
			continue
		}
		byClass[st.item.Class] += st.value.Amount
	}
	if !any {
		return CompositionSummary{}
	}

	var shares []ClassShare
	for _, c := range classOrder {
		v, ok := byClass[c]
		if !ok {
			continue
		}
		cs := ClassShare{Class: c, Value: v}
		if total != 0 {
			cs.Share = AvailableValue(v / total)
		}
		shares = append(shares, cs)
	}

	summary := CompositionSummary{Available: true, ByClass: shares, UnclassifiedValue: unclassified}
	if total != 0 {
		summary.UnclassifiedShare = AvailableValue(unclassified / total)
	}
	return summary
}
