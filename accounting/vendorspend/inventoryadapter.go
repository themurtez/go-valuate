package vendorspend

import "github.com/themurtez/go-valuate/accounting/inventory"

// InventoryPurchaseReceipt is the caller-supplied bridge between one
// accounting/inventory Movement (a MovementTypePurchase or
// MovementTypeReceipt row) and the SupplierID/authoritative purchase
// value this package requires — task section 26. accounting/inventory's
// own Movement type carries NO SupplierID field at all, so this adapter
// can never attribute a Movement to a supplier on its own; a caller must
// supply that identity explicitly via this bridge struct (see
// SpendRecordsFromInventoryReceipts's doc comment).
type InventoryPurchaseReceipt struct {
	MovementID string
	SupplierID string
	// Amount is the authoritative purchase value for this receipt —
	// caller-supplied, since Movement.Amount/UnitCost*Quantity may be
	// absent or (per accounting/inventory's own costing boundary) not
	// authoritative for vendor-spend purposes.
	Amount float64
	Period string
}

// SpendRecordsFromInventoryReceipts converts accounting/inventory
// Movements into SpendRecords under BasisReceipt, using only the
// (MovementID -> InventoryPurchaseReceipt) bridge the caller explicitly
// supplies in receipts — task section 26's "may adapt only when
// SupplierID + authoritative purchase value are explicitly available"
// rule. A Movement whose ID has no entry in receipts is skipped (never
// guessed); ok reports whether at least one Movement had explicit
// supplier attribution, so a caller can distinguish "adapter ran but
// found nothing" from "adapter is unavailable because no attribution was
// ever supplied" — task section 26's "if current inventory data lacks
// supplier identity, adapter must be unavailable" rule.
func SpendRecordsFromInventoryReceipts(movements []inventory.Movement, receipts map[string]InventoryPurchaseReceipt) (records []SpendRecord, ok bool) {
	if len(receipts) == 0 {
		return nil, false
	}
	for _, m := range movements {
		receipt, has := receipts[m.ID]
		if !has || receipt.SupplierID == "" {
			continue
		}
		records = append(records, SpendRecord{
			SpendID:    m.ID,
			SupplierID: receipt.SupplierID,
			Period:     receipt.Period,
			Date:       m.Date,
			Amount:     receipt.Amount,
			Currency:   m.Currency,
			Effect:     EffectNormal,
			Basis:      BasisReceipt,
		})
		ok = true
	}
	return records, ok
}
