package anomalies

import "github.com/themurtez/go-valuate/financial"

// diffAbs returns the absolute difference between a and b, for
// tolerance-based float64 comparisons in tests (float64 arithmetic order
// can produce last-bit differences from a hand-computed expected value) —
// mirroring analytics/concentration/fixtures_test.go's identical helper.
func diffAbs(a, b float64) float64 {
	d := a - b
	if d < 0 {
		return -d
	}
	return d
}

// fourYearMeta builds a PeriodInfo map for four consecutive fiscal years
// "2022"-"2025", mirroring analytics/concentration/fixtures_test.go's
// threeYearMeta pattern extended by one year (several detection rules here
// need three prior periods plus a current one, e.g. RuleRepeatedUnusualValue's
// default three-occurrence threshold).
func fourYearMeta() map[financial.Period]PeriodInfo {
	return map[financial.Period]PeriodInfo{
		"2022": {Type: PeriodTypeFiscalYear, FiscalYear: 2022},
		"2023": {Type: PeriodTypeFiscalYear, FiscalYear: 2023},
		"2024": {Type: PeriodTypeFiscalYear, FiscalYear: 2024},
		"2025": {Type: PeriodTypeFiscalYear, FiscalYear: 2025},
	}
}

// item is a terse constructor for a financial.NormalizedItem, for test
// readability.
func item(code financial.Code, period string, amount float64) financial.NormalizedItem {
	return financial.NormalizedItem{Code: code, Period: financial.Period(period), Amount: amount}
}

// datasetOf builds a financial.FinancialDataset from a flat list of items.
func datasetOf(items ...financial.NormalizedItem) financial.FinancialDataset {
	return financial.FinancialDataset{Currency: "USD", Items: items}
}

// stableDataset models a business with flat, unremarkable revenue/expenses
// across four years — no anomaly in this package's rule set should fire
// against it. Every account's amount changes by a small, steady amount each
// year (revenue/COGS/payroll well under the default $10,000
// AbsoluteAmountSpike floor; gross margin moves less than 1 point per year,
// well under the default 10-point MarginDeteriorationPoints; COGS/OPEX each
// grow slower than revenue's ~0.4%/year, so none can clear the default
// 20-point ExpenseOutpacingRevenueGap).
func stableDataset() financial.FinancialDataset {
	var items []financial.NormalizedItem
	years := []string{"2022", "2023", "2024", "2025"}
	revenue := 1_000_000.0
	cogs := 400_000.0
	payroll := 250_000.0
	rent := 60_000.0
	marketing := 40_000.0
	for _, y := range years {
		items = append(items,
			item(financial.CodeRevProduct, y, revenue),
			item(financial.CodeCogsMaterial, y, cogs),
			item(financial.CodeOpexPayroll, y, payroll),
			item(financial.CodeOpexRent, y, rent),
			item(financial.CodeOpexMarketing, y, marketing),
		)
		revenue += 4_000
		cogs += 1_500
		payroll += 800
		rent += 200
		marketing += 200
	}
	return datasetOf(items...)
}

// spikeDataset is stableDataset with CodeOpexMarketing spiking from $40,000
// in 2024 to $200,000 in 2025 — well past both AbsoluteAmountSpike (10,000)
// and PercentageChangeSpike (50%) defaults.
func spikeDataset() financial.FinancialDataset {
	ds := stableDataset()
	for i, it := range ds.Items {
		if it.Code == financial.CodeOpexMarketing && it.Period == "2025" {
			ds.Items[i].Amount = 200_000
		}
	}
	return ds
}

// revenueLinkedExpenseGrowthDataset models revenue growing modestly (5%)
// from 2024 to 2025 while CodeOpexMarketing grows much faster (60%) —
// triggers RuleExpenseOutpacingRevenue (default gap threshold 20 points; the
// 55-point gap here clears it) without necessarily clearing
// RulePercentageChangeSpike's independent 50% single-account threshold
// coincidentally (it does here too, which is fine — both rules are allowed
// to fire on the same underlying data point; they detect different things).
func revenueLinkedExpenseGrowthDataset() financial.FinancialDataset {
	return datasetOf(
		item(financial.CodeRevProduct, "2024", 1_000_000),
		item(financial.CodeCogsMaterial, "2024", 400_000),
		item(financial.CodeOpexMarketing, "2024", 100_000),

		item(financial.CodeRevProduct, "2025", 1_050_000),
		item(financial.CodeCogsMaterial, "2025", 420_000),
		item(financial.CodeOpexMarketing, "2025", 160_000),
	)
}

// marginLeakDataset models flat revenue and flat OPEX but COGS creeping up
// enough to erode gross margin by more than 10 points from 2024 to 2025.
func marginLeakDataset() financial.FinancialDataset {
	return datasetOf(
		item(financial.CodeRevProduct, "2024", 1_000_000),
		item(financial.CodeCogsMaterial, "2024", 400_000), // 60% gross margin
		item(financial.CodeOpexPayroll, "2024", 300_000),

		item(financial.CodeRevProduct, "2025", 1_000_000),
		item(financial.CodeCogsMaterial, "2025", 550_000), // 45% gross margin: -15 points
		item(financial.CodeOpexPayroll, "2025", 300_000),
	)
}

// signFlipDataset models CodeOtherIncome flipping from a $50,000 positive
// balance in 2024 to a -$30,000 balance in 2025 — both magnitudes clear the
// default SignFlipMinMagnitude (100).
func signFlipDataset() financial.FinancialDataset {
	return datasetOf(
		item(financial.CodeRevProduct, "2024", 1_000_000),
		item(financial.CodeOtherIncome, "2024", 50_000),

		item(financial.CodeRevProduct, "2025", 1_000_000),
		item(financial.CodeOtherIncome, "2025", -30_000),
	)
}

// immaterialChangeDataset models small, steady changes across every account
// that stay strictly under every DefaultThresholds trigger — used to prove
// small/immaterial changes are not flagged. Distinct from stableDataset
// (which is designed to be unremarkable across the FULL rule set including
// materiality-based ones) by being a minimal two-period dataset focused
// purely on the spike-family thresholds.
func immaterialChangeDataset() financial.FinancialDataset {
	return datasetOf(
		item(financial.CodeRevProduct, "2024", 1_000_000),
		item(financial.CodeOpexMarketing, "2024", 40_000),

		item(financial.CodeRevProduct, "2025", 1_005_000),
		item(financial.CodeOpexMarketing, "2025", 41_500), // +3.75%, +1,500 — under both spike thresholds
	)
}

// missingPeriodDataset has CodeOpexSoftware present in 2022 and 2023,
// absent in 2024 and 2025 (COGS/OPEX so RuleAccountDisappearedReappeared
// exercises against it), and CodeOpexTravel absent until 2025, when it
// reports a material new amount.
func missingPeriodDataset() financial.FinancialDataset {
	return datasetOf(
		item(financial.CodeRevProduct, "2022", 1_000_000),
		item(financial.CodeOpexSoftware, "2022", 12_000),

		item(financial.CodeRevProduct, "2023", 1_000_000),
		item(financial.CodeOpexSoftware, "2023", 13_000),

		item(financial.CodeRevProduct, "2024", 1_000_000),
		// CodeOpexSoftware absent in 2024: disappeared.

		item(financial.CodeRevProduct, "2025", 1_000_000),
		item(financial.CodeOpexTravel, "2025", 25_000), // new material category
	)
}

// reappearedDataset has CodeOpexRepairs present in 2022, absent in 2023 and
// 2024 (a two-period gap), then present again in 2025.
func reappearedDataset() financial.FinancialDataset {
	return datasetOf(
		item(financial.CodeRevProduct, "2022", 1_000_000),
		item(financial.CodeOpexRepairs, "2022", 8_000),

		item(financial.CodeRevProduct, "2023", 1_000_000),
		// CodeOpexRepairs absent.

		item(financial.CodeRevProduct, "2024", 1_000_000),
		// CodeOpexRepairs absent.

		item(financial.CodeRevProduct, "2025", 1_000_000),
		item(financial.CodeOpexRepairs, "2025", 9_500),
	)
}

// repeatedValueDataset has CodeOpexInsurance reporting the exact same
// $6,000.00 amount in three of four years (2022, 2023, 2025), varying only
// in 2024 — triggers RuleRepeatedUnusualValue at the default
// three-occurrence threshold.
func repeatedValueDataset() financial.FinancialDataset {
	return datasetOf(
		item(financial.CodeOpexInsurance, "2022", 6_000),
		item(financial.CodeOpexInsurance, "2023", 6_000),
		item(financial.CodeOpexInsurance, "2024", 6_450),
		item(financial.CodeOpexInsurance, "2025", 6_000),
	)
}

// duplicateAmountsDataset has CodeOpexMarketing (2025) and CodeOpexTravel
// (2024) both reporting the exact same $17,250.00 amount — two distinct
// suspicious accounts, one repeated value — triggers RuleDuplicateLikeAmounts.
func duplicateAmountsDataset() financial.FinancialDataset {
	return datasetOf(
		item(financial.CodeRevProduct, "2024", 1_000_000),
		item(financial.CodeOpexTravel, "2024", 17_250),

		item(financial.CodeRevProduct, "2025", 1_000_000),
		item(financial.CodeOpexMarketing, "2025", 17_250),
	)
}

// ownerDiscretionaryDataset has CodeOpexOwnerComp at 20% of Total Revenue in
// the most recent period, exceeding the default 15% threshold.
func ownerDiscretionaryDataset() financial.FinancialDataset {
	return datasetOf(
		item(financial.CodeRevProduct, "2024", 1_000_000),
		item(financial.CodeOpexOwnerComp, "2024", 100_000),

		item(financial.CodeRevProduct, "2025", 1_000_000),
		item(financial.CodeOpexOwnerComp, "2025", 200_000),
	)
}

// unexpectedNegativeDataset has CodeRevProduct reporting a negative amount —
// a structural anomaly independent of any period comparison.
func unexpectedNegativeDataset() financial.FinancialDataset {
	return datasetOf(
		item(financial.CodeRevProduct, "2025", -15_000),
		item(financial.CodeOpexRent, "2025", -500),
	)
}
