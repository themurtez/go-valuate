package inventory

import "time"

// InventorySnapshot is one explicit as-of quantity/value record for an
// item (optionally at a specific location/lot). This package computes
// value from QuantityOnHand x UnitCost only when InventoryValue is not
// separately supplied — see resolveSnapshotValue in value.go. When a
// caller supplies both and they materially disagree, this package never
// silently picks one: it reports IssueValueQuantityCostMismatch and uses
// the explicitly supplied InventoryValue (the more direct fact) for
// downstream totals.
type InventorySnapshot struct {
	// ID uniquely identifies this snapshot record within one analysis.
	// Required; duplicates are flagged — see IssueDuplicateSnapshot.
	ID string `json:"id"`
	// ItemID references Item.ID. Required; an unresolved reference is
	// flagged — see IssueUnknownItem.
	ItemID string `json:"item_id"`
	// AsOfDate is this snapshot's own point-in-time date. Required.
	AsOfDate time.Time `json:"as_of_date"`

	QuantityOnHand Qty `json:"quantity_on_hand"`
	// UnitCost is the analytical/accounting unit cost as of AsOfDate.
	// Optional — see the package doc's costing boundary: this is a fact
	// the caller supplies, never derived here.
	UnitCost Value `json:"unit_cost"`
	// InventoryValue is the caller's own authoritative extended value for
	// this snapshot, when directly available (e.g. from a cost-layer
	// system this package does not replicate). Optional; see the type
	// doc comment for precedence when both this and QuantityOnHand x
	// UnitCost are supplied.
	InventoryValue Value `json:"inventory_value"`

	Location string `json:"location,omitempty"`
	// LotID is an opaque lot/batch identifier, preserved as provenance
	// only — see the package doc's serial-number boundary (task section
	// 49): this package never turns LotID into asset-level tracking.
	LotID string `json:"lot_id,omitempty"`
	// ReceivedDate is this lot's receipt date, used for lot-level aging
	// when supplied — see aging.go. Optional.
	ReceivedDate *time.Time `json:"received_date,omitempty"`
	// ExpiryDate is this lot's expiry date, used for expiry review — see
	// expiry.go. Optional.
	ExpiryDate *time.Time `json:"expiry_date,omitempty"`

	Currency string `json:"currency,omitempty"`

	SourceRef SourceRef `json:"source_ref,omitempty"`
}

// snapshotIdentityKey is the tuple this package requires to be unique
// across InventorySnapshot rows sharing the same ItemID and AsOfDate (task
// section 52: two snapshots for the same item/location/lot/as-of-date is a
// conflict, not an arbitrary pick). AsOfDate is normalized to its calendar
// date (see dateKey) so two snapshots differing only in time-of-day still
// collide, matching this package's "as-of DATE" (not timestamp) semantics
// used throughout aging and reconciliation.
type snapshotIdentityKey struct {
	itemID   string
	asOf     string // dateKey(AsOfDate)
	location string
	lotID    string
}

func snapshotIdentity(s InventorySnapshot) snapshotIdentityKey {
	return snapshotIdentityKey{itemID: s.ItemID, asOf: dateKey(s.AsOfDate), location: s.Location, lotID: s.LotID}
}

// dateKey formats t as its calendar date only ("2006-01-02"), the
// granularity every as-of/aging/reconciliation comparison in this package
// uses. Zero time.Time formats to "" so callers never mistake it for a
// real date collision.
func dateKey(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}
