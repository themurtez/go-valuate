package statements

import "math"

// MappingCoverage summarizes mapping completeness across an entire chart
// — task section 8's explicit requirement that coverage-by-count alone is
// not enough ("a single unmapped account containing 40% of expenses
// should be obvious"): this type reports both count coverage and amount
// coverage, computed independently.
type MappingCoverage struct {
	// TotalAccounts is the number of accounts considered.
	TotalAccounts int `json:"total_accounts"`
	// MappedAccounts is the number of accounts with MappingStatus ==
	// MappingStatusMapped or MappingStatusSuggested (a suggestion counts
	// as mapped for coverage purposes — task section 8 does not
	// distinguish confirmed from suggested for this summary; a review UI
	// wanting that finer split should filter Result.Mappings by
	// MappingStatus/Confirmed itself).
	MappedAccounts int `json:"mapped_accounts"`
	// UnmappedAccounts is TotalAccounts - MappedAccounts -
	// InvalidAccounts.
	UnmappedAccounts int `json:"unmapped_accounts"`
	// InvalidAccounts is the number of accounts whose mapping failed
	// validation (MappingStatusInvalid) — counted separately from
	// UnmappedAccounts since "mapped but wrong" and "never mapped" are
	// different problems for a review screen to surface.
	InvalidAccounts int `json:"invalid_accounts"`
	// CountCoveragePercent is MappedAccounts / TotalAccounts * 100, in
	// [0, 100]. 0 if TotalAccounts is 0.
	CountCoveragePercent float64 `json:"count_coverage_percent"`

	// MappedBalanceAmount is the sum of |CurrentBalance| across every
	// mapped account (absolute value, since a coverage figure comparing
	// magnitudes should not let an asset's debit-positive balance and a
	// liability's credit-negative raw balance cancel each other out —
	// see the task's explicit "mapped balance by amount" requirement,
	// which is a magnitude concept, not a signed net).
	MappedBalanceAmount float64 `json:"mapped_balance_amount"`
	// UnmappedBalanceAmount mirrors MappedBalanceAmount for unmapped
	// accounts (invalid-mapping accounts are excluded from both sums,
	// since their balance is neither cleanly mapped nor cleanly
	// unmapped).
	UnmappedBalanceAmount float64 `json:"unmapped_balance_amount"`
	// AmountCoveragePercent is MappedBalanceAmount /
	// (MappedBalanceAmount + UnmappedBalanceAmount) * 100, in [0, 100]. 0
	// if the denominator is 0 — this is the figure that makes a single
	// large unmapped account impossible to miss even when it is a small
	// fraction of the account COUNT.
	AmountCoveragePercent float64 `json:"amount_coverage_percent"`
}

// computeCoverage builds a MappingCoverage from resolved
// AccountMappingResult values — "count coverage" and "amount coverage,
// where mathematically meaningful" per task section 8.
func computeCoverage(results []AccountMappingResult) MappingCoverage {
	var c MappingCoverage
	c.TotalAccounts = len(results)

	for _, r := range results {
		abs := math.Abs(r.CurrentBalance)
		switch r.MappingStatus {
		case MappingStatusMapped, MappingStatusSuggested:
			c.MappedAccounts++
			c.MappedBalanceAmount += abs
		case MappingStatusInvalid:
			c.InvalidAccounts++
		default: // MappingStatusUnmapped
			c.UnmappedAccounts++
			c.UnmappedBalanceAmount += abs
		}
	}

	if c.TotalAccounts > 0 {
		c.CountCoveragePercent = float64(c.MappedAccounts) / float64(c.TotalAccounts) * 100
	}
	totalAmount := c.MappedBalanceAmount + c.UnmappedBalanceAmount
	if totalAmount > 0 {
		c.AmountCoveragePercent = c.MappedBalanceAmount / totalAmount * 100
	}

	return c
}
