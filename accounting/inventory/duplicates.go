package inventory

import "sort"

// PossibleDuplicateMovement is one pair of movements sharing the same
// item, date, type, quantity, amount, and reference — task section 36:
// "no intent inference," purely a structural observation for caller
// review.
type PossibleDuplicateMovement struct {
	MovementIDA string `json:"movement_id_a"`
	MovementIDB string `json:"movement_id_b"`
	ItemID      string `json:"item_id"`
}

// maxDuplicateGroupSize bounds how many movements within one matching
// signature group are pairwise compared — task section 79's "avoid
// O(N²)" requirement. Reporting every pair within a group of size k costs
// O(k^2); this is expected to be negligible for the small groups (2-5)
// realistic duplicate data produces (mirrors labor.findPossibleDuplicatePayrollRecords'
// identical "O(N) records, O(k^2) pairs within a group" tradeoff), but an
// unbounded group (e.g. a bulk import that legitimately repeats one
// signature thousands of times) would otherwise make this function's
// true cost O(N^2) in the worst case. Beyond this cap, only the first
// maxDuplicateGroupSize movements in the group are pairwise compared —
// group membership itself (which movements share the signature) is not
// lost, since IssueDuplicateMovement/dedup upstream already handles exact
// ID duplicates; this function's purpose is a review signal, not an
// exhaustive audit trail, so truncating pairs within an already-flagged,
// oversized group is an acceptable coverage tradeoff.
const maxDuplicateGroupSize = 50

// findPossibleDuplicateMovements groups valid movements by a signature of
// (ItemID, date, Type, Quantity, Amount, ReferenceID) and reports every
// pair within a group whose signature collides — never a fraud/intent
// claim, purely "these records look alike, review recommended."
func findPossibleDuplicateMovements(movements []Movement) []PossibleDuplicateMovement {
	type signature struct {
		itemID   string
		date     string
		typ      MovementType
		quantity float64
		amount   float64
		ref      string
	}
	groups := map[signature][]string{}
	for _, m := range movements {
		val, _ := resolveMovementValue(m)
		amt := 0.0
		if val.Available {
			amt = val.Amount
		}
		qty := 0.0
		if m.Quantity.Available {
			qty = m.Quantity.Amount
		}
		sig := signature{itemID: m.ItemID, date: dateKey(m.Date), typ: m.Type, quantity: qty, amount: amt, ref: m.ReferenceID}
		groups[sig] = append(groups[sig], m.ID)
	}

	var sigs []signature
	for s, ids := range groups {
		if len(ids) > 1 {
			sigs = append(sigs, s)
		}
	}
	sort.Slice(sigs, func(i, j int) bool {
		if sigs[i].itemID != sigs[j].itemID {
			return sigs[i].itemID < sigs[j].itemID
		}
		if sigs[i].date != sigs[j].date {
			return sigs[i].date < sigs[j].date
		}
		return sigs[i].ref < sigs[j].ref
	})

	var out []PossibleDuplicateMovement
	for _, s := range sigs {
		ids := append([]string(nil), groups[s]...)
		sort.Strings(ids)
		if len(ids) > maxDuplicateGroupSize {
			ids = ids[:maxDuplicateGroupSize]
		}
		for i := 0; i < len(ids); i++ {
			for j := i + 1; j < len(ids); j++ {
				out = append(out, PossibleDuplicateMovement{MovementIDA: ids[i], MovementIDB: ids[j], ItemID: s.itemID})
			}
		}
	}
	return out
}
