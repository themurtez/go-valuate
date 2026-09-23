package inventory

import "time"

// PurchaseSummary is period purchase-receipt activity — task section 30.
type PurchaseSummary struct {
	Available bool `json:"available"`

	PurchaseQuantity   Qty   `json:"purchase_quantity"`
	PurchaseValue      Value `json:"purchase_value"`
	ReceiptCount       int   `json:"receipt_count"`
	AverageReceiptSize Value `json:"average_receipt_size"`
}

// UsageSummary is period outbound-usage activity, split by movement
// direction category — task section 31: "distinguish customer shipment,
// production issue, transfer, write-off" and "do not treat all outbound
// movements as COGS."
type UsageSummary struct {
	Available bool `json:"available"`

	OutboundQuantity Qty   `json:"outbound_quantity"`
	OutboundValue    Value `json:"outbound_value"`
	ShipmentCount    int   `json:"shipment_count"`

	CustomerShipmentValue Value `json:"customer_shipment_value"`
	ProductionIssueValue  Value `json:"production_issue_value"`
	TransferOutValue      Value `json:"transfer_out_value"`
	ReturnOutValue        Value `json:"return_out_value"`
	OtherOutValue         Value `json:"other_out_value"`
	// WriteOffValue is reported here for visibility but is NOT included
	// in OutboundValue/OutboundQuantity — write-offs are adjustment
	// activity (see AdjustmentSummary), never ordinary usage.
	WriteOffValue Value `json:"write_off_value"`
}

// valueAccumulator sums Value figures while tracking whether any
// contributing row was available — the shared accumulator this file uses
// for every per-category value total instead of a bare (float64, bool)
// pair repeated per category.
type valueAccumulator struct {
	sum float64
	ok  bool
}

func (a *valueAccumulator) add(v Value) {
	if v.Available {
		a.sum += v.Amount
		a.ok = true
	}
}

func (a valueAccumulator) result() Value {
	if !a.ok {
		return Unavailable()
	}
	return AvailableValue(a.sum)
}

// qtyAccumulator sums Qty figures only while every contributing row
// shares the same unit of measure — task section 62's UOM-safety rule
// applied to period quantity totals. The first row seen fixes the
// aggregate's UOM; a later row in a different (non-empty) unit stops the
// aggregate (mismatched), while a row with no stated unit is simply
// skipped for quantity purposes (it still contributes to value totals via
// a separate valueAccumulator).
type qtyAccumulator struct {
	sum        float64
	uom        string
	seenUOM    bool
	ok         bool
	mismatched bool
}

func (a *qtyAccumulator) add(q Qty) {
	if !q.Available || a.mismatched {
		return
	}
	if !a.seenUOM {
		a.uom = q.UnitOfMeasure
		a.seenUOM = true
	} else if a.uom != q.UnitOfMeasure {
		a.mismatched = true
		a.ok = false
		return
	}
	a.sum += q.Amount
	a.ok = true
}

func (a qtyAccumulator) result() Qty {
	if !a.ok {
		return UnavailableQty()
	}
	return AvailableQty(a.sum, a.uom)
}

// periodMovementTotals accumulates one period's movement-derived
// purchase/usage/adjustment facts from valid movements whose Date falls
// within [start, end].
type periodMovementTotals struct {
	purchaseQty   qtyAccumulator
	purchaseValue valueAccumulator
	receiptCount  int

	outboundQty   qtyAccumulator
	outboundValue valueAccumulator
	shipmentCount int

	customerShipmentValue valueAccumulator
	productionIssueValue  valueAccumulator
	transferOutValue      valueAccumulator
	returnOutValue        valueAccumulator
	otherOutValue         valueAccumulator
	writeOffValue         valueAccumulator
}

func buildPeriodMovementTotals(movements []Movement, start, end time.Time) periodMovementTotals {
	var t periodMovementTotals
	for _, m := range movements {
		if m.Date.Before(start) || m.Date.After(end) {
			continue
		}
		val, _ := resolveMovementValue(m)

		switch m.Type {
		case MovementPurchaseReceipt:
			t.receiptCount++
			t.purchaseValue.add(val)
			t.purchaseQty.add(m.Quantity)
		case MovementWriteOff:
			t.writeOffValue.add(val)
		default:
			if !isOutboundType(m.Type) {
				continue
			}
			t.shipmentCount++
			t.outboundValue.add(val)
			t.outboundQty.add(m.Quantity)
			switch m.Type {
			case MovementCustomerShipment:
				t.customerShipmentValue.add(val)
			case MovementProductionIssue:
				t.productionIssueValue.add(val)
			case MovementTransferOut:
				t.transferOutValue.add(val)
			case MovementReturnOut:
				t.returnOutValue.add(val)
			case MovementOtherOut:
				t.otherOutValue.add(val)
			}
		}
	}
	return t
}

// resolvedPurchaseSummary builds PurchaseSummary from movement totals if
// any movement evidence exists for the period, otherwise from
// PeriodFinancials (summary path) — task section 2's convergence rule.
func resolvedPurchaseSummary(mt periodMovementTotals, hasMovementData bool, fin *PeriodFinancials) PurchaseSummary {
	if hasMovementData {
		s := PurchaseSummary{Available: mt.receiptCount > 0 || mt.purchaseValue.ok, ReceiptCount: mt.receiptCount}
		s.PurchaseQuantity = mt.purchaseQty.result()
		s.PurchaseValue = mt.purchaseValue.result()
		if mt.receiptCount > 0 && mt.purchaseValue.ok {
			s.AverageReceiptSize = AvailableValue(mt.purchaseValue.sum / float64(mt.receiptCount))
		}
		return s
	}
	if fin == nil {
		return PurchaseSummary{}
	}
	s := PurchaseSummary{}
	if fin.PurchaseValue.Available {
		s.Available = true
		s.PurchaseValue = fin.PurchaseValue
	}
	if fin.PurchaseUnits.Available {
		s.Available = true
		s.PurchaseQuantity = AvailableQty(fin.PurchaseUnits.Amount, "")
	}
	return s
}

// resolvedUsageSummary mirrors resolvedPurchaseSummary for outbound
// usage.
func resolvedUsageSummary(mt periodMovementTotals, hasMovementData bool, fin *PeriodFinancials) UsageSummary {
	if hasMovementData {
		s := UsageSummary{Available: mt.shipmentCount > 0 || mt.outboundValue.ok, ShipmentCount: mt.shipmentCount}
		s.OutboundQuantity = mt.outboundQty.result()
		s.OutboundValue = mt.outboundValue.result()
		s.CustomerShipmentValue = mt.customerShipmentValue.result()
		s.ProductionIssueValue = mt.productionIssueValue.result()
		s.TransferOutValue = mt.transferOutValue.result()
		s.ReturnOutValue = mt.returnOutValue.result()
		s.OtherOutValue = mt.otherOutValue.result()
		s.WriteOffValue = mt.writeOffValue.result()
		return s
	}
	if fin == nil {
		return UsageSummary{}
	}
	s := UsageSummary{}
	if fin.UnitsSold.Available {
		s.Available = true
		s.OutboundQuantity = AvailableQty(fin.UnitsSold.Amount, "")
	}
	// Note: the summary path has no separate outbound-VALUE fact
	// distinct from COGS (task section 2B's summary fields are
	// beginning/ending inventory, COGS, purchases, units) — OutboundValue
	// is intentionally left Unavailable rather than assuming
	// OutboundValue == COGS, since COGS excludes non-COGS outbound
	// activity (returns, transfers, write-offs) that a detailed-path
	// analysis keeps separate. A caller wanting an approximate
	// summary-path outbound value can compare PeriodFinancials.COGS
	// directly via PurchaseVsUsageTrend's COGS-basis comparison instead.
	return s
}

// PurchaseVsUsageTrend is the factual purchases-vs-usage comparison —
// task section 32.
type PurchaseVsUsageTrend struct {
	Available bool `json:"available"`

	Period             string `json:"period"`
	PurchasesValue     Value  `json:"purchases_value"`
	OutboundUsageValue Value  `json:"outbound_usage_value"`
	// NetInventoryBuild is PurchasesValue - OutboundUsageValue.
	NetInventoryBuild Value `json:"net_inventory_build"`
}

func buildPurchaseVsUsageTrend(period string, purchases PurchaseSummary, usage UsageSummary) PurchaseVsUsageTrend {
	t := PurchaseVsUsageTrend{Period: period, PurchasesValue: purchases.PurchaseValue, OutboundUsageValue: usage.OutboundValue}
	if purchases.PurchaseValue.Available && usage.OutboundValue.Available {
		t.Available = true
		t.NetInventoryBuild = AvailableValue(purchases.PurchaseValue.Amount - usage.OutboundValue.Amount)
	}
	return t
}
