package diagnostics

import "sort"

// findingCodeRank maps each FindingCode to its index in findingOrder, used
// only as a tie-break (never as a scoring input) — computed once so
// FindingOrder's comparator does not do an O(n) scan of findingOrder per
// comparison.
var findingCodeRank = func() map[FindingCode]int {
	ranks := make(map[FindingCode]int, len(findingOrder))
	for i, c := range findingOrder {
		ranks[c] = i
	}
	return ranks
}()

// computePriorityScore implements Finding.PriorityScore's fixed, explicit
// formula: policy.SeverityWeights[f.Severity], plus a magnitude bonus equal
// to |f.Change.Amount| (when Change is Available) scaled by the same
// severity weight — so that among two findings of equal Severity, the one
// with the larger underlying change ranks higher, while a single-period
// finding with no Change (FindingUnresolvedFinancialQuality,
// FindingSaleReadinessOpportunity, or a signal-only change-based finding
// with no usable Prior) still scores purely off its Severity weight.
//
// policy must already be resolved (see resolvePolicy) — Calculate is this
// function's only caller, and always passes the resolved Policy, whose
// SeverityWeights is guaranteed to have an entry for every Severity that
// can occur (including a caller-supplied explicit 0, which this function
// honors rather than silently substituting DefaultPolicy's value: a caller
// configuring SeverityWeights[SeverityInfo] = 0 to exclude info-severity
// findings from ranking entirely is a legitimate, supported use of Policy).
//
// This never introduces a threshold or classification of its own: it only
// weights an already-classified Severity (and already-computed Change) into
// one ranking number, mirroring
// transactions/salereadiness.computeOverallScore's identical
// "never disagrees with the classification, only weights it" design.
func computePriorityScore(f Finding, policy Policy) float64 {
	weight := policy.SeverityWeights[f.Severity]
	score := weight
	if f.Change.Available {
		score += weight * absFloat(f.Change.Amount)
	}
	return score
}

// sortFindings orders findings by PriorityScore descending, then by
// findingOrder's declaration order, then by BusinessID ascending — the
// fixed, deterministic, fully-specified tie-break chain Result.Findings'
// doc comment promises. Sorts in place; callers pass a slice they already
// own.
func sortFindings(findings []Finding) {
	sort.SliceStable(findings, func(i, j int) bool {
		a, b := findings[i], findings[j]
		if a.PriorityScore != b.PriorityScore {
			return a.PriorityScore > b.PriorityScore
		}
		if ra, rb := findingCodeRank[a.Code], findingCodeRank[b.Code]; ra != rb {
			return ra < rb
		}
		return a.BusinessID < b.BusinessID
	})
}
