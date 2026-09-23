package ledger

import "sort"

// validateHierarchy checks Account.ParentID links for two problems:
// a ParentID referencing an account not present in accounts
// (IssueMissingParentAccount), and a ParentID chain that cycles back to its
// starting account, including an account naming itself as its own parent
// (IssueAccountHierarchyCycle). Accounts with no ParentID are always valid
// (a top-level account). Issues are returned in accounts' input order, one
// pass per account; a cycle is reported once per account that participates
// in it (from that account's own perspective), not once globally, so every
// affected account is individually flagged.
func validateHierarchy(accounts []Account) []Issue {
	byID := make(map[string]Account, len(accounts))
	for _, a := range accounts {
		if a.ID != "" {
			byID[a.ID] = a
		}
	}

	var issues []Issue
	for _, a := range accounts {
		if a.ID == "" || a.ParentID == "" {
			continue
		}
		if a.ParentID == a.ID {
			issues = append(issues, Issue{
				Code:     IssueAccountHierarchyCycle,
				Severity: SeverityError,
				Message:  "account " + a.ID + " lists itself as its own parent",
				Account:  a.ID,
			})
			continue
		}
		parent, ok := byID[a.ParentID]
		if !ok {
			issues = append(issues, Issue{
				Code:     IssueMissingParentAccount,
				Severity: SeverityError,
				Message:  "account " + a.ID + " references missing parent account " + a.ParentID,
				Account:  a.ID,
			})
			continue
		}

		// Walk the chain from a's parent upward, bounded by len(byID)+1
		// steps: any longer walk without revisiting a must have revisited
		// some other node first, which the visited set below catches
		// first, but the explicit bound guarantees termination regardless.
		visited := map[string]bool{a.ID: true}
		cur := parent
		cycle := false
		for step := 0; step <= len(byID); step++ {
			if visited[cur.ID] {
				cycle = true
				break
			}
			visited[cur.ID] = true
			if cur.ParentID == "" {
				break
			}
			next, ok := byID[cur.ParentID]
			if !ok {
				break // missing parent further up; already reported for that account
			}
			cur = next
		}
		if cycle {
			issues = append(issues, Issue{
				Code:     IssueAccountHierarchyCycle,
				Severity: SeverityError,
				Message:  "account " + a.ID + " is part of a parent-chain cycle",
				Account:  a.ID,
			})
		}
	}
	return issues
}

// Children returns the IDs of every account in chart whose ParentID equals
// parentID, sorted lexically by ID for determinism.
func (c ChartOfAccounts) Children(parentID string) []string {
	var out []string
	for _, id := range c.order {
		if c.byID[id].ParentID == parentID {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

// Descendants returns every account ID transitively rolling up into
// parentID (children, grandchildren, ...), sorted lexically. If the chart
// contains a hierarchy cycle reachable from parentID, Descendants stops
// expanding a branch the moment it would revisit an already-included node,
// so it always terminates — callers should still run ValidateAccounts to
// be told about the cycle explicitly rather than relying on this silent
// truncation.
func (c ChartOfAccounts) Descendants(parentID string) []string {
	seen := make(map[string]bool)
	var walk func(id string)
	walk = func(id string) {
		for _, childID := range c.Children(id) {
			if seen[childID] {
				continue
			}
			seen[childID] = true
			walk(childID)
		}
	}
	walk(parentID)

	out := make([]string, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}
