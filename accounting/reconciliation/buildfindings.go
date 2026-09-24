package reconciliation

// buildBalanceFindings produces FindingBalanceMismatch when the
// reconciliation equation is available and its Difference exceeds
// tolerance.
func buildBalanceFindings(eq EquationResult, tolerance float64) []Finding {
	if !eq.Available || eq.WithinTolerance {
		return nil
	}
	return []Finding{{
		Code:     FindingBalanceMismatch,
		Severity: SeverityWarning,
		Message:  "adjusted book and external balances do not tie within tolerance",
		Amount:   eq.Difference,
	}}
}

// buildRollforwardFindings produces FindingBookRollforwardMismatch /
// FindingExternalRollforwardMismatch independently of cross-side
// reconciliation — task section 45: "Return side-specific mismatch
// independently of cross-side reconciliation."
func buildRollforwardFindings(bookBalance, externalBalance Balance) []Finding {
	var findings []Finding
	if bookBalance.RollforwardMismatch {
		findings = append(findings, Finding{
			Code:     FindingBookRollforwardMismatch,
			Severity: SeverityWarning,
			Message:  "book side: opening balance + activity does not equal the ending balance within tolerance",
			Amount:   bookBalance.RollforwardDifference,
		})
	}
	if externalBalance.RollforwardMismatch {
		findings = append(findings, Finding{
			Code:     FindingExternalRollforwardMismatch,
			Severity: SeverityWarning,
			Message:  "external side: opening balance + activity does not equal the ending balance within tolerance",
			Amount:   externalBalance.RollforwardDifference,
		})
	}
	return findings
}

// buildMaterialUnmatchedFindings produces FindingMaterialUnmatchedBookItem/
// FindingMaterialUnmatchedExternalItem for unmatched items whose amount
// is material per policy.Materiality, or for EVERY unmatched item
// (material or not) when policy.UnmatchedPolicy is RequireAllItemsMatched
// — task section 29. Immaterial unmatched items under
// AllowImmaterialUnmatched still appear in Result.UnmatchedBookItems/
// UnmatchedExternalItems (never silently discarded) but produce no
// Finding and do not affect Status.
//
// An unmatched item already explained by an explicit ReconcilingItem
// (via ReconcilingItem.RelatedItemID) is excluded from this Finding
// entirely, regardless of RequireAllItemsMatched — the caller has
// already told this package what the item is (an outstanding check, a
// deposit in transit, etc.), so flagging it again as an unresolved,
// unexplained gap would contradict the explanation the caller just gave.
// It remains listed in UnmatchedBookItems/UnmatchedExternalItems (task
// section 9: "never drop unmatched rows") and its dollar effect is still
// counted via ReconcilingEffects/the reconciliation equation — only the
// redundant Finding is suppressed.
func buildMaterialUnmatchedFindings(
	book []BookItem, external []ExternalItem,
	unmatchedBookIDs, unmatchedExternalIDs []string,
	bookByID map[string]BookItem, externalByID map[string]ExternalItem,
	explainedItemIDs map[string]bool,
	policy MatchingPolicy,
) []Finding {
	var findings []Finding
	requireAll := policy.unmatchedPolicy() == RequireAllItemsMatched

	for _, id := range unmatchedBookIDs {
		if explainedItemIDs[id] {
			continue
		}
		item := bookByID[id]
		amt := item.SignedAmount()
		if requireAll || isMaterial(amt, nil, policy.Materiality) {
			findings = append(findings, Finding{
				Code:     FindingMaterialUnmatchedBookItem,
				Severity: SeverityWarning,
				Message:  "unmatched book item is unresolved under the configured unmatched-item policy",
				ItemIDs:  []string{id},
				Amount:   amt,
			})
		}
	}
	for _, id := range unmatchedExternalIDs {
		if explainedItemIDs[id] {
			continue
		}
		item := externalByID[id]
		amt := item.OrientedSignedAmount(policy.orientation())
		if requireAll || isMaterial(amt, nil, policy.Materiality) {
			findings = append(findings, Finding{
				Code:     FindingMaterialUnmatchedExternalItem,
				Severity: SeverityWarning,
				Message:  "unmatched external item is unresolved under the configured unmatched-item policy",
				ItemIDs:  []string{id},
				Amount:   amt,
			})
		}
	}
	return findings
}

// buildAmbiguousFindings produces one FindingAmbiguousMatch per distinct
// ambiguous anchor item found across matching/composite stages —
// grouping candidate evidence by anchor so a book item with 3 tied
// candidates yields 1 Finding (with 3 ItemIDs = anchor + every tied
// counterpart), not 3 separate findings.
func buildAmbiguousFindings(candidates []Candidate) []Finding {
	if len(candidates) == 0 {
		return nil
	}
	type key struct {
		bookID     string
		externalID string
	}
	// Group by the single-item anchor (whichever of BookItemID/
	// ExternalItemID is populated for this candidate — composite
	// candidates anchor on the single-ID side).
	byAnchor := make(map[string][]Candidate)
	var anchorOrder []string
	for _, c := range candidates {
		anchor := c.BookItemID
		if anchor == "" {
			anchor = c.ExternalItemID
		}
		if _, seen := byAnchor[anchor]; !seen {
			anchorOrder = append(anchorOrder, anchor)
		}
		byAnchor[anchor] = append(byAnchor[anchor], c)
	}
	sortByID(anchorOrder)

	var findings []Finding
	for _, anchor := range anchorOrder {
		group := byAnchor[anchor]
		itemIDs := []string{anchor}
		for _, c := range group {
			for _, id := range c.ExternalItemIDs {
				itemIDs = append(itemIDs, id)
			}
			if c.ExternalItemID != "" {
				itemIDs = append(itemIDs, c.ExternalItemID)
			}
			for _, id := range c.BookItemIDs {
				itemIDs = append(itemIDs, id)
			}
			if c.BookItemID != "" && c.BookItemID != anchor {
				itemIDs = append(itemIDs, c.BookItemID)
			}
		}
		findings = append(findings, Finding{
			Code:     FindingAmbiguousMatch,
			Severity: SeverityWarning,
			Message:  "item has multiple equally valid match candidates and was left unresolved rather than arbitrarily paired",
			ItemIDs:  itemIDs,
		})
	}
	return findings
}

// buildMaterialReconcilingItemsFinding produces
// FindingMaterialReconcilingItems when the total (gross absolute) amount
// of ReconcilingItems is material — task section 26/29's spirit applied
// to reconciling items themselves, so a caller sees when reconciling
// items are collectively large even though Status may still reach
// RECONCILED_WITH_ITEMS.
func buildMaterialReconcilingItemsFinding(eq EquationResult, policy MatchingPolicy) []Finding {
	total := eq.BookReconcilingEffects.Total + eq.ExternalReconcilingEffects.Total
	count := eq.BookReconcilingEffects.Count + eq.ExternalReconcilingEffects.Count
	if count == 0 {
		return nil
	}
	var balance *float64
	if eq.Available {
		b := eq.BookEndingBalance
		balance = &b
	}
	if !isMaterial(total, balance, policy.Materiality) {
		return nil
	}
	return []Finding{{
		Code:     FindingMaterialReconcilingItems,
		Severity: SeverityInfo,
		Message:  "total reconciling items are material relative to the reconciled balance",
		Amount:   total,
	}}
}
