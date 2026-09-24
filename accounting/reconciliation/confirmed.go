package reconciliation

import "strconv"

// ConfirmedMatch is a caller-supplied, already-decided match between one
// or more BookItems and one or more ExternalItems — the highest-precedence
// input this package accepts (task section 12: "Caller-supplied confirmed
// matches take highest precedence... Never override a valid confirmed
// match with auto-matching"). Many-to-many is only ever reachable through
// a ConfirmedMatch — this package never automatically proposes a
// many-to-many group (task section 11/20).
type ConfirmedMatch struct {
	// MatchID, if supplied, is echoed on the resulting MatchGroup.MatchID;
	// if empty, this package assigns one deterministically (see sort.go).
	MatchID string `json:"match_id,omitempty"`

	BookItemIDs     []string `json:"book_item_ids"`
	ExternalItemIDs []string `json:"external_item_ids"`

	// ExplainedDifference, if non-nil, is the caller's accepted, already-
	// explained amount difference for this group — when set, the group's
	// BookAmount/ExternalAmount are allowed to disagree by up to this
	// amount without being treated as an invalid confirmed match (task
	// section 12: "amounts tie within policy tolerance unless explicitly
	// marked as an explained difference"). When nil, ordinary
	// MatchingPolicy tolerance applies.
	ExplainedDifference *float64 `json:"explained_difference,omitempty"`

	Note string `json:"note,omitempty"`
}

// confirmedValidation is the outcome of validating one ConfirmedMatch
// against the item universe and every other ConfirmedMatch.
type confirmedValidation struct {
	valid bool
	// usedBookItems/usedExternalItems are the subset of the match's
	// declared IDs that actually resolved to a real item — used so a
	// partially-invalid match's still-valid items are not silently
	// double-counted elsewhere; a ConfirmedMatch is applied only when
	// valid is true (fully valid), matching task section 12's "Validate:
	// IDs exist... Never override a valid confirmed match" — a match
	// with ANY unresolved ID or reuse conflict is rejected in full, not
	// partially applied, since a partial application could silently
	// misstate the group's own arithmetic.
}

// validateConfirmedMatches checks every ConfirmedMatch for: referenced
// IDs exist in bookByID/externalByID; no book or external item ID appears
// in more than one confirmed match; currency compatibility across the
// group's own items; and (unless ExplainedDifference is set) amount
// agreement within tolerance. Returns the subset of confirmedMatches that
// passed every check (in input order) plus every Issue found. Confirmed
// matches are never partially applied — task section 12's "Validate...
// Never override a valid confirmed match with auto-matching" is read as:
// a match failing any check is entirely rejected (falls through to
// ordinary auto-matching for its items, if any of those items are
// otherwise unambiguous), never partially honored.
func validateConfirmedMatches(
	confirmedMatches []ConfirmedMatch,
	bookByID map[string]BookItem,
	externalByID map[string]ExternalItem,
	policy MatchingPolicy,
) (accepted []ConfirmedMatch, issues []Issue) {
	usedBook := make(map[string]int)     // itemID -> count of matches referencing it
	usedExternal := make(map[string]int) // itemID -> count of matches referencing it

	for _, m := range confirmedMatches {
		for _, id := range m.BookItemIDs {
			usedBook[id]++
		}
		for _, id := range m.ExternalItemIDs {
			usedExternal[id]++
		}
	}

	for i, m := range confirmedMatches {
		ref := m.MatchID
		if ref == "" {
			ref = confirmedMatchFallbackRef(i)
		}

		ok := true
		if len(m.BookItemIDs) == 0 && len(m.ExternalItemIDs) == 0 {
			issues = append(issues, Issue{
				Code: IssueInvalidConfirmedMatch, Severity: IssueSeverityError,
				Message: "confirmed match has no book or external item IDs", Ref: ref,
			})
			ok = false
		}

		var bookItems []BookItem
		var externalItems []ExternalItem

		for _, id := range m.BookItemIDs {
			item, found := bookByID[id]
			if !found {
				issues = append(issues, Issue{
					Code: IssueUnknownMatchItem, Severity: IssueSeverityError,
					Message: "confirmed match references unknown book item: " + id, Ref: ref, ItemID: id,
				})
				ok = false
				continue
			}
			if usedBook[id] > 1 {
				issues = append(issues, Issue{
					Code: IssueItemUsedInMultipleMatches, Severity: IssueSeverityError,
					Message: "book item referenced by more than one confirmed match: " + id, Ref: ref, ItemID: id,
				})
				ok = false
				continue
			}
			bookItems = append(bookItems, item)
		}
		for _, id := range m.ExternalItemIDs {
			item, found := externalByID[id]
			if !found {
				issues = append(issues, Issue{
					Code: IssueUnknownMatchItem, Severity: IssueSeverityError,
					Message: "confirmed match references unknown external item: " + id, Ref: ref, ItemID: id,
				})
				ok = false
				continue
			}
			if usedExternal[id] > 1 {
				issues = append(issues, Issue{
					Code: IssueItemUsedInMultipleMatches, Severity: IssueSeverityError,
					Message: "external item referenced by more than one confirmed match: " + id, Ref: ref, ItemID: id,
				})
				ok = false
				continue
			}
			externalItems = append(externalItems, item)
		}

		if !ok {
			continue
		}

		// Currency compatibility across the group's own items.
		currency := ""
		mixed := false
		for _, it := range bookItems {
			if it.Currency == "" {
				continue
			}
			if currency == "" {
				currency = it.Currency
			} else if currency != it.Currency {
				mixed = true
			}
		}
		for _, it := range externalItems {
			if it.Currency == "" {
				continue
			}
			if currency == "" {
				currency = it.Currency
			} else if currency != it.Currency {
				mixed = true
			}
		}
		if mixed {
			issues = append(issues, Issue{
				Code: IssueMixedCurrency, Severity: IssueSeverityError,
				Message: "confirmed match mixes incompatible currencies", Ref: ref,
			})
			continue
		}

		// Amount agreement, unless explained.
		var bookSum, externalSum float64
		for _, it := range bookItems {
			bookSum += it.SignedAmount()
		}
		for _, it := range externalItems {
			externalSum += it.OrientedSignedAmount(policy.orientation())
		}
		diff := bookSum - externalSum
		if m.ExplainedDifference != nil {
			if abs(diff) > abs(*m.ExplainedDifference)+policy.AmountTolerance {
				issues = append(issues, Issue{
					Code: IssueInvalidConfirmedMatch, Severity: IssueSeverityError,
					Message: "confirmed match difference exceeds the declared explained difference", Ref: ref,
				})
				continue
			}
		} else if !amountsMatch(bookSum, externalSum, policy.AmountTolerance, policy.RelativeTolerance) {
			issues = append(issues, Issue{
				Code: IssueInvalidConfirmedMatch, Severity: IssueSeverityError,
				Message: "confirmed match book and external amounts do not tie within policy tolerance", Ref: ref,
			})
			continue
		}

		accepted = append(accepted, m)
	}

	return accepted, issues
}

func confirmedMatchFallbackRef(i int) string {
	return "confirmed[" + strconv.Itoa(i) + "]"
}
