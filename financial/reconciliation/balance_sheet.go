package reconciliation

import (
	"fmt"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/metrics"
)

// balanceSheetCodes are every canonical code that belongs to the balance
// sheet section of the taxonomy, used to detect whether a period has any
// balance sheet data at all.
var balanceSheetCodes = []financial.Code{
	financial.CodeBsCash,
	financial.CodeBsAccountsReceivable,
	financial.CodeBsInventory,
	financial.CodeBsPrepaid,
	financial.CodeBsCurrentAssetOther,
	financial.CodeBsFixedAssets,
	financial.CodeBsAccumDepreciation,
	financial.CodeBsIntangibleAssets,
	financial.CodeBsGoodwill,
	financial.CodeBsAccountsPayable,
	financial.CodeBsCurrentLiabilityOther,
	financial.CodeBsShortTermDebt,
	financial.CodeBsLongTermDebt,
	financial.CodeBsRetainedEarnings,
	financial.CodeBsOwnerEquity,
}

var assetCodes = []financial.Code{
	financial.CodeBsCash,
	financial.CodeBsAccountsReceivable,
	financial.CodeBsInventory,
	financial.CodeBsPrepaid,
	financial.CodeBsCurrentAssetOther,
	financial.CodeBsFixedAssets,
	financial.CodeBsAccumDepreciation, // contra-asset; see sumSigned below
	financial.CodeBsIntangibleAssets,
	financial.CodeBsGoodwill,
}

var liabilityAndEquityCodes = []financial.Code{
	financial.CodeBsAccountsPayable,
	financial.CodeBsCurrentLiabilityOther,
	financial.CodeBsShortTermDebt,
	financial.CodeBsLongTermDebt,
	financial.CodeBsRetainedEarnings,
	financial.CodeBsOwnerEquity,
}

// checkBalanceSheetBalances verifies Assets ≈ Liabilities + Equity for one
// period, using the raw dataset directly (not the metrics package) since it
// needs the true total of every balance sheet code rather than a specific
// derived subset.
//
// Returns StatusNotApplicable if the dataset has no balance sheet codes at
// all for period — this is not itself a problem (an income-statement-only
// dataset is valid input), so it must not be reported as a FAIL.
//
// financial.CodeBsAccumDepreciation is a contra-asset: it is stored as a
// positive magnitude (see the metrics package's sign-convention note) but
// reduces total assets, so it is subtracted here rather than added.
func checkBalanceSheetBalances(dataset financial.FinancialDataset, period financial.Period, tol Tolerance) Check {
	check := Check{
		Code:         CheckBalanceSheetBalances,
		Period:       period,
		Tolerance:    tol,
		RelatedCodes: codeStrings(balanceSheetCodes),
	}

	anyPresent := false
	for _, code := range balanceSheetCodes {
		if _, ok := dataset.ByCodeAndPeriod(code, period); ok {
			anyPresent = true
			break
		}
	}
	if !anyPresent {
		check.Status = StatusNotApplicable
		check.Explanation = "no balance sheet data is present for this period"
		return check
	}

	totalAssets := 0.0
	for _, code := range assetCodes {
		item, ok := dataset.ByCodeAndPeriod(code, period)
		if !ok {
			continue
		}
		if code == financial.CodeBsAccumDepreciation {
			totalAssets -= item.Amount
			continue
		}
		totalAssets += item.Amount
	}

	totalLiabEquity := 0.0
	for _, code := range liabilityAndEquityCodes {
		item, ok := dataset.ByCodeAndPeriod(code, period)
		if !ok {
			continue
		}
		totalLiabEquity += item.Amount
	}

	status, diff := tol.Evaluate(totalLiabEquity, totalAssets)
	check.Expected = &totalLiabEquity
	check.Actual = &totalAssets
	check.Difference = &diff
	check.Status = status
	check.Explanation = explainComparison(status, "Liabilities + Equity", totalLiabEquity, "Assets", totalAssets, diff, tol)
	return check
}

// checkCurrentAssetsSubtotal, checkCurrentLiabilitiesSubtotal,
// checkWorkingCapitalCalculated, and checkDebtTotalsCalculated are
// informational sub-checks: they don't compare against a reported figure
// (there is nothing to reconcile against), they simply surface whether the
// metrics package was able to calculate the corresponding figure at all,
// giving a reviewer visibility into partial balance-sheet data without
// treating an incomplete taxonomy/dataset as an error.

func checkCurrentAssetsSubtotal(snapshot metrics.Snapshot) Check {
	return availabilityCheck(CheckCurrentAssetsSubtotal, snapshot.Period, snapshot.CurrentAssets,
		"current assets", []string{
			string(financial.CodeBsCash), string(financial.CodeBsAccountsReceivable),
			string(financial.CodeBsInventory), string(financial.CodeBsPrepaid),
			string(financial.CodeBsCurrentAssetOther),
		})
}

func checkCurrentLiabilitiesSubtotal(snapshot metrics.Snapshot) Check {
	return availabilityCheck(CheckCurrentLiabilitiesSubtotal, snapshot.Period, snapshot.CurrentLiabilities,
		"current liabilities", []string{
			string(financial.CodeBsAccountsPayable), string(financial.CodeBsCurrentLiabilityOther),
			string(financial.CodeBsShortTermDebt),
		})
}

func checkWorkingCapitalCalculated(snapshot metrics.Snapshot) Check {
	return availabilityCheck(CheckWorkingCapitalCalculated, snapshot.Period, snapshot.WorkingCapital,
		"net working capital", []string{metrics.MetricCurrentAssets, metrics.MetricCurrentLiabilities})
}

func checkDebtTotalsCalculated(snapshot metrics.Snapshot) Check {
	return availabilityCheck(CheckDebtTotalsCalculated, snapshot.Period, snapshot.TotalDebt,
		"total debt", []string{string(financial.CodeBsShortTermDebt), string(financial.CodeBsLongTermDebt)})
}

// availabilityCheck builds a Check that reports whether a metrics-derived
// figure was calculable, with no numeric comparison. StatusPass means the
// figure was calculated (of whatever value, including a genuine 0);
// StatusWarning means it could not be calculated from the supplied data,
// which is worth a reviewer's attention but is not a dataset error.
func availabilityCheck(code CheckCode, period financial.Period, value metrics.MetricValue, label string, relatedCodes []string) Check {
	check := Check{
		Code:         code,
		Period:       period,
		RelatedCodes: relatedCodes,
	}
	if value.Available {
		check.Status = StatusPass
		v := value.Value
		check.Actual = &v
		check.Explanation = fmt.Sprintf("%s calculated as %.2f", label, value.Value)
		return check
	}
	check.Status = StatusWarning
	check.Explanation = fmt.Sprintf("%s could not be calculated: no relevant balance sheet codes are present for this period", label)
	return check
}

func codeStrings(codes []financial.Code) []string {
	out := make([]string, len(codes))
	for i, c := range codes {
		out[i] = string(c)
	}
	return out
}
