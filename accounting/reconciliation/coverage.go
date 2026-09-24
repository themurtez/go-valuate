package reconciliation

// PopulationSummary reports counts and gross/net amounts for one
// matched/unmatched population — task section 27: "Keep gross absolute
// amount and signed net semantics distinct where useful."
type PopulationSummary struct {
	Count       int     `json:"count"`
	GrossAmount float64 `json:"gross_amount"` // sum of abs(SignedAmount)
	NetAmount   float64 `json:"net_amount"`   // sum of SignedAmount
}

// MatchSummary aggregates matched/unmatched populations for one side
// (book or external).
type MatchSummary struct {
	Matched   PopulationSummary `json:"matched"`
	Unmatched PopulationSummary `json:"unmatched"`
}

// MatchQualitySummary reports counts/amounts broken out by how each item
// was resolved — task section 49: "No opaque score." Every bucket here
// is mutually exclusive per item (an item is counted in exactly one
// bucket).
type MatchQualitySummary struct {
	Confirmed        PopulationSummary `json:"confirmed"`
	ReferenceMatch   PopulationSummary `json:"reference_match"`
	SameDateAmount   PopulationSummary `json:"same_date_amount"`
	DateWindowAmount PopulationSummary `json:"date_window_amount"`
	Composite        PopulationSummary `json:"composite"`
	Ambiguous        PopulationSummary `json:"ambiguous"`
	Unmatched        PopulationSummary `json:"unmatched"`
}

// Coverage reports factual coverage of the reconciliation — task section
// 50: "No composite quality score." Percentages are 0-1 decimals.
type Coverage struct {
	BookItemCount     int `json:"book_item_count"`
	ExternalItemCount int `json:"external_item_count"`

	MatchedBookItemPercent     float64 `json:"matched_book_item_percent"`
	MatchedExternalItemPercent float64 `json:"matched_external_item_percent"`

	MatchedBookAmountPercent     float64 `json:"matched_book_amount_percent"`
	MatchedExternalAmountPercent float64 `json:"matched_external_amount_percent"`

	BalanceAvailable bool `json:"balance_available"`

	// ReferenceCoveragePercent/DescriptionCoveragePercent are the share
	// of ALL items (book + external combined) carrying a non-empty
	// Reference/Description respectively — a data-quality signal, not a
	// matching-quality signal (task section 50).
	ReferenceCoveragePercent   float64 `json:"reference_coverage_percent"`
	DescriptionCoveragePercent float64 `json:"description_coverage_percent"`
}

func buildPopulationSummary(items []float64) PopulationSummary {
	var p PopulationSummary
	for _, v := range items {
		p.Count++
		p.NetAmount += v
		p.GrossAmount += abs(v)
	}
	return p
}

// buildCoverage computes Coverage from the full (deduplicated, validated)
// item populations and the final match/unmatch partition.
func buildCoverage(
	allBook []BookItem, allExternal []ExternalItem,
	matchedBookIDs, matchedExternalIDs map[string]bool,
	bookBalanceAvailable, externalBalanceAvailable bool,
) Coverage {
	var c Coverage
	c.BookItemCount = len(allBook)
	c.ExternalItemCount = len(allExternal)
	c.BalanceAvailable = bookBalanceAvailable && externalBalanceAvailable

	var matchedBookAmt, totalBookAmt, matchedExtAmt, totalExtAmt float64
	var matchedBookCount, matchedExtCount int
	var refCount, descCount, totalCount int

	for _, b := range allBook {
		amt := abs(b.SignedAmount())
		totalBookAmt += amt
		totalCount++
		if b.Reference != "" {
			refCount++
		}
		if b.Description != "" {
			descCount++
		}
		if matchedBookIDs[b.ItemID] {
			matchedBookCount++
			matchedBookAmt += amt
		}
	}
	for _, e := range allExternal {
		amt := abs(e.SignedAmount())
		totalExtAmt += amt
		totalCount++
		if e.Reference != "" {
			refCount++
		}
		if e.Description != "" {
			descCount++
		}
		if matchedExternalIDs[e.ItemID] {
			matchedExtCount++
			matchedExtAmt += amt
		}
	}

	if len(allBook) > 0 {
		c.MatchedBookItemPercent = float64(matchedBookCount) / float64(len(allBook))
	}
	if len(allExternal) > 0 {
		c.MatchedExternalItemPercent = float64(matchedExtCount) / float64(len(allExternal))
	}
	if totalBookAmt > 0 {
		c.MatchedBookAmountPercent = matchedBookAmt / totalBookAmt
	}
	if totalExtAmt > 0 {
		c.MatchedExternalAmountPercent = matchedExtAmt / totalExtAmt
	}
	if totalCount > 0 {
		c.ReferenceCoveragePercent = float64(refCount) / float64(totalCount)
		c.DescriptionCoveragePercent = float64(descCount) / float64(totalCount)
	}

	return c
}
