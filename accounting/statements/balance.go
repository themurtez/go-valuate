package statements

import (
	"github.com/themurtez/go-valuate/accounting/ledger"
	"github.com/themurtez/go-valuate/financial"
)

// Balance-sheet codes are split into fixed current/non-current buckets
// here (financial.CodeCategory alone only distinguishes
// CategoryBalanceSheet as a whole — see financial/taxonomy.go — so this
// package supplies its own finer split, exactly the way
// financial/reconciliation and financial/metrics already independently
// classify current vs. total balance-sheet codes for their own purposes).
var currentAssetCodes = map[financial.Code]bool{
	financial.CodeBsCash:               true,
	financial.CodeBsAccountsReceivable: true,
	financial.CodeBsInventory:          true,
	financial.CodeBsPrepaid:            true,
	financial.CodeBsCurrentAssetOther:  true,
}

var nonCurrentAssetCodes = map[financial.Code]bool{
	financial.CodeBsFixedAssets:       true,
	financial.CodeBsAccumDepreciation: true, // contra-asset; positive magnitude, subtracted in totals — see signs.go's SignInvert
	financial.CodeBsIntangibleAssets:  true,
	financial.CodeBsGoodwill:          true,
}

var currentLiabilityCodes = map[financial.Code]bool{
	financial.CodeBsAccountsPayable:       true,
	financial.CodeBsCurrentLiabilityOther: true,
	financial.CodeBsShortTermDebt:         true,
}

var nonCurrentLiabilityCodes = map[financial.Code]bool{
	financial.CodeBsLongTermDebt: true,
}

var equityCodes = map[financial.Code]bool{
	financial.CodeBsRetainedEarnings: true,
	financial.CodeBsOwnerEquity:      true,
}

// balanceSheetSectionOrder is the fixed, deterministic section ordering
// for a balance sheet — task section 30's example template.
var balanceSheetSectionOrder = []string{
	"Current Assets",
	"Non-Current Assets",
	"Current Liabilities",
	"Non-Current Liabilities",
	"Equity",
}

func balanceSheetSectionFor(code financial.Code) string {
	switch {
	case currentAssetCodes[code]:
		return "Current Assets"
	case nonCurrentAssetCodes[code]:
		return "Non-Current Assets"
	case currentLiabilityCodes[code]:
		return "Current Liabilities"
	case nonCurrentLiabilityCodes[code]:
		return "Non-Current Liabilities"
	case equityCodes[code]:
		return "Equity"
	default:
		return ""
	}
}

// buildBalanceSheet builds the balance sheet Statement model, with
// calculated structural totals (Total Assets, Total Liabilities, Total
// Equity, Total Liabilities & Equity). financial/metrics.Snapshot has no
// TotalAssets/TotalLiabilities/TotalEquity fields at all (see
// financial/metrics/types.go — it exposes CurrentAssets/
// CurrentLiabilities/WorkingCapital, not full balance-sheet totals), so
// there is no existing formula to defer to for these four figures; this
// package sums the mapped mappedAccountFigures directly instead
// (sumBalanceSheetTotals below), which is a direct sum with no
// independently-invented arithmetic, consistent with task section 26's
// "use existing metrics formulas... if taxonomy structure differs"
// instruction.
func buildBalanceSheet(
	chart ledger.ChartOfAccounts,
	balances []resolvedBalance,
	mappingResults []AccountMappingResult,
	periods []financial.Period,
	opts Options,
) (*Statement, DatasetAvailability) {
	figures := aggregateMappedAccounts(chart, balances, mappingResults, financial.StatementBalanceSheet)
	if len(figures) == 0 {
		return nil, AvailabilityUnavailable
	}

	sectionRows := make(map[string][]Row)
	for _, f := range figures {
		meta, ok := financial.LookupCode(f.code)
		if !ok {
			continue
		}
		sectionName := balanceSheetSectionFor(f.code)
		if sectionName == "" {
			continue
		}
		sectionRows[sectionName] = append(sectionRows[sectionName], mappedFiguresToRow(meta, f))
	}

	totals := sumBalanceSheetTotals(figures, periods)

	sections := make([]Section, 0, len(balanceSheetSectionOrder)+4)
	for _, name := range balanceSheetSectionOrder {
		rows := sectionRows[name]
		if len(rows) == 0 {
			continue
		}
		sections = append(sections, Section{Name: name, Rows: rows})
	}

	sections = append(sections,
		totalRowSection("Total Assets", periods, totals.assets),
		totalRowSection("Total Liabilities", periods, totals.liabilities),
		totalRowSection("Total Equity", periods, totals.equity),
		totalRowSection("Total Liabilities & Equity", periods, totals.liabilitiesAndEquity),
	)

	stmt := &Statement{
		StatementType: financial.StatementBalanceSheet,
		Periods:       periods,
		Sections:      sections,
	}
	return stmt, AvailabilityBuilt
}

// balanceSheetTotals holds the four calculated structural bottom lines
// per period.
type balanceSheetTotals struct {
	assets               map[financial.Period]float64
	liabilities          map[financial.Period]float64
	equity               map[financial.Period]float64
	liabilitiesAndEquity map[financial.Period]float64
}

// sumBalanceSheetTotals sums figures' canonical (already sign-normalized,
// positive-magnitude) amounts into Total Assets/Liabilities/Equity per
// period. Accumulation follows figures' own deterministic (code-sorted)
// order, never Go map order, matching this package's determinism
// guarantee.
func sumBalanceSheetTotals(figures []mappedAccountFigures, periods []financial.Period) balanceSheetTotals {
	totals := balanceSheetTotals{
		assets:               make(map[financial.Period]float64),
		liabilities:          make(map[financial.Period]float64),
		equity:               make(map[financial.Period]float64),
		liabilitiesAndEquity: make(map[financial.Period]float64),
	}

	for _, f := range figures {
		section := balanceSheetSectionFor(f.code)
		for _, p := range periods {
			v, ok := f.valuesByPeriod[p]
			if !ok {
				continue
			}
			switch section {
			case "Current Assets", "Non-Current Assets":
				totals.assets[p] += v
			case "Current Liabilities", "Non-Current Liabilities":
				totals.liabilities[p] += v
				totals.liabilitiesAndEquity[p] += v
			case "Equity":
				totals.equity[p] += v
				totals.liabilitiesAndEquity[p] += v
			}
		}
	}

	return totals
}

// totalRowSection wraps one calculated total figure in its own unnamed
// Section holding a single financial.RowKindTotal Row.
func totalRowSection(label string, periods []financial.Period, values map[financial.Period]float64) Section {
	row := Row{Label: label, Kind: financial.RowKindTotal, Values: make(map[financial.Period]float64, len(periods))}
	for _, p := range periods {
		if v, ok := values[p]; ok {
			row.Values[p] = v
		}
	}
	return Section{Rows: []Row{row}}
}
