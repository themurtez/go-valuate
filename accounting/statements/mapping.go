package statements

import (
	"sort"

	"github.com/themurtez/go-valuate/accounting/ledger"
	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/classification"
)

// AccountMappingResult is the per-account mapping-review contract this
// package exposes for every account in the resolved chart — see the
// task's explicit "mapping review contract" requirement. It carries
// everything a future application's review screen needs to render a row
// and let a human confirm/override it, without this package building any
// UI itself.
type AccountMappingResult struct {
	// AccountID, AccountNumber, AccountName, AccountType, ParentID mirror
	// the source ledger.Account fields directly, so a review screen never
	// needs a second chart lookup.
	AccountID     string             `json:"account_id"`
	AccountNumber string             `json:"account_number,omitempty"`
	AccountName   string             `json:"account_name,omitempty"`
	AccountType   ledger.AccountType `json:"account_type"`
	ParentID      string             `json:"parent_id,omitempty"`
	// Active mirrors ledger.Account.Active — see inactive-account handling
	// in validate.go.
	Active bool `json:"active"`
	// CurrentBalance is the account's resolved raw balance (debit-
	// positive/credit-negative, ledger's own RawBalance convention) for
	// this build's selected period(s), summed across every included
	// period when the build covers more than one — see builder.go.
	CurrentBalance float64 `json:"current_balance"`
	// Currency is the account's resolved currency, when known.
	Currency string `json:"currency,omitempty"`

	// Mapping is the resolved AccountMapping for this account — either the
	// caller's explicit mapping, a deterministic suggestion, or a zero
	// AccountMapping with Source == MappingSourceUnmapped.
	Mapping AccountMapping `json:"mapping"`
	// Alternatives lists other candidate codes the deterministic
	// suggestion mechanism considered (classification.Result.Alternatives,
	// carried forward), when Mapping.Source ==
	// MappingSourceDeterministicSuggestion. Empty otherwise.
	Alternatives []MappingAlternative `json:"alternatives,omitempty"`
	// MappingStatus summarizes this account's resolution state — see
	// MappingStatus's doc comment.
	MappingStatus MappingStatus `json:"mapping_status"`
	// Issues lists every Issue specifically about this account's mapping
	// (a subset of Result.Issues, repeated here for a review screen that
	// wants to render issues inline per row without cross-referencing
	// AccountID against the flat Result.Issues list itself).
	Issues []Issue `json:"issues,omitempty"`
}

// MappingAlternative mirrors classification.Candidate's shape without
// importing that package's exact type into this package's stable
// contract — consistent with review.ClassificationAlternative's identical
// choice and rationale (see review/types.go).
type MappingAlternative struct {
	FinancialCode financial.Code `json:"financial_code"`
	Confidence    float64        `json:"confidence"`
	Reason        string         `json:"reason,omitempty"`
}

// MappingStatus is a per-account, structured summary of mapping
// resolution state, for review-UI filtering/grouping without re-deriving
// it from Mapping.Source/Confirmed/Issues each time.
type MappingStatus string

const (
	// MappingStatusMapped means a valid, non-conflicting mapping (explicit
	// or suggested) was resolved for this account.
	MappingStatusMapped MappingStatus = "MAPPED"
	// MappingStatusSuggested means a deterministic suggestion was
	// produced but not yet confirmed (Mapping.Confirmed == false) — a
	// subset of MappingStatusMapped's population that a review UI may
	// want to highlight separately.
	MappingStatusSuggested MappingStatus = "SUGGESTED"
	// MappingStatusUnmapped means no mapping could be resolved.
	MappingStatusUnmapped MappingStatus = "UNMAPPED"
	// MappingStatusInvalid means a mapping was supplied/resolved but
	// failed validation (see validate.go) and was therefore not used to
	// build the FinancialDataset.
	MappingStatusInvalid MappingStatus = "INVALID"
)

// resolveMappings resolves one AccountMappingResult per account in chart,
// applying the precedence explicit > deterministic suggestion > unmapped
// (task section 4/5), then validates each resolved mapping (validate.go)
// and folds in account-level issues. Deterministic: chart.IDs() order in,
// same order out. Never mutates chart, explicit, or opts.
func resolveMappings(
	chart ledger.ChartOfAccounts,
	balancesByAccount map[string]resolvedBalance,
	explicit []AccountMapping,
	opts MappingOptions,
) ([]AccountMappingResult, []Issue) {
	explicitByAccount, conflictIssues := indexExplicitMappings(explicit)

	mode := resolvedMappingMode(opts.Mode)
	cfg := opts.effectiveClassificationConfig()

	var allIssues []Issue
	allIssues = append(allIssues, conflictIssues...)
	allIssues = append(allIssues, danglingExplicitMappingIssues(explicit, chart)...)

	results := make([]AccountMappingResult, 0, chart.Len())
	for _, id := range chart.IDs() {
		acct, _ := chart.Lookup(id)
		bal := balancesByAccount[id]

		mapping, alternatives := resolveOneMapping(acct, chart, explicitByAccount[id], mode, cfg)

		res := AccountMappingResult{
			AccountID:      acct.ID,
			AccountNumber:  acct.Number,
			AccountName:    acct.Name,
			AccountType:    acct.Type,
			ParentID:       acct.ParentID,
			Active:         acct.Active,
			CurrentBalance: bal.rawBalance,
			Currency:       bal.currency,
			Mapping:        mapping,
			Alternatives:   alternatives,
		}

		acctIssues := validateMapping(acct, mapping, chart)
		res.Issues = acctIssues
		res.MappingStatus = deriveMappingStatus(mapping, acctIssues)

		allIssues = append(allIssues, acctIssues...)
		results = append(results, res)
	}

	return results, allIssues
}

// resolveOneMapping applies the fixed precedence for a single account:
// explicit (if present and non-empty) wins outright; otherwise, if mode
// requests suggestions, run the deterministic classifier; otherwise
// unmapped. Never overrides an explicit mapping using account-name
// heuristics — see the task's explicit rule.
func resolveOneMapping(
	acct ledger.Account,
	chart ledger.ChartOfAccounts,
	explicit *AccountMapping,
	mode MappingMode,
	cfg classification.Config,
) (AccountMapping, []MappingAlternative) {
	if explicit != nil {
		m := *explicit
		if m.Source == "" {
			m.Source = MappingSourceExplicit
		}
		return m, nil
	}

	if mode == MappingSuggestDeterministic {
		return suggestMapping(acct, chart, cfg)
	}

	return AccountMapping{AccountID: acct.ID, Source: MappingSourceUnmapped}, nil
}

// suggestMapping adapts acct into a synthetic financial.RawLineItem (Label
// = account Name, ParentLabel = parent account's Name if any,
// StatementType derived from AccountType via statementTypeForAccountType
// since ledger.Account carries none of its own — see accounttype.go) and
// runs it through financial/classification.Classify, per the task's
// suggested path: "ledger.Account.Name + parent account name + known
// account type -> financial/classification -> suggested financial.Code."
//
// A suggestion is NEVER a confirmation (Confirmed is always false here)
// and NEVER silently accepted when the classifier returns
// classification.SourceUnknown — an UNKNOWN result produces
// MappingSourceUnmapped, exactly the same outcome as if no suggestion had
// been attempted at all, per the task's explicit "do not silently accept
// UNKNOWN" rule.
//
// If the classifier's proposed code is incompatible with acct.Type (see
// isAccountTypeCompatible), the suggestion is also downgraded to
// MappingSourceUnmapped rather than accepted and later merely flagged —
// account-type compatibility is a safety constraint on suggestions
// themselves, not just on caller-supplied explicit mappings, per the
// task's "account type should constrain impossible statement mappings
// where practical" instruction.
func suggestMapping(acct ledger.Account, chart ledger.ChartOfAccounts, cfg classification.Config) (AccountMapping, []MappingAlternative) {
	parentLabel := ""
	if acct.ParentID != "" {
		if parent, ok := chart.Lookup(acct.ParentID); ok {
			parentLabel = parent.Name
		}
	}

	raw := financial.RawLineItem{
		ID:            "account:" + acct.ID,
		StatementType: statementTypeForAccountType(acct.Type),
		Label:         acct.Name,
		ParentLabel:   parentLabel,
		Values:        map[financial.Period]float64{},
	}

	result := classification.Classify(raw, cfg)

	if result.IsUnknown() || result.Code == "" {
		return AccountMapping{AccountID: acct.ID, Source: MappingSourceUnmapped}, nil
	}

	if !isAccountTypeCompatible(acct.Type, result.Code) {
		return AccountMapping{AccountID: acct.ID, Source: MappingSourceUnmapped}, nil
	}

	meta, _ := financial.LookupCode(result.Code)

	mapping := AccountMapping{
		AccountID:                acct.ID,
		FinancialCode:            result.Code,
		StatementType:            meta.StatementType,
		SignTreatment:            SignNatural,
		Source:                   MappingSourceDeterministicSuggestion,
		Confirmed:                false,
		ClassificationReason:     result.Reason,
		ClassificationConfidence: float64(result.Confidence),
	}

	var alternatives []MappingAlternative
	for _, alt := range result.Alternatives {
		alternatives = append(alternatives, MappingAlternative{
			FinancialCode: alt.Code,
			Confidence:    float64(alt.Confidence),
			Reason:        alt.Reason,
		})
	}

	return mapping, alternatives
}

// indexExplicitMappings builds an AccountID -> *AccountMapping index from
// a caller-supplied slice, reporting IssueMappingConflict for any
// AccountID appearing more than once (the caller must resolve the
// ambiguity itself — this package never picks arbitrarily, per the task's
// explicit rule). A conflicting account is excluded from the returned
// index entirely, so it falls through to suggestion/unmapped handling
// rather than using either of its conflicting definitions.
func indexExplicitMappings(mappings []AccountMapping) (map[string]*AccountMapping, []Issue) {
	seen := make(map[string]int, len(mappings)) // account ID -> count
	byAccount := make(map[string]*AccountMapping, len(mappings))
	var conflicted map[string]bool

	for i := range mappings {
		id := mappings[i].AccountID
		if id == "" {
			continue
		}
		seen[id]++
		if seen[id] == 1 {
			m := mappings[i]
			byAccount[id] = &m
			continue
		}
		if conflicted == nil {
			conflicted = make(map[string]bool)
		}
		conflicted[id] = true
	}

	var issues []Issue
	if len(conflicted) > 0 {
		ids := make([]string, 0, len(conflicted))
		for id := range conflicted {
			ids = append(ids, id)
			delete(byAccount, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			issues = append(issues, Issue{
				Code:      IssueMappingConflict,
				Severity:  SeverityError,
				Message:   "account " + id + " has more than one explicit mapping supplied",
				AccountID: id,
			})
		}
	}

	return byAccount, issues
}

// danglingExplicitMappingIssues reports IssueInvalidMapping for every
// caller-supplied AccountMapping whose AccountID does not resolve to a
// real account in chart. This exists because resolveMappings' own main
// loop walks chart.IDs() (every account THE CHART actually has), so a
// mapping targeting an account ID absent from the chart entirely would
// otherwise never be visited by validateMapping at all and would be
// silently dropped rather than flagged — task section 7's "referenced
// account exists" check must fire even for a mapping with no
// corresponding chart entry, not only for mappings on real accounts.
func danglingExplicitMappingIssues(explicit []AccountMapping, chart ledger.ChartOfAccounts) []Issue {
	var issues []Issue
	seen := make(map[string]bool)
	for _, m := range explicit {
		if m.AccountID == "" || seen[m.AccountID] {
			continue
		}
		if _, ok := chart.Lookup(m.AccountID); ok {
			continue
		}
		seen[m.AccountID] = true
		issues = append(issues, Issue{
			Code:      IssueInvalidMapping,
			Severity:  SeverityError,
			Message:   "mapping references unknown account: " + m.AccountID,
			AccountID: m.AccountID,
		})
	}
	return issues
}

// deriveMappingStatus computes MappingStatus from a resolved mapping and
// its own validation issues, in one place so every caller agrees.
func deriveMappingStatus(mapping AccountMapping, issues []Issue) MappingStatus {
	if HasErrors(issues) {
		return MappingStatusInvalid
	}
	switch mapping.Source {
	case MappingSourceExplicit:
		return MappingStatusMapped
	case MappingSourceDeterministicSuggestion:
		return MappingStatusSuggested
	default:
		return MappingStatusUnmapped
	}
}

// MappingOptions configures resolveMappings/Build's mapping stage.
type MappingOptions struct {
	// Mode selects whether deterministic suggestions are attempted for
	// accounts without an explicit mapping. Zero value resolves to
	// MappingExplicitOnly (the safe default).
	Mode MappingMode
	// ClassificationConfig, when Mode == MappingSuggestDeterministic,
	// supplies the financial/classification.Config to classify against
	// (aliases, rules, review threshold). A zero Config still runs
	// structural detection but classifies every account as
	// classification.SourceUnknown for anything else, so a caller wanting
	// real suggestions should normally pass
	// classification.Config{Rules: classification.DefaultRules()} or its
	// own richer config.
	ClassificationConfig classification.Config
}

func (o MappingOptions) effectiveClassificationConfig() classification.Config {
	return o.ClassificationConfig
}
