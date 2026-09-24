package reconciliation

// buildMatchSummaryBook computes MatchSummary for the book side: matched
// items are those whose ItemID is in matchedIDs, unmatched are every
// other item — task section 27.
func buildMatchSummaryBook(items []BookItem, matchedIDs map[string]bool) MatchSummary {
	var matched, unmatched []float64
	for _, it := range items {
		amt := it.SignedAmount()
		if matchedIDs[it.ItemID] {
			matched = append(matched, amt)
		} else {
			unmatched = append(unmatched, amt)
		}
	}
	return MatchSummary{
		Matched:   buildPopulationSummary(matched),
		Unmatched: buildPopulationSummary(unmatched),
	}
}

// buildMatchSummaryExternal mirrors buildMatchSummaryBook for the
// external side, applying Orientation before summing.
func buildMatchSummaryExternal(items []ExternalItem, matchedIDs map[string]bool, orientation Orientation) MatchSummary {
	var matched, unmatched []float64
	for _, it := range items {
		amt := it.OrientedSignedAmount(orientation)
		if matchedIDs[it.ItemID] {
			matched = append(matched, amt)
		} else {
			unmatched = append(unmatched, amt)
		}
	}
	return MatchSummary{
		Matched:   buildPopulationSummary(matched),
		Unmatched: buildPopulationSummary(unmatched),
	}
}
