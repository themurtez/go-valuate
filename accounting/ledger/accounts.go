package ledger

// Account is a single entry in a portable chart of accounts. It is
// deliberately independent of financial.Code — see the package doc
// comment's "Boundary to the existing financial model" section — so it can
// represent any caller's real chart of accounts, however that caller's
// numbering/naming scheme works.
type Account struct {
	// ID is a caller-assigned identifier unique within a Ledger. Required.
	ID string `json:"id"`
	// Number is the caller's account number/code (e.g. "1000",
	// "6100-01"), preserved for display and caller-side lookup. Optional;
	// this package never parses or infers meaning from it.
	Number string `json:"number,omitempty"`
	// Name is the account's display name (e.g. "Accounts Receivable").
	Name string `json:"name"`
	// Type is the account's high-level classification, which determines its
	// NormalBalance. Required; see AccountType.
	Type AccountType `json:"type"`
	// Subtype is an optional, caller-defined finer classification within
	// Type (e.g. "current_asset", "fixed_asset", "accrued_liability").
	// Open string, not a closed enum — this package never interprets it.
	Subtype string `json:"subtype,omitempty"`
	// ParentID is the ID of this account's parent in the chart-of-accounts
	// hierarchy, for rollup purposes (see hierarchy.go). Empty means this
	// account has no parent (a top-level account).
	ParentID string `json:"parent_id,omitempty"`
	// Currency is the ISO 4217 currency code (e.g. "USD") this account's
	// postings are denominated in. Optional; when empty, this account is
	// treated as denominated in the Ledger/report's base currency for
	// mixed-currency detection — see the package doc comment's
	// multi-currency section and currency.go.
	Currency string `json:"currency,omitempty"`
	// Active is false for an account that should no longer receive new
	// postings (e.g. deactivated in the caller's system). Historical
	// balances/postings on an inactive account remain valid; only new
	// postings to it are flagged — see IssueInactiveAccountPosting.
	Active bool `json:"active"`
}

// ChartOfAccounts indexes a slice of Account by ID for O(1) lookup, and is
// the shared lookup structure every function in this package that needs to
// resolve an account ID builds once and reuses, rather than each doing its
// own linear scan.
type ChartOfAccounts struct {
	byID map[string]Account
	// order preserves the original slice order accounts was built from, so
	// any iteration over the full chart stays deterministic and matches
	// caller-supplied order rather than Go map order.
	order []string
}

// BuildChartOfAccounts indexes accounts by ID. It does not validate
// accounts (see ValidateAccounts for that) and does not mutate accounts —
// it copies each Account by value into its internal map.
func BuildChartOfAccounts(accounts []Account) ChartOfAccounts {
	c := ChartOfAccounts{
		byID:  make(map[string]Account, len(accounts)),
		order: make([]string, 0, len(accounts)),
	}
	for _, a := range accounts {
		if _, exists := c.byID[a.ID]; !exists {
			c.order = append(c.order, a.ID)
		}
		c.byID[a.ID] = a
	}
	return c
}

// Lookup returns the Account for id and whether it was found.
func (c ChartOfAccounts) Lookup(id string) (Account, bool) {
	a, ok := c.byID[id]
	return a, ok
}

// IDs returns every account ID in the chart, in the original input order
// (first occurrence, for a chart built from input containing duplicates).
func (c ChartOfAccounts) IDs() []string {
	out := make([]string, len(c.order))
	copy(out, c.order)
	return out
}

// Len returns the number of distinct account IDs in the chart.
func (c ChartOfAccounts) Len() int { return len(c.order) }

// ValidateAccounts checks a chart of accounts for structural problems:
// missing ID, missing/unrecognized Type, duplicate IDs, and (via
// validateHierarchy) missing parents and hierarchy cycles. It does not
// mutate accounts. Issues are returned in a deterministic order: duplicate-
// ID issues in input order, then per-account structural issues in input
// order, then hierarchy issues — see validateHierarchy.
func ValidateAccounts(accounts []Account) []Issue {
	var issues []Issue

	seen := make(map[string]int, len(accounts)) // id -> first index
	for i, a := range accounts {
		if a.ID == "" {
			issues = append(issues, Issue{
				Code:     IssueUnknownAccount,
				Severity: SeverityError,
				Message:  "account has an empty ID",
				Account:  a.Number,
			})
			continue
		}
		if firstIdx, dup := seen[a.ID]; dup {
			issues = append(issues, Issue{
				Code:     IssueDuplicateAccount,
				Severity: SeverityError,
				Message:  "duplicate account ID: " + a.ID,
				Account:  a.ID,
			})
			_ = firstIdx
			continue
		}
		seen[a.ID] = i

		if a.Name == "" {
			issues = append(issues, Issue{
				Code:     IssueInvalidAccount,
				Severity: SeverityWarning,
				Message:  "account " + a.ID + " has an empty Name",
				Account:  a.ID,
			})
		}
		if _, ok := NormalBalance(a.Type); !ok {
			issues = append(issues, Issue{
				Code:     IssueInvalidAccount,
				Severity: SeverityError,
				Message:  "account " + a.ID + " has an unrecognized Type: " + string(a.Type),
				Account:  a.ID,
			})
		}
	}

	issues = append(issues, validateHierarchy(accounts)...)
	return issues
}
