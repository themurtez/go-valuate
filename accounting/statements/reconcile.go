package statements

import (
	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/reconciliation"
)

// BalanceSheetReconciliation is this package's own, focused view of one
// period's balance-sheet reconciliation outcome — task section 25's
// explicit "Assets / Liabilities / Equity / Assets - (Liabilities +
// Equity) / balanced / difference / tolerance" contract. It is a thin
// projection of financial/reconciliation.Check
// (CheckBalanceSheetBalances), not a re-implementation of the check
// itself — see reconcileBalanceSheet, which reuses
// financial/reconciliation.Run directly per the task's explicit "reuse
// financial/reconciliation logic if the semantics match exactly"
// instruction (they do: reconciliation's own contra-asset handling for
// financial.CodeBsAccumDepreciation is exactly the semantics this
// package's own contra-account support needs to agree with).
type BalanceSheetReconciliation struct {
	Period      financial.Period `json:"period"`
	Assets      float64          `json:"assets"`
	Liabilities float64          `json:"liabilities"`
	Equity      float64          `json:"equity"`
	Balanced    bool             `json:"balanced"`
	Difference  float64          `json:"difference"`
	Tolerance   float64          `json:"tolerance"`
	// Status carries reconciliation.Status (PASS/WARNING/FAIL/
	// NOT_APPLICABLE) verbatim, as a plain string so this package's public
	// API does not need to import reconciliation.Status into its own type
	// — consistent with review.ReconciliationPayload's identical choice
	// (see review/types.go).
	Status string `json:"status"`
}

// reconcileBalanceSheet runs financial/reconciliation.Run against the
// built dataset and projects CheckBalanceSheetBalances into
// BalanceSheetReconciliation, one per period, plus IssueUnbalancedBalance
// Sheet for any period reconciliation reports as FAIL — never forcing
// balance, exactly reporting what was computed (task section 25's
// explicit "do not force equality" rule).
func reconcileBalanceSheet(dataset financial.FinancialDataset, periods []financial.Period, opts Options) ([]BalanceSheetReconciliation, []Issue) {
	reconOpts := reconciliation.Options{Tolerance: opts.BalanceTolerance}
	result := reconciliation.Run(dataset, reconOpts)

	checkByPeriod := make(map[financial.Period]reconciliation.Check, len(result.Checks))
	for _, chk := range result.Checks {
		if chk.Code == reconciliation.CheckBalanceSheetBalances {
			checkByPeriod[chk.Period] = chk
		}
	}

	var out []BalanceSheetReconciliation
	var issues []Issue
	for _, p := range periods {
		chk, ok := checkByPeriod[p]
		if !ok || chk.Status == reconciliation.StatusNotApplicable {
			continue
		}

		assets := totalForPeriodByCodes(dataset, p, currentAssetCodes, nonCurrentAssetCodes)
		liabilities := totalForPeriodByCodes(dataset, p, currentLiabilityCodes, nonCurrentLiabilityCodes)
		equity := totalForPeriodByCodes(dataset, p, equityCodes, nil)

		diff := 0.0
		if chk.Difference != nil {
			diff = *chk.Difference
		}
		tol := chk.Tolerance.Absolute

		br := BalanceSheetReconciliation{
			Period:      p,
			Assets:      assets,
			Liabilities: liabilities,
			Equity:      equity,
			Balanced:    chk.Status == reconciliation.StatusPass,
			Difference:  diff,
			Tolerance:   tol,
			Status:      string(chk.Status),
		}
		out = append(out, br)

		if chk.Status == reconciliation.StatusFail {
			issues = append(issues, Issue{
				Code:     IssueUnbalancedBalanceSheet,
				Severity: SeverityError,
				Message:  "balance sheet for period " + string(p) + " does not balance within tolerance",
				Period:   string(p),
			})
		}
	}

	return out, issues
}

// totalForPeriodByCodes sums dataset items in period matching any code in
// codeSets, for reconciliation display purposes (BalanceSheetReconciliation's
// own Assets/Liabilities/Equity fields) — independent of
// reconciliation.Run's own internal totals, which are not exposed on
// Check, so this package recomputes the same classification it already
// uses in balance.go's sumBalanceSheetTotals to keep the two figures in
// exact agreement.
func totalForPeriodByCodes(dataset financial.FinancialDataset, period financial.Period, codeSets ...map[financial.Code]bool) float64 {
	var sum float64
	for _, item := range dataset.Items {
		if item.Period != period {
			continue
		}
		for _, set := range codeSets {
			if set != nil && set[item.Code] {
				sum += item.Amount
				break
			}
		}
	}
	return sum
}
