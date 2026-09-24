package reconciliation

// Status is the overall reconciliation classification for one period —
// task section 28. See deriveStatus for the exact, documented decision
// rule.
type Status string

const (
	StatusReconciled          Status = "RECONCILED"
	StatusReconciledWithItems Status = "RECONCILED_WITH_ITEMS"
	StatusUnreconciled        Status = "UNRECONCILED"
	StatusIncomplete          Status = "INCOMPLETE"
	StatusInvalid             Status = "INVALID"
)

// statusInputs is every fact deriveStatus needs, gathered here so the
// decision rule itself reads as one flat function instead of threading
// half a dozen loose parameters — mirrors closequality's readiness.go
// precedent of isolating the status decision into one small, heavily
// documented function.
type statusInputs struct {
	reconciliationKeyValid bool
	asOfDateValid          bool

	// balanceDataSufficient is true when EquationResult.Available is
	// true (both sides' ending balances resolved) OR transaction-level
	// data was supplied for both sides (so a balance-only Input and a
	// transaction-only Input are each independently "sufficient," per
	// task section 9/51's "balance-only vs transaction coverage stay
	// separate" instruction — a caller who only wants transaction
	// matching, with no balances at all, is not automatically
	// INCOMPLETE).
	balanceDataSufficient bool

	equationAvailable bool
	withinTolerance   bool

	hasUnresolvedMaterialUnmatched bool
	hasUnresolvedReconcilingItems  bool
	hasAmbiguousMatches            bool
}

// deriveStatus applies the fixed decision table (task section 28):
//
//	INVALID:
//	  the reconciliation key or AsOfDate itself is invalid — the input
//	  could not even be meaningfully processed. This is deliberately
//	  narrower than "any error-severity Issue exists anywhere": a
//	  malformed individual item, a duplicate ID, or a confirmed match
//	  that failed validation each produce their own error-severity Issue
//	  (see issues.go) but are recoverable per-item problems this
//	  package's validation already excludes/falls back from — they do
//	  not, on their own, make the whole reconciliation INVALID. Only the
//	  reconciliation's own identity (AccountID/AsOfDate) is load-bearing
//	  enough to justify INVALID.
//
//	INCOMPLETE:
//	  the key/AsOfDate are valid but neither a resolvable balance
//	  (EquationResult.Available) nor any transaction-level item data was
//	  supplied for both sides — there is not enough data to assess
//	  reconciliation at all.
//
//	RECONCILED:
//	  adjusted balances tie within tolerance (or no balance data was
//	  supplied but transaction data was, and every material item is
//	  matched) AND no material unmatched items, unresolved (non-
//	  immaterial, per MatchingPolicy.UnmatchedPolicy) reconciling items,
//	  or ambiguous matches remain.
//
//	RECONCILED_WITH_ITEMS:
//	  the same balance-tie condition as RECONCILED holds, but explicit
//	  ReconcilingItems remain on at least one side (task section 28: "tie
//	  but explicit reconciling items remain").
//
//	UNRECONCILED:
//	  a balance difference beyond tolerance exists, OR material
//	  unresolved items (unmatched or ambiguous) remain, regardless of
//	  balance tie.
//
// This function never mutates its input and performs no I/O — see
// determinism_test.go.
func deriveStatus(in statusInputs) Status {
	if !in.reconciliationKeyValid || !in.asOfDateValid {
		return StatusInvalid
	}
	if !in.balanceDataSufficient {
		return StatusIncomplete
	}

	balanceTies := !in.equationAvailable || in.withinTolerance

	if !balanceTies {
		return StatusUnreconciled
	}
	if in.hasUnresolvedMaterialUnmatched || in.hasAmbiguousMatches {
		return StatusUnreconciled
	}
	if in.hasUnresolvedReconcilingItems {
		return StatusReconciledWithItems
	}
	return StatusReconciled
}
