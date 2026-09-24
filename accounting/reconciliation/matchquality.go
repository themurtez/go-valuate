package reconciliation

// buildMatchQualitySummary classifies every book+external item into
// exactly one MatchQualitySummary bucket by how it was resolved — task
// section 49. Bucket assignment reads MatchGroup.MatchReason/MatchType
// directly (never re-derives it), so this stays in lockstep with
// matcher.go/composite.go/confirmed-match application without
// duplicating their logic.
func buildMatchQualitySummary(
	book []BookItem, external []ExternalItem, groups []MatchGroup,
	unmatchedBookIDs, unmatchedExternalIDs []string,
	bookByID map[string]BookItem, externalByID map[string]ExternalItem,
	ambiguousItemIDs map[string]bool,
	policy MatchingPolicy,
) MatchQualitySummary {
	bucketOf := make(map[string]string, len(book)+len(external)) // itemID -> bucket name

	classify := func(ids []string, bucket string) {
		for _, id := range ids {
			bucketOf[id] = bucket
		}
	}

	for _, g := range groups {
		bucket := bucketForReason(g.MatchReason)
		classify(g.BookItemIDs, bucket)
		classify(g.ExternalItemIDs, bucket)
	}
	for id := range ambiguousItemIDs {
		if _, already := bucketOf[id]; !already {
			bucketOf[id] = "ambiguous"
		}
	}
	for _, id := range unmatchedBookIDs {
		if _, already := bucketOf[id]; !already {
			bucketOf[id] = "unmatched"
		}
	}
	for _, id := range unmatchedExternalIDs {
		if _, already := bucketOf[id]; !already {
			bucketOf[id] = "unmatched"
		}
	}

	amounts := map[string][]float64{}
	for _, it := range book {
		b := bucketOf[it.ItemID]
		amounts[b] = append(amounts[b], it.SignedAmount())
	}
	for _, it := range external {
		b := bucketOf[it.ItemID]
		amounts[b] = append(amounts[b], it.OrientedSignedAmount(policy.orientation()))
	}

	return MatchQualitySummary{
		Confirmed:        buildPopulationSummary(amounts["confirmed"]),
		ReferenceMatch:   buildPopulationSummary(amounts["reference_match"]),
		SameDateAmount:   buildPopulationSummary(amounts["same_date_amount"]),
		DateWindowAmount: buildPopulationSummary(amounts["date_window_amount"]),
		Composite:        buildPopulationSummary(amounts["composite"]),
		Ambiguous:        buildPopulationSummary(amounts["ambiguous"]),
		Unmatched:        buildPopulationSummary(amounts["unmatched"]),
	}
}

func bucketForReason(r MatchReason) string {
	switch r {
	case ReasonConfirmed:
		return "confirmed"
	case ReasonExactReferenceAmountDate, ReasonExactReferenceAmount:
		return "reference_match"
	case ReasonExactAmountDate:
		return "same_date_amount"
	case ReasonExactAmountWithinWindow:
		return "date_window_amount"
	case ReasonCompositeSumMatch:
		return "composite"
	default:
		return "unmatched"
	}
}
