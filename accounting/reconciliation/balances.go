package reconciliation

// BalanceProvenance identifies whether a Balance's EndingBalance came
// directly from the caller (SUPPLIED) or was computed by this package
// from Opening + Activity (DERIVED) — task section 44.
type BalanceProvenance string

const (
	BalanceSupplied BalanceProvenance = "SUPPLIED"
	BalanceDerived  BalanceProvenance = "DERIVED"
)

// BalanceInput is one side's (book or external) caller-supplied opening/
// ending balance declaration for the period being reconciled. Both
// fields are optional and independently availability-tracked — see
// Balance's Available fields. Activity (period movement), when supplied
// separately as BookItems/ExternalItems, is used for rollforward/
// derivation; BalanceInput itself carries only the two balance points.
type BalanceInput struct {
	OpeningBalance     *float64 `json:"opening_balance,omitempty"`
	EndingBalance      *float64 `json:"ending_balance,omitempty"`
	OpeningBalanceDate string   `json:"opening_balance_date,omitempty"`
	EndingBalanceDate  string   `json:"ending_balance_date,omitempty"`
}

// Balance is this package's resolved view of one side's opening/ending
// balance for the period, including provenance and — where sufficient
// data exists — a side-specific rollforward check (task section 45:
// "Opening + Activity = Expected Ending", reported independently of
// cross-side reconciliation).
type Balance struct {
	OpeningAvailable bool    `json:"opening_available"`
	OpeningBalance   float64 `json:"opening_balance,omitempty"`

	EndingAvailable  bool              `json:"ending_available"`
	EndingBalance    float64           `json:"ending_balance,omitempty"`
	EndingProvenance BalanceProvenance `json:"ending_provenance,omitempty"`

	// ActivityTotal is the sum of this side's item SignedAmount()s
	// (OrientedSignedAmount() for the external side) that were supplied
	// as transaction-level activity, when any were. ActivityAvailable is
	// true only if at least one item was supplied for this side.
	ActivityAvailable bool    `json:"activity_available"`
	ActivityTotal     float64 `json:"activity_total,omitempty"`

	// ExpectedEnding = OpeningBalance + ActivityTotal, computed only when
	// both OpeningAvailable and ActivityAvailable are true.
	ExpectedEndingAvailable bool    `json:"expected_ending_available"`
	ExpectedEnding          float64 `json:"expected_ending,omitempty"`

	// RollforwardDifference = EndingBalance - ExpectedEnding, computed
	// only when both EndingAvailable and ExpectedEndingAvailable are
	// true. RollforwardMismatch is true when the absolute difference
	// exceeds MatchingPolicy.AmountTolerance (an absolute-only check —
	// this is a same-side sanity check, not the cross-side reconciliation
	// difference, so it deliberately does not also apply
	// RelativeTolerance, which is reserved for match-level comparisons).
	RollforwardDifferenceAvailable bool    `json:"rollforward_difference_available"`
	RollforwardDifference          float64 `json:"rollforward_difference,omitempty"`
	RollforwardMismatch            bool    `json:"rollforward_mismatch"`

	// SuppliedDerivedMismatch is true only when the caller supplied an
	// EndingBalance AND derivation was enabled AND sufficient data
	// existed to derive one AND the two disagree beyond tolerance — see
	// resolveBalance. In that case EndingBalance/EndingProvenance still
	// reflect the caller's SUPPLIED value (this package never silently
	// replaces it — task section 43), and
	// IssueSuppliedDerivedBalanceMismatch is also reported.
	SuppliedDerivedMismatch bool `json:"supplied_derived_mismatch"`
}

// resolveBalance computes Balance for one side from bal (the caller's
// BalanceInput), items (that side's already-oriented signed item
// amounts, in no particular order — the caller passes OrientedSignedAmount()
// results for the external side and SignedAmount() results for the book
// side), whether derivation is enabled, and orientationMult (+1 for the
// book side, always; orientationMultiplier(policy.Orientation) for the
// external side). orientationMult is applied to bal.OpeningBalance/
// EndingBalance on read, so a caller-supplied external balance is put
// into the SAME signed frame as the book side before any comparison —
// this mirrors the orientation adjustment ExternalItem.OrientedSignedAmount
// already applies to transaction-level items (task section 18: "critical
// for liabilities such as credit cards and loans," which applies just as
// much to a balance-only reconciliation as to a transaction-level one).
// It never mutates bal or items.
func resolveBalance(bal BalanceInput, items []float64, deriveEnabled bool, tolerance, orientationMult float64) (Balance, []Issue) {
	var b Balance
	var issues []Issue

	if bal.OpeningBalance != nil && !isNonFinite(*bal.OpeningBalance) {
		b.OpeningAvailable = true
		b.OpeningBalance = *bal.OpeningBalance * orientationMult
	}

	if len(items) > 0 {
		b.ActivityAvailable = true
		var total float64
		for _, v := range items {
			total += v
		}
		b.ActivityTotal = total
	}

	var derivedEnding float64
	var haveDerived bool
	if b.OpeningAvailable && b.ActivityAvailable {
		b.ExpectedEndingAvailable = true
		b.ExpectedEnding = b.OpeningBalance + b.ActivityTotal
		derivedEnding = b.ExpectedEnding
		haveDerived = true
	}

	var suppliedEnding float64
	var haveSupplied bool
	if bal.EndingBalance != nil && !isNonFinite(*bal.EndingBalance) {
		suppliedEnding = *bal.EndingBalance * orientationMult
		haveSupplied = true
	}

	switch {
	case haveSupplied:
		b.EndingAvailable = true
		b.EndingBalance = suppliedEnding
		b.EndingProvenance = BalanceSupplied
		if deriveEnabled && haveDerived {
			diff := suppliedEnding - derivedEnding
			if abs(diff) > tolerance {
				b.SuppliedDerivedMismatch = true
				issues = append(issues, Issue{
					Code:     IssueSuppliedDerivedBalanceMismatch,
					Severity: IssueSeverityWarning,
					Message:  "supplied ending balance disagrees with the balance derived from opening + activity",
				})
			}
		}
	case deriveEnabled && haveDerived:
		b.EndingAvailable = true
		b.EndingBalance = derivedEnding
		b.EndingProvenance = BalanceDerived
	}

	if b.EndingAvailable && b.ExpectedEndingAvailable {
		b.RollforwardDifferenceAvailable = true
		b.RollforwardDifference = b.EndingBalance - b.ExpectedEnding
		b.RollforwardMismatch = abs(b.RollforwardDifference) > tolerance
	}

	return b, issues
}
