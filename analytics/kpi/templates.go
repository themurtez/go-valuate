package kpi

// This file provides a small optional library of built-in KPI templates —
// task section 30. Every template is an ordinary Definition value,
// evaluated by the exact same Calculate/evaluateTyped/evaluateOperator
// code path as any caller-authored Definition; nothing here special-cases
// a formula internally (task section 30's explicit "do not special-case
// formulas internally" / "do not hard-code hundreds of KPIs" rules). Each
// function below is a convenience constructor a caller may use as-is,
// copy, or ignore entirely.
//
// Every template references metric codes under the "financial.*"/
// "labor.*"/"ar.*"/... adapter namespaces described in doc.go's "Adapter
// metric namespaces" section and demonstrated in the *adapter.go files —
// but nothing prevents a caller from supplying MetricValues under
// entirely different codes and building the identical Definition shape by
// hand instead.

// RevenuePerFTETemplate returns Revenue / FTE — task section 30/33.
func RevenuePerFTETemplate() Definition {
	return Definition{
		Code:        "revenue_per_fte",
		Name:        "Revenue per FTE",
		Description: "Total revenue divided by full-time-equivalent headcount.",
		Formula: Binary(OpDivide,
			Metric(MetricRef{Code: "financial.revenue"}),
			Metric(MetricRef{Code: "labor.fte"}),
		),
		// labor.fte's Unit is UnitCount (see laboradapter.go), so
		// Revenue/FTE's actual combined unit (combineDivisive) is
		// UnitCustom "<currency>_PER_COUNT" -- not a made-up
		// "CURRENCY_PER_FTE" label. A caller using a different currency
		// than USD must adjust this to match, since the currency code is
		// baked into the resolved unit's label; see
		// checkExpectedOutputUnit, which requires an exact match.
		Unit:              Unit{Kind: UnitCustom, CustomLabel: "USD_PER_COUNT"},
		Category:          "Labor",
		DefinitionVersion: "1",
	}
}

// LaborCostPercentRevenueTemplate returns LaborCost / Revenue * 100 —
// task section 30/33.
func LaborCostPercentRevenueTemplate() Definition {
	return Definition{
		Code:        "labor_cost_percent_revenue",
		Name:        "Labor Cost % of Revenue",
		Description: "Total labor cost as a percentage of total revenue.",
		Formula: Binary(OpPercent,
			Metric(MetricRef{Code: "labor.total_labor_cost"}),
			Metric(MetricRef{Code: "financial.revenue"}),
		),
		Unit:              Unit{Kind: UnitPercent},
		Category:          "Labor",
		DefinitionVersion: "1",
	}
}

// AROver60PercentTemplate returns AROver60 / TotalAR * 100 — task section
// 30/33.
func AROver60PercentTemplate() Definition {
	return Definition{
		Code:        "ar_over_60_percent",
		Name:        "AR 60+ % of AR",
		Description: "Receivables more than 60 days past due, as a percentage of total open AR.",
		Formula: Binary(OpPercent,
			Metric(MetricRef{Code: "ar.over_60_amount"}),
			Metric(MetricRef{Code: "ar.total_open_ar"}),
		),
		Unit:              Unit{Kind: UnitPercent},
		Category:          "Receivables",
		DefinitionVersion: "1",
	}
}

// AverageTicketTemplate returns Revenue / TransactionCount — task section
// 30/33.
func AverageTicketTemplate() Definition {
	return Definition{
		Code:        "average_ticket",
		Name:        "Average Ticket",
		Description: "Total revenue divided by transaction count.",
		Formula: Binary(OpDivide,
			Metric(MetricRef{Code: "financial.revenue"}),
			Metric(MetricRef{Code: "custom.transaction_count"}),
		),
		// custom.transaction_count is expected UnitCount, so Revenue/
		// TransactionCount resolves to UnitCustom "<currency>_PER_COUNT"
		// (combineDivisive), not plain UnitCurrency -- see
		// RevenuePerFTETemplate's identical note above.
		Unit:              Unit{Kind: UnitCustom, CustomLabel: "USD_PER_COUNT"},
		Category:          "Sales",
		DefinitionVersion: "1",
	}
}

// GrossMarginTemplate returns GrossProfit / Revenue * 100 — task section
// 30/33.
func GrossMarginTemplate() Definition {
	return Definition{
		Code:        "gross_margin",
		Name:        "Gross Margin %",
		Description: "Gross profit as a percentage of revenue.",
		Formula: Binary(OpPercent,
			Metric(MetricRef{Code: "financial.gross_profit"}),
			Metric(MetricRef{Code: "financial.revenue"}),
		),
		Unit:              Unit{Kind: UnitPercent},
		Category:          "Profitability",
		DefinitionVersion: "1",
	}
}

// RevenuePerSquareFootTemplate returns Revenue / SquareFeet — task
// section 30/33.
func RevenuePerSquareFootTemplate() Definition {
	return Definition{
		Code:        "revenue_per_square_foot",
		Name:        "Revenue per Square Foot",
		Description: "Total revenue divided by square footage.",
		Formula: Binary(OpDivide,
			Metric(MetricRef{Code: "financial.revenue"}),
			Metric(MetricRef{Code: "custom.square_feet"}),
		),
		Unit:              Unit{Kind: UnitCustom, CustomLabel: "USD_PER_AREA"},
		Category:          "Real Estate",
		DefinitionVersion: "1",
	}
}

// BuiltInTemplates returns every template this package ships, in a fixed
// order (declaration order above) — a caller wanting "all of them" as a
// starting Input.Definitions list. Each call returns fresh Definition
// values (Expression trees included); no shared mutable state backs this
// function, so a caller may freely modify the returned slice/Definitions
// without affecting any other caller.
func BuiltInTemplates() []Definition {
	return []Definition{
		RevenuePerFTETemplate(),
		LaborCostPercentRevenueTemplate(),
		AROver60PercentTemplate(),
		AverageTicketTemplate(),
		GrossMarginTemplate(),
		RevenuePerSquareFootTemplate(),
	}
}
