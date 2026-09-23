package inventory

import "time"

// MovementType is a closed set of recognized inventory movement kinds —
// task section 5: "only include types required for useful analysis."
// Direction (inbound/outbound/adjustment) is a semantic property of the
// type (see movementDirection), never inferred from Movement.Quantity's
// sign — Quantity is always a positive magnitude; see Movement's doc
// comment.
type MovementType string

const (
	MovementPurchaseReceipt    MovementType = "PURCHASE_RECEIPT"
	MovementCustomerShipment   MovementType = "CUSTOMER_SHIPMENT"
	MovementProductionIssue    MovementType = "PRODUCTION_ISSUE"
	MovementProductionReceipt  MovementType = "PRODUCTION_RECEIPT"
	MovementTransferIn         MovementType = "TRANSFER_IN"
	MovementTransferOut        MovementType = "TRANSFER_OUT"
	MovementReturnIn           MovementType = "RETURN_IN"
	MovementReturnOut          MovementType = "RETURN_OUT"
	MovementAdjustmentIncrease MovementType = "ADJUSTMENT_INCREASE"
	MovementAdjustmentDecrease MovementType = "ADJUSTMENT_DECREASE"
	MovementWriteOff           MovementType = "WRITE_OFF"
	MovementOtherIn            MovementType = "OTHER_IN"
	MovementOtherOut           MovementType = "OTHER_OUT"
)

// MovementDirection is the semantic effect of a MovementType on quantity
// on hand — task section 5: "prefer positive magnitude + typed direction
// rather than ambiguous signed quantity."
type MovementDirection string

const (
	DirectionInbound    MovementDirection = "INBOUND"
	DirectionOutbound   MovementDirection = "OUTBOUND"
	DirectionAdjustment MovementDirection = "ADJUSTMENT" // signed by type (increase/decrease), not a fixed direction
)

// movementTypeInfo is this package's closed, authoritative table of every
// recognized MovementType: its direction, whether it counts as "outbound
// usage" for last-outbound/velocity/slow-moving purposes, whether it is an
// adjustment-family type for AdjustmentSummary, and its adjustment sign
// (+1/-1/0) when it is.
type movementTypeInfo struct {
	direction      MovementDirection
	isOutbound     bool // counts toward LastOutboundDate/velocity/slow-moving evidence
	isAdjustment   bool // counts toward AdjustmentSummary
	adjustmentSign float64
}

var movementTypeTable = map[MovementType]movementTypeInfo{
	MovementPurchaseReceipt:    {direction: DirectionInbound},
	MovementCustomerShipment:   {direction: DirectionOutbound, isOutbound: true},
	MovementProductionIssue:    {direction: DirectionOutbound, isOutbound: true},
	MovementProductionReceipt:  {direction: DirectionInbound},
	MovementTransferIn:         {direction: DirectionInbound},
	MovementTransferOut:        {direction: DirectionOutbound, isOutbound: true},
	MovementReturnIn:           {direction: DirectionInbound},
	MovementReturnOut:          {direction: DirectionOutbound, isOutbound: true},
	MovementAdjustmentIncrease: {direction: DirectionAdjustment, isAdjustment: true, adjustmentSign: 1},
	MovementAdjustmentDecrease: {direction: DirectionAdjustment, isAdjustment: true, adjustmentSign: -1},
	MovementWriteOff:           {direction: DirectionAdjustment, isAdjustment: true, adjustmentSign: -1, isOutbound: true},
	MovementOtherIn:            {direction: DirectionInbound},
	MovementOtherOut:           {direction: DirectionOutbound, isOutbound: true},
}

// isRecognizedMovementType reports whether t is one of the fixed
// MovementType values.
func isRecognizedMovementType(t MovementType) bool {
	_, ok := movementTypeTable[t]
	return ok
}

// movementDirection returns t's semantic direction. Unrecognized types
// (already flagged during validation and excluded from computation) report
// "".
func movementDirection(t MovementType) MovementDirection {
	return movementTypeTable[t].direction
}

// isOutboundType reports whether t counts as outbound usage for
// last-movement, velocity, and slow/non-moving evidence — task section 19:
// "no outbound movement." WRITE_OFF counts as outbound-evidence (inventory
// left the books) but is analyzed separately as an adjustment, never
// folded into OutboundQuantity/OutboundValue period totals (see usage.go)
// so it is not double-counted as ordinary usage.
func isOutboundType(t MovementType) bool {
	return movementTypeTable[t].isOutbound
}

// isAdjustmentType reports whether t belongs to the adjustment family
// (ADJUSTMENT_INCREASE/ADJUSTMENT_DECREASE/WRITE_OFF) for AdjustmentSummary
// purposes — see adjustments.go.
func isAdjustmentType(t MovementType) bool {
	return movementTypeTable[t].isAdjustment
}

// adjustmentSign returns +1/-1/0 for the given MovementType's effect on
// quantity on hand within the adjustment family; 0 for non-adjustment
// types.
func adjustmentSign(t MovementType) float64 {
	return movementTypeTable[t].adjustmentSign
}

// ReasonCode is an opaque, caller-supplied reason for an adjustment or
// write-off movement. This package never interprets it, never infers
// shrinkage/damage/theft from a negative adjustment lacking one, and
// preserves it verbatim when supplied — task section 6/35: "if caller
// provides ReasonCode=SHRINKAGE, preserve it as source data, not package
// inference."
type ReasonCode string

// Movement is one portable inventory movement fact. Quantity is always a
// non-negative magnitude; MovementType (via movementDirection) determines
// whether it increases or decreases quantity on hand — never an implicit
// sign convention on Quantity itself.
type Movement struct {
	// ID uniquely identifies this movement within one analysis. Required;
	// exact duplicate IDs are flagged — see IssueDuplicateMovement.
	ID     string       `json:"id"`
	ItemID string       `json:"item_id"`
	Date   time.Time    `json:"date"`
	Type   MovementType `json:"type"`

	// Quantity is this movement's magnitude — always >= 0. Direction comes
	// from Type, never from Quantity's sign.
	Quantity Qty `json:"quantity"`
	// UnitCost is this movement's per-unit cost, when supplied — a fact,
	// never derived (costing boundary).
	UnitCost Value `json:"unit_cost"`
	// Amount is this movement's extended value, when directly supplied.
	// May be supplied instead of, or in addition to, UnitCost x Quantity;
	// this package never overwrites one with the other (mirrors
	// InventorySnapshot's identical precedence rule) — see value.go.
	Amount Value `json:"amount"`

	Location string `json:"location,omitempty"`
	// ReferenceID is an opaque caller reference (e.g. a PO or shipment
	// number), used for possible-duplicate-movement detection — see
	// duplicates.go.
	ReferenceID string `json:"reference_id,omitempty"`
	// ReasonCode is preserved source data for an adjustment/write-off
	// movement — see ReasonCode's doc comment.
	ReasonCode ReasonCode `json:"reason_code,omitempty"`

	Currency string `json:"currency,omitempty"`

	SourceRef SourceRef `json:"source_ref,omitempty"`
}
