package ledger

import "sort"

// RollupBalance is one account's balance within a hierarchy rollup: its own
// direct postings, the sum of its descendants' postings, and the combined
// total. Distinguishing Direct from Child balances matters because a
// parent account can itself receive postings (not just serve as a
// grouping node) — see the package doc comment's rollup section — and
// naively summing "every account including parents" would double-count
// whatever a parent's own descendants already contributed.
type RollupBalance struct {
	AccountID   string      `json:"account_id"`
	AccountType AccountType `json:"account_type"`
	// ParentID echoes the account's own Account.ParentID (empty for a
	// top-level account).
	ParentID string `json:"parent_id,omitempty"`

	// DirectRawBalance and DirectDisplayBalance are this account's own
	// Balance.RawBalance/DisplayBalance, from postings made directly to it
	// (not via any descendant).
	DirectRawBalance     float64 `json:"direct_raw_balance"`
	DirectDisplayBalance float64 `json:"direct_display_balance"`

	// ChildRawBalance and ChildDisplayBalance are the sum of every
	// descendant's DirectRawBalance/DirectDisplayBalance (recursively,
	// covering children, grandchildren, and so on) — never double-counted,
	// since each descendant contributes its own Direct figures exactly
	// once, regardless of how deep the hierarchy is.
	ChildRawBalance     float64 `json:"child_raw_balance"`
	ChildDisplayBalance float64 `json:"child_display_balance"`

	// TotalRawBalance and TotalDisplayBalance are Direct + Child — the
	// figure a caller displaying a rolled-up chart of accounts should show
	// for this account's row.
	TotalRawBalance     float64 `json:"total_raw_balance"`
	TotalDisplayBalance float64 `json:"total_display_balance"`

	// DescendantCount is the number of accounts included in ChildRawBalance
	// (len of ChartOfAccounts.Descendants(AccountID)).
	DescendantCount int `json:"descendant_count"`
}

// BuildRollups computes a RollupBalance for every account in chart, from
// balances (typically CalculateBalances' output for the same chart). It
// does not mutate chart or balances. If chart contains a hierarchy cycle,
// BuildRollups still terminates (see ChartOfAccounts.Descendants' doc
// comment on cycle truncation) but the result for accounts in the cycle
// should not be trusted — callers should run ValidateAccounts first and
// treat any IssueAccountHierarchyCycle as blocking. Returned sorted by
// AccountID.
func BuildRollups(chart ChartOfAccounts, balances []Balance) []RollupBalance {
	directRaw := make(map[string]float64, len(balances))
	directDisplay := make(map[string]float64, len(balances))
	for _, b := range balances {
		directRaw[b.AccountID] = b.RawBalance
		directDisplay[b.AccountID] = b.DisplayBalance
	}

	out := make([]RollupBalance, 0, chart.Len())
	for _, id := range chart.IDs() {
		acct, _ := chart.Lookup(id)
		descendants := chart.Descendants(id)

		r := RollupBalance{
			AccountID:            id,
			AccountType:          acct.Type,
			ParentID:             acct.ParentID,
			DirectRawBalance:     directRaw[id],
			DirectDisplayBalance: directDisplay[id],
			DescendantCount:      len(descendants),
		}
		for _, childID := range descendants {
			r.ChildRawBalance += directRaw[childID]
			r.ChildDisplayBalance += directDisplay[childID]
		}
		r.TotalRawBalance = r.DirectRawBalance + r.ChildRawBalance
		r.TotalDisplayBalance = r.DirectDisplayBalance + r.ChildDisplayBalance

		out = append(out, r)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].AccountID < out[j].AccountID })
	return out
}
