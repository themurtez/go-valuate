package statements

import (
	"github.com/themurtez/go-valuate/accounting/ledger"
	"github.com/themurtez/go-valuate/financial/classification"
)

// AccountMappingRule is one reusable, portable matching rule within a
// MappingTemplate — plain domain data with no persistence identity of its
// own (no rule ID, no client ID), per task section 32's explicit "remains
// plain domain data... do not add persistence/account IDs" instruction.
// Exactly one of AccountID/AccountNumber/ExactName should be set per rule
// (see ResolveTemplate's precedence); a rule with more than one set is
// still matched against every field it has, and precedence still applies
// in the fixed order.
type AccountMappingRule struct {
	// AccountID, if set, matches by exact ledger.Account.ID — the highest
	// precedence per task section 33.
	AccountID string `json:"account_id,omitempty"`
	// AccountNumber, if set, matches by exact ledger.Account.Number —
	// second precedence.
	AccountNumber string `json:"account_number,omitempty"`
	// ExactName, if set, matches by exact normalized ledger.Account.Name
	// (via financial/classification.NormalizeLabel's Comparable form,
	// reused directly rather than this package inventing a second
	// normalization routine that could disagree with classification's own
	// — see the task's explicit "no fuzzy name similarity unless already
	// available and clearly deterministic" instruction, which
	// NormalizeLabel satisfies: it is a fixed, deterministic
	// transformation, not fuzzy matching) — third precedence.
	ExactName string `json:"exact_name,omitempty"`
	// Mapping is the AccountMapping to apply when this rule matches. Its
	// own AccountID field is ignored/overwritten with the matched
	// account's real ID when the template is resolved — see
	// ResolveTemplate.
	Mapping AccountMapping `json:"mapping"`
}

// MappingTemplate is a reusable, caller-supplied set of AccountMappingRule
// values — task section 32's explicit "reusable caller-supplied mapping
// templates" requirement. Plain domain data; a future application layer
// is expected to add persistence (a default template per client, a
// per-valuation-run override) around this type, never into it.
type MappingTemplate struct {
	// Version is a caller-defined identifier for this template's own
	// revision (e.g. "retail-v3"), echoed back for provenance but never
	// interpreted by this package.
	Version string `json:"version"`
	// Mappings is the ordered set of rules this template resolves
	// against a chart of accounts.
	Mappings []AccountMappingRule `json:"mappings"`
}

// ResolveTemplate matches template's rules against chart, producing one
// AccountMapping per account that matched, using the fixed deterministic
// precedence task section 33 specifies:
//
//	Account ID -> Account Number -> Exact Normalized Name -> (unmatched)
//
// A higher-precedence rule kind always wins over a lower one for the same
// account, regardless of rule order within Mappings. Within the SAME
// precedence kind, more than one rule matching the same account is an
// unresolved ambiguity — ResolveTemplate reports IssueMappingConflict for
// it (task section 34) and excludes that account from the returned
// mappings entirely, rather than picking arbitrarily. Two rules of
// DIFFERENT kinds matching different accounts is not a conflict, even if
// both kinds are present in the same template.
//
// Deterministic classifier suggestions are explicitly NOT part of
// ResolveTemplate itself — mapping.go's suggestMapping is the caller's
// separate opt-in fallback (via MappingSuggestDeterministic) for accounts
// a template's explicit rules leave unmatched, exactly matching task
// section 33's "-> deterministic classifier suggestion -> unmapped" tail
// of the precedence chain, applied one layer up by Build itself when a
// resolved template's output is passed in as Input.Mappings.
//
// Never mutates template or chart.
func ResolveTemplate(template MappingTemplate, chart ledger.ChartOfAccounts) ([]AccountMapping, []Issue) {
	byID := make(map[string][]AccountMappingRule)
	byNumber := make(map[string][]AccountMappingRule)
	byName := make(map[string][]AccountMappingRule)

	for _, rule := range template.Mappings {
		if rule.AccountID != "" {
			byID[rule.AccountID] = append(byID[rule.AccountID], rule)
		}
		if rule.AccountNumber != "" {
			byNumber[rule.AccountNumber] = append(byNumber[rule.AccountNumber], rule)
		}
		if rule.ExactName != "" {
			key := classification.NormalizeLabel(rule.ExactName).Comparable
			byName[key] = append(byName[key], rule)
		}
	}

	var mappings []AccountMapping
	var issues []Issue

	for _, id := range chart.IDs() {
		acct, _ := chart.Lookup(id)

		matched, conflictIssue := matchAccountToRules(acct, byID, byNumber, byName)
		if conflictIssue != nil {
			issues = append(issues, *conflictIssue)
			continue
		}
		if matched == nil {
			continue
		}

		m := matched.Mapping
		m.AccountID = acct.ID
		if m.Source == "" {
			m.Source = MappingSourceExplicit
		}
		mappings = append(mappings, m)
	}

	return mappings, issues
}

// matchAccountToRules applies the fixed ID -> Number -> ExactName
// precedence for one account, returning the winning rule (or nil if no
// rule matches at any level) and a non-nil conflict Issue if the
// winning-precedence level itself has more than one candidate.
func matchAccountToRules(
	acct ledger.Account,
	byID, byNumber, byName map[string][]AccountMappingRule,
) (*AccountMappingRule, *Issue) {
	levels := []struct {
		key   string
		rules map[string][]AccountMappingRule
		kind  string
	}{
		{acct.ID, byID, "account ID"},
		{acct.Number, byNumber, "account number"},
		{classification.NormalizeLabel(acct.Name).Comparable, byName, "exact name"},
	}

	for _, level := range levels {
		if level.key == "" {
			continue
		}
		candidates := level.rules[level.key]
		if len(candidates) == 0 {
			continue
		}
		if len(candidates) > 1 {
			return nil, &Issue{
				Code:      IssueMappingConflict,
				Severity:  SeverityError,
				Message:   "account " + acct.ID + " matches more than one template rule by " + level.kind,
				AccountID: acct.ID,
			}
		}
		return &candidates[0], nil
	}

	return nil, nil
}
