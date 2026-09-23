// Package synthetic provides one internally-consistent synthetic business
// ("Meridian SaaS") with five fiscal years of data plus every caller-only
// input (customer revenue, budget, forecast assumptions, debt, working
// capital detail, confirmed adjustments, valuation inputs, benchmark peer
// data, covenant tests, and acquisition/deal-structure assumptions) needed
// to exercise most of this module's analytics packages end to end.
//
// This package is deliberately data-only: it builds typed Go values (never
// calls Calculate itself) so both package-local tests and the suite-level
// smoke test in analytics/smoketest can import it and drive their own
// Calculate calls. Every figure is synthetic and redistributable — no
// proprietary or customer data of any kind.
//
// Each Build* function returns a fresh value on every call (no shared
// package-level mutable state, no caching), so concurrent callers and
// repeated test runs never observe cross-call mutation.
package synthetic

import (
	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/metrics"
)

// Periods is Meridian SaaS's five fiscal years, oldest first.
var Periods = []financial.Period{"2021", "2022", "2023", "2024", "2025"}

// annualFigures holds one fiscal year's income-statement and balance-sheet
// line items for Meridian SaaS, before being flattened into NormalizedItems.
type annualFigures struct {
	period financial.Period

	revRecurring float64
	revOther     float64
	cogsOther    float64

	opexPayroll          float64
	opexMarketing        float64
	opexSoftware         float64
	opexProfessionalFees float64
	opexOffice           float64
	opexOther            float64
	opexOwnerComp        float64

	depreciation    float64
	amortization    float64
	interestExpense float64
	interestIncome  float64
	otherIncome     float64
	otherExpense    float64

	bsAR                float64
	bsPrepaid           float64
	bsFixedAssets       float64
	bsAccumDepreciation float64
	bsIntangibleAssets  float64
	bsAP                float64
	bsCurrentLiabOther  float64
	bsShortTermDebt     float64
	bsLongTermDebt      float64
}

// taxRate is the flat effective tax rate applied to pretax income to derive
// each year's INCOME_TAX line item and retained-earnings roll-forward.
const taxRate = 0.24

// startingRetainedDeficit and startingOwnerEquity are Meridian SaaS's
// balance-sheet starting point immediately before fiscal year 2021 — a
// young company with an accumulated deficit from its pre-2021 build-out
// years, funded by founder/investor paid-in capital. bsOwnerEquity is held
// constant across all five years (no new capital raises modeled);
// bsRetainedEarnings rolls forward year over year from each year's derived
// net income; bsCash is the balancing plug, so the balance sheet balances
// exactly by construction for every year without hand-maintaining it.
const (
	startingRetainedDeficit = -900_000
	startingOwnerEquity     = 2_400_000
)

// meridianYears is Meridian SaaS's five fiscal years of history: steady
// ~28-35% recurring-revenue growth, gradually improving margins, one
// deliberate one-time cost (2023 rebranding, folded into OPEX_OTHER) and one
// deliberate margin dip (2023) so QoE/anomaly/variance modules have a real
// signal to detect. Each year's balance sheet balances exactly by
// construction (assets == liabilities + equity), mirroring the existing
// fixtures/normalized_*_multi_year.json convention.
var meridianYears = []annualFigures{
	{
		period:       "2021",
		revRecurring: 3_100_000, revOther: 62_000, cogsOther: 465_000,
		opexPayroll: 1_450_000, opexMarketing: 260_000, opexSoftware: 108_000,
		opexProfessionalFees: 52_000, opexOffice: 34_000, opexOther: 6_000, opexOwnerComp: 0,
		depreciation: 28_000, amortization: 40_000, interestExpense: 22_000, interestIncome: 4_100,
		otherIncome: 0, otherExpense: 0,
		bsAR: 240_000, bsPrepaid: 30_000,
		bsFixedAssets: 180_000, bsAccumDepreciation: 60_000, bsIntangibleAssets: 260_000,
		bsAP: 118_000, bsCurrentLiabOther: 52_000, bsShortTermDebt: 80_000, bsLongTermDebt: 420_000,
	},
	{
		period:       "2022",
		revRecurring: 4_030_000, revOther: 74_000, cogsOther: 585_000,
		opexPayroll: 1_780_000, opexMarketing: 330_000, opexSoftware: 138_000,
		opexProfessionalFees: 61_000, opexOffice: 39_000, opexOther: 7_500, opexOwnerComp: 0,
		depreciation: 33_000, amortization: 46_000, interestExpense: 34_000, interestIncome: 6_800,
		otherIncome: 0, otherExpense: 0,
		bsAR: 315_000, bsPrepaid: 36_000,
		bsFixedAssets: 225_000, bsAccumDepreciation: 93_000, bsIntangibleAssets: 300_000,
		bsAP: 148_000, bsCurrentLiabOther: 64_000, bsShortTermDebt: 90_000, bsLongTermDebt: 560_000,
	},
	{
		// Deliberate margin dip: one-time rebranding cost inflates
		// OPEX_OTHER and payroll growth outpaces revenue growth for one
		// year — this is the year QoE/anomaly detection is expected to
		// flag, and variance/forecast fixtures compare actuals against.
		period:       "2023",
		revRecurring: 5_140_000, revOther: 81_000, cogsOther: 760_000,
		opexPayroll: 2_430_000, opexMarketing: 410_000, opexSoftware: 176_000,
		opexProfessionalFees: 74_000, opexOffice: 45_000, opexOther: 145_000, opexOwnerComp: 0,
		depreciation: 39_000, amortization: 52_000, interestExpense: 48_000, interestIncome: 9_200,
		otherIncome: 0, otherExpense: 0,
		bsAR: 412_000, bsPrepaid: 44_000,
		bsFixedAssets: 268_000, bsAccumDepreciation: 132_000, bsIntangibleAssets: 340_000,
		bsAP: 176_000, bsCurrentLiabOther: 71_000, bsShortTermDebt: 95_000, bsLongTermDebt: 690_000,
	},
	{
		period:       "2024",
		revRecurring: 6_730_000, revOther: 96_000, cogsOther: 965_000,
		opexPayroll: 2_960_000, opexMarketing: 495_000, opexSoftware: 221_000,
		opexProfessionalFees: 86_000, opexOffice: 52_000, opexOther: 9_800, opexOwnerComp: 0,
		depreciation: 46_000, amortization: 58_000, interestExpense: 61_000, interestIncome: 12_400,
		otherIncome: 0, otherExpense: 0,
		bsAR: 528_000, bsPrepaid: 53_000,
		bsFixedAssets: 312_000, bsAccumDepreciation: 178_000, bsIntangibleAssets: 380_000,
		bsAP: 214_000, bsCurrentLiabOther: 82_000, bsShortTermDebt: 100_000, bsLongTermDebt: 810_000,
	},
	{
		period:       "2025",
		revRecurring: 8_610_000, revOther: 114_000, cogsOther: 1_210_000,
		opexPayroll: 3_620_000, opexMarketing: 592_000, opexSoftware: 274_000,
		opexProfessionalFees: 101_000, opexOffice: 61_000, opexOther: 11_400, opexOwnerComp: 0,
		depreciation: 54_000, amortization: 65_000, interestExpense: 76_000, interestIncome: 16_800,
		otherIncome: 0, otherExpense: 0,
		bsAR: 672_000, bsPrepaid: 64_000,
		bsFixedAssets: 361_000, bsAccumDepreciation: 232_000, bsIntangibleAssets: 420_000,
		bsAP: 261_000, bsCurrentLiabOther: 96_000, bsShortTermDebt: 110_000, bsLongTermDebt: 940_000,
	},
}

// BuildDataset returns Meridian SaaS's five-year normalized
// financial.FinancialDataset. Items are sorted by Code then Period, matching
// FinancialDataset.Items' documented ordering contract. INCOME_TAX,
// BS_RETAINED_EARNINGS, BS_OWNER_EQUITY, and BS_CASH are all derived here
// (tax from pretax income at taxRate; retained earnings rolled forward from
// each year's net income; cash as the balancing plug) rather than
// hand-maintained, so the balance sheet is guaranteed to balance exactly
// for every year without needing to re-derive it by hand after an edit to
// any P&L or other balance-sheet figure above.
func BuildDataset() financial.FinancialDataset {
	var items []financial.NormalizedItem
	add := func(code financial.Code, period financial.Period, amount float64) {
		items = append(items, financial.NormalizedItem{Code: code, Period: period, Amount: amount})
	}

	cumRE := float64(startingRetainedDeficit)
	for _, y := range meridianYears {
		revenue := y.revRecurring + y.revOther
		opex := y.opexPayroll + y.opexMarketing + y.opexSoftware + y.opexProfessionalFees + y.opexOffice + y.opexOther + y.opexOwnerComp
		ebit := revenue - y.cogsOther - opex - y.depreciation - y.amortization
		pretax := ebit - y.interestExpense + y.interestIncome + y.otherIncome - y.otherExpense
		incomeTax := 0.0
		if pretax > 0 {
			incomeTax = pretax * taxRate
		}
		netIncome := pretax - incomeTax
		cumRE += netIncome

		nonCashAssets := y.bsAR + y.bsPrepaid + (y.bsFixedAssets - y.bsAccumDepreciation) + y.bsIntangibleAssets
		liabilities := y.bsAP + y.bsCurrentLiabOther + y.bsShortTermDebt + y.bsLongTermDebt
		cash := liabilities + cumRE + startingOwnerEquity - nonCashAssets

		add(financial.CodeRevRecurring, y.period, y.revRecurring)
		add(financial.CodeRevOther, y.period, y.revOther)
		add(financial.CodeCogsOther, y.period, y.cogsOther)
		add(financial.CodeOpexPayroll, y.period, y.opexPayroll)
		add(financial.CodeOpexMarketing, y.period, y.opexMarketing)
		add(financial.CodeOpexSoftware, y.period, y.opexSoftware)
		add(financial.CodeOpexProfessionalFees, y.period, y.opexProfessionalFees)
		add(financial.CodeOpexOffice, y.period, y.opexOffice)
		add(financial.CodeOpexOther, y.period, y.opexOther)
		add(financial.CodeDepreciation, y.period, y.depreciation)
		add(financial.CodeAmortization, y.period, y.amortization)
		add(financial.CodeInterestExpense, y.period, y.interestExpense)
		add(financial.CodeInterestIncome, y.period, y.interestIncome)
		add(financial.CodeIncomeTax, y.period, incomeTax)
		add(financial.CodeBsCash, y.period, cash)
		add(financial.CodeBsAccountsReceivable, y.period, y.bsAR)
		add(financial.CodeBsPrepaid, y.period, y.bsPrepaid)
		add(financial.CodeBsFixedAssets, y.period, y.bsFixedAssets)
		add(financial.CodeBsAccumDepreciation, y.period, y.bsAccumDepreciation)
		add(financial.CodeBsIntangibleAssets, y.period, y.bsIntangibleAssets)
		add(financial.CodeBsAccountsPayable, y.period, y.bsAP)
		add(financial.CodeBsCurrentLiabilityOther, y.period, y.bsCurrentLiabOther)
		add(financial.CodeBsShortTermDebt, y.period, y.bsShortTermDebt)
		add(financial.CodeBsLongTermDebt, y.period, y.bsLongTermDebt)
		add(financial.CodeBsRetainedEarnings, y.period, cumRE)
		add(financial.CodeBsOwnerEquity, y.period, float64(startingOwnerEquity))
	}
	sortItems(items)
	return financial.FinancialDataset{Currency: "USD", Items: items}
}

func sortItems(items []financial.NormalizedItem) {
	for i := 1; i < len(items); i++ {
		for j := i; j > 0; j-- {
			a, b := items[j-1], items[j]
			if a.Code > b.Code || (a.Code == b.Code && a.Period > b.Period) {
				items[j-1], items[j] = items[j], items[j-1]
			} else {
				break
			}
		}
	}
}

// BuildPeriodMeta returns fiscal-year PeriodInfo for all five Periods,
// consumed directly by every package taking a metrics.PeriodInfo map.
func BuildPeriodMeta() map[financial.Period]metrics.PeriodInfo {
	return map[financial.Period]metrics.PeriodInfo{
		"2021": {Type: metrics.PeriodTypeFiscalYear, FiscalYear: 2021},
		"2022": {Type: metrics.PeriodTypeFiscalYear, FiscalYear: 2022},
		"2023": {Type: metrics.PeriodTypeFiscalYear, FiscalYear: 2023},
		"2024": {Type: metrics.PeriodTypeFiscalYear, FiscalYear: 2024},
		"2025": {Type: metrics.PeriodTypeFiscalYear, FiscalYear: 2025},
	}
}
