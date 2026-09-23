package ar

import "sort"

// PriorityWeights configures CollectionPriority's explicit, caller-
// overridable formula components — section 21. This is deliberately
// labeled a heuristic ranking score, never a "probability of default" or
// any other statistical/credit-risk claim, per the task's explicit
// instruction.
type PriorityWeights struct {
	// BalanceWeight scales the customer's normalized OverdueTotal
	// (OverdueTotal / portfolio OverdueTotal) component.
	BalanceWeight float64 `json:"balance_weight,omitempty"`
	// DaysPastDueWeight scales the customer's normalized
	// WeightedAvgDaysPastDue (divided by the oldest bucket's
	// MinDaysPastDue, capped at 1.0) component.
	DaysPastDueWeight float64 `json:"days_past_due_weight,omitempty"`
	// BucketSeverityWeight scales the customer's worst-bucket severity
	// rank (normalized 0..1 across the resolved bucket schema).
	BucketSeverityWeight float64 `json:"bucket_severity_weight,omitempty"`
	// ConcentrationWeight scales the customer's share of total AR (from
	// ConcentrationSummary.TotalAR).
	ConcentrationWeight float64 `json:"concentration_weight,omitempty"`
	// DisputePenalty subtracts this fixed amount (0..1 scale) from a
	// customer's score if DisputedAmount > 0 — a disputed balance is
	// typically not collections-actionable in the same way.
	DisputePenalty float64 `json:"dispute_penalty,omitempty"`
}

// DefaultPriorityWeights returns a simple, equally-weighted baseline.
func DefaultPriorityWeights() PriorityWeights {
	return PriorityWeights{
		BalanceWeight:        0.4,
		DaysPastDueWeight:    0.3,
		BucketSeverityWeight: 0.2,
		ConcentrationWeight:  0.1,
		DisputePenalty:       0.1,
	}
}

func resolvePriorityWeights(w PriorityWeights) PriorityWeights {
	if w == (PriorityWeights{}) {
		return DefaultPriorityWeights()
	}
	return w
}

// PriorityFactor is one named, transparent component contributing to a
// CollectionPriorityEntry.Score — returned so a caller can see exactly how
// the heuristic score was built, per the task's "components are returned"
// instruction.
type PriorityFactor struct {
	Name         string  `json:"name"`
	Normalized   float64 `json:"normalized"`
	Weight       float64 `json:"weight"`
	Contribution float64 `json:"contribution"`
}

// CollectionPriorityEntry is one customer's heuristic collection-priority
// ranking.
type CollectionPriorityEntry struct {
	CustomerID string `json:"customer_id"`
	// Score is an explicit, formula-based heuristic in [0, 1] (higher =
	// higher collection priority). Never a probability-of-default claim —
	// see the package doc comment.
	Score   float64          `json:"score"`
	Factors []PriorityFactor `json:"factors"`
	Rank    int              `json:"rank"`
}

// CollectionPriority is section 21's output: a heuristic ranking, always
// labeled as such, with weights the caller can fully override.
type CollectionPriority struct {
	Available bool                      `json:"available"`
	Weights   PriorityWeights           `json:"weights"`
	Entries   []CollectionPriorityEntry `json:"entries,omitempty"`
	// Label is a fixed, always-present disclaimer distinguishing this from
	// any statistical or credit-risk model.
	Label string `json:"label"`
}

const collectionPriorityLabel = "heuristic collection-priority ranking; not a probability of default or credit-risk model"

func calculateCollectionPriority(customers []CustomerSummary, sortedBuckets []BucketDefinition, totalOverdue float64, arConcentration ConcentrationSummary, weights PriorityWeights) CollectionPriority {
	if len(customers) == 0 {
		return CollectionPriority{}
	}
	w := resolvePriorityWeights(weights)

	arShareByCustomer := map[string]float64{}
	if arConcentration.TotalAR.Available && len(arConcentration.TotalAR.History) > 0 {
		for _, re := range arConcentration.TotalAR.History[len(arConcentration.TotalAR.History)-1].RankedEntities {
			if re.Share.Available {
				arShareByCustomer[re.EntityKey] = re.Share.Value
			}
		}
	}

	maxBucketDays := 0.0
	for _, b := range sortedBuckets {
		if float64(b.MinDaysPastDue) > maxBucketDays {
			maxBucketDays = float64(b.MinDaysPastDue)
		}
	}

	entries := make([]CollectionPriorityEntry, 0, len(customers))
	for _, c := range customers {
		if c.OverdueTotal <= 0 {
			continue
		}

		var balanceNorm float64
		if totalOverdue != 0 {
			balanceNorm = c.OverdueTotal / totalOverdue
		}

		var daysNorm float64
		if maxBucketDays > 0 && c.WeightedAvgDaysPastDue.Available {
			daysNorm = c.WeightedAvgDaysPastDue.Value / maxBucketDays
			if daysNorm > 1 {
				daysNorm = 1
			}
		}

		severityNorm := worstBucketSeverityNorm(c, sortedBuckets)

		concNorm := arShareByCustomer[c.CustomerID]

		factors := []PriorityFactor{
			{Name: "balance_size", Normalized: clamp01(balanceNorm), Weight: w.BalanceWeight, Contribution: clamp01(balanceNorm) * w.BalanceWeight},
			{Name: "days_past_due", Normalized: clamp01(daysNorm), Weight: w.DaysPastDueWeight, Contribution: clamp01(daysNorm) * w.DaysPastDueWeight},
			{Name: "bucket_severity", Normalized: clamp01(severityNorm), Weight: w.BucketSeverityWeight, Contribution: clamp01(severityNorm) * w.BucketSeverityWeight},
			{Name: "ar_concentration", Normalized: clamp01(concNorm), Weight: w.ConcentrationWeight, Contribution: clamp01(concNorm) * w.ConcentrationWeight},
		}
		score := 0.0
		for _, f := range factors {
			score += f.Contribution
		}
		if c.DisputedAmount != 0 {
			penalty := w.DisputePenalty
			factors = append(factors, PriorityFactor{Name: "dispute_penalty", Normalized: 1, Weight: -penalty, Contribution: -penalty})
			score -= penalty
		}
		score = clamp01(score)

		entries = append(entries, CollectionPriorityEntry{CustomerID: c.CustomerID, Score: score, Factors: factors})
	}

	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Score != entries[j].Score {
			return entries[i].Score > entries[j].Score
		}
		return entries[i].CustomerID < entries[j].CustomerID
	})
	for i := range entries {
		entries[i].Rank = i + 1
	}

	return CollectionPriority{Available: len(entries) > 0, Weights: w, Entries: entries, Label: collectionPriorityLabel}
}

func worstBucketSeverityNorm(c CustomerSummary, sortedBuckets []BucketDefinition) float64 {
	if len(sortedBuckets) <= 1 {
		return 0
	}
	worst := -1
	for _, b := range c.Buckets {
		if b.Amount <= 0 {
			continue
		}
		rank, ok := bucketSeverity(sortedBuckets, b.BucketCode)
		if ok && rank > worst {
			worst = rank
		}
	}
	if worst < 0 {
		return 0
	}
	return float64(worst) / float64(len(sortedBuckets)-1)
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
