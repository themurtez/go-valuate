package inventory

// flagInputs bundles everything computeFlags needs, kept as one struct so
// the function signature stays readable — mirrors ap.flagInputs's
// identical convention.
type flagInputs struct {
	states    map[string]*itemState
	itemOrder []string

	aging              AgingSummary
	stockPolicyResults []StockPolicyResult
	periods            []PeriodSummary
	reconciliation     ReconciliationSummary
	expiry             ExpirySummary
	possibleDuplicates []PossibleDuplicateMovement

	adjustmentCountsByItem map[string]int

	policy Policy
}

// computeFlags evaluates every FlagCode rule and returns triggered flags
// sorted by FlagCode declaration order, then ItemID, then Period — task
// section 70.
func computeFlags(in flagInputs) []Flag {
	var flags []Flag
	p := in.policy

	for _, id := range in.itemOrder {
		st, ok := in.states[id]
		if !ok {
			continue
		}
		if st.quantity.Available && st.quantity.Amount < 0 {
			flags = append(flags, Flag{Code: FlagNegativeInventory, ItemID: id, Value: st.quantity.Amount,
				Message: "item has negative quantity on hand"})
		} else if st.value.Available && st.value.Amount < 0 {
			flags = append(flags, Flag{Code: FlagNegativeInventory, ItemID: id, Value: st.value.Amount,
				Message: "item has negative inventory value"})
		}
	}

	for _, r := range in.stockPolicyResults {
		if r.AboveMaximum.Available && r.AboveMaximum.Value {
			flags = append(flags, Flag{Code: FlagAboveCallerMaximum, ItemID: r.ItemID, Value: r.QuantityOnHand.Amount, Threshold: r.Policy.MaximumQuantity,
				Message: "item quantity on hand exceeds caller-supplied maximum"})
		}
		if r.BelowMinimum.Available && r.BelowMinimum.Value {
			flags = append(flags, Flag{Code: FlagBelowCallerMinimum, ItemID: r.ItemID, Value: r.QuantityOnHand.Amount, Threshold: r.Policy.MinimumQuantity,
				Message: "item quantity on hand is below caller-supplied minimum"})
		}
	}

	if in.aging.Available {
		if p.SlowMovingDays > 0 {
			for _, row := range in.aging.Rows {
				days, hasDays := 0.0, row.AgeDays.Available
				if hasDays {
					days = row.AgeDays.Amount
				}
				if row.Evidence == AgeEvidenceUnknown {
					flags = append(flags, Flag{Code: FlagUnknownInventoryAge, ItemID: row.ItemID,
						Message: "item lacks sufficient evidence to determine inventory age"})
					continue
				}
				if hasDays && days >= float64(p.SlowMovingDays) {
					flags = append(flags, Flag{Code: FlagAgedInventoryReview, ItemID: row.ItemID, Value: days, Threshold: float64(p.SlowMovingDays),
						Message: "item inventory age exceeds review threshold"})
				}
			}
		} else {
			for _, row := range in.aging.Rows {
				if row.Evidence == AgeEvidenceUnknown {
					flags = append(flags, Flag{Code: FlagUnknownInventoryAge, ItemID: row.ItemID,
						Message: "item lacks sufficient evidence to determine inventory age"})
				}
			}
		}
	}

	if in.aging.SlowMovingValue.Available && in.aging.SlowMovingValue.Amount > 0 {
		flags = append(flags, Flag{Code: FlagSlowMovingInventory, Value: in.aging.SlowMovingValue.Amount, Threshold: float64(p.SlowMovingDays),
			Message: "portfolio holds slow-moving inventory value"})
	}
	if in.aging.NonMovingValue.Available && in.aging.NonMovingValue.Amount > 0 {
		flags = append(flags, Flag{Code: FlagNonMovingInventory, Value: in.aging.NonMovingValue.Amount, Threshold: float64(p.NonMovingDays),
			Message: "portfolio holds non-moving inventory value"})
	}

	for _, ps := range in.periods {
		if ps.PurchaseVsUsage.Available && ps.PurchaseVsUsage.OutboundUsageValue.Amount > 0 {
			gap := (ps.PurchaseVsUsage.PurchasesValue.Amount - ps.PurchaseVsUsage.OutboundUsageValue.Amount) / ps.PurchaseVsUsage.OutboundUsageValue.Amount
			if gap > p.PurchaseVsUsageThreshold {
				flags = append(flags, Flag{Code: FlagPurchasesOutpaceUsage, Period: ps.Period.Period, Value: gap, Threshold: p.PurchaseVsUsageThreshold,
					Message: "period purchases outpace outbound usage value beyond threshold"})
			}
		}
		if ps.Adjustments.AdjustmentRate.Available && ps.Adjustments.AdjustmentRate.Amount > p.AdjustmentRateThreshold {
			flags = append(flags, Flag{Code: FlagHighInventoryAdjustmentRate, Period: ps.Period.Period, Value: ps.Adjustments.AdjustmentRate.Amount, Threshold: p.AdjustmentRateThreshold,
				Message: "period inventory adjustment rate exceeds threshold"})
		}
		if ps.Adjustments.WriteOffValue.Available && p.LargeWriteOffThreshold > 0 && ps.Adjustments.WriteOffValue.Amount > p.LargeWriteOffThreshold {
			flags = append(flags, Flag{Code: FlagLargeWriteOff, Period: ps.Period.Period, Value: ps.Adjustments.WriteOffValue.Amount, Threshold: p.LargeWriteOffThreshold,
				Message: "period write-off value exceeds threshold"})
		}
	}

	if p.RepeatedItemAdjustmentCount > 0 {
		items := make([]string, 0, len(in.adjustmentCountsByItem))
		for id := range in.adjustmentCountsByItem {
			items = append(items, id)
		}
		sortStrings(items)
		for _, id := range items {
			count := in.adjustmentCountsByItem[id]
			if count >= p.RepeatedItemAdjustmentCount {
				flags = append(flags, Flag{Code: FlagRepeatedItemAdjustments, ItemID: id, Value: float64(count), Threshold: float64(p.RepeatedItemAdjustmentCount),
					Message: "item has repeated adjustment/write-off movements"})
			}
		}
	}

	flags = append(flags, computePeriodEndAdjustmentFlags(in.periods)...)

	if inventoryBuildGap, ok := inventoryBuildGrowthGap(in.periods); ok && inventoryBuildGap > p.InventoryBuildGrowthGap {
		flags = append(flags, Flag{Code: FlagInventoryBuildWithoutMatchingCOGSGrowth, Value: inventoryBuildGap, Threshold: p.InventoryBuildGrowthGap,
			Message: "inventory value growth outpaces COGS growth beyond threshold"})
	}

	if in.reconciliation.Available && !in.reconciliation.AllReconciled {
		for _, c := range in.reconciliation.Components {
			if !c.Reconciled {
				flags = append(flags, Flag{Code: FlagInventoryGLMismatch, Category: c.Component, Value: c.Difference, Threshold: c.Tolerance,
					Message: "subledger inventory value does not match supplied GL control balance"})
			}
		}
	}

	if in.expiry.Available {
		for _, row := range in.expiry.Rows {
			switch row.Status {
			case ExpiryStatusExpiring:
				val := 0.0
				if row.Value.Available {
					val = row.Value.Amount
				}
				flags = append(flags, Flag{Code: FlagExpiringInventory, ItemID: row.ItemID, Value: val,
					Message: "lot is nearing its expiry date"})
			case ExpiryStatusExpired:
				val := 0.0
				if row.Value.Available {
					val = row.Value.Amount
				}
				flags = append(flags, Flag{Code: FlagExpiredInventoryReview, ItemID: row.ItemID, Value: val,
					Message: "lot has passed its expiry date and warrants review"})
			}
		}
	}

	for _, d := range in.possibleDuplicates {
		flags = append(flags, Flag{Code: FlagPossibleDuplicateMovement, ItemID: d.ItemID, MovementID: d.MovementIDA,
			Message: "movement " + d.MovementIDA + " closely resembles movement " + d.MovementIDB})
	}

	sortFlags(flags)
	return flags
}

// computePeriodEndAdjustmentFlags flags a period whose AdjustmentSummary
// found at least one adjustment/write-off movement dated within the
// caller's configured window of that period's own EndDate — task section
// 35's "period-end inventory adjustment" neutral timing observation.
// AdjustmentSummary.PeriodEndAdjustmentCount already performs the actual
// date-proximity check (see buildAdjustmentSummary); this only decides
// whether to flag based on that already-computed count.
func computePeriodEndAdjustmentFlags(periods []PeriodSummary) []Flag {
	var flags []Flag
	for _, ps := range periods {
		if !ps.Adjustments.Available || ps.Adjustments.PeriodEndAdjustmentCount == 0 {
			continue
		}
		flags = append(flags, Flag{Code: FlagPeriodEndInventoryAdjustment, Period: ps.Period.Period, Value: float64(ps.Adjustments.PeriodEndAdjustmentCount),
			Message: "period contains inventory adjustment activity dated near period end; timing review recommended"})
	}
	return flags
}

// inventoryBuildGrowthGap computes (InventoryGrowthPercent -
// COGSGrowthPercent) first-vs-last across periods with an available
// ending inventory value and COGS — task section 33.
func inventoryBuildGrowthGap(periods []PeriodSummary) (float64, bool) {
	var withBoth []PeriodSummary
	for _, ps := range periods {
		if ps.DIO.AverageInventory.Available && ps.DIO.COGS.Available {
			withBoth = append(withBoth, ps)
		}
	}
	if len(withBoth) < 2 {
		return 0, false
	}
	first, last := withBoth[0], withBoth[len(withBoth)-1]
	if first.DIO.AverageInventory.Amount == 0 || first.DIO.COGS.Amount == 0 {
		return 0, false
	}
	invGrowth := (last.DIO.AverageInventory.Amount - first.DIO.AverageInventory.Amount) / first.DIO.AverageInventory.Amount
	cogsGrowth := (last.DIO.COGS.Amount - first.DIO.COGS.Amount) / first.DIO.COGS.Amount
	return invGrowth - cogsGrowth, true
}
