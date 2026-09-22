package reconciliation

import (
	"fmt"
	"math"
	"sort"

	"github.com/themurtez/go-valuate/financial"
)

// runIntegrityChecks evaluates every structural/integrity check that is not
// specific to income-statement or balance-sheet arithmetic: malformed data,
// suspicious duplication, and coarse structural sanity checks. These
// checks deliberately stay out of business judgement (e.g. they never
// assess whether a margin is "reasonable") — they exist to catch
// structural problems that would make every other check's output
// meaningless.
//
// "Missing period metadata" (fiscal year / period-type information needed
// to determine chronological order) is a metrics.Options.PeriodMeta concern
// — see financial/metrics' period.go — since no check in this package needs
// chronological order: gross profit, balance sheet balance, and every
// integrity check here evaluate one period at a time or the dataset as a
// whole. This package's period-shaped integrity concern is narrower and
// covered by checkNoUnknownPeriodReferences: an empty financial.Period
// string on a NormalizedItem, which is a structural defect regardless of
// ordering.
func runIntegrityChecks(dataset financial.FinancialDataset) []Check {
	var checks []Check
	checks = append(checks, checkDatasetNotEmpty(dataset))
	if len(dataset.Items) == 0 {
		// Every remaining integrity check either needs at least one item to
		// inspect or is scoped per-period; with zero items there are no
		// periods and nothing further to meaningfully evaluate.
		return checks
	}
	checks = append(checks, checkValidCurrency(dataset))
	checks = append(checks, checkFiniteValues(dataset))
	checks = append(checks, checkNoDuplicateEntries(dataset))
	checks = append(checks, checkNoUnknownPeriodReferences(dataset))
	checks = append(checks, checkNoSuspiciousDuplicateSources(dataset))

	for _, period := range dataset.Periods() {
		checks = append(checks, checkIncomeStatementHasRevenue(dataset, period))
		checks = append(checks, checkBalanceSheetHasLiabilitiesOrEquity(dataset, period))
	}

	return checks
}

// checkDatasetNotEmpty flags a dataset with zero NormalizedItems. An empty
// dataset is not necessarily a bug (a caller might legitimately have no
// data yet), but every other check is meaningless against it, so this is
// surfaced as a warning rather than silently producing an all-N/A report.
func checkDatasetNotEmpty(dataset financial.FinancialDataset) Check {
	check := Check{Code: CheckDatasetNotEmpty}
	if len(dataset.Items) == 0 {
		check.Status = StatusWarning
		check.Explanation = "dataset contains no items; no reconciliation or metrics can be computed"
		return check
	}
	check.Status = StatusPass
	check.Explanation = fmt.Sprintf("dataset contains %d items", len(dataset.Items))
	return check
}

// checkValidCurrency flags a missing/empty currency. financial.Normalize
// already requires a non-empty currency to produce a FinancialDataset at
// all, but a dataset could reach this package via any other path (a test
// fixture, a hand-built struct, deserialized JSON that skipped Normalize),
// so this is checked independently rather than assumed.
func checkValidCurrency(dataset financial.FinancialDataset) Check {
	check := Check{Code: CheckValidCurrency}
	if dataset.Currency == "" {
		check.Status = StatusFail
		check.Explanation = "dataset has no currency set"
		return check
	}
	check.Status = StatusPass
	check.Explanation = fmt.Sprintf("currency is %q", dataset.Currency)
	return check
}

// checkFiniteValues flags any NormalizedItem whose Amount is NaN or
// infinite. A non-finite value would silently corrupt every downstream sum
// and comparison, so this is a FAIL, not a warning.
func checkFiniteValues(dataset financial.FinancialDataset) Check {
	check := Check{Code: CheckFiniteValues}
	var offenders []string
	for _, item := range dataset.Items {
		if math.IsNaN(item.Amount) || math.IsInf(item.Amount, 0) {
			offenders = append(offenders, fmt.Sprintf("%s/%s", item.Code, item.Period))
		}
	}
	if len(offenders) > 0 {
		check.Status = StatusFail
		check.Explanation = fmt.Sprintf("%d item(s) have a non-finite amount (NaN or Inf): %v", len(offenders), offenders)
		return check
	}
	check.Status = StatusPass
	check.Explanation = "every item has a finite numeric amount"
	return check
}

// checkNoDuplicateEntries flags multiple NormalizedItem entries sharing the
// same (Code, Period) pair. financial.Normalize's own aggregation makes
// this impossible for output it produced itself (see Normalize's dedup-by-
// summation behavior), but a FinancialDataset reaching this package by any
// other route (hand-assembled, deserialized from an external source, or
// merged from multiple Normalize calls without re-aggregating) could
// violate it, and a duplicate silently makes every downstream lookup via
// FinancialDataset.ByCodeAndPeriod pick an arbitrary one of the two.
func checkNoDuplicateEntries(dataset financial.FinancialDataset) Check {
	check := Check{Code: CheckNoDuplicateEntries}
	type key struct {
		code   financial.Code
		period financial.Period
	}
	seen := make(map[key]int)
	for _, item := range dataset.Items {
		seen[key{item.Code, item.Period}]++
	}
	var dupes []string
	for k, count := range seen {
		if count > 1 {
			dupes = append(dupes, fmt.Sprintf("%s/%s (x%d)", k.code, k.period, count))
		}
	}
	if len(dupes) > 0 {
		sort.Strings(dupes)
		check.Status = StatusFail
		check.Explanation = fmt.Sprintf("%d (code, period) pair(s) have more than one entry, which should have been aggregated by Normalize: %v", len(dupes), dupes)
		return check
	}
	check.Status = StatusPass
	check.Explanation = "no duplicate (code, period) entries found"
	return check
}

// checkNoUnknownPeriodReferences flags any NormalizedItem whose Code is not
// a recognized canonical taxonomy code, or whose Period is empty. An
// "unknown period reference" in the sense this check covers is a
// structurally malformed item (empty period, or a code financial.LookupCode
// doesn't recognize) rather than a business-logic judgement about whether a
// period makes sense.
func checkNoUnknownPeriodReferences(dataset financial.FinancialDataset) Check {
	check := Check{Code: CheckNoUnknownPeriods}
	var problems []string
	for _, item := range dataset.Items {
		switch {
		case item.Period == "":
			problems = append(problems, fmt.Sprintf("code %s has an empty period", item.Code))
		case !financial.IsValidCode(item.Code):
			problems = append(problems, fmt.Sprintf("code %q is not a recognized canonical code (period %s)", item.Code, item.Period))
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		check.Status = StatusFail
		check.Explanation = fmt.Sprintf("%d item(s) have an unrecognized code or missing period: %v", len(problems), problems)
		return check
	}
	check.Status = StatusPass
	check.Explanation = "every item has a recognized code and non-empty period"
	return check
}

// checkNoSuspiciousDuplicateSources flags a NormalizedItem whose Sources
// provenance contains the same RowID more than once for the same Period.
// This would mean a single source row was summed into the same aggregate
// twice — a double-count bug upstream of this package (e.g. in a caller
// that ran Normalize more than once over overlapping input and merged the
// results without re-aggregating) — as opposed to two different rows that
// happen to share a label.
//
// This check is a no-op (always StatusPass) for a dataset built without
// provenance (NormalizeOptions.IncludeProvenance == false), since there is
// nothing to inspect.
func checkNoSuspiciousDuplicateSources(dataset financial.FinancialDataset) Check {
	check := Check{Code: CheckNoSuspiciousDuplicateSources}
	var problems []string
	for _, item := range dataset.Items {
		seen := make(map[string]int)
		for _, ref := range item.Sources {
			seen[ref.RowID]++
		}
		for rowID, count := range seen {
			if count > 1 {
				problems = append(problems, fmt.Sprintf("%s/%s: row %s appears %d times in Sources", item.Code, item.Period, rowID, count))
			}
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		check.Status = StatusWarning
		check.Explanation = fmt.Sprintf("%d item(s) reference the same source row more than once, which may indicate double-counting: %v", len(problems), problems)
		return check
	}
	check.Status = StatusPass
	check.Explanation = "no source row is referenced more than once within the same (code, period) aggregate"
	return check
}

// checkIncomeStatementHasRevenue flags a period that has income-statement
// data (any COGS or OPEX code present) but zero revenue codes present — a
// common sign of an incompletely classified or incompletely extracted
// income statement. A period with genuinely no income-statement data at all
// (e.g. a balance-sheet-only dataset) is StatusNotApplicable, not a
// failure: this check only fires when there is income-statement activity
// but revenue specifically is missing.
func checkIncomeStatementHasRevenue(dataset financial.FinancialDataset, period financial.Period) Check {
	check := Check{Code: CheckIncomeStatementHasRevenue, Period: period}

	hasRevenue := false
	for _, code := range revenueCodesForIntegrity {
		if _, ok := dataset.ByCodeAndPeriod(code, period); ok {
			hasRevenue = true
			break
		}
	}
	if hasRevenue {
		check.Status = StatusPass
		check.Explanation = "at least one revenue code is present for this period"
		return check
	}

	hasOtherIncomeStatementActivity := false
	for _, code := range incomeStatementNonRevenueCodesForIntegrity {
		if _, ok := dataset.ByCodeAndPeriod(code, period); ok {
			hasOtherIncomeStatementActivity = true
			break
		}
	}
	if !hasOtherIncomeStatementActivity {
		check.Status = StatusNotApplicable
		check.Explanation = "no income statement data is present for this period"
		return check
	}

	check.Status = StatusWarning
	check.Explanation = "income statement activity (COGS/OPEX/other) is present for this period but no revenue code has any value"
	return check
}

// checkBalanceSheetHasLiabilitiesOrEquity flags a period that has asset
// codes present but zero liability or equity codes present — the balance
// sheet equivalent of checkIncomeStatementHasRevenue, and a strong signal
// of an incomplete balance sheet even before checkBalanceSheetBalances runs
// its arithmetic comparison.
func checkBalanceSheetHasLiabilitiesOrEquity(dataset financial.FinancialDataset, period financial.Period) Check {
	check := Check{Code: CheckBalanceSheetHasLiabilitiesOrEquity, Period: period}

	hasAssets := false
	for _, code := range assetCodes {
		if _, ok := dataset.ByCodeAndPeriod(code, period); ok {
			hasAssets = true
			break
		}
	}
	if !hasAssets {
		check.Status = StatusNotApplicable
		check.Explanation = "no balance sheet asset data is present for this period"
		return check
	}

	hasLiabOrEquity := false
	for _, code := range liabilityAndEquityCodes {
		if _, ok := dataset.ByCodeAndPeriod(code, period); ok {
			hasLiabOrEquity = true
			break
		}
	}
	if hasLiabOrEquity {
		check.Status = StatusPass
		check.Explanation = "assets are present and at least one liability or equity code is also present"
		return check
	}

	check.Status = StatusWarning
	check.Explanation = "balance sheet has asset data for this period but no liability or equity codes are present"
	return check
}

var revenueCodesForIntegrity = []financial.Code{
	financial.CodeRevProduct,
	financial.CodeRevService,
	financial.CodeRevRecurring,
	financial.CodeRevOther,
}

var incomeStatementNonRevenueCodesForIntegrity = []financial.Code{
	financial.CodeCogsMaterial, financial.CodeCogsDirectLabor, financial.CodeCogsFreight, financial.CodeCogsOther,
	financial.CodeOpexPayroll, financial.CodeOpexOwnerComp, financial.CodeOpexRent, financial.CodeOpexMarketing,
	financial.CodeOpexInsurance, financial.CodeOpexUtilities, financial.CodeOpexSoftware, financial.CodeOpexProfessionalFees,
	financial.CodeOpexRepairs, financial.CodeOpexVehicle, financial.CodeOpexTravel, financial.CodeOpexOffice, financial.CodeOpexOther,
	financial.CodeDepreciation, financial.CodeAmortization, financial.CodeInterestExpense, financial.CodeInterestIncome,
	financial.CodeIncomeTax, financial.CodeOtherIncome, financial.CodeOtherExpense,
}
