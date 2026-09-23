package statements

import (
	"sort"

	"github.com/themurtez/go-valuate/accounting/ledger"
	"github.com/themurtez/go-valuate/financial"
)

// aggregateMappedAccounts is the single aggregation function every
// statement/dataset output in this package builds from — see
// mappedAccountFigures' doc comment for why one function serving every
// output matters. For each resolvedBalance belonging to a MAPPED account
// (MappingStatusMapped or MappingStatusSuggested; MappingStatusUnmapped/
// MappingStatusInvalid accounts contribute nothing here — see
// evaluateUnmappedMateriality for how those are surfaced instead), it:
//
//  1. resolves the mapping's target(s) — one (FinancialCode,
//     StatementType, SignTreatment) for a normal mapping, or several for
//     an allocated one (see AccountMapping.isAllocated),
//  2. applies canonicalAmount (signs.go) to convert the raw balance into
//     the canonical positive-magnitude convention, scaled by
//     AllocationRule.Percent when allocated,
//  3. sums every account's contribution into one mappedAccountFigures per
//     (FinancialCode, StatementType) pair, tracking per-account
//     RowContributor provenance,
//  4. applies Options.Hierarchy's leaf-posting policy (HierarchyLeafOnly:
//     every mapped account, parent or leaf, contributes its own DIRECT
//     balance only — resolvedBalance is already a direct, non-rolled-up
//     balance per ledger.CalculateBalances/ledger.NormalizeTrialBalance,
//     so no extra rollup-exclusion step is needed here; this package
//     simply never calls ledger.BuildRollups itself, which is what would
//     introduce the double-count the task warns against).
//
// Returned figures are sorted by FinancialCode for determinism. Never
// mutates its inputs.
func aggregateMappedAccounts(
	chart ledger.ChartOfAccounts,
	balances []resolvedBalance,
	mappingResults []AccountMappingResult,
	wantStatementType financial.StatementType,
) []mappedAccountFigures {
	mappingByAccount := make(map[string]AccountMappingResult, len(mappingResults))
	for _, m := range mappingResults {
		mappingByAccount[m.AccountID] = m
	}

	type key struct {
		code financial.Code
	}
	acc := make(map[key]*mappedAccountFigures)
	var order []key

	get := func(code financial.Code, stType financial.StatementType) *mappedAccountFigures {
		k := key{code: code}
		f, ok := acc[k]
		if !ok {
			f = &mappedAccountFigures{
				code:           code,
				statementType:  stType,
				valuesByPeriod: make(map[financial.Period]float64),
			}
			acc[k] = f
			order = append(order, k)
		}
		return f
	}

	contributorAmounts := make(map[key]map[string]map[financial.Period]float64) // key -> accountID -> period -> amount
	allocationPercent := make(map[key]map[string]float64)                       // key -> accountID -> percent (only set when the mapping was allocated)

	for _, b := range balances {
		mr, ok := mappingByAccount[b.accountID]
		if !ok {
			continue
		}
		if mr.MappingStatus != MappingStatusMapped && mr.MappingStatus != MappingStatusSuggested {
			continue
		}
		acct, _ := chart.Lookup(b.accountID)
		isAllocated := mr.Mapping.isAllocated()

		targets := mappingTargets(mr.Mapping)
		for _, t := range targets {
			if t.statementType != wantStatementType {
				continue
			}
			scaledRaw := b.rawBalance * t.percentOfBalance
			amount := canonicalAmount(scaledRaw, acct.Type, t.sign)

			f := get(t.code, t.statementType)
			f.valuesByPeriod[b.period] += amount

			k := key{code: t.code}
			if contributorAmounts[k] == nil {
				contributorAmounts[k] = make(map[string]map[financial.Period]float64)
			}
			if contributorAmounts[k][b.accountID] == nil {
				contributorAmounts[k][b.accountID] = make(map[financial.Period]float64)
			}
			contributorAmounts[k][b.accountID][b.period] += amount

			if isAllocated {
				if allocationPercent[k] == nil {
					allocationPercent[k] = make(map[string]float64)
				}
				allocationPercent[k][b.accountID] = t.percentOfBalance
			}
		}
	}

	sort.Slice(order, func(i, j int) bool { return order[i].code < order[j].code })

	out := make([]mappedAccountFigures, 0, len(order))
	for _, k := range order {
		f := *acc[k]

		accountIDs := make([]string, 0, len(contributorAmounts[k]))
		for id := range contributorAmounts[k] {
			accountIDs = append(accountIDs, id)
		}
		sort.Strings(accountIDs)

		for _, id := range accountIDs {
			acct, _ := chart.Lookup(id)
			contributor := RowContributor{
				AccountID:     id,
				AccountNumber: acct.Number,
				AccountName:   acct.Name,
				Amounts:       contributorAmounts[k][id],
			}
			if pct, ok := allocationPercent[k][id]; ok {
				contributor.AllocationPercent = &pct
			}
			f.contributors = append(f.contributors, contributor)
		}

		out = append(out, f)
	}

	return out
}

// mappingTarget is one resolved (code, statement type, sign treatment,
// fraction-of-balance) tuple a single AccountMapping expands into — one
// tuple for a normal mapping, one per AllocationRule for an allocated
// mapping.
type mappingTarget struct {
	code             financial.Code
	statementType    financial.StatementType
	sign             SignTreatment
	percentOfBalance float64
}

// mappingTargets expands mapping into its constituent mappingTarget
// values. A mapping that failed validation (has no usable code, e.g.
// empty FinancialCode with no Allocations) returns nil — such a mapping
// never reaches this function in practice, since aggregateMappedAccounts
// only calls it for MappingStatusMapped/MappingStatusSuggested accounts,
// both of which already passed validateMapping.
func mappingTargets(mapping AccountMapping) []mappingTarget {
	if mapping.isAllocated() {
		targets := make([]mappingTarget, 0, len(mapping.Allocations))
		for _, a := range mapping.Allocations {
			targets = append(targets, mappingTarget{
				code:             a.FinancialCode,
				statementType:    a.StatementType,
				sign:             a.SignTreatment,
				percentOfBalance: a.Percent,
			})
		}
		return targets
	}
	if mapping.FinancialCode == "" {
		return nil
	}
	return []mappingTarget{{
		code:             mapping.FinancialCode,
		statementType:    mapping.StatementType,
		sign:             mapping.SignTreatment,
		percentOfBalance: 1.0,
	}}
}
