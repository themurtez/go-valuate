package inventory

import "sort"

// validateItems checks each Item for structural rules, returning issues
// plus a map of every item ID that resolved successfully (first
// occurrence wins on duplicate — task section 3/64: "duplicate item IDs").
// An item with no ID at all is dropped entirely (nothing to key it by);
// a duplicate keeps its first occurrence's data.
func validateItems(items []Item) ([]Issue, map[string]Item, []string) {
	var issues []Issue
	byID := map[string]Item{}
	var order []string

	for _, it := range items {
		if it.ID == "" {
			issues = append(issues, Issue{Code: IssueUnknownItem, Severity: SeverityError, Message: "item missing ID"})
			continue
		}
		if _, exists := byID[it.ID]; exists {
			issues = append(issues, Issue{Code: IssueDuplicateItem, Severity: SeverityError, Message: "duplicate item ID: " + it.ID, ItemID: it.ID})
			continue
		}
		byID[it.ID] = it
		order = append(order, it.ID)
	}
	return issues, byID, order
}

// validateSnapshots checks each InventorySnapshot for structural rules.
// Returns issues plus the set of valid (excluded == false) snapshots in
// input order. A snapshot is excluded when its identity is a duplicate
// (task section 52), its ItemID is unknown, or it has a non-finite/
// negative-where-invalid quantity or cost.
func validateSnapshots(snapshots []InventorySnapshot, knownItems map[string]Item, negativeHandling NegativeInventoryHandling) ([]Issue, []InventorySnapshot) {
	var issues []Issue
	seenIdentity := map[snapshotIdentityKey]bool{}
	seenID := map[string]bool{}
	var valid []InventorySnapshot

	for _, s := range snapshots {
		excluded := false

		if s.ID == "" {
			issues = append(issues, Issue{Code: IssueDuplicateSnapshot, Severity: SeverityError, Message: "snapshot missing ID", ItemID: s.ItemID})
			excluded = true
		} else if seenID[s.ID] {
			issues = append(issues, Issue{Code: IssueDuplicateSnapshot, Severity: SeverityError, Message: "duplicate snapshot ID: " + s.ID, SnapshotID: s.ID, ItemID: s.ItemID})
			excluded = true
		} else {
			seenID[s.ID] = true
		}

		if s.ItemID == "" || (len(knownItems) > 0 && !hasItem(knownItems, s.ItemID)) {
			issues = append(issues, Issue{Code: IssueUnknownItem, Severity: SeverityError, Message: "snapshot references unknown item ID", SnapshotID: s.ID, ItemID: s.ItemID})
			excluded = true
		}

		if s.AsOfDate.IsZero() {
			issues = append(issues, Issue{Code: IssueInvalidAsOfDate, Severity: SeverityError, Message: "snapshot missing AsOfDate", SnapshotID: s.ID, ItemID: s.ItemID})
			excluded = true
		}

		if isNonFinite(s.QuantityOnHand.Amount) || isNonFinite(s.UnitCost.Amount) || isNonFinite(s.InventoryValue.Amount) {
			issues = append(issues, Issue{Code: IssueNonFiniteQuantity, Severity: SeverityError, Message: "snapshot has non-finite quantity/cost/value", SnapshotID: s.ID, ItemID: s.ItemID})
			excluded = true
		} else {
			if s.QuantityOnHand.Available && s.QuantityOnHand.Amount < 0 {
				if resolvedNegativeInventoryHandling(negativeHandling) == NegativeInventoryReject {
					issues = append(issues, Issue{Code: IssueInvalidQuantity, Severity: SeverityError, Message: "negative quantity on hand rejected by policy", SnapshotID: s.ID, ItemID: s.ItemID})
					excluded = true
				}
				// else: allowed through with a warning-level Flag —
				// see flags.go's FlagNegativeInventory, computed later
				// from included snapshots so it is never silently
				// swallowed here.
			}
			if s.UnitCost.Available && s.UnitCost.Amount < 0 {
				issues = append(issues, Issue{Code: IssueInvalidCost, Severity: SeverityError, Message: "negative unit cost", SnapshotID: s.ID, ItemID: s.ItemID})
				excluded = true
			}
			if s.InventoryValue.Available && s.InventoryValue.Amount < 0 && resolvedNegativeInventoryHandling(negativeHandling) == NegativeInventoryReject {
				issues = append(issues, Issue{Code: IssueInvalidQuantity, Severity: SeverityError, Message: "negative inventory value rejected by policy", SnapshotID: s.ID, ItemID: s.ItemID})
				excluded = true
			}
		}

		if s.ExpiryDate != nil && s.ReceivedDate != nil && s.ExpiryDate.Before(*s.ReceivedDate) {
			issues = append(issues, Issue{Code: IssueInvalidExpiryDate, Severity: SeverityError, Message: "expiry date is before received date", SnapshotID: s.ID, ItemID: s.ItemID})
			excluded = true
		}

		if excluded {
			continue
		}

		identity := snapshotIdentity(s)
		if seenIdentity[identity] {
			issues = append(issues, Issue{Code: IssueDuplicateSnapshot, Severity: SeverityError,
				Message: "conflicting snapshot for the same item/location/lot/as-of-date", SnapshotID: s.ID, ItemID: s.ItemID})
			continue
		}
		seenIdentity[identity] = true

		// resolveSnapshotValue's value/quantity-vs-cost mismatch check
		// (task section 8) is surfaced here, once, during validation —
		// every downstream consumer of resolveSnapshotValue (aging.go,
		// portfolio.go, concentration.go, coverage.go, expiry.go) simply
		// uses the resolved value and does not re-report this issue.
		if _, mismatchIssue := resolveSnapshotValue(s); mismatchIssue != nil {
			issues = append(issues, *mismatchIssue)
		}

		valid = append(valid, s)
	}
	return issues, valid
}

// validateMovements checks each Movement for structural rules. Returns
// issues plus the set of valid movements in input order. Exact duplicate
// IDs are excluded (first occurrence wins); an unrecognized MovementType,
// unknown ItemID, non-finite/negative quantity, or zero-value Date also
// excludes the row. Movement.Quantity is always a non-negative magnitude
// (direction comes from Type — see MovementDirection), so
// NegativeInventoryHandling never applies here; it governs only on-hand
// balances (InventorySnapshot), not movement facts.
func validateMovements(movements []Movement, knownItems map[string]Item) ([]Issue, []Movement) {
	var issues []Issue
	seenID := map[string]bool{}
	var valid []Movement

	for _, m := range movements {
		excluded := false

		if m.ID == "" {
			issues = append(issues, Issue{Code: IssueDuplicateMovement, Severity: SeverityError, Message: "movement missing ID", ItemID: m.ItemID})
			excluded = true
		} else if seenID[m.ID] {
			issues = append(issues, Issue{Code: IssueDuplicateMovement, Severity: SeverityError, Message: "duplicate movement ID: " + m.ID, MovementID: m.ID, ItemID: m.ItemID})
			excluded = true
		} else {
			seenID[m.ID] = true
		}

		if m.ItemID == "" || (len(knownItems) > 0 && !hasItem(knownItems, m.ItemID)) {
			issues = append(issues, Issue{Code: IssueUnknownItem, Severity: SeverityError, Message: "movement references unknown item ID", MovementID: m.ID, ItemID: m.ItemID})
			excluded = true
		}

		if !isRecognizedMovementType(m.Type) {
			issues = append(issues, Issue{Code: IssueInvalidMovement, Severity: SeverityError, Message: "unrecognized movement type", MovementID: m.ID, ItemID: m.ItemID})
			excluded = true
		}

		if m.Date.IsZero() {
			issues = append(issues, Issue{Code: IssueInvalidMovement, Severity: SeverityError, Message: "movement missing date", MovementID: m.ID, ItemID: m.ItemID})
			excluded = true
		}

		if isNonFinite(m.Quantity.Amount) || isNonFinite(m.UnitCost.Amount) || isNonFinite(m.Amount.Amount) {
			issues = append(issues, Issue{Code: IssueNonFiniteQuantity, Severity: SeverityError, Message: "movement has non-finite quantity/cost/amount", MovementID: m.ID, ItemID: m.ItemID})
			excluded = true
		} else {
			if m.Quantity.Available && m.Quantity.Amount < 0 {
				issues = append(issues, Issue{Code: IssueInvalidQuantity, Severity: SeverityError, Message: "movement quantity must be a non-negative magnitude; direction comes from type", MovementID: m.ID, ItemID: m.ItemID})
				excluded = true
			}
			if m.UnitCost.Available && m.UnitCost.Amount < 0 {
				issues = append(issues, Issue{Code: IssueInvalidCost, Severity: SeverityError, Message: "negative unit cost", MovementID: m.ID, ItemID: m.ItemID})
				excluded = true
			}
		}

		if excluded {
			continue
		}

		// resolveMovementValue's amount-vs-quantity-x-cost mismatch check
		// (task section 8's identical rule applied to movements) is
		// surfaced here, once, during validation — mirrors the identical
		// snapshot-side check above.
		if _, mismatchIssue := resolveMovementValue(m); mismatchIssue != nil {
			issues = append(issues, *mismatchIssue)
		}

		valid = append(valid, m)
	}
	return issues, valid
}

func hasItem(items map[string]Item, id string) bool {
	_, ok := items[id]
	return ok
}

// validateStockPolicies checks each StockPolicy for structural
// consistency: a known item reference and Min <= Target <= Max when all
// three are set.
func validateStockPolicies(policies []StockPolicy, knownItems map[string]Item) []Issue {
	var issues []Issue
	seen := map[string]bool{}
	for _, p := range policies {
		if p.ItemID == "" {
			issues = append(issues, Issue{Code: IssueInvalidStockPolicy, Severity: SeverityError, Message: "stock policy missing item ID"})
			continue
		}
		if seen[p.ItemID] {
			issues = append(issues, Issue{Code: IssueInvalidStockPolicy, Severity: SeverityError, Message: "duplicate stock policy for item", ItemID: p.ItemID})
			continue
		}
		seen[p.ItemID] = true
		if len(knownItems) > 0 && !hasItem(knownItems, p.ItemID) {
			issues = append(issues, Issue{Code: IssueUnknownItem, Severity: SeverityError, Message: "stock policy references unknown item ID", ItemID: p.ItemID})
			continue
		}
		if p.MinimumQuantitySet && p.MaximumQuantitySet && p.MinimumQuantity > p.MaximumQuantity {
			issues = append(issues, Issue{Code: IssueInvalidStockPolicy, Severity: SeverityWarning, Message: "stock policy minimum exceeds maximum", ItemID: p.ItemID})
		}
		if p.TargetQuantitySet && p.MinimumQuantitySet && p.TargetQuantity < p.MinimumQuantity {
			issues = append(issues, Issue{Code: IssueInvalidStockPolicy, Severity: SeverityWarning, Message: "stock policy target is below minimum", ItemID: p.ItemID})
		}
		if p.TargetQuantitySet && p.MaximumQuantitySet && p.TargetQuantity > p.MaximumQuantity {
			issues = append(issues, Issue{Code: IssueInvalidStockPolicy, Severity: SeverityWarning, Message: "stock policy target exceeds maximum", ItemID: p.ItemID})
		}
		checkFinite := func(v float64, set bool, label string) {
			if set && isNonFinite(v) {
				issues = append(issues, Issue{Code: IssueInvalidStockPolicy, Severity: SeverityError, Message: "stock policy " + label + " is non-finite", ItemID: p.ItemID})
			}
		}
		checkFinite(p.MinimumQuantity, p.MinimumQuantitySet, "minimum_quantity")
		checkFinite(p.TargetQuantity, p.TargetQuantitySet, "target_quantity")
		checkFinite(p.MaximumQuantity, p.MaximumQuantitySet, "maximum_quantity")
		checkFinite(p.ReorderPoint, p.ReorderPointSet, "reorder_point")
	}
	return issues
}

// validatePeriods checks each PeriodInfo for structural rules: non-zero
// StartDate/EndDate, EndDate not before StartDate. Returns issues plus
// the chronologically sorted, valid periods.
func validatePeriods(periods []PeriodInfo) ([]Issue, []PeriodInfo) {
	var issues []Issue
	var valid []PeriodInfo
	seen := map[string]bool{}
	for _, p := range periods {
		if p.StartDate.IsZero() || p.EndDate.IsZero() {
			issues = append(issues, Issue{Code: IssueInvalidPeriod, Severity: SeverityError, Message: "period missing start/end date", Period: p.Period})
			continue
		}
		if p.EndDate.Before(p.StartDate) {
			issues = append(issues, Issue{Code: IssueInvalidPeriod, Severity: SeverityError, Message: "period end date is before start date", Period: p.Period})
			continue
		}
		if p.Period != "" {
			if seen[p.Period] {
				issues = append(issues, Issue{Code: IssueInvalidPeriod, Severity: SeverityError, Message: "duplicate period label: " + p.Period, Period: p.Period})
				continue
			}
			seen[p.Period] = true
		}
		valid = append(valid, p)
	}
	return issues, sortedPeriods(valid)
}

// validateFinancials checks each PeriodFinancials for non-finite values.
func validateFinancials(metrics []PeriodFinancials) []Issue {
	var issues []Issue
	for _, m := range metrics {
		checkFinite := func(v Value, label string) {
			if v.Available && isNonFinite(v.Amount) {
				issues = append(issues, Issue{Code: IssueInvalidFinancialMetric, Severity: SeverityError, Message: "financial metric " + label + " is non-finite", Period: m.Period})
			}
		}
		checkFinite(m.BeginningInventoryValue, "beginning_inventory_value")
		checkFinite(m.EndingInventoryValue, "ending_inventory_value")
		checkFinite(m.Revenue, "revenue")
		checkFinite(m.COGS, "cogs")
		checkFinite(m.PurchaseValue, "purchase_value")
		checkFinite(m.PurchaseUnits, "purchase_units")
		checkFinite(m.UnitsSold, "units_sold")
		if m.COGS.Available && m.COGS.Amount < 0 {
			issues = append(issues, Issue{Code: IssueInvalidFinancialMetric, Severity: SeverityWarning, Message: "cogs is negative", Period: m.Period})
		}
	}
	return issues
}

// validateGLControls checks each GLControl for non-finite balances.
func validateGLControls(controls []GLControl) []Issue {
	var issues []Issue
	for _, c := range controls {
		if isNonFinite(c.Balance) {
			issues = append(issues, Issue{Code: IssueInvalidGLControl, Severity: SeverityError, Message: "GL control balance is non-finite", Period: c.Period})
		}
	}
	return issues
}

// validateMarketValues checks each MarketValue for non-finite components.
func validateMarketValues(values []MarketValue) []Issue {
	var issues []Issue
	for _, v := range values {
		checkFinite := func(val Value, label string) {
			if val.Available && isNonFinite(val.Amount) {
				issues = append(issues, Issue{Code: IssueInvalidFinancialMetric, Severity: SeverityError, Message: "market value " + label + " is non-finite", ItemID: v.ItemID})
			}
		}
		checkFinite(v.NetRealizableValue, "net_realizable_value")
		checkFinite(v.ExpectedSellingPrice, "expected_selling_price")
		checkFinite(v.DisposalCosts, "disposal_costs")
	}
	return issues
}

// resolveReportingCurrency determines the single currency an analysis
// proceeds under: explicit caller choice if supplied, otherwise the most
// common currency among included items (ties broken by currency code
// ascending). Returns ("", nil) only if there are no items with a
// currency at all. Mirrors ap.resolveReportingCurrency's identical
// resolution rule.
func resolveReportingCurrency(items []Item, explicit string) (string, []Issue) {
	if explicit != "" {
		return explicit, checkMixedItemCurrency(items, explicit)
	}
	counts := map[string]int{}
	for _, it := range items {
		if it.Currency == "" {
			continue
		}
		counts[it.Currency]++
	}
	if len(counts) == 0 {
		return "", nil
	}
	codes := make([]string, 0, len(counts))
	for c := range counts {
		codes = append(codes, c)
	}
	sort.Strings(codes)
	best := codes[0]
	for _, c := range codes[1:] {
		if counts[c] > counts[best] {
			best = c
		}
	}
	return best, checkMixedItemCurrency(items, best)
}

// filterItemsByCurrency drops every item whose Currency is set and
// differs from reporting from both itemsByID and itemOrder, preserving
// itemOrder's relative order. An item with no stated Currency ("") is
// always kept — this package has no basis to conclude it conflicts.
func filterItemsByCurrency(itemsByID map[string]Item, itemOrder []string, reporting string) (map[string]Item, []string) {
	if reporting == "" {
		return itemsByID, itemOrder
	}
	filteredByID := make(map[string]Item, len(itemsByID))
	filteredOrder := make([]string, 0, len(itemOrder))
	for _, id := range itemOrder {
		it := itemsByID[id]
		if it.Currency != "" && it.Currency != reporting {
			continue
		}
		filteredByID[id] = it
		filteredOrder = append(filteredOrder, id)
	}
	return filteredByID, filteredOrder
}

func checkMixedItemCurrency(items []Item, reporting string) []Issue {
	for _, it := range items {
		if it.Currency == "" {
			continue
		}
		if it.Currency != reporting {
			return []Issue{{Code: IssueMixedCurrency, Severity: SeverityWarning, Message: "items use more than one currency; only " + reporting + " included in aggregate totals"}}
		}
	}
	return nil
}
