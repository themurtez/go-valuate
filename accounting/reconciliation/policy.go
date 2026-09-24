package reconciliation

// ReferenceNormalization controls which safe, deterministic
// transformations are applied to Reference strings before comparing them
// — task section 14. Only the transformations named here are ever
// applied; there is no fuzzy/NLP normalization anywhere in this package.
type ReferenceNormalization struct {
	Trim              bool `json:"trim"`
	CaseFold          bool `json:"case_fold"`
	RemoveSpaces      bool `json:"remove_spaces"`
	RemovePunctuation bool `json:"remove_punctuation"`
	// StripLeadingZeros, when true, removes leading zeros (e.g. "0042"
	// -> "42"). Default (false, the zero value) leaves leading zeros
	// meaningful — task section 14: "Default leading zeros remain
	// meaningful."
	StripLeadingZeros bool `json:"strip_leading_zeros"`
}

// MaterialityPolicy declares when an unmatched item or an unresolved
// balance difference counts as material — mirrors closequality's
// identically-shaped MaterialityPolicy (an intentionally-narrow local
// copy per this repository's convention; see
// financial/adjustments.HasErrors's doc comment for the "no shared
// cross-package type" rationale). When every field is zero, materiality
// gating is off and everything is treated as material — matching
// review.IsMaterial's/closequality.isMaterial's documented "OFF by
// default" semantics.
type MaterialityPolicy struct {
	AbsoluteAmount   float64 `json:"absolute_amount,omitempty"`
	PercentOfBalance float64 `json:"percent_of_balance,omitempty"`
}

func isMaterial(amount float64, balance *float64, p MaterialityPolicy) bool {
	a := abs(amount)
	if p.AbsoluteAmount > 0 && a >= p.AbsoluteAmount {
		return true
	}
	if p.PercentOfBalance > 0 && balance != nil && a >= p.PercentOfBalance*abs(*balance) {
		return true
	}
	if p.AbsoluteAmount <= 0 && p.PercentOfBalance <= 0 {
		return true
	}
	return false
}

// UnmatchedPolicy controls how required-to-match unresolved populations
// affect Status — task section 29.
type UnmatchedPolicy string

const (
	// RequireAllItemsMatched: any unmatched item at all (regardless of
	// materiality) keeps Status from reaching StatusReconciled/
	// StatusReconciledWithItems.
	RequireAllItemsMatched UnmatchedPolicy = "REQUIRE_ALL_ITEMS_MATCHED"
	// AllowImmaterialUnmatched (the default): only material unmatched
	// items (per Materiality) affect Status; immaterial unmatched items
	// still appear in UnmatchedBookItems/UnmatchedExternalItems (never
	// silently discarded — task section 29) but do not by themselves
	// prevent a reconciled Status.
	AllowImmaterialUnmatched UnmatchedPolicy = "ALLOW_IMMATERIAL_UNMATCHED"
)

// MatchingPolicy is every caller-configurable threshold and rule this
// package uses for matching, tolerance, and status derivation. Nothing
// that materially changes a match decision or Status is a hidden
// constant — see docs/ACCOUNT_RECONCILIATION.md.
type MatchingPolicy struct {
	// AmountTolerance is the absolute tolerance for amount comparisons
	// (task section 16). No universal default is invented — the zero
	// value means exact equality is required.
	AmountTolerance float64 `json:"amount_tolerance,omitempty"`
	// RelativeTolerance is an additional relative tolerance (e.g. 0.001
	// for 0.1%), applied as relative_tolerance * max(abs(a), abs(b)).
	// Two amounts match if they are within EITHER AmountTolerance OR the
	// relative-tolerance band — see amountsMatch.
	RelativeTolerance float64 `json:"relative_tolerance,omitempty"`

	// DateWindowDays is the maximum date difference (in days) allowed for
	// EXACT_AMOUNT_WITHIN_WINDOW matching (task section 17). Zero means
	// no date-window matching is attempted (only same-date matching, per
	// section 70's conservative default).
	DateWindowDays int `json:"date_window_days,omitempty"`

	ReferenceNormalization ReferenceNormalization `json:"reference_normalization"`

	// DescriptionExactMatchEnabled opts in to using normalized-description
	// equality as additional (never sole) matching evidence — task
	// section 15. Off by default; even when on, this package only ever
	// checks exact normalized equality, never fuzzy/NLP similarity.
	DescriptionExactMatchEnabled bool `json:"description_exact_match_enabled"`

	// EnableCompositeMatching opts in to bounded one-to-many/many-to-one
	// automatic matching — off by default (task section 70). See
	// composite.go and the Max* bounds below.
	EnableCompositeMatching bool `json:"enable_composite_matching"`
	// MaxCompositeGroupSize bounds how many items may be summed on the
	// "many" side of a composite match.
	MaxCompositeGroupSize int `json:"max_composite_group_size,omitempty"`
	// MaxCandidatesPerItem bounds how many same-date-window candidate
	// items are considered as composite-sum ingredients for one anchor
	// item, before search is attempted at all.
	MaxCandidatesPerItem int `json:"max_candidates_per_item,omitempty"`
	// MaxCompositeSearchCombinations bounds the total number of subset
	// combinations explored across the whole composite search for one
	// anchor item. If this cap would be exceeded, search for that anchor
	// stops and IssueCompositeSearchLimitReached is reported — never
	// unbounded search (task section 21).
	MaxCompositeSearchCombinations int `json:"max_composite_search_combinations,omitempty"`

	// RequireAllItemsMatched / AllowImmaterialUnmatched — see
	// UnmatchedPolicy. The zero value ("") is treated as
	// AllowImmaterialUnmatched.
	UnmatchedPolicy UnmatchedPolicy   `json:"unmatched_policy,omitempty"`
	Materiality     MaterialityPolicy `json:"materiality"`

	// StaleDaysThreshold is the minimum age (in days, relative to
	// Input.AsOfDate) before an unmatched or reconciling item produces a
	// STALE_* finding (task section 53). Zero means staleness findings
	// are never produced — no universal threshold is invented.
	StaleDaysThreshold int `json:"stale_days_threshold,omitempty"`

	// Orientation declares whether the external side's signed convention
	// runs the same way as the book side's — see the package doc
	// comment's "Orientation" section. The zero value ("") is treated as
	// OrientationSame.
	Orientation Orientation `json:"orientation,omitempty"`

	// ReportingCurrency, if set, is the currency every item is expected
	// to already be denominated in. Purely a label/validation aid — this
	// package never converts currency; see the package doc comment's
	// "Currency" section.
	ReportingCurrency string `json:"reporting_currency,omitempty"`

	// DeriveBalances opts in to computing an ending balance from Opening
	// + Activity when the caller did not supply one directly, or to
	// cross-checking a supplied one against the derived figure — see
	// balances.go and task section 43. Off by default.
	DeriveBalances bool `json:"derive_balances"`

	// BalanceTolerance is the absolute tolerance used for rollforward
	// and supplied-vs-derived balance comparisons (balances.go). Falls
	// back to AmountTolerance when zero.
	BalanceTolerance float64 `json:"balance_tolerance,omitempty"`
}

func (p MatchingPolicy) balanceTolerance() float64 {
	if p.BalanceTolerance > 0 {
		return p.BalanceTolerance
	}
	return p.AmountTolerance
}

func (p MatchingPolicy) unmatchedPolicy() UnmatchedPolicy {
	if p.UnmatchedPolicy == RequireAllItemsMatched {
		return RequireAllItemsMatched
	}
	return AllowImmaterialUnmatched
}

func (p MatchingPolicy) orientation() Orientation {
	return p.Orientation
}

// DefaultMatchingPolicy returns the conservative defaults recommended by
// task section 70: exact/near-exact amounts, same-date matching before
// date-window matching is attempted, no composite matching, no fuzzy
// description matching, and material unmatched items stay unresolved
// (AllowImmaterialUnmatched with no Materiality threshold set means
// everything is material by default — see isMaterial).
func DefaultMatchingPolicy() MatchingPolicy {
	return MatchingPolicy{
		ReferenceNormalization: ReferenceNormalization{
			Trim:     true,
			CaseFold: true,
		},
		UnmatchedPolicy: AllowImmaterialUnmatched,
	}
}

// amountsMatch reports whether a and b are equal within tolerance's
// absolute bound or relTolerance's relative bound (relative to the
// larger magnitude), whichever is more permissive. Both tolerance values
// must be >= 0 (validated separately — see validatePolicy); a negative
// value here is treated as 0.
func amountsMatch(a, b, tolerance, relTolerance float64) bool {
	if tolerance < 0 {
		tolerance = 0
	}
	if relTolerance < 0 {
		relTolerance = 0
	}
	diff := abs(a - b)
	if diff <= tolerance {
		return true
	}
	if relTolerance > 0 {
		base := abs(a)
		if abs(b) > base {
			base = abs(b)
		}
		if diff <= relTolerance*base {
			return true
		}
	}
	return false
}

// validatePolicy checks p for structurally invalid configuration:
// non-finite thresholds and invalid enum values.
func validatePolicy(p MatchingPolicy) []Issue {
	var issues []Issue
	invalid := func(msg string) {
		issues = append(issues, Issue{Code: IssueInvalidPolicy, Severity: IssueSeverityError, Message: msg})
	}

	for name, v := range map[string]float64{
		"amount_tolerance":               p.AmountTolerance,
		"relative_tolerance":             p.RelativeTolerance,
		"balance_tolerance":              p.BalanceTolerance,
		"materiality.absolute_amount":    p.Materiality.AbsoluteAmount,
		"materiality.percent_of_balance": p.Materiality.PercentOfBalance,
	} {
		if isNonFinite(v) {
			invalid("policy field " + name + " is non-finite")
		}
	}
	if p.AmountTolerance < 0 {
		invalid("amount_tolerance must be >= 0")
	}
	if p.RelativeTolerance < 0 {
		invalid("relative_tolerance must be >= 0")
	}
	if p.DateWindowDays < 0 {
		invalid("date_window_days must be >= 0")
	}
	if p.StaleDaysThreshold < 0 {
		invalid("stale_days_threshold must be >= 0")
	}
	if p.BalanceTolerance < 0 {
		invalid("balance_tolerance must be >= 0")
	}
	if !isRecognizedOrientation(p.Orientation) {
		invalid("orientation is not a recognized value")
	}
	if p.UnmatchedPolicy != "" && p.UnmatchedPolicy != RequireAllItemsMatched && p.UnmatchedPolicy != AllowImmaterialUnmatched {
		invalid("unmatched_policy is not a recognized value")
	}
	if p.EnableCompositeMatching {
		if p.MaxCompositeGroupSize <= 0 {
			invalid("max_composite_group_size must be > 0 when composite matching is enabled")
		}
		if p.MaxCandidatesPerItem <= 0 {
			invalid("max_candidates_per_item must be > 0 when composite matching is enabled")
		}
		if p.MaxCompositeSearchCombinations <= 0 {
			invalid("max_composite_search_combinations must be > 0 when composite matching is enabled")
		}
	}
	return issues
}
