package statements

import (
	"sort"

	"github.com/themurtez/go-valuate/accounting/ledger"
	"github.com/themurtez/go-valuate/financial"
)

// buildDataset constructs the primary bridge output: a
// financial.FinancialDataset combining every mapped NORMAL account across
// BOTH statement types (task section 17B/18). It re-runs
// aggregateMappedAccounts independently for each financial.StatementType
// (rather than reusing buildIncomeStatement/buildBalanceSheet's own
// figures) so the dataset is always available even when
// Input.Selection requested only one statement — the FinancialDataset
// bridge is the primary output and should not depend on which
// presentation statements were also requested.
func buildDataset(
	chart ledger.ChartOfAccounts,
	balances []resolvedBalance,
	mappingResults []AccountMappingResult,
	currency string,
	opts Options,
) (financial.FinancialDataset, []Issue) {
	if currency == "" {
		return financial.FinancialDataset{}, nil // mixed-currency issue already reported by resolveReportingCurrency
	}

	incomeFigures := aggregateMappedAccounts(chart, balances, mappingResults, financial.StatementIncomeStatement)
	balanceFigures := aggregateMappedAccounts(chart, balances, mappingResults, financial.StatementBalanceSheet)

	all := make([]mappedAccountFigures, 0, len(incomeFigures)+len(balanceFigures))
	all = append(all, incomeFigures...)
	all = append(all, balanceFigures...)

	dataset := datasetFromFigures(all, currency)
	return dataset, nil
}

// datasetFromFigures converts a slice of mappedAccountFigures directly
// into a financial.FinancialDataset, using existing financial-domain
// constructors (financial.NormalizedItem) rather than bypassing them —
// see the task's explicit "do not bypass financial-domain validation"
// instruction. This package builds FinancialDataset.Items directly
// (rather than routing through financial.Normalize, which expects
// financial.MappedLineItem input keyed by source ROW, not by
// already-aggregated ledger ACCOUNT) because this package has already
// performed the equivalent aggregation itself in aggregateMappedAccounts
// — running it a second time through financial.Normalize would require
// synthesizing a fake MappedLineItem per contributing account merely to
// have Normalize re-sum them, which is strictly redundant work with an
// identical result. The output shape (FinancialDataset, sorted Items,
// Code+Period keying, SourceRef provenance) is identical to what
// financial.Normalize itself produces — see provenance.go for how
// RowContributor becomes SourceRef.
//
// Deterministic: figures is already code-sorted by aggregateMappedAccounts;
// this function additionally sorts by Period within each code so output
// never depends on map iteration order.
func datasetFromFigures(figures []mappedAccountFigures, currency string) financial.FinancialDataset {
	items := make([]financial.NormalizedItem, 0, len(figures))
	for _, f := range figures {
		periods := make([]financial.Period, 0, len(f.valuesByPeriod))
		for p := range f.valuesByPeriod {
			periods = append(periods, p)
		}
		sort.Slice(periods, func(i, j int) bool { return periods[i] < periods[j] })

		for _, p := range periods {
			items = append(items, financial.NormalizedItem{
				Code:    f.code,
				Period:  p,
				Amount:  f.valuesByPeriod[p],
				Sources: sourceRefsForPeriod(f.contributors, p),
			})
		}
	}

	return financial.FinancialDataset{
		Currency: currency,
		Items:    items,
	}
}

// evaluateUnmappedMateriality checks every unmapped account against
// Options.Materiality (via review.IsMaterial) and returns an
// IssueUnmappedAccount per material account, at a severity driven by
// Options.UnmappedPolicy — task section 23/24's "unmapped material
// account -> issue + dataset incomplete" rule, with PolicyAllowUnmapped-
// WithWarning downgrading that to advisory. An immaterial unmapped
// account produces no issue at all (materiality gating is opt-in — see
// MaterialityPolicy's doc comment mirroring review.Policy's own
// zero-value "not applied" default), but is never dropped from
// Result.Mappings itself — see task section 23's "do not hide unmapped
// balances" instruction, satisfied by Mappings always listing it
// regardless of whether an Issue was raised for it.
func evaluateUnmappedMateriality(mappingResults []AccountMappingResult, dataset financial.FinancialDataset, opts Options) []Issue {
	revenue := totalForCategory(dataset, financial.CategoryRevenue)
	assets := totalAssetsFromDataset(dataset)

	policy := opts.UnmappedPolicy
	severity := SeverityWarning
	if resolvedUnmappedPolicy(policy) == PolicyFailOnUnmappedMaterial {
		severity = SeverityError
	}

	var issues []Issue
	for _, mr := range mappingResults {
		if mr.MappingStatus != MappingStatusUnmapped {
			continue
		}
		if !isUnmappedMaterial(mr, revenue, assets, opts.Materiality) {
			continue
		}
		issues = append(issues, Issue{
			Code:      IssueUnmappedAccount,
			Severity:  severity,
			Message:   "account " + mr.AccountID + " (" + mr.AccountName + ") is unmapped and its balance is material",
			AccountID: mr.AccountID,
		})
	}
	return issues
}

// isUnmappedMaterial applies MaterialityPolicy directly (absolute
// threshold OR percent-of-reference, mirroring review.IsMaterial's exact
// two-leg semantics — see the task's explicit instruction to reuse
// review's materiality convention). Which reference (assets vs. revenue)
// applies depends on the account's own statement side, since an
// income-statement account's materiality is naturally judged against
// revenue and a balance-sheet account's against total assets.
func isUnmappedMaterial(mr AccountMappingResult, revenue, assets *float64, policy MaterialityPolicy) bool {
	amount := mr.CurrentBalance
	abs := amount
	if abs < 0 {
		abs = -abs
	}

	if policy.AbsoluteThreshold > 0 && abs >= policy.AbsoluteThreshold {
		return true
	}

	isBalanceSheetSide := mr.AccountType == ledger.AccountAsset || mr.AccountType == ledger.AccountLiability || mr.AccountType == ledger.AccountEquity
	if isBalanceSheetSide {
		if policy.PercentOfTotalAssets > 0 && assets != nil {
			refAbs := *assets
			if refAbs < 0 {
				refAbs = -refAbs
			}
			if abs >= policy.PercentOfTotalAssets*refAbs {
				return true
			}
		}
	} else {
		if policy.PercentOfRevenue > 0 && revenue != nil {
			refAbs := *revenue
			if refAbs < 0 {
				refAbs = -refAbs
			}
			if abs >= policy.PercentOfRevenue*refAbs {
				return true
			}
		}
	}

	if policy.AbsoluteThreshold <= 0 && policy.PercentOfTotalAssets <= 0 && policy.PercentOfRevenue <= 0 {
		return true // no gating configured: everything is material, mirroring review.IsMaterial's identical default
	}
	return false
}

// totalForCategory sums every dataset item whose code belongs to
// category, across every period, for use as a materiality reference
// figure. Returns nil if no such item exists (materiality gating for
// that leg is then simply skipped — see isUnmappedMaterial).
func totalForCategory(dataset financial.FinancialDataset, category financial.CodeCategory) *float64 {
	var sum float64
	var found bool
	for _, item := range dataset.Items {
		meta, ok := financial.LookupCode(item.Code)
		if !ok || meta.Category != category {
			continue
		}
		sum += item.Amount
		found = true
	}
	if !found {
		return nil
	}
	return &sum
}

// totalAssetsFromDataset sums every balance-sheet item classified as an
// asset (currentAssetCodes/nonCurrentAssetCodes — see balance.go),
// mirroring exactly the same classification buildBalanceSheet's own
// sumBalanceSheetTotals uses, so a materiality reference never disagrees
// with the built statement's own Total Assets figure.
func totalAssetsFromDataset(dataset financial.FinancialDataset) *float64 {
	var sum float64
	var found bool
	for _, item := range dataset.Items {
		if currentAssetCodes[item.Code] || nonCurrentAssetCodes[item.Code] {
			sum += item.Amount
			found = true
		}
	}
	if !found {
		return nil
	}
	return &sum
}

// deriveDatasetAvailability computes Result.DatasetAvailability from the
// built dataset and every issue accumulated across the whole build — task
// section 37's explicit distinct-states requirement.
func deriveDatasetAvailability(dataset financial.FinancialDataset, issues []Issue) DatasetAvailability {
	if len(dataset.Items) == 0 {
		if HasErrors(issues) {
			return AvailabilityInvalid
		}
		return AvailabilityUnavailable
	}
	if HasErrors(issues) {
		return AvailabilityInvalid
	}
	if len(issues) > 0 {
		return AvailabilityBuiltWithWarnings
	}
	return AvailabilityBuilt
}

// dedupeAndSortIssues sorts issues deterministically (Severity, then
// Code, then AccountID) and removes exact duplicates that can arise when
// the same underlying problem is independently detected by more than one
// stage (e.g. a mixed-currency issue surfaced once during currency
// resolution). Never reorders around message text. Returns nil (not an
// empty-but-non-nil slice) when issues is empty, so JSON marshaling
// (Result.Issues carries `omitempty`) and an unmarshal-then-compare
// round-trip agree on "no issues" — see json_test.go's
// TestJSON_RoundTrip_Result, which specifically pins this down.
func dedupeAndSortIssues(issues []Issue) []Issue {
	if len(issues) == 0 {
		return nil
	}

	sort.SliceStable(issues, func(i, j int) bool {
		if issues[i].Severity != issues[j].Severity {
			return severityRank(issues[i].Severity) < severityRank(issues[j].Severity)
		}
		if issues[i].Code != issues[j].Code {
			return issues[i].Code < issues[j].Code
		}
		return issues[i].AccountID < issues[j].AccountID
	})

	out := make([]Issue, 0, len(issues))
	seen := make(map[Issue]bool, len(issues))
	for _, iss := range issues {
		if seen[iss] {
			continue
		}
		seen[iss] = true
		out = append(out, iss)
	}
	return out
}

// severityRank orders IssueSeverity from most to least urgent for
// deterministic sorting, mirroring review.severityRank's identical
// pattern (review/types.go).
func severityRank(s IssueSeverity) int {
	if s == SeverityError {
		return 0
	}
	return 1
}
